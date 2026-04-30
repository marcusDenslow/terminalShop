# Roadmap

Synthesised from a four-pronged audit (TUI, checkout, API, code quality) comparing this project against `terminal-shop-source/`. Items are grouped by impact, not by sequence — but the **Phase** ordering reflects what unblocks what. Every item links to the file path (and line where useful) so future sessions can pick up cold.

The legend across every entry:
- **MATCHES** — parity with reference
- **LACKS** — reference has it, we don't
- **EXTRA** — we have it, reference doesn't (kept on purpose unless flagged)
- **WORSE / BETTER** — both sides have it; relative quality

---

## Phase 0 — Hygiene (do first; small, prevents footguns)

These are quick wins that are blocking nothing but should be done before opening larger refactors.

### 0.1 Remove tracked artefacts
- `terminalshop.db` (root) and `api/terminalshop.db` are committed SQLite DBs. May contain test users / Stripe IDs.
  - `git rm --cached terminalshop.db api/terminalshop.db`
- Compiled binaries `main` (~20MB) and `terminalShop` (~22MB) are tracked.
  - `git rm --cached main terminalShop`
- `server.go.old` is a stale duplicate of `main.go`. Delete.
- `test_stripe.go` at root is dead manual test code under `//go:build ignore`. Move to `cmd/striptest/` or delete.
- `pkg/tui/qrfefe/` — likely a typo for `qrcode`; rename and fix the unused `size` parameter in `Generate`.
- Verify `.env` isn't tracked: `git ls-files | grep .env`. If it is, `git rm --cached .env` and **rotate any Stripe keys**.

### 0.2 Resolve port mismatch
`main.go:26` hardcodes port `23456`; `pkg/config` defaults `SSH_PORT` to `23457`; `CLAUDE.md` and `README` claim `23457`. Pick one (use `cfg.SSHPort`) and update docs in `main.go` + `CLAUDE.md`.

### 0.3 Delete dead checkout code
`pkg/tui/confirmation.go:14-125` defines `RenderConfirmation`, `ConfirmView`, `ConfirmUpdate` — superseded by `ReviewSwitch`/`generateReviewContent` and never called. Remove.

### 0.4 Wire or remove `ssh_keys.go`
`api/handlers/ssh_keys.go` defines `GetSSHKeys`/`DeleteSSHKeys` but `api/routes/routes.go` never registers them. Wire or delete.

### 0.5 Lowercase error message
`api/handlers/cards.go:348` returns `fmt.Errorf("Failed to save...")` capitalized — violates code style.

### 0.6 Fix red "press enter to confirm"
`pkg/tui/confirmation.go:158` uses `m.theme.TextError().Render(...)` for an action prompt — misleading. Reference uses `TextBrand()`.

### 0.7 Breadcrumb medium-size threshold
`pkg/tui/breadcrumbs.go:21-25` only abbreviates on `< large`; reference abbreviates on `small||medium`. May overflow on medium terminals.

---

## Phase 1 — Foundational systems (these compound; fix first)

The audits identified three foundations where every new feature lands awkwardly until addressed: theme parameterization, per-page state encapsulation, and API envelope shape.

### 1.1 Parameterized theme + tokens
**Why this first:** Reference `BasicTheme(renderer, highlight *string)` swaps the highlight per selected product, giving the shop view its colour-shift effect. We hardcode highlight to `#4682B4` in `pkg/tui/theme/theme.go:36`, so the per-product theme pattern is impossible.

- Add `pkg/tui/theme/tokens.go` with named palette constants — every `lipgloss.Color("#XXXXXX")` and raw ANSI literal (`"196"`, `"205"`, `"52"`) currently embedded in `pkg/tui/theme/theme.go` and `pkg/tui/shop.go` (line 72,124), `pkg/tui/cart.go:54`.
- Change `BasicTheme(renderer, highlight *string)` to accept an optional highlight; default to brand if nil.
- Add a `Theme.ProductColor(hex string) lipgloss.Style` accessor for dynamic product colours; remove direct `lipgloss.Color(coffee.Color)` usages.
- Make `Theme.Base()` return `t.base.Copy()` (`pkg/tui/theme/theme.go:82`) — chained mutations on the cached style currently leak across renders.
- Add an `UpdateSelectedTheme(coffee)` call in shop nav (mirrors reference `shop.go:523-542`).

