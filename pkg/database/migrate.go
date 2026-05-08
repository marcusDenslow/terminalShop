package database

import (
	"fmt"
	"log"
	"time"

	"terminalShop/pkg/models"

	"gorm.io/gorm"
)

// Migrate runs database migrations
func Migrate(db *gorm.DB) error {
	log.Println("Running database migrations...")

	// AutoMigrate will create tables, missing columns, and indexes
	// It will NOT delete unused columns or change column types
	if err := db.AutoMigrate(
		&models.Cart{},      // Server-side shopping cart
		&models.CartItem{},  // Cart line items
		&models.Coffee{},    // Coffees
		&models.User{},      // SSH key-based authentication
		&models.Card{},      // Saved payment methods
		&models.Order{},     // Completed purchases
		&models.OrderItem{}, // Line items within orders
		&models.Address{},   // Saved addresses
		&models.Shipment{},
	); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	if err := backfillShipments(db); err != nil {
		return fmt.Errorf("shipments backfill failed: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}

func backfillShipments(db *gorm.DB) error {
	if !db.Migrator().HasColumn(&models.Order{}, "carrier") {
		return nil
	}
	type legacy struct {
		ID             uint
		Carrier        string
		TrackingNumber string
		TrackingURL    string
		ShippedAt      *time.Time
	}
	var rows []legacy
	if err := db.Raw(`SELECT o.id, o.carrier, o.tracking_number, o.tracking_url, o.shipped_at
FROM orders o
WHERE o.carrier <> ''
AND NOT EXISTS(SELECT 1 FROM shipments s WHERE s.order_id = o.id)
`).Scan(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		s := models.Shipment{
			OrderID:        r.ID,
			Carrier:        r.Carrier,
			TrackingNumber: r.TrackingNumber,
			TrackingURL:    r.TrackingURL,
			Status:         models.ShipmentStatusInTransit,
			ShippedAt:      r.ShippedAt,
		}
		if err := db.Create(&s).Error; err != nil {
			return fmt.Errorf("backfill order %d: %w", r.ID, err)
		}
		log.Printf("backfilled shipment for order %d", r.ID)
	}
	return nil
}
