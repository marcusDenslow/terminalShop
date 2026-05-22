package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/paymentmethod"
	"github.com/stripe/stripe-go/v78/setupintent"
	"github.com/stripe/stripe-go/v78/webhook"
	"gorm.io/gorm"

	"terminalShop/pkg/audit"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/notify"
	"terminalShop/pkg/utils"
)

func webhookLog() *slog.Logger { return slog.With("component", "webhook") }

// WebhookHandler handles inbound Stripe webhook events.
type WebhookHandler struct {
	webhookSecret string
	stripeKey     string
	shippoSecret  string
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(webhookSecret, stripeKey, shippoSecret string) *WebhookHandler {
	return &WebhookHandler{webhookSecret: webhookSecret, stripeKey: stripeKey, shippoSecret: shippoSecret}
}

// HandleStripe validates the Stripe signature and routes the event.
// This endpoint is intentionally unauthenticated — authentication is
// provided by the webhook signature (HMAC-SHA256).
func (h *WebhookHandler) HandleStripe(w http.ResponseWriter, r *http.Request) {
	// Read the entire body up front; webhook.ConstructEvent requires it.
	const maxBodyBytes = 65536
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "READ_ERROR", "could not read request body", nil)
		return
	}

	// Verify the Stripe signature. Reject anything that doesn't match.
	if h.webhookSecret == "" {
		// No secret configured — log a loud warning but still process in dev.
		webhookLog().Warn("stripe webhook secret not set; skipping signature verification")
	} else {
		sig := r.Header.Get("Stripe-Signature")
		if _, err := webhook.ConstructEventWithOptions(body, sig, h.webhookSecret, webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		}); err != nil {
			webhookLog().Warn("signature verification failed", "error", err)
			utils.RespondError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "webhook signature verification failed", nil)
			return
		}
	}

	var event stripe.Event
	if err := json.Unmarshal(body, &event); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_JSON", "could not parse event", nil)
		return
	}

	switch event.Type {

	case "checkout.session.completed":
		h.handleCheckoutSessionCompleted(r.Context(), event)

	// payment_intent.succeeded fires when a charge completes asynchronously
	// (e.g. 3DS, bank redirects). For confirm=true intents this is normally
	// already handled by ConvertCart, but the webhook is the authoritative source.
	case "payment_intent.succeeded":
		h.handlePaymentIntentSucceeded(r.Context(), event)

	// payment_intent.payment_failed fires when a payment fails after initial
	// confirmation (e.g. bank decline on a deferred 3DS flow).
	case "payment_intent.payment_failed":
		h.handlePaymentIntentFailed(r.Context(), event)

	// Sync local card state when Stripe's side changes.
	case "payment_method.attached",
		"payment_method.detached",
		"payment_method.updated":
		// Handled by the reconcile job and direct API calls; log only.
		webhookLog().Info("payment method event received", "event_type", string(event.Type))
	}

	// Always return 200 so Stripe doesn't retry.
	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) handlePaymentIntentSucceeded(ctx context.Context, event stripe.Event) {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		webhookLog().Error("failed to unmarshal payment_intent", "error", err, "event", "payment_intent.succeeded")
		return
	}

	orderIDStr, ok := pi.Metadata["order_id"]
	if !ok {
		// PaymentIntent not tied to a local order — nothing to do.
		return
	}

	db := database.GetDB().WithContext(ctx)
	var order models.Order
	if err := db.Preload("Items").Where("id = ?", orderIDStr).First(&order).Error; err != nil {
		webhookLog().Warn("order not found for payment intent", "order_id", orderIDStr, "pi", pi.ID)
		return
	}

	if order.Status == models.OrderStatusPaid {
		// Already recorded — idempotent.
		return
	}

	txErr := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status":            models.OrderStatusPaid,
			"stripe_payment_id": pi.ID,
		}).Error; err != nil {
			return err
		}
		// Clear the user's cart if it hasn't been cleared yet.
		var cart models.Cart
		if err := tx.Where("user_id = ?", order.UserID).First(&cart).Error; err != nil {
			return nil // Cart already gone.
		}
		if err := tx.Where("cart_id = ?", cart.ID).Delete(&models.CartItem{}).Error; err != nil {
			return fmt.Errorf("failed to delete cart item: %w", err)
		}
		return tx.Model(&cart).Updates(map[string]interface{}{
			"address_id": nil,
			"card_id":    nil,
		}).Error
	})
	if txErr != nil {
		webhookLog().Error("failed to record paid status",
			"order_id", orderIDStr, "pi", pi.ID, "error", txErr)
		return
	}

	audit.OrderPaid(order.UserID, order.ID, int(pi.Amount), pi.ID)
	go notify.SlackOrderPaid(&order)
	webhookLog().Info("order marked paid", "order_id", orderIDStr, "pi", pi.ID)
}

