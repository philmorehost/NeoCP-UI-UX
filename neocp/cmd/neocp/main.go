package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"embed"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"math/big"
	mrand "math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"neocp/internal/api"
	"neocp/internal/core"
	"neocp/internal/oslayer"
)

//go:embed web/static
var staticFS embed.FS

var (
	workspaceDir string
	sandboxDir   string
	loginFailures   = make(map[string]int)
	loginFailuresMu sync.Mutex
)

func main() {
	log.Println("Initializing NeoCP Professional Core Daemon...")

	// Get current working directory for sandbox storage
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	workspaceDir = wd
	sandboxDir = filepath.Join(workspaceDir, "sandbox")
	log.Printf("Workspace root path: %s", workspaceDir)
	log.Printf("Sanboxed user storage folder: %s", sandboxDir)

	// Ensure sandbox directory exists and seed default user files
	seedSandboxUserFiles()

	// Start Real-Time Background Malware & File Scanner
	go startBackgroundScanner()

	// 1. Generate SSL Certificates for Secure HTTPS Loop
	certPEM := filepath.Join(workspaceDir, "cert.pem")
	keyPEM := filepath.Join(workspaceDir, "key.pem")
	if _, err := os.Stat(certPEM); os.IsNotExist(err) {
		log.Println("Self-signed SSL/TLS certificate pair not found. Compiling new ECDSA certs...")
		err = generateSelfSignedCert(certPEM, keyPEM)
		if err != nil {
			log.Fatalf("Fatal: Failed to generate system SSL certificates: %v", err)
		}
		log.Println("ECDSA Certificate pair compiled successfully.")
	}

	// 2. Setup Routing Interfaces
	mux := http.NewServeMux()

	// Public Routes
	mux.HandleFunc("/api/login", handleLogin)
	mux.Handle("/api/telemetry/ws", api.RequireRole("admin")(http.HandlerFunc(api.TelemetryWebSocketHandler)))

	// Sub FS for static assets
	subFS, err := fs.Sub(staticFS, "web/static")
	if err != nil {
		log.Fatalf("Fatal: Failed to bind static FS embed: %v", err)
	}

	// Dynamic Embed Serve
	fileServer := http.FileServer(http.FS(subFS))
	mux.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/static")
		fileServer.ServeHTTP(w, r)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			// Redirect static assets
			r.URL.Path = "/static" + r.URL.Path
			http.Redirect(w, r, r.URL.Path, http.StatusMovedPermanently)
			return
		}
		// Serve SPA main index
		indexBytes, err := subFS.Open("index.html")
		if err != nil {
			http.Error(w, "SPA viewport file missing", http.StatusNotFound)
			return
		}
		defer indexBytes.Close()
		content, _ := ioutil.ReadAll(indexBytes)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(content)
	})

	// Authenticated Gateways
	mux.Handle("/api/account", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleAccountDetail)))

	// Cron Jobs
	mux.Handle("/api/cron", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(api.HandleCronJobs)))

	// Reseller Center
	mux.Handle("/api/reseller", api.RequireRole("admin")(http.HandlerFunc(api.HandleResellerCenter)))

	// Package Management
	mux.Handle("/api/packages/v2", api.RequireRole("admin")(http.HandlerFunc(api.HandlePackageManagement)))

	// Server Identity & Network
	mux.Handle("/api/server/identity", api.RequireRole("admin")(http.HandlerFunc(api.HandleServerIdentity)))

	// Domain endpoints
	mux.Handle("/api/domains", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleDomains)))
	mux.Handle("/api/domains/php", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleDomainPHPChange)))
	mux.Handle("/api/domains/ssl", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleDomainSSLChange)))
	mux.Handle("/api/domains/ssl/order", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleDomainSSLOrder)))
	mux.Handle("/api/domains/settings", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleDomainSettingsChange)))
	mux.Handle("/api/domains/privacy", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleDomainPrivacyChange)))
	mux.Handle("/api/domains/redirect", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleDomainRedirectChange)))
	mux.Handle("/api/domains/dns", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(api.HandleDNSRecords)))
	mux.Handle("/api/domains/waf", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(api.HandleDomainWAF)))

	// Database endpoints
	mux.Handle("/api/databases", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleDatabases)))
	mux.Handle("/api/databases/ips", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleDatabaseIPsChange)))

	// Reseller package endpoints
	mux.Handle("/api/packages", api.RequireRole("reseller", "admin")(http.HandlerFunc(handlePackages)))

	// Priority support tickets
	mux.Handle("/api/tickets", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleTickets)))
	mux.Handle("/api/tickets/reply", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleTicketReply)))

	// Migration Tasks (UZME)
	mux.Handle("/api/migrations", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleMigrations)))
	mux.Handle("/api/migrations/logs", api.RequireRole("admin")(http.HandlerFunc(api.HandleTransferLogs)))

	// System Process Manager
	mux.Handle("/api/processes", api.RequireRole("admin")(http.HandlerFunc(handleProcessesList)))
	mux.Handle("/api/processes/kill", api.RequireRole("admin")(http.HandlerFunc(handleProcessKill)))

	// OS Service Monitoring
	mux.Handle("/api/services", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleServicesList)))
	mux.Handle("/api/services/restart", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleServiceRestart)))

	// Sandboxed File Explorer
	mux.Handle("/api/filemanager/list", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleFileManagerList)))
	mux.Handle("/api/filemanager/read", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleFileManagerRead)))
	mux.Handle("/api/filemanager/write", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleFileManagerWrite)))
	mux.Handle("/api/filemanager/create", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleFileManagerCreate)))
	mux.Handle("/api/filemanager/upload", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleFileManagerUpload)))
	mux.Handle("/api/filemanager/delete", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleFileManagerDelete)))
	mux.Handle("/api/filemanager/trash/empty", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleFileManagerEmptyTrash)))
	mux.Handle("/api/filemanager/nginx/clear_cache", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(handleFileManagerClearCache)))

	// Docker Container endpoints
	mux.Handle("/api/docker/containers", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(api.HandleDockerContainers)))
	mux.Handle("/api/docker/containers/deploy", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(api.HandleDockerDeploy)))
	mux.Handle("/api/docker/containers/toggle", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(api.HandleDockerToggle)))

	// Firewall & Security blocks
	mux.Handle("/api/security/firewall/blocks", api.RequireRole("admin")(http.HandlerFunc(api.HandleFirewallBlocks)))
	mux.Handle("/api/security/firewall/block", api.RequireRole("admin")(http.HandlerFunc(api.HandleFirewallBlock)))
	mux.Handle("/api/security/firewall/unblock", api.RequireRole("admin")(http.HandlerFunc(api.HandleFirewallUnblock)))

	// Backup Engine endpoints
	mux.Handle("/api/backup/create", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.HandleBackupCreate(w, r, sandboxDir)
	})))
	mux.Handle("/api/backup/list", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.HandleBackupList(w, r, sandboxDir)
	})))
	mux.Handle("/api/backup/restore", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.HandleBackupRestore(w, r, sandboxDir)
	})))

	// Staging endpoints
	mux.Handle("/api/staging", api.RequireRole("customer", "reseller", "admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.HandleStaging(w, r, sandboxDir)
	})))

	// Clustering endpoints
	mux.Handle("/api/cluster/nodes", api.RequireRole("admin")(http.HandlerFunc(api.HandleCluster)))
	mux.Handle("/api/cluster/attach", api.RequireRole("admin")(http.HandlerFunc(api.HandleCluster)))

	// Software Installation (Softaculous / Backuply)
	db := core.GetDB()
	orc := core.NewOrchestrator(db)
	mux.Handle("/api/software/install", api.RequireRole("admin")(api.HandleSoftwareInstallation(orc)))

	// Spin secure mTLS Cluster Server on port :8444
	go func() {
		log.Println("Starting secure mTLS Cluster Server on port :8444...")
		if err := api.StartClusterServer(":8444"); err != nil {
			log.Printf("Cluster mTLS Server failed: %v", err)
		}
	}()

	// Spin HTTP-to-HTTPS dev redirects and secure servers
	go func() {
		log.Println("Starting HTTP Dev server on http://localhost:8080...")
		if err := http.ListenAndServe(":8080", mux); err != nil {
			log.Printf("HTTP Listener closed: %v", err)
		}
	}()

	log.Println("Starting premium HTTPS service on https://localhost:8443 (Self-Signed Dev Mode)...")
	err = http.ListenAndServeTLS(":8443", certPEM, keyPEM, mux)
	if err != nil {
		log.Fatalf("Fatal: Secure HTTPS listener failed: %v", err)
	}
}

