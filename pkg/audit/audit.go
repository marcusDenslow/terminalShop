// Package audit writes structured logs for financial events.
// Every card addition, order creation, payment, and refund is recorded here.
// Pipe this output to your log aggregator (Datadog, Loki, CloudWatch, etc.).
package audit

import (
	"log/slog"
)

type eventType string

const (
	eventCardAdded       eventType = "card_added"
	eventCardDeleted     eventType = "card_deleted"
	eventOrderCreated    eventType = "order_created"
	eventOrderPaid       eventType = "order_paid"
	eventOrderShipped    eventType = "order_shipped"
	eventOrderFailed     eventType = "order_failed"
	eventOrderRefunded   eventType = "order_refunded"
	eventPaymentCritical eventType = "payment_critical"
	eventLabelPurchased  eventType = "label_purchased"
	eventLabelOrphaned   eventType = "label_orphaned"
)

type event struct {
	Event     eventType
	UserID    uint
	OrderID   uint
	CardID    uint
	Amount    int
	StripeID  string
	CardLast4 string
	CardBrand string
	Carrier   string
	Tracking  string
	Error     string
}

func (e event) attrs() []any {
	a := []any{"event", string(e.Event)}
	if e.UserID != 0 {
		a = append(a, "user_id", e.UserID)
	}
	if e.OrderID != 0 {
		a = append(a, "order_id", e.OrderID)
	}
	if e.CardID != 0 {
		a = append(a, "card_id", e.CardID)
	}
	if e.Amount != 0 {
		a = append(a, "amount_cents", e.Amount)
	}
	if e.StripeID != "" {
		a = append(a, "stripe_id", e.StripeID)
	}
	if e.CardLast4 != "" {
		a = append(a, "card_last4", e.CardLast4)
	}
	if e.CardBrand != "" {
		a = append(a, "card_brand", e.CardBrand)
	}
	if e.Carrier != "" {
		a = append(a, "carrier", e.Carrier)
	}
	if e.Tracking != "" {
		a = append(a, "tracking", e.Tracking)
	}
	if e.Error != "" {
		a = append(a, "error", e.Error)
	}
	return a
}

// CardAdded records that a user saved a new payment method.
func CardAdded(userID, cardID uint, brand, last4 string) {
	slog.Info("audit", event{
		Event:     eventCardAdded,
		UserID:    userID,
		CardID:    cardID,
		CardBrand: brand,
		CardLast4: last4,
	}.attrs()...)
}

// CardDeleted records that a user removed a saved payment method.
func CardDeleted(userID, cardID uint) {
	slog.Info("audit", event{
		Event:  eventCardDeleted,
		UserID: userID,
		CardID: cardID,
	}.attrs()...)
}

// OrderCreated records that an order record was created (before payment).
func OrderCreated(userID, orderID uint, amountCents int) {
	slog.Info("audit", event{
		Event:   eventOrderCreated,
		UserID:  userID,
		OrderID: orderID,
		Amount:  amountCents,
	}.attrs()...)
}

// OrderPaid records a successful charge.
func OrderPaid(userID, orderID uint, amountCents int, stripePaymentIntentID string) {
	slog.Info("audit", event{
		Event:    eventOrderPaid,
		UserID:   userID,
		OrderID:  orderID,
		Amount:   amountCents,
		StripeID: stripePaymentIntentID,
	}.attrs()...)
}

// OrderShipped records that an order has been marked as shipped with carrier metadata
func OrderShipped(userID, orderID uint, carrier, trackingNumber string) {
	slog.Info("audit", event{
		Event:    eventOrderShipped,
		UserID:   userID,
		OrderID:  orderID,
		Carrier:  carrier,
		Tracking: trackingNumber,
	}.attrs()...)
}

// OrderFailed records a failed charge attempt. Logged at WARN.
func OrderFailed(userID, orderID uint, amountCents int, reason string) {
	slog.Warn("audit", event{
		Event:   eventOrderFailed,
		UserID:  userID,
		OrderID: orderID,
		Amount:  amountCents,
		Error:   reason,
	}.attrs()...)
}

// OrderRefunded records a refund.
func OrderRefunded(userID, orderID uint, amountCents int, stripeRefundID string) {
	slog.Info("audit", event{
		Event:    eventOrderRefunded,
		UserID:   userID,
		OrderID:  orderID,
		Amount:   amountCents,
		StripeID: stripeRefundID,
	}.attrs()...)
}

// PaymentCritical records the dangerous state where Stripe charged the card
// but the local database transaction failed. This MUST be investigated and
// manually reconciled. Logged at ERROR so monitoring can alert on it.
func PaymentCritical(orderID uint, stripePaymentIntentID string, err error) {
	slog.Error("audit", event{
		Event:    eventPaymentCritical,
		OrderID:  orderID,
		StripeID: stripePaymentIntentID,
		Error:    err.Error(),
	}.attrs()...)
}

func LabelPurchased(orderID uint, carrier, trackingNumber, shippoTxID string, costCents int) {
	slog.Info("audit", event{
		Event:    eventLabelPurchased,
		OrderID:  orderID,
		Carrier:  carrier,
		Tracking: trackingNumber,
		StripeID: shippoTxID,
		Amount:   costCents,
	}.attrs()...)
}

func LabelOrphaned(orderID uint, shippoTxID, trackingNumber string, err error) {
	slog.Error("audit", event{
		Event:    eventLabelOrphaned,
		OrderID:  orderID,
		StripeID: shippoTxID,
		Tracking: trackingNumber,
		Error:    err.Error(),
	}.attrs()...)
}
