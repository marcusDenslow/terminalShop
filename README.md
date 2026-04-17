# Terminal Coffee Shop

A coffee shop you access over SSH. Built with Go, Bubbletea, Wish, Chi, PostgreSQL, Stripe, and Shippo.

```bash
ssh localhost -p 23456
```

## Running locally

```bash
# Copy and fill in your env vars
cp .env.example .env

# Start the API server
go run api/main.go

# Start the SSH server
go run main.go
```

## Stack

| Layer | Tech |
|-------|------|
| TUI | Bubbletea + Wish (SSH) |
| HTTP API | Chi router |
| Database | PostgreSQL + GORM |
| Auth | SSH key fingerprint + JWT |
| Payments | Stripe |
| Shipping | Shippo |

## What's working

- SSH server — key-based auth, auto user creation on first connect
- Splash screen with animated blinking cursor while auth + data loads
- Shop page — browse products with a live detail panel on the right
- Cart — add/remove items, synced to the API
- Checkout flow — shipping address → payment → confirmation
- Save/delete shipping addresses and payment cards
- Order history in the account view
- FAQ page
- Help page
- Responsive layout across small, medium, and large terminals
- Centralized theme system (no more scattered hex values)
- Token refresh

## Known bugs

- Address delete has an off-by-one slice bug in `shipping_form.go:386`
- Account page has a leftover placeholder menu item somewhere in the models

## TODO

- Viewport-based scrolling (currently manual scroll math with hardcoded heights)
- API token management page
- OAuth app management page
- About page
