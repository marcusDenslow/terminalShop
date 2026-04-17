# Terminal Coffee Shop

A coffee shop that runs in your terminal over SSH.

```bash
ssh sshops.uk
```

Hosted on Hetzner, behind Cloudflare.

## Stack

- **TUI** — Bubbletea + Wish
- **API** — Go + Chi
- **Database** — PostgreSQL + GORM
- **Auth** — SSH key fingerprint + JWT
- **Payments** — Stripe
- **Shipping** — Shippo

## What's in it

- SSH key auth, creates your account automatically on first connect
- Splash screen while it loads
- Shop, cart, full checkout flow (shipping → payment → confirmation)
- Save addresses and cards to your account
- Order history
- FAQ and help pages

## Running locally

```bash
cp .env.example .env
# fill in the env vars

go run api/main.go   # API server
go run main.go       # SSH server — connects on localhost:23456
```

## Known issues

- Address delete has an off-by-one bug in `shipping_form.go:386`
- Leftover placeholder menu item in the account page

## TODO

- Viewport-based scrolling (currently hardcoded heights)
- API token management page
- OAuth app management page
- About page
