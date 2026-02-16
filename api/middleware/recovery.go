package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"terminalShop/pkg/utils"
)

// Recovery recovers from panics and returns a 500 error
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("PANIC: %v\n%s", err, debug.Stack())

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
