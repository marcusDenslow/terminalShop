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
}

// NewClient creates a new API client
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
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

	resp, err := c.HTTPClient.Get(url)
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

	resp, err := c.HTTPClient.Get(url)
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

	resp, err := c.HTTPClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
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
