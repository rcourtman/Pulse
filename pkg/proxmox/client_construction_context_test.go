package proxmox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testClientConstructionHonoursContext(t *testing.T, endings ...string) {
	if len(endings) == 0 {
		endings = []string{"cancel", "deadline"}
	}
	for _, mode := range []string{"password", "token"} {
		t.Run("already-ended/"+mode, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				http.Error(w, "unexpected fixture request", http.StatusForbidden)
			}))
			defer server.Close()
			pin := sha256.Sum256(server.Certificate().Raw)
			cfg := ClientConfig{Host: server.URL, User: "fixture@pam", Password: "fixture-password", VerifySSL: true, Fingerprint: hex.EncodeToString(pin[:]), Timeout: 8 * time.Second}
			if mode == "token" {
				cfg.Password, cfg.TokenName, cfg.TokenValue = "", "fixture@pam!pulse", "fixture-token"
			}
			for _, end := range []string{"cancel", "deadline"} {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				if end == "deadline" {
					ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
					defer cancel()
				}
				client, err := NewClientWithContext(ctx, cfg)
				if client != nil || !errors.Is(err, ctx.Err()) || requests.Load() != 0 {
					t.Errorf("ended %s constructor: client=%t error=%v requests=%d", end, client != nil, err, requests.Load())
				}
			}
		})
	}
	for _, encoding := range []string{"json", "form"} {
		for _, end := range endings {
			t.Run(encoding+"/"+end, func(t *testing.T) {
				var logins, reads atomic.Int32
				var blocking atomic.Bool
				blocking.Store(true)
				entered, release, requestEnded := make(chan struct{}), make(chan struct{}), make(chan struct{})
				var releaseOnce sync.Once
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.Copy(io.Discard, r.Body)
					w.Header().Set("Content-Type", "application/json")
					if r.URL.Path != "/api2/json/access/ticket" {
						reads.Add(1)
						if !strings.Contains(r.Header.Get("Cookie"), "fixture-ticket") {
							t.Error("later ordinary read lost its session")
						}
						fmt.Fprint(w, `{"data":[{"node":"member","status":"online"}]}`)
						return
					}
					logins.Add(1)
					if encoding == "form" && r.Header.Get("Content-Type") == "application/json" {
						w.WriteHeader(http.StatusUnsupportedMediaType)
						fmt.Fprint(w, `{"message":"fixture requires form"}`)
						return
					}
					if blocking.Load() {
						close(entered)
						select {
						case <-r.Context().Done():
							close(requestEnded)
							// Abort: returning would answer the departed client with an empty 200.
							panic(http.ErrAbortHandler)
						case <-release:
						}
					}
					fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
				}))
				defer server.Close()
				defer releaseOnce.Do(func() { close(release) })
				pin := sha256.Sum256(server.Certificate().Raw)
				cfg := ClientConfig{Host: server.URL, User: "fixture@pam", Password: "fixture-password", VerifySSL: true, Fingerprint: hex.EncodeToString(pin[:]), Timeout: 8 * time.Second}
				resultBound := time.Second
				if end == "client-timeout" {
					cfg.Timeout = time.Second
					resultBound = 5 * time.Second
				}
				ctx, cancel := context.WithCancel(context.Background())
				if end == "deadline" {
					cancel()
					ctx, cancel = context.WithTimeout(context.Background(), time.Second)
				}
				defer cancel()
				type result struct {
					client *Client
					err    error
				}
				done := make(chan result, 1)
				go func() { c, err := NewClientWithContext(ctx, cfg); done <- result{c, err} }()
				select {
				case <-entered:
				case <-time.After(5 * time.Second):
					releaseOnce.Do(func() { close(release) })
					<-done
					t.Fatal("fixture never reached selected login encoding")
				}
				if end == "cancel" {
					cancel()
				} else if end == "deadline" {
					<-ctx.Done()
				}
				var got result
				select {
				case got = <-done:
				case <-time.After(resultBound):
					t.Error("constructor exceeded ended caller budget")
					releaseOnce.Do(func() { close(release) })
					got = <-done
				}
				wantErr := ctx.Err()
				if end == "client-timeout" {
					wantErr = context.DeadlineExceeded
					if ctx.Err() != nil {
						t.Fatal("client timeout ended the independent caller")
					}
				}
				if got.client != nil || !errors.Is(got.err, wantErr) {
					t.Errorf("cancelled construction returned session or lost context: client=%t error=%v", got.client != nil, got.err)
				}
				select {
				case <-requestEnded:
				case <-time.After(time.Second):
					t.Error("cancelled password request remained active")
				}
				want := int32(1)
				if encoding == "form" {
					want = 2
				}
				if logins.Load() != want || reads.Load() != 0 {
					t.Errorf("cancellation replayed or advanced login: logins=%d reads=%d", logins.Load(), reads.Load())
				}
				blocking.Store(false)
				releaseOnce.Do(func() { close(release) })
				laterCtx, laterCancel := context.WithCancel(context.Background())
				client, err := NewClientWithContext(laterCtx, cfg)
				if err != nil {
					laterCancel()
					t.Fatal(err)
				}
				defer client.httpClient.CloseIdleConnections()
				laterCancel() // Cancelling construction later must not poison ordinary reads.
				if _, err := client.GetNodes(context.Background()); err != nil || reads.Load() != 1 || client.config.Timeout != cfg.Timeout {
					t.Errorf("retained client lost full timeout or ordinary independent reads: %v reads=%d", err, reads.Load())
				}
				before := logins.Load()
				bad := cfg
				bad.Fingerprint = strings.Repeat("0", 64)
				if refused, err := NewClientWithContext(context.Background(), bad); refused != nil || err == nil || logins.Load() != before {
					t.Error("context-aware construction weakened the saved TLS pin")
				}
			})
		}
	}
}

