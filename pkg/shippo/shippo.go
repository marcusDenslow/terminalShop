package shippo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const baseURL = "https://api.goshippo.com"

const (
	labelPollInterval = 1 * time.Second
	labelPollTimeout  = 30 * time.Second
)

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
			Timeout:   10 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
	}
}

type Parcel struct {
	LengthCm float64
	WidthCm  float64
	HeightCm float64
	WeightKg float64
}

type LineItem struct {
	Title    string
	Quantity int
	WeightKg float64
}

type LabelResult struct {
	TransactionID  string
	RateID         string
	Carrier        string
	ServiceLevel   string
	TrackingNumber string
	TrackingURL    string
	LabelURL       string
	CostCents      int
}

type shipmentRate struct {
	ObjectID     string `json:"object_id"`
	Amount       string `json:"amount"`
	Currency     string `json:"currency"`
	Provider     string `json:"provider"`
	ServiceLevel struct {
		Name string `json:"name"`
	} `json:"servicelevel"`
}

type shipmentResponse struct {
	Status string         `json:"status"`
	Rates  []shipmentRate `json:"rates"`
}

type shippoOrderResponse struct {
	ObjectID string `json:"object_id"`
}

type transactionResponse struct {
	ObjectID       string `json:"object_id"`
	Status         string `json:"status"`
	ObjectState    string `json:"object_state"`
	TrackingNumber string `json:"tracking_number"`
	TrackingURL    string `json:"tracking_url_provider"`
	LabelURL       string `json:"label_url"`
	Rate           string `json:"rate"`
	Messages       []struct {
		Text string `json:"text"`
	} `json:"messages"`
}

func (c *Client) CreateLabel(ctx context.Context, to Address, email string, items []LineItem) (*LabelResult, error) {
	from := fromAddress()
	if from.Street1 == "" {
		return nil, fmt.Errorf("SHIP_FROM_STREET1 not configured")
	}

	totalKg := 0.0
	for _, it := range items {
		totalKg += float64(it.Quantity) * it.WeightKg
	}
	if totalKg <= 0 {
		return nil, fmt.Errorf("order has zero shipping weight")
	}

	parcel := Parcel{LengthCm: 30, WidthCm: 30, HeightCm: 30, WeightKg: totalKg}

	rate, err := c.createShipmentForRates(ctx, from, to, parcel)
	if err != nil {
		return nil, fmt.Errorf("get rates: %w", err)
	}

	shippoOrderID, err := c.createShippoOrder(ctx, to, email, items, totalKg)
	if err != nil {
		return nil, fmt.Errorf("create shippo order: %w", err)
	}

	tx, err := c.buyLabel(ctx, rate.ObjectID, shippoOrderID)
	if err != nil {
		return nil, fmt.Errorf("buy label: %w", err)
	}

	final, err := c.pollTransaction(ctx, tx.ObjectID)
	if err != nil {
		return nil, fmt.Errorf("poll transaction: %w", err)
	}

	costCents := parseAmountCents(rate.Amount)

	return &LabelResult{
		TransactionID:  final.ObjectID,
		RateID:         rate.ObjectID,
		Carrier:        rate.Provider,
		ServiceLevel:   rate.ServiceLevel.Name,
		TrackingNumber: final.TrackingNumber,
		TrackingURL:    final.TrackingURL,
		LabelURL:       final.LabelURL,
		CostCents:      costCents,
	}, nil
}

