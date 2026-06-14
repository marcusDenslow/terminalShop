package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// cartBlendColor mixes the colors of all cart items, weighted by quantity,
// so the header price tints toward whatever dominates the cart.
func (m Model) cartBlendColor() color.Color {
	var rSum, gSum, bSum, n int
	for _, item := range m.Cart {
		r, g, b, ok := parseHexColor(item.Coffee.Color)
		if !ok {
			continue
		}
		rSum += r * item.Quantity
		gSum += g * item.Quantity
		bSum += b * item.Quantity
		n += item.Quantity
	}
	if n == 0 {
		return cWhite
	}
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", rSum/n, gSum/n, bSum/n))
}

// BuildHeader renders the top chrome bar: brand, tab pills, and cart summary.
// Layout ported from the design prototype at ~/programming/test/shop.go.
func (m Model) BuildHeader() string {
	_, _, _, wideW := m.shopLayout()

	brand := pWht.Bold(true).Render("terminal")

	pill := lipgloss.NewStyle().Background(cPill).Foreground(cWhite).Bold(true).Padding(0, 1)
	tab := func(key, label string, active bool) string {
		if active {
			return pill.Render(label)
		}
		return pDim.Render(key+" ") + pBody.Render(label)
	}

	var left string
	if m.size == small {
		left = brand
	} else {
		left = strings.Join([]string{
			brand,
			tab("s", "shop", m.currentPage == shopPage),
			tab("a", "account", m.currentPage == accountPage),
		}, "   ")
	}

	cartLabel := pDim.Render("cart  ")
	if m.inCartFlow() {
		cartLabel = pWht.Bold(true).Render("cart  ")
	}
	cart := cartLabel +
		lipgloss.NewStyle().Foreground(m.cartBlendColor()).Bold(true).
			Render(fmt.Sprintf("$%d", m.CalculateSubtotal()/100)) +
		pDim.Render(fmt.Sprintf("  ·  %d", m.CartItemCount()))

	gap := wideW - lipgloss.Width(left) - lipgloss.Width(cart)
	if gap < 1 {
		gap = 1
	}
	row := left + strings.Repeat(" ", gap) + cart

	return panel(wideW, 0, row)
}
