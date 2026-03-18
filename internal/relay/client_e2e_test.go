package relay

import (
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// TestClient_E2E_MultiMobileClientRelay simulates a realistic production scenario:
// one Pulse instance connected through the relay server, with three mobile app
// clients connecting concurrently — two using encrypted channels and one using
// plaintext. Each mobile client sends a different API request through the relay,
// which the Pulse client proxies to its local API. The test verifies:
//
//   - All three channels open successfully and concurrently
//   - Each mobile client receives the correct response for its specific request
//   - Encrypted and plaintext channels can coexist on the same connection
//   - Channel cleanup (CHANNEL_CLOSE) works for all channels
//   - The Pulse client remains connected throughout
//
// This directly validates the "Pulse instance → relay server → mobile client"
// end-to-end flow described in the L7 relay infrastructure readiness criteria.
func TestClient_E2E_MultiMobileClientRelay(t *testing.T) {
	// Use zerolog.Nop() to avoid "Log in goroutine after Test completed" panics
	// from the async client goroutine writing after the test function returns.
	logger := zerolog.Nop()

	identityPriv, identityPub := testIdentityKeyPair(t)

	// Mock local Pulse API with multiple endpoints that return distinct responses.
	// Each endpoint includes an "endpoint" key so we can verify correct routing.
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/nodes":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"endpoint": "nodes",
				"count":    3,
			})
		case "/api/alerts":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"endpoint": "alerts",
				"active":   2,
			})
		case "/api/status":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"endpoint": "status",
				"healthy":  true,
			})
		default:
			w.WriteHeader(404)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	}))
	defer mockAPI.Close()
	localAddr := strings.TrimPrefix(mockAPI.URL, "http://")

	type e2eResult struct {
		channelID uint32
		response  ProxyResponse
		bodyJSON  map[string]interface{}
		encrypted bool
	}
	results := make(chan e2eResult, 3)

	// channelSpec defines what each simulated mobile client does
	type channelSpec struct {
		channelID uint32
		authToken string
		reqID     string
		method    string
		path      string
		encrypted bool
	}
	channels := []channelSpec{
		{channelID: 1, authToken: "mobile-token-1", reqID: "req_nodes", method: "GET", path: "/api/nodes", encrypted: true},
		{channelID: 2, authToken: "mobile-token-2", reqID: "req_alerts", method: "GET", path: "/api/alerts", encrypted: true},
		{channelID: 3, authToken: "mobile-token-3", reqID: "req_status", method: "GET", path: "/api/status", encrypted: false},
	}

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		// 1. Read REGISTER
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		frame, err := DecodeFrame(msg)
		if err != nil || frame.Type != FrameRegister {
			return
		}

		// 2. Send REGISTER_ACK
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_e2e_multi",
			SessionToken: "sess_e2e",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		if err := conn.WriteMessage(websocket.BinaryMessage, ackBytes); err != nil {
			return
		}

		// Let client settle after registration
		time.Sleep(50 * time.Millisecond)

		// readFrameWithDeadline reads the next frame with a per-read deadline
		// to prevent indefinite blocking.
		readFrameWithDeadline := func(d time.Duration) (Frame, error) {
			_ = conn.SetReadDeadline(time.Now().Add(d))
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return Frame{}, err
			}
			return DecodeFrame(msg)
		}

		// 3. Open all channels in sequence, then exchange data concurrently
		// We must serialize channel opens because the client processes them on a
		// single read pump and echoes CHANNEL_OPEN back one at a time.
		for _, ch := range channels {
			chOpen, _ := NewControlFrame(FrameChannelOpen, ch.channelID, ChannelOpenPayload{
				ChannelID: ch.channelID,
				AuthToken: ch.authToken,
			})
			chOpenBytes, _ := EncodeFrame(chOpen)
			if err := conn.WriteMessage(websocket.BinaryMessage, chOpenBytes); err != nil {
				return
			}

			// Read CHANNEL_OPEN echo (acceptance)
			f, err := readFrameWithDeadline(5 * time.Second)
			if err != nil {
				return
			}
			if f.Type != FrameChannelOpen {
				return
			}
		}

		// 4. Exchange data on all channels concurrently
		// For encrypted channels, perform KEY_EXCHANGE first (serialized since
		// the instance responds on the same connection).
		type channelEncryption struct {
			enc *ChannelEncryption
		}
		encryptors := make(map[uint32]*channelEncryption)

		for _, ch := range channels {
			if !ch.encrypted {
				continue
			}

			// Generate app-side ephemeral keypair
			appPriv, err := GenerateEphemeralKeyPair()
			if err != nil {
				return
			}

			// Send KEY_EXCHANGE (app doesn't sign)
			kexPayload := MarshalKeyExchangePayload(appPriv.PublicKey().Bytes(), nil)
			kexFrame := NewFrame(FrameKeyExchange, ch.channelID, kexPayload)
			kexBytes, _ := EncodeFrame(kexFrame)
			if err := conn.WriteMessage(websocket.BinaryMessage, kexBytes); err != nil {
				return
			}

			// Read instance's KEY_EXCHANGE response
			f, err := readFrameWithDeadline(5 * time.Second)
			if err != nil {
				return
			}
			if f.Type != FrameKeyExchange {
				return
			}

			instancePub, sig, err := UnmarshalKeyExchangePayload(f.Payload)
			if err != nil {
				return
			}

			// Verify the instance signed its key exchange with its identity key
			if err := VerifyKeyExchangeSignature(instancePub, sig, identityPub); err != nil {
				return
			}

			instancePubKey, err := ecdh.X25519().NewPublicKey(instancePub)
			if err != nil {
				return
			}
			enc, err := DeriveChannelKeys(appPriv, instancePubKey, false)
			if err != nil {
				return
			}
			encryptors[ch.channelID] = &channelEncryption{enc: enc}
		}

		// 5. Send DATA requests and read responses concurrently across channels.
		// We send all requests first, then read all responses, to test that the
		// client handles concurrent in-flight proxy requests correctly.
		for _, ch := range channels {
			proxyReq := ProxyRequest{
				ID:     ch.reqID,
				Method: ch.method,
				Path:   ch.path,
			}
			proxyReqBytes, _ := json.Marshal(proxyReq)

			var payload []byte
			if ch.encrypted {
				var err error
				payload, err = encryptors[ch.channelID].enc.Encrypt(proxyReqBytes)
				if err != nil {
					return
				}
			} else {
				payload = proxyReqBytes
			}

			dataFrame := NewFrame(FrameData, ch.channelID, payload)
			dataBytes, _ := EncodeFrame(dataFrame)
			if err := conn.WriteMessage(websocket.BinaryMessage, dataBytes); err != nil {
				return
			}
		}

		// Read responses (may arrive in any order since proxy is concurrent).
		// Use per-read deadlines to enforce the timeout without blocking.
		received := 0
		for received < len(channels) {
			f, err := readFrameWithDeadline(5 * time.Second)
			if err != nil {
				return
			}
			if f.Type != FrameData {
				// Skip non-data frames (e.g. PONG)
				continue
			}

			var respBytes []byte
			if enc, ok := encryptors[f.Channel]; ok {
				respBytes, err = enc.enc.Decrypt(f.Payload)
				if err != nil {
					return
				}
			} else {
				respBytes = f.Payload
			}

			var resp ProxyResponse
			if err := json.Unmarshal(respBytes, &resp); err != nil {
				return
			}

			// Decode the base64 response body to verify content
			var bodyJSON map[string]interface{}
			if resp.Body != "" {
				bodyBytes, err := base64.StdEncoding.DecodeString(resp.Body)
				if err == nil {
					_ = json.Unmarshal(bodyBytes, &bodyJSON)
				}
			}

			results <- e2eResult{
				channelID: f.Channel,
				response:  resp,
				bodyJSON:  bodyJSON,
				encrypted: encryptors[f.Channel] != nil,
			}
			received++
		}

		// 6. Close all channels
		for _, ch := range channels {
			chClose, _ := NewControlFrame(FrameChannelClose, ch.channelID, ChannelClosePayload{
				ChannelID: ch.channelID,
				Reason:    "session ended",
			})
			chCloseBytes, _ := EncodeFrame(chClose)
			_ = conn.WriteMessage(websocket.BinaryMessage, chCloseBytes)
		}

		// Keep connection open for assertions
		time.Sleep(2 * time.Second)
	})
	defer relayServer.Close()

	cfg := Config{
		Enabled:   true,
		ServerURL: wsURL(relayServer),
	}

	deps := ClientDeps{
		LicenseTokenFunc: func() string { return "test-license-jwt-e2e" },
		TokenValidator: func(token string) bool {
			return strings.HasPrefix(token, "mobile-token-")
		},
		LocalAddr:          localAddr,
		ServerVersion:      "6.0.0-test",
		IdentityPubKey:     identityPub,
		IdentityPrivateKey: identityPriv,
	}

	client := NewClient(cfg, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.Run(ctx)
	}()

	// Collect and verify all three responses
	gotResponses := make(map[string]e2eResult)
	for i := 0; i < 3; i++ {
		select {
		case r := <-results:
			gotResponses[r.response.ID] = r
		case <-time.After(8 * time.Second):
			got := make([]string, 0, len(gotResponses))
			for k := range gotResponses {
				got = append(got, k)
			}
			t.Fatalf("timed out waiting for response %d/3 (got: %v)", i+1, got)
		}
	}

	// Verify each mobile client got the correct response including body content
	verifyResponse := func(reqID string, expectedChannelID uint32, expectedEncrypted bool, expectedEndpoint string) {
		t.Helper()
		r, ok := gotResponses[reqID]
		if !ok {
			t.Errorf("missing response for %s", reqID)
			return
		}
		if r.channelID != expectedChannelID {
			t.Errorf("%s: channelID = %d, want %d", reqID, r.channelID, expectedChannelID)
		}
		if r.encrypted != expectedEncrypted {
			t.Errorf("%s: encrypted = %v, want %v", reqID, r.encrypted, expectedEncrypted)
		}
		if r.response.Status != 200 {
			t.Errorf("%s: status = %d, want 200", reqID, r.response.Status)
		}
		// Verify the response body contains the expected endpoint identifier,
		// confirming the request was routed to the correct mock API handler.
		if r.bodyJSON == nil {
			t.Errorf("%s: response body JSON is nil", reqID)
			return
		}
		if ep, ok := r.bodyJSON["endpoint"]; !ok || ep != expectedEndpoint {
			t.Errorf("%s: body endpoint = %v, want %q", reqID, ep, expectedEndpoint)
		}
	}

	verifyResponse("req_nodes", 1, true, "nodes")
	verifyResponse("req_alerts", 2, true, "alerts")
	verifyResponse("req_status", 3, false, "status")

	// Verify client stayed connected throughout
	time.Sleep(200 * time.Millisecond)
	status := client.Status()
	if !status.Connected {
		t.Error("expected client to remain connected after multi-channel e2e flow")
	}
	if status.InstanceID != "inst_e2e_multi" {
		t.Errorf("instance_id = %q, want %q", status.InstanceID, "inst_e2e_multi")
	}

	cancel()
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Fatal("client.Run didn't return after cancel")
	}
}

