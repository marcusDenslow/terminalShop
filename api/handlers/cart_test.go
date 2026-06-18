package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stripe/stripe-go/v78"

	"terminalShop/api/middleware"
	"terminalShop/pkg/audit"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
)

// setupCartTestDB creates a temp DB with migrated tables, seeded products,
// and a test user. Returns the DB filename and the user.
func setupCartTestDB(t *testing.T) (string, models.User) {
	t.Helper()
	testDB := "test_cart.db"
	_ = os.Remove(testDB)
	database.ResetForTesting()

	db, err := database.Connect(testDB)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}
	if err := database.Seed(db); err != nil {
		t.Fatalf("Failed to seed: %v", err)
	}

	user := models.User{
		SSHKeyFingerprint: "SHA256:testfingerprint",
		SSHPublicKey:      "ssh-ed25519 AAAA testkey",
		Name:              "Test User",
		Email:             "test@example.com",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return testDB, user
}

// authRequest creates an HTTP request with the test user ID in the context.
func authRequest(method, url string, body []byte, userID uint) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, url, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, url, nil)
	}
	ctx := middleware.ContextWithUserID(req.Context(), userID)
	return req.WithContext(ctx)
}

func TestGetCartEmpty(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	handler := NewCartHandler("", 0)

	req := authRequest("GET", "/api/v1/cart", nil, user.ID)
	w := httptest.NewRecorder()

	handler.GetCart(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Cart struct {
				Items    []any `json:"items"`
				Subtotal int   `json:"subtotal"`
			} `json:"cart"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Error("Expected success to be true")
	}
	if len(resp.Data.Cart.Items) != 0 {
		t.Errorf("Expected empty cart, got %d items", len(resp.Data.Cart.Items))
	}
	if resp.Data.Cart.Subtotal != 0 {
		t.Errorf("Expected subtotal 0, got %d", resp.Data.Cart.Subtotal)
	}
}

func TestSetItemAndGetCart(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	handler := NewCartHandler("", 0)

	// Add item (CoffeeID=1, Quantity=2)
	body, _ := json.Marshal(map[string]any{
		"coffee_id": 1,
		"quantity":  2,
	})
	req := authRequest("PUT", "/api/v1/cart/item", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetItem(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("SetItem expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify via GetCart
	req = authRequest("GET", "/api/v1/cart", nil, user.ID)
	w = httptest.NewRecorder()
	handler.GetCart(w, req)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Cart struct {
				Items []struct {
					CoffeeID uint `json:"coffee_id"`
					Quantity int  `json:"quantity"`
					Subtotal int  `json:"subtotal"`
				} `json:"items"`
				Subtotal int `json:"subtotal"`
			} `json:"cart"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Data.Cart.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(resp.Data.Cart.Items))
	}
	if resp.Data.Cart.Items[0].CoffeeID != 1 {
		t.Errorf("Expected coffee_id 1, got %d", resp.Data.Cart.Items[0].CoffeeID)
	}
	if resp.Data.Cart.Items[0].Quantity != 2 {
		t.Errorf("Expected quantity 2, got %d", resp.Data.Cart.Items[0].Quantity)
	}
	if resp.Data.Cart.Subtotal == 0 {
		t.Error("Expected non-zero subtotal")
	}
}

func TestSetItemUpdateQuantity(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	handler := NewCartHandler("", 0)

	// Add item
	body, _ := json.Marshal(map[string]any{"coffee_id": 1, "quantity": 2})
	req := authRequest("PUT", "/api/v1/cart/item", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetItem(w, req)

	// Update quantity
	body, _ = json.Marshal(map[string]any{"coffee_id": 1, "quantity": 5})
	req = authRequest("PUT", "/api/v1/cart/item", body, user.ID)
	w = httptest.NewRecorder()
	handler.SetItem(w, req)

	// Verify
	req = authRequest("GET", "/api/v1/cart", nil, user.ID)
	w = httptest.NewRecorder()
	handler.GetCart(w, req)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Cart struct {
				Items []struct {
					CoffeeID uint `json:"coffee_id"`
					Quantity int  `json:"quantity"`
				} `json:"items"`
			} `json:"cart"`
		} `json:"data"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data.Cart.Items) != 1 {
		t.Fatalf("Expected 1 item after update, got %d", len(resp.Data.Cart.Items))
	}
	if resp.Data.Cart.Items[0].Quantity != 5 {
		t.Errorf("Expected quantity 5, got %d", resp.Data.Cart.Items[0].Quantity)
	}
}

func TestSetItemRemove(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	handler := NewCartHandler("", 0)

	// Add item
	body, _ := json.Marshal(map[string]any{"coffee_id": 1, "quantity": 3})
	req := authRequest("PUT", "/api/v1/cart/item", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetItem(w, req)

	// Remove item (quantity 0)
	body, _ = json.Marshal(map[string]any{"coffee_id": 1, "quantity": 0})
	req = authRequest("PUT", "/api/v1/cart/item", body, user.ID)
	w = httptest.NewRecorder()
	handler.SetItem(w, req)

	// Verify empty
	req = authRequest("GET", "/api/v1/cart", nil, user.ID)
	w = httptest.NewRecorder()
	handler.GetCart(w, req)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Cart struct {
				Items []any `json:"items"`
			} `json:"cart"`
		} `json:"data"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data.Cart.Items) != 0 {
		t.Errorf("Expected 0 items after removal, got %d", len(resp.Data.Cart.Items))
	}
}

