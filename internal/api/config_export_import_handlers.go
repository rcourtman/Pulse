package api

import (
	"encoding/json"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// ExportConfigRequest represents a request to export configuration
//
// This request type is shared with package-local tests.
type ExportConfigRequest struct {
	Passphrase string `json:"passphrase"`
}

// ImportConfigRequest represents a request to import configuration
//
// This request type is shared with package-local tests.
type ImportConfigRequest struct {
	Data       string `json:"data"`
	Passphrase string `json:"passphrase"`
}

func (h *ConfigHandlers) handleExportConfig(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 8KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)

	// SECURITY: Validating scope for config export
	if !ensureScope(w, r, config.ScopeSettingsRead) {
		return
	}

	var req ExportConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode export request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Passphrase == "" {
		log.Warn().Msg("Export rejected: passphrase is required")
		http.Error(w, "Passphrase is required", http.StatusBadRequest)
		return
	}

	// Require strong passphrase (at least 12 characters)
	if len(req.Passphrase) < 12 {
		log.Warn().Int("length", len(req.Passphrase)).Msg("Export rejected: passphrase too short (minimum 12 characters)")
		http.Error(w, "Passphrase must be at least 12 characters long", http.StatusBadRequest)
		return
	}

	// Export configuration
	exportedData, err := h.getPersistence(r.Context()).ExportConfig(req.Passphrase)
	if err != nil {
		log.Error().Err(err).Msg("Failed to export configuration")
		http.Error(w, "Failed to export configuration", http.StatusInternalServerError)
		return
	}

	log.Info().Msg("Configuration exported successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   exportedData,
	})
}

func (h *ConfigHandlers) handleImportConfig(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 1MB to prevent memory exhaustion (config imports can be large)
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	// SECURITY: Validating scope for config import
	if !ensureScope(w, r, config.ScopeSettingsWrite) {
		return
	}

	var req ImportConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode import request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Passphrase == "" {
		log.Warn().Msg("Import rejected: passphrase is required")
		http.Error(w, "Passphrase is required", http.StatusBadRequest)
		return
	}

	if req.Data == "" {
		log.Warn().Msg("Import rejected: encrypted data is required (ensure backup file has 'data' field)")
		http.Error(w, "Import data is required", http.StatusBadRequest)
		return
	}

	// Import configuration.
	if err := h.getPersistence(r.Context()).ImportConfig(req.Data, req.Passphrase); err != nil {
		log.Error().Err(err).Msg("Failed to import configuration")
		http.Error(w, "Failed to import configuration: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Reload configuration from disk.
	newConfig, err := config.Load()
	if err != nil {
		log.Error().Err(err).Msg("Failed to reload configuration after import")
		http.Error(w, "Configuration imported but failed to reload", http.StatusInternalServerError)
		return
	}

	// Update the config reference.
	*h.getConfig(r.Context()) = *newConfig

	// Reload monitor with new configuration.
	if h.reloadFunc != nil {
		if err := h.reloadFunc(); err != nil {
			log.Error().Err(err).Msg("Failed to reload monitor after import")
			http.Error(w, "Configuration imported but failed to apply changes", http.StatusInternalServerError)
			return
		}
	}

	// Also reload alert and notification configs explicitly
	// (the monitor reload only reloads nodes unless it's a full reload).
	if h.getMonitor(r.Context()) != nil {
		// Reload alert configuration.
		if alertConfig, err := h.getPersistence(r.Context()).LoadAlertConfig(); err == nil {
			h.getMonitor(r.Context()).GetAlertManager().UpdateConfig(*alertConfig)
			log.Info().Msg("Reloaded alert configuration after import")
		} else {
			log.Warn().Err(err).Msg("Failed to reload alert configuration after import")
		}

		// Reload webhook configuration.
		if webhooks, err := h.getPersistence(r.Context()).LoadWebhooks(); err == nil {
			// Clear existing webhooks and add new ones.
			notificationMgr := h.getMonitor(r.Context()).GetNotificationManager()
			// Get current webhooks to clear them.
			for _, webhook := range notificationMgr.GetWebhooks() {
				if err := notificationMgr.DeleteWebhook(webhook.ID); err != nil {
					log.Warn().Err(err).Str("webhook", webhook.ID).Msg("Failed to delete existing webhook during reload")
				}
			}
			// Add imported webhooks.
			for _, webhook := range webhooks {
				notificationMgr.AddWebhook(webhook)
			}
			log.Info().Int("count", len(webhooks)).Msg("Reloaded webhook configuration after import")
		} else {
			log.Warn().Err(err).Msg("Failed to reload webhook configuration after import")
		}

		// Reload email configuration.
		if emailConfig, err := h.getPersistence(r.Context()).LoadEmailConfig(); err == nil {
			h.getMonitor(r.Context()).GetNotificationManager().SetEmailConfig(*emailConfig)
			log.Info().Msg("Reloaded email configuration after import")
		} else {
			log.Warn().Err(err).Msg("Failed to reload email configuration after import")
		}
	}

	// Reload guest metadata from disk.
	if h.guestMetadataHandler != nil {
		if err := h.guestMetadataHandler.Reload(); err != nil {
			log.Warn().Err(err).Msg("Failed to reload guest metadata after import")
		} else {
			log.Info().Msg("Reloaded guest metadata after import")
		}
	}

	log.Info().Msg("Configuration imported successfully")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Configuration imported successfully",
	})
}
