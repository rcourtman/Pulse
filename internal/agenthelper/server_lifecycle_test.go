package agenthelper

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// These listeners expose Accept/Close ordering without depending on socket
// scheduling. Existing command-package tests cover actual Unix sockets and
// kernel-authenticated peer identity.
type lifecycleListener struct {
	accept func() (net.Conn, error)
	close  func() error
}

func (l lifecycleListener) Accept() (net.Conn, error) { return l.accept() }
func (l lifecycleListener) Close() error              { return l.close() }
func (l lifecycleListener) Addr() net.Addr            { return lifecycleAddr{} }

type lifecycleAddr struct{}

func (lifecycleAddr) Network() string { return "test" }
func (lifecycleAddr) String() string  { return "helper-lifecycle" }

func lifecycleWait(t *testing.T, done <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func TestServerServeClosesIdleListenerOnCancellation(t *testing.T) {
	for _, alreadyCanceled := range []bool{false, true} {
		t.Run(map[bool]string{false: "accept-blocked", true: "already-canceled"}[alreadyCanceled], func(t *testing.T) {
			accepting := make(chan struct{})
			closed := make(chan struct{})
			var acceptOnce, closeOnce sync.Once
			listener := lifecycleListener{
				accept: func() (net.Conn, error) {
					acceptOnce.Do(func() { close(accepting) })
					<-closed
					return nil, net.ErrClosed
				},
				close: func() error { closeOnce.Do(func() { close(closed) }); return nil },
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if alreadyCanceled {
				cancel()
			}
			server := newTestServer(t, NewRegistry(nil, nil), authorizedResolver(1000), nil)
			done := make(chan struct{})
			var serveErr error
			go func() { serveErr = server.Serve(ctx, listener); close(done) }()
			t.Cleanup(func() { _ = listener.Close(); <-done })
			if !alreadyCanceled {
				lifecycleWait(t, accepting, "Accept entry")
				cancel()
			}
			lifecycleWait(t, done, "Serve to own canceled listener cleanup")
			lifecycleWait(t, closed, "listener closure")
			if serveErr != nil {
				t.Fatalf("canceled Serve = %v", serveErr)
			}
		})
	}
}

func TestServerServeDrainsAcceptedOperation(t *testing.T) {
	for _, terminal := range []string{"parent-canceled", "accept-failed"} {
		t.Run(terminal, func(t *testing.T) {
			entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			releaseProvider := func() { releaseOnce.Do(func() { close(release) }) }
			provider := fakeSMARTProvider(func(ctx context.Context) (json.RawMessage, error) {
				close(entered)
				<-ctx.Done()
				close(canceled)
				<-release // Serve must not abandon a provider still cleaning up.
				return nil, ctx.Err()
			})
			server, err := NewServer(ServerConfig{
				AllowedUID: 1000, PeerResolver: authorizedResolver(1000), Registry: NewRegistry(provider, nil),
				MaxOperationTimeout: 10 * time.Second,
			})
			if err != nil {
				t.Fatal(err)
			}
			serverConn, clientConn := net.Pipe()
			defer clientConn.Close()
			_ = clientConn.SetDeadline(time.Now().Add(5 * time.Second))
			closed, failAccept := make(chan struct{}), make(chan struct{})
			var closeOnce sync.Once
			acceptErr := errors.New("synthetic terminal accept failure")
			first := true
			listener := lifecycleListener{
				accept: func() (net.Conn, error) {
					if first {
						first = false
						return serverConn, nil
					}
					select {
					case <-closed:
						return nil, net.ErrClosed
					case <-failAccept:
						return nil, acceptErr
					}
				},
				close: func() error { closeOnce.Do(func() { close(closed) }); return nil },
			}
			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan struct{})
			var serveErr error
			go func() { serveErr = server.Serve(ctx, listener); close(done) }()
			t.Cleanup(func() {
				cancel()
				releaseProvider()
				_ = listener.Close()
				_ = clientConn.Close()
				<-done
				// Even the old Serve returned before its worker. Do not leave that
				// worker behind when this regression witness fails on old source.
				deadline := time.Now().Add(time.Second)
				for len(server.connectionSlots) != 0 && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
			})
			request := validRequest(OperationSMARTSnapshot)
			request.DeadlineMillis = 10000
			frame, err := EncodeRequestFrame(request)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := writeAll(clientConn, frame); err != nil {
				t.Fatal(err)
			}
			lifecycleWait(t, entered, "provider entry")
			if terminal == "parent-canceled" {
				cancel()
				_ = listener.Close() // The command previously performed this step.
			} else {
				close(failAccept)
			}
			select {
			case <-done:
				t.Fatal("Serve returned before canceling/draining its accepted operation")
			case <-canceled:
			case <-time.After(time.Second):
				t.Fatal("accepted provider did not receive shutdown cancellation")
			}
			select {
			case <-done:
				t.Fatal("Serve returned while its accepted provider was still cleaning up")
			case <-time.After(20 * time.Millisecond):
			}
			releaseProvider()
			lifecycleWait(t, done, "drained Serve return")
			if terminal == "accept-failed" && !errors.Is(serveErr, acceptErr) {
				t.Fatalf("Serve error = %v, want original Accept error", serveErr)
			}
			if terminal == "parent-canceled" && serveErr != nil {
				t.Fatalf("canceled Serve = %v", serveErr)
			}
			if got := len(server.connectionSlots); got != 0 {
				t.Fatalf("Serve left %d active connection slots", got)
			}
			lifecycleWait(t, closed, "listener closure")
			if _, err := clientConn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
				t.Fatalf("drained client read = %v, want closed transport", err)
			}
		})
	}
}

