package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"terminalShop/pkg/models"
)

func (m Model) BuildAccountView(availableHeight int) string {
	leftWidth := 18
	rightWidth := m.widthContent - leftWidth - 2 // 2 for gap

	// Build the left panel (menu items)
	leftPanel := ""
	for i, item := range models.AccountMenuItems {
		if m.AccountCursor == i {
			style := lipgloss.NewStyle().
				Background(lipgloss.Color("#4682B4")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1).
				Width(leftWidth - 2).
				Align(lipgloss.Left)
			leftPanel += style.Render(item) + "\n"
		} else {
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#AAAAAA")).
				Padding(0, 1).
				Width(leftWidth - 2).
				Align(lipgloss.Left)
			leftPanel += style.Render(item) + "\n"
		}
	}

	leftContainer := lipgloss.NewStyle().
		Width(leftWidth)

	// Build the detail view (right panel) based on cursor position
	detailView := ""
	if m.AccountCursor >= 0 && m.AccountCursor < len(models.AccountMenuItems) {
		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			MarginBottom(2)

		// On small terminals, use full content width
		detailContentWidth := rightWidth - 2
		if m.size < large {
			detailContentWidth = m.widthContent - 2
		}

		contentStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")).
			Width(detailContentWidth)

		selectedItem := models.AccountMenuItems[m.AccountCursor]
		switch selectedItem {
		case "order history":
			if !m.OrdersLoaded {
				detailView = titleStyle.Render("Order History") + "\n\n" + contentStyle.Render("Loading orders...")
			} else if len(m.Orders) == 0 {
				detailView = titleStyle.Render("Order History") + "\n\n" + contentStyle.Render("No orders yet.")
			} else if m.OrderViewState == 2 {
				// State 2: full detail for the selected order
				detailView = m.buildOrderDetailView(m.Orders[m.OrderCursor], detailContentWidth)
			} else {
				// State 0 (preview) or State 1 (browsing with cursor)
				boxWidth := detailContentWidth - 4
				boxPadding := 0
				if m.heightContainer >= 25 {
					boxPadding = 1
				}

				lines := titleStyle.Render("Order History") + "\n\n"
				for i, order := range m.Orders {
					isSelected := m.OrderViewState == 1 && i == m.OrderCursor
					lines += m.buildOrderCard(order, boxWidth, isSelected, boxPadding, detailContentWidth)
					if i < len(m.Orders)-1 {
						lines += "\n"
					}
				}

				if m.OrderViewState == 0 {
					hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
					lines += "\n" + hintStyle.Render("enter: browse orders")
				}
				detailView = lines
			}
		case "faq":
			questionStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#4682B4")).
				Bold(true)

			faqContent := ""
			for i, faq := range m.FAQs {
				faqContent += questionStyle.Render(wordWrap(faq.Question, detailContentWidth)) + "\n"
				faqContent += contentStyle.Render(wordWrap(faq.Answer, detailContentWidth))
				if i < len(m.FAQs)-1 {
					faqContent += "\n\n"
				}
			}
			detailView = titleStyle.Render("FAQ") + "\n\n" + faqContent
			if !m.FaqFocused {
				hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
				detailView += "\n\n" + hintStyle.Render("enter: scroll faq")
			}
		case "about":
			accentStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#4682B4")).
				Bold(true)
			userStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("C6DDF0")).
				Bold(true)

			aboutContent := contentStyle.Render(wordWrap(
				"This project is heavily inspired by the wonderful team who made TerminalDotShop. I want to thank them for open-sourcing the code for that project. Cudos to those smart, funny, amazing looking devs:",
				detailContentWidth,
			)) + "\n\n"
			aboutContent += userStyle.Render("@thdxr") + "\n"
			aboutContent += userStyle.Render("@adamdotdev") + "\n"
			aboutContent += userStyle.Render("@theprimeagen") + "\n"
			aboutContent += userStyle.Render("@teej_dv") + "\n"
			aboutContent += userStyle.Render("@iamdavidhill") + "\n\n"
			aboutContent += contentStyle.Render("And me:") + "\n\n"
			aboutContent += userStyle.Render("@marcusDenslow") + "\n\n"
			aboutContent += accentStyle.Render("Terminal Products, Inc.")

			detailView = titleStyle.Render("About") + "\n\n" + aboutContent

		}
	}

	if m.OrderViewState >= 1 || m.FaqFocused {
		m.AccountDetailVP.SetContent(detailView)
		detailView = m.AccountDetailVP.View()
	}

	detailContainer := lipgloss.NewStyle().
		Width(rightWidth)

	// Responsive layout: side-by-side on large, stacked on small/medium
	if m.size < large {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			leftContainer.Render(leftPanel),
			detailContainer.Width(m.widthContent).Render(detailView),
		)
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftContainer.Render(leftPanel),
		"  ",
		detailContainer.Render(detailView),
	)
}

// buildOrderCard renders a single order as a bordered box for the order list.
// Shows order number, date, status, total, and the first 2 items as a preview.
func (m Model) buildOrderCard(order models.Order, boxWidth int, isSelected bool, boxPadding int, containerWidth int) string {
	// Order header line: "Order #123" left, "Jan 02 2026  PAID" right
	nameStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666"))

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4682B4")).
		Bold(true)

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
			Border(lipgloss.NormalBorder(), true).
			BorderForeground(lipgloss.Color("#FFFFFF")).
			Padding(boxPadding, 2).
			Width(boxWidth)
	} else {
		itemBox = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#666666")).
			Padding(boxPadding, 2).
			Width(boxWidth)
	}

	centered := lipgloss.NewStyle().
		Width(containerWidth).
		Align(lipgloss.Center)

	return centered.Render(itemBox.Render(boxContent))
}

// buildOrderDetailView renders the full detail view for a single order.
func (m Model) buildOrderDetailView(order models.Order, width int) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4682B4")).
		Bold(true)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#AAAAAA"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666"))

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
