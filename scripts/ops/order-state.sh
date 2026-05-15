#!/usr/bin/env bash
# Show the current DB row state for an order (lifecycle + tracking columns).
# Usage: scripts/ops/order-state.sh <order_id>
set -euo pipefail

ORDER_ID="${1:?usage: $0 <order_id>}"

ssh ubuntu-helsinki "sqlite3 -header -column /root/terminalShop/data/terminalshop.db \"SELECT id, status, total, carrier, tracking_number, tracking_status, tracking_status_details, tracking_status_updated_at FROM orders WHERE id=$ORDER_ID;\""
