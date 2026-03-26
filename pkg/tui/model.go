package tui

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	gossh "golang.org/x/crypto/ssh"

	"terminalShop/pkg/api"
	"terminalShop/pkg/auth"
	"terminalShop/pkg/models"
)

// termSize represents the terminal size category for responsive layout.
type termSize int

const (
	undersized termSize = iota // viewportWidth < 20 || viewportHeight < 10
	small                      // viewportWidth < 50
	medium                     // viewportWidth < 80
	large                      // viewportWidth >= 80
)

// page represetnts the currently active TUI screen
type page int

const (
	shopPage page = iota
	cartPage
	shippingPage
	paymentPage
	confirmPage
	accountPage
	splashPage
)

// This msg is sent after a debounce delay to apply a pending resize.
// the seq field makes sure that only the tick from the most recent resize is applied.
type resizeTickMsg struct {
	seq int
}

// Delay before applying a terminal resize.
// this prevents flickering when resizing the terminal
const resizeDebounce = 50 * time.Millisecond

type Model struct {
	// User authentication
	User               *models.User    // Authenticated user (nil if not logged in)
	IsNewUser          bool            // True if user needs to register
	SSHPublicKey       gossh.PublicKey // SSH public key for registration
	AccessToken        string
	AuthFingerprintKey string // shared secret for /auth/token refresh
	UsernameInput      string // Input for username during registration

	// Shop state
	Username         string
	Coffees          []models.Coffee
	Cursor           int
	Cart             map[uint]*models.CartItem // maps CoffeeID to cart item
	CartCursor       int                       // cursor position in cart view
	AccountCursor    int
	currentPage      page  // currently active screen
	switched         bool  // true when the page just changed (for state resets)
	ScrollOffset     int   // scroll position for content
	CheckingOut      bool  // true while saveCardAndConvert is running
	lastCartUpdateID int64 // debounce ID for optimistic cart sync

	// Menu modal state
	ShowingMenu  bool // true when full-screen menu is showing
	ShowingHelp  bool // true when help-screen is showing
	menuLastPage page // page we were on before opening menu

	// Layout fields — responsive container system
	viewportWidth   int      // raw terminal width
	viewportHeight  int      // raw terminal height
	widthContainer  int      // constrained outer container width (max 80)
	heightContainer int      // constrained outer container height (max 30)
	widthContent    int      // widthContainer - 2 (usable inner width)
	size            termSize // current terminal size category

	// Resize debouncing
	resizeSeq     int // incremented on each WindowSizeMsg
	pendingWidth  int // buffered width from latest resize
	pendingHeight int // buffered height from latest resize

	Loading   bool   // true when fetching data from API
	ErrorMsg  string // error message if API fetch fails
	APIClient *api.Client

	// Checkout state
	ShippingForm   *ShippingFormState // Shipping form state (nil when not in shipping step)
	PaymentForm    *PaymentFormState  // Payment form state (nil when not in payment step)
	ShippingInfo   *models.Address    // Selected shipping address for current order
	SavedAddresses []models.Address   // Saved addresses from database
	ShippingView   int
	AddressCursor  int
	StripeKey      string        // Stripe publishable key for client-side tokenization
	SavedCards     []models.Card // Saved cards from database
	SelectedCard   *models.Card  // Card selected from saved list (nil = new card)
	CardCursor     int           // cursor position in the card list
	PaymentView    int           // 0=card list, 1=new card form

	// Order history
	Orders         []models.Order
	OrdersLoaded   bool
	OrderCursor    int // which order is selected in the list
	OrderViewState int // 0=preview, 1=browsing list, 2=viewing detail

	// Static content
	FAQs       []FAQ // loaded from embedded faq.json
	FaqFocused bool  // true when FAQ detail is focused

	// Splash screen
	splashDataReady bool // true when products have loaded
	splashDelayDone bool // true when minimum display time has elapsed
	splashCursor    bool // toggles for blinking cursor animation

}

// SwitchPage changed the active page and marks that a switch happened
func (m Model) SwitchPage(p page) Model {
	m.currentPage = p
	m.switched = true
	return m
}

// inCartFlow returns true when the user is anywhere in the cart or checkout flow
func (m Model) inCartFlow() bool {
	return m.currentPage == cartPage ||
		m.currentPage == shippingPage ||
		m.currentPage == paymentPage ||
		m.currentPage == confirmPage
}

func (m Model) checkoutStep() int {
	switch m.currentPage {
	case shippingPage:
		return 1
	case paymentPage:
		return 2
	case confirmPage:
		return 3
	default:
		return 0
	}
}

