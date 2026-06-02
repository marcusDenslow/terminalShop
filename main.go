// version: 0.1.1
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/wish/v2"
	"charm.land/wish/v2/bubbletea"
	"charm.land/wish/v2/logging"
	"github.com/charmbracelet/ssh"

	"terminalShop/pkg/auth"
	"terminalShop/pkg/config"
	"terminalShop/pkg/tui"
)

func listenHost() string {
	if h := os.Getenv("SSH_HOST"); h != "" {
		return h
	}
	return "localhost"
}

var (
	apiURL             string
	authFingerprintKey string
	stripePublicKey    string
)

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	pubKey := s.PublicKey()

	var fingerprint, pubKeyStr string
	if pubKey != nil {
		fingerprint = auth.GetSSHKeyFingerprint(pubKey)
		pubKeyStr = auth.FormatSSHPublicKey(pubKey)
	}

	m := tui.NewModelWithAuth(fingerprint, pubKeyStr, apiURL, authFingerprintKey, stripePublicKey)
	return m, nil
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
	stripePublicKey = cfg.StripePublicKey
	log.Printf("TUI will connect to API at: %s", apiURL)

	// Create SSH server
	host := listenHost()
	p := cfg.SSHPort
	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%s", host, p)),
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

	log.Printf("Starting SSH server on %s:%s", host, p)
	log.Printf("Connect with: ssh %s -p %s", host, p)

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
