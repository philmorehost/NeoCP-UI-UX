package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"neocp/internal/core"
	"neocp/internal/oslayer"
)

type DeployContainerRequest struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Ports string `json:"ports"`
}

type ToggleContainerRequest struct {
	Name string `json:"name"`
}

// HandleDockerContainers manages GET (list) and DELETE (remove) actions
func HandleDockerContainers(w http.ResponseWriter, r *http.Request) {
	username := r.Header.Get("NeoCP-User")
	role := r.Header.Get("NeoCP-Role")
	isAdmin := (role == "admin")
	db := core.GetDB()

	if r.Method == http.MethodGet {
		// Attempts to fetch active containers from Docker CLI
		execEngine := &oslayer.SafeCommandExec{}
		output, err := execEngine.Execute(r.Context(), "docker", []string{
			"ps", "-a", "--format", "{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}\t{{.CreatedAt}}",
		}, 3*time.Second)

		if err != nil {
			// fallback to DB simulation mode
			log.Printf("[Docker Engine] CLI query failed (%v). Falling back to simulation mode.", err)
			containers := db.GetDockerContainers(username, isAdmin)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(containers)
			return
		}

		// Active Docker PS parsing
		lines := strings.Split(strings.TrimSpace(output), "\n")
		dbContainers := db.GetDockerContainers("", true) // get all to map owner
		ownerMap := make(map[string]string)
		for _, dbC := range dbContainers {
			ownerMap[dbC.Name] = dbC.Owner
		}

		var activeContainers []core.DockerContainer
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.Split(line, "\t")
			if len(parts) < 3 {
				continue
			}
			name := parts[0]
			image := parts[1]
			rawStatus := parts[2]
			ports := ""
			if len(parts) >= 4 {
				ports = parts[3]
			}

			status := "stopped"
			if strings.HasPrefix(rawStatus, "Up") {
				status = "running"
			}

			owner, exists := ownerMap[name]
			if !exists {
				// Container exists on OS but not in NeoCP, default to admin
				owner = "admin"
			}

			// Filter by ownership for multi-tenancy
			if !isAdmin && owner != username {
				continue
			}

			activeContainers = append(activeContainers, core.DockerContainer{
				Name:      name,
				Owner:     owner,
				Image:     image,
				Status:    status,
				Ports:     ports,
				CreatedAt: time.Now(), // rough mapping
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(activeContainers)
		return
	}

	if r.Method == http.MethodDelete {
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, "Missing container name", http.StatusBadRequest)
			return
		}

		// Tenancy Check
		containers := db.GetDockerContainers(username, isAdmin)
		found := false
		for _, c := range containers {
			if c.Name == name {
				found = true
				break
			}
		}

		if !found {
			http.Error(w, "Forbidden: container not owned by tenant", http.StatusForbidden)
			return
		}

		// Active execution attempt
		execEngine := &oslayer.SafeCommandExec{}
		_, err := execEngine.Execute(r.Context(), "docker", []string{"rm", "-f", name}, 5*time.Second)
		if err != nil {
			log.Printf("[Docker Engine] CLI remove failed (%v). Purging from DB simulation state.", err)
		}

		// Delete from core database store
		err = db.DeleteDocker(name)
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

// HandleDockerDeploy provisions a container
func HandleDockerDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := r.Header.Get("NeoCP-User")
	var req DeployContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request payload", http.StatusBadRequest)
		return
	}

	// Sanitize Name
	matched, _ := regexp.MatchString("^[a-zA-Z0-9_-]+$", req.Name)
	if !matched || len(req.Name) < 3 {
		http.Error(w, "Invalid container name. Only letters, numbers, dashes, and underscores permitted (min 3 chars).", http.StatusBadRequest)
		return
	}

	db := core.GetDB()

	// Build container DB metadata
	newCont := core.DockerContainer{
		Name:      req.Name,
		Owner:     username,
		Image:     req.Image,
		Status:    "running",
		Ports:     req.Ports,
		CreatedAt: time.Now(),
	}

	// Prepare Docker run CLI command arguments
	var dockerArgs []string
	dockerArgs = append(dockerArgs, "run", "-d", "--name", req.Name)
	if req.Ports != "" {
		dockerArgs = append(dockerArgs, "-p", req.Ports)
	}
	dockerArgs = append(dockerArgs, req.Image)

	execEngine := &oslayer.SafeCommandExec{}
	_, err := execEngine.Execute(r.Context(), "docker", dockerArgs, 10*time.Second)
	if err != nil {
		log.Printf("[Docker Engine] CLI deploy failed (%v). Spawning container in simulation state.", err)
	}

	// Save container state in NeoCP DB
	err = db.DeployDocker(newCont)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newCont)
}

// HandleDockerToggle starts or stops a container
func HandleDockerToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := r.Header.Get("NeoCP-User")
	role := r.Header.Get("NeoCP-Role")
	isAdmin := (role == "admin")

	var req ToggleContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request payload", http.StatusBadRequest)
		return
	}

	db := core.GetDB()

	// Verify tenancy
	containers := db.GetDockerContainers(username, isAdmin)
	var targetContainer *core.DockerContainer
	for i := range containers {
		if containers[i].Name == req.Name {
			targetContainer = &containers[i]
			break
		}
	}

	if targetContainer == nil {
		http.Error(w, "Container not found or access denied", http.StatusForbidden)
		return
	}

	// Toggle action in Docker CLI
	action := "start"
	if targetContainer.Status == "running" {
		action = "stop"
	}

	execEngine := &oslayer.SafeCommandExec{}
	_, err := execEngine.Execute(r.Context(), "docker", []string{action, req.Name}, 5*time.Second)
	if err != nil {
		log.Printf("[Docker Engine] CLI toggle (%s) failed (%v). Adjusting simulation state.", action, err)
	}

	// Toggle in local database
	err = db.ToggleDockerStatus(req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

// Helper function to check if Docker is running
func isDockerAvailable() bool {
	execEngine := &oslayer.SafeCommandExec{}
	_, err := execEngine.Execute(context.Background(), "docker", []string{"info"}, 2*time.Second)
	return err == nil
}
