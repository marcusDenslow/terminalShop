package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"terminalShop/pkg/models"
)

const (
	// shopBodyInnerHeight fixes the inner height of the shop body panels so
	// the detail box never resizes with the description.
	shopBodyInnerHeight = 13
	// shopDescLines pins the description region to an exact line count.
	shopDescLines = 3
)

// shopLayout returns the shared chrome widths, all derived from one content
// width so the header, body panels, and footer line up exactly.
// Math ported from the design prototype at ~/programming/test/shop.go:
// menu(menuW+4) + gap(1) + detail(detailW+4) = contentW = wideW + 4.
func (m Model) shopLayout() (contentW, menuW, detailW, wideW int) {
	contentW = m.widthContent
	if contentW > 74 {
		contentW = 74
	}
	wideW = contentW - 4
	if m.size < large {
		// Stacked layout: both panels span the full content width.
		return contentW, wideW, wideW, wideW
	}
	menuW = 16
	detailW = contentW - 25
	return contentW, menuW, detailW, wideW
}

// roastLevel maps a roast type to a 1-5 intensity for the roast meter.
func roastLevel(roastType string) int {
	rt := strings.ToLower(roastType)
	switch {
	case strings.Contains(rt, "dark"):
		return 5
	case strings.Contains(rt, "light"):
		return 2
	default:
		return 3
	}
}

// clampLines pads or truncates rendered text to exactly n lines so regions
// like the description never change the panel height.
func clampLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	for len(lines) < n {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// roastMeter renders the strength dots, e.g. ● ● ● ○ ○
func roastMeter(n int) string {
	on := lipgloss.NewStyle().Foreground(cGold).Render(strings.Repeat("● ", n))
	off := pDim.Render(strings.Repeat("○ ", 5-n))
	return strings.TrimRight(on+off, " ")
}

func (m Model) shopMenuContent(w int) string {
	var b strings.Builder
	for i, c := range m.Coffees {
		if i == m.shop.selected {
			b.WriteString(lipgloss.NewStyle().Width(w).
				Background(lipgloss.Color(c.Color)).
				Foreground(contrastColor(c.Color)).
				Bold(true).
				Render(" · " + c.Name))
		} else {
			dot := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Color)).Render("·")
			b.WriteString(lipgloss.NewStyle().Width(w).
				Render(" " + dot + " " + pBody.Render(c.Name)))
		}
		if i < len(m.Coffees)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m Model) shopDetailContent(w int) string {
	coffee := m.Coffees[m.shop.selected]
	quantity := 0
	if item, exists := m.Cart[coffee.ID]; exists {
		quantity = item.Quantity
	}

	name := pWht.Bold(true).Render(coffee.Name)

	sep := pDim.Render(" · ")
	spec := pBody.Render(strings.ToUpper(coffee.RoastType)) + sep +
		pDim.Render(fmt.Sprintf("%dOZ", coffee.Ounces)) + sep +
		pDim.Render(strings.ToUpper(coffee.BeanType))

	strength := pDim.Render("roast  ") + roastMeter(roastLevel(coffee.RoastType))

	price := lipgloss.NewStyle().Foreground(lipgloss.Color(coffee.Color)).Bold(true).
		Render(fmt.Sprintf("$%d.%02d", coffee.Price/100, coffee.Price%100)) +
		pDim.Render("  usd")

	// Stock is not modeled in the backend yet, so the indicator is static.
	stock := lipgloss.NewStyle().Foreground(cGreen).Render("● in stock")

	desc := clampLines(pBody.Width(w).Render(coffee.Description), shopDescLines)

	// stepper: git-diff coloring — red minus, green plus
	minus := lipgloss.NewStyle().Foreground(cRed).Bold(true).Render("−")
	plus := lipgloss.NewStyle().Foreground(cGreen).Bold(true).Render("+")
	count := lipgloss.NewStyle().Foreground(cWhite).Bold(true).
		Padding(0, 2).Render(fmt.Sprintf("%d", quantity))
	stepper := pDim.Render("qty  ") + minus + count + plus

	return lipgloss.JoinVertical(lipgloss.Left,
		name,
		spec,
		strength,
		"",
		price+"    "+stock,
		"",
		desc,
		"",
		stepper,
	)
}

func (m Model) ShopView() string {
	if len(m.Coffees) == 0 {
		return lipgloss.Place(m.widthContent, m.heightContainer,
			lipgloss.Center, lipgloss.Center, "No products available.")
	}

	_, menuW, detailW, _ := m.shopLayout()

	mc := m.shopMenuContent(menuW)
	dc := m.shopDetailContent(detailW)

	var body string
	if m.size < large {
		body = lipgloss.JoinVertical(lipgloss.Left,
			panel(menuW, 0, mc),
			panel(detailW, 0, dc),
		)
	} else {
		bodyH := max(shopBodyInnerHeight, lipgloss.Height(mc), lipgloss.Height(dc))
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			panel(menuW, bodyH, mc),
			" ",
			panel(detailW, bodyH, dc),
		)
	}

	// Blank rows separate the body from the header and footer boxes.
	return "\n" + body + "\n"
}

func (m Model) ShopUpdate(msg tea.Msg) (Model, tea.Cmd) {
	if len(m.Coffees) == 0 {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		coffee := m.Coffees[m.shop.selected]
		switch msg.String() {
		case "up", "k":
			m = m.shopMoveSelected(true)
		case "down", "j":
			m = m.shopMoveSelected(false)
		case "+", "=":
			coffeeID := coffee.ID
			if item, exists := m.Cart[coffeeID]; exists {
				item.Quantity++
			} else {
				m.Cart[coffeeID] = &models.CartItem{CoffeeID: coffeeID, Coffee: coffee, Quantity: 1}
			}
			m, cmd := m.syncCartItemCmd(coffeeID, m.Cart[coffeeID].Quantity)
			return m, cmd
		case "-", "_":
			coffeeID := coffee.ID
			if item, exists := m.Cart[coffeeID]; exists {
				item.Quantity--
				newQty := item.Quantity
				if item.Quantity <= 0 {
					delete(m.Cart, coffeeID)
					newQty = 0
				}
				m, cmd := m.syncCartItemCmd(coffeeID, newQty)
				return m, cmd
			}
		}
	}

	return m, nil
}

func (m Model) shopMoveSelected(previous bool) Model {
	next := m.shop.selected
	if previous {
		next--
	} else {
		next++
	}
	if next < 0 {
		next = 0
	}
	if next >= len(m.Coffees) {
		next = len(m.Coffees) - 1
	}
	m.shop.selected = next
	return m
}
