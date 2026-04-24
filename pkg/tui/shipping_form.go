package tui

import (
	"fmt"
	"strings"

	"terminalShop/pkg/models"

	"github.com/charmbracelet/bubbles/viewport"
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
	// {Name: "Denmark", Code: "DK", DialCode: "+45"},
	// {Name: "Finland", Code: "FI", DialCode: "+358"},
	// {Name: "Iceland", Code: "IS", DialCode: "+354"},
	{Name: "Norway", Code: "NO", DialCode: "+47"},
	// {Name: "Sweden", Code: "SE", DialCode: "+46"},
	{Name: "USA", Code: "US", DialCode: "+1"},
}

// countryDialCodes maps country ISO codes to their dialing prefixes.
var countryDialCodes = map[string]string{
	// "DK": "+45",
	// "FI": "+358",
	// "IS": "+354",
	"NO": "+47",
	// "SE": "+46",
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
				Height(3),
			huh.NewInput().
				Title("Phone").
				Key("phone").
				Value(&state.Phone),
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

// InitShippingForm creates a new heap-allocated shipping form state.
// The returned pointer must be stored directly in Model.ShippingForm
// so that huh's Value() bindings point to stable memory.
func (m Model) InitShippingForm() *ShippingFormState {
	state := &ShippingFormState{
		Country: "NO",
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
		return m.theme.TextAccent().Bold(true).Padding(1).Render("Processing shipping information...")
	}

	title := m.theme.TextAccent().Bold(true).Padding(0, 0, 1, 0).Render("Shipping Address")
	form := state.form.View()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		form,
	)
}

func (m Model) updateShippingViewport() Model {
	headerH := lipgloss.Height(m.BuildHeader())
	breadH := lipgloss.Height(m.BuildBreadcrumbs())
	footerH := lipgloss.Height(m.BuildFooter())
	availH := m.heightContainer - headerH - footerH - breadH
	if availH < 1 {
		availH = 1
	}
	if !m.shipping.viewportReady {
		m.shipping.viewport = viewport.New(m.widthContent, availH)
		m.shipping.viewport.KeyMap = viewport.KeyMap{}
		m.shipping.viewportReady = true
	} else {
		m.shipping.viewport.Width = m.widthContent
		m.shipping.viewport.Height = availH
	}
	return m
}

func (m Model) ShippingPageView() string {
	if !m.shipping.viewportReady {
		m = m.updateShippingViewport()
	}
	if m.shipping.view == 0 && m.shipping.form == nil {
		content := m.RenderAddressList()
		m.shipping.viewport.SetContent(content)
		itemHeight := 4
		targetY := m.shipping.addressCursor * itemHeight
		if targetY < m.shipping.viewport.YOffset {
			m.shipping.viewport.SetYOffset(targetY)
		}
		if targetY+itemHeight > m.shipping.viewport.YOffset+m.shipping.viewport.Height {
			m.shipping.viewport.SetYOffset(targetY - m.shipping.viewport.Height + itemHeight + 1)
		}
		if m.shipping.addressCursor == len(m.SavedAddresses) {
			m.shipping.viewport.GotoBottom()
		}
	} else if m.shipping.form != nil {
		m.shipping.viewport.SetContent(m.RenderShippingForm(m.shipping.form))
	}
	return lipgloss.Place(
		m.widthContainer,
		lipgloss.Height(m.shipping.viewport.View()),
		lipgloss.Center, lipgloss.Center,
		m.shipping.viewport.View(),
	)
}

