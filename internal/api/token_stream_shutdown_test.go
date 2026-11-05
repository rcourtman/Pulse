package api

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
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

// These tests drive the complete production /api/updates/stream chain: the
// real Router (auth context, tenant guard, admin and scope gates, the
// protected-stream token guard, middleware writers) and the real update
// broadcaster, with an API-token reader over a real TCP connection that stops
// reading. A progress frame larger than the socket buffers then blocks in the
// kernel, which is the stalled browser or proxy the stream has to survive.
//
// The outermost probe wraps the ResponseWriter net/http hands to the router,
// so it sees every byte any layer sends. It fails the test if a write or flush
// reaches it after the router's ServeHTTP returned, or is still in flight at
// that moment, and the tests fail if the handler cannot end while a write is
// blocked on the stalled peer.

const shutdownStreamTimeout = 25 * time.Second

const shutdownStreamHeldBytes = 24 << 20

type shutdownStreamProbe struct {
	http.ResponseWriter
	inflight     atomic.Int32
	largeStarted chan struct{}
	largeOnce    sync.Once
	returned     atomic.Bool
	mu           sync.Mutex
	violations   []string
}

func (p *shutdownStreamProbe) Write(b []byte) (int, error) {
	p.inflight.Add(1)
	defer p.inflight.Add(-1)
	if p.returned.Load() {
		p.violation(fmt.Sprintf("write of %d bytes after the router returned", len(b)))
	}
	if len(b) >= shutdownStreamHeldBytes {
		p.largeOnce.Do(func() { close(p.largeStarted) })
	}
	return p.ResponseWriter.Write(b)
}

func (p *shutdownStreamProbe) Flush() {
	if p.returned.Load() {
		p.violation("flush after the router returned")
	}
	p.ResponseWriter.(http.Flusher).Flush()
}

// Unwrap gives the router's own writers the real connection's deadline
// control, exactly as when net/http calls the router directly.
func (p *shutdownStreamProbe) Unwrap() http.ResponseWriter { return p.ResponseWriter }

func (p *shutdownStreamProbe) violation(v string) {
	p.mu.Lock()
	p.violations = append(p.violations, v)
	p.mu.Unlock()
}

type shutdownStreamServer struct {
	t       *testing.T
	fixture *protectedReadFixture
	server  *httptest.Server
	mu      sync.Mutex
	probes  []*shutdownStreamProbe
	done    []chan struct{}
	cancels []context.CancelFunc
}

func newShutdownStreamServer(t *testing.T) *shutdownStreamServer {
	t.Helper()
	s := &shutdownStreamServer{t: t, fixture: newProtectedReadFixture(t, []string{config.ScopeSettingsRead})}
	routerHandler := s.fixture.router.Handler()
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probe := &shutdownStreamProbe{ResponseWriter: w, largeStarted: make(chan struct{})}
		done := make(chan struct{})
		// The context the router admits, cancellable without closing the
		// peer's TCP connection.
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		s.mu.Lock()
		s.probes = append(s.probes, probe)
		s.done = append(s.done, done)
		s.cancels = append(s.cancels, cancel)
		s.mu.Unlock()
		defer close(done)
		routerHandler.ServeHTTP(probe, r.WithContext(ctx))
		if n := probe.inflight.Load(); n > 0 {
			probe.violation(fmt.Sprintf("%d write(s) still in flight when the router returned", n))
		}
		probe.returned.Store(true)
	}))
	t.Cleanup(func() {
		s.server.CloseClientConnections()
		s.server.Close()
	})
	return s
}

