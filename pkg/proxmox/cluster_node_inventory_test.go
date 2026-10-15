package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func nodeInventoryProbeFixture(t *testing.T, mode, body string, complete bool) *nodeProbeFixture {
	t.Helper()
	f := &nodeProbeFixture{mode: mode}
	f.reject.Store(true)
	f.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/access/ticket":
			f.logins.Add(1)
			if mode == "token" {
				t.Error("token inventory probe logged in")
			}
			if mode == "form" && r.Header.Get("Content-Type") == "application/json" {
				w.WriteHeader(415)
				return
			}
			fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
		case "/api2/json/nodes":
			f.nodes.Add(1)
			if mode == "token" {
				if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pam!pulse=fixture-token" {
					t.Error("token authentication changed")
				}
			} else if r.Header.Get("Cookie") != "PVEAuthCookie=fixture-ticket" {
				t.Error("password authentication changed")
			}
			if f.reject.Load() {
				if !complete {
					w.Header().Set("Content-Length", fmt.Sprint(len(body)+100))
				}
				fmt.Fprint(w, body)
			} else {
				fmt.Fprint(w, `{"data":[{"node":"member","status":"online","cpu":0.25}]}`)
			}
		default:
			t.Errorf("inventory admission left ordinary session/nodes paths: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func TestClusterNodeInventoryRequiresCompleteResponse(t *testing.T) {
	cases := []struct {
		name, body string
		complete   bool
	}{
		{"missing", `{}`, true},
		{"truncated-prefix", `{"data":[{"node":"member"}]}`, false},
		{"duplicate", `{"data":{},"data":[]}`, true},
		{"error", `{"data":null,"errors":{"message":"fixture-private-detail"}}`, true},
	}
	for _, phase := range []string{"startup", "recovery", "selection"} {
		for _, mode := range []string{"token", "json", "form"} {
			for _, tc := range cases {
				t.Run(phase+"/"+mode+"/"+tc.name, func(t *testing.T) {
					f := nodeInventoryProbeFixture(t, mode, tc.body, tc.complete)
					cfg := f.config()
					saved := map[string]string{f.server.URL: cfg.Fingerprint}
					var cc *ClusterClient
					if phase == "startup" {
						sibling := nodeInventoryProbeFixture(t, mode, ``, true)
						sibling.reject.Store(false)
						saved[sibling.server.URL] = sibling.config().Fingerprint
						cc = NewClusterClient("inventory", cfg, []string{f.server.URL, sibling.server.URL}, saved)
						if !cc.GetHealthStatus()[sibling.server.URL] {
							t.Fatal("invalid member blocked independently accepted sibling")
						}
					} else {
						cc = NewClusterClient("inventory", cfg, []string{f.server.URL}, saved)
						if phase == "recovery" {
							cc.nodeHealth[f.server.URL] = false
							cc.lastError[f.server.URL] = "previous failure"
							cc.recoverUnhealthyNodes(context.Background())
						} else if c, err := cc.getHealthyClient(context.Background()); c != nil || err == nil {
							t.Error("invalid first selection installed polling client")
						}
					}
					t.Cleanup(func() {
						cc.mu.RLock()
						defer cc.mu.RUnlock()
						for _, c := range cc.clients {
							if c != nil {
								c.httpClient.CloseIdleConnections()
							}
						}
					})
					health := cc.GetHealthStatusWithErrors()[f.server.URL]
					if health.Healthy || health.LastError == "" || health.LastCheck.IsZero() || strings.Contains(health.LastError, "fixture-private-detail") || cc.clients[f.server.URL] != nil {
						t.Errorf("invalid probe published usable health/client or leaked body: %+v", health)
					}
					if f.nodes.Load() != 1 || f.logins.Load() != f.loginAttempts() {
						t.Fatalf("invalid probe replayed or advanced: nodes=%d logins=%d", f.nodes.Load(), f.logins.Load())
					}
					if checkedClusterFingerprint(t, cc, f.server.URL) != cfg.Fingerprint || saved[f.server.URL] != cfg.Fingerprint || !cc.config.VerifySSL {
						t.Fatal("invalid inventory changed saved trust")
					}
					// Advance only this synthetic endpoint's eligibility for a distinct accepted
					// response. Connected consuming proof below honours the real interval.
					f.reject.Store(false)
					cc.mu.Lock()
					delete(cc.lastHealthCheck, f.server.URL)
					cc.mu.Unlock()
					cc.recoverUnhealthyNodes(context.Background())
					health = cc.GetHealthStatusWithErrors()[f.server.URL]
					if c := cc.clients[f.server.URL]; c == nil || c.config.Timeout != cfg.Timeout || !health.Healthy || health.LastError != "" {
						t.Fatalf("later complete recovery failed: %+v", health)
					}
					if f.nodes.Load() != 2 || f.logins.Load() != 3*f.loginAttempts() {
						t.Fatalf("later recovery lost probe/full-session boundary: nodes=%d logins=%d", f.nodes.Load(), f.logins.Load())
					}
				})
			}
		}
	}
}

func TestClusterNodeInventoryCachedFallbackCannotInventRecovery(t *testing.T) {
	f := nodeInventoryProbeFixture(t, "json", `{}`, true)
	f.reject.Store(false)
	cc := NewClusterClient("inventory", f.config(), []string{f.server.URL}, nil)
	if nodes, err := cc.GetNodes(context.Background()); err != nil || len(nodes) != 1 {
		t.Fatal(nodes, err)
	}
	original := cc.clients[f.server.URL]
	defer original.httpClient.CloseIdleConnections()
	f.reject.Store(true)
	cc.markUnhealthyWithError(f.server.URL, "previous failure")
	if c, err := cc.getHealthyClient(context.Background()); err != nil || c != original {
		t.Fatal("cached fallback unavailable", err)
	}
	before := f.nodes.Load()
	if nodes, err := cc.GetNodes(context.Background()); err == nil || nodes != nil {
		t.Fatal("invalid cached read invented an authoritative empty inventory", nodes, err)
	}
	h := cc.GetHealthStatusWithErrors()[f.server.URL]
	if h.Healthy || h.LastError != "previous failure" || f.nodes.Load() != before+1 {
		t.Fatal("cached invalid read invented recovery or replayed", h)
	}
	f.reject.Store(false)
	if nodes, err := cc.GetNodes(context.Background()); err != nil || len(nodes) != 1 || nodes[0].CPU != 0.25 {
		t.Fatal("distinct complete read did not recover", nodes, err)
	}
	h = cc.GetHealthStatusWithErrors()[f.server.URL]
	if !h.Healthy || h.LastError != "" || cc.clients[f.server.URL] != original || f.nodes.Load() != before+2 {
		t.Fatal("complete read lost normal same-client recovery", h)
	}
	if cc.clients[f.server.URL].config.Timeout != 2*time.Second {
		t.Fatal("full timeout changed")
	}
}
