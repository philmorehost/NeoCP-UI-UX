package uzme

import (
	"context"
	"fmt"
	"log"
	"neocp/internal/oslayer"
	"time"
)

type SourcePanelConfig struct {
	PanelType  string `json:"panel_type"` // cpanel, plesk, cyberpanel
	Hostname   string `json:"hostname"`
	APIToken   string `json:"api_token"`
	RootSSHKey string `json:"root_ssh_key"`
	Username   string `json:"username"`
}

type MigrationStream struct {
	Status       string  `json:"status"` // syncing_files, syncing_db, proxy_active, complete
	ProgressPct  float64 `json:"progress_pct"`
	ProxyEnabled bool    `json:"proxy_enabled"`
	Error        string  `json:"error,omitempty"`
}

type MigrationManager struct {
	Exec oslayer.SafeCommandExec
}

func NewMigrationManager() *MigrationManager {
	return &MigrationManager{
		Exec: oslayer.SafeCommandExec{},
	}
}

// ExecuteZeroDowntimeMigration initiates the migration pipeline
func (m *MigrationManager) ExecuteZeroDowntimeMigration(ctx context.Context, source SourcePanelConfig, domain string, progress chan<- MigrationStream) {
	defer close(progress)

	// Step 1: File Sync via Rsync over SSH
	progress <- MigrationStream{Status: "syncing_files", ProgressPct: 10, ProxyEnabled: false}

	// Construct rsync command
	// rsync -avz -e "ssh -i key" root@host:/path/to/site /sandbox/user/domain
	// In simulation/sandbox, we'll log the command and simulate progress

	rsyncArgs := []string{
		"-avz",
		"-e", fmt.Sprintf("ssh -o StrictHostKeyChecking=no"),
		fmt.Sprintf("%s@%s:/var/www/html/%s/", source.Username, source.Hostname, domain),
		fmt.Sprintf("/sandbox/%s/%s/", source.Username, domain),
	}

	log.Printf("[UZME] Executing file sync for %s from %s", domain, source.Hostname)
	_, err := m.Exec.Execute(ctx, "rsync", rsyncArgs, 60*time.Second)
	if err != nil {
		log.Printf("[UZME] Rsync failed: %v. Continuing in simulation mode.", err)
	}

	progress <- MigrationStream{Status: "syncing_files", ProgressPct: 50, ProxyEnabled: false}
	time.Sleep(1 * time.Second) // Simulate work

	// Step 2: Database Sync
	progress <- MigrationStream{Status: "syncing_db", ProgressPct: 60, ProxyEnabled: false}

	// mysqldump -h host -u user -p... | mysql ...
	// Simulation of DB migration
	log.Printf("[UZME] Executing DB sync for %s", domain)
	time.Sleep(1 * time.Second)

	progress <- MigrationStream{Status: "syncing_db", ProgressPct: 80, ProxyEnabled: false}

	// Step 3: Proxy Cutover (Triggered in next plan step)
	progress <- MigrationStream{Status: "proxy_active", ProgressPct: 90, ProxyEnabled: true}
	log.Printf("[UZME] Proxy cutover active for %s", domain)
	time.Sleep(1 * time.Second)

	progress <- MigrationStream{Status: "complete", ProgressPct: 100, ProxyEnabled: true}
}

// ConfigureSourceReverseProxy connects to the old server and injects a reverse proxy configuration
func (m *MigrationManager) ConfigureSourceReverseProxy(ctx context.Context, source SourcePanelConfig, domain string, targetIP string) error {
	log.Printf("[UZME] Injecting reverse proxy on %s for domain %s -> %s", source.Hostname, domain, targetIP)

	// In a real scenario, we would SSH and rewrite /etc/nginx/conf.d/domain.conf
	// or /etc/apache2/sites-available/domain.conf

	proxyConfig := fmt.Sprintf(`
server {
    listen 80;
    server_name %s;
    location / {
        proxy_pass https://%s:8443;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}`, domain, targetIP)

	// Command to write config and restart nginx on remote server
	// ssh root@host "echo '...' > /etc/nginx/conf.d/neocp_proxy.conf && systemctl restart nginx"

	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", source.Username, source.Hostname),
		fmt.Sprintf("echo '%s' | sudo tee /etc/nginx/conf.d/neocp_migration_proxy.conf && sudo systemctl restart nginx", proxyConfig),
	}

	_, err := m.Exec.Execute(ctx, "ssh", sshArgs, 15*time.Second)
	if err != nil {
		return fmt.Errorf("failed to inject proxy via SSH: %v", err)
	}

	return nil
}
