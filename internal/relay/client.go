package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

const (
	// Reconnect backoff parameters
	baseReconnectDelay = 5 * time.Second
	maxReconnectDelay  = 5 * time.Minute
	reconnectJitter    = 0.1

	// WebSocket parameters
	wsPingInterval   = 25 * time.Second
	wsWriteWait      = 10 * time.Second
	wsHandshakeWait  = 15 * time.Second
	sendChBufferSize = 256
)

// TokenValidator validates an API token and returns the raw token if valid.
type TokenValidator func(token string) bool

// ClientDeps holds injectable dependencies for the relay client.
type ClientDeps struct {
	LicenseTokenFunc func() string  // returns the raw license JWT
	TokenValidator   TokenValidator // validates API tokens from CHANNEL_OPEN
	LocalAddr        string         // e.g. "127.0.0.1:7655"
	ServerVersion    string         // Pulse version for ClientVersion in REGISTER
	IdentityPubKey   string         // base64-encoded Ed25519 public key for MITM prevention
}

// ClientStatus represents the current state of the relay client.
type ClientStatus struct {
	Connected      bool   `json:"connected"`
	InstanceID     string `json:"instance_id,omitempty"`
	ActiveChannels int    `json:"active_channels"`
	LastError      string `json:"last_error,omitempty"`
	ReconnectIn    string `json:"reconnect_in,omitempty"`
}

// Client maintains a persistent connection to the relay server.
type Client struct {
	config Config
	deps   ClientDeps
	proxy  *HTTPProxy
	logger zerolog.Logger

	// Connection state (protected by mu)
	mu           sync.RWMutex
	conn         *websocket.Conn
	instanceID   string
	sessionToken string
	channels     map[uint32]string // channelID → apiToken
	connected    bool
	lastError    string

	// Lifecycle
	cancel context.CancelFunc
	done   chan struct{}
}

// NewClient creates a new relay client.
func NewClient(cfg Config, deps ClientDeps, logger zerolog.Logger) *Client {
	return &Client{
		config:   cfg,
		deps:     deps,
		proxy:    NewHTTPProxy(deps.LocalAddr, logger),
		logger:   logger,
		channels: make(map[uint32]string),
		done:     make(chan struct{}),
	}
}

// Run starts the reconnect loop. Blocks until ctx is cancelled.
func (c *Client) Run(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)
	defer close(c.done)

	consecutiveFailures := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := c.connectAndHandle(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			consecutiveFailures++
			c.mu.Lock()
			c.lastError = err.Error()
			c.connected = false
			c.mu.Unlock()

			delay := c.backoffDelay(consecutiveFailures)

			if consecutiveFailures >= 3 {
				c.logger.Warn().Err(err).
					Int("failures", consecutiveFailures).
					Dur("retry_in", delay).
					Msg("Relay connection failed repeatedly")
			} else {
				c.logger.Debug().Err(err).
					Dur("retry_in", delay).
					Msg("Relay connection interrupted, reconnecting")
			}

			// If it's a license error, pause longer
			if isLicenseError(err) {
				delay = maxReconnectDelay
				c.logger.Warn().Msg("License error from relay server, pausing reconnect")
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		} else {
			consecutiveFailures = 0
		}
	}
}

// Close stops the client and closes the connection.
func (c *Client) Close() {
	if c.cancel != nil {
		c.cancel()
	}
	// Wait for Run to finish
	select {
	case <-c.done:
	case <-time.After(5 * time.Second):
	}
}

// Status returns the current client status.
func (c *Client) Status() ClientStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return ClientStatus{
		Connected:      c.connected,
		InstanceID:     c.instanceID,
		ActiveChannels: len(c.channels),
		LastError:      c.lastError,
	}
}

