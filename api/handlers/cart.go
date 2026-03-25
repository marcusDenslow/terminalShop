package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"terminalShop/api/middleware"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/utils"

	"gorm.io/gorm"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/customer"
	"github.com/stripe/stripe-go/v78/paymentintent"
	"github.com/stripe/stripe-go/v78/paymentmethod"
)

type CartHandler struct {
	stripeKey string
}

// NewCartHandler creates a new cart handler with the Stripe secret key
func NewCartHandler(stripeSecretKey string) *CartHandler {
	return &CartHandler{stripeKey: stripeSecretKey}
}

// getOrCreateCart finds the users cart or creates one.
func getOrCreateCart(userID uint) (*models.Cart, error) {
	db := database.GetDB()
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

func cartResponse(cart *models.Cart) map[string]interface{} {
	db := database.GetDB()

	var items []models.CartItem
	db.Where("cart_id = ? AND quantity > 0", cart.ID).Preload("Coffee").Find(&items)

	subtotal := 0
	itemsData := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		priceInCents := int(item.Coffee.Price * 100)
		lineTotal := priceInCents * item.Quantity
		subtotal += lineTotal
		itemsData = append(itemsData, map[string]interface{}{
			"id":        item.ID,
			"coffee_id": item.CoffeeID,
			"quantity":  item.Quantity,
			"subtotal":  lineTotal,
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

	cart, err := getOrCreateCart(userID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "CART_ERROR", "failed to get cart", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"cart": cartResponse(cart),
	})
}

type SetItemRequest struct {
	CoffeeID uint `json:"coffee_id"`
	Quantity int  `json:"quantity"`
}

func (h *CartHandler) SetItem(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
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

	cart, err := getOrCreateCart(userID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "CART_ERROR", "failed to get cart", nil)
		return
	}

	if req.Quantity <= 0 {
		db.Where("cart_id = ? AND coffee_id = ?", cart.ID, req.CoffeeID).Delete(&models.CartItem{})
	} else {
		var item models.CartItem
		result := db.Where("cart_id = ? AND coffee_id = ?", cart.ID, req.CoffeeID).First(&item)
		if result.Error != nil {
			item = models.CartItem{
				CartID:   cart.ID,
				CoffeeID: req.CoffeeID,
				Quantity: req.Quantity,
			}
			db.Create(&item)
		} else {
			db.Model(&item).Update("quantity", req.Quantity)
		}
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"cart": cartResponse(cart),
	})
}

type SetAddressRequest struct {
	AddressID uint `json:"address_id"`
}

func (h *CartHandler) SetAddress(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
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

	cart, err := getOrCreateCart(userID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "CART_ERROR", "failed to get cart", nil)
		return
	}

	db.Model(cart).Update("address_id", req.AddressID)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"message": "address set",
	})
}

type SetCardRequest struct {
	CardID uint `json:"card_id"`
}

func (h *CartHandler) SetCard(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
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

	cart, err := getOrCreateCart(userID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "CART_ERROR", "failed to get cart", nil)
		return
	}

	db.Model(cart).Update("card_id", req.CardID)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"message": "card set",
	})
}

func (h *CartHandler) ClearCart(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	userID := middleware.UserIDFromContext(r.Context())

	cart, err := getOrCreateCart(userID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "CART_ERROR", "failed to get cart", nil)
		return
	}

	db.Where("cart_id = ?", cart.ID).Delete(&models.CartItem{})

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"message": "cart cleared",
	})
}

func (h *CartHandler) ConvertCart(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	userID := middleware.UserIDFromContext(r.Context())

	cart, err := getOrCreateCart(userID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "CART_ERROR", "failed to get cart", nil)
		return
	}

	var items []models.CartItem
	db.Where("cart_id = ? AND quantity > 0", cart.ID).Preload("Coffee").Find(&items)

	if len(items) <= 0 {
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "cart is empty", nil)
		return
	}

	if cart.AddressID == nil {
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "shipping address is required", nil)
		return
	}

	if cart.CardID == nil {
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "payment card is required", nil)
		return
	}

	var address models.Address
	if err := db.First(&address, *cart.AddressID).Error; err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ADDRESS", "shipping address not found", nil)
		return
	}

	var card models.Card
	if err := db.First(&card, *cart.CardID).Error; err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_CARD", "payment card not found", nil)
		return
	}

	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil)
		return
	}

	stripe.Key = h.stripeKey

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

	var subtotal int
	var orderItems []models.OrderItem
	for _, item := range items {
		priceInCents := int(item.Coffee.Price * 100)
		lineTotal := priceInCents * item.Quantity
		subtotal += lineTotal
		orderItems = append(orderItems, models.OrderItem{
			CoffeeID: item.CoffeeID,
			Name:     item.Coffee.Name,
			Price:    priceInCents,
			Quantity: item.Quantity,
		})
	}

	total := subtotal + cart.ShippingCost

	if total < 50 {
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "order total must be at least $0.50", nil)
		return
	}

	// If the saved card has a token (tok_), convert to a PaymentMethod and attach
	paymentMethodID := card.StripePaymentID
	if len(paymentMethodID) >= 4 && paymentMethodID[:3] == "tok" {
		pm, err := paymentmethod.New(&stripe.PaymentMethodParams{
			Type: stripe.String("card"),
			Card: &stripe.PaymentMethodCardParams{
				Token: stripe.String(paymentMethodID),
			},
		})
		if err != nil {
			utils.RespondError(w, http.StatusInternalServerError, "STRIPE_ERROR", "failed to create payment method", nil)
			return
		}

		_, err = paymentmethod.Attach(pm.ID, &stripe.PaymentMethodAttachParams{
			Customer: stripe.String(user.StripeCustomerID),
		})
		if err != nil {
			utils.RespondError(w, http.StatusInternalServerError, "STRIPE_ERROR", "failed to attach payment method", nil)
			return
		}

		paymentMethodID = pm.ID
		card.StripePaymentID = pm.ID
		db.Save(&card)
	}

	order := models.Order{
		// Create the order record first before charging the card so we always
		// have a record we can reconcile against if anything goes wrong mid-flight.
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
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to create order", nil)
		return
	}

	// Charge via stripe, keyed to the order ID so retrieves never double-charge.
	piParams := &stripe.PaymentIntentParams{
		Amount:        stripe.Int64(int64(total)),
		Currency:      stripe.String(string(stripe.CurrencyUSD)),
		Customer:      stripe.String(user.StripeCustomerID),
		PaymentMethod: stripe.String(paymentMethodID),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled:        stripe.Bool(true),
			AllowRedirects: stripe.String("never"),
		},
		Confirm:     stripe.Bool(true),
		Description: stripe.String(fmt.Sprintf("terminal.shop order for user %d", userID)),
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

	pi, err := paymentintent.New(piParams)
	if err != nil {
		// charge failed, mark the otder so it can be identified and or cleaned up
		db.Model(&order).Update("status", models.OrderStatusFailed)
		if stripeErr, ok := err.(*stripe.Error); ok && stripeErr.Type == stripe.ErrorTypeCard {
			utils.RespondError(w, http.StatusPaymentRequired, "CARD_ERROR", stripeErr.Msg, nil)
			return
		}
		utils.RespondError(w, http.StatusPaymentRequired, "PAYMENT_FAILED", "payment failed", nil)
		return
	}

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
		log.Printf("[CRITICAL] order %d charged (pi=%s) but failed to record: %v", order.ID, pi.ID, txErr) 
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "payment succeeded but failed to record order", nil)
		return
	}

	db.Preload("Items").First(&order, order.ID)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"order": order,
	})
}
