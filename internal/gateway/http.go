package gateway

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/wjbbeyond/guardrail/internal/security"
)

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("write json response", slog.Any("err", err))
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func copyHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		if isHopByHop(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func isHopByHop(header string) bool {
	switch http.CanonicalHeaderKey(header) {
	case "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade":
		return true
	default:
		return false
	}
}

func securityHeader(decision security.Decision) string {
	if len(decision.Findings) == 0 {
		return string(security.ActionAllow)
	}
	return string(decision.Action) + "; findings=" + strconv.Itoa(len(decision.Findings))
}
