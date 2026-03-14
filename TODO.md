# TODO

## Done
- [x] Cart persistence across sessions — sync cart to server, fetch on startup
- [x] Token refresh — auto-refresh JWT before expiry
- [x] Help screen — press ? to show keybindings
- [x] Order detail view — press Enter on an order in account view to see full details

## Up Next
- [x] FAQ page — replace hardcoded 2 Q&As with real content (embedded JSON, ~12 entries)
- [x] About page — replace placeholder with real content/credits
- [ ] Loading spinner — replace plain text with animated bubbles/spinner
- [ ] Address update — PUT /api/v1/addresses/:id endpoint
- [ ] Single order endpoint — GET /api/v1/orders/:id
- [ ] User profile editing — PUT /api/v1/profile, set name/email from account view

## Feature Gaps (vs Reference)
- [ ] Bulk data preload — single /view/init endpoint to fetch all user data at once
- [ ] Splash screen — logo + auth + data preload on connect
- [ ] Product variants — multiple variants per product (roast types) with separate prices
- [ ] Featured/originals product sections — categorize products in shop view
- [ ] Per-product theme colors — dynamic accent color based on selected product
- [ ] Region-based product filtering — IP geolocation, press 'r' to toggle region
- [ ] Stripe webhook — sync card updates from Stripe events
- [ ] Shippo webhook — order tracking status updates
- [ ] Email system — order confirmation, shipped notification emails

## Testing
- [ ] API client tests — pkg/api/client.go has no tests
- [ ] Card handler tests — api/handlers/cards.go
- [ ] Order handler tests — api/handlers/orders.go
- [ ] Address handler tests — api/handlers/addresses.go
- [ ] TUI tests — model_test.go and menu_test.go are empty skeletons
- [ ] Shippo/Bring integration tests

## Post-Release
- [ ] Subscription model — database table, CRUD endpoints, scheduling
- [ ] Subscription creation flow — TUI: select variant → shipping → payment → confirm
- [ ] Subscription management — TUI: list, detail view, cancel with confirmation
- [ ] Personal access tokens — model, CRUD API, TUI management in account view
- [ ] OAuth app management — model, CRUD API, TUI create/list/delete in account view
- [ ] Cron jobs — subscription processing, inventory tracking, unshipped alerts
