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

// pollUntil polls the given check function every 20ms until it returns true
// or the deadline is reached. Fails the test on timeout.
func pollUntil(t *testing.T, deadline time.Duration, desc string, check func() bool) {
	t.Helper()
	timer := time.After(deadline)
	for {
		if check() {
			return
		}
		select {
		case <-timer:
			t.Fatalf("timed out waiting for: %s", desc)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// TestClient_DrainDuringInFlightData verifies that when the relay server sends
// a DRAIN frame while DATA handlers are actively processing requests:
//
//  1. The DRAIN is received and processed by the read pump.
//  2. In-flight DATA handler goroutines are cancelled (connCtx is cancelled).
//  3. The client reconnects cleanly to the relay server.
//  4. No goroutine leaks or panics occur from writing to a closed connection.
//
// This is the expected behavior: DRAIN signals a graceful server shutdown, and
// in-flight requests are expected to be retried by the mobile app on the new
// connection. The relay client does not attempt to finish pending work before
// reconnecting.
//
// The first connection handler keeps the WebSocket alive for 30 seconds after
// sending DRAIN. If the client did NOT process DRAIN (e.g. handler was broken),
// the test would timeout because the client would stay on the first connection
// and never reach the second one.
func TestClient_DrainDuringInFlightData(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	// Track API handler lifecycle
	var apiRequestStarted atomic.Int32
	var apiRequestCancelled atomic.Int32
	apiBlocking := make(chan struct{}) // closed when API handler should unblock

	// Mock local Pulse API with a slow endpoint
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/slow-resource" {
			apiRequestStarted.Add(1)
			// Block until context is cancelled (simulates slow API work)
			select {
			case <-r.Context().Done():
				apiRequestCancelled.Add(1)
				return
			case <-apiBlocking:
				// Safety valve — shouldn't be reached in normal test flow
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": r.URL.Path})
	}))
	defer mockAPI.Close()
	defer close(apiBlocking) // ensure any blocked handlers are released on cleanup
	localAddr := strings.TrimPrefix(mockAPI.URL, "http://")

	var connectMu sync.Mutex
	connectCount := 0

	// Track that second connection succeeds
	secondConnRegistered := make(chan struct{}, 1)

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		// REGISTER
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Logf("read register: %v", err)
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

		// REGISTER_ACK
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_drain_inflight",
			SessionToken: "sess_drain",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		if count == 1 {
			// First connection: open channel, send DATA, then DRAIN while DATA is in-flight

			// Open channel
			time.Sleep(50 * time.Millisecond)
			chOpen, _ := NewControlFrame(FrameChannelOpen, 1, ChannelOpenPayload{
				ChannelID: 1,
				AuthToken: "valid-token",
			})
			chOpenBytes, _ := EncodeFrame(chOpen)
			_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

			// Read CHANNEL_OPEN ack
			_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, msg, err = conn.ReadMessage()
			if err != nil {
				t.Logf("read channel open ack: %v", err)
				return
			}
			frame, _ = DecodeFrame(msg)
			if frame.Type != FrameChannelOpen {
				t.Logf("expected CHANNEL_OPEN ack, got %s", FrameTypeName(frame.Type))
				return
			}
			_ = conn.SetReadDeadline(time.Time{})

			// Send DATA request that will block in the API handler
			slowReq := ProxyRequest{
				ID:     "req_slow_inflight",
				Method: "GET",
				Path:   "/api/slow-resource",
			}
			slowReqBytes, _ := json.Marshal(slowReq)
			dataFrame := NewFrame(FrameData, 1, slowReqBytes)
			dataBytes, _ := EncodeFrame(dataFrame)
			_ = conn.WriteMessage(websocket.BinaryMessage, dataBytes)

			// Wait for the API handler to start processing
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

			// Now send DRAIN while the DATA handler is blocked
			drain, _ := NewControlFrame(FrameDrain, 0, DrainPayload{
				Reason: "server maintenance",
			})
			drainBytes, _ := EncodeFrame(drain)
			_ = conn.WriteMessage(websocket.BinaryMessage, drainBytes)

			// Keep connection alive long enough that only DRAIN (not handler
			// exit closing the socket) can explain the client reconnecting
			// within the test timeout.
			time.Sleep(30 * time.Second)
		} else {
			// Second connection: signal success and stay alive
			select {
			case secondConnRegistered <- struct{}{}:
			default:
			}
			time.Sleep(30 * time.Second)
		}
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	// 1. Wait for second connection (proves DRAIN triggered reconnect, not handler exit)
	select {
	case <-secondConnRegistered:
		// Success: client reconnected after DRAIN
	case <-time.After(12 * time.Second):
		t.Fatal("timed out waiting for reconnection after DRAIN with in-flight DATA")
	}

	// 2. Verify the in-flight API request was started
	if started := apiRequestStarted.Load(); started != 1 {
		t.Errorf("expected 1 API request started, got %d", started)
	}

	// 3. Verify the in-flight request was cancelled (context cancellation from connCtx)
	pollUntil(t, 3*time.Second, "API request cancellation", func() bool {
		return apiRequestCancelled.Load() >= 1
	})

	// 4. Verify final state: connected on second connection
	connectMu.Lock()
	finalCount := connectCount
	connectMu.Unlock()
	if finalCount < 2 {
		t.Errorf("expected at least 2 connections, got %d", finalCount)
	}

	pollUntil(t, 3*time.Second, "client connected after reconnect", func() bool {
		return client.Status().Connected
	})

	cancel()
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Fatal("client.Run didn't return after cancel")
	}
}

