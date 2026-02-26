package handlers

import (
	"net/http"

	"terminalShop/api/middleware"
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

	userID := middleware.UserIDFromContext(r.Context())


	var orders []models.Order
	db.Where("user_id = ?", userID).Preload("Items").Order("created_at desc").Find(&orders)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"orders": orders,
	})
}
