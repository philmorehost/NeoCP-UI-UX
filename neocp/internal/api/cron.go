package api

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"neocp/internal/core"
)

func HandleCronJobs(w http.ResponseWriter, r *http.Request) {
	username := r.Header.Get("NeoCP-User")
	role := r.Header.Get("NeoCP-Role")
	isAdmin := (role == "admin")
	db := core.GetDB()

	if r.Method == http.MethodGet {
		jobs := db.GetCronJobs(username, isAdmin)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jobs)
		return
	}

	if r.Method == http.MethodPost {
		var job core.CronJob
		if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Enforce ownership
		if !isAdmin {
			job.Owner = username
		}

		if job.ID == "" {
			rand.Seed(time.Now().UnixNano())
			job.ID = fmt.Sprintf("cron_%d", rand.Intn(90000)+10000)
		}
		job.CreatedAt = time.Now()

		err := db.CreateCronJob(job)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(job)
		return
	}

	if r.Method == http.MethodDelete {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id", http.StatusBadRequest)
			return
		}

		// Tenant check
		jobs := db.GetCronJobs(username, isAdmin)
		found := false
		for _, j := range jobs {
			if j.ID == id {
				found = true
				break
			}
		}

		if !found {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		err := db.DeleteCronJob(id)
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
