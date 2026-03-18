package relay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// TestClient_AbruptDisconnectCancelsInFlightHandlers verifies that when the
// relay server abruptly closes the WebSocket (server crash, network partition)
// while DATA handlers are actively processing:
//
//  1. connCtx is cancelled, which propagates to all in-flight DATA goroutines.
//  2. Channel state is cleaned up (channels map reset to empty).
//  3. The client reconnects successfully.
func TestClient_AbruptDisconnectCancelsInFlightHandlers(t *testing.T) {
	// Use Nop logger to avoid panics from async goroutines logging after test completion.
	logger := zerolog.Nop()

	var apiRequestStarted atomic.Int32
	var apiRequestCancelled atomic.Int32
	apiBlocking := make(chan struct{})

	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/slow-query" {
			apiRequestStarted.Add(1)
			select {
			case <-r.Context().Done():
				apiRequestCancelled.Add(1)
				return
			case <-apiBlocking:
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer mockAPI.Close()
	defer close(apiBlocking)
	localAddr := strings.TrimPrefix(mockAPI.URL, "http://")

	var connectMu sync.Mutex
	connectCount := 0
	secondConnRegistered := make(chan struct{}, 1)

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		frame, err := DecodeFrame(msg)
		if err != nil || frame.Type != FrameRegister {
			return
		}

		connectMu.Lock()
		connectCount++
		count := connectCount
		connectMu.Unlock()

		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_abrupt",
			SessionToken: "sess_abrupt",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		if count == 1 {
			time.Sleep(50 * time.Millisecond)

			// Open a channel
			chOpen, _ := NewControlFrame(FrameChannelOpen, 1, ChannelOpenPayload{
				ChannelID: 1,
				AuthToken: "valid-token",
			})
			chOpenBytes, _ := EncodeFrame(chOpen)
			_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

			// Read CHANNEL_OPEN ack
			_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, _, err = conn.ReadMessage()
			if err != nil {
				t.Logf("read channel ack: %v", err)
				return
			}
			_ = conn.SetReadDeadline(time.Time{})

			// Send DATA that will block in the API handler
			req := ProxyRequest{
				ID:     "req_will_be_interrupted",
				Method: "GET",
				Path:   "/api/slow-query",
			}
			reqBytes, _ := json.Marshal(req)
			dataFrame := NewFrame(FrameData, 1, reqBytes)
			dataBytes, _ := EncodeFrame(dataFrame)
			_ = conn.WriteMessage(websocket.BinaryMessage, dataBytes)

			// Wait for API handler to start
			deadline := time.After(3 * time.Second)
			for {
				if apiRequestStarted.Load() > 0 {
					break
				}
				select {
				case <-deadline:
					t.Log("timed out waiting for API request to start")
					return
				case <-time.After(10 * time.Millisecond):
				}
			}

			// Abruptly close the connection (simulates server crash / network partition)
			// No DRAIN frame, no close message — just drop.
			conn.Close()
			return
		}

		// Second connection: signal success and stay alive
		select {
		case secondConnRegistered <- struct{}{}:
		default:
		}
		time.Sleep(30 * time.Second)
	})
	defer relayServer.Close()

	cfg := Config{
		Enabled:   true,
		ServerURL: wsURL(relayServer),
	}

	deps := ClientDeps{
		LicenseTokenFunc: func() string { return "test-jwt" },
		TokenValidator:   func(token string) bool { return token == "valid-token" },
		LocalAddr:        localAddr,
		ServerVersion:    "1.0.0",
		IdentityPubKey:   "test-pub-key",
	}

	client := NewClient(cfg, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	// Wait for reconnection (proves abrupt close triggered reconnect)
	select {
	case <-secondConnRegistered:
		// Success: client reconnected after abrupt disconnect
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for reconnection after abrupt disconnect")
	}

	// Verify the in-flight request was started
	if started := apiRequestStarted.Load(); started != 1 {
		t.Errorf("expected 1 API request started, got %d", started)
	}

	// Verify the in-flight request was cancelled via connCtx
	pollUntil(t, 3*time.Second, "API request cancellation after abrupt disconnect", func() bool {
		return apiRequestCancelled.Load() >= 1
	})

	// Verify reconnection is healthy
	pollUntil(t, 3*time.Second, "client connected after abrupt reconnect", func() bool {
		return client.Status().Connected
	})

	// Channels from old connection should be cleared
	if status := client.Status(); status.ActiveChannels != 0 {
		t.Errorf("expected 0 active channels after reconnect, got %d", status.ActiveChannels)
	}

	cancel()
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Fatal("client.Run didn't return after cancel")
	}
}

