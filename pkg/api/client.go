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

// TokenRequest represents a request to exchange SSH fingerprint for JWT.
type TokenRequest struct {
	Fingerprint  string `json:"fingerprint"`
	SSHPublicKey string `json:"ssh_public_key"`
	ClientSecret string `json:"client_secret"`
}

// TokenResponse represents the JWT token response.
type TokenResponse struct {
	Success bool `json:"success"`
	Data    struct {
		AccessToken string            `json:"access_token"`
		TokenType   string            `json:"token_type"`
		ExpiresIn   int               `json:"expires_in"`
		User        models.PublicUser `json:"user"`
	} `json:"data"`
	Error *APIError `json:"error,omitempty"`
}

func (c *Client) GetOrCreateToken(fingerprint, pubKeyStr, clientSecret string) (string, models.PublicUser, error) {

	url := fmt.Sprintf("%s/api/v1/auth/token", c.BaseURL)

	reqBody := TokenRequest{
		Fingerprint:  fingerprint,
		SSHPublicKey: pubKeyStr,
		ClientSecret: clientSecret,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", models.PublicUser{}, fmt.Errorf("failed to marshal token request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", models.PublicUser{}, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", models.PublicUser{}, fmt.Errorf("token refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", models.PublicUser{}, fmt.Errorf("token refresh returned status %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", models.PublicUser{}, fmt.Errorf("failed to decode token response: %w", err)
	}

	if !tokenResp.Success || tokenResp.Data.AccessToken == "" {
		return "", models.PublicUser{}, fmt.Errorf("token refresh failed: no access token in response")
	}

	return tokenResp.Data.AccessToken, tokenResp.Data.User, nil
}

func (c *Client) RefreshToken(fingerprint, clientSecret string) (string, error) {
	token, _, err := c.GetOrCreateToken(fingerprint, "", clientSecret)
	return token, err
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

// CartResponse represents the cart API response.
type CartResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Cart CartData `json:"cart"`
	} `json:"data"`
	Error *APIError `json:"error,omitempty"`
}

// CartData represents the cart payload from the API.
type CartData struct {
	Items           []CartItemData `json:"items"`
	Subtotal        int            `json:"subtotal"`
	AddressID       *uint          `json:"address_id"`
	CardID          *uint          `json:"card_id"`
	ShippingCost    int            `json:"shipping_cost"`
	ShippingService string         `json:"shipping_service"`
}

// CartItemData represents a single item in the cart response.
type CartItemData struct {
	ID       uint          `json:"id"`
	CoffeeID uint          `json:"coffee_id"`
	Quantity int           `json:"quantity"`
	Subtotal int           `json:"subtotal"`
	Coffee   models.Coffee `json:"coffee"`
}

// CardsResponse represents the cards list API response.
type CardsResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Cards []models.Card `json:"cards"`
	} `json:"data"`
	Error *APIError `json:"error,omitempty"`
}

// CardResponse represents a single card API response.
type CardResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Card models.Card `json:"card"`
	} `json:"data"`
	Error *APIError `json:"error,omitempty"`
}

// ConvertCartResponse represents the cart conversion (checkout) API response.
type ConvertCartResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Order models.Order `json:"order"`
	} `json:"data"`
	Error *APIError `json:"error,omitempty"`
}

// GetCart fetches the user's current cart.
func (c *Client) GetCart() (*CartData, error) {
	url := fmt.Sprintf("%s/api/v1/cart", c.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cart: %w", err)
	}
	defer resp.Body.Close()

	var cartResp CartResponse
	if err := json.NewDecoder(resp.Body).Decode(&cartResp); err != nil {
		return nil, fmt.Errorf("failed to decode cart response: %w", err)
	}

	if !cartResp.Success {
		if cartResp.Error != nil {
			return nil, fmt.Errorf("%s: %s", cartResp.Error.Code, cartResp.Error.Message)
		}
		return nil, fmt.Errorf("failed to fetch cart")
	}

	return &cartResp.Data.Cart, nil
}

