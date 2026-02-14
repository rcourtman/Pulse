package agentexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return isAllowedWebSocketOrigin(r)
	},
}

var (
	jsonMarshal      = json.Marshal
	writeTextMessage = func(conn *websocket.Conn, data []byte) error {
		return conn.WriteMessage(websocket.TextMessage, data)
	}
	pingInterval    = 5 * time.Second
	pingWriteWait   = 5 * time.Second
	readFileTimeout = 30 * time.Second

	errServerShuttingDown = errors.New("agent execution server is shutting down")
)

const maxWebSocketMessageBytes int64 = 1 << 20 // 1 MiB

const (
	maxAgentIDLength                      = 128
	maxRequestIDLength                    = 128
	maxExecuteCommandLength               = 32 * 1024
	maxTargetIDLength                     = 256
	maxExecuteCommandTimeoutSeconds       = 3600
	defaultReadFileMaxBytes         int64 = 1 << 20  // 1 MiB
	maxReadFileMaxBytes             int64 = 10 << 20 // 10 MiB
	maxReadFilePathLength                 = 4096
)

var safeTargetIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// Server manages WebSocket connections from agents
type Server struct {
	mu            sync.RWMutex
	agents        map[string]*agentConn                // agentID -> connection
	pendingReqs   map[string]chan CommandResultPayload // scoped request key -> response channel
	validateToken func(token string, agentID string) bool
	shutdown      chan struct{}
	shutdownOnce  sync.Once
}

type agentConn struct {
	conn     *websocket.Conn
	agent    ConnectedAgent
	writeMu  sync.Mutex
	done     chan struct{}
	doneOnce sync.Once
}

func (ac *agentConn) signalDone() {
	ac.doneOnce.Do(func() {
		defer func() {
			// Some call sites/tests may have already closed done directly.
			_ = recover()
		}()
		close(ac.done)
	})
}

// NewServer creates a new agent execution server
func NewServer(validateToken func(token string, agentID string) bool) *Server {
	return &Server{
		agents:        make(map[string]*agentConn),
		pendingReqs:   make(map[string]chan CommandResultPayload),
		validateToken: validateToken,
		shutdown:      make(chan struct{}),
	}
}

func (s *Server) isShuttingDown() bool {
	select {
	case <-s.shutdown:
		return true
	default:
		return false
	}
}

func pendingRequestKey(agentID, requestID string) string {
	return agentID + "\x00" + requestID
}

func normalizeTarget(targetType, targetID string) (string, string, error) {
	normalizedType := strings.ToLower(strings.TrimSpace(targetType))
	if normalizedType == "" {
		normalizedType = "host"
	}

	normalizedTargetID := strings.TrimSpace(targetID)
	switch normalizedType {
	case "host":
		// Host-level execution ignores target ID.
		return normalizedType, "", nil
	case "container", "vm":
		if normalizedTargetID == "" {
			return "", "", fmt.Errorf("target id is required for target type %q", normalizedType)
		}
		if len(normalizedTargetID) > maxTargetIDLength {
			return "", "", fmt.Errorf("target id exceeds %d characters", maxTargetIDLength)
		}
		if !safeTargetIDPattern.MatchString(normalizedTargetID) {
			return "", "", fmt.Errorf("target id contains invalid characters")
		}
		return normalizedType, normalizedTargetID, nil
	default:
		return "", "", fmt.Errorf("invalid target type %q", targetType)
	}
}

func validateExecuteCommandPayload(cmd *ExecuteCommandPayload) error {
	if cmd == nil {
		return fmt.Errorf("command payload is required")
	}

	if strings.TrimSpace(cmd.Command) == "" {
		return fmt.Errorf("command is required")
	}
	if len(cmd.Command) > maxExecuteCommandLength {
		return fmt.Errorf("command exceeds %d characters", maxExecuteCommandLength)
	}

	targetType, targetID, err := normalizeTarget(cmd.TargetType, cmd.TargetID)
	if err != nil {
		return err
	}
	cmd.TargetType = targetType
	cmd.TargetID = targetID

	if cmd.Timeout < 0 {
		return fmt.Errorf("timeout cannot be negative")
	}
	if cmd.Timeout > maxExecuteCommandTimeoutSeconds {
		return fmt.Errorf("timeout cannot exceed %d seconds", maxExecuteCommandTimeoutSeconds)
	}

	return nil
}

