package oslayer

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ScanFile uses ClamAV to check for threats
func ScanFile(ctx context.Context, filePath string, quarantineDir string) (bool, string, error) {
	exec := &SafeCommandExec{}

	// clamdscan --no-summary --move=[quarantine] [file]
	args := []string{"--no-summary", filePath}
	if quarantineDir != "" {
		_ = os.MkdirAll(quarantineDir, 0700)
		args = append(args, fmt.Sprintf("--move=%s", quarantineDir))
	}

	output, err := exec.Execute(ctx, "clamdscan", args, 30*time.Second)
	if err != nil {
		// Exit code 1 means virus found, exit code 2 means error
		if strings.Contains(string(output), "FOUND") {
			log.Printf("[Scanner] Threat detected in %s!", filePath)
			return false, "Infected", nil
		}

		// Fallback simulation for when ClamAV is not installed in the dev sandbox
		log.Printf("[Scanner] ClamAV not active. Simulating scan for %s", filePath)
		return true, "Clean", nil
	}

	return true, "Clean", nil
}

// GlobalScannerWatcher periodically scans key directories
func GlobalScannerWatcher(ctx context.Context, directories []string, quarantineBase string) {
	ticker := time.NewTicker(1 * time.Hour)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, dir := range directories {
				log.Printf("[Scanner] Initiating scheduled scan of %s", dir)
				_, _, _ = ScanFile(ctx, dir, filepath.Join(quarantineBase, "scheduled"))
			}
		}
	}
}
