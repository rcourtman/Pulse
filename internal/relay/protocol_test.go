package relay

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		frame   Frame
		wantErr bool
	}{
		{
			name:  "ping frame (no payload)",
			frame: NewPingFrame(),
		},
		{
			name:  "pong frame (no payload)",
			frame: NewPongFrame(),
		},
		{
			name:  "data frame with payload",
			frame: NewFrame(FrameData, 42, []byte("hello world")),
		},
		{
			name:  "channel zero",
			frame: NewFrame(FrameData, 0, []byte("test")),
		},
		{
			name:  "max channel ID",
			frame: NewFrame(FrameData, 0xFFFFFFFF, []byte("test")),
		},
		{
			name:  "empty payload",
			frame: NewFrame(FrameData, 1, []byte{}),
		},
		{
			name:  "nil payload",
			frame: NewFrame(FrameData, 1, nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := EncodeFrame(tt.frame)
			if err != nil {
				t.Fatalf("EncodeFrame() error = %v", err)
			}

			decoded, err := DecodeFrame(encoded)
			if err != nil {
				t.Fatalf("DecodeFrame() error = %v", err)
			}

			if decoded.Version != tt.frame.Version {
				t.Errorf("Version: got %d, want %d", decoded.Version, tt.frame.Version)
			}
			if decoded.Type != tt.frame.Type {
				t.Errorf("Type: got %d, want %d", decoded.Type, tt.frame.Type)
			}
			if decoded.Channel != tt.frame.Channel {
				t.Errorf("Channel: got %d, want %d", decoded.Channel, tt.frame.Channel)
			}
			if !bytes.Equal(decoded.Payload, tt.frame.Payload) {
				// nil and empty slice are equivalent for our purposes
				if len(decoded.Payload) != 0 || len(tt.frame.Payload) != 0 {
					t.Errorf("Payload: got %v, want %v", decoded.Payload, tt.frame.Payload)
				}
			}
		})
	}
}

func TestControlFrameRoundTrip(t *testing.T) {
	t.Run("register payload", func(t *testing.T) {
		orig := RegisterPayload{
			LicenseToken:   "jwt-token-here",
			SessionToken:   "sess-abc",
			ClientVersion:  "1.0.0",
			IdentityPubKey: "identity-pub-key",
		}
		frame, err := NewControlFrame(FrameRegister, 0, orig)
		if err != nil {
			t.Fatalf("NewControlFrame() error = %v", err)
		}

		encoded, err := EncodeFrame(frame)
		if err != nil {
			t.Fatalf("EncodeFrame() error = %v", err)
		}

		decoded, err := DecodeFrame(encoded)
		if err != nil {
			t.Fatalf("DecodeFrame() error = %v", err)
		}

		var got RegisterPayload
		if err := UnmarshalControlPayload(decoded.Payload, &got); err != nil {
			t.Fatalf("UnmarshalControlPayload() error = %v", err)
		}

		if got.LicenseToken != orig.LicenseToken {
			t.Errorf("LicenseToken: got %q, want %q", got.LicenseToken, orig.LicenseToken)
		}
		if got.SessionToken != orig.SessionToken {
			t.Errorf("SessionToken: got %q, want %q", got.SessionToken, orig.SessionToken)
		}
		if got.ClientVersion != orig.ClientVersion {
			t.Errorf("ClientVersion: got %q, want %q", got.ClientVersion, orig.ClientVersion)
		}
		if got.IdentityPubKey != orig.IdentityPubKey {
			t.Errorf("IdentityPubKey: got %q, want %q", got.IdentityPubKey, orig.IdentityPubKey)
		}
	})

	t.Run("connect payload", func(t *testing.T) {
		orig := ConnectPayload{
			InstanceID:  "relay_abc123",
			AuthToken:   "auth-token-xyz",
			DeviceToken: "device-token-123",
			Platform:    "ios",
		}
		frame, err := NewControlFrame(FrameConnect, 0, orig)
		if err != nil {
			t.Fatalf("NewControlFrame() error = %v", err)
		}

		encoded, err := EncodeFrame(frame)
		if err != nil {
			t.Fatalf("EncodeFrame() error = %v", err)
		}

		decoded, err := DecodeFrame(encoded)
		if err != nil {
			t.Fatalf("DecodeFrame() error = %v", err)
		}

		var got ConnectPayload
		if err := UnmarshalControlPayload(decoded.Payload, &got); err != nil {
			t.Fatalf("UnmarshalControlPayload() error = %v", err)
		}

		if got.InstanceID != orig.InstanceID {
			t.Errorf("InstanceID: got %q, want %q", got.InstanceID, orig.InstanceID)
		}
		if got.AuthToken != orig.AuthToken {
			t.Errorf("AuthToken: got %q, want %q", got.AuthToken, orig.AuthToken)
		}
		if got.DeviceToken != orig.DeviceToken {
			t.Errorf("DeviceToken: got %q, want %q", got.DeviceToken, orig.DeviceToken)
		}
		if got.Platform != orig.Platform {
			t.Errorf("Platform: got %q, want %q", got.Platform, orig.Platform)
		}
	})

	t.Run("channel open payload", func(t *testing.T) {
		orig := ChannelOpenPayload{
			ChannelID: 7,
			AuthToken: "api-token-xyz",
		}
		frame, err := NewControlFrame(FrameChannelOpen, 7, orig)
		if err != nil {
			t.Fatalf("NewControlFrame() error = %v", err)
		}

		encoded, err := EncodeFrame(frame)
		if err != nil {
			t.Fatalf("EncodeFrame() error = %v", err)
		}

		decoded, err := DecodeFrame(encoded)
		if err != nil {
			t.Fatalf("DecodeFrame() error = %v", err)
		}

		var got ChannelOpenPayload
		if err := UnmarshalControlPayload(decoded.Payload, &got); err != nil {
			t.Fatalf("UnmarshalControlPayload() error = %v", err)
		}

		if got.ChannelID != orig.ChannelID {
			t.Errorf("ChannelID: got %d, want %d", got.ChannelID, orig.ChannelID)
		}
		if got.AuthToken != orig.AuthToken {
			t.Errorf("AuthToken: got %q, want %q", got.AuthToken, orig.AuthToken)
		}
	})

	t.Run("error payload", func(t *testing.T) {
		frame, err := NewErrorFrame(0, ErrCodeAuthFailed, "bad token")
		if err != nil {
			t.Fatalf("NewErrorFrame() error = %v", err)
		}

		encoded, err := EncodeFrame(frame)
		if err != nil {
			t.Fatalf("EncodeFrame() error = %v", err)
		}

		decoded, err := DecodeFrame(encoded)
		if err != nil {
			t.Fatalf("DecodeFrame() error = %v", err)
		}

		var got ErrorPayload
		if err := UnmarshalControlPayload(decoded.Payload, &got); err != nil {
			t.Fatalf("UnmarshalControlPayload() error = %v", err)
		}

		if got.Code != ErrCodeAuthFailed {
			t.Errorf("Code: got %q, want %q", got.Code, ErrCodeAuthFailed)
		}
		if got.Message != "bad token" {
			t.Errorf("Message: got %q, want %q", got.Message, "bad token")
		}
	})
}

