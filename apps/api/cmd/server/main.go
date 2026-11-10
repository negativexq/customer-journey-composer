package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ofk/customer-journey-composer/apps/api/internal/config"
	"github.com/ofk/customer-journey-composer/apps/api/internal/database"
	"github.com/ofk/customer-journey-composer/apps/api/internal/demo"
	"github.com/ofk/customer-journey-composer/apps/api/internal/httpapi"
	"github.com/ofk/customer-journey-composer/apps/api/internal/observability"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
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
	shutdownTrace, err := observability.Init(ctx, "journey-api", os.Getenv)
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
	cancel()
	if err != nil {
		return err
	}
	defer pool.Close()
	shutdownMetrics, err := observability.Serve(ctx, os.Getenv("METRICS_ADDR"), pool, false, log)
	if err != nil {
		return err
	}
	defer shutdownMetrics()
	seedCtx, seedCancel := context.WithTimeout(ctx, 10*time.Second)
	err = demo.Ensure(seedCtx, pool)
	seedCancel()
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.New(log, func(ctx context.Context) error {
			return database.Ready(ctx, pool)
		}, pool),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	errorsCh := make(chan error, 1)
	go func() {
		log.Info("server listening", "address", cfg.HTTPAddr)
		errorsCh <- server.ListenAndServe()
	}()
	select {
	case err := <-errorsCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	case <-ctx.Done():
		log.Info("draining requests")
	}
	shutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdown); err != nil {
		_ = server.Close()
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}
