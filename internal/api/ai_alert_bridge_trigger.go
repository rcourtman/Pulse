package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rs/zerolog/log"
)

// alertBridgePatrolWiring turns alert bridge events into scoped Patrol run
// requests for one organisation.
type alertBridgePatrolWiring struct {
	// aiConfig returns the current AI settings. They hold the operator's
	// alert-trigger policy and can change while Pulse runs.
	aiConfig func() *config.AIConfig
	// triggerManager returns the organisation's trigger queue, or nil before
	// it has started.
	triggerManager func() *ai.TriggerManager
	// runScoped runs a scoped Patrol directly when no trigger queue exists.
	runScoped func(scope ai.PatrolScope)
	// alertCleared closes the findings tied to an alert that has resolved.
	alertCleared func(alertIdentifier, alertResourceID, alertType string) int
	// investigates reports whether Patrol is on and in a mode that
	// investigates findings (Ask first or Autopilot), as opposed to Watch
	// only. The sustained trigger is limited to those modes.
	investigates func() bool
	// lookedAtSince reports whether a Patrol run has already examined this
	// alert since the given time.
	lookedAtSince func(alertIdentifier string, since time.Time) bool
}

// attach connects the wiring to the bridge.
func (w alertBridgePatrolWiring) attach(bridge *unified.AlertBridge) {
	bridge.SetPatrolTriggerPolicy(w.acceptsFiringAlert)
	bridge.SetSustainedTriggerPolicy(w.acceptsSustainedAlert)
	bridge.SetPatrolTrigger(w.trigger)
}

// acceptsFiringAlert applies the operator's alert-trigger policy: the master
// switch, the minimum severity and the optional alert-type allowlist.
func (w alertBridgePatrolWiring) acceptsFiringAlert(alertType, alertLevel string) bool {
	if w.aiConfig == nil {
		return true
	}
	aiCfg := w.aiConfig()
	return aiCfg == nil || aiCfg.AlertTriggersInvestigation(alertType, alertLevel)
}

// acceptsSustainedAlert decides whether an alert that has stayed active
// without waking Patrol may wake it now. The minimum severity setting governs
// alerts the moment they fire and is not consulted here: this rule exists for
// the warnings below it and for alerts that were already active when Pulse
// started. It applies only where Patrol investigates findings, so Watch only
// and free installs get no extra runs from it, and it respects the alert
// trigger switch and the alert-type allowlist.
func (w alertBridgePatrolWiring) acceptsSustainedAlert(event unified.PatrolTriggerEvent, activeSince time.Time) unified.SustainedTriggerDecision {
	if w.aiConfig == nil || w.investigates == nil {
		return unified.SustainedTriggerWait
	}
	aiCfg := w.aiConfig()
	// Asking at the critical level leaves the master switch and the
	// allowlist as the only things the trigger policy can refuse.
	if aiCfg == nil || !aiCfg.AlertTriggersInvestigation(event.AlertType, config.AlertTriggerSeverityCritical) {
		return unified.SustainedTriggerWait
	}
	if !w.investigates() {
		return unified.SustainedTriggerWait
	}
	if w.lookedAtSince != nil && w.lookedAtSince(event.AlertIdentifier, activeSince) {
		return unified.SustainedTriggerSkip
	}
	return unified.SustainedTriggerRun
}

