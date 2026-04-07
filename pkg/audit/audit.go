// Package audit writes structured JSON logs for financial events.
// Every card addition, order creation, payment, and refund is recorded here.
// Pipe this output to your log aggregator (Datadog, Loki, CloudWatch, etc.).
package audit

import (
	"encoding/json"
	"log"
	"time"
)

type eventType string

const (
	eventCardAdded       eventType = "card_added"
	eventCardDeleted     eventType = "card_deleted"
	eventOrderCreated    eventType = "order_created"
	eventOrderPaid       eventType = "order_paid"
	eventOrderFailed     eventType = "order_failed"
	eventOrderRefunded   eventType = "order_refunded"
	eventPaymentCritical eventType = "payment_critical"
)

type event struct {
	Time      time.Time         `json:"time"`
	Event     eventType         `json:"event"`
	UserID    uint              `json:"user_id,omitempty"`
	OrderID   uint              `json:"order_id,omitempty"`
	CardID    uint              `json:"card_id,omitempty"`
	Amount    int               `json:"amount_cents,omitempty"`
	StripeID  string            `json:"stripe_id,omitempty"`
	CardLast4 string            `json:"card_last4,omitempty"`
	CardBrand string            `json:"card_brand,omitempty"`
	Error     string            `json:"error,omitempty"`
	Extra     map[string]string `json:"extra,omitempty"`
}

func emit(e event) {
	e.Time = time.Now().UTC()
	b, err := json.Marshal(e)
	if err != nil {
		log.Printf("[audit] failed to marshal event: %v", err)
		return
	}
	log.Printf("[AUDIT] %s", b)
}

// CardAdded records that a user saved a new payment method.
func CardAdded(userID, cardID uint, brand, last4 string) {
	emit(event{
		Event:     eventCardAdded,
		UserID:    userID,
		CardID:    cardID,
		CardBrand: brand,
		CardLast4: last4,
	})
}

// CardDeleted records that a user removed a saved payment method.
func CardDeleted(userID, cardID uint) {
	emit(event{
		Event:  eventCardDeleted,
		UserID: userID,
		CardID: cardID,
	})
}

// OrderCreated records that an order record was created (before payment).
func OrderCreated(userID, orderID uint, amountCents int) {
	emit(event{
		Event:   eventOrderCreated,
		UserID:  userID,
		OrderID: orderID,
		Amount:  amountCents,
	})
}

// OrderPaid records a successful charge.
func OrderPaid(userID, orderID uint, amountCents int, stripePaymentIntentID string) {
	emit(event{
		Event:    eventOrderPaid,
		UserID:   userID,
		OrderID:  orderID,
		Amount:   amountCents,
		StripeID: stripePaymentIntentID,
	})
}

// OrderFailed records a failed charge attempt.
func OrderFailed(userID, orderID uint, amountCents int, reason string) {
	emit(event{
		Event:   eventOrderFailed,
		UserID:  userID,
		OrderID: orderID,
		Amount:  amountCents,
		Error:   reason,
	})
}

// OrderRefunded records a refund.
func OrderRefunded(userID, orderID uint, amountCents int, stripeRefundID string) {
	emit(event{
		Event:    eventOrderRefunded,
		UserID:   userID,
		OrderID:  orderID,
		Amount:   amountCents,
		StripeID: stripeRefundID,
	})
}

// PaymentCritical records the dangerous state where Stripe charged the card
// but the local database transaction failed. This MUST be investigated and
// manually reconciled. Emit a loud log line that monitoring can alert on.
func PaymentCritical(orderID uint, stripePaymentIntentID string, err error) {
	emit(event{
		Event:    eventPaymentCritical,
		OrderID:  orderID,
		StripeID: stripePaymentIntentID,
		Error:    err.Error(),
	})
	// Second line so it's impossible to miss in plain-text log tails.
	log.Printf("[AUDIT][CRITICAL] order %d was charged (pi=%s) but DB commit failed: %v — REQUIRES MANUAL RECONCILIATION",
		orderID, stripePaymentIntentID, err)
}
