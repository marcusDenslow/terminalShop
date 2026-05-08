package middleware

import (
	"net/http"
	"os"
	"terminalShop/pkg/utils"
)

// RequireAdmin gates a handler behind X-Admin-Key matching the ADMIN_API_KEY
// environment variable. if ADMIN_API_KEY is not set the endpoint returns 503, admin actions
// must be explicitly enabled, never be the default option
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := os.Getenv("ADMIN_API_KEY")
		if expected == "" {
			utils.RespondError(w, http.StatusServiceUnavailable, "ADMIN_DISABLED", "admin endpoints are disabled", nil)
			return
		}
		if r.Header.Get("X-Admin-Key") != expected {
			utils.RespondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid admin api key", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}
