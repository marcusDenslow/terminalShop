package tui

import (
	"github.com/charmbracelet/lipgloss"
)

type DelayCompleteMsg struct{}
type splashCursorTickMsg struct{}

func (m Model) SplashView() string {
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).Bold(true)

	var cursor string
	if m.splashCursor {
		cursor = lipgloss.NewStyle().Background(lipgloss.Color("#4682B4")).Render(" ")
	} else {
		cursor = " "
	}

	logo := accent.Render("terminal coffe") + cursor

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		logo,
	)

	return lipgloss.Place(
		m.viewportWidth, 
		m.viewportHeight,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}
