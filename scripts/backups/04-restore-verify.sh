#!/usr/bin/env bash
# Restore the latest snapshot to /tmp on the remote box, then:
#   - PRAGMA integrity_check on the restored DB
#   - row-count diff vs the live DB
# Cleans up /tmp dir at the end.
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/lib.sh"

print_header "Restore latest snapshot and verify SQLite"
spinner_start "restoring latest snapshot to /tmp on $SSH_HOST..."

ssh "$SSH_HOST" \
  "PWFILE='$RESTIC_PASSWORD_FILE' \
   REPO='$RESTIC_REPO' \
   KEY='$RESTIC_SSH_KEY' \
   LIVE='$LIVE_DB' \
   bash -s" <<'REMOTE'
set -e
DIR=/tmp/tshop-restore-test
sudo rm -rf "$DIR" && sudo mkdir -p "$DIR"

sudo -E RESTIC_PASSWORD_FILE="$PWFILE" RESTIC_REPOSITORY="$REPO" \
  restic -o sftp.args="-i $KEY" restore latest --target "$DIR"

DB=$(sudo find "$DIR" -name 'terminalshop.db' | head -1)
if [ -z "$DB" ]; then
  echo 'restored DB not found' >&2
  exit 1
fi

echo
echo '--- restored file ---'
sudo stat -c '%y  %s bytes  %n' "$DB"

echo
echo '--- PRAGMA integrity_check ---'
sudo sqlite3 "$DB" 'PRAGMA integrity_check;'

echo
echo '--- PRAGMA quick_check ---'
sudo sqlite3 "$DB" 'PRAGMA quick_check;'

echo
echo '--- restored vs live row counts ---'
printf '%-22s %10s %10s\n' table restored live
for t in $(sudo sqlite3 "$DB" 'SELECT name FROM sqlite_master WHERE type="table";'); do
  r=$(sudo sqlite3 "$DB" "SELECT COUNT(*) FROM \"$t\";")
  l=$(sudo sqlite3 "$LIVE" "SELECT COUNT(*) FROM \"$t\";" 2>/dev/null || echo 'n/a')
  printf '%-22s %10s %10s\n' "$t" "$r" "$l"
done

sudo rm -rf "$DIR"
echo
echo 'OK'
REMOTE
spinner_stop
