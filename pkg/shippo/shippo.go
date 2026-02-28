package shippo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "https://api.goshippo.com"

// Client handles communication with the Shippo API.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Shippo API client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Address represents a shipping address for validation.
type Address struct {
	Name    string `json:"name"`
	Street1 string `json:"street1"`
	Street2 string `json:"street2,omitempty"`
	City    string `json:"city"`
	State   string `json:"state,omitempty"`
	Country string `json:"country"`
	Zip     string `json:"zip"`
	Phone   string `json:"phone,omitempty"`
}

// addressRequest is the payload sent to Shippo's address validation endpoint.
type addressRequest struct {
	Name     string `json:"name"`
	Street1  string `json:"street1"`
	Street2  string `json:"street2,omitempty"`
	City     string `json:"city"`
	State    string `json:"state,omitempty"`
	Country  string `json:"country"`
	Zip      string `json:"zip"`
	Phone    string `json:"phone,omitempty"`
	Validate bool   `json:"validate"`
}

// validationResult is Shippo's validation response nested under validation_results.
// IsValid is a pointer to distinguish between absent (nil) and explicitly false.
// Shippo returns empty validation_results for international addresses, which
// means "no data to validate" rather than "invalid".
type validationResult struct {
	IsValid *bool `json:"is_valid"`
}

// addressResponse is the response from Shippo's address endpoint.
type addressResponse struct {
	Street1           string            `json:"street1"`
	Street2           string            `json:"street2"`
	City              string            `json:"city"`
	State             string            `json:"state"`
	Country           string            `json:"country"`
	Zip               string            `json:"zip"`
	Phone             string            `json:"phone"`
	Test              bool              `json:"test"`
	ValidationResults *validationResult `json:"validation_results"`
}

// ValidateAddress sends an address to Shippo for validation.
// Returns the normalized/corrected address if valid, or an error if invalid.
func (c *Client) ValidateAddress(addr Address) (*Address, error) {
	reqBody := addressRequest{
		Name:     addr.Name,
		Street1:  addr.Street1,
		Street2:  addr.Street2,
		City:     addr.City,
		State:    addr.State,
		Country:  addr.Country,
		Zip:      addr.Zip,
		Phone:    addr.Phone,
		Validate: true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal address: %w", err)
	}

	req, err := http.NewRequest("POST", baseURL+"/v1/addresses", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "ShippoToken "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("shippo request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("shippo returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result addressResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Only accept addresses that Shippo explicitly validates as valid. 
	// International addressees where Shippo cannot validate are rejected.
	// Address validation for addresses in other countries need to be added on a per-country API basis.
	// Going to add Bring validation for Norway next
	if result.ValidationResults == nil || result.ValidationResults.IsValid == nil || !*result.ValidationResults.IsValid {
		return nil, fmt.Errorf("invalid address")
	}

	// Return the normalized address from Shippo
	validated := &Address{
		Name:    addr.Name,
		Street1: result.Street1,
		Street2: result.Street2,
		City:    result.City,
		State:   result.State,
		Country: result.Country,
		Zip:     result.Zip,
		Phone:   result.Phone,
	}

	// If Shippo returned empty phone, keep the original
	if validated.Phone == "" {
		validated.Phone = addr.Phone
	}

	return validated, nil
}
