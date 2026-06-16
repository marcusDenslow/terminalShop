package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"terminalShop/api/middleware"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
)

// adminCapRequest builds a PATCH request carrying the {id} chi URL param, as the
// admin order-cap route would. No JWT — the route is gated by RequireAdmin.
func adminCapRequest(idParam, body string) *http.Request {
	req := httptest.NewRequest("PATCH", "/api/v1/users/"+idParam+"/order-cap", strings.NewReader(body))
	rc := chi.NewRouteContext()
	rc.URLParams.Add("id", idParam)
	return req.WithContext(contextWithRoute(req.Context(), rc))
}

func eqIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func fmtIntPtr(p *int) string {
	if p == nil {
		return "<nil>"
	}
	return strconv.Itoa(*p)
}

func TestSetUserOrderCap(t *testing.T) {
	cases := []struct {
		name       string
		seed       *int // starting MaxOrderCents on the user row
		body       string
		wantStatus int
		wantCode   string // error code; "" means success
		wantCap    *int   // expected DB value after a successful update
	}{
		{"set positive", nil, `{"max_order_cents":7500}`, http.StatusOK, "", intPtr(7500)},
		{"set zero off-switch", nil, `{"max_order_cents":0}`, http.StatusOK, "", intPtr(0)},
		{"clear with null reverts to global", intPtr(500), `{"max_order_cents":null}`, http.StatusOK, "", nil},
		{"overwrite existing value", intPtr(500), `{"max_order_cents":12000}`, http.StatusOK, "", intPtr(12000)},
		{"negative rejected", nil, `{"max_order_cents":-1}`, http.StatusBadRequest, "INVALID_CAP", nil},
		{"missing field rejected", nil, `{}`, http.StatusBadRequest, "MISSING_FIELD", nil},
		{"non-integer rejected", nil, `{"max_order_cents":"lots"}`, http.StatusBadRequest, "INVALID_BODY", nil},
		{"malformed json rejected", nil, `{`, http.StatusBadRequest, "INVALID_BODY", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user := setupOrderTestDB(t)
			db := database.GetDB()
			if tc.seed != nil {
				if err := db.Model(&user).Update("max_order_cents", *tc.seed).Error; err != nil {
					t.Fatalf("seed cap: %v", err)
				}
			}

			handler := NewAdminUserHandler()
			req := adminCapRequest(strconv.FormatUint(uint64(user.ID), 10), tc.body)
			w := httptest.NewRecorder()
			handler.SetUserOrderCap(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status: want %d got %d (%s)", tc.wantStatus, w.Code, w.Body.String())
			}

			var resp struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			if resp.Error.Code != tc.wantCode {
				t.Errorf("error code: want %q got %q", tc.wantCode, resp.Error.Code)
			}

			var got models.User
			if err := db.First(&got, user.ID).Error; err != nil {
				t.Fatalf("reload user: %v", err)
			}
			if tc.wantStatus == http.StatusOK {
				if !eqIntPtr(got.MaxOrderCents, tc.wantCap) {
					t.Errorf("db MaxOrderCents: want %s got %s", fmtIntPtr(tc.wantCap), fmtIntPtr(got.MaxOrderCents))
				}
			} else if !eqIntPtr(got.MaxOrderCents, tc.seed) {
				// A rejected request must not mutate the column.
				t.Errorf("rejected request changed db cap: want %s got %s", fmtIntPtr(tc.seed), fmtIntPtr(got.MaxOrderCents))
			}
		})
	}
}

func TestSetUserOrderCap_InvalidID(t *testing.T) {
	setupOrderTestDB(t)
	w := httptest.NewRecorder()
	NewAdminUserHandler().SetUserOrderCap(w, adminCapRequest("abc", `{"max_order_cents":100}`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400 got %d", w.Code)
	}
	assertErrCode(t, w, "INVALID_ID")
}

func TestSetUserOrderCap_UserNotFound(t *testing.T) {
	setupOrderTestDB(t)
	w := httptest.NewRecorder()
	NewAdminUserHandler().SetUserOrderCap(w, adminCapRequest("99999", `{"max_order_cents":100}`))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status: want 404 got %d", w.Code)
	}
	assertErrCode(t, w, "USER_NOT_FOUND")
}

// TestSetUserOrderCap_AdminGate exercises the RequireAdmin middleware through a
// real router (the handler-direct tests above bypass middleware).
func TestSetUserOrderCap_AdminGate(t *testing.T) {
	user := setupOrderTestDB(t)
	idStr := strconv.FormatUint(uint64(user.ID), 10)

	router := func() http.Handler {
		r := chi.NewRouter()
		r.With(middleware.RequireAdmin).
			Patch("/api/v1/users/{id}/order-cap", NewAdminUserHandler().SetUserOrderCap)
		return r
	}
	newReq := func() *http.Request {
		return httptest.NewRequest("PATCH", "/api/v1/users/"+idStr+"/order-cap",
			strings.NewReader(`{"max_order_cents":100}`))
	}

	t.Run("disabled when ADMIN_API_KEY unset", func(t *testing.T) {
		t.Setenv("ADMIN_API_KEY", "")
		w := httptest.NewRecorder()
		router().ServeHTTP(w, newReq())
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("status: want 503 got %d", w.Code)
		}
		assertErrCode(t, w, "ADMIN_DISABLED")
	})

	t.Run("rejects wrong key", func(t *testing.T) {
		t.Setenv("ADMIN_API_KEY", "secret")
		req := newReq()
		req.Header.Set("X-Admin-Key", "nope")
		w := httptest.NewRecorder()
		router().ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status: want 401 got %d", w.Code)
		}
		assertErrCode(t, w, "UNAUTHORIZED")
	})

	t.Run("accepts correct key", func(t *testing.T) {
		t.Setenv("ADMIN_API_KEY", "secret")
		req := newReq()
		req.Header.Set("X-Admin-Key", "secret")
		w := httptest.NewRecorder()
		router().ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status: want 200 got %d (%s)", w.Code, w.Body.String())
		}
		var got models.User
		if err := database.GetDB().First(&got, user.ID).Error; err != nil {
			t.Fatalf("reload user: %v", err)
		}
		if !eqIntPtr(got.MaxOrderCents, intPtr(100)) {
			t.Errorf("db MaxOrderCents: want 100 got %s", fmtIntPtr(got.MaxOrderCents))
		}
	})
}

func assertErrCode(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error.Code != want {
		t.Errorf("error code: want %q got %q", want, resp.Error.Code)
	}
}