func validateReadFilePayload(req *ReadFilePayload) error {
	if req == nil {
		return fmt.Errorf("read file payload is required")
	}

	req.Path = strings.TrimSpace(req.Path)
	if req.Path == "" {
		return fmt.Errorf("path is required")
	}
	if len(req.Path) > maxReadFilePathLength {
		return fmt.Errorf("path exceeds %d characters", maxReadFilePathLength)
	}
	if strings.ContainsAny(req.Path, "\x00\r\n") {
		return fmt.Errorf("path contains invalid control characters")
	}

	targetType, targetID, err := normalizeTarget(req.TargetType, req.TargetID)
	if err != nil {
		return err
	}
	req.TargetType = targetType
	req.TargetID = targetID

	if req.MaxBytes < 0 {
		return fmt.Errorf("max bytes cannot be negative")
	}
	if req.MaxBytes == 0 {
		req.MaxBytes = defaultReadFileMaxBytes
	}
	if req.MaxBytes > maxReadFileMaxBytes {
		return fmt.Errorf("max bytes cannot exceed %d", maxReadFileMaxBytes)
	}

	return nil
}

func isAllowedWebSocketOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		// Non-browser clients (expected for agents) usually omit Origin.
		return true
	}

	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	return normalizeOriginHost(parsed.Host) == normalizeOriginHost(r.Host)
}

func normalizeOriginHost(host string) string {
	normalized := strings.TrimSpace(strings.ToLower(host))
	if normalized == "" {
		return normalized
	}

	parsedHost, parsedPort, err := net.SplitHostPort(normalized)
	if err != nil {
		return normalized
	}
	if parsedPort == "80" || parsedPort == "443" {
		return parsedHost
	}
	return net.JoinHostPort(parsedHost, parsedPort)
}

