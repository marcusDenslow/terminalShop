# TODO

## In Progress
- [ ] Order detail view — press Enter on an order in account view to see full details

## Up Next
- [ ] User profile editing — set name/email from account view
- [ ] Cart persistence across sessions — sync cart to server, fetch on startup
- [ ] Token refresh — auto-refresh JWT before expiry
- [ ] Address update — PUT /api/v1/addresses/:id endpoint
- [ ] Loading spinner — replace plain text with animated bubbles/spinner
- [ ] Help screen — press ? to show keybindings

## Backlog
- [ ] Shipping rate calculation — actual costs from Shippo/Bring, not always $0
- [ ] Product filtering/pagination — filter by roast type, paginate results
- [ ] Single order detail API endpoint — GET /api/v1/orders/:id
- [ ] Price model consistency — Coffee.Price float64 vs Order.Total int cents
- [ ] Test coverage — auth, cards, orders, addresses, bring/shippo, validators
- [ ] Update stale documentation — README, CLAUDE.md phase references
