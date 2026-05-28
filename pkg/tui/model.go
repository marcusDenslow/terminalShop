package tui

import (
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/stripe/stripe-go/v78"
	stripetoken "github.com/stripe/stripe-go/v78/token"

	"terminalShop/pkg/api"
	"terminalShop/pkg/models"
	"terminalShop/pkg/tui/theme"
)

// termSize represents the terminal size category for responsive layout.
type termSize int

const (
	undersized termSize = iota // viewportWidth < 20 || viewportHeight < 10
	small                      // viewportWidth < 50
	medium                     // viewportWidth < 80
	large                      // viewportWidth >= 80
)

// page represents the currently active TUI screen
type page int

const (
	shopPage page = iota
	cartPage
	shippingPage
	paymentPage
	reviewPage
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

type footerCommand struct {
	key   string
	value string
}

// VisibleError is a typed error message displayed to the user as a banner.
// Using a struct instead of a plain string lets the Update loop catch raw
// errors and convert them, and lets ESC dismiss the banner.
type VisibleError struct {
	message string
}

// VisibleNotice is a typed success message displayed to the user as a banner.
type VisibleNotice struct {
	message string
}

type Model struct {
	// User authentication
	User               *models.User
	Fingerprint        string
	AccessToken        string
	AuthFingerprintKey string // shared secret for /auth/token refresh
	StripePublicKey    string

	// One-shop auth command - capture pubKeyStr in closure, not stored on Model
	authCmd tea.Cmd

	// Shared data
	Username          string
	Coffees           []models.Coffee
	Cart              map[uint]*models.CartItem
	SavedAddresses    []models.Address
	SavedCards        []models.Card
	Orders            []models.Order
	OrdersLoaded      bool
	ordersPollStarted bool
	FAQs              []FAQ

	// Page navigation
	currentPage page // currently active screen
	switched    bool // true when the page just changed (for state resets)

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

	Loading   bool // true when fetching data from API
	error     *VisibleError
	notice    *VisibleNotice
	APIClient *api.Client

	// Rendering
	theme theme.Theme

	// Checkout flow state (spans multiple pages)
	CheckingOut      bool
	ShippingInfo     *models.Address
	SelectedCard     *models.Card
	lastCartUpdateID int64

	// Per-page State
	splash   splashState
	shop     shopState
	cart     cartState
	shipping shippingState
	payment  paymentState
	account  accountState
	refund   refundState
	review   reviewState
	confirm  confirmState

	// Footer
	footer []footerCommand
}

type shopState struct {
	selected       int
	menuViewport   viewport.Model
	detailViewport viewport.Model
	viewportsReady bool
}

type splashState struct {
	dataReady bool
	delayDone bool
	cursor    bool
}

type cartState struct {
	cursor        int
	viewport      viewport.Model
	viewportReady bool
}

type shippingState struct {
	form          *ShippingFormState
	view          int
	addressCursor int
	viewport      viewport.Model
	viewportReady bool
}

type paymentState struct {
	form             *PaymentFormState
	view             int
	cardCursor       int
	collectURL       *string
	collectCardCount int
	collectBaseline  time.Time
	collectDeadline  time.Time
	collectTimeOut   bool
	viewport         viewport.Model
	viewportReady    bool
}

type accountState struct {
	cursor             int
	orderViewState     int
	orderCursor        int
	orderYOffset       int // saved scroll pos when entering order detail
	faqFocused         bool
	addressListFocused bool
	cardListFocused    bool
	addressCursor      int
	cardCursor         int
	addressDeleting    *int
	cardDeleting       *int
	detailViewport     viewport.Model
	viewportReady      bool
}

type reviewState struct {
	cardJustAdded    bool
	cardWasDuplicate bool
	success          bool

	// 3DS / SCA flow: when the backend returns 202 requires_action,
	// requiresAction is true and redirectURL is the Stripe-hosted challenge
	// page. The TUI shows a QR and polls GetOrderStatus(orderID) until the
	// order flips to paid or failed.
	requiresAction bool
	redirectURL    string
	orderID        uint
	authFailed     bool
}

type confirmState struct {
	total         int
	items         []models.CartItem
	shipping      *models.Address
	viewport      viewport.Model
	viewportReady bool
}

var modifiedKeyMap = viewport.KeyMap{
	PageDown:     key.NewBinding(key.WithKeys("pgdown")),
	PageUp:       key.NewBinding(key.WithKeys("pgup")),
	HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u")),
	HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d")),
}

// SwitchPage changed the active page and marks that a switch happened
func (m Model) SwitchPage(p page) Model {
	m.currentPage = p
	m.switched = true
	m.error = nil
	m.notice = nil
	return m
}

func (m Model) ShopSwitch() (Model, tea.Cmd) {
	m = m.SwitchPage(shopPage)
	m = m.updateShopViewports()
	m.footer = []footerCommand{
		{key: "j/k", value: "products"},
		{key: "+/-", value: "qty"},
		{key: "c", value: "cart"},
		{key: "a", value: "account"},
		{key: "?", value: "help"},
		{key: "q", value: "quit"},
	}
	return m, nil
}

func (m Model) CartSwitch() (Model, tea.Cmd) {
	m = m.SwitchPage(cartPage)
	m = m.updateCartViewport()
	if m.IsCartEmpty() {
		m.footer = []footerCommand{
			{key: "j/k", value: "items"},
			{key: "+/-", value: "qty"},
			{key: "s", value: "shop"},
			{key: "a", value: "account"},
			{key: "?", value: "help"},
			{key: "q", value: "quit"},
		}
	} else {
		m.footer = []footerCommand{
			{key: "j/k", value: "items"},
			{key: "+/-", value: "qty"},
			{key: "p/enter", value: "checkout"},
			{key: "s", value: "shop"},
			{key: "a", value: "account"},
			{key: "pgup/pgdn", value: "scroll"},
			{key: "?", value: "help"},
			{key: "q", value: "quit"},
		}
	}
	return m, nil
}

func (m Model) AccountSwitch() (Model, tea.Cmd) {
	m = m.SwitchPage(accountPage)
	m.account = accountState{}
	m.footer = []footerCommand{
		{key: "j/k", value: "navigate"},
		{key: "enter", value: "select"},
		{key: "s", value: "shop"},
		{key: "?", value: "help"},
		{key: "q", value: "quit"},
	}
	m = m.updateAccountViewport()
	m.account.detailViewport.GotoTop()
	if !m.OrdersLoaded {
		return m, m.fetchOrdersCmd()
	}
	return m, nil
}

func (m Model) ShippingSwitch() (Model, tea.Cmd) {
	m = m.SwitchPage(shippingPage)
	m.shipping = shippingState{}
	m = m.updateShippingViewport()
	m.footer = []footerCommand{
		{key: "j/k", value: "addresses"},
		{key: "enter", value: "select"},
		{key: "d/x", value: "delete"},
		{key: "esc", value: "back"},
	}
	return m, m.fetchAddressesCmd()
}

func (m Model) PaymentSwitch() (Model, tea.Cmd) {
	m = m.SwitchPage(paymentPage)
	m.payment = paymentState{}
	m.SelectedCard = nil
	m.footer = []footerCommand{
		{key: "j/k", value: "cards"},
		{key: "enter", value: "select"},
		{key: "d/x", value: "delete"},
		{key: "esc", value: "back"},
	}
	return m, m.fetchCardsCmd()
}

func (m Model) ReviewSwitch() (Model, tea.Cmd) {
	m = m.SwitchPage(reviewPage)
	m.footer = []footerCommand{
		{key: "enter", value: "confirm"},
		{key: "esc", value: "back"},
	}
	return m, nil
}

func (m Model) ConfirmSwitch() (Model, tea.Cmd) {
	m = m.SwitchPage(confirmPage)
	m.confirm = confirmState{}
	m = m.updateConfirmViewport()
	m.footer = []footerCommand{
		{key: "esc", value: "back to shop"},
		{key: "q", value: "quit"},
	}
	return m, nil
}

// inCartFlow returns true when the user is anywhere in the cart or checkout flow
func (m Model) inCartFlow() bool {
	return m.currentPage == cartPage ||
		m.currentPage == shippingPage ||
		m.currentPage == paymentPage ||
		m.currentPage == reviewPage
}

func (m Model) checkoutStep() int {
	switch m.currentPage {
	case shippingPage:
		return 1
	case paymentPage:
		return 2
	case reviewPage:
		return 3
	default:
		return 0
	}
}

// updateLayout recalculates all layout variables from the raw viewport dimensions.
// This should be called whenever the terminal is resized.
func (m Model) updateLayout(width, height int) Model {
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
	return m
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

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.authCmd,
		tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
			return DelayCompleteMsg{}
		}),
		tea.Tick(700*time.Millisecond, func(t time.Time) tea.Msg {
			return splashCursorTickMsg{}
		}),
	)
}

