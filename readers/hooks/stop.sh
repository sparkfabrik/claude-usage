#!/usr/bin/env bash
# SessionEnd hook — kill claude-usage-tray only when last Claude Code session closes.
[ "$(uname -s)" != "Darwin" ] && exit 0

STDIN_DATA=$(cat)
SESSION_ID=$(echo "$STDIN_DATA" | python3 -c "import sys,json; print(json.load(sys.stdin).get('session_id',''))" 2>/dev/null)

[ -n "$SESSION_ID" ] && rm -f "/tmp/claude-usage-sessions/$SESSION_ID"
remaining=$(ls /tmp/claude-usage-sessions/ 2>/dev/null | wc -l | tr -d ' ')

[ "$remaining" -gt 0 ] && exit 0
pkill -f "claude-usage-tray" 2>/dev/null || true
