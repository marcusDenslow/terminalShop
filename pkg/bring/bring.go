package bring

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const baseURL = "https://api.bring.com/address"

// Client handles communication with the Bring Address API.
type Client struct {
	apiUID     string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Bring API client.
func NewClient(apiUID, apiKey string) *Client {
	return &Client{
		apiUID: apiUID,
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
	}
}

// Address represents a Norwegian shipping address
type Address struct {
	Name    string
	Street1 string
	Street2 string
	City    string
	Zip     string
	Country string
	Phone   string
}

type validationMatch struct {
	StreetName  string `json:"street_name"`
	HouseNumber int    `json:"house_number"`
	Letter      string `json:"letter"`
	PostalCode  string `json:"postal_code"`
	City        string `json:"city"`
	Country     string `json:"country"`
}

// validationReason explains why an address was invalid
type validationReason struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// validationResponse is the top-level response from Bring's validation endpoint
type validationResponse struct {
	Valid       bool               `json:"valid"`
	Match       *validationMatch   `json:"match,omitempty"`
	Reasons     []validationReason `json:"reasons,omitempty"`
	Suggestions []validationMatch  `json:"suggestions,omitempty"`
}

// streetNumberRegex splits "Storgata 5" into name="Storgata" number="5"
// and "Storgata 5B" into name="Storgata" number="5" (letter handled by Bring).
var streetNumberRegex = regexp.MustCompile(`^(.+?)\s+(\d+)\s*[A-Za-z]?\s*$`)

// parseStreet splits a street line like "Storgata 5" into name and number.
// Returns the full string as name and empty number if not match.
func parseStreet(street string) (name string, number string) {
	matches := streetNumberRegex.FindStringSubmatch(strings.TrimSpace(street))
	if len(matches) >= 3 {
		return matches[1], matches[2]
	}
	return strings.TrimSpace(street), ""
}

func (c *Client) ValidateAddress(ctx context.Context, addr Address) (*Address, error) {
	streetName, streetNumber := parseStreet(addr.Street1)

	params := url.Values{}
	params.Set("address_type", "street")
	params.Set("street_or_place", streetName)
	params.Set("postal_code", addr.Zip)
	params.Set("city", addr.City)
	if streetNumber != "" {
		params.Set("street_number", streetNumber)
	}

	reqURL := fmt.Sprintf("%s/api/no/validation?%s", baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Mybring-API-Uid", c.apiUID)
	req.Header.Set("X-Mybring-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bring request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bring returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result validationResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Valid {
		reasons := make([]string, 0, len(result.Reasons))
		for _, r := range result.Reasons {
			reasons = append(reasons, r.Description)
		}
		return nil, fmt.Errorf("invalid address: %s", strings.Join(reasons, "; "))
	}

	normalizedStreet := result.Match.StreetName
	if result.Match.HouseNumber > 0 {
		normalizedStreet = fmt.Sprintf("%s %d", result.Match.StreetName, result.Match.HouseNumber)
	}

	if result.Match.Letter != "" {
		normalizedStreet += result.Match.Letter
	}

	validated := &Address{
		Name:    addr.Name,
		Street1: normalizedStreet,
		Street2: addr.Street2,
		City:    result.Match.City,
		Zip:     result.Match.PostalCode,
		Country: addr.Country,
		Phone:   addr.Phone,
	}
	return validated, nil
}