### 1.2 Per-page state structs (refactor `model.go`)
**Why:** `pkg/tui/model.go` is 964 lines with ~40 boolean flags. `account.go` (428) similarly farms booleans — `addressListFocused`, `cardListFocused`, `faqFocused`, etc. Reference uses one `state` struct with per-page substructs (`root.go:89-107`) and a `[]page` array for account sub-pages (`account.go:11-17`).

Files to introduce inside `model.go` (or a new `model_state.go`):
- `splashState`, `cartState`, `shippingState`, `paymentState`, `accountState`, `reviewState`, `confirmState`.
- Replace the boolean farm in `account.go` with `selected int`, `focused bool`, and an explicit `[]page` for account sub-pages.

Then split `model.go` into:
- `model.go` (~150 lines: struct + constructor)
- `model_state.go` (per-page state structs)
- `model_switch.go` (`*Switch` helpers + footer config)
- `commands.go` (every `tea.Cmd` builder: `fetchCart`, `fetchOrders`, `fetchAddresses`, `saveAddress`, `fetchCards`, `deleteCard`, `checkoutWithSavedCard`, `saveCardAndConvert`, `saveCardOnlyCmd`, `collectCardCmd`, `pollCardsCmd`).

While here, extract `resolveCartAddressCmd(m Model) error`: `checkoutWithSavedCard` and `saveCardAndConvert` (`model.go:592-726`) duplicate ~30 lines of address-resolution logic.

### 1.3 Structured validation errors
**Why:** Field-level validation is currently ad-hoc string checks (`addresses.go:69`, `cart.go:102`, `auth.go:121-127`) returning a single message. The TUI shows one error at a time even when a form has multiple bad fields. Matching the reference's exact envelope shape isn't necessary (no external client consumes it), but you do want structured per-field issues so the TUI can highlight the right input.

- Add a central validator middleware (or helper) that returns `{issues: [{path, message}]}` per failing field. Reference does this via Zod (`common.ts:33-117`); a minimal Go equivalent is ~50 lines.
- Add a `pkg/utils/codes.go` with exported error-code constants — current strings (`CART_ERROR`, `INVALID_ID`, etc.) are scattered. One file makes them greppable and prevents typos like `"ADDRESS NOT FOUND"` (the bug in old `NEXT.md`).
- Keep the `{success, data|error}` envelope as-is unless an external client ever shows up.

---

## Phase 2 — Checkout flow correctness (impact on real customers)

### 2.1 Shipping cost calculation + display
**Critical gap.** `pkg/tui/confirmation.go:46-48,154-156` always renders `$0.00`. No shipping rate calculation exists anywhere. Reference pulls `m.cart.Amount.Shipping` from the API.

- Add a `pkg/shipping` rate calculation (Shippo client already exists for address validation — extend it).
- Backend: have `/cart` and `/cart/convert` populate `cart.Amount.Shipping`.
- TUI: add `paymentCostsView()` (mirrors reference `payment.go:714,750-771`) shown above the card list AND inside the form.
- TUI: render shipping line in `confirmation.go` and `confirm.go` review.

### 2.2 Confirm-delete pattern for addresses + cards
Both pages currently delete immediately:
- `pkg/tui/shipping_form.go:414-432` — destructive, no undo.
- `pkg/tui/payment_form.go:503-521` — same.

Reference uses inline `y/n` confirmation tied to `state.deleting *int` (`shipping.go:251-277`, `payment.go:376-402`). Adopt the same pattern.

### 2.3 Auto-scroll focused form field into view
`pkg/tui/shipping_form.go` and `pkg/tui/payment_form.go` set viewport content but never scroll to the focused input. On small terminals a focused field can be off-screen. Reference: `ensureShippingFocusedInputIsVisible` (`shipping.go:73-132`) and `ensurePaymentFocusedInputIsVisible` (`payment.go:104-165`).

### 2.4 Restore field-level validators
`pkg/tui/shipping_form.go:73` removes per-field validators "so Tab/Enter move freely"; same for `payment_form.go`. Reference keeps `validate.NotEmpty` per field (`shipping.go:535-569`) for in-place feedback. Currently we rebuild the whole form on submit-error (`shipping_form.go:216,367`), losing focus.

