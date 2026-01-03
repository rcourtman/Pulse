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
			log.Warn().Err(err).Msg("Failed to shut down metrics server cleanly")
		}
	}()

	go func() {
		log.Info().Str("addr", addr).Msg("Metrics endpoint listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Warn().Err(err).Msg("Metrics server stopped unexpectedly")
		}
	}()
}
