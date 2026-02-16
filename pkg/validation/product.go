package validation

import (
	"fmt"
	"strings"
)

// ProductRequest represents the request body for creating/updating products
type ProductRequest struct {
	Name        string  `json:"name"`
	RoastType   string  `json:"roast_type"`
	Ounces      int     `json:"ounces"`
	BeanType    string  `json:"bean_type"`
	Price       float64 `json:"price"`
	Color       string  `json:"color"`
	Description string  `json:"description"`
}

// ProductErrors holds validation errors for a product request
type ProductErrors map[string]string

// ValidateProductRequest validates a product creation/update request
func ValidateProductRequest(req ProductRequest, isUpdate bool) ProductErrors {
	errors := make(ProductErrors)

	// Name validation (required for create, optional for update)
	if !isUpdate || req.Name != "" {
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" && !isUpdate {
			errors["name"] = "Name is required"
		} else if len(req.Name) > 255 {
			errors["name"] = "Name must be less than 255 characters"
		}
	}

	// Roast type validation
	if req.RoastType != "" {
		validRoastTypes := map[string]bool{
			"Light Roast":  true,
			"Medium Roast": true,
			"Dark Roast":   true,
		}
		if !validRoastTypes[req.RoastType] {
			errors["roast_type"] = "Roast type must be Light Roast, Medium Roast, or Dark Roast"
		}
	}

	// Ounces validation
	if req.Ounces != 0 {
		if req.Ounces < 0 {
			errors["ounces"] = "Ounces must be positive"
		} else if req.Ounces > 64 {
			errors["ounces"] = "Ounces must be less than or equal to 64"
		}
	}

	// Bean type validation
	if req.BeanType != "" && len(req.BeanType) > 100 {
		errors["bean_type"] = "Bean type must be less than 100 characters"
	}

	// Price validation (required for create, optional for update)
	if !isUpdate || req.Price != 0 {
		if req.Price <= 0 && !isUpdate {
			errors["price"] = "Price must be greater than 0"
		} else if req.Price > 1000 {
			errors["price"] = "Price must be less than $1000"
		}
	}

	// Color validation (hex color)
	if req.Color != "" {
		if !strings.HasPrefix(req.Color, "#") || len(req.Color) != 7 {
			errors["color"] = "Color must be a valid hex color (e.g., #8B4513)"
		}
	}

	// Description validation
	if req.Description != "" && len(req.Description) > 1000 {
		errors["description"] = "Description must be less than 1000 characters"
	}

	return errors
}

// HasErrors returns true if there are any validation errors
func (e ProductErrors) HasErrors() bool {
	return len(e) > 0
}

// Error implements the error interface
func (e ProductErrors) Error() string {
	var messages []string
	for field, msg := range e {
		messages = append(messages, fmt.Sprintf("%s: %s", field, msg))
	}
	return strings.Join(messages, "; ")
}
