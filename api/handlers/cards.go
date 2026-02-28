package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/customer"
	"github.com/stripe/stripe-go/v78/paymentmethod"

	"terminalShop/api/middleware"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/utils"
)

// CardHandler handles saved payment method CRUD.
type CardHandler struct {
	stripeKey string
}

// NewCardHandler creates a new card handler with the Stripe secret key.
func NewCardHandler(stripeSecretKey string) *CardHandler {
	return &CardHandler{stripeKey: stripeSecretKey}
}

// GetCards returns all saved cards for the authenticated user.
func (h *CardHandler) GetCards(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	userID := middleware.UserIDFromContext(r.Context())

	var cards []models.Card
	db.Where("user_id = ?", userID).Order("created_at DESC").Find(&cards)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"cards": cards,
	})
}

// GetCard returns a single saved card by ID.
func (h *CardHandler) GetCard(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	userID := middleware.UserIDFromContext(r.Context())

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ID", "invalid card id", nil)
		return
	}

	var card models.Card
	if err := db.Where("id = ? AND user_id = ?", id, userID).First(&card).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "CARD_NOT_FOUND", "card not found", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"card": card,
	})
}

// SaveCard creates a saved card from a Stripe token. The token is converted to
// a PaymentMethod and attached to the user's Stripe customer.
func (h *CardHandler) SaveCard(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	userID := middleware.UserIDFromContext(r.Context())

	var req struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", nil)
		return
	}

	if req.Token == "" {
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "token is required", nil)
		return
	}

	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil)
		return
	}

	stripe.Key = h.stripeKey

	// Ensure the user has a Stripe customer
	if user.StripeCustomerID == "" {
		params := &stripe.CustomerParams{
			Description: stripe.String(fmt.Sprintf("terminal.shop user %d", user.ID)),
		}
		if user.Email != "" {
			params.Email = stripe.String(user.Email)
		}
		if user.Name != "" {
			params.Name = stripe.String(user.Name)
		}

		cust, err := customer.New(params)
		if err != nil {
			utils.RespondError(w, http.StatusInternalServerError, "STRIPE_ERROR", "failed to create stripe customer", nil)
			return
		}
		user.StripeCustomerID = cust.ID
		db.Save(&user)
	}

	// Convert the token to a PaymentMethod
	pm, err := paymentmethod.New(&stripe.PaymentMethodParams{
		Type: stripe.String("card"),
		Card: &stripe.PaymentMethodCardParams{
			Token: stripe.String(req.Token),
		},
	})
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "STRIPE_ERROR", "invalid card token", nil)
		return
	}

	// Check for duplicate cards by fingerprint
	listParams := &stripe.PaymentMethodListParams{
		Customer: stripe.String(user.StripeCustomerID),
		Type:     stripe.String("card"),
	}
	iter := paymentmethod.List(listParams)
	for iter.Next() {
		existing := iter.PaymentMethod()
		if existing.Card != nil && pm.Card != nil &&
			existing.Card.Fingerprint == pm.Card.Fingerprint {
			utils.RespondError(w, http.StatusConflict, "ALREADY_EXISTS", "this card is already saved", nil)
			return
		}
	}

	// Attach the PaymentMethod to the Stripe customer
	_, err = paymentmethod.Attach(pm.ID, &stripe.PaymentMethodAttachParams{
		Customer: stripe.String(user.StripeCustomerID),
	})
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "STRIPE_ERROR", "failed to attach payment method", nil)
		return
	}

	// Save the card locally
	card := models.Card{
		UserID:          user.ID,
		StripePaymentID: pm.ID,
		Last4:           pm.Card.Last4,
		Brand:           string(pm.Card.Brand),
		ExpMonth:        int(pm.Card.ExpMonth),
		ExpYear:         int(pm.Card.ExpYear),
	}

	if err := db.Create(&card).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to save card", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusCreated, map[string]interface{}{
		"card": card,
	})
}

// DeleteCard removes a saved card. If the card is currently set on the user's
// cart, the cart's card selection is cleared.
func (h *CardHandler) DeleteCard(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	userID := middleware.UserIDFromContext(r.Context())

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ID", "invalid card id", nil)
		return
	}

	var card models.Card
	if err := db.Where("id = ? AND user_id = ?", id, userID).First(&card).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "CARD_NOT_FOUND", "card not found", nil)
		return
	}

	// Clear the card from any cart that references it
	cardID := uint(id)
	db.Model(&models.Cart{}).Where("card_id = ? AND user_id = ?", cardID, userID).Update("card_id", nil)

	// Detach from Stripe if it's a PaymentMethod (pm_)
	if len(card.StripePaymentID) >= 3 && card.StripePaymentID[:3] == "pm_" {
		stripe.Key = h.stripeKey
		_, _ = paymentmethod.Detach(card.StripePaymentID, &stripe.PaymentMethodDetachParams{})
	}

	// Soft-delete the local record
	db.Delete(&card)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"message": "card deleted",
	})
}
