# Terminal Coffee Shop

A terminal-based coffee shop, accessible over SSH. Built with Go, Bubbletea, Wish, Chi, PostgreSQL, Stripe, and Shippo.

```bash
ssh localhost -p 23456
```

## Running locally

```bash
# Copy and fill in env vars
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

- SSH server — key-based auth, auto user creation
- Shop page — browse products with live detail panel
- Cart — add/remove items, synced to API
- Checkout flow — shipping address → payment → confirmation
- Saved addresses and cards
- Order history in account view
- FAQ page
- Responsive layout (small/medium/large terminal sizes)
- Token refresh

## TODO

**Features missing vs reference (terminal.shop):**
- [ ] Splash/loading screen on connect
- [ ] Animated logo with blinking cursor
- [ ] Subscriptions (recurring orders) — subscribe, manage, cancel
- [ ] API token management page
- [ ] OAuth app management page
- [ ] About page
- [ ] Theme system — currently 40+ hardcoded hex colors scattered across files
- [ ] Viewport-based scrolling — currently manual scroll math with hardcoded heights

**Known bugs:**
- [ ] Address delete slice bug in `shipping_form.go:386` — `[m.AddressCursor+1:]...` missing
- [ ] Account page has a placeholder "something" menu item in `models/coffee.go`

**Code quality:**
- [ ] Nested state structs (reference uses `state.shop`, `state.cart`, etc. vs flat Model)
- [ ] Centralized styles file
