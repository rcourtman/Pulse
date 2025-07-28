package websocket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestWebSocketMessages(t *testing.T) {
	// Create test state
	testState := models.State{
		Nodes: []models.Node{{
			ID:     "test-node",
			Name:   "test-node",
			Status: "online",
			Memory: models.Memory{Used: 1000, Total: 2000},
		}},
		VMs: []models.VM{{
			ID:         "test-vm",
			VMID:       100,
			Name:       "test-vm",
			NetworkIn:  5000,
			NetworkOut: 6000,
		}},
		LastUpdate: time.Now(),
	}

	// Create hub with state getter
	hub := NewHub(func() interface{} {
		return testState.ToFrontend()
	})
	go hub.Run()
	defer close(hub.broadcast)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWebSocket(w, r)
	}))
	defer server.Close()

	// Connect WebSocket client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	defer ws.Close()

	// Test initial state message
	t.Run("Initial State", func(t *testing.T) {
		var msg Message
		if err := ws.ReadJSON(&msg); err != nil {
			t.Fatalf("Failed to read initial state: %v", err)
		}

		if msg.Type != "initialState" {
			t.Errorf("Expected message type 'initialState', got '%s'", msg.Type)
		}

		// Validate the data structure
		data, err := json.Marshal(msg.Data)
		if err != nil {
			t.Fatalf("Failed to marshal message data: %v", err)
		}

		var state models.StateFrontend
		if err := json.Unmarshal(data, &state); err != nil {
			t.Fatalf("Failed to unmarshal state: %v", err)
		}

		// Check frontend field names
		if len(state.Nodes) != 1 {
			t.Errorf("Expected 1 node, got %d", len(state.Nodes))
		}
		if len(state.Nodes) > 0 && state.Nodes[0].Mem != 1000 {
			t.Errorf("Expected node mem 1000, got %d", state.Nodes[0].Mem)
		}

		if len(state.VMs) != 1 {
			t.Errorf("Expected 1 VM, got %d", len(state.VMs))
		}
		if len(state.VMs) > 0 && state.VMs[0].NetIn != 5000 {
			t.Errorf("Expected VM netin 5000, got %d", state.VMs[0].NetIn)
		}
	})

	// Test broadcast state
	t.Run("Broadcast State", func(t *testing.T) {
		// Update state and broadcast
		updatedState := testState
		updatedState.VMs[0].NetworkIn = 7000
		hub.BroadcastState(updatedState.ToFrontend())

		// Set read deadline to avoid hanging
		ws.SetReadDeadline(time.Now().Add(2 * time.Second))

		// Read broadcast message
		var msg Message
		if err := ws.ReadJSON(&msg); err != nil {
			t.Fatalf("Failed to read broadcast: %v", err)
		}

		if msg.Type != "rawData" {
			t.Errorf("Expected message type 'rawData', got '%s'", msg.Type)
		}

		// Validate updated data
		data, _ := json.Marshal(msg.Data)
		var state models.StateFrontend
		json.Unmarshal(data, &state)

		if len(state.VMs) > 0 && state.VMs[0].NetIn != 7000 {
			t.Errorf("Expected updated VM netin 7000, got %d", state.VMs[0].NetIn)
		}
	})
}

// TestMessageSanitization ensures NaN/Inf values are handled
func TestMessageSanitization(t *testing.T) {
	// Test data with problematic values
	testData := map[string]interface{}{
		"cpu":    0.0 / 0.0,        // NaN
		"memory": 1.0 / 0.0,        // Inf
		"disk":   -1.0 / 0.0,       // -Inf
		"normal": 42.5,
		"nested": map[string]interface{}{
			"value": 0.0 / 0.0, // NaN in nested object
		},
	}

	sanitized := sanitizeData(testData)
	
	// Check sanitization
	result := sanitized.(map[string]interface{})
	
	if result["cpu"] != 0.0 {
		t.Errorf("Expected NaN to be sanitized to 0, got %v", result["cpu"])
	}
	if result["memory"] != 0.0 {
		t.Errorf("Expected Inf to be sanitized to 0, got %v", result["memory"])
	}
	if result["disk"] != 0.0 {
		t.Errorf("Expected -Inf to be sanitized to 0, got %v", result["disk"])
	}
	if result["normal"] != 42.5 {
		t.Errorf("Expected normal value to remain 42.5, got %v", result["normal"])
	}
	
	nested := result["nested"].(map[string]interface{})
	if nested["value"] != 0.0 {
		t.Errorf("Expected nested NaN to be sanitized to 0, got %v", nested["value"])
	}
}