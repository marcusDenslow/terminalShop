package models

import (
	"time"

	"gorm.io/gorm"
)

// OrderStatus represents the lifecycle of an order.
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusPaid      OrderStatus = "paid"
	OrderStatusShipped   OrderStatus = "shipped"
	OrderStatusDelivered OrderStatus = "delivered"
	OrderStatusCancelled OrderStatus = "cancelled"
	OrderStatusRefunded  OrderStatus = "refunded"
	OrderStatusFailed    OrderStatus = "failed"
)

type TrackingStatus string

const (
	TrackingStatusUnknown    TrackingStatus = ""
	TrackingStatusPreTransit TrackingStatus = "pre_transit"
	TrackingStatusTransit    TrackingStatus = "transit"
	TrackingStatusDelivered  TrackingStatus = "delivered"
	TrackingStatusReturned   TrackingStatus = "returned"
	TrackingStatusFailure    TrackingStatus = "failure"
)

// Order represents a completed purchase.
type Order struct {
	ID              uint        `gorm:"primaryKey" json:"id"`
	UserID          uint        `gorm:"not null;index" json:"user_id"`
	CardID          uint        `gorm:"not null" json:"card_id"`
	StripePaymentID string      `gorm:"size:255" json:"-"` // Stripe PaymentIntent ID
	Status          OrderStatus `gorm:"size:50;not null;default:'pending'" json:"status"`
	Subtotal        int         `gorm:"not null" json:"subtotal"`         // Amount in cents
	ShippingCost    int         `gorm:"default:0" json:"shipping_cost"`   // Amount in cents
	Total           int         `gorm:"not null" json:"total"`            // Amount in cents
	ShippingName    string      `gorm:"size:255" json:"shipping_name"`    // The name of the person the shipment is tied to
	ShippingStreet  string      `gorm:"size:255" json:"shipping_street"`  // The street address the shipment is tied to
	ShippingStreet2 string      `gorm:"size:255" json:"shipping_street2"` // The alternate street address the shipment is going to if first is valid for whatever reason
	ShippingCity    string      `gorm:"size:255" json:"shipping_city"`    // The city the shipment is going to
	ShippingState   string      `gorm:"size:50" json:"shipping_state"`    // The state the shipment is tied to
	ShippingZip     string      `gorm:"size:20" json:"shipping_zip"`      // The zip code the order is tied to
	ShippingCountry string      `gorm:"size:2" json:"shipping_country"`   // The country the shipment is tied to
	ShippingPhone   string      `gorm:"size:20" json:"shipping_phone"`    // Phone number the shipment is tied to

	// Shipment / tracking - single embedded blob (since its just one warehouse (my house))
	Carrier                 string         `gorm:"size:100" json:"carrier"`
	TrackingNumber          string         `gorm:"size:255;index" json:"tracking_number"`
	TrackingURL             string         `gorm:"size:512" json:"tracking_url"`
	ShippedAt               *time.Time     `json:"shipped_at"`
	TrackingStatus          TrackingStatus `gorm:"size:50" json:"tracking_status"`
	TrackingStatusDetails   string         `gorm:"size:512" json:"tracking_status_details"`
	TrackingStatusUpdatedAt *time.Time     `json:"tracking_status_updated_at"`

	// Shippo label shit, populated by the PurchaseLabel handler
	ShippoTransactionID *string `gorm:"size:255;uniqueIndex" json:"-"`
	LabelURL            string  `gorm:"size:512" json:"-"`
	LabelCostCents      int     `json:"-"`

	// Slack parent message ts for this order's thread (NULL if Slack notify disabled).
	SlackThreadTS *string `gorm:"size:64" json:"-"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // GORM soft-delete sentinel

	// Associations
	User  User        `gorm:"foreignKey:UserID" json:"-"`
	Card  Card        `gorm:"foreignKey:CardID" json:"-"`
	Items []OrderItem `gorm:"foreignKey:OrderID" json:"items"`
}

// TableName specifies the table name for the Order model.
func (Order) TableName() string {
	return "orders"
}

// OrderItem represents a single line item within an order.
type OrderItem struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	OrderID   uint           `gorm:"not null;index" json:"order_id"`
	CoffeeID  uint           `gorm:"not null" json:"coffee_id"`
	Name      string         `gorm:"size:255;not null" json:"name"` // Snapshot of coffee name at time of order
	Price     int            `gorm:"not null" json:"price"`         // Price in cents at time of order
	Quantity  int            `gorm:"not null" json:"quantity"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Associations
	Order  Order  `gorm:"foreignKey:OrderID" json:"-"`
	Coffee Coffee `gorm:"foreignKey:CoffeeID" json:"-"`
}

// TableName specifies the table name for the OrderItem model.
func (OrderItem) TableName() string {
	return "order_items"
}

// TotalInDollars returns the order total formatted as a float for display.
func (o *Order) TotalInDollars() float64 {
	return float64(o.Total) / 100
}