// updateLayout recalculates all layout variables from the raw viewport dimensions.
// This should be called whenever the terminal is resized.
func (m *Model) updateLayout(width, height int) {
	m.viewportWidth = width
	m.viewportHeight = height

	switch {
	case width < 20 || height < 10:
		m.size = undersized
		m.widthContainer = width
		m.heightContainer = height
	case width < 50:
		m.size = small
		m.widthContainer = width
		m.heightContainer = height
	case width < 80:
		m.size = medium
		m.widthContainer = 50
		m.heightContainer = int(math.Min(float64(height), 30))
	default:
		m.size = large
		m.widthContainer = 80
		m.heightContainer = int(math.Min(float64(height), 30))
	}

	m.widthContent = m.widthContainer - 2
}

// ProductsMsg is sent when products are fetched from API
type ProductsMsg struct {
	Products []models.Coffee
	Err      error
}

// fetchCartCmd fetches the user's cart from the API on startup
func (m Model) fetchCartCmd() tea.Msg {
	if m.APIClient == nil || m.User == nil {
		return CartFetchedMsg{Err: fmt.Errorf("not authenticated")}
	}

	log.Println("[TUI] Fetching cart from API")
	cart, err := m.APIClient.GetCart()
	if err != nil {
		log.Printf("[TUI] Failed to fetch cart: %v", err)
	} else {
		log.Printf("[TUI] Loaded %d cart items from API", len(cart.Items))
	}

	return CartFetchedMsg{Cart: cart, Err: err}
}

// fetchProductsCmd fetches products from the API
func (m Model) fetchProductsCmd() tea.Msg {
	if m.APIClient == nil {
		log.Println("[TUI] APIClient is nil, using fallback products")
		return ProductsMsg{Err: nil} // No API client, use fallback
	}

	log.Printf("[TUI] Fetching products from API at %s", m.APIClient.BaseURL)
	products, err := m.APIClient.GetProducts()
	if err != nil {
		log.Printf("[TUI] Failed to fetch products: %v", err)
	} else {
		log.Printf("[TUI] Loaded %d products from API", len(products))
	}
	return ProductsMsg{Products: products, Err: err}
}

func (m Model) Init() tea.Cmd {
	// Fetch products and cart from API on startup, and schedule token refresh
	cmds := []tea.Cmd {
		m.fetchProductsCmd,
		m.fetchCartCmd,
		tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
			return DelayCompleteMsg{}
		}),
		tea.Tick(700*time.Microsecond, func(t time.Time) tea.Msg {
			return splashCursorTickMsg{}
		}),
	}
	if m.User != nil && m.AuthFingerprintKey != "" {
		cmds = append(cmds, m.scheduleTokenRefreshCmd())
	}
	return tea.Batch(cmds...)
}

func (m Model) resetPageState() Model {
	m.ShippingForm = nil
	m.PaymentForm = nil
	m.ScrollOffset = 0
	m.OrderViewState = 0
	m.FaqFocused = false
	return m
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

// CartSyncedMsg is recieved when the server responds to a cart item update
type CartSyncedMsg struct {
	UpdateID int64
	Cart     *api.CartData
	Err      error
}

type CartFetchedMsg struct {
	Cart *api.CartData
	Err  error
}

// tokenRefreshDuration is the interval between token refreshes.
// Tokens expire after 30 minutes; refresh 5 minutes early.
const tokenRefreshDuration = 25 * time.Minute

// tokenRetryDuration is the delay before retrying a failed token refresh.
const tokenRetryDuration = 1 * time.Minute

// tokenRefreshTickMsg is sent by the background timer to trigger a token refresh.
type tokenRefreshTickMsg struct{}

// TokenRefreshedMsg is sent when a token refresh attempt completes.
type TokenRefreshedMsg struct {
	Token string
	Err   error
}

// scheduleTokenRefreshCmd returns a tea.Cmd that ticks after tokenRefreshDuration.
func (m Model) scheduleTokenRefreshCmd() tea.Cmd {
	return tea.Tick(tokenRefreshDuration, func(t time.Time) tea.Msg {
		return tokenRefreshTickMsg{}
	})
}

// refreshTokenCmd calls the /auth/token endpoint to get a fresh JWT.
func (m Model) refreshTokenCmd() tea.Cmd {
	return func() tea.Msg {
		if m.APIClient == nil || m.User == nil || m.SSHPublicKey == nil {
			return TokenRefreshedMsg{Err: fmt.Errorf("not authenticated, cannot refresh token")}
		}

		fingerprint := auth.GetSSHKeyFingerprint(m.SSHPublicKey)
		token, err := m.APIClient.RefreshToken(fingerprint, m.AuthFingerprintKey)
		if err != nil {
			log.Printf("[TUI] Token refresh failed: %v", err)
			return TokenRefreshedMsg{Err: err}
		}

		log.Println("[TUI] Token refreshed successfully")
		return TokenRefreshedMsg{Token: token}
	}
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
		orders, err := m.APIClient.GetOrders()
		return OrdersMsg{Orders: orders, Err: err}
	}
}

