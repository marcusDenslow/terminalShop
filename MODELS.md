# Technical Specifications & Architecture

This document defines the technical stack, architecture patterns, code standards, and implementation details for the Terminal Coffee Shop project. **Read this before writing any code.**

---

## Table of Contents

1. [Development Workflow](#development-workflow)
2. [Technology Stack](#technology-stack)
3. [Database Schema](#database-schema)
4. [API Specifications](#api-specifications)
5. [Code Organization](#code-organization)
6. [Code Style Guide](#code-style-guide)
7. [Error Handling](#error-handling)
8. [Testing Standards](#testing-standards)
9. [Common Patterns](#common-patterns)

---

## Development Workflow

### Documentation-First Approach

**IMPORTANT**: Before implementing features using any library or technology, **always fetch and read the official documentation first**.

#### Process for Using New Libraries

1. **Identify the library/technology** you need to use (e.g., Chi router, GORM, Stripe SDK)
2. **Fetch the documentation** using WebFetch:
   ```
   WebFetch the official documentation URL for the library
   Read through relevant sections (getting started, API reference, examples)
   ```
3. **Understand the current best practices** from the docs (not from memory/assumptions)
4. **Implement** following the documented patterns
5. **Reference the docs** if you encounter issues

#### Why This Matters

- Libraries evolve - documentation ensures you use current patterns
- Prevents deprecated patterns or outdated approaches
- Helps discover features you might not know about
- Ensures compatibility with the version in go.mod

#### Example Workflow

```
Task: Implement Chi router middleware

Step 1: Check go.mod for version
Step 2: WebFetch https://go-chi.io/#/pages/middleware
Step 3: Read middleware documentation
Step 4: Implement following documented patterns
Step 5: Test implementation
```

#### Documentation Sources

- **Chi Router**: https://go-chi.io/
- **GORM**: https://gorm.io/docs/
- **Stripe Go**: https://stripe.com/docs/api?lang=go
- **Bubbletea**: https://github.com/charmbracelet/bubbletea
- **Lipgloss**: https://github.com/charmbracelet/lipgloss

---

## Technology Stack

### TUI (Terminal User Interface)

| Package | Version | Purpose | Documentation |
|---------|---------|---------|---------------|
| `github.com/charmbracelet/bubbletea` | v1.3.10+ | Main TUI framework (The Elm Architecture) | https://github.com/charmbracelet/bubbletea |
| `github.com/charmbracelet/lipgloss` | v1.1.0+ | Styling and layout | https://github.com/charmbracelet/lipgloss |
| `github.com/charmbracelet/bubbles` | v0.21.0+ | Pre-built UI components (viewport, spinner, etc.) | https://github.com/charmbracelet/bubbles |
| `github.com/charmbracelet/huh` | v0.8.0+ | Forms and input validation | https://github.com/charmbracelet/huh |
| `github.com/charmbracelet/wish` | v1.4.7+ | SSH server framework | https://github.com/charmbracelet/wish |

### Backend API

| Package | Version | Purpose | Documentation |
|---------|---------|---------|---------------|
| `github.com/go-chi/chi/v5` | v5.0.0+ | HTTP router | https://go-chi.io/ |
| `gorm.io/gorm` | v1.25.0+ | ORM for database operations | https://gorm.io/docs/ |
| `gorm.io/driver/postgres` | v1.5.0+ | PostgreSQL driver | https://gorm.io/docs/connecting_to_the_database.html |
| `github.com/golang-jwt/jwt/v5` | v5.0.0+ | JWT token handling | https://github.com/golang-jwt/jwt |
| `github.com/go-playground/validator/v10` | v10.0.0+ | Struct validation | https://github.com/go-playground/validator |
| `github.com/joho/godotenv` | v1.5.0+ | Environment variables | https://github.com/joho/godotenv |

### Payments & Services

| Package | Version | Purpose | Documentation |
|---------|---------|---------|---------------|
| `github.com/stripe/stripe-go/v78` | v78.12.0+ | Payment processing | https://stripe.com/docs/api?lang=go |
| `rsc.io/qr` | v0.2.0 | QR code generation | https://pkg.go.dev/rsc.io/qr |

### Utilities

| Package | Version | Purpose | Documentation |
|---------|---------|---------|---------------|
| `github.com/google/uuid` | v1.6.0+ | UUID generation | https://github.com/google/uuid |
| `golang.org/x/crypto` | latest | Cryptography utilities | https://pkg.go.dev/golang.org/x/crypto |

---

## Database Schema

### Schema Design Principles
- Use UUIDs for primary keys (not auto-incrementing integers)
- Always include `created_at` and `updated_at` timestamps
- Use soft deletes (`deleted_at`) for important records
- Normalize data appropriately (3NF where possible)
- Add indexes on foreign keys and frequently queried columns

### Complete Schema (PostgreSQL)

```sql
-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fingerprint VARCHAR(255) UNIQUE,  -- SSH public key fingerprint
    email VARCHAR(255) UNIQUE,
    name VARCHAR(255),
    stripe_customer_id VARCHAR(255),  -- Stripe customer ID
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

-- Products table
CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    roast_type VARCHAR(100),  -- "Dark Roast", "Medium Roast", etc.
    ounces INTEGER,           -- Size in ounces
    bean_type VARCHAR(100),   -- "Arabica", "Robusta", etc.
    price INTEGER NOT NULL,   -- Price in cents (e.g., 350 = $3.50)
    color VARCHAR(7),         -- Hex color for UI (e.g., "#8B4513")
    image_url VARCHAR(500),
    stock INTEGER DEFAULT 0,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

-- Carts table
CREATE TABLE carts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Cart items table
CREATE TABLE cart_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cart_id UUID REFERENCES carts(id) ON DELETE CASCADE,
    product_id UUID REFERENCES products(id),
    quantity INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(cart_id, product_id)  -- One entry per product per cart
);

-- Addresses table
CREATE TABLE addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    name VARCHAR(255),        -- Recipient name
    street VARCHAR(255) NOT NULL,
    street2 VARCHAR(255),     -- Apt, suite, etc.
    city VARCHAR(100) NOT NULL,
    state VARCHAR(2) NOT NULL,  -- US state code
    zip VARCHAR(10) NOT NULL,
    country VARCHAR(2) DEFAULT 'US',
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Payment methods table (stores Stripe tokens, not raw card data)
CREATE TABLE payment_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    stripe_payment_method_id VARCHAR(255) NOT NULL,  -- Stripe PM ID
    brand VARCHAR(50),        -- "Visa", "Mastercard", etc.
    last4 VARCHAR(4),         -- Last 4 digits
    exp_month INTEGER,
    exp_year INTEGER,
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Orders table
CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    address_id UUID REFERENCES addresses(id),
    payment_method_id UUID REFERENCES payment_methods(id),

    -- Amounts in cents
    subtotal INTEGER NOT NULL,
    shipping INTEGER NOT NULL DEFAULT 0,
    tax INTEGER NOT NULL DEFAULT 0,
    total INTEGER NOT NULL,

    status VARCHAR(50) DEFAULT 'pending',  -- pending, paid, shipped, delivered, cancelled

    stripe_payment_intent_id VARCHAR(255),  -- Stripe Payment Intent ID
    tracking_number VARCHAR(255),

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Order items table (snapshot of products at time of order)
CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID REFERENCES orders(id) ON DELETE CASCADE,
    product_id UUID REFERENCES products(id),

    -- Snapshot product details (in case product changes later)
    product_name VARCHAR(255),
    product_description TEXT,
    price INTEGER NOT NULL,  -- Price at time of order
    quantity INTEGER NOT NULL,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_fingerprint ON users(fingerprint);
CREATE INDEX idx_carts_user_id ON carts(user_id);
CREATE INDEX idx_cart_items_cart_id ON cart_items(cart_id);
CREATE INDEX idx_addresses_user_id ON addresses(user_id);
CREATE INDEX idx_payment_methods_user_id ON payment_methods(user_id);
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_order_items_order_id ON order_items(order_id);
```

### GORM Model Examples

```go
// models/user.go
package models

import (
    "time"
    "gorm.io/gorm"
    "github.com/google/uuid"
)

type User struct {
    ID               uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    Fingerprint      string         `gorm:"uniqueIndex;size:255"`
    Email            string         `gorm:"uniqueIndex;size:255"`
    Name             string         `gorm:"size:255"`
    StripeCustomerID string         `gorm:"size:255"`
    CreatedAt        time.Time
    UpdatedAt        time.Time
    DeletedAt        gorm.DeletedAt `gorm:"index"`
}

// models/product.go
type Product struct {
    ID          uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    Name        string         `gorm:"size:255;not null" json:"name"`
    Description string         `gorm:"type:text" json:"description"`
    RoastType   string         `gorm:"size:100" json:"roast_type"`
    Ounces      int            `json:"ounces"`
    BeanType    string         `gorm:"size:100" json:"bean_type"`
    Price       int            `gorm:"not null" json:"price"` // cents
    Color       string         `gorm:"size:7" json:"color"`
    ImageURL    string         `gorm:"size:500" json:"image_url"`
    Stock       int            `gorm:"default:0" json:"stock"`
    Active      bool           `gorm:"default:true" json:"active"`
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// models/cart.go
type Cart struct {
    ID        uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    UserID    uuid.UUID  `gorm:"type:uuid;not null"`
    User      User       `gorm:"foreignKey:UserID"`
    Items     []CartItem `gorm:"foreignKey:CartID"`
    CreatedAt time.Time
    UpdatedAt time.Time
}

type CartItem struct {
    ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
    CartID    uuid.UUID `gorm:"type:uuid;not null"`
    ProductID uuid.UUID `gorm:"type:uuid;not null"`
    Product   Product   `gorm:"foreignKey:ProductID"`
    Quantity  int       `gorm:"not null;default:1"`
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

---

## API Specifications

### API Design Principles

1. **RESTful routes** - Use standard HTTP methods (GET, POST, PUT, DELETE)
2. **JSON responses** - All responses in JSON format
3. **Consistent error format** - See Error Handling section
4. **JWT authentication** - Bearer token in Authorization header
5. **Versioning** - All routes prefixed with `/api/v1/`
6. **Pagination** - Use `page` and `limit` query params

### API Routes

#### Authentication
```
POST   /api/v1/auth/register      - Create new user
POST   /api/v1/auth/login         - Get JWT token
POST   /api/v1/auth/refresh       - Refresh token
GET    /api/v1/auth/me            - Get current user (requires auth)
PUT    /api/v1/auth/me            - Update current user (requires auth)
```

#### Products
```
GET    /api/v1/products           - List all products (paginated)
GET    /api/v1/products/:id       - Get single product
POST   /api/v1/products           - Create product (admin only)
PUT    /api/v1/products/:id       - Update product (admin only)
DELETE /api/v1/products/:id       - Delete product (admin only)
```

#### Cart
```
GET    /api/v1/cart               - Get current user's cart
POST   /api/v1/cart/items         - Add item to cart
PUT    /api/v1/cart/items/:id     - Update item quantity
DELETE /api/v1/cart/items/:id     - Remove item from cart
DELETE /api/v1/cart               - Clear entire cart
```

#### Payment Methods
```
GET    /api/v1/payment-methods    - List saved payment methods
POST   /api/v1/payment-methods    - Save new payment method
DELETE /api/v1/payment-methods/:id - Remove payment method
```

#### Addresses
```
GET    /api/v1/addresses          - List user's addresses
POST   /api/v1/addresses          - Create new address
PUT    /api/v1/addresses/:id      - Update address
DELETE /api/v1/addresses/:id      - Delete address
```

#### Orders
```
GET    /api/v1/orders             - List user's orders
GET    /api/v1/orders/:id         - Get order details
POST   /api/v1/orders             - Create new order (checkout)
```

### Request/Response Examples

#### GET /api/v1/products
**Response (200 OK)**:
```json
{
  "success": true,
  "data": {
    "products": [
      {
        "id": "123e4567-e89b-12d3-a456-426614174000",
        "name": "Espresso",
        "description": "Bold coffee...",
        "roast_type": "Dark Roast",
        "ounces": 2,
        "bean_type": "Arabica",
        "price": 350,
        "color": "#8B4513",
        "stock": 100,
        "created_at": "2024-01-01T00:00:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "limit": 20,
      "total": 6,
      "total_pages": 1
    }
  }
}
```

#### POST /api/v1/cart/items
**Request**:
```json
{
  "product_id": "123e4567-e89b-12d3-a456-426614174000",
  "quantity": 2
}
```

**Response (201 Created)**:
```json
{
  "success": true,
  "data": {
    "cart": {
      "id": "cart-uuid",
      "items": [
        {
          "id": "item-uuid",
          "product": { /* full product object */ },
          "quantity": 2
        }
      ],
      "subtotal": 700
    }
  }
}
```

#### POST /api/v1/orders (Checkout)
**Request**:
```json
{
  "address_id": "address-uuid",
  "payment_method_id": "pm-uuid"
}
```

**Response (201 Created)**:
```json
{
  "success": true,
  "data": {
    "order": {
      "id": "order-uuid",
      "subtotal": 700,
      "shipping": 500,
      "tax": 0,
      "total": 1200,
      "status": "pending",
      "items": [ /* order items */ ]
    }
  }
}
```

---

## Code Organization

### Directory Structure

```
terminalShop/
├── main.go                 # Entry point (SSH server)
├── server.go               # Current TUI entry point
├── model.go                # Bubbletea TUI models and views
│
├── api/                    # Backend API server
│   ├── main.go            # API server entry point
│   ├── routes/
│   │   ├── auth.go        # Auth routes
│   │   ├── products.go    # Product routes
│   │   ├── cart.go        # Cart routes
│   │   └── orders.go      # Order routes
│   ├── middleware/
│   │   ├── auth.go        # JWT auth middleware
│   │   ├── logger.go      # Request logging
│   │   └── cors.go        # CORS middleware
│   └── handlers/
│       ├── auth.go        # Auth handler functions
│       ├── products.go    # Product handlers
│       └── cart.go        # Cart handlers
│
├── pkg/                    # Shared packages
│   ├── models/            # Database models (GORM)
│   │   ├── user.go
│   │   ├── product.go
│   │   ├── cart.go
│   │   └── order.go
│   ├── database/          # Database connection
│   │   └── db.go
│   ├── apiclient/         # API client for TUI
│   │   └── client.go
│   ├── auth/              # Auth utilities
│   │   ├── jwt.go         # JWT generation/validation
│   │   └── password.go    # Password hashing
│   └── utils/             # Helper functions
│       ├── response.go    # Standard response helpers
│       └── validation.go  # Validation helpers
│
├── migrations/            # Database migrations
│   ├── 001_initial.sql
│   └── migrate.go
│
├── config/               # Configuration
│   └── config.go        # Load env vars
│
├── tests/               # Tests
│   ├── api/
│   └── integration/
│
├── .env                 # Environment variables (not in git)
├── .env.example         # Example env file
├── go.mod
├── go.sum
├── README.md
└── MODELS.md           # This file
```

### Package Organization Rules

1. **One responsibility per package** - Each package should have a clear, single purpose
2. **No circular dependencies** - Organize imports in a clear hierarchy
3. **Shared code in pkg/** - Anything used by multiple packages goes in `pkg/`
4. **Keep handlers thin** - Business logic goes in separate service functions
5. **Models package is database-only** - No business logic in model structs

---

## Code Style Guide

### General Go Style

Follow the official [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) and [Effective Go](https://go.dev/doc/effective_go).

### Specific Rules for This Project

#### 1. Imports Organization
Always organize imports in three groups, separated by blank lines:

```go
import (
    // Standard library
    "context"
    "fmt"
    "net/http"

    // External dependencies
    "github.com/go-chi/chi/v5"
    "gorm.io/gorm"

    // Internal packages
    "github.com/yourusername/terminalshop/pkg/models"
    "github.com/yourusername/terminalshop/pkg/utils"
)
```

#### 2. Naming Conventions

- **Variables**: `camelCase` for local variables, `PascalCase` for exported
- **Constants**: `PascalCase` or `SCREAMING_SNAKE_CASE` for groups
- **Functions**: `PascalCase` for exported, `camelCase` for private
- **Interfaces**: Suffix with `-er` if single method (e.g., `Reader`, `Writer`)
- **Structs**: `PascalCase`, descriptive nouns
- **Files**: `snake_case.go`

```go
// Good
type UserService struct {
    db *gorm.DB
}

func (s *UserService) CreateUser(ctx context.Context, name string) (*models.User, error) {
    // ...
}

// Bad
type userservice struct { } // should be PascalCase if exported
func (s *UserService) create_user() { } // should be camelCase if private
```

#### 3. Error Messages

- Start with lowercase
- No punctuation at the end
- Include context about what failed

```go
// Good
return nil, fmt.Errorf("failed to create user: %w", err)
return nil, errors.New("user not found")

// Bad
return nil, fmt.Errorf("Error creating user: %v", err)  // no caps, use %w
return nil, errors.New("User not found.")  // no caps, no period
```

#### 4. Function Organization

Order functions in files logically:
1. Exported types/constants
2. Constructor functions (`New...`)
3. Public methods (alphabetically)
4. Private methods (alphabetically)
5. Helper functions

```go
// 1. Types
type UserHandler struct {
    db *gorm.DB
}

// 2. Constructor
func NewUserHandler(db *gorm.DB) *UserHandler {
    return &UserHandler{db: db}
}

// 3. Public methods
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    // ...
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
    // ...
}

// 4. Private methods
func (h *UserHandler) validateUser(user *models.User) error {
    // ...
}

// 5. Helper functions
func hashPassword(password string) (string, error) {
    // ...
}
```

#### 5. Comment Style

- Use complete sentences for package and function comments
- Start comments with the name of the thing being described
- Use `//` for single-line, `/* */` for multi-line only when needed

```go
// UserHandler handles HTTP requests related to users.
type UserHandler struct {
    db *gorm.DB
}

// CreateUser creates a new user in the database and returns it.
// Returns an error if validation fails or database operation fails.
func (h *UserHandler) CreateUser(ctx context.Context, user *models.User) error {
    // Validate user input
    if err := h.validateUser(user); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    // Create user in database
    if err := h.db.Create(user).Error; err != nil {
        return fmt.Errorf("failed to create user: %w", err)
    }

    return nil
}
```

#### 6. Struct Tags

Always include JSON tags and use snake_case for JSON field names:

```go
type Product struct {
    ID          uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
    Name        string    `gorm:"size:255" json:"name"`
    Price       int       `gorm:"not null" json:"price"`
    RoastType   string    `gorm:"size:100" json:"roast_type"`  // snake_case!
    CreatedAt   time.Time `json:"created_at"`
}
```

#### 7. Context Usage

Always pass `context.Context` as the first parameter:

```go
// Good
func (s *UserService) GetUser(ctx context.Context, id uuid.UUID) (*models.User, error) {
    return s.db.WithContext(ctx).First(&user, id).Error
}

// Bad
func (s *UserService) GetUser(id uuid.UUID) (*models.User, error) {
    // No context!
}
```

---

## Error Handling

### Principles

1. **Always handle errors** - Never ignore errors with `_`
2. **Wrap errors** - Use `fmt.Errorf("context: %w", err)` to add context
3. **Don't panic** - Only panic in `main()` or `init()` for unrecoverable errors
4. **Return early** - Use guard clauses and early returns

### Standard Error Response Format (API)

All API errors should follow this format:

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid input data",
    "details": {
      "email": "must be a valid email address",
      "password": "must be at least 8 characters"
    }
  }
}
```

### Error Response Helper

```go
// pkg/utils/response.go
package utils

import (
    "encoding/json"
    "net/http"
)

type ErrorResponse struct {
    Success bool   `json:"success"`
    Error   Error  `json:"error"`
}

type Error struct {
    Code    string                 `json:"code"`
    Message string                 `json:"message"`
    Details map[string]interface{} `json:"details,omitempty"`
}

func RespondError(w http.ResponseWriter, code int, errCode, message string, details map[string]interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(ErrorResponse{
        Success: false,
        Error: Error{
            Code:    errCode,
            Message: message,
            Details: details,
        },
    })
}

func RespondSuccess(w http.ResponseWriter, code int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "data":    data,
    })
}
```

### Common Error Codes

```go
const (
    ErrCodeValidation      = "VALIDATION_ERROR"
    ErrCodeUnauthorized    = "UNAUTHORIZED"
    ErrCodeForbidden       = "FORBIDDEN"
    ErrCodeNotFound        = "NOT_FOUND"
    ErrCodeConflict        = "CONFLICT"
    ErrCodeInternal        = "INTERNAL_ERROR"
    ErrCodeBadRequest      = "BAD_REQUEST"
    ErrCodePaymentFailed   = "PAYMENT_FAILED"
)
```

### Error Handling Pattern (API Handlers)

```go
func (h *ProductHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
    // Extract ID from URL
    productID := chi.URLParam(r, "id")
    if productID == "" {
        utils.RespondError(w, http.StatusBadRequest, ErrCodeBadRequest, "product ID required", nil)
        return
    }

    // Parse UUID
    id, err := uuid.Parse(productID)
    if err != nil {
        utils.RespondError(w, http.StatusBadRequest, ErrCodeValidation, "invalid product ID", nil)
        return
    }

    // Get product from database
    product, err := h.productService.GetProduct(r.Context(), id)
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            utils.RespondError(w, http.StatusNotFound, ErrCodeNotFound, "product not found", nil)
            return
        }
        // Log the actual error internally
        log.Printf("failed to get product: %v", err)
        utils.RespondError(w, http.StatusInternalServerError, ErrCodeInternal, "internal server error", nil)
        return
    }

    // Success response
    utils.RespondSuccess(w, http.StatusOK, product)
}
```

### Error Handling in TUI

For the TUI, errors should be displayed to the user in a friendly way:

```go
// In model.go Update() function
switch msg := msg.(type) {
case error:
    m.errorMessage = api.GetErrorMessage(msg)
    m.showError = true
    return m, nil
}

// In View() function
func (m model) View() string {
    if m.showError {
        errorBox := lipgloss.NewStyle().
            Foreground(lipgloss.Color("#FF0000")).
            Bold(true).
            Render("Error: " + m.errorMessage)
        return errorBox + "\n" + m.normalView()
    }
    return m.normalView()
}
```

---

## Testing Standards

### Testing Strategy

1. **Unit tests** for business logic and utilities
2. **Integration tests** for API endpoints with database
3. **No tests for TUI initially** (focus on API)

### Test File Organization

- Test files live next to the code they test
- Name: `<file>_test.go`
- Use table-driven tests for multiple cases

### Example Test Structure

```go
// api/handlers/product_test.go
package handlers

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestProductHandler_GetProduct(t *testing.T) {
    tests := []struct {
        name           string
        productID      string
        setupMock      func(*mockDB)
        expectedStatus int
        expectedError  string
    }{
        {
            name:           "success",
            productID:      "valid-uuid",
            setupMock:      func(m *mockDB) { /* setup */ },
            expectedStatus: 200,
        },
        {
            name:           "not found",
            productID:      "nonexistent-uuid",
            setupMock:      func(m *mockDB) { /* setup */ },
            expectedStatus: 404,
            expectedError:  "NOT_FOUND",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
            // Use require for setup assertions
            // Use assert for test assertions
        })
    }
}
```

### Testing Libraries

- `testing` - Standard library
- `github.com/stretchr/testify` - Assertions and mocking
- Test database: Use in-memory SQLite or Docker PostgreSQL

---

## Common Patterns

### Pattern 1: Service Layer

Separate business logic from HTTP handlers:

```go
// pkg/services/user_service.go
type UserService struct {
    db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
    return &UserService{db: db}
}

func (s *UserService) CreateUser(ctx context.Context, name, email string) (*models.User, error) {
    user := &models.User{
        Name:  name,
        Email: email,
    }

    if err := s.db.WithContext(ctx).Create(user).Error; err != nil {
        return nil, fmt.Errorf("failed to create user: %w", err)
    }

    return user, nil
}

// api/handlers/user_handler.go
type UserHandler struct {
    userService *services.UserService
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    // Parse request
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.RespondError(w, 400, ErrCodeBadRequest, "invalid request body", nil)
        return
    }

    // Call service
    user, err := h.userService.CreateUser(r.Context(), req.Name, req.Email)
    if err != nil {
        // Handle error
        return
    }

    utils.RespondSuccess(w, 201, user)
}
```

### Pattern 2: Middleware for Auth

```go
// api/middleware/auth.go
func AuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Extract token from Authorization header
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                utils.RespondError(w, 401, ErrCodeUnauthorized, "missing auth token", nil)
                return
            }

            // Validate token
            tokenString := strings.TrimPrefix(authHeader, "Bearer ")
            token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
                return []byte(jwtSecret), nil
            })

            if err != nil || !token.Valid {
                utils.RespondError(w, 401, ErrCodeUnauthorized, "invalid token", nil)
                return
            }

            // Extract user ID from claims
            claims := token.Claims.(jwt.MapClaims)
            userID := claims["user_id"].(string)

            // Add to context
            ctx := context.WithValue(r.Context(), "user_id", userID)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// Usage in routes
