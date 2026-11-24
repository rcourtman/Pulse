package relay

import (
	"bytes"
	"context"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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

func wssURL(server *httptest.Server) string {
	return "wss" + strings.TrimPrefix(server.URL, "https")
}

func TestHTTPProxyTrimsRequestBoundaryWhitespace(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	t.Run("handle request trims method and path whitespace", func(t *testing.T) {
		mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("method: got %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/api/resources" {
				t.Fatalf("path: got %q, want %q", r.URL.Path, "/api/resources")
			}
			if token := r.Header.Get("X-API-Token"); token != "test-token" {
				t.Fatalf("token: got %q, want %q", token, "test-token")
			}
			if header := r.Header.Get("Content-Type"); header != "application/json" {
				t.Fatalf("content-type: got %q, want %q", header, "application/json")
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}))
		defer mockAPI.Close()

		proxy := NewHTTPProxy(strings.TrimPrefix(mockAPI.URL, "http://"), logger)
		payload, _ := json.Marshal(ProxyRequest{
			ID:     "req_trim",
			Method: " GET ",
			Path:   " /api/resources ",
			Headers: map[string]string{
				" Content-Type ": "application/json",
			},
		})

		respPayload, err := proxy.HandleRequest(payload, "test-token")
		if err != nil {
			t.Fatalf("HandleRequest() error = %v", err)
		}

		var resp ProxyResponse
		if err := json.Unmarshal(respPayload, &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if resp.Status != http.StatusOK {
			t.Fatalf("status: got %d, want %d", resp.Status, http.StatusOK)
		}
	})

	t.Run("handle stream request trims method and path whitespace", func(t *testing.T) {
		streamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method: got %q, want %q", r.Method, http.MethodPost)
			}
			if r.URL.Path != "/api/ai/chat" {
				t.Fatalf("path: got %q, want %q", r.URL.Path, "/api/ai/chat")
			}
			if token := r.Header.Get("X-API-Token"); token != "test-token" {
				t.Fatalf("token: got %q, want %q", token, "test-token")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "data: done\n\n")
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}))
		defer streamServer.Close()

		proxy := NewHTTPProxy(strings.TrimPrefix(streamServer.URL, "http://"), logger)
		payload, _ := json.Marshal(ProxyRequest{
			ID:     "stream_trim",
			Method: " POST ",
			Path:   " /api/ai/chat ",
		})

		var frames [][]byte
		err := proxy.HandleStreamRequest(context.Background(), payload, "test-token", func(data []byte) {
			frames = append(frames, append([]byte(nil), data...))
		})
		if err != nil {
			t.Fatalf("HandleStreamRequest() error = %v", err)
		}
		if len(frames) == 0 {
			t.Fatal("expected at least one stream frame")
		}
	})
}

func writeServerCertPEM(t *testing.T, server *httptest.Server) string {
	t.Helper()

	if len(server.TLS.Certificates) == 0 || len(server.TLS.Certificates[0].Certificate) == 0 {
		t.Fatal("test server missing TLS certificate")
	}

	block := &pem.Block{Type: "CERTIFICATE", Bytes: server.TLS.Certificates[0].Certificate[0]}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		t.Fatalf("parse test certificate: %v", err)
	}

	path := filepath.Join(t.TempDir(), "relay-ca.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
		t.Fatalf("write relay CA bundle: %v", err)
	}
	return path
}

type blockingTestAEAD struct {
	openStarted chan struct{}
	releaseOpen chan struct{}
}

func (a *blockingTestAEAD) NonceSize() int { return nonceSize }

func (a *blockingTestAEAD) Overhead() int { return 0 }

func (a *blockingTestAEAD) Seal(_ []byte, _ []byte, plaintext []byte, _ []byte) []byte {
	return append([]byte(nil), plaintext...)
}

func (a *blockingTestAEAD) Open(_ []byte, _ []byte, ciphertext []byte, _ []byte) ([]byte, error) {
	select {
	case <-a.openStarted:
	default:
		close(a.openStarted)
	}
	<-a.releaseOpen
	return append([]byte(nil), ciphertext...), nil
}

var _ cipher.AEAD = (*blockingTestAEAD)(nil)

