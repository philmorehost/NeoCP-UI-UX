package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Data schemas representing WHM/cPanel, Plesk, and CyberPanel features
type Account struct {
	Username       string    `json:"username"`
	Password       string    `json:"password"`
	Role           string    `json:"role"` // admin, reseller, customer
	Owner          string    `json:"owner"` // Who owns this account (admin or reseller)
	Plan           string    `json:"plan"`
	Email          string    `json:"email"`
	DiskUsed       int64     `json:"disk_used"`       // in MB
	DiskLimit      int64     `json:"disk_limit"`      // in MB
	BandwidthUsed  int64     `json:"bandwidth_used"`  // in GB
	BandwidthLimit int64     `json:"bandwidth_limit"` // in GB
	DomainsUsed    int       `json:"domains_used"`
	DomainsLimit   int       `json:"domains_limit"`
	DatabasesUsed  int       `json:"databases_used"`
	DatabasesLimit int       `json:"databases_limit"`
	FTPUsed        int       `json:"ftp_used"`
	FTPLimit       int       `json:"ftp_limit"`
	EmailUsed      int       `json:"email_used"`
	EmailLimit     int       `json:"email_limit"`
	SharedIP       string    `json:"shared_ip"`
	DedicatedIP    string    `json:"dedicated_ip"`
	Nameservers    []string  `json:"nameservers"`
	Locale         string    `json:"locale"`
	ShellAccess    bool      `json:"shell_access"`
	CreatedAt      time.Time `json:"created_at"`
}

type DNSRecord struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // A, AAAA, CNAME, MX, TXT, SRV
	Name     string `json:"name"` // e.g. "www" or "@"
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority,omitempty"`
}

type WAFPolicy struct {
	SQLiShield bool `json:"sqli_shield"`
	XSSBlock   bool `json:"xss_block"`
	LFIShield  bool `json:"lfi_shield"`
	CSRFHeader bool `json:"csrf_header"`
}

type Domain struct {
	DomainName           string      `json:"domain_name"`
	Owner                string      `json:"owner"`
	PHPVersion           string      `json:"php_version"`
	DirectoryPrivacy     bool        `json:"directory_privacy"`
	DirectoryPrivacyUser string      `json:"directory_privacy_user"`
	DirectoryPrivacyPass string      `json:"directory_privacy_pass"`
	RedirectURL          string      `json:"redirect_url"`
	SSLActive            bool        `json:"ssl_active"`
	SSLIssuer            string      `json:"ssl_issuer"`
	SSLExpires           time.Time   `json:"ssl_expires"`
	WebDAVEnabled        bool        `json:"webdav_enabled"`
	HotlinkProtected     bool        `json:"hotlink_protected"`
	LeechProtected       bool        `json:"leech_protected"`
	GzipEnabled          bool        `json:"gzip_enabled"`
	BrotliEnabled        bool        `json:"brotli_enabled"`
	DNSRecords           []DNSRecord `json:"dns_records"`
	WAFPolicy            WAFPolicy   `json:"waf_policy"`
	CreatedAt            time.Time   `json:"created_at"`
}

type Database struct {
	Name       string    `json:"name"`
	Owner      string    `json:"owner"`
	DBUser     string    `json:"db_user"`
	Password   string    `json:"password"`
	RemoteIPs  string    `json:"remote_ips"` // Comma-separated whitelisted IPs or "%"
	CreatedAt  time.Time `json:"created_at"`
}

