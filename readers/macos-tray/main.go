//go:build darwin

// Command claude-usage-tray is a macOS menu bar app displaying Claude Code utilization.
// It calls the claude-usage CLI every 60s and renders status in the system tray
// using native NSStatusItem with colored text via NSAttributedString.
package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// init locks the main goroutine to the startup (main) OS thread. Cocoa/AppKit
// requires that NSApplication setup and [NSApp run] execute on the process main
// thread; without this lock the Go scheduler may migrate goroutine 1 onto
// another OS thread, causing a SIGTRAP inside AppKit during cgo execution.
func init() {
	runtime.LockOSThread()
}

type StatusResponse struct {
	CPct          int    `json:"c_pct"`
	CReset        string `json:"c_reset"`
	CColor        string `json:"c_color"`
	WPct          int    `json:"w_pct"`
	WReset        string `json:"w_reset"`
	WColor        string `json:"w_color"`
	Stale         bool   `json:"stale"`
	ClaudeRunning bool   `json:"claude_running"`
	Auth          string `json:"auth"`
	Error         string `json:"error"`
}

// colorErrorRed is system red used for error states.
var colorErrorRed = color.RGBA{R: 0xDC, G: 0x32, B: 0x32, A: 0xFF}

// colorClaudeOrange is the default healthy-state color.
var colorClaudeOrange = color.RGBA{R: 0xD9, G: 0x77, B: 0x57, A: 0xFF}

var (
	mu         sync.Mutex
	status     StatusResponse
	binaryPath string
	refreshCh  = make(chan struct{}, 1)
	quitCh     = make(chan struct{}, 1)
)

func main() {
	binaryPath = findBinary()

	// Init native tray on main thread
	nativeInitTray()

	// Set initial state
	setTitle("◌ —", colorClaudeOrange)

	// Initial poll
	go pollAndUpdate(false)

	// Background poller
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pollAndUpdate(false)
			case <-refreshCh:
				pollAndUpdate(true)
			case <-quitCh:
				nativeStopApp()
				return
			}
		}
	}()

	// Run NSApp (blocks on main thread)
	nativeRunApp()
}

// quotaGlyph returns the appropriate glyph for 5h utilization percentage.
func quotaGlyph(pct int) string {
	switch {
	case pct >= 95:
		return "●"
	case pct >= 75:
		return "◕"
	case pct >= 50:
		return "◑"
	default:
		return "◔"
	}
}

// formatReset returns the reset duration or "—" if empty/unknown.
func formatReset(reset string) string {
	if reset == "" || reset == "?" {
		return "—"
	}
	return reset
}

// setTitle sets the menu bar title with the given color.
func setTitle(text string, clr color.RGBA) {
	nativeSetTitle(text, float64(clr.R)/255.0, float64(clr.G)/255.0, float64(clr.B)/255.0)
}

func findBinary() string {
	if p, err := exec.LookPath("claude-usage"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	localBin := filepath.Join(home, ".local", "bin", "claude-usage")
	if _, err := os.Stat(localBin); err == nil {
		return localBin
	}
	usrLocal := "/usr/local/bin/claude-usage"
	if _, err := os.Stat(usrLocal); err == nil {
		return usrLocal
	}
	return "claude-usage"
}

func pollAndUpdate(forcePoll bool) {
	args := []string{"--status"}
	if forcePoll {
		args = append(args, "--force-poll")
	}

	cmd := exec.Command(binaryPath, args...)
	out, err := cmd.Output()
	if err != nil {
		setErrorState()
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if err := json.Unmarshal(out, &status); err != nil {
		setErrorStateLocked()
		return
	}

	updateDisplay()
}

func updateDisplay() {
	updateAuth(status.Auth)

	if !status.ClaudeRunning {
		setTitle("◌ —", colorClaudeOrange)
		nativeSetMenuItemTitle(tagStatus, "Status: idle")
		nativeSetMenuItemTitle(tag5h, "5h: --")
		nativeSetMenuItemTitle(tagReset5h, "  resets in —")
		nativeSetMenuItemTitle(tag7d, "7d: --")
		nativeSetMenuItemTitle(tagReset7d, "  resets in —")
		nativeSetMenuItemHidden(tagError, true)
		nativeSetMenuItemHidden(tagAuth, true)
		return
	}

	// Title: <glyph> 5h <c_pct>% · 7d <w_pct>%
	glyph := quotaGlyph(status.CPct)
	title := fmt.Sprintf("%s 5h %d%% · 7d %d%%", glyph, status.CPct, status.WPct)
	if status.Stale {
		title += " ?"
	}

	setTitle(title, colorClaudeOrange)

	// Status line
	if status.Stale {
		nativeSetMenuItemTitle(tagStatus, "Status: stale")
	} else {
		nativeSetMenuItemTitle(tagStatus, "Status: active")
	}

	// Dropdown detail
	nativeSetMenuItemTitle(tag5h, fmt.Sprintf("5h: %d%%", status.CPct))
	nativeSetMenuItemTitle(tagReset5h, fmt.Sprintf("  resets in %s", formatReset(status.CReset)))
	nativeSetMenuItemTitle(tag7d, fmt.Sprintf("7d: %d%%", status.WPct))
	nativeSetMenuItemTitle(tagReset7d, fmt.Sprintf("  resets in %s", formatReset(status.WReset)))

	if status.Error != "" {
		nativeSetMenuItemTitle(tagError, "Error: "+status.Error)
		nativeSetMenuItemHidden(tagError, false)
	} else {
		nativeSetMenuItemHidden(tagError, true)
	}
}

func updateAuth(authState string) {
	switch authState {
	case "valid":
		nativeSetMenuItemHidden(tagAuth, true)
	case "expired":
		nativeSetMenuItemTitle(tagAuth, "⚠ Auth expired — run Claude Code to refresh")
		nativeSetMenuItemHidden(tagAuth, false)
	case "missing":
		nativeSetMenuItemTitle(tagAuth, "⚠ No credentials found")
		nativeSetMenuItemHidden(tagAuth, false)
	default:
		nativeSetMenuItemTitle(tagAuth, "Auth: unknown")
		nativeSetMenuItemHidden(tagAuth, false)
	}
}

func setErrorState() {
	mu.Lock()
	defer mu.Unlock()
	setErrorStateLocked()
}

func setErrorStateLocked() {
	setTitle("⚠ —", colorErrorRed)
	nativeSetMenuItemTitle(tagStatus, "Status: error")
	nativeSetMenuItemTitle(tag5h, "5h: --")
	nativeSetMenuItemTitle(tagReset5h, "  resets in —")
	nativeSetMenuItemTitle(tag7d, "7d: --")
	nativeSetMenuItemTitle(tagReset7d, "  resets in —")
	nativeSetMenuItemHidden(tagError, true)
	nativeSetMenuItemHidden(tagAuth, true)
}