// HandleWebSocket handles incoming WebSocket connections from agents
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	remoteAddr := r.RemoteAddr

	if s.isShuttingDown() {
		http.Error(w, "agent execution server is shutting down", http.StatusServiceUnavailable)
		return
	}

	// CRITICAL: Clear http.Server deadlines BEFORE WebSocket upgrade.
	// The http.Server.ReadTimeout sets a deadline on the underlying connection when
	// the request starts. We must clear it before the upgrade or the connection will
	// be closed when that deadline fires (typically ~15 seconds after connection).
	// Use http.ResponseController (Go 1.20+) to clear the deadline.
	rc := http.NewResponseController(w)
	if err := rc.SetReadDeadline(time.Time{}); err != nil {
		log.Debug().
			Err(err).
			Str("remote_addr", remoteAddr).
			Msg("Failed to clear read deadline via ResponseController")
	}
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		log.Debug().
			Err(err).
			Str("remote_addr", remoteAddr).
			Msg("Failed to clear write deadline via ResponseController")
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Str("remote_addr", remoteAddr).Msg("Failed to upgrade WebSocket connection")
		return
	}
	closeConn := func(context string) {
		if closeErr := conn.Close(); closeErr != nil {
			log.Debug().Err(closeErr).Msg(context)
		}
	}

	if s.isShuttingDown() {
		conn.Close()
		return
	}

	// Also clear on the WebSocket's underlying connection as a safety net
	if netConn := conn.NetConn(); netConn != nil {
		if err := netConn.SetReadDeadline(time.Time{}); err != nil {
			log.Debug().Err(err).Msg("Failed to clear net.Conn read deadline")
		}
		if err := netConn.SetWriteDeadline(time.Time{}); err != nil {
			log.Debug().Err(err).Msg("Failed to clear net.Conn write deadline")
		}
	}

	// Read first message (must be agent_register)
	if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		log.Warn().Err(err).Msg("Failed to set initial registration read deadline")
	}
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		log.Error().Err(err).Str("remote_addr", remoteAddr).Msg("Failed to read registration message")
		closeConn("Failed to close connection after registration read error")
		return
	}

	var msg Message
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		log.Error().Err(err).Str("remote_addr", remoteAddr).Msg("Failed to parse registration message")
		closeConn("Failed to close connection after registration parse error")
		return
	}

	if msg.Type != MsgTypeAgentRegister {
		log.Error().Str("type", string(msg.Type)).Str("remote_addr", remoteAddr).Msg("First message must be agent_register")
		closeConn("Failed to close connection after invalid first message type")
		return
	}

	// Parse registration payload
	var reg AgentRegisterPayload
	if err := msg.DecodePayload(&reg); err != nil {
		log.Error().Err(err).Str("remote_addr", remoteAddr).Msg("Failed to parse registration payload")
		closeConn("Failed to close connection after registration payload parse error")
		return
	}

	reg.AgentID = strings.TrimSpace(reg.AgentID)
	if reg.AgentID == "" {
		log.Warn().Msg("Agent registration rejected: missing agent_id")
		rejMsg, rejErr := NewMessage(MsgTypeRegistered, "", RegisteredPayload{Success: false, Message: "Invalid agent_id"})
		if rejErr != nil {
			log.Warn().Err(rejErr).Msg("Failed to encode rejection message")
		} else if sendErr := s.sendMessage(conn, rejMsg); sendErr != nil {
			log.Warn().Err(sendErr).Msg("Failed to send rejection to agent with missing agent_id")
		}
		conn.Close()
		return
	}
	if len(reg.AgentID) > maxAgentIDLength {
		log.Warn().
			Int("agent_id_length", len(reg.AgentID)).
			Msg("Agent registration rejected: agent_id exceeds maximum length")
		rejMsg, rejErr := NewMessage(MsgTypeRegistered, "", RegisteredPayload{Success: false, Message: "Invalid agent_id"})
		if rejErr != nil {
			log.Warn().Err(rejErr).Msg("Failed to encode rejection for oversized agent_id")
		} else if sendErr := s.sendMessage(conn, rejMsg); sendErr != nil {
			log.Warn().Err(sendErr).Msg("Failed to send rejection to agent with oversized agent_id")
		}
		conn.Close()
		return
	}

	// Validate token
	if s.validateToken != nil && !s.validateToken(reg.Token, reg.AgentID) {
		log.Warn().Str("agent_id", reg.AgentID).Msg("Agent registration rejected: invalid token")
		rejectedMsg, err := NewMessage(MsgTypeRegistered, "", RegisteredPayload{Success: false, Message: "Invalid token"})
		if err != nil {
			log.Warn().Err(err).Str("agent_id", reg.AgentID).Msg("Failed to encode rejection message")
			conn.Close()
			return
		}
		if err := s.sendMessage(conn, rejectedMsg); err != nil {
			log.Warn().Err(err).Str("agent_id", reg.AgentID).Msg("Failed to send rejection to agent")
		}
		closeConn("Failed to close connection after registration rejection")
		return
	}

	// Create agent connection
	ac := &agentConn{
		conn: conn,
		agent: ConnectedAgent{
			AgentID:     reg.AgentID,
			Hostname:    reg.Hostname,
			Version:     reg.Version,
			Platform:    reg.Platform,
			Tags:        reg.Tags,
			ConnectedAt: time.Now(),
		},
		done: make(chan struct{}),
	}

	// Clear deadline for normal operation - both on the WebSocket and underlying connection
	// This MUST happen BEFORE registering the agent in the map to avoid race conditions
	// where other goroutines could call ExecuteCommand while we're still configuring the connection.
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		log.Warn().Err(err).Str("agent_id", reg.AgentID).Msg("Failed to clear read deadline after registration")
	}
	if err := conn.SetWriteDeadline(time.Time{}); err != nil {
		log.Warn().Err(err).Str("agent_id", reg.AgentID).Msg("Failed to clear write deadline after registration")
	}
	if netConn := conn.NetConn(); netConn != nil {
		if err := netConn.SetReadDeadline(time.Time{}); err != nil {
			log.Warn().Err(err).Str("agent_id", reg.AgentID).Msg("Failed to clear net.Conn read deadline after registration")
		}
		if err := netConn.SetWriteDeadline(time.Time{}); err != nil {
			log.Warn().Err(err).Str("agent_id", reg.AgentID).Msg("Failed to clear net.Conn write deadline after registration")
		}
	}

	// Set up ping/pong handlers to keep connection alive
	conn.SetPongHandler(func(appData string) error {
		// Reset read deadline on pong received
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			return fmt.Errorf("set read deadline on pong: %w", err)
		}
		return nil
	})

	// Register agent - after this point, other goroutines can access the connection
	s.mu.Lock()
	// Close existing connection if any
	if existing, ok := s.agents[reg.AgentID]; ok {
		log.Info().
			Str("agent_id", reg.AgentID).
			Str("hostname", reg.Hostname).
			Msg("Replacing existing agent connection")
		close(existing.done)
		if err := existing.conn.Close(); err != nil {
			log.Debug().Err(err).Str("agent_id", reg.AgentID).Msg("Failed to close existing connection during reconnect")
		}
	}
	s.agents[reg.AgentID] = ac
	s.mu.Unlock()

	log.Info().
		Str("agent_id", reg.AgentID).
		Str("hostname", reg.Hostname).
		Str("version", reg.Version).
		Str("platform", reg.Platform).
		Msg("Agent connected")

	// Send registration success
	ackMsg, ackErr := NewMessage(MsgTypeRegistered, "", RegisteredPayload{Success: true, Message: "Registered"})
	if ackErr != nil {
		log.Warn().Err(ackErr).Str("agent_id", reg.AgentID).Msg("Failed to encode registration ack")
		conn.Close()
		return
	}
	ac.writeMu.Lock()
	if sendErr := s.sendMessage(conn, ackMsg); sendErr != nil {
		log.Warn().
			Err(sendErr).
			Str("agent_id", reg.AgentID).
			Str("hostname", reg.Hostname).
			Msg("Failed to send registration ack")
	}
	ac.writeMu.Unlock()

	// Start server-side ping loop to keep connection alive
	pingDone := make(chan struct{})
	go s.pingLoop(ac, pingDone)
	defer close(pingDone)

	// Run read loop (blocking) - don't use goroutine, or HTTP handler will close connection
	s.readLoop(ac)
}

