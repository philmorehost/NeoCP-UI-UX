package api

import (
	"encoding/json"
	"net/http"

	"neocp/internal/core"
	"neocp/internal/oslayer"
)

type UpdateWAFRequest struct {
	DomainName string         `json:"domain_name"`
	WAFPolicy  core.WAFPolicy `json:"waf_policy"`
}

// HandleDomainWAF manages OWASP WAF policy changes and triggers configuration rewrites
func HandleDomainWAF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := r.Header.Get("NeoCP-User")
	role := r.Header.Get("NeoCP-Role")
	isAdmin := (role == "admin")
	db := core.GetDB()

	// Get workspaceDir dynamically (use local path "." or relative context)
	workspaceDir := "."

	var req UpdateWAFRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request payload", http.StatusBadRequest)
		return
	}

	if req.DomainName == "" {
		http.Error(w, "Missing domain_name parameter", http.StatusBadRequest)
		return
	}

	// Verify tenant ownership boundaries
	domains := db.GetDomains(username, isAdmin)
	var targetDomain *core.Domain
	for _, d := range domains {
		if d.DomainName == req.DomainName {
			targetDomain = &d
			break
		}
	}

	if targetDomain == nil {
		http.Error(w, "Forbidden: domain not owned by tenant", http.StatusForbidden)
		return
	}

	// Update in database
	err := db.UpdateWAFPolicy(req.DomainName, req.WAFPolicy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Regenerate Nginx configuration incorporating WAF rules
	_, err = oslayer.GenerateNginxConfig(
		targetDomain.DomainName,
		targetDomain.Owner,
		targetDomain.PHPVersion,
		targetDomain.GzipEnabled,
		targetDomain.BrotliEnabled,
		targetDomain.SSLActive,
		workspaceDir,
	)
	if err == nil {
		reg := oslayer.GetServiceRegistry()
		_ = reg.RestartService("web")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
