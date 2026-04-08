// Package console provides a WebSocket-based console proxy that tunnels
// VNC, SPICE, SSH, and serial console connections through the Pulse server.
//
// This allows users to access VM consoles without needing direct network
// access to the hypervisor. The proxy establishes a backend connection to
// the hypervisor's console endpoint and bridges it with the frontend's
// WebSocket connection (noVNC for VNC, xterm.js for SSH/serial, etc.).
package console

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/hypervisor"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		// In production, validate origin against ALLOWED_ORIGINS config.
		return true
	},
}

// Session represents an active console connection.
type Session struct {
	ID         string              `json:"id"`
	VMID       string              `json:"vmId"`
	NodeID     string              `json:"nodeId"`
	ProviderID string              `json:"providerId"`
	Type       hypervisor.ConsoleType `json:"type"`
	CreatedAt  time.Time           `json:"createdAt"`
	Active     bool                `json:"active"`
}

// Proxy manages console sessions and WebSocket connections.
type Proxy struct {
	registry *hypervisor.Registry
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewProxy creates a new console proxy.
func NewProxy(registry *hypervisor.Registry) *Proxy {
	return &Proxy{
		registry: registry,
		sessions: make(map[string]*Session),
	}
}

// ConsoleTicketRequest is the JSON body for requesting a console ticket.
type ConsoleTicketRequest struct {
	ProviderID string `json:"providerId"`
	NodeID     string `json:"nodeId"`
	VMID       string `json:"vmId"`
	Type       string `json:"type"` // "vnc", "spice", "ssh", "serial"
}

// ConsoleTicketResponse is returned when a ticket is acquired.
type ConsoleTicketResponse struct {
	SessionID    string   `json:"sessionId"`
	Type         string   `json:"type"`
	WebSocketURL string   `json:"websocketUrl"` // Relative WebSocket URL for the console
	VMID         string   `json:"vmId"`
	NodeID       string   `json:"nodeId"`
	ProviderID   string   `json:"providerId"`
}

// HandleTicket acquires a console ticket from the provider and creates a session.
func (p *Proxy) HandleTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ConsoleTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate provider supports console.
	cp, ok := p.registry.GetConsoleProvider(req.ProviderID)
	if !ok {
		http.Error(w, "Provider not found or does not support console access", http.StatusNotFound)
		return
	}

	consoleType := hypervisor.ConsoleType(req.Type)

	// Acquire ticket from the hypervisor.
	ticket, err := cp.GetConsoleTicket(r.Context(), req.NodeID, req.VMID, consoleType)
	if err != nil {
		log.Error().Err(err).Str("provider", req.ProviderID).Str("vm", req.VMID).Msg("failed to acquire console ticket")
		http.Error(w, fmt.Sprintf("Failed to acquire console ticket: %v", err), http.StatusInternalServerError)
		return
	}

	// Create session.
	sessionID := fmt.Sprintf("console-%s-%s-%d", req.ProviderID, req.VMID, time.Now().UnixNano())
	session := &Session{
		ID:         sessionID,
		VMID:       req.VMID,
		NodeID:     req.NodeID,
		ProviderID: req.ProviderID,
		Type:       consoleType,
		CreatedAt:  time.Now(),
		Active:     true,
	}

	p.mu.Lock()
	p.sessions[sessionID] = session
	p.mu.Unlock()

	_ = ticket // Ticket info would be used for backend connection.

	resp := ConsoleTicketResponse{
		SessionID:    sessionID,
		Type:         req.Type,
		WebSocketURL: fmt.Sprintf("/api/console/ws/%s", sessionID),
		VMID:         req.VMID,
		NodeID:       req.NodeID,
		ProviderID:   req.ProviderID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleWebSocket upgrades the HTTP connection to WebSocket and proxies console data.
func (p *Proxy) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract session ID from URL path.
	sessionID := r.URL.Path[len("/api/console/ws/"):]
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	p.mu.RLock()
	session, ok := p.sessions[sessionID]
	p.mu.RUnlock()
	if !ok {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Get the console ticket for the backend connection.
	cp, ok := p.registry.GetConsoleProvider(session.ProviderID)
	if !ok {
		http.Error(w, "Provider not available", http.StatusServiceUnavailable)
		return
	}

	ticket, err := cp.GetConsoleTicket(r.Context(), session.NodeID, session.VMID, session.Type)
	if err != nil {
		http.Error(w, "Failed to get console ticket", http.StatusInternalServerError)
		return
	}

	// Upgrade to WebSocket.
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Str("session", sessionID).Msg("WebSocket upgrade failed")
		return
	}
	defer wsConn.Close()

	log.Info().
		Str("session", sessionID).
		Str("type", string(session.Type)).
		Str("vm", session.VMID).
		Msg("Console WebSocket connected")

	// Route to the appropriate console handler.
	switch session.Type {
	case hypervisor.ConsoleVNC:
		p.proxyVNC(r.Context(), wsConn, ticket)
	case hypervisor.ConsoleSSH:
		p.proxySSH(r.Context(), wsConn, ticket)
	case hypervisor.ConsoleSPICE:
		p.proxySPICE(r.Context(), wsConn, ticket)
	case hypervisor.ConsoleSerial:
		p.proxySerial(r.Context(), wsConn, ticket)
	default:
		wsConn.WriteMessage(websocket.TextMessage, []byte("Unsupported console type"))
	}

	// Clean up session.
	p.mu.Lock()
	delete(p.sessions, sessionID)
	p.mu.Unlock()

	log.Info().Str("session", sessionID).Msg("Console session ended")
}

// HandleConsoleTypes returns the supported console types for a specific VM.
func (p *Proxy) HandleConsoleTypes(w http.ResponseWriter, r *http.Request) {
	providerID := r.URL.Query().Get("providerId")
	if providerID == "" {
		http.Error(w, "providerId query parameter required", http.StatusBadRequest)
		return
	}

	cp, ok := p.registry.GetConsoleProvider(providerID)
	if !ok {
		// Provider doesn't support console.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string][]string{"types": {}})
		return
	}

	types := cp.SupportedConsoleTypes()
	typeStrings := make([]string, len(types))
	for i, t := range types {
		typeStrings[i] = string(t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"types": typeStrings})
}

// HandleListSessions returns active console sessions (admin endpoint).
func (p *Proxy) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	sessions := make([]Session, 0, len(p.sessions))
	for _, s := range p.sessions {
		sessions = append(sessions, *s)
	}
	p.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// proxyVNC bridges a WebSocket connection (noVNC frontend) to a VNC server.
func (p *Proxy) proxyVNC(ctx context.Context, ws *websocket.Conn, ticket *hypervisor.ConsoleTicket) {
	if ticket.Host == "" || ticket.Port == 0 {
		ws.WriteMessage(websocket.TextMessage, []byte("VNC connection info not available"))
		return
	}

	addr := fmt.Sprintf("%s:%d", ticket.Host, ticket.Port)
	log.Info().Str("addr", addr).Msg("Connecting to VNC server")

	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		log.Error().Err(err).Str("addr", addr).Msg("Failed to connect to VNC server")
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("VNC connection failed: %v", err)))
		return
	}
	defer conn.Close()

	// Bridge: WebSocket <-> TCP (VNC)
	bridgeWebSocketToTCP(ctx, ws, conn)
}

