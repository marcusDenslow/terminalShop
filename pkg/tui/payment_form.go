package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"terminalShop/pkg/models"
	"terminalShop/pkg/tui/qrfefe"
	"terminalShop/pkg/validate"
)

// PaymentFormState holds the payment form state.
// Always heap-allocated so huh's Value() bindings stay valid.
type PaymentFormState struct {
	form        *huh.Form
	CardName    string
	Email       string
	CardNumber  string
	ExpiryMonth string
	ExpiryYear  string
	CVC         string
	BillingZip  string
	SaveCard    bool
	submitting  bool
}

// PaymentFormCompleteMsg is sent when the payment form passes validation
type PaymentFormCompleteMsg struct {
	CardName    string
	Email       string
	CardNumber  string
	ExpiryMonth string
	ExpiryYear  string
	CVC         string
	BillingZip  string
	SaveCard    bool
}

// CardSavedForReviewMsg is sent after a new card is tokenized and saved
// The user is taken to the review page to confirm before checkout
type CardSavedForReviewMsg struct {
	Card models.Card
	Err  error
}

// PaymentFormErrorMsg is sent when payment form validation fails
type PaymentFormErrorMsg struct {
	Message string
}

// PollPaymentInitMsg is sent when the collect endpoint returns a URL
type PollPaymentInitMsg struct {
	URL string
}

// PollPaymentStatusMsg is sent each polling tick while waiting for the browser
// card entry to complete.
type PollPaymentStatusMsg struct {
	Baseline      time.Time
	BaselineCount int
	Deadline      time.Time
}

// PollPaymentCompleteMsg is sent when the card count increases, meaning the
// webhook saved the new card
type PollPaymentCompleteMsg struct {
	Cards     []models.Card
	Duplicate bool
}

type PollPaymentTimeoutMsg struct{}

// buildPaymentForm creates the huh form bound to state's fields.
// No field-level validators so Tab/Enter move freely.
func (m Model) buildPaymentForm(state *PaymentFormState) *huh.Form {
	// The save prompt doubles as the privacy "save anyway?" warning: when the
	// account privacy default is on, the affirmative choice opposes it, so the
	// title says so. The user can still pick Save (the per-order override).
	savePrompt := "Save this card for next time?"
	if m.User != nil && m.User.PrivacyMode {
		savePrompt = "Privacy mode is on — saving keeps this card. Save anyway?"
	}
	f := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Name on Card").
				Key("card_name").
				Value(&state.CardName),
			huh.NewInput().
				Title("Email Address").
				Key("email").
				Value(&state.Email),
			huh.NewInput().
				Title("Card Number").
				Key("card_number").
				Value(&state.CardNumber),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Expiry Month (MM)").
				Key("exp_month").
				Value(&state.ExpiryMonth),
			huh.NewInput().
				Title("Expiry Year (YY)").
				Key("exp_year").
				Value(&state.ExpiryYear),
			huh.NewInput().
				Title("CVC").
				Key("cvc").
				Value(&state.CVC),
			huh.NewInput().
				Title("Billing ZIP").
				Key("billing_zip").
				Value(&state.BillingZip),
			huh.NewConfirm().
				Key("save_card").
				Title(savePrompt).
				Affirmative("Save").
				Negative("Don't save").
				Value(&state.SaveCard),
		),
	).
		WithShowErrors(false).
		WithShowHelp(false).
		WithTheme(m.theme.Form())

	if m.size < medium {
		f = f.WithLayout(huh.LayoutStack).WithWidth(m.widthContent)
	} else {
		f = f.WithLayout(huh.LayoutColumns(2)).WithWidth(m.widthContent)
	}

	return f
}

