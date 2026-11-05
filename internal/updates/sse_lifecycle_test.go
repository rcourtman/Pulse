package updates

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// These tests pin the SSE writer lifecycle: once a client's owning handler
// has removed it and returned, the broadcaster must never write to or flush
// its ResponseWriter again, whatever retired the client first (the handler,
// an inactivity sweep, a write failure or panic, or broadcaster/manager
// shutdown). Shutdown and heartbeats must not wait on a blocked client.

const lifecycleTimeout = 5 * time.Second

// heldWriter is a streaming ResponseWriter whose writes can be held open to
// stand in for a slow or stalled network peer. It records every write or
// flush that reaches it after its owning handler returned, and any write
// still in flight at that moment.
type heldWriter struct {
	header http.Header

	mu      sync.Mutex
	body    strings.Builder
	holdOn  string        // frames containing this text block until released
	panicOn string        // frames containing this text panic
	release chan struct{} // closed to let held writes finish
	stop    chan struct{} // closed when a write deadline interrupts writes
	stopped bool

	entered    chan string // receives each frame as a held write begins
	inflight   atomic.Int32
	returned   atomic.Bool
	violations []string
}

func newHeldWriter(holdOn string) *heldWriter {
	return &heldWriter{
		header:  http.Header{},
		holdOn:  holdOn,
		release: make(chan struct{}),
		stop:    make(chan struct{}),
		entered: make(chan string, 16),
	}
}

func (w *heldWriter) Header() http.Header { return w.header }
func (w *heldWriter) WriteHeader(int)     {}

func (w *heldWriter) violation(format string, args ...any) {
	w.mu.Lock()
	w.violations = append(w.violations, fmt.Sprintf(format, args...))
	w.mu.Unlock()
}

func (w *heldWriter) Write(p []byte) (int, error) {
	w.inflight.Add(1)
	defer w.inflight.Add(-1)
	frame := string(p)
	if w.returned.Load() {
		w.violation("write after handler returned: %q", frame)
	}
	if w.panicOn != "" && strings.Contains(frame, w.panicOn) {
		panic("simulated writer panic")
	}
	if w.holdOn != "" && strings.Contains(frame, w.holdOn) {
		w.entered <- frame
		select {
		case <-w.release:
		case <-w.stop:
			return 0, os.ErrDeadlineExceeded
		}
	}
	w.mu.Lock()
	w.body.WriteString(frame)
	w.mu.Unlock()
	return len(p), nil
}

func (w *heldWriter) Flush() {
	if w.returned.Load() {
		w.violation("flush after handler returned")
	}
}

func (w *heldWriter) releaseHeld() {
	w.mu.Lock()
	defer w.mu.Unlock()
	select {
	case <-w.release:
	default:
		close(w.release)
	}
}

// handlerReturned marks the end of the owning handler, as net/http would
// reclaim the ResponseWriter at that point.
func (w *heldWriter) handlerReturned() {
	if n := w.inflight.Load(); n > 0 {
		w.violation("%d write(s) still in flight when the handler returned", n)
	}
	w.returned.Store(true)
}

func (w *heldWriter) requireClean(t *testing.T) {
	t.Helper()
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.violations) > 0 {
		t.Fatalf("ResponseWriter used outside its handler: %v", w.violations)
	}
}

func (w *heldWriter) text() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.String()
}

// deadlineWriter adds ResponseController write-deadline support, as the
// net/http response and Pulse's middleware writers provide.
type deadlineWriter struct {
	*heldWriter
	deadlines atomic.Int32
}

func (w *deadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadlines.Add(1)
	if !deadline.After(time.Now()) {
		w.mu.Lock()
		if !w.stopped {
			w.stopped = true
			close(w.stop)
		}
		w.mu.Unlock()
	}
	return nil
}