### 2.5 Country list beyond {NO, US}
`pkg/tui/shipping_form.go:24-27` hardcodes the country list. Reference accepts free-text country (`shipping.go:556-560`). Add ISO-2 validation in the API and a wider list (or free-text) in the form.

### 2.6 Centralize footer commands in `*Switch()` methods
Footers are set ad-hoc inside `Update` branches (`shipping_form.go:350-354`, `payment_form.go:421-425, 487-491, 498-500, 531-536, 550-555`). Easy to drift. Reference sets them once on entry. After Phase 1.2 this becomes a one-liner per `*Switch`.

### 2.7 Robust `error` handling in checkout
`pkg/tui/confirmation.go:185-231` (`ReviewUpdate`) doesn't handle a generic `error` `tea.Msg`. If the checkout cmd returns a non-`CheckoutResultMsg` error, `CheckingOut` stays `true` forever. Mirror reference's `confirm.go:101-103` and `payment.go:584-592` — clear `submitting` on any `error`.

### 2.8 Profile update on payment submit
Reference `payment.go:504-514` calls `Profile.Update` to persist name/email entered during checkout. We don't (we have no profile endpoint anyway — see Phase 4.1).

### 2.9 Distinct submitting states / spinner messages
Reference distinguishes "verifying payment details…" vs "generating payment link…" (`payment.go:653-659`). We show generic strings in `payment_form.go:247,370`.

---

## Phase 3 — TUI feature parity (visible polish)

### 3.1 Reusable cursor + logo
- Add `pkg/tui/cursor.go` (700ms tick, `state.cursor.visible`) — reference has one shared blink, we only blink on splash via `splashCursorTickMsg` (`model.go:151,422`).
- Add `pkg/tui/logo.go` with `LogoView()`. Currently re-implemented inline in `splash.go:45`, `menu.go:29`, `account.go:186-210`.

### 3.2 Region toggle on shop view
`r` key cycles NA / EU / Global with re-fetch (reference `shop.go:155`, `footer.go:49-82`). Touches: `pkg/tui/shop.go`, `pkg/tui/footer.go`, API products endpoint (Phase 4.4 — variants by region).

### 3.3 Featured / originals product sections
Render `~ featured ~` / `~ originals ~` headers from `Tags.Featured` (reference `shop.go:348-389`). We flat-list everything (`shop.go:92-99`).

### 3.4 Splash error path + auth failure surface
`pkg/tui/splash.go:35-59` has no error rendering — auth failure falls through silently (`update.go:74`). Reference handles `error != nil` with an `esc quit` hint (`splash.go:115-165`).

### 3.5 Footer parity
- Top border (`footer.go:91-93` reference) — we just centre text.
- Error banner with esc-hint and word-wrap (reference `footer.go:128-156`); ours sits above content (`views.go:76-79`) and has no esc affordance.
- "Free shipping" hint line + region indicator (reference `footer.go:122-156`).

### 3.6 Account view dual viewports + scroll-pos restore
- Reference has `menuViewport` + `detailViewport` (`account.go:14-16`); we only have `detailViewport` (`model.go:188`). Left menu doesn't scroll for users with many sub-pages.
- Subscription scroll-pos restore — currently only orders saves `yOffset` (`account.go:337,411-414`). Add the equivalent for subscriptions when those land (Phase 4.2).

### 3.7 Promote about + FAQ to dedicated pages
We render both as account sub-panels. Reference has `AboutSwitch`/`FaqSwitch` as page-level. Lower-priority polish but unblocks `aboutPage`/`faqPage` in the page enum (Phase 1.2).

### 3.8 Final / final-sub post-purchase pages
Reference has `final.go` / `final-sub.go` as a separate terminal "thank-you" state with its own viewport. Our flow ends inside `confirmation.go`. Add a `finalPage` after `ReviewSubmit` succeeds.

### 3.9 Unified error-to-Visible-error pipeline
`update.go:55-60` catches raw `err.Error()` and only re-fetches cart. Reference `root.go:266-278` runs `api.GetErrorMessage` to normalise messages and re-fetches the right resource per page.

### 3.10 Command-line entry routing
Reference parses `command []string` from the SSH session — `ssh terminal.shop espresso` lands directly on the espresso product (`root.go:195-258`). We always splash → shop. Easy to add to `main.go` + `model.NewModelWithAuth`.

