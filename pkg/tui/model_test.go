package tui

import (
	"testing"
)

func TestModel_updateLayout(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"large terminal", 120, 40},
		{"medium terminal", 70, 25},
		{"small terminal", 45, 20},
		{"undersized", 15, 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel("")
			m.updateLayout(tt.width, tt.height)
			if m.viewportWidth != tt.width {
				t.Errorf("viewportWidth = %d, want %d", m.viewportWidth, tt.width)
			}
			if m.viewportHeight != tt.height {
				t.Errorf("viewportHeight = %d, want %d", m.viewportHeight, tt.height)
			}
		})
	}
}