func (s *shutdownStreamServer) stream(i int) (*shutdownStreamProbe, <-chan struct{}) {
	s.t.Helper()
	deadline := time.Now().Add(shutdownStreamTimeout)
	for {
		s.mu.Lock()
		if len(s.probes) > i {
			probe, done := s.probes[i], s.done[i]
			s.mu.Unlock()
			return probe, done
		}
		s.mu.Unlock()
		if time.Now().After(deadline) {
			s.t.Fatalf("request %d never reached the router", i)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (s *shutdownStreamServer) broadcaster() *updates.SSEBroadcaster {
	return s.fixture.router.updateManager.GetSSEBroadcaster()
}

func heldProgress() updates.UpdateStatus {
	return updates.UpdateStatus{Status: "downloading", Progress: 10, Message: strings.Repeat("x", shutdownStreamHeldBytes)}
}

// cancelRequest cancels the admitted request context of request i while its
// peer stays connected.
func (s *shutdownStreamServer) cancelRequest(i int) {
	s.mu.Lock()
	cancel := s.cancels[i]
	s.mu.Unlock()
	cancel()
}

// dialStream sends an API-token stream request and, when waitForPreamble is
// set, reads until the connection preamble so the client is registered. It
// never reads again.
func (s *shutdownStreamServer) dialStream(waitForPreamble bool) net.Conn {
	s.t.Helper()
	return s.dialStreamAs("token", waitForPreamble)
}

// dialStreamAs is dialStream with a choice of credential: "token" for an API
// token, or "proxy" for proxy authentication, which carries no token.
func (s *shutdownStreamServer) dialStreamAs(credential string, waitForPreamble bool) net.Conn {
	s.t.Helper()
	auth := "X-API-Token: " + scopedReadToken + "\r\n"
	if credential == "proxy" {
		auth = "X-Proxy-Secret: " + s.fixture.cfg.ProxyAuthSecret + "\r\nX-Remote-User: stream-admin\r\n"
	}
	conn, err := net.Dial("tcp", s.server.Listener.Addr().String())
	if err != nil {
		s.t.Fatalf("dial: %v", err)
	}
	s.t.Cleanup(func() { conn.Close() })
	request := "GET /api/updates/stream HTTP/1.1\r\nHost: pulse.test\r\n" +
		auth + "X-Pulse-Org-ID: default\r\n\r\n"
	if _, err := conn.Write([]byte(request)); err != nil {
		s.t.Fatalf("write request: %v", err)
	}
	if !waitForPreamble {
		return conn
	}
	_ = conn.SetReadDeadline(time.Now().Add(shutdownStreamTimeout))
	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(status, " 200 ") {
		s.t.Fatalf("stream admission: %q %v", status, err)
	}
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

func (s *shutdownStreamServer) awaitHeldWrite(probe *shutdownStreamProbe) {
	s.t.Helper()
	select {
	case <-probe.largeStarted:
	case <-time.After(shutdownStreamTimeout):
		s.t.Fatal("large progress frame never reached the transport")
	}
	time.Sleep(200 * time.Millisecond)
	if probe.inflight.Load() == 0 {
		s.t.Fatal("large progress frame did not block on the stalled peer")
	}
}

func requireRouterReturn(t *testing.T, label string, done <-chan struct{}, within time.Duration) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(within):
		t.Fatalf("%s: /api/updates/stream did not return within %s", label, within)
	}
}

func (p *shutdownStreamProbe) requireClean(t *testing.T) {
	t.Helper()
	time.Sleep(50 * time.Millisecond)
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.violations) > 0 {
		t.Fatalf("/api/updates/stream response used after the router returned: %v", p.violations)
	}
}

// Shutdown while a progress write is blocked on the stalled peer.
func TestUpdateStreamChainShutdownInterruptsBlockedWrite(t *testing.T) {
	s := newShutdownStreamServer(t)
	s.dialStream(true)
	probe, done := s.stream(0)
	s.broadcaster().Broadcast(heldProgress())
	s.awaitHeldWrite(probe)

	closed := make(chan struct{})
	go func() { s.fixture.router.updateManager.Close(); close(closed) }()
	requireRouterReturn(t, "Manager.Close", closed, 5*time.Second)
	requireRouterReturn(t, "shutdown", done, 5*time.Second)
	probe.requireClean(t)
}

// Shutdown while the connection preamble itself is blocked, before the
// handler has reached its wait for the client to go away.
func TestUpdateStreamChainShutdownInterruptsBlockedPreamble(t *testing.T) {
	s := newShutdownStreamServer(t)
	// The preamble carries the current status, so make that too large to
	// fit the socket buffers before the reader connects.
	s.broadcaster().Broadcast(heldProgress())
	deadline := time.Now().Add(shutdownStreamTimeout)
	for {
		if cached, _ := s.broadcaster().GetCachedStatus(); len(cached.Message) == shutdownStreamHeldBytes {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("large status never became current")
		}
		time.Sleep(5 * time.Millisecond)
	}
	s.dialStream(false)
	probe, done := s.stream(0)
	s.awaitHeldWrite(probe)

	closed := make(chan struct{})
	go func() { s.fixture.router.updateManager.Close(); close(closed) }()
	requireRouterReturn(t, "Manager.Close", closed, 5*time.Second)
	requireRouterReturn(t, "shutdown during the preamble", done, 5*time.Second)
	probe.requireClean(t)
}

// The reader's API token is withdrawn while a write is blocked. The guard's
// production 15-second recheck must end the read without waiting for the
// stalled peer.
func TestUpdateStreamChainWithdrawalInterruptsBlockedWrite(t *testing.T) {
	s := newShutdownStreamServer(t)
	s.dialStream(true)
	probe, done := s.stream(0)
	s.broadcaster().Broadcast(heldProgress())
	s.awaitHeldWrite(probe)

	config.Mu.Lock()
	s.fixture.cfg.RemoveAPIToken(s.fixture.cfg.APITokens[0].ID)
	config.Mu.Unlock()
	requireRouterReturn(t, "credential withdrawal", done, 20*time.Second)
	if n := s.broadcaster().GetClientCount(); n != 0 {
		t.Fatalf("withdrawn reader still registered: %d client(s)", n)
	}
	probe.requireClean(t)
}

// The peer goes away while a write is blocked; later broadcasts and
// heartbeats must not reach the finished response.
func TestUpdateStreamChainClientDisconnectWithBlockedWrite(t *testing.T) {
	s := newShutdownStreamServer(t)
	conn := s.dialStream(true)
	probe, done := s.stream(0)
	s.broadcaster().Broadcast(heldProgress())
	s.awaitHeldWrite(probe)

	conn.Close()
	requireRouterReturn(t, "client disconnect", done, 5*time.Second)
	s.broadcaster().Broadcast(updates.UpdateStatus{Status: "verifying", Progress: 25, Message: "after-disconnect"})
	s.broadcaster().SendHeartbeat()
	probe.requireClean(t)
}

// stalledTransport is a response writer whose writes block until its write
// deadline expires, like a net/http connection to a peer that stopped
// reading.
type stalledTransport struct {
	*httptest.ResponseRecorder
	entered  chan struct{}
	expired  chan struct{}
	expireMu sync.Mutex
	expires  int
}

func newStalledTransport() *stalledTransport {
	return &stalledTransport{ResponseRecorder: httptest.NewRecorder(), entered: make(chan struct{}, 4), expired: make(chan struct{})}
}

func (s *stalledTransport) Write(p []byte) (int, error) {
	s.entered <- struct{}{}
	<-s.expired
	return 0, fmt.Errorf("write deadline exceeded")
}

func (s *stalledTransport) SetWriteDeadline(deadline time.Time) error {
	s.expireMu.Lock()
	defer s.expireMu.Unlock()
	s.expires++
	if !deadline.After(time.Now()) {
		select {
		case <-s.expired:
		default:
			close(s.expired)
		}
	}
	return nil
}

func guardOverStalledTransport(allowed func() bool) (*tokenStreamWriter, *stalledTransport, chan struct{}) {
	base := newStalledTransport()
	ctx, cancel := context.WithCancel(context.Background())
	writer := &tokenStreamWriter{writer: base, flusher: base, ctx: ctx, cancel: cancel, allowed: allowed}
	wrote := make(chan struct{})
	go func() { defer close(wrote); _, _ = writer.Write([]byte("data: admitted\n\n")) }()
	<-base.entered
	return writer, base, wrote
}

func requireWithin(t *testing.T, label string, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s waited on the stalled transport write", label)
	}
}

