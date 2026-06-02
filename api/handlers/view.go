package handlers

import (
	"net/http"
	"time"

	"terminalShop/api/middleware"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/utils"

	"gorm.io/gorm"
)

// ViewHandler handles bulk TUI bootstrap data
type ViewHandler struct {
	stripeKey string
}

func NewViewHandler(stripeSecretKey string) *ViewHandler {
	return &ViewHandler{stripeKey: stripeSecretKey}
}

func (h *ViewHandler) GetViewInit(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		utils.RespondError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "user not found", nil)
		return
	}

	var products []models.Coffee
	db.Find(&products)

	var cart models.Cart
	db.Where("user_id = ?", userID).First(&cart)

	var addresses []models.Address
	db.Where("user_id = ?", userID).Find(&addresses)

	if err := expireStoredCards(db, userID, h.stripeKey, time.Now()); err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to expire old cards", nil)
		return
	}

	var cards []models.Card
	db.Where("user_id = ?", userID).Order("id_default DESC, created_at DESC").Find(&cards)

	var orders []models.Order
	db.Where("user_id = ?", userID).
		Preload("Items").
		Preload("Events", func(tx *gorm.DB) *gorm.DB { return tx.Order("created_at ASC") }).
		Order("created_at DESC").Find(&orders)
	var cartItems []models.CartItem
	if cart.ID != 0 {
		db.Where("cart_id = ? AND quantity > 0", cart.ID).Preload("Coffee").Find(&cartItems)
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"user":      user.ToPublic(),
		"products":  products,
		"cart":      cartItems,
		"addresses": addresses,
		"cards":     cards,
		"orders":    orders,
	})
}
