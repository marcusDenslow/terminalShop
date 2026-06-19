package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"terminalShop/pkg/models"
)

// accountBodyHeight returns the inner content height of the account body
// panels. The body is quantized to whole order cards so the menu panel's
// bottom border lines up exactly with the last visible card; the two gap
// rows around the body host the up/down scroll arrows.
func (m Model) accountBodyHeight() int {
	headerHeight := lipgloss.Height(m.BuildHeader())
	footerHeight := lipgloss.Height(m.BuildFooter())
	budget := m.heightContainer - headerHeight - footerHeight - 2 - 1
	cards := budget / orderCardHeight
	cards = max(cards, 1)
	return cards*orderCardHeight - 2
}

// accountDetailWidth returns the width of the unboxed detail area: the chrome
// width minus the menu panel and the gap. The detail side has no outer panel
// (its content draws its own boxes), so it reclaims the border + padding.
func (m Model) accountDetailWidth() int {
	contentW, menuW, _, _ := m.shopLayout()
	if m.size < large {
		return contentW
	}
	return contentW - (menuW + 4) - 1
}

// updateAccountViewport initializes or resizes the detail viewport for the account page.
// The viewport spans the unboxed detail area to the right of the menu panel.
func (m Model) updateAccountViewport() Model {
	detailW := m.accountDetailWidth()
	// +2: no panel on the detail side, so the viewport spans the same rows
	// as the menu panel including its border rows — tops and bottoms align.
	availableHeight := m.accountBodyHeight() + 2
	if m.size < large {
		// Stacked: the menu panel sits above the detail area
		menuPanelHeight := len(models.AccountMenuItems) + 2
		availableHeight -= menuPanelHeight
		if availableHeight < 3 {
			availableHeight = 3
		}
	}

	if !m.account.viewportReady {
		m.account.detailViewport = viewport.New(viewport.WithWidth(detailW), viewport.WithHeight(availableHeight))
		m.account.detailViewport.KeyMap = modifiedKeyMap
		m.account.viewportReady = true
	} else {
		m.account.detailViewport.SetWidth(detailW)
		m.account.detailViewport.SetHeight(availableHeight)
	}

	return m
}

