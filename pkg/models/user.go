package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents a user authenticated via SSH public key
// No passwords - SSH key is the ONLY authentication method (matches terminal.shop exactly)
type User struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	SSHKeyFingerprint string         `gorm:"uniqueIndex;size:100;not null" json:"ssh_key_fingerprint"` // SHA256 fingerprint - PRIMARY identifier
	SSHPublicKey      string         `gorm:"uniqueIndex;type:text;not null" json:"-"`                   // Full SSH public key
	Name              string         `gorm:"size:255" json:"name,omitempty"`                            // Real name (for shipping labels)
	Email             string         `gorm:"size:255" json:"email,omitempty"`                           // Email (for receipts)
	Anonymous         bool           `gorm:"default:false" json:"anonymous"`                            // True if connected without SSH key
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for the User model
func (User) TableName() string {
	return "users"
}

// PublicUser returns a user object safe for public consumption
type PublicUser struct {
	ID                uint      `json:"id"`
	SSHKeyFingerprint string    `json:"ssh_key_fingerprint"`
	Name              string    `json:"name,omitempty"`
	Email             string    `json:"email,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// ToPublic converts a User to a PublicUser
func (u *User) ToPublic() PublicUser {
	return PublicUser{
		ID:                u.ID,
		SSHKeyFingerprint: u.SSHKeyFingerprint,
		Name:              u.Name,
		Email:             u.Email,
		CreatedAt:         u.CreatedAt,
	}
}
