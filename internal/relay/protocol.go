// Package relay implements the Pulse relay client for mobile remote access.
//
// PROTOCOL SYNC: The types and constants in protocol.go are copied from
// pulse-pro/relay-server/protocol.go and MUST stay wire-compatible.
// After changing either copy, run: go test ./internal/relay/ -run TestProtocolDriftGuardrail
package relay

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

// Protocol version
const ProtocolVersion = 0x01

// Maximum payload size (64KB)
const MaxPayloadSize = 64 * 1024

// Header size: 1 (version) + 1 (type) + 4 (channel) = 6 bytes
const HeaderSize = 6

// Frame types
const (
	FrameRegister         = 0x01
	FrameRegisterAck      = 0x02
	FrameConnect          = 0x03
	FrameConnectAck       = 0x04
	FrameChannelOpen      = 0x05
	FrameChannelClose     = 0x06
	FrameData             = 0x07
	FramePing             = 0x08
	FramePong             = 0x09
	FrameError            = 0x0A
	FrameDrain            = 0x0B
	FrameKeyExchange      = 0x0C
	FramePushNotification = 0x0D
)

// Error codes sent in ERROR frames
const (
	ErrCodeInternal       = "INTERNAL_ERROR"
	ErrCodeNotFound       = "INSTANCE_NOT_FOUND"
	ErrCodeAuthFailed     = "AUTH_FAILED"
	ErrCodeLicenseInvalid = "LICENSE_INVALID"
	ErrCodeLicenseExpired = "LICENSE_EXPIRED"
	ErrCodeRateLimited    = "RATE_LIMITED"
	ErrCodeDuplicate      = "DUPLICATE_INSTANCE"
	ErrCodeChannelFull    = "CHANNEL_LIMIT_REACHED"
	ErrCodeDraining       = "SERVER_DRAINING"
)

var (
	ErrFrameTooShort      = errors.New("frame too short: need at least 6 bytes")
	ErrUnsupportedVersion = errors.New("unsupported protocol version")
	ErrUnknownFrameType   = errors.New("unknown frame type")
	ErrPayloadTooLarge    = errors.New("payload exceeds maximum size")
)

// frameTypeName maps type bytes to names for debugging.
var frameTypeName = map[byte]string{
	FrameRegister:         "REGISTER",
	FrameRegisterAck:      "REGISTER_ACK",
	FrameConnect:          "CONNECT",
	FrameConnectAck:       "CONNECT_ACK",
	FrameChannelOpen:      "CHANNEL_OPEN",
	FrameChannelClose:     "CHANNEL_CLOSE",
	FrameData:             "DATA",
	FramePing:             "PING",
	FramePong:             "PONG",
	FrameError:            "ERROR",
	FrameDrain:            "DRAIN",
	FrameKeyExchange:      "KEY_EXCHANGE",
	FramePushNotification: "PUSH_NOTIFICATION",
}

// FrameTypeName returns the human-readable name of a frame type.
func FrameTypeName(t byte) string {
	if name, ok := frameTypeName[t]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN(0x%02X)", t)
}

// Frame represents a relay protocol frame.
type Frame struct {
	Version byte
	Type    byte
	Channel uint32
	Payload []byte
}

// EncodeFrame serializes a frame into bytes.
func EncodeFrame(f Frame) ([]byte, error) {
	if len(f.Payload) > MaxPayloadSize {
		return nil, ErrPayloadTooLarge
	}
	buf := make([]byte, HeaderSize+len(f.Payload))
	buf[0] = f.Version
	buf[1] = f.Type
	binary.BigEndian.PutUint32(buf[2:6], f.Channel)
	copy(buf[HeaderSize:], f.Payload)
	return buf, nil
}

// DecodeFrame deserializes bytes into a frame.
func DecodeFrame(data []byte) (Frame, error) {
	if len(data) < HeaderSize {
		return Frame{}, ErrFrameTooShort
	}
	version := data[0]
	if version != ProtocolVersion {
		return Frame{}, fmt.Errorf("%w: got %d, want %d", ErrUnsupportedVersion, version, ProtocolVersion)
	}
	frameType := data[1]
	if _, ok := frameTypeName[frameType]; !ok {
		return Frame{}, fmt.Errorf("%w: 0x%02X", ErrUnknownFrameType, frameType)
	}
	payload := data[HeaderSize:]
	if len(payload) > MaxPayloadSize {
		return Frame{}, ErrPayloadTooLarge
	}
	return Frame{
		Version: version,
		Type:    frameType,
		Channel: binary.BigEndian.Uint32(data[2:6]),
		Payload: payload,
	}, nil
}

