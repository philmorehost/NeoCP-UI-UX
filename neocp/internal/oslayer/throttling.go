package oslayer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// EnforceUserLimits creates systemd service overrides for resource limits
func EnforceUserLimits(ctx context.Context, username string, cpuLimitPct int, memLimitMB int) error {
	confDir := fmt.Sprintf("/etc/systemd/system/neocp-user-%s.slice.d", username)
	_ = os.MkdirAll(confDir, 0755)

	content := fmt.Sprintf("[Slice]\nCPUQuota=%d%%\nMemoryMax=%dM\n", cpuLimitPct, memLimitMB)
	err := os.WriteFile(filepath.Join(confDir, "limits.conf"), []byte(content), 0644)
	if err != nil {
		return err
	}

	_, err = (&SafeCommandExec{}).Execute(ctx, "systemctl", []string{"daemon-reload"}, 0)
	return err
}

// SetDiskQuota applies filesystem quotas via the 'setquota' command
func SetDiskQuota(ctx context.Context, username string, quotaMB int64) error {
	if quotaMB <= 0 {
		return nil
	}

	quotaKB := quotaMB * 1024
	args := []string{"-u", username, fmt.Sprintf("%d", quotaKB), fmt.Sprintf("%d", quotaKB), "0", "0", "/"}

	_, err := (&SafeCommandExec{}).Execute(ctx, "setquota", args, 0)
	return err
}
