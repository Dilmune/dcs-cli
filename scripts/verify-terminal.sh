#!/usr/bin/env bash
# Ghostty screenshots for docs/DESIGN.md "Verification" item 1: three themes at
# 80 and 120 columns, plain commands, --help and the dcs ui workspace.
#
# This script quits Ghostty after every capture, so run it from another
# terminal such as Terminal.app with Ghostty closed.
#
# Rules this script keeps, each learned from a capture that went wrong:
# - Keys go only through tmux send-keys. System Events keystrokes land in
#   whatever app is frontmost, including the user's own terminal.
# - Ghostty only attaches to the tmux session to render it.
# - Sizes are forced: --window-save-state=never on Ghostty, and a manual
#   tmux window size, so a remembered window cannot change the column count.
# - dcs ui gets --theme explicitly, because tmux hides the terminal
#   background from detection.
# - Ghostty is quit after every capture and the next window waits until the
#   ghostty process has exited.
# - A "could not create image from window" from screencapture means the
#   screen is locked or asleep; the run stops instead of writing blanks.
set -euo pipefail

readonly USAGE='usage: scripts/verify-terminal.sh <dcs-binary> <out-dir>'
readonly GHOSTTY_APP=/Applications/Ghostty.app
readonly THEMES=("dark:" "light:Catppuccin Latte" "dim:Gruvbox Light")
readonly WIDTHS=(80 120)
readonly COMMAND_ROWS=50
readonly UI_ROWS=40
readonly WAIT_STEPS=40
readonly KEY_GAP=0.6
readonly LOCKED_SCREEN_MESSAGE='could not create image from window'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

die() {
  printf 'verify-terminal: %s\n' "$1" >&2
  exit 1
}

[ "$#" -eq 2 ] || die "$USAGE"

[ "$(uname -s)" = Darwin ] || die "macOS only: capture uses Ghostty, screencapture and CoreGraphics"
[ -d "$GHOSTTY_APP" ] || die "Ghostty not found at $GHOSTTY_APP"
command -v tmux >/dev/null || die "tmux not found: it drives every keystroke (brew install tmux)"
command -v swiftc >/dev/null || die "swiftc not found: needed to build the window lookup (xcode-select --install)"
command -v screencapture >/dev/null || die "screencapture not found"
command -v sips >/dev/null || die "sips not found: needed to report image widths"
command -v jq >/dev/null || die "jq not found: needed to read the first server id"
if pgrep -x ghostty >/dev/null; then
  die "Ghostty is running. This script quits Ghostty after every capture, so run it from another terminal such as Terminal.app with Ghostty closed"
fi

BIN="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
readonly BIN
[ -x "$BIN" ] || die "not an executable file: $1"
if ! "$BIN" whoami --json >/dev/null; then
  die "not logged in: run '$BIN login' first"
fi

mkdir -p "$2"
OUT_DIR="$(cd "$2" && pwd)"
readonly OUT_DIR

TMUX_BIN="$(command -v tmux)"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/dcs-verify-terminal.XXXXXX")"
TMUX_SOCKET="dcs-verify-$$"
SESSION=verify
readonly TMUX_BIN WORK_DIR TMUX_SOCKET SESSION

tm() {
  "$TMUX_BIN" -L "$TMUX_SOCKET" -f /dev/null "$@"
}

