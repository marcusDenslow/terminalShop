package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// newReviewModel returns a Model sitting on the review page with the default
// layout already applied. Use as the starting point for every retry-auth
// state machine test so transitions assert from a realistic baseline.
func newReviewModel(t *testing.T) Model {
	t.Helper()
	m := NewModel("test")
	m.currentPage = reviewPage
	return m
}

// keyR / keyEsc / keyJ build the bubbletea v2 key events Update expects.
// KeyMsg is an interface; KeyPressMsg satisfies it. Key.String() returns
// Text when non-empty, otherwise the keystroke (which is why esc relies on
// Code: tea.KeyEsc rather than Text).
func keyR() tea.KeyMsg   { return tea.KeyPressMsg{Text: "r", Code: 'r'} }
func keyEsc() tea.KeyMsg { return tea.KeyPressMsg{Code: tea.KeyEsc} }
func keyJ() tea.KeyMsg   { return tea.KeyPressMsg{Text: "j", Code: 'j'} }

func TestRetryAuthCmd_NilClient(t *testing.T) {
	m := NewModel("test")
	m.APIClient = nil
	cmd := m.retryAuthCmd(42)
	if cmd == nil {
		t.Fatal("retryAuthCmd returned nil cmd")
	}
	msg := cmd()
	rm, ok := msg.(RetryAuthResultMsg)
	if !ok {
		t.Fatalf("expected RetryAuthResultMsg, got %T", msg)
	}
	if rm.OrderID != 42 {
		t.Errorf("OrderID = %d, want 42", rm.OrderID)
	}
	if rm.Err == nil || !strings.Contains(rm.Err.Error(), "api client unavailable") {
		t.Errorf("expected api-client-unavailable err, got %v", rm.Err)
	}
}

func TestReviewView_RetryingShowsCopy(t *testing.T) {
	m := newReviewModel(t)
	m.review.retrying = true
	out := m.ReviewView()
	if !strings.Contains(out, "retrying authentication") {
		t.Fatalf("missing retrying copy in: %q", out)
	}
}

func TestReviewView_RetryExhaustedShowsCopy(t *testing.T) {
	m := newReviewModel(t)
	m.review.retryExhausted = true
	out := m.ReviewView()
	if !strings.Contains(out, "too many authentication attempts") {
		t.Fatalf("missing retry-exhausted copy in: %q", out)
	}
	if !strings.Contains(out, "press esc to start a new order") {
		t.Fatalf("missing recovery hint in: %q", out)
	}
}

func TestReviewView_AuthFailedAdvertisesRetry(t *testing.T) {
	m := newReviewModel(t)
	m.review.authFailed = true
	out := m.ReviewView()
	if !strings.Contains(out, "press r to retry") {
		t.Fatalf("authFailed copy must advertise r: %q", out)
	}
	if !strings.Contains(out, "esc to go back") {
		t.Fatalf("authFailed copy must advertise esc: %q", out)
	}
}

func TestRenderRequiresActionView_FrictionlessShowsVerifying(t *testing.T) {
	m := newReviewModel(t)
	m.review.requiresAction = true
	m.review.redirectURL = ""
	out := m.renderRequiresActionView()
	if !strings.Contains(out, "verifying with your bank") {
		t.Fatalf("frictionless retry-success path must show verifying copy: %q", out)
	}
}

func TestReviewUpdate_AuthFailed_PressR_StartsRetry(t *testing.T) {
	m := newReviewModel(t)
	m.review.authFailed = true
	m.review.orderID = 7
	m.error = &VisibleError{message: "leftover banner"}

	out, cmd := m.ReviewUpdate(keyR())
	if !out.review.retrying {
		t.Fatal("expected retrying=true after r press")
	}
	if out.error != nil {
		t.Fatal("stale banner should be cleared when retry starts")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd to call retryAuth")
	}
}

func TestReviewUpdate_AuthFailed_PressR_ZeroOrderID_NoOp(t *testing.T) {
	m := newReviewModel(t)
	m.review.authFailed = true
	m.review.orderID = 0

	out, cmd := m.ReviewUpdate(keyR())
	if out.review.retrying {
		t.Fatal("retry must not start without an order id")
	}
	if cmd != nil {
		t.Fatal("expected nil cmd when orderID=0")
	}
}

