# Theme System Implementation Reference

> Use this document when implementing or modifying the theme system.
> Every decision and pattern is explained so context loss doesn't stall progress.

---

## What We're Doing

Replacing 40+ scattered `lipgloss.Color("#XXXXXX")` calls across 13 TUI files with a
centralized `Theme` struct in `pkg/tui/theme/`. Copied closely from the reference
implementation in `terminal-shop-source/packages/go/pkg/tui/theme/`.

---

## New Files to Create

```
pkg/tui/theme/theme.go   — Theme struct, BasicTheme(), all color/style methods
pkg/tui/theme/huh.go     — HuhTheme(), copyFieldStyles(), copyTextStyles()
```

---

## Files to Modify (in order)

| Step | File | Type of change |
|------|------|----------------|
| 1 | `pkg/tui/theme/theme.go` | Create |
| 2 | `pkg/tui/theme/huh.go` | Create |
| 3 | `pkg/tui/model.go` | Add `renderer` + `theme` fields; update constructors |
| 4 | `main.go` | Create renderer from SSH session; pass to NewModelWithAuth |
| 5 | `pkg/tui/views.go` | 3 color swaps |
| 6 | `pkg/tui/splash.go` | 2 color swaps |
| 7 | `pkg/tui/header.go` | 4 color swaps |
| 8 | `pkg/tui/footer.go` | 2 color swaps |
| 9 | `pkg/tui/breadcrumbs.go` | 3 color swaps |
| 10 | `pkg/tui/shop.go` | 6 color swaps |
| 11 | `pkg/tui/cart.go` | 9 color swaps |
| 12 | `pkg/tui/account.go` | 14 color swaps |
| 13 | `pkg/tui/menu.go` | 4 color swaps |
| 14 | `pkg/tui/help.go` | 5 color swaps |
| 15 | `pkg/tui/confirmation.go` | 5 color swaps |
| 16 | `pkg/tui/shipping_form.go` | 4 color swaps + `.WithTheme()` |
| 17 | `pkg/tui/payment_form.go` | 5 color swaps + `.WithTheme()` |

---

## Color Mapping (hardcoded → theme)

| Hardcoded | Semantic role | Text style method | Color accessor |
|-----------|--------------|-------------------|----------------|
| `#FFFFFF` | active / selected text | `m.theme.TextAccent()` | `m.theme.Accent()` |
| `#AAAAAA` | body / regular text | `m.theme.TextBody()` | `m.theme.Body()` |
| `#666666` | hints / inactive / dim text | `m.theme.TextDim()` | `m.theme.Dim()` |
| `#666666` | border foreground | — | `m.theme.Border()` |
| `#4682B4` | highlight / brand / steel blue | `m.theme.TextHighlight()` | `m.theme.Highlight()` |
| `#999999` | form label text | `m.theme.TextLabel()` | `m.theme.Label()` |
| `#00FF00` | success / order placed | `m.theme.TextSuccess()` | `m.theme.Success()` |
| `"196"` | error text | `m.theme.TextError()` | `m.theme.Error()` |
| `"205"` | loading / spinner | `m.theme.TextLoading()` | — |
| `"196"` + `"52"` | error banner bg+fg | `m.theme.PanelError()` | — |
| `"C6DDF0"` | user name (light blue) | `m.theme.TextHighlight()` | — |
| `#CCCCCC` | muted subtext | `m.theme.TextBody()` | — |

**Note:** `#666666` is used for BOTH dim text AND border foreground in this project.
Both `Dim()` and `Border()` return the same value. Use `Border()` in `BorderForeground()`,
use `Dim()` / `TextDim()` for text styling.

**Note:** Dynamic product colors (`coffee.Color` = `"#8B4513"` etc.) are NOT replaced.
Keep those as `lipgloss.Color(coffee.Color)`.

---

## Theme Colors (what BasicTheme sets)

```go
background = AdaptiveColor{Dark: "#000000", Light: "#FBFCFD"}
border     = AdaptiveColor{Dark: "#666666", Light: "#D7DBDF"}
body       = Color("#AAAAAA")
dim        = Color("#666666")
accent     = AdaptiveColor{Dark: "#FFFFFF", Light: "#11181C"}
highlight  = Color("#4682B4")
brand      = Color("#4682B4")   // same as highlight for this project
label      = Color("#999999")
success    = Color("#00FF00")
error      = Color("196")
```

---

## How the Renderer Works

