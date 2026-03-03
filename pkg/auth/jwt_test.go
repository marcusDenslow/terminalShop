package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

const testSecret = "test-secret-key-for-jwt-testing"

func TestGenerateToken(t *testing.T) {
	mgr := NewJWTManager(testSecret, 30*time.Minute)

	token, err := mgr.GenerateToken(1, "test@example.com", "testuser")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestValidateTokenRoundTrip(t *testing.T) {
	mgr := NewJWTManager(testSecret, 30*time.Minute)

	token, err := mgr.GenerateToken(42, "user@test.com", "myuser")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	claims, err := mgr.ValidateToken(token)
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}

	if claims.UserID != 42 {
		t.Errorf("expected UserID 42, got %d", claims.UserID)
	}
	if claims.Email != "user@test.com" {
		t.Errorf("expected Email user@test.com, got %s", claims.Email)
	}
	if claims.Username != "myuser" {
		t.Errorf("expected Username myuser, got %s", claims.Username)
	}
}

func TestValidateTokenIssuer(t *testing.T) {
	mgr := NewJWTManager(testSecret, 30*time.Minute)

	token, _ := mgr.GenerateToken(1, "", "")
	claims, err := mgr.ValidateToken(token)
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}

	if claims.Issuer != "terminalshop" {
		t.Errorf("expected issuer terminalshop, got %s", claims.Issuer)
	}
}

func TestValidateTokenExpired(t *testing.T) {
	// Token that expired 1 second ago
	mgr := NewJWTManager(testSecret, -1*time.Second)

	token, err := mgr.GenerateToken(1, "", "")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	_, err = mgr.ValidateToken(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateTokenWrongSecret(t *testing.T) {
	generator := NewJWTManager("secret-a", 30*time.Minute)
	validator := NewJWTManager("secret-b", 30*time.Minute)

	token, _ := generator.GenerateToken(1, "", "")

	_, err := validator.ValidateToken(token)
	if err == nil {
		t.Fatal("expected error when validating with wrong secret")
	}
}

func TestValidateTokenMalformed(t *testing.T) {
	mgr := NewJWTManager(testSecret, 30*time.Minute)

	_, err := mgr.ValidateToken("not.a.valid.jwt")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}

func TestValidateTokenEmptyString(t *testing.T) {
	mgr := NewJWTManager(testSecret, 30*time.Minute)

	_, err := mgr.ValidateToken("")
	if err == nil {
		t.Fatal("expected error for empty token string")
	}
}

func TestGetSSHKeyFingerprint(t *testing.T) {
	pubKey := generateTestSSHKey(t)

	fp := GetSSHKeyFingerprint(pubKey)
	if !strings.HasPrefix(fp, "SHA256:") {
		t.Errorf("expected fingerprint to start with SHA256:, got %s", fp)
	}

	// Should be deterministic
	fp2 := GetSSHKeyFingerprint(pubKey)
	if fp != fp2 {
		t.Errorf("fingerprint not deterministic: %s vs %s", fp, fp2)
	}
}

func TestFormatSSHPublicKey(t *testing.T) {
	pubKey := generateTestSSHKey(t)

	formatted := FormatSSHPublicKey(pubKey)
	if !strings.HasPrefix(formatted, "ssh-ed25519 ") {
		t.Errorf("expected formatted key to start with ssh-ed25519, got %s", formatted[:20])
	}
}

// generateTestSSHKey creates an ed25519 SSH key pair for testing.
func generateTestSSHKey(t *testing.T) gossh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}

	sshPub, err := gossh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("failed to convert to ssh public key: %v", err)
	}

	return sshPub
}
