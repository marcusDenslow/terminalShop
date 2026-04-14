package models

import (
	"time"
)

type Cart struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UserID          uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	AddressID       *uint     `json:"address_id,omitempty"`
	CardID          *uint     `json:"card_id,omitempty"`
	ShippingCost    int       `gorm:"default:0" json:"shipping_cost"`
	ShippingService string    `gorm:"size:255" json:"shipping_service,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	User    User       `gorm:"foreignKey:UserID" json:"-"`
	Address *Address   `gorm:"foreignKey:AddressID" json:"address,omitempty"`
	Card    *Card      `gorm:"foreignKey:CardID" json:"card,omitempty"`
	Items   []CartItem `gorm:"foreignKey:CartID" json:"items"`
}

// TableName specifies the table name for the Cart model.
func (Cart) TableName() string {
	return "carts"
}

type CartItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CartID    uint      `gorm:"not null;index;uniqueIndex:idx_cart_coffee" json:"cart_id"`
	CoffeeID  uint      `gorm:"not null;uniqueIndex:idx_cart_coffee" json:"coffee_id"`
	Quantity  int       `gorm:"not null" json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Cart   Cart   `gorm:"foreignKey:CartID" json:"-"`
	Coffee Coffee `gorm:"foreignKey:CoffeeID" json:"coffee"`
}

func (CartItem) TableName() string {
	return "cart_items"
}
