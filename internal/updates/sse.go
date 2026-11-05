package updates

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// sseClientQueueSize bounds how many undelivered frames a single slow
// client may hold before new ones are dropped for that client.
const sseClientQueueSize = 64

// sseClientInactivityTimeout retires clients whose writer has not completed
// a write for this long.
const sseClientInactivityTimeout = 5 * time.Minute

// SSEClient represents a Server-Sent Events client.
//
// Lifecycle: AddClient registers the client under an ID no other live or
// retired client holds, and starts its single ordered writer. Any path may
// retire it (the owning handler, an inactivity sweep, a write failure or
// panic, or Close): retirement closes Done and moves the client to the
// broadcaster's retired set without waiting for anything, then expires a
// write still in progress so a stalled peer cannot keep it alive. Only the
// owning handler's RemoveClient waits, outside every broadcaster lock, until
// the writer has stopped, so no write or flush can reach the ResponseWriter
// after the handler returns.
type SSEClient struct {
	ID      string
	Writer  http.ResponseWriter
	Flusher http.Flusher
	Done    chan bool
	// LastActive is the connection time. Later activity is tracked in
	// lastActive so the inactivity sweep never waits on a client's writer.
	LastActive time.Time
	mu         sync.Mutex // serializes writes to Writer and Flusher
	// queue delivers statuses and heartbeats to this client in order through
	// a single writer goroutine. Nil for clients not registered via AddClient.
	queue chan sseFrame
	// stopped is closed once the writer goroutine has exited for good. Nil
	// for clients not registered via AddClient.
	stopped    chan struct{}
	writing    atomic.Bool  // set, under mu, before Done is checked for a write
	lastActive atomic.Int64 // unix nanoseconds of the last completed write
	// lifeMu orders write interruption against the owner's join: once the
	// owner has collected the client, nothing touches its ResponseWriter.
	lifeMu    sync.Mutex
	collected bool
}

// interruptWrite expires a write in progress through the ResponseWriter's
// write deadline, unless the owning handler has already collected the
// client. Callers close Done first, so a writer that has not yet published
// a write will see Done and skip it.
func (c *SSEClient) interruptWrite() {
	c.lifeMu.Lock()
	defer c.lifeMu.Unlock()
	if c.collected || !c.writing.Load() {
		return
	}
	err := http.NewResponseController(c.Writer).SetWriteDeadline(time.Now())
	if err != nil && !errors.Is(err, http.ErrNotSupported) {
		log.Debug().Err(err).Str("client_id", c.ID).Msg("Could not interrupt SSE client write")
	}
}

// sseFrame is one unit of a client's ordered stream.
type sseFrame struct {
	status    UpdateStatus
	heartbeat bool
}

func (c *SSEClient) lastActivity() time.Time {
	if ns := c.lastActive.Load(); ns != 0 {
		return time.Unix(0, ns)
	}
	return c.LastActive
}

