#!/bin/bash
# claude-usage-statusline.sh — Claude Code statusline badge
# Reads quota data from claude-usage --status and outputs ANSI-colored status text.
#
# Usage in ~/.claude/settings.json:
#   "statusLine": { "type": "command", "command": "bash ~/.local/bin/claude-usage-statusline.sh" }
#
# Outputs nothing if CLI is missing or returns error.
# Appends ? if data is stale.

# Find claude-usage binary
if command -v claude-usage >/dev/null 2>&1; then
    BIN="claude-usage"
elif [ -x "$HOME/.local/bin/claude-usage" ]; then
    BIN="$HOME/.local/bin/claude-usage"
else
    exit 0
fi

# Call --status, exit silently on error
STATUS=$("$BIN" --status 2>/dev/null) || exit 0
[ -z "$STATUS" ] && exit 0

# Parse and output ANSI-colored text
python3 - "$STATUS" <<'PYEOF'
import json, sys

try:
    d = json.loads(sys.argv[1])
except Exception:
    sys.exit(0)

c_pct = int(d.get("c_pct", 0))
w_pct = int(d.get("w_pct", 0))
stale = bool(d.get("stale", False))
claude_running = d.get("claude_running", False)

# Hide if Claude not running
if not claude_running:
    sys.exit(0)

# Glyph based on 5h utilization
if c_pct < 50:
    glyph = "◔"
elif c_pct < 75:
    glyph = "◑"
elif c_pct < 95:
    glyph = "◕"
else:
    glyph = "●"

stale_suffix = " ?" if stale else ""

# Surface the fullest model-scoped window, if any. A model can be nearly
# exhausted while the overall weekly figure still looks calm, and that is
# exactly the case worth a warning in a one-line badge.
binding = ""
scoped = [limit for limit in (d.get("limits") or []) if limit.get("model")]
if scoped:
    worst = max(scoped, key=lambda limit: int(limit.get("pct", 0)))
    if int(worst.get("pct", 0)) >= max(w_pct, c_pct):
        binding = f" {worst.get('model', '?')}:{int(worst.get('pct', 0))}%"

# Output with ANSI orange (color 172)
print(f"\033[38;5;172m{glyph} 5h:{c_pct}% 7d:{w_pct}%{binding}{stale_suffix}\033[0m", end="")
PYEOF
