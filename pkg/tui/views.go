package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
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
		loadingStyle := m.theme.TextLoading().Padding(2, 4)
		content = loadingStyle.Render("Loading products from API...")
	} else if m.ErrorMsg != "" {
		errorStyle := m.theme.PanelError().Padding(0, 1).MarginBottom(1)
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
	accountHandlesScroll := m.currentPage == accountPage && (m.account.orderViewState >= 1 || m.account.faqFocused)
	if !accountHandlesScroll {
		// Ensure scroll offset is within bounds
		if m.account.scrollOffset < 0 {
			m.account.scrollOffset = 0
		}

		maxScroll := totalLines - availableContentHeight
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.account.scrollOffset > maxScroll {
			m.account.scrollOffset = maxScroll
		}

		if totalLines > availableContentHeight {
			start := m.account.scrollOffset
			end := start + availableContentHeight
			if end > totalLines {
				end = totalLines
			}
			contentLines = contentLines[start:end]
		}
	}
	content = strings.Join(contentLines, "\n")

	footer := m.BuildFooter()

	isViewportPage := m.currentPage == shopPage || m.currentPage == cartPage ||
		m.currentPage == shippingPage || m.currentPage == paymentPage ||
		m.currentPage == confirmPage

	var child string
	if isViewportPage {
		if breadcrumbs != "" {
			child = lipgloss.JoinVertical(lipgloss.Left, header, breadcrumbs, content, footer)
		} else {
			child = lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
		}
	} else if breadcrumbs != "" {
		ccontentWithPadding := lipgloss.NewStyle().MarginTop(marginTop).MarginBottom(marginBottom).Height(availableContentHeight).Render(content)
		child = lipgloss.JoinVertical(lipgloss.Left, header, breadcrumbs, ccontentWithPadding, footer)
	} else {
		ccontentWithPadding := lipgloss.NewStyle().MarginTop(marginTop).MarginBottom(marginBottom).Height(availableContentHeight).Render(content)
		child = lipgloss.JoinVertical(lipgloss.Left, header, ccontentWithPadding, footer)
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
