package tui

import (
	"fmt"
	
	"terminalShop/pkg/models"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/bubbletea"
)

func (m Model) BuildShopView() string {
	leftWidth := 18
	rightWidth := m.widthContent - leftWidth - 2 // 2 for gap

	// On small/medium, items are centered and span the full container width.
	// On large, items are left-aligned in the narrow left column.
	itemWidth := leftWidth - 2
	itemAlign := lipgloss.Left
	if m.size < large {
		itemWidth = m.widthContainer
		itemAlign = lipgloss.Center
	}

	// Build the left panel (coffee list)
	leftPanel := ""
	for i, coffee := range m.Coffees {
		if m.Cursor == i {
			style := lipgloss.NewStyle().
				Background(lipgloss.Color(coffee.Color)).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1).
				Width(itemWidth).
				Align(itemAlign)
			leftPanel += style.Render(coffee.Name) + "\n"
		} else {
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#AAAAAA")).
				Padding(0, 1).
				Width(itemWidth).
				Align(itemAlign)
			leftPanel += style.Render(coffee.Name) + "\n"
		}
	}

	var leftContainer lipgloss.Style
	if m.size < large {
		leftContainer = lipgloss.NewStyle().Width(m.widthContainer)
	} else {
		leftContainer = lipgloss.NewStyle().Width(leftWidth)
	}

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

		// On small terminals, use full content width for description
		descWidth := rightWidth - 2
		if m.size < large {
			descWidth = m.widthContent - 2
		}

		descStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Width(descWidth)

		quantityStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)

		// Get current quantity (look up by CoffeeID)
		quantity := 0
		if item, exists := m.Cart[selectedCoffee.ID]; exists {
			quantity = item.Quantity
		}

		detailView += nameStyle.Render(selectedCoffee.Name) + "\n"
		detailView += infoStyle.Render(fmt.Sprintf("%s | %doz | %s",
			selectedCoffee.RoastType,
			selectedCoffee.Ounces,
			selectedCoffee.BeanType)) + "\n\n"
		detailView += priceStyle.Render(fmt.Sprintf("$%.2f", float64(selectedCoffee.Price)/100)) + "\n\n"
		detailView += descStyle.Render(selectedCoffee.Description) + "\n\n"
		detailView += quantityStyle.Render(fmt.Sprintf("-  %d  +", quantity)) + "\n"
	}

	detailContainer := lipgloss.NewStyle().
		Width(rightWidth)

	// Responsive layout: side-by-side on large, stacked on small/medium
	if m.size < large {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			leftContainer.Render(leftPanel),
			detailContainer.Width(m.widthContainer).Render(detailView),
		)
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftContainer.Render(leftPanel),
		"  ",
		detailContainer.Render(detailView),
	)
}

func (m Model) ShopUpdate(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		if m.Cursor < len(m.Coffees)-1 {
			m.Cursor++
		}
	case "+", "=":
		coffeeID := m.Coffees[m.Cursor].ID
		if item, exists := m.Cart[coffeeID]; exists {
			item.Quantity++
		} else {
			m.Cart[coffeeID] = &models.CartItem{
				CoffeeID: coffeeID,
				Coffee: m.Coffees[m.Cursor],
				Quantity: 1,
			}
		}
		return m, m.syncCartItemCmd(coffeeID, m.Cart[coffeeID].Quantity)
	case "-", "_":
		coffeeID := m.Coffees[m.Cursor].ID
		if item, exists := m.Cart[coffeeID]; exists {
			item.Quantity--
			newQty := item.Quantity
			if item.Quantity <= 0 {
				delete(m.Cart, coffeeID)
				if len(m.Cart) == 0 {
					m.CardCursor = 0
				} else if m.CartCursor >= len(m.Cart) {
					m.CartCursor = len(m.Cart) - 1
				}
				newQty = 0
			}
			return m, m.syncCartItemCmd(coffeeID, newQty)
		}
	}
	return m, nil
}
