package websocket

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// isValidPrivateOrigin checks if the origin is from a valid private network
func isValidPrivateOrigin(host string) bool {
	// Check localhost variations
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}

	// Check if it's a valid IP address
	ip := net.ParseIP(host)
	if ip != nil {
		// Check if it's a private IP
		return ip.IsLoopback() || ip.IsPrivate()
	}

	// Allow common local domain patterns but be more restrictive
	// Only allow if it's clearly a local domain
	if strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".lan") {
		// But not arbitrary subdomains that could be malicious
		parts := strings.Split(host, ".")
		if len(parts) <= 3 { // hostname.local or hostname.subdomain.local
			return true
		}
	}

	return false
}

// normalizeForwardedProto coerces forwarded proto values into the HTTP scheme space so that
// websocket upgrades coming through proxies that emit ws/wss continue to compare equal to the
// browser-sent Origin header (which is always http/https).
func normalizeForwardedProto(proto string, fallback string) string {
	if proto == "" {
		return fallback
	}

	// Some proxies send comma-separated proto chains; take the first hop.
	if comma := strings.IndexByte(proto, ','); comma != -1 {
		proto = proto[:comma]
	}

	cleaned := strings.TrimSpace(strings.ToLower(proto))
	switch cleaned {
	case "wss":
		return "https"
	case "ws":
		return "http"
	case "https", "http":
		return cleaned
	default:
		if cleaned != "" {
			return cleaned
		}
		return fallback
	}
}

// SetAllowedOrigins sets the allowed origins for CORS
func (h *Hub) SetAllowedOrigins(origins []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.allowedOrigins = origins
}

// checkOrigin validates the origin against allowed origins
func (h *Hub) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// No origin header, allow for non-browser clients
		return true
	}

	h.mu.RLock()
	allowedOrigins := h.allowedOrigins
	h.mu.RUnlock()

	// Determine the actual origin based on proxy headers
	scheme := "http"
	host := r.Host

	// Check if we're behind a reverse proxy
	if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
		scheme = normalizeForwardedProto(forwardedProto, scheme)
	} else if forwardedScheme := r.Header.Get("X-Forwarded-Scheme"); forwardedScheme != "" {
		scheme = normalizeForwardedProto(forwardedScheme, scheme)
	} else if r.TLS != nil {
		scheme = "https"
	}

	// Use X-Forwarded-Host if available (for reverse proxy scenarios)
	if forwardedHost := r.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
		host = forwardedHost
	}

	requestOrigin := scheme + "://" + host

	// Allow same-origin requests (accounting for proxy headers)
	if origin == requestOrigin {
		return true
	}

	// Check if wildcard is allowed
	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			return true
		}
		if allowed == origin {
			return true
		}
	}

	// If no origins configured, only allow from truly private networks
	if len(allowedOrigins) == 0 {
		// Parse the origin URL to validate it properly
		originHost := origin
		if strings.HasPrefix(origin, "http://") {
			originHost = strings.TrimPrefix(origin, "http://")
		} else if strings.HasPrefix(origin, "https://") {
			originHost = strings.TrimPrefix(origin, "https://")
		}

		// Extract just the hostname/IP part (remove port)
		if colonIdx := strings.IndexByte(originHost, ':'); colonIdx != -1 {
			originHost = originHost[:colonIdx]
		}

		// Check if it's a valid private IP or localhost
		if isValidPrivateOrigin(originHost) {
			log.Debug().
				Str("origin", origin).
				Str("host", originHost).
				Msg("Allowing WebSocket connection from private network")
			return true
		}

		// Note: same-origin match already handled above (line 116)
		log.Warn().
			Str("origin", origin).
			Str("requestOrigin", requestOrigin).
			Msg("WebSocket connection rejected - not from allowed local/private network")
		return false
	}

	log.Warn().
		Str("origin", origin).
		Str("requestOrigin", requestOrigin).
		Strs("allowedOrigins", allowedOrigins).
		Msg("WebSocket connection rejected due to CORS")

	return false
}

// Client represents a WebSocket client
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	id       string
	lastPing time.Time
	closed   atomic.Bool // Set when the client is unregistered; prevents sends to closed channel
}

// safeSend attempts to send data to the client's send channel.
// Returns false if the client is closed or the channel buffer is full.
func (c *Client) safeSend(data []byte) bool {
	if c.closed.Load() {
		return false
	}
	select {
	case c.send <- data:
		return true
	default:
		return false
	}
}

