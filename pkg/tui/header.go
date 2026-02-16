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
	whiteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	boldWhiteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)

	// Build shop tab
	var shopKeybind, shopName string
	if !m.ViewingCart && !m.ViewingAccount {
		// Active: bold white keybind and name
		shopKeybind = boldWhiteStyle.Render("s")
		shopName = boldWhiteStyle.Render("shop")
	} else {
		// Inactive: bold gray keybind, gray name
		shopKeybind = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).Bold(true).Render("s")
		shopName = grayStyle.Render("shop")
	}

	// Build cart tab
	var cartKeybind, cartName, cartInfo string
	itemCountText := grayStyle.Render(fmt.Sprintf("[%d]", itemCount))

	if m.ViewingCart {
		// Active: bold white keybind and name
		cartKeybind = boldWhiteStyle.Render("c")
		cartName = boldWhiteStyle.Render("cart")
		cartInfo = fmt.Sprintf(" %s %s",
			boldWhiteStyle.Render(fmt.Sprintf("$%.2f", total)),
			itemCountText)
	} else {
		// Inactive: bold gray keybind, gray name
		boldGrayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).Bold(true)
		boldWhitePriceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
		cartKeybind = boldGrayStyle.Render("c")
		cartName = grayStyle.Render("cart")
		cartInfo = fmt.Sprintf(" %s %s",
			boldWhitePriceStyle.Render(fmt.Sprintf("$%.2f", total)),
			itemCountText)
	}

	// Build Account Tabs
	var accountKeybind, accountName string
	if m.ViewingAccount {
		accountKeybind = boldWhiteStyle.Render("a")
		accountName = boldWhiteStyle.Render("account")
	} else {
		accountKeybind = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).Bold(true).Render("a")
		accountName = grayStyle.Render("account")
	}

	// Build the tabs with separator
	separator := whiteStyle.Render("|")
	tabsContent := fmt.Sprintf("%s %s %s %s %s %s %s %s%s",
		shopKeybind,
		shopName,
		separator,
		accountKeybind,
		accountName,
		separator,
		cartKeybind,
		cartName,
		cartInfo,
	)

	// Add box with padding (no vertical padding, just horizontal)
	tabBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0, 2).
		MarginBottom(1)

	// Center the whole thing
	centered := lipgloss.NewStyle().
		Width(m.WindowWidth).
		Align(lipgloss.Center)

	return centered.Render(tabBox.Render(tabsContent))
}
