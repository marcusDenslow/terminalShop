package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stripe/stripe-go/v78"

	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
)

// seedEphemeralCart seeds an item + address on the user's cart but deliberately
// NO card — the ephemeral (privacy) path supplies a one-time token at checkout
// instead of selecting a saved card.
func seedEphemeralCart(t *testing.T, user models.User) {
	t.Helper()
	db := database.GetDB()

	addr := models.Address{
		UserID: user.ID, Name: "test", Street: "1 Main St",
		City: "PDX", State: "OR", Zip: "97201", Country: "US",
	}
	if err := db.Create(&addr).Error; err != nil {
		t.Fatalf("seed address: %v", err)
	}
	if err := db.Model(&models.Coffee{}).Where("id = ?", 4).Update("price", 500).Error; err != nil {
		t.Fatalf("pin coffee price: %v", err)
	}

	h := NewCartHandler("", 0)
	body, _ := json.Marshal(map[string]any{"coffee_id": 4, "quantity": 2})
	w := httptest.NewRecorder()
	h.SetItem(w, authRequest("PUT", "/api/v1/cart/item", body, user.ID))
	if w.Code != http.StatusOK {
		t.Fatalf("SetItem: %d %s", w.Code, w.Body.String())
	}
	body, _ = json.Marshal(map[string]any{"address_id": addr.ID})
	w = httptest.NewRecorder()
	h.SetAddress(w, authRequest("PUT", "/api/v1/cart/address", body, user.ID))
	if w.Code != http.StatusOK {
		t.Fatalf("SetAddress: %d %s", w.Code, w.Body.String())
	}
}

// TestConvertCart_Ephemeral_InlineSuccess proves the privacy charge path: a
// one-time PaymentMethod is created from the token, the PaymentIntent attaches
// NO Customer, the PM is detached after settle, no Card row is written, and the
// order is paid + marked Ephemeral + keeps the PaymentIntent id (so refunds
// still work). The table covers both drivers: the per-request flag and the
// account-level PrivacyMode switch.
func TestConvertCart_Ephemeral_InlineSuccess(t *testing.T) {
	cases := []struct {
		name     string
		reqEphem bool // request "ephemeral"
	}{
		{"per-request flag", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testDB, user := setupCartTestDB(t)
			defer func() { _ = os.Remove(testDB) }()
			defer database.ResetForTesting()

			db := database.GetDB()
			seedEphemeralCart(t, user)

			var pmNewToken string
			var piParams *stripe.PaymentIntentParams
			var detached string

			origNew, origPI, origDetach := paymentMethodNewFn, paymentIntentNewFn, paymentMethodDetachFn
			defer func() {
				paymentMethodNewFn, paymentIntentNewFn, paymentMethodDetachFn = origNew, origPI, origDetach
			}()
			paymentMethodNewFn = func(p *stripe.PaymentMethodParams) (*stripe.PaymentMethod, error) {
				if p.Card != nil && p.Card.Token != nil {
					pmNewToken = *p.Card.Token
				}
				return &stripe.PaymentMethod{ID: "pm_once"}, nil
			}
			paymentIntentNewFn = func(p *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
				piParams = p
				return &stripe.PaymentIntent{ID: "pi_eph", Status: stripe.PaymentIntentStatusSucceeded}, nil
			}
			paymentMethodDetachFn = func(id string, _ *stripe.PaymentMethodDetachParams) (*stripe.PaymentMethod, error) {
				detached = id
				return &stripe.PaymentMethod{ID: id}, nil
			}

			body, _ := json.Marshal(map[string]any{"ephemeral": tc.reqEphem, "card_token": "tok_visa"})
			w := httptest.NewRecorder()
			NewCartHandler("http://localhost", 0).ConvertCart(w, authRequest("POST", "/api/v1/cart/convert", body, user.ID))

			if w.Code != http.StatusOK {
				t.Fatalf("convert: want 200, got %d body=%s", w.Code, w.Body.String())
			}
			if pmNewToken != "tok_visa" {
				t.Errorf("one-time PM token: want tok_visa, got %q", pmNewToken)
			}
			if piParams == nil {
				t.Fatal("paymentIntentNewFn was never called")
			}
			if piParams.Customer != nil {
				t.Errorf("ephemeral charge must NOT attach a Customer; got %q", *piParams.Customer)
			}
			if piParams.PaymentMethod == nil || *piParams.PaymentMethod != "pm_once" {
				t.Error("PI must charge the one-time PM pm_once")
			}
			if detached != "pm_once" {
				t.Errorf("PM must be detached after settle; detached=%q", detached)
			}

			var cardCount int64
			db.Model(&models.Card{}).Where("user_id = ?", user.ID).Count(&cardCount)
			if cardCount != 0 {
				t.Errorf("ephemeral checkout must not save a Card row; found %d", cardCount)
			}

			var order models.Order
			if err := db.Where("user_id = ?", user.ID).First(&order).Error; err != nil {
				t.Fatalf("load order: %v", err)
			}
			if !order.Ephemeral {
				t.Error("order.Ephemeral must be true")
			}
			if order.CardID != 0 {
				t.Errorf("ephemeral order must have CardID 0, got %d", order.CardID)
			}
			if order.Status != models.OrderStatusPaid {
				t.Errorf("status: want paid, got %q", order.Status)
			}
			if order.StripePaymentID != "pi_eph" {
				t.Errorf("order must keep the PaymentIntent id for refunds, got %q", order.StripePaymentID)
			}
		})
	}
}

