package tui

import (
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
			detailView = m.OrdersView(detailContentWidth)
		case "addresses":
			detailView = m.AddressesView(detailContentWidth)
		case "cards":
			detailView = m.CardsView(detailContentWidth)
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
			detailView = m.AboutView(detailContentWidth)
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

func (m Model) AboutView(width int) string {
	titleStyle := m.theme.TextAccent().Bold(true).MarginBottom(2)
	contentStyle := m.theme.TextBody().Width(width)
	accentStyle := m.theme.TextHighlight().Bold(true)
	userStyle := m.theme.TextAccent().Bold(true)

	aboutContent := contentStyle.Render(wordWrap(
		"This project is heavily inspired by the wonderful team who made TerminalDotShop. I want to thank them for open-sourcing the code for that project. Cudos to those smart, funny, amazing looking devs:",
		width,
	)) + "\n\n"
	aboutContent += userStyle.Render("@thdxr") + "\n"
	aboutContent += userStyle.Render("@adamdotdev") + "\n"
	aboutContent += userStyle.Render("@theprimeagen") + "\n"
	aboutContent += userStyle.Render("@teej_dv") + "\n"
	aboutContent += userStyle.Render("@iamdavidhill") + "\n\n"
	aboutContent += contentStyle.Render("And me:") + "\n\n"
	aboutContent += userStyle.Render("@marcusDenslow") + "\n\n"
	aboutContent += contentStyle.Render(wordWrap(
		"This project tries to copy the idea of terminalDotShop, but with one twist: write everything in GO. this is my humble attempt",
		width,
	)) + "\n"
	aboutContent += accentStyle.Render("Terminal Products, Inc.")

	return titleStyle.Render("About") + "\n\n" + aboutContent

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
		} else if m.account.orderViewState == 0 && !m.account.addressListFocused && !m.account.cardListFocused && m.account.cursor < len(models.AccountMenuItems)-1 {
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
		}

	case "n":
		m.account.addressDeleting = nil
		m.account.cardDeleting = nil

	case "p", "enter":
		switch selectedItem {
		case "order history":
			if len(m.Orders) > 0 {
				if m.account.orderViewState == 0 {
					m.account.orderViewState = 1
					m.account.orderCursor = 0
					m.account.scrollOffset = 0
					m.footer = []footerCommand{
						{key: "j/k", value: "orders"},
						{key: "enter", value: "details"},
						{key: "esc", value: "back"},
						{key: "q", value: "quit"},
					}
				} else if m.account.orderViewState == 1 {
					m.account.orderViewState = 2
					m.account.scrollOffset = 0
					m.footer = []footerCommand{
						{key: "esc", value: "back"},
						{key: "s", value: "shop"},
						{key: "q", value: "quit"},
					}
				}
			}
		case "addresses":
			if !m.account.addressListFocused {
				m.account.addressListFocused = true
				m.account.addressCursor = 0
				m.footer = []footerCommand{
					{key: "j/k", value: "addresses"},
					{key: "x", value: "delete"},
					{key: "esc", value: "back"},
				}
			}
		case "cards":
			if !m.account.cardListFocused {
				m.account.cardListFocused = true
				m.account.cardCursor = 0
				m.footer = []footerCommand{
					{key: "j/k", value: "cards"},
					{key: "x", value: "delete"},
					{key: "esc", value: "back"},
				}
			}
		case "faq":
			if !m.account.faqFocused {
				m.account.faqFocused = true
				m.account.scrollOffset = 0
				m.footer = []footerCommand{
					{key: "j/k", value: "scroll"},
					{key: "esc", value: "back"},
					{key: "s", value: "shop"},
					{key: "c", value: "cart"},
					{key: "q", value: "quit"},
				}
			}
		}

	case "esc":
		accountDefaultFooter := []footerCommand{
			{key: "j/k", value: "navigate"},
			{key: "enter", value: "select"},
			{key: "s", value: "shop"},
			{key: "c", value: "cart"},
			{key: "?", value: "help"},
			{key: "q", value: "quit"},
		}
		if m.account.addressDeleting != nil {
			m.account.addressDeleting = nil
		} else if m.account.addressListFocused {
			m.account.addressListFocused = false
			m.account.addressCursor = 0
			m.footer = accountDefaultFooter
		} else if m.account.cardDeleting != nil {
			m.account.cardDeleting = nil
		} else if m.account.cardListFocused {
			m.account.cardListFocused = false
			m.account.cardCursor = 0
			m.footer = accountDefaultFooter
			m.footer = accountDefaultFooter
		} else if m.account.faqFocused {
			m.account.faqFocused = false
			m.account.scrollOffset = 0
			m.footer = accountDefaultFooter
		} else if m.account.orderViewState > 0 {
			m.account.orderViewState--
			m.account.scrollOffset = 0
			if m.account.orderViewState == 1 {
				m.footer = []footerCommand{
					{key: "j/k", value: "orders"},
					{key: "enter", value: "details"},
					{key: "esc", value: "back"},
					{key: "q", value: "quit"},
				}
			} else {
				m.footer = accountDefaultFooter
			}
		}
	}
	return m, nil
}
