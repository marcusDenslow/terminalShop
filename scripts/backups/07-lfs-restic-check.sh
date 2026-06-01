#!/usr/bin/env bash
# Independent restic check from the lfs-remote side. Reads repo via a
# LOCAL filesystem path (not SFTP), so it proves the data is parseable
# without any involvement from the prod box at restic level.
#
# Needs:
#   - restic installed on lfs-remote (see scratchpkg port at /usr/ports/main/restic)
#   - SSH access to both prod (to fetch the repo password) and lfs
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/lib.sh"

print_header "lfs-remote restic check (independent vantage point)"

# Precondition: restic must be on lfs.
if ! ssh "$LFS_HOST" 'command -v restic >/dev/null 2>&1'; then
  echo 'ERROR: restic not in PATH on lfs-remote' >&2
  echo "Install via scratchpkg, then re-run." >&2
  exit 2
fi

# Pull restic password from prod (needs sudo on prod).
spinner_start "fetching restic password from $SSH_HOST..."
PW=$(ssh "$SSH_HOST" "sudo cat '$RESTIC_PASSWORD_FILE'")
spinner_stop
if [ -z "$PW" ]; then
  echo 'ERROR: could not read restic password from prod' >&2
  exit 2
fi

# Run restic against the LOCAL repo path on lfs. Password fed via stdin
# so it never lands in argv or a file on lfs.
spinner_start "running restic check from $LFS_HOST..."
ssh "$LFS_HOST" "restic -r '$LFS_REPO_PATH' --password-file=/dev/stdin check" <<<"$PW"
spinner_stop

echo
print_ok "lfs-side restic check passed"
