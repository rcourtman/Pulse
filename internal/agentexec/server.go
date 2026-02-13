package agentexec

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Agents can connect from anywhere
	},
}

var (
	pingInterval    = 5 * time.Second
	pingWriteWait   = 5 * time.Second
	readFileTimeout = 30 * time.Second
)

// Server manages WebSocket connections from agents
type Server struct {
	mu              sync.RWMutex
	agents          map[string]*agentConn                // agentID -> connection
	pendingRequests map[string]chan CommandResultPayload // requestID -> response channel
	validateToken   func(token string, agentID string) bool
}

type agentConn struct {
	conn    *websocket.Conn
	agent   ConnectedAgent
	writeMu sync.Mutex
	done    chan struct{}
}

// NewServer creates a new agent execution server
func NewServer(validateToken func(token string, agentID string) bool) *Server {
	return &Server{
		agents:          make(map[string]*agentConn),
		pendingRequests: make(map[string]chan CommandResultPayload),
		validateToken:   validateToken,
	}
}

// HandleWebSocket handles incoming WebSocket connections from agents
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	remoteAddr := r.RemoteAddr

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
	payloadBytes, err := jsonMarshal(msg.Payload)
	if err != nil {
		log.Error().Err(err).Str("remote_addr", remoteAddr).Msg("Failed to marshal registration payload")
		closeConn("Failed to close connection after registration payload marshal error")
		return
	}

	var reg AgentRegisterPayload
	if err := msg.DecodePayload(&reg); err != nil {
		log.Error().Err(err).Str("remote_addr", remoteAddr).Msg("Failed to parse registration payload")
		closeConn("Failed to close connection after registration payload parse error")
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

	// Send registration success (with write lock since agent is now in the map
	// and other goroutines could try to send commands via ExecuteCommand)
	registeredMsg, err := NewMessage(MsgTypeRegistered, "", RegisteredPayload{Success: true, Message: "Registered"})
	if err != nil {
		log.Warn().Err(err).Str("agent_id", reg.AgentID).Msg("Failed to encode registration ack")
		conn.Close()
		return
	}
	ac.writeMu.Lock()
	if err := s.sendMessage(conn, registeredMsg); err != nil {
		log.Warn().Err(err).Str("agent_id", reg.AgentID).Msg("Failed to send registration ack")
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
			payloadBytes, err := json.Marshal(msg.Payload)
			if err != nil {
				log.Error().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Failed to marshal command result payload")
				continue
			}
			var result CommandResultPayload
			if err := msg.DecodePayload(&result); err != nil {
				log.Error().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Failed to parse command result")
				continue
			}

			s.mu.RLock()
			ch, ok := s.pendingRequests[result.RequestID]
			s.mu.RUnlock()

			if ok {
				select {
				case ch <- result:
				default:
					log.Warn().
						Str("request_id", result.RequestID).
						Str("agent_id", ac.agent.AgentID).
						Msg("Result channel full, dropping")
				}
			} else {
				log.Warn().
					Str("request_id", result.RequestID).
					Str("agent_id", ac.agent.AgentID).
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

// sendRequestAndWait is a generic helper for sending a request to an agent and waiting for the result
func (s *Server) sendRequestAndWait(ctx context.Context, agentID string, msgType MessageType, requestID string, payload interface{}, timeout time.Duration) (*CommandResultPayload, error) {
	s.mu.RLock()
	ac, ok := s.agents[agentID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}

	// Create response channel
	respCh := make(chan CommandResultPayload, 1)
	s.mu.Lock()
	s.pendingRequests[requestID] = respCh
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pendingRequests, requestID)
		s.mu.Unlock()
	}()

	// Send request
	msg, err := NewMessage(msgType, requestID, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode %s request: %w", msgType, err)
	}

	ac.writeMu.Lock()
	err = s.sendMessage(ac.conn, msg)
	ac.writeMu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to send %s request: %w", msgType, err)
	}

	// Wait for result
	select {
	case result := <-respCh:
		return &result, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("%s timed out after %v", msgType, timeout)
	case <-ctx.Done():
		return nil, fmt.Errorf("execute command %q on agent %q canceled: %w", cmd.RequestID, agentID, ctx.Err())
	}
}

// ExecuteCommand sends a command to an agent and waits for the result
func (s *Server) ExecuteCommand(ctx context.Context, agentID string, cmd ExecuteCommandPayload) (*CommandResultPayload, error) {
	timeout := time.Duration(cmd.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return s.sendRequestAndWait(ctx, agentID, MsgTypeExecuteCmd, cmd.RequestID, cmd, timeout)
}

// ReadFile reads a file from an agent
func (s *Server) ReadFile(ctx context.Context, agentID string, req ReadFilePayload) (*CommandResultPayload, error) {
	return s.sendRequestAndWait(ctx, agentID, MsgTypeReadFile, req.RequestID, req, readFileTimeout)
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