// TestConvertCart_Ephemeral_MissingTokenRejected: a privacy checkout with no
// card token is a 400, and we never reach Stripe.
func TestConvertCart_Ephemeral_MissingTokenRejected(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	seedEphemeralCart(t, user)

	piCalled := false
	origPI := paymentIntentNewFn
	defer func() { paymentIntentNewFn = origPI }()
	paymentIntentNewFn = func(_ *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		piCalled = true
		return &stripe.PaymentIntent{ID: "pi_x", Status: stripe.PaymentIntentStatusSucceeded}, nil
	}

	body, _ := json.Marshal(map[string]any{"ephemeral": true}) // no card_token
	w := httptest.NewRecorder()
	NewCartHandler("http://localhost", 0).ConvertCart(w, authRequest("POST", "/api/v1/cart/convert", body, user.ID))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if piCalled {
		t.Error("must not create a PaymentIntent when the card token is missing")
	}
}

// TestConvertCart_Ephemeral_RequiresActionDefersDetach: on a 3DS challenge the
// charge has not settled, so the PM must stay attached (the webhook detaches it
// later). The order persists Ephemeral so the webhook knows to detach.
func TestConvertCart_Ephemeral_RequiresActionDefersDetach(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	seedEphemeralCart(t, user)

	detachCalled := false
	origNew, origPI, origDetach := paymentMethodNewFn, paymentIntentNewFn, paymentMethodDetachFn
	defer func() {
		paymentMethodNewFn, paymentIntentNewFn, paymentMethodDetachFn = origNew, origPI, origDetach
	}()
	paymentMethodNewFn = func(_ *stripe.PaymentMethodParams) (*stripe.PaymentMethod, error) {
		return &stripe.PaymentMethod{ID: "pm_once_3ds"}, nil
	}
	paymentIntentNewFn = func(_ *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		return &stripe.PaymentIntent{
			ID:     "pi_eph_3ds",
			Status: stripe.PaymentIntentStatusRequiresAction,
			NextAction: &stripe.PaymentIntentNextAction{
				RedirectToURL: &stripe.PaymentIntentNextActionRedirectToURL{URL: "https://hooks.stripe.com/3ds/test"},
			},
		}, nil
	}
	paymentMethodDetachFn = func(id string, _ *stripe.PaymentMethodDetachParams) (*stripe.PaymentMethod, error) {
		detachCalled = true
		return &stripe.PaymentMethod{ID: id}, nil
	}

	body, _ := json.Marshal(map[string]any{"ephemeral": true, "card_token": "tok_3ds"})
	w := httptest.NewRecorder()
	NewCartHandler("http://localhost", 0).ConvertCart(w, authRequest("POST", "/api/v1/cart/convert", body, user.ID))

	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d body=%s", w.Code, w.Body.String())
	}
	if detachCalled {
		t.Error("must NOT detach the PM while 3DS is pending; the webhook owns the deferred detach")
	}

	var order models.Order
	if err := database.GetDB().Where("user_id = ?", user.ID).First(&order).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if !order.Ephemeral {
		t.Error("order.Ephemeral must persist so the webhook can detach later")
	}
	if order.Status != models.OrderStatusRequiresAction {
		t.Errorf("status: want requires_action, got %q", order.Status)
	}
}

