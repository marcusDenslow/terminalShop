package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/utils"
)

// ProductHandler handles product-related requests
type ProductHandler struct{}

// NewProductHandler creates a new product handler
func NewProductHandler() *ProductHandler {
	return &ProductHandler{}
}

// GetProducts returns all products from the database
func (h *ProductHandler) GetProducts(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())

	var products []models.Coffee

	// Query all products from database
	if err := db.Find(&products).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to fetch products", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]any{
		"products": products,
	})
}

// GetProduct returns a single product by ID
func (h *ProductHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())

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

	utils.RespondSuccess(w, http.StatusOK, map[string]any{
		"product": product,
	})
}