func (h *WebhookHandler) handlePaymentIntentFailed(ctx context.Context, event stripe.Event) {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		webhookLog().Error("failed to unmarshal payment_intent", "error", err, "event", "payment_intent.payment_failed")
		return
	}

	orderIDStr, ok := pi.Metadata["order_id"]
	if !ok {
		return
	}

	db := database.GetDB().WithContext(ctx)
	if err := db.Model(&models.Order{}).Where("id = ? AND status = ?", orderIDStr, models.OrderStatusPending).
		Update("status", models.OrderStatusFailed).Error; err != nil {
		webhookLog().Error("failed to mark order failed", "order_id", orderIDStr, "error", err)
	}

	errMsg := "unknown"
	if pi.LastPaymentError != nil {
		errMsg = pi.LastPaymentError.Msg
	}
	webhookLog().Warn("payment failed", "order_id", orderIDStr, "pi", pi.ID, "reason", errMsg)
}

func (h *WebhookHandler) handleCheckoutSessionCompleted(ctx context.Context, event stripe.Event) {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		webhookLog().Error("failed to unmarshal checkout session", "error", err)
		return
	}

	if sess.Mode != stripe.CheckoutSessionModeSetup {
		return
	}

	if sess.SetupIntent == nil || sess.Customer == nil {
		webhookLog().Warn("checkout session missing setup_intent or customer")
		return
	}

	stripe.Key = h.stripeKey

	si, err := setupintent.Get(sess.SetupIntent.ID, nil)
	if err != nil {
		webhookLog().Error("failed to get setup intent", "setup_intent", sess.SetupIntent.ID, "error", err)
		return
	}

	if si.PaymentMethod == nil {
		webhookLog().Warn("setup intent has no payment method", "setup_intent", si.ID)
		return
	}

	pm, err := paymentmethod.Get(si.PaymentMethod.ID, nil)
	if err != nil {
		webhookLog().Error("failed to get payment method", "payment_method", si.PaymentMethod.ID, "error", err)
		return
	}

	db := database.GetDB().WithContext(ctx)
	var user models.User
	if err := db.Where("stripe_customer_id = ?", sess.Customer.ID).First(&user).Error; err != nil {
		webhookLog().Warn("user not found for customer", "customer", sess.Customer.ID, "error", err)
		return
	}

	// Idempotency: skip if this exact PaymentMethod is already in use
	var existing models.Card
	if err := db.Where("user_id = ? AND stripe_payment_id = ?", user.ID, pm.ID).First(&existing).Error; err == nil {
		return
	}

	// dedup against local cards table, matching last4 + brand + expiry means
	// the user is re-adding the same physical card. detach the new pm from
	// stripe and skip the local insert. Local DB is the source of truth
	// the stripe customers paymentmethods list may contain orphans from before
	// this fix, which would cause false-positive dedup hits
	if pm.Card != nil {
		var dup models.Card
		err := db.Where(
			"user_id = ? AND last4 = ? AND brand = ? AND exp_month = ? AND exp_year = ?",
			user.ID, pm.Card.Last4, string(pm.Card.Brand),
			int(pm.Card.ExpMonth), int(pm.Card.ExpYear),
		).First(&dup).Error
		if err == nil {
			if _, derr := paymentmethod.Detach(pm.ID, nil); derr != nil {
				webhookLog().Warn("failed to detach duplicate pm", "user_id", user.ID, "error", derr)
			}
			webhookLog().Info("skipped duplicate card via local-db match", "user_id", user.ID, "existing_card_id", dup.ID)
			return
		}
	}

	card := models.Card{
		UserID:          user.ID,
		StripePaymentID: pm.ID,
		Last4:           pm.Card.Last4,
		Brand:           string(pm.Card.Brand),
		ExpMonth:        int(pm.Card.ExpMonth),
		ExpYear:         int(pm.Card.ExpYear),
	}

	if err := db.Create(&card).Error; err != nil {
		webhookLog().Error("failed to save card", "user_id", user.ID, "error", err)
		return
	}

	audit.CardAdded(user.ID, card.ID, card.Brand, card.Last4)
	webhookLog().Info("card saved via checkout.session.completed", "user_id", user.ID)
}

