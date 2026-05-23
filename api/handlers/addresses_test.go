package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"testing"
)

func setupAddressTestDB(t *testing.T) (string, models.User) {
	t.Helper()
	testDB := "test_addresses.db"
	_ = os.Remove(testDB)
	database.ResetForTesting()

	db, err := database.Connect(testDB)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	if err := database.Seed(db); err != nil {
		t.Fatalf("failed to seed: %v", err)
	}

	user := models.User{
		SSHKeyFingerprint: "SHA256:addresstestfingerprint",
		SSHPublicKey:      "ssh-ed25519 AAAA addresstestkey",
		Name:              "Address Test User",
		Email:             "address@example.com",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	return testDB, user
}

func TestGetAddressesEmpty(t *testing.T) {
	testDB, user := setupAddressTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	handler := NewAddressHandler("", "", "")

	req := authRequest("GET", "/api/v1/addresses", nil, user.ID)
	w := httptest.NewRecorder()
	handler.GetAddresses(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Addresses []interface{} `json:"addresses"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expexted success to be true")
	}
	if len(resp.Data.Addresses) != 0 {
		t.Errorf("expected + addresses, got %d", len(resp.Data.Addresses))
	}
}

func TestGetAddressesIsolation(t *testing.T) {
	testDB, user1 := setupAddressTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	user2 := models.User{
		SSHKeyFingerprint: "SHA256:otheraddressuser",
		SSHPublicKey:      "ssh-ed25519 AAAA otheraddressuser",
	}
	db.Create(&user2)

	db.Create(&models.Address{
		UserID: user1.ID, Name: "User1 Addr", Street: "111 St",
		City: "Portland", State: "OR", Zip: "97201", Country: "US",
	})
	db.Create(&models.Address{
		UserID: user2.ID, Name: "User2 Addr", Street: "222 St",
		City: "Seattle", State: "WA", Zip: "98101", Country: "US",
	})

	handler := NewAddressHandler("", "", "")

	req := authRequest("GET", "/api/v1/addresses", nil, user1.ID)
	w := httptest.NewRecorder()
	handler.GetAddresses(w, req)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Addresses []models.Address `json:"addresses"`
		} `json:"data"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Data.Addresses) != 1 {
		t.Fatalf("expexted 1 address for user1, got %d", len(resp.Data.Addresses))
	}
	if resp.Data.Addresses[0].Name != "User1 Addr" {
		t.Errorf("expected 'User1 Addr', got %q", resp.Data.Addresses[0].Name)
	}
}

func TestGetAddressesWithData(t *testing.T) {
	testDB, user := setupAddressTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()

	address1 := &models.Address{
		UserID: user.ID, Name: "Home", Street: "123 Main St",
		City: "Portland", State: "OR", Zip: "97201", Country: "US",
	}

	address2 := &models.Address{
		UserID: user.ID, Name: "Work", Street: "456 Office Ave",
		City: "Portland", State: "OR", Zip: "97202", Country: "US",
	}

	db.Create(address1)
	db.Create(address2)

	handler := NewAddressHandler("", "", "")

	req := authRequest("GET", "/api/v1/addresses", nil, user.ID)
	w := httptest.NewRecorder()
	handler.GetAddresses(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected code 200, got %d", w.Code)
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Addresses []models.Address `json:"addresses"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data.Addresses) != 2 {
		t.Errorf("expected 2 addresses, got: %d", len(resp.Data.Addresses))
	}
}
