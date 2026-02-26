package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"terminalShop/pkg/auth"
	"terminalShop/api/routes"
	"terminalShop/pkg/config"
	"terminalShop/pkg/database"
)

const version = "v0.1.0"

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database connection
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Run migrations
	if err := database.Migrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Seed database with initial data
	if err := database.Seed(db); err != nil {
		log.Fatalf("Failed to seed database: %v", err)
	}

	// Init JWT manager (same secret + duration as SSH server)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, 30*time.Minute)

	// Setup routes
	router := routes.SetupRoutes(version, cfg.StripeSecretKey, jwtManager, cfg.AuthFingerprintKey)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.APIPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Channel to listen for interrupt signals
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		log.Printf("Starting API server on port %s", cfg.APIPort)
		log.Printf("Environment: %s", cfg.Environment)
		log.Printf("Database: %s", cfg.DatabaseURL)
		log.Printf("Health check: http://localhost:%s/api/v1/health", cfg.APIPort)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	log.Println("API server started. Press Ctrl+C to shutdown.")

	// Block until signal received
	<-done
	log.Println("Server is shutting down...")

	// Graceful shutdown with 30 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}