r.Group(func(r chi.Router) {
    r.Use(middleware.AuthMiddleware(jwtSecret))
    r.Get("/api/v1/cart", handlers.GetCart)
    r.Post("/api/v1/cart/items", handlers.AddCartItem)
})
```

### Pattern 3: Repository Pattern (Optional)

For more complex queries, separate database access:

```go
// pkg/repositories/product_repository.go
type ProductRepository struct {
    db *gorm.DB
}

func (r *ProductRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Product, error) {
    var product models.Product
    err := r.db.WithContext(ctx).Where("id = ? AND active = ?", id, true).First(&product).Error
    return &product, err
}

func (r *ProductRepository) List(ctx context.Context, page, limit int) ([]models.Product, int64, error) {
    var products []models.Product
    var total int64

    offset := (page - 1) * limit

    r.db.Model(&models.Product{}).Where("active = ?", true).Count(&total)
    err := r.db.WithContext(ctx).
        Where("active = ?", true).
        Limit(limit).
        Offset(offset).
        Find(&products).Error

    return products, total, err
}
```

### Pattern 4: Bubbletea View Switching

```go
// In model.go
type page int

const (
    shopPage page = iota
    cartPage
    accountPage
    paymentPage
)

type model struct {
    currentPage page
    // ... other fields
}

