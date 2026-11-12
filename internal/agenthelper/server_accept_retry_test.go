package agenthelper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

type retryAcceptError struct{ temporary bool }

func (retryAcceptError) Error() string     { return "synthetic accept failure" }
func (retryAcceptError) Timeout() bool     { return false }
func (e retryAcceptError) Temporary() bool { return e.temporary }

// Virtual time measures the real Serve retry path without reducing production
// delays, exhausting descriptors, spinning indefinitely or adding a timer hook.
func testAcceptRetryBackoff(t *testing.T, acceptErr error) {
	t.Helper()
	synctest.Test(t, func(t *testing.T) {
		wantDelays := []time.Duration{
			0, 5 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond,
			40 * time.Millisecond, 80 * time.Millisecond, 160 * time.Millisecond,
			320 * time.Millisecond, 640 * time.Millisecond,
			time.Second, time.Second, time.Second, time.Second,
		}
		var delays []time.Duration
		previous := time.Now()
		terminal := errors.New("synthetic terminal listener failure")
		closed := 0
		listener := lifecycleListener{
			accept: func() (net.Conn, error) {
				delays = append(delays, time.Since(previous))
				previous = time.Now()
				if len(delays) == len(wantDelays) {
					return nil, terminal
				}
				return nil, acceptErr
			},
			close: func() error { closed++; return nil },
		}
		server := newTestServer(t, NewRegistry(nil, nil), authorizedResolver(1000), nil)
		if err := server.Serve(context.Background(), listener); !errors.Is(err, terminal) {
			t.Fatalf("Serve = %v, want terminal accept error", err)
		}
		if !reflect.DeepEqual(delays, wantDelays) {
			t.Errorf("accept retry intervals = %v, want %v", delays, wantDelays)
		}
		if closed != 1 || len(server.connectionSlots) != 0 {
			t.Errorf("terminal cleanup: listener closes=%d, active handlers=%d", closed, len(server.connectionSlots))
		}
		t.Logf("accept intervals=%v", delays)
	})
}

func TestServerAcceptRetryBackoff(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"temporary", retryAcceptError{temporary: true}},
		{"wrapped-temporary", fmt.Errorf("listener wrapper: %w", retryAcceptError{temporary: true})},
		{"net-operation", &net.OpError{Op: "accept", Net: "unix", Err: retryAcceptError{temporary: true}}},
	} {
		t.Run(tc.name, func(t *testing.T) { testAcceptRetryBackoff(t, tc.err) })
	}
}

func TestServerAcceptRetryResetsAfterAcceptance(t *testing.T) {
	for _, saturated := range []bool{false, true} {
		t.Run(map[bool]string{false: "admitted", true: "connection-limit"}[saturated], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				server := newTestServer(t, NewRegistry(nil, nil), authorizedResolver(1000), nil)
				if saturated {
					for range cap(server.connectionSlots) {
						server.connectionSlots <- struct{}{}
						defer server.releaseConnection()
					}
				}
				serverConn, clientConn := net.Pipe()
				defer clientConn.Close()
				closedConn := false
				conn := lifecycleConn{
					Conn:  serverConn,
					read:  func([]byte) (int, error) { return 0, io.EOF },
					write: func(data []byte) (int, error) { return len(data), nil },
					close: func() error { closedConn = true; return serverConn.Close() },
				}
				var delays []time.Duration
				previous := time.Now()
				terminal := errors.New("synthetic terminal listener failure")
				listener := lifecycleListener{
					accept: func() (net.Conn, error) {
						delays = append(delays, time.Since(previous))
						previous = time.Now()
						switch len(delays) {
						case 3:
							return conn, nil
						case 6:
							return nil, terminal
						default:
							return nil, retryAcceptError{temporary: true}
						}
					},
					close: func() error { return nil },
				}
				if err := server.Serve(context.Background(), listener); !errors.Is(err, terminal) {
					t.Fatalf("Serve = %v, want terminal error", err)
				}
				want := []time.Duration{0, 5 * time.Millisecond, 10 * time.Millisecond, 0, 5 * time.Millisecond, 10 * time.Millisecond}
				if !reflect.DeepEqual(delays, want) {
					t.Errorf("retry intervals across accepted connection = %v, want %v", delays, want)
				}
				if !closedConn {
					t.Error("accepted connection was not closed")
				}
			})
		})
	}
}

