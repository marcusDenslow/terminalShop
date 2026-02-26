package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	APIPort            string
	APIURL             string
	SSHPort            string
	Environment        string
	DatabaseURL        string
	JWTSecret          string
	StripeSecretKey    string
	StripePublicKey    string
	ShippoAPIKey       string
	AuthFingerprintKey string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file in non-production environments.
	// Try current directory first, then parent directory (for when
	// running from subdirectories like api/).
	if os.Getenv("ENVIRONMENT") != "production" {
		if err := godotenv.Load(); err != nil {
			if err := godotenv.Load("../.env"); err != nil {
				fmt.Println("No .env file found, using environment variables")
			}
		}
	}

	apiPort := getEnvOrDefault("API_PORT", "8080")
	cfg := &Config{
		APIPort:            apiPort,
		APIURL:             getEnvOrDefault("API_URL", fmt.Sprintf("http://localhost:%s", apiPort)),
		SSHPort:            getEnvOrDefault("SSH_PORT", "23457"),
		Environment:        getEnvOrDefault("ENVIRONMENT", "development"),
		DatabaseURL:        getEnvOrDefault("DATABASE_URL", "terminalshop.db"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		StripeSecretKey:    os.Getenv("STRIPE_SECRET_KEY"),
		StripePublicKey:    os.Getenv("STRIPE_PUBLIC_KEY"),
		ShippoAPIKey:       os.Getenv("SHIPPO_API_KEY"),
		AuthFingerprintKey: os.Getenv("AUTH_FINGERPRINT_KEY"),
	}

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
