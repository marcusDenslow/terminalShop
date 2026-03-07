package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// RenderConfirmation renders the order confirmation view
func (m Model) RenderConfirmation() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 0, 1, 0)

	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CCCCCC")).
		Padding(0, 0, 1, 2)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF"))

	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FF00")).
		Bold(true).
		Padding(1, 0)

	title := titleStyle.Render("Order Confirmation")

	// Shipping summary
	shippingSection := ""
	if m.ShippingInfo != nil {
		shippingSection = sectionStyle.Render(
			labelStyle.Render("Ship to: ") +
				valueStyle.Render(m.ShippingInfo.Name) + "\n" +
				labelStyle.Render("         ") +
				valueStyle.Render(m.ShippingInfo.Street) + "\n" +
				labelStyle.Render("         ") +
				valueStyle.Render(fmt.Sprintf("%s, %s %s, %s", m.ShippingInfo.City, m.ShippingInfo.State, m.ShippingInfo.Zip, m.ShippingInfo.Country)),
		)
	}

	// Cart summary
	cartSection := ""
	total := 0.0
	items := m.GetCartItemsSlice()
	for _, item := range items {
		line := fmt.Sprintf("  %s x%d  $%.2f", item.Coffee.Name, item.Quantity, item.Coffee.Price*float64(item.Quantity))
		cartSection += sectionStyle.Render(line) + "\n"
		total += item.Coffee.Price * float64(item.Quantity)
	}
	cartSection += sectionStyle.Render(
		labelStyle.Render("Total: ") + valueStyle.Render(fmt.Sprintf("$%.2f", total)),
	)

	success := successStyle.Render("Order placed successfully!")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		shippingSection,
		"",
		cartSection,
		"",
		success,
	)
}
