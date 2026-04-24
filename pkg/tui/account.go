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
		if m.account.cursor == i {
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
	if m.account.cursor >= 0 && m.account.cursor < len(models.AccountMenuItems) {
		titleStyle := m.theme.TextAccent().Bold(true).MarginBottom(2)

		// On small terminals, use full content width
		detailContentWidth := rightWidth - 2
		if m.size < large {
			detailContentWidth = m.widthContent - 2
		}

		contentStyle := m.theme.TextBody().Width(detailContentWidth)

		selectedItem := models.AccountMenuItems[m.account.cursor]
		switch selectedItem {
		case "order history":
			if !m.OrdersLoaded {
				detailView = titleStyle.Render("Order History") + "\n\n" + contentStyle.Render("Loading orders...")
			} else if len(m.Orders) == 0 {
				detailView = titleStyle.Render("Order History") + "\n\n" + contentStyle.Render("No orders yet.")
			} else if m.account.orderViewState == 2 {
				// State 2: full detail for the selected order
				detailView = m.buildOrderDetailView(m.Orders[m.account.orderCursor], detailContentWidth)
			} else {
				// State 0 (preview) or State 1 (browsing with cursor)
				boxWidth := detailContentWidth - 4
				boxPadding := 0
				if m.heightContainer >= 25 {
					boxPadding = 1
				}

				lines := titleStyle.Render("Order History") + "\n\n"
				for i, order := range m.Orders {
					isSelected := m.account.orderViewState == 1 && i == m.account.orderCursor
					lines += m.buildOrderCard(order, boxWidth, isSelected, boxPadding, detailContentWidth)
					if i < len(m.Orders)-1 {
						lines += "\n"
					}
				}

				if m.account.orderViewState == 0 {
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
					isSelected := m.account.addressListFocused && i == m.account.addressCursor
					label := addr.Name + "  " + addr.Street + ", " + addr.City
					if len(label) > detailContentWidth-4 {
						label = label[:detailContentWidth-4]
					}
					if m.account.addressDeleting != nil && *m.account.addressDeleting == i {
						lines += m.theme.TextError().Bold(true).Render("  deletes? (y/n)") + "\n"
					} else if isSelected {
						lines += m.theme.TextAccent().Bold(true).Render("> "+label) + "\n"
					} else {
						lines += contentStyle.Render(" "+label) + "\n"
					}
				}
			}
			if m.account.addressListFocused {
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
					isSelected := m.account.cardListFocused && i == m.account.cardCursor
					label := fmt.Sprintf("**** **** **** %s  %s  exp %02d/%02d",
						card.Last4, card.Brand, card.ExpMonth, card.ExpYear%100)
					if m.account.cardDeleting != nil && *m.account.cardDeleting == i {
						lines += m.theme.TextError().Bold(true).Render("  delete? (y/n)") + "\n"
					} else if isSelected {
						lines += m.theme.TextAccent().Bold(true).Render("> "+label) + "\n"
					} else {
						lines += contentStyle.Render(" "+label) + "\n"
					}
				}
			}
			if m.account.cardListFocused {
				lines += "\n" + m.theme.TextDim().Render("x: delete  esc: back")
			} else {
				lines += "\n" + m.theme.TextDim().Render("enter: manage")
			}
			detailView = lines

		case "ssh keys":
			lines := titleStyle.Render("SSH keys") + "\n\n"
			if !m.SSHKeysLoaded {
				lines += contentStyle.Render("Loading ssh keys...")
			} else if len(m.SSHKeys) == 0 {
				lines += contentStyle.Render("No saved ssh keys.")
			} else {
				for i, key := range m.SSHKeys {
					isSelected := m.account.sshKeyListFocused && i == m.account.sshKeyCursor
					label := key.Fingerprint
					if key.Comment != "" {
						label += "  " + key.Comment
					}
					if len(label) > detailContentWidth-4 {
						label = label[:detailContentWidth-4]
					}
					if m.account.sshKeyDeleting != nil && *m.account.sshKeyDeleting == i {
						lines += m.theme.TextError().Bold(true).Render("  delete? (y/n)") + "\n"
					} else if isSelected {
						lines += m.theme.TextAccent().Bold(true).Render("> "+label) + "\n"
					} else {
						lines += contentStyle.Render(" "+label) + "\n"
					}
				}
			}
			if m.account.sshKeyListFocused {
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
			if !m.account.faqFocused {
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
	if m.account.orderViewState >= 1 || m.account.faqFocused {
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
		offset := m.account.scrollOffset
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
	if m.account.cursor < len(models.AccountMenuItems) {
		selectedItem = models.AccountMenuItems[m.account.cursor]
	}

	switch keyMsg.String() {
	case "up", "k":
		if m.account.addressListFocused && m.account.addressDeleting == nil {
			if m.account.addressCursor > 0 {
				m.account.addressCursor--
			}
		} else if m.account.cardListFocused && m.account.cardDeleting == nil {
			if m.account.cardCursor > 0 {
				m.account.cardCursor--
			}
		} else if m.account.sshKeyListFocused && m.account.sshKeyDeleting == nil {
			if m.account.sshKeyCursor > 0 {
				m.account.sshKeyCursor--
			}
		} else if m.account.faqFocused {
			m.account.scrollOffset -= 3
			if m.account.scrollOffset < 0 {
				m.account.scrollOffset = 0
			}
		} else if m.account.orderViewState == 1 && m.account.orderCursor > 0 {
			m.account.orderCursor--
			cardHeight := 5
			if m.heightContainer >= 25 {
				cardHeight = 7
			}
			targetTop := 3 + m.account.orderCursor*cardHeight
			if targetTop < m.account.scrollOffset {
				m.account.scrollOffset = targetTop
			}
		} else if m.account.orderViewState == 0 && m.account.cursor > 0 {
			m.account.cursor--
			m.account.scrollOffset = 0
		}

	case "down", "j":
		if m.account.addressListFocused && m.account.addressDeleting == nil {
			if m.account.addressCursor < len(m.SavedAddresses)-1 {
				m.account.addressCursor++
			}
		} else if m.account.cardListFocused && m.account.cardDeleting == nil {
			if m.account.cardCursor < len(m.SavedCards)-1 {
				m.account.cardCursor++
			}
		} else if m.account.sshKeyListFocused && m.account.sshKeyDeleting == nil {
			if m.account.sshKeyCursor < len(m.SSHKeys)-1 {
				m.account.sshKeyCursor++
			}
		} else if m.account.faqFocused {
			m.account.scrollOffset += 3
			maxScroll := m.computeFaqScrollMax()
			if m.account.scrollOffset > maxScroll {
				m.account.scrollOffset = maxScroll
			}
		} else if m.account.orderViewState == 1 && m.account.orderCursor < len(m.Orders)-1 {
			m.account.orderCursor++
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
			targetBottom := 3 + (m.account.orderCursor+1)*cardHeight
			if targetBottom > m.account.scrollOffset+viewportHeight {
				m.account.scrollOffset = targetBottom - viewportHeight
			}
		} else if m.account.orderViewState == 0 && !m.account.addressListFocused && !m.account.cardListFocused && !m.account.sshKeyListFocused && m.account.cursor < len(models.AccountMenuItems)-1 {
			m.account.cursor++
			m.account.scrollOffset = 0
		}

	case "pgup", "ctrl+u":
		if m.account.faqFocused || m.account.orderViewState >= 2 {
			m.account.scrollOffset -= 3
			if m.account.scrollOffset < 0 {
				m.account.scrollOffset = 0
			}
		}

	case "pgdown", "ctrl+d":
		if m.account.faqFocused || m.account.orderViewState >= 2 {
			m.account.scrollOffset += 3
			maxScroll := m.computeFaqScrollMax()
			if m.account.faqFocused && maxScroll >= 0 && m.account.scrollOffset > maxScroll {
				m.account.scrollOffset = maxScroll
			}
		}

	case "x", "d":
		if m.account.addressListFocused && m.account.addressDeleting == nil && m.account.addressCursor < len(m.SavedAddresses) {
			m.account.addressDeleting = &m.account.addressCursor
		} else if m.account.cardListFocused && m.account.cardDeleting == nil && m.account.cardCursor < len(m.SavedCards) {
			m.account.cardDeleting = &m.account.cardCursor
		} else if m.account.sshKeyListFocused && m.account.sshKeyDeleting == nil && m.account.sshKeyCursor < len(m.SSHKeys) {
			m.account.sshKeyDeleting = &m.account.sshKeyCursor
		}

	case "y":
		if m.account.addressDeleting != nil {
			cursor := *m.account.addressDeleting
			addr := m.SavedAddresses[cursor]
			m.account.addressDeleting = nil
			m.SavedAddresses = append(m.SavedAddresses[:cursor], m.SavedAddresses[cursor+1:]...)
			if m.account.addressCursor >= len(m.SavedAddresses) && m.account.addressCursor > 0 {
				m.account.addressCursor--
			}
			return m, m.deleteAddressCmd(addr.ID)
		} else if m.account.cardDeleting != nil {
			cursor := *m.account.cardDeleting
			card := m.SavedCards[cursor]
			m.account.cardDeleting = nil
			m.SavedCards = append(m.SavedCards[:cursor], m.SavedCards[cursor+1:]...)
			if m.account.cardCursor >= len(m.SavedCards) && m.account.cardCursor > 0 {
				m.account.cardCursor--
			}
			return m, m.deleteCardCmd(card.ID)
		} else if m.account.sshKeyDeleting != nil {
			cursor := *m.account.sshKeyDeleting
			key := m.SSHKeys[cursor]
			m.account.sshKeyDeleting = nil
			m.SSHKeys = append(m.SSHKeys[:cursor], m.SSHKeys[cursor+1:]...)
			if m.account.sshKeyCursor >= len(m.SSHKeys) && m.account.sshKeyCursor > 0 {
				m.account.sshKeyCursor--
			}
			return m, m.deleteSSHKeyCmd(key.ID)
		}

	case "n":
		m.account.addressDeleting = nil
		m.account.cardDeleting = nil
		m.account.sshKeyDeleting = nil

	case "p", "enter":
		switch selectedItem {
		case "order history":
			if len(m.Orders) > 0 {
				if m.account.orderViewState == 0 {
					m.account.orderViewState = 1
					m.account.orderCursor = 0
					m.account.scrollOffset = 0
				} else if m.account.orderViewState == 1 {
					m.account.orderViewState = 2
					m.account.scrollOffset = 0
				}
			}
		case "ssh keys":
			if !m.account.sshKeyListFocused {
				m.account.sshKeyListFocused = true
				m.account.sshKeyCursor = 0
			}
		case "addresses":
			if !m.account.addressListFocused {
				m.account.addressListFocused = true
				m.account.addressCursor = 0
			}
		case "cards":
			if !m.account.cardListFocused {
				m.account.cardListFocused = true
				m.account.cardCursor = 0
			}
		case "faq":
			if !m.account.faqFocused {
				m.account.faqFocused = true
				m.account.scrollOffset = 0
			}
		}

	case "esc":
		if m.account.addressDeleting != nil {
			m.account.addressDeleting = nil
		} else if m.account.addressListFocused {
			m.account.addressListFocused = false
			m.account.addressCursor = 0
		} else if m.account.cardDeleting != nil {
			m.account.cardDeleting = nil
		} else if m.account.cardListFocused {
			m.account.cardListFocused = false
			m.account.cardCursor = 0
		} else if m.account.sshKeyDeleting != nil {
			m.account.sshKeyDeleting = nil
		} else if m.account.sshKeyListFocused {
			m.account.sshKeyListFocused = false
			m.account.sshKeyCursor = 0
		} else if m.account.faqFocused {
			m.account.faqFocused = false
			m.account.scrollOffset = 0
		} else if m.account.orderViewState > 0 {
			m.account.orderViewState--
			m.account.scrollOffset = 0
		}
	}
	return m, nil
}
