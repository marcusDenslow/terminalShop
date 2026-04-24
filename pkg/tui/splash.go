package tui

import (
	"fmt"
	"terminalShop/pkg/api"
	"terminalShop/pkg/models"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/lipgloss"
)

type DelayCompleteMsg struct{}
type splashCursorTickMsg struct{}

type SplashAuthMsg struct {
	Token string
	User  models.PublicUser
	Err   error
}

type ViewInitMsg struct {
	Data api.ViewInitData
	Err  error
}

func (m Model) splashViewInitCmd() tea.Msg {
	data, err := m.APIClient.GetViewInit()
	if err != nil {
		return ViewInitMsg{Err: fmt.Errorf("failed to load data: %w", err)}
	}
	return ViewInitMsg{Data: data}
}

func (m Model) SplashView() string {
	accent := m.theme.TextBody().Bold(true)

	var cursor string
	if m.splash.cursor {
		cursor = lipgloss.NewStyle().Background(m.theme.Highlight()).Render(" ")
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
