package tui

import (
	"terminalShop/pkg/api"
	"terminalShop/pkg/models"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

func TestModel_BuildMenuView(t *testing.T) {
	type fields struct {
		User            *models.User
		IsNewUser       bool
		SSHPublicKey    gossh.PublicKey
		AccessToken     string
		UsernameInput   string
		Username        string
		Coffees         []models.Coffee
		Cursor          int
		Cart            map[int]*models.CartItem
		CartCursor      int
		AccountCursor   int
		CheckoutStep    int
		ScrollOffset    int
		ViewingCart     bool
		ViewingAccount  bool
		viewportWidth   int
		viewportHeight  int
		widthContainer  int
		heightContainer int
		widthContent    int
		size            termSize
		resizeSeq       int
		pendingWidth    int
		pendingHeight   int
		Loading         bool
		ErrorMsg        string
		APIClient       *api.Client
		ShippingForm    *ShippingFormState
		PaymentForm     *PaymentFormState
		ShippingInfo    *models.Address
		SavedAddresses  []models.Address
		ShippingView    int
		AddressCursor   int
		StripeKey       string
		Orders          []models.Order
		OrdersLoaded    bool
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				User:            tt.fields.User,
				IsNewUser:       tt.fields.IsNewUser,
				SSHPublicKey:    tt.fields.SSHPublicKey,
				AccessToken:     tt.fields.AccessToken,
				UsernameInput:   tt.fields.UsernameInput,
				Username:        tt.fields.Username,
				Coffees:         tt.fields.Coffees,
				Cursor:          tt.fields.Cursor,
				Cart:            tt.fields.Cart,
				CartCursor:      tt.fields.CartCursor,
				AccountCursor:   tt.fields.AccountCursor,
				CheckoutStep:    tt.fields.CheckoutStep,
				ScrollOffset:    tt.fields.ScrollOffset,
				ViewingCart:     tt.fields.ViewingCart,
				ViewingAccount:  tt.fields.ViewingAccount,
				viewportWidth:   tt.fields.viewportWidth,
				viewportHeight:  tt.fields.viewportHeight,
				widthContainer:  tt.fields.widthContainer,
				heightContainer: tt.fields.heightContainer,
				widthContent:    tt.fields.widthContent,
				size:            tt.fields.size,
				resizeSeq:       tt.fields.resizeSeq,
				pendingWidth:    tt.fields.pendingWidth,
				pendingHeight:   tt.fields.pendingHeight,
				Loading:         tt.fields.Loading,
				ErrorMsg:        tt.fields.ErrorMsg,
				APIClient:       tt.fields.APIClient,
				ShippingForm:    tt.fields.ShippingForm,
				PaymentForm:     tt.fields.PaymentForm,
				ShippingInfo:    tt.fields.ShippingInfo,
				SavedAddresses:  tt.fields.SavedAddresses,
				ShippingView:    tt.fields.ShippingView,
				AddressCursor:   tt.fields.AddressCursor,
				StripeKey:       tt.fields.StripeKey,
				Orders:          tt.fields.Orders,
				OrdersLoaded:    tt.fields.OrdersLoaded,
			}
			if got := m.BuildMenuView(); got != tt.want {
				t.Errorf("Model.BuildMenuView() = %v, want %v", got, tt.want)
			}
		})
	}
}
