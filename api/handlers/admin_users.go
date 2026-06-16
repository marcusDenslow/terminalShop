package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"terminalShop/pkg/audit"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/utils"
)

// AdminUserHandler serves admin-gated user-administration endpoints. Routes that
// use it are mounted behind middleware.RequireAdmin (X-Admin-Key), not the JWT
// auth used by customer-facing endpoints.
type AdminUserHandler struct{}

// NewAdminUserHandler constructs an AdminUserHandler.
func NewAdminUserHandler() *AdminUserHandler {
	return &AdminUserHandler{}
}

// setUserOrderCapRequest is decoded into a raw map first (not this struct) so an
// explicit JSON null (clear the override) can be told apart from an absent field
// (a malformed request). The map carries the single key max_order_cents.

// SetUserOrderCap sets or clears a user's per-user spend-cap override
// (User.MaxOrderCents). Body: {"max_order_cents": <int|null>}.
//   - null  -> clears the override; the user reverts to the global MAX_ORDER_CENTS
//   - 0     -> per-user off-switch (disables the cap for this user)
//   - >0    -> custom ceiling
//
// Negative values are rejected here at the write boundary (HTTP 400) to keep bad
// data out of the column. The read path in cart.go:ConvertCart independently
// defends against a negative that arrives via a direct DB write (falls back to
// the global cap), so the two layers cover each other.
//
// Adapted from the admin OrderHandler.UpdateTracking pattern in
// api/handlers/orders.go (URL id param -> body decode -> row lookup -> update ->
// audit -> respond).
func (h *AdminUserHandler) SetUserOrderCap(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_ID", "invalid user id", nil)
		return
	}

	// Decode raw so an explicit null (clear) is distinguishable from an absent key.
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body", nil)
		return
	}
	rawCap, ok := raw["max_order_cents"]
	if !ok {
		utils.RespondError(w, http.StatusBadRequest, "MISSING_FIELD",
			"max_order_cents is required (use null to clear the override)", nil)
		return
	}

	var capCents *int
	if string(rawCap) != "null" {
		var n int
		if err := json.Unmarshal(rawCap, &n); err != nil {
			utils.RespondError(w, http.StatusBadRequest, "INVALID_BODY",
				"max_order_cents must be an integer or null", nil)
			return
		}
		if n < 0 {
			utils.RespondError(w, http.StatusBadRequest, "INVALID_CAP",
				"max_order_cents must be >= 0 (use null to clear the override)", nil)
			return
		}
		capCents = &n
	}

	var user models.User
	if err := db.First(&user, id).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil)
		return
	}

	// Map form so a nil pointer persists SQL NULL; a struct Updates would skip it.
	if err := db.Model(&user).Updates(map[string]any{"max_order_cents": capCents}).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to update order cap", nil)
		return
	}

	audit.UserOrderCapSet("admin", user.ID, capCents)

	utils.RespondSuccess(w, http.StatusOK, map[string]any{
		"user_id":         user.ID,
		"max_order_cents": capCents,
	})
}
