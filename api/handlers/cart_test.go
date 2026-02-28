package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"

	"terminalShop/api/middleware"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
)

// setupCartTestDB creates a temp DB with migrated tables, seeded products,
// and a test user. Returns the DB filename and the user.
func setupCartTestDB(t *testing.T) (string, models.User) {
	t.Helper()
	testDB := "test_cart.db"
	os.Remove(testDB)
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
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewCartHandler("")

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
				Items    []interface{} `json:"items"`
				Subtotal int           `json:"subtotal"`
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
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewCartHandler("")

	// Add item (CoffeeID=1, Quantity=2)
	body, _ := json.Marshal(map[string]interface{}{
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
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewCartHandler("")

	// Add item
	body, _ := json.Marshal(map[string]interface{}{"coffee_id": 1, "quantity": 2})
	req := authRequest("PUT", "/api/v1/cart/item", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetItem(w, req)

	// Update quantity
	body, _ = json.Marshal(map[string]interface{}{"coffee_id": 1, "quantity": 5})
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
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data.Cart.Items) != 1 {
		t.Fatalf("Expected 1 item after update, got %d", len(resp.Data.Cart.Items))
	}
	if resp.Data.Cart.Items[0].Quantity != 5 {
		t.Errorf("Expected quantity 5, got %d", resp.Data.Cart.Items[0].Quantity)
	}
}

func TestSetItemRemove(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewCartHandler("")

	// Add item
	body, _ := json.Marshal(map[string]interface{}{"coffee_id": 1, "quantity": 3})
	req := authRequest("PUT", "/api/v1/cart/item", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetItem(w, req)

	// Remove item (quantity 0)
	body, _ = json.Marshal(map[string]interface{}{"coffee_id": 1, "quantity": 0})
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
				Items []interface{} `json:"items"`
			} `json:"cart"`
		} `json:"data"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data.Cart.Items) != 0 {
		t.Errorf("Expected 0 items after removal, got %d", len(resp.Data.Cart.Items))
	}
}

func TestSetItemInvalidProduct(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewCartHandler("")

	body, _ := json.Marshal(map[string]interface{}{"coffee_id": 999, "quantity": 1})
	req := authRequest("PUT", "/api/v1/cart/item", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetItem(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid product, got %d", w.Code)
	}
}

func TestSetItemMissingCoffeeID(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewCartHandler("")

	body, _ := json.Marshal(map[string]interface{}{"quantity": 1})
	req := authRequest("PUT", "/api/v1/cart/item", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetItem(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing coffee_id, got %d", w.Code)
	}
}

func TestClearCart(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewCartHandler("")

	// Add two items
	for _, coffeeID := range []uint{1, 2} {
		body, _ := json.Marshal(map[string]interface{}{"coffee_id": coffeeID, "quantity": 1})
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
				Items []interface{} `json:"items"`
			} `json:"cart"`
		} `json:"data"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data.Cart.Items) != 0 {
		t.Errorf("Expected 0 items after clear, got %d", len(resp.Data.Cart.Items))
	}
}

func TestSetAddress(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer os.Remove(testDB)
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

	handler := NewCartHandler("")

	body, _ := json.Marshal(map[string]interface{}{"address_id": addr.ID})
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
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Data.Cart.AddressID == nil || *resp.Data.Cart.AddressID != addr.ID {
		t.Errorf("Expected address_id %d, got %v", addr.ID, resp.Data.Cart.AddressID)
	}
}

func TestSetAddressWrongUser(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer os.Remove(testDB)
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

	handler := NewCartHandler("")

	body, _ := json.Marshal(map[string]interface{}{"address_id": addr.ID})
	req := authRequest("PUT", "/api/v1/cart/address", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetAddress(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for address not owned by user, got %d", w.Code)
	}
}

func TestSetCard(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer os.Remove(testDB)
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

	handler := NewCartHandler("")

	body, _ := json.Marshal(map[string]interface{}{"card_id": card.ID})
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
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Data.Cart.CardID == nil || *resp.Data.Cart.CardID != card.ID {
		t.Errorf("Expected card_id %d, got %v", card.ID, resp.Data.Cart.CardID)
	}
}

func TestSetCardWrongUser(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer os.Remove(testDB)
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

	handler := NewCartHandler("")

	body, _ := json.Marshal(map[string]interface{}{"card_id": card.ID})
	req := authRequest("PUT", "/api/v1/cart/card", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetCard(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for card not owned by user, got %d", w.Code)
	}
}

func TestConvertCartMissingAddress(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewCartHandler("")

	// Add item but no address or card
	body, _ := json.Marshal(map[string]interface{}{"coffee_id": 1, "quantity": 1})
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
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewCartHandler("")
	db := database.GetDB()

	// Add item and address
	body, _ := json.Marshal(map[string]interface{}{"coffee_id": 1, "quantity": 1})
	req := authRequest("PUT", "/api/v1/cart/item", body, user.ID)
	w := httptest.NewRecorder()
	handler.SetItem(w, req)

	addr := models.Address{UserID: user.ID, Name: "Test", Street: "123 St", City: "PDX", State: "OR", Zip: "97201", Country: "US"}
	db.Create(&addr)
	body, _ = json.Marshal(map[string]interface{}{"address_id": addr.ID})
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
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewCartHandler("")

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
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	handler := NewCartHandler("")

	// Add three different items
	for i := uint(1); i <= 3; i++ {
		body, _ := json.Marshal(map[string]interface{}{"coffee_id": i, "quantity": int(i)})
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
				Items    []interface{} `json:"items"`
				Subtotal int           `json:"subtotal"`
			} `json:"cart"`
		} `json:"data"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data.Cart.Items) != 3 {
		t.Errorf("Expected 3 items, got %d", len(resp.Data.Cart.Items))
	}
	if resp.Data.Cart.Subtotal == 0 {
		t.Error("Expected non-zero subtotal for 3 items")
	}
}

func TestCartIsolation(t *testing.T) {
	testDB, user1 := setupCartTestDB(t)
	defer os.Remove(testDB)
	defer database.ResetForTesting()

	db := database.GetDB()
	user2 := models.User{
		SSHKeyFingerprint: "SHA256:user2fingerprint",
		SSHPublicKey:      "ssh-ed25519 AAAA user2key",
	}
	db.Create(&user2)

	handler := NewCartHandler("")
	r := chi.NewRouter()
	r.Put("/cart/item", handler.SetItem)
	r.Get("/cart", handler.GetCart)

	// User 1 adds item
	body, _ := json.Marshal(map[string]interface{}{"coffee_id": 1, "quantity": 5})
	req := authRequest("PUT", "/cart/item", body, user1.ID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// User 2 adds different item
	body, _ = json.Marshal(map[string]interface{}{"coffee_id": 2, "quantity": 3})
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
	json.NewDecoder(w.Body).Decode(&resp)

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

	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data.Cart.Items) != 1 {
		t.Fatalf("User 2 expected 1 item, got %d", len(resp.Data.Cart.Items))
	}
	if resp.Data.Cart.Items[0].CoffeeID != 2 {
		t.Errorf("User 2 expected coffee_id 2, got %d", resp.Data.Cart.Items[0].CoffeeID)
	}
}
