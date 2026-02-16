package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

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
}

// PaymentFormErrorMsg is sent when payment form validation fails
type PaymentFormErrorMsg struct {
	Message string
}

// buildPaymentForm creates the huh form bound to state's fields.
// No field-level validators so Tab/Enter move freely.
func (m Model) buildPaymentForm(state *PaymentFormState) *huh.Form {
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
		),
	).
		WithShowErrors(false).
		WithShowHelp(false)

	if m.WindowWidth < 80 {
		f = f.WithLayout(huh.LayoutStack).WithWidth(m.WindowWidth - 10)
	} else {
		f = f.WithLayout(huh.LayoutColumns(2)).WithWidth(m.WindowWidth - 20)
	}

	return f
}

// InitPaymentForm creates a new heap-allocated payment form state.
func (m Model) InitPaymentForm() *PaymentFormState {
	state := &PaymentFormState{}

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
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width < 80 {
			state.form = state.form.WithLayout(huh.LayoutStack).WithWidth(msg.Width - 10)
		} else {
			state.form = state.form.WithLayout(huh.LayoutColumns(2)).WithWidth(msg.Width - 20)
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
			}
		}
	}

	return cmd
}

// RenderPaymentForm renders the payment form view
func (m Model) RenderPaymentForm(state *PaymentFormState) string {
	if state.submitting {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(1)
		return loadingStyle.Render("Processing payment...")
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 0, 1, 0)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Padding(1, 0, 0, 0)

	cardLast4 := ""
	if len(state.CardNumber) >= 4 {
		cardLast4 = fmt.Sprintf(" (ending %s)", state.CardNumber[len(state.CardNumber)-4:])
	}

	title := titleStyle.Render("Payment Details" + cardLast4)
	form := state.form.View()
	help := helpStyle.Render("enter/tab next • shift+tab prev • esc back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		form,
		help,
	)
}
