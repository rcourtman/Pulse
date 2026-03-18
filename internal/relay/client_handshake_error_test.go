package relay

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

func TestClientRegister_NoLicenseToken(t *testing.T) {
	client := NewClient(Config{}, ClientDeps{
		LicenseTokenFunc: func() string { return "" },
	}, zerolog.Nop())

	err := client.register(nil)
	if err == nil {
		t.Fatal("expected register() to fail without license token")
	}
	if !strings.Contains(err.Error(), "no license token available") {
		t.Fatalf("register() error = %q, want substring %q", err.Error(), "no license token available")
	}
}

func TestClientRegister_ResponseErrorPaths(t *testing.T) {
	tests := []struct {
		name            string
		sendResponse    func(t *testing.T, conn *websocket.Conn)
		wantErrContains string
	}{
		{
			name: "relay error frame",
			sendResponse: func(t *testing.T, conn *websocket.Conn) {
				t.Helper()
				frame, err := NewControlFrame(FrameError, 0, ErrorPayload{
					Code:    ErrCodeAuthFailed,
					Message: "bad token",
				})
				if err != nil {
					t.Fatalf("NewControlFrame() error = %v", err)
				}
				data, err := EncodeFrame(frame)
				if err != nil {
					t.Fatalf("EncodeFrame() error = %v", err)
				}
				if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
					t.Fatalf("WriteMessage() error = %v", err)
				}
			},
			wantErrContains: "relay error (AUTH_FAILED): bad token",
		},
		{
			name: "unexpected frame type",
			sendResponse: func(t *testing.T, conn *websocket.Conn) {
				t.Helper()
				data, err := EncodeFrame(NewPingFrame())
				if err != nil {
					t.Fatalf("EncodeFrame() error = %v", err)
				}
				if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
					t.Fatalf("WriteMessage() error = %v", err)
				}
			},
			wantErrContains: "unexpected frame type during registration: PING",
		},
		{
			name: "malformed register ack payload",
			sendResponse: func(t *testing.T, conn *websocket.Conn) {
				t.Helper()
				data, err := EncodeFrame(NewFrame(FrameRegisterAck, 0, []byte("not-json")))
				if err != nil {
					t.Fatalf("EncodeFrame() error = %v", err)
				}
				if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
					t.Fatalf("WriteMessage() error = %v", err)
				}
			},
			wantErrContains: "unmarshal register ack",
		},
		{
			name: "malformed wire frame",
			sendResponse: func(t *testing.T, conn *websocket.Conn) {
				t.Helper()
				if err := conn.WriteMessage(websocket.BinaryMessage, []byte("bad")); err != nil {
					t.Fatalf("WriteMessage() error = %v", err)
				}
			},
			wantErrContains: "decode register response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
				_, _, err := conn.ReadMessage()
				if err != nil {
					t.Logf("ReadMessage() error = %v", err)
					return
				}
				tt.sendResponse(t, conn)
			})
			defer relayServer.Close()

			client := NewClient(Config{}, ClientDeps{
				LicenseTokenFunc: func() string { return "test-license-jwt" },
				ServerVersion:    "test-version",
			}, zerolog.Nop())

			conn, _, err := websocket.DefaultDialer.Dial(wsURL(relayServer), nil)
			if err != nil {
				t.Fatalf("Dial() error = %v", err)
			}
			defer conn.Close()

			err = client.register(conn)
			if err == nil {
				t.Fatal("expected register() error")
			}
			if !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Fatalf("register() error = %q, want substring %q", err.Error(), tt.wantErrContains)
			}
		})
	}
}

func TestClientReadPump_ReturnsLicenseError(t *testing.T) {
	relayServer := mockRelayServer(t, func(conn *websocket.Conn) {
		frame, err := NewControlFrame(FrameError, 0, ErrorPayload{
			Code:    ErrCodeLicenseInvalid,
			Message: "invalid license",
		})
		if err != nil {
			t.Fatalf("NewControlFrame() error = %v", err)
		}
		data, err := EncodeFrame(frame)
		if err != nil {
			t.Fatalf("EncodeFrame() error = %v", err)
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
			t.Logf("WriteMessage() error = %v", err)
			return
		}
		time.Sleep(50 * time.Millisecond)
	})
	defer relayServer.Close()

	client := NewClient(Config{}, ClientDeps{}, zerolog.Nop())

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(relayServer), nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	err = client.readPump(context.Background(), conn, make(chan []byte, 1), make(chan struct{}, 1))
	if err == nil {
		t.Fatal("expected readPump() to return license error")
	}
	if !isLicenseError(err) {
		t.Fatalf("readPump() error type = %T, want licenseError", err)
	}

	var licenseErr *licenseError
	if !errors.As(err, &licenseErr) {
		t.Fatalf("readPump() error = %v, want *licenseError", err)
	}
	if licenseErr.code != ErrCodeLicenseInvalid {
		t.Fatalf("license error code = %q, want %q", licenseErr.code, ErrCodeLicenseInvalid)
	}
}

func TestHandleKeyExchange_SigningFailureClosesChannel(t *testing.T) {
	appEphemeral, err := GenerateEphemeralKeyPair()
	if err != nil {
		t.Fatalf("GenerateEphemeralKeyPair() error = %v", err)
	}

	client := NewClient(Config{}, ClientDeps{
		IdentityPrivateKey: "invalid-base64-private-key",
	}, zerolog.Nop())
	client.channels[7] = &channelState{apiToken: "token"}

	frame := NewFrame(FrameKeyExchange, 7, MarshalKeyExchangePayload(appEphemeral.PublicKey().Bytes(), nil))
	sendCh := make(chan []byte, 1)

	client.handleKeyExchange(frame, sendCh)

	client.mu.RLock()
	_, exists := client.channels[7]
	client.mu.RUnlock()
	if exists {
		t.Fatal("expected channel to be removed after key exchange signing failure")
	}

	select {
	case msg := <-sendCh:
		closeFrame, err := DecodeFrame(msg)
		if err != nil {
			t.Fatalf("DecodeFrame() error = %v", err)
		}
		if closeFrame.Type != FrameChannelClose {
			t.Fatalf("frame type = %s, want CHANNEL_CLOSE", FrameTypeName(closeFrame.Type))
		}

		var payload ChannelClosePayload
		if err := UnmarshalControlPayload(closeFrame.Payload, &payload); err != nil {
			t.Fatalf("UnmarshalControlPayload() error = %v", err)
		}
		if payload.Reason != "key exchange signing failed" {
			t.Fatalf("close reason = %q, want %q", payload.Reason, "key exchange signing failed")
		}
	default:
		t.Fatal("expected CHANNEL_CLOSE frame after key exchange signing failure")
	}
}
