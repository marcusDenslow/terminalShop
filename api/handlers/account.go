package handlers

import (
	"encoding/json"
	"net/http"

	"terminalShop/api/middleware"
	"terminalShop/pkg/audit"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/utils"
)

// AccountHandler serves JWT-authed, customer-facing account-settings endpoints.
// Unlike AdminUserHandler (X-Admin-Key gated), every route here edits the
// logged-in user's own row, identified by the JWT user id in context.
type AccountHandler struct {
	// maxOrderCents is the global spend cap, needed to bound a self-limit: a
	// customer may only TIGHTEN their effective ceiling, never raise it.
	maxOrderCents int
}

// NewAccountHandler constructs an AccountHandler. maxOrderCents is the global
// MAX_ORDER_CENTS, wired from cfg in routes.SetupRoutes (same value the cart
// handler receives).
func NewAccountHandler(maxOrderCents int) *AccountHandler {
	return &AccountHandler{maxOrderCents: maxOrderCents}
}

// SetSpendLimit sets or clears the logged-in user's self-service spend limit
// (User.SelfLimitCents). Body: {"self_limit_cents": <int|null>}.
//   - null -> clears the self-limit; the user reverts to the admin/global cap
//   - 0    -> a real "block everything" ceiling (NOT an off-switch; that is the
//     whole point of a customer-set limit — it can only tighten)
//   - >0   -> custom ceiling, must be <= the effective admin/global cap
//
// Security: this endpoint is JWT-authed and edits the caller's OWN row, so a
// self-limit is LOWER-ONLY. A value above the user's effective admin/global cap
// is rejected here (HTTP 400) so a compromised session cannot lift its own
// ceiling; the min() fold in cart.go:ConvertCart is the authoritative second
// guard. Negative values are rejected at the write boundary (the ConvertCart
// read path also ignores a stray negative from a direct DB write).
//
// Mirrors AdminUserHandler.SetUserOrderCap but reads the user id from the JWT
// context instead of a {id} URL param, uses a distinct body key + audit event,
// and gates the value against the cap.
func (h *AccountHandler) SetSpendLimit(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body", nil)
		return
	}
	rawLimit, ok := raw["self_limit_cents"]
	if !ok {
		utils.RespondError(w, http.StatusBadRequest, "MISSING_FIELD", "self_limit_cents is required (use null to clear the limit)", nil)
		return
	}

	var limitCents *int
	if string(rawLimit) != "null" {
		var n int
		if err := json.Unmarshal(rawLimit, &n); err != nil {
			utils.RespondError(w, http.StatusBadRequest, "INVALID_BODY", "self_limit_cents must be an integer or null", nil)
			return
		}
		if n < 0 {
			utils.RespondError(w, http.StatusBadRequest, "INVALID_LIMIT", "self_limit_cents must be >= 0 (use null to clear the limit)", nil)
			return
		}
		limitCents = &n
	}

	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil)
		return
	}

	// A self-limit may only tighten the effective cap, never raise it. Compute
	// the user's effective admin/global ceiling exactly as cart.go:ConvertCart
	// does (per-user override wins when present and non-negative; an active cap
	// is > 0). Reject any self-limit above an ACTIVE admin ceiling. When no admin
	// ceiling is active (global disabled and no override) there is nothing to
	// raise, so any non-negative self-limit is allowed — it can only ADD a cap.
	if limitCents != nil {
		adminCap := h.maxOrderCents
		if user.MaxOrderCents != nil && *user.MaxOrderCents >= 0 {
			adminCap = *user.MaxOrderCents
		}
		if adminCap > 0 && *limitCents > adminCap {
			utils.RespondError(w, http.StatusBadRequest, "SELF_LIMIT_ABOVE_CAP", "self_limit_cents may not exceed your account spend cap", map[string]any{"self_limit_cents": *limitCents, "cap_cents": adminCap})
			return
		}
	}

	// Map form so a nil pointer persists SQL NULL; a struct Updates would skip it.
	if err := db.Model(&user).Updates(map[string]any{"self_limit_cents": limitCents}).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to update spend limit", nil)
		return
	}

	audit.UserSelfLimitSet(user.ID, limitCents)

	utils.RespondSuccess(w, http.StatusOK, map[string]any{
		"user_id":          user.ID,
		"self_limit_cents": limitCents,
	})
}

// SetPrivacyMode sets the logged-in user's account-level privacy default
// (User.PrivacyMode). Body: {"privacy_mode": <bool>}.
//
// PrivacyMode is a SOFT default, not a server-enforced guarantee: it only
// pre-selects "don't keep" at checkout in the TUI. The checkout still sends an
// explicit per-order intent which the backend honors (cart.go:ConvertCart) —
// that is what lets a customer override the default with "save anyway". A
// stricter server-enforced "never save" would be a separate mode.
//
// Mirrors SetSpendLimit: JWT-authed, edits the caller's OWN row.
func (h *AccountHandler) SetPrivacyMode(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB().WithContext(r.Context())
	userID := middleware.UserIDFromContext(r.Context())

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body", nil)
		return
	}
	rawMode, ok := raw["privacy_mode"]
	if !ok {
		utils.RespondError(w, http.StatusBadRequest, "MISSING_FIELD", "privacy_mode is required", nil)
		return
	}
	var enabled bool
	if err := json.Unmarshal(rawMode, &enabled); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "INVALID_BODY", "privacy_mode must be a boolean", nil)
		return
	}

	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		utils.RespondError(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil)
		return
	}

	// Map form so an explicit false persists (a struct Updates skips a zero bool)
	if err := db.Model(&user).Updates(map[string]any{"privacy_mode": enabled}).Error; err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "DATABASE_ERROR", "failed to update privacy mode", nil)
		return
	}

	audit.UserPrivacyModeSet(user.ID, enabled)

	utils.RespondSuccess(w, http.StatusOK, map[string]any{
		"user_id":      user.ID,
		"privacy_mode": enabled,
	})
}
