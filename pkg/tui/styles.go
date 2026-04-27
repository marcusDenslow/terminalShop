package tui

import "github.com/charmbracelet/lipgloss"

func (m Model) createBoxInner(content string, selected bool, position lipgloss.Position, paddingH int, paddingV int, totalWidth int) string {
	padded := lipgloss.PlaceHorizontal(totalWidth, position, content)
	base := m.theme.Base().Border(lipgloss.NormalBorder()).Width(totalWidth)

	var style lipgloss.Style
	if selected {
		style = base.BorderForeground(m.theme.Highlight())
	} else {
		style = base.BorderForeground(m.theme.Border())
	}
	return style.Padding(paddingV, paddingH).Render(padded)
}

func (m Model) CreateBox(content string, selected bool) string {
	return m.createBoxInner(content, selected, lipgloss.Left, 1, 0, m.widthContent-2)
}

func (m Model) CreateBoxCustom(content string, selected bool, totalWidth int) string {
	return m.createBoxInner(content, selected, lipgloss.Left, 1, 0, totalWidth)
}
