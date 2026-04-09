package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) BuildHeader() string {
	// Calculate cart total and item count
	total := 0
	itemCount := 0
	for _, item := range m.Cart {
		total += item.Quantity * item.Coffee.Price
		itemCount += item.Quantity
	}

	// Define colors
	grayStyle := m.theme.TextBody()
	boldWhiteStyle := m.theme.TextAccent().Bold(true)
	boldGrayStyle := m.theme.TextBody().Bold(true)

	// Build cart tab (used in all sizes)
	itemCountText := grayStyle.Render(fmt.Sprintf("[%d]", itemCount))

	var cartKeybind, cartName, cartInfo string
	if m.inCartFlow() {
		// Active: bold white keybind and name
		cartKeybind = boldWhiteStyle.Render("c")
		cartName = boldWhiteStyle.Render("cart")
		cartInfo = fmt.Sprintf(" %s %s",
			boldWhiteStyle.Render(fmt.Sprintf("$%.2f", float64(total)/100)),
			itemCountText)
	} else {
		// Inactive: bold gray keybind, gray name
		cartKeybind = boldGrayStyle.Render("c")
		cartName = grayStyle.Render("cart")
		cartInfo = fmt.Sprintf(" %s %s",
			boldWhiteStyle.Render(fmt.Sprintf("$%.2f", float64(total)/100)),
			itemCountText)
	}
	cartTab := cartKeybind + " " + cartName + cartInfo

	var tabsContent string

	switch m.size {
	case small:
		// Small: just a mark + Cart
		mark := boldWhiteStyle.Render("t")
		tabsContent = mark + "    " + cartTab
	case medium:
		menuTab := boldGrayStyle.Render("m") + " " + grayStyle.Render("menu")
		logo := boldWhiteStyle.Render("terminal coffee")
		separator := m.theme.TextAccent().Render("|")
		tabsContent = menuTab + " " + separator + " " + logo + " " + separator + " " + cartTab
	default:
		separator := m.theme.TextAccent().Render("|")

		var shopKeybind, shopName string
		if m.currentPage == shopPage {
			shopKeybind = boldWhiteStyle.Render("s")
			shopName = boldWhiteStyle.Render("shop")
		} else {
			shopKeybind = boldGrayStyle.Render("s")
			shopName = grayStyle.Render("shop")
		}

		var accountKeybind, accountName string
		if m.currentPage == accountPage {
			accountKeybind = boldWhiteStyle.Render("a")
			accountName = boldWhiteStyle.Render("account")
		} else {
			accountKeybind = boldGrayStyle.Render("a")
			accountName = grayStyle.Render("account")
		}

		tabsContent = fmt.Sprintf("%s %s %s %s %s %s %s", shopKeybind, shopName, separator, accountKeybind, accountName, separator, cartTab)
	}

	// Add box with padding (no vertical padding, just horizontal)
	tabBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0, 2).
		MarginBottom(1)

	// Center the whole thing within the container
	centered := lipgloss.NewStyle().
		Width(m.widthContainer).
		Align(lipgloss.Center)

	return centered.Render(tabBox.Render(tabsContent))
}
