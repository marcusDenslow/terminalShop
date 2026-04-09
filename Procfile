api: bash -c 'cd api && go run main.go'
ssh: go run main.go
webhook: stripe listen --api-key $STRIPE_SECRET_KEY --forward-to localhost:8080/api/v1/webhooks/stripe
tunnel: ngrok http 8080 --log stdout