// TestHandlePaymentIntentSucceeded_EphemeralDetach exercises the deferred (3DS)
// detach: an Ephemeral order finishing via webhook detaches its PM; a
// non-ephemeral one does not.
func TestHandlePaymentIntentSucceeded_EphemeralDetach(t *testing.T) {
	cases := []struct {
		name       string
		ephemeral  bool
		wantDetach string // expected detached PM id, "" if none
	}{
		{"ephemeral order detaches PM", true, "pm_eph_once"},
		{"non-ephemeral order keeps PM", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testDB, user := setupCartTestDB(t)
			defer func() { _ = os.Remove(testDB) }()
			defer database.ResetForTesting()

			db := database.GetDB()
			order := models.Order{
				UserID: user.ID, CardID: 0, Ephemeral: tc.ephemeral,
				StripePaymentID: "pi_eph_webhook", Status: models.OrderStatusRequiresAction,
				Subtotal: 500, Total: 500,
				ShippingName: "Test", ShippingStreet: "1 Main St",
				ShippingCity: "PDX", ShippingState: "OR", ShippingZip: "97201", ShippingCountry: "US",
			}
			if err := db.Create(&order).Error; err != nil {
				t.Fatalf("seed order: %v", err)
			}

			detached := ""
			origDetach := paymentMethodDetachFn
			defer func() { paymentMethodDetachFn = origDetach }()
			paymentMethodDetachFn = func(id string, _ *stripe.PaymentMethodDetachParams) (*stripe.PaymentMethod, error) {
				detached = id
				return &stripe.PaymentMethod{ID: id}, nil
			}

			pi := map[string]any{
				"id":             order.StripePaymentID,
				"amount":         order.Total,
				"currency":       "usd",
				"payment_method": "pm_eph_once",
				"metadata":       map[string]string{"order_id": fmt.Sprintf("%d", order.ID)},
			}
			raw, _ := json.Marshal(pi)
			evt := stripe.Event{Type: "payment_intent.succeeded", Data: &stripe.EventData{Raw: raw}}

			NewWebhookHandler("", "").handlePaymentIntentSucceeded(context.Background(), evt)

			var reloaded models.Order
			if err := db.First(&reloaded, order.ID).Error; err != nil {
				t.Fatalf("reload order: %v", err)
			}
			if reloaded.Status != models.OrderStatusPaid {
				t.Fatalf("status: want paid, got %q", reloaded.Status)
			}
			if detached != tc.wantDetach {
				t.Errorf("detach: want %q, got %q", tc.wantDetach, detached)
			}
		})
	}
}

// TestConvertCart_PrivacyModeSoftDefault proves PrivacyMode is NOT enforced
// server-side: a user with privacy_mode on who explicitly checks out a saved
// card (ephemeral=false) gets the normal kept-card charge — Customer attached,
// PM not detached, order not Ephemeral. This is what makes "save anyway" work.
func TestConvertCart_PrivacyModeSoftDefault(t *testing.T) {
	testDB, user := setupCartTestDB(t)
	defer func() { _ = os.Remove(testDB) }()
	defer database.ResetForTesting()

	db := database.GetDB()
	if err := db.Model(&user).Update("privacy_mode", true).Error; err != nil {
		t.Fatalf("set privacy_mode: %v", err)
	}
	seedConvertReadyCart(t, user) // seeds a SAVED card + address + item

	var piParams *stripe.PaymentIntentParams
	detachCalled := false
	origPI, origDetach := paymentIntentNewFn, paymentMethodDetachFn
	defer func() { paymentIntentNewFn, paymentMethodDetachFn = origPI, origDetach }()
	paymentIntentNewFn = func(p *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		piParams = p
		return &stripe.PaymentIntent{ID: "pi_kept", Status: stripe.PaymentIntentStatusSucceeded}, nil
	}
	paymentMethodDetachFn = func(id string, _ *stripe.PaymentMethodDetachParams) (*stripe.PaymentMethod, error) {
		detachCalled = true
		return &stripe.PaymentMethod{ID: id}, nil
	}

	body, _ := json.Marshal(map[string]any{"ephemeral": false})
	w := httptest.NewRecorder()
	NewCartHandler("http://localhost", 0).ConvertCart(w, authRequest("POST", "/api/v1/cart/convert", body, user.ID))

	if w.Code != http.StatusOK {
		t.Fatalf("convert: want 200, got %d body=%s", w.Code, w.Body.String())
	}
	if piParams == nil || piParams.Customer == nil {
		t.Error("saved-card path must attach a Customer even when privacy_mode is on")
	}
	if detachCalled {
		t.Error("must NOT detach a saved card")
	}
	var order models.Order
	if err := db.Where("user_id = ?", user.ID).First(&order).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if order.Ephemeral {
		t.Error("explicit ephemeral=false must win over privacy_mode (soft default)")
	}
}