func (c *Client) createShipmentForRates(ctx context.Context, from, to Address, parcel Parcel) (*shipmentRate, error) {
	payload := map[string]any{
		"address_from": addrPayload(from),
		"address_to":   addrPayload(to),
		"parcels": []map[string]any{{
			"length":        parcel.LengthCm,
			"width":         parcel.WidthCm,
			"height":        parcel.HeightCm,
			"distance_unit": "cm",
			"weight":        parcel.WeightKg,
			"mass_unit":     "kg",
		}},
		"async": false,
		"extra": map[string]any{"bypass_address_validation": true},
	}

	var resp shipmentResponse
	if err := c.doJSON(ctx, "POST", "/shipments", payload, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "SUCCESS" || len(resp.Rates) == 0 {
		return nil, fmt.Errorf("shippo returned no rates (status=%s)", resp.Status)
	}
	sort.Slice(resp.Rates, func(i, j int) bool {
		ai, _ := strconv.ParseFloat(resp.Rates[i].Amount, 64)
		aj, _ := strconv.ParseFloat(resp.Rates[j].Amount, 64)
		return ai < aj
	})
	r := resp.Rates[0]
	return &r, nil
}

func (c *Client) createShippoOrder(ctx context.Context, to Address, email string, items []LineItem, totalKg float64) (string, error) {
	lines := make([]map[string]any, 0, len(items))
	for _, item := range items {
		lines = append(lines, map[string]any{
			"quantity":    item.Quantity,
			"title":       item.Title,
			"weight":      roundN(item.WeightKg, 4),
			"weight_unit": "kg",
		})
	}
	toPayload := addrPayload(to)
	if email != "" {
		toPayload["email"] = email
	}
	payload := map[string]any{
		"to_address":  toPayload,
		"line_items":  lines,
		"placed_at":   time.Now().Format(time.RFC3339),
		"weight":      roundN(totalKg, 2),
		"weight_unit": "kg",
	}
	var resp shippoOrderResponse
	if err := c.doJSON(ctx, "POST", "/v1/orders", payload, &resp); err != nil {
		return "", err
	}
	if resp.ObjectID == "" {
		return "", fmt.Errorf("shippo order returned no object_id")
	}
	return resp.ObjectID, nil
}

func (c *Client) buyLabel(ctx context.Context, rateID, shippoOrderID string) (*transactionResponse, error) {
	payload := map[string]any{
		"rate":  rateID,
		"order": shippoOrderID,
		"async": true,
	}
	var resp transactionResponse
	if err := c.doJSON(ctx, "POST", "/v1/transactions", payload, &resp); err != nil {
		return nil, err
	}
	if resp.ObjectState == "INVALID" {
		return nil, fmt.Errorf("transaction invalid: %s", firstMessage(resp.Messages))
	}
	return &resp, nil
}

func (c *Client) pollTransaction(ctx context.Context, txID string) (*transactionResponse, error) {
	deadLine := time.Now().Add(labelPollTimeout)
	for {
		var resp transactionResponse
		if err := c.doJSON(ctx, "GET", "/transactions/"+txID, nil, &resp); err != nil {
			return nil, err
		}
		switch resp.Status {
		case "SUCCESS":
			return &resp, nil
		case "ERROR":
			return nil, fmt.Errorf("shippo transaction error: %s", firstMessage(resp.Messages))
		}
		if time.Now().After(deadLine) {
			return nil, fmt.Errorf("timed out waiting for label (last status=%s)", resp.Status)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(labelPollInterval):
		}
	}
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, rdr)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "ShippoToken "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("shippo %s %s -> %d: %s", method, path, resp.StatusCode, string(respBody))
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("parse reponse: %w", err)
		}
	}
	return nil
}

func addrPayload(a Address) map[string]any {
	m := map[string]any{
		"name":    a.Name,
		"street1": a.Street1,
		"city":    a.City,
		"country": a.Country,
		"zip":     a.Zip,
	}
	if a.Street2 != "" {
		m["street2"] = a.Street2
	}
	if a.State != "" {
		m["state"] = a.State
	}
	if a.Phone != "" {
		m["phone"] = a.Phone
	}
	if a.Email != "" {
		m["email"] = a.Email
	}
	return m
}

// roundN rounds to n decimal places. Shippo's /v1/orders rejects >2 decimals
// on parcel weight and >4 on line-item weight.
func roundN(v float64, n int) float64 {
	pow := 1.0
	for i := 0; i < n; i++ {
		pow *= 10
	}
	if v >= 0 {
		return float64(int64(v*pow+0.5)) / pow
	}
	return float64(int64(v*pow-0.5)) / pow
}

func parseAmountCents(s string) int {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int(f * 100)
}

func firstMessage(msgs []struct {
	Text string `json:"text"`
}) string {
	if len(msgs) == 0 {
		return "no detail"
	}
	return msgs[0].Text
}

func fromAddress() Address {
	return Address{
		Name:    os.Getenv("SHIP_FROM_NAME"),
		Street1: os.Getenv("SHIP_FROM_STREET1"),
		Street2: os.Getenv("SHIP_FROM_STREET2"),
		City:    os.Getenv("SHIP_FROM_CITY"),
		State:   os.Getenv("SHIP_FROM_STATE"),
		Country: os.Getenv("SHIP_FROM_COUNTRY"),
		Zip:     os.Getenv("SHIP_FROM_ZIP"),
		Phone:   os.Getenv("SHIP_FROM_PHONE"),
		Email:   os.Getenv("SHIP_FROM_EMAIL"),
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
	Email   string `json:"email,omitempty"`
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
func (c *Client) ValidateAddress(ctx context.Context, addr Address) (*Address, error) {
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

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/v1/addresses", bytes.NewReader(body))
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
