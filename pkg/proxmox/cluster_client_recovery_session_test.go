package proxmox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Recovery constructs two password sessions: the short connectivity probe and
// the client retained for ordinary polling. A successful first login cannot
// establish that the second was accepted. All credentials and TLS peers here
// are guest-local fixtures, not an installed authentication/recovery attempt.
func testClusterRecoveryRequiresUsableSession(t *testing.T) {
	for _, refusal := range []string{"unauthorised", "forbidden", "malformed", "form-refused"} {
		for _, retained := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/retained-%t", refusal, retained), func(t *testing.T) {
				var failing atomic.Bool
				var logins, nodeReads atomic.Int32
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch r.URL.Path {
					case "/api2/json/access/ticket":
						login := logins.Add(1)
						if failing.Load() && login > 1 {
							switch refusal {
							case "unauthorised":
								w.WriteHeader(http.StatusUnauthorized)
								fmt.Fprint(w, `{"message":"fixture session refused"}`)
							case "forbidden":
								w.WriteHeader(http.StatusForbidden)
								fmt.Fprint(w, `{"message":"fixture session refused"}`)
							case "malformed":
								fmt.Fprint(w, `{"data":`)
							case "form-refused":
								if r.Header.Get("Content-Type") == "application/json" {
									w.WriteHeader(http.StatusUnsupportedMediaType)
								} else {
									w.WriteHeader(http.StatusForbidden)
								}
								fmt.Fprint(w, `{"message":"fixture session refused"}`)
							}
							return
						}
						fmt.Fprintf(w, `{"data":{"ticket":"fixture-ticket-%d","CSRFPreventionToken":"fixture-csrf-%d"}}`, login, login)
					case "/api2/json/nodes":
						nodeReads.Add(1)
						if !strings.Contains(r.Header.Get("Cookie"), "fixture-ticket-") {
							t.Error("ordinary node read lost the accepted fixture session")
						}
						fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
					default:
						t.Errorf("unexpected request %s", r.URL.Path)
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				pin := sha256.Sum256(server.Certificate().Raw)
				cfg := ClientConfig{Host: server.URL, User: "fixture@pam", Password: "fixture-password", VerifySSL: true, Fingerprint: hex.EncodeToString(pin[:]), Timeout: time.Second}
				cc := NewClusterClient("session-recovery", cfg, []string{server.URL}, nil)
				var original *Client
				if retained {
					var err error
					original, err = NewClient(cfg)
					if err != nil {
						t.Fatal(err)
					}
					defer original.httpClient.CloseIdleConnections()
					cc.clients[server.URL] = original
				}
				logins.Store(0)
				failing.Store(true)
				cc.nodeHealth[server.URL] = false
				cc.lastError[server.URL] = "previous connectivity failure"
				cc.recoverUnhealthyNodes(context.Background())

				health := cc.GetHealthStatusWithErrors()[server.URL]
				if health.Healthy || health.LastError != "Authentication failed - check API token or credentials" || health.LastCheck.IsZero() {
					t.Fatalf("failed polling session published recovery or lost the refusal: %+v", health)
				}
				client, exists := cc.clients[server.URL]
				if exists != retained || client != original {
					t.Fatalf("failed replacement changed retained client: exists=%t retained=%t nil=%t", exists, retained, client == nil)
				}
				wantLogins := int32(2)
				if refusal == "form-refused" {
					wantLogins = 3
				}
				if logins.Load() != wantLogins || nodeReads.Load() != 1 || cc.getEndpointFingerprint(server.URL) != cfg.Fingerprint {
					t.Fatalf("refusal replayed, changed trust or lost its probe: logins=%d nodes=%d", logins.Load(), nodeReads.Load())
				}

				// Advance only this fixture's recovery eligibility. No credential,
				// trust, persistent state or uncertainty fence is reset.
				failing.Store(false)
				delete(cc.lastHealthCheck, server.URL)
				cc.recoverUnhealthyNodes(context.Background())
				actual := cc.clients[server.URL]
				if actual == nil || actual == original || actual.config.Timeout != cfg.Timeout {
					t.Fatal("accepted later recovery did not publish a usable full-timeout client")
				}
				defer actual.httpClient.CloseIdleConnections()
				if health := cc.GetHealthStatusWithErrors()[server.URL]; !health.Healthy || health.LastError != "" {
					t.Fatalf("accepted later session did not clear its own failure: %+v", health)
				}
				nodes, err := cc.GetNodes(context.Background())
				if err != nil || len(nodes) != 1 || nodes[0].Node != "node1" || nodeReads.Load() != 3 {
					t.Fatalf("ordinary polling did not use recovered session: %+v %v reads=%d", nodes, err, nodeReads.Load())
				}
			})
		}
	}
}
