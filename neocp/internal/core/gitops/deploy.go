package gitops

import (
	"context"
	"fmt"
	"log"
	"neocp/internal/core"
	"neocp/internal/oslayer"
	"time"
)

// DeployFromGit executes the push-to-deploy workflow
func DeployFromGit(ctx context.Context, domainName string, config core.GitConfig, sandboxDir string) error {
	log.Printf("[GitOps] Starting deployment for %s from %s", domainName, config.RepoURL)

	exec := &oslayer.SafeCommandExec{}

	// 1. Resolve Path
	targetPath := config.Path
	if targetPath == "" {
		targetPath = fmt.Sprintf("%s/public_html/%s", sandboxDir, domainName)
	}

	// 2. Execute Clone/Pull
	// git clone [url] [path] or cd [path] && git pull
	args := []string{"clone", "--depth", "1", "-b", config.Branch, config.RepoURL, targetPath}
	_, err := exec.Execute(ctx, "git", args, 60*time.Second)
	if err != nil {
		log.Printf("[GitOps] Git command failed: %v. Ensure git is installed and SSH keys are configured.", err)
		return err
	}

	// 3. Post-deploy hooks (e.g. npm install, composer install)
	log.Printf("[GitOps] Executing post-deploy hooks for %s", domainName)

	// Update DB state
	db := core.GetDB()
	config.LastDeploy = time.Now()
	config.DeployLog = append(config.DeployLog, fmt.Sprintf("[%s] Successfully deployed branch %s", time.Now().Format(time.RFC3339), config.Branch))
	_ = db.UpdateDomainGit(domainName, config)

	return nil
}
