package tui

import (
	"fmt"
	"terminalShop/pkg/api"
	"terminalShop/pkg/models"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Resize debounce tick, must fire before page dispatch
	if tick, ok := msg.(resizeTickMsg); ok {
		if tick.seq == m.resizeSeq {
			m.updateLayout(m.pendingWidth, m.pendingHeight)
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
			}
			if m.ShippingForm != nil {
				m.ShippingForm.form = m.buildShippingForm(m.ShippingForm)
			}
			if m.PaymentForm != nil {
				m.PaymentForm.form = m.buildPaymentForm(m.PaymentForm)
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
			m.ErrorMsg = fmt.Sprintf("authentication failed: %v", msg.Err)
			return m, nil
		}
		m.AccessToken = msg.Token
		m.APIClient.Token = msg.Token
		if msg.User.ID != 0 {
			m.Username = msg.User.Name
		}
		return m, tea.Batch(m.splashViewInitCmd, m.scheduleTokenRefreshCmd())
	case ViewInitMsg:
		if msg.Err != nil {
			m.ErrorMsg = fmt.Sprintf("failed to load: %v", msg.Err)
			m.Loading = false
			return m, nil
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
		m.loadCartFromAPI(cart)
		m.SavedAddresses = msg.Data.Addresses
		m.SavedCards = msg.Data.Cards
		m.Orders = msg.Data.Orders
		m.OrdersLoaded = true
		m.splashDataReady = true
		if m.splashDelayDone {
			m = m.SwitchPage(shopPage)
			m = m.updateShopViewports()
			return m, nil
		}
		return m, nil
	case CartSyncedMsg:
		if msg.Err != nil {
			return m, m.fetchCartCmd
		}
		if msg.UpdateID == m.lastCartUpdateID {
			m.loadCartFromAPI(msg.Cart)
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
		}
		return m, nil
	case AddressDeletedMsg:
		if msg.Err != nil {
			m.ErrorMsg = fmt.Sprintf("failed to delete address: %v", msg.Err)
		}
		return m, nil
	case CardDeletedMsg:
		if msg.Err != nil {
			m.ErrorMsg = fmt.Sprintf("failed to delete card: %v", msg.Err)
		}
		return m, nil
	case DelayCompleteMsg:
		m.splashDelayDone = true
		if m.currentPage == splashPage && m.splashDataReady {
			m = m.SwitchPage(shopPage)
			m = m.updateShopViewports()
			return m, nil
		}
		return m, nil
	case splashCursorTickMsg:
		m.splashCursor = !m.splashCursor
		return m, tea.Tick(700*time.Millisecond, func(t time.Time) tea.Msg {
			return splashCursorTickMsg{}
		})
	case tea.KeyMsg:
		// If a form is active, skip global keys — let the page handler own input
		if m.ShippingForm != nil || m.PaymentForm != nil {
			break
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
			m = m.SwitchPage(shopPage).resetPageState()
			m = m.updateShopViewports()
			return m, nil
		case "c":
			m = m.SwitchPage(cartPage).resetPageState()
			m = m.updateCartViewport()
			return m, nil
		case "a":
			m = m.SwitchPage(accountPage).resetPageState()
			if !m.OrdersLoaded {
				return m, m.fetchOrdersCmd()
			}
			return m, nil
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
	case confirmPage:
		return m.ConfirmUpdate(msg)
	case accountPage:
		return m.AccountUpdate(msg)
	}
	return m, nil
}
