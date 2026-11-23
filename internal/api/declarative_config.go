package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/declarativeconfig"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

const declarativeConfigMaxBodyBytes = 4 << 20

var declarativeConfigApplyLocks sync.Map

func declarativeConfigApplyLock(key string) *sync.Mutex {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "default"
	}
	actual, _ := declarativeConfigApplyLocks.LoadOrStore(key, &sync.Mutex{})
	return actual.(*sync.Mutex)
}

type declarativeConfigPlanEnvelope struct {
	Document json.RawMessage `json:"document"`
	Prune    *bool           `json:"prune,omitempty"`
}

type declarativeConfigApplyEnvelope struct {
	Document json.RawMessage `json:"document"`
	Prune    *bool           `json:"prune,omitempty"`
	PlanHash string          `json:"planHash,omitempty"`
	Confirm  *bool           `json:"confirm,omitempty"`
}

type declarativeConfigApplyResponse struct {
	Applied  bool                             `json:"applied"`
	PlanHash string                           `json:"planHash"`
	Summary  declarativeconfig.PlanSummary    `json:"summary"`
	Changes  []declarativeconfig.ConfigChange `json:"changes"`
}

type parsedDeclarativeConfigInput struct {
	Document declarativeconfig.Document
	Prune    bool
	PlanHash string
	Confirm  bool
}

type operatorStateRollbackEntry struct {
	existed bool
	state   unified.ResourceOperatorState
}

