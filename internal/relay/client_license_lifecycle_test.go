package relay

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// TestRunLoop_LicenseTokenRefreshedOnReconnect verifies that the relay client
// calls LicenseTokenFunc on each registration attempt, picking up rotated
// tokens (e.g. from grant refresh). This is critical for v6 grant lifecycle:
// when the licensing subsystem refreshes the grant JWT, the relay client must
// use the fresh token on its next registration.
func TestRunLoop_LicenseTokenRefreshedOnReconnect(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	var mu sync.Mutex
	tokensReceived := []string{}

	// Mock relay server: accept REGISTER, record token, reply, then close.
	// The client will reconnect and re-register with a potentially new token.
	connectionCount := int32(0)
	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		attempt := atomic.AddInt32(&connectionCount, 1)

		// Read REGISTER
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

		mu.Lock()
		tokensReceived = append(tokensReceived, reg.LicenseToken)
		mu.Unlock()

		// Send REGISTER_ACK
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_lifecycle",
			SessionToken: "sess_lifecycle",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		// After two connections, stay alive so the test can assert
		if attempt >= 2 {
			// Block until the connection is closed
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}

		// For the first connection, close after a short pause to trigger reconnect
		time.Sleep(50 * time.Millisecond)
		_ = conn.Close()
	})
	defer relayServer.Close()

	// Token rotates: first call returns "token-v1", subsequent calls return "token-v2"
	callCount := int32(0)
	deps := ClientDeps{
		LicenseTokenFunc: func() string {
			n := atomic.AddInt32(&callCount, 1)
			if n == 1 {
				return "token-v1"
			}
			return "token-v2"
		},
		TokenValidator: func(token string) bool { return true },
		LocalAddr:      "127.0.0.1:9999",
		ServerVersion:  "test",
		IdentityPubKey: "test-pub-key",
	}

	client := NewClient(Config{Enabled: true, ServerURL: wsURL(relayServer)}, deps, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- client.Run(ctx) }()

	// Wait until we've seen at least 2 registrations
	deadline := time.After(8 * time.Second)
	for {
		mu.Lock()
		n := len(tokensReceived)
		mu.Unlock()
		if n >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for 2 registration attempts")
		case <-time.After(100 * time.Millisecond):
		}
	}

	mu.Lock()
	defer mu.Unlock()

	if tokensReceived[0] != "token-v1" {
		t.Errorf("first registration token = %q, want %q", tokensReceived[0], "token-v1")
	}
	if tokensReceived[1] != "token-v2" {
		t.Errorf("second registration token = %q, want %q", tokensReceived[1], "token-v2")
	}

	cancel()
	<-errCh
}

