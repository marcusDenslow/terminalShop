package database

import (
	"fmt"
	"log"
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
	); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	if err := backfillTrackingFromShipments(db); err != nil {
		return fmt.Errorf("backfill tracking failed: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}

func backfillTrackingFromShipments(db *gorm.DB) error {
	if !db.Migrator().HasTable("shipments") {
		return nil
	}
	return db.Exec(`
UPDATE orders SET
		carrier = COALESCE(NULLIF(orders.carrier, ''), s.carrier),
		tracking_number = COALESCE(NULLIF(orders.tracking_number, ''), s.tracking_number),
		tracking_url = COALESCE(NULLIF(orders.tracking_url, ''), s.tracking_url),
		shipped_at = COALESCE(orders.shipped_at, s.shipped_at)
FROM (SELECT order_id,
				carrier, tracking_number, tracking_url, shipped_at
              FROM shipments
              WHERE deleted_at IS NULL) s
WHERE orders.id = s.order_id
`).Error
}