// runHandler mimics HandleUpdateStream: register, wait for the request to
// end or the server to drop the client, remove it, and return.
func runHandler(b *SSEBroadcaster, w http.ResponseWriter, id string, requestDone <-chan struct{}, onReturn func()) (*SSEClient, <-chan struct{}) {
	returned := make(chan struct{})
	client := b.AddClient(w, id)
	if client == nil {
		close(returned)
		return nil, returned
	}
	go func() {
		defer close(returned)
		select {
		case <-requestDone:
		case <-client.Done:
		}
		b.RemoveClient(id)
		onReturn()
	}()
	return client, returned
}

func awaitHeldWrite(t *testing.T, w *heldWriter) {
	t.Helper()
	select {
	case <-w.entered:
	case <-time.After(lifecycleTimeout):
		t.Fatal("broadcast never reached the client writer")
	}
}

func requireReturns(t *testing.T, label string, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(lifecycleTimeout):
		t.Fatalf("%s did not return", label)
	}
}

func requireStillRunning(t *testing.T, label string, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
		t.Fatalf("%s returned while its client's write was still in flight", label)
	case <-time.After(150 * time.Millisecond):
	}
}

func heldStatus(message string) UpdateStatus {
	return UpdateStatus{Status: "downloading", Progress: 10, Message: message}
}

func TestSSERemoveClientInterruptsAndJoinsHeldWrite(t *testing.T) {
	b := NewSSEBroadcaster()
	defer b.Close()
	w := &deadlineWriter{heldWriter: newHeldWriter("held-stage")}
	requestDone := make(chan struct{})
	_, returned := runHandler(b, w, "owner", requestDone, w.handlerReturned)

	b.Broadcast(heldStatus("held-stage"))
	awaitHeldWrite(t, w.heldWriter)

	close(requestDone) // the browser disconnects while the write is held
	requireReturns(t, "handler", returned)
	if w.deadlines.Load() == 0 {
		t.Fatal("RemoveClient did not interrupt the held write with a write deadline")
	}

	// Nothing may reach the writer once the handler has returned.
	w.releaseHeld()
	b.Broadcast(heldStatus("after-return"))
	b.SendHeartbeat()
	time.Sleep(50 * time.Millisecond)
	w.requireClean(t)
}

func TestSSERemoveClientWaitsForHeldWriteWithoutDeadlineSupport(t *testing.T) {
	b := NewSSEBroadcaster()
	defer b.Close()
	w := newHeldWriter("held-stage")
	requestDone := make(chan struct{})
	_, returned := runHandler(b, w, "owner", requestDone, w.handlerReturned)

	b.Broadcast(heldStatus("held-stage"))
	awaitHeldWrite(t, w)
	close(requestDone)
	requireStillRunning(t, "handler", returned)

	w.releaseHeld()
	requireReturns(t, "handler", returned)
	b.Broadcast(heldStatus("after-return"))
	time.Sleep(50 * time.Millisecond)
	w.requireClean(t)
}

func TestSSEOwnerRemovalAfterServerRetirementStillJoinsWriter(t *testing.T) {
	t.Run("broadcaster close", func(t *testing.T) {
		b := NewSSEBroadcaster()
		w := newHeldWriter("held-stage")
		_, returned := runHandler(b, w, "owner", make(chan struct{}), w.handlerReturned)

		b.Broadcast(heldStatus("held-stage"))
		awaitHeldWrite(t, w)

		closed := make(chan struct{})
		go func() { b.Close(); close(closed) }()
		requireReturns(t, "broadcaster Close with a held client write", closed)
		// Close notified the handler; its removal must still join the writer.
		requireStillRunning(t, "handler", returned)

		w.releaseHeld()
		requireReturns(t, "handler", returned)
		w.requireClean(t)
	})

	t.Run("concurrent owner removals", func(t *testing.T) {
		b := NewSSEBroadcaster()
		defer b.Close()
		w := newHeldWriter("held-stage")
		client := b.AddClient(w, "owner")
		if client == nil {
			t.Fatal("AddClient returned nil")
		}
		b.Broadcast(heldStatus("held-stage"))
		awaitHeldWrite(t, w)

		first := make(chan struct{})
		second := make(chan struct{})
		go func() { b.RemoveClient("owner"); close(first) }()
		go func() { b.RemoveClient("owner"); close(second) }()
		requireStillRunning(t, "first RemoveClient", first)
		requireStillRunning(t, "second RemoveClient", second)

		w.releaseHeld()
		requireReturns(t, "first RemoveClient", first)
		requireReturns(t, "second RemoveClient", second)
		w.handlerReturned()
		b.Broadcast(heldStatus("after-return"))
		time.Sleep(50 * time.Millisecond)
		w.requireClean(t)
	})
}

