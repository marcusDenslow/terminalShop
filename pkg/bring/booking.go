package bring

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"terminalShop/pkg/shipping"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const bookingBaseURL = "https://api.bring.com/booking-api/api/booking"

type BookingClient struct {
	apiUID         string
	apiKey         string
	customerNumber string
	testMode       bool
	httpClient     *http.Client
}

func NewBookingClient(apiUID string, apiKey string, customerNumber string, testMode bool) *BookingClient {
	return &BookingClient{
		apiUID:         apiUID,
		apiKey:         apiKey,
		customerNumber: customerNumber,
		testMode:       testMode,
		httpClient: &http.Client{
			Timeout:   20 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
	}
}

type BookingAddress struct {
	Name        string
	Street1     string
	Street2     string
	City        string
	Country     string
	Zip         string
	Phone       string
	Email       string
	ContactName string
}

type LineItem struct {
	Title    string
	Quantity int
	WeightKg float64
}

func (c *BookingClient) CreateLabel(ctx context.Context, to BookingAddress, email string, items []LineItem) (*shipping.LabelResult, error) {
	from := fromAddressNO()
	if from.Street1 == "" {
		return nil, fmt.Errorf("SHIP_FROM_NO_STREET1 not configured")
	}
	if c.customerNumber == "" {
		return nil, fmt.Errorf("BRING_CUSTOMER_NUMBER not configured")
	}

	totalKg := 0.0
	for _, item := range items {
		totalKg += float64(item.Quantity) * item.WeightKg
	}
	if totalKg <= 0 {
		return nil, fmt.Errorf("order has zero shipping weight")
	}

	oslo, err := time.LoadLocation("Europe/Oslo")
	if err != nil {
		oslo = time.UTC
	}
	shippingAt := time.Now().In(oslo).Add(1 * time.Hour).Format("2006-01-02T15:04:05-07:00")

	payload := map[string]any{
		"testIndicator": c.testMode,
		"schemaVersion": 1,
		"consignments": []map[string]any{{
			"shippingDateTime": shippingAt,
			"parties": map[string]any{
				"sender":    partyPayload(from, ""),
				"recipient": partyPayload(to, email),
			},
			"product": map[string]any{
				"id":             "SERVICEPAKKE",
				"customerNumber": c.customerNumber,
			},
			"packages": []map[string]any{{
				"weightInKg":       roundN(totalKg, 3),
				"goodsDescription": "Roasted coffee beans",
				"dimensions": map[string]any{
					"heightInCm": 30,
					"widthInCm":  30,
					"lengthInCm": 30,
				},
			}},
		}},
	}

	var resp bookingResponse
	if err := c.doJSON(ctx, bookingBaseURL, payload, &resp); err != nil {
		return nil, err
	}
	if len(resp.Consignments) == 0 {
		return nil, fmt.Errorf("bring returned no consignments")
	}
	c0 := resp.Consignments[0]
	if c0.Confirmation == nil {
		return nil, fmt.Errorf("bring booking failed: %s", firstError(c0.Errors))
	}

	confirmation := c0.Confirmation
	if len(confirmation.Packages) == 0 || confirmation.Packages[0].TrackingNumber == "" {
		return nil, fmt.Errorf("bring confirmation missing tracking number")
	}

	labelURL := ""
	if confirmation.Links != nil {
		labelURL = confirmation.Links.Labels
	}

	tracking := confirmation.Packages[0].TrackingNumber
	trackingURL := fmt.Sprintf("https://sporing.posten.no/sporing/%s", tracking)

	return &shipping.LabelResult{
		TransactionID:  confirmation.ConsignmentNumber,
		Carrier:        "BRING",
		ServiceLevel:   "SERVICEPAKKE",
		TrackingNumber: tracking,
		TrackingURL:    trackingURL,
		LabelURL:       labelURL,
		CostCents:      0,
	}, nil

}

type bookingResponse struct {
	Consignments []struct {
		Confirmation *struct {
			ConsignmentNumber string `json:"consignmentNumber"`
			Links             *struct {
				Labels string `json:"labels"`
			} `json:"links"`
			Packages []struct {
				CorrelationID  string `json:"correlationId"`
				TrackingNumber string `json:"trackingNumber"`
			} `json:"packages"`
		} `json:"confirmation"`
		Errors []struct {
			Messages []struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"messages"`
		} `json:"errors"`
	} `json:"consignments"`
}

func firstError(errs []struct {
	Messages []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"messages"`
}) string {
	if len(errs) == 0 || len(errs[0].Messages) == 0 {
		return "no detail"
	}
	return errs[0].Messages[0].Message
}

func (c *BookingClient) doJSON(ctx context.Context, url string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Mybring-API-Uid", c.apiUID)
	req.Header.Set("X-Mybring-API-Key", c.apiKey)
	req.Header.Set("X-Mybring-API-Customer-Number", c.customerNumber)

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
		return fmt.Errorf("bring booking %d: %s", resp.StatusCode, string(respBody))
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	return nil
}

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

func partyPayload(a BookingAddress, email string) map[string]any {
	m := map[string]any{
		"name":        a.Name,
		"addressLine": a.Street1,
		"postalCode":  a.Zip,
		"city":        a.City,
		"countryCode": a.Country,
	}
	if a.Street2 != "" {
		m["addressLine2"] = a.Street2
	}
	if email != "" || a.Phone != "" {
		m["contact"] = map[string]any{"name": a.Name, "email": email, "phoneNumber": a.Phone}
	}
	return m
}

func fromAddressNO() BookingAddress {
	return BookingAddress{
		Name:    os.Getenv("SHIP_FROM_NO_NAME"),
		Street1: os.Getenv("SHIP_FROM_NO_STREET1"),
		Street2: os.Getenv("SHIP_FROM_NO_STREET2"),
		City:    os.Getenv("SHIP_FROM_NO_CITY"),
		Country: os.Getenv("SHIP_FROM_NO_COUNTRY"),
		Zip:     os.Getenv("SHIP_FROM_NO_ZIP"),
		Phone:   os.Getenv("SHIP_FROM_NO_PHONE"),
		Email:   os.Getenv("SHIP_FROM_NO_EMAIL"),
	}
}
