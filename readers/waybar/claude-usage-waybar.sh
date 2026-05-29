#!/usr/bin/env bash
# claude-usage-waybar.sh — Waybar custom module for Claude Code usage
# Outputs JSON in Waybar format: {"text": "...", "tooltip": "...", "class": "..."}

set -euo pipefail

# Find claude-usage binary
if command -v claude-usage &>/dev/null; then
    BIN="claude-usage"
elif [[ -x "$HOME/.local/bin/claude-usage" ]]; then
    BIN="$HOME/.local/bin/claude-usage"
else
    # CLI not found — output nothing (hides Waybar module)
    exit 0
fi

# Call --status, exit silently on error
STATUS=$("$BIN" --status 2>/dev/null) || exit 0

# Parse JSON fields
c_pct=$(echo "$STATUS" | jq -r '.c_pct // 0')
w_pct=$(echo "$STATUS" | jq -r '.w_pct // 0')
c_reset=$(echo "$STATUS" | jq -r '.c_reset // "?"')
w_reset=$(echo "$STATUS" | jq -r '.w_reset // "?"')
stale=$(echo "$STATUS" | jq -r '.stale // false')
claude_running=$(echo "$STATUS" | jq -r '.claude_running // false')
error=$(echo "$STATUS" | jq -r '.error // ""')

# If Claude not running and no error, optionally hide
if [[ "$claude_running" == "false" && "$error" == "" ]]; then
    exit 0
fi

# Glyph selection based on c_pct
if (( c_pct >= 95 )); then
    glyph="●"
elif (( c_pct >= 75 )); then
    glyph="◕"
elif (( c_pct >= 50 )); then
    glyph="◑"
else
    glyph="◔"
fi

# CSS class logic
if [[ "$stale" == "true" ]]; then
    class="error"
elif (( c_pct >= 95 )); then
    class="critical"
elif (( c_pct >= 75 )); then
    class="warning"
else
    class="normal"
fi

# Build text
text="$glyph 5h:${c_pct}% 7d:${w_pct}%"
if [[ "$stale" == "true" ]]; then
    text="$text ?"
fi

# Build tooltip
tooltip="Claude Code Quota\n━━━━━━━━━━━━━━━━\n5h: ${c_pct}% (resets in ${c_reset})\n7d: ${w_pct}% (resets in ${w_reset})"
if [[ -n "$error" ]]; then
    tooltip="$tooltip\n\nError: $error"
fi

# Output Waybar JSON
printf '{"text": "%s", "tooltip": "%s", "class": "%s"}\n' "$text" "$tooltip" "$class"
