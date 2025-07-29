package websocket

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024 * 64,  // 64KB to handle large state messages
	WriteBufferSize: 1024 * 64,  // 64KB to handle large state messages
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implement proper origin checking based on config
		return true
	},
}

// Client represents a WebSocket client
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	id       string
	lastPing time.Time
}

// Hub maintains active WebSocket clients and broadcasts messages
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	getState   func() interface{} // Function to get current state
}

// Message represents a WebSocket message
type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
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
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		getState:   getState,
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
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
			if h.getState != nil {
				// Add a small delay to ensure client is ready
				go func() {
					time.Sleep(500 * time.Millisecond)
					
					// First send a small welcome message
					welcomeMsg := Message{
						Type: "welcome",
						Data: map[string]string{"message": "Connected to Pulse WebSocket"},
					}
					if data, err := json.Marshal(welcomeMsg); err == nil {
						log.Info().Str("client", client.id).Msg("Sending welcome message")
						select {
						case client.send <- data:
							log.Info().Str("client", client.id).Msg("Welcome message sent")
						default:
							log.Warn().Str("client", client.id).Msg("Failed to send welcome message")
						}
					}
					
					// Then send the initial state after another delay
					time.Sleep(100 * time.Millisecond)
					
					initialMsg := Message{
						Type: "initialState", 
						Data: sanitizeData(h.getState()),
					}
					if data, err := json.Marshal(initialMsg); err == nil {
						log.Info().Str("client", client.id).Int("dataLen", len(data)).Int("dataKB", len(data)/1024).Msg("Sending initial state to client")
						
						select {
						case client.send <- data:
							log.Info().Str("client", client.id).Msg("Initial state sent successfully")
						default:
							log.Warn().Str("client", client.id).Msg("Client send buffer full, skipping initial state")
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
				select {
				case client.send <- message:
				default:
					// Client's send channel is full, close it
					h.mu.Lock()
					delete(h.clients, client)
					close(client.send)
					h.mu.Unlock()
				}
			}

		case <-pingTicker.C:
			h.sendPing()
		}
	}
}

// HandleWebSocket handles WebSocket upgrade requests
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Info().
		Str("origin", r.Header.Get("Origin")).
		Str("host", r.Host).
		Str("userAgent", r.Header.Get("User-Agent")).
		Msg("WebSocket upgrade request")
		
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade WebSocket connection")
		return
	}

	clientID := generateClientID()
	client := &Client{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, 1024), // Increased buffer for high-frequency updates
		id:       clientID,
		lastPing: time.Now(),
	}

	log.Info().Str("client", clientID).Msg("WebSocket client created")

	client.hub.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// BroadcastState broadcasts state update to all clients
func (h *Hub) BroadcastState(state interface{}) {
	msg := Message{
		Type: "rawData",
		Data: state,
	}
	h.broadcastMessage(msg)
}

// BroadcastAlert broadcasts alert to all clients
func (h *Hub) BroadcastAlert(alert interface{}) {
	log.Info().Interface("alert", alert).Msg("Broadcasting alert to WebSocket clients")
	msg := Message{
		Type: "alert",
		Data: alert,
	}
	h.broadcastMessage(msg)
}

// BroadcastAlertResolved broadcasts alert resolution to all clients
func (h *Hub) BroadcastAlertResolved(alertID string) {
	log.Info().Str("alertID", alertID).Msg("Broadcasting alert resolved to WebSocket clients")
	msg := Message{
		Type: "alertResolved",
		Data: map[string]string{"alertId": alertID},
	}
	h.broadcastMessage(msg)
}

// GetClientCount returns the number of connected clients
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// broadcastMessage sends a message to all clients
func (h *Hub) broadcastMessage(msg Message) {
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
	h.broadcastMessage(msg)
}

// readPump handles incoming messages from the client
func (c *Client) readPump() {
	defer func() {
		log.Info().Str("client", c.id).Msg("ReadPump exiting")
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
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
				c.send <- data
			}
		case "requestData":
			// Send current state
			if c.hub.getState != nil {
				stateMsg := Message{
					Type: "rawData",
					Data: sanitizeData(c.hub.getState()),
				}
				if data, err := json.Marshal(stateMsg); err == nil {
					c.send <- data
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
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				log.Debug().Str("client", c.id).Msg("Send channel closed")
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
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
						return
					}
				default:
					// No more messages
				}
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// generateClientID generates a unique client ID
func generateClientID() string {
	return fmt.Sprintf("client-%d", time.Now().UnixNano())
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