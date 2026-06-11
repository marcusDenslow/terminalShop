package tui

import (
	"fmt"
	"strings"

	"terminalShop/pkg/models"

	"charm.land/lipgloss/v2"
)

func (m Model) OrdersView(width int) string {
	return m.orderListView(m.HistoryOrders(), width)
}

func (m Model) ActiveOrdersView(width int) string {
	return m.orderListView(m.ActiveOrders(), width)
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

// selectedRefundableOrder returns the order under the cursor in the active
// order-detail view, paired with ok=true, when the user is allowed to file a
// refund request for it. Returns (zero, false) otherwise.
func (m Model) selectedRefundableOrder() (models.Order, bool) {
	orders := m.currentOrderList()
	if m.account.orderCursor < 0 || m.account.orderCursor >= len(orders) {
		return models.Order{}, false
	}
	order := orders[m.account.orderCursor]
	if !order.CanRequestRefund() {
		return models.Order{}, false
	}
	return order, true
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

// orderCardHeight is the rendered height of one order card:
// border (2) + content lines (2), no vertical padding.
const orderCardHeight = 4

// orderListPageSize returns how many whole order cards fit in the detail
// area. Both scroll arrows render outside the list, in the gap rows above
// and below the body, so the cards use the full viewport height.
func (m Model) orderListPageSize() int {
	n := m.account.detailViewport.Height() / orderCardHeight
	if n < 1 {
		n = 1
	}
	return n
}

// orderHiddenAbove returns how many order cards are scrolled off above the
// windowed list, or 0 when the windowed order list isn't showing.
func (m Model) orderHiddenAbove() int {
	if m.account.orderViewState != 1 || !m.OrdersLoaded {
		return 0
	}
	orders := m.currentOrderList()
	if len(orders) == 0 {
		return 0
	}
	page := m.orderListPageSize()
	start := m.account.orderWindowStart
	if start > len(orders)-page {
		start = len(orders) - page
	}
	if start < 0 {
		start = 0
	}
	return start
}

// orderHiddenBelow returns how many order cards are scrolled off below the
// windowed list, or 0 when the windowed order list isn't showing.
func (m Model) orderHiddenBelow() int {
	if m.account.orderViewState == 2 || !m.OrdersLoaded {
		return 0
	}
	orders := m.currentOrderList()
	if len(orders) == 0 {
		return 0
	}
	below := len(orders) - (m.orderHiddenAbove() + m.orderListPageSize())
	if below < 0 {
		below = 0
	}
	return below
}

// orderScrollIndicator renders the dim "more items" arrow row, centered
// across the card width.
func orderScrollIndicator(arrow string, count, width int) string {
	return pDim.Width(width).Align(lipgloss.Center).
		Render(fmt.Sprintf("%s %d more", arrow, count))
}

func (m Model) orderListView(orders []models.Order, width int) string {
	contentStyle := m.theme.TextBody().Width(width)

	if !m.OrdersLoaded {
		return contentStyle.Render("Loading orders...")
	}
	if len(orders) == 0 {
		return contentStyle.Render("No orders yet")
	}
	if m.account.orderViewState == 2 {
		return m.buildOrderDetailView(orders[m.account.orderCursor], width)
	}

	// Whole-card windowing: render only fully-visible cards, never clipping a
	// card mid-border. Both scroll arrows render in BuildAccountView's gap
	// rows, so the cards span exactly the menu panel's rows.
	page := m.orderListPageSize()
	start := m.orderHiddenAbove()
	end := start + page
	if end > len(orders) {
		end = len(orders)
	}

	var rows []string
	for i := start; i < end; i++ {
		isSelected := m.account.orderViewState == 1 && i == m.account.orderCursor
		rows = append(rows, m.buildOrderCard(orders[i], width, isSelected))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
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
			b.WriteString(dimStyle.Render(fmt.Sprintf("  https://api.sshops.uk/t/%d", order.ID)))
			b.WriteString("\n")
		}
	}

	visible := make([]models.OrderEvent, 0, len(order.Events))
	for _, event := range order.Events {
		if _, ok := event.DisplayLabel(); ok {
			visible = append(visible, event)
		}
	}
	if len(visible) > 0 {
		b.WriteString(labelStyle.Render("Timeline"))
		b.WriteString("\n")
		for _, event := range visible {
			label, _ := event.DisplayLabel()
			when := event.CreatedAt.Format("Jan 02 3:04 PM")
			b.WriteString(valueStyle.Render(fmt.Sprintf("  %-20s %s", label, dimStyle.Render(when))))
			b.WriteString("\n")
		}
		b.WriteString("\n")
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
	if order.CanRequestRefund() {
		b.WriteString(dimStyle.Render("r: refund  esc: back to orders"))
	} else {
		b.WriteString(dimStyle.Render("esc: back to orders"))
	}

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
		return m.theme.TextLoading()
	case models.DisplayKindRefund:
		return m.theme.TextBrand()
	}
	return m.theme.TextHighlight()
}
