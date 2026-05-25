package api

import (
	"encoding/json"
	"net/http"
	"time"

	"neocp/internal/core"
)

func HandlePackageManagement(w http.ResponseWriter, r *http.Request) {
	db := core.GetDB()

	if r.Method == http.MethodGet {
		pkgs := db.GetPackages()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pkgs)
		return
	}

	if r.Method == http.MethodPost {
		var pkg core.PackageDefinition
		if err := json.NewDecoder(r.Body).Decode(&pkg); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		pkg.CreatedAt = time.Now()

		err := db.CreatePackage(pkg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(pkg)
		return
	}

	if r.Method == http.MethodDelete {
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, "Missing name", http.StatusBadRequest)
			return
		}

		err := db.DeletePackage(name)
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
