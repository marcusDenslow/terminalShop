package models

import (
	"testing"
	"time"
)

func TestCardStorageTTL(t *testing.T) {
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)

	var card Card
	card.InitializeStorageTTL(now)

	if card.LastUsedAt != nil {
		t.Fatalf("InitializeStorageTTL should not mark card as used")
	}
	if card.StorageExpiresAt == nil || !card.StorageExpiresAt.Equal(now.Add(cardStorageTTL)) {
		t.Fatalf("unexpected initial expiration: %v", card.StorageExpiresAt)
	}
	if card.IsStorageExpired(now.Add(cardStorageTTL - time.Second)) {
		t.Fatalf("card expired before ttl elapsed")
	}
	if !card.IsStorageExpired(now.Add(cardStorageTTL)) {
		t.Fatalf("card should expire at ttl boundary")
	}

	usedAt := now.Add(24 * time.Hour)
	card.RefreshStorageTTL(usedAt)
	if card.LastUsedAt == nil || !card.LastUsedAt.Equal(usedAt) {
		t.Fatalf("last used to refreshed: %v", card.LastUsedAt)
	}
	if card.StorageExpiresAt == nil || !card.StorageExpiresAt.Equal(usedAt.Add(cardStorageTTL)) {
		t.Fatalf("expiration not refreshed: %v", card.StorageExpiresAt)
	}
}
