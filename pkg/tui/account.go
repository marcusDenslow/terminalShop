package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
			style := m.theme.Base().Background(m.theme.Highlight()).Foreground(m.theme.Accent()).Padding(0, 1).Width(leftWidth - 2).Align(lipgloss.Left)
			leftPanel += style.Render(item) + "\n"
		} else {
			style := m.theme.TextBody().Padding(0, 1).Width(leftWidth - 2).Align(lipgloss.Left)
			leftPanel += style.Render(item) + "\n"
		}
	}

	leftContainer := m.theme.Base().
		Width(leftWidth)

	// Build the detail view (right panel) based on cursor position
	detailView := ""
	if m.AccountCursor >= 0 && m.AccountCursor < len(models.AccountMenuItems) {
		titleStyle := m.theme.TextAccent().Bold(true).MarginBottom(2)

		// On small terminals, use full content width
		detailContentWidth := rightWidth - 2
		if m.size < large {
			detailContentWidth = m.widthContent - 2
		}

		contentStyle := m.theme.TextBody().Width(detailContentWidth)

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
					hintStyle := m.theme.TextDim()
					lines += "\n" + hintStyle.Render("enter: browse orders")
				}
				detailView = lines
			}

		case "addresses":
			lines := titleStyle.Render("Address") + "\n\n"
			if len(m.SavedAddresses) == 0 {
				lines += contentStyle.Render("No saved addresses")
			} else {
				for i, addr := range m.SavedAddresses {
					isSelected := m.AddressListFocused && i == m.AccountAddressCursor
					label := addr.Name + "  " + addr.Street + ", " + addr.City
					if len(label) > detailContentWidth-4 {
						label = label[:detailContentWidth-4]
					}
					if m.AccountAddressDeleting != nil && *m.AccountAddressDeleting == i {
						lines += m.theme.TextError().Bold(true).Render("  deletes? (y/n)") + "\n"
					} else if isSelected {
						lines += m.theme.TextAccent().Bold(true).Render("> "+label) + "\n"
					} else {
						lines += contentStyle.Render(" "+label) + "\n"
					}
				}
			}
			if m.AddressListFocused {
				lines += "\n" + m.theme.TextDim().Render("x: delete  esc: back")
			} else {
				lines += "\n" + m.theme.TextDim().Render("enter: manage")
			}
			detailView = lines

		case "cards":
			lines := titleStyle.Render("Cards") + "\n\n"
			if len(m.SavedCards) == 0 {
				lines += contentStyle.Render("No saved cards.")
			} else {
				for i, card := range m.SavedCards {
					isSelected := m.CardListFocused && i == m.AccountCardCursor
					label := fmt.Sprintf("**** **** **** %s  %s  exp %02d/%02d",
						card.Last4, card.Brand, card.ExpMonth, card.ExpYear%100)
					if m.AccountCardDeleting != nil && *m.AccountCardDeleting == i {
						lines += m.theme.TextError().Bold(true).Render("  delete? (y/n)") + "\n"
					} else if isSelected {
						lines += m.theme.TextAccent().Bold(true).Render("> "+label) + "\n"
					} else {
						lines += contentStyle.Render(" "+label) + "\n"
					}
				}
			}
			if m.CardListFocused {
				lines += "\n" + m.theme.TextDim().Render("x: delete  esc: back")
			} else {
				lines += "\n" + m.theme.TextDim().Render("enter: manage")
			}
			detailView = lines
		case "faq":
			questionStyle := m.theme.TextHighlight().Bold(true)

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
				hintStyle := m.theme.TextDim()
				detailView += "\n\n" + hintStyle.Render("enter: scroll faq")
			}
		case "about":
			accentStyle := m.theme.TextHighlight().Bold(true)
			userStyle := m.theme.TextAccent().Bold(true)

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
			aboutContent += contentStyle.Render(wordWrap("This project tries to copy the idea of terminalDotShop, but with one twist: write everything in GO. this is my humble attempt",
				detailContentWidth)) + "\n"
			aboutContent += accentStyle.Render("Terminal Products, Inc.")

			detailView = titleStyle.Render("About") + "\n\n" + aboutContent

		}
	}

	// Apply internal scrolling to just the detail view
	// Scrolling is needed for: order detail/list and FAQ when focused
	if m.OrderViewState >= 1 || m.FaqFocused {
		detailLines := strings.Split(detailView, "\n")
		totalLines := len(detailLines)

		// In stacked mode, reduce available height by left panel
		scrollHeight := availableHeight
		if m.size < large {
			leftPanelHeight := len(models.AccountMenuItems) + 1
			scrollHeight = availableHeight - leftPanelHeight
		}
		if scrollHeight < 3 {
			scrollHeight = 3
		}

		// Clamp scroll offset locally (don't mutate m since value receiver)
		offset := m.ScrollOffset
		maxScroll := totalLines - scrollHeight
		if maxScroll < 0 {
			maxScroll = 0
		}
		if offset > maxScroll {
			offset = maxScroll
		}
		if offset < 0 {
			offset = 0
		}

		// Slice to visible window
		if totalLines > scrollHeight {
			end := offset + scrollHeight
			if end > totalLines {
				end = totalLines
			}
			detailLines = detailLines[offset:end]
		}
		detailView = strings.Join(detailLines, "\n")
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
			Border(lipgloss.NormalBorder(), true).
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

	centered := lipgloss.NewStyle().
		Width(containerWidth).
		Align(lipgloss.Center)

	return centered.Render(itemBox.Render(boxContent))
}