func TestSetItemInvalidProduct(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	handler := NewCartHandler("", 0)

	body, _ := json.Marshal(map[string]any{"coffee_id": 999, "quantity": 1})
	req := authRequest("PUT", "/api/v1/cart/item", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetItem(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid product, got %d", w.Code)
	}
}

func TestSetItemMissingCoffeeID(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	handler := NewCartHandler("", 0)

	body, _ := json.Marshal(map[string]any{"quantity": 1})
	req := authRequest("PUT", "/api/v1/cart/item", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetItem(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing coffee_id, got %d", w.Code)
	}
}

func TestClearCart(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	handler := NewCartHandler("", 0)

	// Add two items
	for _, coffeeID := range []uint{1, 2} {
		body, _ := json.Marshal(map[string]any{"coffee_id": coffeeID, "quantity": 1})
		req := authRequest("PUT", "/api/v1/cart/item", body, user.ID)
		w := httptest.NewRecorder()
		handler.SetItem(w, req)
	}

	// Clear cart
	req := authRequest("DELETE", "/api/v1/cart", nil, user.ID)
	w := httptest.NewRecorder()
	handler.ClearCart(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ClearCart expected 200, got %d", w.Code)
	}

	// Verify empty
	req = authRequest("GET", "/api/v1/cart", nil, user.ID)
	w = httptest.NewRecorder()
	handler.GetCart(w, req)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Cart struct {
				Items []any `json:"items"`
			} `json:"cart"`
		} `json:"data"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data.Cart.Items) != 0 {
		t.Errorf("Expected 0 items after clear, got %d", len(resp.Data.Cart.Items))
	}
}

func TestSetAddress(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	addr := models.Address{
		UserID:  user.ID,
		Name:    "Test Address",
		Street:  "123 Main St",
		City:    "Portland",
		State:   "OR",
		Zip:     "97201",
		Country: "US",
	}
	db.Create(&addr)

	handler := NewCartHandler("", 0)

	body, _ := json.Marshal(map[string]any{"address_id": addr.ID})
	req := authRequest("PUT", "/api/v1/cart/address", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetAddress(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("SetAddress expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify cart has the address set
	req = authRequest("GET", "/api/v1/cart", nil, user.ID)
	w = httptest.NewRecorder()
	handler.GetCart(w, req)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Cart struct {
				AddressID *uint `json:"address_id"`
			} `json:"cart"`
		} `json:"data"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)

	if resp.Data.Cart.AddressID == nil || *resp.Data.Cart.AddressID != addr.ID {
		t.Errorf("Expected address_id %d, got %v", addr.ID, resp.Data.Cart.AddressID)
	}
}

