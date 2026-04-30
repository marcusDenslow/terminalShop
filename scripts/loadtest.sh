#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${1:-https://api.sshops.uk}"
WORKERS=10

echo "load test → $BASE_URL"
echo "tail metrics:  watch 'curl -s $BASE_URL/metrics | grep http_requests_total | grep -v _bucket'"
echo

hit() {
  local method="$1" path="$2" count="$3" workers="$4"
  echo "→ $method $path  ($count requests, $workers parallel)"
  seq "$count" | xargs -n1 -P"$workers" -I{} \
    curl -s -o /dev/null -w "" -X "$method" "$BASE_URL$path" || true
  echo "   done"
}

scenario_warmup() {
  echo "=== scenario: warmup (gentle, sequential) ==="
  hit GET  /api/v1/health   20  1
  hit GET  /api/v1/ping     20  1
  hit GET  /api/v1/products 20  1
  echo
}

scenario_sustained() {
  echo "=== scenario: sustained moderate load (60s) ==="
  local end=$((SECONDS + 60))
  while [ $SECONDS -lt $end ]; do
    hit GET /api/v1/health   50 "$WORKERS"
    hit GET /api/v1/products 50 "$WORKERS"
  done
  echo
}

scenario_burst() {
  echo "=== scenario: burst (300 reqs, $WORKERS parallel) ==="
  hit GET /api/v1/products 300 "$WORKERS"
  echo
}

scenario_mixed_with_errors() {
  echo "=== scenario: mixed traffic + intentional 404s ==="
  hit GET  /api/v1/health             100 "$WORKERS"
  hit HEAD /api/v1/health             100 "$WORKERS"
  hit GET  /api/v1/products           100 "$WORKERS"
  hit GET  /api/v1/products/does-not-exist 50 5
  hit GET  /api/v1/this-route-doesnt-exist  50 5
  echo
}

case "${2:-all}" in
  warmup)    scenario_warmup ;;
  sustained) scenario_sustained ;;
  burst)     scenario_burst ;;
  mixed)     scenario_mixed_with_errors ;;
  all)
    scenario_warmup
    scenario_burst
    scenario_mixed_with_errors
    scenario_sustained
    ;;
  *) echo "usage: $0 [BASE_URL] [warmup|sustained|burst|mixed|all]"; exit 1 ;;
esac

echo "✓ load test complete — check Grafana dashboard"
