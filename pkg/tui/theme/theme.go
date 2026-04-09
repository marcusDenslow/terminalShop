package theme

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Theme holds all colors and styles for the terminal coffee shop TUI.
type Theme struct {
	renderer *lipgloss.Renderer

	background lipgloss.TerminalColor
	border     lipgloss.TerminalColor
	body       lipgloss.TerminalColor
	dim        lipgloss.TerminalColor
	accent     lipgloss.TerminalColor
	highlight  lipgloss.TerminalColor
	brand      lipgloss.TerminalColor
	label      lipgloss.TerminalColor
	success    lipgloss.TerminalColor
	error      lipgloss.TerminalColor

	base lipgloss.Style
	form *huh.Theme
}

// BasicTheme constructs the default theme for terminal coffee shop.
func BasicTheme(renderer *lipgloss.Renderer) Theme {
	t := Theme{renderer: renderer}

	t.background = lipgloss.AdaptiveColor{Dark: "#000000", Light: "#FBFCFD"}
	t.border = lipgloss.AdaptiveColor{Dark: "#666666", Light: "#D7DBDF"}
	t.body = lipgloss.Color("#AAAAAA")
	t.dim = lipgloss.Color("#666666")
	t.accent = lipgloss.AdaptiveColor{Dark: "#FFFFFF", Light: "#11181C"}
	t.highlight = lipgloss.Color("#4682B4")
	t.brand = lipgloss.Color("#4682B4")
	t.label = lipgloss.Color("#999999")
	t.success = lipgloss.Color("#00FF00")
	t.error = lipgloss.Color("196")

	t.base = renderer.NewStyle().Foreground(t.body)
	t.form = HuhTheme(t)

	return t
}

// Color accessors — use in .Foreground() / .Background() calls.
func (t Theme) Background() lipgloss.TerminalColor { return t.background }
func (t Theme) Border() lipgloss.TerminalColor     { return t.border }
func (t Theme) Body() lipgloss.TerminalColor       { return t.body }
func (t Theme) Dim() lipgloss.TerminalColor        { return t.dim }
func (t Theme) Accent() lipgloss.TerminalColor     { return t.accent }
func (t Theme) Highlight() lipgloss.TerminalColor  { return t.highlight }
func (t Theme) Brand() lipgloss.TerminalColor      { return t.brand }
func (t Theme) Label() lipgloss.TerminalColor      { return t.label }
func (t Theme) Success() lipgloss.TerminalColor    { return t.success }
func (t Theme) Error() lipgloss.TerminalColor      { return t.error }

// Style helpers — return a copy of base with the correct foreground.
// Chain .Bold(), .Width(), .Padding() etc. on the returned style.
func (t Theme) Base() lipgloss.Style          { return t.base }
func (t Theme) TextBody() lipgloss.Style      { return t.Base().Foreground(t.body) }
func (t Theme) TextAccent() lipgloss.Style    { return t.Base().Foreground(t.accent) }
func (t Theme) TextDim() lipgloss.Style       { return t.Base().Foreground(t.dim) }
func (t Theme) TextHighlight() lipgloss.Style { return t.Base().Foreground(t.highlight) }
func (t Theme) TextBrand() lipgloss.Style     { return t.Base().Foreground(t.brand) }
func (t Theme) TextLabel() lipgloss.Style     { return t.Base().Foreground(t.label) }
func (t Theme) TextSuccess() lipgloss.Style   { return t.Base().Foreground(t.success) }
func (t Theme) TextError() lipgloss.Style     { return t.Base().Foreground(t.error) }
func (t Theme) TextLoading() lipgloss.Style   { return t.Base().Foreground(lipgloss.Color("205")) }

// PanelError returns a style for the error banner: dark red bg, red fg.
func (t Theme) PanelError() lipgloss.Style {
	return t.Base().Background(lipgloss.Color("52")).Foreground(lipgloss.Color("196"))
}

// Form returns the huh form theme for shipping and payment forms.
func (t Theme) Form() *huh.Theme { return t.form }
