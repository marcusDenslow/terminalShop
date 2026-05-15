#!/usr/bin/env bash
# List recent paid orders awaiting label purchase.
# Usage: scripts/ops/list-paid.sh
set -euo pipefail

ssh ubuntu-helsinki "sqlite3 -header -column /root/terminalShop/data/terminalshop.db \"SELECT id, status, total, shipping_name, shipping_city, shipping_country FROM orders WHERE status='paid' ORDER BY id DESC LIMIT 10;\""