// InitPaymentForm creates a new heap-allocated payment form state.
func (m Model) InitPaymentForm() *PaymentFormState {
	state := &PaymentFormState{}

	// Default the save choice from the account privacy setting: privacy users
	// default to NOT saving (a one-time card); everyone else keeps today's
	// behaviour of saving the entered card.
	state.SaveCard = m.User == nil || !m.User.PrivacyMode

	// Pre-fill name and email from user if available
	if m.User != nil {
		if m.User.Name != "" {
			state.CardName = m.User.Name
		}
		if m.User.Email != "" {
			state.Email = m.User.Email
		}
	}

	state.form = m.buildPaymentForm(state)
	return state
}

// validatePaymentForm checks all required fields at submission time.
// Returns an error message or empty string if valid.
func validatePaymentForm(state *PaymentFormState) string {
	if strings.TrimSpace(state.CardName) == "" {
		return "name on card cannot be empty"
	}

	email := strings.TrimSpace(state.Email)
	if email == "" {
		return "email address cannot be empty"
	}
	if err := validate.EmailValidator(email); err != nil {
		return err.Error()
	}

	cardNum := strings.TrimSpace(state.CardNumber)
	if cardNum == "" {
		return "card number cannot be empty"
	}
	if err := validate.CcnValidator(cardNum); err != nil {
		return err.Error()
	}

	month := strings.TrimSpace(state.ExpiryMonth)
	if month == "" {
		return "expiry month cannot be empty"
	}
	if err := validate.Compose(
		validate.IsDigits("expiry month"),
		validate.MustBeLen(2, "expiry month"),
	)(month); err != nil {
		return err.Error()
	}

	year := strings.TrimSpace(state.ExpiryYear)
	if year == "" {
		return "expiry year cannot be empty"
	}
	if err := validate.Compose(
		validate.IsDigits("expiry year"),
		validate.MustBeLen(2, "expiry year"),
	)(year); err != nil {
		return err.Error()
	}

	cvc := strings.TrimSpace(state.CVC)
	if cvc == "" {
		return "CVC cannot be empty"
	}
	if err := validate.Compose(
		validate.IsDigits("CVC"),
		validate.WithinLen(3, 4, "CVC"),
	)(cvc); err != nil {
		return err.Error()
	}

	if strings.TrimSpace(state.BillingZip) == "" {
		return "billing ZIP cannot be empty"
	}

	return ""
}

// UpdatePaymentForm handles updates to the payment form.
// Mutates state in place, returns only the tea.Cmd.
func (m Model) UpdatePaymentForm(msg tea.Msg, state *PaymentFormState) tea.Cmd {
	switch _, ok := msg.(tea.WindowSizeMsg); {
	case ok:
		if m.size < medium {
			state.form = state.form.WithLayout(huh.LayoutStack).WithWidth(m.widthContent)
		} else {
			state.form = state.form.WithLayout(huh.LayoutColumns(2)).WithWidth(m.widthContent)
		}
	}

	form, cmd := state.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		state.form = f
	}

	if state.form.State == huh.StateCompleted && !state.submitting {
		if errMsg := validatePaymentForm(state); errMsg != "" {
			state.form = m.buildPaymentForm(state)
			return tea.Batch(
				state.form.Init(),
				func() tea.Msg { return PaymentFormErrorMsg{Message: errMsg} },
			)
		}

		state.submitting = true
		return func() tea.Msg {
			return PaymentFormCompleteMsg{
				CardName:    state.CardName,
				Email:       state.Email,
				CardNumber:  state.CardNumber,
				ExpiryMonth: state.ExpiryMonth,
				ExpiryYear:  state.ExpiryYear,
				CVC:         state.CVC,
				BillingZip:  state.BillingZip,
				SaveCard:    state.SaveCard,
			}
		}
	}

	return cmd
}

// RenderPaymentForm renders the payment form view
func (m Model) RenderPaymentForm(state *PaymentFormState) string {
	if state.submitting {
		return m.theme.TextAccent().Bold(true).Padding(1).Render("Processing payment...")
	}

	titleStyle := m.theme.TextAccent().Bold(true).Padding(0, 0, 1, 0)

	cardLast4 := ""
	if len(state.CardNumber) >= 4 {
		cardLast4 = fmt.Sprintf(" (ending %s)", state.CardNumber[len(state.CardNumber)-4:])
	}

	title := titleStyle.Render("Payment Details" + cardLast4)
	form := state.form.View()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		form,
	)
}

