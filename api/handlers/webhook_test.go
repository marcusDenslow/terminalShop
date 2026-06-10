package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stripe/stripe-go/v78"

	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
)

// buildPaymentFailedEvent constructs a stripe.Event whose Data.Raw decodes to
// a PaymentIntent referencing the given order via metadata. errCode is set on
// last_payment_error to drive the authentication_required short-circuit
// branch in handlePaymentIntentFailed (pass "" to omit the error block).
func buildPaymentFailedEvent(t *testing.T, piID string, orderID uint, errCode string) stripe.Event {
	t.Helper()
	pi := map[string]any{
		"id":       piID,
		"metadata": map[string]string{"order_id": fmt.Sprintf("%d", orderID)},
	}
	if errCode != "" {
		pi["last_payment_error"] = map[string]any{"code": errCode}
	}
	raw, err := json.Marshal(pi)
	if err != nil {
		t.Fatalf("marshal pi: %v", err)
	}
	return stripe.Event{
		Type: "payment_intent.payment_failed",
		Data: &stripe.EventData{Raw: raw},
	}
}

func seedOrderForWebhook(t *testing.T, userID uint, status models.OrderStatus, piID string) models.Order {
	t.Helper()
	db := database.GetDB()
	card := models.Card{
		UserID:          userID,
		StripePaymentID: fmt.Sprintf("pm_test_%s", piID),
		Last4:           "4242", Brand: "Visa", ExpMonth: 12, ExpYear: 2030,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("seed card: %v", err)
	}
	order := models.Order{
		UserID: userID, CardID: card.ID,
		StripePaymentID: piID,
		Status:          status,
		Subtotal:        500, Total: 500,
		ShippingName: "Test", ShippingStreet: "1 Main St",
		ShippingCity: "PDX", ShippingState: "OR",
		ShippingZip: "97201", ShippingCountry: "US",
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}
	return order
}

// TestHandlePaymentIntentFailed_RequiresActionFlipsToFailed verifies the
// widened predicate (IN (pending, requires_action)) catches a row in the new
// requires_action state. Without this case, a card_declined webhook arriving
// after respondRequiresAction wrote requires_action would leave the row
// stuck forever.
func TestHandlePaymentIntentFailed_RequiresActionFlipsToFailed(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	order := seedOrderForWebhook(t, user.ID, models.OrderStatusRequiresAction, "pi_test_req_action_failed")

	h := NewWebhookHandler("", "")
	evt := buildPaymentFailedEvent(t, order.StripePaymentID, order.ID, "card_declined")
	h.handlePaymentIntentFailed(context.Background(), evt)

	db := database.GetDB()
	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if reloaded.Status != models.OrderStatusFailed {
		t.Fatalf("status: want %q, got %q", models.OrderStatusFailed, reloaded.Status)
	}
}

// TestHandlePaymentIntentFailed_RequiresActionAuthRequiredShortCircuits
// verifies the existing short-circuit (note #3 in sca-psd2-compliance.md)
// still fires when the row is in requires_action and the error code is
// authentication_required. Without the short-circuit, the recovery flow
// would race the webhook and surface a momentarily-failed state to the TUI.
func TestHandlePaymentIntentFailed_RequiresActionAuthRequiredShortCircuits(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	order := seedOrderForWebhook(t, user.ID, models.OrderStatusRequiresAction, "pi_test_req_action_authreq")

	h := NewWebhookHandler("", "")
	evt := buildPaymentFailedEvent(t, order.StripePaymentID, order.ID, "authentication_required")
	h.handlePaymentIntentFailed(context.Background(), evt)

	db := database.GetDB()
	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if reloaded.Status != models.OrderStatusRequiresAction {
		t.Fatalf("auth_required must not flip requires_action; got %q", reloaded.Status)
	}
}

// TestHandlePaymentIntentFailed_PendingStillFlips guards the pre-existing
// pending -> failed transition against a regression where the widened
// predicate accidentally dropped pending from the source set.
func TestHandlePaymentIntentFailed_PendingStillFlips(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	order := seedOrderForWebhook(t, user.ID, models.OrderStatusPending, "pi_test_pending_failed")

	h := NewWebhookHandler("", "")
	evt := buildPaymentFailedEvent(t, order.StripePaymentID, order.ID, "card_declined")
	h.handlePaymentIntentFailed(context.Background(), evt)

	db := database.GetDB()
	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if reloaded.Status != models.OrderStatusFailed {
		t.Fatalf("status: want %q, got %q", models.OrderStatusFailed, reloaded.Status)
	}
}

