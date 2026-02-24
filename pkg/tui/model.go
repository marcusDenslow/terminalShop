package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	gossh "golang.org/x/crypto/ssh"

	"terminalShop/pkg/api"
	"terminalShop/pkg/models"
	"terminalShop/pkg/shippo"
)

type Model struct {
	// User authentication
	User          *models.User    // Authenticated user (nil if not logged in)
	IsNewUser     bool            // True if user needs to register
	SSHPublicKey  gossh.PublicKey // SSH public key for registration
	UsernameInput string          // Input for username during registration

	// Shop state
	Username       string
	Coffees        []models.Coffee
	Cursor         int
	Cart           map[int]*models.CartItem // maps coffee index to cart item
	CartCursor     int                      // cursor position in cart view
	AccountCursor  int
	CheckoutStep   int // 0=cart, 1=shipping, 2=payment, 3=confirmation
	ScrollOffset   int // scroll position for content
	WindowWidth    int
	WindowHeight   int
	ViewingCart    bool // true when viewing cart details
	ViewingAccount bool
	Loading        bool   // true when fetching data from API
	ErrorMsg       string // error message if API fetch fails
	APIClient      *api.Client

	// Checkout state
	ShippingForm   *ShippingFormState        // Shipping form state (nil when not in shipping step)
	PaymentForm    *PaymentFormState         // Payment form state (nil when not in payment step)
	ShippingInfo   *ShippingFormCompleteMsg  // Saved shipping info from completed form
	SavedAddresses []ShippingFormCompleteMsg // Previously used shipping addresses
	ShippingView   int
	AddressCursor  int
	StripeKey      string // Stripe publishable key for client-side tokenization
	ShippoKey      string // Shippo API key for address validation

	// Order history
	Orders       []models.Order
	OrdersLoaded bool
}

// ProductsMsg is sent when products are fetched from API
type ProductsMsg struct {
	Products []models.Coffee
	Err      error
}

// fetchProductsCmd fetches products from the API
func (m Model) fetchProductsCmd() tea.Msg {
	if m.APIClient == nil {
		return ProductsMsg{Err: nil} // No API client, use fallback
	}

	products, err := m.APIClient.GetProducts()
	return ProductsMsg{Products: products, Err: err}
}

