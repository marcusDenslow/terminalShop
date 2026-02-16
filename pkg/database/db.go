package database

import (
	"fmt"
	"log"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	db   *gorm.DB
	once sync.Once
)

// Connect initializes the database connection (singleton pattern)
func Connect(dbPath string) (*gorm.DB, error) {
	var err error

	once.Do(func() {
		// Configure GORM logger for development
		config := &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		}

		// Connect to SQLite database
		db, err = gorm.Open(sqlite.Open(dbPath), config)
		if err != nil {
			err = fmt.Errorf("failed to connect to database: %w", err)
			return
		}

		log.Println("Database connection established")
	})

	return db, err
}

// GetDB returns the database instance (must call Connect first)
func GetDB() *gorm.DB {
	if db == nil {
		log.Fatal("Database not initialized. Call Connect() first.")
	}
	return db
}

// Close closes the database connection
func Close() error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	log.Println("Database connection closed")
	return nil
}

// ResetForTesting resets the singleton for testing purposes
// WARNING: Only use this in tests!
func ResetForTesting() {
	if db != nil {
		Close()
		db = nil
		once = sync.Once{}
	}
}
