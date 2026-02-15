package relay

import (
	"bytes"
	"context"
	"crypto/ecdh"
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
		_ = UnmarshalControlPayload(frame.Payload, &regPayload)
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
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		// 3. Send CHANNEL_OPEN
		chOpen, _ := NewControlFrame(FrameChannelOpen, 1, ChannelOpenPayload{
			ChannelID: 1,
			AuthToken: "valid-api-token",
		})
		chOpenBytes, _ := EncodeFrame(chOpen)
		time.Sleep(50 * time.Millisecond) // let client set up
		_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

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
		_ = conn.WriteMessage(websocket.BinaryMessage, dataBytes)

		// 6. Read DATA response
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

		// 7. Send CHANNEL_CLOSE
		chClose, _ := NewControlFrame(FrameChannelClose, 1, ChannelClosePayload{
			ChannelID: 1,
			Reason:    "test done",
		})
		chCloseBytes, _ := EncodeFrame(chClose)
		_ = conn.WriteMessage(websocket.BinaryMessage, chCloseBytes)

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
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		// Send CHANNEL_OPEN with bad token
		time.Sleep(50 * time.Millisecond)
		chOpen, _ := NewControlFrame(FrameChannelOpen, 99, ChannelOpenPayload{
			ChannelID: 99,
			AuthToken: "INVALID-TOKEN",
		})
		chOpenBytes, _ := EncodeFrame(chOpen)
		_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

		// Read CHANNEL_CLOSE (reject)
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		frame, _ = DecodeFrame(msg)
		if frame.Type == FrameChannelClose {
			var closePayload ChannelClosePayload
			_ = UnmarshalControlPayload(frame.Payload, &closePayload)
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
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		if count == 1 {
			// First connection: send DRAIN after a short delay
			time.Sleep(100 * time.Millisecond)
			drain, _ := NewControlFrame(FrameDrain, 0, DrainPayload{
				Reason: "server shutting down",
			})
			drainBytes, _ := EncodeFrame(drain)
			_ = conn.WriteMessage(websocket.BinaryMessage, drainBytes)
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
		_ = UnmarshalControlPayload(frame.Payload, &regPayload)
		sessionTokens <- regPayload.SessionToken

		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_abc",
			SessionToken: "server-issued-session-token",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

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

func TestClient_RejectsOversizedWebSocketMessage(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		// Read REGISTER
		_, msg, _ := conn.ReadMessage()
		frame, _ := DecodeFrame(msg)
		if frame.Type != FrameRegister {
			return
		}

		// Send REGISTER_ACK
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_oversized",
			SessionToken: "sess_oversized",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		// Send one oversized websocket binary message. This should trip
		// conn.SetReadLimit and force the client to drop/reconnect.
		oversized := make([]byte, wsReadLimit+1)
		_ = conn.WriteMessage(websocket.BinaryMessage, oversized)

		time.Sleep(200 * time.Millisecond)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	deadline := time.After(3 * time.Second)
	observed := false
	for !observed {
		status := client.Status()
		if strings.Contains(status.LastError, "read limit exceeded") {
			observed = true
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for read-limit error, last error=%q", status.LastError)
		case <-time.After(50 * time.Millisecond):
		}
	}

	cancel()

	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("client.Run did not return after cancel on idle connection")
	}

}

// testIdentityKeyPair generates an Ed25519 keypair for testing and returns
// the base64 private key and public key.
func testIdentityKeyPair(t *testing.T) (privB64, pubB64 string) {
	t.Helper()
	priv, pub, _, err := GenerateIdentityKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	return priv, pub
}

func TestClient_EncryptedChannelLifecycle(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	identityPriv, identityPub := testIdentityKeyPair(t)

	dataResponseCh := make(chan ProxyResponse, 1)

	// Mock local Pulse API
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": r.URL.Path, "encrypted": "true"})
	}))
	defer mockAPI.Close()
	localAddr := strings.TrimPrefix(mockAPI.URL, "http://")

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		// 1. Read REGISTER
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Logf("read register: %v", err)
			return
		}
		frame, _ := DecodeFrame(msg)
		if frame.Type != FrameRegister {
			t.Logf("expected REGISTER, got %s", FrameTypeName(frame.Type))
			return
		}

		// 2. Send REGISTER_ACK
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_enc",
			SessionToken: "sess_enc",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		// 3. Send CHANNEL_OPEN
		chOpen, _ := NewControlFrame(FrameChannelOpen, 10, ChannelOpenPayload{
			ChannelID: 10,
			AuthToken: "valid-token",
		})
		chOpenBytes, _ := EncodeFrame(chOpen)
		time.Sleep(50 * time.Millisecond)
		_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

		// 4. Read CHANNEL_OPEN ack
		_, msg, _ = conn.ReadMessage()
		frame, _ = DecodeFrame(msg)
		if frame.Type != FrameChannelOpen {
			t.Logf("expected CHANNEL_OPEN ack, got %s", FrameTypeName(frame.Type))
			return
		}

		// 5. Initiate key exchange: generate app's ephemeral keypair
		appPriv, err := GenerateEphemeralKeyPair()
		if err != nil {
			t.Logf("generate app keypair: %v", err)
			return
		}

		// Send KEY_EXCHANGE from "app" (no signature — app doesn't sign)
		kexPayload := MarshalKeyExchangePayload(appPriv.PublicKey().Bytes(), nil)
		kexFrame := NewFrame(FrameKeyExchange, 10, kexPayload)
		kexBytes, _ := EncodeFrame(kexFrame)
		_ = conn.WriteMessage(websocket.BinaryMessage, kexBytes)

		// 6. Read instance's KEY_EXCHANGE response
		_, msg, err = conn.ReadMessage()
		if err != nil {
			t.Logf("read key exchange: %v", err)
			return
		}
		frame, _ = DecodeFrame(msg)
		if frame.Type != FrameKeyExchange {
			t.Logf("expected KEY_EXCHANGE, got %s", FrameTypeName(frame.Type))
			return
		}

		instancePub, sig, err := UnmarshalKeyExchangePayload(frame.Payload)
		if err != nil {
			t.Logf("unmarshal key exchange: %v", err)
			return
		}

		// Verify signature
		if err := VerifyKeyExchangeSignature(instancePub, sig, identityPub); err != nil {
			t.Logf("key exchange signature verification failed: %v", err)
			return
		}

		// Derive keys on mock-relay side (acting as app)
		instancePubKey, err := ecdh.X25519().NewPublicKey(instancePub)
		if err != nil {
			t.Logf("parse instance pubkey: %v", err)
			return
		}
		appEnc, err := DeriveChannelKeys(appPriv, instancePubKey, false)
		if err != nil {
			t.Logf("derive channel keys: %v", err)
			return
		}

		// 7. Send encrypted DATA request
		proxyReq := ProxyRequest{
			ID:     "req_encrypted",
			Method: "GET",
			Path:   "/api/status",
		}
		proxyReqBytes, _ := json.Marshal(proxyReq)
		encryptedReq, err := appEnc.Encrypt(proxyReqBytes)
		if err != nil {
			t.Logf("encrypt request: %v", err)
			return
		}
		dataFrame := NewFrame(FrameData, 10, encryptedReq)
		dataBytes, _ := EncodeFrame(dataFrame)
		_ = conn.WriteMessage(websocket.BinaryMessage, dataBytes)

		// 8. Read encrypted DATA response
		_, msg, err = conn.ReadMessage()
		if err != nil {
			t.Logf("read data response: %v", err)
			return
		}
		frame, _ = DecodeFrame(msg)
		if frame.Type == FrameData {
			decrypted, err := appEnc.Decrypt(frame.Payload)
			if err != nil {
				t.Logf("decrypt response: %v", err)
				return
			}
			var resp ProxyResponse
			_ = json.Unmarshal(decrypted, &resp)
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
		LicenseTokenFunc:   func() string { return "test-jwt" },
		TokenValidator:     func(token string) bool { return token == "valid-token" },
		LocalAddr:          localAddr,
		ServerVersion:      "1.0.0-test",
		IdentityPubKey:     identityPub,
		IdentityPrivateKey: identityPriv,
	}

	client := NewClient(cfg, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.Run(ctx)
	}()

	select {
	case resp := <-dataResponseCh:
		if resp.ID != "req_encrypted" {
			t.Errorf("response ID: got %q, want %q", resp.ID, "req_encrypted")
		}
		if resp.Status != 200 {
			t.Errorf("response status: got %d, want 200", resp.Status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for encrypted DATA response")
	}

	cancel()
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Fatal("client.Run didn't return after cancel")
	}
}

