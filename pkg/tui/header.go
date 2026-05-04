package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

func (m Model) BuildHeader() string {
	bold := m.theme.TextAccent().Bold(true).Render
	accent := m.theme.TextAccent().Render
	base := m.theme.Base().Render
	cursor := m.theme.Base().Background(m.theme.Brand()).Render(" ")

	mark := bold("t") + cursor
	logo := bold("terminal")
	menu := bold("m") + base(" ☰")

	tab := func(key, name string, active bool) string {
		if active {
			return accent(key + " " + name)
		}
		return accent(key) + base(" "+name)
	}
	shop := tab("s", "shop", m.currentPage == shopPage)
	account := tab("a", "account", m.currentPage == accountPage)

	priceStr := fmt.Sprintf(" $%2v", m.CalculateSubtotal()/100)
	countStr := fmt.Sprintf(" [%d]", m.CartItemCount())
	cartLabel := base(" cart")
	if m.inCartFlow() {
		cartLabel = accent(" cart")
	}
	cart := accent("c") + cartLabel + accent(priceStr) + base(countStr)

	var tabs []string
	switch m.size {
	case small:
		tabs = []string{mark, cart}
	case medium:
		tabs = []string{menu, logo, cart}
	default:
		tabs = []string{logo, shop, account, cart}
	}

	tbl := table.New().Border(lipgloss.NormalBorder()).BorderStyle(m.renderer.NewStyle().Foreground(m.theme.Border())).Row(tabs...).Width(m.widthContent).StyleFunc(func(row, col int) lipgloss.Style {
		return m.theme.Base().Padding(0, 1).AlignHorizontal(lipgloss.Center)
	}).Render()

	return lipgloss.Place(
		m.widthContainer,
		lipgloss.Height(tbl),
		lipgloss.Center,
		lipgloss.Center,
		tbl,
	)
}
