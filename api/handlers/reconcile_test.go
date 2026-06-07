package handlers

import (
	"os"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v78"

	"terminalShop/pkg/audit"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
)

// TestReconcileStale3DSOrders_FlipsAbandonedToFailed verifies the predicate
// and Stripe-Get branching: a row past the threshold whose PaymentIntent
// Stripe still reports as requires_payment_method (the customer never
// completed the challenge) transitions to failed and emits the new
// order_3ds_abandoned audit row.
func TestReconcileStale3DSOrders_FlipsAbandonedToFailed(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	audit.SetDB(db)
	defer audit.SetDB(nil)

	order := seedRequiresActionOrder(t, user.ID, "pi_stale_abandoned",
		time.Now().Add(-1*time.Hour))

	orig := paymentIntentGetFn
	defer func() { paymentIntentGetFn = orig }()
	paymentIntentGetFn = func(id string, _ *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		return &stripe.PaymentIntent{
			ID:     id,
			Status: stripe.PaymentIntentStatusRequiresPaymentMethod,
		}, nil
	}

	ReconcileStale3DSOrders("", 30*time.Minute)

	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if reloaded.Status != models.OrderStatusFailed {
		t.Fatalf("status: want %q, got %q", models.OrderStatusFailed, reloaded.Status)
	}

	var events []models.OrderEvent
	if err := db.Where("order_id = ? AND type = ?",
		order.ID, audit.EventOrder3DSAbandoned).Find(&events).Error; err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 order_3ds_abandoned audit row, got %d", len(events))
	}
}

// TestReconcileStale3DSOrders_SkipsRecent verifies the threshold predicate
// excludes rows still within the grace window. A fresh requires_action row
// must not even reach the Stripe Get call.
func TestReconcileStale3DSOrders_SkipsRecent(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()

	order := seedRequiresActionOrder(t, user.ID, "pi_fresh_3ds",
		time.Now().Add(-5*time.Minute))

	called := false
	orig := paymentIntentGetFn
	defer func() { paymentIntentGetFn = orig }()
	paymentIntentGetFn = func(id string, _ *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		called = true
		return &stripe.PaymentIntent{
			ID:     id,
			Status: stripe.PaymentIntentStatusRequiresPaymentMethod,
		}, nil
	}

	ReconcileStale3DSOrders("", 30*time.Minute)

	if called {
		t.Fatalf("stripe Get called for in-window row")
	}
	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != models.OrderStatusRequiresAction {
		t.Fatalf("status changed unexpectedly: %q", reloaded.Status)
	}
}

// TestReconcileStale3DSOrders_SkipsWhenStripeSaysSucceeded covers the
// webhook-race guard from caveat #20. If Stripe-side already says succeeded,
// the sweep must defer to handlePaymentIntentSucceeded rather than overwrite
// the authoritative end state.
func TestReconcileStale3DSOrders_SkipsWhenStripeSaysSucceeded(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()

	order := seedRequiresActionOrder(t, user.ID, "pi_succeeded_late_webhook",
		time.Now().Add(-1*time.Hour))

	orig := paymentIntentGetFn
	defer func() { paymentIntentGetFn = orig }()
	paymentIntentGetFn = func(id string, _ *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		return &stripe.PaymentIntent{
			ID:     id,
			Status: stripe.PaymentIntentStatusSucceeded,
		}, nil
	}

	ReconcileStale3DSOrders("", 30*time.Minute)

	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != models.OrderStatusRequiresAction {
		t.Fatalf("must defer to webhook on succeeded; got %q", reloaded.Status)
	}
}

// TestReconcileStale3DSOrders_GuardsAgainstWebhookRace covers the conditional
// UPDATE. If the row flipped to paid between the Stripe Get and the flip,
// the WHERE status = requires_action clause prevents clobber.
func TestReconcileStale3DSOrders_GuardsAgainstWebhookRace(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()

	order := seedRequiresActionOrder(t, user.ID, "pi_race_paid",
		time.Now().Add(-1*time.Hour))

	orig := paymentIntentGetFn
	defer func() { paymentIntentGetFn = orig }()
	paymentIntentGetFn = func(id string, _ *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		db.Model(&models.Order{}).Where("id = ?", order.ID).
			Update("status", models.OrderStatusPaid)
		return &stripe.PaymentIntent{
			ID:     id,
			Status: stripe.PaymentIntentStatusRequiresPaymentMethod,
		}, nil
	}

	ReconcileStale3DSOrders("", 30*time.Minute)

	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != models.OrderStatusPaid {
		t.Fatalf("conditional update must preserve paid; got %q", reloaded.Status)
	}
}

