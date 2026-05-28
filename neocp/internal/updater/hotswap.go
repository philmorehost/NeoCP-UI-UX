package updater

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// VerifyBinaryChecksum ensures the downloaded binary matches the expected signature
func VerifyBinaryChecksum(filePath string, expectedHash string) (bool, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}

	actualHash := fmt.Sprintf("%x", h.Sum(nil))
	return actualHash == expectedHash, nil
}

// HotSwapBinary performs a zero-downtime process replacement
func HotSwapBinary(newBinaryPath string) error {
	// In production, this would use Exec to replace the current process
	// while keeping the same file descriptors (like port 8443 listener).

	// For simulation/hardened logic:
	fmt.Printf("[Updater] Preparing hot-swap for %s...\n", newBinaryPath)

	// Swap logic:
	// 1. Rename current binary to neocp.old
	// 2. Move new binary to neocp
	// 3. syscall.Exec into the new binary

	currentPath, _ := os.Executable()
	_ = os.Rename(currentPath, currentPath+".old")

	err := os.Rename(newBinaryPath, currentPath)
	if err != nil {
		return err
	}

	// Hot-swap (Simulated call)
	// syscall.Exec(currentPath, os.Args, os.Environ())

	return nil
}
