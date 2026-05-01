package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"terminalShop/api/middleware"
	"terminalShop/pkg/bring"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/shippo"
	"terminalShop/pkg/utils"
)

func addressLog() *slog.Logger { return slog.With("component", "address") }

type AddressHandler struct {
	shippoKey   string
	bringAPIUID string
	bringAPIKey string
}

// NewAddressHandler creates a new AddressHandler with address validation API keys.
func NewAddressHandler(shippoAPIKey, bringAPIUID, bringAPIKey string) *AddressHandler {
	return &AddressHandler{
		shippoKey:   shippoAPIKey,
		bringAPIUID: bringAPIUID,
		bringAPIKey: bringAPIKey,
	}
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

	// Validate and normalize address via country-specific provider
	var address models.Address
	switch req.Country {
	case "US":
		if h.shippoKey == "" {
			utils.RespondError(w, http.StatusInternalServerError, "CONFIG_ERROR", "shippo not configured for US address validation", nil)
			return
		}
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
			addressLog().Warn("shippo address validation failed", "error", err, "country", "US")
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
	case "NO":
		if h.bringAPIUID == "" || h.bringAPIKey == "" {
			utils.RespondError(w, http.StatusBadRequest, "CONFIG_ERROR", "bring not configured for Norwegian address validation", nil)
			return
		}
		client := bring.NewClient(h.bringAPIUID, h.bringAPIKey)
		validated, err := client.ValidateAddress(bring.Address{
			Name:    req.Name,
			Street1: req.Street,
			Street2: req.Street2,
			City:    req.City,
			Zip:     req.Zip,
			Country: req.Country,
			Phone:   req.Phone,
		})
		if err != nil {
			addressLog().Warn("bring address validation failed", "error", err, "country", "NO")
			utils.RespondError(w, http.StatusBadRequest, "INVALID_ADDRESS", "address is invalid", nil)
			return
		}
		address = models.Address{
			UserID:  userID,
			Name:    validated.Name,
			Street:  validated.Street1,
			Street2: validated.Street2,
			City:    validated.City,
			Zip:     validated.Zip,
			Country: validated.Country,
			Phone:   validated.Phone,
		}
	default:
		utils.RespondError(w, http.StatusBadRequest, "UNSUPPORTED_COUNTRY", "address validation is not supported for this country", nil)
		return
	}

	// remove existing addres with the same street addres
	var existing models.Address
	if err := db.Where("user_id = ? AND street = ?", userID, address.Street).First(&existing).Error; err == nil {
		db.Delete(&existing)
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
		utils.RespondError(w, http.StatusNotFound, "ADDRESS_NOT_FOUND", "address not found", nil)
		return
	}

	if err := db.Delete(&address).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete address", nil)
		return
	}

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

	if err := db.Model(&models.Address{}).Where("user_id = ?", userID).Update("is_default", false).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update address", nil)
		return
	}

	address.IsDefault = true
	if err := db.Save(&address).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save address", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"address": address,
	})
}
