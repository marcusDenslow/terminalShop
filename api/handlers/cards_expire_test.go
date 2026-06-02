package handlers

import (
	"os"
	"testing"
	"time"

	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
)

// TestReconcileExpiredCards_RemovesPastDeadlineOnly drives the real SQL path
// so a column-name typo in the WHERE clause surfaces here, not in production.
func TestReconcileExpiredCards_RemovesPastDeadlineOnly(t *testing.T) {
	testDB, user := setupCardsExpireTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(24 * time.Hour)

	expired := models.Card{
		UserID:           user.ID,
		StripePaymentID:  "tok_expired",
		Last4:            "1111",
		Brand:            "Visa",
		ExpMonth:         1,
		ExpYear:          2030,
		StorageExpiresAt: &past,
	}
	fresh := models.Card{
		UserID:           user.ID,
		StripePaymentID:  "tok_fresh",
		Last4:            "2222",
		Brand:            "Visa",
		ExpMonth:         1,
		ExpYear:          2030,
		StorageExpiresAt: &future,
	}
	noTTL := models.Card{
		UserID:          user.ID,
		StripePaymentID: "tok_legacy",
		Last4:           "3333",
		Brand:           "Visa",
		ExpMonth:        1,
		ExpYear:         2030,
	}
	for _, c := range []*models.Card{&expired, &fresh, &noTTL} {
		if err := db.Create(c).Error; err != nil {
			t.Fatalf("seed card: %v", err)
		}
	}

	cart := models.Cart{UserID: user.ID, CardID: &expired.ID}
	if err := db.Create(&cart).Error; err != nil {
		t.Fatalf("seed cart: %v", err)
	}

	ReconcileExpiredCards("")

	var remaining []models.Card
	if err := db.Where("user_id = ?", user.ID).Find(&remaining).Error; err != nil {
		t.Fatalf("list cards: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("want 2 cards remaining, got %d", len(remaining))
	}
	for _, c := range remaining {
		if c.ID == expired.ID {
			t.Fatalf("expired card was not deleted")
		}
	}

	var refreshed models.Cart
	if err := db.First(&refreshed, cart.ID).Error; err != nil {
		t.Fatalf("reload cart: %v", err)
	}
	if refreshed.CardID != nil {
		t.Fatalf("cart still references expired card: %v", refreshed.CardID)
	}
}

// TestReconcileExpiredCards_SweepsAcrossUsers verifies the sweeper is not
// scoped to one user; it must collect every past-deadline row in one pass.
func TestReconcileExpiredCards_SweepsAcrossUsers(t *testing.T) {
	testDB, user := setupCardsExpireTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	past := time.Now().Add(-1 * time.Hour)

	other := models.User{
		SSHKeyFingerprint: "SHA256:other",
		SSHPublicKey:      "ssh-ed25519 AAAA other",
		Name:              "Other",
		Email:             "other@example.com",
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("seed other user: %v", err)
	}

	for _, uid := range []uint{user.ID, other.ID} {
		c := models.Card{
			UserID:           uid,
			StripePaymentID:  "tok_expired",
			Last4:            "9999",
			Brand:            "Visa",
			ExpMonth:         1,
			ExpYear:          2030,
			StorageExpiresAt: &past,
		}
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("seed card: %v", err)
		}
	}

	ReconcileExpiredCards("")

	var count int64
	if err := db.Model(&models.Card{}).Count(&count).Error; err != nil {
		t.Fatalf("count cards: %v", err)
	}
	if count != 0 {
		t.Fatalf("want 0 cards after sweep, got %d", count)
	}
}

func setupCardsExpireTestDB(t *testing.T) (string, models.User) {
	t.Helper()
	testDB := "test_cards_expire.db"
	_ = os.Remove(testDB)
	database.ResetForTesting()

	db, err := database.Connect(testDB)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	user := models.User{
		SSHKeyFingerprint: "SHA256:cards-expire",
		SSHPublicKey:      "ssh-ed25519 AAAA cardsexpire",
		Name:              "Test User",
		Email:             "cards-expire@example.com",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return testDB, user
}
