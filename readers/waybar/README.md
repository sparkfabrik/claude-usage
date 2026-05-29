# Claude Usage Waybar Module

Custom Waybar module that displays Claude Code API utilization.

## Requirements

- Waybar
- `claude-usage` binary installed (in PATH or `~/.local/bin/`)
- Python 3 (for JSON parsing)

## Install

```bash
# Via the installer (recommended):
curl -fsSL https://raw.githubusercontent.com/sparkfabrik/claude-usage/main/install.sh | bash

# Or manually:
cp readers/waybar/claude-usage-waybar.sh ~/.local/bin/
chmod +x ~/.local/bin/claude-usage-waybar.sh
```

## Waybar Configuration

Add to your Waybar config (`~/.config/waybar/config`):

```jsonc
// Add "custom/claude-usage" to your modules list:
"modules-right": ["custom/claude-usage", "clock", ...]

// Add this module definition:
"custom/claude-usage": {
    "exec": "~/.local/bin/claude-usage-waybar.sh",
    "return-type": "json",
    "interval": 60,
    "tooltip": true
}
```

## Styling

Add to `~/.config/waybar/style.css`:

```css
#custom-claude-usage {
    color: #D97757;  /* Claude orange */
    font-weight: bold;
    padding: 0 8px;
}

#custom-claude-usage.warning {
    color: #FFA500;
}

#custom-claude-usage.critical {
    color: #FF4444;
    animation: blink 1s ease-in-out infinite;
}

#custom-claude-usage.error {
    color: #888888;
    font-style: italic;
}

@keyframes blink {
    50% { opacity: 0.5; }
}
```

## How It Works

- Called by Waybar every 60 seconds
- Runs `claude-usage --status` and parses the JSON output
- Displays glyph + percentages: `◑ 5h:42% 7d:67%`
- Glyphs: ◔ (<50%), ◑ (50-74%), ◕ (75-94%), ● (>=95%)
- CSS classes: `normal`, `warning`, `critical`, `error`
- Tooltip shows reset times
- Outputs nothing when CLI missing or Claude not running (hides module)
- Uses Python for JSON output (proper escaping, no `jq` dependency)
