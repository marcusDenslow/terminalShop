package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests processed, labeled by method, route, and status",
		},
		[]string{"method", "route", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds, labeled by method and route",
			Buckets: []float64{.0001, .0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"method", "route"},
	)

	ordersCreatedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orders_created_total",
			Help: "Total orders successfully created, labeled by status",
		},
		[]string{"status"},
	)
)

// Metrics records request count and latency for every request
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := newResponseWriter(w)

		next.ServeHTTP(wrapped, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unknown"
		}

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(wrapped.statusCode)

		httpRequestsTotal.WithLabelValues(r.Method, route, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, route).Observe(duration)
	})
}

// MetricsHandler returns the Prometheus scrape endpoint
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// RecordOrderCreated records a successfully created order.
func RecordOrderCreated(status string) {
	ordersCreatedTotal.WithLabelValues(status).Inc()
}
