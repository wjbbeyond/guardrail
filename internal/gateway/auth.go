package gateway

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/wjbbeyond/guardrail/internal/authn"
)

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.cfg.Auth.Enabled || isPublicRoute(r) {
			next.ServeHTTP(w, r)
			return
		}
		switch {
		case isProxyRoute(r):
			identity, ok, err := s.auth.AuthenticateProxy(r.Context(), r)
			if err != nil {
				s.logger.ErrorContext(r.Context(), "authenticate proxy request", slog.Any("err", err))
				writeUnauthorized(w)
				return
			}
			if !ok {
				writeUnauthorized(w)
				return
			}
			r = r.WithContext(authn.WithIdentity(r.Context(), identity))
		case isAdminRoute(r):
			identity, ok, err := s.auth.AuthenticateAdmin(r.Context(), r)
			if err != nil {
				s.logger.ErrorContext(r.Context(), "authenticate admin request", slog.Any("err", err))
				writeUnauthorized(w)
				return
			}
			if !ok {
				writeUnauthorized(w)
				return
			}
			r = r.WithContext(authn.WithIdentity(r.Context(), identity))
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

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="GuardRail"`)
	writeError(w, http.StatusUnauthorized, "missing or invalid GuardRail API key")
}