// SSEBroadcaster manages Server-Sent Events connections for update progress
type SSEBroadcaster struct {
	mu      sync.RWMutex
	clients map[string]*SSEClient
	// retired holds clients whose Done is closed until their owning handler
	// has collected them, so the owner can still find and join the writer and
	// the ID stays reserved until that owner's RemoveClient returns.
	retired          map[string]*SSEClient
	messageChan      chan sseFrame // statuses and heartbeats, in emission order
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
		retired:     make(map[string]*SSEClient),
		messageChan: make(chan sseFrame, 100),
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

	now := time.Now()
	client := &SSEClient{
		ID:         clientID,
		Writer:     w,
		Flusher:    flusher,
		Done:       make(chan bool, 1),
		LastActive: now,
		queue:      make(chan sseFrame, sseClientQueueSize),
		stopped:    make(chan struct{}),
	}
	client.lastActive.Store(now.UnixNano())

	b.mu.Lock()
	if b.closed.Load() {
		b.mu.Unlock()
		return nil
	}
	if _, live := b.clients[clientID]; live || b.retired[clientID] != nil {
		// Owners remove clients by ID, so a second client under the same ID
		// could be removed in place of the first and leave its writer
		// running past its handler. Refuse it; the browser reconnects.
		b.mu.Unlock()
		log.Warn().Str("client_id", clientID).Msg("Rejected SSE client with an ID already in use")
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
	// racing this connect is never lost: it is in the snapshot, queued
	// behind it, or both. In the last case the client receives that status
	// again after the snapshot, possibly preceded by an older stage that was
	// still in flight. Stage progress only rises, and the update modal
	// ignores stages that move it backwards, so the repeat is harmless.
	b.statusMu.RLock()
	cachedStatus := b.cachedStatus
	b.statusMu.RUnlock()

	preamble := ": connected\n\n"
	if data, err := json.Marshal(cachedStatus); err == nil {
		preamble += formatSSEData(data)
	} else {
		log.Error().Err(err).Str("client_id", clientID).Msg("Failed to marshal initial status for SSE")
	}
	// The preamble is written on the handler's own goroutine, before the
	// ordered writer starts. A failure retires the client; the writer then
	// exits at once and the handler's RemoveClient returns promptly.
	if err := b.writeToClient(client, preamble); err != nil {
		b.retireClient(client)
	}

	go b.clientWriteLoop(client)

	return client
}

// clientWriteLoop drains a client's queue so statuses and heartbeats reach it
// in the order they were emitted. A goroutine per message (the previous
// design) let a later stage overtake an earlier one on the wire. It is the
// only goroutine that writes to the client after AddClient returns.
func (b *SSEBroadcaster) clientWriteLoop(client *SSEClient) {
	defer b.writerStopped(client)
	for {
		select {
		case <-client.Done:
			return
		case frame := <-client.queue:
			if err := b.writeFrame(client, frame); err != nil {
				b.retireClient(client)
			}
		}
	}
}

// writerStopped records that the client's writer has exited, which is what
// RemoveClient waits for. It does not release the client's ID: only the
// owner's collection does, so a delayed owner cannot later remove a newer
// client admitted under the same ID.
func (b *SSEBroadcaster) writerStopped(client *SSEClient) {
	close(client.stopped)
}

// enqueueFrame hands a frame to the client's ordered writer without
// blocking the broadcast loop on a slow connection.
func (b *SSEBroadcaster) enqueueFrame(client *SSEClient, frame sseFrame) {
	if client.queue == nil {
		// Clients not registered via AddClient have no ordered writer;
		// SendHeartbeat writes their heartbeats directly.
		if !frame.heartbeat {
			go b.sendToClient(client, frame.status)
		}
		return
	}
	select {
	case <-client.Done:
	case client.queue <- frame:
	default:
		if frame.heartbeat {
			return // the client already has undelivered frames
		}
		log.Warn().
			Str("client_id", client.ID).
			Str("status", frame.status.Status).
			Msg("SSE client queue full, dropping update status for this client")
	}
}

// retireLocked closes the client's Done and moves a live client from the
// registry to the retired set, where its ID stays reserved until the owner
// collects it. It never waits on the client's writer, so it is safe under
// b.mu. b.mu must be held.
func (b *SSEBroadcaster) retireLocked(client *SSEClient) {
	closeDone(client.Done)
	if b.clients[client.ID] != client {
		return
	}
	delete(b.clients, client.ID)
	if client.stopped != nil {
		b.retired[client.ID] = client
	}
}

func (b *SSEBroadcaster) retireClient(client *SSEClient) {
	b.mu.Lock()
	b.retireLocked(client)
	b.mu.Unlock()
}

// RemoveClient is the owning handler's removal of its client. It retires the
// client if nothing else has, then waits until the client's writer has
// stopped, so the caller may return its ResponseWriter to net/http. A write
// still in flight is interrupted with an immediate write deadline where the
// ResponseWriter supports one (net/http responses and Pulse's middleware
// writers forward it); otherwise the wait lasts until that write returns.
// The wait happens outside every broadcaster lock, and concurrent removals
// of the same client all wait. Call it only from the handler that owns the
// client's ResponseWriter.
func (b *SSEBroadcaster) RemoveClient(clientID string) {
	b.mu.Lock()
	client, live := b.clients[clientID]
	if live {
		b.retireLocked(client)
	} else {
		client = b.retired[clientID]
	}
	clientCount := len(b.clients)
	b.mu.Unlock()

	if live {
		log.Info().
			Str("client_id", clientID).
			Int("total_clients", clientCount).
			Msg("SSE client disconnected")
	}
	if client != nil {
		b.quiesce(client)
	}
}

// quiesce waits until a retired client's writer has stopped for good, marks
// the client collected so no later interruption touches its writer, and
// only then releases its ID. The registry lock is held just for that
// release, never while waiting on the writer.
func (b *SSEBroadcaster) quiesce(client *SSEClient) {
	if client.stopped == nil {
		return
	}
	// Done is already closed, so a writer that has not yet published that it
	// is writing will observe Done and skip the write. One that has may be
	// blocked on a slow or stalled peer: expire its deadline.
	client.interruptWrite()
	<-client.stopped
	client.lifeMu.Lock()
	client.collected = true
	client.lifeMu.Unlock()

	b.mu.Lock()
	if b.retired[client.ID] == client {
		delete(b.retired, client.ID)
	}
	b.mu.Unlock()
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
	case b.messageChan <- sseFrame{status: status}:
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
		case frame := <-b.messageChan:
			b.mu.RLock()
			clients := make([]*SSEClient, 0, len(b.clients))
			for _, client := range b.clients {
				clients = append(clients, client)
			}
			b.mu.RUnlock()

			// Each client has its own ordered writer, so a slow client
			// neither blocks the others nor receives stages out of order.
			for _, client := range clients {
				b.enqueueFrame(client, frame)
			}

			if !frame.heartbeat {
				log.Debug().
					Str("status", frame.status.Status).
					Int("progress", frame.status.Progress).
					Int("clients", len(clients)).
					Msg("Broadcasted update status to SSE clients")
			}
		}
	}
}

// sendToClient sends a message to a specific client
func (b *SSEBroadcaster) sendToClient(client *SSEClient, status UpdateStatus) {
	if err := b.writeFrame(client, sseFrame{status: status}); err != nil {
		b.retireClient(client)
	}
}

