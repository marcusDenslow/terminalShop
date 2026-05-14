package routes

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"terminalShop/api/handlers"
	"terminalShop/api/middleware"
	"terminalShop/pkg/auth"

	"github.com/riandyrn/otelchi"
)

// SetupRoutes configures all API routes
func SetupRoutes(
	version string,
	stripeSecretKey string,
	stripeWebhookSecret string,
	jwtManager *auth.JWTManager,
	authFingerprintKey string,
	shippoAPIKey string,
	bringAPIUID string,
	bringAPIKey string,
	bringCustomerNumber string,
	shippoWebhookSecret string,
	appURL string,
) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(otelchi.Middleware("terminalshop-api", otelchi.WithChiRoutes(r)))
	r.Use(middleware.Logger)
	r.Use(middleware.CORS())
	r.Use(middleware.Recovery)
	r.Use(middleware.Metrics)
	r.Use(middleware.Auth(jwtManager))
	// Broad IP-level rate limit: 200 req/min per IP across all endpoints.
	r.Use(middleware.RateLimitByIP(200, time.Minute))

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(version)
	productHandler := handlers.NewProductHandler()
	authHandler := handlers.NewAuthHandler(jwtManager, authFingerprintKey)
	cartHandler := handlers.NewCartHandler(stripeSecretKey)
	cardHandler := handlers.NewCardHandler(stripeSecretKey, appURL)
	orderHandler := handlers.NewOrderHandler(stripeSecretKey, bringAPIUID, bringAPIKey, bringCustomerNumber)
	addressHandler := handlers.NewAddressHandler(shippoAPIKey, bringAPIUID, bringAPIKey)
	viewHandler := handlers.NewViewHandler()
	webhookHandler := handlers.NewWebhookHandler(stripeWebhookSecret, stripeSecretKey, shippoWebhookSecret)

	// Short payment redirect and success page — no auth required.
	r.Get("/pay/{token}", handlers.PayRedirect)
	r.Get("/card-added", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><body style="font-family:monospace;text-align:center;padding:4rem"><h2>Card added.</h2><p>You can close this tab.</p></body></html>`))
	})
	r.Handle("/metrics", middleware.MetricsHandler())

	r.Route("/api/v1", func(r chi.Router) {
		// Health — accept HEAD for cheap probes from monitoring tools.
		r.Get("/health", healthHandler.GetHealth)
		r.Head("/health", healthHandler.GetHealth)
		r.Get("/ping", healthHandler.Ping)
		r.Head("/ping", healthHandler.Ping)

		// Stripe webhooks — unauthenticated, verified via HMAC signature.
		// Apply a generous IP rate limit here (Stripe retries from many IPs).
		r.With(middleware.RateLimitByIP(60, time.Minute)).
			Post("/webhooks/stripe", webhookHandler.HandleStripe)
		r.With(middleware.RateLimitByIP(60, time.Minute)).
			Post("/webhooks/shippo", webhookHandler.HandleShippo)

		// Auth — stricter IP rate limit to protect against brute-force.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RateLimitByIP(20, time.Minute))
			r.Post("/auth/register", authHandler.RegisterWithSSHKey)
			r.Get("/auth/user", authHandler.GetUserBySSHKey)
			r.Post("/auth/token", authHandler.GetToken)
		})

		// Public product listing
		r.Get("/products", productHandler.GetProducts)
		r.Get("/products/{id}", productHandler.GetProduct)

		// Protected routes — require a valid JWT
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth)

			r.Get("/view/init", viewHandler.GetViewInit)

			// Cart
			r.Route("/cart", func(r chi.Router) {
				r.Get("/", cartHandler.GetCart)
				r.Put("/item", cartHandler.SetItem)
				r.Put("/address", cartHandler.SetAddress)
				r.Put("/card", cartHandler.SetCard)
				r.Delete("/", cartHandler.ClearCart)
				// Tighter per-user limit on the checkout endpoint.
				r.With(middleware.RateLimitByUser(10, time.Minute)).
					Post("/convert", cartHandler.ConvertCart)
			})

			// Cards — per-user limit on card creation to slow fraud probing.
			r.Route("/cards", func(r chi.Router) {
				r.Get("/", cardHandler.GetCards)
				r.Get("/{id}", cardHandler.GetCard)
				r.With(middleware.RateLimitByUser(15, time.Minute)).
					Post("/", cardHandler.SaveCard)
				r.With(middleware.RateLimitByUser(10, time.Minute)).
					Post("/collect", cardHandler.CollectCard)
				r.Delete("/{id}", cardHandler.DeleteCard)
				r.Put("/{id}/default", cardHandler.SetDefaultCard)
			})

			// Orders
			r.Get("/orders", orderHandler.GetOrders)
			r.Post("/orders/{id}/refund", orderHandler.RefundOrder)

			// Addresses
			r.Route("/addresses", func(r chi.Router) {
				r.Get("/", addressHandler.GetAddresses)
				r.Post("/", addressHandler.CreateAddress)
				r.Delete("/{id}", addressHandler.DeleteAddress)
				r.Put("/{id}/default", addressHandler.SetDefaultAddress)
			})
		})

		// Admin endpoints, gated by ADMIN_API_KEY only, no JWT required
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAdmin)
			r.Patch("/orders/{id}/tracking", orderHandler.UpdateTracking)
			r.Post("/orders/{id}/label", orderHandler.PurchaseLabel)
		})
	})

	return r
}
