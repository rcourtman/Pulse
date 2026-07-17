package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/agenttls"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	sshknownhosts "github.com/rcourtman/pulse-go-rewrite/internal/ssh/knownhosts"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog"
)

// safeTargetIDPattern validates Proxmox VM/LXC IDs used by pct/qm exec wrappers.
// These must be numeric to avoid option injection or shell metacharacter abuse.
var safeTargetIDPattern = regexp.MustCompile(`^[0-9]{1,9}$`)

var execCommandContext = exec.CommandContext

// How long the command client waits before retrying after a connection failure.
// Package var so tests can override to avoid long sleeps.
var reconnectDelay = 10 * time.Second

const (
	maxCommandOutputSize = 1024 * 1024
	outputTruncatedMsg   = "\n... (output truncated)"
)

var (
	reconnectMaxDelay    = 5 * time.Minute
	reconnectJitterRatio = 0.1
	reconnectRandFloat64 = rand.Float64

	commandApprovalGrantRejectionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pulse_hostagent_command_approval_grant_rejections_total",
		Help: "Total number of approval-required commands rejected by the host agent due to invalid approval grants.",
	}, []string{"reason"})
)

type cappedBuffer struct {
	maxBytes  int
	buf       bytes.Buffer
	truncated bool
}

func newCappedBuffer(maxBytes int) *cappedBuffer {
	return &cappedBuffer{maxBytes: maxBytes}
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	remaining := b.maxBytes - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}

	if len(p) <= remaining {
		_, _ = b.buf.Write(p)
		return len(p), nil
	}

	_, _ = b.buf.Write(p[:remaining])
	b.truncated = true
	return len(p), nil
}

func (b *cappedBuffer) String() string {
	out := b.buf.String()
	if b.truncated {
		return out + outputTruncatedMsg
	}
	return out
}

// CommandClient handles WebSocket connection to Pulse for AI command execution
type CommandClient struct {
	pulseURL                  string
	apiToken                  string
	agentID                   string
	hostname                  string
	platform                  string
	version                   string
	stateDir                  string
	insecureSkipVerify        bool
	caCertPath                string
	serverFingerprint         string
	deploySSHUser             string
	commandPolicy             *agentexec.CommandPolicy
	packageUpdates            *packageUpdateManager
	storageCleanup            *storageCleanupManager
	dockerLifecycle           dockerLifecycleManager
	dockerUpdater             DockerContainerUpdater
	operationReceipts         *operationreceipt.Store
	operationReceiptErr       error
	operationReceiptCloseOnce sync.Once
	operationReceiptCloseErr  error
	sshKnownHosts             sshknownhosts.Manager
	sshKnownHostsOnce         sync.Once
	sshKnownHostsErr          error
	logger                    zerolog.Logger

	conn   *websocket.Conn
	connMu sync.Mutex
	done   chan struct{}
}

// NewCommandClient creates a new command execution client
func NewCommandClient(cfg Config, agentID, hostname, platform, version string) *CommandClient {
	logger := cfg.Logger.With().Str("component", "command-client").Logger()
	stateDir := strings.TrimSpace(cfg.StateDir)
	if stateDir == "" {
		stateDir = defaultStateDir
	}

	receipts, receiptErr := operationreceipt.Open(filepath.Join(stateDir, "operation-receipts.db"), hostOperationReceiptConfig())
	return &CommandClient{
		pulseURL:            strings.TrimRight(cfg.PulseURL, "/"),
		apiToken:            cfg.APIToken,
		agentID:             agentID,
		hostname:            hostname,
		platform:            platform,
		version:             version,
		stateDir:            stateDir,
		insecureSkipVerify:  cfg.InsecureSkipVerify,
		caCertPath:          cfg.CACertPath,
		serverFingerprint:   cfg.ServerFingerprint,
		deploySSHUser:       cfg.DeploySSHUser,
		commandPolicy:       agentexec.DefaultPolicy(),
		packageUpdates:      cfg.packageUpdates,
		storageCleanup:      cfg.storageCleanup,
		dockerLifecycle:     newLocalDockerLifecycleManager(),
		dockerUpdater:       cfg.DockerContainerUpdater,
		operationReceipts:   receipts,
		operationReceiptErr: receiptErr,
		logger:              logger,
		done:                make(chan struct{}),
	}
}

func (c *CommandClient) stopChan() <-chan struct{} {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.done == nil {
		c.done = make(chan struct{})
	}

	return c.done
}

// Message types matching agentexec package
type messageType string

