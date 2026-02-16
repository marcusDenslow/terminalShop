package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"terminalShop/pkg/utils"
)

func TestGetHealth(t *testing.T) {
	handler := NewHealthHandler("v1.0.0-test")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	handler.GetHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response struct {
		Success bool             `json:"success"`
		Data    utils.HealthData `json:"data"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}

	if response.Data.Status != "ok" {
		t.Errorf("Expected status 'ok', got %s", response.Data.Status)
	}

	if response.Data.Version != "v1.0.0-test" {
		t.Errorf("Expected version 'v1.0.0-test', got %s", response.Data.Version)
	}

	// Verify timestamp is recent (within last 5 seconds)
	now := time.Now()
	diff := now.Sub(response.Data.Timestamp)
	if diff > 5*time.Second || diff < 0 {
		t.Errorf("Timestamp seems incorrect: %v (diff: %v)", response.Data.Timestamp, diff)
	}

	// Verify Content-Type header
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

func TestPing(t *testing.T) {
	handler := NewHealthHandler("v1.0.0-test")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	w := httptest.NewRecorder()

	handler.Ping(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Message string `json:"message"`
		} `json:"data"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}

	if response.Data.Message != "pong" {
		t.Errorf("Expected 'pong', got %s", response.Data.Message)
	}

	// Verify Content-Type header
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}
