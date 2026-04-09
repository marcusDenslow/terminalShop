package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RenderConfirmation renders the order confirmation view
func (m Model) RenderConfirmation() string {
	titleStyle := m.theme.TextAccent().Bold(true).Padding(0, 0, 1, 0)

	sectionStyle := m.theme.TextBody().Padding(0, 0, 1, 2)

	labelStyle := m.theme.TextDim()

	valueStyle := m.theme.TextAccent()

	successStyle := m.theme.TextSuccess().Bold(true).Padding(1, 0)

	title := titleStyle.Render("Order Confirmation")

	// Shipping summary
	shippingSection := ""
	if m.ConfirmShipping != nil {
		shippingSection = sectionStyle.Render(
			labelStyle.Render("Ship to: ") +
				valueStyle.Render(m.ConfirmShipping.Name) + "\n" +
				labelStyle.Render("         ") +
				valueStyle.Render(m.ConfirmShipping.Street) + "\n" +
				labelStyle.Render("         ") +
				valueStyle.Render(fmt.Sprintf("%s, %s %s, %s", m.ConfirmShipping.City, m.ConfirmShipping.State, m.ConfirmShipping.Zip, m.ConfirmShipping.Country)),
		)
	}

	// Items summary
	cartSection := ""
	for _, item := range m.ConfirmItems {
		line := fmt.Sprintf("  %s x%d  $%.2f", item.Coffee.Name, item.Quantity, float64(item.Coffee.Price*item.Quantity)/100)
		cartSection += sectionStyle.Render(line) + "\n"
	}
	cartSection += sectionStyle.Render(
		labelStyle.Render("Shipping: ") + valueStyle.Render("$0.00"),
	) + "\n"
	cartSection += sectionStyle.Render(
		labelStyle.Render("Total:    ") + valueStyle.Render(fmt.Sprintf("$%.2f", float64(m.ConfirmTotal)/100)),
	)

	success := successStyle.Render("Order placed successfully!")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		shippingSection,
		"",
		cartSection,
		"",
		success,
	)
}

func (m Model) updateConfirmViewport() Model {
	headerH := lipgloss.Height(m.BuildHeader())
	breadH := lipgloss.Height(m.BuildBreadcrumbs())
	footerH := lipgloss.Height(m.BuildFooter())
	availH := m.heightContainer - headerH - footerH - breadH
	if availH < 1 {
		availH = 1
	}
	if !m.confirmVPReady {
		m.confirmVP = viewport.New(m.widthContent, availH)
		m.confirmVP.KeyMap = viewport.DefaultKeyMap()
		m.confirmVPReady = true
	} else {
		m.confirmVP.Width = m.widthContent
		m.confirmVP.Height = availH
	}
	return m
}

func (m Model) ConfirmView() string {
	if !m.confirmVPReady {
		m = m.updateConfirmViewport()
	}
	m.confirmVP.SetContent(m.RenderConfirmation())
	return lipgloss.Place(
		m.widthContainer,
		lipgloss.Height(m.confirmVP.View()),
		lipgloss.Center, lipgloss.Center,
		m.confirmVP.View(),
	)
}

func (m Model) ConfirmUpdate(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	if m.confirmVPReady {
		m.confirmVP, cmd = m.confirmVP.Update(msg)
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, cmd
	}

	if keyMsg.String() != "esc" && keyMsg.String() != "s" {
		return m, cmd
	}

	m = m.SwitchPage(shopPage)
	m.CheckingOut = false
	m.ConfirmTotal = 0
	m.ConfirmItems = nil
	m.ConfirmShipping = nil
	m = m.resetPageState()
	return m, func() tea.Msg {
		if m.APIClient != nil {
			_ = m.APIClient.ClearCart()
		}
		return nil
	}
}
