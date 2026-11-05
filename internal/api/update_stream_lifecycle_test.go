package api

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
)

// These tests run the real HandleUpdateStream behind a real net/http server
// with a real update manager. A client that stops reading makes a large
// progress frame block in the kernel, which is the slow or stalled browser
// the broadcaster has to survive. The probe below records any write or flush
// that reaches the handler's ResponseWriter after HandleUpdateStream
// returned, including a write still in flight at that moment.

const streamLifecycleTimeout = 10 * time.Second

// Larger than loopback socket buffers, so the write blocks while the peer
// is not reading.
const streamLifecycleHeldFrameBytes = 24 << 20

type streamLifecycleProbe struct {
	http.ResponseWriter
	inflight      atomic.Int32
	returned      atomic.Bool
	largeEntered  chan struct{}
	largeEnterOne sync.Once
	mu            sync.Mutex
	violations    []string
}

func (p *streamLifecycleProbe) Write(b []byte) (int, error) {
	p.inflight.Add(1)
	defer p.inflight.Add(-1)
	if p.returned.Load() {
		p.violation(fmt.Sprintf("write of %d bytes after HandleUpdateStream returned", len(b)))
	}
	if len(b) >= streamLifecycleHeldFrameBytes {
		p.largeEnterOne.Do(func() { close(p.largeEntered) })
	}
	return p.ResponseWriter.Write(b)
}

func (p *streamLifecycleProbe) Flush() {
	if p.returned.Load() {
		p.violation("flush after HandleUpdateStream returned")
	}
	p.ResponseWriter.(http.Flusher).Flush()
}

// Unwrap lets http.ResponseController reach the net/http response, as
// Pulse's own middleware writers forward deadline control.
func (p *streamLifecycleProbe) Unwrap() http.ResponseWriter { return p.ResponseWriter }

func (p *streamLifecycleProbe) violation(v string) {
	p.mu.Lock()
	p.violations = append(p.violations, v)
	p.mu.Unlock()
}

func (p *streamLifecycleProbe) requireClean(t *testing.T) {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.violations) > 0 {
		t.Fatalf("update stream ResponseWriter used after its handler returned: %v", p.violations)
	}
}

type streamLifecycleServer struct {
	t       *testing.T
	manager *updates.Manager
	server  *httptest.Server
	mu      sync.Mutex
	probes  []*streamLifecycleProbe
	done    []chan struct{}
}

func newStreamLifecycleServer(t *testing.T) *streamLifecycleServer {
	t.Helper()
	s := &streamLifecycleServer{t: t, manager: updates.NewManager(&config.Config{})}
	handlers := NewUpdateHandlers(s.manager, nil)
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probe := &streamLifecycleProbe{ResponseWriter: w, largeEntered: make(chan struct{})}
		done := make(chan struct{})
		s.mu.Lock()
		s.probes = append(s.probes, probe)
		s.done = append(s.done, done)
		s.mu.Unlock()
		defer close(done)
		handlers.HandleUpdateStream(probe, r)
		if n := probe.inflight.Load(); n > 0 {
			probe.violation(fmt.Sprintf("%d write(s) still in flight when HandleUpdateStream returned", n))
		}
		probe.returned.Store(true)
	}))
	t.Cleanup(func() {
		s.server.CloseClientConnections()
		s.server.Close()
		s.manager.Close()
	})
	return s
}

