package agenthelper

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"testing/synctest"
	"time"
)

type budgetPayload struct {
	delay time.Duration
	calls *int
}

func (p budgetPayload) MarshalJSON() ([]byte, error) {
	*p.calls++
	time.Sleep(p.delay)
	return []byte(`{}`), nil
}

// Exercise the actual framing and clock arithmetic, without changing production
// timeouts, sleeping in wall time or trusting a test-only budget calculator.
func TestClientAdvertisesRemainingHelperBudget(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		idDelay, payloadDelay, dialDelay time.Duration
		parentBudget                     time.Duration
		wantMillis                       int64
	}{
		{name: "immediate", wantMillis: 30000},
		{name: "preparation", idDelay: 2 * time.Second, payloadDelay: 3 * time.Second, wantMillis: 25000},
		{name: "dial", dialDelay: 7 * time.Second, wantMillis: 23000},
		{name: "preparation_and_dial", idDelay: 2 * time.Second, payloadDelay: 3 * time.Second, dialDelay: 7 * time.Second, wantMillis: 18000},
		{name: "earlier_parent", idDelay: 2 * time.Second, payloadDelay: 3 * time.Second, dialDelay: 5 * time.Second, parentBudget: 20 * time.Second, wantMillis: 10000},
		{name: "submillisecond", dialDelay: 30*time.Second - 500*time.Microsecond, wantMillis: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx := context.Background()
				budget := 30 * time.Second
				if tc.parentBudget != 0 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, tc.parentBudget)
					defer cancel()
					budget = tc.parentBudget
				}
				wantDeadline := time.Now().Add(budget)
				var dialDeadline, socketDeadline time.Time
				var request Request
				var payloadCalls int
				done := make(chan struct{})
				client, err := NewClient(ClientConfig{
					SocketPath: "/run/pulse-agent/helper.sock",
					NewRequestID: func() (string, error) {
						time.Sleep(tc.idDelay)
						return "remaining-budget", nil
					},
					DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
						dialDeadline, _ = ctx.Deadline()
						time.Sleep(tc.dialDelay)
						conn, peer := net.Pipe()
						go func() {
							defer close(done)
							defer peer.Close()
							frame, err := readFrame(peer, MaxRequestBytes)
							if err != nil {
								t.Error(err)
								return
							}
							request, err = decodeRequest(frame)
							if err != nil {
								t.Error(err)
								return
							}
							_, err = writeFrame(peer, Response{
								ProtocolVersion: ProtocolVersion, RequestID: request.RequestID,
								Operation: request.Operation, OperationVersion: request.OperationVersion,
								Success: true, Result: json.RawMessage(`{"status":"ok"}`),
							}, MaxResponseBytes)
							if err != nil {
								t.Error(err)
							}
						}()
						return &contextTestConn{Conn: conn, deadlineHook: func(d time.Time) { socketDeadline = d }}, nil
					},
				})
				if err != nil {
					t.Fatal(err)
				}
				var result HealthResult
				id, err := client.Call(ctx, OperationHealth, 1, 30*time.Second,
					budgetPayload{delay: tc.payloadDelay, calls: &payloadCalls}, &result)
				<-done
				if err != nil || id != "remaining-budget" || result.Status != "ok" {
					t.Fatalf("Call: id=%q result=%+v error=%v", id, result, err)
				}
				if request.DeadlineMillis != tc.wantMillis {
					t.Errorf("advertised helper budget=%dms, want remaining %dms", request.DeadlineMillis, tc.wantMillis)
				}
				if !dialDeadline.Equal(wantDeadline) || !socketDeadline.Equal(wantDeadline) || payloadCalls != 1 {
					t.Errorf("shared deadline: dial=%s socket=%s want=%s payload encodes=%d", dialDeadline, socketDeadline, wantDeadline, payloadCalls)
				}
				t.Logf("advertised=%dms remaining=%dms elapsed=%s", request.DeadlineMillis, tc.wantMillis, budget-time.Until(wantDeadline))
			})
		})
	}
}