// TestClient_AbruptDisconnectMultipleChannelCleanup verifies that when the
// relay server abruptly disconnects while multiple channels have active DATA
// handlers:
//
//  1. All in-flight handlers across all channels are cancelled.
//  2. The channels map is completely cleared on reconnect (no stale entries).
//  3. New channels on the second connection work independently.
func TestClient_AbruptDisconnectMultipleChannelCleanup(t *testing.T) {
	// Use Nop logger to avoid panics from async goroutines logging after test completion.
	logger := zerolog.Nop()

	var apiRequestsStarted atomic.Int32
	var apiRequestsCancelled atomic.Int32
	apiBlocking := make(chan struct{})

	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/blocking") {
			apiRequestsStarted.Add(1)
			select {
			case <-r.Context().Done():
				apiRequestsCancelled.Add(1)
				return
			case <-apiBlocking:
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": r.URL.Path})
	}))
	defer mockAPI.Close()
	defer close(apiBlocking)
	localAddr := strings.TrimPrefix(mockAPI.URL, "http://")

	var connectMu sync.Mutex
	connectCount := 0
	secondConnRegistered := make(chan struct{}, 1)
	newChannelDataResponse := make(chan ProxyResponse, 1)

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		frame, err := DecodeFrame(msg)
		if err != nil || frame.Type != FrameRegister {
			return
		}

		connectMu.Lock()
		connectCount++
		count := connectCount
		connectMu.Unlock()

		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_multi_abrupt",
			SessionToken: "sess_multi_abrupt",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		if count == 1 {
			time.Sleep(50 * time.Millisecond)

			// Open three channels
			for _, chID := range []uint32{10, 20, 30} {
				chOpen, _ := NewControlFrame(FrameChannelOpen, chID, ChannelOpenPayload{
					ChannelID: chID,
					AuthToken: "valid-token",
				})
				chOpenBytes, _ := EncodeFrame(chOpen)
				_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)
			}

			// Read three CHANNEL_OPEN acks
			for i := 0; i < 3; i++ {
				_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
				_, _, err = conn.ReadMessage()
				if err != nil {
					t.Logf("read channel ack %d: %v", i, err)
					return
				}
			}
			_ = conn.SetReadDeadline(time.Time{})

			// Send blocking DATA on all three channels
			for _, chID := range []uint32{10, 20, 30} {
				req := ProxyRequest{
					ID:     "req_block_" + string(rune('a'+chID/10-1)),
					Method: "GET",
					Path:   "/api/blocking/" + string(rune('0'+chID/10)),
				}
				reqBytes, _ := json.Marshal(req)
				df := NewFrame(FrameData, chID, reqBytes)
				dfBytes, _ := EncodeFrame(df)
				_ = conn.WriteMessage(websocket.BinaryMessage, dfBytes)
			}

			// Wait for all three API handlers to start
			deadline := time.After(3 * time.Second)
			for {
				if apiRequestsStarted.Load() >= 3 {
					break
				}
				select {
				case <-deadline:
					t.Logf("timed out: only %d of 3 API requests started", apiRequestsStarted.Load())
					return
				case <-time.After(10 * time.Millisecond):
				}
			}

			// Abruptly close — all 3 in-flight handlers should be cancelled
			conn.Close()
			return
		}

		// Second connection: open a new channel and verify it works
		select {
		case secondConnRegistered <- struct{}{}:
		default:
		}

		time.Sleep(50 * time.Millisecond)

		// Open a fresh channel on the new connection
		chOpen, _ := NewControlFrame(FrameChannelOpen, 99, ChannelOpenPayload{
			ChannelID: 99,
			AuthToken: "valid-token",
		})
		chOpenBytes, _ := EncodeFrame(chOpen)
		_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

		// Read CHANNEL_OPEN ack
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, msg, err = conn.ReadMessage()
		if err != nil {
			t.Logf("read new channel ack: %v", err)
			return
		}
		_ = conn.SetReadDeadline(time.Time{})

		// Send a fast DATA request on the new channel
		fastReq := ProxyRequest{
			ID:     "req_new_channel",
			Method: "GET",
			Path:   "/api/status",
		}
		fastReqBytes, _ := json.Marshal(fastReq)
		df := NewFrame(FrameData, 99, fastReqBytes)
		dfBytes, _ := EncodeFrame(df)
		_ = conn.WriteMessage(websocket.BinaryMessage, dfBytes)

		// Read the DATA response
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, msg, err = conn.ReadMessage()
		if err != nil {
			t.Logf("read data response: %v", err)
			return
		}
		frame, _ = DecodeFrame(msg)
		if frame.Type == FrameData {
			var resp ProxyResponse
			_ = json.Unmarshal(frame.Payload, &resp)
			newChannelDataResponse <- resp
		}
		_ = conn.SetReadDeadline(time.Time{})

		time.Sleep(30 * time.Second)
	})
	defer relayServer.Close()

	cfg := Config{
		Enabled:   true,
		ServerURL: wsURL(relayServer),
	}

	deps := ClientDeps{
		LicenseTokenFunc: func() string { return "test-jwt" },
		TokenValidator:   func(token string) bool { return token == "valid-token" },
		LocalAddr:        localAddr,
		ServerVersion:    "1.0.0",
		IdentityPubKey:   "test-pub-key",
	}

	client := NewClient(cfg, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	// Wait for second connection
	select {
	case <-secondConnRegistered:
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for reconnection after abrupt disconnect with 3 channels")
	}

	// All three API requests should have been started and cancelled
	if started := apiRequestsStarted.Load(); started != 3 {
		t.Errorf("expected 3 API requests started, got %d", started)
	}

	pollUntil(t, 3*time.Second, "all 3 API requests cancelled", func() bool {
		return apiRequestsCancelled.Load() >= 3
	})

	// Verify the new channel on the second connection works
	select {
	case resp := <-newChannelDataResponse:
		if resp.ID != "req_new_channel" {
			t.Errorf("new channel response ID: got %q, want %q", resp.ID, "req_new_channel")
		}
		if resp.Status != 200 {
			t.Errorf("new channel response status: got %d, want 200", resp.Status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for DATA response on new channel after reconnect")
	}

	// Verify channel count reflects only the new channel
	pollUntil(t, 3*time.Second, "client has exactly 1 active channel", func() bool {
		return client.Status().ActiveChannels == 1
	})

	cancel()
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Fatal("client.Run didn't return after cancel")
	}
}

// TestClient_AbruptDisconnectPreservesSessionToken verifies that after an
// abrupt disconnect (no DRAIN, no close frame), the client preserves the
// session token and sends it in the REGISTER on the next connection. This
// ensures the relay server can resume the instance session rather than
// creating a new one.
func TestClient_AbruptDisconnectPreservesSessionToken(t *testing.T) {
	// Use Nop logger to avoid panics from async goroutines logging after test completion.
	logger := zerolog.Nop()

	sessionTokens := make(chan string, 3)

	var connectMu sync.Mutex
	connectCount := 0

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		frame, err := DecodeFrame(msg)
		if err != nil || frame.Type != FrameRegister {
			return
		}

		var regPayload RegisterPayload
		_ = UnmarshalControlPayload(frame.Payload, &regPayload)
		sessionTokens <- regPayload.SessionToken

		connectMu.Lock()
		connectCount++
		count := connectCount
		connectMu.Unlock()

		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_session_preserve",
			SessionToken: "server-session-token-v1",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		if count == 1 {
			// First connection: abruptly close after a small delay
			time.Sleep(100 * time.Millisecond)
			conn.Close()
			return
		}

		// Second connection: stay alive
		time.Sleep(30 * time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	// First connection: no session token (fresh connect)
	select {
	case token := <-sessionTokens:
		if token != "" {
			t.Errorf("first connection: expected empty session token, got %q", token)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first REGISTER")
	}

	// Second connection: should reuse session token despite abrupt disconnect
	select {
	case token := <-sessionTokens:
		if token != "server-session-token-v1" {
			t.Errorf("second connection: expected %q, got %q", "server-session-token-v1", token)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for second REGISTER")
	}

	cancel()
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Fatal("client.Run didn't return after cancel")
	}
}
