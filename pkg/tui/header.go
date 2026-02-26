package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) BuildHeader() string {
	// Calculate cart total and item count
	total := 0.0
	itemCount := 0
	for _, item := range m.Cart {
		total += float64(item.Quantity) * item.Coffee.Price
		itemCount += item.Quantity
	}

	// Define colors
	grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	boldWhiteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	boldGrayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).Bold(true)

	// Build cart tab (used in all sizes)
	itemCountText := grayStyle.Render(fmt.Sprintf("[%d]", itemCount))

	var cartKeybind, cartName, cartInfo string
	if m.ViewingCart {
		// Active: bold white keybind and name
		cartKeybind = boldWhiteStyle.Render("c")
		cartName = boldWhiteStyle.Render("cart")
		cartInfo = fmt.Sprintf(" %s %s",
			boldWhiteStyle.Render(fmt.Sprintf("$%.2f", total)),
			itemCountText)
	} else {
		// Inactive: bold gray keybind, gray name
		cartKeybind = boldGrayStyle.Render("c")
		cartName = grayStyle.Render("cart")
		cartInfo = fmt.Sprintf(" %s %s",
			boldWhiteStyle.Render(fmt.Sprintf("$%.2f", total)),
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
		separator := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render("|")
		tabsContent = menuTab + " " + separator + " " + logo + " " + separator + " " + cartTab
	default:
		separator := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render("|")

		var shopKeybind, shopName string
		if !m.ViewingCart && !m.ViewingAccount {
			shopKeybind = boldWhiteStyle.Render("s")
			shopName = boldWhiteStyle.Render("shop")
		} else {
			shopKeybind = boldGrayStyle.Render("s")
			shopName = grayStyle.Render("shop")
		}

		var accountKeybind, accountName string
		if m.ViewingAccount {
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
