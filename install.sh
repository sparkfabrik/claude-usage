#!/usr/bin/env bash
# claude-usage installer — idempotent install/update/uninstall.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/sparkfabrik/claude-usage/main/install.sh | bash
#   curl -fsSL ... | CLAUDE_USAGE_VERSION=v1.0.0 bash
#   INSTALL_DIR=/custom/path ./install.sh
#   ./install.sh --uninstall
#
# Environment variables:
#   CLAUDE_USAGE_VERSION  — Version tag to install (default: latest release)
#   INSTALL_DIR           — Installation directory (default: ~/.local/share/claude-usage)
#
# Output protocol (for Ansible integration):
#   CHANGED: <description>  — printed for each mutation
#   OK: already up to date  — printed when nothing changed
#   Exit 0 = success, Exit 1 = error
#
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/share/claude-usage}"
BIN_DIR="$HOME/.local/bin"
REPO="sparkfabrik/claude-usage"
PLATFORM="$(uname -s)"    # Darwin or Linux
ARCH="$(uname -m)"        # x86_64 or arm64/aarch64
CHANGED=0

# --- Helpers --------------------------------------------------------------
changed() {
  echo "CHANGED: $1"
  CHANGED=1
}

die() {
  echo "ERROR: $1" >&2
  exit 1
}

# Normalize arch to goreleaser naming
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       die "Unsupported architecture: $ARCH" ;;
esac

# Normalize OS to goreleaser naming
case "$PLATFORM" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux" ;;
  *)      die "Unsupported OS: $PLATFORM" ;;
esac

# Download helper (curl preferred, wget fallback)
download() {
  local url="$1" dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$dest" "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url"
  else
    die "Neither curl nor wget found. Install one and retry."
  fi
}

# --- Uninstall ------------------------------------------------------------
if [[ "${1:-}" == "--uninstall" ]]; then
  # Remove symlinks
  rm -f "$BIN_DIR/claude-usage"
  rm -f "$BIN_DIR/claude-usage-tray"
  rm -f "$BIN_DIR/claude-usage-waybar.sh"
  rm -f "$BIN_DIR/claude-usage-statusline.sh"

  # Remove statusLine from Claude Code settings
  SETTINGS="$HOME/.claude/settings.json"
  if [ -f "$SETTINGS" ] && grep -q "claude-usage-statusline" "$SETTINGS" 2>/dev/null; then
    python3 - "$SETTINGS" <<'PYEOF2'
import json, sys
path = sys.argv[1]
with open(path) as f:
    cfg = json.load(f)
if "statusLine" in cfg and "claude-usage" in cfg.get("statusLine", {}).get("command", ""):
    del cfg["statusLine"]
with open(path, "w") as f:
    json.dump(cfg, f, indent=2)
    f.write("\n")
PYEOF2
    changed "statusLine removed from $SETTINGS"
  fi

  # Remove session hooks from settings.json
  HOOKS_DIR="$INSTALL_DIR/hooks"
  if [ -f "$SETTINGS" ] && grep -q "$HOOKS_DIR/start.sh" "$SETTINGS" 2>/dev/null; then
    python3 - "$SETTINGS" "$HOOKS_DIR/start.sh" "$HOOKS_DIR/stop.sh" <<'PYEOF_UNHOOK'
import json, sys

path, start_cmd, stop_cmd = sys.argv[1], sys.argv[2], sys.argv[3]
with open(path) as f:
    cfg = json.load(f)

hooks = cfg.get("hooks", {})
for section, cmd in [("SessionStart", start_cmd), ("SessionEnd", stop_cmd)]:
    entries = hooks.get(section, [])
    cleaned = []
    for e in entries:
        if isinstance(e, dict):
            inner = [h for h in e.get("hooks", [])
                     if not (isinstance(h, dict) and h.get("command") == cmd)]
            if inner:
                cleaned.append({**e, "hooks": inner})
        else:
            cleaned.append(e)
    hooks[section] = cleaned

with open(path, "w") as f:
    json.dump(cfg, f, indent=2)
    f.write("\n")
