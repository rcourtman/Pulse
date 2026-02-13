package main

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

var (
	metricsShutdownTimeout = 5 * time.Second
)

func startMetricsServer(ctx context.Context, addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), metricsShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && err != http.ErrServerClosed {
			log.Warn().
				Err(err).
				Str("component", "metrics_server").
				Str("action", "shutdown_failed").
				Str("addr", addr).
				Msg("Failed to shut down metrics server cleanly")
		}
	}()

	go func() {
		log.Info().
			Str("component", "metrics_server").
			Str("action", "listening").
			Str("addr", addr).
			Msg("Metrics endpoint listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Warn().
				Err(err).
				Str("component", "metrics_server").
				Str("action", "stopped_unexpectedly").
				Str("addr", addr).
				Msg("Metrics server stopped unexpectedly")
		}
	}()
}
