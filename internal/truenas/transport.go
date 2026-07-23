package truenas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var errRPCStreamSessionConsumed = errors.New("truenas rpc stream session cannot be reused")

type discardRPCSessionError struct {
	err error
}

func (e *discardRPCSessionError) Error() string {
	return fmt.Sprintf("discard truenas rpc session: %v", e.err)
}

func (e *discardRPCSessionError) Unwrap() error { return e.err }

func discardRPCSessionForStreamError(err error) error {
	if err == nil {
		return nil
	}
	var transportErr *RPCTransportError
	if errors.As(err, &transportErr) {
		return err
	}
	return &discardRPCSessionError{err: err}
}

// TransportMode is the connection-local API transport selected for an
// appliance. A client never changes modes after successful negotiation.
type TransportMode string

const (
	TransportUnknown    TransportMode = "negotiating"
	TransportJSONRPC    TransportMode = "jsonrpc-websocket"
	TransportLegacyREST TransportMode = "legacy-rest"
)

// TransportStatus is a non-secret diagnostic projection of transport and
// authentication state for one appliance.
type TransportStatus struct {
	Mode             TransportMode `json:"mode"`
	Endpoint         string        `json:"endpoint,omitempty"`
	TLS              bool          `json:"tls"`
	Connected        bool          `json:"connected"`
	AuthMechanism    string        `json:"authMechanism,omitempty"`
	ApplianceVersion string        `json:"applianceVersion,omitempty"`
	LegacyReason     string        `json:"legacyReason,omitempty"`
	Reconnects       int           `json:"reconnects,omitempty"`
	LastError        string        `json:"lastError,omitempty"`
	LastConnectedAt  *time.Time    `json:"lastConnectedAt,omitempty"`
}

// RPCError is a structured JSON-RPC method error. Data is intentionally
// reduced to non-secret middleware diagnostics.
type RPCError struct {
	Code    int
	Method  string
	Message string
	Reason  string
	Errname string
}

func (e *RPCError) Error() string {
	if e == nil {
		return "truenas rpc error"
	}
	detail := strings.TrimSpace(e.Reason)
	if detail == "" {
		detail = strings.TrimSpace(e.Message)
	}
	if e.Errname != "" {
		return fmt.Sprintf("truenas rpc %s failed: code=%d errname=%s message=%q", e.Method, e.Code, e.Errname, detail)
	}
	return fmt.Sprintf("truenas rpc %s failed: code=%d message=%q", e.Method, e.Code, detail)
}

// RPCTransportError distinguishes a failed websocket exchange from an
// authoritative middleware method error.
type RPCTransportError struct {
	Method string
	Phase  string
	Err    error
}

func (e *RPCTransportError) Error() string {
	return fmt.Sprintf("%s truenas rpc %s %s: %v", e.Phase, e.Method, transportPhaseNoun(e.Phase), e.Err)
}

func (e *RPCTransportError) Unwrap() error { return e.Err }

func transportPhaseNoun(phase string) string {
	if phase == "write" {
		return "request"
	}
	return "response"
}

type RPCHandshakeError struct {
	StatusCode int
	Err        error
}

func (e *RPCHandshakeError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("dial truenas rpc websocket failed: status=%d: %v", e.StatusCode, e.Err)
	}
	return fmt.Sprintf("dial truenas rpc websocket failed: %v", e.Err)
}

func (e *RPCHandshakeError) Unwrap() error { return e.Err }

// RPCAuthError is an authoritative authentication refusal returned after the
// websocket handshake. It is distinct from a retryable transport failure and
// intentionally carries no credential material.
type RPCAuthError struct {
	Mechanism    string
	ResponseType string
}

func (e *RPCAuthError) Error() string {
	if e == nil {
		return "truenas rpc authentication failed"
	}
	if strings.TrimSpace(e.ResponseType) == "" {
		return fmt.Sprintf("truenas rpc %s authentication failed", e.Mechanism)
	}
	return fmt.Sprintf("truenas rpc %s authentication failed: response_type=%q", e.Mechanism, e.ResponseType)
}

// TransportStatus returns a secret-free snapshot. It never waits for an
// in-flight API call.
func (c *Client) TransportStatus() TransportStatus {
	if c == nil {
		return TransportStatus{Mode: TransportUnknown}
	}
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	status := c.status
	if status.LastConnectedAt != nil {
		value := *status.LastConnectedAt
		status.LastConnectedAt = &value
	}
	return status
}

