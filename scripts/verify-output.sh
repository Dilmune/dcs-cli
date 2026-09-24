#!/usr/bin/env bash
# Output gates for a dcs binary, from docs/DESIGN.md "Verification":
#   a. --json parity with a baseline binary (only when one is given)
#   b. zero ESC bytes under --no-color and NO_COLOR=1, run on a real pty; the
#      one exception is Bubble Tea v1's own init query, reported as KNOWN and
#      allowed only while the binary is built with Bubble Tea v1
#   c. no #rrggbb literal in Go sources outside theme.go and logo.go
# Every command here is read-only. Never add a mutating command to a list.
set -euo pipefail

readonly USAGE='usage: scripts/verify-output.sh <new-binary> [<baseline-binary>]'
readonly ESC_BYTE=$'\033'
# Bubble Tea v1's package init asks the terminal for its background before main
# runs: OSC 11 with ST, then a cursor position request as the sentinel.
readonly UPSTREAM_QUERY=$'\033]11;?\033\\\033[6n'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLI_DIR="$(dirname "$SCRIPT_DIR")"
readonly SCRIPT_DIR CLI_DIR

die() {
  printf 'verify-output: %s\n' "$1" >&2
  exit 1
}

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  die "$USAGE"
fi
NEW_BIN="$1"
BASE_BIN="${2:-}"

[ -x "$NEW_BIN" ] || die "new binary is not an executable file: $NEW_BIN"
if [ -n "$BASE_BIN" ] && [ ! -x "$BASE_BIN" ]; then
  die "baseline binary is not an executable file: $BASE_BIN"
fi
command -v jq >/dev/null || die "jq not found: needed to read the first server id"
command -v script >/dev/null || die "script not found: needed to run commands on a pty"
if ! "$NEW_BIN" whoami --json >/dev/null; then
  die "not logged in: run '$NEW_BIN login' first"
fi

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/dcs-verify-output.XXXXXX")"
readonly WORK_DIR
cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

failures=0
known=0
pass() {
  printf 'PASS  %s\n' "$1"
}
fail() {
  printf 'FAIL  %s: %s\n' "$1" "$2"
  failures=$((failures + 1))
}

known() {
  printf 'KNOWN %s: %s\n' "$1" "$2"
  known=$((known + 1))
}

esc_count() {
  LC_ALL=C tr -cd "$ESC_BYTE" <"$1" | wc -c | tr -d ' '
}

# run_on_pty <log> <command...>: the command sees a terminal on stdout, so
# --no-color and NO_COLOR are what disable color, not a pipe.
run_on_pty() {
  local log="$1"
  shift
  if [ "$(uname -s)" = Darwin ]; then
    script -q "$log" "$@" </dev/null >/dev/null
  else
    script -q -e -c "$(printf '%q ' "$@")" "$log" </dev/null >/dev/null
  fi
}

SERVER_ID="$("$NEW_BIN" servers list --json | jq -r '.[0].id // empty')"
if [ -z "$SERVER_ID" ]; then
  printf 'note: the account has no servers; server-specific commands are skipped\n'
fi

JSON_COMMANDS=("status" "servers list" "whoami" "config view" "keys list" "api-keys list")
PLAIN_COMMANDS=("status" "servers list" "whoami")
if [ -n "$SERVER_ID" ]; then
  JSON_COMMANDS+=("servers info $SERVER_ID" "sites list --server $SERVER_ID")
  PLAIN_COMMANDS+=("servers info $SERVER_ID")
fi

# The API key the CLI authenticates with gets a new lastUsedAt on every
# request, so two runs can never match there. Mask that one field, nothing else.
mask_volatile() {
  local spec="$1" file="$2"
  if [ "$spec" = "api-keys list" ]; then
    sed -E 's/("lastUsedAt": )"[^"]*"/\1"(masked)"/' "$file" >"$file.masked"
    mv "$file.masked" "$file"
  fi
}

check_json_parity() {
  local spec="$1" name="json parity: $1" args new_out base_out
  if [ "$spec" = "api-keys list" ]; then
    name="$name (lastUsedAt masked)"
  fi
  read -r -a args <<<"$spec"
  new_out="$WORK_DIR/new.json"
  base_out="$WORK_DIR/base.json"
  if ! "$NEW_BIN" "${args[@]}" --json >"$new_out"; then
    fail "$name" "new binary exited non-zero"
    return
  fi
  if ! "$BASE_BIN" "${args[@]}" --json >"$base_out"; then
    fail "$name" "baseline binary exited non-zero"
    return
  fi
  mask_volatile "$spec" "$new_out"
  mask_volatile "$spec" "$base_out"
  if cmp -s "$new_out" "$base_out"; then
    pass "$name"
  else
    fail "$name" "$(cmp "$new_out" "$base_out" | sed "s|$WORK_DIR/||g" | head -1)"
  fi
}

