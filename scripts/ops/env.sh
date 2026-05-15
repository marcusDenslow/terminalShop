#!/usr/bin/env bash
# Source once per shell session: `source scripts/ops/env.sh`
# Fetches ADMIN_API_KEY + SHIPPO_WEBHOOK_SECRET from prod's .env.prod
# and exports them so the other ops scripts can use them.

export BASE_URL="${BASE_URL:-https://api.sshops.uk}"

ADMIN_KEY_FETCHED=$(ssh ubuntu-helsinki "grep '^ADMIN_API_KEY=' /root/terminalShop/.env.prod | cut -d= -f2-")
SECRET_FETCHED=$(ssh ubuntu-helsinki "grep '^SHIPPO_WEBHOOK_SECRET=' /root/terminalShop/.env.prod | cut -d= -f2-")

if [[ -z "$ADMIN_KEY_FETCHED" || -z "$SECRET_FETCHED" ]]; then
  echo "failed to fetch secrets from prod; aborting" >&2
  return 1 2>/dev/null || exit 1
fi

export ADMIN_KEY="$ADMIN_KEY_FETCHED"
export SECRET="$SECRET_FETCHED"

echo "ops env loaded (BASE_URL=$BASE_URL)"
