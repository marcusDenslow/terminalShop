package middleware

import (
	"net/http"
	"time"

	"strconv"

	"github.com/go-chi/httprate"
	"terminalShop/pkg/utils"
)

// RateLimitByIP returns a per-IP rate limiter for public endpoints.
func RateLimitByIP(limit int, window time.Duration) func(http.Handler) http.Handler {
	return httprate.Limit(
		limit, window,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			RecordRateLimitRejection("ip")
			utils.RespondError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests", nil)
		}),
	)
}

// RateLimitByUser returns a per-authenticated-user rate limiter for protected endpoints.
func RateLimitByUser(limit int, window time.Duration) func(http.Handler) http.Handler {
	return httprate.Limit(
		limit, window,
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			userID := UserIDFromContext(r.Context())
			if userID == 0 {
				// Fall back to IP if no user context (shouldn't happen on protected routes).
				return httprate.KeyByIP(r)
			}
			return "user:" + strconv.FormatUint(uint64(userID), 10), nil
		}),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			RecordRateLimitRejection("user")
			utils.RespondError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests", nil)
		}),
	)
}