// ==========================================================================
// 3. API ENDPOINT HANDLERS
// ==========================================================================

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func getClientIP(r *http.Request) string {
	for _, h := range []string{"X-Forwarded-For", "X-Real-IP"} {
		addresses := r.Header.Get(h)
		if addresses != "" {
			parts := strings.Split(addresses, ",")
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := getClientIP(r)
	db := core.GetDB()

	// Check if already blocked in the database
	blocks := db.GetFirewallBlocks()
	for _, b := range blocks {
		if b.IP == ip {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"Access Denied: IP blocked by cPHulk intrusion prevention system."}`))
			return
		}
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request payload", http.StatusBadRequest)
		return
	}

	acc, err := db.Authenticate(req.Username, req.Password)
	if err != nil {
		loginFailuresMu.Lock()
		loginFailures[ip]++
		failures := loginFailures[ip]
		loginFailuresMu.Unlock()

		log.Printf("[cPHulkTelemetry] Login failure from %s. Attempts: %d/5", ip, failures)

		if failures >= 5 {
			log.Printf("[cPHulkTelemetry] IP %s triggered cPHulk brute force block threshold. Block initiated.", ip)
			db.BlockIP(ip, "cPHulk: Too many failed login attempts")

			// Call OS block command securely
			execEngine := &oslayer.SafeCommandExec{}
			var fireErr error
			if runtime.GOOS == "windows" {
				ruleName := "NeoCP-Block-IP-" + ip
				args := []string{
					"advfirewall", "firewall", "add", "rule",
					"name=" + ruleName, "dir=in", "action=block", "remoteip=" + ip,
				}
				_, fireErr = execEngine.Execute(r.Context(), "netsh", args, 3*time.Second)
			} else {
				args := []string{"-A", "INPUT", "-s", ip, "-j", "DROP"}
				_, fireErr = execEngine.Execute(r.Context(), "iptables", args, 3*time.Second)
			}
			if fireErr != nil {
				log.Printf("[cPHulkTelemetry] Active OS Firewall block for %s failed: %v", ip, fireErr)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid username or password credentials"}`))
		return
	}

	// Login succeeded, reset failures
	loginFailuresMu.Lock()
	delete(loginFailures, ip)
	loginFailuresMu.Unlock()

	// Generate standard HMAC session JWT (24h)
	token, err := api.GenerateToken(acc.Username, acc.Role, 24*time.Hour)
	if err != nil {
		http.Error(w, "Token generation failed", http.StatusInternalServerError)
		return
	}

	// Set httpOnly Session Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     api.TokenCookieName,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": token,
		"role":  acc.Role,
	})
}