func TestClient_DataWithoutKeyExchange(t *testing.T) {
	// Verifies backward compatibility: unencrypted DATA still works when no KEY_EXCHANGE occurs.
	logger := zerolog.New(zerolog.NewTestWriter(t))

	dataResponseCh := make(chan ProxyResponse, 1)

	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": r.URL.Path})
	}))
	defer mockAPI.Close()
	localAddr := strings.TrimPrefix(mockAPI.URL, "http://")

	identityPriv, identityPub := testIdentityKeyPair(t)

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		// REGISTER
		_, msg, _ := conn.ReadMessage()
		frame, _ := DecodeFrame(msg)
		if frame.Type != FrameRegister {
			return
		}

		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_plain",
			SessionToken: "sess_plain",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		// CHANNEL_OPEN (no KEY_EXCHANGE follows)
		chOpen, _ := NewControlFrame(FrameChannelOpen, 5, ChannelOpenPayload{
			ChannelID: 5,
			AuthToken: "plain-token",
		})
		chOpenBytes, _ := EncodeFrame(chOpen)
		time.Sleep(50 * time.Millisecond)
		_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

		// Read CHANNEL_OPEN ack
		_, msg, _ = conn.ReadMessage()
		frame, _ = DecodeFrame(msg)
		if frame.Type != FrameChannelOpen {
			return
		}

		// Send unencrypted DATA
		proxyReq := ProxyRequest{
			ID:     "req_plain",
			Method: "GET",
			Path:   "/api/health",
		}
		proxyReqBytes, _ := json.Marshal(proxyReq)
		dataFrame := NewFrame(FrameData, 5, proxyReqBytes)
		dataBytes, _ := EncodeFrame(dataFrame)
		_ = conn.WriteMessage(websocket.BinaryMessage, dataBytes)

		// Read unencrypted DATA response
		_, msg, _ = conn.ReadMessage()
		frame, _ = DecodeFrame(msg)
		if frame.Type == FrameData {
			var resp ProxyResponse
			// Should be plain JSON, not encrypted
			if err := json.Unmarshal(frame.Payload, &resp); err != nil {
				t.Logf("unmarshal response: %v", err)
				return
			}
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
		LicenseTokenFunc:   func() string { return "test-jwt" },
		TokenValidator:     func(token string) bool { return token == "plain-token" },
		LocalAddr:          localAddr,
		ServerVersion:      "1.0.0-test",
		IdentityPubKey:     identityPub,
		IdentityPrivateKey: identityPriv,
	}

	client := NewClient(cfg, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.Run(ctx)
	}()

	select {
	case resp := <-dataResponseCh:
		if resp.ID != "req_plain" {
			t.Errorf("response ID: got %q, want %q", resp.ID, "req_plain")
		}
		if resp.Status != 200 {
			t.Errorf("response status: got %d, want 200", resp.Status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for unencrypted DATA response")
	}

	cancel()
	<-errCh
}

func TestClient_KeyExchangeRejectedWithoutIdentityKey(t *testing.T) {
	// Verifies that KEY_EXCHANGE fails closed when IdentityPrivateKey is empty:
	// the instance sends CHANNEL_CLOSE, removes the channel locally, and
	// ignores any subsequent DATA on that channel.
	logger := zerolog.New(zerolog.NewTestWriter(t))

	channelCloseCh := make(chan ChannelClosePayload, 1)
	dataResponseCh := make(chan struct{}, 1)

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		// REGISTER
		_, msg, _ := conn.ReadMessage()
		frame, _ := DecodeFrame(msg)
		if frame.Type != FrameRegister {
			return
		}

		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_nosign",
			SessionToken: "sess_nosign",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		// CHANNEL_OPEN
		chOpen, _ := NewControlFrame(FrameChannelOpen, 20, ChannelOpenPayload{
			ChannelID: 20,
			AuthToken: "token-nosign",
		})
		chOpenBytes, _ := EncodeFrame(chOpen)
		time.Sleep(50 * time.Millisecond)
		_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

		// Read CHANNEL_OPEN ack
		_, msg, _ = conn.ReadMessage()
		frame, _ = DecodeFrame(msg)
		if frame.Type != FrameChannelOpen {
			return
		}

		// Send KEY_EXCHANGE from "app"
		appPriv, _ := GenerateEphemeralKeyPair()
		kexPayload := MarshalKeyExchangePayload(appPriv.PublicKey().Bytes(), nil)
		kexFrame := NewFrame(FrameKeyExchange, 20, kexPayload)
		kexBytes, _ := EncodeFrame(kexFrame)
		_ = conn.WriteMessage(websocket.BinaryMessage, kexBytes)

		// Should receive CHANNEL_CLOSE (not KEY_EXCHANGE response)
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		frame, _ = DecodeFrame(msg)
		if frame.Type == FrameChannelClose {
			var closePayload ChannelClosePayload
			_ = UnmarshalControlPayload(frame.Payload, &closePayload)
			channelCloseCh <- closePayload
		} else {
			t.Logf("expected CHANNEL_CLOSE, got %s", FrameTypeName(frame.Type))
			return
		}

		// Non-cooperative peer: send DATA on the closed channel anyway.
		// The instance must ignore it (channel removed from map).
		time.Sleep(50 * time.Millisecond)
		proxyReq := ProxyRequest{
			ID:     "req_should_be_ignored",
			Method: "GET",
			Path:   "/api/status",
		}
		proxyReqBytes, _ := json.Marshal(proxyReq)
		dataFrame := NewFrame(FrameData, 20, proxyReqBytes)
		dataBytes, _ := EncodeFrame(dataFrame)
		_ = conn.WriteMessage(websocket.BinaryMessage, dataBytes)

		// Wait briefly for any response — there should be none.
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, _, readErr := conn.ReadMessage()
		if readErr == nil {
			// Got a response — the channel wasn't properly removed
			dataResponseCh <- struct{}{}
		}
		_ = conn.SetReadDeadline(time.Time{})

		time.Sleep(100 * time.Millisecond)
	})
	defer relayServer.Close()

	cfg := Config{
		Enabled:   true,
		ServerURL: wsURL(relayServer),
	}

	deps := ClientDeps{
		LicenseTokenFunc:   func() string { return "test-jwt" },
		TokenValidator:     func(token string) bool { return token == "token-nosign" },
		LocalAddr:          "127.0.0.1:9999",
		ServerVersion:      "1.0.0-test",
		IdentityPubKey:     "some-pub-key",
		IdentityPrivateKey: "", // deliberately empty
	}

	client := NewClient(cfg, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	select {
	case closePayload := <-channelCloseCh:
		if closePayload.ChannelID != 20 {
			t.Errorf("channel ID: got %d, want 20", closePayload.ChannelID)
		}
		if closePayload.Reason != "key exchange signing unavailable" {
			t.Errorf("reason: got %q, want %q", closePayload.Reason, "key exchange signing unavailable")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for CHANNEL_CLOSE from failed KEY_EXCHANGE")
	}

	// Verify no DATA response was sent for the post-close frame
	select {
	case <-dataResponseCh:
		t.Fatal("instance processed DATA on a channel that should have been removed after KEY_EXCHANGE rejection")
	case <-time.After(800 * time.Millisecond):
		// Good — no response
	}

	// Verify channel is gone from client state
	status := client.Status()
	if status.ActiveChannels != 0 {
		t.Errorf("active channels: got %d, want 0", status.ActiveChannels)
	}

	cancel()
	<-errCh
}

func TestQueueFrameLogsStructuredContextOnEncodeFailure(t *testing.T) {
	var logOutput bytes.Buffer
	logger := zerolog.New(&logOutput)
	sendCh := make(chan []byte, 1)

	// Oversized payload ensures EncodeFrame fails.
	queueFrame(sendCh, NewFrame(FrameData, 7, make([]byte, MaxPayloadSize+1)), logger)

	got := logOutput.String()
	for _, expected := range []string{
		`"component":"relay_client"`,
		`"action":"encode_frame"`,
		`"frame_type":"DATA"`,
		`"channel":7`,
		`"payload_bytes":65537`,
		`"message":"Failed to encode frame for send"`,
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected log output to include %s, got %q", expected, got)
		}
	}
}