func TestClient_RegisterAndChannelLifecycle(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	identityPriv, identityPub := testIdentityKeyPair(t)

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
			regPayload.IdentityPubKey == identityPub
		mu.Unlock()

		// 2. Send REGISTER_ACK
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_abc",
			SessionToken: "sess_xyz",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		// 3. Send CHANNEL_OPEN (encrypted auth mode, no plaintext token)
		chOpen, _ := NewControlFrame(FrameChannelOpen, 1, ChannelOpenPayload{
			ChannelID: 1,
			AuthMode:  AuthModeEncrypted,
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

		// 5. Complete KEY_EXCHANGE and encrypted channel_auth as the app
		appEnc, err := testAppKeyExchange(conn, 1, identityPub)
		if err != nil {
			t.Logf("key exchange: %v", err)
			return
		}
		if err := testAppAuthenticateChannelDirect(conn, 1, appEnc, "valid-api-token"); err != nil {
			t.Logf("channel auth: %v", err)
			return
		}

		// 6. Send DATA request
		proxyReq := ProxyRequest{
			ID:     "req_test",
			Method: "GET",
			Path:   "/api/status",
		}
		proxyReqBytes, _ := json.Marshal(proxyReq)
		proxyReqBytes, _ = appEnc.Encrypt(proxyReqBytes)
		dataFrame := NewFrame(FrameData, 1, proxyReqBytes)
		dataBytes, _ := EncodeFrame(dataFrame)
		_ = conn.WriteMessage(websocket.BinaryMessage, dataBytes)

		// 7. Read DATA response
		_, msg, err = conn.ReadMessage()
		if err != nil {
			t.Logf("read data response: %v", err)
			return
		}
		frame, _ = DecodeFrame(msg)
		if frame.Type == FrameData {
			var resp ProxyResponse
			frame.Payload, _ = appEnc.Decrypt(frame.Payload)
			_ = json.Unmarshal(frame.Payload, &resp)
			dataResponseCh <- resp
		}

		// 8. Send CHANNEL_CLOSE
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
		LicenseTokenFunc:   func() string { return "test-license-jwt" },
		TokenValidator:     func(token string) bool { return token == "valid-api-token" },
		LocalAddr:          localAddr,
		ServerVersion:      "1.0.0-test",
		IdentityPubKey:     identityPub,
		IdentityPrivateKey: identityPriv,
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
	identityPriv, identityPub := testIdentityKeyPair(t)

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

		// Open encrypted channel
		time.Sleep(50 * time.Millisecond)
		chOpen, _ := NewControlFrame(FrameChannelOpen, 99, ChannelOpenPayload{
			ChannelID: 99,
			AuthMode:  AuthModeEncrypted,
		})
		chOpenBytes, _ := EncodeFrame(chOpen)
		_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

		// Read CHANNEL_OPEN echo
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		frame, _ = DecodeFrame(msg)
		if frame.Type != FrameChannelOpen {
			return
		}

		// Complete KEY_EXCHANGE then present an invalid token inside encrypted DATA
		appEnc, err := testAppKeyExchange(conn, 99, identityPub)
		if err != nil {
			t.Logf("key exchange: %v", err)
			return
		}
		if err := sendEncryptedChannelAuth(conn, 99, appEnc, "INVALID-TOKEN"); err != nil {
			t.Logf("send channel_auth: %v", err)
			return
		}

		// Read CHANNEL_CLOSE (reject)
		_, msg, err = conn.ReadMessage()
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
		LicenseTokenFunc:   func() string { return "test-jwt" },
		TokenValidator:     func(token string) bool { return token == "good-token" },
		LocalAddr:          "127.0.0.1:9999",
		ServerVersion:      "1.0.0",
		IdentityPubKey:     identityPub,
		IdentityPrivateKey: identityPriv,
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
		if closePayload.Reason != "invalid token" {
			t.Errorf("reason: got %q, want %q", closePayload.Reason, "invalid token")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for CHANNEL_CLOSE")
	}

	cancel()
	<-errCh
}

func TestClient_UsesRelayCABundleFromEnvironment(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mockAPI.Close()

	relayServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

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

		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_tls",
			SessionToken: "sess_tls",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		<-time.After(250 * time.Millisecond)
	}))
	defer relayServer.Close()

	t.Setenv("SSL_CERT_FILE", writeServerCertPEM(t, relayServer))

	client := NewClient(
		Config{
			Enabled:   true,
			ServerURL: wssURL(relayServer),
		},
		ClientDeps{
			LicenseTokenFunc: func() string { return "test-license-jwt" },
			TokenValidator:   func(string) bool { return true },
			LocalAddr:        strings.TrimPrefix(mockAPI.URL, "http://"),
			ServerVersion:    "1.0.0-test",
			IdentityPubKey:   "test-identity-pub-key",
		},
		logger,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if status := client.Status(); status.Connected && status.InstanceID == "inst_tls" {
			cancel()
			select {
			case <-errCh:
			case <-time.After(time.Second):
				t.Fatal("client.Run didn't return after cancel")
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}

	status := client.Status()
	cancel()
	select {
	case <-errCh:
	case <-time.After(time.Second):
		t.Fatal("client.Run didn't return after cancel")
	}
	t.Fatalf("expected TLS relay client to connect, got status=%+v", status)
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

	type regCapture struct {
		sessionToken string
		instanceHint string
	}
	captures := make(chan regCapture, 2)

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		_, msg, _ := conn.ReadMessage()
		frame, _ := DecodeFrame(msg)
		if frame.Type != FrameRegister {
			return
		}

		var regPayload RegisterPayload
		_ = UnmarshalControlPayload(frame.Payload, &regPayload)
		captures <- regCapture{
			sessionToken: regPayload.SessionToken,
			instanceHint: regPayload.InstanceHint,
		}

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
		Enabled:        true,
		ServerURL:      wsURL(relayServer),
		InstanceSecret: "my-raw-secret",
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

	// First connection: no session token, InstanceHint is the raw secret
	select {
	case cap := <-captures:
		if cap.sessionToken != "" {
			t.Errorf("first connection session_token: got %q, want empty", cap.sessionToken)
		}
		if cap.instanceHint != "my-raw-secret" {
			t.Errorf("first connection instance_hint: got %q, want %q", cap.instanceHint, "my-raw-secret")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for first REGISTER")
	}

	// Second connection: session token reused, InstanceHint switches to derived instance ID
	select {
	case cap := <-captures:
		if cap.sessionToken != "server-issued-session-token" {
			t.Errorf("second connection session_token: got %q, want %q", cap.sessionToken, "server-issued-session-token")
		}
		// On reconnect, InstanceHint should be the derived instance_id (from
		// REGISTER_ACK), NOT the raw secret. The relay server's session
		// reconnect path looks up by instance_id directly.
		if cap.instanceHint != "inst_abc" {
			t.Errorf("second connection instance_hint: got %q, want %q (derived instance ID)", cap.instanceHint, "inst_abc")
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

// testAppKeyExchange completes KEY_EXCHANGE on a channel as the app behind the
// relay and returns the app-side channel encryption. The instance's
// KEY_EXCHANGE response must be the next frame on conn.
func testAppKeyExchange(conn *websocket.Conn, channelID uint32, identityPub string) (*ChannelEncryption, error) {
	appPriv, err := GenerateEphemeralKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate app keypair: %w", err)
	}
	kexBytes, err := EncodeFrame(NewFrame(FrameKeyExchange, channelID, MarshalKeyExchangePayload(appPriv.PublicKey().Bytes(), nil)))
	if err != nil {
		return nil, fmt.Errorf("encode key exchange: %w", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, kexBytes); err != nil {
		return nil, fmt.Errorf("write key exchange: %w", err)
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("read key exchange: %w", err)
	}
	frame, err := DecodeFrame(msg)
	if err != nil {
		return nil, fmt.Errorf("decode key exchange: %w", err)
	}
	if frame.Type != FrameKeyExchange || frame.Channel != channelID {
		return nil, fmt.Errorf("expected KEY_EXCHANGE on channel %d, got %s on channel %d", channelID, FrameTypeName(frame.Type), frame.Channel)
	}
	instancePub, sig, err := UnmarshalKeyExchangePayload(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("unmarshal key exchange: %w", err)
	}
	if err := VerifyKeyExchangeSignature(instancePub, sig, identityPub); err != nil {
		return nil, fmt.Errorf("verify key exchange signature: %w", err)
	}
	instancePubKey, err := ecdh.X25519().NewPublicKey(instancePub)
	if err != nil {
		return nil, fmt.Errorf("parse instance public key: %w", err)
	}
	return DeriveChannelKeys(appPriv, instancePubKey, false)
}

func sendEncryptedChannelAuth(conn *websocket.Conn, channelID uint32, appEnc *ChannelEncryption, authToken string) error {
	authBytes, err := json.Marshal(channelAuthMessage{
		Type:      channelAuthMessageType,
		AuthToken: authToken,
	})
	if err != nil {
		return fmt.Errorf("marshal channel_auth: %w", err)
	}
	encrypted, err := appEnc.Encrypt(authBytes)
	if err != nil {
		return fmt.Errorf("encrypt channel_auth: %w", err)
	}
	frameBytes, err := EncodeFrame(NewFrame(FrameData, channelID, encrypted))
	if err != nil {
		return fmt.Errorf("encode channel_auth frame: %w", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, frameBytes); err != nil {
		return fmt.Errorf("write channel_auth: %w", err)
	}
	return nil
}

func readEncryptedChannelAuthAck(conn *websocket.Conn, channelID uint32, appEnc *ChannelEncryption) error {
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read channel_auth_ack: %w", err)
	}
	frame, err := DecodeFrame(msg)
	if err != nil {
		return fmt.Errorf("decode channel_auth_ack frame: %w", err)
	}
	if frame.Type != FrameData || frame.Channel != channelID {
		return fmt.Errorf("expected DATA channel_auth_ack on channel %d, got %s on channel %d", channelID, FrameTypeName(frame.Type), frame.Channel)
	}
	plaintext, err := appEnc.Decrypt(frame.Payload)
	if err != nil {
		return fmt.Errorf("decrypt channel_auth_ack: %w", err)
	}
	var ack channelAuthAckMessage
	if err := json.Unmarshal(plaintext, &ack); err != nil {
		return fmt.Errorf("unmarshal channel_auth_ack: %w", err)
	}
	if ack.Type != channelAuthAckMessageType || !ack.OK {
		return fmt.Errorf("unexpected channel_auth_ack payload: %+v", ack)
	}
	return nil
}

// testAppAuthenticateChannelDirect completes encrypted channel_auth directly
// against a mock relay connection, asserting both the encrypted DATA ack and
// the follow-up CHANNEL_OPEN(authenticated=true) frame sent to the relay.
func testAppAuthenticateChannelDirect(conn *websocket.Conn, channelID uint32, appEnc *ChannelEncryption, authToken string) error {
	if err := sendEncryptedChannelAuth(conn, channelID, appEnc, authToken); err != nil {
		return err
	}
	if err := readEncryptedChannelAuthAck(conn, channelID, appEnc); err != nil {
		return err
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read authenticated CHANNEL_OPEN: %w", err)
	}
	frame, err := DecodeFrame(msg)
	if err != nil {
		return fmt.Errorf("decode authenticated CHANNEL_OPEN: %w", err)
	}
	if frame.Type != FrameChannelOpen || frame.Channel != channelID {
		return fmt.Errorf("expected authenticated CHANNEL_OPEN on channel %d, got %s on channel %d", channelID, FrameTypeName(frame.Type), frame.Channel)
	}
	var open ChannelOpenPayload
	if err := UnmarshalControlPayload(frame.Payload, &open); err != nil {
		return fmt.Errorf("unmarshal authenticated CHANNEL_OPEN: %w", err)
	}
	if open.AuthMode != AuthModeEncrypted || !open.Authenticated || open.AuthToken != "" {
		return fmt.Errorf("unexpected authenticated CHANNEL_OPEN payload: %+v", open)
	}
	return nil
}

// testAppAuthenticateChannelViaRelay completes encrypted channel_auth through a
// real relay-server WebSocket, which consumes CHANNEL_OPEN(authenticated=true)
// internally and forwards only the encrypted DATA channel_auth_ack to the app.
func testAppAuthenticateChannelViaRelay(conn *websocket.Conn, channelID uint32, appEnc *ChannelEncryption, authToken string) error {
	if err := sendEncryptedChannelAuth(conn, channelID, appEnc, authToken); err != nil {
		return err
	}
	return readEncryptedChannelAuthAck(conn, channelID, appEnc)
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

		// 3. Send CHANNEL_OPEN (encrypted auth mode, no plaintext token)
		chOpen, _ := NewControlFrame(FrameChannelOpen, 10, ChannelOpenPayload{
			ChannelID: 10,
			AuthMode:  AuthModeEncrypted,
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

		// 6b. Authenticate inside the encrypted channel
		if err := testAppAuthenticateChannelDirect(conn, 10, appEnc, "valid-token"); err != nil {
			t.Logf("channel auth: %v", err)
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

func TestClient_HandleDataDecryptsBeforeReturning(t *testing.T) {
	logger := zerolog.New(io.Discard)

	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mockAPI.Close()

	blockingAEAD := &blockingTestAEAD{
		openStarted: make(chan struct{}),
		releaseOpen: make(chan struct{}),
	}

	client := &Client{
		proxy:  NewHTTPProxy(strings.TrimPrefix(mockAPI.URL, "http://"), logger),
		logger: logger,
		channels: map[uint32]*channelState{
			7: {
				apiToken:      "valid-token",
				authenticated: true,
				encryption: &ChannelEncryption{
					sendCipher: &channelCipher{aead: blockingAEAD},
					recvCipher: &channelCipher{aead: blockingAEAD},
				},
			},
		},
	}
	defer client.proxy.Close()

	// Any payload with a zero nonce works with the blocking AEAD.
	ciphertext := append(make([]byte, nonceSize), []byte(`{"id":"req-1","method":"GET","path":"/api/ai/approvals"}`)...)
	frame := NewFrame(FrameData, 7, ciphertext)
	sendCh := make(chan []byte, 1)
	dataLimiter := make(chan struct{}, 1)

	done := make(chan struct{})
	go func() {
		client.handleData(context.Background(), frame, sendCh, dataLimiter, nil)
		close(done)
	}()

	select {
	case <-blockingAEAD.openStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for DATA decrypt to start")
	}

	select {
	case <-done:
		t.Fatal("handleData returned before encrypted payload decryption completed")
	case <-time.After(100 * time.Millisecond):
	}

	close(blockingAEAD.releaseOpen)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handleData did not return after decrypt completed")
	}
}

func TestClient_DataWithoutKeyExchange(t *testing.T) {
	// Verifies that DATA fails closed when no KEY_EXCHANGE occurred: the
	// instance sends CHANNEL_CLOSE, removes the channel locally, and never
	// proxies the plaintext request or answers it in plaintext.
	logger := zerolog.New(zerolog.NewTestWriter(t))

	channelCloseCh := make(chan ChannelClosePayload, 1)
	dataResponseCh := make(chan struct{}, 1)
	var apiRequests atomic.Int32

	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiRequests.Add(1)
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
			AuthMode:  AuthModeEncrypted,
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

		// Should receive CHANNEL_CLOSE (not a DATA response)
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

		// Non-cooperative peer: retry the plaintext DATA on the closed channel.
		// The instance must ignore it (channel removed from map).
		time.Sleep(50 * time.Millisecond)
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
	case closePayload := <-channelCloseCh:
		if closePayload.ChannelID != 5 {
			t.Errorf("channel ID: got %d, want 5", closePayload.ChannelID)
		}
		if closePayload.Reason != "key exchange required" {
			t.Errorf("reason: got %q, want %q", closePayload.Reason, "key exchange required")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for CHANNEL_CLOSE from unencrypted DATA")
	}

	// Verify no DATA response was sent for either plaintext frame
	select {
	case <-dataResponseCh:
		t.Fatal("instance answered DATA on a channel that never completed KEY_EXCHANGE")
	case <-time.After(800 * time.Millisecond):
		// Good — no response
	}

	// Verify the plaintext request never reached the local API
	if got := apiRequests.Load(); got != 0 {
		t.Errorf("local API requests: got %d, want 0", got)
	}

	// Verify channel is gone from client state
	status := client.Status()
	if status.ActiveChannels != 0 {
		t.Errorf("active channels: got %d, want 0", status.ActiveChannels)
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
			AuthMode:  AuthModeEncrypted,
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

func TestStatusReportsReconnectDelayDuringLicensePause(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	attempted := make(chan struct{}, 1)
	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		select {
		case attempted <- struct{}{}:
		default:
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		frame, err := DecodeFrame(msg)
		if err != nil || frame.Type != FrameRegister {
			return
		}

		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_status_pause",
			SessionToken: "sess_status_pause",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		errFrame, _ := NewControlFrame(FrameError, 0, ErrorPayload{
			Code:    ErrCodeLicenseExpired,
			Message: "license expired",
		})
		errBytes, _ := EncodeFrame(errFrame)
		_ = conn.WriteMessage(websocket.BinaryMessage, errBytes)

		time.Sleep(100 * time.Millisecond)
	})
	defer relayServer.Close()

	deps := ClientDeps{
		LicenseTokenFunc: func() string { return "expired-jwt" },
		TokenValidator:   func(token string) bool { return true },
		LocalAddr:        "127.0.0.1:9999",
		ServerVersion:    "test",
		IdentityPubKey:   "test-pub-key",
	}

	client := NewClient(Config{Enabled: true, ServerURL: wsURL(relayServer)}, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	select {
	case <-attempted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for relay registration attempt")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status := client.Status()
		if status.LastError != "" && status.ReconnectIn != "" {
			if status.Connected {
				t.Fatalf("expected disconnected status during license pause, got %+v", status)
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for reconnect_in status; final status=%+v", client.Status())
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
		if payload.InstanceID != "inst_push" {
			t.Errorf("payload instance_id: got %q, want %q", payload.InstanceID, "inst_push")
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

	identityPriv, identityPub := testIdentityKeyPair(t)

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
			AuthMode:  AuthModeEncrypted,
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

		// Complete KEY_EXCHANGE and encrypted channel_auth as the app
		appEnc, err := testAppKeyExchange(conn, 1, identityPub)
		if err != nil {
			t.Logf("key exchange: %v", err)
			return
		}
		if err := testAppAuthenticateChannelDirect(conn, 1, appEnc, "valid-token"); err != nil {
			t.Logf("channel auth: %v", err)
			return
		}

		// First request occupies the only in-flight slot.
		firstReq := ProxyRequest{
			ID:     "req_slow",
			Method: "GET",
			Path:   "/api/slow",
		}
		firstReqBytes, _ := json.Marshal(firstReq)
		firstReqBytes, _ = appEnc.Encrypt(firstReqBytes)
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
		secondReqBytes, _ = appEnc.Encrypt(secondReqBytes)
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
		frame.Payload, _ = appEnc.Decrypt(frame.Payload)
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
		LicenseTokenFunc:   func() string { return "test-jwt" },
		TokenValidator:     func(token string) bool { return token == "valid-token" },
		LocalAddr:          localAddr,
		ServerVersion:      "1.0.0",
		IdentityPubKey:     identityPub,
		IdentityPrivateKey: identityPriv,
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

func TestClient_ChannelRateLimitReturnsTooManyRequests(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	origLimit := relayMaxRequestsPerChannelWindow
	relayMaxRequestsPerChannelWindow = 1
	defer func() { relayMaxRequestsPerChannelWindow = origLimit }()

	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": r.URL.Path})
	}))
	defer mockAPI.Close()
	localAddr := strings.TrimPrefix(mockAPI.URL, "http://")

	rateLimitedRespCh := make(chan ProxyResponse, 1)

	identityPriv, identityPub := testIdentityKeyPair(t)

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		_, msg, _ := conn.ReadMessage()
		frame, _ := DecodeFrame(msg)
		if frame.Type != FrameRegister {
			return
		}
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_rate_limit",
			SessionToken: "sess_rate_limit",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		time.Sleep(50 * time.Millisecond)
		chOpen, _ := NewControlFrame(FrameChannelOpen, 1, ChannelOpenPayload{
			ChannelID: 1,
			AuthMode:  AuthModeEncrypted,
		})
		chOpenBytes, _ := EncodeFrame(chOpen)
		_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		frame, _ = DecodeFrame(msg)
		if frame.Type != FrameChannelOpen {
			return
		}

		// Complete KEY_EXCHANGE and encrypted channel_auth as the app
		appEnc, err := testAppKeyExchange(conn, 1, identityPub)
		if err != nil {
			t.Logf("key exchange: %v", err)
			return
		}
		if err := testAppAuthenticateChannelDirect(conn, 1, appEnc, "valid-token"); err != nil {
			t.Logf("channel auth: %v", err)
			return
		}

		firstReqBytes, _ := json.Marshal(ProxyRequest{
			ID:     "req_allowed",
			Method: "GET",
			Path:   "/api/allowed",
		})
		firstReqBytes, _ = appEnc.Encrypt(firstReqBytes)
		firstFrame := NewFrame(FrameData, 1, firstReqBytes)
		firstData, _ := EncodeFrame(firstFrame)
		_ = conn.WriteMessage(websocket.BinaryMessage, firstData)

		_, msg, err = conn.ReadMessage()
		if err != nil {
			return
		}
		frame, _ = DecodeFrame(msg)
		if frame.Type != FrameData {
			return
		}
		var firstResp ProxyResponse
		frame.Payload, _ = appEnc.Decrypt(frame.Payload)
		if err := json.Unmarshal(frame.Payload, &firstResp); err != nil {
			return
		}
		if firstResp.ID != "req_allowed" || firstResp.Status != http.StatusOK {
			return
		}

		secondReqBytes, _ := json.Marshal(ProxyRequest{
			ID:     "req_limited",
			Method: "GET",
			Path:   "/api/limited",
		})
		secondReqBytes, _ = appEnc.Encrypt(secondReqBytes)
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
		frame.Payload, _ = appEnc.Decrypt(frame.Payload)
		if err := json.Unmarshal(frame.Payload, &resp); err != nil {
			return
		}
		rateLimitedRespCh <- resp
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
		ServerVersion:      "1.0.0",
		IdentityPubKey:     identityPub,
		IdentityPrivateKey: identityPriv,
	}

	client := NewClient(cfg, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	select {
	case resp := <-rateLimitedRespCh:
		if resp.ID != "req_limited" {
			t.Errorf("response ID: got %q, want %q", resp.ID, "req_limited")
		}
		if resp.Status != http.StatusTooManyRequests {
			t.Errorf("response status: got %d, want %d", resp.Status, http.StatusTooManyRequests)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for rate-limited response")
	}

	cancel()
	<-errCh
}

func TestRelayChannelRequestLimiterPrunesExpiredAttempts(t *testing.T) {
	limiter := newRelayChannelRequestLimiter(2, time.Minute)
	now := time.Now().UTC()

	if !limiter.Allow(now.Add(-2 * time.Minute)) {
		t.Fatal("expected first request to be allowed")
	}
	if !limiter.Allow(now) {
		t.Fatal("expected second request to be allowed")
	}
	if !limiter.Allow(now.Add(30 * time.Second)) {
		t.Fatal("expected expired attempt to be pruned before third request")
	}
	if limiter.Allow(now.Add(31 * time.Second)) {
		t.Fatal("expected limiter to reject once the current window is full")
	}
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

func TestClient_ChannelOpenRejectsPlaintextAuthToken(t *testing.T) {
	tests := []struct {
		name    string
		payload ChannelOpenPayload
	}{
		{
			name: "legacy plaintext auth_token without auth_mode",
			payload: ChannelOpenPayload{
				ChannelID: 41,
				AuthToken: "valid-token",
			},
		},
		{
			name: "missing auth_mode and auth_token",
			payload: ChannelOpenPayload{
				ChannelID: 42,
			},
		},
		{
			name: "encrypted auth_mode with plaintext auth_token",
			payload: ChannelOpenPayload{
				ChannelID: 43,
				AuthMode:  AuthModeEncrypted,
				AuthToken: "valid-token",
			},
		},
		{
			name: "encrypted auth_mode with spoofed authenticated true",
			payload: ChannelOpenPayload{
				ChannelID:     44,
				AuthMode:      AuthModeEncrypted,
				Authenticated: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient(Config{}, ClientDeps{
				TokenValidator: func(token string) bool { return token == "valid-token" },
			}, zerolog.Nop())

			openFrame, err := NewControlFrame(FrameChannelOpen, tc.payload.ChannelID, tc.payload)
			if err != nil {
				t.Fatalf("NewControlFrame: %v", err)
			}
			sendCh := make(chan []byte, 2)
			client.handleChannelOpen(openFrame, sendCh)

			if got := client.Status().ActiveChannels; got != 0 {
				t.Fatalf("active channels = %d, want 0", got)
			}

			select {
			case raw := <-sendCh:
				frame, err := DecodeFrame(raw)
				if err != nil {
					t.Fatalf("DecodeFrame: %v", err)
				}
				if frame.Type != FrameChannelClose {
					t.Fatalf("frame type = %s, want CHANNEL_CLOSE", FrameTypeName(frame.Type))
				}
				var closePayload ChannelClosePayload
				if err := UnmarshalControlPayload(frame.Payload, &closePayload); err != nil {
					t.Fatalf("UnmarshalControlPayload: %v", err)
				}
				if closePayload.Reason != "encrypted channel authentication required" {
					t.Fatalf("reason = %q, want %q", closePayload.Reason, "encrypted channel authentication required")
				}
			default:
				t.Fatal("expected CHANNEL_CLOSE frame")
			}
		})
	}
}

func TestClient_DataBeforeChannelAuthRefused(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	identityPriv, identityPub := testIdentityKeyPair(t)

	var apiHits atomic.Int32
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockAPI.Close()

	closeCh := make(chan ChannelClosePayload, 1)
	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if frame, _ := DecodeFrame(msg); frame.Type != FrameRegister {
			return
		}
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_preauth",
			SessionToken: "sess_preauth",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		time.Sleep(50 * time.Millisecond)
		chOpen, _ := NewControlFrame(FrameChannelOpen, 51, ChannelOpenPayload{
			ChannelID: 51,
			AuthMode:  AuthModeEncrypted,
		})
		chOpenBytes, _ := EncodeFrame(chOpen)
		_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

		_, msg, err = conn.ReadMessage()
		if err != nil {
			return
		}
		if frame, _ := DecodeFrame(msg); frame.Type != FrameChannelOpen {
			return
		}

		appEnc, err := testAppKeyExchange(conn, 51, identityPub)
		if err != nil {
			t.Logf("key exchange: %v", err)
			return
		}

		// Send an encrypted ProxyRequest without first authenticating the channel.
		reqBytes, _ := json.Marshal(ProxyRequest{
			ID:     "req_unauthenticated",
			Method: "GET",
			Path:   "/api/status",
		})
		encrypted, _ := appEnc.Encrypt(reqBytes)
		frameBytes, _ := EncodeFrame(NewFrame(FrameData, 51, encrypted))
		_ = conn.WriteMessage(websocket.BinaryMessage, frameBytes)

		_, msg, err = conn.ReadMessage()
		if err != nil {
			return
		}
		frame, _ := DecodeFrame(msg)
		if frame.Type == FrameChannelClose {
			var closePayload ChannelClosePayload
			_ = UnmarshalControlPayload(frame.Payload, &closePayload)
			closeCh <- closePayload
		}
	})
	defer relayServer.Close()

	client := NewClient(
		Config{Enabled: true, ServerURL: wsURL(relayServer)},
		ClientDeps{
			LicenseTokenFunc:   func() string { return "test-jwt" },
			TokenValidator:     func(token string) bool { return token == "valid-token" },
			LocalAddr:          strings.TrimPrefix(mockAPI.URL, "http://"),
			ServerVersion:      "1.0.0",
			IdentityPubKey:     identityPub,
			IdentityPrivateKey: identityPriv,
		},
		logger,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	select {
	case closePayload := <-closeCh:
		if closePayload.ChannelID != 51 {
			t.Errorf("channel ID = %d, want 51", closePayload.ChannelID)
		}
		if closePayload.Reason != "channel authentication required" {
			t.Errorf("reason = %q, want %q", closePayload.Reason, "channel authentication required")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for CHANNEL_CLOSE")
	}

	if got := apiHits.Load(); got != 0 {
		t.Fatalf("unauthenticated encrypted DATA reached local API %d times, want 0", got)
	}

	cancel()
	<-errCh
}

func TestClient_MaliciousRelayCannotImpersonateWithoutToken(t *testing.T) {
	// Simulates a malicious relay server that opens its own channel, completes
	// X25519 KEY_EXCHANGE with its own ephemeral keypair, and attempts to call
	// sensitive action routes without possessing the phone's API token.
	logger := zerolog.New(zerolog.NewTestWriter(t))
	identityPriv, identityPub := testIdentityKeyPair(t)

	var apiHits atomic.Int32
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockAPI.Close()

	verdictCh := make(chan error, 1)
	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			verdictCh <- err
			return
		}
		if frame, _ := DecodeFrame(msg); frame.Type != FrameRegister {
			verdictCh <- fmt.Errorf("expected REGISTER")
			return
		}
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_malicious_relay",
			SessionToken: "sess_malicious_relay",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		time.Sleep(50 * time.Millisecond)

		// Attempt 1: Relay opens channel 4242, completes KEY_EXCHANGE with its own
		// X25519 keypair, and tries to call POST /api/actions/act_1/decision directly.
		chOpen1, _ := NewControlFrame(FrameChannelOpen, 4242, ChannelOpenPayload{
			ChannelID: 4242,
			AuthMode:  AuthModeEncrypted,
		})
		chOpenBytes1, _ := EncodeFrame(chOpen1)
		_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes1)
		_, msg, err = conn.ReadMessage()
		if err != nil {
			verdictCh <- err
			return
		}
		if frame, _ := DecodeFrame(msg); frame.Type != FrameChannelOpen {
			verdictCh <- fmt.Errorf("attempt 1: expected CHANNEL_OPEN echo, got %s", FrameTypeName(frame.Type))
			return
		}
		relayEnc1, err := testAppKeyExchange(conn, 4242, identityPub)
		if err != nil {
			verdictCh <- fmt.Errorf("attempt 1 key exchange: %w", err)
			return
		}
		approveReq, _ := json.Marshal(ProxyRequest{
			ID:     "relay-attack-1",
			Method: "POST",
			Path:   "/api/actions/act_1/decision",
			Body:   "eyJvdXRjb21lIjoiYXBwcm92ZWQifQ==",
		})
		encAttack1, _ := relayEnc1.Encrypt(approveReq)
		attackFrame1, _ := EncodeFrame(NewFrame(FrameData, 4242, encAttack1))
		_ = conn.WriteMessage(websocket.BinaryMessage, attackFrame1)

		_, msg, err = conn.ReadMessage()
		if err != nil {
			verdictCh <- err
			return
		}
		frame1, _ := DecodeFrame(msg)
		if frame1.Type != FrameChannelClose {
			verdictCh <- fmt.Errorf("attempt 1: expected CHANNEL_CLOSE, got %s", FrameTypeName(frame1.Type))
			return
		}

		// Attempt 2: Relay opens channel 4243, completes KEY_EXCHANGE, and tries a
		// guessed/empty token in channel_auth before calling the proxy route.
		chOpen2, _ := NewControlFrame(FrameChannelOpen, 4243, ChannelOpenPayload{
			ChannelID: 4243,
			AuthMode:  AuthModeEncrypted,
		})
		chOpenBytes2, _ := EncodeFrame(chOpen2)
		_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes2)
		_, msg, err = conn.ReadMessage()
		if err != nil {
			verdictCh <- err
			return
		}
		if frame, _ := DecodeFrame(msg); frame.Type != FrameChannelOpen {
			verdictCh <- fmt.Errorf("attempt 2: expected CHANNEL_OPEN echo, got %s", FrameTypeName(frame.Type))
			return
		}
		relayEnc2, err := testAppKeyExchange(conn, 4243, identityPub)
		if err != nil {
			verdictCh <- fmt.Errorf("attempt 2 key exchange: %w", err)
			return
		}
		if err := sendEncryptedChannelAuth(conn, 4243, relayEnc2, "guessed-token"); err != nil {
			verdictCh <- err
			return
		}
		_, msg, err = conn.ReadMessage()
		if err != nil {
			verdictCh <- err
			return
		}
		frame2, _ := DecodeFrame(msg)
		if frame2.Type != FrameChannelClose {
			verdictCh <- fmt.Errorf("attempt 2: expected CHANNEL_CLOSE, got %s", FrameTypeName(frame2.Type))
			return
		}
		var close2 ChannelClosePayload
		_ = UnmarshalControlPayload(frame2.Payload, &close2)
		if close2.Reason != "invalid token" {
			verdictCh <- fmt.Errorf("attempt 2: close reason = %q, want %q", close2.Reason, "invalid token")
			return
		}

		verdictCh <- nil
	})
	defer relayServer.Close()

	client := NewClient(
		Config{Enabled: true, ServerURL: wsURL(relayServer)},
		ClientDeps{
			LicenseTokenFunc:   func() string { return "test-jwt" },
			TokenValidator:     func(token string) bool { return token == "phones-secret-token" },
			LocalAddr:          strings.TrimPrefix(mockAPI.URL, "http://"),
			ServerVersion:      "1.0.0",
			IdentityPubKey:     identityPub,
			IdentityPrivateKey: identityPriv,
		},
		logger,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	select {
	case err := <-verdictCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for malicious relay scenario")
	}

	if got := apiHits.Load(); got != 0 {
		t.Fatalf("malicious relay reached local API %d times, want 0", got)
	}

	cancel()
	<-errCh
}

func TestClient_DuplicateChannelAuthRefused(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	identityPriv, identityPub := testIdentityKeyPair(t)

	closeCh := make(chan ChannelClosePayload, 1)
	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if frame, _ := DecodeFrame(msg); frame.Type != FrameRegister {
			return
		}
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_dup_auth",
			SessionToken: "sess_dup_auth",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		time.Sleep(50 * time.Millisecond)
		chOpen, _ := NewControlFrame(FrameChannelOpen, 61, ChannelOpenPayload{
			ChannelID: 61,
			AuthMode:  AuthModeEncrypted,
		})
		chOpenBytes, _ := EncodeFrame(chOpen)
		_ = conn.WriteMessage(websocket.BinaryMessage, chOpenBytes)

		_, msg, err = conn.ReadMessage()
		if err != nil {
			return
		}
		if frame, _ := DecodeFrame(msg); frame.Type != FrameChannelOpen {
			return
		}

		appEnc, err := testAppKeyExchange(conn, 61, identityPub)
		if err != nil {
			t.Logf("key exchange: %v", err)
			return
		}
		if err := testAppAuthenticateChannelDirect(conn, 61, appEnc, "valid-token"); err != nil {
			t.Logf("first channel auth: %v", err)
			return
		}

		// Send duplicate channel_auth on already authenticated channel
		if err := sendEncryptedChannelAuth(conn, 61, appEnc, "valid-token"); err != nil {
			t.Logf("second channel auth: %v", err)
			return
		}

		_, msg, err = conn.ReadMessage()
		if err != nil {
			return
		}
		frame, _ := DecodeFrame(msg)
		if frame.Type == FrameChannelClose {
			var closePayload ChannelClosePayload
			_ = UnmarshalControlPayload(frame.Payload, &closePayload)
			closeCh <- closePayload
		}
	})
	defer relayServer.Close()

	client := NewClient(
		Config{Enabled: true, ServerURL: wsURL(relayServer)},
		ClientDeps{
			LicenseTokenFunc:   func() string { return "test-jwt" },
			TokenValidator:     func(token string) bool { return token == "valid-token" },
			LocalAddr:          "127.0.0.1:9999",
			ServerVersion:      "1.0.0",
			IdentityPubKey:     identityPub,
			IdentityPrivateKey: identityPriv,
		},
		logger,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	select {
	case closePayload := <-closeCh:
		if closePayload.ChannelID != 61 {
			t.Errorf("channel ID = %d, want 61", closePayload.ChannelID)
		}
		if closePayload.Reason != "duplicate channel authentication" {
			t.Errorf("reason = %q, want %q", closePayload.Reason, "duplicate channel authentication")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for CHANNEL_CLOSE on duplicate channel_auth")
	}

	cancel()
	<-errCh
}

func TestClient_ChannelAuthTimeoutClosesUnauthenticatedChannel(t *testing.T) {
	origTimeout := relayChannelAuthTimeout
	relayChannelAuthTimeout = 100 * time.Millisecond
	defer func() { relayChannelAuthTimeout = origTimeout }()

	client := NewClient(Config{}, ClientDeps{
		TokenValidator: func(token string) bool { return token == "valid-token" },
	}, zerolog.Nop())

	openFrame, err := NewControlFrame(FrameChannelOpen, 77, ChannelOpenPayload{
		ChannelID: 77,
		AuthMode:  AuthModeEncrypted,
	})
	if err != nil {
		t.Fatalf("NewControlFrame: %v", err)
	}
	sendCh := make(chan []byte, 4)
	client.handleChannelOpen(openFrame, sendCh)

	// Drain initial CHANNEL_OPEN echo
	echoRaw := <-sendCh
	echoFrame, err := DecodeFrame(echoRaw)
	if err != nil || echoFrame.Type != FrameChannelOpen {
		t.Fatalf("expected CHANNEL_OPEN echo, got frame=%v err=%v", echoFrame, err)
	}

	select {
	case closeRaw := <-sendCh:
		closeFrame, err := DecodeFrame(closeRaw)
		if err != nil {
			t.Fatalf("DecodeFrame: %v", err)
		}
		if closeFrame.Type != FrameChannelClose {
			t.Fatalf("frame type = %s, want CHANNEL_CLOSE", FrameTypeName(closeFrame.Type))
		}
		var closePayload ChannelClosePayload
		if err := UnmarshalControlPayload(closeFrame.Payload, &closePayload); err != nil {
			t.Fatalf("UnmarshalControlPayload: %v", err)
		}
		if closePayload.Reason != "channel authentication timeout" {
			t.Fatalf("reason = %q, want %q", closePayload.Reason, "channel authentication timeout")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for auth timeout CHANNEL_CLOSE")
	}

	if got := client.Status().ActiveChannels; got != 0 {
		t.Fatalf("active channels = %d, want 0 after auth timeout", got)
	}
}