func TestNewControlFrame_MarshalError(t *testing.T) {
	badPayload := struct {
		Func func()
	}{
		Func: func() {},
	}

	_, err := NewControlFrame(FrameRegister, 0, badPayload)
	if err == nil {
		t.Fatal("expected NewControlFrame() to fail for non-JSON payload")
	}
	if !strings.Contains(err.Error(), "marshal control payload") {
		t.Fatalf("NewControlFrame() error = %q, want marshal context", err.Error())
	}
}

// TestWireCompatibility verifies that hand-crafted byte sequences decode correctly.
// These vectors ensure the Go implementation stays compatible with the relay-server.
func TestWireCompatibility(t *testing.T) {
	t.Run("ping frame wire bytes", func(t *testing.T) {
		// Version=1, Type=PING(0x08), Channel=0, no payload
		wire := []byte{0x01, 0x08, 0x00, 0x00, 0x00, 0x00}
		f, err := DecodeFrame(wire)
		if err != nil {
			t.Fatalf("DecodeFrame() error = %v", err)
		}
		if f.Type != FramePing {
			t.Errorf("Type: got 0x%02X, want 0x%02X", f.Type, FramePing)
		}
		if f.Channel != 0 {
			t.Errorf("Channel: got %d, want 0", f.Channel)
		}
		if len(f.Payload) != 0 {
			t.Errorf("Payload: got %d bytes, want 0", len(f.Payload))
		}
	})

	t.Run("data frame wire bytes", func(t *testing.T) {
		// Version=1, Type=DATA(0x07), Channel=256 (0x00000100), Payload="OK"
		wire := []byte{0x01, 0x07, 0x00, 0x00, 0x01, 0x00, 'O', 'K'}
		f, err := DecodeFrame(wire)
		if err != nil {
			t.Fatalf("DecodeFrame() error = %v", err)
		}
		if f.Type != FrameData {
			t.Errorf("Type: got 0x%02X, want 0x%02X", f.Type, FrameData)
		}
		if f.Channel != 256 {
			t.Errorf("Channel: got %d, want 256", f.Channel)
		}
		if !bytes.Equal(f.Payload, []byte("OK")) {
			t.Errorf("Payload: got %q, want %q", f.Payload, "OK")
		}
	})

	t.Run("encode produces expected wire bytes", func(t *testing.T) {
		f := NewFrame(FrameChannelClose, 0x0000CAFE, []byte{0xDE, 0xAD})
		encoded, err := EncodeFrame(f)
		if err != nil {
			t.Fatalf("EncodeFrame() error = %v", err)
		}

		want := make([]byte, 8)
		want[0] = ProtocolVersion
		want[1] = FrameChannelClose
		binary.BigEndian.PutUint32(want[2:6], 0x0000CAFE)
		want[6] = 0xDE
		want[7] = 0xAD

		if !bytes.Equal(encoded, want) {
			t.Errorf("encoded:\n  got  %v\n  want %v", encoded, want)
		}
	})
}

