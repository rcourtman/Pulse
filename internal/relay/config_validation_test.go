package relay

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

func TestNormalizeClientInputs_DefaultsAndTrims(t *testing.T) {
	cfg := Config{
		ServerURL:           "   ",
		InstanceSecret:      "  secret-123  ",
		IdentityPrivateKey:  "  private  ",
		IdentityPublicKey:   "  public  ",
		IdentityFingerprint: "  fingerprint  ",
	}
	deps := ClientDeps{
		LicenseTokenFunc:   func() string { return "jwt" },
		TokenValidator:     func(string) bool { return true },
		LocalAddr:          " 127.0.0.1:7655 ",
		ServerVersion:      " 1.2.3 ",
		IdentityPubKey:     " pub ",
		IdentityPrivateKey: " priv ",
	}

	normalizedCfg, normalizedDeps, warnings, err := normalizeClientInputs(cfg, deps)
	if err != nil {
		t.Fatalf("normalizeClientInputs() error = %v", err)
	}

	if normalizedCfg.ServerURL != DefaultServerURL {
		t.Fatalf("normalized server URL = %q, want %q", normalizedCfg.ServerURL, DefaultServerURL)
	}
	if normalizedCfg.InstanceSecret != "secret-123" {
		t.Fatalf("normalized instance secret = %q, want %q", normalizedCfg.InstanceSecret, "secret-123")
	}
	if normalizedCfg.IdentityPrivateKey != "private" {
		t.Fatalf("normalized private key = %q, want %q", normalizedCfg.IdentityPrivateKey, "private")
	}
	if normalizedCfg.IdentityPublicKey != "public" {
		t.Fatalf("normalized public key = %q, want %q", normalizedCfg.IdentityPublicKey, "public")
	}
	if normalizedCfg.IdentityFingerprint != "fingerprint" {
		t.Fatalf("normalized fingerprint = %q, want %q", normalizedCfg.IdentityFingerprint, "fingerprint")
	}
	if normalizedDeps.LocalAddr != "127.0.0.1:7655" {
		t.Fatalf("normalized local addr = %q, want %q", normalizedDeps.LocalAddr, "127.0.0.1:7655")
	}
	if normalizedDeps.ServerVersion != "1.2.3" {
		t.Fatalf("normalized server version = %q, want %q", normalizedDeps.ServerVersion, "1.2.3")
	}
	if normalizedDeps.IdentityPubKey != "pub" {
		t.Fatalf("normalized identity pub key = %q, want %q", normalizedDeps.IdentityPubKey, "pub")
	}
	if normalizedDeps.IdentityPrivateKey != "priv" {
		t.Fatalf("normalized identity private key = %q, want %q", normalizedDeps.IdentityPrivateKey, "priv")
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings count = %d, want 1", len(warnings))
	}
	if !strings.Contains(warnings[0], "defaulting") {
		t.Fatalf("warning = %q, want defaulting message", warnings[0])
	}
}

func TestNormalizeClientInputs_RejectsInvalidValues(t *testing.T) {
	cfg := Config{
		ServerURL: "http://relay.example.com",
	}
	deps := ClientDeps{
		LocalAddr: "127.0.0.1:0",
	}

	_, _, _, err := normalizeClientInputs(cfg, deps)
	if err == nil {
		t.Fatal("normalizeClientInputs() error = nil, want invalid configuration error")
	}

	errMsg := err.Error()
	for _, want := range []string{
		"invalid relay client configuration",
		"scheme must be ws or wss",
		"license token function is required",
		"token validator function is required",
		"port must be in range 1-65535",
	} {
		if !strings.Contains(errMsg, want) {
			t.Fatalf("error %q does not contain %q", errMsg, want)
		}
	}
}

func TestClientRun_FailsFastOnInvalidStartupConfig(t *testing.T) {
	logger := zerolog.New(io.Discard)
	client := NewClient(
		Config{ServerURL: "http://not-websocket.example.com"},
		ClientDeps{
			LicenseTokenFunc: func() string { return "jwt" },
			TokenValidator:   func(string) bool { return true },
			LocalAddr:        "127.0.0.1:7655",
		},
		logger,
	)

	err := client.Run(context.Background())
	if err == nil {
		t.Fatal("Run() error = nil, want startup validation error")
	}
	if !strings.Contains(err.Error(), "invalid relay client configuration") {
		t.Fatalf("Run() error = %q, want invalid configuration message", err.Error())
	}

	status := client.Status()
	if status.Connected {
		t.Fatal("status connected = true, want false")
	}
	if status.LastError == "" {
		t.Fatal("status last_error = empty, want startup validation error")
	}
}

func TestClientRegister_TrimsLicenseToken(t *testing.T) {
	logger := zerolog.New(io.Discard)
	registerTokenCh := make(chan string, 1)

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}

		frame, err := DecodeFrame(msg)
		if err != nil || frame.Type != FrameRegister {
			return
		}

		var reg RegisterPayload
		if err := UnmarshalControlPayload(frame.Payload, &reg); err != nil {
			return
		}
		registerTokenCh <- reg.LicenseToken

		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_test",
			SessionToken: "sess_test",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		time.Sleep(75 * time.Millisecond)
	})
	defer relayServer.Close()

	client := NewClient(
		Config{Enabled: true, ServerURL: wsURL(relayServer)},
		ClientDeps{
			LicenseTokenFunc: func() string { return "  test-jwt  " },
			TokenValidator:   func(string) bool { return true },
			LocalAddr:        "127.0.0.1:7655",
		},
		logger,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_ = client.connectAndHandle(ctx)

	select {
	case got := <-registerTokenCh:
		if got != "test-jwt" {
			t.Fatalf("register license token = %q, want %q", got, "test-jwt")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for register token")
	}
}
