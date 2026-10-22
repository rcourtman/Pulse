package websocket

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gorillawebsocket "github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestNormalClientCloseIsNotAnError(t *testing.T) {
	normal := &gorillawebsocket.CloseError{
		Code: gorillawebsocket.CloseNormalClosure,
		Text: "Component unmounting",
	}
	if isUnexpectedClientClose(normal) {
		t.Fatal("normal component unmount was classified as an unexpected close")
	}

	policyViolation := &gorillawebsocket.CloseError{
		Code: gorillawebsocket.ClosePolicyViolation,
		Text: "authentication failed",
	}
	if !isUnexpectedClientClose(policyViolation) {
		t.Fatal("policy violation was not classified as an unexpected close")
	}
}

func TestClientSafeSendAndCloseAreSynchronized(t *testing.T) {
	client := &Client{send: make(chan []byte, 1)}
	start := make(chan struct{})
	var senders sync.WaitGroup
	for i := 0; i < 8; i++ {
		senders.Add(1)
		go func() {
			defer senders.Done()
			<-start
			for attempt := 0; attempt < 1_000; attempt++ {
				client.safeSend([]byte("state"))
			}
		}()
	}

	close(start)
	client.closeSend()
	senders.Wait()
	if client.safeSend([]byte("after-close")) {
		t.Fatal("safeSend succeeded after closeSend")
	}
}

func TestClientLifecycleSignalConcurrentInitializationAndClose(t *testing.T) {
	client := &Client{}
	start := make(chan struct{})
	signals := make([]chan struct{}, 32)
	var workers sync.WaitGroup

	for i := range signals {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			<-start
			signals[index] = client.lifecycleSignal()
		}(i)
	}
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			client.closeLifecycle()
		}()
	}

	close(start)
	workers.Wait()

	for i, signal := range signals {
		if signal == nil {
			t.Fatalf("lifecycle signal %d was nil", i)
		}
		if signal != signals[0] {
			t.Fatalf("lifecycle signal %d did not share the client lifecycle", i)
		}
	}
	select {
	case <-signals[0]:
	default:
		t.Fatal("lifecycle signal remained open after closeLifecycle")
	}
}

func TestHubDisconnectedClientsDoNotBuildInitialState(t *testing.T) {
	var stateBuilds atomic.Int64
	hub := NewHub(func(string) interface{} {
		stateBuilds.Add(1)
		return map[string]interface{}{
			"resources": make([]map[string]interface{}, 4_000),
		}
	})
	go hub.Run()
	t.Cleanup(hub.Stop)

	const reconnects = 32
	for i := 0; i < reconnects; i++ {
		client := &Client{
			hub:  hub,
			id:   "disconnected-before-initial-state",
			send: make(chan []byte, 1),
		}
		hub.register <- client
		hub.unregister <- client
	}

	hub.Stop()
	if got := stateBuilds.Load(); got != 0 {
		t.Fatalf("built %d initial states for clients that had already disconnected", got)
	}
}

