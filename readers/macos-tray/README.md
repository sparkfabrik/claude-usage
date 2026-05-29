# Claude Usage macOS Tray

macOS menu bar app that displays Claude Code API utilization.

## Requirements

- macOS (darwin)
- `claude-usage` binary installed (in PATH, `~/.local/bin/`, or `/usr/local/bin/`)
- CGo (for building — uses system tray bindings)

## Build

```bash
# From repo root (macOS only)
make install-macos-tray

# Or manually:
go build -o claude-usage-tray ./cmd/claude-usage-tray/
cp claude-usage-tray /usr/local/bin/
```

## Install as Login Item

To start automatically on login, add `claude-usage-tray` to System Settings > General > Login Items.

## How It Works

- Displays "C:X% W:Y%" in the macOS menu bar
- Polls `claude-usage --status` every 60 seconds
- Click to see dropdown with:
  - 5h utilization + reset time
  - 7d utilization + reset time
  - "Refresh Now" (triggers `--force-poll`)
  - "Quit"
- Shows "C:--" when Claude is not running
- Shows "C:? " suffix when data is stale

## Binary Lookup Order

1. `$PATH`
2. `~/.local/bin/claude-usage`
3. `/usr/local/bin/claude-usage`
