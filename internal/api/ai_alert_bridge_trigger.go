package api

import (
	"context"
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
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
}

// attach connects the wiring to the bridge.
func (w alertBridgePatrolWiring) attach(bridge *unified.AlertBridge) {
	bridge.SetPatrolTriggerPolicy(w.acceptsFiringAlert)
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

// trigger queues a scoped Patrol run for one alert bridge event.
func (w alertBridgePatrolWiring) trigger(event unified.PatrolTriggerEvent) {
	// The alert constructors mark the type as a hint. The unified store's
	// type for an alert ("agent-disk" for a host filesystem) is often not a
	// Patrol resource type, and the scope resolver replaces a hint with the
	// type of the resource the alert resolves to.
	var scope ai.PatrolScope
	switch event.Reason {
	case "alert_fired":
		// The bridge asked acceptsFiringAlert before it sent this event.
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
	}
}
