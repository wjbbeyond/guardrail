package cmd

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/wjbbeyond/guardrail/internal/audit"
	"github.com/wjbbeyond/guardrail/internal/authn"
	"github.com/wjbbeyond/guardrail/internal/config"
	"github.com/wjbbeyond/guardrail/internal/cost"
	"github.com/wjbbeyond/guardrail/internal/gateway"
	"github.com/wjbbeyond/guardrail/internal/metrics"
	"github.com/wjbbeyond/guardrail/internal/provider"
	"github.com/wjbbeyond/guardrail/internal/ratelimit"
	"github.com/wjbbeyond/guardrail/internal/security"
)

func Execute(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("guardrail", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "path to GuardRail YAML config")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := newLogger(cfg.Server.LogLevel)
	slog.SetDefault(logger)

	auditor, err := audit.Open(ctx, cfg.Audit.SQLiteDSN)
	if err != nil {
		return fmt.Errorf("open audit store: %w", err)
	}
	defer auditor.Close()
	costLedger, err := cost.OpenSQLiteLedger(ctx, cfg.Audit.SQLiteDSN)
	if err != nil {
		return fmt.Errorf("open cost ledger: %w", err)
	}
	defer costLedger.Close()
	authManager, err := authn.NewManager(ctx, cfg)
	if err != nil {
		return fmt.Errorf("build auth manager: %w", err)
	}
	priceTable := cost.NewPriceTable(cfg.Pricing)
	if err := priceTable.Refresh(ctx); err != nil {
		logger.WarnContext(ctx, "refresh pricing table", slog.Any("err", err))
	}
	go priceTable.RunAutoRefresh(ctx, cfg.Pricing.RefreshInterval, func(err error) {
		logger.WarnContext(ctx, "refresh pricing table", slog.Any("err", err))
	})

	router, err := provider.NewRouter(cfg.Providers, cfg.Reliability.ProviderTimeout)
	if err != nil {
		return fmt.Errorf("build provider router: %w", err)
	}

	server := gateway.New(gateway.Dependencies{
		Config: cfg,
		Auth:   authManager,
		Router: router,
		Guard:  security.NewGuard(cfg.Security),
		Costs: cost.NewTrackerWithOptions(cost.TrackerOptions{
			Cost:    cfg.Cost,
			Tenants: cfg.Tenants,
			Clock:   cost.RealClock{},
			Ledger:  costLedger,
			Pricer:  priceTable,
		}),
		Limits:  ratelimit.New(cfg.RateLimit, cfg.Tenants, ratelimit.RealClock{}),
		Audit:   auditor,
		Metrics: metrics.NewRegistry(),
		Logger:  logger,
	})
	if err := server.Run(ctx); err != nil {
		return fmt.Errorf("run server: %w", err)
	}
	return nil
}

func newLogger(level string) *slog.Logger {
	var slogLevel slog.Level
	if err := slogLevel.UnmarshalText([]byte(level)); err != nil {
		slogLevel = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel}))
}
