package database

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gorm.io/plugin/opentelemetry/tracing"
)

var (
	db   *gorm.DB
	once sync.Once
)

// Connect initializes the database connection (singleton pattern).
// If dsn starts with postgres:// or postgresql://, connects to PostgreSQL.
// Otherwise falls back to SQLite (development only).
func Connect(dsn string) (*gorm.DB, error) {
	var err error

	once.Do(func() {
		config := &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		}

		isPostgres := strings.HasPrefix(dsn, "postgres://") ||
			strings.HasPrefix(dsn, "postgresql://")

		if isPostgres {
			db, err = gorm.Open(postgres.Open(dsn), config)
		} else {
			db, err = gorm.Open(sqlite.Open(dsn), config)
		}
		if err != nil {
			err = fmt.Errorf("failed to connect to database: %w", err)
			return
		}

		if pluginErr := db.Use(tracing.NewPlugin(
			tracing.WithoutMetrics(),
			tracing.WithoutQueryVariables(),
		)); pluginErr != nil {
			err = fmt.Errorf("failed to register otel plugin: %w", pluginErr)
			return
		}

		// Configure connection pool for PostgreSQL.
		if isPostgres {
			sqlDB, poolErr := db.DB()
			if poolErr != nil {
				err = fmt.Errorf("failed to get sql.DB: %w", poolErr)
				return
			}
			sqlDB.SetMaxOpenConns(25)
			sqlDB.SetMaxIdleConns(5)
			sqlDB.SetConnMaxLifetime(5 * time.Minute)
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
