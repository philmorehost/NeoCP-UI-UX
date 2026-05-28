package cms

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"neocp/internal/oslayer"
)

type WPSetting struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// HardenWordPress applies security best practices to a WP installation
func HardenWordPress(ctx context.Context, path string, settings []WPSetting) error {
	log.Printf("[WP Toolkit] Hardening WordPress at %s", path)
	exec := &oslayer.SafeCommandExec{}

	for _, s := range settings {
		if !s.Enabled {
			continue
		}

		switch s.Name {
		case "disable_xmlrpc":
			htaccess := filepath.Join(path, ".htaccess")
			rule := "\n<Files xmlrpc.php>\nOrder Deny,Allow\nDeny from all\n</Files>\n"
			f, err := os.OpenFile(htaccess, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				f.WriteString(rule)
				f.Close()
			}
		case "disable_file_edit":
			wpConfig := filepath.Join(path, "wp-config.php")
			data, err := os.ReadFile(wpConfig)
			if err == nil && !strings.Contains(string(data), "DISALLOW_FILE_EDIT") {
				newContent := strings.Replace(string(data), "<?php", "<?php\ndefine('DISALLOW_FILE_EDIT', true);", 1)
				os.WriteFile(wpConfig, []byte(newContent), 0644)
			}
		case "update_core":
			_, _ = exec.Execute(ctx, "wp", []string{"core", "update", "--path=" + path, "--allow-root"}, 0)
		}
	}

	return nil
}

// BulkUpdateWordPress updates core/plugins for all sites
func BulkUpdateWordPress(ctx context.Context, paths []string) (int, error) {
	log.Printf("[WP Toolkit] Starting bulk update for %d sites", len(paths))
	exec := &oslayer.SafeCommandExec{}
	updated := 0
	for _, p := range paths {
		_, err := exec.Execute(ctx, "wp", []string{"core", "update", "--path=" + p, "--allow-root"}, 0)
		if err == nil {
			updated++
		}
	}
	return updated, nil
}
