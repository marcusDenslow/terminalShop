package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"terminalShop/pkg/models"
)

// Client handles communication with the API
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      string
}

// NewClient creates a new API client
func NewClient(baseURL string, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// doRequest helper executes an HTTP request with the authorization header set
func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return c.HTTPClient.Do(req)
}

// APIResponse represents the standard API response format
type APIResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data"`
	Error   *APIError              `json:"error,omitempty"`
}

// APIError represents an error from the API
type APIError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ProductsResponse represents the products API response
type ProductsResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Products []models.Coffee `json:"products"`
	} `json:"data"`
}

// GetProducts fetches all products from the API
func (c *Client) GetProducts() ([]models.Coffee, error) {
	url := fmt.Sprintf("%s/api/v1/products", c.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch products: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var productsResp ProductsResponse
	if err := json.NewDecoder(resp.Body).Decode(&productsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !productsResp.Success {
		return nil, fmt.Errorf("API request was not successful")
	}

	return productsResp.Data.Products, nil
}

// Health checks if the API is healthy
func (c *Client) Health() error {
	url := fmt.Sprintf("%s/api/v1/health", c.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return nil
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Username          string `json:"username"`
	SSHPublicKey      string `json:"ssh_public_key"`
	SSHKeyFingerprint string `json:"ssh_key_fingerprint"`
}

// RegisterResponse represents the registration API response
type RegisterResponse struct {
	Success bool `json:"success"`
	Data    struct {
		User models.User `json:"user"`
	} `json:"data"`
	Error *APIError `json:"error,omitempty"`
}

// RegisterUser registers a new user with SSH key authentication
func (c *Client) RegisterUser(username, sshPublicKey, sshKeyFingerprint string) (*models.User, error) {
	url := fmt.Sprintf("%s/api/v1/auth/register", c.BaseURL)

	reqBody := RegisterRequest{
		Username:          username,
		SSHPublicKey:      sshPublicKey,
		SSHKeyFingerprint: sshKeyFingerprint,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to register user: %w", err)
	}
	defer resp.Body.Close()

	var registerResp RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&registerResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !registerResp.Success {
		if registerResp.Error != nil {
			return nil, fmt.Errorf("%s: %s", registerResp.Error.Code, registerResp.Error.Message)
		}
		return nil, fmt.Errorf("registration failed")
	}

	return &registerResp.Data.User, nil
}

// CheckoutRequest represents the checkout payload sent to the API
type CheckoutRequest struct {
	StripeToken string             `json:"stripe_token"`
	Last4       string             `json:"last4"`
	Brand       string             `json:"brand"`
	ExpMonth    int                `json:"exp_month"`
	ExpYear     int                `json:"exp_year"`
	Items       []CheckoutCartItem `json:"items"`
	AddressID   uint               `json:"address_id"`
}

type CheckoutCartItem struct {
	CoffeeID uint `json:"coffee_id"`
	Quantity int  `json:"quantity"`
}

type CheckoutResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Order models.Order `json:"order"`
	} `json:"data"`
	Error *APIError `json:"error,omitempty"`
}

type OrderResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Orders []models.Order `json:"orders"`
	} `json:"data"`
	Error *APIError `json:"error,omitempty"`
}

func (c *Client) Checkout(checkoutReq CheckoutRequest) (*models.Order, error) {
	url := fmt.Sprintf("%s/api/v1/checkout", c.BaseURL)

	jsonData, err := json.Marshal(checkoutReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal checkout request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("checkout request failed: %w", err)
	}

	defer resp.Body.Close()

	var checkoutResp CheckoutResponse
	if err := json.NewDecoder(resp.Body).Decode(&checkoutResp); err != nil {
		return nil, fmt.Errorf("failed to decode checkout response: %w", err)
	}

	if !checkoutResp.Success {
		if checkoutResp.Error != nil {
			return nil, fmt.Errorf("%s: %s", checkoutResp.Error.Code, checkoutResp.Error.Message)
		}
		return nil, fmt.Errorf("checkout failed")
	}
	return &checkoutResp.Data.Order, nil
}

func (c *Client) GetOrders() ([]models.Order, error) {
	url := fmt.Sprintf("%s/api/v1/orders", c.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch orders: %w", err)
	}
	defer resp.Body.Close()

	var ordersResp OrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&ordersResp); err != nil {
		return nil, fmt.Errorf("failed to decode orders response: %w", err)
	}

	if !ordersResp.Success {
		if ordersResp.Error != nil {
			return nil, fmt.Errorf("%s: %s", ordersResp.Error.Code, ordersResp.Error.Message)
		}
		return nil, fmt.Errorf("failed to fetch orders")
	}
	return ordersResp.Data.Orders, nil
}

// The Addresses API response
type AddressResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Addresses []models.Address `json:"addresses"`
	} `json:"data"`
	Error *APIError `json:"error,omitempty"`
}

func (c *Client) GetAddresses() ([]models.Address, error) {
	url := fmt.Sprintf("%s/api/v1/addresses", c.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch addresses: %w", err)
	}

	defer resp.Body.Close()

	var addrResp AddressResponse
	if err := json.NewDecoder(resp.Body).Decode(&addrResp); err != nil {
		return nil, fmt.Errorf("failed to decode addresses response: %w", err)
	}

	if !addrResp.Success {
		if addrResp.Error != nil {
			return nil, fmt.Errorf("%s: %s", addrResp.Error.Code, addrResp.Error.Message)
		}
		return nil, fmt.Errorf("failed to fetch addresses")
	}

	return addrResp.Data.Addresses, nil
}

type CreateAddressRequest struct {
	Name    string `json:"name"`
	Street  string `json:"street"`
	Street2 string `json:"street_2"`
	City    string `json:"city"`
	State   string `json:"state"`
	Zip     string `json:"zip"`
	Country string `json:"country"`
	Phone   string `json:"phone"`
}

func (c *Client) CreateAddress(addrReq CreateAddressRequest) (*models.Address, error) {
	url := fmt.Sprintf("%s/api/v1/addresses", c.BaseURL)

	jsonData, err := json.Marshal(addrReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal address request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create address: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Address models.Address `json:"address"`
		} `json:"data"`
		Error *APIError `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode address response: %w", err)
	}

	if !result.Success {
		if result.Error != nil {
			return nil, fmt.Errorf("%s: %s", result.Error.Code, result.Error.Message)
		}
		return nil, fmt.Errorf("failed to create address")
	}

	return &result.Data.Address, nil
}

// Delete a saved address by ID
func (c *Client) DeleteAddress(id uint) error {
	url := fmt.Sprintf("%s/api/v1/addresses/%d", c.BaseURL, id)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to delete address: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete address failed with status %d", resp.StatusCode)
	}

	return nil
}
