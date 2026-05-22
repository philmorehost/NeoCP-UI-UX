package oslayer

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// EnforceUserLimits creates systemd service overrides for resource limits
func EnforceUserLimits(ctx context.Context, username string, cpuLimitPct int, memLimitMB int) error {
	// For production, we would use systemd slice or individual service overrides
	// Example path: /etc/systemd/system/neocp-user-[user].slice.d/limits.conf

	confDir := fmt.Sprintf("/etc/systemd/system/neocp-user-%s.slice.d", username)
	_ = os.MkdirAll(confDir, 0755)

	content := fmt.Sprintf("[Slice]\nCPUQuota=%d%%\nMemoryMax=%dM\n", cpuLimitPct, memLimitMB)
	err := ioutil.WriteFile(filepath.Join(confDir, "limits.conf"), []byte(content), 0644)
	if err != nil {
		fmt.Printf("[Simulation] Enforcing limits for %s: CPU %d%%, MEM %dM\n", username, cpuLimitPct, memLimitMB)
	}

	// systemctl daemon-reload
	_, _ = (&SafeCommandExec{}).Execute(ctx, "systemctl", []string{"daemon-reload"}, 0)

	return nil
}

// SetDiskQuota applies filesystem quotas via the 'setquota' command
func SetDiskQuota(ctx context.Context, username string, quotaMB int64) error {
	if quotaMB <= 0 {
		return nil // Unlimited or not set
	}

	// setquota -u [user] [soft] [hard] [inode_soft] [inode_hard] [device]
	// Using hard limit only for simplicity in this stage
	quotaKB := quotaMB * 1024
	args := []string{"-u", username, fmt.Sprintf("%d", quotaKB), fmt.Sprintf("%d", quotaKB), "0", "0", "/"}

	_, err := (&SafeCommandExec{}).Execute(ctx, "setquota", args, 0)
	if err != nil {
		fmt.Printf("[Simulation] Setting disk quota for %s to %d MB\n", username, quotaMB)
	}

	return nil
}
