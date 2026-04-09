package tui

import (
	"github.com/charmbracelet/lipgloss"
)

func (m Model) BuildBreadcrumbs() string {
	// Only show breadcrumbs in cart view
	if !m.inCartFlow() {
		return ""
	}

	activeStyle := m.theme.TextAccent().Bold(true)
	inactiveStyle := m.theme.TextDim()
	sepStyle := m.theme.TextDim()

	sep := sepStyle.Render(" / ")

	// Checkout flow steps
	var labels []string
	if m.size < large {
		labels = []string{"cart", "ship", "pay", "confirm"}
	} else {
		labels = []string{"cart", "shipping", "payment", "confirmation"}
	}

	// Get current checkout step from model
	currentStep := m.checkoutStep()

	var items []string
	for i, label := range labels {
		if i == currentStep {
			items = append(items, activeStyle.Render(label))
		} else {
			items = append(items, inactiveStyle.Render(label))
		}

		// Add separator except after last item
		if i < len(labels)-1 {
			items = append(items, sep)
		}
	}

	breadcrumbsContainer := lipgloss.NewStyle().
		MarginBottom(1).
		PaddingLeft(4)

	result := ""
	for _, item := range items {
		result += item
	}

	return breadcrumbsContainer.Render(result)
}
