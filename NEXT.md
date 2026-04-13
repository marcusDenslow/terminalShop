# Next Session

## Feature: SSH Key Management in Account Page

Following the same pattern as address/card management we just built, add an
"ssh keys" section to the account page so users can see and delete their saved
SSH public keys.

### What needs doing

1. **Backend** — check if `api/handlers/` has SSH key endpoints (list, delete).
   If not, add them following the same pattern as address/card handlers.
   Model is likely already in `pkg/models/` or needs adding.

2. **`pkg/models/coffee.go`** — add `"ssh keys"` to `AccountMenuItems`

3. **`pkg/tui/model.go`** — add state fields:
   ```go
   SSHKeys               []models.SSHKey
   SSHKeysLoaded         bool
   SSHKeyListFocused     bool
   AccountSSHKeyCursor   int
   AccountSSHKeyDeleting *int
   ```
   Add `fetchSSHKeysCmd()` and `deleteSSHKeyCmd(id)` following the same
   pattern as `fetchCardsCmd` / `deleteCardCmd`.

4. **`pkg/tui/account.go`** — add `case "ssh keys":` in `BuildAccountView`
   showing each key's fingerprint/comment, and handle j/k, x, y/n, esc in
   `AccountUpdate` — copy the addresses block exactly.

5. **`pkg/tui/update.go`** — fetch SSH keys when switching to account page
   (same as orders).

### After that — API Tokens
Same pattern again but for API access tokens. Reference implementation is in
`terminal-shop-source/packages/go/pkg/tui/tokens.go`. Tokens have a "create"
action in addition to delete (generates a new token and shows it once).

### Staging environment (deferred)
Full setup guide is in the conversation history. Short version:
- Create `staging` git branch
- `docker-compose.staging.yml` already written and committed
- On server: clone repo into `~/terminalShop-staging`, create `.env.staging`,
  run `docker compose -f docker-compose.staging.yml --project-name staging up -d`
- Add `staging-api.sshops.uk` to Caddyfile
- Add `host.docker.internal:host-gateway` to Caddy in `docker-compose.prod.yml`
