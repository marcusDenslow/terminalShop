package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/customer"
	"github.com/stripe/stripe-go/v78/paymentmethod"
	"gorm.io/gorm"

	"terminalShop/api/middleware"
	"terminalShop/pkg/audit"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/utils"

	stripeSession "github.com/stripe/stripe-go/v78/checkout/session"
)

// payRedirects maps short tokens to full Stripe checkout URLs.
var (
	payRedirects   = map[string]string{}
	payRedirectsMu sync.Mutex
)

func storeRedirect(stripeURL string) string {
	b := make([]byte, 16)
	rand.Read(b)
	token := hex.EncodeToString(b)
	payRedirectsMu.Lock()
	payRedirects[token] = stripeURL
	payRedirectsMu.Unlock()
	return token
}

// PayRedirect resolves a short token and redirects to the Stripe checkout URL.
func PayRedirect(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	payRedirectsMu.Lock()
	url, ok := payRedirects[token]
	payRedirectsMu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

// CardHandler handles saved payment method CRUD.
type CardHandler struct {
	stripeKey string
	appURL    string
}

// NewCardHandler creates a new card handler with the Stripe secret key.
func NewCardHandler(stripeSecretKey string, appURL string) *CardHandler {
	return &CardHandler{stripeKey: stripeSecretKey, appURL: appURL}
}

// GetCards returns all saved cards for the authenticated user.
func (h *CardHandler) GetCards(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	userID := middleware.UserIDFromContext(r.Context())

	var cards []models.Card
	db.Where("user_id = ?", userID).Order("is_default DESC, created_at DESC").Find(&cards)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"cards": cards,
	})
}

// func utils.RespondError(w http.ResponseWriter, statusCode int, code string, message string, details map[string]interface{})

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

func (h *CardHandler) SetDefaultCard(w http.ResponseWriter, r *http.Request) {
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

	if err := db.Model(&models.Card{}).Where("user_id = ?", userID).Update("is_default", false).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update card", nil)
		return
	}

	card.IsDefault = true
	if err := db.Save(&card).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save card", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"card": card,
	})
}

func (h *CardHandler) CollectCard(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	userID := middleware.UserIDFromContext(r.Context())

	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil)
		return
	}

	stripe.Key = h.stripeKey

	if err := ensureStripeCustomer(db, &user); err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "STRIPE_ERROR", "failed to create stripe customer", nil)
		return
	}

	params := &stripe.CheckoutSessionParams{
		Mode:               stripe.String(string(stripe.CheckoutSessionModeSetup)),
		Customer:           stripe.String(user.StripeCustomerID),
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		SuccessURL:         stripe.String(h.appURL + "/card-added"),
		CancelURL:          stripe.String(h.appURL + "/card-added"),
	}

	sess, err := stripeSession.New(params)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "STRIPE_ERROR", "failed to create checkout session", nil)
		return
	}

	token := storeRedirect(sess.URL)
	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"url": fmt.Sprintf("%s/pay/%s", h.appURL, token),
	})
}

// SaveCardRequest holds the raw card fields submitted by the TUI.
// Tokenization happens server-side so the Stripe secret key never
// touches the SSH client or TUI layer.
type SaveCardRequest struct {
	Token string `json:"token"`
}

// SaveCard accepts raw card params, tokenizes them via the stripe-go SDK
// (server-side), converts the token to a PaymentMethod, deduplicates by
// fingerprint, attaches to the user's Stripe customer, and saves the
// card metadata locally. No raw card data is stored at any point.
func (h *CardHandler) SaveCard(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	userID := middleware.UserIDFromContext(r.Context())

	var req SaveCardRequest
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

	// Ensure the user has a Stripe customer, creating one if needed.
	if err := ensureStripeCustomer(db, &user); err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "STRIPE_ERROR", "failed to create stripe customer", nil)
		return
	}

	// Convert the single-use token to a reusable PaymentMethod.
	pm, err := paymentmethod.New(&stripe.PaymentMethodParams{
		Type: stripe.String("card"),
		Card: &stripe.PaymentMethodCardParams{
			Token: stripe.String(req.Token),
		},
	})
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "STRIPE_ERROR", "failed to create payment method", nil)
		return
	}

	// Deduplicate: reject if the user already has a card with this fingerprint.
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

	// Attach the PaymentMethod to the Stripe customer.
	_, err = paymentmethod.Attach(pm.ID, &stripe.PaymentMethodAttachParams{
		Customer: stripe.String(user.StripeCustomerID),
	})
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "STRIPE_ERROR", "failed to attach payment method", nil)
		return
	}

	// remove existing local card with the same last4 + brand + expiry
	var existingCard models.Card
	if err := db.Where("user_id = ? AND last4 = ? AND brand = ? AND exp_month = ? AND exp_year = ?",
		user.ID, pm.Card.Last4, string(pm.Card.Brand), pm.Card.ExpMonth, pm.Card.ExpYear).First(&existingCard).Error; err == nil {
		db.Delete(&existingCard)
	}

	// Persist only the safe card metadata — no raw card data is stored.
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

	audit.CardAdded(userID, card.ID, card.Brand, card.Last4)

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

	// Clear the card from any cart that references it.
	cardID := uint(id)
	if err := db.Model(&models.Cart{}).Where("card_id = ? AND user_id = ?", cardID, userID).Update("card_id", nil).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to clear card from cart", nil)
		return
	}

	// Detach from Stripe (best-effort; don't block deletion on Stripe errors).
	if len(card.StripePaymentID) >= 3 && card.StripePaymentID[:3] == "pm_" {
		stripe.Key = h.stripeKey
		_, _ = paymentmethod.Detach(card.StripePaymentID, &stripe.PaymentMethodDetachParams{})
	}

	// Soft-delete the local record.
	if err := db.Delete(&card).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete card", nil)
		return
	}

	audit.CardDeleted(userID, card.ID)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"message": "card deleted",
	})
}

// ensureStripeCustomer creates a Stripe Customer for the user if they don't
// already have one and persists the customer ID. Safe to call concurrently —
// a duplicate Stripe customer is harmless; the local record reflects the first
// successful write.
func ensureStripeCustomer(gdb *gorm.DB, user *models.User) error {
	if user.StripeCustomerID != "" {
		return nil
	}
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
		return err
	}
	user.StripeCustomerID = cust.ID
	if err := gdb.Save(user).Error; err != nil {
		return fmt.Errorf("Failed to save stripe customer: %w", err)
	}
	return nil
}
