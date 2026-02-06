package relay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

var testUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// mockRelayServer creates an httptest.Server that speaks the relay protocol.
// It returns the server and a channel to receive the instance-side connection.
func mockRelayServer(t *testing.T, handler func(conn *websocket.Conn)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()
		handler(conn)
	}))
}

func wsURL(server *httptest.Server) string {
	return "ws" + strings.TrimPrefix(server.URL, "http")
}

func TestClient_RegisterAndChannelLifecycle(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	// Track what the mock relay receives
	var mu sync.Mutex
	var registerReceived bool
	var channelOpenAckReceived bool
	dataResponseCh := make(chan ProxyResponse, 1)

	// Mock local Pulse API
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": r.URL.Path})
	}))
	defer mockAPI.Close()
	localAddr := strings.TrimPrefix(mockAPI.URL, "http://")

	// Mock relay server
	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		// 1. Read REGISTER
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Logf("read register: %v", err)
			return
		}
		frame, err := DecodeFrame(msg)
		if err != nil {
			t.Logf("decode register: %v", err)
			return
		}
		if frame.Type != FrameRegister {
			t.Logf("expected REGISTER, got %s", FrameTypeName(frame.Type))
			return
		}

		var regPayload RegisterPayload
		UnmarshalControlPayload(frame.Payload, &regPayload)
		mu.Lock()
		registerReceived = regPayload.LicenseToken == "test-license-jwt" &&
			regPayload.IdentityPubKey == "test-identity-pub-key"
		mu.Unlock()

		// 2. Send REGISTER_ACK
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_abc",
			SessionToken: "sess_xyz",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		// 3. Send CHANNEL_OPEN
		chOpen, _ := NewControlFrame(FrameChannelOpen, 1, ChannelOpenPayload{
			ChannelID: 1,
			AuthToken: "valid-api-token",
		})
		chOpenBytes, _ := EncodeFrame(chOpen)
		time.Sleep(50 * time.Millisecond) // let client set up
		conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

		// 4. Read CHANNEL_OPEN ack from instance
		_, msg, err = conn.ReadMessage()
		if err != nil {
			t.Logf("read channel open ack: %v", err)
			return
		}
		frame, _ = DecodeFrame(msg)
		mu.Lock()
		channelOpenAckReceived = frame.Type == FrameChannelOpen
		mu.Unlock()

		// 5. Send DATA request
		proxyReq := ProxyRequest{
			ID:     "req_test",
			Method: "GET",
			Path:   "/api/status",
		}
		proxyReqBytes, _ := json.Marshal(proxyReq)
		dataFrame := NewFrame(FrameData, 1, proxyReqBytes)
		dataBytes, _ := EncodeFrame(dataFrame)
		conn.WriteMessage(websocket.BinaryMessage, dataBytes)

		// 6. Read DATA response
		_, msg, err = conn.ReadMessage()
		if err != nil {
			t.Logf("read data response: %v", err)
			return
		}
		frame, _ = DecodeFrame(msg)
		if frame.Type == FrameData {
			var resp ProxyResponse
			json.Unmarshal(frame.Payload, &resp)
			dataResponseCh <- resp
		}

		// 7. Send CHANNEL_CLOSE
		chClose, _ := NewControlFrame(FrameChannelClose, 1, ChannelClosePayload{
			ChannelID: 1,
			Reason:    "test done",
		})
		chCloseBytes, _ := EncodeFrame(chClose)
		conn.WriteMessage(websocket.BinaryMessage, chCloseBytes)

		// Keep connection open so the client stays connected during assertions
		time.Sleep(2 * time.Second)
	})
	defer relayServer.Close()

	cfg := Config{
		Enabled:   true,
		ServerURL: wsURL(relayServer),
	}

	deps := ClientDeps{
		LicenseTokenFunc: func() string { return "test-license-jwt" },
		TokenValidator:   func(token string) bool { return token == "valid-api-token" },
		LocalAddr:        localAddr,
		ServerVersion:    "1.0.0-test",
		IdentityPubKey:   "test-identity-pub-key",
	}

	client := NewClient(cfg, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- client.Run(ctx)
	}()

	// Wait for DATA response
	select {
	case resp := <-dataResponseCh:
		if resp.ID != "req_test" {
			t.Errorf("response ID: got %q, want %q", resp.ID, "req_test")
		}
		if resp.Status != 200 {
			t.Errorf("response status: got %d, want 200", resp.Status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for DATA response")
	}

	// Verify state
	mu.Lock()
	if !registerReceived {
		t.Error("REGISTER not received or had wrong license token")
	}
	if !channelOpenAckReceived {
		t.Error("CHANNEL_OPEN ack not received")
	}
	mu.Unlock()

	// Wait a bit for CHANNEL_CLOSE to be processed
	time.Sleep(200 * time.Millisecond)

	status := client.Status()
	if !status.Connected {
		t.Error("expected client to be connected")
	}
	if status.InstanceID != "inst_abc" {
		t.Errorf("instance_id: got %q, want %q", status.InstanceID, "inst_abc")
	}

	cancel()
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Fatal("client.Run didn't return after cancel")
	}
}

