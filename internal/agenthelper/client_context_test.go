package agenthelper

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const contextTestDeadline = 30 * time.Second

type contextTestConn struct {
	net.Conn
	readHook     func()
	writeHook    func()
	deadlineHook func(time.Time)
	closeHook    func()
}

func (c *contextTestConn) Read(p []byte) (int, error) {
	if c.readHook != nil {
		c.readHook()
	}
	return c.Conn.Read(p)
}

func (c *contextTestConn) Write(p []byte) (int, error) {
	if c.writeHook != nil {
		c.writeHook()
	}
	return c.Conn.Write(p)
}

func (c *contextTestConn) SetDeadline(deadline time.Time) error {
	if c.deadlineHook != nil {
		c.deadlineHook(deadline)
	}
	return c.Conn.SetDeadline(deadline)
}

func (c *contextTestConn) Close() error {
	err := c.Conn.Close()
	if c.closeHook != nil {
		c.closeHook()
	}
	return err
}

func newContextTestClient(t *testing.T, dial DialContextFunc) *Client {
	t.Helper()
	client, err := NewClient(ClientConfig{
		SocketPath: "/run/pulse-agent/helper.sock", MaxDeadline: contextTestDeadline,
		DialContext: dial, NewRequestID: func() (string, error) { return "context-request", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func waitContextTestSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func TestClientCallContextOwnsBlockedExchange(t *testing.T) {
	for _, phase := range []string{"request_write", "response_header", "response_payload"} {
		t.Run(phase, func(t *testing.T) {
			conn, peer := net.Pipe()
			defer conn.Close()
			defer peer.Close()
			reached := make(chan struct{})
			var reachedOnce sync.Once
			markReached := func() { reachedOnce.Do(func() { close(reached) }) }
			wrapped := &contextTestConn{Conn: conn}
			if phase == "request_write" {
				wrapped.writeHook = markReached
			} else {
				reads := 0 // Only Call's reader owns this counter.
				wrapped.readHook = func() {
					reads++
					if phase == "response_header" || reads == 2 {
						markReached()
					}
				}
				serverDone := make(chan struct{})
				go func() {
					defer close(serverDone)
					if _, err := readFrame(peer, MaxRequestBytes); err != nil {
						return
					}
					if phase == "response_payload" {
						var header [4]byte
						binary.BigEndian.PutUint32(header[:], 2)
						_, _ = peer.Write(header[:]) // Announce a body, but do not supply it.
					}
				}()
				defer func() {
					_ = peer.Close()
					waitContextTestSignal(t, serverDone, "fixture reader cleanup")
				}()
			}
			client := newContextTestClient(t, func(context.Context, string, string) (net.Conn, error) { return wrapped, nil })
			// A live future deadline demonstrates cancellation, not timeout expiry.
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			done := make(chan error, 1)
			result := HealthResult{Status: "unchanged", ProtocolVersion: -1}
			go func() {
				id, err := client.Call(ctx, OperationHealth, OperationVersion1, contextTestDeadline, nil, &result)
				if id != "context-request" {
					err = errors.Join(err, errors.New("lost request identity"))
				}
				done <- err
			}()
			waitContextTestSignal(t, reached, phase)
			started := time.Now()
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("canceled exchange error = %v", err)
				}
			case <-time.After(time.Second):
				_ = peer.Close() // Release a regressed call before failing the test.
				<-done
				t.Fatal("canceled helper call kept waiting for its operation deadline")
			}
			t.Logf("cancellation ended %s in %s (operation budget %s)", phase, time.Since(started), contextTestDeadline)
			if result.Status != "unchanged" || result.ProtocolVersion != -1 {
				t.Fatalf("cancellation published a result: %+v", result)
			}
			_ = peer.SetReadDeadline(time.Now().Add(time.Second))
			if _, err := io.ReadAll(peer); err != nil {
				t.Fatalf("canceled client did not close its connection: %v", err)
			}
		})
	}
}

type contextTestPayload struct{ calls *int }

func (p contextTestPayload) MarshalJSON() ([]byte, error) {
	(*p.calls)++
	return []byte(`{}`), nil
}

func TestClientCallEndedContextSkipsPreparation(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(map[bool]string{false: "canceled", true: "expired"}[expired], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			want := context.Canceled
			if expired {
				ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				defer cancel()
				want = context.DeadlineExceeded
			}
			ids, payloads, dials := 0, 0, 0
			client := newContextTestClient(t, func(context.Context, string, string) (net.Conn, error) {
				dials++
				return nil, errors.New("unexpected dial")
			})
			client.newRequestID = func() (string, error) { ids++; return "context-request", nil }
			id, err := client.Call(ctx, OperationHealth, 1, contextTestDeadline, contextTestPayload{&payloads}, nil)
			if id != "" || !errors.Is(err, want) || ids != 0 || payloads != 0 || dials != 0 {
				t.Fatalf("ended call: id=%q error=%v preparation=(%d IDs, %d encodes, %d dials)", id, err, ids, payloads, dials)
			}
		})
	}
}

