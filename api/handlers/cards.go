package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

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

const RedirectTTL = 10 * time.Minute

func storeRedirect(stripeURL, purpose string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	now := time.Now()
	rec := models.PayRedirect{
		Token:     token,
		URL:       stripeURL,
		Purpose:   purpose,
		CreatedAt: now,
		ExpiresAt: now.Add(RedirectTTL),
	}
	if err := database.GetDB().Create(&rec).Error; err != nil {
		return "", err
	}
	return token, nil
}

// PayRedirect resolves a short token and redirects to the Stripe checkout URL.
func PayRedirect(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	db := database.GetDB().WithContext(r.Context())
	var rec models.PayRedirect
	if err := db.Where("token = ?", token).First(&rec).Error; err != nil {
		http.NotFound(w, r)
		return
	}
	if time.Now().After(rec.ExpiresAt) {
		db.Where("token = ?", rec.Token).Delete(&models.PayRedirect{})
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, rec.URL, http.StatusFound)
}

// CardHandler handles saved payment method CRUD.
type CardHandler struct {
	appURL string
}

// NewCardHandler creates a new card handler. Stripe credentials are wired
// once at startup via stripe.Key in api/main.go.
func NewCardHandler(appURL string) *CardHandler {
	return &CardHandler{appURL: appURL}
}

// GetCards returns all saved cards for the authenticated user.
// Cards past their retention deadline are filtered out at read time;
// physical deletion runs in handlers.ReconcileExpiredCards.
func (h *CardHandler) GetCards(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	var cards []models.Card
	db.Where(
		"user_id = ? AND (storage_expires_at IS NULL OR storage_expires_at > ?)",
		userID, time.Now(),
	).Order("is_default DESC, created_at DESC").Find(&cards)

	utils.RespondSuccess(w, http.StatusOK, map[string]any{
		"cards": cards,
	})
}

// func utils.RespondError(w http.ResponseWriter, statusCode int, code string, message string, details map[string]any)

// GetCard returns a single saved card by ID.
func (h *CardHandler) GetCard(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
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

	utils.RespondSuccess(w, http.StatusOK, map[string]any{
		"card": card,
	})
}

func (h *CardHandler) SetDefaultCard(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
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

	if card.IsStorageExpired(time.Now()) {
		if err := expireStoredCard(db, &card); err != nil {
			utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to expire card", nil)
			return
		}
		respondCardStorageExpired(w)
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

	utils.RespondSuccess(w, http.StatusOK, map[string]any{
		"card": card,
	})
}

func (h *CardHandler) CollectCard(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil)
		return
	}

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

	token, err := storeRedirect(sess.URL, models.RedirectPurposeAddCard)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate redirect token", nil)
		return
	}
	utils.RespondSuccess(w, http.StatusOK, map[string]any{
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
	db := database.GetDB().WithContext(r.Context())
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

	utils.RespondSuccess(w, http.StatusCreated, map[string]any{
		"card": card,
	})
}

// DeleteCard removes a saved card. If the card is currently set on the user's
// cart, the cart's card selection is cleared.
func (h *CardHandler) DeleteCard(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
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
		_, _ = paymentmethod.Detach(card.StripePaymentID, &stripe.PaymentMethodDetachParams{})
	}

	// Soft-delete the local record.
	if err := db.Delete(&card).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete card", nil)
		return
	}

	audit.CardDeleted(userID, card.ID)

	utils.RespondSuccess(w, http.StatusOK, map[string]any{
		"message": "card deleted",
	})
}

// expireStoredCard removes a card whose retention deadline elapsed.
// Local state (cart references + card row) is committed atomically;
// Stripe detach and audit run only after the local commit so retries
// cannot double-fire the audit event. Stripe detach is gated on
// stripe.Key being non-empty so tests (which never init the SDK key)
// skip the real network call without an explicit flag.
func expireStoredCard(db *gorm.DB, card *models.Card) error {
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Cart{}).Where("card_id = ? AND user_id = ?", card.ID, card.UserID).Update("card_id", nil).Error; err != nil {
			return fmt.Errorf("failed to clear expired card from cart: %w", err)
		}
		if err := tx.Delete(card).Error; err != nil {
			return fmt.Errorf("failed to delete expired card: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	if stripe.Key != "" && strings.HasPrefix(card.StripePaymentID, "pm_") {
		if _, err := paymentmethod.Detach(card.StripePaymentID, &stripe.PaymentMethodDetachParams{}); err != nil {
			slog.Warn("failed to detach expired card", "card_id", card.ID, "error", err)
		}
	}

	audit.CardExpired(card.UserID, card.ID)
	return nil
}

func refreshCardStorageTTL(db *gorm.DB, cardID uint, now time.Time) error {
	return db.Model(&models.Card{}).Where("id = ?", cardID).Updates(map[string]any{
		"last_used_at":       now,
		"storage_expires_at": models.CardStorageExpiresAt(now),
	}).Error
}

func respondCardStorageExpired(w http.ResponseWriter) {
	utils.RespondError(
		w,
		http.StatusGone,
		"CARD_STORAGE_EXPIRED",
		"saved card expired, add it again",
		nil,
	)
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
		return fmt.Errorf("failed to save stripe customer: %w", err)
	}
	return nil
}
