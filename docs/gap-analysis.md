# Gap Analysis: Terminal Coffee Shop vs Reference (terminal.shop)

**Date:** 2026-04-08

---

## What's Already Working

- SSH server with key-based auth + auto user creation
- Shop page with live detail panel
- Cart (add/remove, synced to API)
- Full checkout flow: shipping → payment → confirmation
- Saved addresses and saved cards
- Order history in account view
- FAQ page
- About page
- Splash screen with blinking cursor animation
- Responsive layout (small/medium/large)
- Token refresh
- Full backend API (Chi router, PostgreSQL, GORM, Stripe webhooks)
- Shippo shipping integration

---

## Missing Features (vs reference)

### 1. API Token Management

- **Reference:** Create, view, and revoke personal API tokens
- **Current:** Not implemented
- **Reference file:** `tokens.go`
- **Task:** Add tokens section under account view with list + create/revoke actions

### 2. OAuth App Management

- **Reference:** Create, edit, delete OAuth apps with redirect URIs; uses `huh` form
- **Current:** Not implemented
- **Reference file:** `apps.go`
- **Task:** Add apps section under account view with form-based CRUD

### 3. Keyboard Interactive SSH Auth (Anonymous Sessions)

- **Reference:** Supports keyboard interactive auth in addition to public key, generates UUID fingerprints for anonymous users
- **Current:** Public key auth only (`WithPublicKeyAuth` in `main.go`)
- **Task:** Add `WithKeyboardInteractiveAuth` to Wish options; generate UUID fingerprint for anonymous sessions

---

## Code Quality Issues

### Theme System

- **Reference:** Centralized `theme.Theme` struct with methods (`Base()`, `TextAccent()`, `Border()`, `Brand()`, `Accent()`)
- **Current:** 40+ hardcoded hex colors scattered across view files (e.g. `#4682B4`, `#FFFFFF`, `#666666`)
- **Task:** Create `pkg/tui/theme/theme.go`, extract all colors and styles, update all view files to reference theme

### State Management

- **Reference:** Nested state substruct: `state.shop`, `state.cart`, `state.account`, etc. — each feature owns its state
- **Current:** Flat `Model` struct with 80+ fields, all mixed together
- **Task:** Refactor into nested substructs per feature area (breaking change — do after features are stable)

### Viewport-Based Scrolling

- **Reference:** Uses `charmbracelet/bubbles/viewport` throughout for scrollable content
- **Current:** Manual scroll offset math with hardcoded heights (e.g. `-9` for cart viewport height)
- **Task:** Replace manual scroll math with `viewport.Model` in cart, account, and FAQ views

---

## Architecture Comparison

| Area | Current | Reference |
|------|---------|-----------|
| State management | Flat `Model` struct | Nested `state` substructs per feature |
| Theme/styling | 40+ hardcoded colors | Centralized `theme.Theme` |
| Scrolling | Manual offset math | `viewport.Model` |
| Backend | Full custom API (Chi + GORM) | External SDK (`terminal-sdk-go`) |
| Auth | SSH pubkey + JWT | SSH pubkey + keyboard interactive |
| Product model | Custom `Coffee` struct | SDK `terminal.Product` with variants |
| Forms | `huh` (partial) | `huh` (comprehensive, with huh theme) |

---

## Recommended Order of Work

1. **Fix known bugs** — address delete bug and placeholder item (quick wins, low risk)
2. **Theme system** — extract colors; unblocks all visual polish
3. **Viewport-based scrolling** — replace manual math, fixes responsiveness
4. **API token management** — useful feature, account section
5. **OAuth app management** — lowest priority unless API access is a goal
6. **Keyboard interactive SSH auth** — enables anonymous browsing
7. **Nested state structs** — last, after features are stable (pure refactor)