// cloneAlertData returns a broadcast-safe copy of alert data to avoid data races when
// downstream sanitization/encoding happens concurrently with alert manager mutations.
func cloneAlertData(alert interface{}) interface{} {
	switch a := alert.(type) {
	case *alerts.Alert:
		cloned := cloneAlert(a)
		return cloned
	case alerts.Alert:
		cloned := cloneAlert(&a)
		return cloned
	default:
		return alert
	}
}

// cloneAlert performs a deep copy of the mutable fields within alerts.Alert.
func cloneAlert(src *alerts.Alert) alerts.Alert {
	if src == nil {
		return alerts.Alert{}
	}
	clone := *src

	if src.AckTime != nil {
		t := *src.AckTime
		clone.AckTime = &t
	}

	if len(src.EscalationTimes) > 0 {
		clone.EscalationTimes = append([]time.Time(nil), src.EscalationTimes...)
	}

	if src.Metadata != nil {
		clone.Metadata = cloneMetadata(src.Metadata)
	}

	return clone
}

// cloneMetadata creates a deep copy of alert metadata to detach from shared maps/slices.
func cloneMetadata(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}

	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = cloneMetadataValue(v)
	}
	return dst
}

func cloneMetadataValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return cloneMetadata(v)
	case map[string]string:
		m := make(map[string]interface{}, len(v))
		for key, val := range v {
			m[key] = val
		}
		return m
	case []interface{}:
		arr := make([]interface{}, len(v))
		for i, elem := range v {
			arr[i] = cloneMetadataValue(elem)
		}
		return arr
	case []string:
		arr := make([]string, len(v))
		copy(arr, v)
		return arr
	case []int:
		arr := make([]int, len(v))
		copy(arr, v)
		return arr
	case []float64:
		arr := make([]float64, len(v))
		copy(arr, v)
		return arr
	default:
		return v
	}
}

// Hub maintains active WebSocket clients and broadcasts messages
type Hub struct {
	clients        map[*Client]bool
	broadcast      chan []byte
	broadcastSeq   chan Message // Sequenced broadcast channel for ordering
	register       chan *Client
	unregister     chan *Client
	stopChan       chan struct{} // Signals shutdown
	mu             sync.RWMutex
	getState       func() interface{} // Function to get current state
	allowedOrigins []string           // Allowed origins for CORS
	// Broadcast coalescing fields
	coalesceWindow  time.Duration
	coalescePending *Message
	coalesceTimer   *time.Timer
	coalesceMutex   sync.Mutex
}

// Message represents a WebSocket message
type Message struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp,omitempty"`
}

// SetStateGetter sets the state getter function
func (h *Hub) SetStateGetter(getState func() interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.getState = getState
}

