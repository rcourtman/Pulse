package hostagent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// safeTargetIDPattern validates target IDs to prevent shell injection.
// Allows alphanumeric, dash, underscore, period (no colons or special chars).
var safeTargetIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

var execCommandContext = exec.CommandContext

// How long the command client waits before retrying after a connection failure.
// Package var so tests can override to avoid long sleeps.
var reconnectDelay = 10 * time.Second

// CommandClient handles WebSocket connection to Pulse for AI command execution
type CommandClient struct {
	pulseURL           string
	apiToken           string
	agentID            string
	hostname           string
	platform           string
	version            string
	insecureSkipVerify bool
	logger             zerolog.Logger

	conn   *websocket.Conn
	connMu sync.Mutex
	done   chan struct{}
}

// NewCommandClient creates a new command execution client
func NewCommandClient(cfg Config, agentID, hostname, platform, version string) *CommandClient {
	logger := cfg.Logger.With().Str("component", "command-client").Logger()

	return &CommandClient{
		pulseURL:           strings.TrimRight(cfg.PulseURL, "/"),
		apiToken:           cfg.APIToken,
		agentID:            agentID,
		hostname:           hostname,
		platform:           platform,
		version:            version,
		insecureSkipVerify: cfg.InsecureSkipVerify,
		logger:             logger,
		done:               make(chan struct{}),
	}
}

// Message types matching agentexec package
type messageType string

const (
	msgTypeAgentRegister messageType = "agent_register"
	msgTypeAgentPing     messageType = "agent_ping"
	msgTypeCommandResult messageType = "command_result"
	msgTypeRegistered    messageType = "registered"
	msgTypePong          messageType = "pong"
	msgTypeExecuteCmd    messageType = "execute_command"
	msgTypeReadFile      messageType = "read_file"
)

type wsMessage struct {
	Type      messageType     `json:"type"`
	ID        string          `json:"id,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type registerPayload struct {
	AgentID  string   `json:"agent_id"`
	Hostname string   `json:"hostname"`
	Version  string   `json:"version"`
	Platform string   `json:"platform"`
	Tags     []string `json:"tags,omitempty"`
	Token    string   `json:"token"`
}

type registeredPayload struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type executeCommandPayload struct {
	RequestID  string `json:"request_id"`
	Command    string `json:"command"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id,omitempty"`
	Timeout    int    `json:"timeout,omitempty"`
}