PYEOF_UNHOOK
    changed "session hooks removed from $SETTINGS"
  fi

  # Kill tray if running
  pkill -x "claude-usage-tray" 2>/dev/null || true
  rm -rf /tmp/claude-usage-sessions 2>/dev/null || true

  # Remove GNOME extension symlink
  EXT_DIR="$HOME/.local/share/gnome-shell/extensions/claude-usage@claude-code-usage"
  if [ -L "$EXT_DIR" ]; then
    rm -f "$EXT_DIR"
    changed "GNOME extension symlink removed"
  fi

  # Remove KDE plasmoid
  if command -v kpackagetool6 >/dev/null 2>&1; then
    if kpackagetool6 --type Plasma/Applet --show org.kde.plasma.claude-usage >/dev/null 2>&1; then
      kpackagetool6 --type Plasma/Applet --remove org.kde.plasma.claude-usage 2>/dev/null && changed "KDE plasmoid removed"
    fi
  fi

  # Waybar hint
  if [ "$PLATFORM" != "Darwin" ] && command -v waybar >/dev/null 2>&1; then
    WAYBAR_CFG="${XDG_CONFIG_HOME:-$HOME/.config}/waybar/config"
    [ ! -f "$WAYBAR_CFG" ] && WAYBAR_CFG="${XDG_CONFIG_HOME:-$HOME/.config}/waybar/config.jsonc"
    if [ -f "$WAYBAR_CFG" ] && grep -q "claude-usage" "$WAYBAR_CFG" 2>/dev/null; then
      echo ""
      echo "NOTE: Remove 'custom/claude-usage' from your Waybar config manually:"
      echo "  $WAYBAR_CFG"
      echo ""
    fi
  fi

  # Remove install directory
  if [ -d "$INSTALL_DIR" ]; then
    rm -rf "$INSTALL_DIR"
    changed "removed $INSTALL_DIR"
  fi

  if [ "$CHANGED" -eq 0 ]; then
    echo "OK: nothing to uninstall"
  else
    echo "claude-usage uninstalled."
  fi
  exit 0
fi

# --- Version resolution ---------------------------------------------------
if [ -z "${CLAUDE_USAGE_VERSION:-}" ]; then
  CLAUDE_USAGE_VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/') \
    || die "Failed to fetch latest release. Set CLAUDE_USAGE_VERSION manually."
  [ -z "$CLAUDE_USAGE_VERSION" ] && die "Could not determine latest version. Set CLAUDE_USAGE_VERSION manually."
fi

# --- Version skip check ---------------------------------------------------
CURRENT_VERSION=""
if [ -f "$INSTALL_DIR/.version" ]; then
  CURRENT_VERSION=$(cat "$INSTALL_DIR/.version")
fi

# --- Download binaries ----------------------------------------------------
if [ "$CURRENT_VERSION" = "$CLAUDE_USAGE_VERSION" ]; then
  SKIP_DOWNLOAD=true
else
  SKIP_DOWNLOAD=false
fi

if [ "$SKIP_DOWNLOAD" = false ]; then
RELEASE_URL="https://github.com/$REPO/releases/download/$CLAUDE_USAGE_VERSION"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

# CLI binary
BINARY_NAME="claude-usage_${OS}_${ARCH}"
download "$RELEASE_URL/$BINARY_NAME" "$TMP_DIR/claude-usage" \
  || die "Failed to download CLI binary: $RELEASE_URL/$BINARY_NAME"

# macOS tray binary
if [ "$OS" = "darwin" ]; then
  TRAY_NAME="claude-usage-tray_${OS}_${ARCH}"
  download "$RELEASE_URL/$TRAY_NAME" "$TMP_DIR/claude-usage-tray" \
    || die "Failed to download tray binary: $RELEASE_URL/$TRAY_NAME"
fi

# Readers archive
download "$RELEASE_URL/claude-usage-readers.tar.gz" "$TMP_DIR/readers.tar.gz" \
  || die "Failed to download readers archive"

# --- Install binaries -----------------------------------------------------
mkdir -p "$INSTALL_DIR/bin"
mv "$TMP_DIR/claude-usage" "$INSTALL_DIR/bin/claude-usage"
chmod +x "$INSTALL_DIR/bin/claude-usage"

