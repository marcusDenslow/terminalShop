package handlers

import (
	"net/http"
	"strconv"

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
