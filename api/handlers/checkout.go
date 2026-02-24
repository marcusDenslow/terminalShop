package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/charge"
	"github.com/stripe/stripe-go/v78/customer"

	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/utils"
)

// CheckoutHandler handles checkout and order creation.
type CheckoutHandler struct {
	stripeKey string
}

// NewCheckoutHandler creates a new checkout handler with the Stripe secret key.
func NewCheckoutHandler(stripeSecretKey string) *CheckoutHandler {
	return &CheckoutHandler{stripeKey: stripeSecretKey}
}

type CheckoutCartItem struct {
	CoffeeID uint `json:"coffee_id"`
	Quantity int  `json:"quantity"`
}

type CheckoutRequest struct {
	Fingerprint     string             `json:"fingerprint"`
	StripeToken     string             `json:"stripe_token"`
	Last4           string             `json:"last4"`
	Brand           string             `json:"brand"`
	ExpMonth        int                `json:"exp_month"`
	ExpYear         int                `json:"exp_year"`
	Items           []CheckoutCartItem `json:"items"`
	ShippingName    string             `json:"shipping_name"`
	ShippingStreet  string             `json:"shipping_street"`
	ShippingStreet2 string             `json:"shipping_street2"`
	ShippingCity    string             `json:"shipping_city"`
	ShippingState   string             `json:"shipping_state"`
	ShippingZip     string             `json:"shipping_zip"`
	ShippingCountry string             `json:"shipping_country"`
	ShippingPhone   string             `json:"shipping_phone"`
}

func (h *CheckoutHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()

	var req CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", nil)
		return
	}

	if req.Fingerprint == "" {
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "fingerprint is required", nil)
		return
	}

	if req.StripeToken == "" {
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "stripe token is required", nil)
		return
	}

	if len(req.Items) <= 0 {
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "cart is empty", nil)
		return
	}

	var user models.User
	if err := db.Where("ssh_key_fingerprint = ?", req.Fingerprint).First(&user).Error; err != nil {
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

	for _, item := range req.Items {
		var coffee models.Coffee
		if err := db.First(&coffee, item.CoffeeID).Error; err != nil {
			utils.RespondError(w, http.StatusBadRequest, "INVALID_PRODUCT", fmt.Sprintf("product %d not found", item.CoffeeID), nil)
			return
		}

		priceInCents := int(coffee.Price * 100)
		lineTotal := priceInCents * item.Quantity
		subtotal += lineTotal

		orderItems = append(orderItems, models.OrderItem{
			CoffeeID: coffee.ID,
			Name:     coffee.Name,
			Price:    priceInCents,
			Quantity: item.Quantity,
		})
	}

	total := subtotal

	chargeParams := &stripe.ChargeParams{
		Amount:      stripe.Int64(int64(total)),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Customer:    stripe.String(user.StripeCustomerID),
		Description: stripe.String(fmt.Sprintf("terminal.shop order for user %d", user.ID)),
	}
	chargeParams.SetSource(req.StripeToken)

	// Stripe minimum charge is $0.50 (50 cents)
	if total < 50 {
		utils.RespondError(w, http.StatusBadRequest, "VALIDATION_ERROR", "order total must be at least $0.50", nil)
		return
	}

	ch, err := charge.New(chargeParams)
	if err != nil {
		utils.RespondError(w, http.StatusPaymentRequired, "PAYMENT_FAILED", fmt.Sprintf("payment failed: %v", err), nil)
		return
	}

	card := models.Card{
		UserID:          user.ID,
		StripePaymentID: req.StripeToken,
		Last4:           req.Last4,
		Brand:           req.Brand,
		ExpMonth:        req.ExpMonth,
		ExpYear:         req.ExpYear,
	}

	db.Create(&card)

	order := models.Order{
		UserID:          user.ID,
		CardID:          card.ID,
		StripePaymentID: ch.ID,
		Status:          models.OrderStatusPaid,
		Subtotal:        subtotal,
		Total:           total,
		ShippingName:    req.ShippingName,
		ShippingStreet:  req.ShippingStreet,
		ShippingStreet2: req.ShippingStreet2,
		ShippingCity:    req.ShippingCity,
		ShippingState:   req.ShippingState,
		ShippingZip:     req.ShippingZip,
		ShippingCountry: req.ShippingCountry,
		ShippingPhone:   req.ShippingPhone,
		Items:           orderItems,
	}

	if err := db.Create(&order).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to create order", nil)
		return
	}

	db.Preload("Items").First(&order, order.ID)

	utils.RespondSuccess(w, http.StatusCreated, map[string]interface{}{
		"order": order,
	})
}
