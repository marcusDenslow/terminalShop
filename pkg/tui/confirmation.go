package tui

import (
	"fmt"

	"terminalShop/pkg/models"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
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
	total := 0
	items := m.GetCartItemsSlice()
	for _, item := range items {
		line := fmt.Sprintf("  %s x%d  $%.2f", item.Coffee.Name, item.Quantity, float64(item.Coffee.Price * item.Quantity)/100) 
		cartSection += sectionStyle.Render(line) + "\n"
		total += item.Coffee.Price * item.Quantity
	}
	cartSection += sectionStyle.Render(
		labelStyle.Render("Total: ") + valueStyle.Render(fmt.Sprintf("$%.2f", float64(total)/100)),
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

func (m Model) ConfirmUpdate(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if keyMsg.String() != "esc" {
		return m, nil
	}

	m = m.SwitchPage(shopPage)
	m.ShippingInfo = nil
	m.CheckingOut = false
	m.Cart = make(map[uint]*models.CartItem)
	m.CartCursor = 0 
	m = m.resetPageState()
	return m, func() tea.Msg {
		if m.APIClient != nil {
			_ = m.APIClient.ClearCart()
		}
		return nil
	}
}