// writeFrame writes one stream frame. It returns an error only when the
// ResponseWriter failed, in which case the caller retires the client.
func (b *SSEBroadcaster) writeFrame(client *SSEClient, frame sseFrame) error {
	select {
	case <-client.Done:
		return nil
	default:
	}
	if frame.heartbeat {
		return b.writeToClient(client, ": heartbeat\n\n")
	}
	data, err := json.Marshal(frame.status)
	if err != nil {
		log.Error().Err(err).Str("client_id", client.ID).Msg("Failed to marshal status for SSE")
		return nil
	}
	return b.writeToClient(client, formatSSEData(data))
}

// formatSSEData frames a JSON payload as a single SSE data event.
func formatSSEData(data []byte) string {
	return fmt.Sprintf("data: %s\n\n", string(data))
}

// writeToClient writes a raw SSE frame and flushes it immediately. Every
// write to a client goes through here under the client lock: the
// ResponseWriter is not safe for concurrent use, and an unflushed frame is
// indistinguishable from a silent stream on the browser side. A retired
// client is never written. It returns the write error, or an error for a
// recovered panic; the caller retires the client without holding its lock.
func (b *SSEBroadcaster) writeToClient(client *SSEClient, frame string) (err error) {
	client.mu.Lock()
	defer client.mu.Unlock()

	// Publish the write before checking Done. Retirement closes Done before
	// quiesce reads writing, so either this check sees Done closed and skips
	// the write, or quiesce sees the write and can interrupt it.
	client.writing.Store(true)
	defer client.writing.Store(false)
	select {
	case <-client.Done:
		return nil
	default:
	}

	defer func() {
		if r := recover(); r != nil {
			log.Warn().
				Str("client_id", client.ID).
				Interface("panic", r).
				Msg("Recovered from panic while sending to SSE client")
			err = fmt.Errorf("panic writing to SSE client: %v", r)
		}
	}()

	if _, err := fmt.Fprint(client.Writer, frame); err != nil {
		log.Debug().
			Err(err).
			Str("client_id", client.ID).
			Msg("Failed to write to SSE client, removing")
		return err
	}

	// Flush immediately
	client.Flusher.Flush()
	client.lastActive.Store(time.Now().UnixNano())
	return nil
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
			b.retireInactiveClients(time.Now())
		}
	}
}

// retireInactiveClients retires clients whose writer has not completed a
// write within sseClientInactivityTimeout, and expires the write a stalled
// client is blocked in. It reads activity atomically, so it never waits on a
// client's writer while holding b.mu.
func (b *SSEBroadcaster) retireInactiveClients(now time.Time) {
	b.mu.Lock()
	retired := make([]*SSEClient, 0)
	for id, client := range b.clients {
		if now.Sub(client.lastActivity()) > sseClientInactivityTimeout {
			b.retireLocked(client)
			retired = append(retired, client)
			log.Info().
				Str("client_id", id).
				Msg("Removed inactive SSE client")
		}
	}
	b.mu.Unlock()
	for _, client := range retired {
		client.interruptWrite()
	}
}

// SendHeartbeat sends a heartbeat comment to every client to keep
// connections alive. It never waits on a client's writer: the heartbeat
// follows the same broadcast path as statuses, so it keeps emission order
// and reaches each client through that client's ordered queue. A client
// whose queue is full already has undelivered frames and skips it.
func (b *SSEBroadcaster) SendHeartbeat() {
	if b.closed.Load() {
		return
	}

	b.mu.RLock()
	legacy := make([]*SSEClient, 0)
	for _, client := range b.clients {
		if client.queue == nil {
			legacy = append(legacy, client)
		}
	}
	b.mu.RUnlock()

	// Clients not registered via AddClient have no ordered writer.
	for _, client := range legacy {
		b.sendHeartbeatToClient(client)
	}

	select {
	case <-b.stopCh:
	case b.messageChan <- sseFrame{heartbeat: true}:
	default:
	}
}

// Close closes the broadcaster, retires all clients and expires any write
// they are blocked in, including a connection preamble still being written
// before the handler reached its wait. It waits for the broadcaster's own
// loops, which never block on a client, but not for client writers: each
// owning handler joins its writer through RemoveClient once Done wakes it.
func (b *SSEBroadcaster) Close() {
	b.closeOnce.Do(func() {
		b.closed.Store(true)
		close(b.stopCh)

		b.mu.Lock()
		retired := make([]*SSEClient, 0, len(b.clients))
		for _, client := range b.clients {
			b.retireLocked(client)
			retired = append(retired, client)
		}
		b.mu.Unlock()
		for _, client := range retired {
			client.interruptWrite()
		}

		b.loopWg.Wait()
		log.Info().Msg("SSE broadcaster closed")
	})
}

func (b *SSEBroadcaster) sendHeartbeatToClient(c *SSEClient) {
	if err := b.writeFrame(c, sseFrame{heartbeat: true}); err != nil {
		b.retireClient(c)
	}
}

func closeDone(ch chan bool) {
	select {
	case <-ch:
	default:
		close(ch)
	}
}