func (c *Client) updateTransportStatus(update func(*TransportStatus)) {
	if c == nil {
		return
	}
	c.statusMu.Lock()
	update(&c.status)
	c.statusMu.Unlock()
}

func (c *Client) ensureTransport(ctx context.Context) (TransportMode, error) {
	if c == nil {
		return TransportUnknown, fmt.Errorf("truenas client is nil")
	}
	c.rpcMu.Lock()
	defer c.rpcMu.Unlock()
	return c.ensureTransportLocked(ctx)
}

func (c *Client) useLegacyREST(ctx context.Context) (bool, error) {
	mode, err := c.ensureTransport(ctx)
	return mode == TransportLegacyREST, err
}

func (c *Client) ensureTransportLocked(ctx context.Context) (TransportMode, error) {
	if c.closed {
		return TransportUnknown, fmt.Errorf("truenas client is closed")
	}
	if c.mode == TransportLegacyREST {
		return c.mode, nil
	}
	if c.mode == TransportJSONRPC && c.rpc != nil {
		return c.mode, nil
	}

	reconnecting := c.mode == TransportJSONRPC
	rpc, authMechanism, err := c.openAuthenticatedRPC(ctx)
	if err == nil {
		now := time.Now().UTC()
		c.rpc = rpc
		c.mode = TransportJSONRPC
		c.updateTransportStatus(func(status *TransportStatus) {
			status.Mode = TransportJSONRPC
			status.Endpoint = c.rpcURL
			status.Connected = true
			status.AuthMechanism = authMechanism
			status.LegacyReason = ""
			status.LastError = ""
			status.LastConnectedAt = &now
			if reconnecting {
				status.Reconnects++
			}
		})
		return c.mode, nil
	}

	if reconnecting {
		lockedErr := fmt.Errorf("truenas JSON-RPC reconnect failed; transport is immutable for this client and REST downgrade is disabled: %w", err)
		c.recordTransportError(lockedErr)
		return TransportJSONRPC, lockedErr
	}

	if !isUnsupportedWebSocketEndpoint(err) {
		c.recordTransportError(err)
		return TransportUnknown, err
	}

	var response systemInfoResponse
	if restErr := c.getJSON(ctx, http.MethodGet, "/system/info", &response); restErr != nil {
		joined := fmt.Errorf("truenas JSON-RPC endpoint is unavailable and legacy version probe failed: websocket=%w rest=%v", err, restErr)
		c.recordTransportError(joined)
		return TransportUnknown, joined
	}
	if !supportsLegacyREST(response.Version) {
		version := strings.TrimSpace(response.Version)
		if version == "" {
			version = "unknown"
		}
		rejected := fmt.Errorf("truenas %s requires JSON-RPC WebSocket at /api/current; refusing REST downgrade after websocket endpoint failure", version)
		c.recordTransportError(rejected)
		return TransportUnknown, rejected
	}

	c.mode = TransportLegacyREST
	version := strings.TrimSpace(response.Version)
	c.updateTransportStatus(func(status *TransportStatus) {
		status.Mode = TransportLegacyREST
		status.Endpoint = c.baseURL
		status.Connected = true
		status.AuthMechanism = legacyRESTAuthMechanism(c.config)
		status.ApplianceVersion = version
		status.LegacyReason = "JSON-RPC /api/current is unavailable on this recognized legacy TrueNAS release"
		status.LastError = ""
	})
	return c.mode, nil
}

func (c *Client) openAuthenticatedRPC(ctx context.Context) (*trueNASRPCClient, string, error) {
	conn, err := c.dialRPC(ctx)
	if err != nil {
		return nil, "", err
	}
	if !c.config.UseHTTPS && !c.allowInsecureRPC {
		_ = conn.Close()
		return nil, "", fmt.Errorf("truenas JSON-RPC credentials require TLS; configure an https endpoint (certificate verification can be pinned or explicitly disabled)")
	}
	rpc := &trueNASRPCClient{conn: conn, nextID: 1}
	authMechanism, err := rpc.authenticate(ctx, c.config)
	if err != nil {
		_ = conn.Close()
		return nil, "", err
	}
	return rpc, authMechanism, nil
}

