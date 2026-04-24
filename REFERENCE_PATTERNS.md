# Reference Patterns to Adopt

Patterns from `terminal-shop-source` that we're copying into this project.

---

## 1. Footer: Command Array Pattern

### Structs (in model.go or footer.go)
```go
type footerState struct {
    commands []footerCommand
}

type footerCommand struct {
    key   string
    value string
}
```

### Setting commands (in each *Switch method)
```go
func (m Model) CartSwitch() (Model, tea.Cmd) {
    m = m.SwitchPage(cartPage)
    m.footer.commands = []footerCommand{
        {key: "esc", value: "back"},
        {key: "↑/↓", value: "items"},
        {key: "+/-", value: "qty"},
        {key: "c", value: "checkout"},
    }
    m = m.updateCartViewport()
    return m, nil
}
```

### Rendering (footer.go)
```go
// Iterate commands instead of if/else chain
commands := []string{}
for _, cmd := range m.footer.commands {
    commands = append(commands, bold(" "+cmd.key+" ")+base(cmd.value+"  "))
}

// Error display replaces default content when m.ErrorMsg != ""
```

### Small screen: only show "m menu"
```go
if m.size == small {
    return table.Render(bold("m") + base(" menu"))
}
```

---

## 2. Helper Methods

### Cart helpers (add to model.go or cart.go)
```go
func (m Model) IsCartEmpty() bool {
    return len(m.Cart) == 0
}

func (m Model) CartItemCount() int {
    return len(m.Cart)
}

func (m Model) CalculateSubtotal() int {
    subtotal := 0
    for _, item := range m.Cart {
        subtotal += item.Coffee.Price * item.Quantity
    }
    return subtotal
}
```

### Used in reference for: header cart badge, footer conditional commands, checkout validation, review page totals.

---

## 3. Account View Split

### Reference splits into separate files per sub-view:
- `account.go` — parent: menu + delegation (~80 lines)
- `orders.go` — OrdersView() + OrdersUpdate()
- `subscriptions.go` — SubscriptionsView() + SubscriptionsUpdate()
- `tokens.go` — TokensView() + TokensUpdate()
- `apps.go` — AppsView() + AppsUpdate()
- `faq.go` — FaqView()
- `about.go` — AboutView() (in account.go, inline)

### Parent dispatches via:
```go
func (m model) GetAccountPageContent(accountPage page, width int) string {
    switch accountPage {
    case ordersPage:
        return m.OrdersView(width, m.state.account.focused)
    case addressesPage:
        return m.AddressesView(width, m.state.account.focused)
    // ...
    }
}
```

### Each sub-view owns its own state, cursor, and deletion logic.
