package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
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

func TestGetProduct(t *testing.T) {
	testDB := setupTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewProductHandler()

	r := chi.NewRouter()
	r.Get("/products/{id}", handler.GetProduct)

	req := httptest.NewRequest(http.MethodGet, "/products/1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Product models.Coffee `json:"product"`
		} `json:"data"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}

	if response.Data.Product.Name != "Espresso" {
		t.Errorf("Expected Espresso, got %s", response.Data.Product.Name)
	}
}

func TestGetProductNotFound(t *testing.T) {
	testDB := setupTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewProductHandler()

	r := chi.NewRouter()
	r.Get("/products/{id}", handler.GetProduct)

	req := httptest.NewRequest(http.MethodGet, "/products/999", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
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
