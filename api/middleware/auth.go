package middleware

import (
	"context"
	"net/http"
	"strings"

	"terminalShop/pkg/auth"
	"terminalShop/pkg/utils"
)

type contextKey string

const userIDKey contextKey = "userID"

func Auth(jwtManager *auth.JWTManager) func(http.Handler) http.Handler {
	// Auth validates JWT tokens on all requests.
	// if a valid token is present, store an userID in context
	// if no token, continue as public (no userID).
	// If invalid token, returns 401
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// No token - public req
				next.ServeHTTP(w, r)
				return
			}

			// Expect "Bearer <token>"
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				utils.RespondError(w, http.StatusUnauthorized, "INVALID_AUTH_BEARER", "authorization header must be Bearer <token>", nil)
				return
			}

			claims, err := jwtManager.ValidateToken(parts[1])
			if err != nil {
				utils.RespondError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid or expired token", nil)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Reject requests without a valid user context.
// User on protected route groups.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserIDFromContext(r.Context()) == 0 {
			utils.RespondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func UserIDFromContext(ctx context.Context) uint {
	if id, ok := ctx.Value(userIDKey).(uint); ok {
		return id
	}
	return 0
}
