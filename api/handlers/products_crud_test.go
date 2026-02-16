package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/validation"
)

func TestGetProduct(t *testing.T) {
	testDB := setupTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewProductHandler()

	// Create a router with chi to test URL parameters
	r := chi.NewRouter()
	r.Get("/products/{id}", handler.GetProduct)

	// Test getting existing product
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

func TestCreateProduct(t *testing.T) {
	testDB := "test_create.db"
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	database.ResetForTesting()
	db, err := database.Connect(testDB)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	handler := NewProductHandler()

	productReq := validation.ProductRequest{
		Name:        "Test Coffee",
		RoastType:   "Medium Roast",
		Ounces:      10,
		BeanType:    "Test Beans",
		Price:       6.99,
		Color:       "#FF0000",
		Description: "A test coffee",
	}

	body, _ := json.Marshal(productReq)
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.CreateProduct(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
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

	if response.Data.Product.Name != "Test Coffee" {
		t.Errorf("Expected Test Coffee, got %s", response.Data.Product.Name)
	}

	if response.Data.Product.ID == 0 {
		t.Error("Expected product ID to be set")
	}
}

func TestCreateProductValidationError(t *testing.T) {
	testDB := "test_create_validation.db"
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	database.ResetForTesting()
	db, err := database.Connect(testDB)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	handler := NewProductHandler()

	// Invalid product (missing name, negative price)
	productReq := validation.ProductRequest{
		Name:  "",
		Price: -5.00,
	}

	body, _ := json.Marshal(productReq)
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.CreateProduct(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestUpdateProduct(t *testing.T) {
	testDB := setupTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewProductHandler()

	r := chi.NewRouter()
	r.Put("/products/{id}", handler.UpdateProduct)

	// Update price of Espresso
	updateReq := validation.ProductRequest{
		Price: 4.50,
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPut, "/products/1", bytes.NewReader(body))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
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

	if response.Data.Product.Price != 4.50 {
		t.Errorf("Expected price 4.50, got %.2f", response.Data.Product.Price)
	}

	// Name should still be Espresso
	if response.Data.Product.Name != "Espresso" {
		t.Errorf("Expected name Espresso, got %s", response.Data.Product.Name)
	}
}

func TestUpdateProductNotFound(t *testing.T) {
	testDB := setupTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewProductHandler()

	r := chi.NewRouter()
	r.Put("/products/{id}", handler.UpdateProduct)

	updateReq := validation.ProductRequest{
		Price: 4.50,
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPut, "/products/999", bytes.NewReader(body))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestDeleteProduct(t *testing.T) {
	testDB := setupTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewProductHandler()

	r := chi.NewRouter()
	r.Delete("/products/{id}", handler.DeleteProduct)

	// Delete product 1
	req := httptest.NewRequest(http.MethodDelete, "/products/1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify product is soft deleted
	db := database.GetDB()
	var product models.Coffee
	err := db.First(&product, 1).Error
	if err == nil {
		t.Error("Expected product to be soft deleted (not found in normal query)")
	}

	// Verify it exists in unscoped query
	err = db.Unscoped().First(&product, 1).Error
	if err != nil {
		t.Error("Expected product to still exist in database (soft delete)")
	}

	if product.DeletedAt.Time.IsZero() {
		t.Error("Expected DeletedAt to be set")
	}
}

func TestDeleteProductNotFound(t *testing.T) {
	testDB := setupTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewProductHandler()

	r := chi.NewRouter()
	r.Delete("/products/{id}", handler.DeleteProduct)

	req := httptest.NewRequest(http.MethodDelete, "/products/999", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestCRUDFlow(t *testing.T) {
	testDB := "test_crud_flow.db"
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	database.ResetForTesting()
	db, err := database.Connect(testDB)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	handler := NewProductHandler()

	// 1. Create a product
	productReq := validation.ProductRequest{
		Name:        "Flow Test Coffee",
		RoastType:   "Dark Roast",
		Ounces:      8,
		BeanType:    "Colombian",
		Price:       7.99,
		Color:       "#000000",
		Description: "Testing full CRUD flow",
	}

	body, _ := json.Marshal(productReq)
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.CreateProduct(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create product: %d", w.Code)
	}

	var createResp struct {
		Success bool `json:"success"`
		Data    struct {
			Product models.Coffee `json:"product"`
		} `json:"data"`
	}
	json.NewDecoder(w.Body).Decode(&createResp)
	productID := createResp.Data.Product.ID

	// 2. Get the product
	r := chi.NewRouter()
	r.Get("/products/{id}", handler.GetProduct)

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d", productID), nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to get product: %d", w.Code)
	}

	// 3. Update the product
	r = chi.NewRouter()
	r.Put("/products/{id}", handler.UpdateProduct)

	updateReq := validation.ProductRequest{
		Price: 8.99,
		Name:  "Updated Flow Test Coffee",
	}
	body, _ = json.Marshal(updateReq)
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/products/%d", productID), bytes.NewReader(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to update product: %d", w.Code)
	}

	var updateResp struct {
		Success bool `json:"success"`
		Data    struct {
			Product models.Coffee `json:"product"`
		} `json:"data"`
	}
	json.NewDecoder(w.Body).Decode(&updateResp)

	if updateResp.Data.Product.Price != 8.99 {
		t.Errorf("Expected price 8.99, got %.2f", updateResp.Data.Product.Price)
	}

	if updateResp.Data.Product.Name != "Updated Flow Test Coffee" {
		t.Errorf("Expected name Updated Flow Test Coffee, got %s", updateResp.Data.Product.Name)
	}

	// 4. Delete the product
	r = chi.NewRouter()
	r.Delete("/products/{id}", handler.DeleteProduct)

	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/products/%d", productID), nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to delete product: %d", w.Code)
	}

	// 5. Verify product is gone
	var product models.Coffee
	err = db.First(&product, productID).Error
	if err == nil {
		t.Error("Expected product to be soft deleted")
	}
}
