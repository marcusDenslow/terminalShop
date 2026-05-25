package tui

import (
	"fmt"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m Model) BuildMenuView() string {
	bold := m.theme.TextAccent().Bold(true)

	base := m.theme.TextBody()

	rowStyle := lipgloss.NewStyle().Padding(0, 1)
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

	logoStyle := m.theme.TextAccent().Bold(true)
	logo := logoStyle.Render("terminal coffee")

	modal := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(m.theme.Border()).Padding(1, 2).Render(menuContent)

	hint := base.Render("press esc to close")

	total := m.CalculateSubtotal()
	itemCount := m.CartItemCount()

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

func (m Model) MenuUpdate(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "s":
		m.ShowingMenu = false
		m, cmd := m.ShopSwitch()
		return m, cmd
	case "a":
		m.ShowingMenu = false
		return m.AccountSwitch()
	case "c":
		m.ShowingMenu = false
		m, cmd := m.CartSwitch()
		return m, cmd
	case "esc":
		m.ShowingMenu = false
	case "?":
		m.ShowingMenu = false
		m.ShowingHelp = true
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}