const (
	msgTypeAgentRegister                  messageType = "agent_register"
	msgTypeAgentPing                      messageType = "agent_ping"
	msgTypeCommandResult                  messageType = "command_result"
	msgTypeRegistered                     messageType = "registered"
	msgTypePong                           messageType = "pong"
	msgTypeExecuteCmd                     messageType = "execute_command"
	msgTypeReadFile                       messageType = "read_file"
	msgTypeHostStorageCleanup             messageType = "host_storage_cleanup"
	msgTypeHostStorageCleanupResult       messageType = "host_storage_cleanup_result"
	msgTypeHostUpdate                     messageType = "host_update"
	msgTypeHostUpdateResult               messageType = "host_update_result"
	msgTypeDockerContainerLifecycle       messageType = "docker_container_lifecycle"
	msgTypeDockerContainerLifecycleResult messageType = "docker_container_lifecycle_result"
	msgTypeDockerContainerUpdate          messageType = "docker_container_update"
	msgTypeDockerContainerUpdateResult    messageType = "docker_container_update_result"
	msgTypeOperationQuery                 messageType = "agent_operation_query"
	msgTypeOperationQueryResult           messageType = "agent_operation_query_result"
	msgTypeDeployPreflight                messageType = "deploy_preflight"
	msgTypeDeployInstall                  messageType = "deploy_install"
	msgTypeDeployCancel                   messageType = "deploy_cancel"
	msgTypeDeployProgress                 messageType = "deploy_progress"
)

