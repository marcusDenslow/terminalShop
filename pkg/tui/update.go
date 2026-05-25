package tui

import (
	"fmt"
	"terminalShop/pkg/api"
	"terminalShop/pkg/models"
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Resize debounce tick, must fire before page dispatch
	if tick, ok := msg.(resizeTickMsg); ok {
		if tick.seq == m.resizeSeq {
			m = m.updateLayout(m.pendingWidth, m.pendingHeight)
			switch m.currentPage {
			case shopPage:
				m = m.updateShopViewports()
			case cartPage:
				m = m.updateCartViewport()
			case shippingPage:
				m = m.updateShippingViewport()
			case paymentPage:
				m = m.updatePaymentViewport()
			case confirmPage:
				m = m.updateConfirmViewport()
			case accountPage:
				m = m.updateAccountViewport()
			case reviewPage:
				// no viewport on review page
			}
			if m.shipping.form != nil {
				m.shipping.form.form = m.buildShippingForm(m.shipping.form)
			}
			if m.payment.form != nil {
				m.payment.form.form = m.buildPaymentForm(m.payment.form)
			}
		}
		return m, nil
	}

	// Modal overlays intercept all input
	if m.ShowingMenu {
		return m.MenuUpdate(msg)
	}
	if m.ShowingHelp {
		return m.HelpUpdate(msg)
	}

	switch msg := msg.(type) {
	case VisibleError:
		m.error = &msg
		return m, nil
	case error:
		m.error = &VisibleError{message: msg.Error()}
		if m.currentPage == shopPage || m.currentPage == cartPage {
			return m, m.fetchCartCmd
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.pendingWidth = msg.Width
		m.pendingHeight = msg.Height
		m.resizeSeq++
		seq := m.resizeSeq
		return m, tea.Tick(resizeDebounce, func(t time.Time) tea.Msg {
			return resizeTickMsg{seq: seq}
		})
	case SplashAuthMsg:
		if msg.Err != nil {
			m.error = &VisibleError{message: fmt.Sprintf("authentication failed: %v", msg.Err)}
			// Still attempt to load view data (products may not require auth)
			// so the splash screen doesn't get permanently stuck.
			return m, m.splashViewInitCmd
		}
		m.AccessToken = msg.Token
		m.APIClient.Token = msg.Token
		if msg.User.ID != 0 {
			m.Username = msg.User.Name
		}
		return m, tea.Batch(m.splashViewInitCmd, m.scheduleTokenRefreshCmd())
	case ViewInitMsg:
		if msg.Err != nil {
			return m.ShopSwitch()
		}
		m.Loading = false
		if len(msg.Data.Products) > 0 {
			m.Coffees = msg.Data.Products
		}
		if msg.Data.User.ID != 0 {
			u := models.User{
				ID:    msg.Data.User.ID,
				Name:  msg.Data.User.Email,
				Email: msg.Data.User.Email,
			}
			m.User = &u
			m.Username = u.Name
		}
		cart := &api.CartData{Items: msg.Data.Cart}
		m = m.loadCartFromAPI(cart)
		m.SavedAddresses = msg.Data.Addresses
		m.SavedCards = msg.Data.Cards
		m.Orders = msg.Data.Orders
		m.OrdersLoaded = true
		m.splash.dataReady = true
		m.ordersPollStarted = true
		pollCmd := m.pollOrderCmd()
		if m.splash.delayDone {
			nextM, switchCmd := m.ShopSwitch()
			return nextM, tea.Batch(switchCmd, pollCmd)
		}
		return m, pollCmd
	case CartSyncedMsg:
		if msg.Err != nil {
			return m, m.fetchCartCmd
		}
		if msg.UpdateID == m.lastCartUpdateID {
			m = m.loadCartFromAPI(msg.Cart)
		}
		return m, nil
	case tokenRefreshTickMsg:
		return m, m.refreshTokenCmd()
	case TokenRefreshedMsg:
		if msg.Err != nil {
			return m, tea.Tick(tokenRetryDuration, func(t time.Time) tea.Msg {
				return tokenRefreshTickMsg{}
			})
		}
		m.AccessToken = msg.Token
		m.APIClient.Token = msg.Token
		return m, m.scheduleTokenRefreshCmd()
	case OrdersMsg:
		if msg.Err == nil {
			m.Orders = msg.Orders
			m.OrdersLoaded = true
			if !m.ordersPollStarted {
				m.ordersPollStarted = true
				return m, m.pollOrderCmd()
			}
		}
		return m, nil
	case OrdersPollTickMsg:
		return m, tea.Batch(m.fetchOrdersCmd(), m.pollOrderCmd())
	case AddressDeletedMsg:
		if msg.Err != nil {
			m.error = &VisibleError{message: fmt.Sprintf("failed to delete address: %v", msg.Err)}
		}
		return m, nil
	case CardDeletedMsg:
		if msg.Err != nil {
			m.error = &VisibleError{message: fmt.Sprintf("failed to delete card: %v", msg.Err)}
		}
		return m, nil
	case DelayCompleteMsg:
		m.splash.delayDone = true
		if m.currentPage == splashPage && m.splash.dataReady {
			return m.ShopSwitch()
		}
		return m, nil
	case splashCursorTickMsg:
		m.splash.cursor = !m.splash.cursor
		return m, tea.Tick(700*time.Millisecond, func(t time.Time) tea.Msg {
			return splashCursorTickMsg{}
		})
	case tea.KeyMsg:
		// If a form is active, skip global keys — let the page handler own input
		if m.shipping.form != nil || m.payment.form != nil {
			break
		}
		// Dismiss error banner with ESC
		if msg.String() == "esc" && m.error != nil {
			m.error = nil
			return m, nil
		}
		// Global navigation
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "?":
			m.ShowingHelp = true
			return m, nil
		case "m":
			m.ShowingMenu = true
			m.menuLastPage = m.currentPage
			return m, nil
		case "s":
			return m.ShopSwitch()
		case "c":
			return m.CartSwitch()
		case "a":
			return m.AccountSwitch()
		}
		// non-globals. fall through to page dispatch below
	}
	// Page-specifics
	switch m.currentPage {
	case shopPage:
		return m.ShopUpdate(msg)
	case cartPage:
		return m.CartUpdate(msg)
	case shippingPage:
		return m.ShippingUpdate(msg)
	case paymentPage:
		return m.PaymentUpdate(msg)
	case reviewPage:
		return m.ReviewUpdate(msg)
	case confirmPage:
		return m.ConfirmUpdate(msg)
	case accountPage:
		return m.AccountUpdate(msg)
	}
	return m, nil
}