func TestHubCurrentStateBroadcastsStaySerializedUnderChurn(t *testing.T) {
	hub := NewHub(nil)
	hub.coalesceWindow = time.Millisecond
	go hub.Run()
	t.Cleanup(hub.Stop)

	client := &Client{
		hub:  hub,
		id:   "state-broadcast-soak",
		send: make(chan []byte, 128),
	}
	hub.register <- client

	var active, maxActive, builds atomic.Int64
	hub.SetStateGetter(func(string) interface{} {
		builds.Add(1)
		current := active.Add(1)
		for {
			maximum := maxActive.Load()
			if current <= maximum || maxActive.CompareAndSwap(maximum, current) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		active.Add(-1)
		return map[string]string{"status": "ok"}
	})

	for i := 0; i < 50; i++ {
		hub.BroadcastCurrentState()
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(250 * time.Millisecond)
	hub.Stop()

	if got := maxActive.Load(); got != 1 {
		t.Fatalf("maximum concurrent current-state builds = %d, want 1", got)
	}
	if got := builds.Load(); got > 10 {
		t.Fatalf("current-state builds = %d for 50 supersedable signals, want at most 10 serialized builds", got)
	}
}

func TestHubInitialAndBroadcastStateBuildsShareSerialization(t *testing.T) {
	var active, maxActive atomic.Int64
	hub := NewHub(func(string) interface{} {
		current := active.Add(1)
		for {
			maximum := maxActive.Load()
			if current <= maximum || maxActive.CompareAndSwap(maximum, current) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		active.Add(-1)
		return map[string]string{"status": "ok"}
	})
	hub.coalesceWindow = time.Millisecond
	go hub.Run()
	t.Cleanup(hub.Stop)

	const clients = 8
	for i := 0; i < clients; i++ {
		hub.register <- &Client{
			hub:  hub,
			id:   "simultaneous-initial-state",
			send: make(chan []byte, 8),
		}
	}
	time.AfterFunc(initialWelcomeDelay+initialStateMessageDelay, hub.BroadcastCurrentState)
	time.Sleep(initialWelcomeDelay + initialStateMessageDelay + 400*time.Millisecond)
	hub.Stop()

	if got := maxActive.Load(); got != 1 {
		t.Fatalf("maximum concurrent initial and broadcast state builds = %d, want 1", got)
	}
}

func TestHubRequestedStateBuildsShareSerialization(t *testing.T) {
	var active, maxActive atomic.Int64
	hub := NewHub(func(string) interface{} {
		current := active.Add(1)
		for {
			maximum := maxActive.Load()
			if current <= maximum || maxActive.CompareAndSwap(maximum, current) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		active.Add(-1)
		return map[string]string{"status": "ok"}
	})
	go hub.Run()
	t.Cleanup(hub.Stop)

	const clients = 8
	registered := make([]*Client, 0, clients)
	for i := 0; i < clients; i++ {
		client := &Client{
			hub:  hub,
			id:   "simultaneous-requested-state",
			send: make(chan []byte, 8),
		}
		hub.register <- client
		registered = append(registered, client)
	}

	var done sync.WaitGroup
	done.Add(len(registered))
	for _, client := range registered {
		go func() {
			defer done.Done()
			hub.sendRequestedState(client)
		}()
	}
	done.Wait()
	hub.Stop()

	if got := maxActive.Load(); got != 1 {
		t.Fatalf("maximum concurrent requested state builds = %d, want 1", got)
	}
}

func TestIsValidPrivateOrigin(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		// Localhost variations
		{"localhost", "localhost", true},
		{"ipv4 loopback", "127.0.0.1", true},
		{"ipv6 loopback", "::1", true},

		// Private IPv4 ranges
		{"10.x.x.x private", "10.0.0.1", true},
		{"10.x.x.x edge", "10.255.255.255", true},
		{"172.16.x.x private", "172.16.0.1", true},
		{"172.31.x.x private", "172.31.255.255", true},
		{"192.168.x.x private", "192.168.1.1", true},
		{"192.168.x.x edge", "192.168.255.255", true},

		// Local domain suffixes
		{"hostname.local", "myhost.local", true},
		{"hostname.lan", "myhost.lan", true},
		{"subdomain.hostname.local", "sub.myhost.local", true},
		{"too many subdomains .local", "a.b.c.d.local", false},

		// Public IPs (should reject)
		{"public IP 8.8.8.8", "8.8.8.8", false},
		{"public IP 1.1.1.1", "1.1.1.1", false},
		{"public IP 203.0.113.1", "203.0.113.1", false},

		// Public domains (should reject)
		{"example.com", "example.com", false},
		{"google.com", "google.com", false},
		{"malicious.attacker.com", "malicious.attacker.com", false},

		// Edge cases
		{"empty string", "", false},
		{"just dot", ".", false},
		{"numbers only", "12345", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isValidPrivateOrigin(tc.host)
			if result != tc.expected {
				t.Errorf("isValidPrivateOrigin(%q) = %v, want %v", tc.host, result, tc.expected)
			}
		})
	}
}

func TestNormalizeForwardedProto(t *testing.T) {
	tests := []struct {
		name     string
		proto    string
		fallback string
		expected string
	}{
		// Empty proto returns fallback
		{"empty proto returns fallback", "", "http", "http"},
		{"empty proto returns https fallback", "", "https", "https"},

		// Standard HTTP schemes
		{"http passthrough", "http", "https", "http"},
		{"https passthrough", "https", "http", "https"},
		{"HTTP uppercase", "HTTP", "http", "http"},
		{"HTTPS uppercase", "HTTPS", "http", "https"},

		// WebSocket schemes normalized to HTTP
		{"ws becomes http", "ws", "https", "http"},
		{"wss becomes https", "wss", "http", "https"},
		{"WS uppercase", "WS", "https", "http"},
		{"WSS uppercase", "WSS", "http", "https"},

		// Comma-separated chains (take first)
		{"chain wss,https", "wss,https", "http", "https"},
		{"chain https,wss", "https,wss", "http", "https"},
		{"chain http,wss,https", "http,wss,https", "https", "http"},

		// Whitespace handling
		{"whitespace trimmed", "  https  ", "http", "https"},
		{"whitespace in chain", "  wss , https  ", "http", "https"},

		// Unknown protos pass through
		{"unknown proto", "ftp", "http", "ftp"},
		{"unknown empty after trim", "   ", "http", "http"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizeForwardedProto(tc.proto, tc.fallback)
			if result != tc.expected {
				t.Errorf("normalizeForwardedProto(%q, %q) = %q, want %q", tc.proto, tc.fallback, result, tc.expected)
			}
		})
	}
}

func TestSanitizeValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		// Normal values pass through
		{"normal float64", float64(42.5), float64(42.5)},
		{"zero float64", float64(0), float64(0)},
		{"negative float64", float64(-100.5), float64(-100.5)},
		{"string", "hello", "hello"},
		{"int via float64", float64(100), float64(100)},
		{"bool true", true, true},
		{"bool false", false, false},
		{"nil", nil, nil},

		// NaN becomes nil
		{"NaN float64", math.NaN(), nil},

		// Inf becomes nil
		{"positive Inf", math.Inf(1), nil},
		{"negative Inf", math.Inf(-1), nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeValue(tc.input)

			// Special handling for NaN comparison
			if tc.expected == nil {
				if result != nil {
					t.Errorf("sanitizeValue() = %v, want nil", result)
				}
				return
			}

			if result != tc.expected {
				t.Errorf("sanitizeValue() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestSanitizeValue_Map(t *testing.T) {
	input := map[string]interface{}{
		"normal":   float64(42.5),
		"nan":      math.NaN(),
		"inf":      math.Inf(1),
		"negInf":   math.Inf(-1),
		"string":   "test",
		"bool":     true,
		"nilValue": nil,
	}

	result := sanitizeValue(input)
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result)
	}

	// Check normal value preserved
	if resultMap["normal"] != float64(42.5) {
		t.Errorf("normal value = %v, want 42.5", resultMap["normal"])
	}

	// Check NaN/Inf became nil
	if resultMap["nan"] != nil {
		t.Errorf("nan value = %v, want nil", resultMap["nan"])
	}
	if resultMap["inf"] != nil {
		t.Errorf("inf value = %v, want nil", resultMap["inf"])
	}
	if resultMap["negInf"] != nil {
		t.Errorf("negInf value = %v, want nil", resultMap["negInf"])
	}

	// Check other types preserved
	if resultMap["string"] != "test" {
		t.Errorf("string value = %v, want 'test'", resultMap["string"])
	}
	if resultMap["bool"] != true {
		t.Errorf("bool value = %v, want true", resultMap["bool"])
	}
}

func TestSanitizeValue_Slice(t *testing.T) {
	input := []interface{}{
		float64(1.0),
		math.NaN(),
		float64(2.0),
		math.Inf(1),
		"string",
	}

	result := sanitizeValue(input)
	resultSlice, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", result)
	}

	if len(resultSlice) != 5 {
		t.Fatalf("slice length = %d, want 5", len(resultSlice))
	}

	if resultSlice[0] != float64(1.0) {
		t.Errorf("resultSlice[0] = %v, want 1.0", resultSlice[0])
	}
	if resultSlice[1] != nil {
		t.Errorf("resultSlice[1] (NaN) = %v, want nil", resultSlice[1])
	}
	if resultSlice[2] != float64(2.0) {
		t.Errorf("resultSlice[2] = %v, want 2.0", resultSlice[2])
	}
	if resultSlice[3] != nil {
		t.Errorf("resultSlice[3] (Inf) = %v, want nil", resultSlice[3])
	}
	if resultSlice[4] != "string" {
		t.Errorf("resultSlice[4] = %v, want 'string'", resultSlice[4])
	}
}

func TestSanitizeValue_NestedMap(t *testing.T) {
	input := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner": map[string]interface{}{
				"nan": math.NaN(),
				"ok":  float64(42),
			},
		},
	}

	result := sanitizeValue(input)
	resultMap := result.(map[string]interface{})
	outer := resultMap["outer"].(map[string]interface{})
	inner := outer["inner"].(map[string]interface{})

	if inner["nan"] != nil {
		t.Errorf("nested nan = %v, want nil", inner["nan"])
	}
	if inner["ok"] != float64(42) {
		t.Errorf("nested ok = %v, want 42", inner["ok"])
	}
}

func TestCloneMetadata(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		result := cloneMetadata(nil)
		if result != nil {
			t.Errorf("cloneMetadata(nil) = %v, want nil", result)
		}
	})

	t.Run("empty map returns empty map", func(t *testing.T) {
		input := map[string]interface{}{}
		result := cloneMetadata(input)
		if result == nil {
			t.Fatal("cloneMetadata(empty) returned nil")
		}
		if len(result) != 0 {
			t.Errorf("cloneMetadata(empty) has length %d, want 0", len(result))
		}
	})

	t.Run("clones simple values", func(t *testing.T) {
		input := map[string]interface{}{
			"string": "hello",
			"int":    42,
			"float":  3.14,
			"bool":   true,
		}
		result := cloneMetadata(input)

		// Verify values copied
		if result["string"] != "hello" {
			t.Errorf("string = %v, want 'hello'", result["string"])
		}
		if result["int"] != 42 {
			t.Errorf("int = %v, want 42", result["int"])
		}
		if result["float"] != 3.14 {
			t.Errorf("float = %v, want 3.14", result["float"])
		}
		if result["bool"] != true {
			t.Errorf("bool = %v, want true", result["bool"])
		}

		// Verify it's a different map
		input["string"] = "modified"
		if result["string"] == "modified" {
			t.Error("cloned map should not be affected by original modifications")
		}
	})

	t.Run("deep clones nested maps", func(t *testing.T) {
		nested := map[string]interface{}{"key": "value"}
		input := map[string]interface{}{
			"nested": nested,
		}
		result := cloneMetadata(input)

		// Modify original nested map
		nested["key"] = "modified"

		// Cloned nested map should be unaffected
		resultNested := result["nested"].(map[string]interface{})
		if resultNested["key"] != "value" {
			t.Errorf("nested key = %v, want 'value' (should not be affected by original)", resultNested["key"])
		}
	})

	t.Run("deep clones slices", func(t *testing.T) {
		slice := []string{"a", "b", "c"}
		input := map[string]interface{}{
			"slice": slice,
		}
		result := cloneMetadata(input)

		// Modify original slice
		slice[0] = "modified"

		// Cloned slice should be unaffected
		resultSlice := result["slice"].([]string)
		if resultSlice[0] != "a" {
			t.Errorf("slice[0] = %v, want 'a' (should not be affected by original)", resultSlice[0])
		}
	})
}

