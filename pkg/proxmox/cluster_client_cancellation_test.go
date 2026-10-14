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

// The public read/recovery paths must not manufacture a password session after
// their caller has stopped. These peers and credentials are local fixtures.
func testClusterCancelledCallerStartsNoSession(t *testing.T) {
	for _, authMode := range []string{"password", "token"} {
		t.Run(authMode, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path == "/api2/json/access/ticket" {
					fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
				} else {
					fmt.Fprint(w, `{"data":[]}`)
				}
			}))
			defer server.Close()
			pin := sha256.Sum256(server.Certificate().Raw)
			cfg := ClientConfig{Host: server.URL, User: "fixture@pam", Password: "fixture-password", VerifySSL: true, Fingerprint: hex.EncodeToString(pin[:]), Timeout: 8 * time.Second}
			if authMode == "token" {
				cfg.Password, cfg.TokenName, cfg.TokenValue = "", "fixture@pam!pulse", "fixture-token"
			}
			cc := NewClusterClient("cancelled", cfg, []string{server.URL}, nil)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := cc.GetNodes(ctx); !errors.Is(err, context.Canceled) {
				t.Errorf("cancelled ordinary read = %v, want context cancellation", err)
			}
			if requests.Load() != 0 || len(cc.clients) != 0 || !cc.nodeHealth[server.URL] || cc.lastError[server.URL] != "" {
				t.Errorf("cancelled caller changed session/health: requests=%d clients=%d healthy=%t error=%q", requests.Load(), len(cc.clients), cc.nodeHealth[server.URL], cc.lastError[server.URL])
			}
		})
	}
}

func testClusterSessionConstructionHonoursCallerBudget(t *testing.T) {
	for _, path := range []string{"selection", "recovery"} {
		for _, stage := range []string{"first-login", "probe", "full-login"} {
			for _, end := range []string{"cancel", "deadline"} {
				t.Run(path+"/"+stage+"/"+end, func(t *testing.T) {
					var logins, reads atomic.Int32
					entered, release, requestEnded := make(chan struct{}), make(chan struct{}), make(chan struct{})
					var enteredOnce, releaseOnce sync.Once
					var block atomic.Bool
					block.Store(true)
					server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						_, _ = io.Copy(io.Discard, r.Body)
						w.Header().Set("Content-Type", "application/json")
						login := r.URL.Path == "/api2/json/access/ticket"
						var ordinal int32
						if login {
							ordinal = logins.Add(1)
						} else {
							ordinal = reads.Add(1)
							if r.URL.Path != "/api2/json/nodes" || !strings.Contains(r.Header.Get("Cookie"), "fixture-ticket-") {
								t.Errorf("non-ordinary or unauthenticated fixture read: %s", r.URL.Path)
							}
						}
						target := (stage == "first-login" && login && ordinal == 1) || (stage == "probe" && !login && ordinal == 1) || (stage == "full-login" && login && ordinal == 2)
						if block.Load() && target {
							enteredOnce.Do(func() { close(entered) })
							select {
							case <-r.Context().Done():
								close(requestEnded)
								return
							case <-release:
							}
						}
						if login {
							fmt.Fprintf(w, `{"data":{"ticket":"fixture-ticket-%d","CSRFPreventionToken":"fixture-csrf-%d"}}`, ordinal, ordinal)
						} else {
							fmt.Fprint(w, `{"data":[{"node":"member","status":"online"}]}`)
						}
					}))
					defer server.Close()
					defer releaseOnce.Do(func() { close(release) })
					pin := sha256.Sum256(server.Certificate().Raw)
					cfg := ClientConfig{Host: server.URL, User: "fixture@pam", Password: "fixture-password", VerifySSL: true, Fingerprint: hex.EncodeToString(pin[:]), Timeout: 8 * time.Second}
					cc := NewClusterClient("budget", cfg, []string{server.URL}, nil)
					cc.nodeHealth[server.URL] = path == "selection"
					cc.lastError[server.URL] = "previous fixture outage"
					ctx, cancel := context.WithCancel(context.Background())
					if end == "deadline" {
						cancel()
						ctx, cancel = context.WithTimeout(context.Background(), time.Second)
					}
					defer cancel()
					done := make(chan error, 1)
					go func() {
						if path == "recovery" {
							cc.recoverUnhealthyNodes(ctx)
							done <- nil
						} else {
							_, err := cc.GetNodes(ctx)
							done <- err
						}
					}()
					select {
					case <-entered:
					case <-time.After(5 * time.Second):
						releaseOnce.Do(func() { close(release) })
						<-done
						t.Fatal("fixture did not reach selected session phase")
					}
					if end == "cancel" {
						cancel()
					} else {
						<-ctx.Done()
					}
					var err error
					select {
					case err = <-done:
					case <-time.After(time.Second):
						t.Error("session construction outlived the ended caller by over one second")
						releaseOnce.Do(func() { close(release) })
						err = <-done // Join the old implementation too; no leaked fixture worker.
					}
					if path == "selection" && !errors.Is(err, ctx.Err()) {
						t.Errorf("selection lost caller context: %v, want %v", err, ctx.Err())
					}
					select {
					case <-requestEnded:
					case <-time.After(time.Second):
						t.Error("ended caller left its fixture request active")
					}
					if len(cc.clients) != 0 || cc.nodeHealth[server.URL] != (path == "selection") || cc.lastError[server.URL] != "previous fixture outage" || checkedClusterFingerprint(t, cc, server.URL) != cfg.Fingerprint {
						t.Errorf("cancelled construction published a session/recovery or changed prior trust/error: clients=%d healthy=%t error=%q", len(cc.clients), cc.nodeHealth[server.URL], cc.lastError[server.URL])
					}
					wantLogins, wantReads := int32(1), int32(0)
					if stage == "probe" || stage == "full-login" {
						wantReads = 1
						if stage == "full-login" {
							wantLogins = 2
						}
					}
					if logins.Load() != wantLogins || reads.Load() != wantReads {
						t.Errorf("ended construction dispatched another phase: logins=%d reads=%d", logins.Load(), reads.Load())
					}
					// A later ordinary caller can recover under the unchanged pin.
					block.Store(false)
					releaseOnce.Do(func() { close(release) })
					cc.nodeHealth[server.URL] = false
					delete(cc.lastHealthCheck, server.URL) // Only advance this fixture's eligibility.
					cc.recoverUnhealthyNodes(context.Background())
					if client := cc.clients[server.URL]; client == nil || client.config.Timeout != cfg.Timeout || !cc.nodeHealth[server.URL] || cc.lastError[server.URL] != "" {
						t.Fatal("ordinary later recovery did not publish its accepted full-timeout session")
					}
					defer cc.clients[server.URL].httpClient.CloseIdleConnections()
					if nodes, err := cc.GetNodes(context.Background()); err != nil || len(nodes) != 1 || nodes[0].Node != "member" {
						t.Fatalf("later recovered session cannot serve ordinary reads: %+v %v", nodes, err)
					}
				})
			}
		}
	}
}