// trigger queues a scoped Patrol run for one alert bridge event.
func (w alertBridgePatrolWiring) trigger(event unified.PatrolTriggerEvent) {
	// The alert constructors mark the type as a hint. The unified store's
	// type for an alert ("agent-disk" for a host filesystem) is often not a
	// Patrol resource type, and the scope resolver replaces a hint with the
	// type of the resource the alert resolves to.
	var scope ai.PatrolScope
	switch event.Reason {
	case "alert_fired":
		// The bridge asked acceptsFiringAlert, or acceptsSustainedAlert for a
		// sustained event, before it sent this.
		if event.Sustained {
			scope = ai.SustainedAlertPatrolScope(event.AlertIdentifier, event.ResourceID, event.ResourceType, event.AlertType)
			if event.AlertType != "" {
				scope.Context = fmt.Sprintf("Alert still active: %s = %.1f (threshold %.1f)", event.AlertType, event.Value, event.Threshold)
			}
			break
		}
		scope = ai.AlertTriggeredPatrolScope(event.AlertIdentifier, event.ResourceID, event.ResourceType, event.AlertType)
		if event.AlertType != "" {
			scope.Context = fmt.Sprintf("Alert: %s = %.1f (threshold %.1f)", event.AlertType, event.Value, event.Threshold)
		}
	case "alert_cleared":
		// Findings that restate the alert are closed here, whatever their
		// investigation concluded and whether or not the resource still
		// exists. The run below then checks the rest with current evidence.
		if w.alertCleared != nil {
			if closed := w.alertCleared(event.AlertIdentifier, event.ResourceID, event.AlertType); closed > 0 {
				log.Info().
					Str("alert_identifier", event.AlertIdentifier).
					Int("findings_closed", closed).
					Msg("Alert bridge: closed findings tied to the cleared alert")
			}
		}
		scope = ai.AlertClearedPatrolScope(event.AlertIdentifier, event.ResourceID, event.ResourceType)
		if event.AlertType != "" {
			scope.Context = "Alert cleared: " + event.AlertType
		}
	default:
		scope = ai.PatrolScope{
			ResourceIDs:     []string{event.ResourceID},
			ResourceTypes:   []string{event.ResourceType},
			Depth:           ai.PatrolDepthQuick,
			Reason:          ai.TriggerReasonManual,
			Context:         "Alert bridge: " + event.Reason,
			Priority:        50,
			AlertIdentifier: event.AlertIdentifier,
		}
	}
	if event.AlertType != "" {
		// Shown until the run resolves the scope, which replaces it with the
		// collected alert's own evidence.
		scope.AlertContext = &ai.PatrolAlertContext{
			AlertType: event.AlertType,
			Level:     event.AlertLevel,
			Value:     event.Value,
			Threshold: event.Threshold,
			Message:   event.Message,
		}
	}

	log.Debug().
		Str("resource_id", event.ResourceID).
		Str("reason", event.Reason).
		Msg("Alert bridge: Triggering mini-patrol")
	if w.triggerManager != nil {
		if triggerManager := w.triggerManager(); triggerManager != nil {
			if triggerManager.TriggerPatrol(scope) {
				log.Debug().
					Str("resource_id", event.ResourceID).
					Str("reason", event.Reason).
					Msg("Alert bridge: Queued patrol via trigger manager")
			} else {
				log.Warn().
					Str("resource_id", event.ResourceID).
					Str("reason", event.Reason).
					Msg("Alert bridge: Patrol trigger rejected by trigger manager")
			}
			return
		}
	}
	if w.runScoped != nil {
		w.runScoped(scope)
	}
}

// newAlertBridgePatrolWiring builds the production wiring for an organisation.
func (r *Router) newAlertBridgePatrolWiring(orgID string, aiService *ai.Service, patrol *ai.PatrolService) alertBridgePatrolWiring {
	return alertBridgePatrolWiring{
		aiConfig: aiService.GetAIConfig,
		triggerManager: func() *ai.TriggerManager {
			return r.aiSettingsHandler.GetTriggerManagerForOrg(orgID)
		},
		runScoped: func(scope ai.PatrolScope) {
			orgCtx := context.WithValue(context.Background(), OrgIDContextKey, orgID)
			patrol.TriggerScopedPatrol(orgCtx, scope)
		},
		alertCleared: patrol.ResolveFindingsForClearedAlert,
		investigates: func() bool {
			return patrol.GetConfig().Enabled &&
				aiService.GetEffectivePatrolAutonomyLevel() != config.PatrolAutonomyMonitor
		},
		lookedAtSince: patrol.AlertExaminedSince,
	}
}

// patrolAlertSources remembers, per organisation, which alert manager Patrol's
// alert-driven wiring listens to and how to point it at another one.
//
// Patrol is started once per organisation and binds the alert bridge, the
// flapping callback and the threshold reader to the alert manager of the
// monitor that is live at that moment. A full monitor reload (a node added, an
// auto-registration, a polling interval change) stops that monitor and builds
// a new one with a new alert manager. Nothing told Patrol, so after any reload
// alerts stopped waking it until the process restarted.
type patrolAlertSources struct {
	mu    sync.Mutex
	byOrg map[string]*patrolAlertSource
}

type patrolAlertSource struct {
	bound  *alerts.Manager
	rebind func(*alerts.Manager)
}

// register records the alert manager an organisation's wiring was just built
// on, and the function that moves it to another.
func (s *patrolAlertSources) register(orgID string, bound *alerts.Manager, rebind func(*alerts.Manager)) {
	if s == nil || rebind == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byOrg == nil {
		s.byOrg = make(map[string]*patrolAlertSource)
	}
	s.byOrg[normalizePatrolOrgID(orgID)] = &patrolAlertSource{bound: bound, rebind: rebind}
}

