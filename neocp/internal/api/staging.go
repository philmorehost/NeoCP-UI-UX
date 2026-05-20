package api

import (
	"encoding/json"
	"net/http"
	"neocp/internal/core/staging"
)

// StagingCloneRequest represents the staging clone POST body
type StagingCloneRequest struct {
	ProductionDomain string `json:"production_domain"`
	StagingSubdomain string `json:"staging_subdomain"`
}

// StagingPushRequest represents the staging push/pull production payload
type StagingPushRequest struct {
	StagingSubdomain string `json:"staging_subdomain"`
	SyncMode         string `json:"sync_mode"` // files, db, both
}

// HandleStaging manages 1-Click Staging actions
func HandleStaging(w http.ResponseWriter, r *http.Request) {
	username := r.Header.Get("NeoCP-User")
	sandboxDir := "sandbox" // Standard sandbox directory

	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodPost {
		// Detect whether cloning or pushing
		action := r.URL.Query().Get("action")
		if action == "clone" {
			var req StagingCloneRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request payload"})
				return
			}

			if req.ProductionDomain == "" || req.StagingSubdomain == "" {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "Missing production_domain or staging_subdomain"})
				return
			}

			if err := staging.CloneToStaging(username, req.ProductionDomain, req.StagingSubdomain, sandboxDir); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Cloned to staging successfully"})
			return
		} else if action == "push" {
			var req StagingPushRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request payload"})
				return
			}

			if req.StagingSubdomain == "" || req.SyncMode == "" {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "Missing staging_subdomain or sync_mode"})
				return
			}

			if err := staging.PushStagingToProduction(username, req.StagingSubdomain, req.SyncMode, sandboxDir); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Pushed staging changes to production successfully"})
			return
		} else {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid or missing action parameter"})
			return
		}
	}

	if r.Method == http.MethodDelete {
		subdomain := r.URL.Query().Get("subdomain")
		if subdomain == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Missing subdomain query parameter"})
			return
		}

		if err := staging.DeleteStaging(username, subdomain, sandboxDir); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Deleted staging environment successfully"})
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
	json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
}
