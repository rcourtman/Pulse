package relay

import (
	"context"
	"crypto/ecdh"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// ErrNotConnected is returned when attempting to send on a disconnected client.
var ErrNotConnected = errors.New("relay client not connected")

const (
	// Reconnect backoff parameters
	baseReconnectDelay = 5 * time.Second
	maxReconnectDelay  = 5 * time.Minute
	reconnectJitter    = 0.1

	// WebSocket parameters
	wsPingInterval        = 25 * time.Second
	wsPongWait            = 70 * time.Second
	wsWriteWait           = 10 * time.Second
	wsHandshakeWait       = 15 * time.Second
	sendChBufferSize      = 256
	wsMaxMessageSize      = HeaderSize + MaxPayloadSize
	proxyStreamTimeout    = 15 * time.Minute
	relayOverloadedReason = "relay overloaded, retry request"
)

// maxConcurrentDataHandlers limits active DATA stream handlers per connection.
// This prevents unbounded goroutine growth if the relay floods DATA frames.
var maxConcurrentDataHandlers = 64

// TokenValidator validates an API token and returns the raw token if valid.
type TokenValidator func(token string) bool

// channelState holds per-channel state including auth and encryption.
type channelState struct {
	apiToken   string
	encryption *ChannelEncryption // nil until key exchange completes
	ephemeral  *ecdh.PrivateKey   // ephemeral keypair, cleared after handshake
}

// ClientDeps holds injectable dependencies for the relay client.
type ClientDeps struct {
	LicenseTokenFunc   func() string  // returns the raw license JWT
	TokenValidator     TokenValidator // validates API tokens from CHANNEL_OPEN
	LocalAddr          string         // e.g. "127.0.0.1:7655"
	ServerVersion      string         // Pulse version for ClientVersion in REGISTER
	IdentityPubKey     string         // base64-encoded Ed25519 public key for MITM prevention
	IdentityPrivateKey string         // base64-encoded Ed25519 private key for signing KEY_EXCHANGE
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
	sendCh       chan<- []byte // per-connection send channel (nil when disconnected)
	instanceID   string
	sessionToken string
	channels     map[uint32]*channelState // channelID → channel state
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
		channels: make(map[uint32]*channelState),
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

// SendPushNotification sends a push notification through the relay.
// Returns ErrNotConnected if the client has not completed registration
// with the relay server.
func (c *Client) SendPushNotification(notification PushNotificationPayload) error {
	frame, err := NewControlFrame(FramePushNotification, 0, notification)
	if err != nil {
		return fmt.Errorf("build push frame: %w", err)
	}
	data, err := EncodeFrame(frame)
	if err != nil {
		return fmt.Errorf("encode push frame: %w", err)
	}

	c.mu.RLock()
	ch := c.sendCh
	connected := c.connected
	c.mu.RUnlock()

	if ch == nil || !connected {
		return ErrNotConnected
	}

	select {
	case ch <- data:
		return nil
	default:
		return fmt.Errorf("send channel full")
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
	c.channels = make(map[uint32]*channelState)
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		instanceID := c.instanceID
		activeChannels := len(c.channels)
		c.sendCh = nil
		c.conn = nil
		c.channels = make(map[uint32]*channelState)
		c.connected = false
		c.mu.Unlock()
		conn.Close()
		c.logger.Info().
			Str("instance_id", instanceID).
			Int("active_channels", activeChannels).
			Msg("Relay connection closed")
	}()

	// Register with relay server
	if err := c.register(conn); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	// Enforce connection liveness: each Pong extends the read deadline.
	conn.SetReadLimit(int64(wsMaxMessageSize))
	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	// Expose sendCh only after successful registration so
	// SendPushNotification callers can't enqueue frames during
	// the handshake window before a relay session exists.
	c.mu.Lock()
	c.sendCh = sendCh
	c.connected = true
	c.lastError = ""
	c.mu.Unlock()

	c.logger.Info().Str("instance_id", c.instanceID).Msg("Registered with relay server")

	// Per-connection context: cancelled when this connection ends (for any
	// reason), which tears down the write pump and any in-flight stream
	// goroutines spawned by handleData. Without this, stream goroutines
	// would keep running against a stale sendCh until the whole client stops.
	connCtx, connCancel := context.WithCancel(ctx)
	defer connCancel()
	dataLimiter := make(chan struct{}, maxConcurrentDataHandlers)

	go c.writePump(connCtx, conn, sendCh)

	// Read pump (blocking) — passes connCtx so handleData streams inherit it
	return c.readPump(connCtx, conn, sendCh, dataLimiter)
}

func (c *Client) register(conn *websocket.Conn) error {
	if c.deps.LicenseTokenFunc == nil {
		return fmt.Errorf("license token provider not configured")
	}
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

func (c *Client) readPump(ctx context.Context, conn *websocket.Conn, sendCh chan<- []byte, dataLimiter chan struct{}) error {
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
		_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))

		frame, err := DecodeFrame(msg)
		if err != nil {
			c.logger.Warn().Err(err).Msg("Failed to decode frame, skipping")
			continue
		}

		switch frame.Type {
		case FrameChannelOpen:
			c.handleChannelOpen(frame, sendCh)

		case FrameKeyExchange:
			c.handleKeyExchange(frame, sendCh)

		case FrameData:
			c.handleData(ctx, frame, sendCh, dataLimiter)

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
	// Always close the socket on writer exit so blocked readers unblock and reconnect.
	defer conn.Close()

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

		case data, ok := <-sendCh:
			if !ok {
				return
			}
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

	if c.deps.TokenValidator == nil {
		c.logger.Error().Uint32("channel", payload.ChannelID).Msg("Rejecting channel: token validator not configured")
		closeFrame, err := NewControlFrame(FrameChannelClose, payload.ChannelID, ChannelClosePayload{
			ChannelID: payload.ChannelID,
			Reason:    "token validation unavailable",
		})
		if err == nil {
			queueFrame(sendCh, closeFrame, c.logger)
		}
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
	c.channels[payload.ChannelID] = &channelState{apiToken: payload.AuthToken}
	c.mu.Unlock()

	c.logger.Info().Uint32("channel", payload.ChannelID).Msg("Channel opened")

	// Echo CHANNEL_OPEN to acknowledge
	ackFrame, err := NewControlFrame(FrameChannelOpen, payload.ChannelID, payload)
	if err == nil {
		queueFrame(sendCh, ackFrame, c.logger)
	}
}

func (c *Client) handleData(connCtx context.Context, frame Frame, sendCh chan<- []byte, dataLimiter chan struct{}) {
	channelID := frame.Channel

	// Snapshot channel state under lock so the goroutine below doesn't race
	// with handleKeyExchange writing state.encryption.
	c.mu.RLock()
	state, ok := c.channels[channelID]
	var enc *ChannelEncryption
	var apiToken string
	if ok {
		enc = state.encryption
		apiToken = state.apiToken
	}
	c.mu.RUnlock()

	if !ok {
		c.logger.Warn().Uint32("channel", channelID).Msg("DATA for unknown channel")
		return
	}

	select {
	case dataLimiter <- struct{}{}:
	default:
		c.handleOverloadedData(channelID, frame.Payload, enc, sendCh)
		return
	}

	// Handle in background goroutine so we don't block the read pump
	go func() {
		defer func() { <-dataLimiter }()
		payload := frame.Payload

		// Decrypt incoming if encryption is active
		if enc != nil {
			decrypted, err := enc.Decrypt(payload)
			if err != nil {
				c.logger.Warn().Err(err).Uint32("channel", channelID).Msg("Failed to decrypt DATA payload")
				return
			}
			payload = decrypted
		}

		// Derive from the connection context so streams are cancelled on disconnect.
		// The 15-minute timeout is a safety net for runaway streams.
		ctx, cancel := context.WithTimeout(connCtx, proxyStreamTimeout)
		defer cancel()

		err := c.proxy.HandleStreamRequest(ctx, payload, apiToken, func(respPayload []byte) {
			if enc != nil {
				encrypted, err := enc.Encrypt(respPayload)
				if err != nil {
					c.logger.Warn().Err(err).Uint32("channel", channelID).Msg("Failed to encrypt DATA response")
					return
				}
				respPayload = encrypted
			}
			respFrame := NewFrame(FrameData, channelID, respPayload)
			queueFrame(sendCh, respFrame, c.logger)
		})
		if err != nil && connCtx.Err() == nil {
			c.logger.Warn().Err(err).Uint32("channel", channelID).Msg("Stream proxy error")
		}
	}()
}

func (c *Client) handleOverloadedData(channelID uint32, payload []byte, enc *ChannelEncryption, sendCh chan<- []byte) {
	if enc != nil {
		decrypted, err := enc.Decrypt(payload)
		if err != nil {
			c.logger.Warn().Err(err).Uint32("channel", channelID).Msg("Failed to decrypt overloaded DATA payload")
			return
		}
		payload = decrypted
	}

	requestID := extractProxyRequestID(payload)
	respPayload := c.proxy.errorResponse(requestID, http.StatusServiceUnavailable, relayOverloadedReason)
	if enc != nil {
		encrypted, err := enc.Encrypt(respPayload)
		if err != nil {
			c.logger.Warn().Err(err).Uint32("channel", channelID).Msg("Failed to encrypt overload response")
			return
		}
		respPayload = encrypted
	}

	c.logger.Warn().
		Uint32("channel", channelID).
		Str("request_id", requestID).
		Int("max_in_flight", maxConcurrentDataHandlers).
		Msg("DATA handler limit reached, rejecting request")
	queueFrame(sendCh, NewFrame(FrameData, channelID, respPayload), c.logger)
}

func extractProxyRequestID(payload []byte) string {
	var req ProxyRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return ""
	}
	return req.ID
}

func (c *Client) handleKeyExchange(frame Frame, sendCh chan<- []byte) {
	channelID := frame.Channel

	c.mu.RLock()
	state, ok := c.channels[channelID]
	c.mu.RUnlock()

	if !ok {
		c.logger.Warn().Uint32("channel", channelID).Msg("KEY_EXCHANGE for unknown channel")
		return
	}

	// Unmarshal the app's ephemeral public key
	appPubBytes, _, err := UnmarshalKeyExchangePayload(frame.Payload)
	if err != nil {
		c.logger.Warn().Err(err).Uint32("channel", channelID).Msg("Failed to unmarshal KEY_EXCHANGE")
		return
	}

	appPubKey, err := ecdh.X25519().NewPublicKey(appPubBytes)
	if err != nil {
		c.logger.Warn().Err(err).Uint32("channel", channelID).Msg("Invalid X25519 public key in KEY_EXCHANGE")
		return
	}

	// Generate instance's ephemeral X25519 keypair
	instancePriv, err := GenerateEphemeralKeyPair()
	if err != nil {
		c.logger.Error().Err(err).Uint32("channel", channelID).Msg("Failed to generate ephemeral keypair")
		return
	}

	// Derive channel keys
	encryption, err := DeriveChannelKeys(instancePriv, appPubKey, true)
	if err != nil {
		c.logger.Error().Err(err).Uint32("channel", channelID).Msg("Failed to derive channel keys")
		return
	}

	// Sign instance's ephemeral public key with Ed25519 identity key.
	// Fail closed: refuse key exchange if we can't sign (prevents unsigned/MITM-vulnerable channels).
	if c.deps.IdentityPrivateKey == "" {
		c.logger.Error().Uint32("channel", channelID).Msg("Rejecting KEY_EXCHANGE: identity private key not configured")
		c.closeAndRemoveChannel(channelID, "key exchange signing unavailable", sendCh)
		return
	}

	instancePubBytes := instancePriv.PublicKey().Bytes()
	sig, err := SignKeyExchange(instancePubBytes, c.deps.IdentityPrivateKey)
	if err != nil {
		c.logger.Error().Err(err).Uint32("channel", channelID).Msg("Failed to sign KEY_EXCHANGE")
		c.closeAndRemoveChannel(channelID, "key exchange signing failed", sendCh)
		return
	}

	// Send KEY_EXCHANGE response with instance public key + signature
	respPayload := MarshalKeyExchangePayload(instancePubBytes, sig)
	respFrame := NewFrame(FrameKeyExchange, channelID, respPayload)
	queueFrame(sendCh, respFrame, c.logger)

	// Store encryption state
	c.mu.Lock()
	state.encryption = encryption
	c.mu.Unlock()

	c.logger.Info().Uint32("channel", channelID).Msg("Key exchange completed, channel encrypted")
}

// closeAndRemoveChannel sends CHANNEL_CLOSE to the peer and removes the
// channel locally so no further DATA frames are processed.
func (c *Client) closeAndRemoveChannel(channelID uint32, reason string, sendCh chan<- []byte) {
	c.mu.Lock()
	delete(c.channels, channelID)
	c.mu.Unlock()

	closeFrame, err := NewControlFrame(FrameChannelClose, channelID, ChannelClosePayload{
		ChannelID: channelID,
		Reason:    reason,
	})
	if err == nil {
		queueFrame(sendCh, closeFrame, c.logger)
	}
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