func TestDecodeErrors(t *testing.T) {
	t.Run("too short", func(t *testing.T) {
		_, err := DecodeFrame([]byte{0x01, 0x08})
		if err == nil {
			t.Fatal("expected error for short frame")
		}
	})

	t.Run("wrong version", func(t *testing.T) {
		wire := []byte{0xFF, 0x08, 0x00, 0x00, 0x00, 0x00}
		_, err := DecodeFrame(wire)
		if err == nil {
			t.Fatal("expected error for wrong version")
		}
	})

	t.Run("unknown frame type", func(t *testing.T) {
		wire := []byte{0x01, 0xFF, 0x00, 0x00, 0x00, 0x00}
		_, err := DecodeFrame(wire)
		if err == nil {
			t.Fatal("expected error for unknown frame type")
		}
	})
}

func TestFrameTypeName(t *testing.T) {
	if got := FrameTypeName(FrameRegister); got != "REGISTER" {
		t.Errorf("got %q, want REGISTER", got)
	}
	if got := FrameTypeName(0xFF); got != "UNKNOWN(0xFF)" {
		t.Errorf("got %q, want UNKNOWN(0xFF)", got)
	}
}

// TestProtocolDriftGuardrail pins every protocol constant and frame type to its
// expected value. If pulse-pro/relay-server/protocol.go changes a constant and
// this file isn't updated to match, this test fails.
//
// Update this test whenever the relay-server protocol is intentionally changed.
func TestProtocolDriftGuardrail(t *testing.T) {
	// Protocol fundamentals
	if ProtocolVersion != 0x01 {
		t.Errorf("ProtocolVersion: got 0x%02X, want 0x01", ProtocolVersion)
	}
	if MaxPayloadSize != 64*1024 {
		t.Errorf("MaxPayloadSize: got %d, want %d", MaxPayloadSize, 64*1024)
	}
	if HeaderSize != 6 {
		t.Errorf("HeaderSize: got %d, want 6", HeaderSize)
	}

	// Frame type values — must match relay-server exactly
	expectedFrameTypes := map[byte]string{
		0x01: "REGISTER",
		0x02: "REGISTER_ACK",
		0x03: "CONNECT",
		0x04: "CONNECT_ACK",
		0x05: "CHANNEL_OPEN",
		0x06: "CHANNEL_CLOSE",
		0x07: "DATA",
		0x08: "PING",
		0x09: "PONG",
		0x0A: "ERROR",
		0x0B: "DRAIN",
		0x0C: "KEY_EXCHANGE",
		0x0D: "PUSH_NOTIFICATION",
	}

	for val, wantName := range expectedFrameTypes {
		gotName := FrameTypeName(val)
		if gotName != wantName {
			t.Errorf("frame type 0x%02X: got name %q, want %q", val, gotName, wantName)
		}
	}

	// Verify we haven't lost any frame types
	if len(frameTypeName) != len(expectedFrameTypes) {
		t.Errorf("frame type count: got %d, want %d (did relay-server add/remove a frame type?)",
			len(frameTypeName), len(expectedFrameTypes))
	}

	// Error code values — must match relay-server
	expectedErrCodes := map[string]string{
		ErrCodeInternal:       "INTERNAL_ERROR",
		ErrCodeNotFound:       "INSTANCE_NOT_FOUND",
		ErrCodeAuthFailed:     "AUTH_FAILED",
		ErrCodeLicenseInvalid: "LICENSE_INVALID",
		ErrCodeLicenseExpired: "LICENSE_EXPIRED",
		ErrCodeRateLimited:    "RATE_LIMITED",
		ErrCodeDuplicate:      "DUPLICATE_INSTANCE",
		ErrCodeChannelFull:    "CHANNEL_LIMIT_REACHED",
		ErrCodeDraining:       "SERVER_DRAINING",
	}
	for got, want := range expectedErrCodes {
		if got != want {
			t.Errorf("error code constant drift: got %q, want %q", got, want)
		}
	}
}
