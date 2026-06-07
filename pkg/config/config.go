package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	APIPort                      string
	APIURL                       string
	AppURL                       string
	SSHPort                      string
	Environment                  string
	DatabaseURL                  string
	JWTSecret                    string
	StripeSecretKey              string
	StripePublicKey              string
	StripeWebhookSecret          string
	ShippoAPIKey                 string
	AuthFingerprintKey           string
	BringAPIUID                  string
	BringAPIKey                  string
	BringCustomerNumber          string
	ShippoWebhookSecret          string
	SlackSigningSecret           string
	AdminAPIKey                  string
	MaxOrderCents                int
	Abandoned3DSThresholdMinutes int
}

// Load reads configuration from environment variables and validates required secrets.
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
	apiURL := getEnvOrDefault("API_URL", fmt.Sprintf("http://localhost:%s", apiPort))
	cfg := &Config{
		APIPort:                      apiPort,
		APIURL:                       apiURL,
		AppURL:                       getEnvOrDefault("APP_URL", apiURL),
		SSHPort:                      getEnvOrDefault("SSH_PORT", "23457"),
		Environment:                  getEnvOrDefault("ENVIRONMENT", "development"),
		DatabaseURL:                  getEnvOrDefault("DATABASE_URL", "terminalshop.db"),
		JWTSecret:                    os.Getenv("JWT_SECRET"),
		StripeSecretKey:              os.Getenv("STRIPE_SECRET_KEY"),
		StripePublicKey:              os.Getenv("STRIPE_PUBLIC_KEY"),
		StripeWebhookSecret:          os.Getenv("STRIPE_WEBHOOK_SECRET"),
		ShippoAPIKey:                 os.Getenv("SHIPPO_API_KEY"),
		AuthFingerprintKey:           os.Getenv("AUTH_FINGERPRINT_KEY"),
		BringAPIUID:                  os.Getenv("BRING_API_UID"),
		BringAPIKey:                  os.Getenv("BRING_API_KEY"),
		BringCustomerNumber:          os.Getenv("BRING_CUSTOMER_NUMBER"),
		ShippoWebhookSecret:          os.Getenv("SHIPPO_WEBHOOK_SECRET"),
		SlackSigningSecret:           os.Getenv("SLACK_SIGNING_SECRET"),
		AdminAPIKey:                  os.Getenv("ADMIN_API_KEY"),
		MaxOrderCents:                loadMaxOrderCents(),
		Abandoned3DSThresholdMinutes: loadAbandoned3DSThresholdMinutes(),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate checks that all required secrets are present and well-formed.
// In production, it is strict; in development it warns but does not fail.
func (c *Config) validate() error {
	type check struct {
		name  string
		value string
	}
	required := []check{
		{"JWT_SECRET", c.JWTSecret},
		{"STRIPE_SECRET_KEY", c.StripeSecretKey},
		{"AUTH_FINGERPRINT_KEY", c.AuthFingerprintKey},
	}

	var missing []string
	for _, ch := range required {
		if strings.TrimSpace(ch.value) == "" {
			missing = append(missing, ch.name)
		}
	}

	if len(missing) > 0 {
		msg := fmt.Sprintf("missing required environment variables: %s", strings.Join(missing, ", "))
		if c.Environment == "production" {
			return errors.New(msg)
		}
		// Development: warn but continue so the server is still runnable without full config.
		fmt.Printf("WARNING: %s\n", msg)
	}

	// Warn if the Stripe key looks like a test key in production.
	if c.Environment == "production" && strings.HasPrefix(c.StripeSecretKey, "sk_test_") {
		fmt.Println("WARNING: STRIPE_SECRET_KEY is a test key in a production environment")
	}

	return nil
}

// SafeDatabaseURL returns the database URL with the password masked for logging.
func (c *Config) SafeDatabaseURL() string {
	parsed, err := url.Parse(c.DatabaseURL)
	if err != nil || parsed.User == nil {
		return c.DatabaseURL
	}
	if _, hasPassword := parsed.User.Password(); hasPassword {
		parsed.User = url.UserPassword(parsed.User.Username(), "***")
	}
	return parsed.String()
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// loadMaxOrderCents reads MAX_ORDER_CENTS. Default $200. Explicit 0 disables
// the cap. Garbage or negative values fall back to the default with a warning
// so an operator typo cannot silently disable a fraud control.
func loadMaxOrderCents() int {
	const defaultCap = 20000
	raw := os.Getenv("MAX_ORDER_CENTS")
	if raw == "" {
		return defaultCap
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		fmt.Printf("WARNING: MAX_ORDER_CENTS=%q invalid, falling back to default %d\n", raw, defaultCap)
		return defaultCap
	}
	return n
}

// loadAbandoned3DSThresholdMinutes read ABANDONED_3DS_THRESHOLD_MINUTES.
// The abandoned-3DS sweep only flips requires_action rows whose updated_at iconfigs
// older than this many minutes. Default 30 min. Values below 1 fall back to
// the default so a typo cannot collapse the grace window the TUI polls inside.
func loadAbandoned3DSThresholdMinutes() int {
	const defaultMinutes = 30
	raw := os.Getenv("ABANDONED_3DS_THRESHOLD_MINUTES")
	if raw == "" {
		return defaultMinutes
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		slog.Warn("invalid ABANDONED_3DS_THRESHOLD_MINUTES, falling back",
			"raw", raw, "default", defaultMinutes)
		return defaultMinutes
	}
	if n < 10 {
		slog.Warn("ABANDONED_3DS_THRESHOLD_MINUTES below 10 may race in-flight 3ds challenges",
			"value", n)
	}
	return n
}
