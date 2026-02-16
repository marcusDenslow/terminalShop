# AGENTS.md

Guidance for AI coding agents working in this repository.

## Project Overview

Terminal Coffee Shop is a terminal-based e-commerce app for ordering coffee, built with Go and the Bubbletea TUI framework. It runs as an SSH server (Wish framework) with a REST API backend (Chi router), SQLite database (GORM), and JWT authentication.

## Build & Run Commands

```bash
# Build and run SSH server (TUI)
go build -o terminalshop && ./terminalshop
# Connect: ssh localhost -p 23457

# Quick run without building
go run main.go

# Run API server (separate process, port 8080)
go run main.go          # from api/ directory
```

## Test Commands

```bash
# Run all tests
go test ./...

# Run a single test by name
go test ./api/handlers/ -run TestGetHealth

# Run tests in a specific package
go test ./pkg/database/

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

## Project Structure

- `main.go` -- SSH server entry point (Wish framework)
- `api/` -- REST API server: `handlers/`, `middleware/`, `routes/`
- `pkg/api/` -- API client used by TUI
- `pkg/auth/` -- JWT + SSH key authentication
- `pkg/config/` -- Environment configuration
- `pkg/database/` -- SQLite connection, migrations, seeding
- `pkg/models/` -- GORM data models (Coffee, User, CartItem)
- `pkg/tui/` -- Bubbletea TUI components
- `pkg/utils/` -- Standard API response helpers
- `pkg/validate/` -- Generic validators; `pkg/validation/` -- request validation
- `terminal-shop-source/` -- Reference implementation (read-only, do not modify)

## Code Style

### Imports

Three groups separated by blank lines: (1) stdlib, (2) external packages, (3) internal packages.

```go
import (
    "context"
    "fmt"
    "net/http"

    "github.com/go-chi/chi/v5"
    "gorm.io/gorm"

    "terminalShop/pkg/database"
    "terminalShop/pkg/models"
)
```

### Naming Conventions

- **Types/exports**: `PascalCase` (`ProductHandler`, `JWTManager`)
- **Local variables**: `camelCase`
- **Constructors**: `New<Type>` (`NewModel`, `NewJWTManager`, `NewClient`)
- **Files**: `snake_case.go` (`ssh_auth.go`, `shipping_form.go`)
- **JSON tags**: `snake_case` (`json:"roast_type"`, `json:"created_at"`)
- **Error codes**: `PascalCase` constants (`ErrCodeValidation`)

### Error Handling

- Messages are lowercase, no trailing punctuation
- Wrap errors with `%w`: `fmt.Errorf("failed to create user: %w", err)`
- Use guard clauses with early returns in handlers
- API errors use: `utils.RespondError(w, statusCode, "ERROR_CODE", "message", details)`
- API success uses: `utils.RespondSuccess(w, statusCode, data)`

### Comments

- Doc comments start with the name of the thing: `// GenerateToken generates a new JWT token for a user`
- Complete sentences with proper punctuation
- Every exported type and function should have a doc comment

### Struct Tags

Always include `json` tags. Use GORM tags for database models.

```go
type Coffee struct {
    ID        uint    `gorm:"primaryKey" json:"id"`
    Name      string  `gorm:"size:255;not null" json:"name"`
    RoastType string  `gorm:"size:50;not null" json:"roast_type"`
    Price     float64 `gorm:"not null" json:"price"`
}
```

### Function Ordering in Files

1. Exported types and constants
2. Constructors (`New...`)
3. Public methods (alphabetically)
4. Private methods
5. Helpers

### Context

Always pass `context.Context` as the first parameter in functions that need it.

## Testing Patterns

- Use stdlib `testing` package with `t.Errorf`/`t.Fatalf` (no testify)
- HTTP tests use `httptest.NewRequest` and `httptest.NewRecorder`
- Test DB setup: create temp SQLite file, connect, migrate, seed, defer cleanup
- Always `defer os.Remove(testDB)` and `defer database.ResetForTesting()`
- Test functions named `Test<FunctionName>` or `Test<FunctionName><Scenario>`

## TUI Architecture (Bubbletea / Elm Architecture)

- All state lives in `Model` struct in `pkg/tui/model.go`
- Views follow **left-panel + right-panel** layout with `lipgloss.JoinHorizontal()`
- Right panel auto-updates based on cursor position (no Enter/ESC to view details)
- Left panel width is 18 chars; right panel fills remaining space
- Each view has a builder: `BuildShopView()`, `BuildCartView()`, `BuildAccountView()`
- Styling: active=#FFFFFF bold, inactive=#666666, accent=#4682B4
- Cart uses `map[int]*CartItem` -- always sort via `GetCartItemsSlice()` before display
- Reset `ScrollOffset = 0` when switching views

## Commit Messages

Use conventional commits: `feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `chore:`.

## Important Notes

- The `terminal-shop-source/` directory is a read-only reference -- do not modify it
- Environment config goes in `.env` (copy from `.env.example`); never commit `.env`
- Read CLAUDE.md for detailed architecture decisions and common gotchas
- Read MODELS.md for complete API specs, database schema, and design patterns