func TestReviewUpdate_AuthFailed_PressEsc_BouncesToPayment(t *testing.T) {
	m := newReviewModel(t)
	m.review.authFailed = true
	m.review.orderID = 7

	out, _ := m.ReviewUpdate(keyEsc())
	if out.review.authFailed {
		t.Fatal("authFailed must clear on esc")
	}
	if out.currentPage != paymentPage {
		t.Fatalf("currentPage = %v, want paymentPage", out.currentPage)
	}
}

func TestReviewUpdate_AuthFailed_OtherKey_NoOp(t *testing.T) {
	m := newReviewModel(t)
	m.review.authFailed = true
	m.review.orderID = 7

	out, cmd := m.ReviewUpdate(keyJ())
	if !out.review.authFailed {
		t.Fatal("unrelated key must not clear authFailed")
	}
	if out.review.retrying {
		t.Fatal("unrelated key must not start retry")
	}
	if cmd != nil {
		t.Fatal("unrelated key must not produce a cmd")
	}
}

func TestReviewUpdate_Retrying_BlocksKeys(t *testing.T) {
	m := newReviewModel(t)
	m.review.retrying = true
	m.review.authFailed = true
	m.review.orderID = 7

	out, cmd := m.ReviewUpdate(keyR())
	if !out.review.retrying {
		t.Fatal("retrying flag should persist while a retry is in flight")
	}
	if cmd != nil {
		t.Fatal("r press while retrying must not spawn a second cmd")
	}

	out, cmd = m.ReviewUpdate(keyEsc())
	if out.currentPage != reviewPage {
		t.Fatalf("currentPage changed while retrying: %v", out.currentPage)
	}
	if cmd != nil {
		t.Fatal("esc while retrying must not navigate away")
	}
}

func TestReviewUpdate_RetryAuthResult_RequiresAction(t *testing.T) {
	m := newReviewModel(t)
	m.review.retrying = true
	m.review.authFailed = true
	m.review.orderID = 7

	out, cmd := m.ReviewUpdate(RetryAuthResultMsg{
		OrderID:     7,
		Status:      "requires_action",
		RedirectURL: "https://stripe/auth/abc",
	})

	if out.review.retrying {
		t.Fatal("retrying must clear when the result arrives")
	}
	if out.review.authFailed {
		t.Fatal("authFailed must clear on retry success")
	}
	if !out.review.requiresAction {
		t.Fatal("expected requiresAction=true so the QR view renders")
	}
	if out.review.redirectURL != "https://stripe/auth/abc" {
		t.Fatalf("redirectURL = %q", out.review.redirectURL)
	}
	if cmd == nil {
		t.Fatal("expected schedulePollOrderStatus cmd")
	}
	if len(out.footer) != 1 || out.footer[0].key != "esc" || out.footer[0].value != "cancel" {
		t.Fatalf("footer = %+v, want only esc=cancel", out.footer)
	}
}

func TestReviewUpdate_RetryAuthResult_FrictionlessSuccess(t *testing.T) {
	m := newReviewModel(t)
	m.review.retrying = true
	m.review.authFailed = true
	m.review.orderID = 9

	out, cmd := m.ReviewUpdate(RetryAuthResultMsg{
		OrderID: 9,
		Status:  "succeeded",
	})

	if !out.review.requiresAction {
		t.Fatal("frictionless retry must set requiresAction=true to render the verifying view")
	}
	if out.review.redirectURL != "" {
		t.Fatalf("redirectURL must stay empty for frictionless retry, got %q", out.review.redirectURL)
	}
	if cmd == nil {
		t.Fatal("expected poll cmd to wait for the webhook")
	}
}

func TestReviewUpdate_RetryAuthResult_RetryLimit(t *testing.T) {
	m := newReviewModel(t)
	m.review.retrying = true
	m.review.authFailed = true
	m.review.orderID = 7

	out, cmd := m.ReviewUpdate(RetryAuthResultMsg{
		OrderID: 7,
		Err:     errors.New("RETRY_LIMIT: too many authentication attempts, please use a different card"),
	})

	if out.review.retrying {
		t.Fatal("retrying must clear after a result")
	}
	if !out.review.retryExhausted {
		t.Fatal("expected retryExhausted=true on RETRY_LIMIT")
	}
	if out.review.authFailed {
		t.Fatal("authFailed must clear when retryExhausted takes over")
	}
	if cmd != nil {
		t.Fatal("RETRY_LIMIT is terminal — no follow-up cmd")
	}
	if len(out.footer) != 1 || out.footer[0].key != "esc" {
		t.Fatalf("footer = %+v, want only esc=back", out.footer)
	}
}

