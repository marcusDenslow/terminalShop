package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/paymentintent"
	"github.com/stripe/stripe-go/v78/paymentmethod"
	"gorm.io/gorm"

	"go.opentelemetry.io/otel"

	"terminalShop/api/middleware"
	"terminalShop/pkg/audit"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/notify"
	"terminalShop/pkg/utils"
)

type CartHandler struct {
	stripeKey string
	appURL    string
}

// NewCartHandler creates a new cart handler with the Stripe secret key
func NewCartHandler(stripeSecretKey, appURL string) *CartHandler {
	return &CartHandler{stripeKey: stripeSecretKey, appURL: appURL}
}

// getOrCreateCart finds the users cart or creates one.
func getOrCreateCart(ctx context.Context, userID uint) (*models.Cart, error) {
	db := database.GetDB().WithContext(ctx)
	var cart models.Cart
	err := db.Where("user_id = ?", userID).First(&cart).Error
	if err != nil {
		cart = models.Cart{UserID: userID}
		if createErr := db.Create(&cart).Error; createErr != nil {
			return nil, fmt.Errorf("failed to create cart: %w", createErr)
		}
	}
	return &cart, nil
}

func cartResponse(ctx context.Context, cart *models.Cart) map[string]interface{} {
	db := database.GetDB().WithContext(ctx)

	var items []models.CartItem
	db.Where("cart_id = ? AND quantity > 0", cart.ID).Preload("Coffee").Find(&items)

	subtotal := 0
	itemsData := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		lineTotal := item.Coffee.Price * item.Quantity
		subtotal += lineTotal
		itemsData = append(itemsData, map[string]interface{}{
			"id":        item.ID,
			"coffee_id": item.CoffeeID,
			"quantity":  item.Quantity,
			"subtotal":  lineTotal, // already in cents
			"coffee":    item.Coffee,
		})
	}

	return map[string]interface{}{
		"items":            itemsData,
		"subtotal":         subtotal,
		"address_id":       cart.AddressID,
		"card_id":          cart.CardID,
		"shipping_cost":    cart.ShippingCost,
		"shipping_service": cart.ShippingService,
	}
}

func (h *CartHandler) GetCart(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	cart, err := getOrCreateCart(r.Context(), userID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "CART_ERROR", "failed to get cart", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"cart": cartResponse(r.Context(), cart),
	})
}

type SetItemRequest struct {
	CoffeeID uint `json:"coffee_id"`
	Quantity int  `json:"quantity"`
}

func (h *CartHandler) SetItem(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	var req SetItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", nil)
		return
	}

	if req.CoffeeID == 0 {
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "coffee_id is required", nil)
		return
	}

	var coffee models.Coffee
	if err := db.First(&coffee, req.CoffeeID).Error; err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_PRODUCT", "product not found", nil)
		return
	}

	cart, err := getOrCreateCart(r.Context(), userID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "CART_ERROR", "failed to get cart", nil)
		return
	}

	if req.Quantity <= 0 {
		if err := db.Where("cart_id = ? AND coffee_id = ?", cart.ID, req.CoffeeID).Delete(&models.CartItem{}).Error; err != nil {
			utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete cart item", nil)
			return
		}
	} else {
		var item models.CartItem
		result := db.Where("cart_id = ? AND coffee_id = ?", cart.ID, req.CoffeeID).First(&item)
		if result.Error != nil {
			item = models.CartItem{
				CartID:   cart.ID,
				CoffeeID: req.CoffeeID,
				Quantity: req.Quantity,
			}
			if err := db.Create(&item).Error; err != nil {
				utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create item", nil)
				return
			}
		} else {
			if err := db.Model(&item).Update("quantity", req.Quantity).Error; err != nil {
				utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update quantity", nil)
				return
			}
		}
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"cart": cartResponse(r.Context(), cart),
	})
}

type SetAddressRequest struct {
	AddressID uint `json:"address_id"`
}

func (h *CartHandler) SetAddress(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	var req SetAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", nil)
		return
	}

	var address models.Address
	if err := db.Where("id = ? AND user_id = ?", req.AddressID, userID).First(&address).Error; err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ADDRESS", "address not found", nil)
		return
	}

	cart, err := getOrCreateCart(r.Context(), userID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "CART_ERROR", "failed to get cart", nil)
		return
	}

	if err := db.Model(cart).Update("address_id", req.AddressID).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to set address", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"message": "address set",
	})
}

