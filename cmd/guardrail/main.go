package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/wjbbeyond/guardrail/internal/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := cmd.Execute(ctx, os.Args[1:]); err != nil {
		slog.Error("guardrail stopped", slog.Any("err", err))
		os.Exit(1)
	}
}