// TestHandlePaymentIntentFailed_PaidNotTouched locks the upper boundary of
// the predicate: a paid row must NEVER be flipped to failed by a stale
// payment_failed webhook. Without this guard, a future maintainer could
// widen the IN clause to include paid (e.g. for some refund-retry flow)
// and silently break idempotency.
func TestHandlePaymentIntentFailed_PaidNotTouched(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	order := seedOrderForWebhook(t, user.ID, models.OrderStatusPaid, "pi_test_paid_untouched")

	h := NewWebhookHandler("", "")
	evt := buildPaymentFailedEvent(t, order.StripePaymentID, order.ID, "card_declined")
	h.handlePaymentIntentFailed(context.Background(), evt)

	db := database.GetDB()
	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if reloaded.Status != models.OrderStatusPaid {
		t.Fatalf("paid order was modified by payment_failed webhook: got %q", reloaded.Status)
	}
}

// TestHandlePaymentIntentFailed_MissingMetadataNoOp covers the no-order
// short-circuit so the test file exercises every early-return branch in
// handlePaymentIntentFailed touched by the predicate work.
func TestHandlePaymentIntentFailed_MissingMetadataNoOp(t *testing.T) {
	testDB, _ := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	pi := map[string]any{"id": "pi_test_no_meta"}
	raw, err := json.Marshal(pi)
	if err != nil {
		t.Fatalf("marshal pi: %v", err)
	}
	evt := stripe.Event{
		Type: "payment_intent.payment_failed",
		Data: &stripe.EventData{Raw: raw},
	}

	h := NewWebhookHandler("", "")
	h.handlePaymentIntentFailed(context.Background(), evt) // must not panic
}

func TestHandlePaymentIntentSucceeded_RequiresActionFlipsToPaid(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	order := seedOrderForWebhook(t, user.ID, models.OrderStatusRequiresAction, "pi_test_req_action_paid")

	pi := map[string]any{
		"id":       order.StripePaymentID,
		"amount":   order.Total,
		"metadata": map[string]string{"order_id": fmt.Sprintf("%d", order.ID)},
	}
	raw, err := json.Marshal(pi)
	if err != nil {
		t.Fatalf("marshal pi: %v", err)
	}
	evt := stripe.Event{
		Type: "payment_intent.succeeded",
		Data: &stripe.EventData{Raw: raw},
	}

	h := NewWebhookHandler("", "")
	h.handlePaymentIntentSucceeded(context.Background(), evt)

	db := database.GetDB()
	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if reloaded.Status != models.OrderStatusPaid {
		t.Fatalf("status: want %q, got %q", models.OrderStatusPaid, reloaded.Status)
	}
	if reloaded.StripePaymentID != order.StripePaymentID {
		t.Fatalf("stripe_payment_id: want %q, got %q", order.StripePaymentID, reloaded.StripePaymentID)
	}
}

func TestHandlePaymentIntentSucceeded_AmountMismatchDoesNotPay(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	order := seedOrderForWebhook(t, user.ID, models.OrderStatusRequiresAction, "pi_test_amount_mismatch")

	// PI claims 1 cent; seed order.Total is 500.
	pi := map[string]any{
		"id":       order.StripePaymentID,
		"amount":   1,
		"metadata": map[string]string{"order_id": fmt.Sprintf("%d", order.ID)},
	}
	raw, err := json.Marshal(pi)
	if err != nil {
		t.Fatalf("marshal pi: %v", err)
	}
	evt := stripe.Event{
		Type: "payment_intent.succeeded",
		Data: &stripe.EventData{Raw: raw},
	}

	h := NewWebhookHandler("", "")
	h.handlePaymentIntentSucceeded(context.Background(), evt)

	db := database.GetDB()
	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reloaded order: %v", err)
	}
	if reloaded.Status == models.OrderStatusPaid {
		t.Fatalf("order marked paid despite amount mismatch")
	}
}
