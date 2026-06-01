#!/usr/bin/env bash
# List every restic snapshot with timestamps and tags.
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/lib.sh"

print_header "All restic snapshots"
spinner_start "querying restic via SFTP..."
ssh "$SSH_HOST" "
  export RESTIC_PASSWORD_FILE='$RESTIC_PASSWORD_FILE'
  export RESTIC_REPOSITORY='$RESTIC_REPO'
  sudo -E restic -o sftp.args='-i $RESTIC_SSH_KEY' snapshots
"
spinner_stop