func (m Model) updatePaymentViewport() Model {
	headerH := lipgloss.Height(m.BuildHeader())
	breadH := lipgloss.Height(m.BuildBreadcrumbs())
	footerH := lipgloss.Height(m.BuildFooter())
	availH := m.heightContainer - headerH - footerH - breadH - 1
	availH = max(availH, 1)
	if !m.payment.viewportReady {
		m.payment.viewport = viewport.New(viewport.WithWidth(m.widthContent), viewport.WithHeight(availH))
		m.payment.viewport.KeyMap = viewport.KeyMap{}
		m.payment.viewportReady = true
	} else {
		m.payment.viewport.SetWidth(m.widthContent)
		m.payment.viewport.SetHeight(availH)
	}
	return m
}

func (m Model) PaymentPageView() string {
	if !m.payment.viewportReady {
		m = m.updatePaymentViewport()
	}
	var content string
	if m.CheckingOut {
		content = "  submitting order..."
	} else if m.payment.view == 2 {
		// Bypass viewport — it strips the raw ANSI codes qrfefe emits.
		return lipgloss.Place(
			m.widthContainer,
			m.payment.viewport.Height(),
			lipgloss.Center, lipgloss.Center,
			m.renderPaymentHTTPSView(),
		)
	} else if m.payment.view == 0 && m.payment.form == nil {
		content = m.RenderCardList()
		m.payment.viewport.SetContent(content)
		itemHeight := 3
		targetY := m.payment.cardCursor * itemHeight
		if targetY < m.payment.viewport.YOffset() {
			m.payment.viewport.SetYOffset(targetY)
		}
		if targetY+itemHeight > m.payment.viewport.YOffset()+m.payment.viewport.Height() {
			m.payment.viewport.SetYOffset(targetY - m.payment.viewport.Height() + itemHeight + 1)
		}
		if m.payment.cardCursor == len(m.SavedCards) {
			m.payment.viewport.GotoBottom()
		}
		return lipgloss.Place(
			m.widthContainer,
			lipgloss.Height(m.payment.viewport.View()),
			lipgloss.Center, lipgloss.Center,
			m.payment.viewport.View(),
		)
	} else if m.payment.form != nil {
		content = m.RenderPaymentForm(m.payment.form)
	}
	m.payment.viewport.SetContent(content)
	return lipgloss.Place(
		m.widthContainer,
		lipgloss.Height(m.payment.viewport.View()),
		lipgloss.Center, lipgloss.Center,
		m.payment.viewport.View(),
	)
}

