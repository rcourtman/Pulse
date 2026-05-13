package unifiedresources

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ResourceCriticality is an operator-set hint about how important a resource
// is to the operator. Patrol uses it as a sort-order tiebreaker on findings
// of equal severity — high-criticality resources surface first. Empty string
// is the default ("no operator-set criticality"), distinct from "medium".
type ResourceCriticality string

const (
	CriticalityHigh   ResourceCriticality = "high"
	CriticalityMedium ResourceCriticality = "medium"
	CriticalityLow    ResourceCriticality = "low"
)

// IsValidCriticality reports whether the value is empty or one of the three
// canonical levels. Empty is valid (operator has not set a hint). Anything
// else is rejected at the API boundary so freeform strings cannot accumulate
// into per-deployment dialects.
func IsValidCriticality(value string) bool {
	switch ResourceCriticality(value) {
	case "", CriticalityHigh, CriticalityMedium, CriticalityLow:
		return true
	}
	return false
}

// ResourceOperatorState captures operator-set per-resource intent that
// modulates Patrol's behavior on findings against this resource. The shape
// is intentionally narrow: every field encodes a specific operator intent
// (intentionally offline, never auto-remediate, maintenance window,
// criticality hint) rather than a freeform metadata bag, so the
// finding-suppression and severity-weighting logic that consumes this state
// has a fixed contract to honor.
//
// All fields are optional; an empty record is equivalent to the default
// "no operator-set state" posture. Persistence stores must treat the
// canonical_id as the primary key and overwrite the entire record on
// SetResourceOperatorState — there is no per-field merge, so callers who
// want to flip a single flag must read-modify-write.
type ResourceOperatorState struct {
	// CanonicalID is the resource identity this state attaches to. Must
	// match the canonical resource ID format used elsewhere in the
	// unified-resources store; the API boundary trims and rejects empty.
	CanonicalID string `json:"canonicalId"`

	// IntentionallyOffline marks the resource as expected-to-be-offline.
	// Findings of the form "resource X is offline" against this resource
	// will be auto-acknowledged with reason=intentionally_offline by the
	// Patrol findings store. Other finding categories (high CPU, disk
	// pressure on a still-mounted volume, etc.) are unaffected.
	IntentionallyOffline bool `json:"intentionallyOffline"`

	// NeverAutoRemediate forbids Patrol from dispatching automated fixes
	// against this resource even under approval policy. The action broker
	// must refuse a dispatch targeting this resource with a stable error
	// ("resource_remediation_locked") rather than silently degrading. The
	// operator must clear the flag to allow remediation.
	NeverAutoRemediate bool `json:"neverAutoRemediate"`

	// MaintenanceStartAt and MaintenanceEndAt define a time-bounded
	// suppression window. When now is within [start, end), all findings
	// raised against this resource get auto-acknowledged with
	// reason=maintenance. Both values must be set together; either alone
	// is treated as no window. End must be strictly after Start.
	MaintenanceStartAt *time.Time `json:"maintenanceStartAt,omitempty"`
	MaintenanceEndAt   *time.Time `json:"maintenanceEndAt,omitempty"`

	// MaintenanceReason is freeform operator note attached to the window
	// for audit / Assistant context. Surfaced verbatim in the
	// auto-acknowledge note so the operator can see WHY future findings
	// were quiet during the window.
	MaintenanceReason string `json:"maintenanceReason,omitempty"`

	// Criticality is an operator hint that affects finding sort order on
	// the Patrol surface. Empty = default; CriticalityHigh promotes
	// findings on this resource above same-severity peers; CriticalityLow
	// demotes them. Severity itself is not modified — escalation paths
	// stay deterministic.
	Criticality ResourceCriticality `json:"criticality,omitempty"`

	// Note is a freeform operator explanation surfaced alongside the
	// state on the resource detail surface. Distinct from
	// MaintenanceReason which is window-scoped.
	Note string `json:"note,omitempty"`

	// SetAt and SetBy track who last touched the state for audit. SetAt
	// must be populated on every Set; SetBy may be empty when the state
	// was set by a system path (e.g. a maintenance window completing
	// itself).
	SetAt time.Time `json:"setAt"`
	SetBy string    `json:"setBy,omitempty"`
}

// IsEmpty reports whether the state carries no operator intent — every
// field is at its zero value. Stores may treat an IsEmpty record as
// equivalent to "no entry" for the purpose of the GET API surface, though
// the persistence layer MAY keep an audit row to track that the operator
// explicitly cleared the state.
func (s ResourceOperatorState) IsEmpty() bool {
	return !s.IntentionallyOffline &&
		!s.NeverAutoRemediate &&
		s.MaintenanceStartAt == nil &&
		s.MaintenanceEndAt == nil &&
		strings.TrimSpace(s.MaintenanceReason) == "" &&
		s.Criticality == "" &&
		strings.TrimSpace(s.Note) == ""
}

