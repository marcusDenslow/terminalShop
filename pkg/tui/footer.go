package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) BuildFooter() string {
	// Style for keybinds (bold)
	keybindStyle := m.theme.TextDim().Bold(true)

	// Style for descriptions (normal)
	descStyle := m.theme.TextDim()

	// On small terminals, only show the menu hint
	if m.size == small {
		footerText := keybindStyle.Render("m") + " " + descStyle.Render("menu")
		return lipgloss.NewStyle().Align(lipgloss.Center).Width(m.widthContainer).Render(footerText)
	}

	var footerText string
	if m.account.faqFocused {
		footerText = fmt.Sprintf("%s %s    %s %s    %s %s    %s %s    %s %s",
			keybindStyle.Render("j/k"),
			descStyle.Render("scroll"),
			keybindStyle.Render("esc"),
			descStyle.Render("back"),
			keybindStyle.Render("s"),
			descStyle.Render("shop"),
			keybindStyle.Render("c"),
			descStyle.Render("cart"),
			keybindStyle.Render("q"),
			descStyle.Render("quit"),
		)
	} else if m.currentPage == accountPage {
		switch m.account.orderViewState {
		case 2:
			// Viewing single order detail
			footerText = fmt.Sprintf("%s %s    %s %s    %s %s",
				keybindStyle.Render("esc"),
				descStyle.Render("back"),
				keybindStyle.Render("s"),
				descStyle.Render("shop"),
				keybindStyle.Render("q"),
				descStyle.Render("quit"),
			)
		case 1:
			// Browsing order list
			footerText = fmt.Sprintf("%s %s    %s %s    %s %s    %s %s",
				keybindStyle.Render("j/k"),
				descStyle.Render("orders"),
				keybindStyle.Render("enter"),
				descStyle.Render("details"),
				keybindStyle.Render("esc"),
				descStyle.Render("back"),
				keybindStyle.Render("q"),
				descStyle.Render("quit"),
			)
		default:
			// Account tabs
			footerText = fmt.Sprintf("%s %s    %s %s    %s %s    %s %s    %s %s    %s %s",
				keybindStyle.Render("j/k"),
				descStyle.Render("navigate"),
				keybindStyle.Render("enter"),
				descStyle.Render("select"),
				keybindStyle.Render("s"),
				descStyle.Render("shop"),
				keybindStyle.Render("c"),
				descStyle.Render("cart"),
				keybindStyle.Render("?"),
				descStyle.Render("help"),
				keybindStyle.Render("q"),
				descStyle.Render("quit"),
			)
		}
	} else if m.currentPage == confirmPage {
		// Confirmation screen
		footerText = fmt.Sprintf("%s %s    %s %s",
			keybindStyle.Render("esc"),
			descStyle.Render("back to shop"),
			keybindStyle.Render("q"),
			descStyle.Render("quit"),
		)
	} else if m.currentPage == shippingPage {
		if m.shipping.view == 0 && m.shipping.form == nil {
			// Address list
			footerText = fmt.Sprintf("%s %s    %s %s    %s %s    %s %s",
				keybindStyle.Render("j/k"),
				descStyle.Render("addresses"),
				keybindStyle.Render("enter"),
				descStyle.Render("select"),
				keybindStyle.Render("d/x"),
				descStyle.Render("delete"),
				keybindStyle.Render("esc"),
				descStyle.Render("back"),
			)
		} else {
			// Shipping form
			footerText = fmt.Sprintf("%s %s    %s %s    %s %s",
				keybindStyle.Render("tab"),
				descStyle.Render("next"),
				keybindStyle.Render("enter"),
				descStyle.Render("submit"),
				keybindStyle.Render("esc"),
				descStyle.Render("back"),
			)
		}
	} else if m.currentPage == paymentPage {
		if m.payment.view == 0 && m.payment.form == nil {
			// Card list
			footerText = fmt.Sprintf("%s %s    %s %s    %s %s    %s %s",
				keybindStyle.Render("j/k"),
				descStyle.Render("cards"),
				keybindStyle.Render("enter"),
				descStyle.Render("select"),
				keybindStyle.Render("d/x"),
				descStyle.Render("delete"),
				keybindStyle.Render("esc"),
				descStyle.Render("back"),
			)
		} else if m.CheckingOut {
			// Submitting order
			footerText = descStyle.Render("submitting order...")
		} else {
			// Payment form
			footerText = fmt.Sprintf("%s %s    %s %s    %s %s",
				keybindStyle.Render("tab"),
				descStyle.Render("next"),
				keybindStyle.Render("enter"),
				descStyle.Render("submit"),
				keybindStyle.Render("esc"),
				descStyle.Render("back"),
			)
		}
	} else if m.currentPage == cartPage {
		// In cart view, show proceed option
		if !m.IsCartEmpty() {
			footerText = fmt.Sprintf("%s %s    %s %s    %s %s    %s %s    %s %s    %s %s    %s %s    %s %s",
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
				keybindStyle.Render("?"),
				descStyle.Render("help"),
				keybindStyle.Render("q"),
				descStyle.Render("quit"),
			)
		} else {
			footerText = fmt.Sprintf("%s %s    %s %s    %s %s    %s %s    %s %s    %s %s",
				keybindStyle.Render("j/k"),
				descStyle.Render("items"),
				keybindStyle.Render("+/-"),
				descStyle.Render("qty"),
				keybindStyle.Render("s"),
				descStyle.Render("shop"),
				keybindStyle.Render("a"),
				descStyle.Render("account"),
				keybindStyle.Render("?"),
				descStyle.Render("help"),
				keybindStyle.Render("q"),
				descStyle.Render("quit"),
			)
		}
	} else {
		footerText = fmt.Sprintf("%s %s    %s %s    %s %s    %s %s    %s %s    %s %s",
			keybindStyle.Render("j/k"),
			descStyle.Render("products"),
			keybindStyle.Render("+/-"),
			descStyle.Render("qty"),
			keybindStyle.Render("c"),
			descStyle.Render("cart"),
			keybindStyle.Render("a"),
			descStyle.Render("account"),
			keybindStyle.Render("?"),
			descStyle.Render("help"),
			keybindStyle.Render("q"),
			descStyle.Render("quit"),
		)
	}

	return lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(m.widthContainer).
		Render(footerText)
}
