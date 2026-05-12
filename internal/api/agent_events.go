package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// AgentEventKind is the agent-stable event-type identifier. Values are
// closed and snake_case-with-dot to match the convention agents use
// for tool/event names. New kinds extend the set; existing kinds must
// not be renamed (agents branch on them).
type AgentEventKind string

const (
	// AgentEventFindingCreated fires when a new finding is recorded
	// against any resource. Payload carries the finding's canonical
	// id, resource id, severity, title, and category — enough for an
	// agent to decide whether to fetch the full situated context.
	AgentEventFindingCreated AgentEventKind = "finding.created"

	// AgentEventApprovalPending fires when a remediation request enters
	// StatusPending and is waiting on an operator (or operator-acting
	// agent) to approve, reject, or cancel. Payload carries the
	// approval id, the resource it targets, risk, who requested it,
	// and when it expires. Raw command text is included only for
	// action-capable API tokens; monitoring-read subscribers receive
	// a redacted doorbell and can fetch governed detail through the
	// approval/action surfaces when authorized.
	AgentEventApprovalPending AgentEventKind = "approval.pending"

	// AgentEventActionCompleted fires when an action audit reaches a
	// terminal state — Completed (executed and verified) or Failed
	// (refused before dispatch with a stable error-token prefix, or
	// errored during execution). Payload carries the canonical action
	// id, the resource it targeted, the capability, the
	// redacted-or-full command, the outcome (success bool + optional
	// error message), who acted, and the completion timestamp —
	// enough for an agent to close the dispatch loop without polling
	// the audit endpoint.
	AgentEventActionCompleted AgentEventKind = "action.completed"

	// AgentEventHeartbeat is a keepalive that fires at a fixed
	// interval. Agents that hold an open SSE connection use it to
	// confirm the stream is healthy without waiting for a real
	// event.
	AgentEventHeartbeat AgentEventKind = "heartbeat"
)

// AgentEvent is the agent-stable event envelope. Carries the kind, a
// payload appropriate to the kind, and a timestamp + monotonic id
// agents use to dedupe and resume after disconnects.
type AgentEvent struct {
	Kind    AgentEventKind `json:"kind"`
	ID      uint64         `json:"id"`
	At      time.Time      `json:"at"`
	Payload any            `json:"payload,omitempty"`
}

// AgentEventFindingCreatedPayload is the payload shape for
// finding.created events. Narrow on purpose — the full finding
// surface is reachable via /api/agent/resource-context/{id} or the
// existing finding endpoints; the event is a notification, not a
// dump.
type AgentEventFindingCreatedPayload struct {
	FindingID    string `json:"findingId"`
	ResourceID   string `json:"resourceId"`
	ResourceName string `json:"resourceName,omitempty"`
	Severity     string `json:"severity"`
	Title        string `json:"title"`
	Category     string `json:"category,omitempty"`
}

