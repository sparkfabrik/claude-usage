BINARY_NAME   := claude-usage
EXT_UUID      := claude-usage@claude-code-usage
EXT_DIR       := $(HOME)/.local/share/gnome-shell/extensions/$(EXT_UUID)
INSTALL_DIR   := $(HOME)/.local/bin

.PHONY: build install install-binary install-gnome-extension install-kde install-waybar install-macos-tray uninstall uninstall-binary uninstall-gnome-extension reload-gnome-extension test-gnome-extension clean

## Build the Go binary
build:
	go build -o $(BINARY_NAME) ./cmd/claude-usage/

## Install everything
install: build install-binary install-gnome-extension

## Install the Go binary to ~/.local/bin
install-binary: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installed $(BINARY_NAME) to $(INSTALL_DIR)"
	@echo "Make sure $(INSTALL_DIR) is in your PATH"

## Install the GNOME Shell extension
install-gnome-extension:
	mkdir -p $(EXT_DIR)
	cp readers/gnome-shell-extension/extension.js $(EXT_DIR)/
	cp readers/gnome-shell-extension/metadata.json $(EXT_DIR)/
	cp readers/gnome-shell-extension/sparkle.svg $(EXT_DIR)/
	@echo "Extension installed to $(EXT_DIR)"
	@echo "Enable with: gnome-extensions enable $(EXT_UUID)"

## Reload GNOME Shell extension (disable + enable)
reload-gnome-extension:
	gnome-extensions disable $(EXT_UUID) 2>/dev/null || true
	sleep 1
	gnome-extensions enable $(EXT_UUID)

## Run a nested GNOME Shell on Wayland for testing (requires mutter-devkit)
test-gnome-extension: install-gnome-extension
	dbus-run-session gnome-shell --devkit --wayland

## Uninstall everything
uninstall: uninstall-binary uninstall-gnome-extension

## Remove the binary
uninstall-binary:
	rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Removed $(BINARY_NAME) from $(INSTALL_DIR)"

## Remove the GNOME Shell extension
uninstall-gnome-extension:
	gnome-extensions disable $(EXT_UUID) 2>/dev/null || true
	rm -rf $(EXT_DIR)
	@echo "Removed extension $(EXT_UUID)"

## Remove build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f claude-usage-tray

## Install KDE Plasma 6 plasmoid
install-kde:
	mkdir -p $(HOME)/.local/share/plasma/plasmoids/org.kde.plasma.claude-usage/contents/ui
	cp readers/kde-plasmoid/metadata.json $(HOME)/.local/share/plasma/plasmoids/org.kde.plasma.claude-usage/
	cp readers/kde-plasmoid/contents/ui/main.qml $(HOME)/.local/share/plasma/plasmoids/org.kde.plasma.claude-usage/contents/ui/
	@echo "KDE plasmoid installed. Add 'Claude Usage' widget to your panel."

## Install Waybar module
install-waybar:
	mkdir -p $(INSTALL_DIR)
	cp readers/waybar/claude-usage-waybar.sh $(INSTALL_DIR)/
	chmod +x $(INSTALL_DIR)/claude-usage-waybar.sh
	@echo "Waybar module installed to $(INSTALL_DIR)/claude-usage-waybar.sh"
	@echo "Add custom/claude-usage module to your Waybar config"

## Build and install macOS tray app
install-macos-tray:
	CGO_ENABLED=1 go build -o claude-usage-tray ./readers/macos-tray/
	cp claude-usage-tray /usr/local/bin/
	@echo "macOS tray installed to /usr/local/bin/claude-usage-tray"