func (s *Server) readLoop(ac *agentConn) {
	defer func() {
		s.mu.Lock()
		if existing, ok := s.agents[ac.agent.AgentID]; ok && existing == ac {
			delete(s.agents, ac.agent.AgentID)
		}
		s.mu.Unlock()
		if err := ac.conn.Close(); err != nil {
			log.Debug().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Failed to close connection during read-loop cleanup")
		}
		log.Info().Str("agent_id", ac.agent.AgentID).Msg("Agent disconnected")
	}()

	log.Debug().Str("agent_id", ac.agent.AgentID).Msg("Starting read loop for agent")

	for {
		select {
		case <-ac.done:
			log.Debug().Str("agent_id", ac.agent.AgentID).Msg("Read loop exiting: done channel closed")
			return
		default:
		}

		_, msgBytes, err := ac.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Unexpected WebSocket close error")
			} else {
				log.Debug().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Read loop exiting: read error")
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			log.Error().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Failed to parse message")
			continue
		}

		switch msg.Type {
		case MsgTypeAgentPing:
			pongMsg, err := NewMessage(MsgTypePong, "", nil)
			if err != nil {
				log.Debug().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Failed to encode pong message")
				continue
			}
			ac.writeMu.Lock()
			if err := s.sendMessage(ac.conn, pongMsg); err != nil {
				log.Debug().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Failed to send pong")
			}
			ac.writeMu.Unlock()

		case MsgTypeCommandResult:
			var result CommandResultPayload
			if err := msg.DecodePayload(&result); err != nil {
				log.Error().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Failed to parse command result")
				continue
			}
			result.RequestID = strings.TrimSpace(result.RequestID)
			if result.RequestID == "" {
				log.Warn().Str("agent_id", ac.agent.AgentID).Msg("Dropping command result with empty request_id")
				continue
			}
			if len(result.RequestID) > maxRequestIDLength {
				log.Warn().
					Str("agent_id", ac.agent.AgentID).
					Int("request_id_length", len(result.RequestID)).
					Msg("Dropping command result with oversized request_id")
				continue
			}

			s.mu.RLock()
			ch, ok := s.pendingReqs[pendingRequestKey(ac.agent.AgentID, result.RequestID)]
			s.mu.RUnlock()

			if ok {
				select {
				case ch <- result:
					log.Debug().
						Str("agent_id", ac.agent.AgentID).
						Str("request_id", result.RequestID).
						Bool("success", result.Success).
						Int("exit_code", result.ExitCode).
						Int64("duration_ms", result.Duration).
						Msg("Received command result from agent")
				default:
					log.Warn().
						Str("agent_id", ac.agent.AgentID).
						Str("request_id", result.RequestID).
						Msg("Result channel full, dropping")
				}
			} else {
				log.Warn().
					Str("agent_id", ac.agent.AgentID).
					Str("request_id", result.RequestID).
					Msg("No pending request for result")
			}
		}
	}
}

