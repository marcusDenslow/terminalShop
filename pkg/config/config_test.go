package config

import (
	"testing"
)

func TestLoadMaxOrderCents(t *testing.T) {
	cases := []struct {
		name string
		env  string // "" means unset
		want int
	}{
		{"empty falls back to default", "", 20000},
		{"explicit zero disables", "0", 0},
		{"valid override", "50000", 50000},
		{"negative falls back to default", "-1", 20000},
		{"garbage falls back to default", "two hundred", 20000},
		{"empty after trim falls back to default", "", 20000},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env == "" {
				t.Setenv("MAX_ORDER_CENTS", "")
			} else {
				t.Setenv("MAX_ORDER_CENTS", tc.env)
			}
			got := loadMaxOrderCents()
			if got != tc.want {
				t.Errorf("loadMaxOrderCents() = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestLoadAbandoned3DSThresholdMinutes covers the env-parsing surface for
// the sweep threshold. Zero and negative fall back to the default (unlike
// MaxOrderCents where zero is a deliberate off-switch) because a zero
// threshold would sweep every requires_action row on the first tick and
// race the customer's in-progress challenge.
func TestLoadAbandoned3DSThresholdMinutes(t *testing.T) {
	cases := []struct {
		name string
		env  string // "" means unset
		want int
	}{
		{"empty falls back to default", "", 30},
		{"valid override", "45", 45},
		{"zero falls back to default", "0", 30},
		{"negative falls back to default", "-5", 30},
		{"garbage falls back to default", "half an hour", 30},
		{"valid minimum", "1", 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ABANDONED_3DS_THRESHOLD_MINUTES", tc.env)
			got := loadAbandoned3DSThresholdMinutes()
			if got != tc.want {
				t.Errorf("loadAbandoned3DSThresholdMinutes() = %d, want %d", got, tc.want)
			}
		})
	}
}
