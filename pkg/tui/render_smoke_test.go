package tui

import (
	"regexp"
	"strings"
	"testing"

	"terminalShop/pkg/models"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

// TestRenderShopSmoke prints the rendered shop frame so chrome alignment can
// be inspected, and asserts that the header, body, and footer boxes share the
// same left and right edges.
func TestRenderShopSmoke(t *testing.T) {
	m := NewModel("smoke")
	m.Loading = false
	m.Cart[m.Coffees[0].ID] = &models.CartItem{CoffeeID: m.Coffees[0].ID, Coffee: m.Coffees[0], Quantity: 4}
	m = m.updateLayout(100, 40)
	m, _ = m.ShopSwitch()

	out := m.renderString()
	plain := ansiRE.ReplaceAllString(out, "")
	t.Log("\n" + plain)

	// Collect left/right border columns of every box-drawing row.
	var lefts, rights []int
	for _, line := range strings.Split(plain, "\n") {
		runes := []rune(line)
		first, last := -1, -1
		for i, r := range runes {
			if strings.ContainsRune("╭╮╰╯│", r) {
				if first == -1 {
					first = i
				}
				last = i
			}
		}
		if first != -1 {
			lefts = append(lefts, first)
			rights = append(rights, last)
		}
	}
	if len(lefts) == 0 {
		t.Fatal("no box-drawing rows found")
	}
	for i := range lefts {
		if lefts[i] != lefts[0] {
			t.Errorf("left edge misaligned: row %d at col %d, expected %d", i, lefts[i], lefts[0])
			break
		}
		if rights[i] != rights[0] {
			t.Errorf("right edge misaligned: row %d at col %d, expected %d", i, rights[i], rights[0])
			break
		}
	}
}

// TestRenderAccountSmoke verifies the account page boxes stay within the
// header/footer chrome bounds. The detail area is unboxed, so inner boxes may
// be narrower than the chrome but must never poke outside it.
func TestRenderAccountSmoke(t *testing.T) {
	m := NewModel("smoke")
	m.Loading = false
	m = m.updateLayout(100, 40)
	m, _ = m.AccountSwitch()

	plain := ansiRE.ReplaceAllString(m.renderString(), "")
	t.Log("\n" + plain)

	var lefts, rights []int
	for _, line := range strings.Split(plain, "\n") {
		runes := []rune(line)
		first, last := -1, -1
		for i, r := range runes {
			if strings.ContainsRune("╭╮╰╯│", r) {
				if first == -1 {
					first = i
				}
				last = i
			}
		}
		if first != -1 {
			lefts = append(lefts, first)
			rights = append(rights, last)
		}
	}
	if len(lefts) == 0 {
		t.Fatal("no box-drawing rows found")
	}
	// Row 0 is the header box: it defines the chrome bounds.
	for i := range lefts {
		if lefts[i] < lefts[0] || rights[i] > rights[0] {
			t.Errorf("box outside chrome: row %d spans %d-%d, chrome is %d-%d",
				i, lefts[i], rights[i], lefts[0], rights[0])
			break
		}
	}
}

// TestRenderShopSmokeStacked prints the stacked (medium/small) layouts.
func TestRenderShopSmokeStacked(t *testing.T) {
	for _, size := range [][2]int{{60, 32}, {45, 30}} {
		m := NewModel("smoke")
		m.Loading = false
		m = m.updateLayout(size[0], size[1])
		m, _ = m.ShopSwitch()
		t.Logf("--- %dx%d ---\n%s", size[0], size[1], ansiRE.ReplaceAllString(m.renderString(), ""))
	}
}