func (s *streamLifecycleServer) stream(i int) (*streamLifecycleProbe, <-chan struct{}) {
	s.t.Helper()
	deadline := time.Now().Add(streamLifecycleTimeout)
	for {
		s.mu.Lock()
		if len(s.probes) > i {
			probe, done := s.probes[i], s.done[i]
			s.mu.Unlock()
			return probe, done
		}
		s.mu.Unlock()
		if time.Now().After(deadline) {
			s.t.Fatalf("stream %d never reached the handler", i)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// openStalledStream connects, reads the connection preamble so the stream is
// registered, and then stops reading.
func (s *streamLifecycleServer) openStalledStream() net.Conn {
	s.t.Helper()
	conn, err := net.Dial("tcp", s.server.Listener.Addr().String())
	if err != nil {
		s.t.Fatalf("dial: %v", err)
	}
	s.t.Cleanup(func() { conn.Close() })
	if _, err := fmt.Fprintf(conn, "GET /api/updates/stream HTTP/1.1\r\nHost: pulse.test\r\n\r\n"); err != nil {
		s.t.Fatalf("write request: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(streamLifecycleTimeout))
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			s.t.Fatalf("read preamble: %v", err)
		}
		if strings.HasPrefix(line, ": connected") {
			return conn
		}
	}
}

// holdLargeWrite broadcasts a frame too large for the socket buffers and
// waits until the stalled stream's writer is blocked inside it.
func (s *streamLifecycleServer) holdLargeWrite(probe *streamLifecycleProbe) {
	s.t.Helper()
	s.manager.GetSSEBroadcaster().Broadcast(updates.UpdateStatus{
		Status:   "downloading",
		Progress: 10,
		Message:  strings.Repeat("x", streamLifecycleHeldFrameBytes),
	})
	select {
	case <-probe.largeEntered:
	case <-time.After(streamLifecycleTimeout):
		s.t.Fatal("large progress frame never reached the stream writer")
	}
	time.Sleep(200 * time.Millisecond)
	if probe.inflight.Load() == 0 {
		s.t.Fatal("large progress frame did not block; the reproduction needs a held network write")
	}
}

func requireStreamReturn(t *testing.T, label string, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(streamLifecycleTimeout):
		t.Fatalf("%s did not return", label)
	}
}

// Shutdown while the stream's write is blocked: Manager.Close must not wait
// for the stalled peer, and the handler it releases must not return its
// ResponseWriter while the broadcaster is still writing to it.
func TestUpdateStreamServerShutdownWithHeldWrite(t *testing.T) {
	s := newStreamLifecycleServer(t)
	s.openStalledStream()
	probe, handlerDone := s.stream(0)
	s.holdLargeWrite(probe)

	closed := make(chan struct{})
	go func() { s.manager.Close(); close(closed) }()
	requireStreamReturn(t, "Manager.Close with a held stream write", closed)
	requireStreamReturn(t, "HandleUpdateStream after shutdown", handlerDone)
	time.Sleep(50 * time.Millisecond)
	probe.requireClean(t)
}

// The browser goes away while a write is blocked.
func TestUpdateStreamClientDisconnectWithHeldWrite(t *testing.T) {
	s := newStreamLifecycleServer(t)
	conn := s.openStalledStream()
	probe, handlerDone := s.stream(0)
	s.holdLargeWrite(probe)

	conn.Close()
	requireStreamReturn(t, "HandleUpdateStream after client disconnect", handlerDone)
	s.manager.GetSSEBroadcaster().Broadcast(updates.UpdateStatus{Status: "verifying", Progress: 25, Message: "after-disconnect"})
	s.manager.GetSSEBroadcaster().SendHeartbeat()
	time.Sleep(50 * time.Millisecond)
	probe.requireClean(t)
}

// A stalled stream must not hold heartbeats back from a healthy stream, and
// a (re)connecting stream starts from the current status.
func TestUpdateStreamHeartbeatIsolationAndReconnect(t *testing.T) {
	s := newStreamLifecycleServer(t)
	s.openStalledStream()
	stalled, _ := s.stream(0)
	s.holdLargeWrite(stalled)
	// Keep the cached status small for the connecting stream.
	s.manager.GetSSEBroadcaster().Broadcast(updates.UpdateStatus{Status: "verifying", Progress: 25, Message: "current-stage"})

	resp, err := s.server.Client().Get(s.server.URL + "/api/updates/stream")
	if err != nil {
		t.Fatalf("open healthy stream: %v", err)
	}
	defer resp.Body.Close()
	lines := make(chan string, 64)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()
	healthy, healthyDone := s.stream(1)

	waitFor := func(want string) {
		t.Helper()
		deadline := time.After(streamLifecycleTimeout)
		for {
			select {
			case line, ok := <-lines:
				if !ok {
					t.Fatalf("healthy stream ended before %q", want)
				}
				if strings.Contains(line, want) {
					return
				}
			case <-deadline:
				t.Fatalf("healthy stream never delivered %q", want)
			}
		}
	}
	waitFor("current-stage")

	heartbeat := make(chan struct{})
	go func() { s.manager.GetSSEBroadcaster().SendHeartbeat(); close(heartbeat) }()
	requireStreamReturn(t, "SendHeartbeat with a stalled stream", heartbeat)
	waitFor(": heartbeat")

	resp.Body.Close()
	requireStreamReturn(t, "healthy HandleUpdateStream after disconnect", healthyDone)
	time.Sleep(50 * time.Millisecond)
	healthy.requireClean(t)
	stalled.requireClean(t)
}
