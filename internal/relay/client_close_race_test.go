package relay

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// TestClient_ConcurrentCloseAndRun verifies that calling Close() concurrently
// with Run() does not cause a data race on the lifecycle fields (cancel, done).
// Before the fix, Close() read these fields under mu.RLock while Run() wrote
// them under lifecycleMu — a lock mismatch that the race detector catches.
func TestClient_ConcurrentCloseAndRun(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	// Use a relay server that accepts registration so Run() reaches the
	// lifecycle field assignment before Close() fires.
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
			InstanceID:   "inst_race",
			SessionToken: "sess_race",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		// Block until the client disconnects.
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

	// Run 20 iterations to stress the race window.
	for i := 0; i < 20; i++ {
		client := NewClient(cfg, deps, logger)
		ctx, cancel := context.WithCancel(context.Background())

		var wg sync.WaitGroup

		// Start Run() in a goroutine.
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = client.Run(ctx)
		}()

		// Call Close() concurrently after a brief yield to let Run() start
		// its lifecycle field setup.
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Small sleep to overlap with Run() setting cancel/done under lifecycleMu.
			time.Sleep(time.Duration(i%5) * time.Millisecond)
			client.Close()
		}()

		// Safety cancel in case Close() fires before Run() sets up its cancel.
		cancel()
		wg.Wait()
	}
}

// TestClient_MultipleCloseCallsSafe verifies that calling Close() multiple
// times does not panic or deadlock.
func TestClient_MultipleCloseCallsSafe(t *testing.T) {
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
			InstanceID:   "inst_multi_close",
			SessionToken: "sess_multi_close",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	// Wait for connection.
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for {
		if client.Status().Connected {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("timed out waiting for relay connection")
		case <-time.After(25 * time.Millisecond):
		}
	}

	// Fire Close() from multiple goroutines simultaneously.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.Close()
		}()
	}
	wg.Wait()

	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Fatal("Run() did not return after multiple Close() calls")
	}
}
