package models

import "time"

type OrderEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	OrderID   uint      `gorm:"not null;index" json:"order_id"`
	Type      string    `gorm:"size:100;not null;index" json:"type"`
	Payload   string    `gorm:"type:text" json:"payload"`
	Actor     string    `gorm:"size:255" json:"actor"`
	CreatedAt time.Time `json:"created_at"`
}

func (OrderEvent) TableName() string {
	return "order_events"
}
