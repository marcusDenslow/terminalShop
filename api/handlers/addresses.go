package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/utils"
)

type AddressHandler struct{}

// Create a new AddressHandler
func NewAddressHandler() *AddressHandler {
	return &AddressHandler{}
}

// Retrieve all saved addresses for a user
func (h *AddressHandler) GetAddresses(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	fingerprint := r.URL.Query().Get("fingerprint")

	if fingerprint == "" {
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION ERROR", "fingerprint is required", nil)
		return
	}

	var user models.User
	if err := db.Where("ssh_key_fingerprint = ?", fingerprint).First(&user).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil)
		return
	}

	var addresses []models.Address
	db.Where("user_id = ?", user.ID).Order("is_default DESC, created_at DESC").Find(&addresses)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"addresses": addresses,
	})
}

func (h *AddressHandler) CreateAddress(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()

	var req struct {
		Fingerprint string `json:"fingerprint"`
		Name        string `json:"name"`
		Street      string `json:"street"`
		Street2     string `json:"street_2"`
		City        string `json:"city"`
		State       string `json:"state"`
		Zip         string `json:"zip"`
		Country     string `json:"country"`
		Phone       string `json:"phone"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", nil)
		return
	}

	if req.Fingerprint == "" || req.Name == "" || req.Street == "" || req.City == "" req.Zip == "" || req.Country == "" {
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "fingerprint, name, street, city, zip, and country are required", nil)
	}
}
