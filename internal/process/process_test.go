package process

import (
	"testing"
)

func TestIsClaudeRunning(t *testing.T) {
	// This is a runtime check — we can only verify it doesn't panic.
	// Result depends on whether Claude Code is actually running.
	_ = IsClaudeRunning()
}
