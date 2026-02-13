package websocket

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestBroadcastStateLogsStructuredContextWhenSequencerFull(t *testing.T) {
	hub := NewHub(nil)

	for i := 0; i < cap(hub.broadcastSeq); i++ {
		hub.broadcastSeq <- Message{Type: "rawData"}
	}

	var buf bytes.Buffer
	restore := setWebSocketTestLogger(&buf, zerolog.WarnLevel)
	defer restore()

	hub.BroadcastState(map[string]string{"state": "full"})

	entry := singleWebSocketLogEntry(t, buf.String())
	assertWebSocketLogStringField(t, entry, "component", websocketHubComponent)
	assertWebSocketLogStringField(t, entry, "action", "enqueue_state_dropped")
	assertWebSocketLogStringField(t, entry, "channel", "broadcast_seq")
	assertWebSocketLogIntField(t, entry, "channel_depth", cap(hub.broadcastSeq))
	assertWebSocketLogIntField(t, entry, "channel_capacity", cap(hub.broadcastSeq))
}

func TestBroadcastStateToTenantLogsStructuredContextWhenQueueFull(t *testing.T) {
	hub := NewHub(nil)

	for i := 0; i < cap(hub.tenantBroadcast); i++ {
		hub.tenantBroadcast <- TenantBroadcast{OrgID: "org-fill", Message: Message{Type: "rawData"}}
	}

	var buf bytes.Buffer
	restore := setWebSocketTestLogger(&buf, zerolog.WarnLevel)
	defer restore()

	hub.BroadcastStateToTenant("org-123", map[string]string{"state": "full"})

	entry := singleWebSocketLogEntry(t, buf.String())
	assertWebSocketLogStringField(t, entry, "component", websocketHubComponent)
	assertWebSocketLogStringField(t, entry, "action", "enqueue_tenant_state_dropped")
	assertWebSocketLogStringField(t, entry, "org_id", "org-123")
	assertWebSocketLogStringField(t, entry, "channel", "tenant_broadcast")
	assertWebSocketLogIntField(t, entry, "channel_depth", cap(hub.tenantBroadcast))
	assertWebSocketLogIntField(t, entry, "channel_capacity", cap(hub.tenantBroadcast))
}

func TestBroadcastMessageLogsStructuredContextWhenQueueFull(t *testing.T) {
	hub := NewHub(nil)

	for i := 0; i < cap(hub.broadcast); i++ {
		hub.broadcast <- []byte("occupied")
	}

	var buf bytes.Buffer
	restore := setWebSocketTestLogger(&buf, zerolog.WarnLevel)
	defer restore()

	hub.BroadcastMessage(Message{
		Type: "custom",
		Data: map[string]string{"status": "ok"},
	})

	entry := singleWebSocketLogEntry(t, buf.String())
	assertWebSocketLogStringField(t, entry, "component", websocketHubComponent)
	assertWebSocketLogStringField(t, entry, "action", "enqueue_broadcast_dropped")
	assertWebSocketLogStringField(t, entry, "message_type", "custom")
	assertWebSocketLogStringField(t, entry, "channel", "broadcast")
	assertWebSocketLogIntField(t, entry, "channel_depth", cap(hub.broadcast))
	assertWebSocketLogIntField(t, entry, "channel_capacity", cap(hub.broadcast))
}

func setWebSocketTestLogger(buf *bytes.Buffer, level zerolog.Level) func() {
	original := log.Logger
	log.Logger = zerolog.New(buf).Level(level)
	return func() {
		log.Logger = original
	}
}

func singleWebSocketLogEntry(t *testing.T, raw string) map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected exactly one log line, got %d: %q", len(lines), raw)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}

	return parsed
}

func assertWebSocketLogStringField(t *testing.T, entry map[string]any, field, want string) {
	t.Helper()

	raw, ok := entry[field]
	if !ok {
		t.Fatalf("missing field %q in log entry: %+v", field, entry)
	}

	got, ok := raw.(string)
	if !ok {
		t.Fatalf("field %q has non-string value %#v", field, raw)
	}

	if got != want {
		t.Fatalf("field %q = %q, want %q", field, got, want)
	}
}

func assertWebSocketLogIntField(t *testing.T, entry map[string]any, field string, want int) {
	t.Helper()

	raw, ok := entry[field]
	if !ok {
		t.Fatalf("missing field %q in log entry: %+v", field, entry)
	}

	got, ok := raw.(float64)
	if !ok {
		t.Fatalf("field %q has non-numeric value %#v", field, raw)
	}

	if int(got) != want {
		t.Fatalf("field %q = %d, want %d", field, int(got), want)
	}
}
