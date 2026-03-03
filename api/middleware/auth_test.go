package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"terminalShop/pkg/auth"
)

const testSecret = "middleware-test-secret"

func newTestJWTManager() *auth.JWTManager {
	return auth.NewJWTManager(testSecret, 30*time.Minute)
}

// captureHandler is a dummy handler that records whether it was called
// and what userID was in the context.
type captureHandler struct {
	called bool
	userID uint
}

func (h *captureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.called = true
	h.userID = UserIDFromContext(r.Context())
	w.WriteHeader(http.StatusOK)
}

func TestAuthNoHeader(t *testing.T) {
	mgr := newTestJWTManager()
	capture := &captureHandler{}

	handler := Auth(mgr)(capture)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !capture.called {
		t.Fatal("expected handler to be called")
	}
	if capture.userID != 0 {
		t.Errorf("expected userID 0 (no auth), got %d", capture.userID)
	}
}

func TestAuthMalformedHeader(t *testing.T) {
	mgr := newTestJWTManager()
	capture := &captureHandler{}

	handler := Auth(mgr)(capture)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "NotBearer some-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !capture.called {
		t.Fatal("expected handler to be called")
	}
	if capture.userID != 0 {
		t.Errorf("expected userID 0 for malformed header, got %d", capture.userID)
	}
}

func TestAuthInvalidToken(t *testing.T) {
	mgr := newTestJWTManager()
	capture := &captureHandler{}

	handler := Auth(mgr)(capture)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer totally-invalid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !capture.called {
		t.Fatal("expected handler to be called (soft auth)")
	}
	if capture.userID != 0 {
		t.Errorf("expected userID 0 for invalid token, got %d", capture.userID)
	}
}

func TestAuthValidToken(t *testing.T) {
	mgr := newTestJWTManager()
	capture := &captureHandler{}

	handler := Auth(mgr)(capture)

	token, err := mgr.GenerateToken(99, "test@example.com", "testuser")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !capture.called {
		t.Fatal("expected handler to be called")
	}
	if capture.userID != 99 {
		t.Errorf("expected userID 99, got %d", capture.userID)
	}
}

func TestRequireAuthWithUser(t *testing.T) {
	capture := &captureHandler{}

	handler := RequireAuth(capture)

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := ContextWithUserID(req.Context(), 42)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !capture.called {
		t.Fatal("expected handler to be called when user is present")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRequireAuthWithoutUser(t *testing.T) {
	capture := &captureHandler{}

	handler := RequireAuth(capture)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if capture.called {
		t.Fatal("expected handler to NOT be called when no user")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	var resp struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected error code UNAUTHORIZED, got %s", resp.Error.Code)
	}
}

func TestUserIDFromContextEmpty(t *testing.T) {
	ctx := context.Background()
	if id := UserIDFromContext(ctx); id != 0 {
		t.Errorf("expected 0 from empty context, got %d", id)
	}
}

func TestUserIDFromContextSet(t *testing.T) {
	ctx := ContextWithUserID(context.Background(), 42)
	if id := UserIDFromContext(ctx); id != 42 {
		t.Errorf("expected 42, got %d", id)
	}
}
