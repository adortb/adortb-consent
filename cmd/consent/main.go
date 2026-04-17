// Package main 是 adortb-consent GDPR/CCPA 合规服务入口。
// 端口：8089（业务 API）、9101（Prometheus metrics）
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

	"github.com/adortb/adortb-consent/internal/api"
	"github.com/adortb/adortb-consent/internal/gvl"
	"github.com/adortb/adortb-consent/internal/metrics"
	"github.com/adortb/adortb-consent/internal/store"
)

const (
	defaultAPIPort     = "8089"
	defaultMetricsPort = "9101"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("adortb-consent starting")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 存储初始化（生产：PostgreSQL；开发：内存）
	var consentStore store.ConsentStore
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		pgStore, err := store.NewPGStore(dsn)
		if err != nil {
			logger.Error("database init failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		if err := pgStore.Ping(ctx); err != nil {
			logger.Error("database ping failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		consentStore = pgStore
		logger.Info("connected to PostgreSQL")
	} else {
		logger.Warn("DATABASE_URL not set, using in-memory store (data not persisted)")
		consentStore = store.NewMemoryStore()
	}

	// GVL 客户端（每 24h 刷新）
	gvlClient := gvl.NewClient(logger, 24*time.Hour)
	gvlClient.Start(ctx)
	defer gvlClient.Stop()

	// Prometheus metrics
	m := metrics.New()

	// HTTP API
	mux := http.NewServeMux()
	handler := api.NewHandler(consentStore, gvlClient, logger)
	handler.Register(mux)

	apiPort := envOr("PORT", defaultAPIPort)
	srv := &http.Server{
		Addr:         ":" + apiPort,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Metrics server
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", m.Handler())
	metricsPort := envOr("METRICS_PORT", defaultMetricsPort)
	metricsSrv := &http.Server{
		Addr:    ":" + metricsPort,
		Handler: metricsMux,
	}

	go func() {
		logger.Info("metrics server listening", slog.String("addr", ":"+metricsPort))
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("metrics server error", slog.String("error", err.Error()))
		}
	}()

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("API server listening", slog.String("addr", ":"+apiPort))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		logger.Error("server error", slog.String("error", err.Error()))
		os.Exit(1)
	case sig := <-quit:
		logger.Info("shutting down", slog.String("signal", sig.String()))
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", slog.String("error", err.Error()))
	}
	_ = metricsSrv.Shutdown(shutdownCtx)

	logger.Info("adortb-consent stopped")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// suppress unused import warning for fmt
var _ = fmt.Sprintf
