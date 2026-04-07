package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) updateCartViewport() Model {
	headerH := lipgloss.Height(m.BuildHeader())
	breadH := lipgloss.Height(m.BuildBreadcrumbs())
	footerH := lipgloss.Height(m.BuildFooter())
	availH := m.heightContainer - headerH - footerH - breadH
	if availH < 1 {
		availH = 1
	}
	if !m.cartVPReady {
		m.cartVP = viewport.New(m.widthContent, availH)
		m.cartVP.KeyMap = viewport.KeyMap{}
		m.cartVPReady = true
	} else {
		m.cartVP.Width = m.widthContent
		m.cartVP.Height = availH
	}
	return m
}

func (m Model) generateCartContent() string {
	if len(m.Cart) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Align(lipgloss.Center).
			Width(m.widthContent).
			Padding(2, 0)
		return emptyStyle.Render("Your cart is empty\n\nPress s to go back to shop")
	}

	cartItems := ""
	boxWidth := m.widthContent - 4
	itemSlice := m.GetCartItemsSlice()

	boxPadding := 0
	if m.heightContainer >= 25 {
		boxPadding = 1
	}

	for idx, item := range itemSlice {
		itemTotal := item.Quantity * item.Coffee.Price

		nameStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))

		individualPriceStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(item.Coffee.Color)).
			Bold(true)

		infoStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA"))

		quantityStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

		totalPriceStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)

		nameText := nameStyle.Render(item.Coffee.Name)
		individualPriceText := individualPriceStyle.Render(fmt.Sprintf(" $%.2f", float64(item.Coffee.Price)/100))
		infoText := infoStyle.Render(fmt.Sprintf("%s | %doz | %s",
			item.Coffee.RoastType,
			item.Coffee.Ounces,
			item.Coffee.BeanType))
		quantityText := quantityStyle.Render(fmt.Sprintf("-  %d  +", item.Quantity))
		totalPriceText := totalPriceStyle.Render(fmt.Sprintf("$%.2f", float64(itemTotal)/100))

		nameAndPriceWidth := lipgloss.Width(nameText) + lipgloss.Width(individualPriceText)
		quantityWidth := lipgloss.Width(quantityText)
		totalPriceWidth := lipgloss.Width(totalPriceText)
		spacing := boxWidth - nameAndPriceWidth - quantityWidth - totalPriceWidth - 8

		if spacing < 1 {
			spacing = 1
		}

		contentLine1 := nameText + individualPriceText +
			lipgloss.NewStyle().Width(spacing).Render("") +
			quantityText + "  " + totalPriceText

		contentLine2 := infoText

		boxContent := contentLine1 + "\n" + contentLine2

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

		centered := lipgloss.NewStyle().
			Width(m.widthContent).
			Align(lipgloss.Center)

		cartItems += centered.Render(itemBox.Render(boxContent))

		if idx < len(itemSlice)-1 {
			cartItems += "\n"
		}
	}
	return cartItems
}

func (m Model) CartView() string {
	if !m.cartVPReady {
		m = m.updateCartViewport()
	}
	content := m.generateCartContent()
	m.cartVP.SetContent(content)
	if len(m.Cart) > 0 {
		itemHeight := 5
		targetY := m.CartCursor * itemHeight
		if targetY < m.cartVP.YOffset {
			m.cartVP.SetYOffset(targetY)
		}
		if targetY+itemHeight > m.cartVP.YOffset+m.cartVP.Height {
			m.cartVP.SetYOffset(targetY - m.cartVP.Height + itemHeight)
		}
	}
	return lipgloss.Place(
		m.widthContainer,
		lipgloss.Height(m.cartVP.View()),
		lipgloss.Center, lipgloss.Center,
		m.cartVP.View(),
	)
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
		}
	case "down", "j":
		if m.CartCursor < len(m.Cart)-1 {
			m.CartCursor++
		}
	case "+", "=":
		cartItems := m.GetCartItemsSlice()
		if m.CartCursor >= 0 && m.CartCursor < len(cartItems) {
			cartItems[m.CartCursor].Quantity++
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
			m = m.updateShippingViewport()
			return m, m.fetchAddressesCmd()
		}
	}
	return m, nil
}