func (m Model) Init() tea.Cmd {
	// Fetch products from API on startup
	return m.fetchProductsCmd
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// When the shipping form is active, route ALL messages to it.
	// huh generates internal messages (focus, cursor blink, etc.) from
	// form.Init() and form.Update() that must reach the form to work.

	if m.ViewingCart && m.CheckoutStep == 1 {
		switch msg := msg.(type) {
		case ShippingFormCompleteMsg:
			return m, m.validateAddress(msg)
		case ShippoValidatedMsg:
			m.SavedAddresses = append(m.SavedAddresses, msg.Address)
			m.ShippingInfo = &msg.Address
			m.ShippingForm = nil
			m.CheckoutStep = 2
			m.PaymentForm = m.InitPaymentForm()
			return m, m.PaymentForm.form.Init()
		case ShippoValidationErrMsg:
			if m.ShippingForm != nil {
				m.ShippingForm.submitting = false
				m.ShippingForm.form = m.buildShippingForm(m.ShippingForm)
				m.ErrorMsg = fmt.Sprintf("Invalid address: %v", msg.Err)
				return m, m.ShippingForm.form.Init()
			}
			m.ErrorMsg = fmt.Sprintf("Invalid address: %v", msg.Err)
			return m, nil
		case ShippingFormErrorMsg:
			m.ErrorMsg = msg.Message
			return m, nil
		case tea.WindowSizeMsg:
			m.WindowWidth = msg.Width
			m.WindowHeight = msg.Height
		}

		// Address list view
		if m.ShippingView == 0 && m.ShippingForm == nil {
			if keyMsg, ok := msg.(tea.KeyMsg); ok {
				m.ErrorMsg = ""
				switch keyMsg.String() {
				case "esc":
					m.CheckoutStep = 0
					m.ErrorMsg = ""
					return m, nil
				case "up", "k":
					if m.AddressCursor > 0 {
						m.AddressCursor--
					}
				case "down", "j":
					if m.AddressCursor < len(m.SavedAddresses) {
						m.AddressCursor++
					}
				case "enter":
					if m.AddressCursor < len(m.SavedAddresses) {
						addr := m.SavedAddresses[m.AddressCursor]
						m.ShippingInfo = &addr
						m.CheckoutStep = 2
						m.PaymentForm = m.InitPaymentForm()
						return m, m.PaymentForm.form.Init()
					}
					m.ShippingView = 1
					m.ShippingForm = m.InitShippingForm()
					return m, m.ShippingForm.form.Init()
				case "d", "x":
					if m.AddressCursor < len(m.SavedAddresses) {
						m.SavedAddresses = append(m.SavedAddresses[:m.AddressCursor], m.SavedAddresses[m.AddressCursor+1:]...)
						if m.AddressCursor >= len(m.SavedAddresses) && m.AddressCursor > 0 {
							m.AddressCursor--
						}
					}
				}
			}
			return m, nil
		}

		// Form view
		if m.ShippingForm != nil {
			if keyMsg, ok := msg.(tea.KeyMsg); ok {
				if keyMsg.String() == "esc" {
					m.ErrorMsg = ""
					if len(m.SavedAddresses) > 0 {
						m.ShippingView = 0
						m.ShippingForm = nil
						return m, nil
					}

					m.CheckoutStep = 0
					m.ShippingForm = nil
					return m, nil
				}
				m.ErrorMsg = ""
			}
			cmd := m.UpdateShippingForm(msg, m.ShippingForm)
			return m, cmd
		}
		return m, nil
	}

	// When the payment form is active, route ALL messages to it.
	if m.ViewingCart && m.CheckoutStep == 2 && m.PaymentForm != nil {
		switch msg := msg.(type) {
		case PaymentFormCompleteMsg:
			// kick off Stripe tokenization (runs async)
			return m, m.tokenizeCard(msg)
		case StripeTokenMsg:
			m.PaymentForm = nil
			return m, m.submitCheckout(msg)
		case CheckoutResultMsg:
			if msg.Err != nil {
				m.ErrorMsg = fmt.Sprintf("checkout failed: %v", msg.Err)
				m.CheckoutStep = 2
				m.PaymentForm = m.InitPaymentForm()
				return m, m.PaymentForm.form.Init()
			}
			m.CheckoutStep = 3
			return m, nil
		case StripeTokenErrMsg:
			// Tokenization failed, show error and let user retry
			m.PaymentForm.submitting = false
			m.ErrorMsg = fmt.Sprintf("Payment Failed: %v", msg.Err)
			m.PaymentForm.form = m.buildPaymentForm(m.PaymentForm)
			return m, m.PaymentForm.form.Init()
		case PaymentFormErrorMsg:
			m.ErrorMsg = msg.Message
			return m, nil
		case tea.WindowSizeMsg:
			m.WindowWidth = msg.Width
			m.WindowHeight = msg.Height
		case tea.KeyMsg:
			if msg.String() == "esc" {
				// Go back to shipping
				m.CheckoutStep = 1
				m.PaymentForm = nil
				m.ErrorMsg = ""
				m.ShippingForm = m.InitShippingForm()
				// Restore shipping values if we have them
				if m.ShippingInfo != nil {
					m.ShippingForm.Name = m.ShippingInfo.Name
					m.ShippingForm.Street1 = m.ShippingInfo.Street1
					m.ShippingForm.Street2 = m.ShippingInfo.Street2
					m.ShippingForm.City = m.ShippingInfo.City
					m.ShippingForm.State = m.ShippingInfo.State
					m.ShippingForm.Country = m.ShippingInfo.Country
					m.ShippingForm.Zip = m.ShippingInfo.Zip
					m.ShippingForm.Phone = m.ShippingInfo.Phone
					// Rebuild form with restored values
					m.ShippingForm.form = m.buildShippingForm(m.ShippingForm)
				}
				return m, m.ShippingForm.form.Init()
			}
			m.ErrorMsg = ""
		}

		cmd := m.UpdatePaymentForm(msg, m.PaymentForm)
		return m, cmd
	}

	switch msg := msg.(type) {
	case ShippingFormCompleteMsg:
		return m, m.validateAddress(msg)

	case ShippoValidatedMsg:
		m.SavedAddresses = append(m.SavedAddresses, msg.Address)
		m.ShippingInfo = &msg.Address
		m.ShippingForm = nil
		m.CheckoutStep = 2
		m.PaymentForm = m.InitPaymentForm()
		return m, m.PaymentForm.form.Init()

	case ShippoValidationErrMsg:
		m.ErrorMsg = fmt.Sprintf("invalid address: %v", msg.Err)
		return m, nil

	case ShippingFormErrorMsg:
		m.ErrorMsg = msg.Message
		return m, nil

	case PaymentFormCompleteMsg:
		m.CheckoutStep = 3
		m.PaymentForm = nil
		return m, nil

	case PaymentFormErrorMsg:
		m.ErrorMsg = msg.Message
		return m, nil

	case StripeTokenMsg:
		m.CheckoutStep = 3
		m.PaymentForm = nil
		return m, nil

	case StripeTokenErrMsg:
		m.ErrorMsg = fmt.Sprintf("payment failed: %v", msg.Err)
		return m, nil

	case ProductsMsg:
		m.Loading = false
		if msg.Err != nil {
			m.ErrorMsg = "Failed to load products from API, using fallback data"
			// Keep the fallback products that were set in NewModel
		} else if len(msg.Products) > 0 {
			m.Coffees = msg.Products
			m.ErrorMsg = ""
		}
		return m, nil

	case OrdersMsg:
		if msg.Err == nil {
			m.Orders = msg.Orders
			m.OrdersLoaded = true
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.WindowWidth = msg.Width
		m.WindowHeight = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Normal shop mode
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "s":
			m.ViewingCart = false
			m.ViewingAccount = false
			m.CheckoutStep = 0
			m.ShippingForm = nil
			m.PaymentForm = nil
			m.ScrollOffset = 0

		case "c":
			m.ViewingCart = true
			m.ViewingAccount = false
			m.CheckoutStep = 0
			m.ShippingForm = nil
			m.PaymentForm = nil
			m.ScrollOffset = 0

		case "a":
			m.ViewingCart = false
			m.ViewingAccount = true
			m.CheckoutStep = 0
			m.ShippingForm = nil
			m.PaymentForm = nil
			m.ScrollOffset = 0
			if !m.OrdersLoaded {
				return m, m.fetchOrdersCmd()
			}

		case "p", "enter":
			// Proceed to checkout from cart
			if m.ViewingCart && m.CheckoutStep == 0 && len(m.Cart) > 0 {
				m.CheckoutStep = 1
				if len(m.SavedAddresses) > 0 {
					m.ShippingView = 0
					m.AddressCursor = 0
					m.ShippingForm = nil
					return m, nil
				}
				m.ShippingView = 1
				m.ShippingForm = m.InitShippingForm()
				return m, m.ShippingForm.form.Init()

			}

		case "esc":
			// From confirmation, go back to shop
			if m.ViewingCart && m.CheckoutStep == 3 {
				m.ViewingCart = false
				m.CheckoutStep = 0
				m.ShippingForm = nil
				m.PaymentForm = nil
				m.ShippingInfo = nil
				m.Cart = make(map[int]*models.CartItem)
				m.CartCursor = 0
				m.ScrollOffset = 0
				return m, nil
			}
			// Go back in checkout flow
			if m.ViewingCart && m.CheckoutStep > 0 {
				m.CheckoutStep--
				m.ShippingForm = nil
				m.PaymentForm = nil
				return m, nil
			}

		case "up", "k":
			if m.ViewingAccount {
				if m.AccountCursor > 0 {
					m.AccountCursor--
				}
			} else if !m.ViewingCart && !m.ViewingAccount && m.Cursor > 0 {
				m.Cursor--
			} else if m.ViewingCart && m.CartCursor > 0 {
				m.CartCursor--
				// Auto-scroll to keep cursor visible (each cart item is ~5 lines)
				itemHeight := 5
				targetLine := m.CartCursor * itemHeight
				if targetLine < m.ScrollOffset {
					m.ScrollOffset = targetLine
				}
			}

		case "down", "j":
			if m.ViewingAccount {
				if m.AccountCursor < len(models.AccountMenuItems)-1 {
					m.AccountCursor++
				}
			} else if !m.ViewingCart && !m.ViewingAccount && m.Cursor < len(m.Coffees)-1 {
				m.Cursor++
			} else if m.ViewingCart && m.CartCursor < len(m.Cart)-1 {
				m.CartCursor++
				// Auto-scroll to keep cursor visible (each cart item is ~5 lines)
				itemHeight := 5
				targetLine := m.CartCursor * itemHeight
				// Add extra lines to ensure the entire item is visible, not cut off
				targetLineEnd := targetLine + itemHeight

				// Calculate viewport height
				// In cart: header=3, breadcrumbs=2, footer=1, margins=2, buffer=1 = 9
				viewportHeight := m.WindowHeight - 9
				if viewportHeight < 6 {
					viewportHeight = 6
				}

				// Scroll if the bottom of item would be beyond viewport
				if targetLineEnd > m.ScrollOffset+viewportHeight {
					// Scroll just enough to show the entire item
					m.ScrollOffset = targetLineEnd - viewportHeight
				}
			}

		case "pgup", "ctrl+u":
			// Scroll up
			m.ScrollOffset -= 3
			if m.ScrollOffset < 0 {
				m.ScrollOffset = 0
			}

		case "pgdown", "ctrl+d":
			// Scroll down
			m.ScrollOffset += 3

		case "+", "=":
			if !m.ViewingCart && !m.ViewingAccount {
				// Increment quantity in shop view
				if item, exists := m.Cart[m.Cursor]; exists {
					item.Quantity++
				} else {
					m.Cart[m.Cursor] = &models.CartItem{
						Coffee:   m.Coffees[m.Cursor],
						Quantity: 1,
					}
				}
			} else if m.ViewingCart {
				// Increment quantity in cart view
				cartItems := m.GetCartItemsSlice()
				if m.CartCursor >= 0 && m.CartCursor < len(cartItems) {
					cartItems[m.CartCursor].Quantity++
				}
			}

		case "-", "_":
			if !m.ViewingCart && !m.ViewingAccount {
				// Decrement quantity in shop view
				if item, exists := m.Cart[m.Cursor]; exists {
					item.Quantity--
					if item.Quantity <= 0 {
						delete(m.Cart, m.Cursor)
						// Reset cart cursor if we deleted the last item
						if len(m.Cart) == 0 {
							m.CartCursor = 0
						} else if m.CartCursor >= len(m.Cart) {
							m.CartCursor = len(m.Cart) - 1
						}
					}
				}
			} else if m.ViewingCart {
				// Decrement quantity in cart view
				cartItems := m.GetCartItemsSlice()
				if m.CartCursor >= 0 && m.CartCursor < len(cartItems) {
					cartItems[m.CartCursor].Quantity--
					if cartItems[m.CartCursor].Quantity <= 0 {
						// Find and delete this item from the cart
						for idx, item := range m.Cart {
							if item == cartItems[m.CartCursor] {
								delete(m.Cart, idx)
								break
							}
						}
						// Reset cart cursor
						if len(m.Cart) == 0 {
							m.CartCursor = 0
						} else if m.CartCursor >= len(m.Cart) {
							m.CartCursor = len(m.Cart) - 1
						}
					}
				}
			}
		}
	}
	return m, nil
}