func TestSSEWriterFailureAndPanicRetireClientForItsOwner(t *testing.T) {
	t.Run("panic", func(t *testing.T) {
		b := NewSSEBroadcaster()
		defer b.Close()
		w := newHeldWriter("")
		w.panicOn = "boom-stage"
		_, returned := runHandler(b, w, "owner", make(chan struct{}), w.handlerReturned)

		b.Broadcast(heldStatus("boom-stage"))
		requireReturns(t, "handler after writer panic", returned)
		b.Broadcast(heldStatus("after-return"))
		b.SendHeartbeat()
		time.Sleep(50 * time.Millisecond)
		w.requireClean(t)
	})

	t.Run("write error", func(t *testing.T) {
		b := NewSSEBroadcaster()
		defer b.Close()
		w := &deadlineWriter{heldWriter: newHeldWriter("fail-stage")}
		_, returned := runHandler(b, w, "owner", make(chan struct{}), w.handlerReturned)

		b.Broadcast(heldStatus("fail-stage"))
		awaitHeldWrite(t, w.heldWriter)
		_ = w.SetWriteDeadline(time.Now()) // the held write now fails
		requireReturns(t, "handler after write failure", returned)
		b.Broadcast(heldStatus("after-return"))
		time.Sleep(50 * time.Millisecond)
		w.requireClean(t)
	})
}

// A stalled client must not delay heartbeats to other clients, and neither
// heartbeats nor manager shutdown may wait on its blocked write.
func TestSSEHeartbeatAndShutdownDoNotWaitOnBlockedClient(t *testing.T) {
	m := NewManager(nil)
	stalled := newHeldWriter("held-stage")
	healthy := newHeldWriter("")
	stalledClient := m.AddSSEClient(stalled, "stalled")
	healthyClient := m.AddSSEClient(healthy, "healthy")
	if stalledClient == nil || healthyClient == nil {
		t.Fatal("AddSSEClient returned nil")
	}

	m.updateStatus("downloading", 10, "held-stage")
	awaitHeldWrite(t, stalled)

	heartbeat := make(chan struct{})
	go func() { m.GetSSEBroadcaster().SendHeartbeat(); close(heartbeat) }()
	requireReturns(t, "SendHeartbeat with a stalled client", heartbeat)
	deadline := time.Now().Add(lifecycleTimeout)
	for !strings.Contains(healthy.text(), ": heartbeat") {
		if time.Now().After(deadline) {
			t.Fatalf("healthy client never received a heartbeat: %q", healthy.text())
		}
		time.Sleep(5 * time.Millisecond)
	}

	closed := make(chan struct{})
	go func() { m.Close(); close(closed) }()
	requireReturns(t, "Manager.Close with a stalled client write", closed)

	stalled.releaseHeld()
	m.RemoveSSEClient("stalled")
	m.RemoveSSEClient("healthy")
	stalled.handlerReturned()
	healthy.handlerReturned()
	stalled.requireClean(t)
	healthy.requireClean(t)
}

