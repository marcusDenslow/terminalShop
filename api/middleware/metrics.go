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

	httpInflightRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_inflight_requests",
			Help: "Number of HTTP requests currently being handled",
		},
	)

	httpRequestSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "HTTP request body size in bytes",
			Buckets: prometheus.ExponentialBuckets(64, 4, 8),
		},
		[]string{"method", "route"},
	)

	httpResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response body size in bytes",
			Buckets: prometheus.ExponentialBuckets(64, 4, 8),
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

	cartConversionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cart_conversion_total",
			Help: "Cart checkout outcomes (success, validation_failed, card_declined, stripe_failed, db_failed, payment_critical)",
		},
		[]string{"outcome"},
	)

	orderValueDollars = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "order_value_dollars",
			Help:    "Distribution of completed order totals, in dollars",
			Buckets: []float64{1, 5, 10, 20, 30, 50, 75, 100, 150, 250, 500, 1000},
		},
	)

	stripeRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "stripe_request_duration_seconds",
			Help:    "Latency of outbound Stripe API calls",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"op", "outcome"},
	)

	authAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_attempts_total",
			Help: "Authentication attempts, by result (success, invalid_secret, missing_fingerprint, user_not_found, registered, error)",
		},
		[]string{"result"},
	)

	rateLimitRejectionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limit_rejections_total",
			Help: "Number of requests rejected by rate limiters",
		},
		[]string{"scope"},
	)

	logMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "log_messages_total",
			Help: "Total log lines emitted, labeled by level",
		},
		[]string{"level"},
	)

	buildInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "terminalshop_build_info",
			Help: "Build metadata; value is always 1, version is on the label",
		},
		[]string{"version"},
	)
)

// Metrics records request count, latency, sizes, and concurrency for every request.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := newResponseWriter(w)

		httpInflightRequests.Inc()
		defer httpInflightRequests.Dec()

		next.ServeHTTP(wrapped, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unknown"
		}

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(wrapped.statusCode)

		httpRequestsTotal.WithLabelValues(r.Method, route, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, route).Observe(duration)
		if r.ContentLength > 0 {
			httpRequestSize.WithLabelValues(r.Method, route).Observe(float64(r.ContentLength))
		}
		httpResponseSize.WithLabelValues(r.Method, route).Observe(float64(wrapped.bytesWritten))
	})
}

// MetricsHandler returns the Prometheus scrape endpoint.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// SetBuildInfo records build metadata as a gauge of value 1.
func SetBuildInfo(version string) {
	buildInfo.WithLabelValues(version).Set(1)
}

// RecordOrderCreated records a successfully created order.
func RecordOrderCreated(status string) {
	ordersCreatedTotal.WithLabelValues(status).Inc()
}

// RecordCartConversion records the outcome of a checkout attempt.
func RecordCartConversion(outcome string) {
	cartConversionTotal.WithLabelValues(outcome).Inc()
}

// ObserveOrderValueCents records the value of a completed order (cents → dollars).
func ObserveOrderValueCents(cents int) {
	orderValueDollars.Observe(float64(cents) / 100.0)
}

// ObserveStripeRequest records latency of a Stripe API call.
func ObserveStripeRequest(op, outcome string, durSec float64) {
	stripeRequestDuration.WithLabelValues(op, outcome).Observe(durSec)
}

// RecordAuthAttempt records an authentication attempt outcome.
func RecordAuthAttempt(result string) {
	authAttemptsTotal.WithLabelValues(result).Inc()
}

// RecordRateLimitRejection records a request blocked by a rate limiter.
func RecordRateLimitRejection(scope string) {
	rateLimitRejectionsTotal.WithLabelValues(scope).Inc()
}

// RecordLogLevel increments the per-level log counter.
func RecordLogLevel(level string) {
	logMessagesTotal.WithLabelValues(level).Inc()
}