func (c *Client) callRPC(ctx context.Context, method string, params any, result any) error {
	return c.callRPCWithRetry(ctx, method, params, result, true)
}

func (c *Client) withRPC(ctx context.Context, operation func(*trueNASRPCClient) error) error {
	if c == nil {
		return fmt.Errorf("truenas client is nil")
	}
	c.rpcMu.Lock()
	defer c.rpcMu.Unlock()

	mode, err := c.ensureTransportLocked(ctx)
	if err != nil {
		return err
	}
	if mode != TransportJSONRPC || c.rpc == nil {
		return fmt.Errorf("truenas JSON-RPC operation is unavailable over negotiated transport %s", mode)
	}
	if err := operation(c.rpc); err != nil {
		if errors.Is(err, errRPCStreamSessionConsumed) {
			c.closeRPCLocked()
			c.reconnect = 0
			return nil
		}
		var discardErr *discardRPCSessionError
		if errors.As(err, &discardErr) {
			c.closeRPCLocked()
			c.recordTransportError(err)
			return err
		}
		var transportErr *RPCTransportError
		if !errors.As(err, &transportErr) {
			c.recordRPCOperationError(err)
			return err
		}
		c.closeRPCLocked()
		c.recordTransportError(err)
		if err := c.waitReconnectBackoff(ctx); err != nil {
			return err
		}
		rpc, authMechanism, reconnectErr := c.openAuthenticatedRPC(ctx)
		if reconnectErr != nil {
			c.recordTransportError(reconnectErr)
			return reconnectErr
		}
		c.rpc = rpc
		c.reconnect++
		now := time.Now().UTC()
		c.updateTransportStatus(func(status *TransportStatus) {
			status.Connected = true
			status.AuthMechanism = authMechanism
			status.Reconnects++
			status.LastConnectedAt = &now
			status.LastError = ""
		})
		if err := operation(c.rpc); err != nil {
			var secondTransportErr *RPCTransportError
			if errors.As(err, &secondTransportErr) {
				c.closeRPCLocked()
				c.recordTransportError(err)
			} else {
				c.recordRPCOperationError(err)
			}
			return err
		}
	}
	c.reconnect = 0
	return nil
}

func (c *Client) callRPCAction(ctx context.Context, method string, params any, result any) error {
	return c.callRPCWithRetry(ctx, method, params, result, false)
}

func (c *Client) callRPCWithRetry(ctx context.Context, method string, params any, result any, retryRead bool) error {
	if c == nil {
		return fmt.Errorf("truenas client is nil")
	}
	c.rpcMu.Lock()
	defer c.rpcMu.Unlock()

	mode, err := c.ensureTransportLocked(ctx)
	if err != nil {
		return err
	}
	if mode != TransportJSONRPC || c.rpc == nil {
		return fmt.Errorf("truenas rpc %s is unavailable over negotiated transport %s", method, mode)
	}

	err = c.rpc.call(ctx, method, params, result)
	if err == nil {
		c.reconnect = 0
		c.updateTransportStatus(func(status *TransportStatus) {
			status.Connected = true
			status.LastError = ""
		})
		return nil
	}
	var transportErr *RPCTransportError
	if !errors.As(err, &transportErr) {
		c.recordRPCOperationError(err)
		return err
	}

	c.closeRPCLocked()
	c.recordTransportError(err)
	if !retryRead {
		return fmt.Errorf("truenas action %s transport failed after dispatch; outcome is unknown and Pulse will not replay it: %w", method, err)
	}

	if err := c.waitReconnectBackoff(ctx); err != nil {
		return err
	}
	rpc, authMechanism, reconnectErr := c.openAuthenticatedRPC(ctx)
	if reconnectErr != nil {
		c.recordTransportError(reconnectErr)
		return reconnectErr
	}
	c.rpc = rpc
	c.mode = TransportJSONRPC
	c.reconnect++
	now := time.Now().UTC()
	c.updateTransportStatus(func(status *TransportStatus) {
		status.Connected = true
		status.AuthMechanism = authMechanism
		status.Reconnects++
		status.LastConnectedAt = &now
		status.LastError = ""
	})
	if err := c.rpc.call(ctx, method, params, result); err != nil {
		var secondTransportErr *RPCTransportError
		if errors.As(err, &secondTransportErr) {
			c.closeRPCLocked()
			c.recordTransportError(err)
		} else {
			c.recordRPCOperationError(err)
		}
		return err
	}
	c.reconnect = 0
	c.updateTransportStatus(func(status *TransportStatus) {
		status.Connected = true
		status.LastError = ""
	})
	return nil
}

