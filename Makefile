BINARY_NAME   := claude-usage
VERSION       := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT        := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS       := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE) -X main.builtBy=make
EXT_UUID      := claude-usage@claude-code-usage
EXT_DIR       := $(HOME)/.local/share/gnome-shell/extensions/$(EXT_UUID)
INSTALL_DIR   := $(HOME)/.local/bin

# Tool versions (pinned for reproducibility)
# https://github.com/golangci/golangci-lint/releases
GOLANGCI_LINT_VERSION := 2.12.2

# Local tool binaries
BIN_DIR       := $(CURDIR)/.bin
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint

# Detect OS and architecture for tool downloads
OS            := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH          := $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')

.PHONY: build-cli test-cli test-cli-short test-cli-cover lint-cli install-cli install-gnome-extension install-kde install-waybar install-macos-tray uninstall-cli uninstall-gnome-extension reload-gnome-extension test-gnome-extension clean

## Download golangci-lint if not present
$(GOLANGCI_LINT):
	@mkdir -p $(BIN_DIR)
	@echo "Downloading golangci-lint v$(GOLANGCI_LINT_VERSION)..."
	@curl -sSfL https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCI_LINT_VERSION)/golangci-lint-$(GOLANGCI_LINT_VERSION)-$(OS)-$(ARCH).tar.gz | \
		tar xz -C $(BIN_DIR) --strip-components=1 golangci-lint-$(GOLANGCI_LINT_VERSION)-$(OS)-$(ARCH)/golangci-lint
	@echo "Installed golangci-lint v$(GOLANGCI_LINT_VERSION) to $(BIN_DIR)"

## Run linter
lint-cli: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run ./...

## Build the Go binary
build-cli:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) ./cmd/claude-usage/

## Run all tests (unit + integration)
test-cli:
	go test -v -count=1 ./...

## Run unit tests only (skip integration)
test-cli-short:
	go test -v -short -count=1 ./...

## Show per-function test coverage
test-cli-cover:
	go test -short -coverprofile=/tmp/claude-usage-coverage.out ./cmd/claude-usage/
	go tool cover -func=/tmp/claude-usage-coverage.out

## Install the Go binary to ~/.local/bin
install-cli: build-cli
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

## Remove the binary
uninstall-cli:
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
	rm -rf $(BIN_DIR)

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
