package tui

import (
	"errors"
	"strings"
	"testing"
)

func TestFriendlyCheckoutError_CardStorageExpired(t *testing.T) {
	got := friendlyCheckoutError(errors.New("CARD_STORAGE_EXPIRED: saved card epired, add it again"))
	if !strings.Contains(got, "saved card expired") {
		t.Fatalf("want saved-card-expired copy, got %q", got)
	}
	if strings.Contains(got, "card expired. Add a new card") {
		t.Fatalf("CARD_STORAGE_EXPIRED was matched by the CARD_EXPIRED branch: %q", got)
	}
}

func TestFriendlyCheckoutError_CardExpiredStillReachable(t *testing.T) {
	got := friendlyCheckoutError(errors.New("CARD_EXPIRED: card expired"))
	if !strings.Contains(got, "card expired. Add a new card") {
		t.Fatalf("want plain card-expired copy, got %q", got)
	}
}
