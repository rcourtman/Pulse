package websocket

import (
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

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
	stateGetter := func() interface{} {
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

	newGetter := func() interface{} {
		return "new state"
	}
	hub.SetStateGetter(newGetter)

	hub.mu.RLock()
	defer hub.mu.RUnlock()

	if hub.getState == nil {
		t.Error("getState should be set after SetStateGetter")
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
		// No origin header - always allowed for non-browser clients
		{
			name:     "no origin header",
			origin:   "",
			host:     "localhost:8080",
			expected: true,
		},

		// Same-origin requests
		{
			name:     "same origin http",
			origin:   "http://localhost:8080",
			host:     "localhost:8080",
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
