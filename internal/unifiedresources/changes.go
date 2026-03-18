package unifiedresources

import (
	"time"
)

// ChangeConfidence represents the certainty that a change actually occurred.
type ChangeConfidence string

const (
	ConfidenceHigh   ChangeConfidence = "high"
	ConfidenceMedium ChangeConfidence = "medium"
	ConfidenceLow    ChangeConfidence = "low"
)

// ChangeKind defines the specific type of deterministic change.
type ChangeKind string

const (
	ChangeStateTransition ChangeKind = "state_transition"
	ChangeRestart         ChangeKind = "restart"
	ChangeConfigUpdate    ChangeKind = "config_update"
	ChangeAnomaly         ChangeKind = "metric_anomaly"
	ChangeRelationship    ChangeKind = "relationship_change"
	ChangeCapability      ChangeKind = "capability_change"
)

// ChangeSourceType defines the high-level origin of a change.
type ChangeSourceType string

const (
	SourcePlatformEvent ChangeSourceType = "platform_event"
	SourcePulseDiff     ChangeSourceType = "pulse_diff"
	SourceHeuristic     ChangeSourceType = "heuristic"
	SourceUserAction    ChangeSourceType = "user_action"
	SourceAgentAction   ChangeSourceType = "agent_action"
)

// ChangeSourceAdapter specifies the specific integration responsible.
type ChangeSourceAdapter string

const (
	AdapterDocker   ChangeSourceAdapter = "docker_adapter"
	AdapterProxmox  ChangeSourceAdapter = "proxmox_adapter"
	AdapterTrueNAS  ChangeSourceAdapter = "truenas_adapter"
	AdapterOpsAgent ChangeSourceAdapter = "agent:ops-helper"
)

// ResourceChangeFilters narrows the resource timeline to specific change kinds
// and source origins while preserving the canonical change record shape.
type ResourceChangeFilters struct {
	Kinds          []ChangeKind          `json:"kinds,omitempty"`
	SourceTypes    []ChangeSourceType    `json:"sourceTypes,omitempty"`
	SourceAdapters []ChangeSourceAdapter `json:"sourceAdapters,omitempty"`
}

func (filters ResourceChangeFilters) matches(change ResourceChange) bool {
	if len(filters.Kinds) > 0 {
		match := false
		for _, kind := range filters.Kinds {
			if kind == change.Kind {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	if len(filters.SourceTypes) > 0 {
		match := false
		for _, sourceType := range filters.SourceTypes {
			if sourceType == change.SourceType {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	if len(filters.SourceAdapters) > 0 {
		match := false
		for _, sourceAdapter := range filters.SourceAdapters {
			if sourceAdapter == change.SourceAdapter {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	return true
}

// ResourceChange represents a deterministic point-in-time state transition,
// event, or metadata change tracked by Pulse, forming the historical "Court Record".
type ResourceChange struct {
	ID               string              `json:"id"`
	ObservedAt       time.Time           `json:"observedAt"`
	OccurredAt       *time.Time          `json:"occurredAt,omitempty"`
	ResourceID       string              `json:"resourceId"`
	Kind             ChangeKind          `json:"kind"`
	From             string              `json:"from,omitempty"`
	To               string              `json:"to,omitempty"`
	SourceType       ChangeSourceType    `json:"sourceType"`
	SourceAdapter    ChangeSourceAdapter `json:"sourceAdapter,omitempty"`
	Confidence       ChangeConfidence    `json:"confidence"`
	Actor            string              `json:"actor,omitempty"`
	RelatedResources []string            `json:"relatedResources,omitempty"`
	Reason           string              `json:"reason,omitempty"`
	Metadata         map[string]any      `json:"metadata,omitempty"`
}
