package api

import (
	"encoding/json"
	"net/http"

	"neocp/internal/core"
)

func HandleServerIdentity(w http.ResponseWriter, r *http.Request) {
	role := r.Header.Get("NeoCP-Role")
	if role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	db := core.GetDB()

	if r.Method == http.MethodGet {
		settings := db.GetSettings()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settings)
		return
	}

	orc := core.NewOrchestrator(db)

	if r.Method == http.MethodPost {
		action := r.URL.Query().Get("action")
		switch action {
		case "set_hostname":
			var req struct {
				Hostname string `json:"hostname"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			err := orc.SetServerHostname(req.Hostname)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Write([]byte(`{"success":true}`))
			return

		case "update_resolvers":
			var req struct {
				Primary   string `json:"primary"`
				Secondary string `json:"secondary"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			err := orc.UpdateResolvers(req.Primary, req.Secondary)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Write([]byte(`{"success":true}`))
			return

		case "assign_ip":
			var req struct {
				IP     string `json:"ip"`
				Subnet string `json:"subnet"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			err := orc.AssignIPv4ToInterface(req.IP, req.Subnet)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Write([]byte(`{"success":true}`))
			return
		}
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