// ShippoValidatedMsg is sent when Shippo address validation succeeds
type ShippoValidatedMsg struct {
	Address ShippingFormCompleteMsg
}

// ShippoValidationErrMsg is sent when Shippo address validation fails
type ShippoValidationErrMsg struct {
	Err error
}

// validateAddress sends the shipping address to Shippo for validation
func (m Model) validateAddress(info ShippingFormCompleteMsg) tea.Cmd {
	return func() tea.Msg {
		if m.ShippoKey == "" {
			// No Shippo key configured, skip validation
			return ShippoValidatedMsg{Address: info}
		}

		client := shippo.NewClient(m.ShippoKey)
		validated, err := client.ValidateAddress(shippo.Address{
			Name:    info.Name,
			Street1: info.Street1,
			Street2: info.Street2,
			City:    info.City,
			State:   info.State,
			Country: info.Country,
			Zip:     info.Zip,
			Phone:   info.Phone,
		})
		if err != nil {
			return ShippoValidationErrMsg{Err: err}
		}

		return ShippoValidatedMsg{
			Address: ShippingFormCompleteMsg{
				Name:    validated.Name,
				Street1: validated.Street1,
				Street2: validated.Street2,
				City:    validated.City,
				State:   validated.State,
				Country: validated.Country,
				Zip:     validated.Zip,
				Phone:   validated.Phone,
			},
		}
	}
}

