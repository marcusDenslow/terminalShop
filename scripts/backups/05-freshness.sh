#!/usr/bin/env bash
# Print latest snapshot age. Exit 0 if fresh, 1 if older than STALE_HOURS.
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/lib.sh"

print_header "Latest snapshot freshness"

spinner_start "querying latest snapshot..."
LATEST=$(ssh "$SSH_HOST" "
  export RESTIC_PASSWORD_FILE='$RESTIC_PASSWORD_FILE'
  export RESTIC_REPOSITORY='$RESTIC_REPO'
  sudo -E restic -o sftp.args='-i $RESTIC_SSH_KEY' snapshots --json
" | jq -r 'max_by(.time) | .time')
spinner_stop

if [ -z "$LATEST" ] || [ "$LATEST" = "null" ]; then
  echo "ERROR: could not parse latest snapshot time" >&2
  exit 2
fi

NOW=$(date -u +%s)
# GNU date (Linux) first, then BSD date (macOS) as fallback.
if THEN=$(date -u -d "$LATEST" +%s 2>/dev/null); then
  :
else
  CLEAN=$(echo "$LATEST" | sed 's/\.[0-9]*Z$/Z/')
  THEN=$(date -u -j -f "%Y-%m-%dT%H:%M:%SZ" "$CLEAN" +%s)
fi

AGE_HRS=$(( (NOW - THEN) / 3600 ))
echo "Latest snapshot: $LATEST"
echo "Age:             ${AGE_HRS}h"
echo "Threshold:       ${STALE_HOURS}h"

if [ "$AGE_HRS" -lt "$STALE_HOURS" ]; then
  print_ok "snapshot within freshness window"
else
  print_fail "STALE — investigate"
  exit 1
fi