type CheckoutResultMsg struct {
	OrderID        uint
	Total          int
	RequiresAction bool
	RedirectURL    string
	Err            error
}

// OrderStatusMsg carries the result of a poll against /orders/{id}/status.
// Emitted on the 2s tick while waiting for 3DS authentication to complete.
type OrderStatusMsg struct {
	OrderID uint
	Status  string
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
		if m.APIClient == nil || m.Fingerprint == "" {
			return TokenRefreshedMsg{Err: fmt.Errorf("not authenticated, cannot refresh token")}
		}

		token, err := m.APIClient.RefreshToken(m.Fingerprint, m.AuthFingerprintKey)
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

// resolveCartAddress sets the shipping address on the cart, creating it first
// if the address had no ID yet (i.e the user typed it in fresh rather than picking
// from saved addresses)
func (m Model) resolveCartAddress() error {
	if m.ShippingInfo == nil {
		return nil
	}
	if m.ShippingInfo.ID != 0 {
		if err := m.APIClient.SetCartAddress(m.ShippingInfo.ID); err != nil {
			return fmt.Errorf("failed to set address: %w", err)
		}
		return nil
	}
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
		return fmt.Errorf("failed to save address: %w", err)
	}
	if err := m.APIClient.SetCartAddress(saved.ID); err != nil {
		return fmt.Errorf("failed to set address: %w", err)
	}
	return nil
}

