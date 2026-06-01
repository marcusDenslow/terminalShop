#!/usr/bin/env bash
# Show last 20 lines of the cron log + its mtime. Cheapest "did it run" check.
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/lib.sh"

print_header "Backup log: mtime + last 20 lines"
spinner_start "fetching log from $SSH_HOST..."
ssh "$SSH_HOST" "
  sudo stat -c 'Last modified: %y' '$BACKUP_LOG'
  echo; sudo tail -20 '$BACKUP_LOG'
"
spinner_stop