// TestClient_DrainWithMultipleInFlightChannels verifies DRAIN behavior when
// multiple channels have active DATA handlers. All in-flight handlers across
// all channels should be cancelled, and the client should reconnect cleanly.
//
// Like the single-channel test, the first connection handler stays alive for
// 30 seconds, so only DRAIN can explain the fast reconnect.
func TestClient_DrainWithMultipleInFlightChannels(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	var apiRequestsStarted atomic.Int32
	var apiRequestsCancelled atomic.Int32
	apiBlocking := make(chan struct{})

	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/slow") {
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
			InstanceID:   "inst_multi_drain",
			SessionToken: "sess_multi",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		if count == 1 {
			time.Sleep(50 * time.Millisecond)

			// Open two channels
			for _, chID := range []uint32{1, 2} {
				chOpen, _ := NewControlFrame(FrameChannelOpen, chID, ChannelOpenPayload{
					ChannelID: chID,
					AuthToken: "valid-token",
				})
				chOpenBytes, _ := EncodeFrame(chOpen)
				_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)
			}

			// Read two CHANNEL_OPEN acks
			for i := 0; i < 2; i++ {
				_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
				_, _, err = conn.ReadMessage()
				if err != nil {
					t.Logf("read channel ack %d: %v", i, err)
					return
				}
			}
			_ = conn.SetReadDeadline(time.Time{})

			// Send DATA on both channels (both will block)
			for i, chID := range []uint32{1, 2} {
				req := ProxyRequest{
					ID:     "req_multi_" + string(rune('a'+i)),
					Method: "GET",
					Path:   "/api/slow/" + string(rune('1'+i)),
				}
				reqBytes, _ := json.Marshal(req)
				df := NewFrame(FrameData, chID, reqBytes)
				dfBytes, _ := EncodeFrame(df)
				_ = conn.WriteMessage(websocket.BinaryMessage, dfBytes)
			}

			// Wait for both API handlers to start
			deadline := time.After(3 * time.Second)
			for {
				if apiRequestsStarted.Load() >= 2 {
					break
				}
				select {
				case <-deadline:
					t.Log("timed out waiting for both API requests to start")
					return
				case <-time.After(10 * time.Millisecond):
				}
			}

			// DRAIN while both handlers are in-flight
			drain, _ := NewControlFrame(FrameDrain, 0, DrainPayload{
				Reason: "rolling update",
			})
			drainBytes, _ := EncodeFrame(drain)
			_ = conn.WriteMessage(websocket.BinaryMessage, drainBytes)

			// Keep connection alive — only DRAIN can trigger the fast reconnect
			time.Sleep(30 * time.Second)
		} else {
			select {
			case secondConnRegistered <- struct{}{}:
			default:
			}
			time.Sleep(30 * time.Second)
		}
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	// Wait for reconnection
	select {
	case <-secondConnRegistered:
	case <-time.After(12 * time.Second):
		t.Fatal("timed out waiting for reconnection after DRAIN with multiple in-flight channels")
	}

	// Both API requests should have been started
	if started := apiRequestsStarted.Load(); started != 2 {
		t.Errorf("expected 2 API requests started, got %d", started)
	}

	// Both should have been cancelled via context
	pollUntil(t, 3*time.Second, "both API requests cancelled", func() bool {
		return apiRequestsCancelled.Load() >= 2
	})

	// Channels from first connection should be cleared
	pollUntil(t, 3*time.Second, "client connected after reconnect", func() bool {
		return client.Status().Connected
	})

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