// Closure, withdrawal and deadline control take effect while a transport
// write is blocked: none of them queues behind the write, closure and
// withdrawal expire it, and nothing is written afterwards.
func TestTokenStreamWriterInterruptsStalledTransport(t *testing.T) {
	t.Run("close", func(t *testing.T) {
		writer, base, wrote := guardOverStalledTransport(func() bool { return true })
		closed := make(chan struct{})
		go func() { writer.close(); close(closed) }()
		requireWithin(t, "close", closed)
		requireWithin(t, "blocked write", wrote)
		if _, err := writer.Write([]byte("late")); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("late write = %v", err)
		}
		if base.Body.Len() != 0 {
			t.Fatalf("stalled transport received %q", base.Body.String())
		}
	})

	t.Run("withdrawal", func(t *testing.T) {
		var allowed atomic.Bool
		allowed.Store(true)
		writer, _, wrote := guardOverStalledTransport(allowed.Load)
		allowed.Store(false)
		checked := make(chan struct{})
		go func() { writer.check(); close(checked) }()
		requireWithin(t, "withdrawal recheck", checked)
		requireWithin(t, "blocked write", wrote)
		if writer.ctx.Err() == nil {
			t.Fatal("withdrawal did not end the read")
		}
		closed := make(chan struct{})
		go func() { writer.close(); close(closed) }()
		requireWithin(t, "close after withdrawal", closed)
	})

	t.Run("deadline control", func(t *testing.T) {
		writer, base, wrote := guardOverStalledTransport(func() bool { return true })
		set := make(chan struct{})
		go func() {
			_ = http.NewResponseController(writer).SetWriteDeadline(time.Now())
			close(set)
		}()
		requireWithin(t, "SetWriteDeadline", set)
		requireWithin(t, "blocked write", wrote)
		base.expireMu.Lock()
		expires := base.expires
		base.expireMu.Unlock()
		if expires == 0 {
			t.Fatal("deadline did not reach the transport")
		}
		writer.close()
	})
}