func TestReviewUpdate_RetryAuthResult_CardGone(t *testing.T) {
	m := newReviewModel(t)
	m.review.retrying = true
	m.review.authFailed = true
	m.review.orderID = 7

	out, _ := m.ReviewUpdate(RetryAuthResultMsg{
		OrderID: 7,
		Err:     errors.New("CARD_NO_LONGER_AVAILABLE: saved card is no longer available, add it again to retry"),
	})

	if out.currentPage != paymentPage {
		t.Fatalf("currentPage = %v, want paymentPage", out.currentPage)
	}
	if out.review.authFailed || out.review.requiresAction {
		t.Fatal("auth state must reset when bouncing to payment")
	}
	if out.review.orderID != 0 {
		t.Fatalf("orderID should reset to 0, got %d", out.review.orderID)
	}
	if out.error == nil || !strings.Contains(out.error.message, "saved card is no longer available") {
		t.Fatalf("error banner missing or wrong copy: %+v", out.error)
	}
}

func TestReviewUpdate_RetryAuthResult_Transient(t *testing.T) {
	m := newReviewModel(t)
	m.review.retrying = true
	m.review.authFailed = true
	m.review.orderID = 7

	out, cmd := m.ReviewUpdate(RetryAuthResultMsg{
		OrderID: 7,
		Err:     errors.New("CARD_DECLINED: card declined"),
	})

	if out.review.retrying {
		t.Fatal("retrying must clear after a result")
	}
	if !out.review.authFailed {
		t.Fatal("authFailed must persist so the user can retry again")
	}
	if out.review.retryExhausted {
		t.Fatal("transient errors must not flip retryExhausted")
	}
	if out.error == nil {
		t.Fatal("expected friendly banner for transient error")
	}
	if cmd != nil {
		t.Fatal("transient error must not start a poll")
	}
}

func TestReviewUpdate_RetryAuthResult_OrderIDMismatch(t *testing.T) {
	m := newReviewModel(t)
	m.review.retrying = true
	m.review.authFailed = true
	m.review.orderID = 7

	out, cmd := m.ReviewUpdate(RetryAuthResultMsg{
		OrderID:     99,
		Status:      "requires_action",
		RedirectURL: "https://wrong",
	})

	if !out.review.retrying {
		t.Fatal("mismatched orderID must leave retrying unchanged")
	}
	if out.review.requiresAction {
		t.Fatal("mismatched orderID must not flip requiresAction")
	}
	if out.review.redirectURL == "https://wrong" {
		t.Fatal("mismatched orderID must not adopt the foreign URL")
	}
	if cmd != nil {
		t.Fatal("mismatched orderID must not start a poll")
	}
}

func TestReviewUpdate_RetryExhausted_PressEsc_BouncesToPayment(t *testing.T) {
	m := newReviewModel(t)
	m.review.retryExhausted = true
	m.review.orderID = 7

	out, _ := m.ReviewUpdate(keyEsc())
	if out.review.retryExhausted {
		t.Fatal("retryExhausted must clear on esc")
	}
	if out.review.orderID != 0 {
		t.Fatalf("orderID should reset to 0, got %d", out.review.orderID)
	}
	if out.currentPage != paymentPage {
		t.Fatalf("currentPage = %v, want paymentPage", out.currentPage)
	}
}

func TestReviewUpdate_RetryExhausted_OtherKey_NoOp(t *testing.T) {
	m := newReviewModel(t)
	m.review.retryExhausted = true

	out, cmd := m.ReviewUpdate(keyR())
	if !out.review.retryExhausted {
		t.Fatal("unrelated key must not clear retryExhausted")
	}
	if cmd != nil {
		t.Fatal("unrelated key must not produce a cmd")
	}
}

func TestReviewUpdate_OrderStatusFailed_SetsRetryFooter(t *testing.T) {
	m := newReviewModel(t)
	m.review.requiresAction = true
	m.review.orderID = 7

	out, _ := m.ReviewUpdate(OrderStatusMsg{OrderID: 7, Status: "failed"})

	if !out.review.authFailed {
		t.Fatal("expected authFailed=true after failed status")
	}
	if out.review.requiresAction {
		t.Fatal("requiresAction must clear when poll resolves to failed")
	}
	hasR, hasEsc := false, false
	for _, c := range out.footer {
		if c.key == "r" && c.value == "retry" {
			hasR = true
		}
		if c.key == "esc" && c.value == "back" {
			hasEsc = true
		}
	}
	if !hasR || !hasEsc {
		t.Fatalf("footer missing r/esc retry hints: %+v", out.footer)
	}
}
