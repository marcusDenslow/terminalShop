package tui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
)


func (m Model) BuildMenuView() string {
	bold := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)

	base := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))

	rowStyle := lipgloss.NewStyle().Padding(0,1)
	rows := []string{
		rowStyle.Render(bold.Render("s") + " " + base.Render("shop")),
		rowStyle.Render(bold.Render("a") + " " + base.Render("account")),
		rowStyle.Render(bold.Render("c") + " " + base.Render("cart")),
		rowStyle.Render(""),
		rowStyle.Render(bold.Render("q") + " " + base.Render("quit")),
	}


	menuContent := ""
	for _, row := range rows {
		menuContent += row + "\n"
	}


	logoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	logo := logoStyle.Render("terminal coffee")

	modal := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#666666")).Padding(1,2).Render(menuContent)

	hint := base.Render("press esc to close")

	itemCount := 0
	total := 0
	for _, item := range m.Cart {
		total += item.Quantity * item.Coffee.Price
		itemCount += item.Quantity
	}

	cartHint := base.Render(fmt.Sprintf("cart $%.2f [%d]", float64(total)/100, itemCount))

	assembled := lipgloss.JoinVertical(
		lipgloss.Center,
		logo,
		"",
		modal,
		"",
		hint,
		cartHint,
		)

	return lipgloss.Place(
		m.viewportWidth,
		m.viewportHeight,
		lipgloss.Center,
		lipgloss.Center,
		assembled,
		)
	
}