func (s *Server) pingLoop(ac *agentConn, done chan struct{}) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	// Track consecutive ping failures to detect dead connections faster
	consecutiveFailures := 0
	const maxConsecutiveFailures = 3

	for {
		select {
		case <-done:
			return
		case <-ac.done:
			return
		case <-ticker.C:
			ac.writeMu.Lock()
			err := ac.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(pingWriteWait))
			ac.writeMu.Unlock()
			if err != nil {
				consecutiveFailures++
				log.Warn().
					Err(err).
					Str("agent_id", ac.agent.AgentID).
					Str("hostname", ac.agent.Hostname).
					Int("consecutive_failures", consecutiveFailures).
					Msg("Failed to send ping to agent")

				if consecutiveFailures >= maxConsecutiveFailures {
					log.Error().
						Err(err).
						Str("agent_id", ac.agent.AgentID).
						Str("hostname", ac.agent.Hostname).
						Int("failures", consecutiveFailures).
						Msg("Agent connection appears dead after multiple ping failures, closing connection")

					// Close the connection - this will cause readLoop to exit and clean up
					if closeErr := ac.conn.Close(); closeErr != nil {
						log.Debug().Err(closeErr).Str("agent_id", ac.agent.AgentID).Msg("Failed to close dead connection after ping failures")
					}
					return
				}
			} else {
				// Reset failure counter on successful ping
				consecutiveFailures = 0
			}
		}
	}
}

func (s *Server) sendMessage(conn *websocket.Conn, msg Message) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal websocket message: %w", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		return fmt.Errorf("write websocket message: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the server by closing all active agent connections.
// The method is idempotent.
func (s *Server) Shutdown() {
	s.shutdownOnce.Do(func() {
		close(s.shutdown)

		s.mu.Lock()
		agents := make([]*agentConn, 0, len(s.agents))
		for _, ac := range s.agents {
			agents = append(agents, ac)
		}
		s.agents = make(map[string]*agentConn)
		s.mu.Unlock()

		for _, ac := range agents {
			ac.signalDone()
			_ = ac.conn.Close()
		}
	})
}