// buildOrderDetailView renders the full detail view for a single order.
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

// computeFaqScrollMax calculates the maximum scroll offset for the FAQ content.
// This renders the FAQ the same way BuildAccountView does, counts the lines,
// and returns the max scroll value. Returns 0 if no scrolling is needed.
func (m Model) computeFaqScrollMax() int {
	leftWidth := 18
	rightWidth := m.widthContent - leftWidth - 2
	detailContentWidth := rightWidth - 2
	if m.size < large {
		detailContentWidth = m.widthContent - 2
	}

	questionStyle := m.theme.TextHighlight().Bold(true)
	contentStyle := m.theme.TextBody().Width(detailContentWidth)

	faqContent := ""
	for i, faq := range m.FAQs {
		faqContent += questionStyle.Render(wordWrap(faq.Question, detailContentWidth)) + "\n"
		faqContent += contentStyle.Render(wordWrap(faq.Answer, detailContentWidth))
		if i < len(m.FAQs)-1 {
			faqContent += "\n\n"
		}
	}

	titleStyle := m.theme.TextAccent().Bold(true).MarginBottom(2)
	detailView := titleStyle.Render("FAQ") + "\n\n" + faqContent

	totalLines := len(strings.Split(detailView, "\n"))

	scrollHeight := m.heightContainer - 7
	if m.size < large {
		scrollHeight -= len(models.AccountMenuItems) + 1
	}
	if scrollHeight < 3 {
		scrollHeight = 3
	}

	maxScroll := totalLines - scrollHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

func (m Model) AccountUpdate(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	selectedItem := ""
	if m.AccountCursor < len(models.AccountMenuItems) {
		selectedItem = models.AccountMenuItems[m.AccountCursor]
	}

	switch keyMsg.String() {
	case "up", "k":
		if m.AddressListFocused && m.AccountAddressDeleting == nil {
			if m.AccountAddressCursor > 0 {
				m.AccountAddressCursor--
			}
		} else if m.CardListFocused && m.AccountCardDeleting == nil {
			if m.AccountCardCursor > 0 {
				m.AccountCardCursor--
			}
		} else if m.FaqFocused {
			m.ScrollOffset -= 3
			if m.ScrollOffset < 0 {
				m.ScrollOffset = 0
			}
		} else if m.OrderViewState == 1 && m.OrderCursor > 0 {
			m.OrderCursor--
			cardHeight := 5
			if m.heightContainer >= 25 {
				cardHeight = 7
			}
			targetTop := 3 + m.OrderCursor*cardHeight
			if targetTop < m.ScrollOffset {
				m.ScrollOffset = targetTop
			}
		} else if m.OrderViewState == 0 && m.AccountCursor > 0 {
			m.AccountCursor--
			m.ScrollOffset = 0
		}

	case "down", "j":
		if m.AddressListFocused && m.AccountAddressDeleting == nil {
			if m.AccountAddressCursor < len(m.SavedAddresses)-1 {
				m.AccountAddressCursor++
			}
		} else if m.CardListFocused && m.AccountCardDeleting == nil {
			if m.AccountCardCursor < len(m.SavedCards)-1 {
				m.AccountCardCursor++
			}
		} else if m.FaqFocused {
			m.ScrollOffset += 3
			maxScroll := m.computeFaqScrollMax()
			if m.ScrollOffset > maxScroll {
				m.ScrollOffset = maxScroll
			}
		} else if m.OrderViewState == 1 && m.OrderCursor < len(m.Orders)-1 {
			m.OrderCursor++
			cardHeight := 5
			if m.heightContainer >= 25 {
				cardHeight = 7
			}
			viewportHeight := m.heightContainer - 7
			if m.size < large {
				viewportHeight -= len(models.AccountMenuItems) + 1
			}
			if viewportHeight < 5 {
				viewportHeight = 5
			}
			targetBottom := 3 + (m.OrderCursor+1)*cardHeight
			if targetBottom > m.ScrollOffset+viewportHeight {
				m.ScrollOffset = targetBottom - viewportHeight
			}
		} else if m.OrderViewState == 0 && !m.AddressListFocused && !m.CardListFocused && m.AccountCursor < len(models.AccountMenuItems)-1 {
			m.AccountCursor++
			m.ScrollOffset = 0
		}

	case "pgup", "ctrl+u":
		if m.FaqFocused || m.OrderViewState >= 2 {
			m.ScrollOffset -= 3
			if m.ScrollOffset < 0 {
				m.ScrollOffset = 0
			}
		}

	case "pgdown", "ctrl+d":
		if m.FaqFocused || m.OrderViewState >= 2 {
			m.ScrollOffset += 3
			maxScroll := m.computeFaqScrollMax()
			if m.FaqFocused && maxScroll >= 0 && m.ScrollOffset > maxScroll {
				m.ScrollOffset = maxScroll
			}
		}

	case "x", "d":
		if m.AddressListFocused && m.AccountAddressDeleting == nil && m.AccountAddressCursor < len(m.SavedAddresses) {
			m.AccountAddressDeleting = &m.AccountAddressCursor
		} else if m.CardListFocused && m.AccountCardDeleting == nil && m.AccountCardCursor < len(m.SavedCards) {
			m.AccountCardDeleting = &m.AccountCardCursor
		}

	case "y":
		if m.AccountAddressDeleting != nil {
			cursor := *m.AccountAddressDeleting
			addr := m.SavedAddresses[cursor]
			m.AccountAddressDeleting = nil
			m.SavedAddresses = append(m.SavedAddresses[:cursor], m.SavedAddresses[cursor+1:]...)
			if m.AccountAddressCursor >= len(m.SavedAddresses) && m.AccountAddressCursor > 0 {
				m.AccountAddressCursor--
			}
			return m, m.deleteAddressCmd(addr.ID)
		} else if m.AccountCardDeleting != nil {
			cursor := *m.AccountCardDeleting
			card := m.SavedCards[cursor]
			m.AccountCardDeleting = nil
			m.SavedCards = append(m.SavedCards[:cursor], m.SavedCards[cursor+1:]...)
			if m.AccountCardCursor >= len(m.SavedCards) && m.AccountCardCursor > 0 {
				m.AccountCardCursor--
			}
			return m, m.deleteCardCmd(card.ID)
		}

	case "n":
		m.AccountAddressDeleting = nil
		m.AccountCardDeleting = nil

	case "p", "enter":
		switch selectedItem {
		case "order history":
			if len(m.Orders) > 0 {
				if m.OrderViewState == 0 {
					m.OrderViewState = 1
					m.OrderCursor = 0
					m.ScrollOffset = 0
				} else if m.OrderViewState == 1 {
					m.OrderViewState = 2
					m.ScrollOffset = 0
				}
			}
		case "addresses":
			if !m.AddressListFocused {
				m.AddressListFocused = true
				m.AccountAddressCursor = 0
			}
		case "cards":
			if !m.CardListFocused {
				m.CardListFocused = true
				m.AccountCardCursor = 0
			}
		case "faq":
			if !m.FaqFocused {
				m.FaqFocused = true
				m.ScrollOffset = 0
			}
		}

	case "esc":
		if m.AccountAddressDeleting != nil {
			m.AccountAddressDeleting = nil
		} else if m.AddressListFocused {
			m.AddressListFocused = false
			m.AccountAddressCursor = 0
		} else if m.AccountCardDeleting != nil {
			m.AccountCardDeleting = nil
		} else if m.CardListFocused {
			m.CardListFocused = false
			m.AccountCardCursor = 0
		} else if m.FaqFocused {
			m.FaqFocused = false
			m.ScrollOffset = 0
		} else if m.OrderViewState > 0 {
			m.OrderViewState--
			m.ScrollOffset = 0
		}
	}
	return m, nil
}
