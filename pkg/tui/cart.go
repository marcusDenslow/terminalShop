package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) BuildCartView() string {
	if len(m.Cart) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Align(lipgloss.Center).
			Width(m.widthContent).
			Padding(2, 0)
		return emptyStyle.Render("Your cart is empty\n\nPress s to go back to shop")
	}

	cartItems := ""
	boxWidth := m.widthContent - 4 // Leave some margin within the container
	itemSlice := m.GetCartItemsSlice()

	// Adjust padding based on container height
	boxPadding := 0
	if m.heightContainer >= 25 {
		boxPadding = 1
	}

	for idx, item := range itemSlice {
		itemTotal := item.Quantity * item.Coffee.Price

		// Left side: name and individual price
		nameStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))

		individualPriceStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(item.Coffee.Color)).
			Bold(true)

		infoStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA"))

		// Right side: quantity controls and total price
		quantityStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

		totalPriceStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)

		// Build content with proper spacing
		nameText := nameStyle.Render(item.Coffee.Name)
		individualPriceText := individualPriceStyle.Render(fmt.Sprintf(" $%.2f", float64(item.Coffee.Price)/100))
		infoText := infoStyle.Render(fmt.Sprintf("%s | %doz | %s",
			item.Coffee.RoastType,
			item.Coffee.Ounces,
			item.Coffee.BeanType))
		quantityText := quantityStyle.Render(fmt.Sprintf("-  %d  +", item.Quantity))
		totalPriceText := totalPriceStyle.Render(fmt.Sprintf("$%.2f", float64(itemTotal)/100))

		// Calculate spacing
		nameAndPriceWidth := lipgloss.Width(nameText) + lipgloss.Width(individualPriceText)
		quantityWidth := lipgloss.Width(quantityText)
		totalPriceWidth := lipgloss.Width(totalPriceText)
		spacing := boxWidth - nameAndPriceWidth - quantityWidth - totalPriceWidth - 8 // 8 for padding and gaps

		if spacing < 1 {
			spacing = 1
		}

		contentLine1 := nameText + individualPriceText +
			lipgloss.NewStyle().Width(spacing).Render("") +
			quantityText + "  " + totalPriceText

		contentLine2 := infoText

		boxContent := contentLine1 + "\n" + contentLine2

		// Create box for this item - highlight if it's the selected item
		var itemBox lipgloss.Style
		if idx == m.CartCursor {
			itemBox = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), true).
				BorderForeground(lipgloss.Color("#FFFFFF")).
				Padding(boxPadding, 2).
				Width(boxWidth).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true)
		} else {
			itemBox = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#666666")).
				Padding(boxPadding, 2).
				Width(boxWidth).
				Foreground(lipgloss.Color("#FFFFFF"))
		}

		// Center the box within the container
		centered := lipgloss.NewStyle().
			Width(m.widthContent).
			Align(lipgloss.Center)

		cartItems += centered.Render(itemBox.Render(boxContent))

		// Add spacing between items
		if idx < len(itemSlice)-1 {
			cartItems += "\n"
		}
	}

	return cartItems
}

func (m Model) CartUpdate(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "up", "k":
		if m.CartCursor > 0 {
			m.CartCursor--
			itemHeight := 5
			targetLine := m.CartCursor * itemHeight
			if targetLine < m.ScrollOffset {
				m.ScrollOffset = targetLine
			}
		}
	case "down", "j":
		if m.CartCursor < len(m.Cart)-1 {
			m.CartCursor++
			itemHeight := 5
			targetLineEnd := (m.CartCursor * itemHeight) + itemHeight
			viewportHeight := m.heightContainer - 9
			if viewportHeight < 6 {
				viewportHeight = 6
			}
			if targetLineEnd > m.ScrollOffset+viewportHeight {
				m.ScrollOffset = targetLineEnd - viewportHeight
			}
		}
	case "pgup", "ctrl+u":
		m.ScrollOffset -= 3
		if m.ScrollOffset < 0 {
			m.ScrollOffset = 0
		}
	case "pgdown", "ctrl+d":
		m.ScrollOffset += 3
	case "+", "=":
		cartItems := m.GetCartItemsSlice()
		if m.CartCursor >= 0 && m.CartCursor < len(cartItems) {
			cartItems[m.CardCursor].Quantity++
			return m, m.syncCartItemCmd(cartItems[m.CartCursor].CoffeeID, cartItems[m.CartCursor].Quantity)
		}
	case "-", "_":
		cartItems := m.GetCartItemsSlice()
		if m.CartCursor >= 0 && m.CartCursor < len(cartItems) {
			coffeeID := cartItems[m.CartCursor].CoffeeID
			cartItems[m.CartCursor].Quantity--
			newQty := cartItems[m.CartCursor].Quantity
			if cartItems[m.CartCursor].Quantity <= 0 {
				delete(m.Cart, coffeeID)
				if len(m.Cart) == 0 {
					m.CartCursor = 0
				} else if m.CartCursor >= len(m.Cart) {
					m.CartCursor = len(m.Cart) - 1
				}
				newQty = 0
			}
			return m, m.syncCartItemCmd(coffeeID, newQty)
		}
	case "p", "enter":
		if len(m.Cart) > 0 {
			m = m.SwitchPage(shippingPage)
			return m, m.fetchAddressesCmd()
		}
	}
	return m, nil
}
