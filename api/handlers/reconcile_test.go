package handlers

import (
	"fmt"
	"os"
	"testing"
	"time"

	"terminalShop/api/middleware"
	"terminalShop/pkg/audit"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stripe/stripe-go/v78"
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

	flipped := middleware.Abandoned3DSSweepCounter().WithLabelValues("flipped")
	before := testutil.ToFloat64(flipped)

	ReconcileStale3DSOrders(30 * time.Minute)

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
	if got := testutil.ToFloat64(flipped) - before; got != 1 {
		t.Fatalf("want 1 flipped sweep counter increment, got %v", got)
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
	audit.SetDB(db)
	defer audit.SetDB(nil)

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

	ReconcileStale3DSOrders(30 * time.Minute)

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
	audit.SetDB(db)
	defer audit.SetDB(nil)

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

	skip := middleware.Abandoned3DSSweepCounter().WithLabelValues("succeeded_skip")
	before := testutil.ToFloat64(skip)

	ReconcileStale3DSOrders(30 * time.Minute)

	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != models.OrderStatusRequiresAction {
		t.Fatalf("must defer to webhook on succeeded; got %q", reloaded.Status)
	}
	if got := testutil.ToFloat64(skip) - before; got != 1 {
		t.Fatalf("want 1 succeeded_skip sweep counter increment, got %v", got)
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
	audit.SetDB(db)
	defer audit.SetDB(nil)

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

	noop := middleware.Abandoned3DSSweepCounter().WithLabelValues("flip_noop")
	before := testutil.ToFloat64(noop)

	ReconcileStale3DSOrders(30 * time.Minute)

	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != models.OrderStatusPaid {
		t.Fatalf("conditional update must preserve paid; got %q", reloaded.Status)
	}
	if got := testutil.ToFloat64(noop) - before; got != 1 {
		t.Fatalf("want 1 flip_noop sweep counter increment, got %v", got)
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

	ReconcileStale3DSOrders(30 * time.Minute)

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

	ReconcileStale3DSOrders(30 * time.Minute)

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

// TestReconcileStale3DSOrders_SkipsTransitioningStatuses covers the
// processing and requires_capture branches: Stripe still owns the PI's
// next move, so the sweep must not flip even past the threshold.
func TestReconcileStale3DSOrders_SkipsTransitioningStatuses(t *testing.T) {
	cases := []struct {
		name   string
		status stripe.PaymentIntentStatus
	}{
		{"processing", stripe.PaymentIntentStatusProcessing},
		{"requires_capture", stripe.PaymentIntentStatusRequiresCapture},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testDB, user := setupCartTestDB(t)
			defer func() { _ = os.Remove(testDB) }()
			defer database.ResetForTesting()

			db := database.GetDB()
			audit.SetDB(db)
			defer audit.SetDB(nil)

			order := seedRequiresActionOrder(t, user.ID,
				"pi_transitioning_"+tc.name, time.Now().Add(-1*time.Hour))

			orig := paymentIntentGetFn
			defer func() { paymentIntentGetFn = orig }()
			paymentIntentGetFn = func(id string, _ *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
				return &stripe.PaymentIntent{ID: id, Status: tc.status}, nil
			}

			skip := middleware.Abandoned3DSSweepCounter().WithLabelValues("processing_skip")
			before := testutil.ToFloat64(skip)

			ReconcileStale3DSOrders(30 * time.Minute)

			var reloaded models.Order
			if err := db.First(&reloaded, order.ID).Error; err != nil {
				t.Fatalf("reload: %v", err)
			}
			if reloaded.Status != models.OrderStatusRequiresAction {
				t.Fatalf("%s must stay requires_action; got %q", tc.name, reloaded.Status)
			}
			if got := testutil.ToFloat64(skip) - before; got != 1 {
				t.Fatalf("want 1 processing_skip increment, got %v", got)
			}
		})
	}
}

// TestReconcileStale3DSOrders_SkipsUnknownStatus covers the switch's default
// arm: an unrecognized Stripe status logs and skips rather than guessing a
// transition. Defends against new PI statuses appearing in future SDKs.
func TestReconcileStale3DSOrders_SkipsUnknownStatus(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	audit.SetDB(db)
	defer audit.SetDB(nil)

	order := seedRequiresActionOrder(t, user.ID, "pi_unknown_status",
		time.Now().Add(-1*time.Hour))

	orig := paymentIntentGetFn
	defer func() { paymentIntentGetFn = orig }()
	paymentIntentGetFn = func(id string, _ *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		return &stripe.PaymentIntent{
			ID:     id,
			Status: stripe.PaymentIntentStatus("some_new_future_status"),
		}, nil
	}

	unknown := middleware.Abandoned3DSSweepCounter().WithLabelValues("unrecognized_status")
	before := testutil.ToFloat64(unknown)

	ReconcileStale3DSOrders(30 * time.Minute)

	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != models.OrderStatusRequiresAction {
		t.Fatalf("unknown status must not flip; got %q", reloaded.Status)
	}
	if got := testutil.ToFloat64(unknown) - before; got != 1 {
		t.Fatalf("want 1 unrecognized_status increment, got %v", got)
	}
}

// TestReconcileStale3DSOrders_SkipsOnStripeError verifies a Stripe Get
// failure logs and leaves the row alone. Transient network errors must not
// look like an abandoned 3DS.
func TestReconcileStale3DSOrders_SkipsOnStripeError(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	audit.SetDB(db)
	defer audit.SetDB(nil)

	order := seedRequiresActionOrder(t, user.ID, "pi_stripe_error",
		time.Now().Add(-1*time.Hour))

	orig := paymentIntentGetFn
	defer func() { paymentIntentGetFn = orig }()
	paymentIntentGetFn = func(_ string, _ *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		return nil, fmt.Errorf("simulated stripe outage")
	}

	stripeErr := middleware.Abandoned3DSSweepCounter().WithLabelValues("stripe_error")
	before := testutil.ToFloat64(stripeErr)

	ReconcileStale3DSOrders(30 * time.Minute)

	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != models.OrderStatusRequiresAction {
		t.Fatalf("stripe error must not flip; got %q", reloaded.Status)
	}
	if got := testutil.ToFloat64(stripeErr) - before; got != 1 {
		t.Fatalf("want 1 stripe_error sweep counter increment, got %v", got)
	}
}

// TestReconcileStale3DSOrders_SkipsEmptyPaymentIntentID guards the
// defensive empty-PI short-circuit: even past the threshold, a row missing
// stripe_payment_id must not reach paymentintent.Get (which would 4xx).
func TestReconcileStale3DSOrders_SkipsEmptyPaymentIntentID(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	audit.SetDB(db)
	defer audit.SetDB(nil)

	order := seedRequiresActionOrder(t, user.ID, "pi_will_be_blanked",
		time.Now().Add(-1*time.Hour))
	if err := db.Model(&order).UpdateColumn("stripe_payment_id", "").Error; err != nil {
		t.Fatalf("blank PI id: %v", err)
	}

	called := false
	orig := paymentIntentGetFn
	defer func() { paymentIntentGetFn = orig }()
	paymentIntentGetFn = func(_ string, _ *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		called = true
		return &stripe.PaymentIntent{Status: stripe.PaymentIntentStatusRequiresPaymentMethod}, nil
	}

	missing := middleware.Abandoned3DSSweepCounter().WithLabelValues("missing_pi")
	before := testutil.ToFloat64(missing)

	ReconcileStale3DSOrders(30 * time.Minute)

	if called {
		t.Fatalf("stripe Get called for empty-PI row")
	}
	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != models.OrderStatusRequiresAction {
		t.Fatalf("empty-PI guard must leave status alone; got %q", reloaded.Status)
	}
	if got := testutil.ToFloat64(missing) - before; got != 1 {
		t.Fatalf("want 1 missing_pi increment, got %v", got)
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
