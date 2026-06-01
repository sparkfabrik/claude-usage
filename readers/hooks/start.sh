#!/usr/bin/env bash
# SessionStart hook — launch claude-usage-tray if not already running (macOS only)
[ "$(uname -s)" != "Darwin" ] && exit 0

# Read entire stdin (until EOF) with timeout to avoid blocking if Claude Code
# doesn't close stdin. -d '' reads past newlines so multiline JSON is captured.
STDIN_DATA=""
IFS= read -t 2 -r -d '' STDIN_DATA || true

SESSION_ID=$(echo "$STDIN_DATA" | python3 -c "import sys,json; print(json.load(sys.stdin).get('session_id',''))" 2>/dev/null || true)

# Sanitize session ID — allow only safe filename characters
SESSION_ID=$(echo "$SESSION_ID" | tr -cd 'A-Za-z0-9._-')

# Track session if we got an ID, but don't abort if we didn't
if [ -n "$SESSION_ID" ]; then
  mkdir -p /tmp/claude-usage-sessions
  touch "/tmp/claude-usage-sessions/$SESSION_ID"
fi

pgrep -x "claude-usage-tray" >/dev/null 2>&1 && exit 0

# Find the tray binary
TRAY_BIN=""
if command -v claude-usage-tray >/dev/null 2>&1; then
  TRAY_BIN="claude-usage-tray"
elif [ -x "$HOME/.local/bin/claude-usage-tray" ]; then
  TRAY_BIN="$HOME/.local/bin/claude-usage-tray"
fi

[ -z "$TRAY_BIN" ] && exit 0

nohup "$TRAY_BIN" < /dev/null >> /tmp/claude-usage-tray.log 2>&1 &
