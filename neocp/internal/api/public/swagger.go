package public

import (
	"encoding/json"
	"net/http"
)

// SwaggerSpec returns the OpenAPI 3.0 specification for the public API
func SwaggerSpec(w http.ResponseWriter, r *http.Request) {
	spec := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]string{
			"title":   "NeoCP Public Provisioning API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/api/v1/provision": map[string]interface{}{
				"post": map[string]interface{}{
					"summary": "Provision a new hosting account",
					"responses": map[string]interface{}{
						"201": map[string]string{"description": "Account created"},
					},
				},
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(spec)
}
