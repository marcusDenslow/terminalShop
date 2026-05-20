package tui

import (
	"fmt"
	"strings"

	"terminalShop/pkg/models"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) OrdersView(width int) string {
	return m.orderListView("Order History", m.HistoryOrders(), width)
}

func (m Model) ActiveOrdersView(width int) string {
	return m.orderListView("Active Orders", m.ActiveOrders(), width)
}

func (m Model) ActiveOrders() []models.Order {
	out := make([]models.Order, 0, len(m.Orders))
	for _, o := range m.Orders {
		if o.IsActive() {
			out = append(out, o)
		}
	}
	return out
}

func (m Model) HistoryOrders() []models.Order {
	out := make([]models.Order, 0, len(m.Orders))
	for _, o := range m.Orders {
		if !o.IsActive() {
			out = append(out, o)
		}
	}
	return out
}

func (m Model) currentOrderList() []models.Order {
	if m.account.cursor < 0 || m.account.cursor >= len(models.AccountMenuItems) {
		return nil
	}
	switch models.AccountMenuItems[m.account.cursor] {
	case "active orders":
		return m.ActiveOrders()
	case "order history":
		return m.HistoryOrders()
	}
	return nil
}

func (m Model) orderListView(title string, orders []models.Order, width int) string {
	titleStyle := m.theme.TextAccent().Bold(true).MarginBottom(1)
	contentStyle := m.theme.TextBody().Width(width)

	if !m.OrdersLoaded {
		return titleStyle.Render(title) + "\n" + contentStyle.Render("Loading orders...")
	}
	if len(orders) == 0 {
		return titleStyle.Render(title) + "\n" + contentStyle.Render("No orders yet")
	}
	if m.account.orderViewState == 2 {
		return m.buildOrderDetailView(orders[m.account.orderCursor], width)
	}

	boxWidth := width - 2

	// Build cards and join vertically — no manual "\n" gaps means fixed spacing
	var cards []string
	for i, order := range orders {
		isSelected := m.account.orderViewState == 1 && i == m.account.orderCursor
		cards = append(cards, m.buildOrderCard(order, boxWidth, isSelected))
	}
	cardList := lipgloss.JoinVertical(lipgloss.Left, cards...)

	// In preview mode (viewState 0), show title + hint
	if m.account.orderViewState == 0 {
		hintStyle := m.theme.TextDim()
		return titleStyle.Render(title) + "\n" +
			cardList + "\n" +
			hintStyle.Render("enter: browse orders")
	}

	// In list browsing mode (viewState 1), just show cards — no title
	// (the left panel menu already shows "order history" as selected)
	return cardList
}

// buildOrderCard renders a single order as a bordered box with fixed height.
// Shows order number + total on line 1, date + status on line 2.
// Item details are shown in the detail view (viewState 2).
func (m Model) buildOrderCard(order models.Order, boxWidth int, isSelected bool) string {
	nameStyle := m.theme.TextDim()
	if isSelected {
		nameStyle = m.theme.TextAccent().Bold(true)
	}

	// When selected, secondary text uses accent color so the whole card "lights up"
	dimStyle := m.theme.TextDim()
	if isSelected {
		dimStyle = m.theme.TextAccent()
	}

	// Line 1: "Order #N" left, "$X.XX" right
	orderLabel := nameStyle.Render(fmt.Sprintf("Order #%d", order.ID))
	total := nameStyle.Render(fmt.Sprintf("$%.2f", float64(order.Total)/100.0))

	innerWidth := boxWidth - 4 // account for border + padding
	leftWidth := lipgloss.Width(orderLabel)
	rightWidth := lipgloss.Width(total)
	spacing := innerWidth - leftWidth - rightWidth
	if spacing < 1 {
		spacing = 1
	}
	line1 := orderLabel + lipgloss.NewStyle().Width(spacing).Render("") + total

	// Line 2: date + derived status
	date := dimStyle.Render(order.CreatedAt.Format("Jan 02 2006"))
	status := m.displayStyle(order.DisplayKind()).Bold(true).Render(order.DisplayState())
	line2 := date + "  " + status

	boxContent := line1 + "\n" + line2

	return m.CreateBoxCustom(boxContent, isSelected, boxWidth)
}

// buildOrderDetailView renders the full detail view for a single order
func (m Model) buildOrderDetailView(order models.Order, _ int) string {
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
	b.WriteString(m.displayStyle(order.DisplayKind()).Render(order.DisplayState()))
	b.WriteString("\n")
	if order.TrackingStatusDetails != "" {
		b.WriteString(dimStyle.Render("         " + order.TrackingStatusDetails))
		b.WriteString("\n")
	}
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

	// Shipping (only when at least one shipment exists)
	if order.Carrier != "" {
		b.WriteString(labelStyle.Render("Shipping"))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  %-10s %s", "Carrier", order.Carrier)))
		b.WriteString("\n")
		b.WriteString(valueStyle.Render(fmt.Sprintf("  %-10s %s", "Tracking", order.TrackingNumber)))
		b.WriteString("\n")
		if order.TrackingStatus != "" {
			b.WriteString(valueStyle.Render(fmt.Sprintf("  %-10s %s", "Status", string(order.TrackingStatus))))
			b.WriteString("\n")
		}
		if order.ShippedAt != nil {
			b.WriteString(valueStyle.Render(fmt.Sprintf("  %-10s %s",
				"Shipped", order.ShippedAt.Format("Jan 02 2006"))))
			b.WriteString("\n")
		}
		if order.TrackingURL != "" {
			b.WriteString(hyperlink(dimStyle.Render(fmt.Sprintf("  View on %s ->", order.Carrier)), order.TrackingURL))
			b.WriteString("\n")
		}
	}

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

// displayStyle maps a model-layer DisplayKind token to a themed lipgloss style.
// Keeps pkg/models free of UI deps.
func (m Model) displayStyle(k models.DisplayKind) lipgloss.Style {
	switch k {
	case models.DisplayKindSuccess:
		return m.theme.TextSuccess()
	case models.DisplayKindBrand:
		return m.theme.TextBrand()
	case models.DisplayKindAccent:
		return m.theme.TextAccent()
	case models.DisplayKindError:
		return m.theme.TextError()
	case models.DisplayKindWarning:
		// !TODO add refunds and change this
		// No warning color in theme yet, reuse loading pink as visual flag
		// for now
		return m.theme.TextLoading()
	}
	return m.theme.TextHighlight()
}

// hyperlink wraps label in OSC 8 escape sequences so supporting terminals
// render it as a clickable link to url. Uses BEL (\x07) as the string
// terminator — some renderers in the pipeline (bubbletea/lipgloss) handled
// the BEL form when they didn't handle the ST (\x1b\\) form.
func hyperlink(label, url string) string {
	return fmt.Sprintf("\x1b]8;;%s\x07%s\x1b]8;;\x07", url, label)
}
