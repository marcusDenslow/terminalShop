package theme

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// HuhTheme creates a huh form theme using this project's color palette.
func HuhTheme(t Theme) *huh.Theme {
	var th huh.Theme

	th.FieldSeparator = t.renderer.NewStyle().SetString("\n\n")

	f := &th.Focused
	f.Base = t.renderer.NewStyle().
		PaddingLeft(1).
		BorderStyle(lipgloss.ThickBorder()).
		BorderLeft(true).
		BorderForeground(t.accent)
	f.Title = t.renderer.NewStyle().Foreground(t.body)
	f.Description = t.renderer.NewStyle().Foreground(t.body)
	f.TextInput.Cursor = t.renderer.NewStyle().Foreground(t.highlight)
	f.TextInput.Placeholder = t.renderer.NewStyle().Foreground(t.dim)
	f.TextInput.Prompt = t.renderer.NewStyle().Foreground(t.accent)
	f.TextInput.Text = t.renderer.NewStyle().Foreground(t.accent)
	f.ErrorIndicator = t.renderer.NewStyle().Foreground(t.error)
	f.ErrorMessage = t.renderer.NewStyle().Foreground(t.error)

	f.SelectSelector = t.renderer.NewStyle().Foreground(t.accent).SetString("> ")
	f.Option = t.renderer.NewStyle().Foreground(t.body)
	f.SelectedOption = t.renderer.NewStyle().Foreground(t.highlight).Bold(true)
	f.MultiSelectSelector = t.renderer.NewStyle().Foreground(t.accent).SetString("> ")
	f.SelectedPrefix = t.renderer.NewStyle().Foreground(t.highlight).SetString("[x] ")
	f.UnselectedPrefix = t.renderer.NewStyle().Foreground(t.dim).SetString("[ ] ")
	f.UnselectedOption = t.renderer.NewStyle().Foreground(t.body)

	th.Help = help.New().Styles

	th.Blurred = copyFieldStyles(*f)
	th.Blurred.Base = th.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
	th.Blurred.Title = th.Blurred.Title.Foreground(t.body)
	th.Blurred.SelectSelector = t.renderer.NewStyle().SetString("  ")
	th.Blurred.SelectedOption = t.renderer.NewStyle().Foreground(t.success).Bold(true)
	th.Blurred.Option = t.renderer.NewStyle().Foreground(t.dim)

	return &th
}

// copyFieldStyles returns a copy of FieldStyles (value type, assignment is a copy).
func copyFieldStyles(f huh.FieldStyles) huh.FieldStyles { return f }