type CronJob struct {
	ID        string    `json:"id"`
	Owner     string    `json:"owner"`
	TaskName  string    `json:"task_name"`
	Command   string    `json:"command"`
	Schedule  string    `json:"schedule"` // e.g. "*/5 * * * *"
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type ResellerPackage struct {
	Name             string    `json:"name"`
	Owner            string    `json:"owner"` // Who created the package
	DiskQuota        string    `json:"disk_quota"` // "1GB", "unlimited"
	Bandwidth        string    `json:"bandwidth"`  // "100GB", "unlimited"
	MaxDomains       int       `json:"max_domains"` // -1 for unlimited
	MaxDatabases     int       `json:"max_databases"`
	MaxFTP           int       `json:"max_ftp"`
	MaxEmail         int       `json:"max_email"`
	MaxSubdomains    int       `json:"max_subdomains"`
	MaxParkedDomains int       `json:"max_parked_domains"`
	MaxAddonDomains  int       `json:"max_addon_domains"`
	HourlyEmailLimit int       `json:"hourly_email_limit"`
	FailedEmailPct   int       `json:"failed_email_pct"`
	MaxEmailQuota    string    `json:"max_email_quota"`
	LVECpuPct        int       `json:"lve_cpu_pct"`
	LVERamMB         int       `json:"lve_ram_mb"`
	IsReseller       bool      `json:"is_reseller"` // WHMReseller extension
	DedicatedIP      bool      `json:"dedicated_ip"`
	ShellAccess      bool      `json:"shell_access"`
	CGIAccess        bool      `json:"cgi_access"`
	DigestAuth       bool      `json:"digest_auth"`
	CreatedAt        time.Time `json:"created_at"`
}

type TicketMessage struct {
	Sender    string    `json:"sender"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type Ticket struct {
	ID        string          `json:"id"`
	Owner     string          `json:"owner"`
	Subject   string          `json:"subject"`
	Status    string          `json:"status"` // open, answered, closed
	Category  string          `json:"category"`
	Messages  []TicketMessage `json:"messages"`
	CreatedAt time.Time       `json:"created_at"`
}

type DockerContainer struct {
	Name      string    `json:"name"`
	Owner     string    `json:"owner"`
	Image     string    `json:"image"`
	Status    string    `json:"status"` // running, stopped
	Ports     string    `json:"ports"`
	CreatedAt time.Time `json:"created_at"`
}

type FirewallBlock struct {
	IP        string    `json:"ip"`
	Reason    string    `json:"reason"`
	BlockedAt time.Time `json:"blocked_at"`
}

type IPAddress struct {
	IP        string `json:"ip"`
	Subnet    string `json:"subnet"`
	IsShared  bool   `json:"is_shared"`
	IsAssigned bool   `json:"is_assigned"`
	Owner     string `json:"owner"` // Who it's delegated to (admin, reseller, user)
}

type ServerConfig struct {
	Hostname    string   `json:"hostname"`
	Nameservers []string `json:"nameservers"`
	Resolvers   []string `json:"resolvers"`
	SharedIP    string   `json:"shared_ip"`
}

type ClusterNode struct {
	NodeID    string    `json:"node_id"`
	IP        string    `json:"ip"`
	Role      string    `json:"role"` // web, db, mail
	IsActive  bool      `json:"is_active"`
	CPULoad   float64   `json:"cpu_load"`
	RAMLoad   float64   `json:"ram_load"`
	LastPing  time.Time `json:"last_ping"`
}


type MigrationTask struct {
	ID           string    `json:"id"`
	Owner        string    `json:"owner"`
	PanelType    string    `json:"panel_type"` // cpanel, plesk, cyberpanel
	Hostname     string    `json:"hostname"`
	TargetDomain string    `json:"target_domain"`
	Status       string    `json:"status"` // syncing_files, syncing_db, proxy_active, complete
	ProgressPct  float64   `json:"progress_pct"`
	Logs         []string  `json:"logs"`
	CreatedAt    time.Time `json:"created_at"`
}

// DatabaseStore manages the raw file payload with thread-safety
type DatabaseStore struct {
	Accounts         map[string]Account         `json:"accounts"`
	Domains          map[string]Domain          `json:"domains"`
	Databases        map[string]Database        `json:"databases"`
	CronJobs         map[string]CronJob         `json:"cron_jobs"`
	ResellerPackages map[string]ResellerPackage `json:"reseller_packages"`
	Tickets          map[string]Ticket          `json:"tickets"`
	DockerContainers map[string]DockerContainer `json:"docker_containers"`
	MigrationTasks   map[string]MigrationTask   `json:"migration_tasks"`
	FirewallBlocks   map[string]FirewallBlock   `json:"firewall_blocks"`
	ClusterNodes     map[string]ClusterNode     `json:"cluster_nodes"`
	IPAddresses      []IPAddress                `json:"ip_addresses"`
	Server           ServerConfig               `json:"server"`
}

type DatabaseEngine struct {
	filePath string
	mu       sync.RWMutex
	store    DatabaseStore
}

var (
	engineInstance *DatabaseEngine
	once           sync.Once
)

// GetDB initializes and returns the database singleton
func GetDB() *DatabaseEngine {
	once.Do(func() {
		engineInstance = &DatabaseEngine{
			filePath: "neocp_data.json",
			store: DatabaseStore{
				Accounts:         make(map[string]Account),
				Domains:          make(map[string]Domain),
				Databases:        make(map[string]Database),
				CronJobs:         make(map[string]CronJob),
				ResellerPackages: make(map[string]ResellerPackage),
				Tickets:          make(map[string]Ticket),
				DockerContainers: make(map[string]DockerContainer),
				MigrationTasks:   make(map[string]MigrationTask),
				FirewallBlocks:   make(map[string]FirewallBlock),
				ClusterNodes:     make(map[string]ClusterNode),
			},
		}
		engineInstance.load()
		engineInstance.seedDefaultData()
	})
	return engineInstance
}

func (db *DatabaseEngine) load() {
	if _, err := os.Stat(db.filePath); os.IsNotExist(err) {
		db.save()
		return
	}
	data, err := ioutil.ReadFile(db.filePath)
	if err != nil {
		return
	}
	var loaded DatabaseStore
	if err := json.Unmarshal(data, &loaded); err == nil {
		if loaded.Accounts != nil {
			db.store.Accounts = loaded.Accounts
		}
		if loaded.Domains != nil {
			db.store.Domains = loaded.Domains
		}
		if loaded.Databases != nil {
			db.store.Databases = loaded.Databases
		}
		if loaded.CronJobs != nil {
			db.store.CronJobs = loaded.CronJobs
		}
		if loaded.ResellerPackages != nil {
			db.store.ResellerPackages = loaded.ResellerPackages
		}
		if loaded.Tickets != nil {
			db.store.Tickets = loaded.Tickets
		}
		if loaded.DockerContainers != nil {
			db.store.DockerContainers = loaded.DockerContainers
		}
		if loaded.MigrationTasks != nil {
			db.store.MigrationTasks = loaded.MigrationTasks
		}
		if loaded.FirewallBlocks != nil {
			db.store.FirewallBlocks = loaded.FirewallBlocks
		}
		if loaded.ClusterNodes != nil {
			db.store.ClusterNodes = loaded.ClusterNodes
		}
	}
}


func (db *DatabaseEngine) save() {
	data, err := json.MarshalIndent(db.store, "", "  ")
	if err == nil {
		_ = ioutil.WriteFile(db.filePath, data, 0644)
	}
}

func (db *DatabaseEngine) seedDefaultData() {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Seed primary system administrator
	if _, exists := db.store.Accounts["admin"]; !exists {
		db.store.Accounts["admin"] = Account{
			Username:       "admin",
			Password:       "admin123", // In production this would be hashed. Keep plain text for easy visual testing of custom dashboard.
			Role:           "admin",
			Owner:          "root",
			Plan:           "Unlimited System Plan",
			Email:          "admin@neocp.io",
			DiskUsed:       1024,
			DiskLimit:      -1, // -1 = unlimited
			BandwidthUsed:  45,
			BandwidthLimit: -1,
			DomainsUsed:    3,
			DomainsLimit:   -1,
			DatabasesLimit: -1,
			FTPLimit:       -1,
			EmailLimit:     -1,
			SharedIP:       "192.168.1.100",
			CreatedAt:      time.Now(),
		}
	}

	// Seed a reseller
	if _, exists := db.store.Accounts["reseller1"]; !exists {
		db.store.Accounts["reseller1"] = Account{
			Username:       "reseller1",
			Password:       "reseller123",
			Role:           "reseller",
			Owner:          "admin",
			Plan:           "Gold Reseller Pack",
			Email:          "reseller@neocp.io",
			DiskUsed:       240,
			DiskLimit:      50000,
			BandwidthUsed:  15,
			BandwidthLimit: 2000,
			DomainsUsed:    1,
			DomainsLimit:   50,
			DatabasesLimit: 100,
			FTPLimit:       100,
			EmailLimit:     100,
			SharedIP:       "192.168.1.100",
			CreatedAt:      time.Now(),
		}
	}

	// Seed standard user
	if _, exists := db.store.Accounts["patel"]; !exists {
		db.store.Accounts["patel"] = Account{
			Username:       "patel",
			Password:       "patel123",
			Role:           "customer",
			Owner:          "reseller1",
			Plan:           "Standard Hosting Plan",
			Email:          "a.patel@digitalneo.net",
			DiskUsed:       120,
			DiskLimit:      5000,
			BandwidthUsed:  4,
			BandwidthLimit: 100,
			DomainsUsed:    2,
			DomainsLimit:   10,
			DatabasesLimit: 5,
			FTPLimit:       5,
			EmailLimit:     10,
			SharedIP:       "192.168.1.100",
			CreatedAt:      time.Now(),
		}
	}

	// Seed default domains
	if len(db.store.Domains) == 0 {
		db.store.Domains["blog.digitalneo.net"] = Domain{
			DomainName:    "blog.digitalneo.net",
			Owner:         "patel",
			PHPVersion:    "8.2",
			SSLActive:     true,
			SSLIssuer:     "Let's Encrypt Authority X3",
			SSLExpires:    time.Now().AddDate(0, 2, 15),
			WebDAVEnabled: false,
			GzipEnabled:   true,
			CreatedAt:     time.Now().AddDate(0, -1, 0),
		}
		db.store.Domains["shop.digitalneo.net"] = Domain{
			DomainName:    "shop.digitalneo.net",
			Owner:         "patel",
			PHPVersion:    "8.3",
			SSLActive:     true,
			SSLIssuer:     "Let's Encrypt Authority X3",
			SSLExpires:    time.Now().AddDate(0, 1, 28),
			WebDAVEnabled: true,
			GzipEnabled:   true,
			CreatedAt:     time.Now().AddDate(0, 0, -10),
		}
	}

	// Seed default reseller packages
	if len(db.store.ResellerPackages) == 0 {
		db.store.ResellerPackages["Premium-Personal"] = ResellerPackage{
			Name:             "Premium-Personal",
			Owner:            "admin",
			DiskQuota:        "2GB",
			Bandwidth:        "50GB",
			MaxDomains:       3,
			MaxDatabases:     5,
			MaxFTP:           5,
			MaxEmail:         10,
			HourlyEmailLimit: 100,
			LVECpuPct:        50,
			LVERamMB:         512,
			CreatedAt:        time.Now(),
		}
		db.store.ResellerPackages["Enterprise-Cluster"] = ResellerPackage{
			Name:             "Enterprise-Cluster",
			Owner:            "admin",
			DiskQuota:        "50GB",
			Bandwidth:        "1000GB",
			MaxDomains:       100,
			MaxDatabases:     250,
			MaxFTP:           250,
			MaxEmail:         500,
			HourlyEmailLimit: 1000,
			LVECpuPct:        100,
			LVERamMB:         2048,
			CreatedAt:        time.Now(),
		}
	}

	// Seed databases
	if len(db.store.Databases) == 0 {
		db.store.Databases["patel_wpblog"] = Database{
			Name:      "patel_wpblog",
			Owner:     "patel",
			DBUser:    "patel_wpuser",
			Password:  "dbpassword123",
			RemoteIPs: "%",
			CreatedAt: time.Now(),
		}
	}

	// Seed crons
	if len(db.store.CronJobs) == 0 {
		db.store.CronJobs["cron_1"] = CronJob{
			ID:        "cron_1",
			Owner:     "patel",
			TaskName:  "WordPress Core Cron",
			Command:   "php /home/patel/public_html/wp-cron.php",
			Schedule:  "*/15 * * * *",
			Active:    true,
			CreatedAt: time.Now(),
		}
		// Default system crons
		db.store.CronJobs["cron_system_scan"] = CronJob{
			ID:        "cron_system_scan",
			Owner:     "admin",
			TaskName:  "Weekly Full Server Malware Scan",
			Command:   "/usr/local/neocp/bin/scan-server --full",
			Schedule:  "0 3 * * 0", // Every Sunday at 3 AM
			Active:    true,
			CreatedAt: time.Now(),
		}
		db.store.CronJobs["cron_trash_purge"] = CronJob{
			ID:        "cron_trash_purge",
			Owner:     "admin",
			TaskName:  "Automatic 30-Day Trash Purge",
			Command:   "/usr/local/neocp/bin/purge-trash --days 30",
			Schedule:  "0 4 * * *", // Every day at 4 AM
			Active:    true,
			CreatedAt: time.Now(),
		}
	}

	// Seed a support ticket
	if len(db.store.Tickets) == 0 {
		db.store.Tickets["t_101"] = Ticket{
			ID:       "t_101",
			Owner:    "patel",
			Subject:  "Migration from cPanel assistance",
			Status:   "answered",
			Category: "Migration Support",
			Messages: []TicketMessage{
				{
					Sender:    "patel",
					Message:   "Hello team, I would like to migrate my main site from cPanel to NeoCP. Can you guide me on using the UZME tool?",
					Timestamp: time.Now().Add(-2 * time.Hour),
				},
				{
					Sender:    "admin",
					Message:   "Hello! The Universal Zero-Downtime Migration Engine (UZME) is fully automated. Simply go to Tools -> Migration, enter your old cPanel credential/root access, and NeoCP will sync all files and databases, and dynamically inject a reverse-proxy so you have zero DNS propagation downtime! Let us know if you need any assistance.",
					Timestamp: time.Now().Add(-1 * time.Hour),
				},
			},
			CreatedAt: time.Now().Add(-2 * time.Hour),
		}
	}

	// Seed IP Addresses
	if len(db.store.IPAddresses) == 0 {
		db.store.IPAddresses = []IPAddress{
			{IP: "192.168.1.100", Subnet: "255.255.255.0", IsShared: true, IsAssigned: true, Owner: "admin"},
			{IP: "192.168.1.101", Subnet: "255.255.255.0", IsShared: false, IsAssigned: false, Owner: "admin"},
		}
	}

	// Seed Server Config
	if db.store.Server.Hostname == "" {
		db.store.Server = ServerConfig{
			Hostname:    "neocp.professional.server",
			Nameservers: []string{"ns1.neocp.io", "ns2.neocp.io"},
			Resolvers:   []string{"8.8.8.8", "8.8.4.4"},
			SharedIP:    "192.168.1.100",
		}
	}

	db.save()
}

// ---------------- CRUD Operations for Multi-Tenancy ----------------

func (db *DatabaseEngine) GetAccounts() []Account {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var accs []Account
	for _, acc := range db.store.Accounts {
		accs = append(accs, acc)
	}
	return accs
}

func (db *DatabaseEngine) TransferOwnership(username, newOwner string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	acc, exists := db.store.Accounts[username]
	if !exists {
		return errors.New("account not found")
	}
	acc.Owner = newOwner
	db.store.Accounts[username] = acc
	db.save()
	return nil
}

func (db *DatabaseEngine) UpdateAccount(acc Account) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if _, exists := db.store.Accounts[acc.Username]; !exists {
		return errors.New("account not found")
	}
	db.store.Accounts[acc.Username] = acc
	db.save()
	return nil
}

func (db *DatabaseEngine) Authenticate(username, password string) (*Account, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	acc, exists := db.store.Accounts[username]
	if !exists || acc.Password != password {
		return nil, errors.New("invalid username or password")
	}
	return &acc, nil
}

func (db *DatabaseEngine) GetAccount(username string) (*Account, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	acc, exists := db.store.Accounts[username]
	if !exists {
		return nil, errors.New("account not found")
	}
	return &acc, nil
}

func (db *DatabaseEngine) GetDomains(owner string, isAdmin bool) []Domain {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var domains []Domain
	for _, dom := range db.store.Domains {
		if isAdmin || dom.Owner == owner {
			domains = append(domains, dom)
		}
	}
	return domains
}

func (db *DatabaseEngine) CreateDomain(dom Domain) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.store.Domains[dom.DomainName]; exists {
		return errors.New("domain already exists")
	}

	acc, exists := db.store.Accounts[dom.Owner]
	if exists {
		if acc.DomainsLimit > 0 && acc.DomainsUsed >= acc.DomainsLimit {
			return fmt.Errorf("domain registration limit reached (%d domains limit)", acc.DomainsLimit)
		}
		acc.DomainsUsed++
		db.store.Accounts[dom.Owner] = acc
	}

	db.store.Domains[dom.DomainName] = dom
	db.save()
	return nil
}

// ---------------- IP Management ----------------

func (db *DatabaseEngine) GetIPAddresses() []IPAddress {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.store.IPAddresses
}

func (db *DatabaseEngine) AddIPAddress(ip IPAddress) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	for _, existing := range db.store.IPAddresses {
		if existing.IP == ip.IP {
			return errors.New("IP address already exists")
		}
	}
	db.store.IPAddresses = append(db.store.IPAddresses, ip)
	db.save()
	return nil
}

func (db *DatabaseEngine) DelegateIP(ip, resellerID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	for i, existing := range db.store.IPAddresses {
		if existing.IP == ip {
			if existing.IsShared {
				return errors.New("cannot delegate shared IP")
			}
			db.store.IPAddresses[i].Owner = resellerID
			db.store.IPAddresses[i].IsAssigned = true
			db.save()
			return nil
		}
	}
	return errors.New("IP address not found")
}

func (db *DatabaseEngine) UpdateServerConfig(conf ServerConfig) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.store.Server = conf
	db.save()
}

func (db *DatabaseEngine) GetServerConfig() ServerConfig {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.store.Server
}

func (db *DatabaseEngine) DeleteDomain(domainName string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	dom, exists := db.store.Domains[domainName]
	if !exists {
		return errors.New("domain not found")
	}

	acc, accExists := db.store.Accounts[dom.Owner]
	if accExists {
		acc.DomainsUsed--
		if acc.DomainsUsed < 0 {
			acc.DomainsUsed = 0
		}
		db.store.Accounts[dom.Owner] = acc
	}

	delete(db.store.Domains, domainName)
	db.save()
	return nil
}


func (db *DatabaseEngine) UpdateDomainPHP(domainName, phpVersion string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	dom, exists := db.store.Domains[domainName]
	if !exists {
		return errors.New("domain not found")
	}
	dom.PHPVersion = phpVersion
	db.store.Domains[domainName] = dom
	db.save()
	return nil
}

func (db *DatabaseEngine) UpdateDomainSSL(domainName string, active bool, issuer string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	dom, exists := db.store.Domains[domainName]
	if !exists {
		return errors.New("domain not found")
	}
	dom.SSLActive = active
	dom.SSLIssuer = issuer
	dom.SSLExpires = time.Now().AddDate(0, 3, 0)
	db.store.Domains[domainName] = dom
	db.save()
	return nil
}

func (db *DatabaseEngine) UpdateDomainSettings(domainName string, gzip, brotli, hotlink, leech bool) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	dom, exists := db.store.Domains[domainName]
	if !exists {
		return errors.New("domain not found")
	}
	dom.GzipEnabled = gzip
	dom.BrotliEnabled = brotli
	dom.HotlinkProtected = hotlink
	dom.LeechProtected = leech
	db.store.Domains[domainName] = dom
	db.save()
	return nil
}

func (db *DatabaseEngine) UpdateDomainPrivacy(domainName string, active bool, user, pass string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	dom, exists := db.store.Domains[domainName]
	if !exists {
		return errors.New("domain not found")
	}
	dom.DirectoryPrivacy = active
	dom.DirectoryPrivacyUser = user
	dom.DirectoryPrivacyPass = pass
	db.store.Domains[domainName] = dom
	db.save()
	return nil
}

func (db *DatabaseEngine) UpdateDomainRedirect(domainName, redirectURL string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	dom, exists := db.store.Domains[domainName]
	if !exists {
		return errors.New("domain not found")
	}
	dom.RedirectURL = redirectURL
	db.store.Domains[domainName] = dom
	db.save()
	return nil
}

func (db *DatabaseEngine) GetDatabases(owner string, isAdmin bool) []Database {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var databases []Database
	for _, dbSpec := range db.store.Databases {
		if isAdmin || dbSpec.Owner == owner {
			databases = append(databases, dbSpec)
		}
	}
	return databases
}

func (db *DatabaseEngine) CreateDatabase(dbSpec Database) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.store.Databases[dbSpec.Name]; exists {
		return errors.New("database already exists")
	}
	db.store.Databases[dbSpec.Name] = dbSpec
	db.save()
	return nil
}

func (db *DatabaseEngine) DeleteDatabase(dbName string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.store.Databases[dbName]; !exists {
		return errors.New("database not found")
	}
	delete(db.store.Databases, dbName)
	db.save()
	return nil
}

func (db *DatabaseEngine) GetDatabase(dbName string) (*Database, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	d, exists := db.store.Databases[dbName]
	if !exists {
		return nil, errors.New("database not found")
	}
	return &d, nil
}

func (db *DatabaseEngine) UpdateDatabasePassword(dbName, newPass string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	d, exists := db.store.Databases[dbName]
	if !exists {
		return errors.New("database not found")
	}
	d.Password = newPass
	db.store.Databases[dbName] = d
	db.save()
	return nil
}

func (db *DatabaseEngine) UpdateDatabaseIPs(dbName, ips string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	dbSpec, exists := db.store.Databases[dbName]
	if !exists {
		return errors.New("database not found")
	}
	dbSpec.RemoteIPs = ips
	db.store.Databases[dbName] = dbSpec
	db.save()
	return nil
}

func (db *DatabaseEngine) GetCronJobs(owner string, isAdmin bool) []CronJob {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var jobs []CronJob
	for _, job := range db.store.CronJobs {
		if isAdmin || job.Owner == owner {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

func (db *DatabaseEngine) CreateCronJob(job CronJob) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.store.CronJobs[job.ID] = job
	db.save()
	return nil
}

func (db *DatabaseEngine) DeleteCronJob(id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.store.CronJobs[id]; !exists {
		return errors.New("cron job not found")
	}
	delete(db.store.CronJobs, id)
	db.save()
	return nil
}

func (db *DatabaseEngine) GetPackages() []ResellerPackage {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var pkgs []ResellerPackage
	for _, pkg := range db.store.ResellerPackages {
		pkgs = append(pkgs, pkg)
	}
	return pkgs
}

func (db *DatabaseEngine) CreatePackage(pkg ResellerPackage) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.store.ResellerPackages[pkg.Name]; exists {
		return errors.New("package name already exists")
	}

	// Verification logic for package owner (only admin or reseller) can be added here or in API layer

	db.store.ResellerPackages[pkg.Name] = pkg
	db.save()
	return nil
}

func (db *DatabaseEngine) UpdatePackage(pkg ResellerPackage) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.store.ResellerPackages[pkg.Name]; !exists {
		return errors.New("package not found")
	}

	db.store.ResellerPackages[pkg.Name] = pkg
	db.save()
	return nil
}

func (db *DatabaseEngine) GetPackagesByOwner(owner string) []ResellerPackage {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var pkgs []ResellerPackage
	for _, pkg := range db.store.ResellerPackages {
		if pkg.Owner == owner {
			pkgs = append(pkgs, pkg)
		}
	}
	return pkgs
}

func (db *DatabaseEngine) DeletePackage(name string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.store.ResellerPackages[name]; !exists {
		return errors.New("package not found")
	}
	delete(db.store.ResellerPackages, name)
	db.save()
	return nil
}

func (db *DatabaseEngine) GetTickets(owner string, isAdmin bool) []Ticket {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var tickets []Ticket
	for _, ticket := range db.store.Tickets {
		if isAdmin || ticket.Owner == owner {
			tickets = append(tickets, ticket)
		}
	}
	return tickets
}

func (db *DatabaseEngine) CreateTicket(ticket Ticket) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.store.Tickets[ticket.ID] = ticket
	db.save()
	return nil
}

func (db *DatabaseEngine) AddTicketReply(ticketID, sender, message string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	ticket, exists := db.store.Tickets[ticketID]
	if !exists {
		return errors.New("ticket not found")
	}
	ticket.Messages = append(ticket.Messages, TicketMessage{
		Sender:    sender,
		Message:   message,
		Timestamp: time.Now(),
	})
	if sender == "admin" {
		ticket.Status = "answered"
	} else {
		ticket.Status = "open"
	}
	db.store.Tickets[ticketID] = ticket
	db.save()
	return nil
}

func (db *DatabaseEngine) GetDockerContainers(owner string, isAdmin bool) []DockerContainer {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var containers []DockerContainer
	for _, cont := range db.store.DockerContainers {
		if isAdmin || cont.Owner == owner {
			containers = append(containers, cont)
		}
	}
	return containers
}

func (db *DatabaseEngine) DeployDocker(cont DockerContainer) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.store.DockerContainers[cont.Name] = cont
	db.save()
	return nil
}

func (db *DatabaseEngine) ToggleDockerStatus(name string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	cont, exists := db.store.DockerContainers[name]
	if !exists {
		return errors.New("container not found")
	}
	if cont.Status == "running" {
		cont.Status = "stopped"
	} else {
		cont.Status = "running"
	}
	db.store.DockerContainers[name] = cont
	db.save()
	return nil
}

func (db *DatabaseEngine) DeleteDocker(name string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.store.DockerContainers[name]; !exists {
		return errors.New("container not found")
	}
	delete(db.store.DockerContainers, name)
	db.save()
	return nil
}

func (db *DatabaseEngine) CreateMigration(task MigrationTask) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.store.MigrationTasks[task.ID] = task
	db.save()
	return nil
}

func (db *DatabaseEngine) UpdateMigrationProgress(id string, progress float64, status string, log string) {
	db.mu.Lock()
	defer db.mu.Unlock()

	task, exists := db.store.MigrationTasks[id]
	if !exists {
		return
	}
	task.ProgressPct = progress
	task.Status = status
	if log != "" {
		task.Logs = append(task.Logs, log)
	}
	db.store.MigrationTasks[id] = task
	db.save()
}

func (db *DatabaseEngine) GetMigrations(owner string, isAdmin bool) []MigrationTask {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var tasks []MigrationTask
	for _, task := range db.store.MigrationTasks {
		if isAdmin || task.Owner == owner {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// RecalculateDiskUsage audits user directory size physically on disk and sets DB state
func (db *DatabaseEngine) RecalculateDiskUsage(username string, sandboxDir string) (int64, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	acc, exists := db.store.Accounts[username]
	if !exists {
		return 0, errors.New("account not found")
	}

	userDir := filepath.Join(sandboxDir, username)
	
	// Create user directory if missing during lookup
	os.MkdirAll(userDir, 0755)

	var totalSize int64
	err := filepath.Walk(userDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // ignore unreadable paths
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	mbSize := totalSize / (1024 * 1024)
	if mbSize == 0 && totalSize > 0 {
		mbSize = 1 // Floor metric representation
	}

	acc.DiskUsed = mbSize
	db.store.Accounts[username] = acc
	db.save()
	return mbSize, nil
}

// CheckQuota enforces hard quotas on file uploads / actions
func (db *DatabaseEngine) CheckQuota(username string, sandboxDir string, sizeToAddBytes int64) error {
	used, err := db.RecalculateDiskUsage(username, sandboxDir)
	if err != nil {
		return err
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	acc := db.store.Accounts[username]
	if acc.DiskLimit > 0 {
		mbToAdd := sizeToAddBytes / (1024 * 1024)
		if used+mbToAdd >= acc.DiskLimit {
			return fmt.Errorf("quota violation: action exceeds storage package limit of %d MB (Current: %d MB)", acc.DiskLimit, used)
		}
	}
	return nil
}

func (db *DatabaseEngine) GetFirewallBlocks() []FirewallBlock {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var blocks []FirewallBlock
	for _, block := range db.store.FirewallBlocks {
		blocks = append(blocks, block)
	}
	return blocks
}

func (db *DatabaseEngine) BlockIP(ip string, reason string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.store.FirewallBlocks == nil {
		db.store.FirewallBlocks = make(map[string]FirewallBlock)
	}

	db.store.FirewallBlocks[ip] = FirewallBlock{
		IP:        ip,
		Reason:    reason,
		BlockedAt: time.Now(),
	}
	db.save()
	return nil
}

func (db *DatabaseEngine) UnblockIP(ip string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.store.FirewallBlocks[ip]; !exists {
		return errors.New("IP is not blocked")
	}

	delete(db.store.FirewallBlocks, ip)
	db.save()
	return nil
}

func (db *DatabaseEngine) GetDNSRecords(domainName string) ([]DNSRecord, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	dom, exists := db.store.Domains[domainName]
	if !exists {
		return nil, errors.New("domain not found")
	}

	records := dom.DNSRecords
	if records == nil {
		records = []DNSRecord{}
	}
	return records, nil
}

func (db *DatabaseEngine) AddDNSRecord(domainName string, rec DNSRecord) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	dom, exists := db.store.Domains[domainName]
	if !exists {
		return errors.New("domain not found")
	}

	if dom.DNSRecords == nil {
		dom.DNSRecords = []DNSRecord{}
	}

	// Check duplicates (e.g. same name + type + value)
	for _, r := range dom.DNSRecords {
		if r.Type == rec.Type && r.Name == rec.Name && r.Value == rec.Value {
			return errors.New("DNS record already exists")
		}
	}

	dom.DNSRecords = append(dom.DNSRecords, rec)
	db.store.Domains[domainName] = dom
	db.save()
	return nil
}

func (db *DatabaseEngine) DeleteDNSRecord(domainName string, id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	dom, exists := db.store.Domains[domainName]
	if !exists {
		return errors.New("domain not found")
	}

	found := false
	var updated []DNSRecord
	for _, r := range dom.DNSRecords {
		if r.ID == id {
			found = true
			continue
		}
		updated = append(updated, r)
	}

	if !found {
		return errors.New("DNS record not found")
	}

	if updated == nil {
		updated = []DNSRecord{}
	}

	dom.DNSRecords = updated
	db.store.Domains[domainName] = dom
	db.save()
	return nil
}

func (db *DatabaseEngine) UpdateWAFPolicy(domainName string, policy WAFPolicy) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	dom, exists := db.store.Domains[domainName]
	if !exists {
		return errors.New("domain not found")
	}

	dom.WAFPolicy = policy
	db.store.Domains[domainName] = dom
	db.save()
	return nil
}

func (db *DatabaseEngine) GetClusterNodes() []ClusterNode {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var nodes []ClusterNode
	for _, node := range db.store.ClusterNodes {
		nodes = append(nodes, node)
	}
	if nodes == nil {
		nodes = []ClusterNode{}
	}
	return nodes
}

func (db *DatabaseEngine) CreateClusterNode(node ClusterNode) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.store.ClusterNodes == nil {
		db.store.ClusterNodes = make(map[string]ClusterNode)
	}
	if _, exists := db.store.ClusterNodes[node.NodeID]; exists {
		return errors.New("node already exists")
	}
	db.store.ClusterNodes[node.NodeID] = node
	db.save()
	return nil
}

func (db *DatabaseEngine) UpdateClusterNodeMetrics(nodeID string, cpu, ram float64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	node, exists := db.store.ClusterNodes[nodeID]
	if !exists {
		return errors.New("node not found")
	}
	node.CPULoad = cpu
	node.RAMLoad = ram
	node.LastPing = time.Now()
	node.IsActive = true
	db.store.ClusterNodes[nodeID] = node
	db.save()
	return nil
}

func (db *DatabaseEngine) UpdateClusterNodeStatus(nodeID string, active bool) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	node, exists := db.store.ClusterNodes[nodeID]
	if !exists {
		return errors.New("node not found")
	}
	node.IsActive = active
	db.store.ClusterNodes[nodeID] = node
	db.save()
	return nil
}

func (db *DatabaseEngine) DeleteClusterNode(nodeID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.store.ClusterNodes[nodeID]; !exists {
		return errors.New("node not found")
	}
	delete(db.store.ClusterNodes, nodeID)
	db.save()
	return nil
}


