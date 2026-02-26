package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) BuildFooter() string {
	// Style for keybinds (bold)
	keybindStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Bold(true)

	// Style for descriptions (normal)
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666"))

	// On small terminals, only show the menu hint 
	if m.size == small {
		footerText := keybindStyle.Render("m") + " " + descStyle.Render("menu")
		return lipgloss.NewStyle().Align(lipgloss.Center).Width(m.widthContainer).Render(footerText)
	}

	var footerText string
	if m.ViewingAccount {
		footerText = fmt.Sprintf("%s %s    %s %s    %s %s    %s %s",
			keybindStyle.Render("j/k"),
			descStyle.Render("menu"),
			keybindStyle.Render("s"),
			descStyle.Render("shop"),
			keybindStyle.Render("c"),
			descStyle.Render("cart"),
			keybindStyle.Render("q"),
			descStyle.Render("quit"),
		)
	} else if m.ViewingCart && m.CheckoutStep == 3 {
		// Confirmation screen
		footerText = fmt.Sprintf("%s %s    %s %s",
			keybindStyle.Render("esc"),
			descStyle.Render("back to shop"),
			keybindStyle.Render("q"),
			descStyle.Render("quit"),
		)
	} else if m.ViewingCart && (m.CheckoutStep == 1 || m.CheckoutStep == 2) {
		// In shipping/payment form
		footerText = fmt.Sprintf("%s %s    %s %s",
			keybindStyle.Render("esc"),
			descStyle.Render("back"),
			keybindStyle.Render("q"),
			descStyle.Render("quit"),
		)
	} else if m.ViewingCart && m.CheckoutStep == 0 {
		// In cart view, show proceed option
		if len(m.Cart) > 0 {
			footerText = fmt.Sprintf("%s %s    %s %s    %s %s    %s %s    %s %s    %s %s    %s %s",
				keybindStyle.Render("j/k"),
				descStyle.Render("items"),
				keybindStyle.Render("+/-"),
				descStyle.Render("qty"),
				keybindStyle.Render("p/enter"),
				descStyle.Render("checkout"),
				keybindStyle.Render("s"),
				descStyle.Render("shop"),
				keybindStyle.Render("a"),
				descStyle.Render("account"),
				keybindStyle.Render("pgup/pgdn"),
				descStyle.Render("scroll"),
				keybindStyle.Render("q"),
				descStyle.Render("quit"),
			)
		} else {
			footerText = fmt.Sprintf("%s %s    %s %s    %s %s    %s %s    %s %s",
				keybindStyle.Render("j/k"),
				descStyle.Render("items"),
				keybindStyle.Render("+/-"),
				descStyle.Render("qty"),
				keybindStyle.Render("s"),
				descStyle.Render("shop"),
				keybindStyle.Render("a"),
				descStyle.Render("account"),
				keybindStyle.Render("q"),
				descStyle.Render("quit"),
			)
		}
	} else if m.ViewingCart {
		// Other checkout steps
		footerText = fmt.Sprintf("%s %s    %s %s    %s %s",
			keybindStyle.Render("esc"),
			descStyle.Render("back"),
			keybindStyle.Render("s"),
			descStyle.Render("shop"),
			keybindStyle.Render("q"),
			descStyle.Render("quit"),
		)
	} else {
		footerText = fmt.Sprintf("%s %s    %s %s    %s %s    %s %s    %s %s",
			keybindStyle.Render("j/k"),
			descStyle.Render("products"),
			keybindStyle.Render("+/-"),
			descStyle.Render("qty"),
			keybindStyle.Render("c"),
			descStyle.Render("cart"),
			keybindStyle.Render("a"),
			descStyle.Render("account"),
			keybindStyle.Render("q"),
			descStyle.Render("quit"),
		)
	}

	return lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(m.widthContainer).
		Render(footerText)
}
