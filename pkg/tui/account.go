package tui

import (
	"github.com/charmbracelet/lipgloss"

	"terminalShop/pkg/models"
)

func (m Model) BuildAccountView() string {
	leftWidth := 18
	leftMargin := 4
	rightWidth := m.WindowWidth - leftWidth - leftMargin - 4

	// Build the left panel (menu items)
	leftPanel := ""
	for i, item := range models.AccountMenuItems {
		// Apply background color when selected, white text
		if m.AccountCursor == i {
			style := lipgloss.NewStyle().
				Background(lipgloss.Color("#4682B4")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1).
				Width(leftWidth - 2).
				Align(lipgloss.Left)
			leftPanel += style.Render(item) + "\n"
		} else {
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#AAAAAA")).
				Padding(0, 1).
				Width(leftWidth - 2).
				Align(lipgloss.Left)
			leftPanel += style.Render(item) + "\n"
		}
	}

	// Add left margin to the list
	leftContainer := lipgloss.NewStyle().
		Width(leftWidth).
		MarginLeft(leftMargin)

	// Build the detail view (right panel) based on cursor position
	detailView := ""
	if m.AccountCursor >= 0 && m.AccountCursor < len(models.AccountMenuItems) {
		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			MarginBottom(2)

		contentStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")).
			Width(rightWidth - 4)

		selectedItem := models.AccountMenuItems[m.AccountCursor]
		switch selectedItem {
		case "order history":
			detailView = titleStyle.Render("Order History") + "\n\n" +
				contentStyle.Render("No orders yet. Your order history will appear here once you make a purchase.")
		case "faq":
			detailView = titleStyle.Render("FAQ") + "\n\n" +
				contentStyle.Render("Q: How do I place an order?\nA: Browse products with j/k, adjust quantity with +/-, and checkout via the cart.\n\nQ: What payment methods do you accept?\nA: We accept all major credit cards via Stripe.")
		case "about":
			detailView = titleStyle.Render("About Terminal Coffee Shop") + "\n\n" +
				contentStyle.Render("A terminal-based coffee ordering experience.\nBuilt with Go and Bubbletea.")
		}
	}

	detailContainer := lipgloss.NewStyle().
		Width(rightWidth)

	// Layout the panels side by side
	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftContainer.Render(leftPanel),
		detailContainer.Render(detailView),
	)

	return mainContent
}
