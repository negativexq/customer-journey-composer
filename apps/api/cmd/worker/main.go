package main

import (
	"context"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/analytics"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/delivery"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/config"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/observability"
	engine "github.com/negativexq/customer-journey-composer/apps/api/internal/runtime"
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
	log := slog.New(observability.Correlate(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownTrace, err := observability.Init(ctx, "journey-worker", os.Getenv)
	if err != nil {
		return err
	}
	defer func() {
		ending, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTrace(ending)
	}()
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
	shutdownMetrics, err := observability.Serve(ctx, os.Getenv("METRICS_ADDR"), pool, true, log)
	if err != nil {
		return err
	}
	defer shutdownMetrics()
	worker := engine.NewWorker(pool, nil)
	worker.Log = log
	log.Info("durable worker started")
	defer log.Info("durable worker drained")
	sender := delivery.New(pool, nil)
	sender.Log = log
	var group sync.WaitGroup
	group.Go(func() { _ = sender.Run(ctx) })
	group.Go(func() { analytics.Run(ctx, pool, log) })
	hooks := engine.NewWebhookDispatcher(pool, engine.AllowedWebhookHosts(os.Getenv("WEBHOOK_ALLOWED_HOSTS")))
	hooks.Log = log
	group.Go(func() { hooks.Run(ctx) })
	err = worker.Run(ctx)
	group.Wait()
	return err
}