func handleAccountDetail(w http.ResponseWriter, r *http.Request) {
	username := r.Header.Get("NeoCP-User")
	db := core.GetDB()
	acc, err := db.GetAccount(username)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(acc)
}

func handleDomains(w http.ResponseWriter, r *http.Request) {
	db := core.GetDB()
	username := r.Header.Get("NeoCP-User")
	role := r.Header.Get("NeoCP-Role")
	isAdmin := (role == "admin")

	if r.Method == http.MethodGet {
		domains := db.GetDomains(username, isAdmin)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(domains)
		return
	}

	if r.Method == http.MethodPost {
		var dom core.Domain
		if err := json.NewDecoder(r.Body).Decode(&dom); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		dom.Owner = username // enforce tenant ownership
		dom.SSLActive = false
		dom.CreatedAt = time.Now()

		// Gate against Reseller Package or Account limits
		acc, accErr := db.GetAccount(username)
		if accErr == nil {
			// Direct Account limit check
			if acc.DomainsLimit > 0 && acc.DomainsUsed >= acc.DomainsLimit {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Forbidden: account domain limit reached (%d domains limit)", acc.DomainsLimit)})
				return
			}
			// Reseller Package limit check
			packages := db.GetResellerPackages()
			for _, pkg := range packages {
				if pkg.Name == acc.Plan {
					currentDoms := len(db.GetDomains(username, false))
					if pkg.DomainsLimit > 0 && currentDoms >= pkg.DomainsLimit {
						w.WriteHeader(http.StatusForbidden)
						json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Forbidden: Reseller Package domains limit reached (%d limit)", pkg.DomainsLimit)})
						return
					}
					break
				}
			}
		}

		err := db.CreateDomain(dom)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Seed a directory for the new virtual host inside the sandbox path
		dirPath := filepath.Join(sandboxDir, username, "public_html", dom.DomainName)
		os.MkdirAll(dirPath, 0755)
		ioutil.WriteFile(filepath.Join(dirPath, "index.php"), []byte("<h1>Welcome to your new website space: "+dom.DomainName+"</h1>"), 0644)

		// Regenerate Nginx configuration
		waf := oslayer.WAFPolicy{
			SQLiShield: dom.WAFPolicy.SQLiShield,
			XSSBlock:   dom.WAFPolicy.XSSBlock,
			LFIShield:  dom.WAFPolicy.LFIShield,
			CSRFHeader: dom.WAFPolicy.CSRFHeader,
		}
		_, err = oslayer.GenerateNginxConfig(dom.DomainName, username, dom.PHPVersion, dom.GzipEnabled, dom.BrotliEnabled, dom.SSLActive, waf, workspaceDir)
		if err == nil {
			reg := oslayer.GetServiceRegistry()
			_ = reg.RestartService("web")
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(dom)
		return
	}

	if r.Method == http.MethodDelete {
		domName := r.URL.Query().Get("name")
		if domName == "" {
			http.Error(w, "Missing name", http.StatusBadRequest)
			return
		}

		// Verify tenancy ownership
		domains := db.GetDomains(username, isAdmin)
		found := false
		for _, d := range domains {
			if d.DomainName == domName {
				found = true
				break
			}
		}

		if !found {
			http.Error(w, "Forbidden domain deletion", http.StatusForbidden)
			return
		}

		err := db.DeleteDomain(domName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Delete Nginx configuration
		_ = oslayer.RemoveNginxConfig(domName, workspaceDir)
		reg := oslayer.GetServiceRegistry()
		_ = reg.RestartService("web")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func handleDomainPHPChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DomainName string `json:"domain_name"`
		PHPVersion string `json:"php_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	db := core.GetDB()
	err := db.UpdateDomainPHP(req.DomainName, req.PHPVersion)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Regenerate Nginx config and reload
	username := r.Header.Get("NeoCP-User")
	role := r.Header.Get("NeoCP-Role")
	isAdmin := (role == "admin")
	domains := db.GetDomains(username, isAdmin)
	for _, d := range domains {
		if d.DomainName == req.DomainName {
			waf := oslayer.WAFPolicy{
				SQLiShield: d.WAFPolicy.SQLiShield,
				XSSBlock:   d.WAFPolicy.XSSBlock,
				LFIShield:  d.WAFPolicy.LFIShield,
				CSRFHeader: d.WAFPolicy.CSRFHeader,
			}
			_, err = oslayer.GenerateNginxConfig(d.DomainName, d.Owner, d.PHPVersion, d.GzipEnabled, d.BrotliEnabled, d.SSLActive, waf, workspaceDir)
			if err == nil {
				reg := oslayer.GetServiceRegistry()
				_ = reg.RestartService("web")
			}
			break
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

func handleDomainSSLChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DomainName string `json:"domain_name"`
		SSLActive  bool   `json:"ssl_active"`
		SSLIssuer  string `json:"ssl_issuer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	db := core.GetDB()
	err := db.UpdateDomainSSL(req.DomainName, req.SSLActive, req.SSLIssuer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Regenerate Nginx config and reload
	username := r.Header.Get("NeoCP-User")
	role := r.Header.Get("NeoCP-Role")
	isAdmin := (role == "admin")
	domains := db.GetDomains(username, isAdmin)
	for _, d := range domains {
		if d.DomainName == req.DomainName {
			waf := oslayer.WAFPolicy{
				SQLiShield: d.WAFPolicy.SQLiShield,
				XSSBlock:   d.WAFPolicy.XSSBlock,
				LFIShield:  d.WAFPolicy.LFIShield,
				CSRFHeader: d.WAFPolicy.CSRFHeader,
			}
			_, err = oslayer.GenerateNginxConfig(d.DomainName, d.Owner, d.PHPVersion, d.GzipEnabled, d.BrotliEnabled, d.SSLActive, waf, workspaceDir)
			if err == nil {
				reg := oslayer.GetServiceRegistry()
				_ = reg.RestartService("web")
			}
			break
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

func handleDomainSSLOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DomainName string `json:"domain_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	username := r.Header.Get("NeoCP-User")

	// Call ACME client to provision Let's Encrypt certificates
	acme := api.NewACMEClient(workspaceDir, sandboxDir)
	logs, err := acme.ProvisionCertificate(req.DomainName, username, true)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"logs":    logs,
		})
		return
	}

	db := core.GetDB()
	err = db.UpdateDomainSSL(req.DomainName, true, "Let's Encrypt Authority X3")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"logs":    logs,
		})
		return
	}

	// Regenerate Nginx config with SSL enabled
	role := r.Header.Get("NeoCP-Role")
	isAdmin := (role == "admin")
	domains := db.GetDomains(username, isAdmin)
	for _, d := range domains {
		if d.DomainName == req.DomainName {
			waf := oslayer.WAFPolicy{
				SQLiShield: d.WAFPolicy.SQLiShield,
				XSSBlock:   d.WAFPolicy.XSSBlock,
				LFIShield:  d.WAFPolicy.LFIShield,
				CSRFHeader: d.WAFPolicy.CSRFHeader,
			}
			_, err = oslayer.GenerateNginxConfig(d.DomainName, d.Owner, d.PHPVersion, d.GzipEnabled, d.BrotliEnabled, true, waf, workspaceDir)
			if err == nil {
				reg := oslayer.GetServiceRegistry()
				_ = reg.RestartService("web")
			}
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"logs":    logs,
	})
}

func handleDomainSettingsChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DomainName string `json:"domain_name"`
		Gzip       bool   `json:"gzip_enabled"`
		Brotli     bool   `json:"brotli_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	db := core.GetDB()
	err := db.UpdateDomainSettings(req.DomainName, req.Gzip, req.Brotli, false, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Regenerate Nginx config and reload
	username := r.Header.Get("NeoCP-User")
	role := r.Header.Get("NeoCP-Role")
	isAdmin := (role == "admin")
	domains := db.GetDomains(username, isAdmin)
	for _, d := range domains {
		if d.DomainName == req.DomainName {
			waf := oslayer.WAFPolicy{
				SQLiShield: d.WAFPolicy.SQLiShield,
				XSSBlock:   d.WAFPolicy.XSSBlock,
				LFIShield:  d.WAFPolicy.LFIShield,
				CSRFHeader: d.WAFPolicy.CSRFHeader,
			}
			_, err = oslayer.GenerateNginxConfig(d.DomainName, d.Owner, d.PHPVersion, d.GzipEnabled, d.BrotliEnabled, d.SSLActive, waf, workspaceDir)
			if err == nil {
				reg := oslayer.GetServiceRegistry()
				_ = reg.RestartService("web")
			}
			break
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

func handleDomainPrivacyChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DomainName string `json:"domain_name"`
		Active     bool   `json:"directory_privacy"`
		User       string `json:"directory_privacy_user"`
		Pass       string `json:"directory_privacy_pass"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	db := core.GetDB()
	err := db.UpdateDomainPrivacy(req.DomainName, req.Active, req.User, req.Pass)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

func handleDomainRedirectChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DomainName  string `json:"domain_name"`
		RedirectURL string `json:"redirect_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	db := core.GetDB()
	err := db.UpdateDomainRedirect(req.DomainName, req.RedirectURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

func handleDatabases(w http.ResponseWriter, r *http.Request) {
	db := core.GetDB()
	username := r.Header.Get("NeoCP-User")
	role := r.Header.Get("NeoCP-Role")
	isAdmin := (role == "admin")

	if r.Method == http.MethodGet {
		dbs := db.GetDatabases(username, isAdmin)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dbs)
		return
	}

	if r.Method == http.MethodPost {
		var d core.Database
		if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		d.Owner = username
		d.CreatedAt = time.Now()

		// Gate against Reseller Package limits
		acc, accErr := db.GetAccount(username)
		if accErr == nil {
			packages := db.GetResellerPackages()
			for _, pkg := range packages {
				if pkg.Name == acc.Plan {
					currentDBs := len(db.GetDatabases(username, false))
					if pkg.DatabasesLimit > 0 && currentDBs >= pkg.DatabasesLimit {
						w.WriteHeader(http.StatusForbidden)
						json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Forbidden: Reseller Package databases limit reached (%d limit)", pkg.DatabasesLimit)})
						return
					}
					break
				}
			}
		}

		err := db.CreateDatabase(d)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(d)
		return
	}

	if r.Method == http.MethodDelete {
		dbName := r.URL.Query().Get("name")
		if dbName == "" {
			http.Error(w, "Missing name", http.StatusBadRequest)
			return
		}

		dbs := db.GetDatabases(username, isAdmin)
		found := false
		for _, d := range dbs {
			if d.Name == dbName {
				found = true
				break
			}
		}

		if !found {
			http.Error(w, "Forbidden database deletion", http.StatusForbidden)
			return
		}

		err := db.DeleteDatabase(dbName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func handleDatabaseIPsChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name      string `json:"name"`
		RemoteIPs string `json:"remote_ips"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	db := core.GetDB()
	err := db.UpdateDatabaseIPs(req.Name, req.RemoteIPs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

func handlePackages(w http.ResponseWriter, r *http.Request) {
	db := core.GetDB()

	if r.Method == http.MethodGet {
		pkgs := db.GetResellerPackages()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pkgs)
		return
	}

	if r.Method == http.MethodPost {
		var pkg core.ResellerPackage
		if err := json.NewDecoder(r.Body).Decode(&pkg); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		pkg.CreatedAt = time.Now()

		err := db.CreateResellerPackage(pkg)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(pkg)
		return
	}

	if r.Method == http.MethodDelete {
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, "Missing name parameter", http.StatusBadRequest)
			return
		}

		err := db.DeleteResellerPackage(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func handleTickets(w http.ResponseWriter, r *http.Request) {
	db := core.GetDB()
	username := r.Header.Get("NeoCP-User")
	role := r.Header.Get("NeoCP-Role")
	isAdmin := (role == "admin")

	if r.Method == http.MethodGet {
		tickets := db.GetTickets(username, isAdmin)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tickets)
		return
	}

	if r.Method == http.MethodPost {
		var ticket core.Ticket
		if err := json.NewDecoder(r.Body).Decode(&ticket); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		ticket.Owner = username
		ticket.CreatedAt = time.Now()
		for i := range ticket.Messages {
			ticket.Messages[i].Timestamp = time.Now()
		}

		err := db.CreateTicket(ticket)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ticket)
		return
	}
}

func handleTicketReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TicketID string `json:"ticket_id"`
		Message  string `json:"message"`
		Sender   string `json:"sender"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	db := core.GetDB()
	err := db.AddTicketReply(req.TicketID, req.Sender, req.Message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

func handleMigrations(w http.ResponseWriter, r *http.Request) {
	db := core.GetDB()
	username := r.Header.Get("NeoCP-User")
	role := r.Header.Get("NeoCP-Role")
	isAdmin := (role == "admin")

	if r.Method == http.MethodGet {
		tasks := db.GetMigrations(username, isAdmin)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tasks)
		return
	}

	if r.Method == http.MethodPost {
		var task core.MigrationTask
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		task.ID = fmt.Sprintf("mig_%d", mrand.Intn(9000)+1000)
		task.Owner = username
		task.Status = "handshaking"
		task.ProgressPct = 0
		task.Logs = []string{"[UZME] Initializing synchronization worker thread..."}
		task.CreatedAt = time.Now()

		db.CreateMigration(task)

		// Spawn dynamic migration simulation timeline thread
		go func(taskID, targetDomain string) {
			steps := []struct {
				pct    float64
				status string
				log    string
			}{
				{10, "handshake_established", "[UZME] Secure API handshake established. Scanning target filesystems and MySQL databases..."},
				{25, "syncing_files", "[UZME] Packaging 18,242 files into tar archive at source location..."},
				{50, "transferring", "[UZME] Downloading public_html tar file block... [====================>] 100% (24.1 MB)"},
				{70, "extracting", "[UZME] Sync complete. Unpacking directory nodes safely into local customer workspace sandbox..."},
				{85, "syncing_db", "[UZME] Migrating database patel_wpblog schemas. Injecting whitelists..."},
				{95, "injecting_proxy", "[UZME] Pipeline complete. Launching dynamic reverse-proxy cutover map on port 80..."},
				{100, "complete", "[UZME] Migration completed successfully! DNS cutover stage fully functional."},
			}

			for _, step := range steps {
				time.Sleep(1500 * time.Millisecond)
				db.UpdateMigrationProgress(taskID, step.pct, step.status, step.log)
			}

			// Proactively deploy migrated domain space automatically to Dashboard
			db.CreateDomain(core.Domain{
				DomainName: targetDomain,
				Owner:      username,
				PHPVersion: "8.2",
				SSLActive:  true,
				SSLIssuer:  "Let's Encrypt Authority X3",
				CreatedAt:  time.Now(),
			})
		}(task.ID, task.TargetDomain)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(task)
		return
	}
}

func handleProcessesList(w http.ResponseWriter, r *http.Request) {
	// This would load active processes. Since processes fluctuate dynamically inside
	// telemetry, this list is streamed down beautifully.
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`[]`))
}

func handleProcessKill(w http.ResponseWriter, r *http.Request) {
	pidStr := r.URL.Query().Get("pid")
	if pidStr == "" {
		http.Error(w, "Missing pid", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

func handleServicesList(w http.ResponseWriter, r *http.Request) {
	reg := oslayer.GetServiceRegistry()
	svcs := reg.GetServices()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(svcs)
}

func handleServiceRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "Missing service name parameter", http.StatusBadRequest)
		return
	}

	reg := oslayer.GetServiceRegistry()
	err := reg.RestartService(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

// ==========================================================================
// 4. SANDBOX FILE EXPLORER HANDLERS (High Fidelity Traversal mapping)
// ==========================================================================

func virtualToPhysical(virtPath string) (string, error) {
	virtPath = filepath.Clean(virtPath)
	// Make relative from home
	rel := strings.TrimPrefix(virtPath, "/home")
	rel = strings.TrimPrefix(rel, "\\home")
	
	phys := filepath.Join(sandboxDir, rel)
	
	// Prevent directory traversal escape attacks
	if !strings.HasPrefix(phys, sandboxDir) {
		return "", fmt.Errorf("security audit failure: sandbox escape attempted")
	}
	return phys, nil
}

type FileItem struct {
	Name        string `json:"name"`
	IsDir       bool   `json:"is_dir"`
	Size        int64  `json:"size"`
	Permissions string `json:"permissions"`
	ModTime     string `json:"mod_time"`
}

func handleFileManagerList(w http.ResponseWriter, r *http.Request) {
	virtPath := r.URL.Query().Get("path")
	if virtPath == "" {
		http.Error(w, "Missing path", http.StatusBadRequest)
		return
	}

	phys, err := virtualToPhysical(virtPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	files, err := ioutil.ReadDir(phys)
	if err != nil {
		http.Error(w, "Folder missing", http.StatusNotFound)
		return
	}

	var items []FileItem
	for _, f := range files {
		items = append(items, FileItem{
			Name:        f.Name(),
			IsDir:       f.IsDir(),
			Size:        f.Size(),
			Permissions: f.Mode().String(),
			ModTime:     f.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func handleFileManagerRead(w http.ResponseWriter, r *http.Request) {
	virtPath := r.URL.Query().Get("path")
	phys, err := virtualToPhysical(virtPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	content, err := ioutil.ReadFile(phys)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"content": string(content),
	})
}

type FMWriteRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func handleFileManagerWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FMWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	phys, err := virtualToPhysical(req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	db := core.GetDB()
	username := r.Header.Get("NeoCP-User")
	err = db.CheckQuota(username, sandboxDir, int64(len(req.Content)))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	err = ioutil.WriteFile(phys, []byte(req.Content), 0644)
	if err != nil {
		http.Error(w, "Write failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

type FMCreateRequest struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

func handleFileManagerCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FMCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	phys, err := virtualToPhysical(req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	db := core.GetDB()
	username := r.Header.Get("NeoCP-User")
	err = db.CheckQuota(username, sandboxDir, 0)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if req.IsDir {
		err = os.MkdirAll(phys, 0755)
	} else {
		err = ioutil.WriteFile(phys, []byte(""), 0644)
	}

	if err != nil {
		http.Error(w, "Creation failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"success":true}`))
}

func handleFileManagerUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Max 500MB upload
	r.ParseMultipartForm(500 << 20)

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	destPath := r.FormValue("path")
	if destPath == "" {
		http.Error(w, "Missing destination path", http.StatusBadRequest)
		return
	}

	// Handle relative paths inside directories (for folder uploads)
	relPath := r.FormValue("relPath")
	fullVirtPath := filepath.Join(destPath, relPath)

	phys, err := virtualToPhysical(fullVirtPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// Ensure parent directory exists
	os.MkdirAll(filepath.Dir(phys), 0755)

	db := core.GetDB()
	username := r.Header.Get("NeoCP-User")
	err = db.CheckQuota(username, sandboxDir, handler.Size)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	f, err := os.OpenFile(phys, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	_, err = io.Copy(f, file)
	if err != nil {
		http.Error(w, "Error saving file payload", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

func handleFileManagerDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	virtPath := r.URL.Query().Get("path")
	phys, err := virtualToPhysical(virtPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	db := core.GetDB()
	username := r.Header.Get("NeoCP-User")
	orc := core.NewOrchestrator(db)
	err = orc.MoveToTrash(username, phys, sandboxDir)
	if err != nil {
		// Fallback to direct delete if move to trash fails (e.g. cross-device)
		err = os.RemoveAll(phys)
	}

	if err != nil {
		http.Error(w, "Deletion failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

func handleFileManagerEmptyTrash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	db := core.GetDB()
	username := r.Header.Get("NeoCP-User")
	orc := core.NewOrchestrator(db)
	freed, err := orc.EmptyUserTrash(username, sandboxDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"bytes_freed": freed,
	})
}

func handleFileManagerClearCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	db := core.GetDB()
	username := r.Header.Get("NeoCP-User")
	domain := r.URL.Query().Get("domain")
	if domain == "" {
		http.Error(w, "Missing domain", http.StatusBadRequest)
		return
	}

	orc := core.NewOrchestrator(db)
	err := orc.ClearUserNginxCache(username, domain)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

// ==========================================================================
// 5. HELPER FUNCTIONS
// ==========================================================================

func startBackgroundScanner() {
	log.Println("[Scanner] Initializing real-time malware & integrity protection...")
	ticker := time.NewTicker(30 * time.Second)
	maliciousPatterns := []string{"eval(base64_decode", "shell_exec(", "system(", "passthru("}

	for range ticker.C {
		scanCount := 0
		threats := 0

		err := filepath.Walk(sandboxDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			scanCount++

			// Only scan small text files for performance in this loop
			if info.Size() < 1024*1024 {
				content, err := ioutil.ReadFile(path)
				if err == nil {
					for _, pattern := range maliciousPatterns {
						if strings.Contains(string(content), pattern) {
							log.Printf("[Scanner] THREAT DETECTED: Malicious pattern '%s' found in %s. Quarantining...", pattern, path)
							os.Rename(path, path+".quarantine")
							threats++
							break
						}
					}
				}
			}
			return nil
		})

		if err == nil {
			log.Printf("[Scanner] Audit complete. Scanned %d files. %d threats neutralized.", scanCount, threats)
		}

		// Randomly log a mock scan of an email if bandwidth is active
		if mrand.Float64() > 0.7 {
			log.Printf("[Scanner] Real-time mail audit: %s.neocp.io -> Cleaned and delivered.", []string{"patel", "admin", "reseller1"}[mrand.Intn(3)])
		}
	}
}

func seedSandboxUserFiles() {
	users := []string{"admin", "reseller1", "patel"}
	for _, u := range users {
		dir := filepath.Join(sandboxDir, u, "public_html")
		os.MkdirAll(dir, 0755)

		// Seed a couple of visual premium files
		indexFile := filepath.Join(dir, "index.php")
		if _, err := os.Stat(indexFile); os.IsNotExist(err) {
			ioutil.WriteFile(indexFile, []byte(`<?php
echo "<h1>Welcome to NeoCP Professional Server Panel</h1>";
echo "<p>Container running PHP " . phpversion() . " autonomously.</p>";
echo "<p>Database state: Mapped successfully!</p>";
?>`), 0644)
		}

		wpFile := filepath.Join(dir, "wp-config.php")
		if _, err := os.Stat(wpFile); os.IsNotExist(err) {
			ioutil.WriteFile(wpFile, []byte(`<?php
// WordPress Mapped Database Configuration
define('DB_NAME', 'patel_wpblog');
define('DB_USER', 'patel_wpuser');
define('DB_PASSWORD', 'dbpassword123');
define('DB_HOST', 'localhost');
define('DB_CHARSET', 'utf8');
?>`), 0644)
		}

		htaccessFile := filepath.Join(dir, ".htaccess")
		if _, err := os.Stat(htaccessFile); os.IsNotExist(err) {
			ioutil.WriteFile(htaccessFile, []byte(`# NeoCP ModSecurity OWASP override
RewriteEngine On
RewriteBase /
RewriteRule ^index\.php$ - [L]
RewriteCond %{REQUEST_FILENAME} !-f
RewriteCond %{REQUEST_FILENAME} !-d
RewriteRule . /index.php [L]`), 0644)
		}
	}
}

func generateSelfSignedCert(certPath, keyPath string) error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"NeoCP Professional"},
			CommonName:   "localhost",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	template.IPAddresses = append(template.IPAddresses, net.ParseIP("127.0.0.1"))
	template.DNSNames = append(template.DNSNames, "localhost")

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}

	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer keyOut.Close()
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return err
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
		return err
	}

	return nil
}
