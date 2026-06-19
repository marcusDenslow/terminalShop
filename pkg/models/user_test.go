package models

import "testing"

// TestToPublic_IncludesPrivacyMode guards the converter: PublicUser must carry
// the user's PrivacyMode so the TUI can reflect the account toggle's real state.
// (Regression: ToPublic originally omitted this field and always reported false.)
func TestToPublic_IncludesPrivacyMode(t *testing.T) {
	for _, want := range []bool{true, false} {
		u := User{ID: 1, PrivacyMode: want}
		if got := u.ToPublic().PrivacyMode; got != want {
			t.Errorf("ToPublic().PrivacyMode = %v, want %v", got, want)
		}
	}
}
