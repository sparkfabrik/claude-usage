// Command claude-usage-tray is a macOS menu bar app displaying Claude Code utilization.
// It calls the claude-usage CLI every 60s and renders status in the system tray.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"fyne.io/systray"
)

type StatusResponse struct {
	CPct         int    `json:"c_pct"`
	CReset       string `json:"c_reset"`
	CColor       string `json:"c_color"`
	WPct         int    `json:"w_pct"`
	WReset       string `json:"w_reset"`
	WColor       string `json:"w_color"`
	Stale        bool   `json:"stale"`
	ClaudeRunning bool  `json:"claude_running"`
	Auth         string `json:"auth"`
	Error        string `json:"error"`
}

var (
	mu          sync.Mutex
	status      StatusResponse
	binaryPath  string
	mDetail5h   *systray.MenuItem
	mDetail7d   *systray.MenuItem
	mAuth       *systray.MenuItem
	mError      *systray.MenuItem
	mRefresh    *systray.MenuItem
	mQuit       *systray.MenuItem
)

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetTitle("C:--")
	systray.SetTooltip("Claude Usage")

	mDetail5h = systray.AddMenuItem("5h: --", "5-hour utilization")
	mDetail7d = systray.AddMenuItem("7d: --", "7-day utilization")
	mAuth = systray.AddMenuItem("", "Auth state")
	mAuth.Hide()
	mError = systray.AddMenuItem("", "Error info")
	mError.Hide()
	systray.AddSeparator()
	mRefresh = systray.AddMenuItem("Refresh Now", "Force poll and refresh")
	mQuit = systray.AddMenuItem("Quit", "Quit the tray app")

	binaryPath = findBinary()

	// Initial poll
	pollAndUpdate(false)

	// Poll every 60s
	ticker := time.NewTicker(60 * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:
				pollAndUpdate(false)
			case <-mRefresh.ClickedCh:
				pollAndUpdate(true)
			case <-mQuit.ClickedCh:
				systray.Quit()
			}
		}
	}()
}

func onExit() {}

func findBinary() string {
	// Check PATH
	if p, err := exec.LookPath("claude-usage"); err == nil {
		return p
	}
	// Fallback: ~/.local/bin
	home, _ := os.UserHomeDir()
	localBin := filepath.Join(home, ".local", "bin", "claude-usage")
	if _, err := os.Stat(localBin); err == nil {
		return localBin
	}
	// Fallback: /usr/local/bin
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
		setIdleState()
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if err := json.Unmarshal(out, &status); err != nil {
		setIdleState()
		return
	}

	updateDisplay()
}

func updateDisplay() {
	updateAuth(status.Auth)

	if !status.ClaudeRunning {
		systray.SetTitle("C:--")
		mDetail5h.SetTitle("5h: idle")
		mDetail7d.SetTitle("7d: idle")
		mError.Hide()
		return
	}

	title := fmt.Sprintf("C:%d%% W:%d%%", status.CPct, status.WPct)
	if status.Stale {
		title += " ?"
	}
	systray.SetTitle(title)

	mDetail5h.SetTitle(fmt.Sprintf("5h: %d%% (resets in %s)", status.CPct, status.CReset))
	mDetail7d.SetTitle(fmt.Sprintf("7d: %d%% (resets in %s)", status.WPct, status.WReset))

	if status.Error != "" {
		mError.SetTitle("Error: " + status.Error)
		mError.Show()
	} else {
		mError.Hide()
	}
}

// updateAuth shows the auth-state menu item (hidden when valid).
// systray has no per-item color, so state is conveyed by text/glyph.
func updateAuth(authState string) {
	switch authState {
	case "valid":
		mAuth.Hide()
	case "expired":
		mAuth.SetTitle("⚠ Auth expired — run Claude Code to refresh")
		mAuth.Show()
	case "missing":
		mAuth.SetTitle("⚠ No credentials found")
		mAuth.Show()
	default:
		mAuth.SetTitle("Auth: unknown")
		mAuth.Show()
	}
}

func setIdleState() {
	mu.Lock()
	defer mu.Unlock()
	systray.SetTitle("C:?")
	mDetail5h.SetTitle("5h: error")
	mDetail7d.SetTitle("7d: error")
	mError.Hide()
	mAuth.SetTitle("Auth: unknown")
	mAuth.Show()
}
