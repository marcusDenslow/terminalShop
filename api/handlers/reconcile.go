package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"terminalShop/api/middleware"
	"terminalShop/pkg/audit"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/notify"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/paymentintent"
	"gorm.io/gorm"
)

// paymentIntentGetFn lets tests stub the Stripe Get call without spinning
// up an httptest backend. Production callers go straight through to the
// real paymentintent.Get.
var paymentIntentGetFn = paymentintent.Get

func reconcileLog() *slog.Logger { return slog.With("component", "reconcile") }

// ReconcileOrders finds pending orders with no Stripe ID older than 10 minutes
// and checks whether Stripe has a succeeded PaymentIntent for them. This fixes
// the case where the server crashed after charging but before the DB transaction
// completed.
func ReconcileOrders(ctx context.Context) {
	db := database.GetDB().WithContext(ctx)

	var orders []models.Order
	cutoff := time.Now().Add(-10 * time.Minute)
	db.Where("status = ? AND stripe_payment_id = '' AND created_at < ?",
		models.OrderStatusPending, cutoff).Find(&orders)

	if len(orders) == 0 {
		return
	}

	reconcileLog().Info("checking pending orders", "count", len(orders))

	for _, order := range orders {
		if err := ctx.Err(); err != nil {
			reconcileLog().Info("reconcile orders cancelled mid-loop", "error", err)
			return
		}
		params := &stripe.PaymentIntentSearchParams{
			SearchParams: stripe.SearchParams{
				Context: ctx,
				Query:   fmt.Sprintf("metadata['order_id']:'%d' AND status:'succeeded'", order.ID),
			},
		}

		iter := paymentintent.Search(params)
		for iter.Next() {
			pi := iter.PaymentIntent()
			reconcileLog().Warn("found succeeded payment intent for unrecorded order — patching",
				"order_id", order.ID, "pi", pi.ID)

			txErr := db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Model(&order).Updates(map[string]any{
					"status":            models.OrderStatusPaid,
					"stripe_payment_id": pi.ID,
				}).Error; err != nil {
					return err
				}
				var cart models.Cart
				if err := tx.Where("user_id = ?", order.UserID).First(&cart).Error; err != nil {
					return nil // cart already gone, nothing to clear
				}
				if err := tx.Where("cart_id = ?", cart.ID).Delete(&models.CartItem{}).Error; err != nil {
					return err
				}
				return tx.Model(&cart).Updates(map[string]any{
					"address_id": nil,
					"card_id":    nil,
				}).Error
			})
			if txErr != nil {
				reconcileLog().Error("failed to patch order", "order_id", order.ID, "error", txErr)
			}
		}
		if err := iter.Err(); err != nil {
			reconcileLog().Error("stripe search error", "order_id", order.ID, "error", err)
		}
	}
}

// ReconcileStale3DSOrders flips requires_action orders that have been
// awaiting the customer's 3DS challenge longer than threshold to failed,
// after re-checking the PaymentIntent with Stripe so a late
// payment_intent.succeeded webhook is respected. Deliberately separate
// from ReconcileOrders: the empty-PI predicate there is load-bearing for
// the crash-mid-create case (see sca-psd2-compliance.md caveat #20).
// stripe.Key is wired once at startup in api/main.go.
func ReconcileStale3DSOrders(ctx context.Context, threshold time.Duration) {
	db := database.GetDB().WithContext(ctx)

	var orders []models.Order
	// Predicate keys off updated_at, not created_at: respondRequiresAction
	// and RetryAuth both bump updated_at via GORM auto-stamping, so every
	// customer-driven retry extends the grace window. Intentional - sweeping
	// a row the customer just re-armed would race the new challenge.
	cutoff := time.Now().Add(-threshold)
	if err := db.Where("status = ? AND updated_at < ?",
		models.OrderStatusRequiresAction, cutoff).Find(&orders).Error; err != nil {
		reconcileLog().Error("stale 3ds scan failed", "error", err)
		return
	}
	if len(orders) == 0 {
		return
	}

	reconcileLog().Info("checking stale requires_action orders", "count", len(orders))

	piParams := &stripe.PaymentIntentParams{Params: stripe.Params{Context: ctx}}
	for i := range orders {
		if err := ctx.Err(); err != nil {
			reconcileLog().Info("stale 3ds sweep cancelled mid-loop", "error", err)
			return
		}
		order := &orders[i]
		if order.StripePaymentID == "" {
			// respondRequiresAction writes stripe_payment_id and status in
			// one UPDATE so this should be unreachable; guard anyway so a
			// future regression noisily logs instead of 4xx'ing Stripe.
			reconcileLog().Warn("requires_action row missing PI id; skipping", "order_id", order.ID)
			middleware.RecordAbandoned3DSSweep("missing_pi")
			continue
		}
		actual, err := paymentIntentGetFn(order.StripePaymentID, piParams)
		if err != nil {
			reconcileLog().Error("stripe get failed",
				"order_id", order.ID, "pi", order.StripePaymentID, "error", err)
			middleware.RecordAbandoned3DSSweep("stripe_error")
			continue
		}
		switch actual.Status {
		case stripe.PaymentIntentStatusSucceeded:
			// Stripe says paid but our row still says requires_action, the
			// payment_intent.succeeded webhook has not landed yet (or was
			//  dropped). Defer to handlePaymentIntentsucceeded; it owns the
			// paid transition + cart cleanup atomically. The next sweep
			// tick re-checks and surfaces the dropped webhook if it persists
			reconcileLog().Warn("stripe reports succeeded for requires_action row; deferring to webhook",
				"order_id", order.ID, "pi", order.StripePaymentID)
			middleware.RecordAbandoned3DSSweep("succeeded_skip")
		case stripe.PaymentIntentStatusProcessing,
			stripe.PaymentIntentStatusRequiresCapture:
			reconcileLog().Info("payment intent still transitioning; skipping",
				"order_id", order.ID, "pi", order.StripePaymentID, "stripe_status", string(actual.Status))
			middleware.RecordAbandoned3DSSweep("processing_skip")
		case stripe.PaymentIntentStatusRequiresAction,
			stripe.PaymentIntentStatusRequiresPaymentMethod,
			stripe.PaymentIntentStatusRequiresConfirmation,
			stripe.PaymentIntentStatusCanceled:
			flipStale3DSToFailed(db, order, string(actual.Status))
		default:
			reconcileLog().Warn("unrecognized stripe status; skipping",
				"order_id", order.ID, "pi", order.StripePaymentID, "stripe_status", string(actual.Status))
			middleware.RecordAbandoned3DSSweep("unrecognized_status")
		}
	}
}

