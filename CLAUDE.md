# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Terminal Coffee Shop is a terminal-based e-commerce application for ordering coffee, built with Go and the Bubbletea TUI framework. It's designed to eventually run as an SSH server similar to terminal.shop, with a backend API, PostgreSQL database, and Stripe payment integration.

**Current Status**: Phase 1 (Enhanced UI & Navigation) - The TUI is functional with shop, cart, and account views. Backend API has not been implemented yet.

## Essential Reading

Before making any changes, read these files in order:
1. **README.md** - Complete roadmap and current phase status
2. **MODELS.md** - Technical specifications, architecture, code standards, error handling patterns
3. **CONTRIBUTING.md** - Workflow guidance for AI-assisted development

## Architecture

### Current Structure (TUI-only)

```
terminalShop/
├── main.go                    # SSH server entry point (Wish framework)
├── pkg/
│   ├── tui/                   # Bubbletea TUI components
│   │   ├── model.go          # Main model with state management
│   │   ├── views.go          # View rendering and layout
│   │   ├── shop.go           # Shop view (left: coffee list, right: details)
│   │   ├── cart.go           # Cart view with scrolling
│   │   ├── account.go        # Account view (auto-updating right panel)
│   │   ├── header.go         # Tab navigation header
│   │   ├── footer.go         # Keybind hints
│   │   └── breadcrumbs.go    # Checkout flow breadcrumbs (cart only)
│   └── models/
│       └── coffee.go         # Coffee and CartItem models
└── .ssh/                     # SSH host keys
```

### Planned Structure (Backend)

The `api/` directory structure from MODELS.md will be created in Phase 2 with:
- Chi router for HTTP endpoints
- GORM models for PostgreSQL
- JWT authentication middleware
- Stripe payment integration

## Key Architectural Patterns

### TUI State Management (Bubbletea/Elm Architecture)

The `Model` struct in `pkg/tui/model.go` contains all application state:
- Navigation state: `ViewingCart`, `ViewingAccount`, `CheckoutStep`
- Cursor positions: `Cursor` (shop), `CartCursor` (cart), `AccountCursor` (account)
- Data: `Coffees` (product list), `Cart` (map[int]*CartItem)
- Scroll: `ScrollOffset` for handling overflow

**Critical Pattern**: All views (shop, cart, account) follow a **left-panel + right-panel layout**:
- Left panel: List of items with cursor navigation (j/k)
- Right panel: Details that auto-update based on cursor position
- No Enter/ESC navigation - hovering updates the right panel immediately

### View Rendering Flow

1. `views.go:View()` - Main entry point, handles vertical centering and scrolling
2. Calls view builders: `BuildShopView()`, `BuildCartView()`, `BuildAccountView()`
3. Each view uses `lipgloss.JoinHorizontal()` for left+right layout
4. Fixed content height calculated dynamically to prevent UI shifting

### Cart Management

**Important**: Cart uses `map[int]*models.CartItem` where the key is the coffee index in `m.Coffees`. This enables fast lookups but requires sorting in `GetCartItemsSlice()` because Go maps have randomized iteration order.

### Scrolling System

Implemented for cart view to handle long lists on small terminals:
- `ScrollOffset` tracks current scroll position (line-based, not item-based)
- Each cart item is ~5 lines tall (including borders)
- Auto-scrolls when navigating with j/k to keep selected item visible
- Manual scroll with pgup/pgdn

## Build and Run Commands

```bash
# Build and run SSH server
go build -o terminalshop && ./terminalshop
# Server listens on localhost:23457
# Connect with: ssh localhost -p 23457

# Quick run without building
go run main.go

# Add dependencies
go get <package>
go mod tidy
```

## Development Workflow

### Adding a New View Component

1. Create new file in `pkg/tui/` (e.g., `shipping.go`)
2. Add build function following the pattern: `func (m Model) BuildShippingView() string`
3. Use left-panel + right-panel layout with `lipgloss.JoinHorizontal()`
4. Update `views.go:View()` to conditionally render the new view
5. Add navigation state to `Model` struct in `model.go`
6. Add keybinds in `model.go:Update()`
7. Update `footer.go` with new keybind hints

