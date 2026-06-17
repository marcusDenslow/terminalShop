package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"terminalShop/api/middleware"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
)

func TestSetSpendLimit(t *testing.T) {
	const globalCap = 5000
	cases := []struct {
		name       string
		seedSelf   *int // starting SelfLimitCents on the row
		seedAdmin  *int // per-user MaxOrderCents override (effective admin cap)
		body       string
		wantStatus int
		wantCode   string // error code; "" means success
		wantLimit  *int   // expected SelfLimitCents after a successful update
	}{
		{"set positive under cap", nil, nil, `{"self_limit_cents":2000}`, http.StatusOK, "", intPtr(2000)},
		{"set zero blocks all", nil, nil, `{"self_limit_cents":0}`, http.StatusOK, "", intPtr(0)},
		{"set equal to cap", nil, nil, `{"self_limit_cents":5000}`, http.StatusOK, "", intPtr(5000)},
		{"clear with null", intPtr(2000), nil, `{"self_limit_cents":null}`, http.StatusOK, "", nil},
		{"overwrite existing", intPtr(2000), nil, `{"self_limit_cents":1000}`, http.StatusOK, "", intPtr(1000)},
		{"above global cap rejected", nil, nil, `{"self_limit_cents":6000}`, http.StatusBadRequest, "SELF_LIMIT_ABOVE_CAP", nil},
		{"under per-user override accepted", nil, intPtr(10000), `{"self_limit_cents":8000}`, http.StatusOK, "", intPtr(8000)},
		{"above per-user override rejected", nil, intPtr(10000), `{"self_limit_cents":12000}`, http.StatusBadRequest, "SELF_LIMIT_ABOVE_CAP", nil},
		{"negative rejected", nil, nil, `{"self_limit_cents":-1}`, http.StatusBadRequest, "INVALID_LIMIT", nil},
		{"missing field rejected", nil, nil, `{}`, http.StatusBadRequest, "MISSING_FIELD", nil},
		{"non-integer rejected", nil, nil, `{"self_limit_cents":"lots"}`, http.StatusBadRequest, "INVALID_BODY", nil},
		{"malformed json rejected", nil, nil, `{`, http.StatusBadRequest, "INVALID_BODY", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user := setupOrderTestDB(t)
			db := database.GetDB()
			if tc.seedSelf != nil {
				if err := db.Model(&user).Update("self_limit_cents", *tc.seedSelf).Error; err != nil {
					t.Fatalf("seed self limit: %v", err)
				}
			}
			if tc.seedAdmin != nil {
				if err := db.Model(&user).Update("max_order_cents", *tc.seedAdmin).Error; err != nil {
					t.Fatalf("seed admin cap: %v", err)
				}
			}

			handler := NewAccountHandler(globalCap)
			req := authRequest("PUT", "/api/v1/account/spend-limit", []byte(tc.body), user.ID)
			w := httptest.NewRecorder()
			handler.SetSpendLimit(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status: want %d got %d (%s)", tc.wantStatus, w.Code, w.Body.String())
			}
			assertErrCode(t, w, tc.wantCode)

			var got models.User
			if err := db.First(&got, user.ID).Error; err != nil {
				t.Fatalf("reload user: %v", err)
			}
			if tc.wantStatus == http.StatusOK {
				if !eqIntPtr(got.SelfLimitCents, tc.wantLimit) {
					t.Errorf("db SelfLimitCents: want %s got %s", fmtIntPtr(tc.wantLimit), fmtIntPtr(got.SelfLimitCents))
				}
			} else if !eqIntPtr(got.SelfLimitCents, tc.seedSelf) {
				// A rejected request must not mutate the column.
				t.Errorf("rejected request changed db limit: want %s got %s", fmtIntPtr(tc.seedSelf), fmtIntPtr(got.SelfLimitCents))
			}
		})
	}
}

// TestSetSpendLimit_CapDisabledAllowsAnyLimit: with no active admin ceiling
// (global 0, no override) any non-negative self-limit is accepted — a self-limit
// can only ADD a ceiling, so there is nothing to raise.
func TestSetSpendLimit_CapDisabledAllowsAnyLimit(t *testing.T) {
	user := setupOrderTestDB(t)
	handler := NewAccountHandler(0) // global cap disabled
	req := authRequest("PUT", "/api/v1/account/spend-limit", []byte(`{"self_limit_cents":999999}`), user.ID)
	w := httptest.NewRecorder()
	handler.SetSpendLimit(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: want 200 got %d (%s)", w.Code, w.Body.String())
	}
	var got models.User
	if err := database.GetDB().First(&got, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if !eqIntPtr(got.SelfLimitCents, intPtr(999999)) {
		t.Errorf("db SelfLimitCents: want 999999 got %s", fmtIntPtr(got.SelfLimitCents))
	}
}

// TestSetSpendLimit_RequireAuthGate exercises the JWT gate through a real
// router: no user context => 401, the handler is never reached.
func TestSetSpendLimit_RequireAuthGate(t *testing.T) {
	setupOrderTestDB(t)
	router := chi.NewRouter()
	router.With(middleware.RequireAuth).
		Put("/api/v1/account/spend-limit", NewAccountHandler(5000).SetSpendLimit)

	req := httptest.NewRequest("PUT", "/api/v1/account/spend-limit", strings.NewReader(`{"self_limit_cents":1000}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: want 401 got %d", w.Code)
	}
	assertErrCode(t, w, "UNAUTHORIZED")
}