func (r *Router) handleGetDeclarativeConfig(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	wantsYAML, err := resolveDeclarativeExportFormat(req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidDeclarativeConfig, sanitizeDeclarativeClientMessage(err.Error()))
		return
	}

	snapshot, status, err := r.loadDeclarativeSnapshot(req.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to load declarative configuration snapshot")
		writeJSONError(w, status, agentcapabilities.AgentErrCodeDeclarativeConfigApplyFailed, "Failed to load current configuration snapshot")
		return
	}

	doc := declarativeconfig.Export(snapshot)
	if wantsYAML {
		encoded, err := declarativeconfig.MarshalYAML(doc)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal declarative configuration as YAML")
			writeJSONError(w, http.StatusInternalServerError, agentcapabilities.AgentErrCodeDeclarativeConfigApplyFailed, "Failed to encode declarative configuration")
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(encoded)
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

func (r *Router) handlePlanDeclarativeConfig(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parsed, status, code, msg := parseDeclarativeConfigInput(w, req, false)
	if status != 0 {
		writeJSONError(w, status, code, sanitizeDeclarativeClientMessage(msg))
		return
	}

	snapshot, snapStatus, err := r.loadDeclarativeSnapshot(req.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to load declarative configuration snapshot for plan")
		writeJSONError(w, snapStatus, agentcapabilities.AgentErrCodeDeclarativeConfigApplyFailed, "Failed to load current configuration snapshot")
		return
	}

	result, _, err := declarativeconfig.Plan(snapshot, parsed.Document, declarativeconfig.PlanOptions{
		Prune: parsed.Prune,
	})
	if err != nil {
		writeDeclarativePlanError(w, err)
		return
	}
	if result.Changes == nil {
		result.Changes = []declarativeconfig.ConfigChange{}
	}

	writeJSON(w, http.StatusOK, result)
}

func (r *Router) handleApplyDeclarativeConfig(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parsed, status, code, msg := parseDeclarativeConfigInput(w, req, true)
	if status != 0 {
		writeJSONError(w, status, code, sanitizeDeclarativeClientMessage(msg))
		return
	}

	planHash := strings.TrimSpace(parsed.PlanHash)
	if planHash == "" && !parsed.Confirm {
		writeJSONError(
			w,
			http.StatusBadRequest,
			agentcapabilities.AgentErrCodeDeclarativeConfigConfirmationRequired,
			"Apply requires either planHash from plan_declarative_config or confirm set to true",
		)
		return
	}

	ctx := req.Context()
	orgID := GetOrgID(ctx)
	applyMu := declarativeConfigApplyLock(orgID)
	applyMu.Lock()
	defer applyMu.Unlock()

	persistence := r.persistenceForOrg(ctx)
	if persistence == nil {
		writeJSONError(w, http.StatusServiceUnavailable, agentcapabilities.AgentErrCodeDeclarativeConfigApplyFailed, "Configuration persistence is unavailable")
		return
	}

	current, snapStatus, err := r.loadDeclarativeSnapshot(ctx)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("Failed to load declarative configuration snapshot for apply")
		writeJSONError(w, snapStatus, agentcapabilities.AgentErrCodeDeclarativeConfigApplyFailed, "Failed to load current configuration snapshot")
		return
	}

	result, resolved, err := declarativeconfig.Plan(current, parsed.Document, declarativeconfig.PlanOptions{
		Prune: parsed.Prune,
	})
	if err != nil {
		writeDeclarativePlanError(w, err)
		return
	}
	if result.Changes == nil {
		result.Changes = []declarativeconfig.ConfigChange{}
	}

	if planHash != "" && planHash != result.PlanHash {
		writeJSONErrorWithDetails(
			w,
			http.StatusConflict,
			agentcapabilities.AgentErrCodeDeclarativeConfigPlanDrift,
			"Configuration changed since the plan was computed. Run plan again and retry with the updated planHash.",
			map[string]string{"expectedPlanHash": result.PlanHash},
		)
		return
	}

	if !result.Summary.HasChanges {
		writeJSON(w, http.StatusOK, declarativeConfigApplyResponse{
			Applied:  false,
			PlanHash: result.PlanHash,
			Summary:  result.Summary,
			Changes:  result.Changes,
		})
		return
	}

	monitor := r.getTenantMonitor(ctx)
	if (resolved.EmailChanged || resolved.WebhooksChanged || resolved.AppriseChanged) && monitor != nil {
		if notifMgr := monitor.GetNotificationManager(); notifMgr != nil && notifMgr.ConfigurationError() != nil {
			writeJSONError(
				w,
				http.StatusServiceUnavailable,
				agentcapabilities.AgentErrCodeDeclarativeConfigApplyFailed,
				sanitizeDeclarativeClientMessage(notifications.ConfigurationHoldMessage(notifMgr.ConfigurationError())),
			)
			return
		}
	}

	if !r.validateDeclarativeApplyPolicies(w, req, current, resolved) {
		return
	}

	var operatorStore unified.ResourceStore
	if resolved.OperatorStatesChanged {
		if r.resourceHandlers == nil {
			writeJSONError(w, http.StatusServiceUnavailable, agentcapabilities.AgentErrCodeDeclarativeConfigApplyFailed, "Resource operator state store is unavailable")
			return
		}
		store, storeErr := r.resourceHandlers.getStore(orgID)
		if storeErr != nil || store == nil {
			log.Error().Err(storeErr).Str("org_id", orgID).Msg("Failed to open resource store for declarative apply")
			writeJSONError(w, http.StatusServiceUnavailable, agentcapabilities.AgentErrCodeDeclarativeConfigApplyFailed, "Resource operator state store is unavailable")
			return
		}
		operatorStore = store
	}

	actor := auth.GetUser(ctx)
	now := time.Now().UTC()
	var touchedOperatorIDs []string
	var rollbackEntries map[string]operatorStateRollbackEntry

	txErr := persistence.WithTransaction(func(writer *config.ConfigPersistence) error {
		if resolved.AlertsChanged && resolved.AlertConfig != nil {
			if err := writer.SaveAlertConfig(*resolved.AlertConfig); err != nil {
				return fmt.Errorf("save alert config: %w", err)
			}
		}
		if resolved.AlertIntentChanged && resolved.AlertIntentPolicies != nil {
			if err := writer.SaveAlertIntentPolicies(*resolved.AlertIntentPolicies); err != nil {
				return fmt.Errorf("save alert intent policies: %w", err)
			}
		}
		if resolved.AvailabilityTargetsChanged {
			if err := writer.SaveAvailabilityTargets(resolved.AvailabilityTargets); err != nil {
				return fmt.Errorf("save availability targets: %w", err)
			}
		}
		if resolved.EmailChanged && resolved.Email != nil {
			if err := writer.SaveEmailConfig(*resolved.Email); err != nil {
				return fmt.Errorf("save email config: %w", err)
			}
		}
		if resolved.WebhooksChanged {
			if err := writer.SaveWebhooks(resolved.Webhooks); err != nil {
				return fmt.Errorf("save webhooks: %w", err)
			}
		}
		if resolved.AppriseChanged && resolved.Apprise != nil {
			if err := writer.SaveAppriseConfig(*resolved.Apprise); err != nil {
				return fmt.Errorf("save apprise config: %w", err)
			}
		}
		if resolved.DeadManChanged && resolved.DeadMan != nil {
			if err := writer.SaveDeadManConfig(*resolved.DeadMan); err != nil {
				return fmt.Errorf("save dead-man config: %w", err)
			}
		}
		if resolved.OperatorStatesChanged {
			var opErr error
			touchedOperatorIDs, rollbackEntries, opErr = r.applyDeclarativeOperatorStates(operatorStore, resolved, now, actor)
			if opErr != nil {
				rollbackDeclarativeOperatorStates(operatorStore, rollbackEntries)
				rollbackEntries = nil
				return fmt.Errorf("apply operator states: %w", opErr)
			}
		}
		return nil
	})
	if txErr != nil {
		if len(rollbackEntries) > 0 && operatorStore != nil {
			rollbackDeclarativeOperatorStates(operatorStore, rollbackEntries)
		}
		log.Error().Err(txErr).Str("org_id", orgID).Str("plan_hash", result.PlanHash).Msg("Failed to persist declarative configuration")
		LogAuditEventForTenant(
			orgID,
			"declarative_config_apply",
			actor,
			GetClientIP(req),
			req.URL.Path,
			false,
			fmt.Sprintf("Failed to persist declarative config planHash=%s", result.PlanHash),
		)
		writeJSONError(w, http.StatusInternalServerError, agentcapabilities.AgentErrCodeDeclarativeConfigApplyFailed, "Failed to persist declarative configuration changes")
		return
	}

	if resolved.OperatorStatesChanged && r.resourceHandlers != nil && r.resourceHandlers.operatorStateChanged != nil {
		for _, canonicalID := range touchedOperatorIDs {
			r.resourceHandlers.operatorStateChanged(orgID, canonicalID)
		}
	}

	reloadFailures := r.reloadManagersAfterDeclarativeApply(monitor, resolved)
	if len(reloadFailures) > 0 {
		failedList := strings.Join(reloadFailures, ", ")
		LogAuditEventForTenant(
			orgID,
			"declarative_config_apply",
			actor,
			GetClientIP(req),
			req.URL.Path,
			false,
			fmt.Sprintf("Persisted declarative config planHash=%s but live reload failed for %s", result.PlanHash, failedList),
		)
		writeJSONError(
			w,
			http.StatusInternalServerError,
			agentcapabilities.AgentErrCodeDeclarativeConfigApplyFailed,
			"Configuration was saved, but live reload failed for "+failedList,
		)
		return
	}

	auditDetails := fmt.Sprintf(
		"Applied declarative config planHash=%s creates=%d updates=%d deletes=%d unchanged=%d",
		result.PlanHash,
		result.Summary.Creates,
		result.Summary.Updates,
		result.Summary.Deletes,
		result.Summary.Unchanged,
	)
	LogAuditEventForTenant(orgID, "declarative_config_apply", actor, GetClientIP(req), req.URL.Path, true, auditDetails)

	writeJSON(w, http.StatusOK, declarativeConfigApplyResponse{
		Applied:  true,
		PlanHash: result.PlanHash,
		Summary:  result.Summary,
		Changes:  result.Changes,
	})
}

