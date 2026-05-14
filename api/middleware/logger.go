package middleware

import (
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// responseWriter wraps http.ResponseWriter to capture status code and response size.
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
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
			r.URL.Path,
			wrapped.statusCode,
			duration,
			tid,
		)
	})
}
