package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	discoveryinternal "github.com/rcourtman/pulse-go-rewrite/internal/discovery"
	pkgdiscovery "github.com/rcourtman/pulse-go-rewrite/pkg/discovery"
	"github.com/rs/zerolog/log"
)

func (h *ConfigHandlers) handleDiscoverServers(w http.ResponseWriter, r *http.Request) {
	// Support both GET (for cached results) and POST (for manual scan)
	switch r.Method {
	case http.MethodGet:
		// Return cached results from background discovery service
		if discoveryService := h.getMonitor(r.Context()).GetDiscoveryService(); discoveryService != nil {
			result, updated := discoveryService.GetCachedResult()
			h.writeCachedDiscoveryResponse(w, result, updated)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"servers":     []interface{}{},
			"errors":      []string{},
			"environment": nil,
			"cached":      false,
			"updated":     int64(0),
			"age":         float64(0),
		})
		return

	case http.MethodPost:
		// Limit request body to 8KB to prevent memory exhaustion
		r.Body = http.MaxBytesReader(w, r.Body, 8*1024)

		var req struct {
			Subnet   string `json:"subnet"`
			UseCache bool   `json:"use_cache"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.UseCache {
			if discoveryService := h.getMonitor(r.Context()).GetDiscoveryService(); discoveryService != nil {
				result, updated := discoveryService.GetCachedResult()
				h.writeCachedDiscoveryResponse(w, result, updated)
				return
			}
		}

		subnet := strings.TrimSpace(req.Subnet)
		if subnet == "" {
			subnet = "auto"
		}

		log.Info().Str("subnet", subnet).Msg("Starting manual discovery scan")

		scanner, buildErr := discoveryinternal.BuildScanner(h.getConfig(r.Context()).Discovery)
		if buildErr != nil {
			log.Warn().Err(buildErr).Msg("Falling back to default scanner for manual discovery")
			scanner = pkgdiscovery.NewScanner()
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		result, err := scanner.DiscoverServers(ctx, subnet)
		if err != nil {
			log.Error().Err(err).Msg("Discovery failed")
			http.Error(w, fmt.Sprintf("Discovery failed: %v", err), http.StatusInternalServerError)
			return
		}

		if result.Environment != nil {
			log.Info().
				Str("environment", result.Environment.Type).
				Float64("confidence", result.Environment.Confidence).
				Int("phases", len(result.Environment.Phases)).
				Msg("Manual discovery environment summary")
		}

		response := map[string]interface{}{
			"servers":     result.Servers,
			"errors":      result.Errors,
			"environment": result.Environment,
			"cached":      false,
			"scanning":    false,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ConfigHandlers) writeCachedDiscoveryResponse(w http.ResponseWriter, result *pkgdiscovery.DiscoveryResult, updated time.Time) {
	var updatedUnix int64
	var ageSeconds float64
	if !updated.IsZero() {
		updatedUnix = updated.Unix()
		ageSeconds = time.Since(updated).Seconds()
	}

	response := map[string]interface{}{
		"servers":     result.Servers,
		"errors":      result.Errors,
		"environment": result.Environment,
		"cached":      true,
		"updated":     updatedUnix,
		"age":         ageSeconds,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