func TestServerAcceptRetryCancellation(t *testing.T) {
	for _, atCap := range []bool{false, true} {
		for _, deadline := range []bool{false, true} {
			name := fmt.Sprintf("capped=%t/deadline=%t", atCap, deadline)
			t.Run(name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					attempts, wantAttempts := 0, 1
					wantElapsed := time.Millisecond
					if atCap {
						wantAttempts = 9
						wantElapsed += 1275 * time.Millisecond // 5+10+20+40+80+160+320+640.
					}
					ctx, cancel := context.WithCancel(context.Background())
					if deadline {
						cancel()
						ctx, cancel = context.WithTimeout(context.Background(), wantElapsed)
					}
					defer cancel()
					cancelDone := make(chan struct{})
					closed := 0
					listener := lifecycleListener{
						accept: func() (net.Conn, error) {
							attempts++
							if attempts > wantAttempts {
								// Keep the old, broken implementation finite too.
								return nil, errors.New("retry ran before cancellation")
							}
							if attempts == wantAttempts && !deadline {
								go func() { time.Sleep(time.Millisecond); cancel(); close(cancelDone) }()
							}
							return nil, retryAcceptError{temporary: true}
						},
						close: func() error { closed++; return nil },
					}
					server := newTestServer(t, NewRegistry(nil, nil), authorizedResolver(1000), nil)
					started := time.Now()
					err := server.Serve(ctx, listener)
					if err != nil || attempts != wantAttempts || time.Since(started) != wantElapsed {
						t.Errorf("canceled retry: err=%v attempts=%d want=%d elapsed=%s want=%s", err, attempts, wantAttempts, time.Since(started), wantElapsed)
					}
					if closed != 1 || len(server.connectionSlots) != 0 {
						t.Errorf("canceled retry cleanup: closes=%d active=%d", closed, len(server.connectionSlots))
					}
					if !deadline {
						<-cancelDone // Join even when old Serve returns before the timer.
					}
				})
			})
		}
	}
}

func TestServerAcceptRetryKeepsAcceptedOperation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		entered, release := make(chan struct{}), make(chan struct{})
		var operationCtx context.Context
		var calls atomic.Int32
		provider := fakeSMARTProvider(func(ctx context.Context) (json.RawMessage, error) {
			calls.Add(1)
			operationCtx = ctx
			close(entered)
			<-release
			return json.RawMessage(`{"disks":[]}`), nil
		})
		server := newTestServer(t, NewRegistry(provider, nil), authorizedResolver(1000), nil)
		serverConn, clientConn := net.Pipe()
		defer clientConn.Close()
		clientDone := make(chan struct{})
		go func() {
			defer close(clientDone)
			frame, err := EncodeRequestFrame(validRequest(OperationSMARTSnapshot))
			if err != nil {
				t.Error(err)
				return
			}
			if _, err := writeAll(clientConn, frame); err != nil {
				t.Errorf("write accepted request: %v", err)
				return
			}
			payload, err := ReadResponseFrame(clientConn)
			if err != nil {
				t.Errorf("read accepted operation: %v", err)
				return
			}
			response, err := DecodeResponse(payload)
			if err != nil || !response.Success || string(response.Result) != `{"disks":[]}` {
				t.Errorf("temporary accept error lost accepted result: response=%+v err=%v", response, err)
			}
		}()
		attempts := 0
		terminal := errors.New("synthetic terminal listener failure")
		var failedAt time.Time
		listener := lifecycleListener{
			accept: func() (net.Conn, error) {
				attempts++
				switch attempts {
				case 1:
					return serverConn, nil
				case 2:
					<-entered
					failedAt = time.Now()
					return nil, retryAcceptError{temporary: true}
				default:
					if operationCtx.Err() != nil || len(server.connectionSlots) != 1 {
						t.Error("temporary accept error canceled accepted work")
					}
					if elapsed := time.Since(failedAt); elapsed != 5*time.Millisecond {
						t.Errorf("accepted operation retry wait = %s, want 5ms", elapsed)
					}
					close(release)
					<-clientDone
					return nil, terminal
				}
			},
			close: func() error { return nil },
		}
		if err := server.Serve(context.Background(), listener); !errors.Is(err, terminal) {
			t.Errorf("Serve = %v, want terminal error", err)
		}
		if calls.Load() != 1 || len(server.connectionSlots) != 0 {
			t.Error("accepted operation duplicated or not drained")
		}
	})
}

func TestServerAcceptRetryPermanentError(t *testing.T) {
	for _, terminal := range []error{net.ErrClosed, errors.New("synthetic permanent listener failure"), retryAcceptError{}} {
		t.Run(fmt.Sprintf("%T", terminal), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				attempts, closed := 0, 0
				listener := lifecycleListener{
					accept: func() (net.Conn, error) { attempts++; return nil, terminal },
					close:  func() error { closed++; return nil },
				}
				server := newTestServer(t, NewRegistry(nil, nil), authorizedResolver(1000), nil)
				started := time.Now()
				if err := server.Serve(context.Background(), listener); !errors.Is(err, terminal) {
					t.Errorf("permanent failure changed identity: %v", err)
				}
				if attempts != 1 || closed != 1 || time.Since(started) != 0 {
					t.Errorf("permanent error retried: attempts=%d closes=%d elapsed=%s", attempts, closed, time.Since(started))
				}
			})
		})
	}
}
