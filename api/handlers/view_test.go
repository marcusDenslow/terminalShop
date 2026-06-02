package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
)

// TestGetViewInit_FiltersExpiredCardsAndOrders drives the WHERE + ORDER BY
// clauses in GetViewInit. a column-name typo in either clause (e.g.
// id_default instead of is_default) surfaces here as a sqlite error
func TestGetViewInit_FiltersExpiredCardsAndOrders(t *testing.T) {
	testDB, user := setupCardsExpireTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add((24 * time.Hour))

	expired := models.Card{
		UserID: user.ID, StripePaymentID: "pm_expired", Last4: "1111",
		Brand: "Visa", ExpMonth: 1, ExpYear: 2030, StorageExpiresAt: &past,
	}
	freshDefault := models.Card{
		UserID: user.ID, StripePaymentID: "pm_fresh_d", Last4: "2222",
		Brand: "Visa", ExpMonth: 1, ExpYear: 2030, StorageExpiresAt: &future,
		IsDefault: true,
	}
	freshOther := models.Card{
		UserID: user.ID, StripePaymentID: "pm_fresh_o", Last4: "3333",
		Brand: "Visa", ExpMonth: 1, ExpYear: 2030, StorageExpiresAt: &future,
	}
	for _, card := range []*models.Card{&expired, &freshDefault, &freshOther} {
		if err := db.Create(card).Error; err != nil {
			t.Fatalf("seed card: %v", err)
		}
	}

	handler := NewViewHandler()
	req := authRequest("GET", "/api/v1/view/init", nil, user.ID)
	w := httptest.NewRecorder()
	handler.GetViewInit(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GetViewInit status %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Cards []models.Card `json:"cards"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data.Cards) != 2 {
		t.Fatalf("want 2 visible cards, got %d", len(resp.Data.Cards))
	}
	if resp.Data.Cards[0].ID != freshDefault.ID {
		t.Fatalf("default card should sort first, got id=%d (want %d)", resp.Data.Cards[0].ID, freshDefault.ID)
	}
}