func TestQueueFrameLogsStructuredContextOnDrop(t *testing.T) {
	var logOutput bytes.Buffer
	logger := zerolog.New(&logOutput)
	sendCh := make(chan []byte, 1)
	sendCh <- []byte("full")

	queueFrame(sendCh, NewFrame(FramePing, 11, nil), logger)

	got := logOutput.String()
	for _, expected := range []string{
		`"component":"relay_client"`,
		`"action":"drop_frame"`,
		`"frame_type":"PING"`,
		`"channel":11`,
		`"payload_bytes":0`,
		`"send_queue_depth":1`,
		`"send_queue_capacity":1`,
		`"message":"Send channel full, dropping frame"`,
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected log output to include %s, got %q", expected, got)
		}
	}
}

func TestClient_SendPushNotification(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	pushFrameCh := make(chan Frame, 1)

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		// REGISTER
		_, msg, _ := conn.ReadMessage()
		frame, _ := DecodeFrame(msg)
		if frame.Type != FrameRegister {
			return
		}

		// REGISTER_ACK
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_push",
			SessionToken: "sess_push",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		// Read frames from the instance; expect PUSH_NOTIFICATION
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			frame, err := DecodeFrame(msg)
			if err != nil {
				continue
			}
			if frame.Type == FramePushNotification {
				pushFrameCh <- frame
				return
			}
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	// Wait for connection to be established
	deadline := time.After(3 * time.Second)
	for {
		if client.Status().Connected {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for connection")
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Send push notification
	notification := NewPatrolFindingNotification("finding-test", "critical", "performance", "Test Push")
	if err := client.SendPushNotification(notification); err != nil {
		t.Fatalf("SendPushNotification() error = %v", err)
	}

	// Verify the frame was received by the mock server
	select {
	case frame := <-pushFrameCh:
		if frame.Type != FramePushNotification {
			t.Errorf("frame type: got 0x%02X, want 0x%02X", frame.Type, FramePushNotification)
		}
		if frame.Channel != 0 {
			t.Errorf("channel: got %d, want 0 (control channel)", frame.Channel)
		}
		var payload PushNotificationPayload
		if err := UnmarshalControlPayload(frame.Payload, &payload); err != nil {
			t.Fatalf("unmarshal push payload: %v", err)
		}
		if payload.Type != PushTypePatrolCritical {
			t.Errorf("payload type: got %q, want %q", payload.Type, PushTypePatrolCritical)
		}
		if payload.Title != "Test Push" {
			t.Errorf("payload title: got %q, want %q", payload.Title, "Test Push")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for PUSH_NOTIFICATION frame")
	}

	cancel()
	<-errCh
}

func TestClient_SendPushNotificationDisconnected(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	cfg := Config{
		Enabled:   true,
		ServerURL: "ws://127.0.0.1:1", // unreachable
	}

	deps := ClientDeps{
		LicenseTokenFunc: func() string { return "test-jwt" },
		TokenValidator:   func(token string) bool { return true },
		LocalAddr:        "127.0.0.1:9999",
		ServerVersion:    "1.0.0",
		IdentityPubKey:   "test-pub-key",
	}

	// Create client but don't run it — stays disconnected
	client := NewClient(cfg, deps, logger)

	notification := NewPatrolFindingNotification("finding-test", "warning", "capacity", "Test")
	err := client.SendPushNotification(notification)
	if err == nil {
		t.Fatal("expected error when sending on disconnected client")
	}
	if err != ErrNotConnected {
		t.Errorf("expected ErrNotConnected, got %v", err)
	}
}

func TestClient_RegisterFailsWithoutLicenseTokenProvider(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		time.Sleep(100 * time.Millisecond)
	})
	defer relayServer.Close()

	cfg := Config{
		Enabled:   true,
		ServerURL: wsURL(relayServer),
	}

	deps := ClientDeps{
		LicenseTokenFunc: nil, // explicit hardening guard
		TokenValidator:   func(token string) bool { return true },
		LocalAddr:        "127.0.0.1:9999",
		ServerVersion:    "1.0.0",
		IdentityPubKey:   "test-pub-key",
	}

	client := NewClient(cfg, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := client.connectAndHandle(ctx)
	if err == nil {
		t.Fatal("expected error when LicenseTokenFunc is nil")
	}
	if !strings.Contains(err.Error(), "license token provider not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_RejectChannelWhenTokenValidatorMissing(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	cfg := Config{
		Enabled:   true,
		ServerURL: "wss://relay.example.com",
	}

	deps := ClientDeps{
		LicenseTokenFunc: func() string { return "test-jwt" },
		TokenValidator:   nil, // explicit hardening guard
		LocalAddr:        "127.0.0.1:9999",
		ServerVersion:    "1.0.0",
		IdentityPubKey:   "test-pub-key",
	}

	client := NewClient(cfg, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// With nil TokenValidator, Run() should fail fast at startup validation
	// rather than connecting to the server.
	err := client.Run(ctx)
	if err == nil {
		t.Fatal("expected error when TokenValidator is nil")
	}
	if !strings.Contains(err.Error(), "token validator") {
		t.Fatalf("expected token validator error, got: %v", err)
	}
}

func TestClient_OverloadedDataReturnsBusyResponse(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	origLimit := maxConcurrentDataHandlers
	maxConcurrentDataHandlers = 1
	defer func() { maxConcurrentDataHandlers = origLimit }()

	releaseSlow := make(chan struct{})
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/slow" {
			<-releaseSlow
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": r.URL.Path})
	}))
	defer mockAPI.Close()
	localAddr := strings.TrimPrefix(mockAPI.URL, "http://")

	overloadedRespCh := make(chan ProxyResponse, 1)

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		// REGISTER
		_, msg, _ := conn.ReadMessage()
		frame, _ := DecodeFrame(msg)
		if frame.Type != FrameRegister {
			return
		}
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_overload",
			SessionToken: "sess_overload",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		// Open channel
		time.Sleep(50 * time.Millisecond)
		chOpen, _ := NewControlFrame(FrameChannelOpen, 1, ChannelOpenPayload{
			ChannelID: 1,
			AuthToken: "valid-token",
		})
		chOpenBytes, _ := EncodeFrame(chOpen)
		_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

		// Read CHANNEL_OPEN ack
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		frame, _ = DecodeFrame(msg)
		if frame.Type != FrameChannelOpen {
			return
		}

		// First request occupies the only in-flight slot.
		firstReq := ProxyRequest{
			ID:     "req_slow",
			Method: "GET",
			Path:   "/api/slow",
		}
		firstReqBytes, _ := json.Marshal(firstReq)
		firstFrame := NewFrame(FrameData, 1, firstReqBytes)
		firstData, _ := EncodeFrame(firstFrame)
		_ = conn.WriteMessage(websocket.BinaryMessage, firstData)

		// Second request should get immediate 503 overload response.
		secondReq := ProxyRequest{
			ID:     "req_overload",
			Method: "GET",
			Path:   "/api/fast",
		}
		secondReqBytes, _ := json.Marshal(secondReq)
		secondFrame := NewFrame(FrameData, 1, secondReqBytes)
		secondData, _ := EncodeFrame(secondFrame)
		_ = conn.WriteMessage(websocket.BinaryMessage, secondData)

		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, msg, err = conn.ReadMessage()
		if err != nil {
			return
		}
		frame, _ = DecodeFrame(msg)
		if frame.Type != FrameData {
			return
		}
		var resp ProxyResponse
		if err := json.Unmarshal(frame.Payload, &resp); err != nil {
			return
		}
		overloadedRespCh <- resp

		close(releaseSlow)
		time.Sleep(100 * time.Millisecond)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	select {
	case resp := <-overloadedRespCh:
		if resp.ID != "req_overload" {
			t.Errorf("response ID: got %q, want %q", resp.ID, "req_overload")
		}
		if resp.Status != http.StatusServiceUnavailable {
			t.Errorf("response status: got %d, want %d", resp.Status, http.StatusServiceUnavailable)
		}
	case <-time.After(3 * time.Second):
		close(releaseSlow)
		t.Fatal("timed out waiting for overload response")
	}

	cancel()
	<-errCh
}

func TestNextConsecutiveFailures(t *testing.T) {
	tests := []struct {
		name      string
		current   int
		connected bool
		want      int
	}{
		{
			name:      "increments when connection attempt never established",
			current:   2,
			connected: false,
			want:      3,
		},
		{
			name:      "resets streak after a registered session disconnects",
			current:   5,
			connected: true,
			want:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextConsecutiveFailures(tt.current, tt.connected)
			if got != tt.want {
				t.Fatalf("nextConsecutiveFailures(%d, %v) = %d, want %d", tt.current, tt.connected, got, tt.want)
			}
		})
	}
}