// checkoutWithSavedCard sets address and saved card on the cart, then converts.
// Used when the user selects an existing card from the list (no tokenization needed).
func (m Model) checkoutWithSavedCard() tea.Cmd {
	return func() tea.Msg {
		if m.APIClient == nil || m.SelectedCard == nil {
			return CheckoutResultMsg{Err: fmt.Errorf("API client or card not available")}
		}

		if err := m.resolveCartAddress(); err != nil {
			return CheckoutResultMsg{Err: err}
		}

		// 2. Set the saved card on the cart
		if err := m.APIClient.SetCartCard(m.SelectedCard.ID); err != nil {
			return CheckoutResultMsg{Err: fmt.Errorf("failed to set card: %w", err)}
		}

		// 3. Convert the cart to an order
		outcome, err := m.APIClient.ConvertCart()
		if err != nil {
			return CheckoutResultMsg{Err: err}
		}

		if outcome.RequiresAction {
			return CheckoutResultMsg{
				OrderID:        outcome.OrderID,
				RequiresAction: true,
				RedirectURL:    outcome.RedirectURL,
			}
		}

		return CheckoutResultMsg{OrderID: outcome.Order.ID, Total: outcome.Order.Total}
	}
}

// schedulePollOrderStatus sleeps 2s then polls /orders/{id}/status. Used while
// waiting for the customer to complete 3DS authentication.
func (m Model) schedulePollOrderStatus(orderID uint) tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		if m.APIClient == nil {
			return OrderStatusMsg{OrderID: orderID, Err: fmt.Errorf("api client unavailable")}
		}
		status, err := m.APIClient.GetOrderStatus(orderID)
		if err != nil {
			return OrderStatusMsg{OrderID: orderID, Err: err}
		}
		return OrderStatusMsg{OrderID: orderID, Status: status}
	})
}