// follow moves an organisation's wiring to the alert manager of the monitor
// that is live now. It does nothing for an organisation whose Patrol has not
// started, and nothing when the wiring already listens to this manager.
func (s *patrolAlertSources) follow(orgID string, current *alerts.Manager) bool {
	if s == nil || current == nil {
		return false
	}
	s.mu.Lock()
	source := s.byOrg[normalizePatrolOrgID(orgID)]
	if source == nil || source.bound == current {
		s.mu.Unlock()
		return false
	}
	source.bound = current
	rebind := source.rebind
	s.mu.Unlock()
	rebind(current)
	return true
}

// followMonitorAlertManager runs for every monitor the router is handed,
// including the ones a full reload builds, and keeps Patrol listening to the
// alert manager that is in use.
func (r *Router) followMonitorAlertManager(m *monitoring.Monitor) {
	if r == nil || m == nil {
		return
	}
	alertManager := m.GetAlertManager()
	if alertManager == nil {
		return
	}
	if r.patrolAlertSources.follow(m.GetOrgID(), alertManager) {
		log.Info().
			Str("org_id", normalizePatrolOrgID(m.GetOrgID())).
			Msg("AI Intelligence: Patrol alert wiring moved to the reloaded monitor's alert manager")
	}
}

// wireAlertFlappingPostmortem has the alert manager tell Patrol when an alert
// starts flapping: a postmortem finding the operator can see, and a scoped run.
func wireAlertFlappingPostmortem(alertManager *alerts.Manager, patrol *ai.PatrolService, triggerManager *ai.TriggerManager) {
	if alertManager == nil || patrol == nil || triggerManager == nil {
		return
	}
	alertManager.SetFlappingDetectedCallback(func(alert *alerts.Alert, trackingKey string) {
		if alert == nil || trackingKey == "" {
			return
		}
		emitFlappingPostmortemFinding(patrol, alertManager, alert, trackingKey)
		scope := ai.FlappingPostmortemPatrolScope(
			alert.ID,
			alert.ResourceID,
			alert.CanonicalKind,
			alert.Type,
		)
		if triggerManager.TriggerPatrol(scope) {
			log.Debug().
				Str("alertID", alert.ID).
				Str("trackingKey", trackingKey).
				Str("alertType", alert.Type).
				Msg("Flapping-detected triggered postmortem patrol via TriggerManager")
		}
	})
}

// startUnifiedAlertBridge builds an organisation's unified alert store and
// alert bridge on the given alert manager, wires the bridge to Patrol, starts
// it, and records how to move it when a monitor reload replaces the manager.
func (r *Router) startUnifiedAlertBridge(orgID string, aiService *ai.Service, alertManager *alerts.Manager) *unified.AlertBridge {
	if r == nil || r.aiSettingsHandler == nil || aiService == nil || alertManager == nil {
		return nil
	}
	unifiedStore := unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig())
	r.aiSettingsHandler.SetUnifiedStoreForOrg(orgID, unifiedStore)

	alertBridge := unified.NewAlertBridge(unifiedStore, unified.DefaultBridgeConfig())
	alertBridge.SetAlertProvider(unified.NewAlertManagerAdapter(alertManager))

	// Set patrol trigger function (triggers mini-patrol on alert events)
	patrol := aiService.GetPatrolService()
	if patrol != nil && ai.BackgroundAutomationDisabledForDev() {
		log.Info().
			Str("env", ai.DevDisableBackgroundAIEnv).
			Str("org_id", orgID).
			Msg("Pulse dev background AI disabled; alert bridge patrol trigger not wired")
	} else if patrol != nil {
		r.newAlertBridgePatrolWiring(orgID, aiService, patrol).attach(alertBridge)
	}

	alertBridge.Start()
	r.aiSettingsHandler.SetAlertBridgeForOrg(orgID, alertBridge)
	log.Info().Msg("AI Intelligence: Unified alert/finding bridge initialized and started")

	// A monitor reload builds a new alert manager. Everything here that
	// listens to this one has to move with it.
	r.patrolAlertSources.register(orgID, alertManager, func(current *alerts.Manager) {
		alertBridge.ReplaceAlertProvider(unified.NewAlertManagerAdapter(current))
		aiService.SetPatrolThresholdProvider(ai.NewAlertThresholdAdapter(current))
		wireAlertFlappingPostmortem(current, aiService.GetPatrolService(), r.aiSettingsHandler.GetTriggerManagerForOrg(orgID))
	})
	return alertBridge
}
