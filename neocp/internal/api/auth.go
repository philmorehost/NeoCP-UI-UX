package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	// Secure system-wide master key for JWT signatures
	jwtSecret      = []byte("neocp_professional_secret_key_2026")
	TokenCookieName = "neocp_auth_token"
)

type JWTHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type JWTPayload struct {
	Username string `json:"username"`
	Role     string `json:"role"` // admin, reseller, customer
	Exp      int64  `json:"exp"`
}

func base64URLEncode(src []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(src), "=")
}

func base64URLDecode(src string) ([]byte, error) {
	if l := len(src) % 4; l > 0 {
		src += strings.Repeat("=", 4-l)
	}
	return base64.URLEncoding.DecodeString(src)
}

// GenerateToken creates an HMAC-SHA256 signed JWT
func GenerateToken(username, role string, duration time.Duration) (string, error) {
	header := JWTHeader{Alg: "HS256", Typ: "JWT"}
	headerBytes, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	payload := JWTPayload{
		Username: username,
		Role:     role,
		Exp:      time.Now().Add(duration).Unix(),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	unsignedToken := base64URLEncode(headerBytes) + "." + base64URLEncode(payloadBytes)

	h := hmac.New(sha256.New, jwtSecret)
	h.Write([]byte(unsignedToken))
	signature := base64URLEncode(h.Sum(nil))

	return unsignedToken + "." + signature, nil
}

// VerifyToken decodes and validates the signature and expiration of a token
func VerifyToken(tokenStr string) (*JWTPayload, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed authentication token")
	}

	unsignedToken := parts[0] + "." + parts[1]
	signature, err := base64URLDecode(parts[2])
	if err != nil {
		return nil, errors.New("invalid signature encoding")
	}

	// Recompute signature to verify integrity
	h := hmac.New(sha256.New, jwtSecret)
	h.Write([]byte(unsignedToken))
	expectedSignature := h.Sum(nil)

	if !hmac.Equal(signature, expectedSignature) {
		return nil, errors.New("cryptographic signature mismatch")
	}

	// Decode payload
	payloadBytes, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, errors.New("invalid payload encoding")
	}

	var payload JWTPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, errors.New("corrupted payload data")
	}

	// Verify expiration
	if time.Now().Unix() > payload.Exp {
		return nil, errors.New("token has expired")
	}

	return &payload, nil
}

// RequireRole defines an HTTP middleware that filters based on tenant capabilities
func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := ""

			// 1. Check Bearer Authorization Header
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			}

			// 2. Fallback to Cookie
			if tokenStr == "" {
				cookie, err := r.Cookie(TokenCookieName)
				if err == nil {
					tokenStr = cookie.Value
				}
			}

			if tokenStr == "" {
				http.Error(w, `{"error":"unauthorized session"}`, http.StatusUnauthorized)
				return
			}

			// Verify token
			payload, err := VerifyToken(tokenStr)
			if err != nil {
				// Expired or invalid
				http.SetCookie(w, &http.Cookie{
					Name:     TokenCookieName,
					Value:    "",
					Path:     "/",
					Expires:  time.Unix(0, 0),
					HttpOnly: true,
				})
				http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusUnauthorized)
				return
			}

			// Audit multi-tenant roles
			authorized := false
			for _, role := range allowedRoles {
				if payload.Role == role {
					authorized = true
					break
				}
			}

			if !authorized {
				http.Error(w, `{"error":"forbidden resource access denied"}`, http.StatusForbidden)
				return
			}

			// Pass active session variables downstream via headers
			r.Header.Set("NeoCP-User", payload.Username)
			r.Header.Set("NeoCP-Role", payload.Role)

			next.ServeHTTP(w, r)
		})
	}
}