func (m Model) syncCartItemCmd(coffeeID uint, quantity int) (Model, tea.Cmd) {
	if m.APIClient == nil || m.User == nil {
		return m, nil
	}

	updateID := time.Now().UTC().UnixMilli()
	m.lastCartUpdateID = updateID

	return m, func() tea.Msg {
		cart, err := m.APIClient.SetCartItem(coffeeID, quantity)
		return CartSyncedMsg{
			UpdateID: updateID,
			Cart:     cart,
			Err:      err,
		}
	}
}

func (m Model) saveCardOnlyCmd(form PaymentFormCompleteMsg) tea.Cmd {
	return func() tea.Msg {
		if m.APIClient == nil {
			return CardSavedForReviewMsg{Err: fmt.Errorf("API client not available")}
		}
		stripe.Key = m.StripePublicKey
		tok, err := stripetoken.New(&stripe.TokenParams{
			Card: &stripe.CardParams{
				Name:       stripe.String(form.CardName),
				Number:     stripe.String(form.CardNumber),
				ExpMonth:   stripe.String(form.ExpiryMonth),
				ExpYear:    stripe.String(form.ExpiryYear),
				CVC:        stripe.String(form.CVC),
				AddressZip: stripe.String(form.BillingZip),
			},
		})
		if err != nil {
			if stripeErr, ok := err.(*stripe.Error); ok {
				return CardSavedForReviewMsg{Err: fmt.Errorf("%s", stripeErr.Msg)}
			}
			return CardSavedForReviewMsg{Err: fmt.Errorf("failed to tokenize card: %w", err)}
		}
		card, err := m.APIClient.SaveCard(api.SaveCardRequest{Token: tok.ID})
		if err != nil {
			return CardSavedForReviewMsg{Err: fmt.Errorf("failed to save card: %w", err)}
		}
		return CardSavedForReviewMsg{Card: *card}
	}
}

// loadCartFromAPI populates the in-memory cart map from an API CartData response
// It matches cart items to local Coffees by ID so the Coffee struct is populated
func (m Model) loadCartFromAPI(cartData *api.CartData) Model {
	m.Cart = make(map[uint]*models.CartItem)
	if cartData == nil {
		return m
	}

	for _, item := range cartData.Items {
		m.Cart[item.CoffeeID] = &models.CartItem{
			CoffeeID: item.CoffeeID,
			Coffee:   item.Coffee,
			Quantity: item.Quantity,
		}
	}

	if m.cart.cursor >= len(m.Cart) {
		m.cart.cursor = 0
	}
	return m
}

// GetCartItemsSlice converts the cart map to a sorted slice for consistent iteration.
func (m Model) GetCartItemsSlice() []*models.CartItem {
	// Get keys and sort them for stable ordering
	keys := make([]uint, 0, len(m.Cart))
	for cartKey := range m.Cart {
		keys = append(keys, cartKey)
	}

	// Sort keys to ensure consistent order
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	// Build items slice in sorted order
	items := make([]*models.CartItem, 0, len(m.Cart))
	for _, cartKey := range keys {
		items = append(items, m.Cart[cartKey])
	}
	return items
}

// IsCartEmpty returns true when the cart has no items
func (m Model) IsCartEmpty() bool {
	return len(m.Cart) == 0
}