// NewHub creates a new WebSocket hub
func NewHub(getState func() interface{}) *Hub {
	return &Hub{
		clients:        make(map[*Client]bool),
		broadcast:      make(chan []byte, 256),
		broadcastSeq:   make(chan Message, 256), // Buffered sequenced channel
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		stopChan:       make(chan struct{}),
		getState:       getState,
		allowedOrigins: []string{},             // Default to empty (will be set based on actual host)
		coalesceWindow: 100 * time.Millisecond, // Coalesce rapid updates within 100ms
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	// Start broadcast sequencer goroutine
	go h.runBroadcastSequencer()

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Info().Str("client", client.id).Msg("WebSocket client connected")

			// Send initial state to the new client immediately
			log.Debug().Bool("hasGetState", h.getState != nil).Msg("Checking getState function for new client")
			if h.getState != nil {
				// Add a small delay to ensure client is ready
				go func() {
					log.Debug().Str("client", client.id).Msg("Starting initial state goroutine")
					time.Sleep(500 * time.Millisecond)

					// First send a small welcome message
					welcomeMsg := Message{
						Type: "welcome",
						Data: map[string]string{"message": "Connected to Pulse WebSocket"},
					}
					if data, err := json.Marshal(welcomeMsg); err == nil {
						// Check if client is still registered before sending (must hold lock)
						h.mu.RLock()
						_, stillRegistered := h.clients[client]
						h.mu.RUnlock()

						if stillRegistered {
							log.Info().Str("client", client.id).Msg("Sending welcome message")
							if client.safeSend(data) {
								log.Info().Str("client", client.id).Msg("Welcome message sent")
							} else {
								log.Warn().Str("client", client.id).Msg("Failed to send welcome message - client closed or buffer full")
							}
						} else {
							log.Debug().Str("client", client.id).Msg("Client disconnected before welcome message")
						}
					}

					// Then send the initial state after another delay
					time.Sleep(100 * time.Millisecond)
					log.Debug().Str("client", client.id).Msg("About to get state")

					// Get the state
					stateData := h.getState()
					log.Debug().Str("client", client.id).Interface("stateType", fmt.Sprintf("%T", stateData)).Msg("Got state for initial message")

					initialMsg := Message{
						Type: "initialState",
						Data: sanitizeData(stateData),
					}
					if data, err := json.Marshal(initialMsg); err == nil {
						// Check if client is still registered before sending (must hold lock)
						h.mu.RLock()
						_, stillRegistered := h.clients[client]
						h.mu.RUnlock()

						if stillRegistered {
							log.Info().Str("client", client.id).Int("dataLen", len(data)).Int("dataKB", len(data)/1024).Msg("Sending initial state to client")
							if client.safeSend(data) {
								log.Info().Str("client", client.id).Msg("Initial state sent successfully")
							} else {
								log.Warn().Str("client", client.id).Msg("Client closed or buffer full, skipping initial state")
							}
						} else {
							log.Debug().Str("client", client.id).Msg("Client disconnected before initial state")
						}
					} else {
						log.Error().Err(err).Str("client", client.id).Msg("Failed to marshal initial state")
					}
				}()
			} else {
				log.Warn().Msg("No getState function defined")
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.closed.Store(true) // Mark closed before closing channel to prevent sends
				close(client.send)
				h.mu.Unlock()
				log.Info().Str("client", client.id).Msg("WebSocket client disconnected")
			} else {
				h.mu.Unlock()
			}

		case message := <-h.broadcast:
			h.mu.RLock()
			clients := make([]*Client, 0, len(h.clients))
			for client := range h.clients {
				clients = append(clients, client)
			}
			h.mu.RUnlock()

			for _, client := range clients {
				if !client.safeSend(message) {
					// Client closed or buffer full - remove if still registered
					h.mu.Lock()
					if _, stillPresent := h.clients[client]; stillPresent {
						delete(h.clients, client)
						client.closed.Store(true)
						close(client.send)
					}
					h.mu.Unlock()
				}
			}

		case <-pingTicker.C:
			h.sendPing()

		case <-h.stopChan:
			log.Info().Msg("WebSocket hub shutting down")
			// Close all client connections
			h.mu.Lock()
			for client := range h.clients {
				client.closed.Store(true)
				close(client.send)
			}
			h.clients = make(map[*Client]bool)
			h.mu.Unlock()
			return
		}
	}
}

// Stop gracefully shuts down the hub
func (h *Hub) Stop() {
	close(h.stopChan)
}

// HandleWebSocket handles WebSocket upgrade requests
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Info().
		Str("origin", r.Header.Get("Origin")).
		Str("host", r.Host).
		Str("userAgent", r.Header.Get("User-Agent")).
		Msg("WebSocket upgrade request")

	// Create upgrader with our origin check
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024 * 1024 * 4, // 4MB to handle large state messages
		WriteBufferSize: 1024 * 1024 * 4, // 4MB to handle large state messages
		CheckOrigin:     h.checkOrigin,
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade WebSocket connection")
		return
	}

	clientID := utils.GenerateID("client")
	client := &Client{
		hub:  h,
		conn: conn,
		// Keep buffer bounded to avoid holding large state snapshots indefinitely if a client stalls.
		send:     make(chan []byte, 128),
		id:       clientID,
		lastPing: time.Now(),
	}

	log.Info().Str("client", clientID).Msg("WebSocket client created")

	client.hub.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// dispatchToClients fan-outs a marshaled payload to all clients, dropping any that