func TestCloneMetadataValue(t *testing.T) {
	t.Run("clones map[string]interface{}", func(t *testing.T) {
		input := map[string]interface{}{"key": "value"}
		result := cloneMetadataValue(input)
		resultMap := result.(map[string]interface{})

		input["key"] = "modified"
		if resultMap["key"] != "value" {
			t.Error("cloned map should not be affected by original")
		}
	})

	t.Run("clones map[string]string", func(t *testing.T) {
		input := map[string]string{"key": "value"}
		result := cloneMetadataValue(input)

		// map[string]string gets converted to map[string]interface{}
		resultMap := result.(map[string]interface{})
		if resultMap["key"] != "value" {
			t.Errorf("key = %v, want 'value'", resultMap["key"])
		}
	})

	t.Run("clones []interface{}", func(t *testing.T) {
		input := []interface{}{"a", "b", "c"}
		result := cloneMetadataValue(input)
		resultSlice := result.([]interface{})

		input[0] = "modified"
		if resultSlice[0] != "a" {
			t.Error("cloned slice should not be affected by original")
		}
	})

	t.Run("clones []string", func(t *testing.T) {
		input := []string{"a", "b", "c"}
		result := cloneMetadataValue(input)
		resultSlice := result.([]string)

		input[0] = "modified"
		if resultSlice[0] != "a" {
			t.Error("cloned string slice should not be affected by original")
		}
	})

	t.Run("clones []int", func(t *testing.T) {
		input := []int{1, 2, 3}
		result := cloneMetadataValue(input)
		resultSlice := result.([]int)

		input[0] = 999
		if resultSlice[0] != 1 {
			t.Error("cloned int slice should not be affected by original")
		}
	})

	t.Run("clones []float64", func(t *testing.T) {
		input := []float64{1.1, 2.2, 3.3}
		result := cloneMetadataValue(input)
		resultSlice := result.([]float64)

		input[0] = 999.9
		if resultSlice[0] != 1.1 {
			t.Error("cloned float64 slice should not be affected by original")
		}
	})

	t.Run("primitives returned as-is", func(t *testing.T) {
		// Primitives are immutable, so returning as-is is fine
		if cloneMetadataValue("string") != "string" {
			t.Error("string should pass through")
		}
		if cloneMetadataValue(42) != 42 {
			t.Error("int should pass through")
		}
		if cloneMetadataValue(3.14) != 3.14 {
			t.Error("float should pass through")
		}
		if cloneMetadataValue(true) != true {
			t.Error("bool should pass through")
		}
		if cloneMetadataValue(nil) != nil {
			t.Error("nil should pass through")
		}
	})
}

func TestCloneAlert(t *testing.T) {
	t.Run("nil alert returns empty alert", func(t *testing.T) {
		result := cloneAlert(nil)
		if result.ID != "" {
			t.Error("cloneAlert(nil) should return empty alert")
		}
	})

	t.Run("clones basic fields", func(t *testing.T) {
		now := time.Now()
		original := &alerts.Alert{
			ID:         "alert-123",
			Type:       "cpu",
			Level:      alerts.AlertLevelWarning,
			ResourceID: "vm/100",
			Message:    "CPU high",
			Value:      85.5,
			StartTime:  now,
		}

		result := cloneAlert(original)

		if result.ID != "alert-123" {
			t.Errorf("ID = %v, want alert-123", result.ID)
		}
		if result.Type != "cpu" {
			t.Errorf("Type = %v, want cpu", result.Type)
		}
		if result.Level != alerts.AlertLevelWarning {
			t.Errorf("Level = %v, want AlertLevelWarning", result.Level)
		}
		if result.ResourceID != "vm/100" {
			t.Errorf("ResourceID = %v, want vm/100", result.ResourceID)
		}
		if result.Message != "CPU high" {
			t.Errorf("Message = %v, want 'CPU high'", result.Message)
		}
		if result.Value != 85.5 {
			t.Errorf("Value = %v, want 85.5", result.Value)
		}
	})

	t.Run("deep clones AckTime", func(t *testing.T) {
		ackTime := time.Now()
		original := &alerts.Alert{
			ID:      "alert-123",
			AckTime: &ackTime,
		}

		result := cloneAlert(original)

		// Verify AckTime is cloned
		if result.AckTime == nil {
			t.Fatal("AckTime should not be nil")
		}
		if result.AckTime == original.AckTime {
			t.Error("AckTime should be a different pointer")
		}
		if !result.AckTime.Equal(*original.AckTime) {
			t.Error("AckTime values should be equal")
		}
	})

	t.Run("deep clones EscalationTimes", func(t *testing.T) {
		original := &alerts.Alert{
			ID:              "alert-123",
			EscalationTimes: []time.Time{time.Now(), time.Now().Add(time.Hour)},
		}

		result := cloneAlert(original)

		if len(result.EscalationTimes) != 2 {
			t.Fatalf("EscalationTimes length = %d, want 2", len(result.EscalationTimes))
		}

		// Modify original
		original.EscalationTimes[0] = time.Now().Add(24 * time.Hour)

		// Clone should be unaffected
		if result.EscalationTimes[0].Equal(original.EscalationTimes[0]) {
			t.Error("cloned EscalationTimes should not be affected by original modifications")
		}
	})

	t.Run("deep clones Metadata", func(t *testing.T) {
		original := &alerts.Alert{
			ID: "alert-123",
			Metadata: map[string]interface{}{
				"key": "value",
				"nested": map[string]interface{}{
					"inner": "data",
				},
			},
		}

		result := cloneAlert(original)

		// Modify original metadata
		original.Metadata["key"] = "modified"

		// Clone should be unaffected
		if result.Metadata["key"] != "value" {
			t.Error("cloned Metadata should not be affected by original modifications")
		}
	})
}

func TestCloneAlertData(t *testing.T) {
	t.Run("handles *alerts.Alert", func(t *testing.T) {
		original := &alerts.Alert{ID: "alert-123", Message: "test"}
		result := cloneAlertData(original)

		cloned, ok := result.(alerts.Alert)
		if !ok {
			t.Fatalf("expected alerts.Alert, got %T", result)
		}
		if cloned.ID != "alert-123" {
			t.Errorf("ID = %v, want alert-123", cloned.ID)
		}
	})

	t.Run("handles alerts.Alert value", func(t *testing.T) {
		original := alerts.Alert{ID: "alert-456", Message: "test"}
		result := cloneAlertData(original)

		cloned, ok := result.(alerts.Alert)
		if !ok {
			t.Fatalf("expected alerts.Alert, got %T", result)
		}
		if cloned.ID != "alert-456" {
			t.Errorf("ID = %v, want alert-456", cloned.ID)
		}
	})

	t.Run("returns other types unchanged", func(t *testing.T) {
		// Strings, maps, etc. that aren't alerts should pass through
		result := cloneAlertData("not an alert")
		if result != "not an alert" {
			t.Errorf("non-alert data should pass through unchanged")
		}
	})
}

