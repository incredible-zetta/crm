package mcpserver

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AuthHandler wraps an inner http.Handler, requiring a valid API key.
// Accepts the key via Authorization: Bearer <key> OR X-API-Key: <key>.
// On missing/invalid key, responds 401 with a small JSON body and does NOT call inner.
func AuthHandler(apiKey string, inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiKey == "" {
			unauthorized(w)
			return
		}

		authHeader := r.Header.Get("Authorization")
		var token string
		if len(authHeader) >= 7 && strings.EqualFold(authHeader[:7], "bearer ") {
			token = authHeader[7:]
		} else {
			token = r.Header.Get("X-API-Key")
		}

		if subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) != 1 {
			unauthorized(w)
			return
		}

		inner.ServeHTTP(w, r)
	})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
}