func TestClientCallOperationDeadlineIncludesDial(t *testing.T) {
	const budget = 50 * time.Millisecond
	started := time.Now()
	client := newContextTestClient(t, func(ctx context.Context, _, _ string) (net.Conn, error) {
		deadline, ok := ctx.Deadline()
		if !ok || deadline.After(started.Add(budget+10*time.Millisecond)) {
			return nil, errors.New("operation budget does not bound dialing")
		}
		<-ctx.Done()
		return nil, ctx.Err()
	})
	id, err := client.Call(context.Background(), OperationHealth, 1, budget, nil, nil)
	if id != "context-request" || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bounded dial: id=%q error=%v", id, err)
	}
}

func TestClientCallDialAndIOShareDeadline(t *testing.T) {
	for _, parentBounds := range []bool{false, true} {
		t.Run(map[bool]string{false: "operation", true: "parent"}[parentBounds], func(t *testing.T) {
			ctx := context.Background()
			if parentBounds {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
			}
			var dialDeadline, ioDeadline time.Time
			serverDone := make(chan struct{})
			client := newContextTestClient(t, func(ctx context.Context, _, _ string) (net.Conn, error) {
				dialDeadline, _ = ctx.Deadline()
				conn, peer := net.Pipe()
				go func() { defer close(serverDone); serveClientResponse(t, peer, nil) }()
				return &contextTestConn{Conn: conn, deadlineHook: func(d time.Time) { ioDeadline = d }}, nil
			})
			_, err := client.Call(ctx, OperationHealth, 1, contextTestDeadline, nil, &HealthResult{})
			waitContextTestSignal(t, serverDone, "successful fixture cleanup")
			if err != nil || dialDeadline.IsZero() || !dialDeadline.Equal(ioDeadline) {
				t.Fatalf("dial budget was restarted for I/O: dial=%s I/O=%s error=%v", dialDeadline, ioDeadline, err)
			}
		})
	}
}

func TestClientCallCanceledDialResultSendsNoFrame(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()
	var writes atomic.Int32
	client := newContextTestClient(t, func(context.Context, string, string) (net.Conn, error) {
		cancel() // A dialer may return a connection concurrently with cancellation.
		return &contextTestConn{Conn: conn, writeHook: func() { writes.Add(1) }}, nil
	})
	// If a regressed client writes, unblock it rather than waiting 30 seconds.
	serverDone := make(chan struct{})
	go func() { defer close(serverDone); _, _ = io.Copy(io.Discard, peer) }()
	id, err := client.Call(ctx, OperationHealth, 1, contextTestDeadline, nil, nil)
	if id != "context-request" || !errors.Is(err, context.Canceled) || writes.Load() != 0 {
		t.Fatalf("canceled dial result: id=%q error=%v frame writes=%d", id, err, writes.Load())
	}
	waitContextTestSignal(t, serverDone, "canceled dial cleanup")
}

func TestClientCallJoinsCancellationClose(t *testing.T) {
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()
	reading, closing, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var readOnce, releaseOnce sync.Once
	var firstClose atomic.Bool
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	client := newContextTestClient(t, func(context.Context, string, string) (net.Conn, error) {
		return &contextTestConn{Conn: conn,
			readHook: func() { readOnce.Do(func() { close(reading) }) },
			closeHook: func() {
				if !firstClose.Swap(true) {
					close(closing)
					<-release
				}
			},
		}, nil
	})
	serverDone := make(chan struct{})
	go func() { defer close(serverDone); _, _ = readFrame(peer, MaxRequestBytes) }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := client.Call(ctx, OperationHealth, 1, contextTestDeadline, nil, nil); done <- err }()
	waitContextTestSignal(t, reading, "response wait")
	cancel()
	waitContextTestSignal(t, closing, "cancellation close")
	select {
	case err := <-done:
		t.Fatalf("Call returned before its cancellation callback finished: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	unblock()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("joined call error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Call did not join its completed cancellation callback")
	}
	waitContextTestSignal(t, serverDone, "joined fixture cleanup")
}
