package agentcapabilities

import "time"

// EventKind is the stable Pulse Intelligence event identifier used by API SSE
// streams and external-agent adapters.
type EventKind string

const (
	// EventKindFindingCreated fires when Patrol records a new finding.
	EventKindFindingCreated EventKind = "finding.created"

	// EventKindApprovalPending fires when a governed action waits on operator
	// approval.
	EventKindApprovalPending EventKind = "approval.pending"

	// EventKindActionCompleted fires when an action audit reaches a terminal
	// state.
	EventKindActionCompleted EventKind = "action.completed"

	// EventKindAlertFired fires when Pulse records an alert firing (or
	// re-firing) on a resource.
	EventKindAlertFired EventKind = "alert.fired"

	// EventKindAlertResolved fires when Pulse records an alert clearing.
	EventKindAlertResolved EventKind = "alert.resolved"

	// EventKindResourceStatusChanged fires when Pulse records a resource's
	// status changing, coalesced per resource so flaps cannot flood the
	// stream.
	EventKindResourceStatusChanged EventKind = "resource.status_changed"

	// EventKindStreamConnected is the one-shot SSE synchronization marker for a
	// newly opened stream.
	EventKindStreamConnected EventKind = "stream.connected"

	// EventKindHeartbeat is the stream-local SSE keepalive.
	EventKindHeartbeat EventKind = "heartbeat"
)

var agentActionableEventKinds = []string{
	string(EventKindFindingCreated),
	string(EventKindApprovalPending),
	string(EventKindActionCompleted),
	string(EventKindAlertFired),
	string(EventKindAlertResolved),
	string(EventKindResourceStatusChanged),
}

// ResourceStatusEventCoalesceWindow bounds resource.status_changed volume: a
// resource's first status change is announced at once, and further changes
// inside the window fold into at most one event at the window's end.
const ResourceStatusEventCoalesceWindow = 30 * time.Second

// AgentActionableEventKinds returns the non-transport event kinds external
// agents should expect to handle as product notifications.
func AgentActionableEventKinds() []string {
	return append([]string(nil), agentActionableEventKinds...)
}

// IsTransportEventKind reports whether an event kind is stream plumbing rather
// than a product notification.
func IsTransportEventKind(kind string) bool {
	switch EventKind(kind) {
	case EventKindStreamConnected, EventKindHeartbeat:
		return true
	default:
		return false
	}
}

// SubscribeEventsDescription returns the manifest-owned description for the
// streaming capability. Event names are built from the shared constants so the
// API manifest, SSE producer, and external-agent adapters cannot drift apart.
func SubscribeEventsDescription() string {
	return "Subscribe to the SSE event stream for real-time notifications: " +
		string(EventKindFindingCreated) + " when a new finding is raised, " +
		string(EventKindApprovalPending) + " when a remediation request enters StatusPending and waits on operator decision, " +
		string(EventKindActionCompleted) + " when an action audit reaches a terminal state (Completed or Failed, including refused-before-dispatch failures with stable error-token prefixes; carries a verification block with the read-after-write probe outcome so agents close the certainty loop without polling /api/actions/{id}), " +
		string(EventKindAlertFired) + " and " + string(EventKindAlertResolved) + " when an alert fires or clears on a resource (alert identifier, type and level), " +
		string(EventKindResourceStatusChanged) + " when a resource's status changes (from/to; further changes within " + ResourceStatusEventCoalesceWindow.String() + " fold into one event carrying coalescedTransitions). Change events carry the canonical resourceId, resource name and type, and the changeId and kind that " + ListRecentChangesCapabilityName + " returns, so agents can read the surrounding history; replayed or imported history is not announced. Stream-local " +
		string(EventKindHeartbeat) + " every 15s. Command fields are redacted for monitoring-read API tokens unless the token also has ai:execute. Long-lived connection; agents listen instead of polling."
}
