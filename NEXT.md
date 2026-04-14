# Next Session

## Where the Project is Genuinely Stronger than the Reference

Before the gap list — these are intentional design wins worth keeping:

- **Server-side authoritative cart** — reference uses client-side optimistic updates which can
  diverge across sessions. Project's DB-backed cart can't desync.
- **Idempotent payments** — order record is created *before* the Stripe charge. If the TUI
  crashes mid-flow, there's a reconciliation trail. Reference has no explicit idempotency.
- **Atomic checkout transaction** — order + cart clear + address/card sync happen in one
  transaction. Reference handles this via webhook reconciliation (weaker).
- **Multi-key SSH support** — reference only trusts the single SSH transport key, can't manage
  per-user keys. Project's DB model supports revocation and multiple keys per user.
- **Rate limiting** — auth (20/min), card creation (15/min), checkout (10/min). Reference has none visible.

---

## Critical Fixes

### 1. SSH public key stored in TUI memory
**File:** `main.go:59` (`SSHPublicKeyStr` passed into model)

The reference stores only the fingerprint in TUI context — never the full key material. Storing
the full public key in the TUI model means it lives in memory for the session duration. If the
process is inspected or crashes with a heap dump, key material is exposed.

Fix: pass fingerprint only into the TUI. The full key is only needed during auth, not in the UI.

### 2. Silent database failures (8 locations)
Handlers return 200 OK even when the DB operation failed. Full list:
- `api/handlers/ssh_keys.go:53` — `db.Delete()` result ignored
- `api/handlers/addresses.go:174, 200, 203` — `db.Delete()`, `db.Update()`, `db.Save()` ignored
- `api/handlers/cards.go:122, 125` — `db.Update()` results ignored
- `api/handlers/cart.go:120, 130, 132, 167, 200, 217` — `db.Delete()`, `db.Create()`, `db.Update()` ignored

Pattern to use everywhere:
```go
if err := db.Delete(&key).Error; err != nil {
    utils.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
    return
}
```

### 3. ErrorMsg never cleared
**File:** `pkg/tui/model.go` (ErrorMsg string field)

Once set, `ErrorMsg` persists until the next state change. If a fetch fails then the user
navigates away and back, the stale error is still showing. The reference uses an explicit
`*VisibleError` type with a dismiss action.

Fix: clear `ErrorMsg` when entering a new page, or add explicit dismiss logic.

### 4. No pagination on list endpoints
`GET /api/v1/addresses`, `/cards`, `/orders` return all records. Fine now but a structural
issue for production. The reference's SDK handles pagination. Add `limit`/`offset` query
params and cap at a sensible default (e.g. 50).

### 5. Cart quantity=0 rows accumulate
When items are removed from cart, rows stay in the DB with quantity=0 (soft delete). Add a
cleanup job or switch to hard deletes — quantity=0 has no semantic value.

---

## TUI Architecture: Flat State vs Nested State

The reference uses nested state substruct per feature:
```go
type model struct {
    cartState    cartState
    accountState accountState
    paymentState paymentState
}
```

The project uses a flat struct with ~40 boolean flags and nullable pointers. This works but
as the feature set grows it becomes hard to reason about — e.g. what happens when
`ShowingMenu=true` AND `CheckoutInProgress=true`? The flat model doesn't enforce invariants.

This isn't a quick fix — it's a refactor to consider before adding more features. Each new
page/feature added to the flat model increases the mental load of reading `Update()`.

---

## Missing Features (Reference Has, Project Doesn't)

### API Token Management
The reference has a full tokens page: list tokens, create new (show secret once), revoke.
This is the standard way to give programmatic access without exposing the user's session JWT.

Required pieces:
1. `pkg/models/token.go` — Token model (ID, UserID, Secret hash, CreatedAt, LastUsedAt, Comment)
2. `api/handlers/tokens.go` — GetTokens, CreateToken (return plaintext once), DeleteToken
3. Routes in `api/routes/routes.go`
4. API client methods in `pkg/api/client.go`
5. TUI account section — list + create + revoke. Create shows secret in a "copy this now" panel,
   any keypress clears it permanently.

Reference: `terminal-shop-source/packages/go/pkg/tui/tokens.go`

### User Profile Editing
No `PUT /api/v1/profile` endpoint exists. Users can't update name or email after registration.

### Product Variants
Without variants the shop can only sell one SKU per product. This is the most limiting gap for
a real store. The reference models variant as a separate entity with its own price and inventory.

Requires: Variant model, DB migration, API response change, TUI variant selector in shop detail
panel, cart items reference VariantID not CoffeeID.

### Auto-registration on First SSH Connection
The reference registers users silently on first connection — zero friction. The project requires
an explicit registration step. This is a UX gap, not a security one (both are equally sound
auth-wise). Worth closing if public-facing.

---

## Token Refresh Recovery
If the background token refresh fails 5 times, the user is stuck with an expired token but
there's no recovery path — no prompt, no re-auth. Add a fallback: after N failures, show
an error with instructions to reconnect.

---

## Polish (Lower Priority)

- **Inconsistent error code string:** `api/handlers/addresses.go:169` uses `"ADDRESS NOT FOUND"`
  (space) instead of `"ADDRESS_NOT_FOUND"` (underscore) like the rest of the codebase.
- **Splash screen animation:** Reference has a blinking cursor on the logo (700ms tick).
  `pkg/tui/splash.go` exists but is static.
- **Per-field card validation:** Reference validates each form field inline. Project validates
  on submit only. Minor UX improvement.
- **Theme system:** 40+ hardcoded hex colors in TUI files. `pkg/tui/theme/` directory exists
  but colors are duplicated everywhere. Low priority until the UI stabilises.

---

## Staging Environment (Deferred)

- Create `staging` git branch
- `docker-compose.staging.yml` already written and committed
- On server: clone into `~/terminalShop-staging`, create `.env.staging`,
  run `docker compose -f docker-compose.staging.yml --project-name staging up -d`
- Add `staging-api.sshops.uk` to Caddyfile
- Add `host.docker.internal:host-gateway` to Caddy in `docker-compose.prod.yml`