### 3.11 Resize debounce — keep
`pkg/tui/update.go:14-41` debounces resize via `resizeTickMsg`. Reference resizes synchronously (`root.go:279-303`). Ours is **BETTER**; document the rationale in a comment so it isn't accidentally removed during the Phase 1.2 refactor.

---

## Phase 4 — API completeness (closes the largest functional gap)

Reference TS API at `terminal-shop-source/packages/functions/src/api/`. Our Go API at `api/handlers/`.

### 4.1 Profile resource — **missing entirely**
- `pkg/models/profile.go` (or extend user model)
- `api/handlers/profile.go`: `GET /profile`, `PUT /profile {name, email}`
- Wire group in `api/routes/routes.go`

### 4.2 Subscriptions — **missing entirely**
Major gap: products carry a `subscription` flag but we have no model, table, routes, or TUI flow.

- `pkg/models/subscription.go` + `pkg/models/subscription_schedule.go` (matches `core/src/subscription/index.ts`)
- `api/handlers/subscriptions.go`: `GET /subscription`, `GET /:id`, `POST /`, `DELETE /:id`, `PUT /:id`
- Reference TUI counterparts to port: `subscribe.go` (variant picker), `subscriptions.go` (list/detail/cancel)

### 4.3 Product variants + region filtering
**Largest single product gap.** `pkg/models/coffee.go` has flat `price`; reference has `variants[]` with prices + inventory + `subscription` setting + `tags` + `order` (`core/src/product/index.ts:19-86`).

- Migrate to string IDs (`prd_*`) and add `variants` table.
- Cart `SetItemRequest` body becomes `{productVariantID, quantity}` instead of `{coffee_id, quantity}` (`api/handlers/cart.go:87-90`).
- Add IP geolocation (reference `ipinfo.ts`) and filter variants by `Region`.

### 4.4 Address / card / cart / order schema cleanup
Reference field shapes are listed for cross-reference, but since this API only ever serves the SSH TUI client, **field renames are optional cosmetic churn** — only make changes that improve correctness or unblock other work.

- **Keep** (correctness / unblocks Phase 2.1):
  - Cart (`api/handlers/cart.go:43-71`): nest `amount: {subtotal, shipping, total}` and `shipping: {service, timeframe}` so the TUI can render shipping cost.
  - Addresses (`api/handlers/addresses.go:54-72`): ISO-2 country validation.
  - Orders: add `GET /orders/{id}` (Phase 3.6 order detail benefits); expose `tracking` once Phase 4.5 lands.
- **Optional / skip unless you feel like it**: rename `street_2` → `street2`, `state` → `province`, switch SCREAMING_SNAKE error codes to lowercase, drop `is_default` on cards. None of this matters with a single in-tree client.
- **Audit, don't necessarily remove**: `POST /orders/{id}/refund` (`orders.go:40-95`) — reference handles refunds via Stripe dashboard. If you want admin self-serve, keep it; otherwise gate behind admin auth or drop.

### 4.5 Shippo tracking webhook
`api/handlers/webhook.go` doesn't handle `/webhooks/shippo`. Without this, orders never advance past `paid` to `shipped`/`delivered`. Spec: reference `hook.ts:32-82` → `Order.UpdateTrackingStatus`.

### 4.6 Inventory tracking on order create
`api/handlers/cart.go:324-335` (`ConvertCart`) doesn't decrement inventory or reject out-of-stock. Reference writes `inventoryRecordTable` on every order.

### 4.7 Email
- Order receipt + shipped notification. Choose a transactional provider; wire on order status transitions.
- Newsletter signup (`POST /email`) — only if you actually plan to send a newsletter; otherwise skip.

### 4.8 `/view/init` parity
`api/handlers/view.go:47-54` returns `{user, products, cart-as-items, addresses, cards, orders}`. After Phase 4.1–4.2 land, expand to include `profile`, full `Cart.Info` (with `amount`/`shipping`), `subscriptions`, and `region`. Drop `tokens` / `apps` — see out-of-scope note below.

---

## Phase 5 — Tests (lift coverage to match reference)

### 5.1 API handler tests
Currently no tests for: `cards.go` (351 lines, Stripe-critical), `orders.go`, `webhook.go`, `reconcile.go`, `view.go`. Reference has matching `card.test.ts`, `order.test.ts`, `view.test.ts`.