func (m Model) RenderCardList() string {
	titleStyle := m.theme.TextAccent().Bold(true).Padding(0, 0, 1, 0)

	title := titleStyle.Render("Select Payment Method")

	var boxes []string
	for i, card := range m.SavedCards {
		focused := i == m.payment.cardCursor

		brand := strings.ToUpper(card.Brand[:1]) + card.Brand[1:]
		last4 := fmt.Sprintf("**** **** **** %s", card.Last4)
		exp := fmt.Sprintf("Expires %02d/%02d", card.ExpMonth, card.ExpYear%100)
		content := brand + " " + last4 + "\n" + exp

		box := m.CreateBox(m.formatListItem(content, focused), focused)
		boxes = append(boxes, box)
	}

	// SSH inline card-add is dormant until i have a norwegian org
	// number wired to stripe, something i dont have since its $300+. Keep the row visible so the option is
	// discoverable, but render it dimmed and exclude it from cursor space
	// (handled in PaymentUpdate below) so it can not be picked
	dim := m.theme.TextDim().Render
	sshLabel := dim("+ add card via ssh") + "  " + dim("(WIP)")
	sshBox := m.CreateBox(m.formatListItem(sshLabel, false), false)
	boxes = append(boxes, sshBox)

	httpsFocused := m.payment.cardCursor == len(m.SavedCards)
	httpsBox := m.CreateBox(m.formatListItem("+ add payment via browser", httpsFocused), httpsFocused)
	boxes = append(boxes, httpsBox)

	parts := []string{title}
	parts = append(parts, lipgloss.JoinVertical(lipgloss.Left, boxes...))

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m Model) renderPaymentHTTPSView() string {
	baseStyle := m.theme.TextLabel()
	accentStyle := m.theme.TextHighlight()

	if m.payment.collectTimeOut {
		minutes := int(cardPollTimeout / time.Minute)
		return m.theme.TextError().Bold(true).Render("payment link timed out") + "\n\n" +
			baseStyle.Render(fmt.Sprintf("no card saved after %d minutes.", minutes)) + "\n" +
			baseStyle.Render("press ") + accentStyle.Render("esc") + baseStyle.Render(" to go back and try again")
	}

	if m.payment.collectURL == nil {
		return baseStyle.Render("  generating payment link...")
	}

	qr, qrSize, err := qrfefe.Generate(*m.payment.collectURL)
	if err != nil || qrSize > m.widthContent {
		// QR too wide for the terminal — just show the URL.
		return baseStyle.Render("open in browser to add payment information:") + "\n\n" +
			accentStyle.Render(*m.payment.collectURL) + "\n"
	}

	// Concatenate manually — lipgloss layout functions (JoinVertical with Center)
	// measure string widths and add padding that corrupts qrfefe's raw ANSI codes.
	return qr +
		baseStyle.Render("scan or visit to add payment information") + "\n" +
		accentStyle.Render(*m.payment.collectURL) + "\n"
}