// TestRunLoop_LicenseErrorPausesReconnect verifies that when the relay server
// returns a license error (LICENSE_EXPIRED), the Run loop pauses for
// maxReconnectDelay before retrying. We contrast this with a non-license error
// (server close) which uses normal backoff (~5s) and retries quickly.
//
// The test runs two scenarios against the same 10s window:
//   - Non-license disconnect: expects ≥2 registration attempts (normal backoff ~5s)
//   - License error: expects exactly 1 attempt (maxReconnectDelay = 5 min >> 10s)
//
// This contrast proves the license error path uses the extended delay, not
// just that the first attempt happened.
func TestRunLoop_LicenseErrorPausesReconnect(t *testing.T) {
	// Subtest 1: Non-license error allows quick reconnect
	t.Run("non_license_error_allows_retry", func(t *testing.T) {
		logger := zerolog.New(zerolog.NewTestWriter(t))

		var mu sync.Mutex
		attempts := 0

		relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			frame, err := DecodeFrame(msg)
			if err != nil || frame.Type != FrameRegister {
				return
			}

			mu.Lock()
			attempts++
			mu.Unlock()

			// Register successfully, then close connection (non-license error)
			ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
				InstanceID:   "inst_test",
				SessionToken: "sess_test",
				ExpiresAt:    time.Now().Add(time.Hour).Unix(),
			})
			ackBytes, _ := EncodeFrame(ack)
			_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

			// Close server side to trigger reconnect (non-license error path)
			time.Sleep(50 * time.Millisecond)
			_ = conn.Close()
		})
		defer relayServer.Close()

		deps := ClientDeps{
			LicenseTokenFunc: func() string { return "valid-jwt" },
			TokenValidator:   func(token string) bool { return true },
			LocalAddr:        "127.0.0.1:9999",
			ServerVersion:    "test",
			IdentityPubKey:   "test-pub-key",
		}

		client := NewClient(Config{Enabled: true, ServerURL: wsURL(relayServer)}, deps, logger)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		errCh := make(chan error, 1)
		go func() { errCh <- client.Run(ctx) }()

		// Wait for at least 2 attempts within the 10s window (normal backoff ~5s)
		deadline := time.After(9 * time.Second)
		for {
			mu.Lock()
			n := attempts
			mu.Unlock()
			if n >= 2 {
				break
			}
			select {
			case <-deadline:
				mu.Lock()
				n := attempts
				mu.Unlock()
				t.Fatalf("expected ≥2 attempts with normal backoff within 10s, got %d", n)
			case <-time.After(100 * time.Millisecond):
			}
		}

		cancel()
		<-errCh
	})

	// Subtest 2: License error blocks reconnect for maxReconnectDelay (5 min)
	t.Run("license_error_blocks_retry", func(t *testing.T) {
		logger := zerolog.New(zerolog.NewTestWriter(t))

		var mu sync.Mutex
		attempts := 0

		relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			frame, err := DecodeFrame(msg)
			if err != nil || frame.Type != FrameRegister {
				return
			}

			mu.Lock()
			attempts++
			mu.Unlock()

			// Register successfully, then send license error
			ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
				InstanceID:   "inst_test",
				SessionToken: "sess_test",
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
		// License error triggers maxReconnectDelay (5 min).
		// With a 10s window, only 1 attempt should occur.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		errCh := make(chan error, 1)
		go func() { errCh <- client.Run(ctx) }()

		select {
		case <-errCh:
		case <-time.After(15 * time.Second):
			t.Fatal("Run() didn't exit after context cancellation")
		}

		mu.Lock()
		defer mu.Unlock()
		if attempts != 1 {
			t.Errorf("expected exactly 1 registration attempt (license error pauses for %v), got %d", maxReconnectDelay, attempts)
		}
	})
}

// TestRegister_LicenseRejectionThenRecovery verifies the registration-level
// recovery flow: a bad license token is rejected, then a good token succeeds.
// This proves that the relay client re-invokes LicenseTokenFunc (which will
// return the refreshed grant) and the relay server accepts the new token.
//
// Note: this tests register() directly rather than Run(), because the Run
// loop's license-error backoff is 5 minutes — too long for a unit test. The
// Run loop's backoff behavior is separately verified by
// TestRunLoop_LicenseErrorPausesReconnect.
func TestRegister_LicenseRejectionThenRecovery(t *testing.T) {
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

		if reg.LicenseToken == "bad-token" {
			errFrame, _ := NewControlFrame(FrameError, 0, ErrorPayload{
				Code:    ErrCodeLicenseInvalid,
				Message: "invalid license",
			})
			data, _ := EncodeFrame(errFrame)
			_ = conn.WriteMessage(websocket.BinaryMessage, data)
			time.Sleep(50 * time.Millisecond)
			return
		}

		// Accept "good-token"
		ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
			InstanceID:   "inst_recovered",
			SessionToken: "sess_recovered",
			ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		})
		ackBytes, _ := EncodeFrame(ack)
		_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer relayServer.Close()

	client := NewClient(Config{}, ClientDeps{
		LicenseTokenFunc: func() string { return "bad-token" },
		ServerVersion:    "test",
	}, zerolog.Nop())

	// Step 1: Register with bad token — expect LICENSE_INVALID rejection
	badConn, _, err := websocket.DefaultDialer.Dial(wsURL(relayServer), nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	regErr := client.register(badConn)
	badConn.Close()

	if regErr == nil {
		t.Fatal("expected register() to fail with bad token")
	}
	if !strings.Contains(regErr.Error(), "LICENSE_INVALID") {
		t.Fatalf("register() error = %q, want substring %q", regErr.Error(), "LICENSE_INVALID")
	}

	// Step 2: Simulate grant refresh by switching to a good token
	client.deps.LicenseTokenFunc = func() string { return "good-token" }
	goodConn, _, err := websocket.DefaultDialer.Dial(wsURL(relayServer), nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer goodConn.Close()

	regErr = client.register(goodConn)
	if regErr != nil {
		t.Fatalf("register() with good token failed: %v", regErr)
	}

	status := client.Status()
	if status.InstanceID != "inst_recovered" {
		t.Errorf("instance_id = %q, want %q", status.InstanceID, "inst_recovered")
	}
}

// TestRunLoop_LicenseErrorIsDetectedByIsLicenseError verifies that the error
// returned from readPump when the relay sends LICENSE_EXPIRED or LICENSE_INVALID
// is correctly identified by isLicenseError(). This is the exact branching point
// in Run() that determines whether maxReconnectDelay or normal backoff is used.
func TestRunLoop_LicenseErrorIsDetectedByIsLicenseError(t *testing.T) {
	for _, code := range []string{ErrCodeLicenseInvalid, ErrCodeLicenseExpired} {
		t.Run(code, func(t *testing.T) {
			relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
				frame, _ := NewControlFrame(FrameError, 0, ErrorPayload{
					Code:    code,
					Message: "test: " + code,
				})
				data, _ := EncodeFrame(frame)
				_ = conn.WriteMessage(websocket.BinaryMessage, data)
				time.Sleep(50 * time.Millisecond)
			})
			defer relayServer.Close()

			client := NewClient(Config{}, ClientDeps{}, zerolog.Nop())
			wsConn, _, err := websocket.DefaultDialer.Dial(wsURL(relayServer), nil)
			if err != nil {
				t.Fatalf("Dial() error = %v", err)
			}
			defer wsConn.Close()

			rpErr := client.readPump(context.Background(), wsConn, make(chan []byte, 1), make(chan struct{}, 1))
			if rpErr == nil {
				t.Fatal("expected readPump() to return error")
			}
			if !isLicenseError(rpErr) {
				t.Errorf("isLicenseError(%v) = false, want true for code %q", rpErr, code)
			}
		})
	}
}