func (r *Router) loadDeclarativeSnapshot(ctx context.Context) (declarativeconfig.Snapshot, int, error) {
	persistence := r.persistenceForOrg(ctx)
	if persistence == nil {
		return declarativeconfig.Snapshot{}, http.StatusServiceUnavailable, errors.New("configuration persistence is unavailable")
	}

	alertConfig, err := persistence.LoadAlertConfig()
	if err != nil {
		return declarativeconfig.Snapshot{}, http.StatusInternalServerError, fmt.Errorf("load alert config: %w", err)
	}
	intentPolicies, err := persistence.LoadAlertIntentPolicies()
	if err != nil {
		return declarativeconfig.Snapshot{}, http.StatusInternalServerError, fmt.Errorf("load alert intent policies: %w", err)
	}
	availabilityTargets, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		return declarativeconfig.Snapshot{}, http.StatusInternalServerError, fmt.Errorf("load availability targets: %w", err)
	}
	emailConfig, err := persistence.LoadEmailConfig()
	if err != nil {
		return declarativeconfig.Snapshot{}, http.StatusInternalServerError, fmt.Errorf("load email config: %w", err)
	}
	webhooks, err := persistence.LoadWebhooks()
	if err != nil {
		return declarativeconfig.Snapshot{}, http.StatusInternalServerError, fmt.Errorf("load webhooks: %w", err)
	}
	appriseConfig, err := persistence.LoadAppriseConfig()
	if err != nil {
		return declarativeconfig.Snapshot{}, http.StatusInternalServerError, fmt.Errorf("load apprise config: %w", err)
	}
	deadManConfig, err := persistence.LoadDeadManConfig()
	if err != nil {
		return declarativeconfig.Snapshot{}, http.StatusInternalServerError, fmt.Errorf("load dead-man config: %w", err)
	}

	var operatorStates []unified.ResourceOperatorState
	if r.resourceHandlers != nil {
		store, storeErr := r.resourceHandlers.getStore(GetOrgID(ctx))
		if storeErr != nil {
			return declarativeconfig.Snapshot{}, http.StatusInternalServerError, fmt.Errorf("open resource store: %w", storeErr)
		}
		if store != nil {
			states, listErr := store.ListResourceOperatorStates()
			if listErr != nil {
				return declarativeconfig.Snapshot{}, http.StatusInternalServerError, fmt.Errorf("list operator states: %w", listErr)
			}
			operatorStates = states
		}
	}

	return declarativeconfig.Snapshot{
		AlertConfig:         alertConfig,
		AlertIntentPolicies: intentPolicies,
		AvailabilityTargets: availabilityTargets,
		OperatorStates:      operatorStates,
		Email:               emailConfig,
		Webhooks:            webhooks,
		Apprise:             appriseConfig,
		DeadMan:             deadManConfig,
	}, http.StatusOK, nil
}

