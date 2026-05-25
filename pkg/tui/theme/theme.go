package theme

import (
	"image/color"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

// Theme holds all colors and styles for the TUI
type Theme struct {
	background color.Color
	border     color.Color
	body       color.Color
	dim        color.Color
	accent     color.Color
	highlight  color.Color
	brand      color.Color
	label      color.Color
	success    color.Color
	error      color.Color

	base lipgloss.Style
	form huh.Theme
}

func BasicTheme() Theme {
	t := Theme{}

	t.background = compat.AdaptiveColor{Dark: lipgloss.Color("#000000"), Light: lipgloss.Color("#FBFCFD")}
	t.border = compat.AdaptiveColor{Dark: lipgloss.Color("#666666"), Light: lipgloss.Color("#D7DBDF")}
	t.body = lipgloss.Color("#AAAAAA")
	t.dim = lipgloss.Color("#666666")
	t.accent = compat.AdaptiveColor{Dark: lipgloss.Color("#FFFFFF"), Light: lipgloss.Color("#11181C")}
	t.highlight = lipgloss.Color("#4682B4")
	t.brand = lipgloss.Color("#4682B4")
	t.label = lipgloss.Color("#999999")
	t.success = lipgloss.Color("#00FF00")
	t.error = lipgloss.Color("196")

	t.base = lipgloss.NewStyle().Foreground(t.body)
	t.form = HuhTheme(t)

	return t
}

func Catppuccin() Theme {
	t := Theme{}

	t.background = compat.AdaptiveColor{Dark: lipgloss.Color("#1e1e2e"), Light: lipgloss.Color("#eff1f5")}
	t.border = compat.AdaptiveColor{Dark: lipgloss.Color("#45475A"), Light: lipgloss.Color("#BCC0CC")}
	t.body = lipgloss.Color("#AAAAAA")
	t.dim = lipgloss.Color("#6C7086")
	t.accent = lipgloss.Color("#CBA6F7")
	t.highlight = lipgloss.Color("#CBA6F7")
	t.brand = lipgloss.Color("#CBA6F7")
	t.label = lipgloss.Color("#A6ADC8")
	t.success = lipgloss.Color("#A6E3A1")
	t.error = lipgloss.Color("#F38BA8")

	t.base = lipgloss.NewStyle().Foreground(t.body)
	t.form = HuhTheme(t)

	return t
}

// Color accessors.
func (t Theme) Background() color.Color { return t.background }
func (t Theme) Border() color.Color     { return t.border }
func (t Theme) Body() color.Color       { return t.body }
func (t Theme) Dim() color.Color        { return t.dim }
func (t Theme) Accent() color.Color     { return t.accent }
func (t Theme) Highlight() color.Color  { return t.highlight }
func (t Theme) Brand() color.Color      { return t.brand }
func (t Theme) Label() color.Color      { return t.label }
func (t Theme) Success() color.Color    { return t.success }
func (t Theme) Error() color.Color      { return t.error }

// Style helpers.
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

func (t Theme) PanelError() lipgloss.Style {
	return t.Base().Background(lipgloss.Color("52")).Foreground(lipgloss.Color("196"))
}

func (t Theme) Form() huh.Theme { return t.form }
