// Package audit writes structured logs for financial events.
// Every card addition, order creation, payment, and refund is recorded here.
// Pipe this output to your log aggregator (Datadog, Loki, CloudWatch, etc.).
package audit

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"terminalShop/pkg/models"

	"gorm.io/gorm"
)

// EventOrderRequiresAction is the audit event-type string emitted whenever a
// PaymentIntent is pushed into requires_action. Exposed so cart-flow handler
// can count retry attempts without hardcoding the string literal.
const EventOrderRequiresAction = "order_requires_action"

// EventOrder3DSAbandoned is the audit event-type string emitted by the
// abandoned-3DS sweep when it flips a stale requires_action row to failed.
// Distinct from EventOrderRequiresAction (caveat #17) so the retry-cap count
// stays decoupled from sweep activity
const EventOrder3DSAbandoned = "order_3ds_abandoned"

// EventCartRejected is the audit event-type string emitted when the per-order
// spend cap blocks a checkout BEFORE an order row is created. The event lives
// in slog only, persist() no-ops on zero order id, which is the correct shape
// here (cap rejection is upstream of order state). See caveat #13.
const EventCartRejected = "cart_rejected"

type eventType string

const (
	eventCardAdded              eventType = "card_added"
	eventCardDeleted            eventType = "card_deleted"
	eventCardExpired            eventType = "card_expired"
	eventOrderCreated           eventType = "order_created"
	eventOrderPaid              eventType = "order_paid"
	eventOrderShipped           eventType = "order_shipped"
	eventOrderFailed            eventType = "order_failed"
	eventOrderRequiresAction    eventType = EventOrderRequiresAction
	eventOrder3DSAbandoned      eventType = EventOrder3DSAbandoned
	eventCartRejected           eventType = EventCartRejected
	eventOrderRefunded          eventType = "order_refunded"
	eventPaymentCritical        eventType = "payment_critical"
	eventLabelPurchased         eventType = "label_purchased"
	eventLabelOrphaned          eventType = "label_orphaned"
	eventTrackingUpdated        eventType = "tracking_updated"
	eventOrderDelivered         eventType = "order_delivered"
	eventTrackingMarkedManually eventType = "tracking_marked_manually"
)

type event struct {
	Event      eventType
	UserID     uint
	OrderID    uint
	CardID     uint
	Amount     int
	StripeID   string
	CardLast4  string
	CardBrand  string
	Carrier    string
	Tracking   string
	Status     string
	Error      string
	Actor      string
	TotalCents int
	CapCents   int
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
	if e.Status != "" {
		a = append(a, "status", e.Status)
	}
	if e.Error != "" {
		a = append(a, "error", e.Error)
	}
	if e.Actor != "" {
		a = append(a, "actor", e.Actor)
	}
	if e.TotalCents != 0 {
		a = append(a, "total_cents", e.TotalCents)
	}
	if e.CapCents != 0 {
		a = append(a, "cap_cents", e.CapCents)
	}
	return a
}

// CardAdded records that a user saved a new payment method.
func CardAdded(userID, cardID uint, brand, last4 string) {
	e := event{
		Event:     eventCardAdded,
		UserID:    userID,
		CardID:    cardID,
		CardBrand: brand,
		CardLast4: last4,
	}
	slog.Info("audit", e.attrs()...)
	persist(0, eventCardAdded, e)
}

// CardDeleted records that a user removed a saved payment method.
func CardDeleted(userID, cardID uint) {
	e := event{
		Event:  eventCardDeleted,
		UserID: userID,
		CardID: cardID,
	}
	slog.Info("audit", e.attrs()...)
	persist(0, eventCardDeleted, e)
}

// CardExpired records that a saved payment method expired from inactivity
func CardExpired(userID, cardID uint) {
	e := event{
		Event:  eventCardExpired,
		UserID: userID,
		CardID: cardID,
	}
	slog.Info("audit", e.attrs()...)
	persist(0, eventCardExpired, e)
}

// OrderCreated records that an order record was created (before payment).
func OrderCreated(userID, orderID uint, amountCents int) {
	e := event{
		Event:   eventOrderCreated,
		UserID:  userID,
		OrderID: orderID,
		Amount:  amountCents,
	}
	slog.Info("audit", e.attrs()...)
	persist(orderID, eventOrderCreated, e)
}

// OrderPaid records a successful charge.
func OrderPaid(userID, orderID uint, amountCents int, stripePaymentIntentID string) {
	e := event{
		Event:    eventOrderPaid,
		UserID:   userID,
		OrderID:  orderID,
		Amount:   amountCents,
		StripeID: stripePaymentIntentID,
	}
	slog.Info("audit", e.attrs()...)
	persist(orderID, eventOrderPaid, e)
}

// OrderShipped records that an order has been marked as shipped with carrier metadata
func OrderShipped(userID, orderID uint, carrier, trackingNumber string) {
	e := event{
		Event:    eventOrderShipped,
		UserID:   userID,
		OrderID:  orderID,
		Carrier:  carrier,
		Tracking: trackingNumber,
	}
	slog.Info("audit", e.attrs()...)
	persist(orderID, eventOrderShipped, e)
}

// OrderFailed records a failed charge attempt. Logged at WARN.
func OrderFailed(userID, orderID uint, amountCents int, reason string) {
	e := event{
		Event:   eventOrderFailed,
		UserID:  userID,
		OrderID: orderID,
		Amount:  amountCents,
		Error:   reason,
	}
	slog.Warn("audit", e.attrs()...)
	persist(orderID, eventOrderFailed, e)
}

