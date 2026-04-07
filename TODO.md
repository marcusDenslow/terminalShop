# TODO

## Done
- [x] Cart persistence across sessions
- [x] Token refresh — auto-refresh JWT before expiry
- [x] Help screen — press ? to show keybindings
- [x] Order detail view — press Enter on an order to see full details
- [x] FAQ page
- [x] About page
- [x] Splash screen — logo + auth + data preload on connect
- [x] Bulk data preload — /view/init endpoint fetches all user data at once
- [x] Stripe webhook — payment_intent.succeeded/failed sync order status
- [x] PCI-safe card tokenization — publishable key in SSH process, raw card data never reaches backend
- [x] Rate limiting — per-IP and per-user limits on sensitive endpoints
- [x] Audit logging — structured JSON logs for all financial events
- [x] Shop page — dual viewport (menu left, detail right), matches reference shop.go
- [x] Cart page — single viewport with proper scroll
- [x] Shipping page — single viewport
- [x] Payment page — single viewport
- [x] Confirm page — single viewport (uses DefaultKeyMap for pgup/pgdn scrolling)
---

## Do First (meaningful gaps vs reference)

### Product variants
The reference supports multiple variants per product (roast type, grind, size) with
separate prices. We treat every product as single-variant. Without this the shop is
very limited once you have real products.
- [ ] Variant model + DB migration
- [ ] API: return variants in /products response
- [ ] TUI: show variant selector in shop detail panel

### User profile editing
Users currently cannot update their name or email from inside the app.
- [ ] PUT /api/v1/profile endpoint
- [ ] TUI: editable name/email fields in account view

### Missing API endpoints
- [ ] PUT /api/v1/addresses/:id — update a saved address
- [ ] GET /api/v1/orders/:id — single order (needed for better order detail view)

---

## Polish (noticeable but not blocking)

- [ ] Animated cursor — reference has a blinking block cursor on the splash/logo
      (cursor.go, CursorTickMsg, 700ms interval). Small but gives the app personality.
- [ ] Themed UI — reference has a theme/ package; we use hardcoded hex colors everywhere.
      Makes it hard to change the look later.
- [ ] Shipping timeframe — Shippo returns estimated delivery window; show it on confirm page
- [ ] Loading spinner — replace plain "loading..." text with animated spinner
- [ ] HTTPS/QR payment option — third card-entry method, generates a one-time URL + QR code
      so users can enter card details in their browser instead of over SSH.
      Reference: payment.go paymentHttpsView, PollPaymentInitMsg/StatusMsg/CompleteMsg.
- [ ] Per-field card validation — use huh .Validate() on each form field so errors
      show inline as user tabs through, instead of a single error on submit.

---

## Testing

- [ ] API client tests — pkg/api/client.go has no tests
- [ ] Card handler tests — api/handlers/cards.go
- [ ] Order handler tests — api/handlers/orders.go
- [ ] Address handler tests — api/handlers/addresses.go (partial)
- [ ] Webhook handler tests — api/handlers/webhook.go
- [ ] TUI payment form tests — validatePaymentForm() is pure and easy to unit test

---

## Post-Launch

- [ ] Shippo webhook — order tracking status updates pushed from Shippo
- [ ] Email system — order confirmation + shipped notification
- [ ] Subscription model — DB table, CRUD endpoints, TUI flow (select → shipping → payment → confirm)
- [ ] Subscription management — list, detail, cancel with confirmation in account view
- [ ] Personal access tokens — model, CRUD API, TUI management in account view
- [ ] OAuth app management — model, CRUD API, TUI create/list/delete
- [ ] Region-based product filtering — IP geolocation, press 'r' to toggle
- [ ] Cron jobs — subscription processing, inventory tracking, unshipped alerts