func (r *Router) validateDeclarativeApplyPolicies(
	w http.ResponseWriter,
	req *http.Request,
	current declarativeconfig.Snapshot,
	resolved declarativeconfig.ResolvedState,
) bool {
	if resolved.AvailabilityTargetsChanged {
		existingByID := make(map[string]config.AvailabilityTarget, len(current.AvailabilityTargets))
		for _, existing := range current.AvailabilityTargets {
			existingByID[strings.TrimSpace(existing.ID)] = existing
		}
		needsExternalProbeLicense := false
		for _, target := range resolved.AvailabilityTargets {
			if refuseTenantAvailabilityDestination(w, req, target) {
				return false
			}
			nextAgents := target.AssignedProbeAgentIDs()
			if len(nextAgents) == 0 {
				continue
			}
			prev, exists := existingByID[strings.TrimSpace(target.ID)]
			if !exists || !slices.Equal(prev.AssignedProbeAgentIDs(), nextAgents) {
				needsExternalProbeLicense = true
			}
		}
		if needsExternalProbeLicense {
			if !r.requireDeclarativeExternalProbeLicense(w, req.Context()) {
				return false
			}
		}
	}

	if resolved.EmailChanged && resolved.Email != nil && strings.TrimSpace(resolved.Email.SMTPHost) != "" {
		if refuseTenantOutboundDestination(w, req, strings.TrimSpace(resolved.Email.SMTPHost)) {
			return false
		}
	}

	if resolved.WebhooksChanged {
		for _, webhook := range resolved.Webhooks {
			if strings.TrimSpace(webhook.URL) != "" && refuseTenantOutboundDestination(w, req, webhook.URL) {
				return false
			}
		}
	}

	if resolved.AppriseChanged && resolved.Apprise != nil {
		normalized := notifications.NormalizeAppriseConfig(*resolved.Apprise)
		if normalized.Mode == notifications.AppriseModeCLI && len(normalized.Targets) > 0 {
			var existingNormalized notifications.AppriseConfig
			if current.Apprise != nil {
				existingNormalized = notifications.NormalizeAppriseConfig(*current.Apprise)
			}
			if existingNormalized.Mode != notifications.AppriseModeCLI || !slices.Equal(normalized.Targets, existingNormalized.Targets) {
				if !requestCarriesInstanceAuthority(r.config, req) {
					writeJSONError(
						w,
						http.StatusForbidden,
						agentcapabilities.AgentErrCodeDeclarativeConfigApplyFailed,
						"Apprise command-line delivery needs an instance administrator. Use an Apprise API server instead.",
					)
					return false
				}
			}
		}
		if strings.TrimSpace(normalized.ServerURL) != "" && refuseTenantOutboundDestination(w, req, normalized.ServerURL) {
			return false
		}
	}

	if resolved.DeadManChanged && resolved.DeadMan != nil && strings.TrimSpace(resolved.DeadMan.PingURL) != "" {
		if refuseTenantOutboundDestination(w, req, resolved.DeadMan.PingURL) {
			return false
		}
	}

	return true
}

