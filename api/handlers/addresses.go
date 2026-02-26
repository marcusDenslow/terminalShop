package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"terminalShop/api/middleware"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/shippo"
	"terminalShop/pkg/utils"
)

type AddressHandler struct {
	shippoKey string
}

// NewAddressHandler creates a new AddressHandler with the Shippo API key
func NewAddressHandler(shippoAPIKey string) *AddressHandler {
	return &AddressHandler{shippoKey: shippoAPIKey}
}

// Retrieve all saved addresses for a user
func (h *AddressHandler) GetAddresses(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()

	userID := middleware.UserIDFromContext(r.Context())

	var addresses []models.Address
	db.Where("user_id = ?", userID).Order("is_default DESC, created_at DESC").Find(&addresses)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"addresses": addresses,
	})
}

func (h *AddressHandler) CreateAddress(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()

	userID := middleware.UserIDFromContext(r.Context())

	var req struct {
		Name    string `json:"name"`
		Street  string `json:"street"`
		Street2 string `json:"street_2"`
		City    string `json:"city"`
		State   string `json:"state"`
		Zip     string `json:"zip"`
		Country string `json:"country"`
		Phone   string `json:"phone"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", nil)
		return
	}

	if req.Name == "" || req.Street == "" || req.City == "" || req.Zip == "" || req.Country == "" {
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name, street, city, zip, and country are required", nil)
		return
	}

	// Validate and normalize address via Shippo
	var address models.Address
	if h.shippoKey != "" {
		client := shippo.NewClient(h.shippoKey)
		validated, err := client.ValidateAddress(shippo.Address{
			Name:    req.Name,
			Street1: req.Street,
			Street2: req.Street2,
			City:    req.City,
			State:   req.State,
			Country: req.Country,
			Zip:     req.Zip,
			Phone:   req.Phone,
		})
		if err != nil {
			log.Printf("shippo address validation failed: %v", err)
			utils.RespondError(w, http.StatusBadRequest, "INVALID_ADDRESS", "address is invalid", nil)
			return
		}
		address = models.Address{
			UserID:  userID,
			Name:    validated.Name,
			Street:  validated.Street1,
			Street2: validated.Street2,
			City:    validated.City,
			State:   validated.State,
			Zip:     validated.Zip,
			Country: validated.Country,
			Phone:   validated.Phone,
		}
	} else {
		address = models.Address{
			UserID:  userID,
			Name:    req.Name,
			Street:  req.Street,
			Street2: req.Street2,
			City:    req.City,
			State:   req.State,
			Zip:     req.Zip,
			Country: req.Country,
			Phone:   req.Phone,
		}
	}

	if err := db.Create(&address).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to create address", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusCreated, map[string]interface{}{
		"address": address,
	})
}

// DeleteAddress deltes a saved address
func (h *AddressHandler) DeleteAddress(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	userID := middleware.UserIDFromContext(r.Context())

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ID", "invalid address id", nil)
		return
	}

	var address models.Address

	if err := db.Where("id = ? AND user_id = ?", id, userID).First(&address).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "ADDRESS NOT FOUND", "address not found", nil)
		return
	}

	// ownership is guaranteed by the WHERE clause above
	db.Delete(&address)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"message": "address successfully deleted",
	})
}

// Mark an address as the users default address

func (h *AddressHandler) SetDefaultAddress(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	userID := middleware.UserIDFromContext(r.Context())

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ID", "invalid address id", nil)
		return
	}

	var address models.Address
	if err := db.Where("id = ? AND user_id = ?", id, userID).First(&address).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "ADDRESS_NOT_FOUND", "address not found", nil)
		return
	}

	db.Model(&models.Address{}).Where("user_id = ?", userID).Update("is_default", false)

	address.IsDefault = true
	db.Save(&address)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"address": address,
	})
}
