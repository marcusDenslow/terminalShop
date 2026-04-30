package middleware

import (
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Logger logs HTTP requests
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := newResponseWriter(w)
		next.ServeHTTP(wrapped, r)

		sc := trace.SpanContextFromContext(r.Context())
		tid := ""
		if sc.HasTraceID() {
			tid = sc.TraceID().String()
		}

		duration := time.Since(start)

		log.Printf(
			"%s %s %d %s trace_id=%s",
			r.Method,
			r.RequestURI,
			wrapped.statusCode,
			duration,
			tid,
		)
	})
}