func TestNewHub(t *testing.T) {
	stateGetter := func(orgID string) interface{} {
		return map[string]string{"status": "ok"}
	}

	hub := NewHub(stateGetter)

	if hub == nil {
		t.Fatal("NewHub returned nil")
	}

	if hub.clients == nil {
		t.Error("clients map should be initialized")
	}
	if hub.broadcast == nil {
		t.Error("broadcast channel should be initialized")
	}
	if hub.broadcastSeq == nil {
		t.Error("broadcastSeq channel should be initialized")
	}
	if hub.register == nil {
		t.Error("register channel should be initialized")
	}
	if hub.unregister == nil {
		t.Error("unregister channel should be initialized")
	}
	if hub.stopChan == nil {
		t.Error("stopChan should be initialized")
	}
	if hub.getState == nil {
		t.Error("getState should be set")
	}
	if len(hub.allowedOrigins) != 0 {
		t.Error("allowedOrigins should be empty by default")
	}
	if hub.coalesceWindow != 100*time.Millisecond {
		t.Errorf("coalesceWindow = %v, want 100ms", hub.coalesceWindow)
	}
}

func TestHub_SetAllowedOrigins(t *testing.T) {
	hub := NewHub(nil)

	origins := []string{"http://localhost:3000", "https://example.com"}
	hub.SetAllowedOrigins(origins)

	hub.mu.RLock()
	defer hub.mu.RUnlock()

	if len(hub.allowedOrigins) != 2 {
		t.Errorf("allowedOrigins length = %d, want 2", len(hub.allowedOrigins))
	}
	if hub.allowedOrigins[0] != "http://localhost:3000" {
		t.Errorf("allowedOrigins[0] = %v, want http://localhost:3000", hub.allowedOrigins[0])
	}
}

func TestHub_SetAllowedOrigins_DefensiveCopy(t *testing.T) {
	hub := NewHub(nil)
	origins := []string{"http://localhost:3000", "https://example.com"}

	hub.SetAllowedOrigins(origins)
	origins[0] = "https://mutated.example.com"

	hub.mu.RLock()
	defer hub.mu.RUnlock()

	if hub.allowedOrigins[0] != "http://localhost:3000" {
		t.Fatalf("allowedOrigins leaked caller mutation, got %q", hub.allowedOrigins[0])
	}
}

func TestHub_SetStateGetter(t *testing.T) {
	hub := NewHub(nil)

	if hub.getState != nil {
		t.Error("getState should be nil initially when passed nil")
	}

	newGetter := func(orgID string) interface{} {
		return "new state:" + orgID
	}
	hub.SetStateGetter(newGetter)

	hub.mu.RLock()
	if hub.getState == nil {
		hub.mu.RUnlock()
		t.Error("getState should be set after SetStateGetter")
		return
	}
	hub.mu.RUnlock()
	if got := hub.getStateForClient(&Client{orgID: ""}); got != "new state:default" {
		t.Fatalf("default tenant state = %v, want %q", got, "new state:default")
	}
	if got := hub.getStateForClient(&Client{orgID: "org-123"}); got != "new state:org-123" {
		t.Fatalf("tenant state = %v, want %q", got, "new state:org-123")
	}
}

func TestHub_GetClientCount(t *testing.T) {
	hub := NewHub(nil)

	if hub.GetClientCount() != 0 {
		t.Error("client count should be 0 initially")
	}

	// Manually add clients (bypassing register channel for unit test)
	hub.mu.Lock()
	hub.clients[&Client{id: "client-1"}] = true
	hub.clients[&Client{id: "client-2"}] = true
	hub.mu.Unlock()

	if hub.GetClientCount() != 2 {
		t.Errorf("client count = %d, want 2", hub.GetClientCount())
	}
}

func TestMessage_Fields(t *testing.T) {
	msg := Message{
		Type:      "test",
		Data:      map[string]string{"key": "value"},
		Timestamp: "2024-01-01T00:00:00Z",
	}

	if msg.Type != "test" {
		t.Errorf("Type = %v, want test", msg.Type)
	}
	if msg.Timestamp != "2024-01-01T00:00:00Z" {
		t.Errorf("Timestamp = %v, want 2024-01-01T00:00:00Z", msg.Timestamp)
	}
}

