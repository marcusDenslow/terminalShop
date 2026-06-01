#!/usr/bin/env bash
# restic repo integrity check.
#   ./03-check.sh         metadata only (~5s)
#   ./03-check.sh --deep  + 10% data-pack re-read (~30s)
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/lib.sh"

EXTRA=""
if [ "${1:-}" = "--deep" ]; then
  EXTRA="--read-data-subset=10%"
fi

print_header "restic check ${EXTRA:-(metadata only)}"
spinner_start "running restic check on $SSH_HOST..."
ssh "$SSH_HOST" "
  export RESTIC_PASSWORD_FILE='$RESTIC_PASSWORD_FILE'
  export RESTIC_REPOSITORY='$RESTIC_REPO'
  sudo -E restic -o sftp.args='-i $RESTIC_SSH_KEY' check $EXTRA
"
spinner_stop
