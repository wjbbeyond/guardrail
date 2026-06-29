package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/wjbbeyond/guardrail/internal/audit"
	"github.com/wjbbeyond/guardrail/internal/config"
	"github.com/wjbbeyond/guardrail/internal/cost"
	"github.com/wjbbeyond/guardrail/internal/metrics"
	"github.com/wjbbeyond/guardrail/internal/provider"
	"github.com/wjbbeyond/guardrail/internal/security"
)

type Dependencies struct {
	Config  config.Config
	Router  *provider.Router
	Guard   *security.Guard
	Costs   *cost.Tracker
	Audit   *audit.Store
	Metrics *metrics.Registry
	Logger  *slog.Logger
}

type Server struct {
	cfg     config.Config
	router  *provider.Router
	guard   *security.Guard
	costs   *cost.Tracker
	audit   *audit.Store
	metrics *metrics.Registry
	logger  *slog.Logger
	handler http.Handler
}

func New(deps Dependencies) *Server {
	server := &Server{
		cfg:     deps.Config,
		router:  deps.Router,
		guard:   deps.Guard,
		costs:   deps.Costs,
		audit:   deps.Audit,
		metrics: deps.Metrics,
		logger:  deps.Logger,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.healthz)
	mux.HandleFunc("GET /metrics", server.metricsHandler)
	mux.HandleFunc("GET /v1/admin/costs", server.costsHandler)
	mux.HandleFunc("GET /v1/admin/audit", server.auditHandler)
	mux.HandleFunc("POST /v1/chat/completions", server.chatCompletions)
	server.handler = withRequestID(mux)
	return server
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) Run(ctx context.Context) error {
	httpServer := &http.Server{
		Addr:         s.cfg.Server.ListenAddr,
		Handler:      s.handler,
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		WriteTimeout: s.cfg.Server.WriteTimeout,
	}
	errCh := make(chan error, 1)
	go func() {
		s.logger.InfoContext(ctx, "guardrail listening", slog.String("addr", s.cfg.Server.ListenAddr))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.Server.ShutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}
		return nil
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
		return nil
	}
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

func (s *Server) metricsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, s.metrics.Prometheus())
}

type healthResponse struct {
	Status string `json:"status"`
}
