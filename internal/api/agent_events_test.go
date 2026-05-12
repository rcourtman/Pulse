package api

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestAgentEventBroadcaster_SubscribeReceivesPublishedEvents(t *testing.T) {
	b := NewAgentEventBroadcaster()
	events, unsubscribe := b.Subscribe()
	defer unsubscribe()

	b.PublishFindingCreated(AgentEventFindingCreatedPayload{
		FindingID:  "f1",
		ResourceID: "vm:101",
		Severity:   "warning",
		Title:      "CPU saturated",
	})

	select {
	case event := <-events:
		if event.Kind != AgentEventFindingCreated {
			t.Errorf("kind: got %q want %q", event.Kind, AgentEventFindingCreated)
		}
		if event.ID == 0 {
			t.Error("event ID must be assigned by the broadcaster (monotonic)")
		}
		if event.At.IsZero() {
			t.Error("event timestamp must be populated")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestAgentEventBroadcaster_UnsubscribeStopsDelivery(t *testing.T) {
	b := NewAgentEventBroadcaster()
	events, unsubscribe := b.Subscribe()

	unsubscribe()

	// After unsubscribe the channel must be closed; receive should
	// drain whatever was buffered then return zero-value with !ok.
	b.PublishFindingCreated(AgentEventFindingCreatedPayload{FindingID: "f-after"})

	select {
	case _, open := <-events:
		if open {
			t.Error("channel must be closed after unsubscribe; agents that resubscribe should get a fresh channel")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel close")
	}
}

func TestAgentEventBroadcaster_FanOutToMultipleSubscribers(t *testing.T) {
	b := NewAgentEventBroadcaster()
	a1, unsub1 := b.Subscribe()
	defer unsub1()
	a2, unsub2 := b.Subscribe()
	defer unsub2()

	b.PublishFindingCreated(AgentEventFindingCreatedPayload{FindingID: "fan-out"})

	deadline := time.After(time.Second)
	for _, ch := range []<-chan AgentEvent{a1, a2} {
		select {
		case event := <-ch:
			if event.Payload == nil {
				t.Error("event payload must round-trip to all subscribers")
			}
		case <-deadline:
			t.Fatal("a subscriber did not receive the published event")
		}
	}
}

func TestAgentEventBroadcaster_DropsForSlowSubscriber(t *testing.T) {
	// Slow consumers that don't drain their channel must not stall
	// the publish path. Fill the buffer past capacity and verify
	// publishes don't block.
	b := NewAgentEventBroadcaster()
	_, unsub := b.Subscribe() // never read
	defer unsub()

	done := make(chan struct{})
	go func() {
		// Publish more than the buffer's bufSize. The publish must
		// not block; if it did the goroutine would not finish and
		// the test would time out.
		for i := 0; i < 200; i++ {
			b.PublishFindingCreated(AgentEventFindingCreatedPayload{FindingID: "spam"})
		}
		close(done)
	}()

	select {
	case <-done:
		// Pass: publishes did not block on the slow subscriber.
	case <-time.After(2 * time.Second):
		t.Fatal("publish path stalled on a slow subscriber; broadcaster must drop rather than block")
	}
}

func TestAgentEventBroadcaster_AssignsMonotonicIDs(t *testing.T) {
	b := NewAgentEventBroadcaster()
	events, unsub := b.Subscribe()
	defer unsub()

	for i := 0; i < 3; i++ {
		b.PublishFindingCreated(AgentEventFindingCreatedPayload{FindingID: "mono"})
	}

	var ids []uint64
	for i := 0; i < 3; i++ {
		select {
		case event := <-events:
			ids = append(ids, event.ID)
		case <-time.After(time.Second):
			t.Fatalf("timed out at i=%d", i)
		}
	}
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Errorf("event IDs must be strictly monotonic; got %v", ids)
			break
		}
	}
}

func TestHandleAgentEvents_RejectsNonGet(t *testing.T) {
	b := NewAgentEventBroadcaster()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/agent/events", nil)
	b.HandleAgentEvents(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405; got %d", rec.Code)
	}
}

func TestHandleAgentEvents_StreamsConnectedAndPublishedEvents(t *testing.T) {
	b := NewAgentEventBroadcaster()

	server := httptest.NewServer(http.HandlerFunc(b.HandleAgentEvents))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	// Pin the streaming-response contract: text/event-stream content
	// type, no-cache, and X-Accel-Buffering off so reverse proxies
	// don't buffer the response.
	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type: got %q want text/event-stream", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control: got %q want no-cache", got)
	}
	if got := resp.Header.Get("X-Accel-Buffering"); got != "no" {
		t.Errorf("X-Accel-Buffering must be 'no' so nginx doesn't buffer; got %q", got)
	}

	// Wait for at least one subscriber so the publish below
	// definitely lands; the stream-connected event should have
	// already been written.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && b.SubscriberCount() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if b.SubscriberCount() == 0 {
		t.Fatal("no subscriber registered after handler started")
	}

	// Publish a finding event so the read loop sees content beyond
	// the connected event.
	b.PublishFindingCreated(AgentEventFindingCreatedPayload{
		FindingID: "f-stream",
		Severity:  "critical",
		Title:     "Streamed",
	})

	// Read the response in a goroutine; close the context after we've
	// seen what we expect so the handler returns and the test does
	// not hang on the open stream.
	got := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		var collected strings.Builder
		for scanner.Scan() {
			collected.WriteString(scanner.Text())
			collected.WriteString("\n")
			text := collected.String()
			if strings.Contains(text, "\"kind\":\"finding.created\"") {
				got <- text
				return
			}
		}
	}()

	select {
	case payload := <-got:
		if !strings.Contains(payload, "stream.connected") {
			t.Error("expected stream.connected event before any published events")
		}
		if !strings.Contains(payload, "f-stream") {
			t.Error("published finding payload must reach the wire")
		}
		// Cancel context so handler exits cleanly.
		cancel()
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for finding.created event on the stream")
	}
}