// A reconnecting client receives the current status first, then later
// stages and heartbeats in order through its own writer.
func TestSSEReconnectReceivesSnapshotThenOrderedStream(t *testing.T) {
	b := NewSSEBroadcaster()
	defer b.Close()

	first := newHeldWriter("")
	b.AddClient(first, "first")
	b.Broadcast(UpdateStatus{Status: "downloading", Progress: 10, Message: "stage-1"})
	b.RemoveClient("first")
	first.handlerReturned()

	second := newHeldWriter("")
	if b.AddClient(second, "second") == nil {
		t.Fatal("AddClient returned nil")
	}
	if !strings.Contains(second.text(), "stage-1") {
		t.Fatalf("reconnect did not start with the current status: %q", second.text())
	}
	b.Broadcast(UpdateStatus{Status: "verifying", Progress: 25, Message: "stage-2"})
	b.SendHeartbeat()
	b.Broadcast(UpdateStatus{Status: "extracting", Progress: 40, Message: "stage-3"})

	deadline := time.Now().Add(lifecycleTimeout)
	for !strings.Contains(second.text(), "stage-3") {
		if time.Now().After(deadline) {
			t.Fatalf("reconnected client did not receive later stages: %q", second.text())
		}
		time.Sleep(5 * time.Millisecond)
	}
	body := second.text()
	i1, i2, ih, i3 := strings.Index(body, "stage-1"), strings.Index(body, "stage-2"), strings.Index(body, ": heartbeat"), strings.Index(body, "stage-3")
	if !(i1 < i2 && i2 < ih && ih < i3) {
		t.Fatalf("stream out of order (stage-1 %d, stage-2 %d, heartbeat %d, stage-3 %d): %q", i1, i2, ih, i3, body)
	}
	b.RemoveClient("second")
	second.handlerReturned()
	first.requireClean(t)
	second.requireClean(t)
}

// Close writes the closed flag that every publication reads, while status
// publication keeps running; the race detector checks the synchronization.
// After Close, the status snapshot still updates but nothing is delivered.
func TestManagerCloseSynchronizesWithStatusPublication(t *testing.T) {
	m := NewManager(nil)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; ; n++ {
				select {
				case <-stop:
					return
				default:
				}
				m.updateStatus("downloading", n%100, "racing")
			}
		}()
	}
	time.Sleep(10 * time.Millisecond)
	m.Close()
	close(stop)
	wg.Wait()

	before, _ := m.GetSSECachedStatus()
	m.updateStatus("idle", 0, "after close")
	if got := m.GetStatus(); got.Message != "after close" {
		t.Fatalf("status snapshot after Close = %+v, want the new status", got)
	}
	if after, _ := m.GetSSECachedStatus(); after != before {
		t.Fatalf("SSE cache changed after Close: %+v -> %+v", before, after)
	}
	for range m.GetProgressChannel() {
	} // closed by Close, so this drains and ends
	if client := m.AddSSEClient(newHeldWriter(""), "late"); client != nil {
		t.Fatal("AddSSEClient succeeded after Close")
	}
}

// Owners remove clients by ID, so a second client under an ID that is still
// live, or retired but not yet collected, is refused rather than allowed to
// shadow the first client's writer.
func TestSSEDuplicateClientIDRejectedUntilCollected(t *testing.T) {
	b := NewSSEBroadcaster()
	defer b.Close()
	first := newHeldWriter("held-stage")
	if b.AddClient(first, "dup") == nil {
		t.Fatal("first AddClient returned nil")
	}
	if b.AddClient(newHeldWriter(""), "dup") != nil {
		t.Fatal("second live client under the same ID was admitted")
	}

	b.Broadcast(heldStatus("held-stage"))
	awaitHeldWrite(t, first)
	removed := make(chan struct{})
	go func() { b.RemoveClient("dup"); close(removed) }()
	requireStillRunning(t, "RemoveClient", removed)
	if b.AddClient(newHeldWriter(""), "dup") != nil {
		t.Fatal("client admitted under the ID of a retired, still-writing client")
	}

	first.releaseHeld()
	requireReturns(t, "RemoveClient", removed)
	first.handlerReturned()
	again := newHeldWriter("")
	if b.AddClient(again, "dup") == nil {
		t.Fatal("ID not reusable after its client was collected")
	}
	b.RemoveClient("dup")
	again.handlerReturned()
	first.requireClean(t)
	again.requireClean(t)
}