if [ "$OS" = "darwin" ]; then
  mv "$TMP_DIR/claude-usage-tray" "$INSTALL_DIR/bin/claude-usage-tray"
  chmod +x "$INSTALL_DIR/bin/claude-usage-tray"
fi

changed "installed binaries ($CLAUDE_USAGE_VERSION)"

# --- Install readers ------------------------------------------------------
mkdir -p "$INSTALL_DIR/readers"
tar xzf "$TMP_DIR/readers.tar.gz" -C "$INSTALL_DIR/readers"
chmod +x "$INSTALL_DIR/readers/waybar/claude-usage-waybar.sh" 2>/dev/null || true

# --- Create symlinks ------------------------------------------------------
mkdir -p "$BIN_DIR"
ln -sf "$INSTALL_DIR/bin/claude-usage" "$BIN_DIR/claude-usage"

if [ "$OS" = "darwin" ]; then
  ln -sf "$INSTALL_DIR/bin/claude-usage-tray" "$BIN_DIR/claude-usage-tray"
fi

# PATH warning
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *)
    echo ""
    echo "WARNING: $BIN_DIR is not in your PATH."
    echo "Add this to your shell profile:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    echo ""
    ;;
esac

fi

# --- Reader detection and install -----------------------------------------
READER="none"
SETTINGS="$HOME/.claude/settings.json"

if [ "$OS" = "darwin" ]; then
  READER="macos"
elif command -v gnome-shell >/dev/null 2>&1; then
  READER="gnome"
elif command -v plasmashell >/dev/null 2>&1; then
  READER="kde"
elif command -v waybar >/dev/null 2>&1; then
  READER="waybar"
fi

case "$READER" in
  macos)
    # Tray already installed above; register session hooks
    HOOKS_DIR="$INSTALL_DIR/hooks"
    HOOKS_SRC="$INSTALL_DIR/readers/hooks"
    mkdir -p "$HOOKS_DIR"
    if [ -d "$HOOKS_SRC" ]; then
      cp "$HOOKS_SRC/start.sh" "$HOOKS_DIR/start.sh"
      cp "$HOOKS_SRC/stop.sh" "$HOOKS_DIR/stop.sh"
      chmod +x "$HOOKS_DIR/start.sh" "$HOOKS_DIR/stop.sh"
    else
      echo "WARNING: hooks source not found at $HOOKS_SRC"
      echo "  Contents of $INSTALL_DIR/readers/:"
      ls "$INSTALL_DIR/readers/" 2>&1 || true
    fi

    # Register hooks in ~/.claude/settings.json (if hook scripts exist)
    START_CMD="$HOOKS_DIR/start.sh"
    STOP_CMD="$HOOKS_DIR/stop.sh"

    if [ -x "$START_CMD" ] && [ -x "$STOP_CMD" ]; then
      HOOKS_NEEDED=false

      if [ ! -f "$SETTINGS" ]; then
        mkdir -p "$(dirname "$SETTINGS")"
        echo '{}' > "$SETTINGS"
        HOOKS_NEEDED=true
      elif ! grep -q "$HOOKS_DIR/start.sh" "$SETTINGS" 2>/dev/null; then
        HOOKS_NEEDED=true
      fi

      if [ "$HOOKS_NEEDED" = true ]; then
        python3 - "$SETTINGS" "$START_CMD" "$STOP_CMD" <<'PYEOF_HOOKS'
import json, sys

path, start_cmd, stop_cmd = sys.argv[1], sys.argv[2], sys.argv[3]
with open(path) as f:
    cfg = json.load(f)

hooks = cfg.setdefault("hooks", {})

def add_hook(section, cmd):
    entries = hooks.setdefault(section, [])
    # Remove stale entries for this command, then re-add
    for e in entries[:]:
        if not isinstance(e, dict):
            continue
        e["hooks"] = [h for h in e.get("hooks", []) if not (isinstance(h, dict) and h.get("command") == cmd)]
        if not e.get("hooks"):
            entries.remove(e)
    entries.append({"hooks": [{"type": "command", "command": cmd, "timeout": 5}]})

add_hook("SessionStart", start_cmd)
add_hook("SessionEnd", stop_cmd)

with open(path, "w") as f:
    json.dump(cfg, f, indent=2)
    f.write("\n")
