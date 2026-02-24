package models

import (
	"time"

	"gorm.io/gorm"
)

// Address represents a saved shipping address for a user.
type Address struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	Name      string         `gorm:"size:255;not null" json:"name"`
	Street    string         `gorm:"size:255;not null" json:"street"`
	Street2   string         `gorm:"size:255" json:"street2,omitempty"`
	City      string         `gorm:"size:255;not null" json:"city"`
	State     string         `gorm:"size:255" json:"state,omitempty"`
	Zip       string         `gorm:"size:20;not null" json:"zip"`
	Country   string         `gorm:"size:2;not null" json:"country"`
	Phone     string         `gorm:"size:20" json:"phone,omitempty"`
	IsDefault bool           `gorm:"default:false" json:"is_default"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Associations
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// TableName specifies the table name for the Address model.
func (Address) TableName() string {
	return "addresses"
}
