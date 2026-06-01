#!/usr/bin/env bash
# SessionEnd hook — kill claude-usage-tray only when last Claude Code session closes.
[ "$(uname -s)" != "Darwin" ] && exit 0

# Read entire stdin (until EOF) with timeout to avoid blocking. -d '' reads past
# newlines so multiline JSON is captured.
STDIN_DATA=""
IFS= read -t 2 -r -d '' STDIN_DATA || true

SESSION_ID=$(echo "$STDIN_DATA" | python3 -c "import sys,json; print(json.load(sys.stdin).get('session_id',''))" 2>/dev/null || true)

# Sanitize session ID — allow only safe filename characters
SESSION_ID=$(echo "$SESSION_ID" | tr -cd 'A-Za-z0-9._-')

[ -n "$SESSION_ID" ] && rm -f "/tmp/claude-usage-sessions/$SESSION_ID"
remaining=$(ls /tmp/claude-usage-sessions/ 2>/dev/null | wc -l | tr -d ' ')

[ "$remaining" -gt 0 ] && exit 0
pkill -x "claude-usage-tray" 2>/dev/null || true
