package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"terminalShop/pkg/models"
)

// updateAccountViewport initializes or resizes the detail viewport for the account page.
// Follows the reference pattern: each page owns its own viewport(s).
func (m Model) updateAccountViewport() Model {
	headerHeight := lipgloss.Height(m.BuildHeader())
	footerHeight := 1
	marginTop := 1
	marginBottom := 1
	bufferSpace := 1
	verticalMarginHeight := headerHeight + footerHeight + marginTop + marginBottom + bufferSpace

	availableHeight := m.heightContainer - verticalMarginHeight

	leftWidth := 18
	rightWidth := m.widthContent - leftWidth - 2
	detailWidth := rightWidth
	if m.size < large {
		detailWidth = m.widthContent
		// In stacked mode, reduce available height by the left panel
		leftPanelHeight := len(models.AccountMenuItems) + 1
		availableHeight -= leftPanelHeight
	}
	if availableHeight < 3 {
		availableHeight = 3
	}

	if !m.account.viewportReady {
		m.account.detailViewport = viewport.New(detailWidth, availableHeight)
		m.account.detailViewport.KeyMap = modifiedKeyMap
		m.account.viewportReady = true
	} else {
		m.account.detailViewport.Width = detailWidth
		m.account.detailViewport.Height = availableHeight
	}

	return m
}

// getAccountDetailContent generates the content string for the current detail panel.
func (m Model) getAccountDetailContent() string {
	leftWidth := 18
	rightWidth := m.widthContent - leftWidth - 2
	detailContentWidth := rightWidth - 2
	if m.size < large {
		detailContentWidth = m.widthContent - 2
	}

	if m.account.cursor < 0 || m.account.cursor >= len(models.AccountMenuItems) {
		return ""
	}

	selectedItem := models.AccountMenuItems[m.account.cursor]
	switch selectedItem {
	case "order history":
		return m.OrdersView(detailContentWidth)
	case "addresses":
		return m.AddressesView(detailContentWidth)
	case "cards":
		return m.CardsView(detailContentWidth)
	case "faq":
		titleStyle := m.theme.TextAccent().Bold(true).MarginBottom(2)
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
		detailView := titleStyle.Render("FAQ") + "\n\n" + faqContent
		if !m.account.faqFocused {
			hintStyle := m.theme.TextDim()
			detailView += "\n\n" + hintStyle.Render("enter: scroll faq")
		}
		return detailView
	case "about":
		return m.AboutView(detailContentWidth)
	}
	return ""
}

// scrollToAccountDetailItem adjusts the viewport scroll to keep the selected item visible.
// Follows the reference pattern: cursor moves immediately, viewport catches up.
func (m Model) scrollToAccountDetailItem() Model {
	var itemHeight, itemCount, selectedIndex int

	switch {
	case m.account.orderViewState == 1:
		itemHeight = 7
		if m.heightContainer < 25 {
			itemHeight = 5
		}
		itemCount = len(m.Orders)
		selectedIndex = m.account.orderCursor
	case m.account.addressListFocused:
		itemHeight = 5
		itemCount = len(m.SavedAddresses)
		selectedIndex = m.account.addressCursor
	case m.account.cardListFocused:
		itemHeight = 4
		itemCount = len(m.SavedCards)
		selectedIndex = m.account.cardCursor
	default:
		return m
	}

	if itemCount == 0 {
		return m
	}

	targetY := (selectedIndex * itemHeight) + 2
	vpH := m.account.detailViewport.Height
	offset := m.account.detailViewport.YOffset

	if targetY < offset {
		m.account.detailViewport.SetYOffset(targetY - 2)
	}
	if targetY+itemHeight > offset+vpH {
		m.account.detailViewport.SetYOffset(targetY - vpH + itemHeight)
	}

	return m
}

