package api

import (
	"encoding/json"
	"net/http"
	"time"

	"neocp/internal/core"
)

func HandleResellerCenter(w http.ResponseWriter, r *http.Request) {
	role := r.Header.Get("NeoCP-Role")
	if role != "admin" {
		http.Error(w, "Forbidden: Admin only", http.StatusForbidden)
		return
	}

	db := core.GetDB()

	if r.Method == http.MethodGet {
		accounts := db.GetAccounts()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(accounts)
		return
	}

	orc := core.NewOrchestrator(db)

	if r.Method == http.MethodPost {
		action := r.URL.Query().Get("action")
		switch action {
		case "transfer_ownership":
			var req struct {
				Account  string `json:"account"`
				Reseller string `json:"reseller"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			err := orc.TransferAccountOwnership(req.Account, req.Reseller)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Write([]byte(`{"success":true}`))
			return

		case "delegate_ip":
			var req struct {
				Reseller string `json:"reseller"`
				IP       string `json:"ip"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			err := orc.DelegateIPToReseller(req.Reseller, req.IP)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Write([]byte(`{"success":true}`))
			return
		case "create_account":
			var acc core.Account
			if err := json.NewDecoder(r.Body).Decode(&acc); err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			acc.Owner = r.Header.Get("NeoCP-User")
			err := db.CreateAccount(acc)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Automatically create domain for the account
			db.CreateDomain(core.Domain{
				DomainName: acc.IPAddress, // overloaded IPAddress field used for domain in UI for now
				Owner:      acc.Username,
				PHPVersion: "8.2",
				CreatedAt:  time.Now(),
			})
			w.Write([]byte(`{"success":true}`))
			return
		case "update_account":
			var req core.Account
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			acc, err := db.GetAccount(req.Username)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			acc.Email = req.Email
			acc.Plan = req.Plan
			acc.Role = req.Role
			if req.Password != "" {
				acc.Password = req.Password
			}
			err = db.UpdateAccount(*acc)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Write([]byte(`{"success":true}`))
			return
		case "terminate_account":
			username := r.URL.Query().Get("username")
			if username == "" || username == "admin" {
				http.Error(w, "Invalid username", http.StatusBadRequest)
				return
			}
			err := db.DeleteAccount(username)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Cleanup domains for that user
			doms := db.GetDomains(username, false)
			for _, d := range doms {
				db.DeleteDomain(d.DomainName)
			}
			w.Write([]byte(`{"success":true}`))
			return
		}
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
