package api

import (
	"encoding/json"
	"net/http"

	"neocp/internal/core"
)

func HandleTransferLogs(w http.ResponseWriter, r *http.Request) {
	db := core.GetDB()

	if r.Method == http.MethodGet {
		logs := db.GetTransferLogs()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(logs)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
