package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"terminalShop/pkg/bring"
	"terminalShop/pkg/notify"
	"terminalShop/pkg/shipping"
	"terminalShop/pkg/shippo"

	"terminalShop/api/middleware"
	"terminalShop/pkg/audit"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/utils"

	"github.com/go-chi/chi/v5"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/refund"
	"gorm.io/gorm"
)

type OrderHandler struct {
	stripeKey           string
	bringAPIUID         string
	bringAPIKey         string
	bringCustomerNumber string
}

type refundRequestPayload struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// refundRequestCooldown is the minimum interval between customer-initiated
// refund requests for the same order. Prevents accidental double-clicks and
// repeated submissions from spamming the operator's Slack channel.
const refundRequestCooldown = 15 * time.Minute

func NewOrderHandler(stripeSecretKey, bringAPIUID, bringAPIKey, bringCustomerNumber string) *OrderHandler {
	return &OrderHandler{
		stripeKey:           stripeSecretKey,
		bringAPIUID:         bringAPIUID,
		bringAPIKey:         bringAPIKey,
		bringCustomerNumber: bringCustomerNumber,
	}
}

// CreateRefundRequest posts a customer refund request into the order's Slack
// thread. It does not issue a Stripe refund.
func (h *OrderHandler) CreateRefundRequest(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ID", "invalid order id", nil)
		return
	}

	var req refundRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", nil)
		return
	}

	req.Reason = strings.TrimSpace(req.Reason)
	req.Message = strings.TrimSpace(req.Message)

	if !models.IsValidRefundReason(req.Reason) {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_REASON", "invalid refund reason", nil)
		return
	}
	if req.Reason == models.RefundRequestReasonOther && req.Message == "" {
		utils.RespondError(w, http.StatusBadRequest, "MESSAGE_REQUIRED", "message is required when reason is other", nil)
		return
	}

	var order models.Order
	if err := db.Preload("User").Where("id = ? AND user_id = ?", id, userID).First(&order).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found", nil)
		return
	}
	if !order.CanRequestRefund() {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_STATE",
			"only paid, shipped, or delivered orders can request refunds", nil)
		return
	}

	if order.LastRefundRequestAt != nil {
		elapsed := time.Since(*order.LastRefundRequestAt)
		if elapsed < refundRequestCooldown {
			retryAfter := int((refundRequestCooldown - elapsed).Seconds()) + 1
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			utils.RespondError(w, http.StatusTooManyRequests, "RATE_LIMITED",
				fmt.Sprintf("another refund request was submitted recently; try again in %d min", retryAfter/60+1),
				nil)
			return
		}
	}

	text := fmt.Sprintf(":money_with_wings: *Refund request* — %s\n*From:* %s <%s>\n*Total:* $%.2f",
		req.Reason, order.User.Name, order.User.Email, order.TotalInDollars())
	if req.Message != "" {
		quoted := "> " + strings.ReplaceAll(req.Message, "\n", "\n> ")
		text += "\n\n" + quoted
	}

	if err := notify.SlackPostToOrderThreadBroadcast(order.ID, text); err != nil {
		utils.RespondError(w, http.StatusBadGateway, "SLACK_ERROR", "failed to send refund request", nil)
		return
	}

	// Record the timestamp after a successful Slack post so a Slack outage
	// doesn't consume the user's cooldown — they can retry immediately.
	if err := db.Model(&order).Update("last_refund_request_at", time.Now()).Error; err != nil {
		slog.Warn("refund request: failed to record timestamp", "order_id", order.ID, "error", err)
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]any{
		"status": "sent",
	})
}

func (h *OrderHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	var orders []models.Order
	db.Where("user_id = ?", userID).
		Preload("Items").
		Preload("Events", func(tx *gorm.DB) *gorm.DB { return tx.Order("created_at ASC") }).
		Order("created_at desc").Find(&orders)

	utils.RespondSuccess(w, http.StatusOK, map[string]any{
		"orders": orders,
	})
}

