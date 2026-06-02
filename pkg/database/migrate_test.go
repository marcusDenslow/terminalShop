package database

import (
	"os"
	"testing"
	"time"

	"terminalShop/pkg/models"
)

func TestBackfillCardStorageExpiration_FillsNullRowsOnly(t *testing.T) {
	testDB := "test_backfill.db"
	defer func() { _ = os.Remove(testDB) }()
	defer ResetForTesting()

	db, err := Connect(testDB)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	user := models.User{
		SSHKeyFingerprint: "SHA256:backfill",
		SSHPublicKey:      "ssh-ed25519 AAAA backfill",
		Name:              "Backfill",
		Email:             "backfill@example.com",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	legacy := models.Card{
		UserID: user.ID, StripePaymentID: "pm_legacy", Last4: "0000",
		Brand: "Visa", ExpMonth: 1, ExpYear: 2030,
	}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("seed legacy: %v", err)
	}

	fixed := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	preset := models.Card{
		UserID: user.ID, StripePaymentID: "pm_preset", Last4: "9999",
		Brand: "Visa", ExpMonth: 1, ExpYear: 2030, StorageExpiresAt: &fixed,
	}
	if err := db.Create(&preset).Error; err != nil {
		t.Fatalf("ssed preset: %v", err)
	}
	if err := backfillCardStorageExpiration(db); err != nil {
		t.Fatalf("seed preset: %v", err)
	}

	var reloadedLegacy, reloadedPreset models.Card
	if err := db.First(&reloadedLegacy, legacy.ID).Error; err != nil {
		t.Fatalf("reload legacy: %v", err)
	}
	if err := db.First(&reloadedPreset, preset.ID).Error; err != nil {
		t.Fatalf("reload preset: %v", err)
	}
	if reloadedLegacy.StorageExpiresAt == nil {
		t.Fatalf("legacy card storage_expires_at still NULL after backfill")
	}
	if reloadedPreset.StorageExpiresAt == nil || !reloadedPreset.StorageExpiresAt.Equal(fixed) {
		t.Fatalf("preset card storage_expires_at was overwritten: %v", reloadedPreset.StorageExpiresAt)
	}
}