func TestClient_RejectInvalidToken(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	channelCloseCh := make(chan ChannelClosePayload, 1)

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		// Read REGISTER
		_, msg, _ := conn.ReadMessage()
		frame, _ := DecodeFrame(msg)
		if frame.Type != FrameRegister {
			return
		}

		// Send REGISTER_ACK
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_abc",
			SessionToken: "sess_xyz",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		// Send CHANNEL_OPEN with bad token
		time.Sleep(50 * time.Millisecond)
		chOpen, _ := NewControlFrame(FrameChannelOpen, 99, ChannelOpenPayload{
			ChannelID: 99,
			AuthToken: "INVALID-TOKEN",
		})
		chOpenBytes, _ := EncodeFrame(chOpen)
		conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

		// Read CHANNEL_CLOSE (reject)
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		frame, _ = DecodeFrame(msg)
		if frame.Type == FrameChannelClose {
			var closePayload ChannelClosePayload
			UnmarshalControlPayload(frame.Payload, &closePayload)
			channelCloseCh <- closePayload
		}

		time.Sleep(100 * time.Millisecond)
	})
	defer relayServer.Close()

	cfg := Config{
		Enabled:   true,
		ServerURL: wsURL(relayServer),
	}

	deps := ClientDeps{
		LicenseTokenFunc: func() string { return "test-jwt" },
		TokenValidator:   func(token string) bool { return token == "good-token" },
		LocalAddr:        "127.0.0.1:9999",
		ServerVersion:    "1.0.0",
		IdentityPubKey:   "test-pub-key",
	}

	client := NewClient(cfg, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	select {
	case closePayload := <-channelCloseCh:
		if closePayload.ChannelID != 99 {
			t.Errorf("channel ID: got %d, want 99", closePayload.ChannelID)
		}
		if closePayload.Reason != "invalid auth token" {
			t.Errorf("reason: got %q, want %q", closePayload.Reason, "invalid auth token")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for CHANNEL_CLOSE")
	}

	cancel()
	<-errCh
}

func TestClient_DrainTriggersReconnect(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	connectCount := 0
	var connectMu sync.Mutex

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		// Read REGISTER
		_, msg, _ := conn.ReadMessage()
		frame, _ := DecodeFrame(msg)
		if frame.Type != FrameRegister {
			return
		}

		connectMu.Lock()
		connectCount++
		count := connectCount
		connectMu.Unlock()

		// Send REGISTER_ACK
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_abc",
			SessionToken: "sess_xyz",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		if count == 1 {
			// First connection: send DRAIN after a short delay
			time.Sleep(100 * time.Millisecond)
			drain, _ := NewControlFrame(FrameDrain, 0, DrainPayload{
				Reason: "server shutting down",
			})
			drainBytes, _ := EncodeFrame(drain)
			conn.WriteMessage(websocket.BinaryMessage, drainBytes)
		} else {
			// Second connection: keep alive briefly
			time.Sleep(500 * time.Millisecond)
		}
	})
	defer relayServer.Close()

	cfg := Config{
		Enabled:   true,
		ServerURL: wsURL(relayServer),
	}

	deps := ClientDeps{
		LicenseTokenFunc: func() string { return "test-jwt" },
		TokenValidator:   func(token string) bool { return true },
		LocalAddr:        "127.0.0.1:9999",
		ServerVersion:    "1.0.0",
		IdentityPubKey:   "test-pub-key",
	}

	client := NewClient(cfg, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	// Wait for second connection
	deadline := time.After(8 * time.Second)
	for {
		connectMu.Lock()
		c := connectCount
		connectMu.Unlock()
		if c >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for reconnect, connect count: %d", c)
		case <-time.After(100 * time.Millisecond):
		}
	}

	cancel()
	<-errCh
}

func TestClient_SessionTokenReuse(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	sessionTokens := make(chan string, 2)

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		_, msg, _ := conn.ReadMessage()
		frame, _ := DecodeFrame(msg)
		if frame.Type != FrameRegister {
			return
		}

		var regPayload RegisterPayload
		UnmarshalControlPayload(frame.Payload, &regPayload)
		sessionTokens <- regPayload.SessionToken

		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_abc",
			SessionToken: "server-issued-session-token",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		// Close after a short delay to trigger reconnect
		time.Sleep(100 * time.Millisecond)
		conn.Close()
	})
	defer relayServer.Close()

	cfg := Config{
		Enabled:   true,
		ServerURL: wsURL(relayServer),
	}

	deps := ClientDeps{
		LicenseTokenFunc: func() string { return "test-jwt" },
		TokenValidator:   func(token string) bool { return true },
		LocalAddr:        "127.0.0.1:9999",
		ServerVersion:    "1.0.0",
		IdentityPubKey:   "test-pub-key",
	}

	client := NewClient(cfg, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	// First connection: no session token
	select {
	case token := <-sessionTokens:
		if token != "" {
			t.Errorf("first connection session_token: got %q, want empty", token)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for first REGISTER")
	}

	// Second connection: should reuse session token from first REGISTER_ACK
	select {
	case token := <-sessionTokens:
		if token != "server-issued-session-token" {
			t.Errorf("second connection session_token: got %q, want %q", token, "server-issued-session-token")
		}
	case <-time.After(8 * time.Second):
		t.Fatal("timed out waiting for second REGISTER")
	}

	cancel()
	<-errCh
}