// ExecuteCommand sends a command to an agent and waits for the result
func (s *Server) ExecuteCommand(ctx context.Context, agentID string, cmd ExecuteCommandPayload) (*CommandResultPayload, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	cmd.RequestID = strings.TrimSpace(cmd.RequestID)
	if cmd.RequestID == "" {
		cmd.RequestID = uuid.New().String()
	}
	if err := validateExecuteCommandPayload(&cmd); err != nil {
		return nil, err
	}

	startedAt := time.Now()

	s.mu.RLock()
	ac, ok := s.agents[agentID]
	s.mu.RUnlock()

	if !ok {
		log.Warn().
			Str("agent_id", agentID).
			Str("request_id", cmd.RequestID).
			Msg("Execute command requested for disconnected agent")
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}

	execLog := log.With().
		Str("agent_id", agentID).
		Str("request_id", cmd.RequestID).
		Str("target_type", cmd.TargetType).
		Str("target_id", cmd.TargetID).
		Logger()

	// Create response channel
	respCh := make(chan CommandResultPayload, 1)
	reqKey := pendingRequestKey(agentID, cmd.RequestID)
	s.mu.Lock()
	s.pendingReqs[reqKey] = respCh
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pendingReqs, reqKey)
		s.mu.Unlock()
	}()

	// Send command
	execMsg, execErr := NewMessage(MsgTypeExecuteCmd, cmd.RequestID, cmd)
	if execErr != nil {
		return nil, fmt.Errorf("failed to encode command: %w", execErr)
	}

	ac.writeMu.Lock()
	err := s.sendMessage(ac.conn, execMsg)
	ac.writeMu.Unlock()

	if err != nil {
		execLog.Error().
			Err(err).
			Dur("duration", time.Since(startedAt)).
			Msg("Failed to send command to agent")
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// Wait for result
	timeout := time.Duration(cmd.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()

	select {
	case result := <-respCh:
		execLog.Info().
			Bool("success", result.Success).
			Int("exit_code", result.ExitCode).
			Int64("agent_duration_ms", result.Duration).
			Dur("duration", time.Since(startedAt)).
			Msg("Agent command completed")
		return &result, nil
	case <-time.After(timeout):
		execLog.Warn().
			Dur("timeout", timeout).
			Dur("duration", time.Since(startedAt)).
			Msg("Agent command timed out")
		return nil, fmt.Errorf("command timed out after %v", timeout)
	case <-ctx.Done():
		execLog.Warn().
			Err(ctx.Err()).
			Dur("duration", time.Since(startedAt)).
			Msg("Agent command canceled")
		return nil, ctx.Err()
	case <-s.shutdown:
		return nil, errServerShuttingDown
	}
}

// ReadFile reads a file from an agent
func (s *Server) ReadFile(ctx context.Context, agentID string, req ReadFilePayload) (*CommandResultPayload, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	req.RequestID = strings.TrimSpace(req.RequestID)
	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}
	if err := validateReadFilePayload(&req); err != nil {
		return nil, err
	}

	s.mu.RLock()
	ac, ok := s.agents[agentID]
	s.mu.RUnlock()

	if !ok {
		log.Warn().
			Str("agent_id", agentID).
			Str("request_id", req.RequestID).
			Msg("Read file requested for disconnected agent")
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}

	readLog := log.With().
		Str("agent_id", agentID).
		Str("request_id", req.RequestID).
		Str("path", req.Path).
		Str("target_type", req.TargetType).
		Str("target_id", req.TargetID).
		Int64("max_bytes", req.MaxBytes).
		Logger()

	startedAt := time.Now()

	// Create response channel
	respCh := make(chan CommandResultPayload, 1)
	reqKey := pendingRequestKey(agentID, req.RequestID)
	s.mu.Lock()
	s.pendingReqs[reqKey] = respCh
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pendingReqs, reqKey)
		s.mu.Unlock()
	}()

	// Send request
	readPayloadBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode read_file request: %w", err)
	}
	msg := Message{
		Type:      MsgTypeReadFile,
		ID:        req.RequestID,
		Timestamp: time.Now(),
		Payload:   readPayloadBytes,
	}

	ac.writeMu.Lock()
	sendErr := s.sendMessage(ac.conn, msg)
	ac.writeMu.Unlock()

	if sendErr != nil {
		readLog.Error().
			Err(sendErr).
			Dur("duration", time.Since(startedAt)).
			Msg("Failed to send read_file request to agent")
		return nil, fmt.Errorf("failed to send read_file request: %w", sendErr)
	}

	// Wait for result
	timeout := readFileTimeout
	timer := time.NewTimer(timeout)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()

	select {
	case result := <-respCh:
		readLog.Info().
			Bool("success", result.Success).
			Int("exit_code", result.ExitCode).
			Int64("agent_duration_ms", result.Duration).
			Dur("duration", time.Since(startedAt)).
			Msg("Agent read_file completed")
		return &result, nil
	case <-timer.C:
		return nil, fmt.Errorf("read_file timed out after %v", timeout)
	case <-ctx.Done():
		return nil, fmt.Errorf("read_file %q on agent %q canceled: %w", req.RequestID, agentID, ctx.Err())
	}
}

// GetConnectedAgents returns a list of currently connected agents
func (s *Server) GetConnectedAgents() []ConnectedAgent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]ConnectedAgent, 0, len(s.agents))
	for _, ac := range s.agents {
		agents = append(agents, ac.agent)
	}
	return agents
}

// IsAgentConnected checks if an agent is currently connected
func (s *Server) IsAgentConnected(agentID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.agents[agentID]
	return ok
}

// GetAgentForHost finds the agent for a given hostname
func (s *Server) GetAgentForHost(hostname string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ac := range s.agents {
		if ac.agent.Hostname == hostname {
			return ac.agent.AgentID, true
		}
	}
	return "", false
}
