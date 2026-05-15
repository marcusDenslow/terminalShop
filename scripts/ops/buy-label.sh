#!/usr/bin/env bash
# Buy a shipping label via the admin endpoint.
# Usage: scripts/ops/buy-label.sh <order_id>
set -euo pipefail

ORDER_ID="${1:?usage: $0 <order_id>}"
: "${ADMIN_KEY:?ADMIN_KEY not set; source scripts/ops/env.sh first}"
: "${BASE_URL:=https://api.sshops.uk}"

curl -sS -X POST "$BASE_URL/api/v1/orders/$ORDER_ID/label" \
  -H "X-Admin-Key: $ADMIN_KEY" \
  -H "Content-Type: application/json" | jq .
