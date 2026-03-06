package relay

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

func TestClientRegister_SessionResumeRejectionClearsCachedSession(t *testing.T) {
	registerPayloads := make(chan RegisterPayload, 1)

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
		if err := UnmarshalControlPayload(frame.Payload, &regPayload); err != nil {
			t.Fatalf("UnmarshalControlPayload() error = %v", err)
		}
		registerPayloads <- regPayload

		rejectFrame, err := NewControlFrame(FrameError, 0, ErrorPayload{
			Code:    ErrCodeAuthFailed,
			Message: "stale session",
		})
		if err != nil {
			t.Fatalf("NewControlFrame() error = %v", err)
		}
		rejectBytes, err := EncodeFrame(rejectFrame)
		if err != nil {
			t.Fatalf("EncodeFrame() error = %v", err)
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, rejectBytes); err != nil {
			t.Fatalf("WriteMessage() error = %v", err)
		}
	})
	defer relayServer.Close()

	client := NewClient(Config{
		ServerURL:      wsURL(relayServer),
		InstanceSecret: "raw-instance-secret",
	}, ClientDeps{
		LicenseTokenFunc: func() string { return "test-license-jwt" },
		ServerVersion:    "test-version",
	}, zerolog.Nop())

	client.mu.Lock()
	client.instanceID = "inst_stale"
	client.sessionToken = "sess_stale"
	client.mu.Unlock()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(relayServer), nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	err = client.register(conn)
	if err == nil {
		t.Fatal("expected register() to fail for rejected session resume")
	}

	var resumeErr *sessionResumeRejectedError
	if !errors.As(err, &resumeErr) {
		t.Fatalf("register() error = %T, want sessionResumeRejectedError", err)
	}
	if resumeErr.code != ErrCodeAuthFailed {
		t.Fatalf("resume rejection code = %q, want %q", resumeErr.code, ErrCodeAuthFailed)
	}

	select {
	case payload := <-registerPayloads:
		if payload.SessionToken != "sess_stale" {
			t.Fatalf("session_token = %q, want %q", payload.SessionToken, "sess_stale")
		}
		if payload.InstanceHint != "inst_stale" {
			t.Fatalf("instance_hint = %q, want %q", payload.InstanceHint, "inst_stale")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for REGISTER payload")
	}

	client.mu.RLock()
	defer client.mu.RUnlock()
	if client.sessionToken != "" {
		t.Fatalf("session token not cleared after resume rejection: %q", client.sessionToken)
	}
	if client.instanceID != "" {
		t.Fatalf("instance ID not cleared after resume rejection: %q", client.instanceID)
	}
}

func TestRunLoop_SessionResumeRejectionFallsBackToFreshRegister(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	type registerAttempt struct {
		sessionToken string
		instanceHint string
		receivedAt   time.Time
	}

	registers := make(chan registerAttempt, 3)
	secondRejectAt := make(chan time.Time, 1)
	thirdAcked := make(chan struct{}, 1)
	var attempts atomic.Int32

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
		if err := UnmarshalControlPayload(frame.Payload, &regPayload); err != nil {
			t.Fatalf("UnmarshalControlPayload() error = %v", err)
		}
		registers <- registerAttempt{
			sessionToken: regPayload.SessionToken,
			instanceHint: regPayload.InstanceHint,
			receivedAt:   time.Now(),
		}

		switch attempt := attempts.Add(1); attempt {
		case 1:
			ack, err := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
				InstanceID:   "inst_resume",
				SessionToken: "server-session-token",
				ExpiresAt:    time.Now().Add(time.Hour).Unix(),
			})
			if err != nil {
				t.Fatalf("NewControlFrame() error = %v", err)
			}
			ackBytes, err := EncodeFrame(ack)
			if err != nil {
				t.Fatalf("EncodeFrame() error = %v", err)
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, ackBytes); err != nil {
				t.Fatalf("WriteMessage() error = %v", err)
			}
			time.Sleep(50 * time.Millisecond)
			_ = conn.Close()
		case 2:
			rejectFrame, err := NewControlFrame(FrameError, 0, ErrorPayload{
				Code:    ErrCodeAuthFailed,
				Message: "stale session",
			})
			if err != nil {
				t.Fatalf("NewControlFrame() error = %v", err)
			}
			rejectBytes, err := EncodeFrame(rejectFrame)
			if err != nil {
				t.Fatalf("EncodeFrame() error = %v", err)
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, rejectBytes); err != nil {
				t.Fatalf("WriteMessage() error = %v", err)
			}
			secondRejectAt <- time.Now()
		case 3:
			ack, err := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
				InstanceID:   "inst_resume",
				SessionToken: "server-session-token-v2",
				ExpiresAt:    time.Now().Add(time.Hour).Unix(),
			})
			if err != nil {
				t.Fatalf("NewControlFrame() error = %v", err)
			}
			ackBytes, err := EncodeFrame(ack)
			if err != nil {
				t.Fatalf("EncodeFrame() error = %v", err)
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, ackBytes); err != nil {
				t.Fatalf("WriteMessage() error = %v", err)
			}
			select {
			case thirdAcked <- struct{}{}:
			default:
			}
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		default:
			t.Fatalf("unexpected register attempt %d", attempt)
		}
	})
	defer relayServer.Close()

	client := NewClient(Config{
		Enabled:        true,
		ServerURL:      wsURL(relayServer),
		InstanceSecret: "raw-instance-secret",
	}, ClientDeps{
		LicenseTokenFunc: func() string { return "test-license-jwt" },
		TokenValidator:   func(string) bool { return true },
		LocalAddr:        "127.0.0.1:9999",
		ServerVersion:    "test-version",
		IdentityPubKey:   "test-identity-pub-key",
	}, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	first := <-registers
	if first.sessionToken != "" {
		t.Fatalf("first register session_token = %q, want empty", first.sessionToken)
	}
	if first.instanceHint != "raw-instance-secret" {
		t.Fatalf("first register instance_hint = %q, want %q", first.instanceHint, "raw-instance-secret")
	}

	var second registerAttempt
	select {
	case second = <-registers:
	case <-time.After(8 * time.Second):
		t.Fatal("timed out waiting for resumed REGISTER")
	}
	if second.sessionToken != "server-session-token" {
		t.Fatalf("second register session_token = %q, want %q", second.sessionToken, "server-session-token")
	}
	if second.instanceHint != "inst_resume" {
		t.Fatalf("second register instance_hint = %q, want %q", second.instanceHint, "inst_resume")
	}

	rejectedAt := <-secondRejectAt

	select {
	case third := <-registers:
		if third.receivedAt.Sub(rejectedAt) >= baseReconnectDelay {
			t.Fatalf("fresh fallback register arrived after %v, want sooner than base reconnect delay %v", third.receivedAt.Sub(rejectedAt), baseReconnectDelay)
		}
		if third.sessionToken != "" {
			t.Fatalf("third register session_token = %q, want empty after stale session rejection", third.sessionToken)
		}
		if third.instanceHint != "raw-instance-secret" {
			t.Fatalf("third register instance_hint = %q, want %q after stale session rejection", third.instanceHint, "raw-instance-secret")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for fresh fallback REGISTER after session rejection")
	}

	select {
	case <-thirdAcked:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for fallback registration ACK")
	}

	pollUntil(t, 2*time.Second, "client reconnected after fallback registration", func() bool {
		return client.Status().Connected
	})

	cancel()
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Fatal("client.Run didn't return after cancel")
	}
}
