package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"terminalShop/pkg/models"
)

func TestParseSpendLimitInput(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantNil bool
		wantErr bool
	}{
		{"", 0, true, false},             // blank clears
		{"0", 0, false, false},           // $0 is block-all, NOT a clear
		{"25", 2500, false, false},       // whole dollars
		{"25.50", 2550, false, false},    // dollars + cents
		{"25.5", 2550, false, false},     // one decimal => tenths
		{"0.05", 5, false, false},        // sub-dollar
		{" $25.00 ", 2500, false, false}, // strips $ and whitespace
		{".50", 50, false, false},        // leading dot
		{"-5", 0, false, true},           // negative rejected
		{"abc", 0, false, true},          // non-numeric rejected
		{"25.999", 0, false, true},       // too many decimals rejected
	}
	for _, tc := range cases {
		got, err := parseSpendLimitInput(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%q: want error, got value %v", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error: %v", tc.in, err)
			continue
		}
		if tc.wantNil {
			if got != nil {
				t.Errorf("%q: want nil, got %d", tc.in, *got)
			}
			continue
		}
		if got == nil || *got != tc.want {
			t.Errorf("%q: want %d, got %v", tc.in, tc.want, got)
		}
	}
}

func TestSetSpendLimitCmd_NilClient(t *testing.T) {
	m := NewModel("test")
	m.APIClient = nil
	cmd := m.setSpendLimitCmd(nil)
	if cmd == nil {
		t.Fatal("setSpendLimitCmd returned nil cmd")
	}
	saved, ok := cmd().(spendLimitSavedMsg)
	if !ok {
		t.Fatalf("expected spendLimitSavedMsg, got %T", cmd())
	}
	if saved.Err == nil {
		t.Error("expected error when API client is nil")
	}
}

// TestSpendLimitFocusFlow: enter focuses the input and prefills from the user's
// current limit; esc cancels without mutating the stored limit.
func TestSpendLimitFocusFlow(t *testing.T) {
	m := NewModel("test")
	m = m.updateLayout(100, 40)
	m, _ = m.AccountSwitch()
	m.User = &models.User{ID: 1, SelfLimitCents: intp(2500)}
	for i, item := range models.AccountMenuItems {
		if item == "spend limit" {
			m.account.cursor = i
		}
	}

	m, _ = m.AccountUpdate(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.account.spendLimitFocused {
		t.Fatal("enter did not focus the spend-limit input")
	}
	if got := m.account.spendLimitInput.Value(); got != "25.00" {
		t.Errorf("prefill: want \"25.00\", got %q", got)
	}

	m, _ = m.AccountUpdate(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.account.spendLimitFocused {
		t.Error("esc did not blur the spend-limit input")
	}
	if m.User.SelfLimitCents == nil || *m.User.SelfLimitCents != 2500 {
		t.Error("esc must not change the stored limit")
	}
}

// TestSpendLimitSavedMsg: the async-result handler updates the local user and
// clears focus on success.
func TestSpendLimitSavedMsg(t *testing.T) {
	m := NewModel("test")
	m = m.updateLayout(100, 40)
	m, _ = m.AccountSwitch()
	m.User = &models.User{ID: 1}
	m.account.spendLimitFocused = true
	m.account.spendLimitSaving = true

	m, _ = m.AccountUpdate(spendLimitSavedMsg{Cents: intp(750)})
	if m.account.spendLimitFocused {
		t.Error("successful save should blur the input")
	}
	if m.User.SelfLimitCents == nil || *m.User.SelfLimitCents != 750 {
		t.Errorf("save should update local user limit to 750, got %v", m.User.SelfLimitCents)
	}
}

func intp(i int) *int { return &i }
