package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestJWTAuthentication(t *testing.T) {
	username := "admin_test"
	role := "admin"

	t.Run("GenerateAndVerify", func(t *testing.T) {
		token, err := GenerateToken(username, role, 1*time.Hour)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		payload, err := VerifyToken(token)
		if err != nil {
			t.Fatalf("Failed to verify token: %v", err)
		}

		if payload.Username != username || payload.Role != role {
			t.Errorf("Payload mismatch: got %v", payload)
		}
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		token, _ := GenerateToken(username, role, -1*time.Minute)
		_, err := VerifyToken(token)
		if err == nil || !strings.Contains(err.Error(), "expired") {
			t.Errorf("Expected expiration error, got: %v", err)
		}
	})

	t.Run("TamperedToken", func(t *testing.T) {
		token, _ := GenerateToken(username, role, 1*time.Hour)
		parts := strings.Split(token, ".")
		// Tamper with the payload (middle part)
		tamperedPart := base64URLEncode([]byte(`{"username":"hacker","role":"admin","exp":9999999999}`))
		tamperedToken := parts[0] + "." + tamperedPart + "." + parts[2]

		_, err := VerifyToken(tamperedToken)
		if err == nil || !strings.Contains(err.Error(), "signature mismatch") {
			t.Errorf("Expected signature mismatch error, got: %v", err)
		}
	})
}

func TestRequireRoleMiddleware(t *testing.T) {
	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	t.Run("AuthorizedAdmin", func(t *testing.T) {
		token, _ := GenerateToken("boss", "admin", 1*time.Hour)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}
	})

	t.Run("UnauthorizedRole", func(t *testing.T) {
		token, _ := GenerateToken("pleb", "customer", 1*time.Hour)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden, got %d", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "forbidden resource access denied") {
			t.Errorf("Unexpected body: %s", rr.Body.String())
		}
	})

	t.Run("MissingToken", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized, got %d", rr.Code)
		}
	})
}
