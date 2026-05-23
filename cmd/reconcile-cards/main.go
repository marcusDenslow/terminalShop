package main

import (
	"flag"
	"log"
	"terminalShop/pkg/config"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/paymentmethod"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "list orphans without detaching")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if _, err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatalf("db: %v", err)
	}
	db := database.GetDB()
	stripe.Key = cfg.StripeSecretKey

	var users []models.User
	if err := db.Where("stripe_customer_id != ''").Find(&users).Error; err != nil {
		log.Fatalf("list users: %v", err)
	}

	totalOrphans, totalDetached := 0, 0
	for _, user := range users {
		var activeCards []models.Card
		if err := db.Where("user_id = ?", user.ID).Find(&activeCards).Error; err != nil {
			log.Printf("user %d: list cards: %v", user.ID, err)
			continue
		}
		keep := make(map[string]bool, len(activeCards))
		for _, card := range activeCards {
			keep[card.StripePaymentID] = true
		}
		iter := paymentmethod.List(&stripe.PaymentMethodListParams{
			Customer: stripe.String(user.StripeCustomerID),
			Type:     stripe.String("card"),
		})
		for iter.Next() {
			pm := iter.PaymentMethod()
			if keep[pm.ID] {
				continue
			}
			totalOrphans++
			last4 := ""
			if pm.Card != nil {
				last4 = pm.Card.Last4
			}
			log.Printf("orphan: user=%d cus=%s pm=%s last4=%s", user.ID, user.StripeCustomerID, pm.ID, last4)
			if *dryRun {
				continue
			}
			if _, err := paymentmethod.Detach(pm.ID, nil); err != nil {
				log.Printf("  detach failed: %v", err)
				continue
			}
			totalDetached++
		}
		if err := iter.Err(); err != nil {
			log.Printf("user %d: list pms: %v", user.ID, err)
		}
	}
	log.Printf("done. orphans=%d detached=%d dry_run=%v", totalOrphans, totalDetached, *dryRun)
}
