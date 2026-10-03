package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"finance-tracker/backend/internal/config"
	"finance-tracker/backend/internal/observability"
	"finance-tracker/backend/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database config failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err = db.Ping(ctx); err != nil {
		log.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	poll := pollInterval()
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	repository := store.New(db)
	metrics := observability.New()
	metricsServer := &http.Server{Addr: workerMetricsAddr(), Handler: workerMetricsAuth(metrics.Handler(func() int64 { return repository.CountActiveSessions(context.Background(), time.Now().UTC()) })), ReadHeaderTimeout: 3 * time.Second}
	go func() {
		log.Info("worker metrics listening", "address", metricsServer.Addr)
		if serveErr := metricsServer.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			log.Error("worker metrics stopped", "error", serveErr)
			stop()
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = metricsServer.Shutdown(shutdownCtx)
	}()
	log.Info("recurring worker started", "poll_interval", poll.String())
	run := func() {
		now := time.Now().UTC()
		if err := repository.DeleteExpiredSessions(ctx, now); err != nil {
			log.Error("expired session cleanup failed", "error", err)
		}
		result, err := repository.ProcessDueRecurrences(ctx, now, 100)
		if err != nil {
			metrics.WorkerFailure()
			log.Error("recurring batch failed", "error", err)
			return
		}
		metrics.Recurring("success", result.Generated)
		metrics.Recurring("skipped", result.Duplicates+result.Paused)
		if result.Generated > 0 || result.Duplicates > 0 || result.Paused > 0 {
			log.Info("recurring batch complete", "generated", result.Generated, "duplicates", result.Duplicates, "paused", result.Paused)
		}
	}
	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
func workerMetricsAddr() string {
	if value := os.Getenv("WORKER_METRICS_ADDR"); value != "" {
		return value
	}
	return ":9091"
}
func workerMetricsAuth(next http.Handler) http.Handler {
	token := os.Getenv("METRICS_TOKEN")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" || r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func pollInterval() time.Duration {
	raw := os.Getenv("WORKER_POLL_SECONDS")
	if raw == "" {
		return 30 * time.Second
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 1 {
		return 30 * time.Second
	}
	return time.Duration(seconds) * time.Second
}