func TestClientExpiredHelperPreparationDoesNotDial(t *testing.T) {
	for _, phase := range []string{"request_id", "payload"} {
		t.Run(phase, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				dials, payloadCalls := 0, 0
				payloadDelay := time.Duration(0)
				if phase == "payload" {
					payloadDelay = 31 * time.Second
				}
				client, err := NewClient(ClientConfig{
					SocketPath: "/run/pulse-agent/helper.sock",
					NewRequestID: func() (string, error) {
						if phase == "request_id" {
							time.Sleep(31 * time.Second)
						}
						return "expired-budget", nil
					},
					DialContext: func(context.Context, string, string) (net.Conn, error) {
						dials++
						return nil, errors.New("unexpected dial")
					},
				})
				if err != nil {
					t.Fatal(err)
				}
				_, err = client.Call(context.Background(), OperationHealth, 1, 30*time.Second,
					budgetPayload{delay: payloadDelay, calls: &payloadCalls}, nil)
				if !errors.Is(err, context.DeadlineExceeded) || dials != 0 || payloadCalls != 1 {
					t.Fatalf("expired preparation: error=%v dials=%d payload encodes=%d", err, dials, payloadCalls)
				}
			})
		})
	}
}

func TestServerHelperBudgetIncludesAdmission(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		requestBudget, parentBudget time.Duration
		peerDelay, frameDelay       time.Duration
		wantElapsed                 time.Duration
		wantCalls                   int
		wantCode                    string
	}{
		{name: "request_limited", requestBudget: 10 * time.Second, peerDelay: 500 * time.Millisecond, frameDelay: 500 * time.Millisecond, wantElapsed: 10 * time.Second, wantCalls: 1, wantCode: ErrorDeadlineExceeded},
		{name: "maximum_limited", requestBudget: 30 * time.Second, peerDelay: 500 * time.Millisecond, frameDelay: time.Second, wantElapsed: 30 * time.Second, wantCalls: 1, wantCode: ErrorDeadlineExceeded},
		{name: "earlier_parent", requestBudget: 10 * time.Second, parentBudget: 5 * time.Second, peerDelay: 500 * time.Millisecond, frameDelay: 500 * time.Millisecond, wantElapsed: 5 * time.Second, wantCalls: 1, wantCode: ErrorDeadlineExceeded},
		{name: "expired_before_dispatch", requestBudget: time.Second, peerDelay: 500 * time.Millisecond, frameDelay: 750 * time.Millisecond, wantElapsed: 1250 * time.Millisecond, wantCode: ErrorDeadlineExceeded},
		{name: "immediate_success", requestBudget: 10 * time.Second, wantElapsed: 100 * time.Millisecond, wantCalls: 1},
		{name: "zero_budget", wantCode: ErrorInvalidRequest},
		{name: "excessive_budget", requestBudget: 31 * time.Second, wantCode: ErrorInvalidRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				started := time.Now()
				var calls int
				var providerDeadline time.Time
				var event AuditEvent
				provider := fakeSMARTProvider(func(ctx context.Context) (json.RawMessage, error) {
					calls++
					providerDeadline, _ = ctx.Deadline()
					if tc.wantCode == "" {
						time.Sleep(100 * time.Millisecond)
						return json.RawMessage(`{"complete":true}`), nil
					}
					<-ctx.Done()
					return nil, ctx.Err()
				})
				server, err := NewServer(ServerConfig{
					AllowedUID: 1000, Registry: NewRegistry(provider, nil),
					MaxOperationTimeout: 30 * time.Second,
					PeerResolver: PeerResolverFunc(func(net.Conn) (Peer, error) {
						time.Sleep(tc.peerDelay)
						return Peer{UID: 1000, PID: 1234}, nil
					}),
					Audit: func(e AuditEvent) { event = e },
				})
				if err != nil {
					t.Fatal(err)
				}
				ctx := context.Background()
				if tc.parentBudget != 0 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, tc.parentBudget)
					defer cancel()
				}
				conn, peer := net.Pipe()
				defer peer.Close()
				done := make(chan struct{})
				go func() { defer close(done); server.HandleConnection(ctx, conn) }()
				request := validRequest(OperationSMARTSnapshot)
				request.DeadlineMillis = tc.requestBudget.Milliseconds()
				// The server starts its deadline before peer resolution; wait for
				// that stage, then hold a partial frame before supplying the rest.
				synctest.Wait()
				frame, err := EncodeRequestFrame(request)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := writeAll(peer, frame[:4]); err != nil {
					t.Fatal(err)
				}
				time.Sleep(tc.frameDelay)
				if _, err := writeAll(peer, frame[4:]); err != nil {
					t.Fatal(err)
				}
				body, readErr := ReadResponseFrame(peer)
				<-done
				// A parent deadline closes the transport as well as the operation.
				if tc.parentBudget == 0 {
					if readErr != nil {
						t.Fatal(readErr)
					}
					response, err := DecodeResponse(body)
					if err != nil {
						t.Fatal(err)
					}
					if tc.wantCode != "" {
						requireErrorCode(t, response, tc.wantCode)
					} else if !response.Success || string(response.Result) != `{"complete":true}` {
						t.Errorf("response=%+v", response)
					}
				}
				if calls != tc.wantCalls || time.Since(started) != tc.wantElapsed || len(server.connectionSlots) != 0 {
					t.Errorf("calls=%d elapsed=%s slots=%d, want calls=%d elapsed=%s", calls, time.Since(started), len(server.connectionSlots), tc.wantCalls, tc.wantElapsed)
				}
				if calls != 0 {
					budget := tc.requestBudget
					if tc.parentBudget != 0 {
						budget = min(budget, tc.parentBudget)
					}
					if !providerDeadline.Equal(started.Add(budget)) {
						t.Errorf("provider deadline=%s want=%s", providerDeadline, started.Add(budget))
					}
				}
				if event.ErrorCode != tc.wantCode || event.Success != (tc.wantCode == "") || event.RequestID != request.RequestID {
					t.Errorf("audit=%+v", event)
				}
				t.Logf("provider calls=%d cooperative lifetime=%s code=%s", calls, time.Since(started), event.ErrorCode)
			})
		})
	}
}

func TestHelperBudgetEndsProviderAtCallerDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started := time.Now()
		var providerDeadline, providerStopped time.Time
		var providerCalls, payloadCalls int
		server, err := NewServer(ServerConfig{
			AllowedUID: 1000, PeerResolver: authorizedResolver(1000),
			Registry: NewRegistry(fakeSMARTProvider(func(ctx context.Context) (json.RawMessage, error) {
				providerCalls++
				providerDeadline, _ = ctx.Deadline()
				<-ctx.Done()
				providerStopped = time.Now()
				return nil, ctx.Err()
			}), nil),
		})
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		client, err := NewClient(ClientConfig{
			SocketPath:   "/run/pulse-agent/helper.sock",
			NewRequestID: func() (string, error) { time.Sleep(time.Second); return "whole-budget", nil },
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				time.Sleep(3 * time.Second)
				conn, peer := net.Pipe()
				firstRead := true
				go func() {
					defer close(done)
					server.HandleConnection(context.Background(), lifecycleConn{Conn: peer, read: func(p []byte) (int, error) {
						if firstRead {
							firstRead = false
							time.Sleep(500 * time.Millisecond)
						}
						return peer.Read(p)
					}})
				}()
				return conn, nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		id, err := client.Call(context.Background(), OperationSMARTSnapshot, 1, 30*time.Second,
			budgetPayload{delay: 2 * time.Second, calls: &payloadCalls}, nil)
		<-done // The provider must not retain a slot after the caller is gone.
		if id != "whole-budget" || !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("bounded call: id=%q error=%v", id, err)
		}
		want := started.Add(30 * time.Second)
		if providerCalls != 1 || payloadCalls != 1 || !providerDeadline.Equal(want) || !providerStopped.Equal(want) || len(server.connectionSlots) != 0 {
			t.Errorf("calls=%d encodes=%d provider deadline=%s stop=%s slots=%d want=%s", providerCalls, payloadCalls, providerDeadline, providerStopped, len(server.connectionSlots), want)
		}
		t.Logf("caller and provider finish after %s, including 3s preparation, 3s dial and 500ms ingress", providerStopped.Sub(started))
	})
}