cleanup() {
  if tm has-session 2>/dev/null; then
    tm kill-server
  fi
  if pgrep -x ghostty >/dev/null; then
    quit_ghostty
  fi
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

swiftc -O -o "$WORK_DIR/winid" "$SCRIPT_DIR/terminal/winid.swift" ||
  die "could not compile scripts/terminal/winid.swift"

SERVER_ID="$("$BIN" servers list --json | jq -r '.[0].id // empty')"
if [ -z "$SERVER_ID" ]; then
  printf 'note: the account has no servers; servers info and the server detail screen are skipped\n'
fi

# open_ghostty <cols> <rows> <ghostty-theme> <runner-script>
# The user's Ghostty config is not loaded, so "dark" is Ghostty's own default
# and a local theme, font or padding cannot change what is being verified.
# The runner goes in --command, not -e: -e raises a modal "Allow Ghostty to
# execute" prompt that blocks the quit, and only a click can answer it.
open_ghostty() {
  local args=(
    --config-default-files=false
    --window-width="$1" --window-height="$2"
    --window-padding-x=16 --window-padding-y=12
    --window-save-state=never
    --confirm-close-surface=false
  )
  if [ -n "$3" ]; then
    args+=(--theme="$3")
  fi
  open -na "$GHOSTTY_APP" --args "${args[@]}" "--command=/bin/bash $(printf '%q' "$4")"
}

# The window lookup only sees the current desktop, so a window that opens on
# another Space is invisible to it. Activating pulls it here; the caller
# retries once with a fresh window before giving up.
wait_for_window() {
  local i id
  for ((i = 0; i < WAIT_STEPS; i++)); do
    id="$("$WORK_DIR/winid" | sort -k2 -n -r | head -1 | cut -d' ' -f1)"
    if [ -n "$id" ]; then
      printf '%s\n' "$id"
      return 0
    fi
    sleep 0.5
  done
  return 1
}

activate_ghostty() {
  if ! osascript -e 'tell application "Ghostty" to activate' >/dev/null 2>&1; then
    printf 'note: could not activate Ghostty; the window may be on another desktop\n'
  fi
}

wait_for_file() {
  local i
  for ((i = 0; i < WAIT_STEPS * 2; i++)); do
    [ -e "$1" ] && return 0
    sleep 0.5
  done
  die "timed out waiting for the commands to finish: $1"
}

# Ghostty can answer the quit event with -128 while it is already shutting
# down, so the exit of the process is the check, not osascript's status.
quit_ghostty() {
  local i msg rc=0
  msg="$(osascript -e 'tell application "Ghostty" to quit' 2>&1)" || rc=$?
  for ((i = 0; i < WAIT_STEPS; i++)); do
    pgrep -x ghostty >/dev/null || return 0
    sleep 0.5
  done
  die "Ghostty did not exit after quit (osascript exit $rc: $msg)"
}

WRITTEN=()

# capture <name> <window-id>
capture() {
  local out="$OUT_DIR/$1.png" msg rc=0
  msg="$(screencapture -x -o -l "$2" "$out" 2>&1)" || rc=$?
  case "$msg" in
    *"$LOCKED_SCREEN_MESSAGE"*)
      die "screencapture could not read the window: the screen is locked or asleep" ;;
  esac
  [ "$rc" -eq 0 ] || die "screencapture failed for $1: $msg"
  [ -s "$out" ] || die "screencapture wrote nothing for $1"
  WRITTEN+=("$out")
  printf 'captured %s\n' "$1"
}

# shoot_runner <name> <cols> <rows> <ghostty-theme> <runner> <done-marker>
# An empty marker means the window renders something already running (tmux).
shoot_runner() {
  local id="" attempt
  for attempt in 1 2; do
    open_ghostty "$2" "$3" "$4" "$5"
    activate_ghostty
    if id="$(wait_for_window)"; then
      break
    fi
    id=""
    printf 'note: no Ghostty window on attempt %s for %s; retrying with a fresh window\n' "$attempt" "$1"
    quit_ghostty
  done
  [ -n "$id" ] || die "no Ghostty window appeared for $1"
  if [ -n "$6" ]; then
    wait_for_file "$6"
  fi
  sleep 2
  capture "$1" "$id"
  quit_ghostty
}

write_commands_runner() {
  local runner="$1" marker="$2"
  {
    printf 'bin=%q\n' "$BIN"
    # shellcheck disable=SC2016 # the runner expands these, not this script
    printf 'show() { printf "\\$ dcs %%s\\n" "$*"; "$bin" "$@"; echo; }\n'
    printf 'show status\n'
    printf 'show servers list\n'
    if [ -n "$SERVER_ID" ]; then
      printf 'show servers info %q\n' "$SERVER_ID"
    fi
    printf 'show whoami\n'
    printf 'touch %q\n' "$marker"
    printf 'sleep 600\n'
  } >"$runner"
}

write_help_runner() {
  local runner="$1" marker="$2"
  {
    printf '%q --help\n' "$BIN"
    printf 'touch %q\n' "$marker"
    printf 'sleep 600\n'
  } >"$runner"
}

