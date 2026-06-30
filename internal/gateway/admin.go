package gateway

import (
	"net/http"
	"strconv"

	"github.com/wjbbeyond/guardrail/internal/authn"
)

func (s *Server) costsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		tenantID = authn.DefaultTenantID
	}
	snapshot, err := s.costs.SnapshotTenant(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read cost snapshot")
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) auditHandler(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be an integer")
			return
		}
		limit = parsed
	}
	events, err := s.audit.Recent(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read audit events")
		return
	}
	writeJSON(w, http.StatusOK, events)
}
