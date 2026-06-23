package agentcapabilities

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScanSSERecordsParsesCanonicalRecords(t *testing.T) {
	input := strings.Join([]string{
		": connected comment",
		"event: stream.connected",
		"data: {}",
		"",
		"event: finding.created",
		"data: {\"findingId\":\"f1\"}",
		"data: {\"severity\":\"critical\"}",
		"",
	}, "\r\n")

	var got []SSERecord
	if err := ScanSSERecords(strings.NewReader(input), func(record SSERecord) bool {
		got = append(got, record)
		return true
	}); err != nil {
		t.Fatalf("ScanSSERecords: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("records len = %d, want 2: %+v", len(got), got)
	}
	if got[0] != (SSERecord{Event: "stream.connected", Data: "{}"}) {
		t.Fatalf("first record = %+v", got[0])
	}
	wantData := "{\"findingId\":\"f1\"}\n{\"severity\":\"critical\"}"
	if got[1] != (SSERecord{Event: "finding.created", Data: wantData}) {
		t.Fatalf("second record = %+v, want data %q", got[1], wantData)
	}
}

func TestScanSSERecordsStopsWhenHandlerReturnsFalse(t *testing.T) {
	input := "event: finding.created\ndata: {\"findingId\":\"f1\"}\n\nevent: action.completed\ndata: {\"actionId\":\"a1\"}\n\n"
	var got []SSERecord
	if err := ScanSSERecords(strings.NewReader(input), func(record SSERecord) bool {
		got = append(got, record)
		return false
	}); err != nil {
		t.Fatalf("ScanSSERecords: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("records len = %d, want 1: %+v", len(got), got)
	}
	if got[0].Event != string(EventKindFindingCreated) {
		t.Fatalf("first event = %q, want %q", got[0].Event, EventKindFindingCreated)
	}
}

func TestScanSSERecordsFlushesFinalRecordAtEOF(t *testing.T) {
	input := "event: action.completed\ndata: {\"actionId\":\"a1\"}"
	var got []SSERecord
	if err := ScanSSERecords(strings.NewReader(input), func(record SSERecord) bool {
		got = append(got, record)
		return true
	}); err != nil {
		t.Fatalf("ScanSSERecords: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("records len = %d, want 1: %+v", len(got), got)
	}
	if got[0] != (SSERecord{Event: string(EventKindActionCompleted), Data: "{\"actionId\":\"a1\"}"}) {
		t.Fatalf("record = %+v", got[0])
	}
}

func TestStreamAgentSSERecordsBuildsCanonicalSubscriptionRequest(t *testing.T) {
	stream := strings.Join([]string{
		"event: " + string(EventKindFindingCreated),
		"data: {\"findingId\":\"f1\"}",
		"",
		"event: " + string(EventKindActionCompleted),
		"data: {\"actionId\":\"a1\"}",
		"",
	}, "\n")

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != AgentEventsPath {
			t.Errorf("path = %s, want %s", r.URL.Path, AgentEventsPath)
		}
		if r.Header.Get(AgentAPITokenHeader) != "test-token" {
			t.Errorf("%s header = %q", AgentAPITokenHeader, r.Header.Get(AgentAPITokenHeader))
		}
		if r.Header.Get("Accept") != AgentSSEAccept {
			t.Errorf("Accept header = %q, want %q", r.Header.Get("Accept"), AgentSSEAccept)
		}
		w.Header().Set("Content-Type", AgentSSEAccept)
		_, _ = w.Write([]byte(stream))
	}))
	defer server.Close()

	var got []SSERecord
	err := StreamAgentSSERecords(context.Background(), server.Client(), server.URL, AgentEventsPath, "test-token", func(record SSERecord) bool {
		got = append(got, record)
		return false
	})
	if err != nil {
		t.Fatalf("StreamAgentSSERecords: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	if len(got) != 1 || got[0].Event != string(EventKindFindingCreated) {
		t.Fatalf("got records = %+v, want first finding.created only", got)
	}
}

func TestStreamAgentSSERecordsReturnsStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	err := StreamAgentSSERecords(context.Background(), server.Client(), server.URL, AgentEventsPath, "bad-token", nil)
	if err == nil || !strings.Contains(err.Error(), "subscribe: status 401") {
		t.Fatalf("error = %v, want subscribe status error", err)
	}
}

