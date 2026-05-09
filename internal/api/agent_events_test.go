package api

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