// TestReconcileStale3DSOrders_FlipsCanceledPI covers the canceled-PI branch:
// Stripe-terminal-canceled maps to OrderStatusFailed (matches RetryAuth's
// existing precedent — see api/handlers/cart.go RetryAuth canceled branch).
func TestReconcileStale3DSOrders_FlipsCanceledPI(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	audit.SetDB(db)
	defer audit.SetDB(nil)

	order := seedRequiresActionOrder(t, user.ID, "pi_canceled_stale",
		time.Now().Add(-1*time.Hour))

	orig := paymentIntentGetFn
	defer func() { paymentIntentGetFn = orig }()
	paymentIntentGetFn = func(id string, _ *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		return &stripe.PaymentIntent{
			ID:     id,
			Status: stripe.PaymentIntentStatusCanceled,
		}, nil
	}

	ReconcileStale3DSOrders("", 30*time.Minute)

	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != models.OrderStatusFailed {
		t.Fatalf("canceled PI must flip to failed; got %q", reloaded.Status)
	}
}

// TestReconcileStale3DSOrders_DoesNotConsumeRetryCap regression-tests the
// caveat #17 decoupling: the new sweep emits EventOrder3DSAbandoned, never
// EventOrderRequiresAction, so a swept-then-stuck order does not silently
// shrink the retry ceiling for any future user-driven retry attempts.
func TestReconcileStale3DSOrders_DoesNotConsumeRetryCap(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	audit.SetDB(db)
	defer audit.SetDB(nil)

	order := seedRequiresActionOrder(t, user.ID, "pi_decoupled_cap",
		time.Now().Add(-1*time.Hour))

	orig := paymentIntentGetFn
	defer func() { paymentIntentGetFn = orig }()
	paymentIntentGetFn = func(id string, _ *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		return &stripe.PaymentIntent{
			ID:     id,
			Status: stripe.PaymentIntentStatusRequiresPaymentMethod,
		}, nil
	}

	ReconcileStale3DSOrders("", 30*time.Minute)

	var retryCount int64
	if err := db.Model(&models.OrderEvent{}).
		Where("order_id = ? AND type = ?", order.ID, audit.EventOrderRequiresAction).
		Count(&retryCount).Error; err != nil {
		t.Fatalf("count retry events: %v", err)
	}
	if retryCount != 0 {
		t.Fatalf("sweep must not emit EventOrderRequiresAction; got %d", retryCount)
	}
}

// seedRequiresActionOrder inserts a card + order pair in the requires_action
// state. updatedAt is forced via UpdateColumn so GORM's auto-stamping of
// UpdatedAt on Create does not bump the row out of the sweep predicate.
func seedRequiresActionOrder(t *testing.T, userID uint, piID string, updatedAt time.Time) models.Order {
	t.Helper()
	db := database.GetDB()
	card := models.Card{
		UserID:          userID,
		StripePaymentID: "pm_test_" + piID,
		Last4:           "4242", Brand: "Visa",
		ExpMonth: 12, ExpYear: 2030,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("seed card: %v", err)
	}
	order := models.Order{
		UserID: userID, CardID: card.ID,
		StripePaymentID: piID,
		Status:          models.OrderStatusRequiresAction,
		Subtotal:        500, Total: 500,
		ShippingName: "Test", ShippingStreet: "1 Main St",
		ShippingCity: "PDX", ShippingState: "OR",
		ShippingZip: "97201", ShippingCountry: "US",
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}
	if err := db.Model(&order).UpdateColumn("updated_at", updatedAt).Error; err != nil {
		t.Fatalf("backdate updated_at: %v", err)
	}
	return order
}
