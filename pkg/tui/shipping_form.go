package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// countries defines the available countries with their dial codes.
var countries = []struct {
	Name     string
	Code     string
	DialCode string
}{
	{Name: "Denmark", Code: "DK", DialCode: "+45"},
	{Name: "Finland", Code: "FI", DialCode: "+358"},
	{Name: "Iceland", Code: "IS", DialCode: "+354"},
	{Name: "Norway", Code: "NO", DialCode: "+47"},
	{Name: "Sweden", Code: "SE", DialCode: "+46"},
	{Name: "USA", Code: "US", DialCode: "+1"},
}

// countryDialCodes maps country ISO codes to their dialing prefixes.
var countryDialCodes = map[string]string{
	"DK": "+45",
	"FI": "+358",
	"IS": "+354",
	"NO": "+47",
	"SE": "+46",
	"US": "+1",
}

// ShippingFormState holds the shipping form state.
// Always allocate on the heap (use &ShippingFormState{}) so that huh's
// Value() pointers to the string fields remain valid.
type ShippingFormState struct {
	form       *huh.Form
	Name       string
	Street1    string
	Street2    string
	City       string
	State      string
	Country    string
	Zip        string
	Phone      string
	submitting bool
}

// ShippingFormCompleteMsg is sent when the shipping form is completed
type ShippingFormCompleteMsg struct {
	Name    string
	Street1 string
	Street2 string
	City    string
	State   string
	Country string
	Zip     string
	Phone   string
}

// ShippingFormErrorMsg is sent when form validation fails at submission time
type ShippingFormErrorMsg struct {
	Message string
}

// buildShippingForm creates the huh form bound to state's fields.
// No field-level validators so Tab/Enter move freely between fields.
// Validation happens at submission time instead.
func (m Model) buildShippingForm(state *ShippingFormState) *huh.Form {
	countryOptions := make([]huh.Option[string], 0, len(countries))
	for _, c := range countries {
		label := fmt.Sprintf("%s (%s)", c.Name, c.DialCode)
		countryOptions = append(countryOptions, huh.NewOption(label, c.Code))
	}

	f := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Full Name").
				Key("name").
				Value(&state.Name),
			huh.NewInput().
				Title("Street Address").
				Key("street1").
				Value(&state.Street1),
			huh.NewInput().
				Title("Apt, Suite, etc. (optional)").
				Key("street2").
				Value(&state.Street2),
			huh.NewInput().
				Title("City").
				Key("city").
				Value(&state.City),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Region (optional)").
				Key("state").
				Value(&state.State),
			huh.NewInput().
				Title("Postal Code").
				Key("zip").
				Value(&state.Zip),
			huh.NewSelect[string]().
				Title("Country").
				Key("country").
				Options(countryOptions...).
				Value(&state.Country).
				Height(5).
				Filtering(true),
			huh.NewInput().
				Title("Phone").
				Key("phone").
				Value(&state.Phone),
		),
	).
		WithShowErrors(false).
		WithShowHelp(false)

	if m.size < medium {
		f = f.WithLayout(huh.LayoutStack).WithWidth(m.widthContent)
	} else {
		f = f.WithLayout(huh.LayoutColumns(2)).WithWidth(m.widthContent)
	}

	return f
}

// InitShippingForm creates a new heap-allocated shipping form state.
// The returned pointer must be stored directly in Model.ShippingForm
// so that huh's Value() bindings point to stable memory.
func (m Model) InitShippingForm() *ShippingFormState {
	state := &ShippingFormState{
		Country: "SE",
	}

	if m.User != nil && m.User.Name != "" {
		state.Name = m.User.Name
	}

	state.form = m.buildShippingForm(state)
	return state
}

// validateShippingForm checks required fields after the user submits.
// Returns an error message describing the first missing field, or empty string if valid.
func validateShippingForm(state *ShippingFormState) string {
	if strings.TrimSpace(state.Name) == "" {
		return "name cannot be empty"
	}
	if strings.TrimSpace(state.Street1) == "" {
		return "street address cannot be empty"
	}
	if strings.TrimSpace(state.City) == "" {
		return "city cannot be empty"
	}
	if strings.TrimSpace(state.Zip) == "" {
		return "postal code cannot be empty"
	}
	if strings.TrimSpace(state.Phone) == "" {
		return "phone number cannot be empty"
	}
	return ""
}

