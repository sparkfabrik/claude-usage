#!/bin/bash
# claude-usage-waybar.sh — Waybar custom module for Claude Code quota
# Outputs JSON in Waybar's custom module format.
#
# Waybar config (~/.config/waybar/config):
#   "custom/claude-usage": {
#       "exec": "~/.local/bin/claude-usage-waybar.sh",
#       "return-type": "json",
#       "interval": 60,
#       "tooltip": true
#   }
#
# Outputs nothing if CLI is missing or Claude is not running.

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

# Parse and produce Waybar JSON entirely in Python to avoid
# shell interpolation issues with JSON escaping.
python3 - "$STATUS" <<'PYEOF'
import json, sys

try:
    d = json.loads(sys.argv[1])
except Exception:
    sys.exit(0)

c_pct = int(d.get("c_pct", 0))
w_pct = int(d.get("w_pct", 0))
c_reset = str(d.get("c_reset", "?"))
w_reset = str(d.get("w_reset", "?"))
stale = bool(d.get("stale", False))
claude_running = d.get("claude_running", False)
auth = str(d.get("auth", "unknown"))
error = str(d.get("error", ""))

# Hide if Claude not running and no error
if not claude_running and not error:
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

# CSS class
if auth in ("expired", "missing"):
    css_class = "error"
elif stale:
    css_class = "error"
elif c_pct >= 95:
    css_class = "critical"
elif c_pct >= 75:
    css_class = "warning"
else:
    css_class = "normal"

# Build text
stale_suffix = " ?" if stale else ""
text = f"{glyph} 5h:{c_pct}% 7d:{w_pct}%{stale_suffix}"

def fmt_tokens(n):
    n = int(n or 0)
    if n >= 1_000_000_000:
        return f"{n / 1_000_000_000:.1f}B"
    if n >= 1_000_000:
        return f"{n / 1_000_000:.1f}M"
    if n >= 1_000:
        return f"{n / 1_000:.1f}K"
    return str(n)

# Build tooltip
lines = [
    "Claude Code Quota",
    "━━━━━━━━━━━━━━━━",
]

# limits[] carries the model-scoped windows the OAuth source reports; older
# CLI versions and the header fallback only ever return the two flat windows,
# so fall back to those.
limits = d.get("limits") or []
if limits:
    for limit in limits:
        lines.append(f"{limit.get('title', '?')}: {int(limit.get('pct', 0))}%"
                     f" (resets in {limit.get('reset', '?')})")
else:
    lines.append(f"5h: {c_pct}% (resets in {c_reset})")
    lines.append(f"7d: {w_pct}% (resets in {w_reset})")

today = d.get("today") or {}
if today:
    lines.append("")
    lines.append(f"Today: {fmt_tokens(today.get('tokens'))} tokens"
                 f" · {int(today.get('messages', 0))} messages"
                 f" · {int(today.get('sessions', 0))} sessions")

models = d.get("models") or []
if models:
    lines.append("")
    lines.append("Top models")
    for model in models[:4]:
        lines.append(f"  {model.get('name', '?')}: {fmt_tokens(model.get('tokens'))}")
if auth == "expired":
    lines.append("Auth expired — run Claude Code to refresh")
elif auth == "missing":
    lines.append("No credentials found")
if error:
    lines.append(f"Error: {error}")
if stale:
    lines.append("⚠ Data may be stale")
tooltip = "\n".join(lines)

# Output valid JSON via json.dumps (handles escaping)
print(json.dumps({"text": text, "tooltip": tooltip, "class": css_class}))
PYEOF
