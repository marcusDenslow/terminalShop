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
	"terminalShop/pkg/tui"
)

const (
	host = "localhost"
	port = "23456"
)

var (
	apiURL             string
	authFingerprintKey string
)

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	// Extract SSH public key from session
	pubKey := s.PublicKey()

	var fingerprint, pubKeyStr string 
	if pubKey != nil {
		fingerprint = auth.GetSSHKeyFingerprint(pubKey)
		pubKeyStr = auth.FormatSSHPublicKey(pubKey)
	}

	// Create TUI model with user context (no registration screen needed!)
	m := tui.NewModelWithAuth(fingerprint, pubKeyStr, apiURL, authFingerprintKey)
	return m, []tea.ProgramOption{tea.WithAltScreen()}
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Set API URL and auth key from config for TUI connections
	apiURL = cfg.APIURL
	authFingerprintKey = cfg.AuthFingerprintKey
	log.Printf("TUI will connect to API at: %s", apiURL)

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