func (m Model) RenderAddressList() string {
	titleStyle := m.theme.TextAccent().Bold(true).Padding(0, 0, 1, 0)
	activeStyle := m.theme.TextAccent().Bold(true)
	inactiveStyle := m.theme.TextDim()
	labelStyle := m.theme.TextLabel()

	title := titleStyle.Render("Selected Shipping Address")

	var lines []string
	for i, addr := range m.SavedAddresses {
		cursor := "  "
		style := inactiveStyle
		if i == m.shipping.addressCursor {
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
	if m.shipping.addressCursor == len(m.SavedAddresses) {
		addCursor = "> "
		addStyle = activeStyle
	}

	lines = append(lines, addCursor+addStyle.Render("+ Add new address"))

	parts := []string{title}
	parts = append(parts, strings.Join(lines, "\n"))

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m Model) ShippingUpdate(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ShippingFormCompleteMsg:
		addr := models.Address{
			Name: msg.Name, Street: msg.Street1, Street2: msg.Street2,
			City: msg.City, State: msg.State, Country: msg.Country,
			Zip: msg.Zip, Phone: msg.Phone,
		}
		return m, m.saveAddressCmd(addr)

	case AddressesMsg:
		if msg.Err != nil || len(msg.Addresses) == 0 {
			m.SavedAddresses = nil
			m.shipping.view = 1
			m.shipping.form = m.InitShippingForm()
			return m, m.shipping.form.form.Init()
		}
		m.SavedAddresses = msg.Addresses
		m.shipping.view = 0
		m.shipping.addressCursor = 0
		m.shipping.form = nil
		return m, nil

	case AddressSavedMsg:
		if msg.Err != nil {
			if m.shipping.form != nil {
				m.shipping.form.submitting = false
				m.shipping.form.form = m.buildShippingForm(m.shipping.form)
			}
			m.ErrorMsg = "Invalid address. Currently only US and Norwegian addresses are supported."
			return m, m.shipping.form.form.Init()
		}
		return m.PaymentSwitch()

	case ShippingFormErrorMsg:
		m.ErrorMsg = msg.Message
		return m, nil
	}

	// Address list navigation
	if m.shipping.view == 0 && m.shipping.form == nil {
		keyMsg, ok := msg.(tea.KeyMsg)
		if !ok {
			return m, nil
		}
		m.ErrorMsg = ""
		switch keyMsg.String() {
		case "esc":
			return m.CartSwitch()
		case "up", "k":
			if m.shipping.addressCursor > 0 {
				m.shipping.addressCursor--
			}
		case "down", "j":
			if m.shipping.addressCursor < len(m.SavedAddresses) {
				m.shipping.addressCursor++
			}
		case "enter":
			if m.shipping.addressCursor < len(m.SavedAddresses) {
				selected := m.SavedAddresses[m.shipping.addressCursor]
				m.ShippingInfo = &selected
				return m.PaymentSwitch()
			}
			m.shipping.view = 1
			m.shipping.form = m.InitShippingForm()
			return m, m.shipping.form.form.Init()
		case "d", "x":
			if m.shipping.addressCursor < len(m.SavedAddresses) {
				addr := m.SavedAddresses[m.shipping.addressCursor]
				m.SavedAddresses = append(m.SavedAddresses[:m.shipping.addressCursor], m.SavedAddresses[m.shipping.addressCursor+1:]...)
				if m.shipping.addressCursor >= len(m.SavedAddresses) && m.shipping.addressCursor > 0 {
					m.shipping.addressCursor--
				}
				if len(m.SavedAddresses) == 0 {
					m.shipping.view = 1
					m.shipping.form = m.InitShippingForm()
					return m, tea.Batch(m.shipping.form.form.Init(), m.deleteAddressCmd(addr.ID))
				}
				return m, m.deleteAddressCmd(addr.ID)
			}
		}
		return m, nil
	}

	// Form navigation
	if m.shipping.form != nil {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "esc" {
				m.ErrorMsg = ""
				if len(m.SavedAddresses) > 0 {
					m.shipping.view = 0
					m.shipping.form = nil
					return m, nil
				}
				m.shipping.form = nil
				m, cmd := m.CartSwitch()
				return m, cmd
			}
			// Clear error only when user starts typing, not on internal huh messages
			m.ErrorMsg = ""
		}
		return m, m.UpdateShippingForm(msg, m.shipping.form)
	}
	return m, nil
}
