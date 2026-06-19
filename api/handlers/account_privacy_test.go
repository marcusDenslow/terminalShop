package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
)

// TestSetPrivacyMode covers the account privacy toggle: enable, disable (a real
// true->false transition, which the map-form Updates must persist), and the two
// 400 paths (missing field, wrong type).
func TestSetPrivacyMode(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantValue  bool
	}{
		{"enable", `{"privacy_mode": true}`, http.StatusOK, true},
		{"disable", `{"privacy_mode": false}`, http.StatusOK, false},
		{"missing field", `{}`, http.StatusBadRequest, false},
		{"wrong type", `{"privacy_mode": "yes"}`, http.StatusBadRequest, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testDB, user := setupCartTestDB(t)
			defer func() { _ = os.Remove(testDB) }()
			defer database.ResetForTesting()

			// Seed the opposite so an OK case proves a real transition (and the
			// disable case actually exercises persisting an explicit false).
			if tc.wantStatus == http.StatusOK {
				if err := database.GetDB().Model(&user).Update("privacy_mode", !tc.wantValue).Error; err != nil {
					t.Fatalf("seed privacy_mode: %v", err)
				}
			}

			w := httptest.NewRecorder()
			NewAccountHandler(0).SetPrivacyMode(
				w, authRequest("PUT", "/api/v1/account/privacy-mode", []byte(tc.body), user.ID))

			if w.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d body=%s", tc.wantStatus, w.Code, w.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				return
			}
			var reloaded models.User
			if err := database.GetDB().First(&reloaded, user.ID).Error; err != nil {
				t.Fatalf("reload user: %v", err)
			}
			if reloaded.PrivacyMode != tc.wantValue {
				t.Errorf("privacy_mode: want %v, got %v", tc.wantValue, reloaded.PrivacyMode)
			}
		})
	}
}