// TestRunLoop_NonLicenseErrorIsNotLicenseError verifies that non-license relay
// errors (AUTH_FAILED, INTERNAL_ERROR, RATE_LIMITED) are NOT classified as
// license errors by isLicenseError(). Combined with the contrast test in
// TestRunLoop_LicenseErrorPausesReconnect, this proves the full branching:
// license errors → maxReconnectDelay, non-license errors → normal backoff.
func TestRunLoop_NonLicenseErrorIsNotLicenseError(t *testing.T) {
	nonLicenseCodes := []string{ErrCodeAuthFailed, ErrCodeInternal, ErrCodeRateLimited}

	for _, code := range nonLicenseCodes {
		t.Run(code, func(t *testing.T) {
			relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
				frame, _ := NewControlFrame(FrameError, 0, ErrorPayload{
					Code:    code,
					Message: "test: " + code,
				})
				data, _ := EncodeFrame(frame)
				_ = conn.WriteMessage(websocket.BinaryMessage, data)
				time.Sleep(50 * time.Millisecond)
			})
			defer relayServer.Close()

			client := NewClient(Config{}, ClientDeps{}, zerolog.Nop())
			wsConn, _, err := websocket.DefaultDialer.Dial(wsURL(relayServer), nil)
			if err != nil {
				t.Fatalf("Dial() error = %v", err)
			}
			defer wsConn.Close()

			rpErr := client.readPump(context.Background(), wsConn, make(chan []byte, 1), make(chan struct{}, 1))
			// Non-license errors either return nil or a non-license error
			if rpErr != nil && isLicenseError(rpErr) {
				t.Errorf("isLicenseError() = true for non-license code %q", code)
			}
		})
	}
}

