#!/usr/bin/env bash
# Fire a Shippo-shaped tracking webhook against an order's tracking number.
# Usage: scripts/ops/scan.sh <order_id> <pre_transit|transit|delivered|failure|returned> [--detail "free text"]
set -euo pipefail

ORDER_ID="${1:?usage: $0 <order_id> <status> [--detail \"text\"]}"
STATUS_LOWER="${2:?missing status; one of: pre_transit transit delivered failure returned}"
DETAIL=""
if [[ "${3:-}" == "--detail" ]]; then
  DETAIL="${4:?--detail requires a value}"
fi

: "${SECRET:?SECRET not set; source scripts/ops/env.sh first}"
: "${BASE_URL:=https://api.sshops.uk}"

case "$STATUS_LOWER" in
  pre_transit) STATUS="PRE_TRANSIT" ;;
  transit)     STATUS="TRANSIT" ;;
  delivered)   STATUS="DELIVERED" ;;
  failure)     STATUS="FAILURE" ;;
  returned)    STATUS="RETURNED" ;;
  *) echo "unknown status: $STATUS_LOWER" >&2; exit 1 ;;
esac

if [[ -z "$DETAIL" ]]; then
  case "$STATUS" in
    PRE_TRANSIT) DETAIL="Shipping label created, USPS awaiting item" ;;
    TRANSIT)     DETAIL="In transit to destination" ;;
    DELIVERED)   DETAIL="Delivered, front porch" ;;
    FAILURE)     DETAIL="Delivery exception" ;;
    RETURNED)    DETAIL="Returned to sender" ;;
  esac
fi

TRACKING=$(ssh ubuntu-helsinki "sqlite3 /root/terminalShop/data/terminalshop.db \"SELECT tracking_number FROM orders WHERE id=$ORDER_ID;\"")
if [[ -z "$TRACKING" ]]; then
  echo "order $ORDER_ID has no tracking number; buy a label first" >&2
  exit 1
fi

STATUS_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

curl -sS -X POST "$BASE_URL/api/v1/webhooks/shippo?token=$SECRET" \
  -H "Content-Type: application/json" \
  --data @- <<EOF | jq .
{
  "event": "track_updated",
  "data": {
    "tracking_number": "$TRACKING",
    "carrier": "usps",
    "tracking_status": {
      "status": "$STATUS",
      "status_date": "$STATUS_DATE",
      "status_details": "$DETAIL"
    }
  }
}
EOF
