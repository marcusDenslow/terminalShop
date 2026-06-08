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

	"terminalShop/api/handlers"
	"terminalShop/api/middleware"
	"terminalShop/api/routes"
	"terminalShop/pkg/audit"
	"terminalShop/pkg/auth"
	"terminalShop/pkg/config"
	"terminalShop/pkg/database"
	"terminalShop/pkg/observability"
	"terminalShop/pkg/stripeclient"

	"github.com/stripe/stripe-go/v78"
)

const version = "v0.1.0"

func main() {
	observability.InitLogger()
	middleware.SetBuildInfo(version)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
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

	defer func() { _ = database.Close() }()

	// Run migrations
	if err := database.Migrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	audit.SetDB(db)

	// wire the stripe SDK's package-level key once at startup so concurrent
	// reconcilers and the request path don't race on stripe.Key writes.
	// NOTE: assumes single-process — if you ever embed both the SSH and API
	// binaries together, the last writer wins and one of them silently breaks.
	stripe.Key = cfg.StripeSecretKey

	// Seed database with initial data
	if err := database.Seed(db); err != nil {
		log.Fatalf("Failed to seed database: %v", err)
	}

	otlpEndpoint := os.Getenv("OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		otlpEndpoint = "tempo:4318"
	}
	shutdown, err := observability.InitTracing(context.Background(), "terminalshop-api", otlpEndpoint)
	if err != nil {
		log.Printf("tracing init failed (non-fatal): %v", err)
	} else {
		defer func() { _ = shutdown(context.Background()) }()
	}

	stripeclient.InitOTel()

	// Init JWT manager (same secret + duration as SSH server)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, 30*time.Minute)

	// Setup routes
	router := routes.SetupRoutes(version, cfg.StripeWebhookSecret, jwtManager, cfg.AuthFingerprintKey, cfg.ShippoAPIKey, cfg.BringAPIUID, cfg.BringAPIKey, cfg.BringCustomerNumber, cfg.ShippoWebhookSecret, cfg.AppURL, cfg.SlackSigningSecret, cfg.AdminAPIKey, cfg.APIPort, cfg.MaxOrderCents)

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
		log.Printf("Database: %s", cfg.SafeDatabaseURL())
		log.Printf("Health check: http://localhost:%s/api/v1/health", cfg.APIPort)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Start reconciliation job. runs every 5 minutes to catch any orders
	// that were charged but not recorded due to a crash.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				handlers.ReconcileOrders(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		select {
		case <-time.After(30 * time.Second):
		case <-ctx.Done():
			return
		}
		handlers.ReconcileUnshipped(ctx)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				handlers.ReconcileUnshipped(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Sweep saved cards whose retention deadline elapsed. Read paths filter
	// expired rows from query results, so this sweeper only drives the Stripe
	// detach + audit + physical row delete asynchronously.
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				handlers.ReconcileExpiredCards(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		threshold := time.Duration(cfg.Abandoned3DSThresholdMinutes) * time.Minute
		for {
			select {
			case <-ticker.C:
				handlers.ReconcileStale3DSOrders(ctx, threshold)
			case <-ctx.Done():
				return
			}
		}
	}()

	log.Println("API server started. Press Ctrl+C to shutdown.")

	// Block until signal received
	<-done
	cancel()
	log.Println("Server is shutting down...")

	// Graceful shutdown with 30 second timeout
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}