func TestStreamAgentActionableSSERecordsFiltersTransportRecords(t *testing.T) {
	stream := strings.Join([]string{
		": connected comment",
		"event: " + string(EventKindStreamConnected),
		"data: {}",
		"",
		"event: " + string(EventKindHeartbeat),
		"",
		"data: unnamed payload",
		"",
		"event: " + string(EventKindFindingCreated),
		"data: {\"findingId\":\"f1\"}",
		"",
	}, "\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", AgentSSEAccept)
		_, _ = w.Write([]byte(stream))
	}))
	defer server.Close()

	var got []SSERecord
	err := StreamAgentActionableSSERecords(context.Background(), server.Client(), server.URL, AgentEventsPath, "test-token", func(record SSERecord) bool {
		got = append(got, record)
		return true
	})
	if err != nil {
		t.Fatalf("StreamAgentActionableSSERecords: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("actionable records len = %d, want 1: %+v", len(got), got)
	}
	if got[0] != (SSERecord{Event: string(EventKindFindingCreated), Data: "{\"findingId\":\"f1\"}"}) {
		t.Fatalf("actionable record = %+v", got[0])
	}
}

func TestStreamMCPEventNotificationsProjectsActionableEvents(t *testing.T) {
	stream := strings.Join([]string{
		": connected comment",
		"event: " + string(EventKindStreamConnected),
		"data: {}",
		"",
		"event: " + string(EventKindFindingCreated),
		"data: {\"findingId\":\"f1\"}",
		"",
		"event: " + string(EventKindHeartbeat),
		"",
		"event: " + string(EventKindActionCompleted),
		"data: {\"actionId\":\"a1\"}",
		"",
	}, "\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(AgentAPITokenHeader) != "test-token" {
			t.Errorf("%s header = %q", AgentAPITokenHeader, r.Header.Get(AgentAPITokenHeader))
		}
		if r.Header.Get("Accept") != AgentSSEAccept {
			t.Errorf("Accept header = %q, want %q", r.Header.Get("Accept"), AgentSSEAccept)
		}
		w.Header().Set("Content-Type", AgentSSEAccept)
		_, _ = w.Write([]byte(stream))
	}))
	defer server.Close()

	var got []JSONRPCRequest
	err := StreamMCPEventNotifications(context.Background(), server.Client(), server.URL, AgentEventsPath, "test-token", func(notification JSONRPCRequest) error {
		got = append(got, notification)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamMCPEventNotifications: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("notifications len = %d, want finding.created and action.completed: %+v", len(got), got)
	}
	if got[0].Method != MCPNotificationMethod(string(EventKindFindingCreated)) {
		t.Fatalf("first notification method = %q", got[0].Method)
	}
	if got[1].Method != MCPNotificationMethod(string(EventKindActionCompleted)) {
		t.Fatalf("second notification method = %q", got[1].Method)
	}
	if string(got[0].Params) != `{"findingId":"f1"}` {
		t.Fatalf("first notification params = %s", got[0].Params)
	}
	if len(got[0].ID) != 0 || len(got[1].ID) != 0 {
		t.Fatalf("notifications must not carry ids: %+v", got)
	}
}

func TestStreamMCPEventNotificationsReturnsWriterError(t *testing.T) {
	stream := strings.Join([]string{
		"event: " + string(EventKindFindingCreated),
		"data: {\"findingId\":\"f1\"}",
		"",
	}, "\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", AgentSSEAccept)
		_, _ = w.Write([]byte(stream))
	}))
	defer server.Close()

	sentinel := errors.New("write notification")
	err := StreamMCPEventNotifications(context.Background(), server.Client(), server.URL, AgentEventsPath, "test-token", func(notification JSONRPCRequest) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want writer error", err)
	}
}
