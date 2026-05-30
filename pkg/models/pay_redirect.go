package models

import "time"

// PayRedirect maps a short token to a stripe hosted url with a TTL.
// survives api restarts so a customer mid-3Ds authentication still
// lands on the bank-hosted challenge page after a deploy or crash
type PayRedirect struct {
	Token     string    `gorm:"primaryKey;size:64" json:"token"`
	URL       string    `gorm:"type:text;not null" json:"url"`
	Purpose   string    `gorm:"size:32;not null" json:"purpose"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `gorm:"index" json:"expires_at"`
}

const (
	RedirectPurposeCheckout3DS = "checkout_3ds"
	RedirectPurposeAddCard     = "add_card"
)

func (PayRedirect) TableName() string {
	return "pay_redirects"
}
