package api

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"neocp/internal/core"
	"neocp/internal/oslayer"
)

type AddDNSRecordRequest struct {
	DomainName string         `json:"domain_name"`
	Record     core.DNSRecord `json:"record"`
}

type DeleteDNSRecordRequest struct {
	DomainName string `json:"domain_name"`
	RecordID   string `json:"record_id"`
}

// HandleDNSRecords manages DNS CRUD actions and BIND9 compilations
func HandleDNSRecords(w http.ResponseWriter, r *http.Request) {
	username := r.Header.Get("NeoCP-User")
	role := r.Header.Get("NeoCP-Role")
	isAdmin := (role == "admin")
	db := core.GetDB()

	// Get workspaceDir dynamically from context/main (we can grab it relative to DB file or Getwd)
	workspaceDir := "."

	if r.Method == http.MethodGet {
		domainName := r.URL.Query().Get("domain")
		if domainName == "" {
			http.Error(w, "Missing domain query parameter", http.StatusBadRequest)
			return
		}

		// Tenant authorization check
		domains := db.GetDomains(username, isAdmin)
		authorized := false
		for _, d := range domains {
			if d.DomainName == domainName {
				authorized = true
				break
			}
		}

		if !authorized {
			http.Error(w, "Forbidden: you do not own this domain", http.StatusForbidden)
			return
		}

		records, err := db.GetDNSRecords(domainName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(records)
		return
	}

	if r.Method == http.MethodPost {
		var req AddDNSRecordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad Request payload", http.StatusBadRequest)
			return
		}

		if req.DomainName == "" || req.Record.Type == "" || req.Record.Value == "" {
			http.Error(w, "Missing required record parameters", http.StatusBadRequest)
			return
		}

		// Tenant check
		domains := db.GetDomains(username, isAdmin)
		authorized := false
		for _, d := range domains {
			if d.DomainName == req.DomainName {
				authorized = true
				break
			}
		}

		if !authorized {
			http.Error(w, "Forbidden: domain not owned by tenant", http.StatusForbidden)
			return
		}

		// Clean up priority and TTL fallbacks
		if req.Record.TTL == 0 {
			req.Record.TTL = 86400
		}
		
		// Generate random simple unique ID
		rand.Seed(time.Now().UnixNano())
		req.Record.ID = fmt.Sprintf("dns_%d", rand.Intn(90000)+10000)

		// Insert into db
		err := db.AddDNSRecord(req.DomainName, req.Record)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Recompile BIND9 zone file
		records, _ := db.GetDNSRecords(req.DomainName)
		var osRecords []oslayer.DNSRecord
		for _, r := range records {
			osRecords = append(osRecords, oslayer.DNSRecord{
				ID:       r.ID,
				Type:     r.Type,
				Name:     r.Name,
				Value:    r.Value,
				TTL:      r.TTL,
				Priority: r.Priority,
			})
		}
		_, _ = oslayer.GenerateBind9ZoneFile(req.DomainName, osRecords, workspaceDir)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(req.Record)
		return
	}

	if r.Method == http.MethodDelete {
		var req DeleteDNSRecordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad Request payload", http.StatusBadRequest)
			return
		}

		if req.DomainName == "" || req.RecordID == "" {
			http.Error(w, "Missing domain_name or record_id parameters", http.StatusBadRequest)
			return
		}

		// Tenant check
		domains := db.GetDomains(username, isAdmin)
		authorized := false
		for _, d := range domains {
			if d.DomainName == req.DomainName {
				authorized = true
				break
			}
		}

		if !authorized {
			http.Error(w, "Forbidden: domain not owned by tenant", http.StatusForbidden)
			return
		}

		err := db.DeleteDNSRecord(req.DomainName, req.RecordID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Recompile BIND9 zone file
		records, _ := db.GetDNSRecords(req.DomainName)
		var osRecords []oslayer.DNSRecord
		for _, r := range records {
			osRecords = append(osRecords, oslayer.DNSRecord{
				ID:       r.ID,
				Type:     r.Type,
				Name:     r.Name,
				Value:    r.Value,
				TTL:      r.TTL,
				Priority: r.Priority,
			})
		}
		_, _ = oslayer.GenerateBind9ZoneFile(req.DomainName, osRecords, workspaceDir)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
