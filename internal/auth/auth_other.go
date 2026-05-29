//go:build !darwin

package auth

// readKeychain is a no-op on non-macOS platforms.
func readKeychain() ([]byte, error) {
	return nil, nil
}
