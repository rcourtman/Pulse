package updates

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// sseClientQueueSize bounds how many undelivered statuses a single slow
// client may hold before new ones are dropped for that client.
const sseClientQueueSize = 64

// SSEClient represents a Server-Sent Events client
type SSEClient struct {
	ID         string
	Writer     http.ResponseWriter
	Flusher    http.Flusher
	Done       chan bool
	LastActive time.Time
	mu         sync.Mutex // protects writes to Writer and Flusher
	// queue delivers broadcasts to this client in order through a single
	// writer goroutine. Nil for clients not registered via AddClient.
	queue chan UpdateStatus
}

// SSEBroadcaster manages Server-Sent Events connections for update progress
type SSEBroadcaster struct {
	mu               sync.RWMutex
	clients          map[string]*SSEClient
	messageChan      chan UpdateStatus
	cachedStatus     UpdateStatus
	cachedStatusTime time.Time
	statusMu         sync.RWMutex
	stopCh           chan struct{}
	closeOnce        sync.Once
	loopWg           sync.WaitGroup
	closed           atomic.Bool
}

// NewSSEBroadcaster creates a new SSE broadcaster
func NewSSEBroadcaster() *SSEBroadcaster {
	b := &SSEBroadcaster{
		clients:     make(map[string]*SSEClient),
		messageChan: make(chan UpdateStatus, 100),
		stopCh:      make(chan struct{}),
		cachedStatus: UpdateStatus{
			Status:    "idle",
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
		cachedStatusTime: time.Now(),
	}

	// Start the broadcast loop
	b.loopWg.Add(1)
	go b.broadcastLoop()

	// Start client cleanup loop
	b.loopWg.Add(1)
	go b.cleanupLoop()

	return b
}

// AddClient registers a new SSE client
func (b *SSEBroadcaster) AddClient(w http.ResponseWriter, clientID string) *SSEClient {
	if b.closed.Load() {
		return nil
	}

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
		queue:      make(chan UpdateStatus, sseClientQueueSize),
	}

	b.mu.Lock()
	if b.closed.Load() {
		b.mu.Unlock()
		return nil
	}
	b.clients[clientID] = client
	clientCount := len(b.clients)
	b.mu.Unlock()

	log.Info().
		Str("client_id", clientID).
		Int("total_clients", clientCount).
		Msg("SSE client connected")

	// Write the connection preamble and the current status synchronously,
	// before the ordered writer starts, so a (re)connecting client always
	// sees where the update is right now instead of waiting for the next
	// stage change. The cache is read after registration so a broadcast
	// racing this connect is either in the snapshot or queued behind it.
	b.statusMu.RLock()
	cachedStatus := b.cachedStatus
	b.statusMu.RUnlock()

	if data, err := json.Marshal(cachedStatus); err == nil {
		b.writeToClient(client, ": connected\n\n"+formatSSEData(data))
	} else {
		log.Error().Err(err).Str("client_id", clientID).Msg("Failed to marshal initial status for SSE")
		b.writeToClient(client, ": connected\n\n")
	}

	go b.clientWriteLoop(client)

	return client
}

// clientWriteLoop drains a client's queue so broadcasts reach it in the
// order they were emitted. A goroutine per message (the previous design)
// let a later stage overtake an earlier one on the wire.
func (b *SSEBroadcaster) clientWriteLoop(client *SSEClient) {
	for {
		select {
		case <-client.Done:
			return
		case status := <-client.queue:
			b.sendToClient(client, status)
		}
	}
}

// enqueueForClient hands a status to the client's ordered writer without
// blocking the broadcast loop on a slow connection.
func (b *SSEBroadcaster) enqueueForClient(client *SSEClient, status UpdateStatus) {
	if client.queue == nil {
		go b.sendToClient(client, status)
		return
	}
	select {
	case <-client.Done:
	case client.queue <- status:
	default:
		log.Warn().
			Str("client_id", client.ID).
			Str("status", status.Status).
			Msg("SSE client queue full, dropping update status for this client")
	}
}

