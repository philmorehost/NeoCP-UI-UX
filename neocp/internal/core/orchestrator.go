package core

import "context"

// PackageDefinition defines the resource constraints for an account
type PackageDefinition struct {
	Name             string `json:"name"`
	DiskQuota        string `json:"disk_quota"` // e.g., "10GB", "unlimited"
	Bandwidth        string `json:"bandwidth"`
	MaxDomains       int    `json:"max_domains"`
	MaxDatabases     int    `json:"max_databases"`
	MaxFTP           int    `json:"max_ftp"`
	MaxEmail         int    `json:"max_email"`
	HourlyEmailLimit int    `json:"hourly_email_limit"`
	FailedEmailPct   int    `json:"failed_email_pct"`
	IsReseller       bool   `json:"is_reseller"`
}

// RemoteAuth holds credentials for external panel connections
type RemoteAuth struct {
	IPAddress string `json:"ip_address"`
	Panel     string `json:"panel"` // cpanel, plesk, cyberpanel
	APIToken  string `json:"api_token"`
	Port      int    `json:"port"`
}

// AccountDetails represents the result of a conversion or migration
type AccountDetails struct {
	Username string `json:"username"`
	Domain   string `json:"domain"`
	Success  bool   `json:"success"`
}

// TransferLog contains the results of a bulk migration operation
type TransferLog struct {
	TotalAccounts int      `json:"total_accounts"`
	Successful    []string `json:"successful"`
	Failed        []string `json:"failed"`
	Log           string   `json:"log"`
}

// NeoCPProOrchestrator enforces absolute production parity with cPanel/WHM architecture.
type NeoCPProOrchestrator interface {
	// 1. IP & Server Identity
	AssignIPv4ToInterface(ctx context.Context, ip string, subnet string) error
	SetServerHostname(ctx context.Context, newHostname string) error
	UpdateResolvers(ctx context.Context, primary string, secondary string) error

	// 2. Reseller & Account Hierarchy
	TransferAccountOwnership(ctx context.Context, targetAccount string, newReseller string) error
	DelegateIPToReseller(ctx context.Context, resellerID string, ipAddress string) error
	ConvertAddonToAccount(ctx context.Context, sourceUser string, addonDomain string) (*AccountDetails, error)

	// 3. Package Limits & Parsing
	CreatePackage(ctx context.Context, spec PackageDefinition) error
	EnforcePackageLimits(ctx context.Context, userID string, packageID string) error

	// 4. File Management & Trash
	MoveToTrash(ctx context.Context, userID string, filePath string) error
	EmptyUserTrash(ctx context.Context, userID string) (bytesFreed int64, err error)

	// 5. Transfer Tool (Bulk)
	ExecuteBulkMigration(ctx context.Context, sourceType string, credentials RemoteAuth, accounts []string) (*TransferLog, error)
	RestoreSingleCpanelBackup(ctx context.Context, filePath string) error

	// 6. NGINX & Service Control
	RestartGlobalService(ctx context.Context, serviceName string) error
	ClearUserNginxCache(ctx context.Context, userID string, domain string) error
}