type lifecycleConn struct {
	net.Conn
	read  func([]byte) (int, error)
	write func([]byte) (int, error)
	close func() error
}

func (c lifecycleConn) Read(p []byte) (int, error) {
	if c.read != nil {
		return c.read(p)
	}
	return c.Conn.Read(p)
}

func (c lifecycleConn) Write(p []byte) (int, error) {
	if c.write != nil {
		return c.write(p)
	}
	return c.Conn.Write(p)
}

func (c lifecycleConn) Close() error {
	if c.close != nil {
		return c.close()
	}
	return c.Conn.Close()
}

func TestServerConnectionCancellationInterruptsTransport(t *testing.T) {
	for _, phase := range []string{"header", "partial-header", "partial-payload", "response-write"} {
		t.Run(phase, func(t *testing.T) {
			transportEntered := make(chan struct{})
			var enteredOnce sync.Once
			server, err := NewServer(ServerConfig{
				AllowedUID: 1000, PeerResolver: authorizedResolver(1000), Registry: NewRegistry(nil, nil),
				MaxOperationTimeout: 10 * time.Second, PreFrameTimeout: 10 * time.Second,
			})
			if err != nil {
				t.Fatal(err)
			}
			serverConn, clientConn := net.Pipe()
			wrapped := lifecycleConn{Conn: serverConn}
			wrapped.read = func(p []byte) (int, error) {
				enteredOnce.Do(func() { close(transportEntered) })
				return serverConn.Read(p)
			}
			if phase == "response-write" {
				wrapped.read = nil
				wrapped.write = func(p []byte) (int, error) {
					enteredOnce.Do(func() { close(transportEntered) })
					return serverConn.Write(p)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan struct{})
			go func() { server.HandleConnection(ctx, wrapped); close(done) }()
			t.Cleanup(func() { cancel(); _ = clientConn.Close(); <-done })
			frame, err := EncodeRequestFrame(validRequest(OperationHealth))
			if err != nil {
				t.Fatal(err)
			}
			switch phase {
			case "partial-header":
				_, err = writeAll(clientConn, frame[:2])
			case "partial-payload":
				_, err = writeAll(clientConn, frame[:len(frame)-1])
			case "response-write":
				_, err = writeAll(clientConn, frame)
			}
			if err != nil {
				t.Fatal(err)
			}
			lifecycleWait(t, transportEntered, "blocked "+phase)
			cancel()
			lifecycleWait(t, done, "canceled "+phase+" cleanup")
			if len(server.connectionSlots) != 0 {
				t.Fatal("canceled connection retained its slot")
			}
		})
	}
}

func TestServerDoesNotDispatchRequestCanceledWhileReading(t *testing.T) {
	var dispatched atomic.Int32
	provider := fakeSMARTProvider(func(context.Context) (json.RawMessage, error) {
		dispatched.Add(1)
		return json.RawMessage(`{"disks":[]}`), nil
	})
	server := newTestServer(t, NewRegistry(provider, nil), authorizedResolver(1000), nil)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	serverConn, clientConn := net.Pipe()
	frame, err := EncodeRequestFrame(validRequest(OperationSMARTSnapshot))
	if err != nil {
		t.Fatal(err)
	}
	remaining := len(frame)
	wrapped := lifecycleConn{Conn: serverConn, read: func(p []byte) (int, error) {
		n, err := serverConn.Read(p)
		remaining -= n
		if remaining == 0 {
			cancel() // Full valid frame, but its server lifetime has ended.
		}
		return n, err
	}}
	done := make(chan struct{})
	go func() { server.HandleConnection(ctx, wrapped); close(done) }()
	t.Cleanup(func() { _ = clientConn.Close(); <-done })
	if _, err := writeAll(clientConn, frame); err != nil {
		t.Fatal(err)
	}
	_ = clientConn.Close()
	lifecycleWait(t, done, "canceled full request cleanup")
	if got := dispatched.Load(); got != 0 {
		t.Fatalf("provider dispatched %d times after cancellation", got)
	}
}

func TestServerRejectsLateAcceptWithoutResolvingPeer(t *testing.T) {
	accepting, closed := make(chan struct{}), make(chan struct{})
	var closeOnce sync.Once
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	var resolved atomic.Int32
	server := newTestServer(t, NewRegistry(nil, nil), PeerResolverFunc(func(net.Conn) (Peer, error) {
		resolved.Add(1)
		return Peer{UID: 1000}, nil
	}), nil)
	listener := lifecycleListener{
		accept: func() (net.Conn, error) {
			close(accepting)
			<-closed
			return serverConn, nil // An already pending connection wins the race.
		},
		close: func() error { closeOnce.Do(func() { close(closed) }); return nil },
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() {
		if err := server.Serve(ctx, listener); err != nil {
			t.Errorf("Serve: %v", err)
		}
		close(done)
	}()
	t.Cleanup(func() { cancel(); _ = listener.Close(); <-done })
	lifecycleWait(t, accepting, "pending Accept")
	cancel()
	lifecycleWait(t, done, "late Accept cleanup")
	if resolved.Load() != 0 || len(server.connectionSlots) != 0 {
		t.Fatal("late connection entered peer resolution or retained its slot")
	}
	if _, err := clientConn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("late client read = %v, want EOF", err)
	}
}

func TestServerAlreadyCanceledConnectionDoesNotResolvePeer(t *testing.T) {
	var resolved atomic.Int32
	audited := make(chan AuditEvent, 1)
	server := newTestServer(t, NewRegistry(nil, nil), PeerResolverFunc(func(net.Conn) (Peer, error) {
		resolved.Add(1)
		return Peer{UID: 1000}, nil
	}), func(event AuditEvent) { audited <- event })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	server.HandleConnection(ctx, serverConn)
	if resolved.Load() != 0 || len(server.connectionSlots) != 0 {
		t.Fatal("already canceled connection entered peer resolution or retained its slot")
	}
	event := <-audited
	if event.Success || event.RequestBytes != 0 || event.ResponseBytes != 0 || event.RequestID != "" || event.Operation != "" {
		t.Fatalf("canceled connection audit = %#v", event)
	}
	if _, err := clientConn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("canceled client read = %v, want EOF", err)
	}
}

