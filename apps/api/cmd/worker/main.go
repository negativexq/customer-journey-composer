package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ofk/customer-journey-composer/apps/api/internal/config"
	"github.com/ofk/customer-journey-composer/apps/api/internal/database"
	engine "github.com/ofk/customer-journey-composer/apps/api/internal/runtime"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	pool, err := database.Open(startup, cfg.DatabaseURL)
	if err == nil {
		err = database.Ready(startup, pool)
	}
	cancel()
	if err != nil {
		if pool != nil {
			pool.Close()
		}
		return err
	}
	defer pool.Close()
	worker := engine.NewWorker(pool, nil)
	worker.Log = log
	log.Info("durable worker started")
	defer log.Info("durable worker drained")
	return worker.Run(ctx)
}
