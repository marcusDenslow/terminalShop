package models

import (
	"encoding/json"
	"time"
)

type OrderEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	OrderID   uint      `gorm:"not null;index" json:"order_id"`
	Type      string    `gorm:"size:100;not null;index" json:"type"`
	Payload   string    `gorm:"type:text" json:"payload"`
	Actor     string    `gorm:"size:255" json:"actor"`
	CreatedAt time.Time `json:"created_at"`
}

func (OrderEvent) TableName() string {
	return "order_events"
}

func (e *OrderEvent) PayloadMap() map[string]any {
	if e.Payload == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(e.Payload), &m); err != nil {
		return nil
	}
	return m
}

func (e *OrderEvent) DisplayLabel() (string, bool) {
	switch e.Type {
	case "order_paid":
		return "Paid", true
	case "label_purchased":
		return "Label created", true
	case "order_delivered":
		return "Delivered", true
	case "order_refunded":
		return "Refunded", true
	case "order_failed":
		return "Payment failed", true
	case "tracking_updated":
		status, _ := e.PayloadMap()["status"].(string)
		return trackingLabel(status)
	}
	return "", false
}

func trackingLabel(status string) (string, bool) {
	switch status {
	case "transit":
		return "In transit", true
	case "failure":
		return "Delivery issue", true
	case "returned":
		return "Returned to sender", true
	case "pre_transit":
		return "Label scanned by carrier", true
	}
	return "", false
}
