package api

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// manualStatusWindows replaces the producer's timers so tests close windows
// deterministically.
type manualStatusWindows struct {
	pending []func()
}

func (m *manualStatusWindows) after(_ time.Duration, f func()) {
	m.pending = append(m.pending, f)
}

func (m *manualStatusWindows) closeAll() {
	pending := m.pending
	m.pending = nil
	for _, f := range pending {
		f()
	}
}

func newTestAgentChangeEventProducer(b *AgentEventBroadcaster) (*agentChangeEventProducer, *manualStatusWindows) {
	producer := newAgentChangeEventProducer(b, time.Minute)
	windows := &manualStatusWindows{}
	producer.afterFunc = windows.after
	return producer, windows
}

func testStatusChange(id, resourceID, from, to string) unified.ResourceChange {
	return unified.ResourceChange{
		ID:         id,
		ResourceID: resourceID,
		Kind:       unified.ChangeStateTransition,
		From:       from,
		To:         to,
		ObservedAt: time.Date(2026, 10, 29, 6, 0, 0, 0, time.UTC),
		Metadata:   map[string]any{"changedFields": []string{"status"}},
	}
}

func drainAgentEvents(events <-chan AgentEvent) []AgentEvent {
	var out []AgentEvent
	for {
		select {
		case event := <-events:
			out = append(out, event)
		default:
			return out
		}
	}
}

func TestAgentChangeEventsPublishAlertLifecycleWithoutContent(t *testing.T) {
	b := NewAgentEventBroadcaster()
	events, unsubscribe := b.Subscribe()
	defer unsubscribe()
	producer, _ := newTestAgentChangeEventProducer(b)
	lookup := func(ref string) (*unified.Resource, string, bool) {
		if ref != "vm-web" {
			return nil, "", false
		}
		return &unified.Resource{ID: "vm-web", Name: "web", Type: unified.ResourceTypeVM}, "vm-web", true
	}
	observe := producer.observer("", lookup)
	occurredAt := time.Date(2026, 10, 29, 5, 59, 0, 0, time.UTC)
	observe(unified.ResourceChange{
		ID:         "change-fired",
		ResourceID: "vm-web",
		Kind:       unified.ChangeAlertFired,
		ObservedAt: occurredAt,
		OccurredAt: &occurredAt,
		Reason:     "alert fired: CPU at 97% on web",
		Metadata: map[string]any{
			unified.MetadataAlertIdentifier: "vm-web-cpu",
			unified.MetadataAlertType:       "cpu",
			unified.MetadataAlertLevel:      "critical",
			unified.MetadataAlertMessage:    "CPU at 97% on web",
			unified.MetadataCommand:         "systemctl restart web",
		},
	})
	observe(unified.ResourceChange{
		ID:         "change-resolved",
		ResourceID: "vm-web",
		Kind:       unified.ChangeAlertResolved,
		ObservedAt: occurredAt.Add(time.Minute),
		Metadata: map[string]any{
			unified.MetadataAlertIdentifier: "vm-web-cpu",
			unified.MetadataAlertType:       "cpu",
			unified.MetadataAlertLevel:      "critical",
			unified.MetadataAlertResolution: "moved_to_agent",
		},
	})

	got := drainAgentEvents(events)
	if len(got) != 2 {
		t.Fatalf("events = %+v, want fired and resolved", got)
	}
	if got[0].Kind != AgentEventAlertFired || got[1].Kind != AgentEventAlertResolved {
		t.Fatalf("kinds = %s, %s", got[0].Kind, got[1].Kind)
	}
	fired := got[0].Payload.(AgentEventResourceChangePayload)
	want := AgentEventResourceChangePayload{
		ChangeID:        "change-fired",
		ResourceID:      "vm-web",
		ResourceName:    "web",
		ResourceType:    string(unified.ResourceTypeVM),
		Kind:            string(unified.ChangeAlertFired),
		AlertIdentifier: "vm-web-cpu",
		AlertType:       "cpu",
		AlertLevel:      "critical",
		ObservedAt:      occurredAt,
		OccurredAt:      &occurredAt,
	}
	if fired.OccurredAt == nil || !fired.OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurredAt = %v", fired.OccurredAt)
	}
	fired.OccurredAt = want.OccurredAt
	if fired != want {
		t.Fatalf("fired payload = %+v\nwant %+v", fired, want)
	}
	encoded, err := json.Marshal(got[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"97%", "systemctl", "orgID", "default"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("alert event must not carry %q: %s", forbidden, encoded)
		}
	}
	if resolved := got[1].Payload.(AgentEventResourceChangePayload); resolved.AlertResolution != "moved_to_agent" {
		t.Fatalf("resolved payload = %+v", resolved)
	}
}

