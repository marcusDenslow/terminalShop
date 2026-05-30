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
		&models.OrderEvent{},
		&models.PayRedirect{}, // short-token -> stripe url store (10 min TTL)
	); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}