func (r *Router) requireDeclarativeExternalProbeLicense(w http.ResponseWriter, ctx context.Context) bool {
	if r == nil || r.licenseHandlers == nil {
		WriteLicenseRequired(w, featureExternalProbeValue, "License service unavailable")
		return false
	}
	service := r.licenseHandlers.Service(ctx)
	if service == nil {
		WriteLicenseRequired(w, featureExternalProbeValue, "License service unavailable")
		return false
	}
	if err := service.RequireFeature(featureExternalProbeValue); err != nil {
		WriteLicenseRequired(w, featureExternalProbeValue, sanitizeDeclarativeClientMessage(err.Error()))
		return false
	}
	return true
}

func (r *Router) applyDeclarativeOperatorStates(
	store unified.ResourceStore,
	resolved declarativeconfig.ResolvedState,
	now time.Time,
	actor string,
) ([]string, map[string]operatorStateRollbackEntry, error) {
	if store == nil {
		return nil, nil, errors.New("resource operator state store is unavailable")
	}

	rollback := make(map[string]operatorStateRollbackEntry)
	touched := make([]string, 0, len(resolved.OperatorStatesToUpsert)+len(resolved.OperatorStatesToClear))
	touchedSet := make(map[string]struct{}, len(resolved.OperatorStatesToUpsert)+len(resolved.OperatorStatesToClear))

	recordPrevious := func(canonicalID string) error {
		if _, seen := rollback[canonicalID]; seen {
			return nil
		}
		prev, found, err := store.GetResourceOperatorState(canonicalID)
		if err != nil {
			return err
		}
		rollback[canonicalID] = operatorStateRollbackEntry{
			existed: found,
			state:   prev,
		}
		return nil
	}

	recordTouched := func(canonicalID string) {
		if _, seen := touchedSet[canonicalID]; seen {
			return
		}
		touchedSet[canonicalID] = struct{}{}
		touched = append(touched, canonicalID)
	}

	mutate := func() error {
		for _, desired := range resolved.OperatorStatesToUpsert {
			canonicalID := unified.CanonicalResourceID(desired.CanonicalID)
			if canonicalID == "" {
				return errors.New("operator state canonicalId is required")
			}
			if err := recordPrevious(canonicalID); err != nil {
				return err
			}
			desired.CanonicalID = canonicalID
			desired.SetAt = now
			desired.SetBy = actor
			if _, err := unified.SetResourceOperatorStateWithMaintenanceLifecycle(store, desired); err != nil {
				return err
			}
			recordTouched(canonicalID)
		}

		for _, rawID := range resolved.OperatorStatesToClear {
			canonicalID := unified.CanonicalResourceID(rawID)
			if canonicalID == "" {
				continue
			}
			if err := recordPrevious(canonicalID); err != nil {
				return err
			}
			if err := unified.ClearResourceOperatorStateWithMaintenanceLifecycle(store, canonicalID, now, actor); err != nil {
				return err
			}
			recordTouched(canonicalID)
		}
		return nil
	}

	var err error
	if r.resourceHandlers != nil && r.resourceHandlers.ActionLifecycle() != nil {
		err = r.resourceHandlers.ActionLifecycle().WithPolicyMutation(mutate)
	} else {
		err = mutate()
	}
	if err != nil {
		return nil, rollback, err
	}
	return touched, rollback, nil
}

