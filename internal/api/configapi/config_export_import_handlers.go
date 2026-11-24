package configapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/apihttp"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
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

	// An archive's API tokens and SSO configuration are credentials the
	// importer issues. The transfer boundary marks a caller without the
	// authority to issue them; that import restores every other section and
	// keeps the destination's own.
	keepAccessCredentials := apicontext.ImportKeepsAccessCredentials(r.Context())

	// Import configuration. An organization administrator's archive is held
	// to that administrator's limits before any of it is staged.
	if err := persistence.ImportConfigWithOptions(req.Data, req.Passphrase, config.ImportOptions{
		KeepAccessCredentials: keepAccessCredentials,
		Validate: func(archive *config.ExportData) error {
			return checkTenantImport(r.Context(), archive, persistence)
		},
	}); err != nil {
		// No archive committed: a corrected storage failure or passphrase may
		// be retried. Once persistence succeeds, even failed runtime apply
		// must not replay the replacement over a newer legitimate edit.
		retainAttempt = false
		var refusal *importRefusal
		if errors.As(err, &refusal) {
			deps.AuditEvent(orgID, "config_imported", importUser, deps.ClientIP(r), r.URL.Path, false,
				"Import refused: backup exceeds organization administrator limits")
			log.Warn().Err(err).Str("org_id", orgID).Msg("Refused configuration import")
			http.Error(w, refusal.message, http.StatusForbidden)
			return
		}
		deps.AuditEvent(apicontext.OrgID(r.Context()), "config_imported", importUser, deps.ClientIP(r), r.URL.Path, false,
			"Import failed")
		log.Error().Err(err).Msg("Failed to import configuration")
		http.Error(w, "Failed to import configuration. Verify the backup file and passphrase are correct.", http.StatusBadRequest)
		return
	}

	// Sign-in is served from the SSO configuration the server has published,
	// never from the file the import replaced. Publish the restored section
	// before any reload below can return early: a provider the archive removed
	// or changed must stop admitting sign-ins under its previous policy now,
	// and the next SSO settings save must not write it back over the archive.
	// The SSO configuration is the instance's; a tenant import or an import
	// that kept access credentials does not touch it.
	var ssoReloadErr error
	if !isTenant && !keepAccessCredentials {
		h.stateMu.RLock()
		reloadSSOConfig := h.ssoConfigReload
		h.stateMu.RUnlock()
		if reloadSSOConfig != nil {
			if ssoReloadErr = reloadSSOConfig(); ssoReloadErr != nil {
				log.Warn().Err(ssoReloadErr).Msg("Failed to reload SSO configuration after import")
			} else {
				log.Info().Msg("Reloaded SSO configuration after import")
			}
		}
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
		// Read and publish in one config.Mu section, as the config watcher does
		// for API tokens: a copy read before the lock could replace a token
		// inventory that was changed and saved in between.
		config.Mu.Lock()
		newConfig, err := config.LoadWithoutLoggingInit()
		if err == nil {
			*currentConfig = *newConfig
		}
		config.Mu.Unlock()
		if err != nil {
			deps.AuditEvent(orgID, "config_imported", importUser, deps.ClientIP(r), r.URL.Path, false,
				"Import succeeded but config reload failed")
			log.Error().Err(err).Msg("Failed to reload configuration after import")
			http.Error(w, "Configuration imported but failed to reload", http.StatusInternalServerError)
			return
		}
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
	if ssoReloadErr != nil {
		reloadFailures = append(reloadFailures, "SSO settings")
	}
	// Apply the normal non-notification reloads even while delivery is held.
	// Otherwise a later edit can save stale live state over the imported archive.
	// A full owner reconstruction may already have cleared the original hold.
	var notificationHoldErr error
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
		if notificationMgr != nil && notificationMgr.ConfigurationError() != nil {
			notificationHoldErr = notificationMgr.ConfigurationError()
		} else {
			// Reload webhook configuration.
			if webhooks, err := h.getPersistence(r.Context()).LoadWebhooks(); err == nil {
				if notificationMgr == nil {
					reloadFailures = append(reloadFailures, "webhook settings")
					log.Warn().Msg("Skipped webhook reload after import because notification manager is unavailable")
				} else {
					if err := notificationMgr.ReplaceWebhooks(webhooks); err != nil {
						reloadFailures = append(reloadFailures, "webhook settings")
						log.Warn().Err(err).Msg("Failed to publish restored webhook inventory")
					} else {
						log.Info().Int("count", len(webhooks)).Msg("Reloaded webhook configuration after import")
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
					if err := notificationMgr.SetEmailConfig(*emailConfig); err != nil {
						reloadFailures = append(reloadFailures, "email settings")
						log.Warn().Err(err).Msg("Failed to publish email configuration after import")
					} else {
						log.Info().Msg("Reloaded email configuration after import")
					}
				}
			} else {
				reloadFailures = append(reloadFailures, "email settings")
				log.Warn().Err(err).Msg("Failed to reload email configuration after import")
			}
			// Apprise is part of the same persisted import, so its live recipient
			// generation must be restored before acknowledging the reload.
			if appriseConfig, err := h.getPersistence(r.Context()).LoadAppriseConfig(); err == nil {
				if notificationMgr == nil {
					reloadFailures = append(reloadFailures, "Apprise settings")
				} else if err := notificationMgr.SetAppriseConfig(*appriseConfig); err != nil {
					reloadFailures = append(reloadFailures, "Apprise settings")
					log.Warn().Err(err).Msg("Failed to publish Apprise configuration after import")
				}
			} else {
				reloadFailures = append(reloadFailures, "Apprise settings")
				log.Warn().Err(err).Msg("Failed to reload Apprise configuration after import")
			}
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

	if notificationHoldErr != nil {
		deps.AuditEvent(orgID, "config_imported", importUser, deps.ClientIP(r), r.URL.Path, false, "Import saved; restart required to reconstruct notification authority")
		message := "Configuration saved. Restart Pulse to apply the restored notification settings. Do not repeat the import."
		if notifications.ConfigurationHoldReason(notificationHoldErr) == notifications.NotificationReasonStorageBootstrapUnavailable {
			message = "Configuration saved. Repair notification storage, then restart Pulse to apply the restored notification settings. Do not repeat the import."
		}
		if len(reloadFailures) != 0 {
			message += " Live reload also failed for: " + strings.Join(reloadFailures, ", ") + "."
		}
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	if len(reloadFailures) != 0 {
		failed := strings.Join(reloadFailures, ", ")
		deps.AuditEvent(orgID, "config_imported", importUser, deps.ClientIP(r), r.URL.Path, false,
			"Configuration saved but live reload incomplete: "+failed)
		http.Error(w, "Configuration saved, but live reload failed: "+failed+". Check the destination before relying on it. Do not repeat the import.", http.StatusInternalServerError)
		return
	}

	auditDetail, message := "Configuration imported", "Configuration imported successfully"
	if keepAccessCredentials {
		auditDetail += " (API tokens and SSO settings kept)"
		message += ". API tokens and SSO settings were left unchanged: restoring them needs an instance administrator sign-in or a full-access API token that is not bound to any non-default organization."
	}
	deps.AuditEvent(apicontext.OrgID(r.Context()), "config_imported", importUser, deps.ClientIP(r), r.URL.Path, true,
		auditDetail)

	log.Info().Bool("access_credentials_kept", keepAccessCredentials).Msg("Configuration imported successfully")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":                    "success",
		"message":                   message,
		"accessCredentialsRestored": !keepAccessCredentials,
		"remoteAgentEndpoints": map[string]bool{
			"reconfigured":                         false,
			"actionRequiredIfServerAddressChanged": true,
		},
	})
}