// cannot keep up to prevent unbounded buffering.
func (h *Hub) dispatchToClients(data []byte, dropLog string) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		if !client.safeSend(data) {
			// Client closed or buffer full - remove if still registered
			h.mu.Lock()
			if _, stillPresent := h.clients[client]; stillPresent {
				delete(h.clients, client)
				client.closed.Store(true)
				close(client.send)
				log.Warn().Str("client", client.id).Msg(dropLog)
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) popCoalescedMessage() *Message {
	h.coalesceMutex.Lock()
	defer h.coalesceMutex.Unlock()

	if h.coalescePending == nil {
		return nil
	}

	msg := *h.coalescePending
	h.coalescePending = nil
	h.coalesceTimer = nil
	return &msg
}

// runBroadcastSequencer handles sequenced broadcasts with coalescing for rapid state updates
func (h *Hub) runBroadcastSequencer() {
	for {
		select {
		case msg := <-h.broadcastSeq:
			// Handle raw data (state) messages with coalescing
			if msg.Type == "rawData" {
				h.coalesceMutex.Lock()

				// Cancel pending timer if exists
				if h.coalesceTimer != nil {
					h.coalesceTimer.Stop()
				}

				// Update pending message
				current := msg
				h.coalescePending = &current

				// Set timer to send after coalesce window
				h.coalesceTimer = time.AfterFunc(h.coalesceWindow, func() {
					pending := h.popCoalescedMessage()
					if pending != nil {
						if data, err := json.Marshal(*pending); err == nil {
							h.dispatchToClients(data, "Client send channel full, dropping coalesced message and closing connection")
						}
					}
				})

				h.coalesceMutex.Unlock()
			} else {
				// Non-state messages (alerts, etc.) - send immediately
				if data, err := json.Marshal(msg); err == nil {
					h.dispatchToClients(data, "Client send channel full, dropping message and closing connection")
				}
			}

		case <-h.stopChan:
			log.Debug().Msg("Broadcast sequencer shutting down")
			// Cancel pending timer if exists
			h.coalesceMutex.Lock()
			if h.coalesceTimer != nil {
				h.coalesceTimer.Stop()
			}
			h.coalesceMutex.Unlock()
			return
		}
	}
}

// BroadcastState broadcasts state update to all clients via sequencer
func (h *Hub) BroadcastState(state interface{}) {
	// Debug log to track docker hosts
	dockerHostsCount := -1
	// Use reflection to get dockerHosts field from any struct type
	v := reflect.ValueOf(state)
	if v.Kind() == reflect.Struct {
		field := v.FieldByName("DockerHosts")
		if field.IsValid() && field.Kind() == reflect.Slice {
			dockerHostsCount = field.Len()
		}
	}
	log.Debug().Int("dockerHostsCount", dockerHostsCount).Msg("Broadcasting state")

	msg := Message{
		Type: "rawData",
		Data: state,
	}

	// Send through sequencer for ordering and coalescing
	select {
	case h.broadcastSeq <- msg:
	default:
		log.Warn().Msg("Broadcast sequencer channel full, dropping state update")
	}
}

// BroadcastAlert broadcasts alert to all clients
func (h *Hub) BroadcastAlert(alert interface{}) {
	log.Info().Interface("alert", alert).Msg("Broadcasting alert to WebSocket clients")
	msg := Message{
		Type: "alert",
		Data: cloneAlertData(alert),
	}
	h.BroadcastMessage(msg)
}

// BroadcastAlertResolved broadcasts alert resolution to all clients
func (h *Hub) BroadcastAlertResolved(alertID string) {
	log.Info().Str("alertID", alertID).Msg("Broadcasting alert resolved to WebSocket clients")
	msg := Message{
		Type: "alertResolved",
		Data: map[string]string{"alertId": alertID},
	}
	h.BroadcastMessage(msg)
}

// GetClientCount returns the number of connected clients
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Broadcast sends a custom message to all connected clients
func (h *Hub) Broadcast(data interface{}) {
	h.BroadcastMessage(Message{
		Type:      "custom",
		Data:      data,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// BroadcastMessage sends a message to all clients
func (h *Hub) BroadcastMessage(msg Message) {
	// Sanitize the message data to handle NaN values
	msg.Data = sanitizeData(msg.Data)

	data, err := json.Marshal(msg)
	if err != nil {
		log.Error().Err(err).Str("type", msg.Type).Msg("Failed to marshal WebSocket message")
		// Try to marshal without data to see what's failing
		debugMsg := Message{Type: msg.Type, Data: "[error marshaling data]"}
		if debugData, debugErr := json.Marshal(debugMsg); debugErr == nil {
			log.Debug().Str("debugMsg", string(debugData)).Msg("Debug message")
		}
		return
	}

	select {
	case h.broadcast <- data:
	default:
		log.Warn().Msg("WebSocket broadcast channel full")
	}
}

// sendPing sends a ping message to all clients
func (h *Hub) sendPing() {
	msg := Message{
		Type: "ping",
		Data: map[string]int64{"timestamp": time.Now().Unix()},
	}
	h.BroadcastMessage(msg)
}

// readPump handles incoming messages from the client
func (c *Client) readPump() {
	defer func() {
		log.Info().Str("client", c.id).Msg("ReadPump exiting")
		c.hub.unregister <- c
		c.conn.Close()
	}()

	if err := c.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		log.Warn().Err(err).Str("client", c.id).Msg("Failed to set initial read deadline")
	}
	c.conn.SetPongHandler(func(string) error {
		if err := c.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			log.Warn().Err(err).Str("client", c.id).Msg("Failed to refresh read deadline on pong")
		}
		c.lastPing = time.Now()
		return nil
	})

	log.Info().Str("client", c.id).Msg("ReadPump started")

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error().Err(err).Str("client", c.id).Msg("WebSocket read error")
			} else {
				log.Info().Err(err).Str("client", c.id).Msg("WebSocket closed")
			}
			break
		}

		// Handle incoming messages
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Error().Err(err).Str("client", c.id).Msg("Failed to unmarshal WebSocket message")
			continue
		}

		// Handle different message types
		switch msg.Type {
		case "ping":
			// Respond with pong
			pong := Message{
				Type: "pong",
				Data: map[string]int64{"timestamp": time.Now().Unix()},
			}
			if data, err := json.Marshal(pong); err == nil {
				c.safeSend(data)
			}
		case "requestData":
			// Send current state
			if c.hub.getState != nil {
				stateMsg := Message{
					Type: "rawData",
					Data: sanitizeData(c.hub.getState()),
				}
				if data, err := json.Marshal(stateMsg); err == nil {
					c.safeSend(data)
				} else {
					log.Error().Err(err).Msg("Failed to marshal state for requestData")
				}
			}
		default:
			log.Debug().Str("client", c.id).Str("type", msg.Type).Msg("Received WebSocket message")
		}
	}
}

