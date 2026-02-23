//go:build ignore

package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/paymentintent"
	"github.com/stripe/stripe-go/v78/paymentmethod"
)

func main() {
	godotenv.Load()

	secKey := os.Getenv("STRIPE_SECRET_KEY")
	if secKey == "" {
		fmt.Println("FAILED: missing STRIPE_SECRET_KEY in .env")
		return
	}

	stripe.Key = secKey

	// Step 1: Create a PaymentMethod using Stripe's test token (tok_visa = Visa 4242)
	pm, err := paymentmethod.New(&stripe.PaymentMethodParams{
		Type: stripe.String("card"),
		Card: &stripe.PaymentMethodCardParams{
			Token: stripe.String("tok_visa"),
		},
	})
	if err != nil {
		fmt.Printf("FAILED payment method: %v\n", err)
		return
	}
	fmt.Printf("1. PaymentMethod created: %s (brand: %s, last4: %s)\n", pm.ID, pm.Card.Brand, pm.Card.Last4)

	// Step 2: Create a PaymentIntent and confirm it immediately
	pi, err := paymentintent.New(&stripe.PaymentIntentParams{
		Amount:        stripe.Int64(999999), // $10.00 in cents
		Currency:      stripe.String("usd"),
		PaymentMethod: stripe.String(pm.ID),
		Confirm:       stripe.Bool(true),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled:        stripe.Bool(true),
			AllowRedirects: stripe.String("never"),
		},
	})
	if err != nil {
		fmt.Printf("FAILED payment intent: %v\n", err)
		return
	}
	fmt.Printf("2. PaymentIntent created: %s (status: %s, amount: $%.2f)\n", pi.ID, pi.Status, float64(pi.Amount)/100)
	fmt.Println("\nSUCCESS! Stripe is fully working.")
}