Adopt the existing pattern (table-driven + `httptest`). Files to add: `api/handlers/cards_test.go`, `webhook_test.go`, `orders_test.go`, `reconcile_test.go`, `view_test.go`.

### 5.2 TUI Update tests
`pkg/tui/menu_test.go` is an empty table; `pkg/tui/model_test.go` tests only `updateLayout`. No tests cover `Update`, page switches, cart math, or scroll. Add `pkg/tui/update_test.go` covering `tea.KeyMsg` for nav keys + `CartSyncedMsg` + `CheckoutResultMsg`.

### 5.3 Cart math + form validation
`validatePaymentForm` is pure and easily unit-tested. `CalculateSubtotal` and cart sort order in `GetCartItemsSlice` likewise.

### 5.4 Token refresh test
The JWT rotation logic at `pkg/tui/model.go:468` has no test. Add one.

### 5.5 Subscription / email tests
After Phase 4.2 / 4.7 land, port reference `subscription.test.ts` and `email.test.ts`.

---

## Phase 6 — Things we do better (preserve and document)

These are intentional design wins worth keeping. Document them with comments at the call sites so they aren't reverted by accident.

- **Server-side authoritative cart** — DB-backed, can't desync between sessions. Reference is client-side optimistic.
- **Idempotent payments** — order written before Stripe charge, with `[CRITICAL]` log on partial failure (`docs/checkout.md`).
- **Atomic checkout transaction** — order create + cart clear + address/card sync in one DB tx.
- **Multi-key SSH support** — reference only trusts the SSH transport key.
- **Rate limiting** — auth (20/min), card creation (15/min), checkout (10/min). Reference has none visible.
- **Reconciliation job** (`api/handlers/reconcile.go`) — needed because of our eager-confirm flow; reference's webhook-driven flow doesn't need one.
- **Resize debounce** (`pkg/tui/update.go:14-41`).
- **Custom error envelope hygiene** — handlers consistently use `utils.RespondSuccess`/`RespondError`; no `http.Error` leaks (audited, 0 hits). Keep this discipline as Phase 1.3 reshapes the envelope.
- **Extra theme palette colours** (`success`, `loading`, `label`, `dim`) beyond reference's minimal set — flexibility for spinner / labels.

---

## Reference paths cheat-sheet

User project:
- TUI: `pkg/tui/` (22 files, 4,223 lines)
- API: `api/handlers/`, `api/middleware/`, `api/routes/`
- Models: `pkg/models/`
- Theme: `pkg/tui/theme/`
- Utils: `pkg/utils/response.go`

Reference (`terminal-shop-source/`):
- Go TUI: `packages/go/pkg/tui/` (27 files, 5,854 lines)
- TS API: `packages/functions/src/api/`
- TS domain models: `packages/core/src/{product,cart,address,card,order,user,subscription}/`
- Errors: `packages/core/src/error.ts`
- Tests: `packages/functions/test/api/*.test.ts`

---

## Out of scope / explicitly skipped

- **Personal access tokens (`trm_*`)** — reference has them so third parties can call the API from `curl` / SDKs without an SSH session. We only have one client (the SSH TUI), and the JWT it gets at splash is scoped to that session. Adding PATs would mean a new model, new middleware path, new TUI reveal-once UX, and a longer-lived secret to leak. Not worth it unless someone asks for programmatic access.
- **OAuth apps** — reference has them so third-party websites can act on a user's behalf without seeing the user's credentials. We have no third parties. Same reasoning as above, plus a redirect flow on top.
- **Stainless-generated SDK** — reference ships `stainless.yml` to auto-generate client libraries (JS/Python/Go). Only useful if you have programmatic API consumers, which means PATs or OAuth, which we just skipped.
- **Reference-exact API field naming** — `street1` vs `street_2`, lowercase-snake error codes, bare `{data}` envelope. With one in-tree client these are cosmetic.
- **Stripe Hosted Checkout → Stripe.js custom card form** — described in `docs/stripe-js-card-form.md`. Still a valid future option; not on the critical path. Revisit if branding/UX justifies it.
- **PostgreSQL migration** — currently SQLite via SQLite driver in `pkg/database/db.go`. Move when concurrent-write contention shows up; not before.
- **Catppuccin theme** — present in `pkg/tui/theme/theme.go:49-66` but unused. Harmless; keep or remove with no feature impact.