// The admitted request context is cancelled while the peer keeps its TCP
// connection open and has stopped reading, for an ordinary progress frame
// and for a preamble the handler is itself still blocked in. Token reads and
// proxy reads (no token, no periodic recheck) must both end promptly.
func TestUpdateStreamChainRequestCancellationInterruptsBlockedWrites(t *testing.T) {
	for _, credential := range []string{"token", "proxy"} {
		t.Run(credential+"/frame", func(t *testing.T) {
			s := newShutdownStreamServer(t)
			s.dialStreamAs(credential, true)
			probe, done := s.stream(0)
			s.broadcaster().Broadcast(heldProgress())
			s.awaitHeldWrite(probe)

			s.cancelRequest(0)
			requireRouterReturn(t, "request cancellation during a frame", done, 5*time.Second)
			if n := s.broadcaster().GetClientCount(); n != 0 {
				t.Fatalf("cancelled reader still registered: %d client(s)", n)
			}
			probe.requireClean(t)
		})

		t.Run(credential+"/preamble", func(t *testing.T) {
			s := newShutdownStreamServer(t)
			s.broadcaster().Broadcast(heldProgress())
			deadline := time.Now().Add(shutdownStreamTimeout)
			for {
				if cached, _ := s.broadcaster().GetCachedStatus(); len(cached.Message) == shutdownStreamHeldBytes {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("large status never became current")
				}
				time.Sleep(5 * time.Millisecond)
			}
			s.dialStreamAs(credential, false)
			probe, done := s.stream(0)
			s.awaitHeldWrite(probe)

			s.cancelRequest(0)
			requireRouterReturn(t, "request cancellation during the preamble", done, 5*time.Second)
			if n := s.broadcaster().GetClientCount(); n != 0 {
				t.Fatalf("cancelled reader still registered: %d client(s)", n)
			}
			probe.requireClean(t)
		})
	}
}

