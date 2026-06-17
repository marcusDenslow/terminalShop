package models

import (
	"time"

	"gorm.io/gorm"
)

// Coffee represents a coffee product
type Coffee struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:255;not null" json:"name"`
	RoastType   string         `gorm:"size:100" json:"roast_type"`
	RoastLevel  int            `json:"roast_level"` // 1-5 roast-intensity
	Ounces      int            `json:"ounces"`
	BeanType    string         `gorm:"size:100" json:"bean_type"`
	Price       int            `gorm:"not null" json:"price"`
	Color       string         `gorm:"size:7" json:"color"` // Hex color for background
	Description string         `gorm:"type:text" json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for the Coffee model
func (Coffee) TableName() string {
	return "coffees"
}

// AccountMenuItems
var AccountMenuItems = []string{
	"active orders",
	"order history",
	"addresses",
	"cards",
	"spend limit",
	"faq",
	"about",
}
