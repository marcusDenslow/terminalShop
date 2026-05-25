package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
)

func setupOrderTestDB(t *testing.T) models.User {
	t.Helper()
	testDB := filepath.Join(t.TempDir(), "orders.db")
	database.ResetForTesting()
	t.Cleanup(database.ResetForTesting)

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

	user := models.User{
		SSHKeyFingerprint: "SHA256:ordertestfingerprint",
		SSHPublicKey:      "ssh-ed25519 AAAA ordertestkey",
		Name:              "Order Test User",
		Email:             "orders@example.com",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	return user
}

func orderRequest(method, url string, body []byte, userID uint, orderID string) *http.Request {
	req := authRequest(method, url, body, userID)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", orderID)
	return req.WithContext(contextWithRoute(req.Context(), routeCtx))
}

func contextWithRoute(ctx context.Context, routeCtx *chi.Context) context.Context {
	return context.WithValue(ctx, chi.RouteCtxKey, routeCtx)
}

func TestCreateRefundRequestRequiresMessageForOther(t *testing.T) {
	user := setupOrderTestDB(t)

	handler := NewOrderHandler("", "", "", "")
	body := []byte(`{"reason":"Other","message":"   "}`)
	req := orderRequest("POST", "/api/v1/orders/1/refund-request", body, user.ID, "1")
	w := httptest.NewRecorder()

	handler.CreateRefundRequest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "MESSAGE_REQUIRED" {
		t.Errorf("expected MESSAGE_REQUIRED, got %q", resp.Error.Code)
	}
}

func TestCreateRefundRequestRejectsInvalidOrderState(t *testing.T) {
	user := setupOrderTestDB(t)

	db := database.GetDB()
	order := models.Order{
		UserID:       user.ID,
		CardID:       1,
		Status:       models.OrderStatusFailed,
		Subtotal:     100,
		ShippingCost: 0,
		Total:        100,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	handler := NewOrderHandler("", "", "", "")
	body := []byte(`{"reason":"Quality issue","message":""}`)
	req := orderRequest("POST", "/api/v1/orders/1/refund-request", body, user.ID, fmt.Sprintf("%d", order.ID))
	w := httptest.NewRecorder()

	handler.CreateRefundRequest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "INVALID_STATE" {
		t.Errorf("expected INVALID_STATE, got %q", resp.Error.Code)
	}
}

func TestCreateRefundRequestEnforcesCooldown(t *testing.T) {
	user := setupOrderTestDB(t)

	db := database.GetDB()
	recent := time.Now().Add(-1 * time.Minute)
	order := models.Order{
		UserID:              user.ID,
		CardID:              1,
		Status:              models.OrderStatusPaid,
		Subtotal:            100,
		ShippingCost:        0,
		Total:               100,
		LastRefundRequestAt: &recent,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	handler := NewOrderHandler("", "", "", "")
	body := []byte(`{"reason":"Quality issue","message":""}`)
	req := orderRequest("POST", "/api/v1/orders/1/refund-request", body, user.ID, fmt.Sprintf("%d", order.ID))
	w := httptest.NewRecorder()

	handler.CreateRefundRequest(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", w.Code)
	}
	if got := w.Header().Get("Retry-After"); got == "" {
		t.Errorf("expected Retry-After header, got empty")
	}

	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "RATE_LIMITED" {
		t.Errorf("expected RATE_LIMITED, got %q", resp.Error.Code)
	}
}