func TestSetAddressWrongUser(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	// Create address owned by a different user
	otherUser := models.User{
		SSHKeyFingerprint: "SHA256:otherfingerprint",
		SSHPublicKey:      "ssh-ed25519 AAAA otherkey",
	}
	db.Create(&otherUser)
	addr := models.Address{
		UserID:  otherUser.ID,
		Name:    "Other Address",
		Street:  "456 Other St",
		City:    "Seattle",
		State:   "WA",
		Zip:     "98101",
		Country: "US",
	}
	db.Create(&addr)

	handler := NewCartHandler("", 0)

	body, _ := json.Marshal(map[string]any{"address_id": addr.ID})
	req := authRequest("PUT", "/api/v1/cart/address", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetAddress(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for address not owned by user, got %d", w.Code)
	}
}

func TestSetCard(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	card := models.Card{
		UserID:          user.ID,
		StripePaymentID: "pm_test123",
		Last4:           "4242",
		Brand:           "Visa",
		ExpMonth:        12,
		ExpYear:         2030,
	}
	db.Create(&card)

	handler := NewCartHandler("", 0)

	body, _ := json.Marshal(map[string]any{"card_id": card.ID})
	req := authRequest("PUT", "/api/v1/cart/card", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetCard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("SetCard expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify cart has the card set
	req = authRequest("GET", "/api/v1/cart", nil, user.ID)
	w = httptest.NewRecorder()
	handler.GetCart(w, req)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Cart struct {
				CardID *uint `json:"card_id"`
			} `json:"cart"`
		} `json:"data"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)

	if resp.Data.Cart.CardID == nil || *resp.Data.Cart.CardID != card.ID {
		t.Errorf("Expected card_id %d, got %v", card.ID, resp.Data.Cart.CardID)
	}
}

func TestSetCardWrongUser(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	otherUser := models.User{
		SSHKeyFingerprint: "SHA256:otherfingerprint2",
		SSHPublicKey:      "ssh-ed25519 AAAA otherkey2",
	}
	db.Create(&otherUser)
	card := models.Card{
		UserID:          otherUser.ID,
		StripePaymentID: "pm_other",
		Last4:           "1234",
		Brand:           "Mastercard",
		ExpMonth:        6,
		ExpYear:         2028,
	}
	db.Create(&card)

	handler := NewCartHandler("", 0)

	body, _ := json.Marshal(map[string]any{"card_id": card.ID})
	req := authRequest("PUT", "/api/v1/cart/card", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetCard(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for card not owned by user, got %d", w.Code)
	}
}

func TestConvertCartMissingAddress(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	handler := NewCartHandler("", 0)

	// Add item but no address or card
	body, _ := json.Marshal(map[string]any{"coffee_id": 1, "quantity": 1})
	req := authRequest("PUT", "/api/v1/cart/item", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetItem(w, req)

	// Try to convert
	req = authRequest("POST", "/api/v1/cart/convert", nil, user.ID)
	w = httptest.NewRecorder()
	handler.ConvertCart(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing address, got %d", w.Code)
	}
}

func TestConvertCartMissingCard(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	handler := NewCartHandler("", 0)
	db := database.GetDB()

	// Add item and address
	body, _ := json.Marshal(map[string]any{"coffee_id": 1, "quantity": 1})
	req := authRequest("PUT", "/api/v1/cart/item", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetItem(w, req)

	addr := models.Address{UserID: user.ID, Name: "Test", Street: "123 St", City: "PDX", State: "OR", Zip: "97201", Country: "US"}
	db.Create(&addr)
	body, _ = json.Marshal(map[string]any{"address_id": addr.ID})
	req = authRequest("PUT", "/api/v1/cart/address", body, user.ID)
	w = httptest.NewRecorder()
	handler.SetAddress(w, req)

	// Try to convert (no card)
	req = authRequest("POST", "/api/v1/cart/convert", nil, user.ID)
	w = httptest.NewRecorder()
	handler.ConvertCart(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing card, got %d", w.Code)
	}
}

func TestConvertCartEmpty(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	handler := NewCartHandler("", 0)

	// Empty cart convert
	req := authRequest("POST", "/api/v1/cart/convert", nil, user.ID)
	w := httptest.NewRecorder()
	handler.ConvertCart(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty cart, got %d", w.Code)
	}
}

func TestMultipleItems(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	handler := NewCartHandler("", 0)

	// Add three different items
	for i := uint(1); i <= 3; i++ {
		body, _ := json.Marshal(map[string]any{"coffee_id": i, "quantity": int(i)})
		req := authRequest("PUT", "/api/v1/cart/item", body, user.ID)
		w := httptest.NewRecorder()
		handler.SetItem(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("SetItem for coffee %d failed with %d", i, w.Code)
		}
	}

	// Verify
	req := authRequest("GET", "/api/v1/cart", nil, user.ID)
	w := httptest.NewRecorder()
	handler.GetCart(w, req)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Cart struct {
				Items    []any `json:"items"`
				Subtotal int   `json:"subtotal"`
			} `json:"cart"`
		} `json:"data"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data.Cart.Items) != 3 {
		t.Errorf("Expected 3 items, got %d", len(resp.Data.Cart.Items))
	}
	if resp.Data.Cart.Subtotal == 0 {
		t.Error("Expected non-zero subtotal for 3 items")
	}
}

func TestCartIsolation(t *testing.T) {
	testDB, user1 := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	user2 := models.User{
		SSHKeyFingerprint: "SHA256:user2fingerprint",
		SSHPublicKey:      "ssh-ed25519 AAAA user2key",
	}
	db.Create(&user2)

	handler := NewCartHandler("", 0)
	r := chi.NewRouter()
	r.Put("/cart/item", handler.SetItem)
	r.Get("/cart", handler.GetCart)

	// User 1 adds item
	body, _ := json.Marshal(map[string]any{"coffee_id": 1, "quantity": 5})
	req := authRequest("PUT", "/cart/item", body, user1.ID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// User 2 adds different item
	body, _ = json.Marshal(map[string]any{"coffee_id": 2, "quantity": 3})
	req = authRequest("PUT", "/cart/item", body, user2.ID)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// User 1 cart should have only their item
	req = authRequest("GET", "/cart", nil, user1.ID)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Cart struct {
				Items []struct {
					CoffeeID uint `json:"coffee_id"`
					Quantity int  `json:"quantity"`
				} `json:"items"`
			} `json:"cart"`
		} `json:"data"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data.Cart.Items) != 1 {
		t.Fatalf("User 1 expected 1 item, got %d", len(resp.Data.Cart.Items))
	}
	if resp.Data.Cart.Items[0].CoffeeID != 1 {
		t.Errorf("User 1 expected coffee_id 1, got %d", resp.Data.Cart.Items[0].CoffeeID)
	}
	if resp.Data.Cart.Items[0].Quantity != 5 {
		t.Errorf("User 1 expected quantity 5, got %d", resp.Data.Cart.Items[0].Quantity)
	}

	// User 2 cart should have only their item
	req = authRequest("GET", "/cart", nil, user2.ID)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	_ = json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data.Cart.Items) != 1 {
		t.Fatalf("User 2 expected 1 item, got %d", len(resp.Data.Cart.Items))
	}
	if resp.Data.Cart.Items[0].CoffeeID != 2 {
		t.Errorf("User 2 expected coffee_id 2, got %d", resp.Data.Cart.Items[0].CoffeeID)
	}
}

// intPtr returns a pointer to i, for seeding the nullable User.MaxOrderCents
// per-user cap override in table tests.
func intPtr(i int) *int { return &i }

func TestConvertCartLimit(t *testing.T) {
	cases := []struct {
		name          string
		capCents      int
		quantity      int
		expectOverCap bool
		userCap       *int // per-user admin override; nil = inherit the global capCents
		selfLimit     *int // user self-limit; nil = none. Lower-only: tightens, never raises.
	}{
		{"over cap rejects", 20000, 41, true, nil, nil},   // $205 vs global $200
		{"at cap accepts", 20000, 40, false, nil, nil},    // $200 boundary
		{"under cap accepts", 20000, 39, false, nil, nil}, // $195
		{"zero cap disables", 0, 1000, false, nil, nil},   // explicit global off-switch
		// Per-user admin override (User.MaxOrderCents) takes precedence over the global.
		{"user override below global rejects", 20000, 12, true, intPtr(5000), nil},   // $60 > $50 user cap (global $200 would allow)
		{"user override above global accepts", 20000, 41, false, intPtr(50000), nil}, // $205 < $500 user cap (global $200 would reject)
		{"user override zero disables per user", 20000, 1000, false, intPtr(0), nil}, // per-user off-switch over a live global
		{"nil override falls back to global", 20000, 41, true, nil, nil},             // explicit nil == inherit global $200
		// A negative override is nonsense and must NOT disable the cap; it falls
		// back to the global so a bad DB write can't void a fraud control. Here
		// $205 > global $200 still rejects, at the GLOBAL cap (not the -1).
		{"user override negative falls back to global", 20000, 41, true, intPtr(-1), nil},
		// Self-limit (User.SelfLimitCents) is LOWER-ONLY: it can tighten the effective
		// cap below the admin/global value but must never raise it.
		{"self limit below global rejects", 20000, 12, true, nil, intPtr(5000)},                   // $60 > $50 self-limit (global $200 would allow)
		{"self limit under accepts", 20000, 8, false, nil, intPtr(5000)},                          // $40 < $50 self-limit
		{"self limit cannot raise above global", 20000, 50, true, nil, intPtr(30000)},             // $250: min($200 global,$300 self)=$200 still rejects
		{"self limit tightens disabled global", 0, 12, true, nil, intPtr(5000)},                   // global off-switch, self-limit $50 still caps; $60 rejects
		{"self limit zero blocks everything", 20000, 1, true, nil, intPtr(0)},                     // $5 > $0: self-limit 0 is "block all", NOT an off-switch
		{"self limit negative ignored", 20000, 41, true, nil, intPtr(-1)},                         // self ignored; rejects at global $200
		{"self limit tightens below user override", 10000, 12, true, intPtr(10000), intPtr(5000)}, // $60 > $50 self (under the $100 admin override)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Effective cap the handler should enforce, mirroring ConvertCart:
			// a non-negative admin override wins over the global; then a present,
			// non-negative self-limit folds in via min() (lower-only — 0 means
			// "block all", a negative value is ignored).
			effectiveCap := tc.capCents
			if tc.userCap != nil && *tc.userCap >= 0 {
				effectiveCap = *tc.userCap
			}
			if tc.selfLimit != nil && *tc.selfLimit >= 0 {
				if effectiveCap <= 0 || *tc.selfLimit < effectiveCap {
					effectiveCap = *tc.selfLimit
				}
			}

			testDB, user := setupCartTestDB(t)
			defer func() { _ = os.Remove(testDB) }()
			defer database.ResetForTesting()

			db := database.GetDB()
			user.StripeCustomerID = "cus_test_overlimit"
			user.MaxOrderCents = tc.userCap
			user.SelfLimitCents = tc.selfLimit
			if err := db.Save(&user).Error; err != nil {
				t.Fatalf("saved user: %v", err)
			}

			addr := models.Address{
				UserID: user.ID, Name: "Test", Street: "1 Main St",
				City: "PDX", State: "OR", Zip: "97201", Country: "US",
			}
			if err := db.Create(&addr).Error; err != nil {
				t.Fatalf("seed address: %v", err)
			}

			card := models.Card{
				UserID: user.ID, StripePaymentID: "pm_test_overlimit",
				Last4: "4242", Brand: "Visa", ExpMonth: 12, ExpYear: 2030,
			}
			if err := db.Create(&card).Error; err != nil {
				t.Fatalf("seed card: %v", err)
			}

			// Pin coffee_id=4's price so cart-cap math stays stable across
			// seed-price changes. The cap test is about (price * quantity)
			// vs cap, not which coffee or what the menu currently charges.
			const pinnedPrice = 500
			if err := db.Model(&models.Coffee{}).Where("id = ?", 4).Update("price", pinnedPrice).Error; err != nil {
				t.Fatalf("pin coffee price: %v", err)
			}

			handler := NewCartHandler("", tc.capCents)

			body, _ := json.Marshal(map[string]any{"coffee_id": 4, "quantity": tc.quantity})
			req := authRequest("PUT", "/api/v1/cart/item", body, user.ID)
			w := httptest.NewRecorder()
			handler.SetItem(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("SetItem: %d %s", w.Code, w.Body.String())
			}

			// set address
			body, _ = json.Marshal(map[string]any{"address_id": addr.ID})
			req = authRequest("PUT", "/api/v1/cart/address", body, user.ID)
			w = httptest.NewRecorder()
			handler.SetAddress(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("SetAddress: %d %s", w.Code, w.Body.String())
			}

			// set card
			body, _ = json.Marshal(map[string]any{"card_id": card.ID})
			req = authRequest("PUT", "/api/v1/cart/card", body, user.ID)
			w = httptest.NewRecorder()
			handler.SetCard(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("SetCard: %d %s", w.Code, w.Body.String())
			}

			// Capture audit slog ONLY for ConvertCart's emit window. Placed
			// here (not at top of t.Run) so setup INFO logs from
			// database.Connect/Migrate/Seed don't pollute the buffer with
			// non-audit records - json.Unmarshal below requires a single
			// JSON object, not multi-document input. slog.Default is
			// process-global - do NOT add t.Parallel() to subtests here.
			var auditBuf bytes.Buffer
			if tc.expectOverCap {
				prev := slog.Default()
				slog.SetDefault(slog.New(slog.NewJSONHandler(&auditBuf, nil)))
				t.Cleanup(func() { slog.SetDefault(prev) })
			}

			// Convert
			req = authRequest("POST", "/api/v1/cart/convert", nil, user.ID)
			w = httptest.NewRecorder()
			handler.ConvertCart(w, req)

			var resp struct {
				Success bool `json:"success"`
				Error   struct {
					Code    string         `json:"code"`
					Message string         `json:"message"`
					Details map[string]any `json:"details"`
				} `json:"error"`
			}
			_ = json.NewDecoder(w.Body).Decode(&resp)

			isOverCap := w.Code == http.StatusBadRequest && resp.Error.Code == "CART_OVER_LIMIT"
			if isOverCap != tc.expectOverCap {
				t.Fatalf("expectOverCap=%v got code=%d errCode=%q msg=%q", tc.expectOverCap, w.Code, resp.Error.Code, resp.Error.Message)
			}
			if tc.expectOverCap {
				if got, _ := resp.Error.Details["limit_cents"].(float64); int(got) != effectiveCap {
					t.Errorf("limit_cents in details: want %d, got %v", effectiveCap, resp.Error.Details["limit_cents"])
				}
				if got, _ := resp.Error.Details["total_cents"].(float64); int(got) != tc.quantity*pinnedPrice {
					t.Errorf("total_cents in details: want %d, got %v", tc.quantity*pinnedPrice, resp.Error.Details["total_cents"])
				}

				if auditBuf.Len() == 0 {
					t.Fatalf("audit: expected cart_rejected slog line, got nothing")
				}

				// The buffer may hold more than one JSON line (e.g. the negative-
				// override warning precedes the audit record), so scan for the
				// cart_rejected event rather than assuming it's the only line.
				var rec map[string]any
				for line := range bytes.SplitSeq(bytes.TrimSpace(auditBuf.Bytes()), []byte("\n")) {
					var m map[string]any
					if err := json.Unmarshal(line, &m); err != nil {
						continue
					}
					if m["event"] == "cart_rejected" {
						rec = m
						break
					}
				}
				if rec == nil {
					t.Fatalf("audit: no cart_rejected slog record found\nraw: %s", auditBuf.String())
				}
				if n, _ := rec["user_id"].(float64); uint(n) != user.ID {
					t.Errorf("audit user_id: want %d, got %v", user.ID, rec["user_id"])
				}
				if n, _ := rec["total_cents"].(float64); int(n) != tc.quantity*pinnedPrice {
					t.Errorf("audit total_cents: want %d, got %v", tc.quantity*pinnedPrice, rec["total_cents"])
				}
				if n, _ := rec["cap_cents"].(float64); int(n) != effectiveCap {
					t.Errorf("audit cap_cents: want %d, got %v", effectiveCap, rec["cap_cents"])
				}
			}
		})
	}
}

func seedConvertReadyCart(t *testing.T, user models.User) {
	t.Helper()
	db := database.GetDB()

	user.StripeCustomerID = "cus_test_cit"
	if err := db.Save(&user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}

	addr := models.Address{
		UserID: user.ID, Name: "test", Street: "1 Main St",
		City: "PDX", State: "OR", Zip: "97201", Country: "US",
	}
	if err := db.Create(&addr).Error; err != nil {
		t.Fatalf("seed address: %v", err)
	}

	card := models.Card{
		UserID: user.ID, StripePaymentID: "pm_test_cit",
		Last4: "4242", Brand: "Visa", ExpMonth: 12, ExpYear: 2030,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("seed card: %v", err)
	}
	if err := db.Model(&models.Coffee{}).Where("id = ?", 4).Update("price", 500).Error; err != nil {
		t.Fatalf("pin coffee price: %v", err)
	}

	h := NewCartHandler("", 0)
	body, _ := json.Marshal(map[string]any{"coffee_id": 4, "quantity": 2})
	w := httptest.NewRecorder()
	h.SetItem(w, authRequest("PUT", "/api/v1/cart/item", body, user.ID))
	if w.Code != http.StatusOK {
		t.Fatalf("SetItem: %d %s", w.Code, w.Body.String())
	}
	body, _ = json.Marshal(map[string]any{"address_id": addr.ID})
	w = httptest.NewRecorder()
	h.SetAddress(w, authRequest("PUT", "/api/v1/cart/address", body, user.ID))
	if w.Code != http.StatusOK {
		t.Fatalf("SetAddress: %d %s", w.Code, w.Body.String())
	}
	body, _ = json.Marshal(map[string]any{"card_id": card.ID})
	w = httptest.NewRecorder()
	h.SetCard(w, authRequest("PUT", "/api/v1/cart/card", body, user.ID))
	if w.Code != http.StatusOK {
		t.Fatalf("SetCard: %d %s", w.Code, w.Body.String())
	}
}

func TestConvertCartIsCustomerInitiated(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	var gotParams *stripe.PaymentIntentParams
	orig := paymentIntentNewFn
	defer func() { paymentIntentNewFn = orig }()
	paymentIntentNewFn = func(p *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		gotParams = p
		return &stripe.PaymentIntent{ID: "pi_cit", Status: stripe.PaymentIntentStatusSucceeded}, nil
	}

	seedConvertReadyCart(t, user)
	handler := NewCartHandler("http://localhost", 0)

	w := httptest.NewRecorder()
	handler.ConvertCart(w, authRequest("POST", "/api/v1/cart/convert", nil, user.ID))

	if w.Code != http.StatusOK {
		t.Fatalf("convert: want 200, got %d body=%s", w.Code, w.Body.String())
	}
	if gotParams == nil {
		t.Fatalf("paymentIntentNewFn was never called")
	}
	if gotParams.OffSession != nil {
		t.Errorf("CIT charge must not set off_session; got %v", *gotParams.OffSession)
	}
	if gotParams.Confirm == nil || !*gotParams.Confirm {
		t.Error("expected Confirm=true on the charge")
	}
	if gotParams.ReturnURL == nil || *gotParams.ReturnURL == "" {
		t.Error("expected ReturnURL set so redirect-based 3DS can populate next_action")
	}
}

// TestConvertCartRequiresAction asserts that when the bank requires SCA, the
// CIT charge returns status=requires_action (the customer-initiated shape per
// the Stripe sample) and the handler responds 202 + redirect URL, persists the
// order in requires_action, and records the audit event the retry cap counts.
// next_action.redirect_to_url is pre-populated so respondRequiresAction skips
// its re-confirm and never touches the live Stripe API (hermetic).
func TestConvertCartRequiresAction(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	// Wire audit persistence so the order_requires_action row actually lands
	// (audit.db is package-global and nil unless set — see api/main.go:71).
	// Reset to nil after so it does not leak into other tests in this package.
	audit.SetDB(database.GetDB())
	defer audit.SetDB(nil)

	orig := paymentIntentNewFn
	defer func() { paymentIntentNewFn = orig }()
	paymentIntentNewFn = func(_ *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		return &stripe.PaymentIntent{
			ID:     "pi_sca",
			Status: stripe.PaymentIntentStatusRequiresAction,
			NextAction: &stripe.PaymentIntentNextAction{
				RedirectToURL: &stripe.PaymentIntentNextActionRedirectToURL{
					URL: "https://hooks.stripe.com/3ds/test",
				},
			},
		}, nil
	}

	seedConvertReadyCart(t, user)
	handler := NewCartHandler("http://localhost", 0)

	w := httptest.NewRecorder()
	handler.ConvertCart(w, authRequest("POST", "/api/v1/cart/convert", nil, user.ID))

	if w.Code != http.StatusAccepted {
		t.Fatalf("convert: want 202, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			OrderID     uint   `json:"order_id"`
			Status      string `json:"status"`
			RedirectURL string `json:"redirect_url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Status != "requires_action" {
		t.Errorf("status: want requires_action, got %q", resp.Data.Status)
	}
	if resp.Data.RedirectURL == "" {
		t.Error("expected redirect_url in 202 response")
	}

	db := database.GetDB()
	var order models.Order
	if err := db.First(&order, resp.Data.OrderID).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if order.Status != models.OrderStatusRequiresAction {
		t.Errorf("order status: want requires_action, got %q", order.Status)
	}
	if order.StripePaymentID != "pi_sca" {
		t.Errorf("order stripe_payment_id: want pi_sca, got %q", order.StripePaymentID)
	}

	var events int64
	db.Model(&models.OrderEvent{}).
		Where("order_id = ? AND type = ?", order.ID, audit.EventOrderRequiresAction).
		Count(&events)
	if events != 1 {
		t.Errorf("expected 1 order_requires_action audit event, got %d", events)
	}
}

