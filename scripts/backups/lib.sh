#!/usr/bin/env bash
# Shared loader for the backup verification scripts.
# Each script should: source "$SCRIPT_DIR/lib.sh"

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

if [ ! -f "$SCRIPT_DIR/config.env" ]; then
  echo "ERROR: $SCRIPT_DIR/config.env missing." >&2
  echo "Run: cp $SCRIPT_DIR/config.env.example $SCRIPT_DIR/config.env" >&2
  exit 1
fi

# shellcheck source=/dev/null
source "$SCRIPT_DIR/config.env"

# ANSI colors. Emit only when stdout is a TTY and NO_COLOR is unset.
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  C_RED=$'\033[1;31m'
  C_GREEN=$'\033[1;32m'
  C_YELLOW=$'\033[1;33m'
  C_BLUE=$'\033[1;34m'
  C_CYAN=$'\033[1;36m'
  C_BOLD=$'\033[1m'
  C_RESET=$'\033[0m'
else
  C_RED=""; C_GREEN=""; C_YELLOW=""; C_BLUE=""; C_CYAN=""; C_BOLD=""; C_RESET=""
fi

print_header() { printf '\n%s=== %s ===%s\n' "$C_CYAN" "$1" "$C_RESET"; }
print_ok()     { printf '%sOK%s%s\n'    "$C_GREEN"  "$C_RESET" "${1:+ — $1}"; }
print_fail()   { printf '%sFAIL%s%s\n'  "$C_RED"    "$C_RESET" "${1:+ — $1}" >&2; }
print_warn()   { printf '%sWARN%s%s\n'  "$C_YELLOW" "$C_RESET" "${1:+ — $1}" >&2; }
print_info()   { printf '%s%s%s\n'      "$C_BLUE"   "$1"       "$C_RESET"; }

# Spinner: shows a rotating braille char while a foreground command runs.
# Use as a pair around the silent/blocking call:
#   spinner_start "fetching log..."
#   ssh ...
#   spinner_stop
# Auto-skips when stdout isn't a TTY (cron / piped). Real output from the
# wrapped command will visually overwrite the spinner — by design.
SPINNER_PID=""
spinner_start() {
  [ -t 1 ] && [ -z "${NO_COLOR:-}" ] || return 0
  local msg="${1:-working}"
  (
    trap 'exit 0' TERM
    local chars=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')
    local i=0
    while :; do
      i=$(( (i + 1) % ${#chars[@]} ))
      printf '\r%s%s%s %s' "$C_CYAN" "${chars[$i]}" "$C_RESET" "$msg"
      sleep 0.08
    done
  ) &
  SPINNER_PID=$!
  disown "$SPINNER_PID" 2>/dev/null || true
}
spinner_stop() {
  [ -n "$SPINNER_PID" ] || return 0
  kill "$SPINNER_PID" 2>/dev/null || true
  wait "$SPINNER_PID" 2>/dev/null || true
  printf '\r\033[K'
  SPINNER_PID=""
}
