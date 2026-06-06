package models

// DisplayKind is a ui agnostic color/severity token.
// The TUI (or any other renderer) maps this to a concrete style
// the model stays free of UI deps.
type DisplayKind string

const (
	DisplayKindNeutral DisplayKind = "neutral" // pending, default
	DisplayKindAccent  DisplayKind = "accent"  // paid, awaiting fulfillment
	DisplayKindBrand   DisplayKind = "brand"   // shipped / in transit
	DisplayKindSuccess DisplayKind = "success" // delivered
	DisplayKindError   DisplayKind = "error"   // cancelled, failed, refunded
	DisplayKindWarning DisplayKind = "warning" // delivery issue, returned
	DisplayKindRefund  DisplayKind = "refund"  // refund requested, awaiting review
)

// refundPending reports whether the order has an open customer refund request
// that has not yet been resolved by a terminal status change.
func (order *Order) refundPending() bool {
	if order.LastRefundRequestAt == nil {
		return false
	}
	switch order.Status {
	case OrderStatusRefunded, OrderStatusCancelled, OrderStatusFailed:
		return false
	}
	return true
}

func (order *Order) DisplayState() string {
	if order.refundPending() {
		return "Refund Requested"
	}
	switch order.Status {
	case OrderStatusPending:
		return "Awaiting payment"
	case OrderStatusRequiresAction:
		return "Awaiting authentication"
	case OrderStatusPaid:
		if order.TrackingNumber == "" {
			return "Paid * awaiting label"
		}
		// !IMPORTANT
		// Reachable only if UpdateTracking auto-promote is removed later
		if order.TrackingStatus == TrackingStatusPreTransit {
			return "Paid * ready to ship"
		}
		return "Paid"
	case OrderStatusShipped:
		switch order.TrackingStatus {
		case TrackingStatusTransit:
			return "In Transit"
		case TrackingStatusFailure:
			return "Delivery Issue"
		case TrackingStatusReturned:
			return "Returned to Sender"
		case TrackingStatusDelivered:
			// Webhook hasn't promoted Status yet, trust tracking
			return "Delivered"
		}
		return "Label Created"
	case OrderStatusDelivered:
		return "Delivered"
	case OrderStatusCancelled:
		return "Cancelled"
	case OrderStatusRefunded:
		return "Refunded"
	case OrderStatusFailed:
		return "Payment failed"
	}
	return string(order.Status)
}

func (order *Order) DisplayKind() DisplayKind {
	if order.refundPending() {
		return DisplayKindRefund
	}
	switch order.Status {
	case OrderStatusPending:
		return DisplayKindNeutral
	case OrderStatusRequiresAction:
		return DisplayKindNeutral
	case OrderStatusPaid:
		return DisplayKindAccent
	case OrderStatusShipped:
		switch order.TrackingStatus {
		case TrackingStatusFailure, TrackingStatusReturned:
			return DisplayKindWarning
		case TrackingStatusDelivered:
			return DisplayKindSuccess
		}
		return DisplayKindBrand
	case OrderStatusDelivered:
		return DisplayKindSuccess
	case OrderStatusCancelled, OrderStatusRefunded, OrderStatusFailed:
		return DisplayKindError
	}
	return DisplayKindNeutral
}
