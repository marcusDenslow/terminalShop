#!/bin/bash
# Load .env to get the Stripe key
set -a; source .env; set +a

# Kill all background jobs on exit
trap 'kill $(jobs -p) 2>/dev/null' EXIT

echo "Starting API server..."
(cd api && go run main.go) &

echo "Starting Stripe webhook listener..."
stripe listen --api-key "$STRIPE_SECRET_KEY" --forward-to localhost:8080/api/v1/webhooks/stripe &

echo "Starting ngrok..."
ngrok http 8080 --log stdout &

echo ""
echo "Starting SSH server (Ctrl+C to stop everything)..."
go run main.go