// TestRunLoop_MidSessionLicenseErrorAfterChannelOpen verifies that when a
// LICENSE_EXPIRED or LICENSE_INVALID error arrives after successful registration
// AND after a channel has been opened (true mid-session), the Run loop correctly
// detects it as a license error and applies maxReconnectDelay (5 min). This
// covers the gap where existing tests send the error immediately after
// REGISTER_ACK without any channel activity first.
func TestRunLoop_MidSessionLicenseErrorAfterChannelOpen(t *testing.T) {
	for _, code := range []string{ErrCodeLicenseExpired, ErrCodeLicenseInvalid} {
		t.Run(code, func(t *testing.T) {
			logger := zerolog.New(zerolog.NewTestWriter(t))

			var mu sync.Mutex
			attempts := 0

			relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
				// Read REGISTER
				_, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				frame, err := DecodeFrame(msg)
				if err != nil || frame.Type != FrameRegister {
					return
				}

				mu.Lock()
				attempts++
				mu.Unlock()

				// Send REGISTER_ACK (registration succeeds)
				ack, _ := NewControlFrame(FrameRegisterAck, 0, RegisterAckPayload{
					InstanceID:   "inst_midsession",
					SessionToken: "sess_midsession",
					ExpiresAt:    time.Now().Add(time.Hour).Unix(),
				})
				ackBytes, _ := EncodeFrame(ack)
				_ = conn.WriteMessage(websocket.BinaryMessage, ackBytes)

				// Send CHANNEL_OPEN to establish a channel (simulates app connecting)
				chanOpen, _ := NewControlFrame(FrameChannelOpen, 1, ChannelOpenPayload{
					ChannelID: 1,
					AuthToken: "app_test_token",
				})
				chanOpenBytes, _ := EncodeFrame(chanOpen)
				_ = conn.WriteMessage(websocket.BinaryMessage, chanOpenBytes)

				// Brief delay to let readPump process the channel open
				time.Sleep(50 * time.Millisecond)

				// NOW send license error mid-session (after channel is active)
				errFrame, _ := NewControlFrame(FrameError, 0, ErrorPayload{
					Code:    code,
					Message: "license revoked mid-session",
				})
				errBytes, _ := EncodeFrame(errFrame)
				_ = conn.WriteMessage(websocket.BinaryMessage, errBytes)

				// Keep connection alive briefly so the client processes the error
				time.Sleep(200 * time.Millisecond)
			})
			defer relayServer.Close()

			deps := ClientDeps{
				LicenseTokenFunc: func() string { return "valid-jwt" },
				TokenValidator:   func(token string) bool { return true },
				LocalAddr:        "127.0.0.1:9999",
				ServerVersion:    "test",
				IdentityPubKey:   "test-pub-key",
			}

			client := NewClient(Config{Enabled: true, ServerURL: wsURL(relayServer)}, deps, logger)
			// License error triggers maxReconnectDelay (5 min).
			// With a 10s context, only 1 attempt should occur.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			errCh := make(chan error, 1)
			go func() { errCh <- client.Run(ctx) }()

			select {
			case <-errCh:
			case <-time.After(15 * time.Second):
				t.Fatal("Run() didn't exit after context cancellation")
			}

			mu.Lock()
			defer mu.Unlock()
			if attempts != 1 {
				t.Errorf("expected exactly 1 registration attempt (%s mid-session pauses for %v), got %d", code, maxReconnectDelay, attempts)
			}
		})
	}
}

// TestRegister_ReceivesLicenseRejection verifies that when the relay server
// sends a LICENSE_INVALID or LICENSE_EXPIRED error during registration (before
// REGISTER_ACK), the error is properly returned from register().
func TestRegister_ReceivesLicenseRejection(t *testing.T) {
	for _, tc := range []struct {
		code    string
		wantErr string
	}{
		{ErrCodeLicenseInvalid, "LICENSE_INVALID"},
		{ErrCodeLicenseExpired, "LICENSE_EXPIRED"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
				_, _, err := conn.ReadMessage()
				if err != nil {
					return
				}
				errFrame, _ := NewControlFrame(FrameError, 0, ErrorPayload{
					Code:    tc.code,
					Message: "rejected",
				})
				data, _ := EncodeFrame(errFrame)
				_ = conn.WriteMessage(websocket.BinaryMessage, data)
			})
			defer relayServer.Close()

			client := NewClient(Config{}, ClientDeps{
				LicenseTokenFunc: func() string { return "some-jwt" },
				ServerVersion:    "test",
			}, zerolog.Nop())

			wsConn, _, err := websocket.DefaultDialer.Dial(wsURL(relayServer), nil)
			if err != nil {
				t.Fatalf("Dial() error = %v", err)
			}
			defer wsConn.Close()

			regErr := client.register(wsConn)
			if regErr == nil {
				t.Fatal("expected register() to fail with license rejection")
			}
			if !strings.Contains(regErr.Error(), tc.wantErr) {
				t.Errorf("register() error = %q, want substring %q", regErr.Error(), tc.wantErr)
			}
		})
	}
}
