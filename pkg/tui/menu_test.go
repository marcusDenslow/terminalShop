package tui

import (
	"testing"
)

func TestModel_BuildMenuView(t *testing.T) {
	tests := []struct {
		name string
		m    Model
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.BuildMenuView(); got != tt.want {
				t.Errorf("Model.BuildMenuView() = %v, want %v", got, tt.want)
			}
		})
	}
}
