package theme

import (
	"charm.land/bubbles/v2/help"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// HuhTheme creates a huh form theme using this project's color palette.
func HuhTheme(t Theme) huh.Theme {
	var s huh.Styles

	s.FieldSeparator = lipgloss.NewStyle().SetString("\n\n")

	f := &s.Focused
	f.Base = lipgloss.NewStyle().
		PaddingLeft(1).
		BorderStyle(lipgloss.ThickBorder()).
		BorderLeft(true).
		BorderForeground(t.accent)
	f.Title = lipgloss.NewStyle().Foreground(t.body)
	f.Description = lipgloss.NewStyle().Foreground(t.body)
	f.TextInput.Cursor = lipgloss.NewStyle().Foreground(t.highlight)
	f.TextInput.Placeholder = lipgloss.NewStyle().Foreground(t.dim)
	f.TextInput.Prompt = lipgloss.NewStyle().Foreground(t.accent)
	f.TextInput.Text = lipgloss.NewStyle().Foreground(t.accent)
	f.ErrorIndicator = lipgloss.NewStyle().Foreground(t.error)
	f.ErrorMessage = lipgloss.NewStyle().Foreground(t.error)

	f.SelectSelector = lipgloss.NewStyle().Foreground(t.accent).SetString("> ")
	f.Option = lipgloss.NewStyle().Foreground(t.body)
	f.SelectedOption = lipgloss.NewStyle().Foreground(t.highlight).Bold(true)
	f.MultiSelectSelector = lipgloss.NewStyle().Foreground(t.accent).SetString("> ")
	f.SelectedPrefix = lipgloss.NewStyle().Foreground(t.highlight).SetString("[x] ")
	f.UnselectedPrefix = lipgloss.NewStyle().Foreground(t.dim).SetString("[ ] ")
	f.UnselectedOption = lipgloss.NewStyle().Foreground(t.body)

	s.Help = help.New().Styles

	s.Blurred = copyFieldStyles(*f)
	s.Blurred.Base = s.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
	s.Blurred.Title = s.Blurred.Title.Foreground(t.body)
	s.Blurred.SelectSelector = lipgloss.NewStyle().SetString("  ")
	s.Blurred.SelectedOption = lipgloss.NewStyle().Foreground(t.success).Bold(true)
	s.Blurred.Option = lipgloss.NewStyle().Foreground(t.dim)

	return huh.ThemeFunc(func(bool) *huh.Styles { return &s })
}

func copyFieldStyles(f huh.FieldStyles) huh.FieldStyles { return f }
