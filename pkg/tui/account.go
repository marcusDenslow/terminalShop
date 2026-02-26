package tui

import (
	"fmt"

	"strings"

	"github.com/charmbracelet/lipgloss"

	"terminalShop/pkg/models"
)

func (m Model) BuildAccountView() string {
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
			} else {
				lines := titleStyle.Render("Order History") + "\n\n"
				for _, order := range m.Orders {
					status := strings.ToUpper(string(order.Status))
					date := order.CreatedAt.Format("Jan 02 2006")
					total := fmt.Sprintf("$%.2f", float64(order.Total)/100.0)
					lines += contentStyle.Render(fmt.Sprintf("Order #%d %s %s %s", order.ID, date, total, status)) + "\n"
					for _, item := range order.Items {
						lines += contentStyle.Render(fmt.Sprintf(" %dx %s - $%.2f", item.Quantity, item.Name, float64(item.Price)/100.0)) + "\n"
					}
					lines += "\n"
				}
				detailView = lines
			}
		case "faq":
			detailView = titleStyle.Render("FAQ") + "\n\n" +
				contentStyle.Render("Q: How do I place an order?\nA: Browse products with j/k, adjust quantity with +/-, and checkout via the cart.\n\nQ: What payment methods do you accept?\nA: We accept all major credit cards via Stripe.")
		case "about":
			detailView = titleStyle.Render("About Terminal Coffee Shop") + "\n\n" +
				contentStyle.Render("A terminal-based coffee ordering experience.\nBuilt with Go and Bubbletea.")
		}
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
