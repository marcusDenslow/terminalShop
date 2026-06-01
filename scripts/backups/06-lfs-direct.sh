#!/usr/bin/env bash
# Independent filesystem check on lfs-remote. No restic, no password needed.
# Proves the repo dir exists, files are present, recent uploads landed,
# and the backup partition has room.
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/lib.sh"

print_header "lfs-remote filesystem-level repo check"
spinner_start "inspecting $LFS_HOST filesystem..."

ssh "$LFS_HOST" "
set -e
REPO='$LFS_REPO_PATH'
WARN_PCT=$LFS_DISK_WARN_PCT

if [ ! -d \"\$REPO\" ]; then
  echo 'ERROR: repo dir missing on lfs' >&2
  exit 1
fi

echo '--- snapshot pointer count (one file per restic snapshot) ---'
COUNT=\$(find \"\$REPO/snapshots\" -type f 2>/dev/null | wc -l)
echo \"snapshots/: \$COUNT files\"

echo
echo '--- repo size on disk ---'
du -sh \"\$REPO\"

echo
echo '--- recent uploads (files newer than 48h) ---'
RECENT=\$(find \"\$REPO\" -type f -mtime -2 2>/dev/null | wc -l)
echo \"files added in last 48h: \$RECENT\"
if [ \"\$RECENT\" -eq 0 ]; then
  echo 'WARN: no new files in last 48h' >&2
fi

echo
echo '--- most recent snapshot pointer ---'
LATEST=\$(ls -t \"\$REPO/snapshots\"/* 2>/dev/null | head -1)
if [ -n \"\$LATEST\" ]; then
  stat -c '%y  %n' \"\$LATEST\"
else
  echo 'WARN: no snapshot files found' >&2
fi

echo
echo '--- disk usage ---'
df -h \"\$REPO\"
USED=\$(df --output=pcent \"\$REPO\" 2>/dev/null | tail -1 | tr -dc 0-9)
if [ -n \"\$USED\" ] && [ \"\$USED\" -gt \"\$WARN_PCT\" ]; then
  echo \"FAIL: backup partition \${USED}% full (threshold \${WARN_PCT}%)\" >&2
  exit 1
fi

echo
echo 'OK'
"
spinner_stop