// TestClient_DrainAfterCompletedData verifies the sequence where a DATA
// request completes normally and the response is delivered to the relay,
// followed by DRAIN. This tests that completed work is not affected by
// subsequent DRAIN, and the client reconnects cleanly with zero active
// channels on the new connection.
func TestClient_DrainAfterCompletedData(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	dataResponseCh := make(chan ProxyResponse, 1)
	var connectMu sync.Mutex
	connectCount := 0
	secondConnRegistered := make(chan struct{}, 1)

	// Fast-responding mock API
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": r.URL.Path, "status": "ok"})
	}))
	defer mockAPI.Close()
	localAddr := strings.TrimPrefix(mockAPI.URL, "http://")

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
			InstanceID:   "inst_drain_completed",
			SessionToken: "sess_completed",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		if count == 1 {
			time.Sleep(50 * time.Millisecond)

			// Open channel
			chOpen, _ := NewControlFrame(FrameChannelOpen, 1, ChannelOpenPayload{
				ChannelID: 1,
				AuthToken: "valid-token",
			})
			chOpenBytes, _ := EncodeFrame(chOpen)
			_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

			// Read CHANNEL_OPEN ack
			_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, msg, err = conn.ReadMessage()
			if err != nil {
				t.Logf("read channel ack: %v", err)
				return
			}
			_ = conn.SetReadDeadline(time.Time{})

			// Send a fast DATA request
			req := ProxyRequest{
				ID:     "req_fast",
				Method: "GET",
				Path:   "/api/status",
			}
			reqBytes, _ := json.Marshal(req)
			df := NewFrame(FrameData, 1, reqBytes)
			dfBytes, _ := EncodeFrame(df)
			_ = conn.WriteMessage(websocket.BinaryMessage, dfBytes)

			// Read the DATA response (should arrive quickly since API is fast)
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
				dataResponseCh <- resp
			}
			_ = conn.SetReadDeadline(time.Time{})

			// Now send DRAIN after the response was already delivered
			drain, _ := NewControlFrame(FrameDrain, 0, DrainPayload{
				Reason: "graceful restart",
			})
			drainBytes, _ := EncodeFrame(drain)
			_ = conn.WriteMessage(websocket.BinaryMessage, drainBytes)

			// Keep connection alive — only DRAIN can trigger the fast reconnect
			time.Sleep(30 * time.Second)
		} else {
			select {
			case secondConnRegistered <- struct{}{}:
			default:
			}
			time.Sleep(30 * time.Second)
		}
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	// Verify the fast DATA response was delivered before DRAIN
	select {
	case resp := <-dataResponseCh:
		if resp.ID != "req_fast" {
			t.Errorf("response ID: got %q, want %q", resp.ID, "req_fast")
		}
		if resp.Status != 200 {
			t.Errorf("response status: got %d, want 200", resp.Status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for fast DATA response")
	}

	// Verify reconnection happens after DRAIN
	select {
	case <-secondConnRegistered:
	case <-time.After(12 * time.Second):
		t.Fatal("timed out waiting for reconnection after DRAIN")
	}

	// Poll for connected status
	pollUntil(t, 3*time.Second, "client connected after drain+reconnect", func() bool {
		return client.Status().Connected
	})

	if status := client.Status(); status.ActiveChannels != 0 {
		t.Errorf("expected 0 active channels after reconnect (channels belong to old connection), got %d", status.ActiveChannels)
	}

	cancel()
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Fatal("client.Run didn't return after cancel")
	}
}