// RemoveClient unregisters an SSE client
func (b *SSEBroadcaster) RemoveClient(clientID string) {
	b.mu.Lock()
	client, exists := b.clients[clientID]
	if exists {
		close(client.Done)
		delete(b.clients, clientID)
	}
	clientCount := len(b.clients)
	b.mu.Unlock()

	if exists {
		log.Info().
			Str("client_id", clientID).
			Int("total_clients", clientCount).
			Msg("SSE client disconnected")
	}
}

// Broadcast sends an update status to all connected clients
func (b *SSEBroadcaster) Broadcast(status UpdateStatus) {
	if b.closed.Load() {
		return
	}

	// Update cached status
	b.statusMu.Lock()
	b.cachedStatus = status
	b.cachedStatusTime = time.Now()
	b.statusMu.Unlock()

	// Send to message channel (non-blocking)
	select {
	case <-b.stopCh:
		return
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
	defer b.loopWg.Done()

	for {
		select {
		case <-b.stopCh:
			return
		case status := <-b.messageChan:
			b.mu.RLock()
			clients := make([]*SSEClient, 0, len(b.clients))
			for _, client := range b.clients {
				clients = append(clients, client)
			}
			b.mu.RUnlock()

			// Each client has its own ordered writer, so a slow client
			// neither blocks the others nor receives stages out of order.
			for _, client := range clients {
				b.enqueueForClient(client, status)
			}

			log.Debug().
				Str("status", status.Status).
				Int("progress", status.Progress).
				Int("clients", len(clients)).
				Msg("Broadcasted update status to SSE clients")
		}
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

	b.writeToClient(client, formatSSEData(data))
}

// formatSSEData frames a JSON payload as a single SSE data event.
func formatSSEData(data []byte) string {
	return fmt.Sprintf("data: %s\n\n", string(data))
}

// writeToClient writes a raw SSE frame and flushes it immediately. Every
// write to a client goes through here under the client lock: the
// ResponseWriter is not safe for concurrent use, and an unflushed frame is
// indistinguishable from a silent stream on the browser side.
func (b *SSEBroadcaster) writeToClient(client *SSEClient, frame string) {
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
			go b.RemoveClient(client.ID)
		}
	}()

	if _, err := fmt.Fprint(client.Writer, frame); err != nil {
		log.Debug().
			Err(err).
			Str("client_id", client.ID).
			Msg("Failed to write to SSE client, removing")
		// Removal takes the broadcaster lock; do it off this goroutine so a
		// concurrent cleanup pass holding that lock while waiting on this
		// client's lock cannot deadlock.
		go b.RemoveClient(client.ID)
		return
	}

	// Flush immediately
	client.Flusher.Flush()
	client.LastActive = time.Now()
}

// cleanupLoop periodically removes inactive clients
func (b *SSEBroadcaster) cleanupLoop() {
	defer b.loopWg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
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
}

// SendHeartbeat sends a heartbeat comment to all clients to keep connections alive
func (b *SSEBroadcaster) SendHeartbeat() {
	if b.closed.Load() {
		return
	}

	b.mu.RLock()
	clients := make([]*SSEClient, 0, len(b.clients))
	for _, client := range b.clients {
		clients = append(clients, client)
	}
	b.mu.RUnlock()

	for _, client := range clients {
		b.sendHeartbeatToClient(client)
	}
}

// Close closes the broadcaster and disconnects all clients
func (b *SSEBroadcaster) Close() {
	b.closeOnce.Do(func() {
		b.closed.Store(true)
		close(b.stopCh)

		b.mu.Lock()
		for id, client := range b.clients {
			closeDone(client.Done)
			delete(b.clients, id)
		}
		b.mu.Unlock()

		log.Info().Msg("SSE broadcaster closed")
	})
}

func (b *SSEBroadcaster) sendHeartbeatToClient(c *SSEClient) {
	select {
	case <-c.Done:
		return
	default:
	}

	b.writeToClient(c, ": heartbeat\n\n")
}

func closeDone(ch chan bool) {
	select {
	case <-ch:
	default:
		close(ch)
	}
}
