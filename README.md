# Terminal Coffee Shop

A terminal-based e-commerce application for ordering coffee, built with Go and Bubbletea TUI framework. Inspired by [terminal.shop](https://terminal.shop).

## Current Status

**Phase 1: Enhanced UI & Navigation** - 80% Complete

### What's Working
- ✅ Multi-view navigation (shop tab, cart tab)
- ✅ Product listing with left-aligned coffee names
- ✅ Product detail view with descriptions
- ✅ Cart management (add/remove items, quantity controls)
- ✅ Responsive vertical centering with padding
- ✅ Clean, minimal UI with proper spacing

### What's Next
- ⬜ Account page view (add `viewingAccount bool` to model)
- ⬜ Breadcrumb navigation system
- ⬜ Viewport scrolling for long content
- ⬜ Loading states and error displays

## Project Structure

```
terminalShop/
├── main.go              # Entry point, SSH server (future)
├── server.go            # Current main entry point for TUI
├── model.go             # Bubbletea model, views, and state management
├── README.md            # This file - project overview and roadmap
├── MODELS.md            # Technical specs, architecture, code standards
├── api/                 # (Future) REST API backend
├── db/                  # (Future) Database migrations and schemas
├── config/              # (Future) Configuration files
└── terminal-shop-source/ # Reference implementation (terminal.shop clone)
```

## Development Roadmap

This roadmap is designed to be executed incrementally. Each phase builds on the previous one.

---

## **PHASE 1: Enhanced UI & Navigation** (Current Phase)

### Objective
Build a polished, navigable TUI with shop, cart, and account views.

### Tasks

#### 1.1 Add Account Page View
- [ ] Add `viewingAccount bool` to model struct in `model.go`
- [ ] Add keybind "a" to switch to account view in `Update()` function
- [ ] Create `buildAccountView()` function
- [ ] Show user info: username, email (mock data for now)
- [ ] Add account tab to header with proper active/inactive styling
- [ ] Update footer to show account navigation keybind

**Files to modify**: `model.go`

#### 1.2 Implement Breadcrumb Navigation
- [ ] Create breadcrumb component showing current page path
- [ ] Display breadcrumbs between header and content
- [ ] Style breadcrumbs to match terminal.shop aesthetic

**Reference**: `terminal-shop-source/packages/go/pkg/tui/breadcrumbs.go`

#### 1.3 Add Viewport Scrolling
- [ ] Import `github.com/charmbracelet/bubbles/viewport`
- [ ] Add viewport to cart view for long item lists
- [ ] Add viewport to shop view for product descriptions
- [ ] Handle scroll keybinds (pgup/pgdn, mouse wheel)

**Reference**: `terminal-shop-source/packages/go/pkg/tui/cart.go` (lines 15-20)

#### 1.4 Loading and Error States
- [ ] Create loading spinner component
- [ ] Add error message display system
- [ ] Show loading state when switching views
- [ ] Handle and display errors gracefully

---

## **PHASE 2: Backend API Foundation**

### Objective
Build a REST API backend in Go that the TUI can communicate with.

### Tech Stack
- **Framework**: Chi router (`github.com/go-chi/chi/v5`)
- **Database**: PostgreSQL with GORM (`gorm.io/gorm`)
- **Auth**: JWT tokens (`github.com/golang-jwt/jwt/v5`)
- **Validation**: go-playground/validator (`github.com/go-playground/validator/v10`)

### Tasks

#### 2.1 Project Setup
- [ ] Create `api/` directory structure
- [ ] Initialize Go modules for API
- [ ] Set up Chi router
- [ ] Add middleware: logging, CORS, recovery
- [ ] Create `.env` file for configuration
- [ ] Add `godotenv` for environment variables

**Directory structure**:
```
api/
├── main.go              # API server entry point
├── routes/              # Route handlers
│   ├── products.go
│   ├── users.go
│   └── cart.go
├── middleware/          # Custom middleware
├── models/             # Database models
└── db/                 # Database connection
```

#### 2.2 Database Setup
- [ ] Install PostgreSQL locally (or use Docker)
- [ ] Create database schema
- [ ] Set up GORM connection
- [ ] Create migration system
- [ ] Add seed data for coffee products

**Schema to create**:
```sql
-- See MODELS.md for complete schema
users, products, carts, cart_items, orders, order_items, addresses
```

#### 2.3 Product API Endpoints
- [ ] `GET /api/products` - List all products
- [ ] `GET /api/products/:id` - Get single product
- [ ] `POST /api/products` - Create product (admin only, future)
- [ ] Add pagination to product listing
- [ ] Add filtering by category/type

**Expected response format**:
```json
{
  "products": [
    {
      "id": "uuid",
      "name": "Espresso",
      "description": "Bold coffee...",
      "price": 350,
      "color": "#8B4513",
      "roast_type": "Dark Roast",
      "ounces": 2,
      "bean_type": "Arabica"
    }
  ]
}
```

#### 2.4 Update TUI to Use API
- [ ] Create `pkg/api/client.go` - API client
- [ ] Replace hardcoded coffee data with API calls
- [ ] Add loading states during API calls
- [ ] Handle API errors in TUI
- [ ] Add retry logic for failed requests

---

## **PHASE 3: User Authentication**

### Objective
Implement user registration, login, and session management.

### Tasks

#### 3.1 Auth System Design
- [ ] Choose auth method: SSH fingerprint OR username/password
- [ ] Design user registration flow
- [ ] Design login flow
- [ ] Implement JWT token generation and validation

**Recommended**: Start with username/password, add SSH fingerprint later

#### 3.2 API Endpoints
- [ ] `POST /api/auth/register` - Create new user
- [ ] `POST /api/auth/login` - Get JWT token
- [ ] `POST /api/auth/refresh` - Refresh expired token
- [ ] `GET /api/auth/me` - Get current user info
- [ ] `PUT /api/auth/me` - Update user profile

#### 3.3 TUI Login Flow
- [ ] Create login view with username/password form
- [ ] Use `github.com/charmbracelet/huh` for form inputs
- [ ] Store JWT token in memory (or file for persistence)
- [ ] Send token with all API requests
- [ ] Handle token expiration and refresh

**Reference**: `terminal-shop-source/packages/go/pkg/tui/account.go`

#### 3.4 User Profile
- [ ] Show user info in account page
- [ ] Allow editing name, email
- [ ] Add password change functionality

---

## **PHASE 4: Cart Persistence**

### Objective
Save cart data to database and sync across sessions.

### Tasks

#### 4.1 Cart API
- [ ] `GET /api/cart` - Get current user's cart
- [ ] `POST /api/cart/items` - Add item to cart
- [ ] `PUT /api/cart/items/:id` - Update item quantity
- [ ] `DELETE /api/cart/items/:id` - Remove item
- [ ] Calculate totals on backend

#### 4.2 Update TUI Cart
- [ ] Fetch cart from API on startup
- [ ] Sync cart changes to API immediately
- [ ] Handle offline mode (local-only cart)
- [ ] Merge local cart with server cart on login

---

## **PHASE 5: Payment Integration**

### Objective
Accept real payments via Stripe.

### Tasks

#### 5.1 Stripe Setup
- [ ] Create Stripe account (test mode)
- [ ] Add `github.com/stripe/stripe-go/v78`
- [ ] Set up Stripe API keys in `.env`
- [ ] Create Stripe customer on user registration

#### 5.2 Payment Method Management
- [ ] Create payment form in TUI (like terminal.shop)
- [ ] Collect: card number, expiry, CVC, zip
- [ ] Validate card details client-side
- [ ] Tokenize card with Stripe API
- [ ] Save card token to database

**Reference**: `terminal-shop-source/packages/go/pkg/tui/payment.go`

#### 5.3 Payment API
- [ ] `POST /api/cards` - Save payment method
- [ ] `GET /api/cards` - List saved cards
- [ ] `DELETE /api/cards/:id` - Remove card
- [ ] `POST /api/orders/checkout` - Create order and charge

#### 5.4 Alternative Payment Flow
- [ ] Generate QR code for browser payment
- [ ] Create payment link with Stripe
- [ ] Poll for payment completion
- [ ] Update order status

**Reference**: `terminal-shop-source/packages/go/pkg/tui/qrfefe/qr.go`

---

## **PHASE 6: Shipping & Orders**

### Objective
Collect shipping addresses and create orders.

### Tasks

#### 6.1 Address Management
- [ ] Create address form in TUI
- [ ] Validate addresses (use Shippo or similar)
- [ ] Save addresses to database
- [ ] Allow selecting from saved addresses

#### 6.2 Shipping Calculation
- [ ] Integrate Shippo for shipping rates
- [ ] Calculate shipping based on address
- [ ] Show shipping options to user
- [ ] Add shipping cost to order total

#### 6.3 Order Creation
- [ ] Create checkout confirmation screen
- [ ] Show order summary: items, subtotal, shipping, total
- [ ] `POST /api/orders` - Create order
- [ ] Charge payment method
- [ ] Send confirmation email (optional)

#### 6.4 Order History
- [ ] `GET /api/orders` - List user's orders
- [ ] Show order history in account page
- [ ] Display order details and status

---

## **PHASE 7: SSH Server**

### Objective
Deploy as SSH server accessible via `ssh shop@yourserver.com`

### Tasks

#### 7.1 SSH Server Setup
- [ ] Add `github.com/charmbracelet/wish`
- [ ] Create SSH server in `main.go`
- [ ] Generate or load SSH host keys
- [ ] Handle concurrent user sessions

**Reference**: `terminal-shop-source/packages/go/cmd/ssh/main.go`

#### 7.2 Authentication
- [ ] Implement public key authentication
- [ ] Link SSH fingerprints to user accounts
- [ ] Auto-register new SSH keys
- [ ] Handle key-based login

#### 7.3 Deployment
- [ ] Set up VPS/cloud server (DigitalOcean, AWS, etc.)
- [ ] Configure firewall (port 22 or custom)
- [ ] Set up systemd service
- [ ] Add logging and monitoring
- [ ] Configure domain name

---

## **PHASE 8: Polish & Production**

### Objective
Make it production-ready.

### Tasks

#### 8.1 Testing
- [ ] Write unit tests for API handlers
- [ ] Write integration tests for critical flows
- [ ] Test payment flow thoroughly
- [ ] Load test with multiple concurrent users

#### 8.2 Admin Features
- [ ] Create admin dashboard (web or TUI)
- [ ] Manage products (CRUD operations)
- [ ] View orders and fulfill them
- [ ] Manage inventory

#### 8.3 Production Stripe
- [ ] Switch to Stripe production mode
- [ ] Handle webhooks for payment events
- [ ] Implement proper error handling
- [ ] Set up refund capability

#### 8.4 Monitoring & Logging
- [ ] Add structured logging
- [ ] Set up error tracking (Sentry)
- [ ] Add metrics (Prometheus)
- [ ] Create health check endpoint

---

## Quick Start Commands

```bash
# Run current TUI app
go run server.go model.go

# Run API server (future)
cd api && go run main.go

# Run tests (future)
go test ./...

# Database migrations (future)
go run migrations/migrate.go up

# Deploy (future)
make deploy
```

## Environment Variables

```bash
# .env file (create this)
DATABASE_URL=postgresql://user:password@localhost:5432/terminalshop
JWT_SECRET=your-secret-key-here
STRIPE_SECRET_KEY=sk_test_...
STRIPE_PUBLIC_KEY=pk_test_...
SSH_HOST_KEY_PATH=./ssh_host_key
API_PORT=8080
SSH_PORT=2222
```

## Resources

- **Reference Implementation**: `terminal-shop-source/` directory
- **Technical Specs**: See `MODELS.md`
- **Bubbletea Tutorial**: https://github.com/charmbracelet/bubbletea/tree/master/tutorials
- **Stripe Go Docs**: https://stripe.com/docs/api?lang=go
- **Chi Router**: https://go-chi.io/

## Contributing Guidelines for Claude Code

When working on this project:

1. **Always check MODELS.md** before writing code to follow established patterns
2. **Read the current phase** in this README to understand what to build next
3. **Reference terminal.shop source code** in `terminal-shop-source/` when unsure
4. **Update this README** when completing tasks (check off boxes)
5. **Follow Go best practices** and the style guide in MODELS.md
6. **Write tests** for new API endpoints
7. **Keep commits focused** - one feature/fix per commit

## License

MIT (or your preferred license)
