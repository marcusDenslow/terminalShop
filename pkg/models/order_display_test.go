package models

import "testing"

func TestOrderDisplayState(t *testing.T) {
	tests := []struct {
		name  string
		order Order
		want  string
		kind  DisplayKind
	}{
		{"pending", Order{Status: OrderStatusPending}, "Awaiting payment", DisplayKindNeutral},
		{"paid no tracking", Order{Status: OrderStatusPaid}, "Paid * awaiting label", DisplayKindAccent},
		{"paid with tracking, no event", Order{Status: OrderStatusPaid, TrackingNumber: "12999"}, "Paid", DisplayKindAccent},
		{"Paid pre-transit", Order{Status: OrderStatusPaid, TrackingNumber: "12999", TrackingStatus: TrackingStatusPreTransit}, "Paid * ready to ship", DisplayKindAccent},
		{"shipped no tracking event", Order{Status: OrderStatusShipped}, "Shipped", DisplayKindBrand},
		{"shipped in transit", Order{Status: OrderStatusShipped, TrackingStatus: TrackingStatusTransit}, "Shipped * in transit", DisplayKindBrand},
		{"shipped failure", Order{Status: OrderStatusShipped, TrackingStatus: TrackingStatusFailure}, "Shipped * delivery issue", DisplayKindWarning},
		{"shipped returned", Order{Status: OrderStatusShipped, TrackingStatus: TrackingStatusReturned}, "Shipped * returned to sender", DisplayKindWarning},
		{"shipped tracking-delivered (webhook lag)", Order{Status: OrderStatusShipped, TrackingStatus: TrackingStatusDelivered}, "Delivered", DisplayKindSuccess},
		{"delivered", Order{Status: OrderStatusDelivered}, "Delivered", DisplayKindSuccess},
		{"cancelled", Order{Status: OrderStatusCancelled}, "Cancelled", DisplayKindError},
		{"refunded", Order{Status: OrderStatusRefunded}, "Refunded", DisplayKindError},
		{"failed", Order{Status: OrderStatusFailed}, "Payment failed", DisplayKindError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.order.DisplayState(); got != tc.want {
				t.Errorf("DisplayState() = %q, want %q", got, tc.want)
			}
			if got := tc.order.DisplayKind(); got != tc.kind {
				t.Errorf("DisplayKind() = %q, want %q", got, tc.kind)
			}
		})
	}
}
