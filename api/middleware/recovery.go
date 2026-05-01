package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"terminalShop/pkg/utils"
)

// Recovery recovers from panics and returns a 500 error
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)

				utils.RespondError(
					w,
					http.StatusInternalServerError,
					"INTERNAL_ERROR",
					"an unexpected error occurred",
					nil,
				)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
