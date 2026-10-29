package api

import (
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// AgentEventResourceChangePayload is the payload shape for alert.fired,
// alert.resolved and resource.status_changed events. It mirrors the change
// record list_recent_changes returns (changeId is that record's id, kind its
// kind) so an agent can read the surrounding history, and carries no alert
// message, reason text or command. ResourceName and ResourceType are the
// resource's current values and are empty when Pulse no longer knows it.
//
// CoalescedTransitions is set only on a resource.status_changed event that
// was held back by the per-resource coalescing window. It counts the status
// changes Pulse recorded in that window; From is the status last announced
// and To the status at the window's end. Every individual transition stays
// readable through list_recent_changes.
type AgentEventResourceChangePayload struct {
	ChangeID             string     `json:"changeId"`
	ResourceID           string     `json:"resourceId"`
	ResourceName         string     `json:"resourceName,omitempty"`
	ResourceType         string     `json:"resourceType,omitempty"`
	Kind                 string     `json:"kind"`
	From                 string     `json:"from,omitempty"`
	To                   string     `json:"to,omitempty"`
	AlertIdentifier      string     `json:"alertIdentifier,omitempty"`
	AlertType            string     `json:"alertType,omitempty"`
	AlertLevel           string     `json:"alertLevel,omitempty"`
	AlertResolution      string     `json:"alertResolution,omitempty"`
	ObservedAt           time.Time  `json:"observedAt"`
	OccurredAt           *time.Time `json:"occurredAt,omitempty"`
	CoalescedTransitions int        `json:"coalescedTransitions,omitempty"`
}

// agentResourceLookup resolves a resource reference to the current resource;
// MonitorAdapter.GetByReference satisfies it.
type agentResourceLookup func(reference string) (*unified.Resource, string, bool)

type agentStatusKey struct {
	orgID      string
	resourceID string
}

// agentStatusWindow is one resource's open coalescing window: the status last
// announced and the latest status change folded since then.
type agentStatusWindow struct {
	announced string
	pending   *AgentEventResourceChangePayload
	folded    int
}

// agentChangeEventProducer turns resource changes the monitor adapters just
// recorded into agent events. It reads nothing back from the change store and
// writes nothing, so it adds no disk load. Alert events publish as recorded;
// the alert engine's own delays and hysteresis already bound them. Status
// changes are coalesced per resource: the first change in a quiet period is
// announced at once, later changes inside the window fold into at most one
// event when it closes, and a flap that ends on the status last announced
// publishes nothing.
type agentChangeEventProducer struct {
	broadcaster *AgentEventBroadcaster
	window      time.Duration
	afterFunc   func(time.Duration, func())

	mu      sync.Mutex
	windows map[agentStatusKey]*agentStatusWindow
}

func newAgentChangeEventProducer(broadcaster *AgentEventBroadcaster, window time.Duration) *agentChangeEventProducer {
	return &agentChangeEventProducer{
		broadcaster: broadcaster,
		window:      window,
		afterFunc: func(d time.Duration, f func()) {
			time.AfterFunc(d, f)
		},
		windows: make(map[agentStatusKey]*agentStatusWindow),
	}
}

// observer returns the change observer for one organization's monitor
// adapter. Events it publishes reach only that organization's subscribers.
func (p *agentChangeEventProducer) observer(orgID string, lookup agentResourceLookup) unified.ResourceChangeObserver {
	orgID = normalizeAgentEventOrgID(orgID)
	return func(change unified.ResourceChange) {
		p.observe(orgID, change, lookup)
	}
}

func (p *agentChangeEventProducer) observe(orgID string, change unified.ResourceChange, lookup agentResourceLookup) {
	if p == nil || p.broadcaster == nil {
		return
	}
	kind, ok := agentEventKindForChange(change)
	if !ok {
		return
	}
	payload := agentResourceChangePayload(change)
	if kind != AgentEventResourceStatusChanged {
		p.publish(orgID, kind, payload, lookup)
		return
	}

	key := agentStatusKey{orgID: orgID, resourceID: payload.ResourceID}
	p.mu.Lock()
	if window, open := p.windows[key]; open {
		window.folded++
		payload.From = window.announced
		window.pending = &payload
		p.mu.Unlock()
		return
	}
	p.openStatusWindowLocked(key, payload.To, lookup)
	p.mu.Unlock()
	p.publish(orgID, kind, payload, lookup)
}

func (p *agentChangeEventProducer) openStatusWindowLocked(key agentStatusKey, announced string, lookup agentResourceLookup) {
	p.windows[key] = &agentStatusWindow{announced: announced}
	p.afterFunc(p.window, func() {
		p.closeStatusWindow(key, lookup)
	})
}

// closeStatusWindow publishes the folded change, if it left the resource in a
// different status from the one last announced, and opens a new window for
// it so continued flapping stays at one event per window.
func (p *agentChangeEventProducer) closeStatusWindow(key agentStatusKey, lookup agentResourceLookup) {
	p.mu.Lock()
	window := p.windows[key]
	delete(p.windows, key)
	if window == nil || window.pending == nil || window.pending.To == window.announced {
		p.mu.Unlock()
		return
	}
	payload := *window.pending
	payload.CoalescedTransitions = window.folded
	p.openStatusWindowLocked(key, payload.To, lookup)
	p.mu.Unlock()
	p.publish(key.orgID, AgentEventResourceStatusChanged, payload, lookup)
}

func (p *agentChangeEventProducer) publish(orgID string, kind AgentEventKind, payload AgentEventResourceChangePayload, lookup agentResourceLookup) {
	if lookup != nil {
		if resource, _, ok := lookup(payload.ResourceID); ok && resource != nil {
			payload.ResourceName = resource.Name
			payload.ResourceType = string(resource.Type)
		}
	}
	p.broadcaster.Publish(AgentEvent{
		Kind:    kind,
		Payload: payload,
		orgID:   orgID,
	})
}

// agentEventKindForChange maps a recorded change to the event agents hear.
// A state_transition counts as a status change only when the status field
// changed: discovery, removal, Docker command and Proxmox lifecycle
// transitions stay in list_recent_changes.
func agentEventKindForChange(change unified.ResourceChange) (AgentEventKind, bool) {
	switch change.Kind {
	case unified.ChangeAlertFired:
		return AgentEventAlertFired, true
	case unified.ChangeAlertResolved:
		return AgentEventAlertResolved, true
	case unified.ChangeStateTransition:
		if changeTouchesField(change, "status") && change.From != change.To {
			return AgentEventResourceStatusChanged, true
		}
	}
	return "", false
}

func changeTouchesField(change unified.ResourceChange, field string) bool {
	switch fields := change.Metadata["changedFields"].(type) {
	case []string:
		for _, candidate := range fields {
			if candidate == field {
				return true
			}
		}
	case []any:
		for _, candidate := range fields {
			if candidate == field {
				return true
			}
		}
	}
	return false
}

func agentResourceChangePayload(change unified.ResourceChange) AgentEventResourceChangePayload {
	payload := AgentEventResourceChangePayload{
		ChangeID:   change.ID,
		ResourceID: unified.CanonicalResourceID(change.ResourceID),
		Kind:       string(change.Kind),
		ObservedAt: change.ObservedAt,
	}
	if change.OccurredAt != nil {
		occurredAt := *change.OccurredAt
		payload.OccurredAt = &occurredAt
	}
	switch change.Kind {
	case unified.ChangeAlertFired, unified.ChangeAlertResolved:
		payload.AlertIdentifier = changeMetadataString(change, unified.MetadataAlertIdentifier)
		payload.AlertType = changeMetadataString(change, unified.MetadataAlertType)
		payload.AlertLevel = changeMetadataString(change, unified.MetadataAlertLevel)
		payload.AlertResolution = changeMetadataString(change, unified.MetadataAlertResolution)
	default:
		payload.From = change.From
		payload.To = change.To
	}
	return payload
}

func changeMetadataString(change unified.ResourceChange, key string) string {
	value, _ := change.Metadata[key].(string)
	return strings.TrimSpace(value)
}

// normalizeAgentEventOrgID maps the legacy empty organization to the default
// one, matching how request contexts name it.
func normalizeAgentEventOrgID(orgID string) string {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return "default"
	}
	return orgID
}

// defaultResourceStatusEventWindow is the manifest-declared coalescing window.
const defaultResourceStatusEventWindow = agentcapabilities.ResourceStatusEventCoalesceWindow