type SetCardRequest struct {
	CardID uint `json:"card_id"`
}

func (h *CartHandler) SetCard(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	var req SetCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", nil)
		return
	}

	var card models.Card
	if err := db.Where("id = ? AND user_id = ?", req.CardID, userID).First(&card).Error; err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_CARD", "card not found", nil)
		return
	}

	cart, err := getOrCreateCart(r.Context(), userID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "CART_ERROR", "failed to get cart", nil)
		return
	}

	if err := db.Model(cart).Update("card_id", req.CardID).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to set card", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"message": "card set",
	})
}

func (h *CartHandler) ClearCart(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	cart, err := getOrCreateCart(r.Context(), userID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "CART_ERROR", "failed to get cart", nil)
		return
	}

	if err := db.Where("cart_id = ?", cart.ID).Delete(&models.CartItem{}).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete cart item", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"message": "cart cleared",
	})
}

func (h *CartHandler) ConvertCart(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	ctx, span := otel.Tracer("api").Start(r.Context(), "cart.convert")
	defer span.End()
	r = r.WithContext(ctx)

	cart, err := getOrCreateCart(r.Context(), userID)
	if err != nil {
		middleware.RecordCartConversion("cart_lookup_failed")
		utils.RespondError(w, http.StatusInternalServerError, "CART_ERROR", "failed to get cart", nil)
		return
	}

	var items []models.CartItem
	db.Where("cart_id = ? AND quantity > 0", cart.ID).Preload("Coffee").Find(&items)

	if len(items) == 0 {
		middleware.RecordCartConversion("validation_empty_cart")
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "cart is empty", nil)
		return
	}

	if cart.AddressID == nil {
		middleware.RecordCartConversion("validation_missing_address")
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "shipping address is required", nil)
		return
	}

	if cart.CardID == nil {
		middleware.RecordCartConversion("validation_missing_card")
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "payment card is required", nil)
		return
	}

	var address models.Address
	if err := db.First(&address, *cart.AddressID).Error; err != nil {
		middleware.RecordCartConversion("validation_invalid_address")
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ADDRESS", "shipping address not found", nil)
		return
	}

	var card models.Card
	if err := db.First(&card, *cart.CardID).Error; err != nil {
		middleware.RecordCartConversion("validation_invalid_card")
		utils.RespondError(w, http.StatusBadRequest, "INVALID_CARD", "payment card not found", nil)
		return
	}

	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		middleware.RecordCartConversion("user_not_found")
		utils.RespondError(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil)
		return
	}

	stripe.Key = h.stripeKey

	// Ensure the user has a Stripe customer (shared helper with cards handler).
	if err := ensureStripeCustomer(db, &user); err != nil {
		middleware.RecordCartConversion("stripe_customer_failed")
		utils.RespondError(w, http.StatusInternalServerError, "STRIPE_ERROR", "failed to create stripe customer", nil)
		return
	}

	// All cards are now saved as pm_ PaymentMethods. This branch handles any
	// legacy tok_ records that predate the server-side tokenization refactor.
	paymentMethodID := card.StripePaymentID
	if len(paymentMethodID) >= 3 && paymentMethodID[:3] == "tok" {
		pm, pmErr := paymentmethod.New(&stripe.PaymentMethodParams{
			Type: stripe.String("card"),
			Card: &stripe.PaymentMethodCardParams{
				Token: stripe.String(paymentMethodID),
			},
		})
		if pmErr != nil {
			utils.RespondError(w, http.StatusInternalServerError, "STRIPE_ERROR", "failed to create payment method", nil)
			return
		}
		if _, attachErr := paymentmethod.Attach(pm.ID, &stripe.PaymentMethodAttachParams{
			Customer: stripe.String(user.StripeCustomerID),
		}); attachErr != nil {
			utils.RespondError(w, http.StatusInternalServerError, "STRIPE_ERROR", "failed to attach payment method", nil)
			return
		}
		paymentMethodID = pm.ID
		card.StripePaymentID = pm.ID
		if err := db.Save(&card).Error; err != nil {
			utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save card", nil)
			return
		}
	}

	var subtotal int
	var orderItems []models.OrderItem
	for _, item := range items {
		lineTotal := item.Coffee.Price * item.Quantity
		subtotal += lineTotal
		orderItems = append(orderItems, models.OrderItem{
			CoffeeID: item.CoffeeID,
			Name:     item.Coffee.Name,
			Price:    item.Coffee.Price,
			Quantity: item.Quantity,
		})
	}

	total := subtotal + cart.ShippingCost

	if total < 50 {
		middleware.RecordCartConversion("validation_min_amount")
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "order total must be at least $0.50", nil)
		return
	}

	// Create the order record BEFORE charging so we always have something to
	// reconcile against if the server crashes mid-flight.
	order := models.Order{
		UserID:          user.ID,
		CardID:          card.ID,
		StripePaymentID: "",
		Status:          models.OrderStatusPending,
		Subtotal:        subtotal,
		ShippingCost:    cart.ShippingCost,
		Total:           total,
		ShippingName:    address.Name,
		ShippingStreet:  address.Street,
		ShippingStreet2: address.Street2,
		ShippingCity:    address.City,
		ShippingState:   address.State,
		ShippingZip:     address.Zip,
		ShippingCountry: address.Country,
		ShippingPhone:   address.Phone,
		Items:           orderItems,
	}

	if err := db.Create(&order).Error; err != nil {
		middleware.RecordCartConversion("db_create_order_failed")
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to create order", nil)
		return
	}

	audit.OrderCreated(userID, order.ID, total)

	// Charge the card. The idempotency key is tied to the order ID so retries
	// never produce a double charge.
	piParams := &stripe.PaymentIntentParams{
		Amount:             stripe.Int64(int64(total)),
		Currency:           stripe.String(string(stripe.CurrencyUSD)),
		Customer:           stripe.String(user.StripeCustomerID),
		PaymentMethod:      stripe.String(paymentMethodID),
		OffSession:         stripe.Bool(true),
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		ReturnURL:          stripe.String(h.appURL + "/post-3ds"),
		Confirm:            stripe.Bool(true),
		Description:        stripe.String(fmt.Sprintf("terminal.shop order #%d", order.ID)),
		Shipping: &stripe.ShippingDetailsParams{
			Name: stripe.String(address.Name),
			Address: &stripe.AddressParams{
				Line1:      stripe.String(address.Street),
				Line2:      stripe.String(address.Street2),
				City:       stripe.String(address.City),
				State:      stripe.String(address.State),
				PostalCode: stripe.String(address.Zip),
				Country:    stripe.String(address.Country),
			},
		},
	}
	piParams.SetIdempotencyKey(fmt.Sprintf("order-%d", order.ID))
	piParams.Metadata = map[string]string{
		"order_id": fmt.Sprintf("%d", order.ID),
	}

	stripeStart := time.Now()
	pi, err := paymentintent.New(piParams)
	stripeDur := time.Since(stripeStart).Seconds()
	if err != nil {
		middleware.ObserveStripeRequest("payment_intent_create", "error", stripeDur)
		if stripeErr, ok := err.(*stripe.Error); ok {
			if stripeErr.Code == stripe.ErrorCodeAuthenticationRequired && stripeErr.PaymentIntent != nil {
				h.respondRequiresAction(w, &order, stripeErr.PaymentIntent)
				return
			}
			db.Model(&order).Update("status", models.OrderStatusFailed)
			switch stripeErr.Type {
			case stripe.ErrorTypeCard:
				middleware.RecordCartConversion("card_declined")
				audit.OrderFailed(userID, order.ID, total, stripeErr.Msg)
				utils.RespondError(w, http.StatusPaymentRequired, mapStripeCardErrCode(stripeErr.Code), stripeErr.Msg, nil)
			case stripe.ErrorTypeInvalidRequest:
				middleware.RecordCartConversion("stripe_invalid_request")
				audit.OrderFailed(userID, order.ID, total, stripeErr.Msg)
				utils.RespondError(w, http.StatusBadRequest, "INVALID_PAYMENT", stripeErr.Msg, nil)
			default:
				middleware.RecordCartConversion("stripe_error")
				audit.OrderFailed(userID, order.ID, total, stripeErr.Error())
				utils.RespondError(w, http.StatusInternalServerError, "PAYMENT_FAILED", "payment failed", nil)
			}
			return
		}
		db.Model(&order).Update("status", models.OrderStatusFailed)
		middleware.RecordCartConversion("stripe_error")
		audit.OrderFailed(userID, order.ID, total, err.Error())
		utils.RespondError(w, http.StatusInternalServerError, "PAYMENT_FAILED", "payment failed", nil)
		return
	}
	middleware.ObserveStripeRequest("payment_intent_create", "success", stripeDur)

	if pi.Status == stripe.PaymentIntentStatusRequiresAction {
		h.respondRequiresAction(w, &order, pi)
		return
	}

	// Commit payment ID and clear cart in a single transaction.
	txErr := db.Transaction(func(tx *gorm.DB) error {
		order.StripePaymentID = pi.ID
		order.Status = models.OrderStatusPaid
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		if err := tx.Where("cart_id = ?", cart.ID).Delete(&models.CartItem{}).Error; err != nil {
			return err
		}
		return tx.Model(cart).Updates(map[string]interface{}{
			"address_id": nil,
			"card_id":    nil,
		}).Error
	})
	if txErr != nil {
		// Card was charged but we could not persist the outcome. This is the
		// most dangerous failure mode: the customer paid but has no order.
		// The reconciliation job will catch this within 5 minutes.
		// The audit log entry below must be treated as a critical alert.
		middleware.RecordCartConversion("payment_critical")
		audit.PaymentCritical(order.ID, pi.ID, txErr)
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "payment succeeded but failed to record order — please contact support", nil)
		return
	}

	audit.OrderPaid(userID, order.ID, total, pi.ID)
	middleware.RecordOrderCreated(string(order.Status))
	middleware.RecordCartConversion("success")
	middleware.ObserveOrderValueCents(total)

	if err := db.Preload("Items").First(&order, order.ID).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to preload items", nil)
		return
	}
	go notify.SlackOrderPaid(&order)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"order": order,
	})
}

