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
	DisplayKindWarning DisplayKind = "warning" // refund pending, delivery issue
)

func (order *Order) DisplayState() string {
	switch order.Status {
	case OrderStatusPending:
		return "Awaiting payment"
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
	switch order.Status {
	case OrderStatusPending:
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
