package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// BuildFooter renders the bottom chrome bar with the current page keybinds.
// Layout ported from the design prototype at ~/programming/test/shop.go.
func (m Model) BuildFooter() string {
	_, _, _, wideW := m.shopLayout()

	item := func(k, v string) string {
		return pBody.Render(k) + " " + pDim.Render(v)
	}

	var keys string
	if m.size == small {
		keys = item("m", "menu")
	} else {
		parts := make([]string, 0, len(m.footer))
		for _, cmd := range m.footer {
			parts = append(parts, item(cmd.key, cmd.value))
		}
		keys = strings.Join(parts, pDim.Render(" · "))
	}

	row := lipgloss.NewStyle().Width(wideW).Align(lipgloss.Center).Render(keys)
	return panel(wideW, 0, row)
}
