package tui

import (
	"fmt"
	"strings"

	"terminalShop/pkg/models"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) OrdersView(width int) string {
	titleStyle := m.theme.TextAccent().Bold(true).MarginBottom(2)
	contentStyle := m.theme.TextBody().Width(width)

	if !m.OrdersLoaded {
		return titleStyle.Render("Order History") + "\n\n" + contentStyle.Render("Loading orders...")
	}
	if len(m.Orders) == 0 {
		return titleStyle.Render("Order History") + "\n\n" + contentStyle.Render("No orders yet")
	}
	if m.account.orderViewState == 2 {
		return m.buildOrderDetailView(m.Orders[m.account.orderCursor], width)
	}

	boxWidth := width - 4
	boxPadding := 0
	if m.heightContainer >= 25 {
		boxPadding = 1
	}

	lines := titleStyle.Render("Order History") + "\n\n"
	for i, order := range m.Orders {
		isSelected := m.account.orderViewState == 1 && i == m.account.orderCursor
		lines += m.buildOrderCard(order, boxWidth, isSelected, boxPadding, width)
		if i < len(m.Orders)-1 {
			lines += "\n"
		}
	}

	if m.account.orderViewState == 0 {
		hintStyle := m.theme.TextDim()
		lines += "\n" + hintStyle.Render("enter: browse orders")
	}
	return lines
}

// buildOrderCard renders a single order as a bordered box for the order list.
// Shows order number, date, status, total, and the first 2 items as a preview.
func (m Model) buildOrderCard(order models.Order, boxWidth int, isSelected bool, boxPadding int, containerWidth int) string {
	// Order header line: "Order #123" left, "Jan 02 2026  PAID" right
	nameStyle := m.theme.TextAccent().Bold(true)

	dimStyle := m.theme.TextDim()

	statusStyle := m.theme.TextHighlight().Bold(true)

	orderLabel := nameStyle.Render(fmt.Sprintf("Order #%d", order.ID))
	total := nameStyle.Render(fmt.Sprintf("$%.2f", float64(order.Total)/100.0))
	date := dimStyle.Render(order.CreatedAt.Format("Jan 02 2006"))
	status := statusStyle.Render(strings.ToUpper(string(order.Status)))

	rightSide := date + "  " + status + "  " + total
	rightWidth := lipgloss.Width(rightSide)
	leftWidth := lipgloss.Width(orderLabel)
	spacing := boxWidth - leftWidth - rightWidth - 4 // 4 for padding
	if spacing < 1 {
		spacing = 1
	}

	line1 := orderLabel + lipgloss.NewStyle().Width(spacing).Render("") + rightSide

	// Item preview lines (show first 2 items)
	var lines []string
	lines = append(lines, line1)

	previewCount := 2
	if len(order.Items) < previewCount {
		previewCount = len(order.Items)
	}
	for i := 0; i < previewCount; i++ {
		item := order.Items[i]
		itemLine := dimStyle.Render(fmt.Sprintf("%dx %s - $%.2f", item.Quantity, item.Name, float64(item.Price)/100.0))
		lines = append(lines, itemLine)
	}
	if len(order.Items) > 2 {
		extra := len(order.Items) - 2
		moreText := "item"
		if extra > 1 {
			moreText = "items"
		}
		lines = append(lines, dimStyle.Render(fmt.Sprintf("+ %d more %s", extra, moreText)))
	}

	boxContent := strings.Join(lines, "\n")

	// Create bordered box — highlight if selected
	var itemBox lipgloss.Style
	if isSelected {
		itemBox = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(m.theme.Accent()).
			Padding(boxPadding, 2).
			Width(boxWidth)
	} else {
		itemBox = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(m.theme.Border()).
			Padding(boxPadding, 2).
			Width(boxWidth)
	}

	// Add cursor indicator for selected order
	prefix := "  "
	if isSelected {
		prefix = "> "
	}

	centered := lipgloss.NewStyle().
		Width(containerWidth).
		Align(lipgloss.Center)

	return centered.Render(prefix + itemBox.Render(boxContent))
}

// buildOrderDetailView renders the full detail view for a single order
func (m Model) buildOrderDetailView(order models.Order, width int) string {
	titleStyle := m.theme.TextAccent().Bold(true).MarginBottom(1)

	labelStyle := m.theme.TextHighlight().Bold(true)

	valueStyle := m.theme.TextBody()

	dimStyle := m.theme.TextDim()

	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render(fmt.Sprintf("Order #%d", order.ID)))
	b.WriteString("\n\n")

	// Status and date
	b.WriteString(labelStyle.Render("Status:  "))
	b.WriteString(valueStyle.Render(strings.ToUpper(string(order.Status))))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Date:    "))
	b.WriteString(valueStyle.Render(order.CreatedAt.Format("Jan 02 2006 3:04 PM")))
	b.WriteString("\n\n")

	// Items
	b.WriteString(labelStyle.Render("Items"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("-", 40)))
	b.WriteString("\n")
	for _, item := range order.Items {
		price := fmt.Sprintf("$%.2f", float64(item.Price)/100.0)
		line := fmt.Sprintf("  %dx %-20s %s", item.Quantity, item.Name, price)
		b.WriteString(valueStyle.Render(line))
		b.WriteString("\n")
	}
	b.WriteString(dimStyle.Render(strings.Repeat("-", 40)))
	b.WriteString("\n")

	// Totals
	subtotal := fmt.Sprintf("$%.2f", float64(order.Subtotal)/100.0)
	shipping := fmt.Sprintf("$%.2f", float64(order.ShippingCost)/100.0)
	total := fmt.Sprintf("$%.2f", float64(order.Total)/100.0)
	b.WriteString(valueStyle.Render(fmt.Sprintf("  %-22s %s", "Subtotal", subtotal)))
	b.WriteString("\n")
	b.WriteString(valueStyle.Render(fmt.Sprintf("  %-22s %s", "Shipping", shipping)))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render(fmt.Sprintf("  %-22s %s", "Total", total)))
	b.WriteString("\n\n")

	// Shipping address
	b.WriteString(labelStyle.Render("Ship To"))
	b.WriteString("\n")
	b.WriteString(valueStyle.Render("  " + order.ShippingName))
	b.WriteString("\n")
	b.WriteString(valueStyle.Render("  " + order.ShippingStreet))
	b.WriteString("\n")
	if order.ShippingStreet2 != "" {
		b.WriteString(valueStyle.Render("  " + order.ShippingStreet2))
		b.WriteString("\n")
	}
	cityLine := order.ShippingCity
	if order.ShippingState != "" {
		cityLine += ", " + order.ShippingState
	}
	cityLine += " " + order.ShippingZip
	b.WriteString(valueStyle.Render("  " + cityLine))
	b.WriteString("\n")
	b.WriteString(valueStyle.Render("  " + order.ShippingCountry))
	b.WriteString("\n\n")

	// Footer hint
	b.WriteString(dimStyle.Render("esc: back to orders"))

	return b.String()
}
