# Custom Card Form with Stripe.js

## Why

The reference (terminal.shop) uses their own branded card input page with Stripe.js
instead of Stripe Hosted Checkout. This means:

- Full control over the UI/branding
- No webhooks needed for card saving (token POSTed directly to API)
- No Stripe Checkout Session required
- Cleaner flow, fewer moving parts

## How it would work

1. TUI calls `POST /api/v1/cards/collect` → gets `https://yourdomain.com/pay/TOKEN`
2. User opens URL → sees your own card form (HTML + Stripe.js)
3. Stripe.js tokenizes card in the browser (client-side, no raw card data APIs needed)
4. Page POSTs token + TOKEN to your API: `POST /api/v1/cards/collect/TOKEN`
5. API saves the card to the database
6. TUI polling detects new card and auto-proceeds

## What to build

### 1. HTML page served at `GET /pay/{token}`

Instead of redirecting to Stripe, serve an HTML page with Stripe Elements.
The TOKEN from the URL is used to associate the card with the right user.

```html
<!DOCTYPE html>
<html>
<head>
  <script src="https://js.stripe.com/v3/"></script>
</head>
<body>
  <form id="card-form">
    <div id="card-element"></div>
    <button type="submit">Save Card</button>
  </form>
  <script>
    const stripe = Stripe('pk_test_YOUR_PUBLISHABLE_KEY');
    const elements = stripe.elements();
    const card = elements.create('card');
    card.mount('#card-element');

    document.getElementById('card-form').addEventListener('submit', async (e) => {
      e.preventDefault();
      const { token } = await stripe.createToken(card);
      await fetch('/api/v1/cards/collect/TOKEN_FROM_URL', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token: token.id })
      });
      window.location.href = '/card-added';
    });
  </script>
</body>
</html>

```

### 2. New API endpoint: `POST /api/v1/cards/collect/{token}`

- Looks up the user from the token (stored in DB or memory map)
- Calls Stripe to convert token → PaymentMethod
- Attaches to customer
- Saves card to DB
- No webhook needed

### 3. Remove Stripe Checkout Session logic from CollectCard

`CollectCard` just generates a token, stores user association, returns the URL.
No `stripeSession.New()` call needed.

## Notes

- Requires a publicly accessible HTTPS domain (same as current approach)
- The in-memory token→userID map works fine (same TTL concerns as current approach)
- Could remove the `checkout.session.completed` webhook handler entirely
- Would need to remove the Stripe Checkout Session dependency from `cards.go`