func (h *OrderHandler) GetOrderStatus(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ID", "invalid order id", nil)
		return
	}

	var order models.Order
	if err := db.Where("id = ? AND user_id = ?", id, userID).First(&order).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found", nil)
		return
	}

	utils.RespondSuccess(w, http.StatusOK, map[string]any{
		"id":     order.ID,
		"status": string(order.Status),
	})
}

// RefundOrder issues a full refund for an order via Stripe and marks it as
// refunded. Only orders in paid or shipped status can be refunded.
func (h *OrderHandler) RefundOrder(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ID", "invalid order id", nil)
		return
	}

	var order models.Order
	if err := db.Where("id = ? AND user_id = ?", id, userID).First(&order).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found", nil)
		return
	}

	// Only paid or shipped orders can be refunded.
	if order.Status != models.OrderStatusPaid && order.Status != models.OrderStatusShipped {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_STATE",
			"only paid or shipped orders can be refunded", nil)
		return
	}

	if order.StripePaymentID == "" {
		utils.RespondError(w, http.StatusBadRequest, "MISSING_PAYMENT_ID",
			"order has no stripe payment id — contact support", nil)
		return
	}

	rf, err := refund.New(&stripe.RefundParams{
		PaymentIntent: stripe.String(order.StripePaymentID),
	})
	if err != nil {
		if stripeErr, ok := err.(*stripe.Error); ok {
			utils.RespondError(w, http.StatusBadRequest, "STRIPE_ERROR", stripeErr.Msg, nil)
			return
		}
		utils.RespondError(w, http.StatusInternalServerError, "REFUND_FAILED", "refund failed", nil)
		return
	}

	if err := db.Model(&order).Update("status", models.OrderStatusRefunded).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "refund issued but failed to update order status", nil)
		return
	}

	audit.OrderRefunded(userID, order.ID, order.Total, rf.ID)

	utils.RespondSuccess(w, http.StatusOK, map[string]any{
		"refund_id": rf.ID,
		"status":    "refunded",
	})
}

// updateTrackingRequest is the JSON body accepted by UpdateTracking.
// shipped_at defaults to now if not otherwise specified
type updateTrackingRequest struct {
	Carrier        string     `json:"carrier"`
	TrackingNumber string     `json:"tracking_number"`
	TrackingURL    string     `json:"tracking_url"`
	ShippedAt      *time.Time `json:"shipped_at"`
}

// UpdateTracking sets carrier metadata on the order. Transitions paid orders
// to shipped automatically. Admin only, gated by RequireAdmin in routes.go,
// not RequireAuth, so no user JWT is needed.
func (h *OrderHandler) UpdateTracking(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ID", "invalid order id", nil)
		return
	}

	var req updateTrackingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body", nil)
		return
	}
	if req.Carrier == "" || req.TrackingNumber == "" {
		utils.RespondError(w, http.StatusBadRequest, "MISSING_FIELDS", "carrier and or tracking_number fields are required", nil)
		return
	}

	var order models.Order
	if err := db.Where("id = ?", id).First(&order).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found", nil)
		return
	}

	shippedAt := req.ShippedAt
	if shippedAt == nil {
		now := time.Now()
		shippedAt = &now
	}

	updates := map[string]any{
		"carrier":         req.Carrier,
		"tracking_number": req.TrackingNumber,
		"tracking_url":    req.TrackingURL,
		"shipped_at":      shippedAt,
	}
	if order.Status == models.OrderStatusPaid {
		updates["status"] = models.OrderStatusShipped
	}
	if err := db.Model(&order).Updates(updates).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to record shipment", nil)
		return
	}

	audit.OrderShipped(order.UserID, order.ID, req.Carrier, req.TrackingNumber)

	utils.RespondSuccess(w, http.StatusOK, map[string]any{
		"order_id":        order.ID,
		"carrier":         req.Carrier,
		"tracking_number": req.TrackingNumber,
		"status":          order.Status,
	})
}

