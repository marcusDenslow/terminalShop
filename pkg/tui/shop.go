package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) BuildShopView() string {
	leftWidth := 18 // Narrower list
	leftMargin := 4 // Minimal left margin
	rightWidth := m.WindowWidth - leftWidth - leftMargin - 4

	// Build the left panel (coffee list)
	leftPanel := ""
	for i, coffee := range m.Coffees {
        	// Apply background color when selected, white text
		if m.Cursor == i {
			style := lipgloss.NewStyle().
				Background(lipgloss.Color(coffee.Color)).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1).
				Width(leftWidth - 2).
				Align(lipgloss.Left)
			leftPanel += style.Render(coffee.Name) + "\n"
		} else {
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#AAAAAA")).
				Padding(0, 1).
				Width(leftWidth - 2).
				Align(lipgloss.Left)
			leftPanel += style.Render(coffee.Name) + "\n"
		}
	}

	// Add left margin to the list
	leftContainer := lipgloss.NewStyle().
		Width(leftWidth).
		MarginLeft(leftMargin)

	// Build the detail view (right panel)
	detailView := ""
	if m.Cursor >= 0 && m.Cursor < len(m.Coffees) {
		selectedCoffee := m.Coffees[m.Cursor]

		nameStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))

		infoStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA"))

		priceStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(selectedCoffee.Color)).
			Bold(true)

		descStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Width(rightWidth - 4)

		quantityStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)

		// Get current quantity
		quantity := 0
		if item, exists := m.Cart[m.Cursor]; exists {
			quantity = item.Quantity
		}

		detailView += nameStyle.Render(selectedCoffee.Name) + "\n"
		detailView += infoStyle.Render(fmt.Sprintf("%s | %doz | %s",
			selectedCoffee.RoastType,
			selectedCoffee.Ounces,
			selectedCoffee.BeanType)) + "\n\n"
		detailView += priceStyle.Render(fmt.Sprintf("$%.2f", selectedCoffee.Price)) + "\n\n"
		detailView += descStyle.Render(selectedCoffee.Description) + "\n\n"
		detailView += quantityStyle.Render(fmt.Sprintf("-  %d  +", quantity)) + "\n"
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