func TestHandleAgentEvents_HeartbeatsAreStreamLocal(t *testing.T) {
	b := NewAgentEventBroadcaster()
	b.heartbeatInterval = 20 * time.Millisecond

	otherEvents, unsubscribe := b.Subscribe()
	defer unsubscribe()

	server := httptest.NewServer(http.HandlerFunc(b.HandleAgentEvents))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && b.SubscriberCount() < 2 {
		time.Sleep(5 * time.Millisecond)
	}
	if b.SubscriberCount() < 2 {
		t.Fatalf("subscriber count = %d; want at least 2", b.SubscriberCount())
	}

	heartbeatSeen := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		var collected strings.Builder
		for scanner.Scan() {
			line := scanner.Text()
			collected.WriteString(line)
			collected.WriteString("\n")
			if strings.Contains(line, "\"kind\":\"heartbeat\"") {
				heartbeatSeen <- collected.String()
				return
			}
		}
	}()

	select {
	case payload := <-heartbeatSeen:
		if !strings.Contains(payload, "event: heartbeat") {
			t.Fatalf("heartbeat must be serialized as an SSE event; payload=%s", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stream-local heartbeat")
	}

	select {
	case event := <-otherEvents:
		t.Fatalf("stream-local heartbeat leaked to another subscriber: %+v", event)
	case <-time.After(3 * b.heartbeatInterval):
		// Pass: the direct subscriber did not receive the handler's
		// heartbeat. Real events still fan out through Publish.
	}
}

// nonFlushingResponseWriter is a ResponseWriter that explicitly does
// NOT implement http.Flusher, so we can verify the SSE handler
// degrades cleanly on writers that can't stream. Standard
// httptest.ResponseRecorder satisfies Flusher (as a no-op) so it
// can't be used to test this branch.
type nonFlushingResponseWriter struct {
	header     http.Header
	body       strings.Builder
	statusCode int
}

func newNonFlushingResponseWriter() *nonFlushingResponseWriter {
	return &nonFlushingResponseWriter{header: make(http.Header), statusCode: http.StatusOK}
}

func (w *nonFlushingResponseWriter) Header() http.Header  { return w.header }
func (w *nonFlushingResponseWriter) WriteHeader(code int) { w.statusCode = code }
func (w *nonFlushingResponseWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func TestHandleAgentEvents_RequiresFlusher(t *testing.T) {
	// Without a Flusher, SSE responses would buffer indefinitely on
	// the server before reaching the agent. The handler must refuse
	// rather than open a stream that never delivers.
	b := NewAgentEventBroadcaster()
	w := newNonFlushingResponseWriter()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/events", nil)
	b.HandleAgentEvents(w, req)
	if w.statusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 when ResponseWriter does not implement Flusher; got %d", w.statusCode)
	}
}

func TestAgentEventBroadcaster_PublishApprovalPendingDelivers(t *testing.T) {
	// approval.pending is the second event kind the broadcaster
	// publishes (after finding.created). Pin the contract: kind name,
	// payload round-trip with the agent-decision-relevant fields,
	// monotonic id assignment shared across kinds.
	b := NewAgentEventBroadcaster()
	events, unsub := b.Subscribe()
	defer unsub()

	expires := time.Now().Add(5 * time.Minute).UTC()
	requested := time.Now().UTC()
	b.PublishApprovalPending(AgentEventApprovalPendingPayload{
		ApprovalID:  "appr-1",
		ResourceID:  "container:web-1",
		TargetType:  "container",
		TargetID:    "web-1",
		TargetName:  "web-1",
		Command:     "docker restart web-1",
		RiskLevel:   "medium",
		RequestedBy: "ai:patrol",
		RequestedAt: requested,
		ExpiresAt:   expires,
	})

	select {
	case event := <-events:
		if event.Kind != AgentEventApprovalPending {
			t.Fatalf("kind: got %q want %q", event.Kind, AgentEventApprovalPending)
		}
		if event.ID == 0 {
			t.Error("event id must be assigned")
		}
		if event.At.IsZero() {
			t.Error("event timestamp must be populated")
		}
		payload, ok := event.Payload.(AgentEventApprovalPendingPayload)
		if !ok {
			t.Fatalf("payload type: got %T want AgentEventApprovalPendingPayload", event.Payload)
		}
		if payload.ApprovalID != "appr-1" {
			t.Errorf("ApprovalID: got %q want %q", payload.ApprovalID, "appr-1")
		}
		if payload.ResourceID != "container:web-1" {
			t.Errorf("ResourceID: got %q want %q", payload.ResourceID, "container:web-1")
		}
		if payload.Command != "docker restart web-1" {
			t.Errorf("Command did not round-trip: got %q", payload.Command)
		}
		if payload.RiskLevel != "medium" {
			t.Errorf("RiskLevel did not round-trip: got %q", payload.RiskLevel)
		}
		if !payload.ExpiresAt.Equal(expires) {
			t.Errorf("ExpiresAt did not round-trip: got %v want %v", payload.ExpiresAt, expires)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for approval.pending event")
	}
}

func TestAgentEventApprovalPendingKindIsStable(t *testing.T) {
	// Pin the wire-stable kind string. Renaming it breaks every agent
	// that branches on the event type; the constant is part of the
	// contract, not an implementation detail.
	if AgentEventApprovalPending != "approval.pending" {
		t.Fatalf("AgentEventApprovalPending changed: got %q want %q",
			AgentEventApprovalPending, "approval.pending")
	}
}

func TestAgentEventCommandRedactionForMonitoringReadTokens(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/agent/events", nil)
	attachAPITokenRecord(req, &config.APITokenRecord{Scopes: []string{config.ScopeMonitoringRead}})

	event := redactAgentEventCommandsForRequest(AgentEvent{
		Kind: AgentEventActionCompleted,
		Payload: AgentEventActionCompletedPayload{
			ActionID: "action-1",
			Command:  "systemctl restart nginx",
			Verification: &AgentResourceActionVerification{
				Ran:     true,
				Success: true,
				Command: "systemctl is-active nginx",
			},
		},
	}, req)

	payload, ok := event.Payload.(AgentEventActionCompletedPayload)
	if !ok {
		t.Fatalf("payload type: got %T", event.Payload)
	}
	if payload.Command != "" || !payload.CommandRedacted {
		t.Fatalf("action command must be redacted for monitoring-read token; got %+v", payload)
	}
	if payload.Verification == nil {
		t.Fatal("verification must remain present")
	}
	if payload.Verification.Command != "" || !payload.Verification.CommandRedacted {
		t.Fatalf("verification command must be redacted for monitoring-read token; got %+v", payload.Verification)
	}

	approval := redactAgentEventCommandsForRequest(AgentEvent{
		Kind: AgentEventApprovalPending,
		Payload: AgentEventApprovalPendingPayload{
			ApprovalID: "appr-1",
			Command:    "systemctl restart nginx",
		},
	}, req)
	approvalPayload, ok := approval.Payload.(AgentEventApprovalPendingPayload)
	if !ok {
		t.Fatalf("payload type: got %T", approval.Payload)
	}
	if approvalPayload.Command != "" || !approvalPayload.CommandRedacted {
		t.Fatalf("approval command must be redacted for monitoring-read token; got %+v", approvalPayload)
	}
}

func TestAgentEventCommandRedactionKeepsCommandsForActionTokens(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/agent/events", nil)
	attachAPITokenRecord(req, &config.APITokenRecord{
		Scopes: []string{config.ScopeMonitoringRead, config.ScopeAIExecute},
	})

	event := redactAgentEventCommandsForRequest(AgentEvent{
		Kind: AgentEventApprovalPending,
		Payload: AgentEventApprovalPendingPayload{
			ApprovalID: "appr-1",
			Command:    "systemctl restart nginx",
		},
	}, req)
	payload, ok := event.Payload.(AgentEventApprovalPendingPayload)
	if !ok {
		t.Fatalf("payload type: got %T", event.Payload)
	}
	if payload.Command != "systemctl restart nginx" || payload.CommandRedacted {
		t.Fatalf("action-capable token should keep command; got %+v", payload)
	}
}

func TestAgentEventBroadcaster_PublishActionCompletedDelivers(t *testing.T) {
	// action.completed is the third event kind the broadcaster
	// publishes (alongside finding.created and approval.pending).
	// Pin the contract: kind name, full payload round-trip with
	// dispatch-outcome fields agents branch on, monotonic id
	// assignment shared across kinds.
	b := NewAgentEventBroadcaster()
	events, unsub := b.Subscribe()
	defer unsub()

	completed := time.Now().UTC()
	b.PublishActionCompleted(AgentEventActionCompletedPayload{
		ActionID:       "action-1",
		ResourceID:     "container:web-1",
		CapabilityName: "pulse_docker",
		Command:        "docker restart web-1",
		State:          "completed",
		Success:        true,
		RequestedBy:    "ai:patrol",
		CompletedAt:    completed,
	})

	select {
	case event := <-events:
		if event.Kind != AgentEventActionCompleted {
			t.Fatalf("kind: got %q want %q", event.Kind, AgentEventActionCompleted)
		}
		if event.ID == 0 {
			t.Error("event id must be assigned")
		}
		if event.At.IsZero() {
			t.Error("event timestamp must be populated")
		}
		payload, ok := event.Payload.(AgentEventActionCompletedPayload)
		if !ok {
			t.Fatalf("payload type: got %T want AgentEventActionCompletedPayload", event.Payload)
		}
		if payload.ActionID != "action-1" {
			t.Errorf("ActionID: got %q want %q", payload.ActionID, "action-1")
		}
		if payload.ResourceID != "container:web-1" {
			t.Errorf("ResourceID: got %q want %q", payload.ResourceID, "container:web-1")
		}
		if payload.State != "completed" {
			t.Errorf("State: got %q want %q", payload.State, "completed")
		}
		if !payload.Success {
			t.Error("Success did not round-trip")
		}
		if !payload.CompletedAt.Equal(completed) {
			t.Errorf("CompletedAt did not round-trip: got %v want %v", payload.CompletedAt, completed)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for action.completed event")
	}
}

func TestAgentEventBroadcaster_PublishActionCompletedPreservesRefusalToken(t *testing.T) {
	// Refused-before-dispatch failures carry a stable error-token
	// prefix in ErrorMessage (`plan_drift:` /
	// `resource_remediation_locked:`) that agents branch on. The
	// broadcaster must round-trip it verbatim — drift here breaks
	// the contract that lets agents distinguish "the action was
	// refused due to operator state" from "the action errored".
	b := NewAgentEventBroadcaster()
	events, unsub := b.Subscribe()
	defer unsub()

	b.PublishActionCompleted(AgentEventActionCompletedPayload{
		ActionID:     "action-refused",
		ResourceID:   "vm:db-1",
		State:        "failed",
		Success:      false,
		ErrorMessage: "resource_remediation_locked: resource has been locked by the operator",
		CompletedAt:  time.Now().UTC(),
	})

	select {
	case event := <-events:
		payload, ok := event.Payload.(AgentEventActionCompletedPayload)
		if !ok {
			t.Fatalf("payload type: got %T", event.Payload)
		}
		if payload.Success {
			t.Error("refused dispatches must not surface as Success=true")
		}
		if !strings.HasPrefix(payload.ErrorMessage, "resource_remediation_locked:") {
			t.Errorf("ErrorMessage prefix lost: got %q", payload.ErrorMessage)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for action.completed event")
	}
}

func TestAgentEventActionCompletedKindIsStable(t *testing.T) {
	// Pin the wire-stable kind string. Renaming it breaks every
	// agent that branches on the event type.
	if AgentEventActionCompleted != "action.completed" {
		t.Fatalf("AgentEventActionCompleted changed: got %q want %q",
			AgentEventActionCompleted, "action.completed")
	}
}

func TestAgentEventBroadcaster_PublishActionCompletedRoundTripsVerification(t *testing.T) {
	// The verification block is the agent-stable projection of the
	// post-execution read-after-write probe. Pin the contract: it
	// round-trips through the broadcaster with the fields agents
	// branch on (Ran, Success, Command, Note, RanAt) intact.
	b := NewAgentEventBroadcaster()
	events, unsub := b.Subscribe()
	defer unsub()

	ranAt := time.Now().UTC()
	b.PublishActionCompleted(AgentEventActionCompletedPayload{
		ActionID:    "action-verify-1",
		ResourceID:  "container:web-1",
		State:       "completed",
		Success:     true,
		CompletedAt: ranAt,
		Verification: &AgentResourceActionVerification{
			Ran:     true,
			Success: true,
			Command: "docker inspect --format '{{.State.Status}}' web-1",
			RanAt:   ranAt,
		},
	})

	select {
	case event := <-events:
		payload, ok := event.Payload.(AgentEventActionCompletedPayload)
		if !ok {
			t.Fatalf("payload type: got %T", event.Payload)
		}
		if payload.Verification == nil {
			t.Fatal("verification block must round-trip — agents close the certainty loop on this field")
		}
		if !payload.Verification.Ran || !payload.Verification.Success {
			t.Errorf("verification flags lost: ran=%t success=%t", payload.Verification.Ran, payload.Verification.Success)
		}
		if payload.Verification.Command == "" {
			t.Error("verification command must round-trip so agents see what was checked")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for action.completed event")
	}
}

func TestAgentEventBroadcaster_PublishActionCompletedAbsentVerificationOmitsField(t *testing.T) {
	// Refused-before-dispatch failures have no verification result
	// (verification only runs after a successful execute). The
	// payload's Verification must surface as omitted (nil) rather
	// than an empty struct so agents can branch on field presence
	// to distinguish "no verification attempted" from "verification
	// ran with empty result".
	b := NewAgentEventBroadcaster()
	events, unsub := b.Subscribe()
	defer unsub()

	b.PublishActionCompleted(AgentEventActionCompletedPayload{
		ActionID:     "action-refused-1",
		ResourceID:   "vm:db-1",
		State:        "failed",
		Success:      false,
		ErrorMessage: "resource_remediation_locked: ...",
		CompletedAt:  time.Now().UTC(),
		// Verification deliberately unset.
	})

	select {
	case event := <-events:
		payload, ok := event.Payload.(AgentEventActionCompletedPayload)
		if !ok {
			t.Fatalf("payload type: got %T", event.Payload)
		}
		if payload.Verification != nil {
			t.Errorf("verification must remain nil for refused dispatches; got %+v", payload.Verification)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for action.completed event")
	}
}