// CartItemCount returns the total number of items (sum of quanities) in the cart
func (m Model) CartItemCount() int {
	count := 0
	for _, item := range m.Cart {
		count += item.Quantity
	}
	return count
}

// CalculateSubtotal returns the cart subtotal in cents
func (m Model) CalculateSubtotal() int {
	subtotal := 0
	for _, item := range m.Cart {
		subtotal += item.Coffee.Price * item.Quantity
	}
	return subtotal
}

func (m Model) SavedAddressesIsEmpty() bool {
	return len(m.SavedAddresses) == 0
}

func (m Model) SavedCardsIsEmpty() bool {
	return len(m.SavedCards) == 0
}

// NewModel creates a new model.
func NewModel(username string) Model {
	m := Model{
		Username:    username,
		currentPage: splashPage,
		theme:       theme.BasicTheme(),
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
		Cart:      make(map[uint]*models.CartItem),
		Loading:   true,
		APIClient: api.NewClient("http://localhost:8080", ""),
		FAQs:      LoadFaqs(),
	}
	m = m.updateLayout(120, 30)
	return m
}

// NewModelWithAuth creates a new model with user authentication context.
func NewModelWithAuth(fingerprint string, pubKeyStr string, apiURL string, clientSecret string, stripePublicKey string) Model {
	m := NewModel("")
	m.Fingerprint = fingerprint
	m.AuthFingerprintKey = clientSecret
	m.StripePublicKey = stripePublicKey
	m.currentPage = splashPage
	if apiURL != "" {
		m.APIClient = api.NewClient(apiURL, "")
	} else {
		m.APIClient = api.NewClient("http://localhost:8000", "")
	}

	m.authCmd = func() tea.Msg {
		token, user, err := m.APIClient.GetOrCreateToken(
			fingerprint,
			pubKeyStr,
			clientSecret,
		)
		if err != nil {
			return SplashAuthMsg{Err: fmt.Errorf("auth failed: %w", err)}
		}
		return SplashAuthMsg{Token: token, User: user}
	}
	return m
}

func newViewport(w, h int) viewport.Model {
	return viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
}

func (m Model) collectCardCmd() tea.Cmd {
	return func() tea.Msg {
		if m.APIClient == nil {
			return CheckoutResultMsg{Err: fmt.Errorf("API client not available")}
		}
		url, err := m.APIClient.CollectCard()
		if err != nil {
			return CheckoutResultMsg{Err: fmt.Errorf("failed to generate payment link: %w", err)}
		}
		return PollPaymentInitMsg{URL: url}
	}
}

const (
	ordersPollInterval = 10 * time.Second
	cardPollTimeout    = 10 * time.Minute
)

type OrdersPollTickMsg struct{}

func (m Model) pollOrderCmd() tea.Cmd {
	return tea.Tick(ordersPollInterval, func(_ time.Time) tea.Msg {
		return OrdersPollTickMsg{}
	})
}

func (m Model) pollCardsCmd(baseline time.Time, baselineCount int, deadline time.Time) tea.Cmd {
	return tea.Tick(time.Second, func(now time.Time) tea.Msg {
		if now.After(deadline) {
			return PollPaymentTimeoutMsg{}
		}
		if m.APIClient == nil {
			return PollPaymentStatusMsg{Baseline: baseline, BaselineCount: baselineCount, Deadline: deadline}
		}
		cards, err := m.APIClient.GetCards()
		if err != nil {
			return PollPaymentStatusMsg{Baseline: baseline, BaselineCount: baselineCount, Deadline: deadline}
		}
		var newest time.Time
		for _, card := range cards {
			if card.UpdatedAt.After(newest) {
				newest = card.UpdatedAt
			}
		}
		if newest.After(baseline) {
			return PollPaymentCompleteMsg{Cards: cards, Duplicate: len(cards) == baselineCount}
		}
		return PollPaymentStatusMsg{Baseline: baseline, BaselineCount: baselineCount, Deadline: deadline}
	})
}