write_attach_runner() {
  printf 'exec %q -L %q attach -t %q\n' "$TMUX_BIN" "$TMUX_SOCKET" "$SESSION" >"$1"
}

send_keys() {
  local key
  for key in "$@"; do
    tm send-keys -t "$SESSION" "$key"
    sleep "$KEY_GAP"
  done
}

# Waits until the pane has shown the same text for three samples in a row,
# so a capture never lands on a spinner or a half-drawn view.
wait_pane_settled() {
  local i previous="" current same=0
  for ((i = 0; i < WAIT_STEPS; i++)); do
    sleep 0.5
    tm has-session -t "$SESSION" 2>/dev/null || die "dcs ui exited unexpectedly"
    current="$(tm capture-pane -p -t "$SESSION")"
    if [ -n "$current" ] && [ "$current" = "$previous" ]; then
      same=$((same + 1))
      [ "$same" -ge 2 ] && return 0
    else
      same=0
    fi
    previous="$current"
  done
  die "dcs ui never settled on a screen"
}

start_ui_session() {
  local cols="$1" mode="$2"
  tm new-session -d -s "$SESSION" -x "$cols" -y "$UI_ROWS" \
    "env TERM=xterm-256color COLORTERM=truecolor $(printf '%q' "$BIN") ui --welcome --theme $mode"
  tm set -s -a terminal-features ',xterm-ghostty:RGB'
  # Ghostty sometimes settles a few cells larger than asked (84x41 for 80x40).
  # The tmux window stays at the exact size; blank filler instead of dots
  # leaves one edge line that marks the real boundary in the screenshot.
  tm set -g fill-character ' '
  tm set -t "$SESSION" status off
  tm set -t "$SESSION" window-size manual
  tm resize-window -t "$SESSION" -x "$cols" -y "$UI_ROWS"
}

for theme in "${THEMES[@]}"; do
  mode="${theme%%:*}"
  ghostty_theme="${theme#*:}"
  for cols in "${WIDTHS[@]}"; do
    prefix="$mode-$cols"

    marker="$WORK_DIR/$prefix-commands.done"
    write_commands_runner "$WORK_DIR/commands.sh" "$marker"
    shoot_runner "$prefix-commands" "$cols" "$COMMAND_ROWS" "$ghostty_theme" "$WORK_DIR/commands.sh" "$marker"

    marker="$WORK_DIR/$prefix-help.done"
    write_help_runner "$WORK_DIR/help.sh" "$marker"
    shoot_runner "$prefix-help" "$cols" "$COMMAND_ROWS" "$ghostty_theme" "$WORK_DIR/help.sh" "$marker"

    write_attach_runner "$WORK_DIR/attach.sh"
    start_ui_session "$cols" "$mode"

    wait_pane_settled
    shoot_runner "$prefix-ui-overview" "$cols" "$UI_ROWS" "$ghostty_theme" "$WORK_DIR/attach.sh" ""

    send_keys 1 Enter
    wait_pane_settled
    shoot_runner "$prefix-ui-servers" "$cols" "$UI_ROWS" "$ghostty_theme" "$WORK_DIR/attach.sh" ""

    send_keys Enter
    wait_pane_settled
    if [ -n "$SERVER_ID" ]; then
      shoot_runner "$prefix-ui-detail" "$cols" "$UI_ROWS" "$ghostty_theme" "$WORK_DIR/attach.sh" ""
    fi

    send_keys Escape Escape 2 Enter
    wait_pane_settled
    shoot_runner "$prefix-ui-sites" "$cols" "$UI_ROWS" "$ghostty_theme" "$WORK_DIR/attach.sh" ""

    send_keys Escape Escape 3
    wait_pane_settled
    shoot_runner "$prefix-ui-databases" "$cols" "$UI_ROWS" "$ghostty_theme" "$WORK_DIR/attach.sh" ""

    tm kill-session -t "$SESSION"
  done
done

printf '\nfiles written:\n'
for file in "${WRITTEN[@]}"; do
  printf '  %s  %spx wide\n' "$file" "$(sips -g pixelWidth "$file" | awk '/pixelWidth/ {print $2}')"
done
