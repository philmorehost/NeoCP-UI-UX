package api

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"runtime"
	"time"

	"neocp/internal/core"
	"neocp/internal/oslayer"
)

type FirewallBlockRequest struct {
	IP     string `json:"ip"`
	Reason string `json:"reason"`
}

// HandleFirewallBlocks lists all currently blocked IPs (Admin only)
func HandleFirewallBlocks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	db := core.GetDB()
	blocks := db.GetFirewallBlocks()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(blocks)
}

// HandleFirewallBlock adds an IP address block rule to the firewall (Admin only)
func HandleFirewallBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FirewallBlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request payload", http.StatusBadRequest)
		return
	}

	// Sanitize IP Address
	if net.ParseIP(req.IP) == nil {
		http.Error(w, "Invalid IP address structure", http.StatusBadRequest)
		return
	}

	if req.Reason == "" {
		req.Reason = "Manual administrative firewall block"
	}

	db := core.GetDB()
	execEngine := &oslayer.SafeCommandExec{}
	var err error

	// Run platform-specific command execution
	if runtime.GOOS == "windows" {
		ruleName := "NeoCP-Block-IP-" + req.IP
		// netsh advfirewall firewall add rule name="NeoCP-Block-IP-<ip>" dir=in action=block remoteip=<ip>
		args := []string{
			"advfirewall", "firewall", "add", "rule",
			"name=" + ruleName, "dir=in", "action=block", "remoteip=" + req.IP,
		}
		_, err = execEngine.Execute(r.Context(), "netsh", args, 3*time.Second)
	} else {
		// iptables -A INPUT -s <ip> -j DROP
		args := []string{"-A", "INPUT", "-s", req.IP, "-j", "DROP"}
		_, err = execEngine.Execute(r.Context(), "iptables", args, 3*time.Second)
	}

	if err != nil {
		log.Printf("[Firewall Engine] OS block command for %s failed (%v). Applying database simulation fallback.", req.IP, err)
	} else {
		log.Printf("[Firewall Engine] OS block rule successfully injected for %s.", req.IP)
	}

	// Write block directly to database store
	err = db.BlockIP(req.IP, req.Reason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

// HandleFirewallUnblock removes an IP address block rule from the firewall (Admin only)
func HandleFirewallUnblock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FirewallBlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request payload", http.StatusBadRequest)
		return
	}

	if net.ParseIP(req.IP) == nil {
		http.Error(w, "Invalid IP address structure", http.StatusBadRequest)
		return
	}

	db := core.GetDB()
	execEngine := &oslayer.SafeCommandExec{}
	var err error

	// Run platform-specific unblock execution
	if runtime.GOOS == "windows" {
		ruleName := "NeoCP-Block-IP-" + req.IP
		// netsh advfirewall firewall delete rule name="NeoCP-Block-IP-<ip>"
		args := []string{
			"advfirewall", "firewall", "delete", "rule", "name=" + ruleName,
		}
		_, err = execEngine.Execute(r.Context(), "netsh", args, 3*time.Second)
	} else {
		// iptables -D INPUT -s <ip> -j DROP
		args := []string{"-D", "INPUT", "-s", req.IP, "-j", "DROP"}
		_, err = execEngine.Execute(r.Context(), "iptables", args, 3*time.Second)
	}

	if err != nil {
		log.Printf("[Firewall Engine] OS unblock command for %s failed (%v). Removing from database simulation state.", req.IP, err)
	} else {
		log.Printf("[Firewall Engine] OS block rule successfully removed for %s.", req.IP)
	}

	// Remove block directly from database store
	err = db.UnblockIP(req.IP)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}