// flipStale3DSToFailed transitions a single stale requires_action row
// to failed. The UPDATE is conditional on status = requires_action so a
// payment_intent.succeeded webhook that landed between the stripe get and
// this write (caveat #20 race) wins and we silently no-op
func flipStale3DSToFailed(db *gorm.DB, order *models.Order, stripeStatus string) {
	// Map form so the single-field UPDATE is explicit at the call site.
	// GORM still auto-stamps updated_at; the row exits the sweep predicate
	// naturally on the next tick because status no longer matches anyway.
	res := db.Model(&models.Order{}).
		Where("id = ? AND status = ?", order.ID, models.OrderStatusRequiresAction).
		Updates(map[string]any{"status": models.OrderStatusFailed})
	if res.Error != nil {
		reconcileLog().Error("flip stale 3ds failed", "order_id", order.ID, "error", res.Error)
		middleware.RecordAbandoned3DSSweep("flip_failed")
		return
	}
	if res.RowsAffected == 0 {
		reconcileLog().Info("stale 3ds row already transitioned; sweep no-op",
			"order_id", order.ID)
		middleware.RecordAbandoned3DSSweep("flip_noop")
		return
	}
	audit.Order3DSAbandoned(order.UserID, order.ID, order.StripePaymentID)
	reconcileLog().Info("3ds abandoned order flipped to failed",
		"order_id", order.ID, "pi", order.StripePaymentID, "stripe_status", stripeStatus)
	middleware.RecordAbandoned3DSSweep("flipped")
}

// ReconcileExpiredCards removes saved cards whose retention deadline elapsed.
// Read paths filter out expired rows so users never see them; this job drives
// the Stripe detach + audit + physical row delete out of band so request
// handlers never block on Stripe API calls during a card sweep.
func ReconcileExpiredCards(ctx context.Context) {
	db := database.GetDB().WithContext(ctx)

	var cards []models.Card
	if err := db.Where(
		"storage_expires_at IS NOT NULL AND storage_expires_at <= ?",
		time.Now(),
	).Find(&cards).Error; err != nil {
		reconcileLog().Error("expired card scan failed", "error", err)
		return
	}
	if len(cards) == 0 {
		return
	}

	reconcileLog().Info("expiring inactive saved cards", "count", len(cards))
	for i := range cards {
		if err := ctx.Err(); err != nil {
			reconcileLog().Info("card sweep cancelled mid-loop", "error", err)
			return
		}
		if err := expireStoredCard(db, &cards[i]); err != nil {
			reconcileLog().Error("expire card failed", "card_id", cards[i].ID, "error", err)
		}
	}
}

// ReconcileUnshipped finds paid order orlder than 24h with no tracking
// number and posts single Slack reminder. Pure read + notif
func ReconcileUnshipped(ctx context.Context) {
	db := database.GetDB().WithContext(ctx)

	var orders []models.Order
	cutoff := time.Now().Add(-24 * time.Hour)
	err := db.Where(
		"status = ? AND (tracking_number = '' OR tracking_number IS NULL) AND created_at < ?",
		models.OrderStatusPaid, cutoff,
	).Order("created_at ASC").Find(&orders).Error
	if err != nil {
		reconcileLog().Error("unshipped query failed", "error", err)
		return
	}
	if len(orders) == 0 {
		return
	}

	reconcileLog().Info("unshipped orders found", "count", len(orders))
	notify.SlackUnshippedReminder(orders)
}
