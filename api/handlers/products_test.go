package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
)

func setupTestDB(t *testing.T) string {
	testDB := "test_products.db"

	// Remove if exists
	os.Remove(testDB)

	// Reset singleton
	database.ResetForTesting()

	db, err := database.Connect(testDB)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	if err := database.Seed(db); err != nil {
		t.Fatalf("Failed to seed: %v", err)
	}

	return testDB
}

func TestGetProducts(t *testing.T) {
	testDB := setupTestDB(t)
	defer os.Remove(testDB)

	handler := NewProductHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	w := httptest.NewRecorder()

	handler.GetProducts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Products []models.Coffee `json:"products"`
		} `json:"data"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}

	if len(response.Data.Products) != 6 {
		t.Errorf("Expected 6 products, got %d", len(response.Data.Products))
	}

	// Verify product fields
	for _, p := range response.Data.Products {
		if p.ID == 0 {
			t.Error("Product ID should not be 0")
		}
		if p.Name == "" {
			t.Error("Product name should not be empty")
		}
		if p.Price <= 0 {
			t.Errorf("Product %s has invalid price: %f", p.Name, p.Price)
		}
	}

	// Verify Content-Type header
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

func TestGetProductsEmptyDatabase(t *testing.T) {
	testDB := "test_empty.db"
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	// Create database but don't seed
	database.ResetForTesting()
	db, err := database.Connect(testDB)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	handler := NewProductHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	w := httptest.NewRecorder()

	handler.GetProducts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Products []models.Coffee `json:"products"`
		} `json:"data"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}

	if len(response.Data.Products) != 0 {
		t.Errorf("Expected 0 products, got %d", len(response.Data.Products))
	}
}
