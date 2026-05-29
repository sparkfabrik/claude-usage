// Package process detects whether Claude Code is currently running.
package process

import "os/exec"

// IsClaudeRunning checks if a Claude Code process is currently running.
func IsClaudeRunning() bool {
	err := exec.Command("pgrep", "-x", "claude").Run()
	return err == nil
}
