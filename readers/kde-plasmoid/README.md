# Claude Usage KDE Plasmoid

KDE Plasma 6 panel widget that displays Claude Code API utilization.

## Requirements

- KDE Plasma 6
- `claude-usage` binary installed (in PATH or `~/.local/bin/`)

## Install

```bash
# From repo root
make install-kde

# Or manually:
mkdir -p ~/.local/share/plasma/plasmoids/org.kde.plasma.claude-usage/
cp -r readers/kde-plasmoid/* ~/.local/share/plasma/plasmoids/org.kde.plasma.claude-usage/
```

Then add the "Claude Usage" widget to your panel via Plasma's widget picker.

## How It Works

- Calls `claude-usage --status` every 60 seconds
- Displays "C:X% W:Y%" in the panel (5h and 7d utilization)
- Click to expand dropdown with reset times and error details
- Stale data shown at 50% opacity
- Hides when Claude is not running

## Binary Lookup

1. Checks `$PATH` for `claude-usage`
2. Falls back to `~/.local/bin/claude-usage`
