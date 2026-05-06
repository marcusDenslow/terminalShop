package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"terminalShop/pkg/models"
)

func (m Model) calculateShopWidths() (int, int) {
	menuWidth := 14
	if len(m.Coffees) == 0 {
		if m.size < large {
			return m.widthContent, m.widthContent
		}
		return menuWidth, m.widthContent - menuWidth
	}
	for _, c := range m.Coffees {
		if w := lipgloss.Width(c.Name); w > menuWidth {
			menuWidth = w
		}
	}
	if menuWidth > 0 {
		menuWidth += 4
	}
	if m.size < large {
		menuWidth = m.widthContent
	}
	detailWidth := m.widthContent - menuWidth
	if m.size < large {
		detailWidth = m.widthContent
	}
	return menuWidth, detailWidth
}

func (m Model) updateShopViewports() Model {
	headerHeight := lipgloss.Height(m.BuildHeader())
	footerHeight := lipgloss.Height(m.BuildFooter())
	availableHeight := m.heightContainer - headerHeight - footerHeight

	if len(m.Coffees) == 0 {
		return m
	}

	menuWidth, detailWidth := m.calculateShopWidths()

	if !m.shop.viewportsReady {
		m.shop.menuViewport = newViewport(menuWidth, availableHeight)
		m.shop.menuViewport.KeyMap = viewport.KeyMap{} // menu nav is manual
		m.shop.detailViewport = newViewport(detailWidth, availableHeight)
		m.shop.detailViewport.KeyMap = modifiedKeyMap
		m.shop.viewportsReady = true
	} else {
		m.shop.menuViewport.Width = menuWidth
		m.shop.menuViewport.Height = availableHeight
		m.shop.detailViewport.Width = detailWidth
		m.shop.detailViewport.Height = availableHeight
	}
	return m
}

func (m Model) getShopMenuContent() string {
	menuWidth, _ := m.calculateShopWidths()

	selectedColor := m.theme.Highlight()
	if m.shop.selected >= 0 && m.shop.selected < len(m.Coffees) {
		if c := m.Coffees[m.shop.selected].Color; c != "" {
			selectedColor = lipgloss.Color(c)
		}
	}

	var normal, highlighted lipgloss.Style
	if m.size < large {
		menuWidth = m.widthContent
		normal = m.theme.TextBody().
			Width(menuWidth - 1).Align(lipgloss.Center)
		highlighted = m.theme.TextAccent().
			Width(menuWidth - 1).Align(lipgloss.Center).
			Background(selectedColor).Bold(true)
	} else {
		normal = m.theme.TextBody().
			Width(menuWidth+2).Padding(0, 1)
		highlighted = m.theme.TextAccent().
			Width(menuWidth+2).Padding(0, 1).
			Background(selectedColor).Bold(true)
	}

	var b strings.Builder
	for i, c := range m.Coffees {
		if i == m.shop.selected {
			b.WriteString(highlighted.Render(c.Name) + "\n")
		} else {
			b.WriteString(normal.Render(c.Name) + "\n")
		}
	}
	return lipgloss.NewStyle().Padding(0, 1).Render(b.String())
}

func (m Model) getShopDetailContent() string {
	if len(m.Coffees) == 0 {
		return ""
	}
	coffee := m.Coffees[m.shop.selected]
	_, detailWidth := m.calculateShopWidths()
	if m.size >= large {
		detailWidth -= 2
	}

	quantity := 0
	if item, exists := m.Cart[coffee.ID]; exists {
		quantity = item.Quantity
	}

	detail := lipgloss.JoinVertical(lipgloss.Left,
		m.theme.TextAccent().Bold(true).Render(coffee.Name),
		m.theme.TextBody().Render(
			fmt.Sprintf("%s | %doz | %s", coffee.RoastType, coffee.Ounces, coffee.BeanType),
		),
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color(coffee.Color)).Bold(true).Render(
			fmt.Sprintf("$%.2f", float64(coffee.Price)/100),
		),
		"",
		m.theme.TextAccent().Width(detailWidth).Render(coffee.Description),
		"",
		fmt.Sprintf("-  %d  +", quantity),
	)
	return lipgloss.NewStyle().Width(detailWidth).Render(detail)
}

func (m Model) ShopView() string {
	if !m.shop.viewportsReady {
		m = m.updateShopViewports()
	}
	if len(m.Coffees) == 0 {
		return lipgloss.Place(m.widthContent, m.heightContainer,
			lipgloss.Center, lipgloss.Center, "No products available.")
	}

	m.shop.menuViewport.SetContent(m.getShopMenuContent())
	m.shop.detailViewport.SetContent(m.getShopDetailContent())

	if m.size < large {
		return lipgloss.JoinVertical(lipgloss.Top,
			m.shop.menuViewport.View(),
			m.shop.detailViewport.View(),
		)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.shop.menuViewport.View(),
		"  ",
		m.shop.detailViewport.View(),
	)
}

func (m Model) ShopUpdate(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if len(m.Coffees) == 0 {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		coffee := m.Coffees[m.shop.selected]
		switch msg.String() {
		case "up", "k":
			m = m.shopMoveSelected(true)
		case "down", "j":
			m = m.shopMoveSelected(false)
		case "+", "=":
			coffeeID := coffee.ID
			if item, exists := m.Cart[coffeeID]; exists {
				item.Quantity++
			} else {
				m.Cart[coffeeID] = &models.CartItem{CoffeeID: coffeeID, Coffee: coffee, Quantity: 1}
			}
			m, cmd := m.syncCartItemCmd(coffeeID, m.Cart[coffeeID].Quantity)
			return m, cmd
		case "-", "_":
			coffeeID := coffee.ID
			if item, exists := m.Cart[coffeeID]; exists {
				item.Quantity--
				newQty := item.Quantity
				if item.Quantity <= 0 {
					delete(m.Cart, coffeeID)
					newQty = 0
				}
				m, cmd := m.syncCartItemCmd(coffeeID, newQty)
				return m, cmd
			}
		}
	}

	// Pass messages to the detail viewport so pgup/pgdn scroll works
	m.shop.detailViewport, cmd = m.shop.detailViewport.Update(msg)
	cmds = append(cmds, cmd)

	if m.shop.viewportsReady {
		m.shop.menuViewport.SetContent(m.getShopMenuContent())
		m.shop.detailViewport.SetContent(m.getShopDetailContent())
	}

	return m, tea.Batch(cmds...)
}

func (m Model) shopMoveSelected(previous bool) Model {
	next := m.shop.selected
	if previous {
		next--
	} else {
		next++
	}
	if next < 0 {
		next = 0
	}
	if next >= len(m.Coffees) {
		next = len(m.Coffees) - 1
	}
	m.shop.selected = next

	if m.shop.viewportsReady {
		targetY := (m.shop.selected + 2) * 1
		m.shop.menuViewport.SetYOffset(targetY - m.shop.menuViewport.Height/2)
		m.shop.detailViewport.GotoTop()
	}
	return m
}
