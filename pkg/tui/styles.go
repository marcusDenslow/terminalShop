package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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

func (m Model) formatListItem(text string, focused bool) string {
	return m.formatListItemCustom(text, focused, m.widthContent, true)
}

func (m Model) formatListItemCustom(text string, focused bool, totalWidth int, showRadio bool) string {
	accent := m.theme.TextAccent().Render

	content := "     " + text
	hint := ""
	if focused {
		content = accent(" ☉   " + text)
		hint = accent("enter")
	}

	if !showRadio {
		content = text
	}

	padding := 6
	if !showRadio {
		padding = 2
	}

	var lines = strings.Split(content, "\n")
	var firstLine = lines[0]
	hintSpace := totalWidth - lipgloss.Width(hint) - lipgloss.Width(firstLine) - padding
	lines[0] = firstLine + m.theme.Base().Width(hintSpace).Render() + hint
	return lipgloss.JoinVertical(lipgloss.Left, lines...,)
}