// AgentEventApprovalPendingPayload is the payload shape for
// approval.pending events. Carries the agent-decision-relevant
// fields: which approval, against which resource, what risk, who
// requested it, and when it expires. Command is present only when
// the caller's token also has action execution authority; otherwise
// CommandRedacted is true. Full request detail (preflight, plan,
// raw context) stays behind the existing /api/approvals/{id}
// endpoint — the event is a doorbell, not the approval itself.
type AgentEventApprovalPendingPayload struct {
	ApprovalID      string    `json:"approvalId"`
	ResourceID      string    `json:"resourceId,omitempty"`
	TargetType      string    `json:"targetType,omitempty"`
	TargetID        string    `json:"targetId,omitempty"`
	TargetName      string    `json:"targetName,omitempty"`
	Command         string    `json:"command,omitempty"`
	CommandRedacted bool      `json:"commandRedacted,omitempty"`
	RiskLevel       string    `json:"riskLevel"`
	RequestedBy     string    `json:"requestedBy,omitempty"`
	RequestedAt     time.Time `json:"requestedAt"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

// AgentEventActionCompletedPayload is the payload shape for
// action.completed events. Carries the dispatch outcome agents
// branch on: success/failure, the resource the action ran against,
// the canonical capability + command, who acted, any error
// message, and the agent-stable verification projection
// (read-after-write probe outcome) so the agent can close the
// "did it actually work?" loop without a follow-up audit fetch.
// Refused-before-dispatch failures carry the stable error-token
// prefix (`plan_drift:`, `resource_remediation_locked:`) in
// ErrorMessage so agents branch on the prefix rather than parsing
// human text; refused dispatches have no Verification by
// definition (verification only runs after a successful execute).
// Full audit detail (lifecycle events, preflight, verification
// stdout) stays behind the existing /api/actions/{id} endpoint.
type AgentEventActionCompletedPayload struct {
	ActionID        string                           `json:"actionId"`
	ResourceID      string                           `json:"resourceId,omitempty"`
	CapabilityName  string                           `json:"capabilityName,omitempty"`
	Command         string                           `json:"command,omitempty"`
	CommandRedacted bool                             `json:"commandRedacted,omitempty"`
	State           string                           `json:"state"`
	Success         bool                             `json:"success"`
	ErrorMessage    string                           `json:"errorMessage,omitempty"`
	Verification    *AgentResourceActionVerification `json:"verification,omitempty"`
	RequestedBy     string                           `json:"requestedBy,omitempty"`
	CompletedAt     time.Time                        `json:"completedAt"`
}

// AgentEventBroadcaster is a thread-safe pub/sub for AgentEvents. A
// single broadcaster instance lives at the api-package level; the
// findings store and action broker publish to it, the SSE handler
// subscribes from it. Buffered per-subscriber channel keeps slow
// consumers from stalling the publishers; if a subscriber's buffer
// fills, the broadcaster drops events for that subscriber rather
// than blocking the global publish path.
type AgentEventBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[uint64]chan AgentEvent
	nextSubID   uint64
	nextEventID uint64
	bufSize     int
}

// NewAgentEventBroadcaster creates a broadcaster with a per-subscriber
// buffer of 64 events. Picked to be generous enough for a slow agent
// to catch up across a brief stall without losing events, but small
// enough that runaway publishers don't pin large amounts of memory
// per subscriber.
func NewAgentEventBroadcaster() *AgentEventBroadcaster {
	return &AgentEventBroadcaster{
		subscribers: make(map[uint64]chan AgentEvent),
		bufSize:     64,
	}
}

// Subscribe registers a new subscriber and returns its channel + an
// unsubscribe function the caller MUST call to release the
// subscription. The channel is closed by the broadcaster only when
// the broadcaster itself is shutting down; ordinary unsubscription
// stops sending without closing.
func (b *AgentEventBroadcaster) Subscribe() (<-chan AgentEvent, func()) {
	b.mu.Lock()
	id := b.nextSubID
	b.nextSubID++
	ch := make(chan AgentEvent, b.bufSize)
	b.subscribers[id] = ch
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		if existing, ok := b.subscribers[id]; ok {
			delete(b.subscribers, id)
			close(existing)
		}
		b.mu.Unlock()
	}
	return ch, unsubscribe
}

// Publish fans out an event to every current subscriber. The event's
// ID is assigned by the broadcaster (monotonic, never reused);
// callers populate Kind, At, and Payload. Slow subscribers (full
// channel) get the event dropped rather than blocking the publish
// path — agents that need replay must use a different mechanism
// (none currently exposed; future event log can layer on top).
func (b *AgentEventBroadcaster) Publish(event AgentEvent) {
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	event.ID = atomic.AddUint64(&b.nextEventID, 1)

	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// Full buffer; drop and log so we can spot subscribers
			// that are consistently lagging. Dropping is the right
			// call for an agent-paradigm stream — better to keep
			// publishing than to block the publish path.
			log.Warn().
				Str("kind", string(event.Kind)).
				Uint64("event_id", event.ID).
				Msg("Agent events: dropping event for slow subscriber")
		}
	}
}

// SubscriberCount returns the number of current subscribers. Used by
// tests; not exposed on the wire.
func (b *AgentEventBroadcaster) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

// HandleAgentEvents serves `GET /api/agent/events` — the SSE stream
// agents subscribe to for real-time notifications. The connection
// stays open until the client disconnects or the request context
// cancels. Heartbeats fire every 15s so an agent holding an idle
// connection can confirm the stream is alive.
//
// SSE chosen over WebSocket because it's simpler (one-way, no
// frame-handling), works through every HTTP proxy without
// special-casing, and matches the existing deploy_handlers pattern.
// Agents that need bidirectional comms can call the REST endpoints
// in the capabilities manifest in parallel.
func (b *AgentEventBroadcaster) HandleAgentEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Disable buffering by reverse proxies (nginx in particular)
	// that otherwise hold the response until enough bytes accumulate.
	w.Header().Set("X-Accel-Buffering", "no")

	events, unsubscribe := b.Subscribe()
	defer unsubscribe()

	// Initial sync event so agents that just connected can confirm
	// the stream is open before the first real event lands. This is
	// distinct from heartbeat: the connected event fires once at
	// subscribe time, the heartbeat fires periodically.
	writeAgentSSEEvent(w, AgentEvent{
		Kind: "stream.connected",
		At:   time.Now().UTC(),
	})
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, open := <-events:
			if !open {
				return
			}
			writeAgentSSEEvent(w, redactAgentEventCommandsForRequest(event, r))
			flusher.Flush()
		case <-heartbeat.C:
			b.Publish(AgentEvent{Kind: AgentEventHeartbeat})
		}
	}
}

// writeAgentSSEEvent serializes an AgentEvent as a Server-Sent Event
// frame. Uses the `event:` field so agents can dispatch via
// EventSource.addEventListener(kind, ...) rather than parsing every
// event manually.
func writeAgentSSEEvent(w http.ResponseWriter, event AgentEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Error().Err(err).Str("kind", string(event.Kind)).Msg("Agent events: failed to marshal event")
		return
	}
	if event.Kind != "" {
		fmt.Fprintf(w, "event: %s\n", event.Kind)
	}
	if event.ID != 0 {
		fmt.Fprintf(w, "id: %d\n", event.ID)
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
}

// PublishFindingCreated is a convenience publisher for the most
// common event kind. The findings runtime calls this when a new
// finding is added; the implementation enriches the event with the
// canonical timestamp and a reference to the broadcaster, keeping
// the call site free of broadcaster plumbing.
func (b *AgentEventBroadcaster) PublishFindingCreated(payload AgentEventFindingCreatedPayload) {
	b.Publish(AgentEvent{
		Kind:    AgentEventFindingCreated,
		Payload: payload,
	})
}

// PublishApprovalPending is the convenience publisher the approval
// store's post-create hook routes through. Wraps the payload in the
// canonical envelope and forwards to Publish — the broadcaster
// stamps the timestamp and event id.
func (b *AgentEventBroadcaster) PublishApprovalPending(payload AgentEventApprovalPendingPayload) {
	b.Publish(AgentEvent{
		Kind:    AgentEventApprovalPending,
		Payload: payload,
	})
}

// PublishActionCompleted is the convenience publisher the executor's
// post-completion hook routes through. Wraps the payload in the
// canonical envelope and forwards to Publish — the broadcaster
// stamps the timestamp and event id.
func (b *AgentEventBroadcaster) PublishActionCompleted(payload AgentEventActionCompletedPayload) {
	b.Publish(AgentEvent{
		Kind:    AgentEventActionCompleted,
		Payload: payload,
	})
}

// AgentEventBroadcasterContextKey is the context key used to plumb
// the broadcaster into request-scoped middleware where needed. Most
// call sites don't need this — they hold a direct reference — but
// keeping it here makes the dependency explicit.
type agentEventBroadcasterContextKey struct{}

// ContextWithAgentEventBroadcaster attaches a broadcaster to a
// context. Used by integration tests that need to inject a fake
// broadcaster.
func ContextWithAgentEventBroadcaster(ctx context.Context, b *AgentEventBroadcaster) context.Context {
	return context.WithValue(ctx, agentEventBroadcasterContextKey{}, b)
}
