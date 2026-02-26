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
	// Auth extracts and validates JWT tokens on all requests.
	// If a valid token is present, store the userID in context.
	// If no token or an invalid token, continue as public (no userID).
	// Protected routes use RequireAuth to enforce valid authentication.
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// No token - public request
				next.ServeHTTP(w, r)
				return
			}

			// Expect "Bearer <token>"
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				// Malformed header - continue as public request
				next.ServeHTTP(w, r)
				return
			}

			claims, err := jwtManager.ValidateToken(parts[1])
			if err != nil {
				// Invalid or expired token - continue as public request.
				// RequireAuth will reject on protected routes.
				next.ServeHTTP(w, r)
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
