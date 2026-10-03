package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"finance-tracker/backend/internal/auth"
	"finance-tracker/backend/internal/config"
	"finance-tracker/backend/internal/httpapi"
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
	repository := store.New(db)
	authService := auth.New(repository, cfg.SessionDuration)
	metrics := observability.New()
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.New(repository, authService, log, cfg.FrontendOrigin, cfg.CookieSecure, cfg.CookieSameSite, cfg.MetricsToken, metrics), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		log.Info("api listening", "address", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errorsIsServerClosed(err) {
			log.Error("server stopped", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func errorsIsServerClosed(err error) bool { return err == http.ErrServerClosed }