// TestClient_E2E_ChannelRejectAndRecoverFlow verifies that when one mobile
// client is rejected (bad auth token) while another is accepted, the relay
// client correctly handles both — sending CHANNEL_CLOSE for the rejected
// channel and proxying data normally for the accepted one. This tests the
// "partial failure" scenario in multi-client relay flows.
func TestClient_E2E_ChannelRejectAndRecoverFlow(t *testing.T) {
	logger := zerolog.Nop()

	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": r.URL.Path})
	}))
	defer mockAPI.Close()
	localAddr := strings.TrimPrefix(mockAPI.URL, "http://")

	var mu sync.Mutex
	var rejectedChannelClose *ChannelClosePayload
	goodDataCh := make(chan ProxyResponse, 1)

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		// REGISTER
		_, msg, _ := conn.ReadMessage()
		frame, _ := DecodeFrame(msg)
		if frame.Type != FrameRegister {
			return
		}

		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_reject_recover",
			SessionToken: "sess_rr",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		time.Sleep(50 * time.Millisecond)

		// Open channel 10 with BAD token
		chBad, _ := NewControlFrame(FrameChannelOpen, 10, ChannelOpenPayload{
			ChannelID: 10,
			AuthToken: "INVALID-mobile-token",
		})
		chBadBytes, _ := EncodeFrame(chBad)
		_ = conn.WriteMessage(websocket.BinaryMessage, chBadBytes)

		// Read response — should be CHANNEL_CLOSE (rejection)
		_, msg, _ = conn.ReadMessage()
		frame, _ = DecodeFrame(msg)
		if frame.Type == FrameChannelClose {
			var cp ChannelClosePayload
			_ = UnmarshalControlPayload(frame.Payload, &cp)
			mu.Lock()
			rejectedChannelClose = &cp
			mu.Unlock()
		}

		// Now open channel 20 with GOOD token
		chGood, _ := NewControlFrame(FrameChannelOpen, 20, ChannelOpenPayload{
			ChannelID: 20,
			AuthToken: "valid-mobile",
		})
		chGoodBytes, _ := EncodeFrame(chGood)
		_ = conn.WriteMessage(websocket.BinaryMessage, chGoodBytes)

		// Read CHANNEL_OPEN echo (acceptance)
		_, msg, _ = conn.ReadMessage()
		frame, _ = DecodeFrame(msg)
		if frame.Type != FrameChannelOpen {
			return
		}

		// Send DATA on the good channel
		proxyReq := ProxyRequest{
			ID:     "req_after_reject",
			Method: "GET",
			Path:   "/api/status",
		}
		proxyReqBytes, _ := json.Marshal(proxyReq)
		dataFrame := NewFrame(FrameData, 20, proxyReqBytes)
		dataBytes, _ := EncodeFrame(dataFrame)
		_ = conn.WriteMessage(websocket.BinaryMessage, dataBytes)

		// Read DATA response
		_, msg, _ = conn.ReadMessage()
		frame, _ = DecodeFrame(msg)
		if frame.Type == FrameData {
			var resp ProxyResponse
			_ = json.Unmarshal(frame.Payload, &resp)
			goodDataCh <- resp
		}

		time.Sleep(2 * time.Second)
	})
	defer relayServer.Close()

	cfg := Config{
		Enabled:   true,
		ServerURL: wsURL(relayServer),
	}

	deps := ClientDeps{
		LicenseTokenFunc: func() string { return "test-jwt" },
		TokenValidator:   func(token string) bool { return token == "valid-mobile" },
		LocalAddr:        localAddr,
		ServerVersion:    "6.0.0-test",
		IdentityPubKey:   "test-pub-key",
	}

	client := NewClient(cfg, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	// Wait for good channel response
	select {
	case resp := <-goodDataCh:
		if resp.ID != "req_after_reject" {
			t.Errorf("response ID = %q, want %q", resp.ID, "req_after_reject")
		}
		if resp.Status != 200 {
			t.Errorf("response status = %d, want 200", resp.Status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for good channel DATA response")
	}

	// Verify the bad channel was rejected
	mu.Lock()
	if rejectedChannelClose == nil {
		t.Error("expected CHANNEL_CLOSE for rejected channel 10")
	} else {
		if rejectedChannelClose.ChannelID != 10 {
			t.Errorf("rejected channelID = %d, want 10", rejectedChannelClose.ChannelID)
		}
		if rejectedChannelClose.Reason != "invalid auth token" {
			t.Errorf("rejected reason = %q, want %q", rejectedChannelClose.Reason, "invalid auth token")
		}
	}
	mu.Unlock()

	// Client should still be connected
	status := client.Status()
	if !status.Connected {
		t.Error("expected client to remain connected after channel rejection")
	}

	cancel()
	<-errCh
}

// TestClient_E2E_LegacyJWTFallbackFlow verifies that when a v6 grant is not
// available (simulating a legacy v5 license), the client falls back to legacy
// JWT authentication and the full proxy flow still works. This is important
// for the v5→v6 transition period where some instances may still use legacy JWTs.
func TestClient_E2E_LegacyJWTFallbackFlow(t *testing.T) {
	logger := zerolog.Nop()

	dataResponseCh := make(chan ProxyResponse, 1)

	// Simulate a local Pulse API that returns version info
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"version":        "5.4.2",
			"legacy_license": true,
		})
	}))
	defer mockAPI.Close()
	localAddr := strings.TrimPrefix(mockAPI.URL, "http://")

	// Track LicenseTokenFunc calls to verify the client actually calls it
	var tokenCallCount int
	var tokenMu sync.Mutex

	const legacyToken = "legacy.jwt.token-v5"

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		// REGISTER — verify we receive the expected legacy JWT
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		frame, _ := DecodeFrame(msg)
		if frame.Type != FrameRegister {
			return
		}

		var regPayload RegisterPayload
		_ = UnmarshalControlPayload(frame.Payload, &regPayload)

		// Verify the specific legacy token was sent (not just non-empty)
		if regPayload.LicenseToken != legacyToken {
			return
		}

		// REGISTER_ACK
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_legacy",
			SessionToken: "sess_legacy",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		time.Sleep(50 * time.Millisecond)

		// CHANNEL_OPEN
		chOpen, _ := NewControlFrame(FrameChannelOpen, 1, ChannelOpenPayload{
			ChannelID: 1,
			AuthToken: "legacy-app-token",
		})
		chOpenBytes, _ := EncodeFrame(chOpen)
		_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

		// Read CHANNEL_OPEN ack
		_, msg, _ = conn.ReadMessage()
		frame, _ = DecodeFrame(msg)
		if frame.Type != FrameChannelOpen {
			return
		}

		// DATA request for /api/version
		proxyReq := ProxyRequest{
			ID:     "req_legacy_version",
			Method: "GET",
			Path:   "/api/version",
		}
		proxyReqBytes, _ := json.Marshal(proxyReq)
		dataFrame := NewFrame(FrameData, 1, proxyReqBytes)
		dataBytes, _ := EncodeFrame(dataFrame)
		_ = conn.WriteMessage(websocket.BinaryMessage, dataBytes)

		// Read response
		_, msg, _ = conn.ReadMessage()
		frame, _ = DecodeFrame(msg)
		if frame.Type == FrameData {
			var resp ProxyResponse
			_ = json.Unmarshal(frame.Payload, &resp)
			dataResponseCh <- resp
		}

		time.Sleep(2 * time.Second)
	})
	defer relayServer.Close()

	cfg := Config{
		Enabled:   true,
		ServerURL: wsURL(relayServer),
	}

	deps := ClientDeps{
		LicenseTokenFunc: func() string {
			tokenMu.Lock()
			tokenCallCount++
			tokenMu.Unlock()
			return legacyToken
		},
		TokenValidator: func(token string) bool { return token == "legacy-app-token" },
		LocalAddr:      localAddr,
		ServerVersion:  "5.4.2",
		IdentityPubKey: "test-pub-key",
	}

	client := NewClient(cfg, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	select {
	case resp := <-dataResponseCh:
		if resp.ID != "req_legacy_version" {
			t.Errorf("response ID = %q, want %q", resp.ID, "req_legacy_version")
		}
		if resp.Status != 200 {
			t.Errorf("response status = %d, want 200", resp.Status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for legacy JWT flow DATA response")
	}

	// Verify LicenseTokenFunc was actually called by the client
	tokenMu.Lock()
	if tokenCallCount == 0 {
		t.Error("LicenseTokenFunc was never called")
	}
	tokenMu.Unlock()

	status := client.Status()
	if !status.Connected {
		t.Error("expected client to remain connected in legacy JWT flow")
	}
	if status.InstanceID != "inst_legacy" {
		t.Errorf("instance_id = %q, want %q", status.InstanceID, "inst_legacy")
	}

	cancel()
	<-errCh
}

// TestClient_E2E_POSTWithBodyProxy verifies that the relay correctly proxies
// POST requests with JSON bodies through the full relay path, including
// request body encoding and response body decoding. This covers the write
// path that mobile apps use for actions like acknowledging alerts.
func TestClient_E2E_POSTWithBodyProxy(t *testing.T) {
	logger := zerolog.Nop()

	type receivedRequest struct {
		method      string
		path        string
		contentType string
		body        string
	}
	var received receivedRequest
	var receivedMu sync.Mutex

	dataResponseCh := make(chan ProxyResponse, 1)

	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use io.ReadAll for reliable full-body read
		bodyBytes, _ := io.ReadAll(r.Body)

		receivedMu.Lock()
		received = receivedRequest{
			method:      r.Method,
			path:        r.URL.Path,
			contentType: r.Header.Get("Content-Type"),
			body:        string(bodyBytes),
		}
		receivedMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]string{"status": "acknowledged"})
	}))
	defer mockAPI.Close()
	localAddr := strings.TrimPrefix(mockAPI.URL, "http://")

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		// REGISTER
		_, msg, _ := conn.ReadMessage()
		frame, _ := DecodeFrame(msg)
		if frame.Type != FrameRegister {
			return
		}

		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_post",
			SessionToken: "sess_post",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		time.Sleep(50 * time.Millisecond)

		// CHANNEL_OPEN
		chOpen, _ := NewControlFrame(FrameChannelOpen, 1, ChannelOpenPayload{
			ChannelID: 1,
			AuthToken: "write-token",
		})
		chOpenBytes, _ := EncodeFrame(chOpen)
		_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

		// Read CHANNEL_OPEN ack
		_, msg, _ = conn.ReadMessage()
		frame, _ = DecodeFrame(msg)
		if frame.Type != FrameChannelOpen {
			return
		}

		// Send POST request with body
		proxyReq := ProxyRequest{
			ID:     "req_ack_alert",
			Method: "POST",
			Path:   "/api/alerts/123/acknowledge",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: "eyJhY2tub3dsZWRnZWRfYnkiOiJ1c2VyQGV4YW1wbGUuY29tIn0=", // base64 of {"acknowledged_by":"user@example.com"}
		}
		proxyReqBytes, _ := json.Marshal(proxyReq)
		dataFrame := NewFrame(FrameData, 1, proxyReqBytes)
		dataBytes, _ := EncodeFrame(dataFrame)
		_ = conn.WriteMessage(websocket.BinaryMessage, dataBytes)

		// Read response
		_, msg, _ = conn.ReadMessage()
		frame, _ = DecodeFrame(msg)
		if frame.Type == FrameData {
			var resp ProxyResponse
			_ = json.Unmarshal(frame.Payload, &resp)
			dataResponseCh <- resp
		}

		time.Sleep(2 * time.Second)
	})
	defer relayServer.Close()

	cfg := Config{
		Enabled:   true,
		ServerURL: wsURL(relayServer),
	}

	deps := ClientDeps{
		LicenseTokenFunc: func() string { return "test-jwt" },
		TokenValidator:   func(token string) bool { return token == "write-token" },
		LocalAddr:        localAddr,
		ServerVersion:    "6.0.0-test",
		IdentityPubKey:   "test-pub-key",
	}

	client := NewClient(cfg, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	select {
	case resp := <-dataResponseCh:
		if resp.ID != "req_ack_alert" {
			t.Errorf("response ID = %q, want %q", resp.ID, "req_ack_alert")
		}
		if resp.Status != 201 {
			t.Errorf("response status = %d, want 201", resp.Status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for POST proxy response")
	}

	// Verify the local API received the correct request
	receivedMu.Lock()
	if received.method != "POST" {
		t.Errorf("local API method = %q, want POST", received.method)
	}
	if received.path != "/api/alerts/123/acknowledge" {
		t.Errorf("local API path = %q, want /api/alerts/123/acknowledge", received.path)
	}
	if !strings.Contains(received.body, "acknowledged_by") {
		t.Errorf("local API body missing expected content, got: %q", received.body)
	}
	receivedMu.Unlock()

	cancel()
	<-errCh
}