func TestHub_CheckOrigin(t *testing.T) {
	tests := []struct {
		name           string
		origin         string
		host           string
		allowedOrigins []string
		forwardedProto string
		forwardedHost  string
		remoteAddr     string // Simulated peer IP for CSWSH checks
		expected       bool
	}{
		// Browser-facing hub requires an explicit Origin header.
		{
			name:     "no origin header",
			origin:   "",
			host:     "localhost:8080",
			expected: false,
		},

		// Same-origin requests
		{
			name:     "same origin http",
			origin:   "http://localhost:8080",
			host:     "localhost:8080",
			expected: true,
		},
		{
			name:     "same host allowed when proxy preserves host but terminates tls upstream",
			origin:   "https://tenant.example.com",
			host:     "tenant.example.com",
			expected: true,
		},
		{
			name:     "same host with non-default port allowed when proxy preserves host",
			origin:   "https://tenant.example.com:8443",
			host:     "tenant.example.com:8443",
			expected: true,
		},
		{
			name:           "same origin with forwarded proto https",
			origin:         "https://example.com",
			host:           "example.com",
			forwardedProto: "https",
			expected:       true,
		},
		{
			name:          "same origin with forwarded host",
			origin:        "http://proxy.example.com",
			host:          "backend:8080",
			forwardedHost: "proxy.example.com",
			expected:      true,
		},

		// Wildcard allowed origins
		{
			name:           "wildcard allows any origin",
			origin:         "https://evil.com",
			host:           "localhost:8080",
			allowedOrigins: []string{"*"},
			expected:       true,
		},

		// Explicit allowed origins
		{
			name:           "explicit allowed origin matches",
			origin:         "https://app.example.com",
			host:           "localhost:8080",
			allowedOrigins: []string{"https://app.example.com"},
			expected:       true,
		},
		{
			name:           "explicit allowed origin no match",
			origin:         "https://other.example.com",
			host:           "localhost:8080",
			allowedOrigins: []string{"https://app.example.com"},
			expected:       false,
		},
		{
			name:           "multiple allowed origins - match second",
			origin:         "https://second.example.com",
			host:           "localhost:8080",
			allowedOrigins: []string{"https://first.example.com", "https://second.example.com"},
			expected:       true,
		},

		// Private network fallback (no allowed origins configured)
		// Note: remoteAddr must be private for CSWSH protection to allow
		{
			name:       "private IP 192.168.x.x allowed when no origins configured",
			origin:     "http://192.168.1.100:3000",
			host:       "localhost:8080",
			remoteAddr: "192.168.1.100:54321",
			expected:   true,
		},
		{
			name:       "private IP 10.x.x.x allowed when no origins configured",
			origin:     "http://10.0.0.50:3000",
			host:       "localhost:8080",
			remoteAddr: "10.0.0.50:54321",
			expected:   true,
		},
		{
			name:       "localhost allowed when no origins configured",
			origin:     "http://localhost:3000",
			host:       "localhost:8080",
			remoteAddr: "127.0.0.1:54321",
			expected:   true,
		},
		{
			name:       "127.0.0.1 allowed when no origins configured",
			origin:     "http://127.0.0.1:3000",
			host:       "localhost:8080",
			remoteAddr: "127.0.0.1:54321",
			expected:   true,
		},
		{
			name:       ".local domain allowed when no origins configured",
			origin:     "http://myserver.local:3000",
			host:       "localhost:8080",
			remoteAddr: "192.168.1.50:54321",
			expected:   true,
		},
		{
			name:       ".lan domain allowed when no origins configured",
			origin:     "http://myserver.lan:3000",
			host:       "localhost:8080",
			remoteAddr: "192.168.1.50:54321",
			expected:   true,
		},
		{
			name:     "public IP rejected when no origins configured",
			origin:   "http://8.8.8.8:3000",
			host:     "localhost:8080",
			expected: false,
		},
		{
			name:     "public domain rejected when no origins configured",
			origin:   "https://evil.example.com",
			host:     "localhost:8080",
			expected: false,
		},

		// HTTPS origin stripping
		{
			name:       "https origin with private IP",
			origin:     "https://192.168.1.50:443",
			host:       "localhost:8080",
			remoteAddr: "192.168.1.50:54321",
			expected:   true,
		},

		// Forwarded proto normalization
		{
			name:           "wss forwarded proto normalized to https",
			origin:         "https://example.com",
			host:           "example.com",
			forwardedProto: "wss",
			expected:       true,
		},
		{
			name:           "ws forwarded proto normalized to http",
			origin:         "http://example.com",
			host:           "example.com",
			forwardedProto: "ws",
			expected:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hub := NewHub(nil)
			if len(tc.allowedOrigins) > 0 {
				hub.SetAllowedOrigins(tc.allowedOrigins)
			}

			// When tests use forwarded headers, set a trusted proxy checker
			// so the hub trusts X-Forwarded-* from this peer.
			if tc.forwardedProto != "" || tc.forwardedHost != "" {
				hub.SetTrustedProxyChecker(func(ip string) bool { return true })
			}

			req := &http.Request{
				Host:       tc.host,
				Header:     make(http.Header),
				RemoteAddr: tc.remoteAddr,
			}
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.forwardedProto)
			}
			if tc.forwardedHost != "" {
				req.Header.Set("X-Forwarded-Host", tc.forwardedHost)
			}

			result := hub.checkOrigin(req)
			if result != tc.expected {
				t.Errorf("checkOrigin() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestHub_CheckOrigin_XForwardedScheme(t *testing.T) {
	hub := NewHub(nil)
	hub.SetTrustedProxyChecker(func(ip string) bool { return true })

	req := &http.Request{
		Host:   "example.com",
		Header: make(http.Header),
	}
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("X-Forwarded-Scheme", "https")

	result := hub.checkOrigin(req)
	if !result {
		t.Error("checkOrigin should allow same-origin with X-Forwarded-Scheme")
	}
}

func TestHub_CheckOrigin_UntrustedProxyIgnoresForwarded(t *testing.T) {
	hub := NewHub(nil)
	// No trusted proxy checker set — forwarded headers should be ignored

	req := &http.Request{
		Host:       "backend:8080",
		Header:     make(http.Header),
		RemoteAddr: "10.0.0.5:12345",
	}
	req.Header.Set("Origin", "http://proxy.example.com")
	req.Header.Set("X-Forwarded-Host", "proxy.example.com")

	result := hub.checkOrigin(req)
	if result {
		t.Error("checkOrigin should reject forwarded headers from untrusted peers")
	}
}

func TestBroadcastCurrentStateEnqueuesLazyStateRequest(t *testing.T) {
	getterCalls := 0
	hub := NewHub(func(orgID string) interface{} {
		getterCalls++
		return map[string]string{"org": orgID}
	})

	hub.BroadcastCurrentState()

	msg := <-hub.broadcastSeq
	if msg.Type != "rawData" {
		t.Fatalf("message type = %q, want rawData", msg.Type)
	}
	if _, ok := msg.Data.(stateBroadcastRequest); !ok {
		t.Fatalf("message data = %T, want stateBroadcastRequest", msg.Data)
	}
	if getterCalls != 0 {
		t.Fatalf("state getter called before coalesced flush: %d", getterCalls)
	}

	data, ok := hub.marshalBroadcastMessage(msg, "")
	if !ok {
		t.Fatal("expected lazy state request to marshal")
	}
	if getterCalls != 1 {
		t.Fatalf("state getter calls after marshal = %d, want 1", getterCalls)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal broadcast message: %v", err)
	}
	payload := decoded.Data.(map[string]interface{})
	if payload["org"] != "default" {
		t.Fatalf("lazy state org = %v, want default", payload["org"])
	}
}

func TestBroadcastCurrentStateToTenantResolvesTenantState(t *testing.T) {
	var requestedOrg string
	hub := NewHub(func(orgID string) interface{} {
		requestedOrg = orgID
		return map[string]string{"org": orgID}
	})

	hub.BroadcastCurrentStateToTenant("org-123")

	tb := <-hub.tenantBroadcast
	if tb.OrgID != "org-123" {
		t.Fatalf("tenant broadcast org = %q, want org-123", tb.OrgID)
	}
	if _, ok := tb.Message.Data.(stateBroadcastRequest); !ok {
		t.Fatalf("tenant message data = %T, want stateBroadcastRequest", tb.Message.Data)
	}

	data, ok := hub.marshalBroadcastMessage(tb.Message, tb.OrgID)
	if !ok {
		t.Fatal("expected tenant lazy state request to marshal")
	}
	if requestedOrg != "org-123" {
		t.Fatalf("state getter org = %q, want org-123", requestedOrg)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal tenant broadcast message: %v", err)
	}
	payload := decoded.Data.(map[string]interface{})
	if payload["org"] != "org-123" {
		t.Fatalf("lazy tenant state org = %v, want org-123", payload["org"])
	}
}

// startStateBroadcastHarness runs the sequencer and state worker for hub and
// registers one client, optionally in a tenant.
func startStateBroadcastHarness(t *testing.T, hub *Hub, orgID string) *Client {
	t.Helper()
	client := &Client{hub: hub, send: make(chan []byte, 16), id: "state-rate-client", orgID: orgID}
	hub.mu.Lock()
	hub.clients[client] = true
	if orgID != "" {
		if hub.clientsByTenant[orgID] == nil {
			hub.clientsByTenant[orgID] = make(map[*Client]bool)
		}
		hub.clientsByTenant[orgID][client] = true
	}
	hub.mu.Unlock()
	sequencerDone := make(chan struct{})
	go func() {
		hub.runBroadcastSequencer()
		close(sequencerDone)
	}()
	go hub.runStateBroadcastWorker()
	t.Cleanup(func() {
		close(hub.stopChan)
		<-sequencerDone
		<-hub.stateBroadcastDone
	})
	return client
}

func receiveStateValue(t *testing.T, client *Client, within time.Duration) (string, bool) {
	t.Helper()
	select {
	case data := <-client.send:
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal message: %v", err)
		}
		payload, _ := msg.Data.(map[string]interface{})
		value, _ := payload["value"].(string)
		return value, true
	case <-time.After(within):
		return "", false
	}
}

// Every accepted agent report requests a state broadcast, and each broadcast
// rebuilds the full frontend state. Broadcasts to one audience are spaced by
// stateBroadcastInterval and carry the latest state (#2199).
func TestStateBroadcastsSpacedByInterval(t *testing.T) {
	hub := NewHub(nil)
	hub.coalesceWindow = 5 * time.Millisecond
	hub.stateBroadcastInterval = 300 * time.Millisecond
	client := startStateBroadcastHarness(t, hub, "")

	hub.broadcastSeq <- Message{Type: "rawData", Data: map[string]string{"value": "first"}}
	if value, ok := receiveStateValue(t, client, time.Second); !ok || value != "first" {
		t.Fatalf("first broadcast = %q (received %v), want an immediate first broadcast", value, ok)
	}
	start := time.Now()
	for _, value := range []string{"second", "third", "fourth"} {
		hub.broadcastSeq <- Message{Type: "rawData", Data: map[string]string{"value": value}}
	}
	if value, ok := receiveStateValue(t, client, 150*time.Millisecond); ok {
		t.Fatalf("broadcast %q arrived %v after the previous one, inside the interval", value, time.Since(start))
	}
	if value, ok := receiveStateValue(t, client, 2*time.Second); !ok || value != "fourth" {
		t.Fatalf("spaced broadcast = %q (received %v), want the latest state", value, ok)
	}
	if value, ok := receiveStateValue(t, client, 100*time.Millisecond); ok {
		t.Fatalf("burst produced a second spaced broadcast %q", value)
	}
}

// A signal arriving while a broadcast is due must not push it back. With the
// earlier restart-on-signal debounce, signals closer together than the window
// postponed every broadcast until they stopped.
func TestStateBroadcastNotStarvedBySteadySignals(t *testing.T) {
	hub := NewHub(nil)
	hub.coalesceWindow = 60 * time.Millisecond
	hub.stateBroadcastInterval = 0
	client := startStateBroadcastHarness(t, hub, "")

	stop := time.After(600 * time.Millisecond)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	received := 0
	for streaming := true; streaming; {
		select {
		case <-ticker.C:
			hub.broadcastSeq <- Message{Type: "rawData", Data: map[string]string{"value": "tick"}}
		case <-client.send:
			received++
		case <-stop:
			streaming = false
		}
	}
	if received == 0 {
		t.Fatal("no state broadcast while signals kept arriving every 10 ms")
	}
}

func TestTenantStateBroadcastsSpacedByInterval(t *testing.T) {
	hub := NewHub(nil)
	hub.coalesceWindow = 5 * time.Millisecond
	hub.stateBroadcastInterval = 300 * time.Millisecond
	client := startStateBroadcastHarness(t, hub, "acme")

	send := func(value string) {
		hub.tenantBroadcast <- TenantBroadcast{OrgID: "acme", Message: Message{Type: "rawData", Data: map[string]string{"value": value}}}
	}
	send("first")
	if value, ok := receiveStateValue(t, client, time.Second); !ok || value != "first" {
		t.Fatalf("first tenant broadcast = %q (received %v)", value, ok)
	}
	send("second")
	send("third")
	if value, ok := receiveStateValue(t, client, 150*time.Millisecond); ok {
		t.Fatalf("tenant broadcast %q arrived inside the interval", value)
	}
	if value, ok := receiveStateValue(t, client, 2*time.Second); !ok || value != "third" {
		t.Fatalf("spaced tenant broadcast = %q (received %v), want the latest state", value, ok)
	}
}

func TestHubGlobalEventBoundary(t *testing.T) {
	for _, messageType := range []string{
		"discovery_started", "discovery_server_found", "discovery_progress",
		"discovery_update", "discovery_complete", "node_auto_registered",
		"nodes_changed", "node_deleted", "nodes_updated", "alert", "alertResolved",
		"custom", "future_tenant_event",
	} {
		t.Run(messageType, func(t *testing.T) {
			hub := NewHub(nil)
			go hub.Run()
			t.Cleanup(hub.Stop)
			registerTenantTestClient(t, hub, "client-a", "org-a")
			other := registerTenantTestClient(t, hub, "client-b", "org-b")
			hub.BroadcastMessage(Message{
				Type: messageType,
				Data: map[string]string{"orgId": "org-a", "subnet": "192.0.2.0/24"},
			})
			assertNoClientMessage(t, other, 50*time.Millisecond)
		})
	}
}

func TestHubTenantEventDelivery(t *testing.T) {
	for _, orgID := range []string{"org-a", "default", ""} {
		t.Run("org="+orgID, func(t *testing.T) {
			hub := NewHub(nil)
			go hub.Run()
			t.Cleanup(hub.Stop)
			owner := registerTenantTestClient(t, hub, "owner", normalizeOrgID(orgID))
			second := registerTenantTestClient(t, hub, "second", normalizeOrgID(orgID))
			other := registerTenantTestClient(t, hub, "other", "org-b")
			otherOrgID := "default"
			if normalizeOrgID(orgID) == "default" {
				otherOrgID = "org-a"
			}
			otherOrg := registerTenantTestClient(t, hub, "other-org", otherOrgID)
			for _, messageType := range []string{
				"discovery_started", "discovery_server_found", "discovery_progress",
				"discovery_update", "discovery_complete", "node_auto_registered",
				"nodes_changed", "node_deleted", "nodes_updated",
			} {
				want := Message{Type: messageType, Data: map[string]interface{}{"host": "192.0.2.10"}, Timestamp: "event-time"}
				hub.BroadcastMessageToTenant(orgID, want)
				for _, client := range []*Client{owner, second} {
					if got := readClientMessage(t, client, time.Second); !reflect.DeepEqual(got, want) {
						t.Fatalf("event changed: got %#v, want %#v", got, want)
					}
				}
			}
			// Discovery retains its existing custom envelope on the wire.
			for _, messageType := range []string{"discovery_started", "discovery_server_found", "discovery_progress", "discovery_update", "discovery_complete"} {
				hub.BroadcastToTenant(orgID, Message{Type: messageType})
				for _, client := range []*Client{owner, second} {
					got := readClientMessage(t, client, time.Second)
					payload, ok := got.Data.(map[string]interface{})
					if got.Type != "custom" || !ok || payload["type"] != messageType || got.Timestamp == "" {
						t.Fatalf("custom event changed: %#v", got)
					}
				}
			}
			assertNoClientMessage(t, other, 50*time.Millisecond)
			assertNoClientMessage(t, otherOrg, 50*time.Millisecond)
		})
	}
}

func TestHubEmptyOrgEventsBelongToDefaultTenant(t *testing.T) {
	hub := NewHub(nil)
	go hub.Run()
	t.Cleanup(hub.Stop)
	owner := registerTenantTestClient(t, hub, "legacy", "")
	other := registerTenantTestClient(t, hub, "other", "org-b")
	hub.BroadcastAlertToTenant("", map[string]string{"id": "one"})
	hub.BroadcastAlertResolvedToTenant("", "one")
	hub.BroadcastToTenant("", map[string]string{"host": "192.0.2.10"})
	for _, want := range []string{"alert", "alertResolved", "custom"} {
		if got := readClientMessage(t, owner, time.Second); got.Type != want {
			t.Fatalf("got %q, want %q", got.Type, want)
		}
	}
	assertNoClientMessage(t, other, 50*time.Millisecond)
}

func TestHubInstanceEventsReachAllTenants(t *testing.T) {
	hub := NewHub(nil)
	go hub.Run()
	t.Cleanup(hub.Stop)
	clients := []*Client{
		registerTenantTestClient(t, hub, "a", "org-a"),
		registerTenantTestClient(t, hub, "b", "org-b"),
		registerTenantTestClient(t, hub, "default", "default"),
	}
	for _, messageType := range []string{"ping", "settingsUpdate", "update:progress"} {
		hub.BroadcastMessage(Message{Type: messageType})
		for _, client := range clients {
			if got := readClientMessage(t, client, time.Second); got.Type != messageType {
				t.Fatalf("got %q, want %q", got.Type, messageType)
			}
		}
	}
}

// Every waiter has a deadline, but an uncancellable getter retains its slot:
// reconnect churn must not turn one stalled build into unbounded stuck work.
func TestHubStalledGetterBoundsReconnectWorkAndRecovers(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	hub := NewHub(nil)
	hub.stateBuildTimeout = 50 * time.Millisecond
	go hub.Run()
	t.Cleanup(func() { unblock(); hub.Stop() })
	var builds atomic.Int64
	hub.SetStateGetter(func(org string) interface{} {
		if builds.Add(1) == 1 {
			close(entered)
			<-release
		}
		return map[string]string{"org": org, "state": "current"}
	})
	for round := 0; round < 2; round++ {
		var done sync.WaitGroup
		for i := 0; i < 32; i++ {
			c := registerTenantTestClient(t, hub, "reconnecting", "org-a")
			done.Add(1)
			go func() { defer done.Done(); hub.sendRequestedState(c) }()
		}
		finished := make(chan struct{})
		go func() { done.Wait(); close(finished) }()
		select {
		case <-finished:
		case <-time.After(time.Second):
			t.Fatal("state waiters did not finish within budget")
		}
		if got := builds.Load(); got != 1 {
			t.Fatalf("reconnects started %d blocked getters, want one", got)
		}
		if hub.StateDeliveryHealthy() {
			t.Fatal("stalled getter remains healthy")
		}
		if got := hub.GetClientCount(); got != 0 {
			t.Fatalf("%d timed-out clients remain registered", got)
		}
	}
	for i := 0; i < 100; i++ {
		if hub.StateDeliveryHealthy() {
			t.Fatal("probe cleared stalled state")
		}
	}
	if builds.Load() != 1 {
		t.Fatal("health probes started work")
	}
	unblock()
	waitStateBuildReleased(t, hub)
	recovered := registerTenantTestClient(t, hub, "recovered", "org-b")
	hub.sendRequestedState(recovered)
	msg := readClientMessage(t, recovered, time.Second)
	if msg.Type != "rawData" {
		t.Fatalf("recovered type=%s", msg.Type)
	}
	payload, ok := msg.Data.(map[string]interface{})
	if !ok || payload["org"] != "org-b" || payload["state"] != "current" {
		t.Fatalf("late or cross-tenant result delivered: %v", msg.Data)
	}
	if builds.Load() != 2 || !hub.StateDeliveryHealthy() {
		t.Fatal("ordinary scoped state delivery did not recover")
	}
}

func waitStateBuildReleased(t *testing.T, hub *Hub) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for hub.stateBuildStarted.Load() != nil || len(hub.stateBuildSlot) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("returned getter did not release its slot")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestHubStalledGetterDoesNotHoldShutdown(t *testing.T) {
	for _, path := range []string{"initial", "requested", "broadcast"} {
		t.Run(path, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			hub := NewHub(nil)
			hub.stateBuildTimeout = time.Second
			hub.coalesceWindow = time.Millisecond
			hub.stateBroadcastInterval = 0
			go hub.Run()
			t.Cleanup(func() { unblock(); hub.Stop() })
			getter := func(string) interface{} { close(entered); <-release; return map[string]string{"late": "never deliver"} }
			if path == "initial" {
				hub.SetStateGetter(getter)
			}
			c := registerTenantTestClient(t, hub, "stalled", "default")
			if path != "initial" {
				hub.SetStateGetter(getter)
			}
			switch path {
			case "requested":
				go hub.sendRequestedState(c)
			case "broadcast":
				hub.BroadcastCurrentState()
			}
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("getter did not enter")
			}
			stopped := make(chan struct{})
			go func() { hub.Stop(); close(stopped) }()
			select {
			case <-stopped:
			case <-time.After(300 * time.Millisecond):
				t.Fatal("shutdown waited for blocked getter")
			}
			if len(hub.stateBuildSlot) != 1 {
				t.Fatal("shutdown freed the still-running getter's slot")
			}
			if hub.StateDeliveryHealthy() {
				t.Fatal("stopped hub reports healthy")
			}
			unblock()
			waitStateBuildReleased(t, hub)
			for raw := range c.send {
				var msg Message
				if err := json.Unmarshal(raw, &msg); err != nil {
					t.Fatal(err)
				}
				if msg.Type != "welcome" {
					t.Fatalf("shutdown delivered abandoned result: %s", raw)
				}
			}
		})
	}
}