func TestServerShutdownJoinsCloseCallbacks(t *testing.T) {
	for _, target := range []string{"listener", "connection"} {
		t.Run(target, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			closeEntered, release, ready := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			releaseClose := func() { releaseOnce.Do(func() { close(release) }) }
			done := make(chan struct{})
			server := newTestServer(t, NewRegistry(nil, nil), authorizedResolver(1000), nil)
			if target == "listener" {
				listener := lifecycleListener{
					accept: func() (net.Conn, error) { close(ready); <-ctx.Done(); return nil, net.ErrClosed },
					close:  func() error { close(closeEntered); <-release; return nil },
				}
				go func() {
					if err := server.Serve(ctx, listener); err != nil {
						t.Errorf("Serve: %v", err)
					}
					close(done)
				}()
			} else {
				serverConn, clientConn := net.Pipe()
				defer clientConn.Close()
				var readOnce, closeOnce sync.Once
				wrapped := lifecycleConn{
					Conn: serverConn,
					read: func(p []byte) (int, error) {
						readOnce.Do(func() { close(ready) })
						return serverConn.Read(p)
					},
					close: func() error {
						closeOnce.Do(func() {
							_ = serverConn.Close() // Read is awake while Close still owns cleanup.
							close(closeEntered)
							<-release
						})
						return nil
					},
				}
				go func() { server.HandleConnection(ctx, wrapped); close(done) }()
			}
			t.Cleanup(func() { cancel(); releaseClose(); <-done })
			lifecycleWait(t, ready, "transport entry")
			cancel()
			lifecycleWait(t, closeEntered, "close callback entry")
			select {
			case <-done:
				t.Fatal("owner returned before its close callback completed")
			case <-time.After(20 * time.Millisecond):
			}
			releaseClose()
			lifecycleWait(t, done, "joined close callback")
		})
	}
}

func TestServerPeerDisconnectDoesNotPromiseOperationCancellation(t *testing.T) {
	entered, peerClosed := make(chan struct{}), make(chan struct{})
	var ended atomic.Bool
	provider := fakeSMARTProvider(func(ctx context.Context) (json.RawMessage, error) {
		close(entered)
		<-peerClosed
		ended.Store(ctx.Err() != nil)
		return json.RawMessage(`{"disks":[]}`), nil
	})
	server, err := NewServer(ServerConfig{
		AllowedUID: 1000, PeerResolver: authorizedResolver(1000), Registry: NewRegistry(provider, nil),
		MaxOperationTimeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() { server.HandleConnection(t.Context(), serverConn); close(done) }()
	var peerOnce sync.Once
	release := func() { peerOnce.Do(func() { close(peerClosed) }) }
	t.Cleanup(func() { release(); _ = clientConn.Close(); <-done })
	request := validRequest(OperationSMARTSnapshot)
	request.DeadlineMillis = 10000
	frame, err := EncodeRequestFrame(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeAll(clientConn, frame); err != nil {
		t.Fatal(err)
	}
	lifecycleWait(t, entered, "accepted operation")
	_ = clientConn.Close()
	release()
	lifecycleWait(t, done, "accepted operation completion after peer disconnect")
	if ended.Load() {
		t.Fatal("peer disconnect unexpectedly canceled the accepted operation context")
	}
}
