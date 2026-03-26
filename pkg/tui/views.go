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

	if m.currentPage == splashPage {
		return m.SplashView()
	}

	// If the menu modal is showing, render it full-screen (bypass container)
	if m.ShowingMenu {
		return m.BuildMenuView()
	}

	// If the help modal is showing, render it full-screen (bypass container)
	if m.ShowingHelp {
		return m.BuildHelpView()
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

	var content string
	if m.Loading {
		loadingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Padding(2, 4)
		content = loadingStyle.Render("Loading products from API...")
	} else if m.ErrorMsg != "" {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Background(lipgloss.Color("52")).Padding(0, 1).MarginBottom(1)
		errorBanner := errorStyle.Render(m.ErrorMsg)
		content = errorBanner + "\n" + m.buildPageContent(availableContentHeight)
	} else {
		content = m.buildPageContent(availableContentHeight)
	}

	// Split content into lines and handle scrolling
	contentLines := strings.Split(content, "\n")
	totalLines := len(contentLines)

	// Skip global scroll when account view handles its own scrolling
	// (order list/detail and FAQ focused use internal scrolling in BuildAccountView)
	accountHandlesScroll := m.currentPage == accountPage && (m.OrderViewState >= 1 || m.FaqFocused)
	if !accountHandlesScroll {
		// Ensure scroll offset is within bounds
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

func (m Model) buildPageContent(height int) string {
	switch m.currentPage {
	case accountPage:
		return m.BuildAccountView(height)
	case shippingPage:
		if m.ShippingView == 0 && m.ShippingForm == nil {
			return m.RenderAddressList()
		}
		return m.RenderShippingForm(m.ShippingForm)
	case paymentPage:
		if m.CheckingOut {
			return "  submitting order..."
		}
		if m.PaymentView == 0 && m.PaymentForm == nil {
			return m.RenderCardList()
		}
		return m.RenderPaymentForm(m.PaymentForm)
	case confirmPage:
		return m.RenderConfirmation()
	case cartPage:
		return m.BuildCartView()
	default:
		return m.BuildShopView()
	}
}