func (c *Client) connectAndHandle(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: wsHandshakeWait,
	}

	c.logger.Info().Str("url", c.config.ServerURL).Msg("Connecting to relay server")

	conn, _, err := dialer.DialContext(ctx, c.config.ServerURL, nil)
	if err != nil {
		return fmt.Errorf("dial relay: %w", err)
	}

	// Per-connection send channel — no races because each writePump gets its own
	sendCh := make(chan []byte, sendChBufferSize)

	c.mu.Lock()
	c.conn = conn
	c.channels = make(map[uint32]string)
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.conn = nil
		c.connected = false
		c.mu.Unlock()
		conn.Close()
	}()

	// Register with relay server
	if err := c.register(conn); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	c.mu.Lock()
	c.connected = true
	c.lastError = ""
	c.mu.Unlock()

	c.logger.Info().Str("instance_id", c.instanceID).Msg("Registered with relay server")

	// Start write pump with per-connection sendCh
	writeCtx, writeCancel := context.WithCancel(ctx)
	defer writeCancel()
	go c.writePump(writeCtx, conn, sendCh)

	// Read pump (blocking) — passes sendCh for responses
	return c.readPump(ctx, conn, sendCh)
}

func (c *Client) register(conn *websocket.Conn) error {
	token := c.deps.LicenseTokenFunc()
	if token == "" {
		return fmt.Errorf("no license token available")
	}

	payload := RegisterPayload{
		LicenseToken:   token,
		InstanceHint:   c.config.InstanceSecret,
		ClientVersion:  c.deps.ServerVersion,
		IdentityPubKey: c.deps.IdentityPubKey,
	}

	// Reuse session token if we have one from a previous connection
	c.mu.RLock()
	payload.SessionToken = c.sessionToken
	c.mu.RUnlock()

	frame, err := NewControlFrame(FrameRegister, 0, payload)
	if err != nil {
		return fmt.Errorf("build register frame: %w", err)
	}

	data, err := EncodeFrame(frame)
	if err != nil {
		return fmt.Errorf("encode register frame: %w", err)
	}

	conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("send register: %w", err)
	}

	// Wait for REGISTER_ACK or ERROR
	conn.SetReadDeadline(time.Now().Add(wsHandshakeWait))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read register response: %w", err)
	}
	conn.SetReadDeadline(time.Time{})

	frame, err = DecodeFrame(msg)
	if err != nil {
		return fmt.Errorf("decode register response: %w", err)
	}

	switch frame.Type {
	case FrameRegisterAck:
		var ack RegisterAckPayload
		if err := UnmarshalControlPayload(frame.Payload, &ack); err != nil {
			return fmt.Errorf("unmarshal register ack: %w", err)
		}
		c.mu.Lock()
		c.instanceID = ack.InstanceID
		c.sessionToken = ack.SessionToken
		c.mu.Unlock()
		return nil

	case FrameError:
		var errPayload ErrorPayload
		if err := UnmarshalControlPayload(frame.Payload, &errPayload); err != nil {
			return fmt.Errorf("unmarshal error: %w", err)
		}
		return fmt.Errorf("relay error (%s): %s", errPayload.Code, errPayload.Message)

	default:
		return fmt.Errorf("unexpected frame type during registration: %s", FrameTypeName(frame.Type))
	}
}

func (c *Client) readPump(ctx context.Context, conn *websocket.Conn, sendCh chan<- []byte) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read message: %w", err)
		}

		frame, err := DecodeFrame(msg)
		if err != nil {
			c.logger.Warn().Err(err).Msg("Failed to decode frame, skipping")
			continue
		}

		switch frame.Type {
		case FrameChannelOpen:
			c.handleChannelOpen(frame, sendCh)

		case FrameData:
			c.handleData(frame, sendCh)

		case FrameChannelClose:
			c.handleChannelClose(frame)

		case FramePing:
			queueFrame(sendCh, NewPongFrame(), c.logger)

		case FrameDrain:
			var drain DrainPayload
			if err := UnmarshalControlPayload(frame.Payload, &drain); err == nil {
				c.logger.Info().Str("reason", drain.Reason).Msg("Relay server draining, will reconnect")
			}
			return nil // exit readPump, triggers reconnect

		case FrameError:
			var errPayload ErrorPayload
			if err := UnmarshalControlPayload(frame.Payload, &errPayload); err == nil {
				c.logger.Warn().Str("code", errPayload.Code).Str("message", errPayload.Message).Msg("Relay error")
				if errPayload.Code == ErrCodeLicenseInvalid || errPayload.Code == ErrCodeLicenseExpired {
					return &licenseError{code: errPayload.Code, message: errPayload.Message}
				}
			}

		default:
			c.logger.Debug().Str("type", FrameTypeName(frame.Type)).Msg("Ignoring unhandled frame type")
		}
	}
}

