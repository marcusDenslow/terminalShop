package database

import (
	"fmt"
	"log"

	"terminalShop/pkg/models"

	"gorm.io/gorm"
)

// Seed populates the database with initial data
func Seed(db *gorm.DB) error {
	log.Println("Seeding database...")

	// Check if products already exist
	var count int64
	db.Model(&models.Coffee{}).Count(&count)
	if count > 0 {
		log.Printf("Database already has %d products, skipping seed", count)
		return nil
	}

	// Define seed products (from handlers/products.go)
	products := []models.Coffee{
		{
			Name:        "Espresso",
			RoastType:   "Dark Roast",
			Ounces:      2,
			BeanType:    "Arabica",
			Price:       1800, // $18.00
			Color:       "#8B4513",
			Description: "Bold and intense coffee with a rich crema",
		},
		{
			Name:        "Americano",
			RoastType:   "Medium Roast",
			Ounces:      8,
			BeanType:    "Arabica",
			Price:       2200, // $22.00
			Color:       "#A0522D",
			Description: "Smooth and balanced espresso diluted with hot water",
		},
		{
			Name:        "Cappuccino",
			RoastType:   "Medium Roast",
			Ounces:      6,
			BeanType:    "Arabica",
			Price:       2200, // $22.00
			Color:       "#C4A574",
			Description: "Espresso with steamed milk and a thick layer of foam",
		},
		{
			Name:        "Latte",
			RoastType:   "Medium Roast",
			Ounces:      12,
			BeanType:    "Arabica",
			Price:       2500, // $25.00
			Color:       "#D2B48C",
			Description: "Creamy espresso with plenty of steamed milk and a touch of foam",
		},
		{
			Name:        "Mocha",
			RoastType:   "Dark Roast",
			Ounces:      12,
			BeanType:    "Arabica",
			Price:       3000, // $30.00
			Color:       "#8B4513",
			Description: "Chocolate and espresso combined with steamed milk. Sweet and indulgent.",
		},
		{
			Name:        "Macchiato",
			RoastType:   "Dark Roast",
			Ounces:      3,
			BeanType:    "Robusta Blend",
			Price:       1800, // $18.00
			Color:       "#DAA520",
			Description: "Espresso 'marked' with a dollop of foamed milk. Small but mighty, like a tiny caffeinated warrior.",
		},
	}

	// Insert all products in a transaction
	if err := db.Create(&products).Error; err != nil {
		return fmt.Errorf("failed to seed products: %w", err)
	}

	log.Printf("Successfully seeded %d products", len(products))
	return nil
}