// IsInMaintenanceAt reports whether `now` falls within the configured
// maintenance window. Returns false when no window is configured, when only
// one of start/end is set (treated as no window), or when end <= start.
func (s ResourceOperatorState) IsInMaintenanceAt(now time.Time) bool {
	if s.MaintenanceStartAt == nil || s.MaintenanceEndAt == nil {
		return false
	}
	if !s.MaintenanceEndAt.After(*s.MaintenanceStartAt) {
		return false
	}
	if now.Before(*s.MaintenanceStartAt) {
		return false
	}
	if !now.Before(*s.MaintenanceEndAt) {
		return false
	}
	return true
}

// ErrResourceOperatorStateInvalid is returned by stores when the supplied
// state fails validation (empty canonical ID, malformed maintenance window,
// unknown criticality value). The action broker translates it into a 400
// at the API boundary; the audit path treats it as a refused write.
var ErrResourceOperatorStateInvalid = errors.New("resource_operator_state_invalid")

// ValidateResourceOperatorState applies the contract checks the Set path
// must enforce before persisting. Returns nil on a valid record, an
// ErrResourceOperatorStateInvalid-wrapped error on a violation. Validation
// is structural only — operator-set meaning (was the maintenance window
// actually intended? is the note correct?) is the operator's call.
func ValidateResourceOperatorState(state ResourceOperatorState) error {
	if strings.TrimSpace(state.CanonicalID) == "" {
		return fmt.Errorf("%w: canonical_id is required", ErrResourceOperatorStateInvalid)
	}
	if !IsValidCriticality(string(state.Criticality)) {
		return fmt.Errorf("%w: criticality %q is not one of (high, medium, low, empty)", ErrResourceOperatorStateInvalid, state.Criticality)
	}
	startSet := state.MaintenanceStartAt != nil
	endSet := state.MaintenanceEndAt != nil
	if startSet != endSet {
		return fmt.Errorf("%w: maintenance window requires both start_at and end_at", ErrResourceOperatorStateInvalid)
	}
	if startSet && endSet {
		if !state.MaintenanceEndAt.After(*state.MaintenanceStartAt) {
			return fmt.Errorf("%w: maintenance end_at must be strictly after start_at", ErrResourceOperatorStateInvalid)
		}
	}
	return nil
}

// NormalizeResourceOperatorState applies the canonical trim / default
// behavior expected before persisting. Returns a copy with whitespace
// trimmed on string fields and Criticality coerced to lower-case. Does
// NOT validate — call ValidateResourceOperatorState afterward.
func NormalizeResourceOperatorState(state ResourceOperatorState) ResourceOperatorState {
	state.CanonicalID = strings.TrimSpace(state.CanonicalID)
	state.MaintenanceReason = strings.TrimSpace(state.MaintenanceReason)
	state.Note = strings.TrimSpace(state.Note)
	state.SetBy = strings.TrimSpace(state.SetBy)
	state.Criticality = ResourceCriticality(strings.ToLower(strings.TrimSpace(string(state.Criticality))))
	return state
}

const (
	MaintenanceWindowLifecycleEventScheduled = "maintenance_window_scheduled"
	MaintenanceWindowLifecycleEventUpdated   = "maintenance_window_updated"
	MaintenanceWindowLifecycleEventCleared   = "maintenance_window_cleared"

	resourceOperatorStateSourceAdapter ChangeSourceAdapter = "operator_state"
)

type maintenanceWindowLifecycleSnapshot struct {
	start  time.Time
	end    time.Time
	reason string
}

