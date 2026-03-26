package database

import (
	"os"
	"testing"

	"terminalShop/pkg/models"
)

func TestConnect(t *testing.T) {
	// Use a temporary test database
	testDB := "test_terminalshop.db"
	defer os.Remove(testDB)
	defer ResetForTesting()

	db, err := Connect(testDB)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	if db == nil {
		t.Fatal("Database connection is nil")
	}

	// Verify we can execute queries
	var result int
	if err := db.Raw("SELECT 1").Scan(&result).Error; err != nil {
		t.Fatalf("Failed to execute query: %v", err)
	}

	if result != 1 {
		t.Errorf("Expected 1, got %d", result)
	}
}

func TestMigrate(t *testing.T) {
	testDB := "test_migrate.db"
	defer os.Remove(testDB)
	defer ResetForTesting()

	db, err := Connect(testDB)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify table exists
	var count int64
	db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='coffees'").Scan(&count)
	if count != 1 {
		t.Errorf("Expected coffees table to exist, got count: %d", count)
	}
}

func TestSeed(t *testing.T) {
	testDB := "test_seed.db"
	defer os.Remove(testDB)
	defer ResetForTesting()

	db, err := Connect(testDB)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// First seed should insert products
	if err := Seed(db); err != nil {
		t.Fatalf("First seed failed: %v", err)
	}

	var count int64
	db.Model(&models.Coffee{}).Count(&count)
	if count != 6 {
		t.Errorf("Expected 6 products, got %d", count)
	}

	// Second seed should be idempotent (no duplicates)
	if err := Seed(db); err != nil {
		t.Fatalf("Second seed failed: %v", err)
	}

	db.Model(&models.Coffee{}).Count(&count)
	if count != 6 {
		t.Errorf("Expected 6 products after second seed, got %d", count)
	}

	// Verify product data
	var products []models.Coffee
	db.Find(&products)

	expectedNames := map[string]bool{
		"Espresso":   true,
		"Americano":  true,
		"Cappuccino": true,
		"Latte":      true,
		"Mocha":      true,
		"Macchiato":  true,
	}

	for _, p := range products {
		if !expectedNames[p.Name] {
			t.Errorf("Unexpected product name: %s", p.Name)
		}
		if p.Price <= 0 {
			t.Errorf("Product %s has invalid price: %d", p.Name, p.Price)
		}
	}
}

func TestGetDB(t *testing.T) {
	testDB := "test_getdb.db"
	defer os.Remove(testDB)
	defer ResetForTesting()

	// Initialize connection
	_, err := Connect(testDB)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// GetDB should return the same instance
	db1 := GetDB()
	db2 := GetDB()

	if db1 != db2 {
		t.Error("GetDB should return the same singleton instance")
	}
}
