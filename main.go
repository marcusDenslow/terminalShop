package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"

	"terminalShop/pkg/auth"
	"terminalShop/pkg/config"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/tui"
)

const (
	host = "localhost"
	port = "23456"
)

var authService *auth.SSHAuthService

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	// Extract SSH public key from session
	pubKey := s.PublicKey()

	var user *models.User

	if pubKey != nil {
		// Authenticate with SSH key (auto-creates user on first connect!)
		authenticatedUser, _, err := authService.AuthenticateSSHKey(pubKey)
		if err != nil {
			log.Printf("Auth error: %v", err)
			// Continue as guest
		} else {
			user = authenticatedUser
			displayName := user.SSHKeyFingerprint[:16]
			if user.Name != "" {
				displayName = user.Name
			}
			log.Printf("User authenticated: %s (ID: %d)", displayName, user.ID)
		}
	}

	// Create TUI model with user context (no registration screen needed!)
	m := tui.NewModelWithAuth(user, false, s.PublicKey())
	return m, []tea.ProgramOption{tea.WithAltScreen()}
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database connection (for user management)
	// Use same database as API server
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Run migrations (includes users table)
	if err := database.Migrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize JWT manager (uses same secret as API)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, 24*time.Hour)

	// Initialize SSH auth service
	authService = auth.NewSSHAuthService(db, jwtManager)

	// Create SSH server
	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%s", host, port)),
		wish.WithHostKeyPath(".ssh/term_info_ed25519"),
		wish.WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
			// Allow all public keys (we handle auth in the tea handler)
			return true
		}),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler),
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Starting SSH server on %s:%s", host, port)
	log.Printf("Connect with: ssh %s -p %s", host, port)

	go func() {
		if err = s.ListenAndServe(); err != nil {
			log.Fatalln(err)
		}
	}()

	<-done
	log.Println("Stopping SSH server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		log.Fatalln(err)
	}
}