The reference implementation creates a `*lipgloss.Renderer` **per SSH session**:

```go
// main.go — SSH handler
renderer := bubbletea.MakeRenderer(s)   // s = ssh.Session
```

This lets lipgloss detect the correct color profile for each client's terminal
(e.g. TrueColor for Ghostty vs 256-color for basic terminals). All style creation
inside the theme uses `renderer.NewStyle()` instead of the global `lipgloss.NewStyle()`.

In this project:
- SSH sessions → renderer from `bubbletea.MakeRenderer(s)`
- Local / test mode → `lipgloss.DefaultRenderer()`

---

## Usage Pattern in Views

All view functions are methods on `Model`, so `m.theme` is always available.

**Text-only style (before → after):**
```go
// before
lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Width(18)

// after
m.theme.TextAccent().Bold(true).Width(18)
```

**Border foreground (before → after):**
```go
// before
lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#666666"))

// after
lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(m.theme.Border())
```

**Selected item background (before → after):**
```go
// before
lipgloss.NewStyle().Background(lipgloss.Color("#4682B4")).Foreground(lipgloss.Color("#FFFFFF"))

// after
lipgloss.NewStyle().Background(m.theme.Highlight()).Foreground(m.theme.Accent())
```

**`lipgloss.NewStyle()` stays for layout-only styles** (Width, Margin, Padding, Align
with no color). Only color-setting calls move to the theme.

---

## Huh Form Theme

`HuhTheme()` creates a `*huh.Theme` applied to shipping and payment forms.

Applied by chaining `.WithTheme(m.theme.Form())` on the `huh.NewForm(...)` call:

```go
// shipping_form.go and payment_form.go — in buildShippingForm / buildPaymentForm
f := huh.NewForm(...).
    WithShowErrors(false).
    WithShowHelp(false).
    WithTheme(m.theme.Form())   // ← add this line
```

Form theme styles:
- Focused border: thick left border in accent color (`#FFFFFF`)
- Field title + body text: body color (`#AAAAAA`)
- Cursor: highlight color (`#4682B4`)
- Placeholder text: dim color (`#666666`)
- Error: color `"196"`
- Blurred fields: hidden border (same styles, border invisible)

---

## Constructor Changes (model.go)

**Before:** `NewModel(username string)` initializes everything.
`NewModelWithAuth(fingerprint, pubKey, apiURL, secret, stripeKey)` calls NewModel.

**After:** A private `newModelWithRenderer(username, renderer)` does the heavy init.
Both public constructors call it:

```go
func NewModel(username string) Model {
    return newModelWithRenderer(username, lipgloss.DefaultRenderer())
}

func NewModelWithAuth(renderer *lipgloss.Renderer, fingerprint, pubKey, apiURL, secret, stripeKey string) Model {
    m := newModelWithRenderer("", renderer)
    m.Fingerprint = fingerprint
    // ... etc
    return m
}
```

`NewModelWithAuth` signature change: **renderer moves to first parameter**.
Update the call in `main.go` accordingly.

---

## Theme Methods Quick Reference

```go
// Color accessors (lipgloss.TerminalColor — use in .Foreground() / .Background())
m.theme.Accent()      // #FFFFFF / #11181C adaptive
m.theme.Body()        // #AAAAAA
m.theme.Dim()         // #666666
m.theme.Border()      // #666666 / #D7DBDF adaptive
m.theme.Highlight()   // #4682B4
m.theme.Brand()       // #4682B4
m.theme.Label()       // #999999
m.theme.Success()     // #00FF00
m.theme.Error()       // "196"
m.theme.Background()  // #000000 / #FBFCFD adaptive

// Style helpers (lipgloss.Style — chain .Bold(), .Width(), .Padding() etc.)
m.theme.Base()           // base style, body foreground
m.theme.TextAccent()     // white text
m.theme.TextBody()       // gray text (#AAAAAA)
m.theme.TextDim()        // dim text (#666666)
m.theme.TextHighlight()  // steel blue text (#4682B4)
m.theme.TextBrand()      // same as TextHighlight
m.theme.TextLabel()      // label text (#999999)
m.theme.TextSuccess()    // green text (#00FF00)
m.theme.TextError()      // error text ("196")
m.theme.TextLoading()    // loading pink text ("205")
m.theme.PanelError()     // error banner: dark red bg + red fg
m.theme.Form()           // *huh.Theme for form styling
```