// respondRequiresAction handles a PaymentIntent that needs 3DS auth
// The order stays pending. the webhook flips it to paid once the customer
// completes the bank-hosted challenge, the tui polls /api/v1/orders/{id}/status
func (h *CartHandler) respondRequiresAction(w http.ResponseWriter, order *models.Order, pi *stripe.PaymentIntent) {
	db := database.GetDB()

	if pi.NextAction == nil || pi.NextAction.RedirectToURL == nil || pi.NextAction.RedirectToURL.URL == "" {
		middleware.RecordCartConversion("sca_missing_next_action")
		db.Model(order).Update("status", models.OrderStatusFailed)
		audit.OrderFailed(order.UserID, order.ID, order.Total, "stripe returned requires_action without redirect url")
		utils.RespondError(w, http.StatusBadGateway, "PAYMENT_FAILED", "stripe did not return a 3ds url", nil)
		return
	}

	// persist the paymentIntent id before responding so the reconciliation jon
	// and webhook can match the eventual succeeded/failed event to this order
	// even if the customer drops off mid-auth
	if err := db.Model(order).Update("stripe_payment_id", pi.ID).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to save payment id", nil)
		return
	}

	token, err := storeRedirect(pi.NextAction.RedirectToURL.URL)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to allocate redirect token", nil)
		return
	}

	middleware.RecordCartConversion("requires_action")
	audit.OrderRequiresAction(order.UserID, order.ID, pi.ID)

	utils.RespondSuccess(w, http.StatusAccepted, map[string]any{
		"order_id":          order.ID,
		"payment_intent_id": pi.ID,
		"status":            "requires_action",
		"redirect_token":    token,
		"redirect_url":      fmt.Sprintf("%s/pay/%s", h.appURL, token),
	})
}

func mapStripeCardErrCode(code stripe.ErrorCode) string {
	switch code {
	case stripe.ErrorCodeAuthenticationRequired:
		return "AUTHENTICATION_REQUIRED"
	case "authentication_not_supported":
		return "AUTHENTICATION_NOT_SUPPORTED"
	case "card_declined", "":
		return "CARD_DECLINED"
	case "expired_card":
		return "CARD_EXPIRED"
	case "incorrect_cvc":
		return "CARD_CVC_FAILED"
	case "insufficient_funds":
		return "CARD_INSUFFICIENT_FUNDS"
	case "processing_error":
		return "CARD_PROCESSING_ERROR"
	}
	return "CARD_DECLINED"
}

//nolint:unused // wired into Convert cart later
func appURLForHandler(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if fh := r.Header.Get("X-Forwarded-Host"); fh != "" {
		host = fh
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}
