package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/utils"
	"terminalShop/pkg/validation"
)

// ProductHandler handles product-related requests
type ProductHandler struct{}

// NewProductHandler creates a new product handler
func NewProductHandler() *ProductHandler {
	return &ProductHandler{}
}

// GetProducts returns all products from the database
func (h *ProductHandler) GetProducts(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()

	var products []models.Coffee

	// Query all products from database
	if err := db.Find(&products).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to fetch products", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"products": products,
	})
}

// GetProduct returns a single product by ID
func (h *ProductHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()

	// Get ID from URL parameter
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ID", "invalid product ID", nil)
		return
	}

	var product models.Coffee
	if err := db.First(&product, id).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "NOT_FOUND", "product not found", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"product": product,
	})
}

// CreateProduct creates a new product
func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()

	// Parse request body
	var req validation.ProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", nil)
		return
	}

	// Validate request
	if validationErrors := validation.ValidateProductRequest(req, false); validationErrors.HasErrors() {
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "validation failed", map[string]interface{}{
			"errors": validationErrors,
		})
		return
	}

	// Create product
	product := models.Coffee{
		Name:        req.Name,
		RoastType:   req.RoastType,
		Ounces:      req.Ounces,
		BeanType:    req.BeanType,
		Price:       req.Price,
		Color:       req.Color,
		Description: req.Description,
	}

	if err := db.Create(&product).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to create product", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusCreated, map[string]interface{}{
		"product": product,
	})
}

// UpdateProduct updates an existing product
func (h *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()

	// Get ID from URL parameter
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ID", "invalid product ID", nil)
		return
	}

	// Check if product exists
	var product models.Coffee
	if err := db.First(&product, id).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "NOT_FOUND", "product not found", nil)
		return
	}

	// Parse request body
	var req validation.ProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", nil)
		return
	}

	// Validate request (isUpdate = true for partial updates)
	if validationErrors := validation.ValidateProductRequest(req, true); validationErrors.HasErrors() {
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "validation failed", map[string]interface{}{
			"errors": validationErrors,
		})
		return
	}

	// Update only provided fields
	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.RoastType != "" {
		updates["roast_type"] = req.RoastType
	}
	if req.Ounces != 0 {
		updates["ounces"] = req.Ounces
	}
	if req.BeanType != "" {
		updates["bean_type"] = req.BeanType
	}
	if req.Price != 0 {
		updates["price"] = req.Price
	}
	if req.Color != "" {
		updates["color"] = req.Color
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}

	if err := db.Model(&product).Updates(updates).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to update product", nil)
		return
	}

	// Reload product to get updated values
	db.First(&product, id)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"product": product,
	})
}

// DeleteProduct soft deletes a product
func (h *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()

	// Get ID from URL parameter
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ID", "invalid product ID", nil)
		return
	}

	// Check if product exists
	var product models.Coffee
	if err := db.First(&product, id).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "NOT_FOUND", "product not found", nil)
		return
	}

	// Soft delete (sets deleted_at timestamp)
	if err := db.Delete(&product).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to delete product", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"message": "product deleted successfully",
	})
}