// --- Control frame JSON payloads ---

// RegisterPayload is sent by the instance in REGISTER frames.
type RegisterPayload struct {
	LicenseToken   string `json:"license_token"`
	SessionToken   string `json:"session_token,omitempty"`
	InstanceHint   string `json:"instance_hint,omitempty"`
	ClientVersion  string `json:"client_version,omitempty"`
	IdentityPubKey string `json:"identity_pub_key,omitempty"`
}

// RegisterAckPayload is sent by the relay in REGISTER_ACK frames.
type RegisterAckPayload struct {
	InstanceID   string `json:"instance_id"`
	SessionToken string `json:"session_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

// ConnectPayload is sent by the app in CONNECT frames.
type ConnectPayload struct {
	InstanceID  string `json:"instance_id"`
	AuthToken   string `json:"auth_token"`
	DeviceToken string `json:"device_token,omitempty"` // push notification device token (mobile apps only)
	Platform    string `json:"platform,omitempty"`     // "ios" or "android" (mobile apps only)
}

// ConnectAckPayload is sent by the relay in CONNECT_ACK frames.
type ConnectAckPayload struct {
	ChannelID  uint32 `json:"channel_id"`
	InstanceID string `json:"instance_id"`
}

// ChannelOpenPayload is sent by the relay to the instance in CHANNEL_OPEN frames.
type ChannelOpenPayload struct {
	ChannelID uint32 `json:"channel_id"`
	AuthToken string `json:"auth_token"`
}

// ChannelClosePayload is sent in CHANNEL_CLOSE frames.
type ChannelClosePayload struct {
	ChannelID uint32 `json:"channel_id"`
	Reason    string `json:"reason,omitempty"`
}

// ErrorPayload is sent in ERROR frames.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// DrainPayload is sent in DRAIN frames.
type DrainPayload struct {
	Reason    string `json:"reason,omitempty"`
	ReconnURL string `json:"reconn_url,omitempty"`
}

// PushNotificationPayload is sent in PUSH_NOTIFICATION frames.
// Push payloads are visible to Apple/Google — they must NOT contain
// API keys, IP addresses, node names, detailed metrics, or anything
// that would expose infrastructure details.
type PushNotificationPayload struct {
	Type       string `json:"type"`                  // "patrol_finding", "patrol_critical", "approval_request", "fix_completed"
	Priority   string `json:"priority"`              // "normal", "high"
	Title      string `json:"title"`                 // Short title (≤100 chars)
	Body       string `json:"body"`                  // Body text (≤200 chars)
	ActionType string `json:"action_type,omitempty"` // "view_finding", "approve_fix", "view_fix_result"
	ActionID   string `json:"action_id,omitempty"`   // Finding ID or Approval ID
	Category   string `json:"category,omitempty"`    // Finding category (performance, capacity, etc.)
	Severity   string `json:"severity,omitempty"`    // Finding severity
}

// MarshalControlPayload encodes a control frame payload as JSON bytes.
func MarshalControlPayload[T any](v T) ([]byte, error) {
	return json.Marshal(v)
}

// UnmarshalControlPayload decodes a JSON payload into the target struct.
func UnmarshalControlPayload[T any](data []byte, out *T) error {
	return json.Unmarshal(data, out)
}

// NewFrame creates a frame with the current protocol version.
func NewFrame(frameType byte, channel uint32, payload []byte) Frame {
	return Frame{
		Version: ProtocolVersion,
		Type:    frameType,
		Channel: channel,
		Payload: payload,
	}
}

// NewControlFrame creates a frame with a JSON-encoded control payload.
func NewControlFrame[T any](frameType byte, channel uint32, payload T) (Frame, error) {
	data, err := MarshalControlPayload(payload)
	if err != nil {
		return Frame{}, fmt.Errorf("marshal control payload: %w", err)
	}
	return NewFrame(frameType, channel, data), nil
}

// NewErrorFrame creates an ERROR frame with the given code and message.
func NewErrorFrame(channel uint32, code, message string) (Frame, error) {
	return NewControlFrame(FrameError, channel, ErrorPayload{
		Code:    code,
		Message: message,
	})
}

// NewPingFrame creates a PING frame.
func NewPingFrame() Frame {
	return NewFrame(FramePing, 0, nil)
}

// NewPongFrame creates a PONG frame.
func NewPongFrame() Frame {
	return NewFrame(FramePong, 0, nil)
}
