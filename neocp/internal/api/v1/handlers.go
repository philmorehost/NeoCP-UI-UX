package v1

import (
	"encoding/json"
	"net/http"
	"neocp/internal/core"
)

// RegisterV1Routes maps public REST API endpoints
func RegisterV1Routes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/domains", handlePublicDomains)
	mux.HandleFunc("/api/v1/status", handlePublicStatus)
}

func handlePublicDomains(w http.ResponseWriter, r *http.Request) {
	// In a real scenario, we would use API Token Scoping here
	db := core.GetDB()
	username := r.Header.Get("NeoCP-API-User")

	if username == "" {
		http.Error(w, "Unauthorized: API User required", http.StatusUnauthorized)
		return
	}

	domains := db.GetDomains(username, false)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domains)
}

func handlePublicStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]string{
		"version": "1.0.0-professional",
		"status":  "operational",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
