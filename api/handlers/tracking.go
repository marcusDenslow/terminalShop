package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
)

// TrackingRedirect resolves an order ID and 302s to the carrier's tracking URL.
// Public (no auth) so the URL stays short enough to copy from any TUI width.
func TrackingRedirect(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "bad order id", http.StatusBadRequest)
		return
	}

	var order models.Order
	if err := database.GetDB().Select("tracking_url").Where("id = ?", uint(id)).First(&order).Error; err != nil {
		http.NotFound(w, r)
		return
	}
	if order.TrackingURL == "" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, order.TrackingURL, http.StatusFound)
}