### Modifying Existing Views

The shop, cart, and account views are split across multiple files but all follow the same pattern:
- Use `lipgloss.NewStyle()` for styling
- Fixed widths for panels (leftWidth typically 18, rightWidth calculated)
- Cursor-based selection with colored highlights
- Responsive to `WindowWidth` and `WindowHeight`

### Working with the Model

When adding state to the Model struct:
1. Add field to struct definition in `model.go`
2. Initialize in `NewModel()` constructor
3. Reset appropriately when changing views (see `case "s"`, `case "c"`, `case "a"`)
4. Update in `Update()` function based on user input

## Important Implementation Details

### Styling Conventions

- Active/selected: White (#FFFFFF), bold
- Inactive: Gray (#666666)
- Accent color: Steel blue (#4682B4) - used for cart breadcrumbs and highlights
- Coffee colors: Use individual coffee's `.Color` field (hex values like "#8B4513")

### Window Size Handling

`views.go` automatically adjusts content height based on terminal size:
- Calculates available space after header, breadcrumbs, footer, margins
- Minimum content height of 3 lines
- Margins scale down on small windows (< 25 lines height)
- Truncates content and enables scrolling when needed

### Breadcrumbs

Only visible in cart view, showing checkout flow: `cart / shipping / payment / confirmation`
- Current step (tracked in `CheckoutStep`) is white/bold
- Future steps are gray
- Will be used when implementing checkout pages in later phases

### SSH Server

`main.go` uses Wish (Charm's SSH framework) to serve the TUI over SSH:
- Host key in `.ssh/term_info_ed25519`
- Port 23457
- Each SSH session gets its own Bubbletea program instance
- Username passed to `tui.NewModel(username)`

## Common Gotchas

1. **Map iteration order**: Always sort cart items in `GetCartItemsSlice()` to prevent random reordering
2. **Scroll offset**: When adding scrolling to new views, update viewport height calculation (currently hardcoded as `-9` for cart with breadcrumbs)
3. **Content height**: Fixed height prevents UI shifting between pages but must account for all UI elements (header, breadcrumbs, footer, margins)
4. **Panel widths**: Left panel is 18 chars, right panel is `WindowWidth - leftWidth - leftMargin - 4`
5. **Reset scroll**: Always reset `ScrollOffset = 0` when switching views

## Code Style (from MODELS.md)

- Imports: stdlib, external deps, internal packages (separated by blank lines)
- Error messages: lowercase, no punctuation, use `%w` for wrapping
- Comments: Start with function/type name, complete sentences
- Struct tags: Always include `json` tags with snake_case
- Context: Always first parameter in functions

## Reference Implementation

`terminal-shop-source/` contains the full terminal.shop codebase (TypeScript/Go hybrid):
- `packages/go/pkg/tui/*.go` - TUI patterns and components
- `packages/go/cmd/ssh/main.go` - SSH server setup
- Study patterns, don't copy verbatim (different tech stack)

## Next Steps (from Roadmap)

Current phase is wrapping up. Next major phase:
- **Phase 2**: Backend API Foundation with Chi router, PostgreSQL, GORM models
- See MODELS.md for complete API specification and database schema
- See README.md Phase 2 tasks for implementation order

## Environment Setup

Copy `.env.example` to `.env` and configure:
- `DATABASE_URL` - PostgreSQL connection (for Phase 2)
- `JWT_SECRET` - Auth token secret (for Phase 3)
- `STRIPE_SECRET_KEY` / `STRIPE_PUBLIC_KEY` - Payment integration (for Phase 5)
- `SSH_PORT` - Default 2222 (currently using 23457)

## Testing

Currently no tests. Testing standards defined in MODELS.md for future API implementation:
- Unit tests for business logic
- Integration tests for API endpoints
- Use testify for assertions
- Table-driven test pattern
