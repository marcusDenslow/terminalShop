package tui

import (
	"strings"
	"testing"

	"terminalShop/pkg/models"

	tea "charm.land/bubbletea/v2"
)

// newPaymentCardListModel parks a Model on the payment page card-list view
// with n synthetic saved cards. Baseline for assertint that the dormant
// "+ add card via ssh" row is dimmed and unreachable by cursor navigation.
func newPaymentCardListModel(t *testing.T, n int) Model {
	t.Helper()
	m := NewModel("test")
	m = m.updateLayout(120, 40)
	m.currentPage = paymentPage
	m.payment.view = 0
	m.payment.form = nil
	m.payment.cardCursor = 0
	for i := range n {
		m.SavedCards = append(m.SavedCards, models.Card{
			ID:       uint(i + 1),
			Brand:    "visa",
			Last4:    "4242",
			ExpMonth: 12,
			ExpYear:  2030,
		})
	}
	return m
}

func keyEnter() tea.KeyMsg { return tea.KeyPressMsg{Code: tea.KeyEnter} }

func TestRenderCardList_SshOptionComingSoon(t *testing.T) {
	m := newPaymentCardListModel(t, 1)
	out := m.RenderCardList()
	if !strings.Contains(out, "add card via ssh") {
		t.Fatalf("ssh option missing from card list output:\n%s", out)
	}
	if !strings.Contains(out, "coming soon") {
		t.Fatalf("want 'coming soon' marked on ssh option:\n%s", out)
	}
}

func TestPaymentUpdate_DownNavigationStopsAtBrowserSlot(t *testing.T) {
	m := newPaymentCardListModel(t, 2)
	for range 10 {
		var cmd tea.Cmd
		m, cmd = m.PaymentUpdate(keyJ())
		_ = cmd
	}
	want := len(m.SavedCards) // browser slot; ssh slot in unreachable
	if m.payment.cardCursor != want {
		t.Fatalf("cursor stopped at %d, want %d (browser slot)", m.payment.cardCursor, want)
	}
}

func TestPaymentUpdate_EnterOnBrowserSlotLaunchesBrowserFlow(t *testing.T) {
	m := newPaymentCardListModel(t, 1)
	m.payment.cardCursor = len(m.SavedCards) // browser slot
	newM, cmd := m.PaymentUpdate(keyEnter())
	if newM.payment.view != 2 {
		t.Fatalf("payment.view = %d, want 2 (browser flow)", newM.payment.view)
	}
	if newM.payment.form != nil {
		t.Fatalf("payment.form must be nil after enter; ssh inline form is dormant")
	}
	if cmd == nil {
		t.Fatalf("expected collectCardCmd, got nil")
	}
}

func TestPaymentUpdate_EnterWithZeroCardsLaunchesBrowserFlow(t *testing.T) {
	m := newPaymentCardListModel(t, 0)
	if m.payment.cardCursor != 0 {
		t.Fatalf("default cursor = %d, want 0 ", m.payment.cardCursor)
	}
	newM, cmd := m.PaymentUpdate(keyEnter())
	if newM.payment.view != 2 {
		t.Fatalf("payment.view = %d, want 2 (browser flow)", newM.payment.view)
	}
	if newM.payment.form != nil {
		t.Fatalf("payment.form must be nil; ssh inline form is dormant")
	}
	if cmd == nil {
		t.Fatalf("expected collectCardCmd, got nil")
	}
}