func TestHubClientCancellationDiscardsGetterResult(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	hub := NewHub(nil)
	hub.stateBuildTimeout = time.Second
	go hub.Run()
	t.Cleanup(func() { unblock(); hub.Stop() })
	c := registerTenantTestClient(t, hub, "cancelled", "org-a")
	hub.SetStateGetter(func(string) interface{} { close(entered); <-release; return map[string]string{"secret": "org-a"} })
	done := make(chan struct{})
	go func() { hub.sendRequestedState(c); close(done) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("getter did not enter")
	}
	hub.unregister <- c
	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("disconnect did not cancel state wait")
	}
	if len(hub.stateBuildSlot) != 1 {
		t.Fatal("disconnect freed active getter slot")
	}
	unblock()
	waitStateBuildReleased(t, hub)
	for raw := range c.send {
		t.Fatalf("late state reached disconnected client: %s", raw)
	}
}

func TestHubSlowGetterAndSerializationStayHealthy(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	hub := NewHub(func(org string) interface{} { close(entered); <-release; return org })
	hub.stateBuildTimeout = time.Second
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(func() { unblock(); hub.Stop() })
	done := make(chan struct{})
	go func() {
		state, finish, ok := hub.buildStateForOrg(nil, "org-b")
		if !ok {
			t.Error("slow getter timed out")
		} else {
			if state != "org-b" {
				t.Errorf("wrong org: %v", state)
			}
			if len(hub.stateBuildSlot) != 1 {
				t.Error("slot released before encoding/queueing")
			}
			finish()
		}
		close(done)
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("getter did not enter")
	}
	if !hub.StateDeliveryHealthy() {
		t.Fatal("short build marked unhealthy")
	}
	unblock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("slow build did not complete")
	}
	waitStateBuildReleased(t, hub)
	if !hub.StateDeliveryHealthy() {
		t.Fatal("idle hub marked unhealthy")
	}
}