// proxySSH bridges a WebSocket connection (xterm.js frontend) to an SSH server.
func (p *Proxy) proxySSH(ctx context.Context, ws *websocket.Conn, ticket *hypervisor.ConsoleTicket) {
	if ticket.Host == "" {
		ws.WriteMessage(websocket.TextMessage, []byte("SSH connection info not available"))
		return
	}

	port := ticket.Port
	if port == 0 {
		port = 22
	}

	addr := fmt.Sprintf("%s:%d", ticket.Host, port)
	log.Info().Str("addr", addr).Msg("SSH console requested")

	// Full implementation would:
	// 1. Use golang.org/x/crypto/ssh to dial the SSH server
	// 2. Authenticate with the ticket credentials
	// 3. Open a session and request a PTY
	// 4. Bridge stdin/stdout over the WebSocket

	ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("SSH console connecting to %s...\r\n", addr)))
	ws.WriteMessage(websocket.TextMessage, []byte("SSH console proxy: full implementation requires golang.org/x/crypto/ssh\r\n"))
}

// proxySPICE bridges a WebSocket to a SPICE server.
func (p *Proxy) proxySPICE(ctx context.Context, ws *websocket.Conn, ticket *hypervisor.ConsoleTicket) {
	if ticket.Host == "" || ticket.Port == 0 {
		ws.WriteMessage(websocket.TextMessage, []byte("SPICE connection info not available"))
		return
	}

	addr := fmt.Sprintf("%s:%d", ticket.Host, ticket.Port)
	log.Info().Str("addr", addr).Msg("Connecting to SPICE server")

	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		log.Error().Err(err).Str("addr", addr).Msg("Failed to connect to SPICE server")
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("SPICE connection failed: %v", err)))
		return
	}
	defer conn.Close()

	bridgeWebSocketToTCP(ctx, ws, conn)
}

// proxySerial bridges a WebSocket to a serial console.
func (p *Proxy) proxySerial(ctx context.Context, ws *websocket.Conn, ticket *hypervisor.ConsoleTicket) {
	ws.WriteMessage(websocket.TextMessage, []byte("Serial console proxy: implementation pending\r\n"))
}

// bridgeWebSocketToTCP bidirectionally proxies data between a WebSocket and TCP connection.
func bridgeWebSocketToTCP(ctx context.Context, ws *websocket.Conn, tcp net.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// TCP -> WebSocket
	go func() {
		defer cancel()
		buf := make([]byte, 4096)
		for {
			n, err := tcp.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Debug().Err(err).Msg("TCP read error")
				}
				return
			}
			if err := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				log.Debug().Err(err).Msg("WebSocket write error")
				return
			}
		}
	}()

	// WebSocket -> TCP
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_, msg, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Debug().Err(err).Msg("WebSocket read error")
			}
			return
		}
		if _, err := tcp.Write(msg); err != nil {
			log.Debug().Err(err).Msg("TCP write error")
			return
		}
	}
}
