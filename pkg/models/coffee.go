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
	Ounces      int            `json:"ounces"`
	BeanType    string         `gorm:"size:100" json:"bean_type"`
	Price       float64        `gorm:"not null" json:"price"`
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


// AccountMenuItems something
var AccountMenuItems = []string{
	"order history",
	"faq",
	"about",
	"something",
}
