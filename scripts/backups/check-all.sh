#!/usr/bin/env bash
# Master runner. Executes every backup check in sequence.
#   ./check-all.sh              stop on first failure; interactive menu when TTY
#   ./check-all.sh --keep-going keep running after failures (no menu)
#   ./check-all.sh --deep       use --deep on 03-check.sh
#
# When stdin and stdout are TTYs, a failed step opens a drill-down menu:
#   [r]etry  [l]og  [u]nlock  [s]kip  [q]uit
# Non-TTY (cron / piped) skips the menu and behaves like the old runner.
set -uo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

KEEP_GOING=0
DEEP=0
for arg in "$@"; do
  case "$arg" in
    --keep-going) KEEP_GOING=1 ;;
    --deep)       DEEP=1 ;;
    -h|--help)
      echo "Usage: $0 [--keep-going] [--deep]"
      echo "  --keep-going  don't stop on first failure, suppress interactive menu"
      echo "  --deep        pass --deep to 03-check.sh (10% data re-read)"
      exit 0
      ;;
    *)
      echo "unknown flag: $arg" >&2
      exit 2
      ;;
  esac
done

# Pull in config + color helpers (also gives us SSH_HOST + restic env for unlock).
source "$SCRIPT_DIR/lib.sh"

declare -a STEP_NAMES=()
declare -a STEP_STATUS=()   # OK | FAIL
FAIL=0

prompt_after_fail() {
  # returns 0=skip, 1=quit, 2=retry
  local script="$1"
  while :; do
    printf '\n%sNext?%s [r]etry  [l]og  [u]nlock  [s]kip  [q]uit > ' "$C_BOLD" "$C_RESET"
    local choice
    read -r choice
    case "$choice" in
      r|R) return 2 ;;
      l|L) "$SCRIPT_DIR/01-log-tail.sh"; echo ;;
      u|U)
        echo
        ssh "$SSH_HOST" "
          export RESTIC_PASSWORD_FILE='$RESTIC_PASSWORD_FILE'
          export RESTIC_REPOSITORY='$RESTIC_REPO'
          sudo -E restic -o sftp.args='-i $RESTIC_SSH_KEY' unlock
        "
        echo
        ;;
      s|S) return 0 ;;
      q|Q) return 1 ;;
      "")  ;;
      *)   printf 'unknown: %s\n' "$choice" ;;
    esac
  done
}

run_step() {
  local script="$1"; shift
  echo
  printf '%s########## %s %s ##########%s\n' "$C_CYAN$C_BOLD" "$script" "$*" "$C_RESET"

  local attempts=0
  while :; do
    attempts=$((attempts + 1))
    if "$SCRIPT_DIR/$script" "$@"; then
      STEP_NAMES+=("$script")
      STEP_STATUS+=("OK")
      return 0
    fi

    printf '%sFAILED:%s %s\n' "$C_RED$C_BOLD" "$C_RESET" "$script" >&2

    if [ -t 0 ] && [ -t 1 ] && [ "$KEEP_GOING" -eq 0 ]; then
      prompt_after_fail "$script"
      case "$?" in
        0) STEP_NAMES+=("$script"); STEP_STATUS+=("FAIL"); FAIL=$((FAIL + 1)); return 0 ;;
        1) STEP_NAMES+=("$script"); STEP_STATUS+=("FAIL"); FAIL=$((FAIL + 1)); print_summary; exit 1 ;;
        2) echo
           printf '%s>>>>>> retry %s (attempt %d) <<<<<<%s\n' "$C_CYAN$C_BOLD" "$script" $((attempts + 1)) "$C_RESET"
           continue ;;
      esac
    fi

    # non-interactive path
    STEP_NAMES+=("$script")
    STEP_STATUS+=("FAIL")
    FAIL=$((FAIL + 1))
    if [ "$KEEP_GOING" -eq 0 ]; then
      print_summary
      exit 1
    fi
    return 0
  done
}

print_summary() {
  echo
  printf '%s===============================================%s\n' "$C_CYAN$C_BOLD" "$C_RESET"
  printf '   %-28s %s\n' "Script" "Result"
  printf '   %-28s %s\n' "----------------------------" "--------"
  local i
  for i in "${!STEP_NAMES[@]}"; do
    local name="${STEP_NAMES[$i]}"
    local status="${STEP_STATUS[$i]}"
    local color sym
    case "$status" in
      OK)   color="$C_GREEN"; sym="✓ OK"   ;;
      FAIL) color="$C_RED"  ; sym="✗ FAIL" ;;
    esac
    printf '   %-28s %s%s%s\n' "$name" "$color" "$sym" "$C_RESET"
  done
  printf '%s===============================================%s\n' "$C_CYAN$C_BOLD" "$C_RESET"
  if [ "$FAIL" -eq 0 ]; then
    printf '%sAll checks passed.%s\n' "$C_GREEN$C_BOLD" "$C_RESET"
  else
    printf '%s%d check(s) failed.%s\n' "$C_RED$C_BOLD" "$FAIL" "$C_RESET"
  fi
}

run_step 01-log-tail.sh
run_step 02-snapshots.sh
if [ "$DEEP" -eq 1 ]; then run_step 03-check.sh --deep; else run_step 03-check.sh; fi
run_step 04-restore-verify.sh
run_step 05-freshness.sh
run_step 06-lfs-direct.sh
run_step 07-lfs-restic-check.sh

print_summary
[ "$FAIL" -eq 0 ] || exit 1