// End the login while its response is consumed, after HTTP has already
// returned an unusable 200 or a refusal. This reproduces the constructor race
// without scheduler timing.
type endingAuthBody struct {
	io.Reader
	end func()
}

func (r endingAuthBody) Read(p []byte) (int, error) {
	r.end()
	return r.Reader.Read(p)
}

// The caller's deadline and the client's own login timeout both reach the
// password exchange as a login context ending with DeadlineExceeded. The
// response read ends this one, so no wall-clock timer races the answer.
type endedLoginDeadline struct {
	context.Context
	done <-chan struct{}
}

func (c endedLoginDeadline) Done() <-chan struct{} { return c.done }

func (c endedLoginDeadline) Err() error {
	select {
	case <-c.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func testClientEndedAuthResponseOwnsErrorAndSession(t *testing.T) {
	const good = `{"data":{"ticket":"recovered-ticket","CSRFPreventionToken":"recovered-csrf"}}`
	for _, ending := range []string{"cancel", "deadline"} {
		for _, encoding := range []string{"json", "form"} {
			for _, outcome := range []struct {
				name, body string
				status     int
				readError  bool
			}{
				{"empty-success", ``, http.StatusOK, false},
				{"invalid-success", `{}`, http.StatusOK, false},
				{"encoding-refusal", `{}`, http.StatusUnsupportedMediaType, false},
				{"credential-refusal", `{}`, http.StatusUnauthorized, false},
				{"body-read-failure", good, http.StatusOK, true},
			} {
				t.Run(ending+"/"+encoding+"/"+outcome.name, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					deadline := make(chan struct{})
					end, want := func() { cancel() }, context.Canceled
					if ending == "deadline" {
						end, want = sync.OnceFunc(func() { close(deadline) }), context.DeadlineExceeded
					}
					calls, recovering := 0, false
					var budget time.Time
					c := &Client{
						baseURL: "https://pve.fixture.invalid/api2/json",
						config:  ClientConfig{Password: "fixture-password"},
						auth:    auth{user: "fixture", realm: "pam"},
					}
					c.httpClient = &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
						calls++
						if ending == "cancel" && !recovering {
							// authenticate gives every encoding attempt one login budget.
							deadline, ok := r.Context().Deadline()
							if !ok || (!budget.IsZero() && !deadline.Equal(budget)) {
								t.Error("login attempts lost their shared login budget")
							}
							budget = deadline
						}
						if encoding == "form" && r.Header.Get("Content-Type") == "application/json" {
							return &http.Response{StatusCode: http.StatusUnsupportedMediaType, Body: io.NopCloser(strings.NewReader(`{}`)), ContentLength: -1}, nil
						}
						if recovering {
							return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(good)), ContentLength: -1}, nil
						}
						var body io.Reader = strings.NewReader(outcome.body)
						if outcome.readError {
							body = sessionAdmissionReadError{body}
						}
						return &http.Response{StatusCode: outcome.status, Body: io.NopCloser(endingAuthBody{body, end}), ContentLength: -1}, nil
					})}
					before, generation := c.authSnapshot()
					var err error
					if ending == "deadline" {
						err = c.authenticatePassword(endedLoginDeadline{context.Background(), deadline}, "fixture@pam")
					} else {
						err = c.authenticate(ctx)
					}
					if after, afterGeneration := c.authSnapshot(); !errors.Is(err, want) || after != before || afterGeneration != generation {
						t.Errorf("ended login response lost its ending or changed the session: error=%v", err)
					}
					wantCalls := 1
					if encoding == "form" {
						wantCalls = 2
					}
					if calls != wantCalls {
						t.Errorf("ended login tried another encoding: calls=%d", calls)
					}
					recovering = true
					if err := c.authenticate(context.Background()); err != nil || c.auth.ticket != "recovered-ticket" || c.auth.csrfToken != "recovered-csrf" || calls != 2*wantCalls {
						t.Errorf("distinct later login did not recover: error=%v calls=%d", err, calls)
					}
				})
			}
		}
	}
}