type wsMessage struct {
	Type      messageType     `json:"type"`
	ID        string          `json:"id,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type registerPayload struct {
	AgentID                 string   `json:"agent_id"`
	Hostname                string   `json:"hostname"`
	Version                 string   `json:"version"`
	Platform                string   `json:"platform"`
	Tags                    []string `json:"tags,omitempty"`
	Token                   string   `json:"token"`
	OperationReceiptVersion int      `json:"operation_receipt_version,omitempty"`
}

type registeredPayload struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type executeCommandPayload struct {
	RequestID     string                          `json:"request_id"`
	Command       string                          `json:"command"`
	ApprovalID    string                          `json:"approval_id,omitempty"`
	ApprovalGrant *agentexec.CommandApprovalGrant `json:"approval_grant,omitempty"`
	TargetType    string                          `json:"target_type"`
	TargetID      string                          `json:"target_id,omitempty"`
	Timeout       int                             `json:"timeout,omitempty"`
	// Trusted is set by the Pulse server when the command originates from a
	// vetted internal subsystem (e.g. servicediscovery deep scans whose
	// command catalog is hardcoded in Pulse source). When set, the agent's
	// policy approval gate is bypassed since the command did not come from
	// a user-driven path. PolicyBlock is still enforced.
	Trusted bool `json:"trusted,omitempty"`
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
	stopCh := c.stopChan()
	consecutiveFailures := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-stopCh:
			return nil
		default:
		}

		err := c.connectAndHandle(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			consecutiveFailures++
			delay := computeReconnectDelay(consecutiveFailures)

			// Distinguish between transient issues and persistent failures
			// Normal close errors (server restart, reconnection) are expected and logged at debug
			errStr := err.Error()
			isNormalClose := strings.Contains(errStr, "close 1000") ||
				strings.Contains(errStr, "close 1001") ||
				strings.Contains(errStr, "use of closed network connection") ||
				strings.Contains(errStr, "connection reset by peer")

			if isNormalClose {
				c.logger.Debug().Err(err).Dur("retry_in", delay).Msg("WebSocket closed, reconnecting")
			} else if consecutiveFailures >= 3 {
				c.logger.Warn().Err(err).Int("failures", consecutiveFailures).Dur("retry_in", delay).Msg("WebSocket connection failed repeatedly, reconnecting")
			} else {
				c.logger.Debug().Err(err).Dur("retry_in", delay).Msg("WebSocket connection interrupted, reconnecting")
			}

			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return ctx.Err()
			case <-stopCh:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return nil
			case <-timer.C:
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
	origin, err := c.buildWebSocketOrigin()
	if err != nil {
		return fmt.Errorf("build websocket origin: %w", err)
	}

	c.logger.Debug().Str("url", wsURL).Msg("Connecting to Pulse command server")

	// Create dialer with TLS config
	tlsConfig, err := agenttls.NewClientTLSConfig(c.caCertPath, c.insecureSkipVerify, c.serverFingerprint)
	if err != nil {
		return fmt.Errorf("build websocket TLS config: %w", err)
	}

	dialer := websocket.Dialer{
		TLSClientConfig:  tlsConfig,
		HandshakeTimeout: 45 * time.Second,
	}
	headers := http.Header{}
	headers.Set("Origin", origin)

	// Connect
	conn, _, err := dialer.DialContext(ctx, wsURL, headers)
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
		if closeErr := conn.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			c.logger.Debug().Err(closeErr).Msg("Failed to close websocket connection")
		}
	}()

	cancelCloseDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-cancelCloseDone:
		}
	}()
	defer close(cancelCloseDone)

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
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		c.logger.Debug().Err(err).Msg("Failed to clear websocket read deadline")
	}
	if err := conn.SetWriteDeadline(time.Time{}); err != nil {
		c.logger.Debug().Err(err).Msg("Failed to clear websocket write deadline")
	}
	if netConn := conn.NetConn(); netConn != nil {
		if err := netConn.SetReadDeadline(time.Time{}); err != nil {
			c.logger.Debug().Err(err).Msg("Failed to clear network read deadline")
		}
		if err := netConn.SetWriteDeadline(time.Time{}); err != nil {
			c.logger.Debug().Err(err).Msg("Failed to clear network write deadline")
		}
	}

	// Start ping loop
	pingDone := make(chan struct{})
	go c.pingLoop(ctx, conn, pingDone)
	defer close(pingDone)

	// Handle incoming messages
	return c.handleMessages(ctx, conn)
}

func (c *CommandClient) buildWebSocketURL() (string, error) {
	parsed, err := securityutil.NormalizePulseWebSocketBaseURLWithOptions(c.pulseURL, securityutil.PulseURLValidationOptions{
		AllowLocalNetworkHTTP: true,
	})
	if err != nil {
		return "", err
	}

	basePath := strings.TrimRight(parsed.Path, "/")
	if basePath == "" {
		parsed.Path = "/api/agent/ws"
	} else {
		parsed.Path = basePath + "/api/agent/ws"
	}
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func (c *CommandClient) buildWebSocketOrigin() (string, error) {
	return securityutil.HTTPOriginForWebSocketBaseURLWithOptions(c.pulseURL, securityutil.PulseURLValidationOptions{
		AllowLocalNetworkHTTP: true,
	})
}

func (c *CommandClient) sendRegistration(conn *websocket.Conn) error {
	payload, err := json.Marshal(registerPayload{
		AgentID:                 c.agentID,
		Hostname:                c.hostname,
		Version:                 c.version,
		Platform:                c.platform,
		Token:                   c.apiToken,
		OperationReceiptVersion: c.operationReceiptVersion(),
	})
	if err != nil {
		return fmt.Errorf("marshal registration payload: %w", err)
	}

	msg := wsMessage{
		Type:      msgTypeAgentRegister,
		Timestamp: time.Now(),
		Payload:   payload,
	}

	return conn.WriteJSON(msg)
}

func (c *CommandClient) operationReceiptVersion() int {
	if c != nil && c.operationReceiptErr == nil && c.operationReceipts != nil {
		return operationreceipt.ProtocolVersion
	}
	return 0
}

func (c *CommandClient) waitForRegistration(conn *websocket.Conn) error {
	if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return fmt.Errorf("set registration read deadline: %w", err)
	}
	defer func() {
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			c.logger.Debug().Err(err).Msg("Failed to clear registration read deadline")
		}
	}()

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
					c.logger.Debug().Err(err).Msg("Failed to send ping, closing connection for reconnect")
					_ = conn.Close()
					c.connMu.Unlock()
					return
				}
			}
			c.connMu.Unlock()
		}
	}
}

func computeReconnectDelay(failures int) time.Duration {
	return utils.ExponentialBackoff(reconnectDelay, reconnectMaxDelay, failures, reconnectJitterRatio, reconnectRandFloat64)
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

		case msgTypeHostUpdate:
			payload, err := agentexec.DecodeHostUpdatePayload(msg.Payload)
			if err != nil {
				c.logger.Error().Err(err).Msg("Failed to parse host_update payload")
				continue
			}
			go c.handleHostUpdate(ctx, conn, payload)

		case msgTypeHostStorageCleanup:
			payload, err := agentexec.DecodeHostStorageCleanupPayload(msg.Payload)
			if err != nil {
				c.logger.Error().Err(err).Msg("Failed to parse host_storage_cleanup payload")
				continue
			}
			go c.handleHostStorageCleanup(ctx, conn, payload)

		case msgTypeDockerContainerUpdate:
			payload, err := agentexec.DecodeDockerContainerUpdatePayload(msg.Payload)
			if err != nil {
				c.logger.Warn().Err(err).Msg("Dropping invalid docker container update request")
				continue
			}
			go c.handleDockerContainerUpdate(ctx, conn, payload)

		case msgTypeDockerContainerLifecycle:
			payload, err := agentexec.DecodeDockerContainerLifecyclePayload(msg.Payload)
			if err != nil {
				c.logger.Error().Err(err).Msg("Failed to parse docker container lifecycle payload")
				continue
			}
			go c.handleDockerContainerLifecycle(ctx, conn, payload)

		case msgTypeOperationQuery:
			query, err := operationreceipt.DecodeQuery(msg.Payload)
			if err != nil {
				c.logger.Warn().Err(err).Msg("Rejected malformed operation receipt query")
				continue
			}
			go c.handleOperationQuery(conn, msg.ID, query)

		case msgTypeDeployPreflight:
			var payload deployPreflightPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				c.logger.Error().Err(err).Msg("Failed to parse deploy_preflight payload")
				continue
			}
			go c.handleDeployPreflight(ctx, conn, payload)

		case msgTypeDeployInstall:
			var payload deployInstallPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				c.logger.Error().Err(err).Msg("Failed to parse deploy_install payload")
				continue
			}
			go c.handleDeployInstall(ctx, conn, payload)

		case msgTypeDeployCancel:
			var payload deployCancelPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				c.logger.Error().Err(err).Msg("Failed to parse deploy_cancel payload")
				continue
			}
			c.handleDeployCancel(payload)

		default:
			c.logger.Debug().Str("type", string(msg.Type)).Msg("Unknown message type")
		}
	}
}

func (c *CommandClient) handleHostUpdate(ctx context.Context, conn *websocket.Conn, payload agentexec.HostUpdatePayload) {
	identity := agentexec.HostUpdateOperationIdentity(c.agentID, payload)
	updateCtx, cancel, ok := c.beginHostAPTOperation(ctx, conn, identity, payload.RequestID, "Host update", 15*time.Minute, payload.Timeout, c.replayHostUpdate)
	if !ok {
		return
	}
	defer cancel()

	result := agentexec.HostUpdateResultPayload{
		RequestID: strings.TrimSpace(payload.RequestID), ActionID: strings.TrimSpace(payload.ActionID),
		ExecutionPhase: agentexec.HostUpdatePhasePreflight, Verification: agentexec.HostUpdateVerificationInconclusive,
	}
	if c.packageUpdates == nil {
		result.Error = "host package update service is unavailable"
	} else {
		result = c.packageUpdates.Apply(updateCtx, payload)
	}
	completeHostAPTOperation(c, conn, identity, sanitizeHostUpdateReceipt(result), agentexec.HostUpdateReceiptKind, payload.RequestID, "Failed to persist host update terminal receipt", c.sendHostUpdateResult)
}

// beginHostAPTOperation runs the shared durable admission prefix of the typed
// host APT operation handlers: admit → replay-if-terminal → mark started →
// derive the bounded execution context. ok is false when the handler must
// return without executing.
func (c *CommandClient) beginHostAPTOperation(ctx context.Context, conn *websocket.Conn, identity operationreceipt.Identity, requestID, logLabel string, defaultTimeout time.Duration, payloadTimeout int, replay func(*websocket.Conn, operationreceipt.Record)) (context.Context, context.CancelFunc, bool) {
	record, admitted, err := c.admitOperation(identity)
	if err != nil {
		c.logger.Warn().Err(err).Str("request_id", requestID).Msg(logLabel + " durable admission refused")
		return nil, nil, false
	}
	if !admitted {
		if record.State == operationreceipt.StateTerminal {
			replay(conn, record)
		}
		return nil, nil, false
	}
	if _, err := c.operationReceipts.MarkStarted(identity); err != nil {
		c.logger.Warn().Err(err).Str("request_id", requestID).Msg(logLabel + " durable start refused")
		return nil, nil, false
	}
	timeout := time.Duration(payloadTimeout) * time.Second
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	return opCtx, cancel, true
}

// completeHostAPTOperation persists the sanitized terminal receipt and sends
// the result over the live connection, sharing the terminal suffix of the
// typed host APT operation handlers.
func completeHostAPTOperation[Res any](c *CommandClient, conn *websocket.Conn, identity operationreceipt.Identity, result Res, receiptKind string, requestID, persistFailureMsg string, send func(*websocket.Conn, Res)) {
	encoded, err := json.Marshal(result)
	if err != nil {
		return
	}
	if _, err := c.operationReceipts.Complete(identity, operationreceipt.TerminalEnvelope{Kind: receiptKind, Version: agentexec.HostAPTReceiptVersion, Payload: encoded}); err != nil {
		c.logger.Error().Err(err).Str("request_id", requestID).Msg(persistFailureMsg)
		return
	}
	send(conn, result)
}

func (c *CommandClient) sendHostUpdateResult(conn *websocket.Conn, result agentexec.HostUpdateResultPayload) {
	encoded, err := json.Marshal(result)
	if err != nil {
		c.logger.Error().Err(err).Str("request_id", result.RequestID).Msg("Failed to marshal host update result")
		return
	}
	msg := wsMessage{
		Type:      msgTypeHostUpdateResult,
		ID:        result.RequestID,
		Timestamp: time.Now(),
		Payload:   encoded,
	}
	c.connMu.Lock()
	err = conn.WriteJSON(msg)
	c.connMu.Unlock()
	if err != nil {
		c.logger.Error().Err(err).Str("request_id", result.RequestID).Msg("Failed to send host update result")
	}
}

func (c *CommandClient) handleHostStorageCleanup(ctx context.Context, conn *websocket.Conn, payload agentexec.HostStorageCleanupPayload) {
	identity := agentexec.HostStorageCleanupOperationIdentity(c.agentID, payload)
	cleanupCtx, cancel, ok := c.beginHostAPTOperation(ctx, conn, identity, payload.RequestID, "Host cleanup", 5*time.Minute, payload.Timeout, c.replayHostStorageCleanup)
	if !ok {
		return
	}
	defer cancel()

	result := agentexec.HostStorageCleanupResultPayload{
		RequestID: strings.TrimSpace(payload.RequestID), ActionID: strings.TrimSpace(payload.ActionID),
		ExecutionPhase: agentexec.HostStorageCleanupPhasePreflight, Verification: agentexec.HostStorageCleanupVerificationInconclusive,
	}
	if c.storageCleanup == nil {
		result.Error = "host storage cleanup service is unavailable"
	} else {
		result = c.storageCleanup.Apply(cleanupCtx, payload)
	}
	completeHostAPTOperation(c, conn, identity, sanitizeHostStorageCleanupReceipt(result), agentexec.HostStorageCleanupReceiptKind, payload.RequestID, "Failed to persist host cleanup terminal receipt", c.sendHostStorageCleanupResult)
}

func (c *CommandClient) handleDockerContainerLifecycle(ctx context.Context, conn *websocket.Conn, payload agentexec.DockerContainerLifecyclePayload) {
	identity := agentexec.DockerContainerLifecycleOperationIdentity(c.agentID, payload)
	record, admitted, err := c.admitOperation(identity)
	if err != nil {
		c.logger.Warn().Err(err).Str("request_id", payload.RequestID).Msg("Docker lifecycle durable admission refused")
		return
	}
	if !admitted {
		if record.State == operationreceipt.StateTerminal {
			c.replayDockerContainerLifecycle(conn, record)
		}
		return
	}
	if _, err := c.operationReceipts.MarkStarted(identity); err != nil {
		c.logger.Warn().Err(err).Str("request_id", payload.RequestID).Msg("Docker lifecycle durable start refused")
		return
	}
	timeout := time.Duration(payload.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	operationCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result := agentexec.DockerContainerLifecycleResultPayload{
		RequestID: payload.RequestID, ActionID: payload.ActionID, Operation: payload.Operation,
		OperationVersion: payload.OperationVersion, RequestDigest: payload.RequestDigest, ContainerID: payload.ContainerID,
		ExecutionPhase: agentexec.DockerContainerPhasePreflight,
	}
	if c.dockerLifecycle == nil {
		result.Error = "typed container lifecycle service is unavailable"
	} else {
		result = c.dockerLifecycle.Apply(operationCtx, payload)
	}
	result.Error = sanitizeDockerLifecycleError(result.Error)
	encoded, err := json.Marshal(result)
	if err != nil {
		return
	}
	if _, err := c.operationReceipts.Complete(identity, operationreceipt.TerminalEnvelope{Kind: agentexec.DockerContainerLifecycleReceiptKind, Version: agentexec.DockerContainerLifecycleReceiptVersion, Payload: encoded}); err != nil {
		c.logger.Error().Err(err).Str("request_id", payload.RequestID).Msg("Failed to persist docker lifecycle terminal receipt")
		return
	}
	c.sendDockerContainerLifecycleResult(conn, result)
}

func (c *CommandClient) sendDockerContainerLifecycleResult(conn *websocket.Conn, result agentexec.DockerContainerLifecycleResultPayload) {
	encoded, err := json.Marshal(result)
	if err != nil {
		return
	}
	msg := wsMessage{Type: msgTypeDockerContainerLifecycleResult, ID: result.RequestID, Timestamp: time.Now(), Payload: encoded}
	c.connMu.Lock()
	err = conn.WriteJSON(msg)
	c.connMu.Unlock()
	if err != nil {
		c.logger.Error().Err(err).Str("request_id", result.RequestID).Msg("Failed to send docker lifecycle result")
	}
}

func (c *CommandClient) admitOperation(identity operationreceipt.Identity) (operationreceipt.Record, bool, error) {
	if c.operationReceiptErr != nil {
		return operationreceipt.Record{}, false, c.operationReceiptErr
	}
	if c.operationReceipts == nil {
		return operationreceipt.Record{}, false, fmt.Errorf("operation receipt store unavailable")
	}
	return c.operationReceipts.Admit(identity)
}

func (c *CommandClient) handleOperationQuery(conn *websocket.Conn, correlationID string, query operationreceipt.Query) {
	var result operationreceipt.QueryResult
	var err error
	if query.Identity.AgentID != c.agentID {
		err = operationreceipt.ErrBindingConflict
	} else if c.operationReceiptErr != nil {
		err = c.operationReceiptErr
	} else if c.operationReceipts == nil {
		err = fmt.Errorf("operation receipt store unavailable")
	} else {
		result, err = c.operationReceipts.Query(query.Identity)
	}
	if err != nil {
		c.logger.Warn().Err(err).Str("request_id", query.Identity.AttemptID).Msg("Operation receipt query refused")
		return
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return
	}
	msg := wsMessage{Type: msgTypeOperationQueryResult, ID: strings.TrimSpace(correlationID), Timestamp: time.Now(), Payload: encoded}
	c.connMu.Lock()
	err = conn.WriteJSON(msg)
	c.connMu.Unlock()
	if err != nil {
		c.logger.Debug().Err(err).Str("request_id", query.Identity.AttemptID).Msg("Failed to send operation receipt query result")
	}
}

func (c *CommandClient) replayHostUpdate(conn *websocket.Conn, record operationreceipt.Record) {
	var result agentexec.HostUpdateResultPayload
	if err := json.Unmarshal(record.Result, &result); err == nil {
		c.sendHostUpdateResult(conn, result)
	}
}

func (c *CommandClient) replayHostStorageCleanup(conn *websocket.Conn, record operationreceipt.Record) {
	var result agentexec.HostStorageCleanupResultPayload
	if err := json.Unmarshal(record.Result, &result); err == nil {
		c.sendHostStorageCleanupResult(conn, result)
	}
}

func (c *CommandClient) replayDockerContainerLifecycle(conn *websocket.Conn, record operationreceipt.Record) {
	var result agentexec.DockerContainerLifecycleResultPayload
	if err := json.Unmarshal(record.Result, &result); err == nil {
		c.sendDockerContainerLifecycleResult(conn, result)
	}
}

func sanitizeDockerLifecycleError(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "typed container lifecycle did not complete as requested"
}

func sanitizeHostUpdateReceipt(result agentexec.HostUpdateResultPayload) agentexec.HostUpdateResultPayload {
	result.Before.Packages, result.After.Packages = nil, nil
	result.Before.Error, result.After.Error = "", ""
	if result.Error != "" {
		result.Error = "typed host update did not complete"
	}
	return result
}

func sanitizeHostStorageCleanupReceipt(result agentexec.HostStorageCleanupResultPayload) agentexec.HostStorageCleanupResultPayload {
	result.Before.Error, result.After.Error = "", ""
	if result.Error != "" {
		result.Error = "typed host cleanup did not complete"
	}
	return result
}

func hostOperationReceiptConfig() operationreceipt.Config {
	return operationreceipt.Config{Validators: map[string]map[int]operationreceipt.TerminalValidator{
		agentexec.HostUpdateReceiptKind: {agentexec.HostAPTReceiptVersion: func(identity operationreceipt.Identity, payload json.RawMessage) error {
			result, err := agentexec.DecodeHostUpdateResultPayload(payload)
			if err != nil {
				return err
			}
			if len(result.Before.Packages) > 0 || len(result.After.Packages) > 0 || result.Before.Error != "" || result.After.Error != "" {
				return fmt.Errorf("host update receipt contains non-persistable detail")
			}
			return agentexec.ValidateHostUpdateReceiptForIdentity(identity, result)
		}},
		agentexec.HostStorageCleanupReceiptKind: {agentexec.HostAPTReceiptVersion: func(identity operationreceipt.Identity, payload json.RawMessage) error {
			result, err := agentexec.DecodeHostStorageCleanupResultPayload(payload)
			if err != nil {
				return err
			}
			if result.Before.Error != "" || result.After.Error != "" {
				return fmt.Errorf("host cleanup receipt contains non-persistable detail")
			}
			return agentexec.ValidateHostStorageCleanupReceiptForIdentity(identity, result)
		}},
		agentexec.DockerContainerLifecycleReceiptKind: {agentexec.DockerContainerLifecycleReceiptVersion: func(identity operationreceipt.Identity, payload json.RawMessage) error {
			result, err := agentexec.DecodeDockerContainerLifecycleResultPayload(payload)
			if err != nil {
				return err
			}
			if result.RequestID != identity.AttemptID || result.ActionID != identity.ActionID || result.Operation != identity.OperationKind || result.OperationVersion != identity.OperationVersion || result.RequestDigest != identity.RequestDigest {
				return operationreceipt.ErrBindingConflict
			}
			return nil
		}},
		agentexec.DockerContainerUpdateReceiptKind: {agentexec.DockerContainerUpdateReceiptVersion: func(identity operationreceipt.Identity, payload json.RawMessage) error {
			result, err := agentexec.DecodeDockerContainerUpdateResultPayload(payload)
			if err != nil {
				return err
			}
			if result.RequestID != identity.AttemptID || result.ActionID != identity.ActionID || result.Operation != identity.OperationKind || result.OperationVersion != identity.OperationVersion || result.RequestDigest != identity.RequestDigest {
				return operationreceipt.ErrBindingConflict
			}
			return nil
		}},
	}}
}

func (c *CommandClient) sendHostStorageCleanupResult(conn *websocket.Conn, result agentexec.HostStorageCleanupResultPayload) {
	encoded, err := json.Marshal(result)
	if err != nil {
		c.logger.Error().Err(err).Str("request_id", result.RequestID).Msg("Failed to marshal host storage cleanup result")
		return
	}
	msg := wsMessage{
		Type:      msgTypeHostStorageCleanupResult,
		ID:        result.RequestID,
		Timestamp: time.Now(),
		Payload:   encoded,
	}
	c.connMu.Lock()
	err = conn.WriteJSON(msg)
	c.connMu.Unlock()
	if err != nil {
		c.logger.Error().Err(err).Str("request_id", result.RequestID).Msg("Failed to send host storage cleanup result")
	}
}

func (c *CommandClient) handleExecuteCommand(ctx context.Context, conn *websocket.Conn, payload executeCommandPayload) {
	startTime := time.Now()

	c.logger.Info().
		Str("request_id", payload.RequestID).
		Str("command_hash", agentexec.ComputeCommandApprovalHash(payload.Command, normalizedCommandTargetType(payload.TargetType), payload.TargetID)).
		Str("approval_id", payload.ApprovalID).
		Str("target_type", payload.TargetType).
		Str("target_id", payload.TargetID).
		Msg("Executing command")

	result := c.executeCommand(ctx, payload)
	result.Duration = time.Since(startTime).Milliseconds()

	// Send result back
	resultPayload, err := json.Marshal(result)
	if err != nil {
		c.logger.Error().Err(err).Str("request_id", payload.RequestID).Msg("Failed to marshal command result")
		return
	}
	msg := wsMessage{
		Type:      msgTypeCommandResult,
		ID:        payload.RequestID,
		Timestamp: time.Now(),
		Payload:   resultPayload,
	}

	c.connMu.Lock()
	writeErr := conn.WriteJSON(msg)
	c.connMu.Unlock()

	if writeErr != nil {
		c.logger.Error().Err(writeErr).Str("request_id", payload.RequestID).Msg("Failed to send command result")
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
	targetType := strings.ToLower(strings.TrimSpace(payload.TargetType))
	targetID := strings.TrimSpace(payload.TargetID)

	// Only validate TargetID when it will be interpolated into the command
	// (container and vm types). Host type doesn't use TargetID in the command.
	needsTargetID := targetType == "container" || targetType == "vm"

	if needsTargetID {
		if targetID == "" {
			return "sh -c 'echo \"Error: missing target ID\" >&2; exit 1'"
		}
		// Validate TargetID to prevent shell injection - defense in depth
		if !safeTargetIDPattern.MatchString(targetID) {
			// Return a command that fails with non-zero exit and error message
			return "sh -c 'echo \"Error: invalid target ID\" >&2; exit 1'"
		}

		// Wrap command in sh -c so shell metacharacters (pipes, redirects, globs)
		// are processed inside the container/VM, not on the Proxmox host.
		// Without this, "pct exec 141 -- grep pattern /var/log/*.log" would
		// expand the glob on the host (where /var/log/*.log doesn't exist).
		quotedCmd := shellQuote(payload.Command)

		if targetType == "container" {
			return fmt.Sprintf("pct exec %s -- sh -c %s", targetID, quotedCmd)
		}
		if targetType == "vm" {
			return fmt.Sprintf("qm guest exec %s -- sh -c %s", targetID, quotedCmd)
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

func (c *CommandClient) currentCommandPolicy() *agentexec.CommandPolicy {
	if c != nil && c.commandPolicy != nil {
		return c.commandPolicy
	}
	return agentexec.DefaultPolicy()
}

func (c *CommandClient) authorizeCommand(payload executeCommandPayload) error {
	switch c.currentCommandPolicy().Evaluate(payload.Command) {
	case agentexec.PolicyBlock:
		return fmt.Errorf("command blocked by policy")
	case agentexec.PolicyRequireApproval:
		// Trusted commands come from vetted Pulse-internal subsystems over an
		// authenticated WebSocket; they do not require user approval. The
		// PolicyBlock branch above still enforces hard limits.
		if payload.Trusted {
			return nil
		}
		if strings.TrimSpace(payload.ApprovalID) == "" {
			return fmt.Errorf("command requires approval")
		}
		if err := agentexec.VerifyCommandApprovalGrant(c.apiToken, c.agentID, payload.toAgentExecPayload(), time.Now()); err != nil {
			commandApprovalGrantRejectionsTotal.WithLabelValues(agentexec.ApprovalGrantVerificationReason(err)).Inc()
			return fmt.Errorf("command approval grant invalid: %w", err)
		}
	}
	return nil
}

func (p executeCommandPayload) toAgentExecPayload() agentexec.ExecuteCommandPayload {
	return agentexec.ExecuteCommandPayload{
		RequestID:     strings.TrimSpace(p.RequestID),
		Command:       p.Command,
		ApprovalID:    strings.TrimSpace(p.ApprovalID),
		ApprovalGrant: p.ApprovalGrant,
		TargetType:    normalizedCommandTargetType(p.TargetType),
		TargetID:      strings.TrimSpace(p.TargetID),
		Timeout:       p.Timeout,
		Trusted:       p.Trusted,
	}
}

func normalizedCommandTargetType(targetType string) string {
	normalized := strings.ToLower(strings.TrimSpace(targetType))
	if normalized == "" || normalized == "host" {
		return "agent"
	}
	return normalized
}

func (c *CommandClient) ensureSSHKnownHostsManager() (sshknownhosts.Manager, error) {
	if c == nil {
		return nil, fmt.Errorf("command client is nil")
	}

	c.sshKnownHostsOnce.Do(func() {
		if c.sshKnownHosts != nil {
			return
		}

		stateDir := strings.TrimSpace(c.stateDir)
		if stateDir == "" {
			stateDir = defaultStateDir
		}

		c.sshKnownHosts, c.sshKnownHostsErr = sshknownhosts.NewManager(filepath.Join(stateDir, "ssh_known_hosts"))
	})

	if c.sshKnownHostsErr != nil {
		return nil, c.sshKnownHostsErr
	}
	if c.sshKnownHosts == nil {
		return nil, fmt.Errorf("ssh known_hosts manager is not configured")
	}
	return c.sshKnownHosts, nil
}

func (c *CommandClient) executeCommand(ctx context.Context, payload executeCommandPayload) commandResultPayload {
	result := commandResultPayload{
		RequestID: payload.RequestID,
	}

	if err := c.authorizeCommand(payload); err != nil {
		result.Error = err.Error()
		result.ExitCode = -1
		result.Success = false
		return result
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

	stdout := newCappedBuffer(maxCommandOutputSize)
	stderr := newCappedBuffer(maxCommandOutputSize)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

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

	return result
}

// Close closes the command client connection
func (c *CommandClient) Close() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.done == nil {
		c.done = make(chan struct{})
	}
	select {
	case <-c.done:
	default:
		close(c.done)
	}

	var closeErr error
	if c.conn != nil {
		closeErr = c.conn.Close()
		c.conn = nil
	}
	c.operationReceiptCloseOnce.Do(func() {
		if c.operationReceipts != nil {
			c.operationReceiptCloseErr = c.operationReceipts.Close()
		}
	})
	if closeErr != nil {
		return closeErr
	}
	return c.operationReceiptCloseErr
}
