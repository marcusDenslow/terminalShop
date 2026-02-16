package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.WindowWidth == 0 {
		m.WindowWidth = 120
		m.WindowHeight = 30
	}

	// Build header with cart tab
	header := m.BuildHeader()

	// Build breadcrumbs
	breadcrumbs := m.BuildBreadcrumbs()

	// Build main content based on view
	var content string
	if m.Loading {
		// Show loading message
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Padding(2, 4)
		content = loadingStyle.Render("Loading products from API...")
	} else if m.ErrorMsg != "" {
		// Show error message at top of shop view
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Background(lipgloss.Color("52")).
			Padding(0, 1).
			MarginBottom(1)
		errorBanner := errorStyle.Render("⚠ " + m.ErrorMsg)

		if m.ViewingAccount {
			content = errorBanner + "\n" + m.BuildAccountView()
		} else if m.ViewingCart && m.CheckoutStep == 1 {
			if m.ShippingView == 0 && m.ShippingForm == nil {
				content = errorBanner + "\n" + m.RenderAddressList()
			} else if m.ShippingForm != nil {
				content = errorBanner + "\n" + m.RenderShippingForm(m.ShippingForm)
			}
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
		content = m.BuildAccountView()
	} else if m.ViewingCart && m.CheckoutStep == 1 {
		if m.ShippingView == 0 && m.ShippingForm == nil {
			content = m.RenderAddressList()
		} else if m.ShippingForm != nil {
			content = m.RenderShippingForm(m.ShippingForm)
		}
	} else if m.ViewingCart && m.CheckoutStep == 2 && m.PaymentForm != nil {
		content = m.RenderPaymentForm(m.PaymentForm)
	} else if m.ViewingCart && m.CheckoutStep == 3 {
		content = m.RenderConfirmation()
	} else if m.ViewingCart {
		content = m.BuildCartView()
	} else {
		content = m.BuildShopView()
	}

	// Calculate content height based on window size
	headerHeight := lipgloss.Height(header)
	breadcrumbsHeight := 0
	if breadcrumbs != "" {
		breadcrumbsHeight = lipgloss.Height(breadcrumbs)
	}
	footerHeight := 1

	// Scale margins based on window height
	marginTop := 1
	marginBottom := 1

	// Reserve space for header, breadcrumbs (if present), footer, and margins
	// Add extra buffer to prevent footer from cutting off last item
	bufferSpace := 1
	reservedHeight := headerHeight + breadcrumbsHeight + footerHeight + marginTop + marginBottom + bufferSpace
	availableContentHeight := m.WindowHeight - reservedHeight

	// Ensure minimum content height
	if availableContentHeight < 3 {
		availableContentHeight = 3
		marginTop = 0
		marginBottom = 0
	}

	// Split content into lines and handle scrolling
	contentLines := strings.Split(content, "\n")
	totalLines := len(contentLines)

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

	// Apply scroll offset and truncate to fit available height
	if totalLines > availableContentHeight {
		start := m.ScrollOffset
		end := start + availableContentHeight
		if end > totalLines {
			end = totalLines
		}
		contentLines = contentLines[start:end]
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

	// Combine all app content
	var appContent string
	if breadcrumbs != "" {
		appContent = lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			breadcrumbs,
			contentWithPadding,
			footer,
		)
	} else {
		appContent = lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			contentWithPadding,
			footer,
		)
	}

	// Check if content fits in window
	appHeight := lipgloss.Height(appContent)

	if appHeight <= m.WindowHeight {
		// Content fits, center it vertically
		return lipgloss.Place(
			m.WindowWidth,
			m.WindowHeight,
			lipgloss.Left,
			lipgloss.Center,
			appContent,
		)
	} else {
		// Content doesn't fit, align to top (no centering)
		return appContent
	}
}