// A guarded read with no token or session revalidator has no periodic
// recheck, but cancelling its request context must still end a handler that
// is blocked inside a transport write, and the ended read refuses later
// deadline control.
func TestTokenStreamGuardContextCancellationWithoutRevalidator(t *testing.T) {
	base := newStalledTransport()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	afterWrite := make(chan error, 1)
	handler := retainTokenStreamAuthority(&config.Config{}, config.ScopeSettingsRead, true, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("data: admitted\n\n"))
		if r.Context().Err() == nil {
			afterWrite <- errors.New("request context not cancelled")
			return
		}
		afterWrite <- http.NewResponseController(w).SetWriteDeadline(time.Now().Add(time.Hour))
	})
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		handler(base, httptest.NewRequest(http.MethodGet, "/api/updates/stream", nil).WithContext(ctx))
	}()
	<-base.entered

	cancel()
	requireWithin(t, "guarded handler after context cancellation", returned)
	if err := <-afterWrite; !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("deadline reset after cancellation = %v, want io.ErrClosedPipe", err)
	}
}

// panickingTransport panics in the chosen operation and counts deadline
// changes, which must not happen after the panic has been cleaned up.
type panickingTransport struct {
	*httptest.ResponseRecorder
	op        string
	deadlines atomic.Int32
}

func (p *panickingTransport) Write(b []byte) (int, error) {
	if p.op == "write" {
		panic("transport write panic")
	}
	return p.ResponseRecorder.Write(b)
}

func (p *panickingTransport) WriteHeader(status int) {
	if p.op == "header" {
		panic("transport header panic")
	}
	p.ResponseRecorder.WriteHeader(status)
}

func (p *panickingTransport) Flush() {
	if p.op == "flush" {
		panic("transport flush panic")
	}
	p.ResponseRecorder.Flush()
}

func (p *panickingTransport) SetWriteDeadline(time.Time) error {
	p.deadlines.Add(1)
	return nil
}

// A transport operation that panics must still clear the guard's admitted
// I/O state and end the read, so closure neither waits on nor expires a
// write that no longer exists.
func TestTokenStreamWriterClearsIOStateOnTransportPanic(t *testing.T) {
	for _, op := range []string{"write", "header", "flush"} {
		t.Run(op, func(t *testing.T) {
			base := &panickingTransport{ResponseRecorder: httptest.NewRecorder(), op: op}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			writer := &tokenStreamWriter{writer: base, flusher: base, ctx: ctx, cancel: cancel, allowed: func() bool { return true }}
			func() {
				defer func() {
					if recover() == nil {
						t.Fatal("transport panic did not propagate")
					}
				}()
				switch op {
				case "write":
					_, _ = writer.Write([]byte("data: admitted\n\n"))
				case "header":
					writer.WriteHeader(http.StatusOK)
				case "flush":
					writer.Flush()
				}
			}()
			writer.mu.Lock()
			inIO, closed := writer.inIO, writer.closed
			writer.mu.Unlock()
			if inIO {
				t.Fatal("admitted I/O state survived the transport panic")
			}
			if !closed || ctx.Err() == nil {
				t.Fatal("transport panic did not end the read")
			}
			closedDone := make(chan struct{})
			go func() { writer.close(); close(closedDone) }()
			requireWithin(t, "close after transport panic", closedDone)
			if n := base.deadlines.Load(); n != 0 {
				t.Fatalf("closure expired a write deadline %d time(s) after the panic", n)
			}
			if _, err := writer.Write([]byte("late")); !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("late write = %v", err)
			}
		})
	}
}
