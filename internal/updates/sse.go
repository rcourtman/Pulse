package updates

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// SSEClient represents a Server-Sent Events client
type SSEClient struct {
	ID         string
	Writer     http.ResponseWriter
	Flusher    http.Flusher
	Done       chan bool
	LastActive time.Time
	mu         sync.Mutex // protects writes to Writer and Flusher
}

// SSEBroadcaster manages Server-Sent Events connections for update progress
type SSEBroadcaster struct {
	mu               sync.RWMutex
	clients          map[string]*SSEClient
	messageChan      chan UpdateStatus
	cachedStatus     UpdateStatus
	cachedStatusTime time.Time
	statusMu         sync.RWMutex
}

// NewSSEBroadcaster creates a new SSE broadcaster
func NewSSEBroadcaster() *SSEBroadcaster {
	b := &SSEBroadcaster{
		clients:     make(map[string]*SSEClient),
		messageChan: make(chan UpdateStatus, 100),
		cachedStatus: UpdateStatus{
			Status:    "idle",
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
		cachedStatusTime: time.Now(),
	}

	// Start the broadcast loop
	go b.broadcastLoop()

	// Start client cleanup loop
	go b.cleanupLoop()

	return b
}

// AddClient registers a new SSE client
func (b *SSEBroadcaster) AddClient(w http.ResponseWriter, clientID string) *SSEClient {
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Error().Msg("Streaming not supported by response writer")
		return nil
	}

	client := &SSEClient{
		ID:         clientID,
		Writer:     w,
		Flusher:    flusher,
		Done:       make(chan bool, 1),
		LastActive: time.Now(),
	}

	b.mu.Lock()
	b.clients[clientID] = client
	b.mu.Unlock()

	log.Info().
		Str("client_id", clientID).
		Int("total_clients", len(b.clients)).
		Msg("SSE client connected")

	// Send the current cached status immediately to the new client
	b.statusMu.RLock()
	cachedStatus := b.cachedStatus
	b.statusMu.RUnlock()

	go func() {
		b.sendToClient(client, cachedStatus)
	}()

	return client
}

// RemoveClient unregisters an SSE client
func (b *SSEBroadcaster) RemoveClient(clientID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if client, exists := b.clients[clientID]; exists {
		close(client.Done)
		delete(b.clients, clientID)
		log.Info().
			Str("client_id", clientID).
			Int("total_clients", len(b.clients)).
			Msg("SSE client disconnected")
	}
}

// Broadcast sends an update status to all connected clients
func (b *SSEBroadcaster) Broadcast(status UpdateStatus) {
	// Update cached status
	b.statusMu.Lock()
	b.cachedStatus = status
	b.cachedStatusTime = time.Now()
	b.statusMu.Unlock()

	// Send to message channel (non-blocking)
	select {
	case b.messageChan <- status:
	default:
		log.Warn().Msg("SSE message channel full, dropping message")
	}
}

// GetCachedStatus returns the last broadcasted status
func (b *SSEBroadcaster) GetCachedStatus() (UpdateStatus, time.Time) {
	b.statusMu.RLock()
	defer b.statusMu.RUnlock()
	return b.cachedStatus, b.cachedStatusTime
}

// GetClientCount returns the number of connected clients
func (b *SSEBroadcaster) GetClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// broadcastLoop continuously broadcasts messages to all clients
func (b *SSEBroadcaster) broadcastLoop() {
	for status := range b.messageChan {
		b.mu.RLock()
		clients := make([]*SSEClient, 0, len(b.clients))
		for _, client := range b.clients {
			clients = append(clients, client)
		}
		b.mu.RUnlock()

		// Send to all clients in parallel
		for _, client := range clients {
			go b.sendToClient(client, status)
		}

		log.Debug().
			Str("status", status.Status).
			Int("progress", status.Progress).
			Int("clients", len(clients)).
			Msg("Broadcasted update status to SSE clients")
	}
}

// sendToClient sends a message to a specific client
func (b *SSEBroadcaster) sendToClient(client *SSEClient, status UpdateStatus) {
	// Check if client is already disconnected
	select {
	case <-client.Done:
		return
	default:
	}

	// Marshal status to JSON
	data, err := json.Marshal(status)
	if err != nil {
		log.Error().Err(err).Str("client_id", client.ID).Msg("Failed to marshal status for SSE")
		return
	}

	// Write SSE message format
	// Format: data: {json}\n\n
	message := fmt.Sprintf("data: %s\n\n", string(data))

	// Acquire client lock to prevent concurrent writes
	client.mu.Lock()
	defer client.mu.Unlock()

	// Double-check client is not disconnected while waiting for lock
	select {
	case <-client.Done:
		return
	default:
	}

	defer func() {
		if r := recover(); r != nil {
			log.Warn().
				Str("client_id", client.ID).
				Interface("panic", r).
				Msg("Recovered from panic while sending to SSE client")
			b.RemoveClient(client.ID)
		}
	}()

	// Write to client
	_, err = fmt.Fprint(client.Writer, message)
	if err != nil {
		log.Debug().
			Err(err).
			Str("client_id", client.ID).
			Msg("Failed to write to SSE client, removing")
		b.RemoveClient(client.ID)
		return
	}

	// Flush immediately
	client.Flusher.Flush()
	client.LastActive = time.Now()
}

// cleanupLoop periodically removes inactive clients
func (b *SSEBroadcaster) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		b.mu.Lock()

		idsToRemove := make([]string, 0)
		for id, client := range b.clients {
			// Acquire client lock to safely read LastActive (written by sendToClient/SendHeartbeat)
			client.mu.Lock()
			inactive := now.Sub(client.LastActive) > 5*time.Minute
			client.mu.Unlock()

			// Remove clients inactive for more than 5 minutes
			if inactive {
				idsToRemove = append(idsToRemove, id)
			}
		}

		for _, id := range idsToRemove {
			if client, exists := b.clients[id]; exists {
				close(client.Done)
				delete(b.clients, id)
				log.Info().
					Str("client_id", id).
					Msg("Removed inactive SSE client")
			}
		}

		b.mu.Unlock()
	}
}

// SendHeartbeat sends a heartbeat comment to all clients to keep connections alive
func (b *SSEBroadcaster) SendHeartbeat() {
	b.mu.RLock()
	clients := make([]*SSEClient, 0, len(b.clients))
	for _, client := range b.clients {
		clients = append(clients, client)
	}
	b.mu.RUnlock()

	for _, client := range clients {
		go func(c *SSEClient) {
			select {
			case <-c.Done:
				return
			default:
			}

			// Acquire client lock to prevent concurrent writes
			c.mu.Lock()
			defer c.mu.Unlock()

			// Double-check client is not disconnected while waiting for lock
			select {
			case <-c.Done:
				return
			default:
			}

			defer func() {
				if r := recover(); r != nil {
					b.RemoveClient(c.ID)
				}
			}()

			// Send SSE comment as heartbeat
			_, err := fmt.Fprint(c.Writer, ": heartbeat\n\n")
			if err != nil {
				b.RemoveClient(c.ID)
				return
			}
			c.Flusher.Flush()
			// Update LastActive so cleanup loop doesn't prune active clients
			c.LastActive = time.Now()
		}(client)
	}
}

// Close closes the broadcaster and disconnects all clients
func (b *SSEBroadcaster) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for id, client := range b.clients {
		close(client.Done)
		delete(b.clients, id)
	}

	close(b.messageChan)
	log.Info().Msg("SSE broadcaster closed")
}
