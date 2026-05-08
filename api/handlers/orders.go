package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/refund"

	"terminalShop/api/middleware"
	"terminalShop/pkg/audit"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/utils"
)

type OrderHandler struct {
	stripeKey string
}

func NewOrderHandler(stripeSecretKey string) *OrderHandler {
	return &OrderHandler{stripeKey: stripeSecretKey}
}

func (h *OrderHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	var orders []models.Order
	db.Where("user_id = ?", userID).Preload("Items").Order("created_at desc").Find(&orders)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"orders": orders,
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

	stripe.Key = h.stripeKey

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

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
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
// not RequireAuth, so no user JWT is needed
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

	updates := map[string]interface{}{
		"carrier":         req.Carrier,
		"tracking_number": req.TrackingNumber,
		"tracking_url":    req.TrackingURL,
		"shipped_at":      shippedAt,
	}
	if order.Status == models.OrderStatusPaid {
		updates["status"] = models.OrderStatusShipped
	}

	if err := db.Model(&order).Updates(updates).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to update order", nil)
		return
	}

	audit.OrderShipped(order.UserID, order.ID, req.Carrier, req.TrackingURL)

	utils.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"order_id":        order.ID,
		"carrier":         req.Carrier,
		"tracking_number": req.TrackingNumber,
		"status":          updates["status"],
	})
}
