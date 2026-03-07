package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ResizeView renders a message when the terminal is too small.
func (m Model) ResizeView() string {
	msg := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Align(lipgloss.Center).
		Render("Terminal too small.\nPlease resize.")

	return lipgloss.Place(
		m.viewportWidth,
		m.viewportHeight,
		lipgloss.Center,
		lipgloss.Center,
		msg,
	)
}

func (m Model) View() string {
	if m.viewportWidth == 0 {
		m.updateLayout(120, 30)
	}

	// If the terminal is too small, show a resize message
	if m.size == undersized {
		return m.ResizeView()
	}

	// If the menu modal is showing, render it full-screen (bypass container)
	if m.ShowingMenu {
		return m.BuildMenuView()
	}

	// Build header with cart tab
	header := m.BuildHeader()

	// Build breadcrumbs
	breadcrumbs := m.BuildBreadcrumbs()

	// Calculate available content height within the container
	headerHeight := lipgloss.Height(header)
	breadcrumbsHeight := 0
	if breadcrumbs != "" {
		breadcrumbsHeight = lipgloss.Height(breadcrumbs)
	}
	footerHeight := 1
	marginTop := 1
	marginBottom := 1
	bufferSpace := 1
	reservedHeight := headerHeight + breadcrumbsHeight + footerHeight + marginTop + marginBottom + bufferSpace
	availableContentHeight := m.heightContainer - reservedHeight

	// Ensure minimum content height
	if availableContentHeight < 3 {
		availableContentHeight = 3
		marginTop = 0
		marginBottom = 0
	}

	// Build main content based on view
	var content string
	if m.Loading {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Padding(2, 4)
		content = loadingStyle.Render("Loading products from API...")
	} else if m.ErrorMsg != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Background(lipgloss.Color("52")).
			Padding(0, 1).
			MarginBottom(1)
		errorBanner := errorStyle.Render("⚠ " + m.ErrorMsg)

		if m.ViewingAccount {
			content = errorBanner + "\n" + m.BuildAccountView(availableContentHeight)
		} else if m.ViewingCart && m.CheckoutStep == 1 {
			if m.ShippingView == 0 && m.ShippingForm == nil {
				content = errorBanner + "\n" + m.RenderAddressList()
			} else if m.ShippingForm != nil {
				content = errorBanner + "\n" + m.RenderShippingForm(m.ShippingForm)
			}
		} else if m.ViewingCart && m.CheckoutStep == 2 && m.CheckingOut {
			content = errorBanner + "\n  submitting order..."
		} else if m.ViewingCart && m.CheckoutStep == 2 && m.PaymentView == 0 && m.PaymentForm == nil {
			content = errorBanner + "\n" + m.RenderCardList()
		} else if m.ViewingCart && m.CheckoutStep == 2 && m.PaymentForm != nil {
			content = errorBanner + "\n" + m.RenderPaymentForm(m.PaymentForm)
		} else if m.ViewingCart && m.CheckoutStep == 3 {
			content = errorBanner + "\n" + m.RenderConfirmation()
		} else if m.ViewingCart {
			content = errorBanner + "\n" + m.BuildCartView()
		} else {
			content = errorBanner + "\n" + m.BuildShopView()
		}
	} else if m.ViewingAccount {
		content = m.BuildAccountView(availableContentHeight)
	} else if m.ViewingCart && m.CheckoutStep == 1 {
		if m.ShippingView == 0 && m.ShippingForm == nil {
			content = m.RenderAddressList()
		} else if m.ShippingForm != nil {
			content = m.RenderShippingForm(m.ShippingForm)
		}
	} else if m.ViewingCart && m.CheckoutStep == 2 && m.CheckingOut {
		content = "  submitting order..."
	} else if m.ViewingCart && m.CheckoutStep == 2 && m.PaymentView == 0 && m.PaymentForm == nil {
		content = m.RenderCardList()
	} else if m.ViewingCart && m.CheckoutStep == 2 && m.PaymentForm != nil {
		content = m.RenderPaymentForm(m.PaymentForm)
	} else if m.ViewingCart && m.CheckoutStep == 3 {
		content = m.RenderConfirmation()
	} else if m.ViewingCart {
		content = m.BuildCartView()
	} else {
		content = m.BuildShopView()
	}

	// Split content into lines and handle scrolling
	contentLines := strings.Split(content, "\n")
	totalLines := len(contentLines)

	// Skip global scroll when account view handles its own scrolling
	if !(m.ViewingAccount && m.OrderViewState >= 1) {
		// Ensure scroll offset is without bounds
		if m.ScrollOffset < 0 {
			m.ScrollOffset = 0
		}

		maxScroll := totalLines - availableContentHeight
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.ScrollOffset > maxScroll {
			m.ScrollOffset = maxScroll
		}

		if totalLines > availableContentHeight {
			start := m.ScrollOffset
			end := start + availableContentHeight
			if end > totalLines {
				end = totalLines
			}
			contentLines = contentLines[start:end]
		}
	}
	content = strings.Join(contentLines, "\n")

	// Add vertical spacing around content with fixed height
	contentWithPadding := lipgloss.NewStyle().
		MarginTop(marginTop).
		MarginBottom(marginBottom).
		Height(availableContentHeight).
		Render(content)

	// Build footer
	footer := m.BuildFooter()

	// Assemble all layers vertically within the container
	var child string
	if breadcrumbs != "" {
		child = lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			breadcrumbs,
			contentWithPadding,
			footer,
		)
	} else {
		child = lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			contentWithPadding,
			footer,
		)
	}

	// Constrain the assembled layout to the container dimensions
	constrained := lipgloss.NewStyle().
		MaxWidth(m.widthContainer).
		MaxHeight(m.heightContainer).
		Render(child)

	// Center the entire container in the terminal viewport
	return lipgloss.Place(
		m.viewportWidth,
		m.viewportHeight,
		lipgloss.Center,
		lipgloss.Center,
		constrained,
	)
}