func rollbackDeclarativeOperatorStates(store unified.ResourceStore, rollback map[string]operatorStateRollbackEntry) {
	if store == nil || len(rollback) == 0 {
		return
	}
	for canonicalID, entry := range rollback {
		if entry.existed {
			if err := store.SetResourceOperatorState(entry.state); err != nil {
				log.Error().Err(err).Str("canonical_id", canonicalID).Msg("Failed to restore operator state during declarative apply rollback")
			}
		} else {
			if err := store.ClearResourceOperatorState(canonicalID); err != nil {
				log.Error().Err(err).Str("canonical_id", canonicalID).Msg("Failed to clear operator state during declarative apply rollback")
			}
		}
	}
}

func (r *Router) reloadManagersAfterDeclarativeApply(
	monitor *monitoring.Monitor,
	resolved declarativeconfig.ResolvedState,
) []string {
	if monitor == nil {
		return nil
	}
	var reloadFailures []string

	if resolved.AlertsChanged && resolved.AlertConfig != nil {
		if alertMgr := monitor.GetAlertManager(); alertMgr != nil {
			alertMgr.UpdateConfig(*resolved.AlertConfig)
			updatedCfg := alertMgr.GetConfig()
			if notifMgr := monitor.GetNotificationManager(); notifMgr != nil {
				notifMgr.SetEnabled(updatedCfg.Enabled && updatedCfg.ActivationState == alerts.ActivationActive)
				notifMgr.SetCooldown(updatedCfg.Schedule.Cooldown)
				notifMgr.SetGroupingConfig(
					updatedCfg.Schedule.Grouping.Enabled,
					updatedCfg.Schedule.Grouping.Window,
					updatedCfg.Schedule.Grouping.ByNode,
					updatedCfg.Schedule.Grouping.ByGuest,
				)
				notifMgr.SetInitialNotifyTarget(updatedCfg.Schedule.InitialNotify)
				notifMgr.SetNotifyOnResolve(updatedCfg.Schedule.NotifyOnResolve)
			}
		}
	}

	if resolved.AlertIntentChanged && resolved.AlertIntentPolicies != nil {
		if alertMgr := monitor.GetAlertManager(); alertMgr != nil {
			if err := alertMgr.LoadIntentPolicies(*resolved.AlertIntentPolicies); err != nil {
				reloadFailures = append(reloadFailures, "alert intent policies")
				log.Warn().Err(err).Msg("Failed to reload alert intent policies after declarative apply")
			}
		}
	}

	if resolved.AvailabilityTargetsChanged {
		monitor.RefreshAvailabilityTargets()
	}

	if notifMgr := monitor.GetNotificationManager(); notifMgr != nil {
		if resolved.EmailChanged && resolved.Email != nil {
			if err := notifMgr.SetEmailConfig(*resolved.Email); err != nil {
				reloadFailures = append(reloadFailures, "email notifications")
				log.Warn().Err(err).Msg("Failed to reload email configuration after declarative apply")
			}
		}
		if resolved.WebhooksChanged {
			if err := notifMgr.ReplaceWebhooks(resolved.Webhooks); err != nil {
				reloadFailures = append(reloadFailures, "webhook notifications")
				log.Warn().Err(err).Msg("Failed to reload webhooks after declarative apply")
			}
		}
		if resolved.AppriseChanged && resolved.Apprise != nil {
			if err := notifMgr.SetAppriseConfig(*resolved.Apprise); err != nil {
				reloadFailures = append(reloadFailures, "Apprise notifications")
				log.Warn().Err(err).Msg("Failed to reload Apprise configuration after declarative apply")
			}
		}
	}

	if resolved.DeadManChanged && resolved.DeadMan != nil {
		if err := monitor.UpdateDeadManConfig(*resolved.DeadMan); err != nil {
			reloadFailures = append(reloadFailures, "dead-man watchdog")
			log.Warn().Err(err).Msg("Failed to reload dead-man configuration after declarative apply")
		}
	}

	if resolved.AlertsChanged || resolved.AlertIntentChanged {
		monitor.SyncAlertState()
	}

	return reloadFailures
}

