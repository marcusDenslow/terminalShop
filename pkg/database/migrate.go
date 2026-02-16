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
		&models.Coffee{},
		&models.User{}, // SSH key-based authentication
		// Add other models here as they're created
		// &models.Cart{},
		// &models.Order{},
	); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}