func TestHubTimedOutTenantBroadcastClosesOnlyItsAudience(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	hub := NewHub(nil)
	hub.stateBuildTimeout = 50 * time.Millisecond
	hub.coalesceWindow = time.Millisecond
	hub.stateBroadcastInterval = 0
	go hub.Run()
	t.Cleanup(func() { unblock(); hub.Stop() })
	a := registerTenantTestClient(t, hub, "a", "org-a")
	b := registerTenantTestClient(t, hub, "b", "org-b")
	// Deltas require an actually accepted initial snapshot. Do not mistake
	// the existing pre-hydration suppression for failed recovery.
	if _, sent, err := b.queueFullState("initialState", map[string]string{"org": "org-b", "phase": "before"}); err != nil || !sent {
		t.Fatalf("seed accepted org-b baseline: sent=%t err=%v", sent, err)
	}
	_ = readClientMessage(t, b, time.Second)
	var builds atomic.Int64
	hub.SetStateGetter(func(org string) interface{} {
		if builds.Add(1) == 1 {
			close(entered)
			<-release
		} else if org != "org-b" {
			t.Errorf("recovery getter scope = %q, want org-b", org)
		}
		return map[string]string{"org": org, "phase": "after"}
	})
	hub.BroadcastCurrentStateToTenant("org-a")
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("broadcast getter did not enter")
	}
	select {
	case <-a.lifecycleSignal():
	case <-time.After(time.Second):
		t.Fatal("timed-out audience remained live")
	}
	if !a.closed.Load() || b.closed.Load() || hub.GetClientCount() != 1 {
		t.Fatal("tenant timeout retired the wrong audience")
	}
	if hub.StateDeliveryHealthy() || builds.Load() != 1 {
		t.Fatal("stalled shared work is not faithfully retained")
	}
	unblock()
	waitStateBuildReleased(t, hub)
	hub.BroadcastCurrentStateToTenant("org-b")
	msg := readClientMessage(t, b, time.Second)
	if msg.Type != "rawData" || msg.Data.(map[string]interface{})["phase"] != "after" || builds.Load() != 2 {
		t.Fatalf("wrong recovery audience/state: %+v", msg)
	}
	for raw := range a.send {
		t.Fatalf("late result reached retired audience: %s", raw)
	}
}