// SetCartItem upserts an item in the cart. Quantity <= 0 removes it.
func (c *Client) SetCartItem(coffeeID uint, quantity int) (*CartData, error) {
	url := fmt.Sprintf("%s/api/v1/cart/item", c.BaseURL)

	body := map[string]interface{}{
		"coffee_id": coffeeID,
		"quantity":  quantity,
	}
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to set cart item: %w", err)
	}
	defer resp.Body.Close()

	var cartResp CartResponse
	if err := json.NewDecoder(resp.Body).Decode(&cartResp); err != nil {
		return nil, fmt.Errorf("failed to decode cart response: %w", err)
	}

	if !cartResp.Success {
		if cartResp.Error != nil {
			return nil, fmt.Errorf("%s: %s", cartResp.Error.Code, cartResp.Error.Message)
		}
		return nil, fmt.Errorf("failed to set cart item")
	}

	return &cartResp.Data.Cart, nil
}

// SetCartAddress assigns a saved address to the cart.
func (c *Client) SetCartAddress(addressID uint) error {
	url := fmt.Sprintf("%s/api/v1/cart/address", c.BaseURL)

	body := map[string]interface{}{
		"address_id": addressID,
	}
	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to set cart address: %w", err)
	}
	defer resp.Body.Close()

	var result APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		if result.Error != nil {
			return fmt.Errorf("%s: %s", result.Error.Code, result.Error.Message)
		}
		return fmt.Errorf("failed to set cart address")
	}

	return nil
}

// SetCartCard assigns a saved card to the cart.
func (c *Client) SetCartCard(cardID uint) error {
	url := fmt.Sprintf("%s/api/v1/cart/card", c.BaseURL)

	body := map[string]interface{}{
		"card_id": cardID,
	}
	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to set cart card: %w", err)
	}
	defer resp.Body.Close()

	var result APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		if result.Error != nil {
			return fmt.Errorf("%s: %s", result.Error.Code, result.Error.Message)
		}
		return fmt.Errorf("failed to set cart card")
	}

	return nil
}

// ClearCart removes all items from the user's cart.
func (c *Client) ClearCart() error {
	url := fmt.Sprintf("%s/api/v1/cart", c.BaseURL)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to clear cart: %w", err)
	}
	defer resp.Body.Close()

	var result APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		if result.Error != nil {
			return fmt.Errorf("%s: %s", result.Error.Code, result.Error.Message)
		}
		return fmt.Errorf("failed to clear cart")
	}

	return nil
}

// ConvertCart converts the cart into an order, charging the saved card.
func (c *Client) ConvertCart() (*models.Order, error) {
	url := fmt.Sprintf("%s/api/v1/cart/convert", c.BaseURL)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert cart: %w", err)
	}
	defer resp.Body.Close()

	var convertResp ConvertCartResponse
	if err := json.NewDecoder(resp.Body).Decode(&convertResp); err != nil {
		return nil, fmt.Errorf("failed to decode convert response: %w", err)
	}

	if !convertResp.Success {
		if convertResp.Error != nil {
			return nil, fmt.Errorf("%s: %s", convertResp.Error.Code, convertResp.Error.Message)
		}
		return nil, fmt.Errorf("failed to convert cart")
	}

	return &convertResp.Data.Order, nil
}

// GetCards fetches all saved cards for the user.
func (c *Client) GetCards() ([]models.Card, error) {
	url := fmt.Sprintf("%s/api/v1/cards", c.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cards: %w", err)
	}
	defer resp.Body.Close()

	var cardsResp CardsResponse
	if err := json.NewDecoder(resp.Body).Decode(&cardsResp); err != nil {
		return nil, fmt.Errorf("failed to decode cards response: %w", err)
	}

	if !cardsResp.Success {
		if cardsResp.Error != nil {
			return nil, fmt.Errorf("%s: %s", cardsResp.Error.Code, cardsResp.Error.Message)
		}
		return nil, fmt.Errorf("failed to fetch cards")
	}

	return cardsResp.Data.Cards, nil
}

