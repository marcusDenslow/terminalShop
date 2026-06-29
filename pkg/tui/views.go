package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ResizeView renders a message when the terminal is too small.
func (m Model) ResizeView() string {
	msg := m.theme.TextDim().
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

func (m Model) View() tea.View {
	v := tea.NewView(m.renderString())
	v.AltScreen = true
	return v
}

func (m Model) renderString() string {
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
	footer := m.BuildFooter()
	footerHeight := lipgloss.Height(footer)
	marginTop := 1
	marginBottom := 1
	bufferSpace := 1
	bannerHeight := 1
	reservedHeight := bannerHeight + headerHeight + breadcrumbsHeight + footerHeight + marginBottom + marginTop + bufferSpace
	availableContentHeight := m.heightContainer - reservedHeight

	// Ensure minimum content height
	if availableContentHeight < 3 {
		availableContentHeight = 3
		marginTop = 0
		marginBottom = 0
	}

	var content string
	if m.Loading {
		loadingStyle := m.theme.TextLoading().Padding(2, 4)
		content = loadingStyle.Render("Loading products from API...")
	} else {
		content = m.buildPageContent()
	}

	bannerW := lipgloss.Width(header) - 2
	var topBanner string
	switch {
	case m.error != nil:
		topBanner = m.theme.PanelError().Width(bannerW).MaxWidth(bannerW).Render(m.error.message)
	case m.notice != nil:
		topBanner = m.theme.PanelNotice().Width(bannerW).MaxWidth(bannerW).Render(m.notice.message)
	default:
		topBanner = lipgloss.NewStyle().Width(m.widthContent).Render("")
	}

	isViewportPage := m.currentPage == shopPage || m.currentPage == cartPage ||
		m.currentPage == shippingPage || m.currentPage == paymentPage ||
		m.currentPage == confirmPage || m.currentPage == accountPage

	var child string
	if isViewportPage {
		if breadcrumbs != "" {
			child = lipgloss.JoinVertical(lipgloss.Left, topBanner, header, breadcrumbs, content, footer)
		} else {
			child = lipgloss.JoinVertical(lipgloss.Left, topBanner, header, content, footer)
		}
	} else if breadcrumbs != "" {
		contentWithPadding := lipgloss.NewStyle().MarginTop(marginTop).MarginBottom(marginBottom).Height(availableContentHeight).Render(content)
		child = lipgloss.JoinVertical(lipgloss.Left, topBanner, header, breadcrumbs, contentWithPadding, footer)
	} else {
		contentWithPadding := lipgloss.NewStyle().MarginTop(marginTop).MarginBottom(marginBottom).Height(availableContentHeight).Render(content)
		child = lipgloss.JoinVertical(lipgloss.Left, topBanner, header, contentWithPadding, footer)
	}

	// Constrain the assembled layout to the container dimensions
	constrained := lipgloss.NewStyle().
		Width(m.widthContainer).
		MaxWidth(m.widthContainer).
		MaxHeight(m.heightContainer).
		Render(child)

	if m.refund.open {
		constrained = m.RenderRefundOverlay()
	}

	// Center the entire container in the terminal viewport
	return lipgloss.Place(
		m.viewportWidth,
		m.viewportHeight,
		lipgloss.Center,
		lipgloss.Center,
		constrained,
	)
}

func (m Model) buildPageContent() string {
	switch m.currentPage {
	case accountPage:
		return m.BuildAccountView()
	case shippingPage:
		return m.ShippingPageView()
	case paymentPage:
		return m.PaymentPageView()
	case reviewPage:
		return m.ReviewView()
	case confirmPage:
		return m.ConfirmView()
	case cartPage:
		return m.CartView()
	case shopPage:
		return m.ShopView()
	default:
		return m.ShopView()
	}
}
