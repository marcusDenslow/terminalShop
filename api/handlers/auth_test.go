package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"terminalShop/pkg/auth"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
)

const testAuthSecret = "test-auth-handler-secret"
const testClientSecret = "test-client-secret"

// setupAuthHandlerTestDB creates a temp DB with migrated tables and seeded products.
// Returns the DB filename, a JWTManager, and an AuthHandler.
func setupAuthHandlerTestDB(t *testing.T) (string, *auth.JWTManager, *AuthHandler) {
	t.Helper()
	testDB := "test_auth.db"
	os.Remove(testDB)
	database.ResetForTesting()

	db, err := database.Connect(testDB)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	if err := database.Seed(db); err != nil {
		t.Fatalf("failed to seed: %v", err)
	}

	mgr := auth.NewJWTManager(testAuthSecret, 30*time.Minute)
	handler := NewAuthHandler(mgr, testClientSecret)

	return testDB, mgr, handler
}

// createAuthTestUser inserts a user into the DB and returns it.
func createAuthTestUser(t *testing.T, fingerprint string) models.User {
	t.Helper()
	db := database.GetDB()
	user := models.User{
		SSHKeyFingerprint: fingerprint,
		SSHPublicKey:      "ssh-ed25519 AAAA " + fingerprint,
		Name:              "Test User",
		Email:             "test@example.com",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return user
}

// --- GetToken tests ---

func TestGetTokenSuccess(t *testing.T) {
	testDB, _, handler := setupAuthHandlerTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	user := createAuthTestUser(t, "SHA256:testfingerprint123")

	body, _ := json.Marshal(map[string]string{
		"fingerprint":   user.SSHKeyFingerprint,
		"client_secret": testClientSecret,
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.GetToken(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
			ExpiresIn   int    `json:"expires_in"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success to be true")
	}
	if resp.Data.AccessToken == "" {
		t.Error("expected non-empty access_token")
	}
	if resp.Data.TokenType != "Bearer" {
		t.Errorf("expected token_type Bearer, got %s", resp.Data.TokenType)
	}
	if resp.Data.ExpiresIn != 1800 {
		t.Errorf("expected expires_in 1800, got %d", resp.Data.ExpiresIn)
	}
}

func TestGetTokenBadSecret(t *testing.T) {
	testDB, _, handler := setupAuthHandlerTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	body, _ := json.Marshal(map[string]string{
		"fingerprint":   "SHA256:anything",
		"client_secret": "wrong-secret",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.GetToken(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGetTokenMissingFingerprint(t *testing.T) {
	testDB, _, handler := setupAuthHandlerTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	body, _ := json.Marshal(map[string]string{
		"client_secret": testClientSecret,
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.GetToken(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetTokenUnknownFingerprint(t *testing.T) {
	testDB, _, handler := setupAuthHandlerTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	body, _ := json.Marshal(map[string]string{
		"fingerprint":   "SHA256:doesnotexist",
		"client_secret": testClientSecret,
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.GetToken(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGetTokenInvalidJSON(t *testing.T) {
	testDB, _, handler := setupAuthHandlerTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	req := httptest.NewRequest("POST", "/api/v1/auth/token", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.GetToken(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- RegisterWithSSHKey tests ---

func TestRegisterWithSSHKeySuccess(t *testing.T) {
	testDB, _, handler := setupAuthHandlerTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	body, _ := json.Marshal(map[string]string{
		"ssh_public_key":      "ssh-ed25519 AAAA newkey",
		"ssh_key_fingerprint": "SHA256:brandnewfingerprint",
		"name":                "New User",
		"email":               "new@example.com",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RegisterWithSSHKey(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegisterWithSSHKeyDuplicate(t *testing.T) {
	testDB, _, handler := setupAuthHandlerTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	createAuthTestUser(t, "SHA256:existingfp")

	body, _ := json.Marshal(map[string]string{
		"ssh_public_key":      "ssh-ed25519 AAAA existingkey",
		"ssh_key_fingerprint": "SHA256:existingfp",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RegisterWithSSHKey(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestRegisterWithSSHKeyMissingFields(t *testing.T) {
	testDB, _, handler := setupAuthHandlerTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	body, _ := json.Marshal(map[string]string{
		"name": "No Key User",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RegisterWithSSHKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- GetUserBySSHKey tests ---

func TestGetUserBySSHKeySuccess(t *testing.T) {
	testDB, _, handler := setupAuthHandlerTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	createAuthTestUser(t, "SHA256:lookupfp")

	req := httptest.NewRequest("GET", "/api/v1/auth/ssh?fingerprint=SHA256:lookupfp", nil)
	w := httptest.NewRecorder()

	handler.GetUserBySSHKey(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetUserBySSHKeyNotFound(t *testing.T) {
	testDB, _, handler := setupAuthHandlerTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	req := httptest.NewRequest("GET", "/api/v1/auth/ssh?fingerprint=SHA256:nope", nil)
	w := httptest.NewRecorder()

	handler.GetUserBySSHKey(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetUserBySSHKeyMissingParam(t *testing.T) {
	testDB, _, handler := setupAuthHandlerTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	req := httptest.NewRequest("GET", "/api/v1/auth/ssh", nil)
	w := httptest.NewRecorder()

	handler.GetUserBySSHKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