func (c *Client) writePump(ctx context.Context, conn *websocket.Conn, sendCh <-chan []byte) {
	ticker := time.NewTicker(wsPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Send close message
			conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return

		case data := <-sendCh:
			conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				c.logger.Debug().Err(err).Msg("Write failed")
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.logger.Debug().Err(err).Msg("WS ping failed")
				return
			}
		}
	}
}

func (c *Client) handleChannelOpen(frame Frame, sendCh chan<- []byte) {
	var payload ChannelOpenPayload
	if err := UnmarshalControlPayload(frame.Payload, &payload); err != nil {
		c.logger.Warn().Err(err).Msg("Failed to unmarshal CHANNEL_OPEN")
		return
	}

	// Validate the auth token
	if !c.deps.TokenValidator(payload.AuthToken) {
		c.logger.Warn().Uint32("channel", payload.ChannelID).Msg("Rejecting channel: invalid auth token")
		closeFrame, err := NewControlFrame(FrameChannelClose, payload.ChannelID, ChannelClosePayload{
			ChannelID: payload.ChannelID,
			Reason:    "invalid auth token",
		})
		if err == nil {
			queueFrame(sendCh, closeFrame, c.logger)
		}
		return
	}

	// Accept: store channel and echo CHANNEL_OPEN back
	c.mu.Lock()
	c.channels[payload.ChannelID] = payload.AuthToken
	c.mu.Unlock()

	c.logger.Info().Uint32("channel", payload.ChannelID).Msg("Channel opened")

	// Echo CHANNEL_OPEN to acknowledge
	ackFrame, err := NewControlFrame(FrameChannelOpen, payload.ChannelID, payload)
	if err == nil {
		queueFrame(sendCh, ackFrame, c.logger)
	}
}

func (c *Client) handleData(frame Frame, sendCh chan<- []byte) {
	channelID := frame.Channel

	c.mu.RLock()
	apiToken, ok := c.channels[channelID]
	c.mu.RUnlock()

	if !ok {
		c.logger.Warn().Uint32("channel", channelID).Msg("DATA for unknown channel")
		return
	}

	// Handle in background goroutine so we don't block the read pump
	go func() {
		respPayload, err := c.proxy.HandleRequest(frame.Payload, apiToken)
		if err != nil {
			c.logger.Warn().Err(err).Uint32("channel", channelID).Msg("Proxy error")
			return
		}

		respFrame := NewFrame(FrameData, channelID, respPayload)
		queueFrame(sendCh, respFrame, c.logger)
	}()
}

func (c *Client) handleChannelClose(frame Frame) {
	var payload ChannelClosePayload
	if err := UnmarshalControlPayload(frame.Payload, &payload); err != nil {
		// Fall back to using frame channel ID
		payload.ChannelID = frame.Channel
	}

	c.mu.Lock()
	delete(c.channels, payload.ChannelID)
	c.mu.Unlock()

	c.logger.Info().Uint32("channel", payload.ChannelID).Str("reason", payload.Reason).Msg("Channel closed")
}

// queueFrame encodes and sends a frame to the send channel (non-blocking).
func queueFrame(sendCh chan<- []byte, f Frame, logger zerolog.Logger) {
	data, err := EncodeFrame(f)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to encode frame for send")
		return
	}

	select {
	case sendCh <- data:
	default:
		logger.Warn().Msg("Send channel full, dropping frame")
	}
}

func (c *Client) backoffDelay(failures int) time.Duration {
	delay := baseReconnectDelay * time.Duration(math.Pow(2, float64(failures-1)))
	if delay > maxReconnectDelay {
		delay = maxReconnectDelay
	}
	// Add jitter
	jitter := time.Duration(float64(delay) * reconnectJitter * (rand.Float64()*2 - 1))
	return delay + jitter
}

// licenseError is returned when the relay server rejects us due to license issues.
type licenseError struct {
	code    string
	message string
}

func (e *licenseError) Error() string {
	return fmt.Sprintf("license error (%s): %s", e.code, e.message)
}

func isLicenseError(err error) bool {
	_, ok := err.(*licenseError)
	return ok
}

// marshalStatus returns the client status as JSON bytes.
func (c *Client) marshalStatus() ([]byte, error) {
	return json.Marshal(c.Status())
}