func (m model) SwitchToPage(p page) (model, tea.Cmd) {
    m.currentPage = p
    // Reset page-specific state
    switch p {
    case shopPage:
        m.cursor = 0
    case cartPage:
        m.cartCursor = 0
    // ...
    }
    return m, nil
}

func (m model) View() string {
    header := m.buildHeader()
    footer := m.buildFooter()

    var content string
    switch m.currentPage {
    case shopPage:
        content = m.buildShopView()
    case cartPage:
        content = m.buildCartView()
    case accountPage:
        content = m.buildAccountView()
    default:
        content = "Unknown page"
    }

    return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}
```

---

## Configuration Management

### Environment Variables

Use a `.env` file for local development:

```bash
# .env (DO NOT COMMIT)
DATABASE_URL=postgresql://user:password@localhost:5432/terminalshop
JWT_SECRET=your-super-secret-jwt-key-change-this
STRIPE_SECRET_KEY=sk_test_...
STRIPE_PUBLIC_KEY=pk_test_...
API_PORT=8080
SSH_PORT=2222
ENVIRONMENT=development
```

### Config Package

```go
// config/config.go
package config

import (
    "os"
    "github.com/joho/godotenv"
)

type Config struct {
    DatabaseURL      string
    JWTSecret        string
    StripeSecretKey  string
    StripePublicKey  string
    APIPort          string
    SSHPort          string
    Environment      string
}

