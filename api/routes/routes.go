package routes

import (
	"github.com/go-chi/chi/v5"

	"terminalShop/api/handlers"
	"terminalShop/api/middleware"
	"terminalShop/pkg/auth"
)

// SetupRoutes configures all API routes
func SetupRoutes(version string, stripeSecretKey string, jwtManager *auth.JWTManager, authFingerprintKey string) *chi.Mux {
	r := chi.NewRouter()

	// Apply global middleware
	r.Use(middleware.Logger)
	r.Use(middleware.CORS())
	r.Use(middleware.Recovery)
	r.Use(middleware.Auth(jwtManager))

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(version)
	productHandler := handlers.NewProductHandler()
	authHandler := handlers.NewAuthHandler(jwtManager, authFingerprintKey)
	checkoutHandler := handlers.NewCheckoutHandler(stripeSecretKey)
	orderHandler := handlers.NewOrderHandler()
	addressHandler := handlers.NewAddressHandler()

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Health & testing endpoints
		r.Get("/health", healthHandler.GetHealth)
		r.Get("/ping", healthHandler.Ping)

		// Authentication endpoints (SSH key-based)
		r.Post("/auth/register", authHandler.RegisterWithSSHKey) // POST /api/v1/auth/register
		r.Get("/auth/user", authHandler.GetUserBySSHKey)         // GET /api/v1/auth/user?fingerprint=xxx
		r.Post("/auth/token", authHandler.GetToken)

		// Product endpoints (full CRUD)
		r.Get("/products", productHandler.GetProducts)     // Public
		r.Get("/products/{id}", productHandler.GetProduct) // Public

		// Protected routes - requires a valid JWT
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth)

			// Admin-only product management (TODO: add admin check)
			r.Post("/products", productHandler.CreateProduct)
			r.Put("/products/{id}", productHandler.UpdateProduct)
			r.Delete("/products/{id}", productHandler.DeleteProduct)

			// Checkout
			r.Post("/checkout", checkoutHandler.Checkout)

			// Orders
			r.Get("/orders", orderHandler.GetOrders)

			// Addresses
			r.Route("/addresses", func(r chi.Router) {
				r.Get("/", addressHandler.GetAddresses)
				r.Post("/", addressHandler.CreateAddress)
				r.Delete("/{id}", addressHandler.DeleteAddress)
				r.Put("/{id}/default", addressHandler.SetDefaultAddress)
			})
		})
	})

	return r
}