PYEOF_HOOKS
        changed "session hooks registered"
      fi
    fi

    changed "macOS tray reader installed"
    echo ""
    echo "macOS tray app installed with session hooks."
    echo "The tray will auto-start/stop with Claude Code sessions."
    echo ""
    ;;
  gnome)
    EXT_DIR="$HOME/.local/share/gnome-shell/extensions/claude-usage@claude-code-usage"
    mkdir -p "$(dirname "$EXT_DIR")"
    ln -sfn "$INSTALL_DIR/readers/gnome-shell-extension" "$EXT_DIR"
    changed "GNOME extension installed"
    echo ""
    echo "GNOME extension installed! Enable with:"
    echo "  gnome-extensions enable claude-usage@claude-code-usage"
    echo ""
    ;;
  kde)
    PLASMOID_DIR="$INSTALL_DIR/readers/kde-plasmoid"
    if command -v kpackagetool6 >/dev/null 2>&1; then
      if kpackagetool6 --type Plasma/Applet --show org.kde.plasma.claude-usage >/dev/null 2>&1; then
        kpackagetool6 --type Plasma/Applet --upgrade "$PLASMOID_DIR" 2>/dev/null && changed "KDE plasmoid upgraded"
      else
        kpackagetool6 --type Plasma/Applet --install "$PLASMOID_DIR" && changed "KDE plasmoid installed"
      fi
      echo ""
      echo "KDE plasmoid installed! Add 'Claude Usage' widget to your panel."
      echo ""
    else
      echo ""
      echo "KDE detected but kpackagetool6 not found."
      echo "Install plasmoid manually from: $PLASMOID_DIR"
      echo ""
    fi
    ;;
  waybar)
    ln -sf "$INSTALL_DIR/readers/waybar/claude-usage-waybar.sh" "$BIN_DIR/claude-usage-waybar.sh"
    changed "Waybar reader installed"
    echo ""
    echo "Waybar module installed! Add to your Waybar config:"
    echo ""
    echo '  "custom/claude-usage": {'
    echo "      \"exec\": \"$BIN_DIR/claude-usage-waybar.sh\","
    echo '      "return-type": "json",'
    echo '      "interval": 60'
    echo '  }'
    echo ""
    ;;
  none)
    echo ""
    echo "NOTE: No desktop environment detected (GNOME/KDE/Waybar)."
    echo "CLI binary installed — use 'claude-usage' directly."
    echo ""
    ;;
esac

# --- Claude Code statusline ------------------------------------------------
STATUSLINE_SCRIPT="$INSTALL_DIR/readers/statusline/claude-usage-statusline.sh"
SETTINGS="$HOME/.claude/settings.json"
ln -sf "$STATUSLINE_SCRIPT" "$BIN_DIR/claude-usage-statusline.sh"

if [ -f "$STATUSLINE_SCRIPT" ]; then
  STATUSLINE_CMD="bash $BIN_DIR/claude-usage-statusline.sh"
  STATUSLINE_NEEDED=false

  if [ ! -f "$SETTINGS" ]; then
    mkdir -p "$(dirname "$SETTINGS")"
    echo '{}' > "$SETTINGS"
    STATUSLINE_NEEDED=true
  elif ! grep -q "claude-usage-statusline" "$SETTINGS" 2>/dev/null; then
    STATUSLINE_NEEDED=true
  fi

  if [ "$STATUSLINE_NEEDED" = true ]; then
    python3 - "$SETTINGS" "$STATUSLINE_CMD" <<'PYEOF2'
import json, sys

path, cmd = sys.argv[1], sys.argv[2]
with open(path) as f:
    cfg = json.load(f)

cfg["statusLine"] = {"type": "command", "command": cmd}

with open(path, "w") as f:
    json.dump(cfg, f, indent=2)
    f.write("\n")
PYEOF2
    changed "statusLine registered in $SETTINGS"
  fi
fi

# --- Finalize -------------------------------------------------------------
echo "$CLAUDE_USAGE_VERSION" > "$INSTALL_DIR/.version"
cp "$0" "$INSTALL_DIR/install.sh" 2>/dev/null || true

if [ "$CHANGED" -eq 0 ]; then
  echo "OK: already up to date"
fi
exit 0
