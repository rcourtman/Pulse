package api

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
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
		if _, err := fmt.Fprintf(w, "data: %s\n\n", line); err != nil {
			return
		}
	}
	flusher.Flush()

	notify := r.Context().Done()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
				return
			}
			flusher.Flush()
		case <-notify:
			return
		}
	}
}

// HandleDownloadBundle creates a zip file with system logs and sanitized config
func (h *LogHandlers) HandleDownloadBundle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"pulse-support-bundle-%s.zip\"", time.Now().Format("20060102-150405")))

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// 1. Add Logs
	// If a log file is configured and exists, add it.
	// Otherwise (or in addition), dump the in-memory buffer.
	addedLogFile := false
	if h.config.LogFile != "" {
		if f, err := os.Open(h.config.LogFile); err == nil {
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
		// Create a sanitized copy of config
		sanitizedConfig := h.config.DeepCopy()

		// Scrub top-level secrets
		if sanitizedConfig.AuthPass != "" {
			sanitizedConfig.AuthPass = "[REDACTED]"
		}
		if sanitizedConfig.APIToken != "" {
			sanitizedConfig.APIToken = "[REDACTED]"
		}
		if sanitizedConfig.ProxyAuthSecret != "" {
			sanitizedConfig.ProxyAuthSecret = "[REDACTED]"
		}

		// Scrub array secrets
		for i := range sanitizedConfig.PVEInstances {
			sanitizedConfig.PVEInstances[i].Password = "[REDACTED]"
			sanitizedConfig.PVEInstances[i].TokenValue = "[REDACTED]"
		}
		for i := range sanitizedConfig.PBSInstances {
			sanitizedConfig.PBSInstances[i].Password = "[REDACTED]"
			sanitizedConfig.PBSInstances[i].TokenValue = "[REDACTED]"
		}
		for i := range sanitizedConfig.PMGInstances {
			sanitizedConfig.PMGInstances[i].Password = "[REDACTED]"
			sanitizedConfig.PMGInstances[i].TokenValue = "[REDACTED]"
		}

		// Sanitize environment variables
		rawEnv := os.Environ()
		sanitizedEnv := make([]string, 0, len(rawEnv))
		for _, e := range rawEnv {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				upperKey := strings.ToUpper(key)
				// Redact known sensitive keys
				if strings.Contains(upperKey, "TOKEN") ||
					strings.Contains(upperKey, "PASS") ||
					strings.Contains(upperKey, "SECRET") ||
					strings.Contains(upperKey, "KEY") {
					sanitizedEnv = append(sanitizedEnv, fmt.Sprintf("%s=[REDACTED]", key))
				} else {
					sanitizedEnv = append(sanitizedEnv, e)
				}
			}
		}

		safeConfig := struct {
			Version   string
			GoVersion string
			OS        string
			Config    *config.Config
			Env       []string
		}{
			Version:   "5.x",
			GoVersion: "go1.21+",
			OS:        "linux",
			Config:    sanitizedConfig,
			Env:       sanitizedEnv,
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
	h.config.LogLevel = level
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