// AddressesMsg is sent when sdaved addresses are fetched from the API
type AddressesMsg struct {
	Addresses []models.Address
	Err       error
}

// AddressSavedMsg is sent when an address is saved to the api
type AddressSavedMsg struct {
	Address models.Address
	Err     error
}

type AddressDeletedMsg struct {
	Err error
}

// CardsMsg is sent when saved cards are fetched form the API
type CardsMsg struct {
	Cards []models.Card
	Err   error
}

// CardDeletedMsg is sent when card is deleted
type CardDeletedMsg struct {
	Err error
}

func (m Model) fetchAddressesCmd() tea.Cmd {
	return func() tea.Msg {
		if m.APIClient == nil || m.User == nil {
			return AddressesMsg{Err: fmt.Errorf("not authenticated")}
		}
		addresses, err := m.APIClient.GetAddresses()
		return AddressesMsg{Addresses: addresses, Err: err}
	}
}

func (m Model) saveAddressCmd(addr models.Address) tea.Cmd {
	return func() tea.Msg {
		if m.APIClient == nil || m.User == nil {
			return AddressSavedMsg{Err: fmt.Errorf("not authenticated")}
		}
		saved, err := m.APIClient.CreateAddress(api.CreateAddressRequest{
			Name:    addr.Name,
			Street:  addr.Street,
			Street2: addr.Street2,
			City:    addr.City,
			State:   addr.State,
			Zip:     addr.Zip,
			Country: addr.Country,
			Phone:   addr.Phone,
		})
		if err != nil {
			return AddressSavedMsg{Err: err}
		}
		return AddressSavedMsg{Address: *saved}
	}
}

func (m Model) deleteAddressCmd(id uint) tea.Cmd {
	return func() tea.Msg {
		if m.APIClient == nil || m.User == nil {
			return AddressDeletedMsg{Err: fmt.Errorf("not authenticated")}
		}
		err := m.APIClient.DeleteAddress(id)
		return AddressDeletedMsg{Err: err}
	}
}

func (m Model) fetchCardsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.APIClient == nil || m.User == nil {
			return CardsMsg{Err: fmt.Errorf("not authenticated")}
		}
		cards, err := m.APIClient.GetCards()
		return CardsMsg{Cards: cards, Err: err}
	}
}

func (m Model) deleteCardCmd(id uint) tea.Cmd {
	return func() tea.Msg {
		if m.APIClient == nil || m.User == nil {
			return CardDeletedMsg{Err: fmt.Errorf("not authenticated")}
		}
		err := m.APIClient.DeleteCard(id)
		return CardDeletedMsg{Err: err}
	}
}

// checkoutWithSavedCard sets address and saved card on the cart, then converts.
// Used when the user selects an existing card from the list (no tokenization needed).
func (m Model) checkoutWithSavedCard() tea.Cmd {
	return func() tea.Msg {
		if m.APIClient == nil || m.SelectedCard == nil {
			return CheckoutResultMsg{Err: fmt.Errorf("API client or card not available")}
		}

		// 1. Set the shipping address on the cart
		if m.ShippingInfo != nil && m.ShippingInfo.ID != 0 {
			if err := m.APIClient.SetCartAddress(m.ShippingInfo.ID); err != nil {
				return CheckoutResultMsg{Err: fmt.Errorf("failed to set address: %w", err)}
			}
		} else if m.ShippingInfo != nil {
			saved, err := m.APIClient.CreateAddress(api.CreateAddressRequest{
				Name:    m.ShippingInfo.Name,
				Street:  m.ShippingInfo.Street,
				Street2: m.ShippingInfo.Street2,
				City:    m.ShippingInfo.City,
				State:   m.ShippingInfo.State,
				Zip:     m.ShippingInfo.Zip,
				Country: m.ShippingInfo.Country,
				Phone:   m.ShippingInfo.Phone,
			})
			if err != nil {
				return CheckoutResultMsg{Err: fmt.Errorf("failed to save address: %w", err)}
			}
			if err := m.APIClient.SetCartAddress(saved.ID); err != nil {
				return CheckoutResultMsg{Err: fmt.Errorf("failed to set address: %w", err)}
			}
		}

		// 2. Set the saved card on the cart
		if err := m.APIClient.SetCartCard(m.SelectedCard.ID); err != nil {
			return CheckoutResultMsg{Err: fmt.Errorf("failed to set card: %w", err)}
		}

		// 3. Convert the cart to an order
		order, err := m.APIClient.ConvertCart()
		if err != nil {
			return CheckoutResultMsg{Err: err}
		}

		return CheckoutResultMsg{OrderID: order.ID, Total: order.Total}
	}
}