type StripeTokenMsg struct {
	TokenID string
	Last4   string
}

type StripeTokenErrMsg struct {
	Err error
}

type CheckoutResultMsg struct {
	OrderID uint
	Total   int
	Err     error
}

// OrdersMsg is sent when order history is fetched from the API
type OrdersMsg struct {
	Orders []models.Order
	Err    error
}

func (m Model) fetchOrdersCmd() tea.Cmd {
	return func() tea.Msg {
		if m.APIClient == nil || m.User == nil {
			return OrdersMsg{Err: fmt.Errorf("not authenticated")}
		}
		orders, err := m.APIClient.GetOrders(m.User.SSHKeyFingerprint)
		return OrdersMsg{Orders: orders, Err: err}
	}
}

func (m Model) tokenizeCard(card PaymentFormCompleteMsg) tea.Cmd {
	return func() tea.Msg {
		// Use Stripe's HTTP API directly with the publishable key.
		// The Go SDK's token.New() is blocked for raw card numbers
		// unless your account has direct PCI compliance enabled.
		// This HTTP POST mimics what a browser does with Stripe.js.
		pubKey := m.StripeKey

		data := fmt.Sprintf(
			"card[number]=%s&card[exp_month]=%s&card[exp_year]=%s&card[cvc]=%s",
			card.CardNumber, card.ExpiryMonth, card.ExpiryYear, card.CVC,
		)

		req, err := http.NewRequest("POST", "https://api.stripe.com/v1/tokens", strings.NewReader(data))
		if err != nil {
			return StripeTokenErrMsg{Err: err}
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Bearer "+pubKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return StripeTokenErrMsg{Err: err}
		}
		defer resp.Body.Close()

		var result struct {
			ID   string `json:"id"`
			Card struct {
				Last4 string `json:"last4"`
				Brand string `json:"brand"`
			} `json:"card"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return StripeTokenErrMsg{Err: err}
		}

		if result.Error != nil {
			return StripeTokenErrMsg{Err: fmt.Errorf("%s", result.Error.Message)}
		}

		return StripeTokenMsg{
			TokenID: result.ID,
			Last4:   result.Card.Last4,
		}
	}
}

func (m Model) submitCheckout(tok StripeTokenMsg) tea.Cmd {
	return func() tea.Msg {
		if m.APIClient == nil {
			return CheckoutResultMsg{Err: fmt.Errorf("API client not available")}
		}

		items := m.GetCartItemsSlice()
		cartItems := make([]api.CheckoutCartItem, 0, len(items))
		for _, item := range items {
			cartItems = append(cartItems, api.CheckoutCartItem{
				CoffeeID: item.Coffee.ID,
				Quantity: item.Quantity,
			})
		}

		req := api.CheckoutRequest{
			StripeToken:     tok.TokenID,
			Last4:           tok.Last4,
			Items:           cartItems,
			ShippingName:    m.ShippingInfo.Name,
			ShippingStreet:  m.ShippingInfo.Street1,
			ShippingStreet2: m.ShippingInfo.Street2,
			ShippingCity:    m.ShippingInfo.City,
			ShippingState:   m.ShippingInfo.State,
			ShippingZip:     m.ShippingInfo.Zip,
			ShippingCountry: m.ShippingInfo.Country,
			ShippingPhone:   m.ShippingInfo.Phone,
		}

		if m.User != nil {
			req.Fingerprint = m.User.SSHKeyFingerprint
		}

		order, err := m.APIClient.Checkout(req)
		if err != nil {
			return CheckoutResultMsg{Err: err}
		}

		return CheckoutResultMsg{
			OrderID: order.ID,
			Total:   order.Total,
		}
	}
}

// GetCartItemsSlice converts the cart map to a sorted slice for consistent iteration
func (m Model) GetCartItemsSlice() []*models.CartItem {
	// Get keys and sort them for stable ordering
	keys := make([]int, 0, len(m.Cart))
	for key := range m.Cart {
		keys = append(keys, key)
	}

	// Sort keys to ensure consistent order
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	// Build items slice in sorted order
	items := make([]*models.CartItem, 0, len(m.Cart))
	for _, key := range keys {
		items = append(items, m.Cart[key])
	}
	return items
}

// NewModel creates a new model with default coffee options
func NewModel(username string) Model {
	return Model{
		Username: username,
		Coffees: []models.Coffee{
			{
				Name:        "Espresso",
				RoastType:   "Dark Roast",
				Ounces:      2,
				BeanType:    "Arabica",
				Price:       3.50,
				Color:       "#8B4513",
				Description: "A bold, concentrated shot of pure coffee bliss. Perfect for those who need an immediate caffeine injection to survive the day.",
			},
			{
				Name:        "Latte",
				RoastType:   "Medium Roast",
				Ounces:      12,
				BeanType:    "Arabica Blend",
				Price:       5.00,
				Color:       "#D2691E",
				Description: "Smooth espresso paired with steamed milk and a light layer of foam. For when you want coffee but also want to feel fancy about it.",
			},
			{
				Name:        "Cappuccino",
				RoastType:   "Medium Roast",
				Ounces:      8,
				BeanType:    "Italian Blend",
				Price:       4.50,
				Color:       "#CD853F",
				Description: "Equal parts espresso, steamed milk, and foam. The classic Italian choice for people who know what they're doing.",
			},
			{
				Name:        "Americano",
				RoastType:   "Dark Roast",
				Ounces:      16,
				BeanType:    "Arabica",
				Price:       4.00,
				Color:       "#A0522D",
				Description: "Espresso with hot water. Simple, strong, and no-nonsense. This is coffee for people who actually like the taste of coffee.",
			},
			{
				Name:        "Mocha",
				RoastType:   "Medium Roast",
				Ounces:      16,
				BeanType:    "Colombian",
				Price:       5.50,
				Color:       "#4682B4",
				Description: "Some really good Mocha",
			},
			{
				Name:        "Macchiato",
				RoastType:   "Dark Roast",
				Ounces:      3,
				BeanType:    "Robusta Blend",
				Price:       4.25,
				Color:       "#DAA520",
				Description: "Espresso 'marked' with a dollop of foamed milk. Small but mighty, like a tiny caffeinated warrior.",
			},
		},
		Cursor:         0,
		Cart:           make(map[int]*models.CartItem),
		CartCursor:     0,
		AccountCursor:  0,
		WindowWidth:    120,
		WindowHeight:   30,
		ViewingCart:    false,
		ViewingAccount: false,
		Loading:        true,
		APIClient:      api.NewClient("http://localhost:8080"),
		StripeKey:      os.Getenv("STRIPE_PUBLIC_KEY"),
		ShippoKey:      os.Getenv("SHIPPO_API_KEY"),
	}
}

// NewModelWithAuth creates a new model with user authentication context
func NewModelWithAuth(user *models.User, isNewUser bool, pubKey gossh.PublicKey) Model {
	username := "guest"
	if user != nil {
		username = user.Name
	}

	m := NewModel(username)
	m.User = user
	m.IsNewUser = isNewUser
	m.SSHPublicKey = pubKey

	return m
}
