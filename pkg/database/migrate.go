package database

import (
	"fmt"
	"log"

	"gorm.io/gorm"
	"terminalShop/pkg/models"
)

// Migrate runs database migrations
func Migrate(db *gorm.DB) error {
	log.Println("Running database migrations...")

	// AutoMigrate will create tables, missing columns, and indexes
	// It will NOT delete unused columns or change column types
	if err := db.AutoMigrate(
		&models.Cart{},     // Server-side shipping cart
		&models.CartItem{}, // Cart line items
		&models.Coffee{},
		&models.User{},      // SSH key-based authentication
		&models.Card{},      // Saved payment methods
		&models.Order{},     // Completed purchases
		&models.OrderItem{}, // Line items within orders
		&models.Address{},   // Saved addresses
		&models.SSHKey{},    // One of the users ssh keys
	); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}