func (c *Client) waitReconnectBackoff(ctx context.Context) error {
	attempt := c.reconnect
	if attempt > 4 {
		attempt = 4
	}
	delay := 100 * time.Millisecond * time.Duration(1<<attempt)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) closeRPCLocked() {
	if c.rpc != nil && c.rpc.conn != nil {
		_ = c.rpc.conn.Close()
	}
	c.rpc = nil
	c.updateTransportStatus(func(status *TransportStatus) {
		status.Connected = false
	})
}

func (c *Client) recordTransportError(err error) {
	message := c.sanitizeTransportError(err)
	c.updateTransportStatus(func(status *TransportStatus) {
		status.Connected = false
		status.LastError = message
	})
}

func (c *Client) recordRPCOperationError(err error) {
	message := c.sanitizeTransportError(err)
	c.updateTransportStatus(func(status *TransportStatus) {
		status.Connected = true
		status.LastError = message
	})
}

func (c *Client) sanitizeTransportError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if c == nil {
		return message
	}
	for _, secret := range []string{c.config.APIKey, c.config.Password} {
		secret = strings.TrimSpace(secret)
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[redacted]")
		}
	}
	return message
}

func isUnsupportedWebSocketEndpoint(err error) bool {
	var handshake *RPCHandshakeError
	if !errors.As(err, &handshake) {
		return false
	}
	switch handshake.StatusCode {
	case http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusGone, http.StatusNotImplemented:
		return true
	default:
		return false
	}
}

func supportsLegacyREST(version string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(version))
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, "SCALE") {
		major, minor, ok := trueNASYearRelease(normalized)
		return ok && (major < 25 || (major == 25 && minor < 4))
	}
	// TrueNAS CORE 13 does not expose the versioned JSON-RPC 2.0 endpoint.
	return strings.Contains(normalized, "CORE") || strings.HasPrefix(normalized, "TRUENAS-13") || strings.HasPrefix(normalized, "FREENAS-")
}

func trueNASYearRelease(version string) (int, int, bool) {
	for _, field := range strings.FieldsFunc(version, func(r rune) bool {
		return r < '0' || r > '9'
	}) {
		if len(field) != 2 && len(field) != 4 {
			continue
		}
		major, err := strconv.Atoi(field)
		if err != nil || major < 20 || major > 99 {
			continue
		}
		rest := version[strings.Index(version, field)+len(field):]
		for _, minorField := range strings.FieldsFunc(rest, func(r rune) bool {
			return r < '0' || r > '9'
		}) {
			minor, err := strconv.Atoi(minorField)
			if err == nil {
				return major, minor, true
			}
		}
	}
	return 0, 0, false
}

func legacyRESTAuthMechanism(config ClientConfig) string {
	if strings.TrimSpace(config.APIKey) != "" {
		return "rest-bearer-api-key"
	}
	return "rest-basic-password"
}

func rpcErrorFromWire(method string, wire *trueNASRPCError) *RPCError {
	rpcErr := &RPCError{Method: method}
	if wire == nil {
		return rpcErr
	}
	rpcErr.Code = wire.Code
	rpcErr.Message = strings.TrimSpace(wire.Message)
	if len(wire.Data) > 0 && string(wire.Data) != "null" {
		var data map[string]any
		if json.Unmarshal(wire.Data, &data) == nil {
			rpcErr.Reason = strings.TrimSpace(readStringAny(data, "reason", "message"))
			rpcErr.Errname = strings.TrimSpace(readStringAny(data, "errname", "name"))
		}
	}
	return rpcErr
}

func isMethodUnavailable(err error) bool {
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		return false
	}
	message := strings.ToLower(rpcErr.Message + " " + rpcErr.Reason)
	return rpcErr.Code == -32601 || strings.Contains(message, "method not found") || strings.Contains(message, "not found")
}
