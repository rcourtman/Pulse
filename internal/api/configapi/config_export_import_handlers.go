package configapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/apihttp"
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
	if !apihttp.EnsureScope(w, r, config.ScopeSettingsRead) {
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
		deps := h.runtimeDependencies()
		deps.AuditEvent(apicontext.OrgID(r.Context()), "config_exported", deps.AuthUsername(h.getConfig(r.Context()), r), deps.ClientIP(r), r.URL.Path, false,
			"Export failed")
		log.Error().Err(err).Msg("Failed to export configuration")
		http.Error(w, "Failed to export configuration", http.StatusInternalServerError)
		return
	}

	deps := h.runtimeDependencies()
	deps.AuditEvent(apicontext.OrgID(r.Context()), "config_exported", deps.AuthUsername(h.getConfig(r.Context()), r), deps.ClientIP(r), r.URL.Path, true,
		"Configuration exported")

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
	if !apihttp.EnsureScope(w, r, config.ScopeSettingsWrite) {
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

	// Capture the resolved scope once. A non-default request must never use
	// the single-runtime fallback or commit an archive without a scoped apply.
	orgID := apicontext.OrgID(r.Context())
	isTenant := orgID != "" && orgID != "default"
	currentConfig, persistence, _ := h.getContextState(r.Context())
	h.stateMu.RLock()
	mtMonitor := h.mtMonitor
	h.stateMu.RUnlock()
	if currentConfig == nil || persistence == nil || (isTenant && mtMonitor == nil) {
		http.Error(w, "Configuration runtime unavailable", http.StatusServiceUnavailable)
		return
	}

	// A browser or proxy can resend a POST even when application retries are
	// disabled. Admit only one import per workspace through persistence AND
	// runtime apply, and retain committed attempts before writing the response.
	attempt, rejection := h.importRequests.begin(orgID, req, time.Now())
	if rejection != nil {
		http.Error(w, rejection.message, rejection.status)
		return
	}
	retainAttempt := true // Fail closed if an unexpected panic interrupts apply.
	defer func() { h.importRequests.finish(attempt, retainAttempt, time.Now()) }()

	// Capture actor identity before config swap (config replacement changes auth resolution)
	deps := h.runtimeDependencies()
	importUser := deps.AuthUsername(currentConfig, r)

	// Import configuration.
	if err := persistence.ImportConfig(req.Data, req.Passphrase); err != nil {
		// No archive committed: a corrected storage failure or passphrase may
		// be retried. Once persistence succeeds, even failed runtime apply
		// must not replay the replacement over a newer legitimate edit.
		retainAttempt = false
		deps.AuditEvent(apicontext.OrgID(r.Context()), "config_imported", importUser, deps.ClientIP(r), r.URL.Path, false,
			"Import failed")
		log.Error().Err(err).Msg("Failed to import configuration")
		http.Error(w, "Failed to import configuration. Verify the backup file and passphrase are correct.", http.StatusBadRequest)
		return
	}

	if isTenant {
		// Recreate only this tenant. Loading process-global configuration here
		// copied the default org's paths and credentials into the live tenant,
		// and the shared hook restarted unrelated monitoring.
		if _, err := mtMonitor.ReloadTenant(orgID); err != nil {
			deps.AuditEvent(orgID, "config_imported", importUser, deps.ClientIP(r), r.URL.Path, false,
				"Import succeeded but tenant runtime reload failed")
			log.Error().Err(err).Str("org_id", orgID).Msg("Failed to apply imported tenant configuration")
			http.Error(w, "Configuration imported but failed to apply changes", http.StatusInternalServerError)
			return
		}
	} else {
		// The default organization retains its existing instance-wide reload.
		// Reloading must not reinitialize the process logger under live workers.
		newConfig, err := config.LoadWithoutLoggingInit()
		if err != nil {
			deps.AuditEvent(orgID, "config_imported", importUser, deps.ClientIP(r), r.URL.Path, false,
				"Import succeeded but config reload failed")
			log.Error().Err(err).Msg("Failed to reload configuration after import")
			http.Error(w, "Configuration imported but failed to reload", http.StatusInternalServerError)
			return
		}
		config.Mu.Lock()
		*currentConfig = *newConfig
		config.Mu.Unlock()
		if h.reloadFunc != nil {
			if err := h.reloadFunc(); err != nil {
				deps.AuditEvent(orgID, "config_imported", importUser, deps.ClientIP(r), r.URL.Path, false,
					"Import succeeded but monitor reload failed")
				log.Error().Err(err).Msg("Failed to reload monitor after import")
				http.Error(w, "Configuration imported but failed to apply changes", http.StatusInternalServerError)
				return
			}
		}
	}

	// Also reload alert and notification configs explicitly
	// (the monitor reload only reloads nodes unless it's a full reload).
	// Persistence has already committed. Continue independent reloads, but do
	// not turn a partial live apply into a successful import acknowledgement.
	// Only fixed component names may reach the response or audit detail.
	var reloadFailures []string
	if monitor := h.getMonitor(r.Context()); monitor != nil {
		// Reload alert configuration.
		if alertConfig, err := h.getPersistence(r.Context()).LoadAlertConfig(); err == nil {
			if alertManager := monitor.GetAlertManager(); alertManager == nil {
				reloadFailures = append(reloadFailures, "alert settings")
				log.Warn().Msg("Skipped alert reload after import because alert manager is unavailable")
			} else {
				alertManager.UpdateConfig(*alertConfig)
				log.Info().Msg("Reloaded alert configuration after import")
			}
		} else {
			reloadFailures = append(reloadFailures, "alert settings")
			log.Warn().Err(err).Msg("Failed to reload alert configuration after import")
		}

		notificationMgr := monitor.GetNotificationManager()
		// Reload webhook configuration.
		if webhooks, err := h.getPersistence(r.Context()).LoadWebhooks(); err == nil {
			if notificationMgr == nil {
				reloadFailures = append(reloadFailures, "webhook settings")
				log.Warn().Msg("Skipped webhook reload after import because notification manager is unavailable")
			} else {
				// Clear existing webhooks and add new ones.
				webhooksApplied := true
				for _, webhook := range notificationMgr.GetWebhooks() {
					if err := notificationMgr.DeleteWebhook(webhook.ID); err != nil {
						webhooksApplied = false
						log.Warn().Err(err).Str("webhook", webhook.ID).Msg("Failed to delete existing webhook during reload")
					}
				}
				// Add imported webhooks.
				for _, webhook := range webhooks {
					notificationMgr.AddWebhook(webhook)
				}
				if webhooksApplied {
					log.Info().Int("count", len(webhooks)).Msg("Reloaded webhook configuration after import")
				} else {
					reloadFailures = append(reloadFailures, "webhook settings")
				}
			}
		} else {
			reloadFailures = append(reloadFailures, "webhook settings")
			log.Warn().Err(err).Msg("Failed to reload webhook configuration after import")
		}

		// Reload email configuration.
		if emailConfig, err := h.getPersistence(r.Context()).LoadEmailConfig(); err == nil {
			if notificationMgr == nil {
				reloadFailures = append(reloadFailures, "email settings")
				log.Warn().Msg("Skipped email reload after import because notification manager is unavailable")
			} else {
				notificationMgr.SetEmailConfig(*emailConfig)
				log.Info().Msg("Reloaded email configuration after import")
			}
		} else {
			reloadFailures = append(reloadFailures, "email settings")
			log.Warn().Err(err).Msg("Failed to reload email configuration after import")
		}
	}

	// Reload guest metadata from disk.
	if h.guestMetadataHandler != nil {
		if err := h.guestMetadataHandler.Reload(r.Context()); err != nil {
			reloadFailures = append(reloadFailures, "guest metadata")
			log.Warn().Err(err).Msg("Failed to reload guest metadata after import")
		} else {
			log.Info().Msg("Reloaded guest metadata after import")
		}
	}

	if len(reloadFailures) != 0 {
		failed := strings.Join(reloadFailures, ", ")
		deps.AuditEvent(orgID, "config_imported", importUser, deps.ClientIP(r), r.URL.Path, false,
			"Configuration saved but live reload incomplete: "+failed)
		http.Error(w, "Configuration saved, but live reload failed: "+failed+". Check the destination before relying on it. Do not repeat the import.", http.StatusInternalServerError)
		return
	}

	deps.AuditEvent(apicontext.OrgID(r.Context()), "config_imported", importUser, deps.ClientIP(r), r.URL.Path, true,
		"Configuration imported")

	log.Info().Msg("Configuration imported successfully")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Configuration imported successfully",
		"remoteAgentEndpoints": map[string]bool{
			"reconfigured":                         false,
			"actionRequiredIfServerAddressChanged": true,
		},
	})
}