// getAccountDetailContent generates the content string for the current detail panel.
func (m Model) getAccountDetailContent() string {
	detailContentWidth := m.accountDetailWidth()

	if m.account.cursor < 0 || m.account.cursor >= len(models.AccountMenuItems) {
		return ""
	}

	selectedItem := models.AccountMenuItems[m.account.cursor]
	switch selectedItem {
	case "active orders":
		return m.ActiveOrdersView(detailContentWidth)
	case "order history":
		return m.OrdersView(detailContentWidth)
	case "addresses":
		return m.AddressesView(detailContentWidth)
	case "cards":
		return m.CardsView(detailContentWidth)
	case "spend limit":
		return m.SpendLimitView(detailContentWidth)
	case "privacy":
		return m.PrivacyView(detailContentWidth)
	case "faq":
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
		detailView := faqContent
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
//
// Each sub-page has a title header (e.g. "Order History" + margin + newlines) that
// pushes the first item down. headerOffset accounts for those lines. itemHeight
// must match the actual rendered card height including borders, padding, and the
// inter-card gap ("\n" between cards in the view function).
func (m Model) scrollToAccountDetailItem() Model {
	var itemHeight, itemCount, selectedIndex, headerOffset int

	switch {
	case m.account.orderViewState == 1:
		// Order list uses whole-card windowing instead of viewport scroll:
		// shift the window just enough to keep the cursor's card visible.
		page := m.orderListPageSize()
		if m.account.orderCursor < m.account.orderWindowStart {
			m.account.orderWindowStart = m.account.orderCursor
		}
		if m.account.orderCursor >= m.account.orderWindowStart+page {
			m.account.orderWindowStart = m.account.orderCursor - page + 1
		}
		m.account.detailViewport.GotoTop()
		return m
	case m.account.addressListFocused:
		// Titles were removed from the detail views, lists start at the top
		headerOffset = 0
		itemHeight = 5
		itemCount = len(m.SavedAddresses)
		selectedIndex = m.account.addressCursor
	case m.account.cardListFocused:
		headerOffset = 0
		itemHeight = 4
		itemCount = len(m.SavedCards)
		selectedIndex = m.account.cardCursor
	default:
		return m
	}

	if itemCount == 0 {
		return m
	}

	targetY := headerOffset + (selectedIndex * itemHeight)
	vpH := m.account.detailViewport.Height()
	offset := m.account.detailViewport.YOffset()

	// 1-item buffer: scroll when the NEXT item would be off-screen,
	// not when the current item hits the very edge
	if targetY-itemHeight < offset {
		m.account.detailViewport.SetYOffset(targetY - itemHeight)
	}
	if targetY+itemHeight+itemHeight > offset+vpH {
		m.account.detailViewport.SetYOffset(targetY - vpH + itemHeight + itemHeight)
	}

	return m
}

func (m Model) BuildAccountView() string {
	_, menuW, _, _ := m.shopLayout()

	// Left panel: menu items. The highlight is blue while the user navigates
	// the menu, and falls back to the gray pill when focus has moved into the
	// detail area (browsing orders, managing addresses/cards, scrolling faq).
	focusedInside := m.account.orderViewState > 0 || m.account.addressListFocused ||
		m.account.cardListFocused || m.account.faqFocused || m.account.spendLimitFocused
	pillBg := m.theme.Highlight()
	if focusedInside {
		pillBg = cPill
	}
	var menu []string
	for i, item := range models.AccountMenuItems {
		if m.account.cursor == i {
			menu = append(menu, lipgloss.NewStyle().Width(menuW).
				Background(pillBg).Foreground(cWhite).Bold(true).
				Render(" "+item))
		} else {
			menu = append(menu, lipgloss.NewStyle().Width(menuW).
				Render(" "+pBody.Render(item)))
		}
	}
	menuContent := lipgloss.JoinVertical(lipgloss.Left, menu...)

	// Generate detail content and set it on the viewport
	detailContent := m.getAccountDetailContent()
	m.account.detailViewport.SetContent(detailContent)

	// Viewport handles scrolling/clipping — no manual line-slicing needed
	detailRendered := m.account.detailViewport.View()

	// Responsive layout: menu panel beside the unboxed detail area on large,
	// stacked on small/medium. The detail side draws its own item boxes.
	var body string
	if m.size < large {
		body = lipgloss.JoinVertical(
			lipgloss.Left,
			panel(menuW, 0, menuContent),
			detailRendered,
		)
	} else {
		bodyH := m.accountBodyHeight()
		body = lipgloss.JoinHorizontal(
			lipgloss.Top,
			panel(menuW, bodyH, menuContent),
			" ",
			detailRendered,
		)
	}

	// Blank rows separate the body from the header and footer boxes. When the
	// windowed order list has hidden items, the arrows occupy those gap rows,
	// centered over the card column, so the cards themselves always span
	// exactly the menu panel's rows — top and bottom borders aligned.
	topGap := ""
	if above := m.orderHiddenAbove(); above > 0 && m.size >= large {
		topGap = strings.Repeat(" ", menuW+5) +
			orderScrollIndicator("↑", above, m.accountDetailWidth())
	}
	bottomGap := ""
	if below := m.orderHiddenBelow(); below > 0 && m.size >= large {
		bottomGap = strings.Repeat(" ", menuW+5) +
			orderScrollIndicator("↓", below, m.accountDetailWidth())
	}
	return topGap + "\n" + body + "\n" + bottomGap
}

func (m Model) AboutView(width int) string {
	contentStyle := m.theme.TextBody().Width(width)
	// USE this if wanting to add a corp name later
	// accentStyle := m.theme.TextHighlight().Bold(true)
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

	return aboutContent
}

func (m Model) AccountUpdate(msg tea.Msg) (Model, tea.Cmd) {
	// Async result of a spend-limit save (delivered here because the account
	// page is active when the command resolves).
	if saved, ok := msg.(spendLimitSavedMsg); ok {
		m.account.spendLimitSaving = false
		if saved.Err != nil {
			m.account.spendLimitErr = friendlySpendLimitError(saved.Err)
			return m, nil
		}
		if m.User != nil {
			m.User.SelfLimitCents = saved.Cents
		}
		m.account.spendLimitFocused = false
		m.account.spendLimitInput.Blur()
		m.account.spendLimitErr = ""
		m.footer = defaultAccountFooter()
		m.notice = &VisibleNotice{message: "Spend limit updated"}
		return m, nil
	}

	if saved, ok := msg.(privacyModeSavedMsg); ok {
		m.account.privacyModeSaving = false
		if saved.Err != nil {
			m.account.privacyModeErr = saved.Err.Error()
			return m, nil
		}
		if m.User != nil {
			m.User.PrivacyMode = saved.Enabled
		}
		m.account.privacyModeErr = ""
		state := "off"
		if saved.Enabled {
			state = "on"
		}
		m.notice = &VisibleNotice{message: "Privacy mode " + state}
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		// Pass non-key messages to viewport (e.g. window resize)
		var cmd tea.Cmd
		m.account.detailViewport, cmd = m.account.detailViewport.Update(msg)
		return m, cmd
	}

	// While the spend-limit input is focused it owns the keyboard: enter saves,
	// esc cancels, everything else edits the field.
	if m.account.spendLimitFocused {
		switch keyMsg.String() {
		case "esc":
			m.account.spendLimitFocused = false
			m.account.spendLimitInput.Blur()
			m.account.spendLimitErr = ""
			m.footer = defaultAccountFooter()
			return m, nil
		case "enter":
			return m.submitSpendLimit()
		}
		var cmd tea.Cmd
		m.account.spendLimitInput, cmd = m.account.spendLimitInput.Update(msg)
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
			m.account.detailViewport.SetContent(m.getAccountDetailContent())
			m.account.detailViewport.ScrollUp(3)
			return m, nil
		}
		if m.account.addressListFocused && m.account.addressDeleting == nil {
			if m.account.addressCursor > 0 {
				m.account.addressCursor--
				m.account.detailViewport.SetContent(m.getAccountDetailContent())
				m = m.scrollToAccountDetailItem()
			}
		} else if m.account.cardListFocused && m.account.cardDeleting == nil {
			if m.account.cardCursor > 0 {
				m.account.cardCursor--
				m.account.detailViewport.SetContent(m.getAccountDetailContent())
				m = m.scrollToAccountDetailItem()
			}
		} else if m.account.orderViewState == 1 && m.account.orderCursor > 0 {
			m.account.orderCursor--
			m.account.detailViewport.SetContent(m.getAccountDetailContent())
			m = m.scrollToAccountDetailItem()
		} else if m.account.orderViewState == 0 && m.account.cursor > 0 {
			m.account.cursor--
			m.account.detailViewport.GotoTop()
		}

	case "down", "j":
		if m.account.orderViewState == 2 || m.account.faqFocused {
			m.account.detailViewport.SetContent(m.getAccountDetailContent())
			m.account.detailViewport.ScrollDown(3)
			return m, nil
		}
		if m.account.addressListFocused && m.account.addressDeleting == nil {
			if m.account.addressCursor < len(m.SavedAddresses)-1 {
				m.account.addressCursor++
				m.account.detailViewport.SetContent(m.getAccountDetailContent())
				m = m.scrollToAccountDetailItem()
			}
		} else if m.account.cardListFocused && m.account.cardDeleting == nil {
			if m.account.cardCursor < len(m.SavedCards)-1 {
				m.account.cardCursor++
				m.account.detailViewport.SetContent(m.getAccountDetailContent())
				m = m.scrollToAccountDetailItem()
			}
		} else if m.account.orderViewState == 1 && m.account.orderCursor < len(m.currentOrderList())-1 {
			m.account.orderCursor++
			m.account.detailViewport.SetContent(m.getAccountDetailContent())
			m = m.scrollToAccountDetailItem()
		} else if m.account.orderViewState == 0 && !m.account.addressListFocused && !m.account.cardListFocused && m.account.cursor < len(models.AccountMenuItems)-1 {
			m.account.cursor++
			m.account.detailViewport.GotoTop()
		}

	case "pgup", "ctrl+u", "pgdown", "ctrl+d":
		// Ensure viewport has content before delegating scroll
		m.account.detailViewport.SetContent(m.getAccountDetailContent())
		var cmd tea.Cmd
		m.account.detailViewport, cmd = m.account.detailViewport.Update(msg)
		return m, cmd

	case "r":
		if m.account.orderViewState == 2 {
			if order, ok := m.selectedRefundableOrder(); ok {
				return m.OpenRefundComposer(order.ID)
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
		case "order history", "active orders":
			if len(m.currentOrderList()) > 0 {
				switch m.account.orderViewState {
				case 0:
					m.account.orderViewState = 1
					m.account.orderCursor = 0
					m.account.orderWindowStart = 0
					m.account.detailViewport.GotoTop()
					m.footer = []footerCommand{
						{key: "j/k", value: "orders"},
						{key: "enter", value: "details"},
						{key: "esc", value: "back"},
						{key: "q", value: "quit"},
					}
				case 1:
					m.account.orderYOffset = m.account.detailViewport.YOffset()
					m.account.orderViewState = 2
					m.account.detailViewport.GotoTop()
					footer := []footerCommand{
						{key: "esc", value: "back"},
						{key: "s", value: "shop"},
						{key: "q", value: "quit"},
					}
					if _, ok := m.selectedRefundableOrder(); ok {
						footer = append([]footerCommand{{key: "r", value: "refund"}}, footer...)
					}
					m.footer = footer
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
		case "spend limit":
			if !m.account.spendLimitFocused {
				ti := textinput.New()
				ti.Prompt = "$"
				ti.Placeholder = "dollars (blank = no limit)"
				ti.CharLimit = 12
				// Prefill with the current limit so the user edits rather than retypes.
				if m.User != nil && m.User.SelfLimitCents != nil {
					ti.SetValue(formatCentsDollars(*m.User.SelfLimitCents))
				}
				cmd := ti.Focus()
				m.account.spendLimitInput = ti
				m.account.spendLimitFocused = true
				m.account.spendLimitErr = ""
				m.footer = []footerCommand{
					{key: "enter", value: "save"},
					{key: "esc", value: "cancel"},
				}
				return m, cmd
			}
		case "privacy":
			return m.togglePrivacyMode()
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
		accountDefaultFooter := defaultAccountFooter()
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
				// Set content first so SetYOffset can clamp against correct line count
				m.account.detailViewport.SetContent(m.getAccountDetailContent())
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

// spendLimitSavedMsg carries the result of an async SetSpendLimit call back to
// AccountUpdate. Cents is the value that was sent (echoed so the view can update
// without a refetch); Err is non-nil when the save failed.
type spendLimitSavedMsg struct {
	Cents *int
	Err   error
}

// SpendLimitView renders the self-service spend-limit panel: the current limit,
// and — when focused — the editable input. Layout follows the label/box idiom
// used by the refund composer (pkg/tui/refund.go).
func (m Model) SpendLimitView(width int) string {
	labelStyle := m.theme.TextLabel()
	bodyStyle := m.theme.TextBody().Width(width)
	hintStyle := m.theme.TextDim()

	current := "no limit set"
	if m.User != nil && m.User.SelfLimitCents != nil {
		current = "$" + formatCentsDollars(*m.User.SelfLimitCents)
	}

	out := bodyStyle.Render(wordWrap(
		"Set your own spending limit. It can only lower the cap on your account, never raise it. Leave blank to remove your limit; 0 blocks all orders.",
		width,
	)) + "\n\n"
	out += labelStyle.Render("Current limit") + "\n"
	out += m.theme.TextAccent().Render(current) + "\n\n"

	if !m.account.spendLimitFocused {
		return out + hintStyle.Render("enter: edit limit")
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Highlight()).
		Width(max(1, width-2))
	m.account.spendLimitInput.SetWidth(max(1, width-3))
	out += labelStyle.Render("New limit (dollars)") + "\n"
	out += box.Render(m.account.spendLimitInput.View())
	switch {
	case m.account.spendLimitSaving:
		out += "\n" + m.theme.TextAccent().Render("saving...")
	case m.account.spendLimitErr != "":
		out += "\n" + m.theme.TextError().Render(m.account.spendLimitErr)
	}
	out += "\n" + hintStyle.Render("enter: save · esc: cancel")
	return out
}

// submitSpendLimit validates the input and, if valid, fires the save command.
// Validation mirrors the server's write-boundary rules so the user gets instant
// feedback for the negative/non-numeric cases before any round-trip.
func (m Model) submitSpendLimit() (Model, tea.Cmd) {
	cents, err := parseSpendLimitInput(m.account.spendLimitInput.Value())
	if err != nil {
		m.account.spendLimitErr = err.Error()
		return m, nil
	}
	m.account.spendLimitErr = ""
	m.account.spendLimitSaving = true
	return m, m.setSpendLimitCmd(cents)
}

// setSpendLimitCmd calls the API off the UI thread and reports back via
// spendLimitSavedMsg. Mirrors the command shape of createRefundRequestCmd.
func (m Model) setSpendLimitCmd(cents *int) tea.Cmd {
	return func() tea.Msg {
		if m.APIClient == nil || m.User == nil {
			return spendLimitSavedMsg{Err: fmt.Errorf("not authenticated")}
		}
		err := m.APIClient.SetSpendLimit(cents)
		return spendLimitSavedMsg{Cents: cents, Err: err}
	}
}

// privacyModeSavedMsg carries the result of an async SetPrivacyMode call back to
// AccountUpdate. Enabled echoes the value sent so the view updates without a
// refetch; Err is non-nil when the save failed.
type privacyModeSavedMsg struct {
	Enabled bool
	Err     error
}

// PrivacyView renders the account-level privacy toggle ("keep as little as
// possible"). A soft default: the checkout can still override it per order.
func (m Model) PrivacyView(width int) string {
	labelStyle := m.theme.TextLabel()
	bodyStyle := m.theme.TextBody().Width(width)
	hintStyle := m.theme.TextDim()

	state := "off"
	if m.User != nil && m.User.PrivacyMode {
		state = "on"
	}

	// Privacy copy is split into short labelled sections instead of one wall of
	// text, reusing the same labelStyle/bodyStyle pairing as the rest of the
	// account view so it reads as scannable blocks.
	sections := []struct{ title, body string }{
		{
			"Philosophy",
			"Regardless of privacy mode, I never keep your full card number. Full stop.",
		},
		{
			"Privacy disabled",
			"Only the last 4 digits, card brand and a payment token is stored",
		},
		{
			"Privacy enabled",
			"With privacy mode ENABLED checkout uses a one-time card. All of the data is wiped the millisecond you pay",
		},
		{
			"Your address",
			"This one I have to keep. The law makes me, sometimes for years. So I store it offline, on my own network, where only I can reach it. Sorry about that.",
		},
	}

	out := ""
	for _, s := range sections {
		out += labelStyle.Render(s.title) + "\n"
		out += bodyStyle.Render(wordWrap(s.body, width)) + "\n\n"
	}
	out += labelStyle.Render("Privacy mode") + "\n"
	out += m.theme.TextAccent().Render(state) + "\n\n"

	switch {
	case m.account.privacyModeSaving:
		out += m.theme.TextAccent().Render("saving...") + "\n"
	case m.account.privacyModeErr != "":
		out += m.theme.TextError().Render(m.account.privacyModeErr) + "\n"
	}
	return out + hintStyle.Render("enter: toggle")
}

// togglePrivacyMode flips the account privacy default and fires the save command.
func (m Model) togglePrivacyMode() (Model, tea.Cmd) {
	if m.account.privacyModeSaving {
		return m, nil
	}
	current := m.User != nil && m.User.PrivacyMode
	m.account.privacyModeErr = ""
	m.account.privacyModeSaving = true
	return m, m.setPrivacyModeCmd(!current)
}

// setPrivacyModeCmd calls the API off the UI thread and reports back via
// privacyModeSavedMsg. Mirrors setSpendLimitCmd.
func (m Model) setPrivacyModeCmd(on bool) tea.Cmd {
	return func() tea.Msg {
		if m.APIClient == nil || m.User == nil {
			return privacyModeSavedMsg{Err: fmt.Errorf("not authenticated")}
		}
		err := m.APIClient.SetPrivacyMode(on)
		return privacyModeSavedMsg{Enabled: on, Err: err}
	}
}

// parseSpendLimitInput maps the raw dollar input to a *int of cents. Blank
// clears the limit (nil -> revert to admin/global). "0" is a real "block
// everything" ceiling, NOT a clear. Negative, non-numeric, or more-than-2-
// decimal input is rejected. Parsed as integer cents (dollars*100 + cents) with
// no float multiply, so an amount like 50.01 never loses a cent to binary
// rounding. The server still stores cents (api/handlers/account.go).
func parseSpendLimitInput(s string) (*int, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSpace(strings.TrimPrefix(s, "$"))
	if s == "" {
		return nil, nil
	}
	if strings.HasPrefix(s, "-") {
		return nil, fmt.Errorf("limit cannot be negative")
	}

	whole, frac, hasDot := strings.Cut(s, ".")
	if whole == "" {
		whole = "0" // allow ".50"
	}
	dollars, err := strconv.Atoi(whole)
	if err != nil {
		return nil, fmt.Errorf("enter a dollar amount, e.g 25 or 25.50, or blank to clear")
	}

	cents := 0
	if hasDot {
		switch len(frac) {
		case 0:
			// trailing dot, e.g "25." -> treat as .00
		case 1:
			frac += "0" // "5" -> 50 cents
			fallthrough
		case 2:
			cents, err = strconv.Atoi(frac)
			if err != nil {
				return nil, fmt.Errorf("enter a dollar amount, e.g 25 or 25.00, or blank to clear")
			}
		default:
			return nil, fmt.Errorf("use at most two decimal places, e.g 25.00")
		}
	}

	total := dollars*100 + cents
	return &total, nil
}

// formatCentsDollars renders an integer cents value as a plain dollar string
// (no leading "$"), e.g. 2500 -> "25.00", 50 -> "0.50". Used for the input
// prefill and the current-limit display.
func formatCentsDollars(cents int) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

// defaultAccountFooter is the account-menu footer shown when no sub-view owns
// the keyboard. Extracted so the spend-limit save/cancel paths can restore it
// without duplicating the slice.
func defaultAccountFooter() []footerCommand {
	return []footerCommand{
		{key: "j/k", value: "navigate"},
		{key: "enter", value: "select"},
		{key: "s", value: "shop"},
		{key: "c", value: "cart"},
		{key: "?", value: "help"},
		{key: "q", value: "quit"},
	}
}

// friendlySpendLimitError turns a raw API error into one line of TUI copy.
// SELF_LIMIT_ABOVE_CAP is the one case worth calling out specifically.
func friendlySpendLimitError(err error) string {
	if err == nil {
		return ""
	}
	if strings.Contains(err.Error(), "SELF_LIMIT_ABOVE_CAP") {
		return "limit can't exceed your account cap"
	}
	return "failed to save limit"
}
