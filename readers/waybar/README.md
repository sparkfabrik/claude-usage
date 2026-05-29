# Claude Usage Waybar Module

Custom Waybar module that displays Claude Code API utilization.

## Requirements

- Waybar
- `claude-usage` binary installed (in PATH or `~/.local/bin/`)
- `jq` for JSON parsing

## Install

```bash
# From repo root
make install-waybar

# Or manually:
cp readers/waybar/claude-usage-waybar.sh ~/.local/bin/
chmod +x ~/.local/bin/claude-usage-waybar.sh
```

## Waybar Configuration

Add to your Waybar config (`~/.config/waybar/config`):

```json
"custom/claude-usage": {
    "exec": "~/.local/bin/claude-usage-waybar.sh",
    "return-type": "json",
    "interval": 60
}
```

Add to your modules list:

```json
"modules-right": ["custom/claude-usage", ...]
```

## Styling

Add to `~/.config/waybar/style.css`:

```css
#custom-claude-usage.normal {
    color: #32c850;
}
#custom-claude-usage.warning {
    color: #f0c020;
}
#custom-claude-usage.critical {
    color: #dc3232;
}
#custom-claude-usage.error {
    color: #888888;
}
```

## How It Works

- Called by Waybar every 60 seconds
- Displays glyph + percentages: `◑ 5h:42% 7d:67%`
- Glyphs: ◔ (<50%), ◑ (50-74%), ◕ (75-94%), ● (>=95%)
- CSS classes: normal, warning, critical, error
- Tooltip shows reset times
- Outputs nothing when CLI missing or Claude not running (hides module)
