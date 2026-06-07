package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"terminalShop/pkg/database"
	"terminalShop/pkg/models"

	"github.com/go-chi/chi/v5"
)

// TestGetCards_FiltersExpiredRows drives the SQL WHERE claude used by
// GetCards. a column-name typo (e.g used_id instead of user_id) surfaces
// here as a sqlite error, not in prod
func TestGetCards_FiltersExpiredRows(t *testing.T) {
	testDB, user := setupCardsExpireTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(24 * time.Hour)

	expired := models.Card{
		UserID: user.ID, StripePaymentID: "pm_expired", Last4: "1111",
		Brand: "Visa", ExpMonth: 1, ExpYear: 2030, StorageExpiresAt: &past,
	}
	fresh := models.Card{
		UserID: user.ID, StripePaymentID: "pm_fresh", Last4: "2222",
		Brand: "Visa", ExpMonth: 1, ExpYear: 2030, StorageExpiresAt: &future,
	}
	legacy := models.Card{
		UserID: user.ID, StripePaymentID: "pm_legacy", Last4: "3333",
		Brand: "Visa", ExpMonth: 1, ExpYear: 2030,
	}
	for _, card := range []*models.Card{&expired, &fresh, &legacy} {
		if err := db.Create(card).Error; err != nil {
			t.Fatalf("seed card: %v", err)
		}
	}

	handler := NewCardHandler("")
	req := authRequest("GET", "/api/v1/cards", nil, user.ID)
	w := httptest.NewRecorder()
	handler.GetCards(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GetCards status %d, body %s", w.Code, w.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Cards []models.Card `json:"cards"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data.Cards) != 2 {
		t.Fatalf("want 2 visible cards (fresh + legacy NULL), got %d", len(resp.Data.Cards))
	}
	for _, card := range resp.Data.Cards {
		if card.ID == expired.ID {
			t.Fatalf("expired card leaked into GetCards response")
		}
	}
}

// TestSetDefaultCard_ExpiredReturns410 verifies the sync-expire branch when
// the user acts on a card whose retention deadline already elapsed
func TestSetDefaultCard_ExpiredReturns410(t *testing.T) {
	testDB, user := setupCardsExpireTestDB(t)
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

	handler := NewCardHandler("")
	idStr := strconv.FormatUint(uint64(card.ID), 10)
	req := authRequest("PUT", "/api/v1/cards/"+idStr+"/default", nil, user.ID)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", idStr)
	req = req.WithContext(contextWithRoute(req.Context(), routeCtx))

	w := httptest.NewRecorder()
	handler.SetDefaultCard(w, req)

	if w.Code != http.StatusGone {
		t.Fatalf("want 410 gone, got %d, body %s", w.Code, w.Body.String())
	}
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error.Code != "CARD_STORAGE_EXPIRED" {
		t.Fatalf("want CARD_STORAGE_EXPIRED code, got %q", resp.Error.Code)
	}

	var soft models.Card
	if err := db.Unscoped().First(&soft, card.ID).Error; err != nil {
		t.Fatalf("reload card unscoped: %v", err)
	}
	if !soft.DeletedAt.Valid {
		t.Fatalf("expired card was not soft-deleted")
	}
}

// TestRefreshCardStorageTTL_BumpLastUsedAndExpiry exercises the SQL Updates
// map used by both the payment_intent.succeede webhook and the duplicate-card
// add path. Column-name typos in the map surface here
func TestRefreshCardStorageTTL_BumpLastUsedAndExpiry(t *testing.T) {
	testDB, user := setupCardsExpireTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	old := time.Now().Add(-30 * 24 * time.Hour)
	card := models.Card{
		UserID: user.ID, StripePaymentID: "pm_refresh", Last4: "4242",
		Brand: "Visa", ExpMonth: 1, ExpYear: 2030, StorageExpiresAt: &old,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("seed card: %v", err)
	}

	now := time.Now()
	if err := refreshCardStorageTTL(db, card.ID, now); err != nil {
		t.Fatalf("refreshCardStorageTTL: %v", err)
	}

	var reloaded models.Card
	if err := db.First(&reloaded, card.ID).Error; err != nil {
		t.Fatalf("reload card: %v", err)
	}
	if reloaded.LastUsedAt == nil || !reloaded.StorageExpiresAt.After(now.Add(179*24*time.Hour)) {
		t.Fatalf("storage_expires_at not extended, got %v", reloaded.StorageExpiresAt)
	}
}