// TestSetCard_ExpiredReturns410 verifies that selecting an already-expired
// card on the cart returns 410 + CARD_STORAGE_EXPIRED and soft-deletes the
// card. Exercises cart.go SetCard expired-card branch
func TestSetCard_ExpiredReturns410(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	past := time.Now().Add(-1 * time.Hour)
	card := models.Card{
		UserID: user.ID, StripePaymentID: "pm_expired", Last4: "1111",
		Brand: "Visa", ExpMonth: 1, ExpYear: 2030, StorageExpiresAt: &past,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("seed card: %v", err)
	}

	handler := NewCartHandler("", 0)
	body, _ := json.Marshal(map[string]any{"card_id": card.ID})
	req := authRequest("PUT", "/api/v1/cart/card", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetCard(w, req)

	if w.Code != http.StatusGone {
		t.Fatalf("want 410, got %d, body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error.Code != "CARD_STORAGE_EXPIRED" {
		t.Fatalf("want CARD_STORAGE_EXPIRED, got %q", resp.Error.Code)
	}

	var soft models.Card
	if err := db.Unscoped().First(&soft, card.ID).Error; err != nil {
		t.Fatalf("reload card unscoped: %v", err)
	}
	if !soft.DeletedAt.Valid {
		t.Fatalf("expired card was not soft-deleted")
	}
}

// TestConvertCart_ExpiredReturns410 verifies that attempting checkout with an
// expired saved card shor-circuits at the validation step with 410 +
// CARD_STORAGE_EXPIRED before any stripe interaction
func TestConvertCart_ExpiredReturns410(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()

	body, _ := json.Marshal(map[string]any{"coffee_id": 1, "quantity": 1})
	req := authRequest("PUT", "/api/v1/vart/item", body, user.ID)
	w := httptest.NewRecorder()
	NewCartHandler("", 0).SetItem(w, req)

	addr := models.Address{
		UserID: user.ID, Name: "Test", Street: "123 St",
		City: "PDX", State: "OR", Zip: "97201", Country: "US",
	}
	db.Create(&addr)
	body, _ = json.Marshal(map[string]any{"address_id": addr.ID})
	req = authRequest("PUT", "/api/v1/cart/address", body, user.ID)
	w = httptest.NewRecorder()
	NewCartHandler("", 0).SetAddress(w, req)

	past := time.Now().Add(-1 * time.Hour)
	card := models.Card{
		UserID: user.ID, StripePaymentID: "pm_expired", Last4: "1111",
		Brand: "Visa", ExpMonth: 1, ExpYear: 2030, StorageExpiresAt: &past,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("seed card: %v", err)
	}
	if err := db.Model(&models.Cart{}).Where("user_id = ?", user.ID).Update("card_id", card.ID).Error; err != nil {
		t.Fatalf("attach card to card: %v", err)
	}

	req = authRequest("POST", "/api/v1/cart/convert", nil, user.ID)
	w = httptest.NewRecorder()
	NewCartHandler("", 0).ConvertCart(w, req)

	if w.Code != http.StatusGone {
		t.Fatalf("want 410, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error.Code != "CARD_STORAGE_EXPIRED" {
		t.Fatalf("want CARD_STORAGE_EXPIRED, got %q", resp.Error.Code)
	}
}

// TestRetryAuth exercises the pre-Stripe validation branches of
// CartHandler.RetryAuth. The happy paths (Confirm → requires_action /
// succeeded) hit the live Stripe API and aren't covered here — see
// sca-psd2-compliance.md "Test plan" for manual coverage with the
// 4000 0027 6000 3184 test card. Caveat #14 in that doc still applies:
// NewCartHandler takes 2 args (appURL, maxOrderCents) and the seeded user
// must have a non-empty StripeCustomerID so the Convert/Retry paths stay
// hermetic.
func TestRetryAuth(t *testing.T) {
	cases := []struct {
		name        string
		ownedByUser bool
		seedStatus  models.OrderStatus
		seedPayment string
		seedEvents  int
		wantStatus  int
		wantErrCode string
	}{
		{
			name:        "wrong user returns not found",
			ownedByUser: false, seedStatus: models.OrderStatusFailed,
			seedPayment: "pi_test_wrong_user", seedEvents: 1,
			wantStatus: http.StatusNotFound, wantErrCode: "ORDER_NOT_FOUND",
		},
		{
			name:        "missing payment intent rejects",
			ownedByUser: true, seedStatus: models.OrderStatusPending,
			seedPayment: "", seedEvents: 0,
			wantStatus: http.StatusConflict, wantErrCode: "INVALID_STATE",
		},
		{
			name:        "paid order is not retryable",
			ownedByUser: true, seedStatus: models.OrderStatusPaid,
			seedPayment: "pi_test_paid", seedEvents: 1,
			wantStatus: http.StatusConflict, wantErrCode: "INVALID_STATE",
		},
		{
			name:        "refunded order is not retryable",
			ownedByUser: true, seedStatus: models.OrderStatusRefunded,
			seedPayment: "pi_test_refunded", seedEvents: 1,
			wantStatus: http.StatusConflict, wantErrCode: "INVALID_STATE",
		},
		{
			name:        "requires_action honors retry cap",
			ownedByUser: true, seedStatus: models.OrderStatusRequiresAction,
			seedPayment: "pi_test_req_action_cap", seedEvents: maxAuthIssuances,
			wantStatus: http.StatusTooManyRequests, wantErrCode: "RETRY_LIMIT",
		},
		{
			name:        "retry cap reached returns 429",
			ownedByUser: true, seedStatus: models.OrderStatusFailed,
			seedPayment: "pi_test_capped", seedEvents: maxAuthIssuances,
			wantStatus: http.StatusTooManyRequests, wantErrCode: "RETRY_LIMIT",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testDB, user := setupCartTestDB(t)
			defer func() { _ = os.Remove(testDB) }()
			defer database.ResetForTesting()

			db := database.GetDB()
			user.StripeCustomerID = "cus_test_retry"
			if err := db.Save(&user).Error; err != nil {
				t.Fatalf("save user: %v", err)
			}

			ownerID := user.ID
			if !tc.ownedByUser {
				other := models.User{
					SSHKeyFingerprint: "SHA256:retryother",
					SSHPublicKey:      "ssh-ed25519 AAAA retryother",
				}
				if err := db.Create(&other).Error; err != nil {
					t.Fatalf("seed other user: %v", err)
				}
				ownerID = other.ID
			}

			card := models.Card{
				UserID: ownerID, StripePaymentID: "pm_test_retry",
				Last4: "4242", Brand: "Visa", ExpMonth: 12, ExpYear: 2030,
			}
			if err := db.Create(&card).Error; err != nil {
				t.Fatalf("seed card: %v", err)
			}

			order := models.Order{
				UserID:          ownerID,
				CardID:          card.ID,
				StripePaymentID: tc.seedPayment,
				Status:          tc.seedStatus,
				Subtotal:        500, Total: 500,
				ShippingName: "Test", ShippingStreet: "1 Main St",
				ShippingCity: "PDX", ShippingState: "OR",
				ShippingZip: "97201", ShippingCountry: "US",
			}
			if err := db.Create(&order).Error; err != nil {
				t.Fatalf("seed order: %v", err)
			}

			for i := 0; i < tc.seedEvents; i++ {
				evt := models.OrderEvent{
					OrderID: order.ID,
					Type:    audit.EventOrderRequiresAction,
					Actor:   fmt.Sprintf("user:%d", ownerID),
					Payload: `{}`,
				}
				if err := db.Create(&evt).Error; err != nil {
					t.Fatalf("seed event: %v", err)
				}
			}

			handler := NewCartHandler("http://localhost", 0)
			req := orderRequest("POST",
				fmt.Sprintf("/api/v1/orders/%d/retry-auth", order.ID), nil, user.ID, fmt.Sprintf("%d", order.ID))
			w := httptest.NewRecorder()
			handler.RetryAuth(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d body=%s", tc.wantStatus, w.Code, w.Body.String())
			}

			var resp struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			_ = json.NewDecoder(w.Body).Decode(&resp)
			if resp.Error.Code != tc.wantErrCode {
				t.Fatalf("err code: want %q, got %q, body=%s", tc.wantErrCode, resp.Error.Code, w.Body.String())
			}
		})
	}
}

func TestRetryAuth_OrderNotFound(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	handler := NewCartHandler("http://localhost", 0)
	req := orderRequest("POST", "/api/v1/orders/9999/retry-auth", nil, user.ID, "9999")
	w := httptest.NewRecorder()
	handler.RetryAuth(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRespondRequiresAction_SetStatusRequiresAction(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	card := models.Card{
		UserID: user.ID, StripePaymentID: "pm_test_req_action",
		Last4: "4242", Brand: "Visa", ExpMonth: 12, ExpYear: 2030,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("seed card: %v", err)
	}

	order := models.Order{
		UserID: user.ID, CardID: card.ID,
		Status:   models.OrderStatusPending,
		Subtotal: 500, Total: 500,
		ShippingName: "Test", ShippingStreet: "1 Main St",
		ShippingCity: "PDX", ShippingState: "OR",
		ShippingZip: "97201", ShippingCountry: "US",
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}

	pi := &stripe.PaymentIntent{
		ID: "pi_test_action_set",
		NextAction: &stripe.PaymentIntentNextAction{
			RedirectToURL: &stripe.PaymentIntentNextActionRedirectToURL{
				URL: "https://hooks.stripe.com/redirect/test/test_3ds",
			},
		},
	}

	handler := NewCartHandler("http://localhost", 0)
	w := httptest.NewRecorder()
	handler.respondRequiresAction(w, &order, card.StripePaymentID, pi)

	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			OrderID         uint   `json:"order_id"`
			PaymentIntentID string `json:"payment_intent_id"`
			Status          string `json:"status"`
			RedirectURL     string `json:"redirect_url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Data.Status != "requires_action" {
		t.Fatalf("response status: %q, got %q", "requires_action", resp.Data.Status)
	}
	if resp.Data.RedirectURL == "" {
		t.Fatalf("expected redirect url, got empty")
	}
	if resp.Data.PaymentIntentID != pi.ID {
		t.Fatalf("payment intent id: want %q, got %q", pi.ID, resp.Data.PaymentIntentID)
	}

	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if reloaded.Status != models.OrderStatusRequiresAction {
		t.Fatalf("order.Status: want %q, got %q", models.OrderStatusRequiresAction, reloaded.Status)
	}
	if reloaded.StripePaymentID != pi.ID {
		t.Fatalf("order.StripePaymentID: want %q, got %q", pi.ID, reloaded.StripePaymentID)
	}
}

// TestRespondRequiresAction_OverridesFailedFromWebhookRace simulates the race
// described in sca-psd2-compliance.md note #3 / caveat #4: the
// payment_intent.payment_failed webhook arrives BEFORE respondRequiresAction
// returns and flips the row to OrderStatusFailed. The helper must override
// that back to OrderStatusRequiresAction so the in-flight 3DS state surfaces
// correctly to the TUI.
func TestRespondRequiresAction_OverridesFailedFromWebhookRace(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	card := models.Card{
		UserID: user.ID, StripePaymentID: "pm_test_race",
		Last4: "4242", Brand: "Visa", ExpMonth: 12, ExpYear: 2030,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("seed card: %v", err)
	}

	order := models.Order{
		UserID: user.ID, CardID: card.ID,
		Status:   models.OrderStatusFailed,
		Subtotal: 500, Total: 500,
		ShippingName: "Test", ShippingStreet: "1 Main St",
		ShippingCity: "PDX", ShippingState: "OR",
		ShippingZip: "97201", ShippingCountry: "US",
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}

	pi := &stripe.PaymentIntent{
		ID: "pi_test_race",
		NextAction: &stripe.PaymentIntentNextAction{
			RedirectToURL: &stripe.PaymentIntentNextActionRedirectToURL{
				URL: "https://hooks.stripe.com/redirect/test_race",
			},
		},
	}

	handler := NewCartHandler("http://localhost", 0)
	w := httptest.NewRecorder()
	handler.respondRequiresAction(w, &order, card.StripePaymentID, pi)

	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d body=%s", w.Code, w.Body.String())
	}

	var reloaded models.Order
	if err := db.First(&reloaded, order.ID).Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if reloaded.Status != models.OrderStatusRequiresAction {
		t.Fatalf("override failed: want %q, got %q", models.OrderStatusRequiresAction, reloaded.Status)
	}
}