// BuildMaintenanceWindowLifecycleChange returns the canonical resource
// timeline record for a maintenance-window lifecycle transition. It is
// intentionally scoped to the maintenance window fields; other
// operator-state flags have their own product meaning and must not be
// folded into this lifecycle evidence.
func BuildMaintenanceWindowLifecycleChange(previous ResourceOperatorState, previousFound bool, current ResourceOperatorState, currentFound bool, observedAt time.Time, actor string) (ResourceChange, bool) {
	if !previousFound {
		previous = ResourceOperatorState{}
	}
	if !currentFound {
		current = ResourceOperatorState{CanonicalID: previous.CanonicalID}
	}
	canonicalID := CanonicalResourceID(current.CanonicalID)
	if canonicalID == "" {
		canonicalID = CanonicalResourceID(previous.CanonicalID)
	}
	if canonicalID == "" {
		return ResourceChange{}, false
	}

	before, beforeOK := maintenanceWindowSnapshot(previous)
	after, afterOK := maintenanceWindowSnapshot(current)

	event := ""
	switch {
	case !beforeOK && afterOK:
		event = MaintenanceWindowLifecycleEventScheduled
	case beforeOK && afterOK && !before.equal(after):
		event = MaintenanceWindowLifecycleEventUpdated
	case beforeOK && !afterOK:
		event = MaintenanceWindowLifecycleEventCleared
	default:
		return ResourceChange{}, false
	}

	if observedAt.IsZero() {
		observedAt = maintenanceWindowObservedAt(previous, previousFound, current, currentFound)
	} else {
		observedAt = observedAt.UTC()
	}
	actor = strings.TrimSpace(actor)
	if actor == "" && currentFound {
		actor = strings.TrimSpace(current.SetBy)
	}
	if actor == "" && previousFound {
		actor = strings.TrimSpace(previous.SetBy)
	}

	metadata := map[string]any{
		"activityType":        event,
		"operatorStateChange": "maintenance_window_lifecycle",
	}
	if beforeOK {
		metadata["previousMaintenanceStartAt"] = before.start.UTC().Format(time.RFC3339)
		metadata["previousMaintenanceEndAt"] = before.end.UTC().Format(time.RFC3339)
		if before.reason != "" {
			metadata["previousMaintenanceReason"] = before.reason
		}
	}
	if afterOK {
		metadata["maintenanceStartAt"] = after.start.UTC().Format(time.RFC3339)
		metadata["maintenanceEndAt"] = after.end.UTC().Format(time.RFC3339)
		if after.reason != "" {
			metadata["maintenanceReason"] = after.reason
		}
	}

	return ResourceChange{
		ID:            resourceChangeID("resource-operator-state", canonicalID, event, observedAt),
		ObservedAt:    observedAt,
		ResourceID:    canonicalID,
		Kind:          ChangeActivity,
		From:          maintenanceWindowSummary(before, beforeOK),
		To:            maintenanceWindowSummary(after, afterOK),
		SourceType:    SourceUserAction,
		SourceAdapter: resourceOperatorStateSourceAdapter,
		Confidence:    ConfidenceHigh,
		Actor:         actor,
		Reason:        maintenanceWindowLifecycleReason(event),
		Metadata:      metadata,
	}, true
}

func maintenanceWindowSnapshot(state ResourceOperatorState) (maintenanceWindowLifecycleSnapshot, bool) {
	if state.MaintenanceStartAt == nil || state.MaintenanceEndAt == nil {
		return maintenanceWindowLifecycleSnapshot{}, false
	}
	return maintenanceWindowLifecycleSnapshot{
		start:  state.MaintenanceStartAt.UTC(),
		end:    state.MaintenanceEndAt.UTC(),
		reason: strings.TrimSpace(state.MaintenanceReason),
	}, true
}

func (s maintenanceWindowLifecycleSnapshot) equal(other maintenanceWindowLifecycleSnapshot) bool {
	return s.start.Equal(other.start) && s.end.Equal(other.end) && s.reason == other.reason
}

func maintenanceWindowObservedAt(previous ResourceOperatorState, previousFound bool, current ResourceOperatorState, currentFound bool) time.Time {
	if currentFound && !current.SetAt.IsZero() {
		return current.SetAt.UTC()
	}
	if previousFound && !previous.SetAt.IsZero() {
		return previous.SetAt.UTC()
	}
	return time.Now().UTC()
}

func maintenanceWindowSummary(window maintenanceWindowLifecycleSnapshot, ok bool) string {
	if !ok {
		return "no maintenance window"
	}
	summary := window.start.UTC().Format(time.RFC3339) + " to " + window.end.UTC().Format(time.RFC3339)
	if window.reason != "" {
		summary += " (" + window.reason + ")"
	}
	return summary
}

func maintenanceWindowLifecycleReason(event string) string {
	switch event {
	case MaintenanceWindowLifecycleEventScheduled:
		return "Maintenance window scheduled"
	case MaintenanceWindowLifecycleEventUpdated:
		return "Maintenance window updated"
	case MaintenanceWindowLifecycleEventCleared:
		return "Maintenance window cleared"
	default:
		return "Maintenance window lifecycle changed"
	}
}

func resourceChangeID(prefix, canonicalID, event string, observedAt time.Time) string {
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	return fmt.Sprintf("%s:%s:%s:%d", prefix, sanitizeResourceChangeIDComponent(canonicalID), sanitizeResourceChangeIDComponent(event), observedAt.UTC().UnixNano())
}

func sanitizeResourceChangeIDComponent(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case (r >= 'a' && r <= 'z'), (r >= 'A' && r <= 'Z'), (r >= '0' && r <= '9'):
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
