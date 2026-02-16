package routes

import (
	"github.com/go-chi/chi/v5"

	"terminalShop/api/handlers"
	"terminalShop/api/middleware"
)

// SetupRoutes configures all API routes
func SetupRoutes(version string) *chi.Mux {
	r := chi.NewRouter()

	// Apply global middleware
	r.Use(middleware.Logger)
	r.Use(middleware.CORS())
	r.Use(middleware.Recovery)

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(version)
	productHandler := handlers.NewProductHandler()
	authHandler := handlers.NewAuthHandler()

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Health & testing endpoints
		r.Get("/health", healthHandler.GetHealth)
		r.Get("/ping", healthHandler.Ping)

		// Authentication endpoints (SSH key-based)
		r.Post("/auth/register", authHandler.RegisterWithSSHKey) // POST /api/v1/auth/register
		r.Get("/auth/user", authHandler.GetUserBySSHKey)         // GET /api/v1/auth/user?fingerprint=xxx

		// Product endpoints (full CRUD)
		r.Route("/products", func(r chi.Router) {
			r.Get("/", productHandler.GetProducts)       // GET /api/v1/products
			r.Post("/", productHandler.CreateProduct)    // POST /api/v1/products
			r.Get("/{id}", productHandler.GetProduct)    // GET /api/v1/products/:id
			r.Put("/{id}", productHandler.UpdateProduct) // PUT /api/v1/products/:id
			r.Delete("/{id}", productHandler.DeleteProduct) // DELETE /api/v1/products/:id
		})
	})

	return r
}