// Keep the actual Gorilla upgrade/control/close path. Only the server-side
// transport's post-upgrade writes are controllably slow, without depending on
// kernel send-buffer sizes or allocating a large message to fill them.
type stateCloseConn struct {
	net.Conn
	slow     atomic.Bool
	closed   chan struct{}
	once     sync.Once
	mu       sync.Mutex
	deadline time.Time
	writes   []time.Time
}

func (c *stateCloseConn) SetWriteDeadline(deadline time.Time) error {
	c.mu.Lock()
	c.deadline = deadline
	c.mu.Unlock()
	return c.Conn.SetWriteDeadline(deadline)
}

func (c *stateCloseConn) Write(data []byte) (int, error) {
	c.mu.Lock()
	deadline := c.deadline
	c.writes = append(c.writes, deadline)
	c.mu.Unlock()
	if !c.slow.Load() {
		return c.Conn.Write(data)
	}
	if deadline.IsZero() {
		return 0, errors.New("fixture: control write has no deadline")
	}
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case <-timer.C:
		return 0, os.ErrDeadlineExceeded
	case <-c.closed:
		return 0, net.ErrClosed
	}
}

func (c *stateCloseConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

type stateCloseListener struct {
	net.Listener
	accepted chan *stateCloseConn
}

func (l stateCloseListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	wrapped := &stateCloseConn{Conn: c, closed: make(chan struct{})}
	l.accepted <- wrapped
	return wrapped, nil
}

func stateCloseSocketFactory(t *testing.T, hub *Hub) func(string, bool) (*Client, *gorillawebsocket.Conn, *stateCloseConn) {
	t.Helper()
	accepted := make(chan *stateCloseConn, 8)
	upgraded := make(chan *gorillawebsocket.Conn, 8)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&gorillawebsocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("fixture upgrade: %v", err)
			return
		}
		upgraded <- conn
	}))
	server.Listener = stateCloseListener{Listener: server.Listener, accepted: accepted}
	server.Start()
	t.Cleanup(server.Close)
	return func(org string, slow bool) (*Client, *gorillawebsocket.Conn, *stateCloseConn) {
		t.Helper()
		peer, _, err := gorillawebsocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
		if err != nil {
			t.Fatal(err)
		}
		conn, transport := <-upgraded, <-accepted
		transport.mu.Lock()
		transport.writes = nil // HTTP upgrade is not a control-frame write.
		transport.mu.Unlock()
		transport.slow.Store(slow)
		client := &Client{hub: hub, conn: conn, id: fmt.Sprint(peer.LocalAddr()), orgID: org, send: make(chan []byte, 1)}
		// No client pumps or synthetic getter are needed to test retirement.
		hub.mu.Lock()
		hub.clients[client] = true
		if hub.clientsByTenant[org] == nil {
			hub.clientsByTenant[org] = make(map[*Client]bool)
		}
		hub.clientsByTenant[org][client] = true
		hub.mu.Unlock()
		t.Cleanup(func() { _ = peer.Close(); _ = conn.Close() })
		return client, peer, transport
	}
}

func TestHubUnavailableAudienceSharesControlDeadlineAndKeepsOtherTenant(t *testing.T) {
	hub := NewHub(nil)
	dial := stateCloseSocketFactory(t, hub)
	var clients []*Client
	var peers []*gorillawebsocket.Conn
	var transports []*stateCloseConn
	for i := 0; i < 3; i++ {
		client, peer, transport := dial("org-a", false)
		clients, peers, transports = append(clients, client), append(peers, peer), append(transports, transport)
	}
	other, otherPeer, _ := dial("org-b", false)
	hub.closeUnavailableStateAudience("org-a")
	var common time.Time
	for i, client := range clients {
		_ = peers[i].SetReadDeadline(time.Now().Add(time.Second))
		_, _, err := peers[i].ReadMessage()
		var closed *gorillawebsocket.CloseError
		if !errors.As(err, &closed) || closed.Code != gorillawebsocket.CloseTryAgainLater || closed.Text != "State delivery temporarily unavailable" {
			t.Fatalf("client %d close = %v", i, err)
		}
		if !client.closed.Load() {
			t.Fatal("closed socket remained in the audience")
		}
		transports[i].mu.Lock()
		writes := append([]time.Time(nil), transports[i].writes...)
		transports[i].mu.Unlock()
		if len(writes) != 1 || writes[0].IsZero() {
			t.Fatalf("client %d control deadlines = %v", i, writes)
		}
		if i == 0 {
			common = writes[0]
		} else if !writes[0].Equal(common) {
			t.Fatalf("audience received a new deadline per client: %v != %v", writes[0], common)
		}
	}
	if other.closed.Load() || hub.GetClientCount() != 1 || hub.GetTenantClientCount("org-a") != 0 || hub.GetTenantClientCount("org-b") != 1 {
		t.Fatal("retirement crossed the tenant audience")
	}
	if err := other.conn.WriteMessage(gorillawebsocket.TextMessage, []byte("org-b-still-live")); err != nil {
		t.Fatal(err)
	}
	_ = otherPeer.SetReadDeadline(time.Now().Add(time.Second))
	if _, body, err := otherPeer.ReadMessage(); err != nil || string(body) != "org-b-still-live" {
		t.Fatalf("other tenant lost delivery: %q, %v", body, err)
	}
}

func TestHubUnavailableAudienceSlowWritesHaveOneBudget(t *testing.T) {
	hub := NewHub(nil)
	dial := stateCloseSocketFactory(t, hub)
	var transports []*stateCloseConn
	for i := 0; i < 5; i++ {
		_, _, transport := dial("org-a", true)
		transports = append(transports, transport)
	}
	started := time.Now()
	hub.closeUnavailableStateAudience("org-a")
	elapsed := time.Since(started)
	// The former per-client deadline consumed five seconds. Allow scheduling
	// room without relying on sub-millisecond comparisons as the sole proof;
	// the responsive-socket test checks exact common deadline identity too.
	if elapsed > 3*time.Second {
		t.Fatalf("five slow peers multiplied the one-second budget: %v", elapsed)
	}
	if hub.GetClientCount() != 0 || hub.GetTenantClientCount("org-a") != 0 {
		t.Fatal("slow clients remain registered")
	}
	for _, transport := range transports {
		select {
		case <-transport.closed:
		default:
			t.Fatal("expired control budget left a socket open")
		}
	}
	t.Logf("five slow peers retired in %v", elapsed)
}
