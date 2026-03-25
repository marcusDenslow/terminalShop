# Checkout FLow

## The Problemo

Charging a card and recording the order are two different operations accross
two seperate systems (Stripe and the database). If anything fails betweenj
them, you can end up in a state where a customer was charged but no 
order exists in the database.

## How It Works

The ´ConvertCart´ handler (´api/handlers/cart.go´) avoids this by following 
a specific order of operations:

1. **Create the order first** written to the DB with ´status = "pending"´
    and no Stripe ID yet. This means there is always a DB record before any 
    money moves.

+2. **Charge Stripe** — using an idempotency key of `order-{id}`. If the
   request is retried for any reason, Stripe will return the original result
   instead of charging twice.

+3. **On charge failure** — the order is marked `status = "failed"` and the
   error is returned to the client. The cart is left untouched.

+4. **On charge success** — a single DB transaction atomically:
   - Sets `status = "paid"` and writes the Stripe PaymentIntent ID
   - Deletes the cart items
   - Clears the address and card from the cart

+5. **If the transaction fails after a successful charge** — this is the
   worst case. The charge went through but the DB update failed. A
   `[CRITICAL]` log line is emitted with the order ID and PaymentIntent ID
   for manual reconciliation. See below.

## Reconciliation

If a `[CRITICAL]` log appears, find the order and patch it manually:

```sql
UPDATE orders
SET status = 'paid', stripe_payment_id = '<pi_id_from_log>'
WHERE id = <order_id_from_log>;
```

Then verify the PaymentIntent status in the Stripe dashboard matches.

A pending order with no ´stripe_payment_id´ after more than a few minutes
indicates a failed charge (status should be ´failed´). A pending order
with a ´stripe_payment_id´ indicates a reconciliation gap.

## Testing

Use Stripe´s test mode with the following card numbers (any future expiry,
any CVC, any ZIP):

the folling table is constructed by ["Card number"]["Behaviour"]

4242 4242 4242 4242 - Success
4000 0000 0000 0002 - Generic decline
4000 0000 0000 9995 - Insufficient funds
4000 0000 0000 0069 - Expired card
4000 0000 0000 0127 - Incorrect CVC

### What to verify on success
 - Response is ´200´ with an order object
 - Order in DB has ´status = "paid"´ and a non-empty ´stripe_payment_id´
 - Cart items for that user are deleted
 - Cart ´address_id´ and ´card_id´ are cleared

### What to verify on decline
- Response is `402` with `CARD_ERROR`
- Order in DB has `status = "failed"`
- Cart items are untouched

## Known Limitations

A server crash (not an error — a literal process kill) between the Stripe
charge succeeding and the DB transaction committing will leave the order as
`pending` with no `stripe_payment_id`. The `[CRITICAL]` log won't fire
because the process is dead. The fix is a reconciliation job that
periodically queries Stripe for PaymentIntents and cross-checks them
against orders — not implemented yet.