func resolveDeclarativeExportFormat(req *http.Request) (bool, error) {
	formatParam := strings.ToLower(strings.TrimSpace(req.URL.Query().Get("format")))
	switch formatParam {
	case "yaml", "yml":
		return true, nil
	case "json":
		return false, nil
	case "":
		return acceptHeaderPrefersYAML(req.Header.Get("Accept")), nil
	default:
		return false, errors.New("unsupported format parameter. Use json or yaml")
	}
}

func acceptHeaderPrefersYAML(accept string) bool {
	accept = strings.TrimSpace(accept)
	if accept == "" {
		return false
	}
	wantsYAML := false
	wantsJSON := false
	for _, part := range strings.Split(accept, ",") {
		mediaType := strings.TrimSpace(part)
		if idx := strings.IndexByte(mediaType, ';'); idx >= 0 {
			mediaType = strings.TrimSpace(mediaType[:idx])
		}
		switch strings.ToLower(mediaType) {
		case "application/yaml", "text/yaml", "application/x-yaml":
			wantsYAML = true
		case "application/json":
			wantsJSON = true
		}
	}
	return wantsYAML && !wantsJSON
}

func parseDeclarativeConfigInput(w http.ResponseWriter, req *http.Request, forApply bool) (parsedDeclarativeConfigInput, int, string, string) {
	var out parsedDeclarativeConfigInput

	query := req.URL.Query()
	if rawPrune := strings.TrimSpace(query.Get("prune")); rawPrune != "" {
		val, err := strconv.ParseBool(rawPrune)
		if err != nil {
			return out, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidDeclarativeConfig, "Invalid prune query parameter. Use true or false."
		}
		out.Prune = val
	}
	if forApply {
		out.PlanHash = strings.TrimSpace(query.Get("planHash"))
		if rawConfirm := strings.TrimSpace(query.Get("confirm")); rawConfirm != "" {
			val, err := strconv.ParseBool(rawConfirm)
			if err != nil {
				return out, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidDeclarativeConfig, "Invalid confirm query parameter. Use true or false."
			}
			out.Confirm = val
		}
	}

	limited := http.MaxBytesReader(w, req.Body, declarativeConfigMaxBodyBytes)
	rawBody, err := io.ReadAll(limited)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return out, http.StatusRequestEntityTooLarge, agentcapabilities.AgentErrCodeInvalidDeclarativeConfig, "Request body exceeds the 4 MiB limit"
		}
		return out, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidDeclarativeConfig, "Failed to read request body"
	}

	trimmed := bytes.TrimSpace(rawBody)
	if len(trimmed) == 0 {
		return out, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidDeclarativeConfig, "Request body is required"
	}

	var docBytes []byte
	var top map[string]json.RawMessage
	if json.Unmarshal(trimmed, &top) == nil && top != nil {
		if _, hasDocument := top["document"]; hasDocument {
			if forApply {
				var env declarativeConfigApplyEnvelope
				dec := json.NewDecoder(bytes.NewReader(trimmed))
				dec.DisallowUnknownFields()
				if err := dec.Decode(&env); err != nil {
					return out, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidDeclarativeConfig, "Invalid declarative apply request payload"
				}
				var extra json.RawMessage
				if err := dec.Decode(&extra); err != io.EOF {
					return out, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidDeclarativeConfig, "Request body must contain a single JSON object"
				}
				if env.Prune != nil {
					out.Prune = *env.Prune
				}
				if strings.TrimSpace(env.PlanHash) != "" {
					out.PlanHash = strings.TrimSpace(env.PlanHash)
				}
				if env.Confirm != nil {
					out.Confirm = *env.Confirm
				}
				docBytes = bytes.TrimSpace(env.Document)
			} else {
				var env declarativeConfigPlanEnvelope
				dec := json.NewDecoder(bytes.NewReader(trimmed))
				dec.DisallowUnknownFields()
				if err := dec.Decode(&env); err != nil {
					return out, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidDeclarativeConfig, "Invalid declarative plan request payload"
				}
				var extra json.RawMessage
				if err := dec.Decode(&extra); err != io.EOF {
					return out, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidDeclarativeConfig, "Request body must contain a single JSON object"
				}
				if env.Prune != nil {
					out.Prune = *env.Prune
				}
				docBytes = bytes.TrimSpace(env.Document)
			}

			if len(docBytes) == 0 || bytes.Equal(docBytes, []byte("null")) {
				return out, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidDeclarativeConfig, "Document field is required"
			}
			if docBytes[0] == '"' {
				var rawDocString string
				if err := json.Unmarshal(docBytes, &rawDocString); err != nil {
					return out, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidDeclarativeConfig, "Invalid declarative document string"
				}
				docBytes = []byte(rawDocString)
			}
		} else {
			docBytes = trimmed
		}
	} else {
		docBytes = trimmed
	}

	doc, err := declarativeconfig.UnmarshalDocument(docBytes)
	if err != nil {
		return out, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidDeclarativeConfig, err.Error()
	}
	out.Document = doc
	return out, 0, "", ""
}

func writeDeclarativePlanError(w http.ResponseWriter, err error) {
	msg := sanitizeDeclarativeClientMessage(err.Error())
	switch {
	case errors.Is(err, declarativeconfig.ErrUnresolvedSecretRef):
		writeJSONError(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeDeclarativeConfigUnresolvedSecretRef, msg)
	case errors.Is(err, declarativeconfig.ErrSecretMissing):
		writeJSONError(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeDeclarativeConfigSecretMissing, msg)
	default:
		writeJSONError(w, http.StatusBadRequest, agentcapabilities.AgentErrCodeInvalidDeclarativeConfig, msg)
	}
}

func sanitizeDeclarativeClientMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return "Invalid declarative configuration request"
	}
	msg = strings.ReplaceAll(msg, "—", "-")
	msg = strings.ReplaceAll(msg, ";", ",")
	return msg
}
