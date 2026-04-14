package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/paymentmethod"
	"github.com/stripe/stripe-go/v78/setupintent"
	"github.com/stripe/stripe-go/v78/webhook"
	"gorm.io/gorm"

	"terminalShop/pkg/audit"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/utils"
)

// WebhookHandler handles inbound Stripe webhook events.
type WebhookHandler struct {
	webhookSecret string
	stripeKey     string
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(webhookSecret string, stripeKey string) *WebhookHandler {
	return &WebhookHandler{webhookSecret: webhookSecret, stripeKey: stripeKey}
}

// HandleStripe validates the Stripe signature and routes the event.
// This endpoint is intentionally unauthenticated — authentication is
// provided by the webhook signature (HMAC-SHA256).
func (h *WebhookHandler) HandleStripe(w http.ResponseWriter, r *http.Request) {
	// Read the entire body up front; webhook.ConstructEvent requires it.
	const maxBodyBytes = 65536
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "READ_ERROR", "could not read request body", nil)
		return
	}

	// Verify the Stripe signature. Reject anything that doesn't match.
	if h.webhookSecret == "" {
		// No secret configured — log a loud warning but still process in dev.
		log.Println("WARNING: STRIPE_WEBHOOK_SECRET is not set; skipping signature verification")
	} else {
		sig := r.Header.Get("Stripe-Signature")
		if _, err := webhook.ConstructEventWithOptions(body, sig, h.webhookSecret, webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		}); err != nil {
			log.Printf("[webhook] signature verification failed: %v", err)
			utils.RespondError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "webhook signature verification failed", nil)
			return
		}
	}

	var event stripe.Event
	if err := json.Unmarshal(body, &event); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_JSON", "could not parse event", nil)
		return
	}

	switch event.Type {

	case "checkout.session.completed":
		h.handleCheckoutSessionCompleted(event)

	// payment_intent.succeeded fires when a charge completes asynchronously
	// (e.g. 3DS, bank redirects). For confirm=true intents this is normally
	// already handled by ConvertCart, but the webhook is the authoritative source.
	case "payment_intent.succeeded":
		h.handlePaymentIntentSucceeded(event)

	// payment_intent.payment_failed fires when a payment fails after initial
	// confirmation (e.g. bank decline on a deferred 3DS flow).
	case "payment_intent.payment_failed":
		h.handlePaymentIntentFailed(event)

	// Sync local card state when Stripe's side changes.
	case "payment_method.attached",
		"payment_method.detached",
		"payment_method.updated":
		// Handled by the reconcile job and direct API calls; log only.
		log.Printf("[webhook] %s received for payment method", event.Type)
	}

	// Always return 200 so Stripe doesn't retry.
	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) handlePaymentIntentSucceeded(event stripe.Event) {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		log.Printf("[webhook] failed to unmarshal payment_intent: %v", err)
		return
	}

	orderIDStr, ok := pi.Metadata["order_id"]
	if !ok {
		// PaymentIntent not tied to a local order — nothing to do.
		return
	}

	db := database.GetDB()
	var order models.Order
	if err := db.Where("id = ?", orderIDStr).First(&order).Error; err != nil {
		log.Printf("[webhook] order %s not found for pi %s", orderIDStr, pi.ID)
		return
	}

	if order.Status == models.OrderStatusPaid {
		// Already recorded — idempotent.
		return
	}

	txErr := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status":            models.OrderStatusPaid,
			"stripe_payment_id": pi.ID,
		}).Error; err != nil {
			return err
		}
		// Clear the user's cart if it hasn't been cleared yet.
		var cart models.Cart
		if err := tx.Where("user_id = ?", order.UserID).First(&cart).Error; err != nil {
			return nil // Cart already gone.
		}
		if err := tx.Where("cart_id = ?", cart.ID).Delete(&models.CartItem{}).Error; err != nil {
			return fmt.Errorf("failed to delete cart item: %w", err)
		}
		return tx.Model(&cart).Updates(map[string]interface{}{
			"address_id": nil,
			"card_id":    nil,
		}).Error
	})
	if txErr != nil {
		log.Printf("[webhook] failed to record paid status for order %s (pi=%s): %v", orderIDStr, pi.ID, txErr)
		return
	}

	audit.OrderPaid(order.UserID, order.ID, int(pi.Amount), pi.ID)
	log.Printf("[webhook] order %s marked paid via payment_intent.succeeded (pi=%s)", orderIDStr, pi.ID)
}

func (h *WebhookHandler) handlePaymentIntentFailed(event stripe.Event) {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		log.Printf("[webhook] failed to unmarshal payment_intent: %v", err)
		return
	}

	orderIDStr, ok := pi.Metadata["order_id"]
	if !ok {
		return
	}

	db := database.GetDB()
	if err := db.Model(&models.Order{}).Where("id = ? AND status = ?", orderIDStr, models.OrderStatusPending).
		Update("status", models.OrderStatusFailed).Error; err != nil {
		log.Printf("[webhook] failed to mark order %s as failed: %v", orderIDStr, err)
	}

	errMsg := "unknown"
	if pi.LastPaymentError != nil {
		errMsg = pi.LastPaymentError.Msg
	}
	log.Printf("[webhook] payment failed for order %s (pi=%s): %s", orderIDStr, pi.ID, errMsg)
}

func (h *WebhookHandler) handleCheckoutSessionCompleted(event stripe.Event) {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		log.Printf("[webhook] failed to unmarshal checkout session: %v", err)
		return
	}

	if sess.Mode != stripe.CheckoutSessionModeSetup {
		return
	}

	if sess.SetupIntent == nil || sess.Customer == nil {
		log.Printf("[webhook] checkout session missing setup_intent or customer")
		return
	}

	stripe.Key = h.stripeKey

	si, err := setupintent.Get(sess.SetupIntent.ID, nil)
	if err != nil {
		log.Printf("[webhook] failed to get setup intent %s: %v", sess.SetupIntent.ID, err)
		return
	}

	if si.PaymentMethod == nil {
		log.Printf("[webhook] setup intent %s has no payment method", si.ID)
		return
	}

	pm, err := paymentmethod.Get(si.PaymentMethod.ID, nil)
	if err != nil {
		log.Printf("[webhook] failed to get payment method %s: %v", si.PaymentMethod.ID, err)
		return
	}

	db := database.GetDB()
	var user models.User
	if err := db.Where("stripe_customer_id = ?", sess.Customer.ID).First(&user).Error; err != nil {
		log.Printf("[webhook] user not found for customer %s: %v", sess.Customer.ID, err)
		return
	}

	// Idempotency: skip if this payment method is already saved.
	var existing models.Card
	if err := db.Where("user_id = ? AND stripe_payment_id = ?", user.ID, pm.ID).First(&existing).Error; err == nil {
		return
	}

	card := models.Card{
		UserID:          user.ID,
		StripePaymentID: pm.ID,
		Last4:           pm.Card.Last4,
		Brand:           string(pm.Card.Brand),
		ExpMonth:        int(pm.Card.ExpMonth),
		ExpYear:         int(pm.Card.ExpYear),
	}

	if err := db.Create(&card).Error; err != nil {
		log.Printf("[webhook] failed to save card for user %d: %v", user.ID, err)
		return
	}

	audit.CardAdded(user.ID, card.ID, card.Brand, card.Last4)
	log.Printf("[webhook] card saved via checkout.session.completed for user %d", user.ID)
}
