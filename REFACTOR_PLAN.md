# Refactor: Extract Flat Model State into Nested Page Structs

## Context

The TUI's `Model` struct has 40+ flat fields for page-specific state (cursors, viewports, form pointers, booleans). The reference implementation (`terminal-shop-source`) uses nested per-page state structs, which keeps related state grouped, simplifies page transitions, and makes it easier to add new pages. The `shopState` struct already follows this pattern — this refactor extends it to all other pages.

This is a **pure refactoring** — no new features, no behavior changes. It unblocks future work (product variants, profile editing) by making the Model manageable.

---

## New Struct Definitions (all in `model.go`)

### `splashState` (3 fields, from Model)
| Old Field | New Field |
|-----------|-----------|
| `splashDataReady` | `splash.dataReady` |
| `splashDelayDone` | `splash.delayDone` |
| `splashCursor` | `splash.cursor` |

### `cartState` (3 fields, new struct)
| Old Field | New Field |
|-----------|-----------|
| `CartCursor` | `cart.cursor` |
| `cartVP` | `cart.viewport` |
| `cartVPReady` | `cart.viewportReady` |

### `shippingState` (5 fields, new struct)
| Old Field | New Field |
|-----------|-----------|
| `ShippingForm` | `shipping.form` |
| `ShippingView` | `shipping.view` |
| `AddressCursor` | `shipping.addressCursor` |
| `shippingVP` | `shipping.viewport` |
| `shippingVPReady` | `shipping.viewportReady` |

### `paymentState` (7 fields, new struct)
| Old Field | New Field |
|-----------|-----------|
| `PaymentForm` | `payment.form` |
| `PaymentView` | `payment.view` |
| `CardCursor` | `payment.cardCursor` |
| `CollectURL` | `payment.collectURL` |
| `CollectCardCount` | `payment.collectCardCount` |
| `paymentVP` | `payment.viewport` |
| `paymentVPReady` | `payment.viewportReady` |

### `accountState` (14 fields, new struct)
| Old Field | New Field |
|-----------|-----------|
| `AccountCursor` | `account.cursor` |
| `ScrollOffset` | `account.scrollOffset` |
| `OrderViewState` | `account.orderViewState` |
| `OrderCursor` | `account.orderCursor` |
| `FaqFocused` | `account.faqFocused` |
| `AddressListFocused` | `account.addressListFocused` |
| `CardListFocused` | `account.cardListFocused` |
| `AccountAddressCursor` | `account.addressCursor` |
| `AccountCardCursor` | `account.cardCursor` |
| `AccountAddressDeleting` | `account.addressDeleting` |
| `AccountCardDeleting` | `account.cardDeleting` |
| `SSHKeyListFocused` | `account.sshKeyListFocused` |
| `AccountSSHKeyCursor` | `account.sshKeyCursor` |
| `AccountSSHKeyDeleting` | `account.sshKeyDeleting` |

### `reviewState` (2 fields, new struct)
| Old Field | New Field |
|-----------|-----------|
| `CardJustAdded` | `review.cardJustAdded` |
| `ReviewSuccess` | `review.success` |

### `confirmState` (5 fields, new struct)
| Old Field | New Field |
|-----------|-----------|
| `ConfirmTotal` | `confirm.total` |
| `ConfirmItems` | `confirm.items` |
| `ConfirmShipping` | `confirm.shipping` |
| `confirmVP` | `confirm.viewport` |
| `confirmVPReady` | `confirm.viewportReady` |

### Fields that STAY on Model (shared across pages)
`Cart`, `Coffees`, `SavedAddresses`, `SavedCards`, `SSHKeys`, `Orders`, `OrdersLoaded`, `SSHKeysLoaded`, `FAQs`, `ShippingInfo`, `SelectedCard`, `CheckingOut`, `lastCartUpdateID`, `User`, `AccessToken`, `APIClient`, `ErrorMsg`, `Loading`, all layout/resize/auth/rendering fields.

---

## Execution Order (one struct at a time, compile+test after each)

### Phase 1: `splashState` (simplest — 3 fields, 3 files)
- Files: `model.go`, `update.go`, `splash.go`

### Phase 2: `cartState` (3 fields, 3 files)
- Files: `model.go`, `cart.go`, `confirmation.go` (resets cart cursor after checkout)

### Phase 3: `confirmState` (5 fields, 1 file)
- Files: `model.go`, `confirmation.go`

### Phase 4: `reviewState` (2 fields, 2 files)
- Files: `model.go`, `confirmation.go`, `payment_form.go`

### Phase 5: `shippingState` (5 fields, 4 files)
- Files: `model.go`, `shipping_form.go`, `update.go`, `footer.go`

### Phase 6: `paymentState` (7 fields, 4 files)
- Files: `model.go`, `payment_form.go`, `update.go`, `footer.go`

### Phase 7: `accountState` (14 fields, 5 files — largest change)
- Files: `model.go`, `account.go`, `views.go`, `footer.go`, `update.go`
- Note: `ScrollOffset` moves here. All other pages use viewports, so the generic scroll block in `views.go` becomes account-only.

### Phase 8: Add `*Switch()` methods + remove `resetPageState()`
- Add `ShopSwitch()`, `CartSwitch()`, `AccountSwitch()`, `ShippingSwitch()`, `PaymentSwitch()`, `ReviewSwitch()` to `model.go`
- Each calls `SwitchPage()`, zero-initializes its page state (`m.cart = cartState{}`), updates viewport
- Replace `SwitchPage(X).resetPageState()` calls in `update.go`, `menu.go`, `confirmation.go`
- Delete `resetPageState()`

---

## Files Modified (summary)

| File | Scope of Changes |
|------|-----------------|
| `pkg/tui/model.go` | Define structs, remove flat fields, add Switch methods, delete `resetPageState()` |
| `pkg/tui/update.go` | Rename all field accesses, replace `SwitchPage().resetPageState()` with `*Switch()` |
| `pkg/tui/cart.go` | `m.CartCursor` -> `m.cart.cursor`, viewport fields |
| `pkg/tui/account.go` | 14 field renames to `m.account.*` |
| `pkg/tui/shipping_form.go` | `m.ShippingForm` -> `m.shipping.form`, viewport/cursor fields |
| `pkg/tui/payment_form.go` | `m.PaymentForm` -> `m.payment.form`, viewport/cursor fields |
| `pkg/tui/confirmation.go` | confirm + review + cart cursor field renames |
| `pkg/tui/views.go` | `m.ScrollOffset` -> `m.account.scrollOffset`, account field refs |
| `pkg/tui/footer.go` | Multiple field renames for page-state reads |
| `pkg/tui/menu.go` | Replace `SwitchPage().resetPageState()` with `*Switch()` |
| `pkg/tui/splash.go` | `m.splashCursor` -> `m.splash.cursor` |
| `pkg/tui/header.go` | No changes |
| `pkg/tui/breadcrumbs.go` | No changes |
| `pkg/tui/help.go` | No changes |
| `pkg/tui/faq.go` | No changes |

---

## Verification

After each phase:
1. `go build ./...` — must compile
2. `go test ./pkg/tui/...` — existing tests pass
3. After all phases complete, manual test:
   - SSH in, verify splash -> shop transition
   - Browse shop, add items to cart
   - Full checkout: shipping -> payment -> review -> confirm
   - Account page: orders, addresses, cards, SSH keys, FAQ tabs
   - Menu (m) and help (?) modals
   - Terminal resize during each page
