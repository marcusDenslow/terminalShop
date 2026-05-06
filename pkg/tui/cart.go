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
	if !m.cart.viewportReady {
		m.cart.viewport = viewport.New(m.widthContent, availH)
		m.cart.viewport.KeyMap = viewport.KeyMap{}
		m.cart.viewportReady = true
	} else {
		m.cart.viewport.Width = m.widthContent
		m.cart.viewport.Height = availH
	}
	return m
}

func (m Model) generateCartContent() string {
	if m.IsCartEmpty() {
		emptyStyle := m.theme.TextDim().
			Align(lipgloss.Center).
			Width(m.widthContent).
			Padding(2, 0)
		render := emptyStyle.Render("Your cart is empty\n\nPress s to go back to shop")
		return render
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

		nameStyle := m.theme.TextAccent().Bold(true)

		individualPriceStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(item.Coffee.Color)).
			Bold(true)

		infoStyle := m.theme.TextBody()

		quantityStyle := m.theme.TextAccent()

		totalPriceStyle := m.theme.TextAccent().Bold(true)

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

		isSelected := idx == m.cart.cursor
		if isSelected {
			boxContent = m.theme.TextAccent().Bold(true).Render(boxContent)
		} else {
			boxContent = m.theme.TextAccent().Render(boxContent)
		}
		box := m.createBoxInner(boxContent, isSelected, lipgloss.Left, 2, boxPadding, boxWidth)
		cartItems += m.theme.Base().Width(m.widthContent).Align(lipgloss.Center).Render(box)

		if idx < len(itemSlice)-1 {
			cartItems += "\n"
		}
	}
	return cartItems
}

func (m Model) CartView() string {
	if !m.cart.viewportReady {
		m = m.updateCartViewport()
	}
	content := m.generateCartContent()
	m.cart.viewport.SetContent(content)
	if len(m.Cart) > 0 {
		boxPadding := 0
		if m.heightContainer >= 25 {
			boxPadding = 1
		}
		itemHeight := 4 + 2*boxPadding + 1
		targetY := m.cart.cursor * itemHeight
		if targetY < m.cart.viewport.YOffset {
			m.cart.viewport.SetYOffset(targetY)
		}
		if targetY+itemHeight > m.cart.viewport.YOffset+m.cart.viewport.Height {
			m.cart.viewport.SetYOffset(targetY - m.cart.viewport.Height + itemHeight)
		}
	}
	return lipgloss.Place(
		m.widthContainer,
		lipgloss.Height(m.cart.viewport.View()),
		lipgloss.Center, lipgloss.Center,
		m.cart.viewport.View(),
	)
}

func (m Model) CartUpdate(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "up", "k":
		if m.cart.cursor > 0 {
			m.cart.cursor--
		}
	case "down", "j":
		if m.cart.cursor < len(m.Cart)-1 {
			m.cart.cursor++
		}
	case "+", "=":
		cartItems := m.GetCartItemsSlice()
		if m.cart.cursor >= 0 && m.cart.cursor < len(cartItems) {
			cartItems[m.cart.cursor].Quantity++
			m, cmd := m.syncCartItemCmd(cartItems[m.cart.cursor].CoffeeID, cartItems[m.cart.cursor].Quantity)
			return m, cmd
		}
	case "-", "_":
		cartItems := m.GetCartItemsSlice()
		if m.cart.cursor >= 0 && m.cart.cursor < len(cartItems) {
			coffeeID := cartItems[m.cart.cursor].CoffeeID
			cartItems[m.cart.cursor].Quantity--
			newQty := cartItems[m.cart.cursor].Quantity
			if cartItems[m.cart.cursor].Quantity <= 0 {
				delete(m.Cart, coffeeID)
				if len(m.Cart) == 0 {
					m.cart.cursor = 0
				} else if m.cart.cursor >= len(m.Cart) {
					m.cart.cursor = len(m.Cart) - 1
				}
				newQty = 0
			}
			m, cmd := m.syncCartItemCmd(coffeeID, newQty)
			return m, cmd
		}
	case "p", "enter":
		if !m.IsCartEmpty() {
			return m.ShippingSwitch()
		}
	}
	return m, nil
}
