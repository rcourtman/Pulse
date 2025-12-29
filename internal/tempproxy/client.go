package tempproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"time"
)

const (
	defaultSocketPath   = "/run/pulse-sensor-proxy/pulse-sensor-proxy.sock"
	containerSocketPath = "/mnt/pulse-proxy/pulse-sensor-proxy.sock"
	defaultTimeout      = 30 * time.Second // Increased to accommodate SSH operations

	// Exponential backoff constants
	initialBackoff = 100 * time.Millisecond
	maxBackoff     = 10 * time.Second
	backoffFactor  = 2.0
	jitterFraction = 0.1
	maxRetries     = 3
)

var statFn = os.Stat

var dialContextFn = func(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
	dialer := net.Dialer{Timeout: timeout}
	return dialer.DialContext(ctx, network, address)
}

// ErrorType classifies proxy errors for better error handling
type ErrorType int

const (
	ErrorTypeUnknown   ErrorType = iota
	ErrorTypeTransport           // Socket connection/communication failures
	ErrorTypeAuth                // Authorization failures
	ErrorTypeSSH                 // SSH connectivity issues
	ErrorTypeSensor              // Sensor command failures
	ErrorTypeTimeout             // Operation timeout
	ErrorTypeNode                // Node allowlist or validation failures
)

// ProxyError wraps errors with classification
type ProxyError struct {
	Type      ErrorType
	Message   string
	Retryable bool
	Wrapped   error
}

func (e *ProxyError) Error() string {
	if e.Wrapped != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Wrapped)
	}
	return e.Message
}

func (e *ProxyError) Unwrap() error {
	return e.Wrapped
}

// Client communicates with pulse-sensor-proxy via unix socket
type Client struct {
	socketPath string
	timeout    time.Duration
}

// NewClient creates a new proxy client
func NewClient() *Client {
	socketPath := os.Getenv("PULSE_SENSOR_PROXY_SOCKET")
	if socketPath == "" {
		if _, err := statFn(defaultSocketPath); err == nil {
			socketPath = defaultSocketPath
		} else if _, err := statFn(containerSocketPath); err == nil {
			socketPath = containerSocketPath
		} else {
			socketPath = defaultSocketPath
		}
	}

	return &Client{
		socketPath: socketPath,
		timeout:    defaultTimeout,
	}
}

// IsAvailable checks if the proxy is running and accessible
func (c *Client) IsAvailable() bool {
	_, err := statFn(c.socketPath)
	return err == nil
}

// RPCRequest represents a request to the proxy
type RPCRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// RPCResponse represents a response from the proxy
type RPCResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// calculateBackoff calculates exponential backoff with jitter
func calculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return initialBackoff
	}

	// Calculate base backoff: initialBackoff * (backoffFactor ^ attempt)
	backoff := float64(initialBackoff) * math.Pow(backoffFactor, float64(attempt))

	// Cap at maxBackoff
	if backoff > float64(maxBackoff) {
		backoff = float64(maxBackoff)
	}

	// Add jitter: ±10% randomization
	jitter := backoff * jitterFraction * (rand.Float64()*2 - 1)
	backoff += jitter

	return time.Duration(backoff)
}

// classifyError categorizes errors for retry logic
func classifyError(err error, respError string) *ProxyError {
	// Check response error messages first (even if err is nil)
	// This handles cases where the socket succeeds but the proxy returns an application error
	if respError != "" {
		// Node validator/allowlist rejections should not disable the proxy globally
		if contains(respError, "rejected by validator", "not in allowlist", "node \"") {
			return &ProxyError{
				Type:      ErrorTypeNode,
				Message:   respError,
				Retryable: false,
				Wrapped:   fmt.Errorf("%s", respError),
			}
		}

		// Rate limiting - never retry
		if contains(respError, "rate limit") {
			return &ProxyError{
				Type:      ErrorTypeTransport,
				Message:   respError,
				Retryable: false,
				Wrapped:   fmt.Errorf("%s", respError),
			}
		}

		// Authorization errors - never retry
		if respError == "unauthorized" || respError == "method requires host-level privileges" || respError == "method requires admin capability" || contains(respError, "admin capability") {
			return &ProxyError{
				Type:      ErrorTypeAuth,
				Message:   respError,
				Retryable: false,
				Wrapped:   fmt.Errorf("%s", respError),
			}
		}

		// SSH-related errors - retryable
		if contains(respError, "ssh", "connection", "timeout") {
			return &ProxyError{
				Type:      ErrorTypeSSH,
				Message:   "SSH connectivity issue",
				Retryable: true,
				Wrapped:   fmt.Errorf("%s", respError),
			}
		}

		// Sensor errors - never retry
		if contains(respError, "sensor", "temperature") {
			return &ProxyError{
				Type:      ErrorTypeSensor,
				Message:   "sensor command failed",
				Retryable: false,
				Wrapped:   fmt.Errorf("%s", respError),
			}
		}
	}

	// If no response error and no network error, nothing to classify
	if err == nil {
		return nil
	}

	// Check for timeout
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return &ProxyError{
			Type:      ErrorTypeTimeout,
			Message:   "operation timed out",
			Retryable: true,
			Wrapped:   err,
		}
	}

	// Check for connection errors (socket unavailable)
	if _, ok := err.(*net.OpError); ok {
		return &ProxyError{
			Type:      ErrorTypeTransport,
			Message:   "failed to connect to proxy socket",
			Retryable: true,
			Wrapped:   err,
		}
	}

	// Unknown error
	return &ProxyError{
		Type:      ErrorTypeUnknown,
		Message:   "unknown proxy error",
		Retryable: false,
		Wrapped:   err,
	}
}