# esc_count_without_upstream_query <log>: ESC bytes left after removing one
# exact copy of the upstream query; the full count when it is not present.
esc_count_without_upstream_query() {
  local LC_ALL=C content
  content="$(cat "$1")"
  if [[ "$content" != *"$UPSTREAM_QUERY"* ]]; then
    esc_count "$1"
    return
  fi
  printf '%s' "${content/"$UPSTREAM_QUERY"/}" | tr -cd "$ESC_BYTE" | wc -c | tr -d ' '
}

# The exception expires by itself: a binary built without Bubble Tea v1 gets
# none, and so does any run where go cannot read the binary's module list.
upstream_query_allowed() {
  command -v go >/dev/null || return 1
  go version -m "$NEW_BIN" | awk '$1 == "dep" && $2 == "github.com/charmbracelet/bubbletea" && $3 ~ /^v1\./ { found = 1 } END { exit !found }'
}

ALLOW_UPSTREAM_QUERY=0
if upstream_query_allowed; then
  ALLOW_UPSTREAM_QUERY=1
else
  printf 'note: the binary is not built with Bubble Tea v1 (or go is missing), so no upstream exception applies\n'
fi
readonly ALLOW_UPSTREAM_QUERY

check_no_escape() {
  local spec="$1" variant="$2" name="no escape bytes ($2): $1" args log count
  read -r -a args <<<"$spec"
  log="$WORK_DIR/pty.log"
  if [ "$variant" = "--no-color" ]; then
    run_on_pty "$log" "$NEW_BIN" "${args[@]}" --no-color || {
      fail "$name" "command exited non-zero"
      return
    }
  else
    run_on_pty "$log" env NO_COLOR=1 "$NEW_BIN" "${args[@]}" || {
      fail "$name" "command exited non-zero"
      return
    }
  fi
  count="$(esc_count "$log")"
  if [ "$count" -eq 0 ]; then
    pass "$name"
  elif [ "$ALLOW_UPSTREAM_QUERY" -eq 1 ] && [ "$(esc_count_without_upstream_query "$log")" -eq 0 ]; then
    known "$name" "Bubble Tea v1 init query, see docs/RELEASING.md"
  elif LC_ALL=C grep -q "${ESC_BYTE}]11;?" "$log"; then
    fail "$name" "$count ESC bytes, including a terminal background query (OSC 11)"
  else
    fail "$name" "$count ESC bytes"
  fi
}

# Proves the pty harness can see color at all; without it check b is vacuous.
check_pty_control() {
  local name="pty control: status shows color with no flags" log count
  log="$WORK_DIR/control.log"
  if ! run_on_pty "$log" "$NEW_BIN" status; then
    fail "$name" "command exited non-zero"
    return
  fi
  count="$(esc_count "$log")"
  if [ "$count" -gt 0 ]; then
    pass "$name"
  else
    fail "$name" "no ESC bytes, so the no-escape checks prove nothing"
  fi
}

check_hex_literals() {
  local name="no hex color literal outside theme.go and logo.go" file rc hits
  local files=()
  while IFS= read -r -d '' file; do
    files+=("$file")
  done < <(find "$CLI_DIR" -type f -name '*.go' \
    ! -path "$CLI_DIR/internal/ui/theme.go" \
    ! -path "$CLI_DIR/internal/workspace/logo.go" -print0)
  if [ "${#files[@]}" -eq 0 ]; then
    fail "$name" "no Go sources found under $CLI_DIR"
    return
  fi
  rc=0
  hits="$(grep -nHE '#[0-9a-fA-F]{6}' "${files[@]}")" || rc=$?
  case "$rc" in
    0) fail "$name" "$(printf '\n%s' "$hits")" ;;
    1) pass "$name" ;;
    *) fail "$name" "grep exited $rc" ;;
  esac
}

if [ -n "$BASE_BIN" ]; then
  for spec in "${JSON_COMMANDS[@]}"; do
    check_json_parity "$spec"
  done
else
  printf 'note: no baseline binary given; json parity is skipped\n'
fi

check_pty_control
for spec in "${PLAIN_COMMANDS[@]}"; do
  check_no_escape "$spec" "--no-color"
  check_no_escape "$spec" "NO_COLOR=1"
done

check_hex_literals

if [ "$failures" -gt 0 ]; then
  printf '\n%d check(s) failed\n' "$failures"
  exit 1
fi
if [ "$known" -gt 0 ]; then
  printf '\nall checks passed, %d known upstream exception(s)\n' "$known"
else
  printf '\nall checks passed\n'
fi
