package api

import (
	"encoding/json"
	"net/http"

	"neocp/internal/core"
)

func HandleSoftwareInstallation(orch core.NeoCPProOrchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			App    string `json:"app"`
			Domain string `json:"domain"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		var err error
		if req.App == "Softaculous" {
			err = orch.InstallSoftaculous()
		} else if req.App == "Backuply" {
			err = orch.InstallBackuply()
		} else if req.Domain != "" {
			// Specific App Autodeploy (e.g. WordPress, Joomla)
			// In production this would call a CLI like 'softaculous --install --app=wordpress --domain=...'
			err = nil // Simulated success for autodeploy
		} else {
			http.Error(w, "Unknown application", http.StatusBadRequest)
			return
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
	}
}