func (m Model) tokenizeCard(card PaymentFormCompleteMsg) tea.Cmd {
	return func() tea.Msg {
		// Use the Stripe secret key for server-side tokenization.
		// This is a terminal app running over SSH — there is no browser,
		// so the publishable key / Stripe.js approach does not work.
		// The secret key is allowed to create tokens with raw card data.
		secretKey := os.Getenv("STRIPE_SECRET_KEY")
		if secretKey == "" {
			// Fall back to the publishable key (will fail on most accounts)
			secretKey = m.StripeKey
		}

		data := fmt.Sprintf(
			"card[number]=%s&card[exp_month]=%s&card[exp_year]=%s&card[cvc]=%s",
			card.CardNumber, card.ExpiryMonth, card.ExpiryYear, card.CVC,
		)

		req, err := http.NewRequest("POST", "https://api.stripe.com/v1/tokens", strings.NewReader(data))
		if err != nil {
			return StripeTokenErrMsg{Err: err}
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Bearer "+secretKey)

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

func (m *Model) syncCartItemCmd(coffeeID uint, quantity int) tea.Cmd {
	if m.APIClient == nil || m.User == nil {
		return nil
	}

	updateID := time.Now().UTC().UnixMilli()
	m.lastCartUpdateID = updateID

	return func() tea.Msg {
		cart, err := m.APIClient.SetCartItem(coffeeID, quantity)
		return CartSyncedMsg{
			UpdateID: updateID,
			Cart:     cart,
			Err:      err,
		}
	}
}

// saveCardAndConvert saves the tokenized card, sets address and card on the
// server cart, then converts the cart to an order
func (m Model) saveCardAndConvert(tok StripeTokenMsg) tea.Cmd {
	return func() tea.Msg {
		if m.APIClient == nil {
			return CheckoutResultMsg{Err: fmt.Errorf("API client not available")}
		}

		// 1. Save the card
		card, err := m.APIClient.SaveCard(tok.TokenID)
		if err != nil {
			// Card may already exist — try fetching existing cards
			cards, listErr := m.APIClient.GetCards()
			if listErr != nil || len(cards) == 0 {
				return CheckoutResultMsg{Err: fmt.Errorf("failed to save card: %w", err)}
			}
			// Use the most recent card
			card = &cards[0]
		}

		// 2. Set the shipping address on the cart
		if m.ShippingInfo != nil && m.ShippingInfo.ID != 0 {
			if err := m.APIClient.SetCartAddress(m.ShippingInfo.ID); err != nil {
				return CheckoutResultMsg{Err: fmt.Errorf("failed to set address: %w", err)}
			}
		} else if m.ShippingInfo != nil {
			// Address was entered fresh (not from saved list), save it first
			saved, err := m.APIClient.CreateAddress(api.CreateAddressRequest{
				Name:    m.ShippingInfo.Name,
				Street:  m.ShippingInfo.Street,
				Street2: m.ShippingInfo.Street2,
				City:    m.ShippingInfo.City,
				State:   m.ShippingInfo.State,
				Zip:     m.ShippingInfo.Zip,
				Country: m.ShippingInfo.Country,
				Phone:   m.ShippingInfo.Phone,
			})
			if err != nil {
				return CheckoutResultMsg{Err: fmt.Errorf("failed to save address: %w", err)}
			}
			if err := m.APIClient.SetCartAddress(saved.ID); err != nil {
				return CheckoutResultMsg{Err: fmt.Errorf("failed to set address: %w", err)}
			}
		}

		// 3. Set the card on the cart
		if err := m.APIClient.SetCartCard(card.ID); err != nil {
			return CheckoutResultMsg{Err: fmt.Errorf("failed to set card: %w", err)}
		}

		// 4. Convert the cart to an order
		order, err := m.APIClient.ConvertCart()
		if err != nil {
			return CheckoutResultMsg{Err: err}
		}

		return CheckoutResultMsg{
			OrderID: order.ID,
			Total:   order.Total,
		}
	}
}

// loadCartFromAPI populates the in-memory cart map from an API CartData response
// It matches cart items to local Coffees by ID so the Coffee struct is populated
func (m *Model) loadCartFromAPI(cartData *api.CartData) {
	m.Cart = make(map[uint]*models.CartItem)
	if cartData == nil {
		return
	}

	for _, item := range cartData.Items {
		m.Cart[item.CoffeeID] = &models.CartItem{
			CoffeeID: item.CoffeeID,
			Coffee:   item.Coffee,
			Quantity: item.Quantity,
		}
	}

	if m.CartCursor >= len(m.Cart) {
		m.CartCursor = 0
	}
}

// GetCartItemsSlice converts the cart map to a sorted slice for consistent iteration.
func (m Model) GetCartItemsSlice() []*models.CartItem {
	// Get keys and sort them for stable ordering
	keys := make([]uint, 0, len(m.Cart))
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
	m := Model{
		Username: username,
		currentPage: splashPage,
		Coffees: []models.Coffee{
			{
				Name:        "Espresso",
				RoastType:   "Dark Roast",
				Ounces:      2,
				BeanType:    "Arabica",
				Price:       350, // $3.50
				Color:       "#8B4513",
				Description: "A bold, concentrated shot of pure coffee bliss. Perfect for those who need an immediate caffeine injection to survive the day.",
			},
			{
				Name:        "Latte",
				RoastType:   "Medium Roast",
				Ounces:      12,
				BeanType:    "Arabica Blend",
				Price:       500, // $5.00
				Color:       "#D2691E",
				Description: "Smooth espresso paired with steamed milk and a light layer of foam. For when you want coffee but also want to feel fancy about it.",
			},
			{
				Name:        "Cappuccino",
				RoastType:   "Medium Roast",
				Ounces:      8,
				BeanType:    "Italian Blend",
				Price:       450, // $4.50
				Color:       "#CD853F",
				Description: "Equal parts espresso, steamed milk, and foam. The classic Italian choice for people who know what they're doing.",
			},
			{
				Name:        "Americano",
				RoastType:   "Dark Roast",
				Ounces:      16,
				BeanType:    "Arabica",
				Price:       400, // $4.00
				Color:       "#A0522D",
				Description: "Espresso with hot water. Simple, strong, and no-nonsense. This is coffee for people who actually like the taste of coffee.",
			},
			{
				Name:        "Mocha",
				RoastType:   "Medium Roast",
				Ounces:      16,
				BeanType:    "Colombian",
				Price:       550, // $5.50
				Color:       "#4682B4",
				Description: "Some really good Mocha",
			},
			{
				Name:        "Macchiato",
				RoastType:   "Dark Roast",
				Ounces:      3,
				BeanType:    "Robusta Blend",
				Price:       425, // $4.25
				Color:       "#DAA520",
				Description: "Espresso 'marked' with a dollop of foamed milk. Small but mighty, like a tiny caffeinated warrior.",
			},
		},
		Cursor:         0,
		Cart:           make(map[uint]*models.CartItem),
		CartCursor:     0,
		AccountCursor:  0,
		OrderCursor:    0,
		OrderViewState: 0,
		Loading:        true,
		APIClient:      api.NewClient("http://localhost:8080", ""),
		StripeKey:      os.Getenv("STRIPE_PUBLIC_KEY"),
		FAQs:           LoadFaqs(),
	}
	m.updateLayout(120, 30)
	return m
}

// NewModelWithAuth creates a new model with user authentication context
func NewModelWithAuth(user *models.User, isNewUser bool, pubKey gossh.PublicKey, token string, apiURL string, authFingerprintKey string) Model {
	username := "guest"
	if user != nil {
		username = user.Name
	}

	m := NewModel(username)
	m.User = user
	m.IsNewUser = isNewUser
	m.SSHPublicKey = pubKey
	m.AccessToken = token
	m.AuthFingerprintKey = authFingerprintKey
	if apiURL != "" {
		m.APIClient = api.NewClient(apiURL, token)
	} else {
		m.APIClient = api.NewClient("http://localhost:8080", token)
	}

	return m
}