// SaveCardRequest holds the raw card fields sent to the backend for
// server-side tokenization. The Stripe secret key never leaves the server.
type SaveCardRequest struct {
	Token string `json:"token"`
}

// SaveCard sends raw card fields to the backend, which tokenizes them
// server-side and returns the saved card metadata.
func (c *Client) SaveCard(params SaveCardRequest) (*models.Card, error) {
	url := fmt.Sprintf("%s/api/v1/cards", c.BaseURL)

	jsonData, err := json.Marshal(params)
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
		return nil, fmt.Errorf("failed to save card: %w", err)
	}
	defer resp.Body.Close()

	var cardResp CardResponse
	if err := json.NewDecoder(resp.Body).Decode(&cardResp); err != nil {
		return nil, fmt.Errorf("failed to decode card response: %w", err)
	}

	if !cardResp.Success {
		if cardResp.Error != nil {
			return nil, fmt.Errorf("%s: %s", cardResp.Error.Code, cardResp.Error.Message)
		}
		return nil, fmt.Errorf("failed to save card")
	}

	return &cardResp.Data.Card, nil
}

type CollectCardResponse struct {
	Success bool `json:"success"`
	Data    struct {
		URL string `json:"url"`
	} `json:"data"`
	Error *APIError `json:"error,omitempty"`
}

func (c *Client) CollectCard() (string, error) {
	url := fmt.Sprintf("%s/api/v1/cards/collect", c.BaseURL)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return "", fmt.Errorf("failed to collect card: %w", err)
	}
	defer resp.Body.Close()

	var collectResp CollectCardResponse
	if err := json.NewDecoder(resp.Body).Decode(&collectResp); err != nil {
		return "", fmt.Errorf("failed to decode collect response: %w", err)
	}

	if !collectResp.Success {
		if collectResp.Error != nil {
			return "", fmt.Errorf("%s: %s", collectResp.Error.Code, collectResp.Error.Message)
		}
		return "", fmt.Errorf("failed to collect card")
	}
	return collectResp.Data.URL, nil
}

// DeleteCard removes a saved card by ID.
func (c *Client) DeleteCard(id uint) error {
	url := fmt.Sprintf("%s/api/v1/cards/%d", c.BaseURL, id)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to delete card: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete card failed with status %d", resp.StatusCode)
	}

	return nil
}

type OrderResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Orders []models.Order `json:"orders"`
	} `json:"data"`
	Error *APIError `json:"error,omitempty"`
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

type ViewInitData struct {
	User      models.PublicUser `json:"user"`
	Products  []models.Coffee   `json:"products"`
	Cart      []CartItemData    `json:"cart"`
	Addresses []models.Address  `json:"addresses"`
	Cards     []models.Card     `json:"cards"`
	Orders    []models.Order    `json:"orders"`
}

type ViewInitResponse struct {
	Success bool         `json:"success"`
	Data    ViewInitData `json:"data"`
	Error   *APIError    `json:"error"`
}

// GetViewInit fetches all initial data in one request
func (c *Client) GetViewInit() (ViewInitData, error) {
	url := fmt.Sprintf("%s/api/v1/view/init", c.BaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ViewInitData{}, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := c.doRequest(req)
	if err != nil {
		return ViewInitData{}, fmt.Errorf("view init request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ViewInitData{}, fmt.Errorf("view init returned status: %d", resp.StatusCode)
	}
	var viewResp ViewInitResponse

	if err := json.NewDecoder(resp.Body).Decode(&viewResp); err != nil {
		return ViewInitData{}, fmt.Errorf("failed to decode view init response: %w", err)
	}
	return viewResp.Data, nil
}