func (h *WebhookHandler) HandleShippo(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if h.shippoSecret == "" || token != h.shippoSecret {
		webhookLog().Warn("shippo webhook: token missing or invalid")
		utils.RespondError(w, http.StatusUnauthorized, "INVALID_TOKEN", "token missing or invalid", nil)
		return
	}

	const maxBodyBytes = 65536
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "READ_ERROR", "could not read request body", nil)
		return
	}

	var payload struct {
		Event string `json:"event"`
		Data  struct {
			TrackingNumber string `json:"tracking_number"`
			Carrier        string `json:"carrier"`
			TrackingStatus struct {
				Status        string    `json:"status"`
				StatusDetails string    `json:"status_details"`
				StatusDate    time.Time `json:"status_date"`
			} `json:"tracking_status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_JSON", "could not parse event", nil)
		return
	}

	if payload.Event != "track_updated" {
		webhookLog().Info("shippo webhook: ignoring no-tracking event", "event", payload.Event)
		writeShippoResponse(w, map[string]any{
			"received": true,
			"skipped":  "non-tracking event",
			"event":    payload.Event,
		})
		return
	}

	newStatus, known := mapShippoStatus(payload.Data.TrackingStatus.Status)
	if !known {
		webhookLog().Warn("shippo webhook: unrecognized status, skipping",
			"status", payload.Data.TrackingStatus.Status,
			"tracking_number", payload.Data.TrackingNumber)
		writeShippoResponse(w, map[string]any{
			"received":   true,
			"skipped":    "unrecognized status",
			"raw_status": payload.Data.TrackingStatus.Status,
		})
		return
	}

	db := database.GetDB().WithContext(r.Context())
	var order models.Order
	if err := db.Where("tracking_number = ?", payload.Data.TrackingNumber).First(&order).Error; err != nil {
		webhookLog().Warn("shippo webhook: order not found",
			"tracking_number", payload.Data.TrackingNumber)
		writeShippoResponse(w, map[string]any{
			"received":        true,
			"skipped":         "order not found",
			"tracking_number": payload.Data.TrackingNumber,
		})
		return
	}

	if order.TrackingStatusUpdatedAt != nil && !payload.Data.TrackingStatus.StatusDate.After(*order.TrackingStatusUpdatedAt) {
		writeShippoResponse(w, map[string]any{
			"received":                true,
			"skipped":                 "stale status_date",
			"order_id":                order.ID,
			"current_tracking_status": string(order.TrackingStatus),
		})
		return
	}

	statusDate := payload.Data.TrackingStatus.StatusDate
	previousTrackingStatus := order.TrackingStatus
	updates := map[string]any{
		"tracking_status":            newStatus,
		"tracking_status_details":    payload.Data.TrackingStatus.StatusDetails,
		"tracking_status_updated_at": &statusDate,
	}
	deliveredTransition := newStatus == models.TrackingStatusDelivered &&
		order.Status != models.OrderStatusDelivered
	if deliveredTransition {
		updates["status"] = models.OrderStatusDelivered
	}

	if err := db.Model(&order).Updates(updates).Error; err != nil {
		webhookLog().Error("shippo webhook: failed to persist tracking update",
			"order_id", order.ID, "error", err)
		utils.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "failed to persist", nil)
		return
	}

	if previousTrackingStatus != newStatus {
		audit.TrackingUpdated(order.ID, order.Carrier, payload.Data.TrackingNumber, string(newStatus))
		go notify.SlackPostToOrderThread(order.ID, formatTrackingUpdate(newStatus, payload.Data.TrackingStatus.StatusDetails))
	}
	if deliveredTransition {
		audit.OrderDelivered(order.ID, payload.Data.TrackingNumber)
	}
	writeShippoResponse(w, map[string]any{
		"received":                 true,
		"order_id":                 order.ID,
		"tracking_status":          string(newStatus),
		"previous_tracking_status": string(previousTrackingStatus),
		"lifecycle_status":         string(order.Status),
		"delivered_transition":     deliveredTransition,
		"status_details":           payload.Data.TrackingStatus.StatusDetails,
	})
}

func formatTrackingUpdate(status models.TrackingStatus, details string) string {
	var icon, label string
	switch status {
	case models.TrackingStatusPreTransit:
		icon, label = ":label:", "Label created"
	case models.TrackingStatusTransit:
		icon, label = ":truck:", "In transit"
	case models.TrackingStatusDelivered:
		icon, label = ":white_check_mark:", "Delivered"
	case models.TrackingStatusReturned:
		icon, label = ":leftwards_arrow_with_hook:", "Returned to sender"
	case models.TrackingStatusFailure:
		icon, label = ":warning:", "Delivery exception"
	default:
		icon, label = ":package:", string(status)
	}
	if details != "" {
		return fmt.Sprintf("%s *%s* — %s", icon, label, details)
	}
	return fmt.Sprintf("%s *%s*", icon, label)
}

func mapShippoStatus(s string) (models.TrackingStatus, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "UNKNOWN":
		return models.TrackingStatusUnknown, true
	case "PRE_TRANSIT":
		return models.TrackingStatusPreTransit, true
	case "TRANSIT":
		return models.TrackingStatusTransit, true
	case "DELIVERED":
		return models.TrackingStatusDelivered, true
	case "RETURNED":
		return models.TrackingStatusReturned, true
	case "FAILURE":
		return models.TrackingStatusFailure, true
	}
	return models.TrackingStatusUnknown, false
}

func writeShippoResponse(w http.ResponseWriter, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(payload)
}