// formatPhoneWithDialCode prepends the country dial code to the phone number
// if the user hasn't already included it.
func formatPhoneWithDialCode(phone, countryCode string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return phone
	}

	if strings.HasPrefix(phone, "+") {
		return phone
	}

	// Strip leading "0" (common in Scandinavian local numbers)
	phone = strings.TrimLeft(phone, "0")

	dialCode, ok := countryDialCodes[countryCode]
	if !ok {
		return phone
	}

	return dialCode + phone
}

// UpdateShippingForm handles updates to the shipping form.
// It mutates state in place (no struct copying) so huh's Value()
// pointers remain valid.
func (m Model) UpdateShippingForm(msg tea.Msg, state *ShippingFormState) tea.Cmd {
	if _, ok := msg.(tea.WindowSizeMsg); ok {
		if m.size < medium {
			state.form = state.form.WithLayout(huh.LayoutStack).WithWidth(m.widthContent)
		} else {
			state.form = state.form.WithLayout(huh.LayoutColumns(2)).WithWidth(m.widthContent)
		}
	}

	// Pass message to huh form
	form, cmd := state.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		state.form = f
	}

	// When the form is done, validate and submit (always save address)
	if state.form.State == huh.StateCompleted && !state.submitting {
		if errMsg := validateShippingForm(state); errMsg != "" {
			state.form = m.buildShippingForm(state)
			return tea.Batch(
				state.form.Init(),
				func() tea.Msg { return ShippingFormErrorMsg{Message: errMsg} },
			)
		}
		state.submitting = true
		phone := formatPhoneWithDialCode(state.Phone, state.Country)
		return func() tea.Msg {
			return ShippingFormCompleteMsg{
				Name:    state.Name,
				Street1: state.Street1,
				Street2: state.Street2,
				City:    state.City,
				State:   state.State,
				Country: state.Country,
				Zip:     state.Zip,
				Phone:   phone,
			}
		}
	}

	return cmd
}

// RenderShippingForm renders the shipping form view
func (m Model) RenderShippingForm(state *ShippingFormState) string {
	if state.submitting {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(1)
		return loadingStyle.Render("Processing shipping information...")
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 0, 1, 0)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Padding(1, 0, 0, 0)

	title := titleStyle.Render("Shipping Address")
	form := state.form.View()

	help := helpStyle.Render("enter/tab next • shift+tab prev • esc back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		form,
		help,
	)
}

func (m Model) RenderAddressList() string {
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Padding(0, 0, 1, 0)
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Padding(1, 0, 0, 0)

	title := titleStyle.Render("Selected Shipping Address")

	var lines []string
	for i, addr := range m.SavedAddresses {
		cursor := "  "
		style := inactiveStyle
		if i == m.AddressCursor {
			cursor = "> "
			style = activeStyle
		}

		name := style.Render(addr.Name)
		street := labelStyle.Render(addr.Street)
		if addr.Street2 != "" {
			street += ", " + labelStyle.Render(addr.Street2)
		}
		city := labelStyle.Render(fmt.Sprintf("%s, %s %s", addr.City, addr.Country, addr.Zip))

		lines = append(lines, cursor+name)
		lines = append(lines, "  "+street)
		lines = append(lines, "  "+city)
		lines = append(lines, "")
	}

	addCursor := "  "
	addStyle := inactiveStyle
	if m.AddressCursor == len(m.SavedAddresses) {
		addCursor = "> "
		addStyle = activeStyle
	}

	lines = append(lines, addCursor+addStyle.Render("+ Add new address"))

	help := helpStyle.Render("j/k, enter select, d delete, esc back")

	parts := []string{title}
	parts = append(parts, strings.Join(lines, "\n"))
	parts = append(parts, help)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