func Load() (*Config, error) {
    // Load .env file in development
    if os.Getenv("ENVIRONMENT") != "production" {
        godotenv.Load()
    }

    return &Config{
        DatabaseURL:      os.Getenv("DATABASE_URL"),
        JWTSecret:        os.Getenv("JWT_SECRET"),
        StripeSecretKey:  os.Getenv("STRIPE_SECRET_KEY"),
        StripePublicKey:  os.Getenv("STRIPE_PUBLIC_KEY"),
        APIPort:          getEnvOrDefault("API_PORT", "8080"),
        SSHPort:          getEnvOrDefault("SSH_PORT", "2222"),
        Environment:      getEnvOrDefault("ENVIRONMENT", "development"),
    }, nil
}

func getEnvOrDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
```

---

## Security Best Practices

1. **Never commit secrets** - Use `.env` and add to `.gitignore`
2. **Hash passwords** - Use `bcrypt` with cost >= 12
3. **Validate all inputs** - Never trust user input
4. **Use parameterized queries** - GORM does this automatically
5. **HTTPS in production** - Always use TLS
6. **Rate limiting** - Add to API endpoints
7. **CORS configuration** - Only allow trusted origins

---

## Reference Implementation

The `terminal-shop-source/` directory contains the complete terminal.shop codebase. When in doubt:

1. **Check similar functionality** in their code
2. **Copy patterns, not code verbatim** - adapt to our stack
3. **Understand before implementing** - don't blindly copy

Key files to reference:
- `packages/go/pkg/tui/*.go` - TUI views and components
- `packages/go/pkg/api/api.go` - API client integration
- `packages/core/src/` - Backend business logic (TypeScript, but concepts apply)

---

## Quick Reference Checklist

Before writing new code, ask:
- [ ] Does this follow the file organization structure?
- [ ] Are imports organized correctly?
- [ ] Are errors wrapped with context?
- [ ] Are all errors handled?
- [ ] Does this need a test?
- [ ] Are naming conventions followed?
- [ ] Is the API response in the standard format?
- [ ] Are database queries using context?
- [ ] Is sensitive data in environment variables?
- [ ] Does this match patterns from terminal.shop reference code?

---

**Remember**: Consistency is more important than perfection. Follow these patterns throughout the codebase, and reference this document frequently!
