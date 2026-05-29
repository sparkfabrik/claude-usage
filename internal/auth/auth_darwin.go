//go:build darwin

package auth

import (
	"context"
	"os/exec"
	"os/user"
	"time"
)

// readKeychain reads OAuth credentials from the macOS login Keychain.
// Returns nil, nil if the item is not found or the security command fails.
func readKeychain() ([]byte, error) {
	u, err := user.Current()
	if err != nil {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "security", "find-generic-password",
		"-s", "Claude Code-credentials",
		"-a", u.Username,
		"-w",
	)

	out, err := cmd.Output()
	if err != nil {
		// Missing item or any failure → not found, not an error
		return nil, nil
	}

	if len(out) == 0 {
		return nil, nil
	}

	return out, nil
}