// writePump handles outgoing messages to the client
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		log.Info().Str("client", c.id).Msg("WritePump exiting")
		ticker.Stop()
		c.conn.Close()
	}()

	log.Info().Str("client", c.id).Msg("WritePump started")

	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				log.Warn().Err(err).Str("client", c.id).Msg("Failed to set write deadline before message send")
			}
			if !ok {
				log.Debug().Str("client", c.id).Msg("Send channel closed")
				if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					log.Warn().Err(err).Str("client", c.id).Msg("Failed to send close message")
				}
				return
			}

			// Send the primary message
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Error().Err(err).Str("client", c.id).Msg("Failed to write message")
				return
			}

			// Send any queued messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				select {
				case msg := <-c.send:
					if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
						log.Error().Err(err).Str("client", c.id).Msg("Failed to flush queued message")
						return
					}
				default:
					// No more messages
				}
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				log.Warn().Err(err).Str("client", c.id).Msg("Failed to set write deadline for ping")
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Debug().Err(err).Str("client", c.id).Msg("Failed to send ping; closing connection")
				return
			}
		}
	}
}

// sanitizeData recursively sanitizes data to replace NaN/Inf values with nil
func sanitizeData(data interface{}) interface{} {
	// First, marshal to JSON to convert structs to maps
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return data
	}

	var jsonData interface{}
	if err := json.Unmarshal(jsonBytes, &jsonData); err != nil {
		return data
	}

	return sanitizeValue(jsonData)
}

// sanitizeValue recursively sanitizes JSON-compatible values
func sanitizeValue(data interface{}) interface{} {
	switch v := data.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil
		}
		return v
	case float32:
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return nil
		}
		return v
	case map[string]interface{}:
		sanitized := make(map[string]interface{})
		for k, val := range v {
			sanitized[k] = sanitizeValue(val)
		}
		return sanitized
	case []interface{}:
		sanitized := make([]interface{}, len(v))
		for i, val := range v {
			sanitized[i] = sanitizeValue(val)
		}
		return sanitized
	default:
		// For all other types (string, bool, nil, etc.), return as-is
		return v
	}
}
