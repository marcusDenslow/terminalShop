package models

import (
	"time"

	"gorm.io/gorm"
)

// Card represents a saved payment method linked to a Stripe token.
// The actual card details live on Stripe's servers — we only store
// a reference and display info (last 4 digits, brand).
type Card struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	UserID          uint           `gorm:"not null;index" json:"user_id"`
	StripePaymentID string         `gorm:"size:255;not null" json:"-"`   // Stripe payment method or token ID
	Last4           string         `gorm:"size:4;not null" json:"last4"` // Last 4 digits for display
	Brand           string         `gorm:"size:50" json:"brand"`         // Visa, Mastercard, etc.
	ExpMonth        int            `gorm:"not null" json:"exp_month"`
	ExpYear         int            `gorm:"not null" json:"exp_year"`
	IsDefault       bool           `gorm:"default:false" json:"is_default"` // User's preferred card
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	// Associations
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// TableName specifies the table name for the Card model.
func (Card) TableName() string {
	return "cards"
}
