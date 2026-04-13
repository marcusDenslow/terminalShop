package models

import (
	"time"

	"gorm.io/gorm"
)

// SSHKey represents one of a users registered SSH public keys.
// A user may have multiple keys, any of them grants access.
type SSHKey struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      uint           `gorm:"not null;index" json:"user_id"`
	Fingerprint string         `gorm:"uniqueIndex;size:100;not null" json:"fingerprint"`
	PublicKey   string         `gorm:"type:text" json:"-"`
	Comment     string         `gorm:"size:255" json:"comment"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Associations
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// TableName specifies the table name for the SSHKey model.
func (SSHKey) TableName() string {
	return "ssh_keys"
}
