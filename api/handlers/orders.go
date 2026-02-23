package handlers

import (
	"net/http"

	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/utils"
)


type OrderHandler struct {}


func NewOrderHandler() *OrderHandler {
	return &OrderHandler{}
} 


func (h *OrderHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()

	fingerprint := r.URL.Query().Get("fingerprint")
	if fingerprint == "" {
		utils.RespondError(w, http.StatusBadRequest, "MISSING_FINGERPRINT", "fingerprint query parameter is required", nil)
		return 
	}
	var user models.User
	if err := db.Where("ssh_key_fingerprint = ?", fingerprint).First(&user).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil) 
		return
	}

	var orders []models.Order
	db.Where("user_id = ?", user.ID).Preload("Items").Order("created_at desc").Find(&orders)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"orders": orders,
	})
}
