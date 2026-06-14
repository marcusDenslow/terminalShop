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

// TestRenderOrderWindowing verifies the windowed order list: whole cards
// only, with ↑/↓ arrow rows marking hidden items.
func TestRenderOrderWindowing(t *testing.T) {
	m := NewModel("smoke")
	m.Loading = false
	m.OrdersLoaded = true
	for i := 1; i <= 9; i++ {
		m.Orders = append(m.Orders, models.Order{
			ID:     uint(9000 + i),
			Status: models.OrderStatusPaid,
			Total:  1000 + i*100,
		})
	}
	m = m.updateLayout(100, 40)
	m, _ = m.AccountSwitch()

	// Preview mode: window starts at the top, arrow only below.
	plain := ansiRE.ReplaceAllString(m.renderString(), "")
	t.Log("\n" + plain)
	if !strings.Contains(plain, "↓ 4 more") {
		t.Errorf("preview mode: expected '↓ 4 more' indicator")
	}
	if strings.Contains(plain, "↑") {
		t.Errorf("preview mode: unexpected up arrow")
	}

	// Browse mode scrolled to the middle: arrows on both ends.
	m.account.orderViewState = 1
	m.account.orderCursor = 6
	m = m.scrollToAccountDetailItem()
	plain = ansiRE.ReplaceAllString(m.renderString(), "")
	t.Log("\n" + plain)
	if !strings.Contains(plain, "↑ 2 more") || !strings.Contains(plain, "↓ 2 more") {
		t.Errorf("browse mode: expected '↑ 2 more' and '↓ 2 more' indicators")
	}
}

// TestRenderCartSmoke verifies the cart page cards line up with the chrome.
func TestRenderCartSmoke(t *testing.T) {
	m := NewModel("smoke")
	m.Loading = false
	// Seed coffees carry no IDs, so assign them for distinct cart keys
	for i := range m.Coffees {
		m.Coffees[i].ID = uint(i + 1)
	}
	m.Cart[m.Coffees[0].ID] = &models.CartItem{CoffeeID: m.Coffees[0].ID, Coffee: m.Coffees[0], Quantity: 2}
	m.Cart[m.Coffees[1].ID] = &models.CartItem{CoffeeID: m.Coffees[1].ID, Coffee: m.Coffees[1], Quantity: 1}
	m = m.updateLayout(100, 40)
	m, _ = m.CartSwitch()

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
	// Every box row (header, cards, footer) must share the chrome edges.
	for i := range lefts {
		if lefts[i] != lefts[0] || rights[i] != rights[0] {
			t.Errorf("box edge misaligned: row %d spans %d-%d, expected %d-%d",
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
