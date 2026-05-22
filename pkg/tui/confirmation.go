package tui

import (
	"fmt"
	"strings"
	"terminalShop/pkg/models"

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
	if m.confirm.shipping != nil {
		shippingSection = sectionStyle.Render(
			labelStyle.Render("Ship to: ") +
				valueStyle.Render(m.confirm.shipping.Name) + "\n" +
				labelStyle.Render("         ") +
				valueStyle.Render(m.confirm.shipping.Street) + "\n" +
				labelStyle.Render("         ") +
				valueStyle.Render(fmt.Sprintf("%s, %s %s, %s", m.confirm.shipping.City, m.confirm.shipping.State, m.confirm.shipping.Zip, m.confirm.shipping.Country)),
		)
	}

	// Items summary
	cartSection := ""
	for _, item := range m.confirm.items {
		line := fmt.Sprintf("  %s x%d  $%.2f", item.Coffee.Name, item.Quantity, float64(item.Coffee.Price*item.Quantity)/100)
		cartSection += sectionStyle.Render(line) + "\n"
	}
	cartSection += sectionStyle.Render(
		labelStyle.Render("Shipping: ")+valueStyle.Render("$0.00"),
	) + "\n"
	cartSection += sectionStyle.Render(
		labelStyle.Render("Total:    ") + valueStyle.Render(fmt.Sprintf("$%.2f", float64(m.confirm.total)/100)),
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
	if !m.confirm.viewportReady {
		m.confirm.viewport = viewport.New(m.widthContent, availH)
		m.confirm.viewport.KeyMap = viewport.DefaultKeyMap()
		m.confirm.viewportReady = true
	} else {
		m.confirm.viewport.Width = m.widthContent
		m.confirm.viewport.Height = availH
	}
	return m
}

func (m Model) ConfirmView() string {
	if !m.confirm.viewportReady {
		m = m.updateConfirmViewport()
	}
	m.confirm.viewport.SetContent(m.RenderConfirmation())
	return lipgloss.Place(
		m.widthContainer,
		lipgloss.Height(m.confirm.viewport.View()),
		lipgloss.Center, lipgloss.Center,
		m.confirm.viewport.View(),
	)
}

func (m Model) ConfirmUpdate(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	if m.confirm.viewportReady {
		m.confirm.viewport, cmd = m.confirm.viewport.Update(msg)
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, cmd
	}

	if keyMsg.String() != "esc" && keyMsg.String() != "s" {
		return m, cmd
	}

	m.CheckingOut = false
	m.confirm.total = 0
	m.confirm.items = nil
	m.confirm.shipping = nil
	m, _ = m.ShopSwitch()
	return m, func() tea.Msg {
		if m.APIClient != nil {
			_ = m.APIClient.ClearCart()
		}
		return nil
	}
}

func formatUSD(cents int) string {
	return fmt.Sprintf("$%d.%02d", cents/100, cents%100)
}

func (m Model) generateReviewContent() string {
	view := strings.Builder{}

	if m.review.cardJustAdded {
		if m.review.cardWasDuplicate {
			view.WriteString(m.theme.TextDim().Render("identical card already saved — using existing card") + "\n\n")
		} else {
			view.WriteString(m.theme.TextSuccess().Bold(true).Render("card added successfully") + "\n\n")
		}
	}

	if m.ShippingInfo != nil {
		view.WriteString(m.ShippingInfo.Name + "\n")
		view.WriteString(m.ShippingInfo.Street + "\n")
		if m.ShippingInfo.Street2 != "" {
			view.WriteString(m.ShippingInfo.Street2 + "\n")
		}
		view.WriteString(fmt.Sprintf("%s,  %s %s,  %s\n", m.ShippingInfo.City, m.ShippingInfo.State, m.ShippingInfo.Zip, m.ShippingInfo.Country))
	}
	view.WriteString("\n")

	if m.SelectedCard != nil {
		view.WriteString(fmt.Sprintf("cc: **** **** **** %s\n", m.SelectedCard.Last4))
	}
	view.WriteString("\n")

	subtotal := m.CalculateSubtotal()

	view.WriteString(fmt.Sprintf("subtotal: %s\n", formatUSD(subtotal)))
	view.WriteString(m.theme.TextAccent().Render(fmt.Sprintf("total:    %s", formatUSD(subtotal))) + "\n")
	view.WriteString("\n")
	view.WriteString(m.theme.TextBrand().Render("press enter to confirm") + "\n")

	return m.theme.Base().Padding(0, 1).Render(view.String())
}

func (m Model) ReviewView() string {
	if m.CheckingOut {
		return m.theme.TextAccent().Bold(true).Padding(1).Render("  submitting order...")
	}
	if m.review.success {
		content := m.theme.TextSuccess().Bold(true).Padding(1).Render("order placed successfully!")
		return lipgloss.Place(
			m.widthContainer,
			lipgloss.Height(content),
			lipgloss.Center, lipgloss.Center,
			content,
		)
	}
	content := m.generateReviewContent()
	return lipgloss.Place(
		m.widthContainer,
		lipgloss.Height(content),
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func (m Model) ReviewUpdate(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CheckoutResultMsg:
		m.CheckingOut = false
		if msg.Err != nil {
			m = m.SwitchPage(paymentPage)
			m.error = &VisibleError{message: fmt.Sprintf("checkout failed: %v", msg.Err)}
			m.review.cardJustAdded = false
			m.payment.view = 0
			m.payment.form = nil
			return m, nil
		}
		m.Cart = make(map[uint]*models.CartItem)
		m.cart.cursor = 0
		m.ShippingInfo = nil
		m.OrdersLoaded = false
		m.review.success = true
		return m, nil

	case tea.KeyMsg:
		if m.CheckingOut {
			return m, nil
		}
		if m.review.success {
			m.review.success = false
			m.review.cardJustAdded = false
			m.shipping.form = nil
			m.payment.form = nil
			m, _ = m.ShopSwitch()
			return m, func() tea.Msg {
				if m.APIClient != nil {
					_ = m.APIClient.ClearCart()
				}
				return nil
			}
		}
		switch msg.String() {
		case "esc":
			m = m.SwitchPage(paymentPage)
			return m, nil

		case "enter":
			m.CheckingOut = true
			return m, m.checkoutWithSavedCard()
		}
	}
	return m, nil
}