// contains checks if any of the substrings are in the main string (case-insensitive)
func contains(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				match := true
				for j := 0; j < len(substr); j++ {
					c1 := s[i+j]
					c2 := substr[j]
					if c1 >= 'A' && c1 <= 'Z' {
						c1 += 32
					}
					if c2 >= 'A' && c2 <= 'Z' {
						c2 += 32
					}
					if c1 != c2 {
						match = false
						break
					}
				}
				if match {
					return true
				}
			}
		}
	}
	return false
}

// callWithContext sends an RPC request with context and retry support
func (c *Client) callWithContext(ctx context.Context, method string, params map[string]interface{}) (*RPCResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check if context is already cancelled
		select {
		case <-ctx.Done():
			return nil, &ProxyError{
				Type:      ErrorTypeTimeout,
				Message:   "context cancelled before retry",
				Retryable: false,
				Wrapped:   ctx.Err(),
			}
		default:
		}

		// Try the call
		resp, err := c.callOnce(ctx, method, params)

		// Success
		if err == nil && resp != nil && resp.Success {
			return resp, nil
		}

		// Classify error
		respError := ""
		if resp != nil {
			respError = resp.Error
		}
		proxyErr := classifyError(err, respError)

		// Don't retry non-retryable errors
		if proxyErr != nil && !proxyErr.Retryable {
			return resp, proxyErr
		}

		if proxyErr != nil {
			lastErr = proxyErr
		} else {
			lastErr = err
		}

		// Don't sleep after last attempt
		if attempt < maxRetries {
			backoff := calculateBackoff(attempt)

			select {
			case <-time.After(backoff):
				// Continue to next attempt
			case <-ctx.Done():
				return nil, &ProxyError{
					Type:      ErrorTypeTimeout,
					Message:   "context cancelled during backoff",
					Retryable: false,
					Wrapped:   ctx.Err(),
				}
			}
		}
	}

	// All retries exhausted
	return nil, &ProxyError{
		Type:      ErrorTypeTransport,
		Message:   fmt.Sprintf("max retries (%d) exhausted", maxRetries),
		Retryable: false,
		Wrapped:   lastErr,
	}
}

// callOnce sends a single RPC request without retries
func (c *Client) callOnce(ctx context.Context, method string, params map[string]interface{}) (*RPCResponse, error) {
	// Connect to unix socket with context
	conn, err := dialContextFn(ctx, "unix", c.socketPath, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}
	defer conn.Close()

	// Set deadline from context or use default timeout
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.timeout)
	}
	conn.SetDeadline(deadline)

	// Send request
	req := RPCRequest{
		Method: method,
		Params: params,
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	// Read response (server uses newline-delimited framing)
	var resp RPCResponse
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &resp, nil
}

// call sends an RPC request and returns the response (legacy method, uses default context)
func (c *Client) call(method string, params map[string]interface{}) (*RPCResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	return c.callWithContext(ctx, method, params)
}

// GetStatus returns proxy status
func (c *Client) GetStatus() (map[string]interface{}, error) {
	resp, err := c.call("get_status", nil)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// RegisterNodes returns list of discovered nodes with SSH status
func (c *Client) RegisterNodes() ([]map[string]interface{}, error) {
	resp, err := c.call("register_nodes", nil)
	if err != nil {
		return nil, err
	}

	// Extract nodes array from data
	nodesRaw, ok := resp.Data["nodes"]
	if !ok {
		return nil, fmt.Errorf("no nodes in response")
	}

	// Type assertion to []interface{} first, then convert
	nodesArray, ok := nodesRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("nodes is not an array")
	}

	nodes := make([]map[string]interface{}, len(nodesArray))
	for i, nodeRaw := range nodesArray {
		node, ok := nodeRaw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("node %d is not a map", i)
		}
		nodes[i] = node
	}

	return nodes, nil
}

// GetTemperature fetches temperature data from a specific node
func (c *Client) GetTemperature(nodeHost string) (string, error) {
	params := map[string]interface{}{
		"node": nodeHost,
	}

	resp, err := c.call("get_temperature", params)
	if err != nil {
		return "", err
	}

	// Extract temperature JSON string
	tempRaw, ok := resp.Data["temperature"]
	if !ok {
		return "", fmt.Errorf("no temperature data in response")
	}

	tempStr, ok := tempRaw.(string)
	if !ok {
		return "", fmt.Errorf("temperature is not a string")
	}

	return tempStr, nil
}

// RequestCleanup signals the proxy to trigger host-side cleanup workflow.
func (c *Client) RequestCleanup(host string) error {
	params := make(map[string]interface{}, 1)
	if host != "" {
		params["host"] = host
	}

	if _, err := c.call("request_cleanup", params); err != nil {
		return err
	}

	return nil
}