// PurchaseLabel buys a shipping label from shippo for the order, writes
// the carrier, tracking, and label fields onto the order, and changes the status
// from paid to shipped
func (h *OrderHandler) PurchaseLabel(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ID", "invalid order id", nil)
		return
	}

	var order models.Order
	if err := db.Preload("Items.Coffee").Preload("User").Where("id = ?", id).First(&order).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found", nil)
		return
	}

	if order.TrackingNumber != "" {
		utils.RespondError(w, http.StatusConflict, "ALREADY_LABELED", "order already has a tracking number", nil)
		return
	}

	if order.Status != models.OrderStatusPaid {
		utils.RespondError(w, http.StatusConflict, "INVALID_STATE", "only paid orders can be labeled", nil)
		return
	}

	const ozToKg = 0.0283495
	const defaultOunces = 12.0

	totalKg := 0.0
	bringItems := make([]bring.LineItem, 0, len(order.Items))
	shippoItems := make([]shippo.LineItem, 0, len(order.Items))
	for _, item := range order.Items {
		oz := float64(item.Coffee.Ounces)
		if oz <= 0 {
			oz = defaultOunces
		}
		kg := oz * ozToKg
		totalKg += float64(item.Quantity) * kg
		bringItems = append(bringItems, bring.LineItem{Title: item.Name, Quantity: item.Quantity, WeightKg: kg})
		shippoItems = append(shippoItems, shippo.LineItem{Title: item.Name, Quantity: item.Quantity, WeightKg: kg})
	}

	var result *shipping.LabelResult
	switch order.ShippingCountry {
	case "US":
		to := shippo.Address{
			Name: order.ShippingName, Street1: order.ShippingStreet, Street2: order.ShippingStreet2,
			City: order.ShippingCity, State: order.ShippingState, Country: order.ShippingCountry,
			Zip: order.ShippingZip, Phone: order.ShippingPhone,
		}
		client := shippo.NewClient(os.Getenv("SHIPPO_API_KEY"))
		result, err = client.CreateLabel(r.Context(), to, order.User.Email, shippoItems)
		if err != nil {
			utils.RespondError(w, http.StatusBadGateway, "SHIPPO_ERROR", err.Error(), nil)
			return
		}
	case "NO":
		to := bring.BookingAddress{
			Name: order.ShippingName, Street1: order.ShippingStreet, Street2: order.ShippingStreet2,
			City: order.ShippingCity, Country: order.ShippingCountry, Zip: order.ShippingZip,
			Phone: order.ShippingPhone,
		}
		testMode := os.Getenv("ENVIRONMENT") != "production"
		client := bring.NewBookingClient(h.bringAPIUID, h.bringAPIKey, h.bringCustomerNumber, testMode)
		result, err = client.CreateLabel(r.Context(), to, order.User.Email, bringItems)
		if err != nil {
			utils.RespondError(w, http.StatusBadGateway, "BRING_ERROR", err.Error(), nil)
			return
		}
	default:
		utils.RespondError(w, http.StatusBadRequest, "UNSUPPORTED_DESTINATION", "no label vendor configured for this country", nil)
		return
	}

	now := time.Now()
	updates := map[string]any{
		"carrier":               result.Carrier,
		"tracking_number":       result.TrackingNumber,
		"tracking_url":          result.TrackingURL,
		"shipped_at":            &now,
		"status":                models.OrderStatusShipped,
		"shippo_transaction_id": result.TransactionID,
		"label_url":             result.LabelURL,
		"label_cost_cents":      result.CostCents,
	}
	if err := db.Model(&order).Updates(updates).Error; err != nil {
		audit.LabelOrphaned(order.ID, result.TransactionID, result.TrackingNumber, err)
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "label purchased but failed to record on order; ops will reconcile", nil)
		return
	}

	audit.LabelPurchased(order.ID, result.Carrier, result.TrackingNumber, result.TransactionID, result.CostCents)
	go notify.SlackUnpinOrder(order.ID)

	utils.RespondSuccess(w, http.StatusOK, map[string]any{
		"order_id":        order.ID,
		"carrier":         result.Carrier,
		"service":         result.ServiceLevel,
		"tracking_number": result.TrackingNumber,
		"tracking_url":    result.TrackingURL,
		"label_url":       result.LabelURL,
		"cost_cents":      result.CostCents,
		"status":          models.OrderStatusShipped,
	})
}
