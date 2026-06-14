package tui

import (
	"image/color"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// Palette and base styles for the boxed shop chrome.
// Ported from the design prototype at ~/programming/test/shop.go.
var (
	cWhite = lipgloss.Color("#EAEAEA")
	cBody  = lipgloss.Color("#9A9A9A")
	cDim   = lipgloss.Color("#5A5A5A")
	cLine  = lipgloss.Color("#3A3A3A")
	cInk   = lipgloss.Color("#161616") // text on a light colored block
	cGreen = lipgloss.Color("#4FB477")
	cRed   = lipgloss.Color("#E06C6C")
	cGold  = lipgloss.Color("#D0962E") // single accent
	cPill  = lipgloss.Color("#2E2E2E") // active-tab pill bg

	pBody = lipgloss.NewStyle().Foreground(cBody)
	pDim  = lipgloss.NewStyle().Foreground(cDim)
	pWht  = lipgloss.NewStyle().Foreground(cWhite)
)

// parseHexColor splits a "#RRGGBB" string into its channel values.
func parseHexColor(s string) (r, g, b int, ok bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(v >> 16), int((v >> 8) & 0xFF), int(v & 0xFF), true
}

// contrastColor returns ink for light backgrounds and white for dark ones,
// using perceived luminance so text stays readable on any product color.
func contrastColor(hex string) color.Color {
	r, g, b, ok := parseHexColor(hex)
	if !ok {
		return cWhite
	}
	lum := (299*r + 587*g + 114*b) / 1000
	if lum > 140 {
		return cInk
	}
	return cWhite
}

// panel wraps content in a rounded box. textW is the inner text width and
// innerH the inner content height; the total render size is textW+4 by
// innerH+2. Ported from the design prototype at ~/programming/test/shop.go,
// adjusted for lipgloss v2 where Width/Height include the border.
func panel(textW, innerH int, content string) string {
	s := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cLine).
		Padding(0, 1).
		Width(textW + 4)
	if innerH > 0 {
		s = s.Height(innerH + 2)
	}
	return s.Render(content)
}
