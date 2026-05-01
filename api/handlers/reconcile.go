package handlers

import (
	"fmt"
	"log/slog"
	"time"

	"terminalShop/pkg/database"
	"terminalShop/pkg/models"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/paymentintent"
	"gorm.io/gorm"
)

var reconcileLog = slog.With("component", "reconcile")

// ReconcileOrders finds pending orders with no Stripe ID older than 10 minutes
// and checks whether Stripe has a succeeded PaymentIntent for them. This fixes
// the case where the server crashed after charging but before the DB transaction
// completed.
func ReconcileOrders(stripeKey string) {
	stripe.Key = stripeKey
	db := database.GetDB()

	var orders []models.Order
	cutoff := time.Now().Add(-10 * time.Minute)
	db.Where("status = ? AND stripe_payment_id = '' AND created_at < ?",
		models.OrderStatusPending, cutoff).Find(&orders)

	if len(orders) == 0 {
		return
	}

	reconcileLog.Info("checking pending orders", "count", len(orders))

	for _, order := range orders {
		params := &stripe.PaymentIntentSearchParams{
			SearchParams: stripe.SearchParams{
				Query: fmt.Sprintf("metadata['order_id']:'%d' AND status:'succeeded'", order.ID),
			},
		}

		iter := paymentintent.Search(params)
		for iter.Next() {
			pi := iter.PaymentIntent()
			reconcileLog.Warn("found succeeded payment intent for unrecorded order — patching",
				"order_id", order.ID, "pi", pi.ID)

			txErr := db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Model(&order).Updates(map[string]interface{}{
					"status":            models.OrderStatusPaid,
					"stripe_payment_id": pi.ID,
				}).Error; err != nil {
					return err
				}
				var cart models.Cart
				if err := tx.Where("user_id = ?", order.UserID).First(&cart).Error; err != nil {
					return nil // cart already gone, nothing to clear
				}
				if err := tx.Where("cart_id = ?", cart.ID).Delete(&models.CartItem{}).Error; err != nil {
					return err
				}
				return tx.Model(&cart).Updates(map[string]interface{}{
					"address_id": nil,
					"card_id":    nil,
				}).Error
			})
			if txErr != nil {
				reconcileLog.Error("failed to patch order", "order_id", order.ID, "error", txErr)
			}
		}
		if err := iter.Err(); err != nil {
			reconcileLog.Error("stripe search error", "order_id", order.ID, "error", err)
		}
	}
}