func (m Model) BuildAccountView(availableHeight int) string {
	leftWidth := 18

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

	// Generate detail content and set it on the viewport
	detailContent := m.getAccountDetailContent()
	m.account.detailViewport.SetContent(detailContent)

	// Viewport handles scrolling/clipping — no manual line-slicing needed
	detailRendered := m.account.detailViewport.View()

	// Responsive layout: side-by-side on large, stacked on small/medium
	if m.size < large {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			leftContainer.Render(leftPanel),
			detailRendered,
		)
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftContainer.Render(leftPanel),
		"  ",
		detailRendered,
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

func (m Model) AccountUpdate(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		// Pass non-key messages to viewport (e.g. window resize)
		var cmd tea.Cmd
		m.account.detailViewport, cmd = m.account.detailViewport.Update(msg)
		return m, cmd
	}

	selectedItem := ""
	if m.account.cursor < len(models.AccountMenuItems) {
		selectedItem = models.AccountMenuItems[m.account.cursor]
	}

	switch keyMsg.String() {
	case "up", "k":
		// In order detail or FAQ, scroll the viewport directly
		if m.account.orderViewState == 2 || m.account.faqFocused {
			m.account.detailViewport.ScrollUp(3)
			return m, nil
		}
		if m.account.addressListFocused && m.account.addressDeleting == nil {
			if m.account.addressCursor > 0 {
				m.account.addressCursor--
				m = m.scrollToAccountDetailItem()
			}
		} else if m.account.cardListFocused && m.account.cardDeleting == nil {
			if m.account.cardCursor > 0 {
				m.account.cardCursor--
				m = m.scrollToAccountDetailItem()
			}
		} else if m.account.orderViewState == 1 && m.account.orderCursor > 0 {
			m.account.orderCursor--
			m = m.scrollToAccountDetailItem()
		} else if m.account.orderViewState == 0 && m.account.cursor > 0 {
			m.account.cursor--
			m.account.detailViewport.GotoTop()
		}

	case "down", "j":
		if m.account.orderViewState == 2 || m.account.faqFocused {
			m.account.detailViewport.ScrollDown(3)
			return m, nil
		}
		if m.account.addressListFocused && m.account.addressDeleting == nil {
			if m.account.addressCursor < len(m.SavedAddresses)-1 {
				m.account.addressCursor++
				m = m.scrollToAccountDetailItem()
			}
		} else if m.account.cardListFocused && m.account.cardDeleting == nil {
			if m.account.cardCursor < len(m.SavedCards)-1 {
				m.account.cardCursor++
				m = m.scrollToAccountDetailItem()
			}
		} else if m.account.orderViewState == 1 && m.account.orderCursor < len(m.Orders)-1 {
			m.account.orderCursor++
			m = m.scrollToAccountDetailItem()
		} else if m.account.orderViewState == 0 && !m.account.addressListFocused && !m.account.cardListFocused && m.account.cursor < len(models.AccountMenuItems)-1 {
			m.account.cursor++
			m.account.detailViewport.GotoTop()
		}

	case "pgup", "ctrl+u", "pgdown", "ctrl+d":
		// Let viewport handle page scrolling directly
		var cmd tea.Cmd
		m.account.detailViewport, cmd = m.account.detailViewport.Update(msg)
		return m, cmd

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
					m.account.detailViewport.GotoTop()
					m.footer = []footerCommand{
						{key: "j/k", value: "orders"},
						{key: "enter", value: "details"},
						{key: "esc", value: "back"},
						{key: "q", value: "quit"},
					}
				} else if m.account.orderViewState == 1 {
					// Save scroll position before entering detail view
					m.account.orderYOffset = m.account.detailViewport.YOffset
					m.account.orderViewState = 2
					m.account.detailViewport.GotoTop()
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
				m.account.detailViewport.GotoTop()
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
		} else if m.account.faqFocused {
			m.account.faqFocused = false
			m.account.detailViewport.GotoTop()
			m.footer = accountDefaultFooter
		} else if m.account.orderViewState > 0 {
			m.account.orderViewState--
			if m.account.orderViewState == 1 {
				// Restore scroll position from before entering detail view
				if m.account.orderYOffset > 0 {
					m.account.detailViewport.SetYOffset(m.account.orderYOffset)
					m.account.orderYOffset = 0
				}
				m.footer = []footerCommand{
					{key: "j/k", value: "orders"},
					{key: "enter", value: "details"},
					{key: "esc", value: "back"},
					{key: "q", value: "quit"},
				}
			} else {
				m.account.detailViewport.GotoTop()
				m.footer = accountDefaultFooter
			}
		}
	}
	return m, nil
}