// OrderRefunded records a refund.
func OrderRefunded(userID, orderID uint, amountCents int, stripeRefundID string) {
	e := event{
		Event:    eventOrderRefunded,
		UserID:   userID,
		OrderID:  orderID,
		Amount:   amountCents,
		StripeID: stripeRefundID,
	}
	slog.Info("audit", e.attrs()...)
	persist(orderID, eventOrderRefunded, e)
}

// PaymentCritical records the dangerous state where Stripe charged the card
// but the local database transaction failed. This MUST be investigated and
// manually reconciled. Logged at ERROR so monitoring can alert on it.
func PaymentCritical(orderID uint, stripePaymentIntentID string, err error) {
	e := event{
		Event:    eventPaymentCritical,
		OrderID:  orderID,
		StripeID: stripePaymentIntentID,
		Error:    err.Error(),
	}
	slog.Error("audit", e.attrs()...)
	persist(orderID, eventPaymentCritical, e)
}

func LabelPurchased(orderID uint, carrier, trackingNumber, shippoTxID string, costCents int) {
	e := event{
		Event:    eventLabelPurchased,
		OrderID:  orderID,
		Carrier:  carrier,
		Tracking: trackingNumber,
		StripeID: shippoTxID,
		Amount:   costCents,
	}
	slog.Info("audit", e.attrs()...)
	persist(orderID, eventLabelPurchased, e)
}

func LabelOrphaned(orderID uint, shippoTxID, trackingNumber string, err error) {
	e := event{
		Event:    eventLabelOrphaned,
		OrderID:  orderID,
		StripeID: shippoTxID,
		Tracking: trackingNumber,
		Error:    err.Error(),
	}
	slog.Error("audit", e.attrs()...)
	persist(orderID, eventLabelOrphaned, e)
}

func TrackingUpdated(orderID uint, carrier, trackingNumber, status string) {
	e := event{
		Event:    eventTrackingUpdated,
		OrderID:  orderID,
		Carrier:  carrier,
		Tracking: trackingNumber,
		Status:   status,
	}
	slog.Info("audit", e.attrs()...)
	persist(orderID, eventTrackingUpdated, e)
}

func OrderDelivered(orderID uint, trackingNumber string) {
	e := event{
		Event:    eventOrderDelivered,
		OrderID:  orderID,
		Tracking: trackingNumber,
	}
	slog.Info("audit", e.attrs()...)
	persist(orderID, eventOrderDelivered, e)
}

// TrackingMarkedManually records operator made tracking status mark
// from the slack interactivity surface. Actor is the slack username that
// clicked the button
func TrackingMarkedManually(orderID uint, status, actor string) {
	e := event{
		Event:   eventTrackingMarkedManually,
		OrderID: orderID,
		Status:  status,
		Actor:   actor,
	}
	slog.Info("audit", e.attrs()...)
	persist(orderID, eventTrackingMarkedManually, e)
}

// db is the optional persistence target. nil = slog-only.
// set once at startup with SetDB, funcs stay slog-primary regardless.
var db *gorm.DB

// SetDB wires the audit package to a database for persistent event rows.
// call once after database.Migrate. Audit funcs still write slog when
// db is nil, so tests don't need to wire this.
func SetDB(d *gorm.DB) { db = d }

func persist(orderID uint, t eventType, e event) {
	if db == nil || orderID == 0 {
		return
	}
	actor := e.Actor
	if actor == "" {
		if e.UserID > 0 {
			actor = fmt.Sprintf("user:%d", e.UserID)
		} else {
			actor = "system"
		}
	}
	payload, err := json.Marshal(attrsToMap(e.attrs()))
	if err != nil {
		slog.Error("audit persist marshal", "error", err, "event", string(t))
		return
	}
	row := &models.OrderEvent{
		OrderID: orderID,
		Type:    string(t),
		Payload: string(payload),
		Actor:   actor,
	}
	if err := db.Create(row).Error; err != nil {
		slog.Error("audit persist write", "error", err, "event", string(t), "order_id", orderID)
	}
}

func attrsToMap(a []any) map[string]any {
	m := make(map[string]any, len(a)/2)
	for i := 0; i+1 < len(a); i += 2 {
		k, ok := a[i].(string)
		if !ok {
			continue
		}
		m[k] = a[i+1]
	}
	return m
}

func OrderRequiresAction(userID, orderID uint, paymentIntentID string) {
	e := event{
		Event:    eventOrderRequiresAction,
		UserID:   userID,
		OrderID:  orderID,
		StripeID: paymentIntentID,
	}
	slog.Info("audit", e.attrs()...)
	persist(orderID, eventOrderRequiresAction, e)
}

// Order3DSAbandoned records the abandoned-3DS sweep flipping a stale
// requires_action row to failed because Stripe still reports the underlying
// PaymentIntent as outstanding past the sweep threshold.
func Order3DSAbandoned(userID, orderID uint, paymentIntentID string) {
	e := event{
		Event:    eventOrder3DSAbandoned,
		UserID:   userID,
		OrderID:  orderID,
		StripeID: paymentIntentID,
	}
	slog.Info("audit", e.attrs()...)
	persist(orderID, eventOrder3DSAbandoned, e)
}

// CartRejected records the per-order cap blocking a checkout BEFORE an
// order row is created. Carries no order id by design (cap rejection runs
// upstream of order creation) so persist() is a no-op; this event lives in
// slog only. Counterpart to validation_over_limit Prometheus counter.
func CartRejected(userID uint, totalCents, capCents int) {
	e := event{
		Event:      eventCartRejected,
		UserID:     userID,
		TotalCents: totalCents,
		CapCents:   capCents,
	}
	slog.Info("audit", e.attrs()...)
	persist(0, eventCartRejected, e)
}