type commandResultPayload struct {
	RequestID string `json:"request_id"`
	Success   bool   `json:"success"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
	ExitCode  int    `json:"exit_code"`
	Error     string `json:"error,omitempty"`
	Duration  int64  `json:"duration_ms"`
}

// Run starts the command client and maintains the WebSocket connection
func (c *CommandClient) Run(ctx context.Context) error {
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

			// Distinguish between transient issues and persistent failures
			// Normal close errors (server restart, reconnection) are expected and logged at debug
			errStr := err.Error()
			isNormalClose := strings.Contains(errStr, "close 1000") ||
				strings.Contains(errStr, "close 1001") ||
				strings.Contains(errStr, "use of closed network connection") ||
				strings.Contains(errStr, "connection reset by peer")

			if isNormalClose {
				c.logger.Debug().Err(err).Msg("WebSocket closed, reconnecting in 10s")
			} else if consecutiveFailures >= 3 {
				c.logger.Warn().Err(err).Int("failures", consecutiveFailures).Msg("WebSocket connection failed repeatedly, reconnecting in 10s")
			} else {
				c.logger.Debug().Err(err).Msg("WebSocket connection interrupted, reconnecting in 10s")
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(reconnectDelay):
			}
		} else {
			// Connection closed cleanly (shouldn't happen in normal operation)
			consecutiveFailures = 0
		}
	}
}

func (c *CommandClient) connectAndHandle(ctx context.Context) error {
	// Build WebSocket URL
	wsURL, err := c.buildWebSocketURL()
	if err != nil {
		return fmt.Errorf("build websocket url: %w", err)
	}

	c.logger.Debug().Str("url", wsURL).Msg("Connecting to Pulse command server")

	// Create dialer with TLS config
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: c.insecureSkipVerify,
		},
		HandshakeTimeout: 45 * time.Second,
	}

	// Connect
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial websocket: %w", err)
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	defer func() {
		c.connMu.Lock()
		c.conn = nil
		c.connMu.Unlock()
		conn.Close()
	}()

	// Send registration
	if err := c.sendRegistration(conn); err != nil {
		return fmt.Errorf("send registration: %w", err)
	}

	// Wait for registration response
	if err := c.waitForRegistration(conn); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	c.logger.Info().Msg("Connected and registered with Pulse command server")

	// Clear any deadlines that may have been set during handshake
	// The HandshakeTimeout in the Dialer may have set a deadline on the underlying connection
	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Time{})
	if netConn := conn.NetConn(); netConn != nil {
		netConn.SetReadDeadline(time.Time{})
		netConn.SetWriteDeadline(time.Time{})
	}

	// Start ping loop
	pingDone := make(chan struct{})
	go c.pingLoop(ctx, conn, pingDone)
	defer close(pingDone)

	// Handle incoming messages
	return c.handleMessages(ctx, conn)
}

func (c *CommandClient) buildWebSocketURL() (string, error) {
	parsed, err := url.Parse(c.pulseURL)
	if err != nil {
		return "", fmt.Errorf("pulse URL is invalid: %w", err)
	}

	if parsed.Scheme == "" {
		return "", fmt.Errorf("pulse URL %q must include scheme", c.pulseURL)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("pulse URL %q must include host", c.pulseURL)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("pulse URL %q must not include user credentials", c.pulseURL)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("pulse URL %q must not include query or fragment", c.pulseURL)
	}

	switch strings.ToLower(parsed.Scheme) {
	case "https":
		parsed.Scheme = "wss"
	case "http":
		if !isLoopbackPulseHost(parsed.Hostname()) {
			return "", fmt.Errorf("pulse URL %q must use https/wss unless host is loopback", c.pulseURL)
		}
		parsed.Scheme = "ws"
	case "wss":
		parsed.Scheme = "wss"
	case "ws":
		if !isLoopbackPulseHost(parsed.Hostname()) {
			return "", fmt.Errorf("pulse URL %q must use https/wss unless host is loopback", c.pulseURL)
		}
		parsed.Scheme = "ws"
	default:
		return "", fmt.Errorf("pulse URL %q has unsupported scheme %q", c.pulseURL, parsed.Scheme)
	}

	parsed.Path = "/api/agent/ws"
	parsed.RawPath = ""
	return parsed.String(), nil
}

func (c *CommandClient) sendRegistration(conn *websocket.Conn) error {
	payload, _ := json.Marshal(registerPayload{
		AgentID:  c.agentID,
		Hostname: c.hostname,
		Version:  c.version,
		Platform: c.platform,
		Token:    c.apiToken,
	})

	msg := wsMessage{
		Type:      msgTypeAgentRegister,
		Timestamp: time.Now(),
		Payload:   payload,
	}

	return conn.WriteJSON(msg)
}

func (c *CommandClient) waitForRegistration(conn *websocket.Conn) error {
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	defer conn.SetReadDeadline(time.Time{})

	var msg wsMessage
	if err := conn.ReadJSON(&msg); err != nil {
		return fmt.Errorf("read registration response: %w", err)
	}

	if msg.Type != msgTypeRegistered {
		return fmt.Errorf("unexpected message type: %s", msg.Type)
	}

	var payload registeredPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("parse registration response: %w", err)
	}

	if !payload.Success {
		return fmt.Errorf("registration rejected: %s", payload.Message)
	}

	return nil
}

func (c *CommandClient) pingLoop(ctx context.Context, conn *websocket.Conn, done chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			c.connMu.Lock()
			if c.conn != nil {
				msg := wsMessage{
					Type:      msgTypeAgentPing,
					Timestamp: time.Now(),
				}
				if err := conn.WriteJSON(msg); err != nil {
					c.logger.Debug().Err(err).Msg("Failed to send ping")
				}
			}
			c.connMu.Unlock()
		}
	}
}

func (c *CommandClient) handleMessages(ctx context.Context, conn *websocket.Conn) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var msg wsMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return fmt.Errorf("read message: %w", err)
		}

		switch msg.Type {
		case msgTypePong:
			// Ignore pong responses

		case msgTypeExecuteCmd:
			var payload executeCommandPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				c.logger.Error().Err(err).Msg("Failed to parse execute_command payload")
				continue
			}

			// Execute command in background
			go c.handleExecuteCommand(ctx, conn, payload)

		case msgTypeReadFile:
			// Handle read_file similarly (uses cat command internally)
			var payload executeCommandPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				c.logger.Error().Err(err).Msg("Failed to parse read_file payload")
				continue
			}
			go c.handleExecuteCommand(ctx, conn, payload)

		default:
			c.logger.Debug().Str("type", string(msg.Type)).Msg("Unknown message type")
		}
	}
}

func (c *CommandClient) handleExecuteCommand(ctx context.Context, conn *websocket.Conn, payload executeCommandPayload) {
	startTime := time.Now()

	c.logger.Info().
		Str("request_id", payload.RequestID).
		Str("command", payload.Command).
		Str("target_type", payload.TargetType).
		Str("target_id", payload.TargetID).
		Msg("Executing command")

	result := c.executeCommand(ctx, payload)
	result.Duration = time.Since(startTime).Milliseconds()

	// Send result back
	resultPayload, _ := json.Marshal(result)
	msg := wsMessage{
		Type:      msgTypeCommandResult,
		ID:        payload.RequestID,
		Timestamp: time.Now(),
		Payload:   resultPayload,
	}

	c.connMu.Lock()
	err := conn.WriteJSON(msg)
	c.connMu.Unlock()

	if err != nil {
		c.logger.Error().Err(err).Str("request_id", payload.RequestID).Msg("Failed to send command result")
	} else {
		c.logger.Info().
			Str("request_id", payload.RequestID).
			Bool("success", result.Success).
			Int("exit_code", result.ExitCode).
			Int64("duration_ms", result.Duration).
			Msg("Command completed")
	}
}

func wrapCommand(payload executeCommandPayload) string {
	// Only validate TargetID when it will be interpolated into the command
	// (container and vm types). Host type doesn't use TargetID in the command.
	needsTargetID := (payload.TargetType == "container" || payload.TargetType == "vm") && payload.TargetID != ""

	if needsTargetID {
		// Validate TargetID to prevent shell injection - defense in depth
		if !safeTargetIDPattern.MatchString(payload.TargetID) {
			// Return a command that fails with non-zero exit and error message
			return "sh -c 'echo \"Error: invalid target ID\" >&2; exit 1'"
		}

		// Wrap command in sh -c so shell metacharacters (pipes, redirects, globs)
		// are processed inside the container/VM, not on the Proxmox host.
		// Without this, "pct exec 141 -- grep pattern /var/log/*.log" would
		// expand the glob on the host (where /var/log/*.log doesn't exist).
		quotedCmd := shellQuote(payload.Command)

		if payload.TargetType == "container" {
			return fmt.Sprintf("pct exec %s -- sh -c %s", payload.TargetID, quotedCmd)
		}
		if payload.TargetType == "vm" {
			return fmt.Sprintf("qm guest exec %s -- sh -c %s", payload.TargetID, quotedCmd)
		}
	}

	return payload.Command
}

// shellQuote safely quotes a string for use as a shell argument.
// Uses single quotes and escapes any embedded single quotes.
func shellQuote(s string) string {
	escaped := strings.ReplaceAll(s, "'", "'\"'\"'")
	return "'" + escaped + "'"
}

func (c *CommandClient) executeCommand(ctx context.Context, payload executeCommandPayload) commandResultPayload {
	result := commandResultPayload{
		RequestID: payload.RequestID,
	}

	// Determine timeout
	timeout := time.Duration(payload.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	command := wrapCommand(payload)

	// Execute the command
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = execCommandContext(cmdCtx, "cmd", "/C", command)
	} else {
		cmd = execCommandContext(cmdCtx, "sh", "-c", command)
		// Ensure PATH includes common binary locations for docker, kubectl, etc.
		cmd.Env = append(os.Environ(), "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:"+os.Getenv("PATH"))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			result.Error = "command timed out"
			result.ExitCode = -1
			result.Success = false
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Success = false
		} else {
			result.Error = err.Error()
			result.ExitCode = -1
			result.Success = false
		}
	} else {
		result.ExitCode = 0
		result.Success = true
	}

	// Truncate output if too large (1MB limit)
	const maxOutputSize = 1024 * 1024
	if len(result.Stdout) > maxOutputSize {
		result.Stdout = result.Stdout[:maxOutputSize] + "\n... (output truncated)"
	}
	if len(result.Stderr) > maxOutputSize {
		result.Stderr = result.Stderr[:maxOutputSize] + "\n... (output truncated)"
	}

	return result
}

// Close closes the command client connection
func (c *CommandClient) Close() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