func TestAgentChangeEventsIgnoreChangesThatAreNotStatusOrAlerts(t *testing.T) {
	b := NewAgentEventBroadcaster()
	events, unsubscribe := b.Subscribe()
	defer unsubscribe()
	producer, _ := newTestAgentChangeEventProducer(b)
	observe := producer.observer("default", nil)

	discovered := testStatusChange("discovered", "vm-new", "absent", "online")
	discovered.Metadata = map[string]any{"changeType": "resource_created"}
	lifecycle := testStatusChange("lock", "vm-web", "online", "online")
	lifecycle.Metadata = map[string]any{"changedFields": []string{"proxmox.lifecycle"}}
	for _, change := range []unified.ResourceChange{
		discovered,
		lifecycle,
		{ID: "ack", ResourceID: "vm-web", Kind: unified.ChangeAlertAcknowledged},
		{ID: "restart", ResourceID: "vm-web", Kind: unified.ChangeRestart},
		{ID: "cmd", ResourceID: "vm-web", Kind: unified.ChangeCommandExecuted, Metadata: map[string]any{unified.MetadataCommand: "reboot"}},
	} {
		observe(change)
	}
	if got := drainAgentEvents(events); len(got) != 0 {
		t.Fatalf("events = %+v, want none", got)
	}
}

// A flapping resource yields its first change at once and at most one more
// event per window; a flap that ends where it started publishes nothing.
func TestAgentChangeEventsCoalesceStatusFlapsPerResource(t *testing.T) {
	b := NewAgentEventBroadcaster()
	events, unsubscribe := b.Subscribe()
	defer unsubscribe()
	producer, windows := newTestAgentChangeEventProducer(b)
	observe := producer.observer("default", nil)

	observe(testStatusChange("c1", "vm-web", "online", "offline"))
	observe(testStatusChange("other", "vm-db", "online", "offline"))
	for i, to := range []string{"online", "offline", "online", "offline", "online"} {
		from := map[string]string{"online": "offline", "offline": "online"}[to]
		observe(testStatusChange("flap-"+string(rune('a'+i)), "vm-web", from, to))
	}
	got := drainAgentEvents(events)
	if len(got) != 2 {
		t.Fatalf("immediate events = %+v, want one per resource", got)
	}
	first := got[0].Payload.(AgentEventResourceChangePayload)
	if got[0].Kind != AgentEventResourceStatusChanged || first.ChangeID != "c1" || first.From != "online" || first.To != "offline" || first.CoalescedTransitions != 0 {
		t.Fatalf("first event = %s %+v", got[0].Kind, first)
	}

	windows.closeAll()
	got = drainAgentEvents(events)
	if len(got) != 1 {
		t.Fatalf("window close events = %+v, want one coalesced event for vm-web", got)
	}
	coalesced := got[0].Payload.(AgentEventResourceChangePayload)
	if coalesced.ResourceID != "vm-web" || coalesced.ChangeID != "flap-e" || coalesced.From != "offline" || coalesced.To != "online" || coalesced.CoalescedTransitions != 5 {
		t.Fatalf("coalesced = %+v", coalesced)
	}

	// The coalesced event opened a new window. A flap inside it that returns
	// to the announced status publishes nothing when it closes.
	observe(testStatusChange("g", "vm-web", "online", "offline"))
	observe(testStatusChange("h", "vm-web", "offline", "online"))
	windows.closeAll()
	if got := drainAgentEvents(events); len(got) != 0 {
		t.Fatalf("round-trip flap events = %+v, want none", got)
	}

	// With every window closed, the next change is announced at once again.
	observe(testStatusChange("i", "vm-web", "online", "offline"))
	got = drainAgentEvents(events)
	if len(got) != 1 || got[0].Payload.(AgentEventResourceChangePayload).ChangeID != "i" {
		t.Fatalf("post-quiet events = %+v", got)
	}
	windows.closeAll()
	producer.mu.Lock()
	open := len(producer.windows)
	producer.mu.Unlock()
	if open != 0 {
		t.Fatalf("closed windows must be forgotten, %d still open", open)
	}
}
