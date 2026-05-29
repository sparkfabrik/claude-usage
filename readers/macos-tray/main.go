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

var (
	mu         sync.Mutex
	status     StatusResponse
	binaryPath string

	mStatus    *systray.MenuItem
	mDetail5h  *systray.MenuItem
	mReset5h   *systray.MenuItem
	mDetail7d  *systray.MenuItem
	mReset7d   *systray.MenuItem
	mAuth      *systray.MenuItem
	mError     *systray.MenuItem
	mRefresh   *systray.MenuItem
	mQuit      *systray.MenuItem
)

func main() {
	systray.Run(onReady, onExit)
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

func onReady() {
	systray.SetTitle("◌ —")
	systray.SetTooltip("Claude Usage")

	mStatus = systray.AddMenuItem("Status: idle", "Current state")
	systray.AddSeparator()
	mDetail5h = systray.AddMenuItem("5h: --", "5-hour utilization")
	mReset5h = systray.AddMenuItem("  resets in —", "5h reset timer")
	mDetail7d = systray.AddMenuItem("7d: --", "7-day utilization")
	mReset7d = systray.AddMenuItem("  resets in —", "7d reset timer")
	mAuth = systray.AddMenuItem("", "Auth state")
	mAuth.Hide()
	mError = systray.AddMenuItem("", "Error info")
	mError.Hide()
	systray.AddSeparator()
	mRefresh = systray.AddMenuItem("Refresh now", "Force poll and refresh")
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
		systray.SetTitle("◌ —")
		mStatus.SetTitle("Status: idle")
		mDetail5h.SetTitle("5h: --")
		mReset5h.SetTitle("  resets in —")
		mDetail7d.SetTitle("7d: --")
		mReset7d.SetTitle("  resets in —")
		mError.Hide()
		mAuth.Hide()
		return
	}

	// Title: <glyph> 5h <c_pct>% · 7d <w_pct>%
	glyph := quotaGlyph(status.CPct)
	title := fmt.Sprintf("%s 5h %d%% · 7d %d%%", glyph, status.CPct, status.WPct)
	if status.Stale {
		title += " ?"
	}
	systray.SetTitle(title)

	// Status line
	if status.Stale {
		mStatus.SetTitle("Status: stale")
	} else {
		mStatus.SetTitle("Status: active")
	}

	// Dropdown detail
	mDetail5h.SetTitle(fmt.Sprintf("5h: %d%%", status.CPct))
	mReset5h.SetTitle(fmt.Sprintf("  resets in %s", formatReset(status.CReset)))
	mDetail7d.SetTitle(fmt.Sprintf("7d: %d%%", status.WPct))
	mReset7d.SetTitle(fmt.Sprintf("  resets in %s", formatReset(status.WReset)))

	if status.Error != "" {
		mError.SetTitle("Error: " + status.Error)
		mError.Show()
	} else {
		mError.Hide()
	}
}

// updateAuth shows the auth-state menu item (hidden when valid).
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

func setErrorState() {
	mu.Lock()
	defer mu.Unlock()
	setErrorStateLocked()
}

func setErrorStateLocked() {
	systray.SetTitle("⚠ —")
	mStatus.SetTitle("Status: error")
	mDetail5h.SetTitle("5h: --")
	mReset5h.SetTitle("  resets in —")
	mDetail7d.SetTitle("7d: --")
	mReset7d.SetTitle("  resets in —")
	mError.Hide()
	mAuth.Hide()
}
