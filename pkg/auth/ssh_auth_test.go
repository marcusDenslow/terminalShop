package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"terminalShop/pkg/database"
)

// setupAuthTestDB creates a temp DB with migrated tables.
// Returns the DB filename for cleanup.
func setupAuthTestDB(t *testing.T) string {
	t.Helper()
	testDB := "test_ssh_auth.db"
	_ = os.Remove(testDB)
	database.ResetForTesting()

	db, err := database.Connect(testDB)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return testDB
}

// newTestSSHKey generates a fresh ed25519 SSH key for testing.
func newTestSSHKey(t *testing.T) gossh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	sshPub, err := gossh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("failed to convert key: %v", err)
	}
	return sshPub
}

func TestAuthenticateSSHKeyNewUser(t *testing.T) {
	testDB := setupAuthTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	mgr := NewJWTManager("test-secret", 30*time.Minute)
	svc := NewSSHAuthService(db, mgr)

	key := newTestSSHKey(t)
	user, token, err := svc.AuthenticateSSHKey(key)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if user.ID == 0 {
		t.Error("expected user to have an ID assigned")
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestAuthenticateSSHKeyExistingUser(t *testing.T) {
	testDB := setupAuthTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	mgr := NewJWTManager("test-secret", 30*time.Minute)
	svc := NewSSHAuthService(db, mgr)

	key := newTestSSHKey(t)

	// First call creates the user
	user1, _, _ := svc.AuthenticateSSHKey(key)

	// Second call should find the same user
	user2, _, err := svc.AuthenticateSSHKey(key)
	if err != nil {
		t.Fatalf("second auth failed: %v", err)
	}
	if user1.ID != user2.ID {
		t.Errorf("expected same user ID, got %d and %d", user1.ID, user2.ID)
	}
}

func TestRegisterWithSSHKey(t *testing.T) {
	testDB := setupAuthTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	mgr := NewJWTManager("test-secret", 30*time.Minute)
	svc := NewSSHAuthService(db, mgr)

	key := newTestSSHKey(t)
	user, token, err := svc.RegisterWithSSHKey(key, "Coffee Fan", "coffee@test.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user.Name != "Coffee Fan" {
		t.Errorf("expected name Coffee Fan, got %s", user.Name)
	}
	if user.Email != "coffee@test.com" {
		t.Errorf("expected email coffee@test.com, got %s", user.Email)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestRegisterWithSSHKeyDuplicate(t *testing.T) {
	testDB := setupAuthTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	mgr := NewJWTManager("test-secret", 30*time.Minute)
	svc := NewSSHAuthService(db, mgr)

	key := newTestSSHKey(t)

	// First registration succeeds
	_, _, err := svc.RegisterWithSSHKey(key, "User One", "")
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	// Second registration with same key should fail
	_, _, err = svc.RegisterWithSSHKey(key, "User Two", "")
	if err == nil {
		t.Fatal("expected error for duplicate SSH key registration")
	}
}

func TestIsSSHKeyRegistered(t *testing.T) {
	testDB := setupAuthTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	mgr := NewJWTManager("test-secret", 30*time.Minute)
	svc := NewSSHAuthService(db, mgr)

	key := newTestSSHKey(t)

	// Before registration
	registered, err := svc.IsSSHKeyRegistered(key)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if registered {
		t.Error("expected key to not be registered yet")
	}

	// After authentication (which auto-creates)
	_, _, _ = svc.AuthenticateSSHKey(key)

	registered, err = svc.IsSSHKeyRegistered(key)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if !registered {
		t.Error("expected key to be registered after authentication")
	}
}
