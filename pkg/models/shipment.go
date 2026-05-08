package models

import (
	"time"

	"gorm.io/gorm"
)

// ShipmentStatus represents the states a shipment can be in
type ShipmentStatus string

const (
	ShipmentStatusPending      ShipmentStatus = "pending"
	ShipmentStatusLabelCreated ShipmentStatus = "label_created"
	ShipmentStatusInTransit    ShipmentStatus = "in_transit"
	ShipmentStatusDelivered    ShipmentStatus = "delivered"
	ShipmentStatusException    ShipmentStatus = "exception"
)

// Shipment represents a single physical dispatch attached to an order
// Most orders have one, the scheme permits multiple (split shipments)
type Shipment struct {
	ID                  uint           `gorm:"primaryKey" json:"id"`
	OrderID             uint           `gorm:"not null;index" json:"order_id"`
	Carrier             string         `gorm:"size:100" json:"carrier"`
	TrackingNumber      string         `gorm:"size:255" json:"tracking_number"`
	TrackingURL         string         `gorm:"size:512" json:"tracking_url"`
	Status              ShipmentStatus `gorm:"size:50, not null; default:'pending'" json:"status"`
	ShippoTransactionID *string        `gorm:"size:255;uniqueIndex" json:"shippo_transaction_id,omitempty"`
	LabelPDFURL         string         `gorm:"size:255" json:"label_pdf_url,omitempty,omitempty"`
	CostCents           int            `gorm:"default:0" json:"cost_cents"`
	ShippedAt           *time.Time     `json:"shipped_at"`
	DeliveredAt         *time.Time     `json:"delivered_at"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`

	Order Order `gorm:"foreignKey:OrderID" json:"-"`
}

func (Shipment) TableName() string {
	return "shipments"
}
