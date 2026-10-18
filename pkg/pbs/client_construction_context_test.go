package pbs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testClientConstructionHonoursContext(t *testing.T) {
	for _, mode := range []string{"password", "token"} {
		t.Run("already-ended/"+mode, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				http.Error(w, "unexpected fixture request", http.StatusForbidden)
			}))
			defer server.Close()
			pin := sha256.Sum256(server.Certificate().Raw)
			cfg := ClientConfig{Host: server.URL, User: "fixture@pbs", Password: "fixture-password", VerifySSL: true, Fingerprint: hex.EncodeToString(pin[:]), Timeout: 8 * time.Second}
			if mode == "token" {
				cfg.Password, cfg.TokenName, cfg.TokenValue = "", "fixture@pbs!pulse", "fixture-token"
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
		for _, end := range []string{"cancel", "deadline"} {
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
						fmt.Fprint(w, `{"data":{"version":"3.4.2"}}`)
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
							return
						case <-release:
						}
					}
					fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
				}))
				defer server.Close()
				defer releaseOnce.Do(func() { close(release) })
				pin := sha256.Sum256(server.Certificate().Raw)
				cfg := ClientConfig{Host: server.URL, User: "fixture@pbs", Password: "fixture-password", VerifySSL: true, Fingerprint: hex.EncodeToString(pin[:]), Timeout: 8 * time.Second}
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
				} else {
					<-ctx.Done()
				}
				var got result
				select {
				case got = <-done:
				case <-time.After(time.Second):
					t.Error("constructor exceeded ended caller budget")
					releaseOnce.Do(func() { close(release) })
					got = <-done
				}
				if got.client != nil || !errors.Is(got.err, ctx.Err()) {
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
				if _, err := client.GetVersion(context.Background()); err != nil || reads.Load() != 1 || client.config.Timeout != cfg.Timeout {
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

// Cancel while consuming the response, after HTTP has already returned 200 or
// a refusal. This reproduces the constructor race without scheduler timing.
type cancellationAuthBody struct {
	io.Reader
	cancel context.CancelFunc
}

func (r cancellationAuthBody) Read(p []byte) (int, error) {
	r.cancel()
	return r.Reader.Read(p)
}

func testClientCancelledAuthResponseOwnsErrorAndSession(t *testing.T) {
	const good = `{"data":{"ticket":"recovered-ticket","CSRFPreventionToken":"recovered-csrf"}}`
	for _, encoding := range []string{"json", "form"} {
		for _, outcome := range []struct {
			name, body string
			status     int
			readError  bool
		}{
			{"invalid-success", `{}`, http.StatusOK, false},
			{"complete-success", good, http.StatusOK, false},
			{"encoding-refusal", `{}`, http.StatusUnsupportedMediaType, false},
			{"credential-refusal", `{}`, http.StatusForbidden, false},
			{"body-read-failure", good, http.StatusOK, true},
		} {
			t.Run(encoding+"/"+outcome.name, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				calls, recovering := 0, false
				c := &Client{
					baseURL: "https://pbs.fixture.invalid/api2/json",
					config:  ClientConfig{Password: "fixture-password"},
					auth: auth{user: "fixture", realm: "pbs", ticket: "retained-ticket",
						csrfToken: "retained-csrf", expiresAt: time.Now().Add(-time.Hour)},
				}
				c.httpClient = &http.Client{Timeout: 5 * time.Second, Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					calls++
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
					body = cancellationAuthBody{Reader: body, cancel: cancel}
					return &http.Response{StatusCode: outcome.status, Body: io.NopCloser(body), ContentLength: -1}, nil
				})}
				before := c.authSnapshot()
				err := c.authenticate(ctx)
				if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(c.authSnapshot(), before) {
					t.Errorf("ended response lost caller or changed retained session: error=%v session_changed=%t", err, !reflect.DeepEqual(c.authSnapshot(), before))
				}
				want := 1
				if encoding == "form" {
					want = 2
				}
				if calls != want || c.authInFlight != nil {
					t.Errorf("cancellation replayed login or retained owner: calls=%d pending=%t", calls, c.authInFlight != nil)
				}
				recovering = true
				if err := c.authenticate(context.Background()); err != nil || c.auth.ticket != "recovered-ticket" || c.auth.csrfToken != "recovered-csrf" || calls != 2*want {
					t.Errorf("distinct later authentication did not recover coherently: error=%v calls=%d", err, calls)
				}
			})
		}
	}
}