// The connection preamble is written on the handler's goroutine before it
// waits for the client to go away. Shutdown and the inactivity sweep must
// interrupt it when the peer is stalled.
func TestSSERetirementInterruptsBlockedPreamble(t *testing.T) {
	for _, retire := range []string{"close", "sweep"} {
		t.Run(retire, func(t *testing.T) {
			b := NewSSEBroadcaster()
			defer b.Close()
			w := &deadlineWriter{heldWriter: newHeldWriter(": connected")}
			added := make(chan *SSEClient, 1)
			go func() { added <- b.AddClient(w, "owner") }()
			awaitHeldWrite(t, w.heldWriter)

			if retire == "close" {
				b.Close()
			} else {
				b.retireInactiveClients(time.Now().Add(2 * sseClientInactivityTimeout))
			}
			var client *SSEClient
			select {
			case client = <-added:
			case <-time.After(lifecycleTimeout):
				t.Fatalf("%s did not interrupt the blocked preamble", retire)
			}
			if client == nil {
				t.Fatal("AddClient returned nil")
			}
			select {
			case <-client.Done:
			default:
				t.Fatal("interrupted client was not retired")
			}
			removed := make(chan struct{})
			go func() { b.RemoveClient("owner"); close(removed) }()
			requireReturns(t, "RemoveClient after interrupted preamble", removed)
			w.handlerReturned()
			b.Broadcast(heldStatus("after-return"))
			time.Sleep(50 * time.Millisecond)
			w.requireClean(t)
		})
	}
}

// A retired client whose writer has already stopped keeps its ID reserved
// until the owning handler collects it. Otherwise a new stream could be
// admitted under that ID and the delayed owner's RemoveClient would remove
// it instead.
func TestSSEStoppedButUncollectedClientKeepsIDReserved(t *testing.T) {
	b := NewSSEBroadcaster()
	defer b.Close()
	old := newHeldWriter("")
	client := b.AddClient(old, "reused")
	if client == nil {
		t.Fatal("first AddClient returned nil")
	}

	// Retire it without the owner: the inactivity sweep.
	b.retireInactiveClients(time.Now().Add(2 * sseClientInactivityTimeout))
	select {
	case <-client.stopped:
	case <-time.After(lifecycleTimeout):
		t.Fatal("retired client's writer never stopped")
	}
	// Give any post-exit cleanup a chance to run before probing the ID.
	time.Sleep(50 * time.Millisecond)

	if b.AddClient(newHeldWriter(""), "reused") != nil {
		t.Fatal("client admitted under the ID of a stopped but uncollected client")
	}

	// The delayed owner now collects its client.
	b.RemoveClient("reused")
	old.handlerReturned()

	fresh := newHeldWriter("")
	freshClient := b.AddClient(fresh, "reused")
	if freshClient == nil {
		t.Fatal("ID not reusable after its client was collected")
	}
	if n := b.GetClientCount(); n != 1 {
		t.Fatalf("live clients = %d, want 1", n)
	}
	b.Broadcast(UpdateStatus{Status: "verifying", Progress: 25, Message: "fresh-stream"})
	deadline := time.Now().Add(lifecycleTimeout)
	for !strings.Contains(fresh.text(), "fresh-stream") {
		if time.Now().After(deadline) {
			t.Fatalf("new stream under the reused ID did not stay live: %q", fresh.text())
		}
		time.Sleep(5 * time.Millisecond)
	}
	select {
	case <-freshClient.Done:
		t.Fatal("new stream under the reused ID was retired")
	default:
	}

	b.RemoveClient("reused")
	fresh.handlerReturned()
	old.requireClean(t)
	fresh.requireClean(t)
}
