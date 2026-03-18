package relay

import (
	"context"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

func TestClient_CloseBeforeRunReturnsQuickly(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	client := NewClient(Config{Enabled: true, ServerURL: "ws://127.0.0.1:1"}, ClientDeps{}, logger)

	start := time.Now()
	client.Close()
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("Close() blocked too long before Run: %v", elapsed)
	}
}

func TestClient_CloseCancelsBlockedReadPump(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		frame, err := DecodeFrame(msg)
		if err != nil || frame.Type != FrameRegister {
			return
		}

		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_shutdown",
			SessionToken: "sess_shutdown",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		if err := conn.WriteMessage(websocket.BinaryMessage, ackBytes); err != nil {
			return
		}

		// Block on read until client shutdown closes the connection.
		_, _, _ = conn.ReadMessage()
	})
	defer relayServer.Close()

	cfg := Config{Enabled: true, ServerURL: wsURL(relayServer)}
	deps := ClientDeps{
		LicenseTokenFunc: func() string { return "test-jwt" },
		TokenValidator:   func(string) bool { return true },
		LocalAddr:        "127.0.0.1:9999",
		ServerVersion:    "1.0.0",
		IdentityPubKey:   "test-pub-key",
	}

	client := NewClient(cfg, deps, logger)
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(runCtx) }()

	connectDeadline := time.NewTimer(3 * time.Second)
	defer connectDeadline.Stop()
	for {
		if client.Status().Connected {
			break
		}
		select {
		case <-connectDeadline.C:
			t.Fatal("timed out waiting for relay connection")
		case <-time.After(25 * time.Millisecond):
		}
	}

	client.Close()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected Run() to return cancellation error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return promptly after Close()")
	}
}
