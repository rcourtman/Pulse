package api

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rs/zerolog/log"
)

type LogHandlers struct {
	config      *config.Config
	persistence *config.ConfigPersistence
}

func NewLogHandlers(cfg *config.Config, persistence *config.ConfigPersistence) *LogHandlers {
	return &LogHandlers{
		config:      cfg,
		persistence: persistence,
	}
}

// HandleStreamLogs streams logs using SSE (Server-Sent Events)
func (h *LogHandlers) HandleStreamLogs(w http.ResponseWriter, r *http.Request) {
	revalidate := tokenStreamSnapshotRevalidator(h.config, r, config.ScopeSettingsRead)
	allowed := func() bool {
		if revalidate == nil {
			// Existing browser-session/proxy admission remains with the route.
			return true
		}
		current, ok := revalidate()
		return ok && tokenActsForInstance(current)
	}
	if !allowed() {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	broadcaster := logging.GetBroadcaster()
	subscriptionID, ch, history := broadcaster.Subscribe()
	defer broadcaster.Unsubscribe(subscriptionID)

	// Send history first
	for _, line := range history {
		if !allowed() {
			return
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", line); err != nil {
			return
		}
	}
	flusher.Flush()

	notify := r.Context().Done()
	// End an idle withdrawn token reader too, without generating log entries,
	// writing keepalive frames or changing session/proxy stream behaviour.
	var recheck <-chan time.Time
	if revalidate != nil {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		recheck = ticker.C
	}

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if !allowed() {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
				return
			}
			flusher.Flush()
		case <-notify:
			return
		case <-recheck:
			if !allowed() {
				return
			}
		}
	}
}

// HandleDownloadBundle creates a private log archive with an allowlisted diagnostic summary.
// Log contents are not scrubbed; this is not an anonymised export or configuration backup.
func (h *LogHandlers) HandleDownloadBundle(w http.ResponseWriter, r *http.Request) {
	// Snapshot only diagnostic fields while holding configuration ownership.
	// Do not serialize Config or the full process environment into this archive.
	config.Mu.RLock()
	logFile := h.config.LogFile
	summary := newSupportBundleConfig(h.config)
	config.Mu.RUnlock()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"pulse-support-bundle-%s.zip\"", time.Now().Format("20060102-150405")))

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// 1. Add Logs
	// If a log file is configured and exists, add it.
	// Otherwise (or in addition), dump the in-memory buffer.
	addedLogFile := false
	if logFile != "" {
		if f, err := os.Open(logFile); err == nil {
			defer f.Close()
			if wr, err := zipWriter.Create("pulse.log"); err == nil {
				if _, err := io.Copy(wr, f); err != nil {
					log.Warn().Err(err).Msg("failed to copy log file to zip")
				}
				addedLogFile = true
			}
		}
	}

	if !addedLogFile {
		// Dump memory buffer
		if wr, err := zipWriter.Create("pulse-tail.log"); err == nil {
			for _, line := range logging.GetBroadcaster().GetHistory() {
				fmt.Fprintln(wr, line)
			}
		}
	}

	// 2. Add Sanitized System Info
	if wr, err := zipWriter.Create("system-info.json"); err == nil {
		version := strings.TrimSpace(updates.BuildVersion)
		if version == "" {
			version = "unknown"
		}
		safeConfig := struct {
			Version   string
			GoVersion string
			OS        string
			Config    supportBundleConfig
			Env       []string
		}{
			Version:   version,
			GoVersion: runtime.Version(),
			OS:        runtime.GOOS,
			Config:    summary,
			Env:       supportBundleEnvironment(os.Environ()),
		}

		enc := json.NewEncoder(wr)
		enc.SetIndent("", "  ")
		enc.Encode(safeConfig)
	}
}

type SetLogLevelRequest struct {
	Level string `json:"level"`
}

// HandleSetLevel changes the log level at runtime and persists it
func (h *LogHandlers) HandleSetLevel(w http.ResponseWriter, r *http.Request) {
	var req SetLogLevelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	level := strings.ToLower(req.Level)
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[level] {
		http.Error(w, "Invalid log level", http.StatusBadRequest)
		return
	}

	// 1. Update Runtime
	logging.SetGlobalLevel(level)
	config.Mu.Lock()
	h.config.LogLevel = level
	config.Mu.Unlock()
	log.Info().Str("level", level).Msg("Log level updated via API")

	// 2. Persist to system.json
	if h.persistence != nil {
		settings, err := h.persistence.LoadSystemSettings()
		if err == nil {
			settings.LogLevel = level
			if err := h.persistence.SaveSystemSettings(*settings); err != nil {
				log.Error().Err(err).Msg("Failed to persist log level change")
				// Don't fail the request, runtime update succeeded
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// HandleGetLevel returns the current log level
func (h *LogHandlers) HandleGetLevel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"level": logging.GetGlobalLevel(),
	})
}
