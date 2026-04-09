package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) BuildHelpView() string {
	bold := m.theme.TextAccent().Bold(true)
	base := m.theme.TextBody()
	header := m.theme.TextHighlight().Bold(true)
	rowStyle := lipgloss.NewStyle().Padding(0, 1)

	sections := []struct {
		title string
		keys  []struct{ key, desc string }
	}{
		{
			title: "navigation",
			keys: []struct{ key, desc string }{
				{"s", "shop"},
				{"c", "cart"},
				{"a", "account"},
				{"m", "menu"},
			},
		},
		{
			title: "browsing",
			keys: []struct{ key, desc string }{
				{"j/k", "move cursor down/up"},
				{"+/-", "adjust quantity"},
				{"pgup/pgdn", "scroll content"},
			},
		},
		{
			title: "cart & checkout",
			keys: []struct{ key, desc string }{
				{"p/enter", "proceed to checkout"},
				{"d/x", "delete address or card"},
				{"esc", "go back one step"},
			},
		},
		{
			title: "account",
			keys: []struct{ key, desc string }{
				{"enter", "select / drill in"},
				{"esc", "go back"},
			},
		},
		{
			title: "forms",
			keys: []struct{ key, desc string }{
				{"tab", "next field"},
				{"enter", "submit form"},
				{"esc", "cancel"},
			},
		},
		{
			title: "general",
			keys: []struct{ key, desc string }{
				{"?", "toggle help page"},
				{"q", "quit"},
				{"ctrl+c", "force quit"},
			},
		},
	}

	content := ""
	for i, section := range sections {
		if i > 0 {
			content += "\n"
		}
		content += rowStyle.Render(header.Render(section.title)) + "\n"
		for _, k := range section.keys {
			content += rowStyle.Render(bold.Render(k.key)+"  "+base.Render(k.desc)) + "\n"
		}
	}

	logoStyle := m.theme.TextAccent().Bold(true)
	logo := logoStyle.Render("keybindings")

	modal := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.theme.Border()).
		Padding(1, 2).
		Render(content)

	hint := base.Render("press esc or ? to close")

	assembled := lipgloss.JoinVertical(
		lipgloss.Center,
		logo,
		"",
		modal,
		"",
		hint,
	)

	return lipgloss.Place(
		m.viewportWidth,
		m.viewportHeight,
		lipgloss.Center,
		lipgloss.Center,
		assembled,
	)
}

func (m Model) HelpUpdate(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "esc", "?":
		m.ShowingHelp = false
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}
