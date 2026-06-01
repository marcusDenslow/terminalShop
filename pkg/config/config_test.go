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
		{"unset uses default", "", 20000},
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
