package gateway

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

const (
	proxyAPIKeyHeader = "X-GuardRail-API-Key"
	adminAPIKeyHeader = "X-GuardRail-Admin-Key"
)

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.cfg.Auth.Enabled || isPublicRoute(r) {
			next.ServeHTTP(w, r)
			return
		}
		switch {
		case isProxyRoute(r):
			if !hasValidKey(r, proxyAPIKeyHeader, s.cfg.Auth.ProxyAPIKeys) {
				writeUnauthorized(w)
				return
			}
		case isAdminRoute(r):
			if !hasValidKey(r, adminAPIKeyHeader, s.cfg.Auth.AdminAPIKeys) {
				writeUnauthorized(w)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isPublicRoute(r *http.Request) bool {
	return r.URL.Path == "/healthz"
}

func isProxyRoute(r *http.Request) bool {
	return r.URL.Path == "/v1/chat/completions"
}

func isAdminRoute(r *http.Request) bool {
	return r.URL.Path == "/metrics" || strings.HasPrefix(r.URL.Path, "/v1/admin/")
}

func hasValidKey(r *http.Request, header string, allowed []string) bool {
	for _, presented := range presentedKeys(r, header) {
		for _, key := range allowed {
			if secretEqual(presented, key) {
				return true
			}
		}
	}
	return false
}

func presentedKeys(r *http.Request, header string) []string {
	keys := make([]string, 0, 2)
	if key := strings.TrimSpace(r.Header.Get(header)); key != "" {
		keys = append(keys, key)
	}
	if key := bearerToken(r.Header.Get("Authorization")); key != "" {
		keys = append(keys, key)
	}
	return keys
}

func bearerToken(value string) string {
	value = strings.TrimSpace(value)
	token, ok := strings.CutPrefix(value, "Bearer ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(token)
}

func secretEqual(a string, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	ah := sha256.Sum256([]byte(a))
	bh := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ah[:], bh[:]) == 1
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="GuardRail"`)
	writeError(w, http.StatusUnauthorized, "missing or invalid GuardRail API key")
}