// PaymentUpdate handles all messages while on the payment page.
// Moved from model.go Update() lines 429-578.
func (m Model) PaymentUpdate(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CardsMsg:
		if msg.Err != nil {
			m.SavedCards = nil
		} else {
			m.SavedCards = msg.Cards
		}
		m.payment.view = 0
		m.payment.cardCursor = 0
		m.payment.form = nil
		return m, nil

	case PaymentFormCompleteMsg:
		if m.payment.form != nil && !m.payment.form.submitting {
			m.payment.form.submitting = true
			if msg.SaveCard {
				return m, m.saveCardOnlyCmd(msg)
			}
			// Privacy checkout: charge a one-time card without saving it. Jump to
			// the review page in checking-out state so ReviewUpdate handles the
			// resulting CheckoutResultMsg (same terminal handling as the saved-card
			// path), then forget the card.
			m, _ = m.ReviewSwitch()
			m.CheckingOut = true
			return m, m.checkoutEphemeralCmd(msg)
		}
		return m, nil

	case PaymentFormErrorMsg:
		m.error = &VisibleError{message: msg.Message}
		return m, nil

	case CardSavedForReviewMsg:
		if m.payment.form != nil {
			m.payment.form.submitting = false
		}
		if msg.Err != nil {
			m.error = &VisibleError{message: fmt.Sprintf("failed to save card: %v", msg.Err)}
			m.payment.view = 1
			m.payment.form = m.InitPaymentForm()
			m.footer = []footerCommand{
				{key: "tab", value: "next"},
				{key: "enter", value: "submit"},
				{key: "esc", value: "back"},
			}
			return m, m.payment.form.form.Init()
		}
		m.SavedCards = append(m.SavedCards, msg.Card)
		selected := msg.Card
		m.SelectedCard = &selected
		m.payment.form = nil
		m.review.cardJustAdded = true
		return m.ReviewSwitch()

	case PollPaymentInitMsg:
		url := msg.URL
		m.payment.collectURL = &url
		return m, m.pollCardsCmd(m.payment.collectBaseline, m.payment.collectCardCount, m.payment.collectDeadline)

	case PollPaymentStatusMsg:
		if m.payment.view != 2 {
			return m, nil // user navigated away, stop polling
		}
		return m, m.pollCardsCmd(msg.Baseline, msg.BaselineCount, msg.Deadline)
	case PollPaymentTimeoutMsg:
		if m.payment.view != 2 {
			return m, nil
		}
		m.payment.collectTimeOut = true
		return m, nil

	case PollPaymentCompleteMsg:
		if m.payment.view != 2 {
			return m, nil
		}
		m.SavedCards = msg.Cards
		m.payment.view = 0
		m.payment.collectURL = nil
		var selected models.Card
		if msg.Duplicate {
			for _, card := range m.SavedCards {
				if selected.ID == 0 || card.UpdatedAt.After(selected.UpdatedAt) {
					selected = card
				}
			}
		} else {
			selected = m.SavedCards[len(m.SavedCards)-1]
		}
		m.SelectedCard = &selected
		m.review.cardJustAdded = true
		m.review.cardWasDuplicate = msg.Duplicate
		return m.ReviewSwitch()
	}

	// Card list navigation
	if m.payment.view == 0 && m.payment.form == nil {
		keyMsg, ok := msg.(tea.KeyMsg)
		if !ok || m.CheckingOut {
			return m, nil
		}
		m.error = nil
		switch keyMsg.String() {
		case "esc":
			return m.ShippingSwitch()
		case "up", "k":
			if m.payment.cardCursor > 0 {
				m.payment.cardCursor--
			}
		case "down", "j":
			if m.payment.cardCursor < len(m.SavedCards) {
				m.payment.cardCursor++
			}

		case "enter":
			if m.payment.cardCursor < len(m.SavedCards) {
				selected := m.SavedCards[m.payment.cardCursor]
				m.SelectedCard = &selected
				return m.ReviewSwitch()
			}
			// Browser flow, the only reachable add-card path while the SSH
			// inline form is dormant (see RenderCardList)
			m.payment.view = 2
			m.payment.collectURL = nil
			m.payment.collectTimeOut = false
			m.payment.collectCardCount = len(m.SavedCards)
			var maxT time.Time
			for _, card := range m.SavedCards {
				if card.UpdatedAt.After(maxT) {
					maxT = card.UpdatedAt
				}
			}
			m.payment.collectBaseline = maxT
			m.payment.collectDeadline = time.Now().Add(cardPollTimeout)
			m.footer = []footerCommand{
				{key: "esc", value: "back"},
			}
			return m, m.collectCardCmd()

		case "d", "x":
			if m.payment.cardCursor < len(m.SavedCards) {
				card := m.SavedCards[m.payment.cardCursor]
				m.SavedCards = append(m.SavedCards[:m.payment.cardCursor], m.SavedCards[m.payment.cardCursor+1:]...)
				if m.payment.cardCursor >= len(m.SavedCards) && m.payment.cardCursor > 0 {
					m.payment.cardCursor--
				}
				return m, m.deleteCardCmd(card.ID)
			}
		}
		return m, nil
	}

	// HTTPS/browser view navigation
	if m.payment.view == 2 {
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
			m.payment.view = 0
			m.payment.collectURL = nil
			m.payment.collectTimeOut = false
			m.footer = []footerCommand{
				{key: "j/k", value: "cards"},
				{key: "enter", value: "select"},
				{key: "d/x", value: "delete"},
				{key: "esc", value: "back"},
			}
			return m, nil
		}
		return m, nil
	}

	// Payment form navigation
	if m.payment.form != nil {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "esc" {
				m.error = nil
				if len(m.SavedCards) > 0 {
					m.payment.view = 0
					m.payment.form = nil
					m.footer = []footerCommand{
						{key: "j/k", value: "cards"},
						{key: "enter", value: "select"},
						{key: "d/x", value: "delete"},
						{key: "esc", value: "back"},
					}
					return m, nil
				}
				m.payment.form = nil
				m = m.SwitchPage(shippingPage)
				return m, m.fetchAddressesCmd()
			}
			// Clear error only when user starts typing, not on internal huh messages
			m.error = nil
		}
		return m, m.UpdatePaymentForm(msg, m.payment.form)
	}
	return m, nil
}
