package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) BuildFooter() string {
	keybindStyle := m.theme.TextDim().Bold(true)
	descStyle := m.theme.TextDim()

	if m.size == small {
		footerText := keybindStyle.Render("m") + " " + descStyle.Render("menu")
		return lipgloss.NewStyle().Align(lipgloss.Center).Width(m.widthContainer).Render(footerText)
	}

	parts := []string{}
	for _, cmd := range m.footer {
		parts = append(parts, keybindStyle.Render(cmd.key)+" "+descStyle.Render(cmd.value))
	}
	footerText := strings.Join(parts, "    ")

	return lipgloss.NewStyle().Align(lipgloss.Center).Width(m.widthContainer).Render(footerText)
}
