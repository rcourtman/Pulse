package proxmox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A complete HTTP error is not necessarily a completed QGA command: an
// intermediary may fail after the command was accepted. Error text cannot
// turn a gateway status into a definitive refusal.
func TestGuestAgentHTTPFailureDefersEveryRead(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"request-timeout", 408, "request did not finish"},
		{"plain-server-error", 500, `{"data":null}`},
		{"gateway", 502, "upstream unavailable"},
		{"service-unavailable", 503, "temporary failure"},
		{"gateway-deadline", 504, "<html>request did not finish</html>"},
		{"node-proxy", 595, "no ticket"},
		{"unexplained-not-implemented", 501, "upstream unavailable"},
		{"gateway-quoting-refusal", 502, "upstream API error 403: permission denied"},
		{"gateway-quoting-unsupported", 502, "unsupported command: guest-get-osinfo"},
		{"server-quoting-refusal", 500, "upstream API error 403: permission denied"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for name, first := range backupAgentReads() {
				t.Run(name, func(t *testing.T) {
					var calls atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						switch {
						case strings.HasSuffix(r.URL.Path, "/config"):
							fmt.Fprint(w, `{"data":{}}`)
						case strings.HasSuffix(r.URL.Path, "/status/current"):
							fmt.Fprint(w, `{"data":{"status":"running","cpu":0.25,"diskread":4096}}`)
						default:
							if calls.Add(1) == 1 {
								w.WriteHeader(tc.status)
								fmt.Fprint(w, tc.body)
								return
							}
							backupAgentPayload(w, r)
						}
					}))
					defer server.Close()
					c := backupTestClient(t, server.URL)
					err := first(context.Background(), c, 105)
					if !errors.Is(err, ErrGuestAgentDeferred) || GuestAgentDeferredReason(err) != "agent-completion-unverified" {
						t.Errorf("unknown completion = %v, want completion-unverified deferral", err)
					}
					// A separately constructed diagnostic cannot follow with any command.
					diagnostic := backupTestClient(t, server.URL)
					for later, read := range backupAgentReads() {
						if err := read(context.Background(), diagnostic, 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
							t.Errorf("next %s = %v, want VM-wide cooldown", later, err)
						}
					}
					status, err := c.GetVMStatus(context.Background(), "node", 105)
					if err != nil || status.CPU != 0.25 || status.DiskRead != 4096 {
						t.Errorf("live non-agent counters lost: %+v %v", status, err)
					}
					if calls.Load() != 1 {
						t.Errorf("wire guest commands = %d, want one", calls.Load())
					}
				})
			}
		})
	}
}

func TestGuestAgentHTTPFailureDoesNotFailOver(t *testing.T) {
	var calls atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/nodes"):
			fmt.Fprint(w, `{"data":[]}`)
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, `{"data":{}}`)
		default:
			calls.Add(1)
			w.WriteHeader(502)
			fmt.Fprint(w, "upstream unavailable")
		}
	})
	a, b := httptest.NewServer(handler), httptest.NewServer(handler)
	defer a.Close()
	defer b.Close()
	cc := NewClusterClient("http-uncertainty", ClientConfig{Host: a.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture"}, []string{a.URL, b.URL}, nil)
	if _, err := cc.GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-completion-unverified" {
		t.Errorf("first command not deferred: %v", err)
	}
	for _, endpoint := range []string{a.URL, b.URL} {
		if _, err := backupTestClient(t, endpoint).GetVMAgentInfo(context.Background(), "migrated", 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
			t.Errorf("alias escaped cooldown: %v", err)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("cluster wire commands = %d, want one", calls.Load())
	}
	for endpoint, healthy := range cc.GetHealthStatus() {
		if !healthy {
			t.Errorf("VM error poisoned endpoint %s", endpoint)
		}
	}
}

func TestGuestAgentHTTPExplicitRefusalsRemainErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"invalid-request", 400, "invalid argument"},
		{"unauthorized", 401, "unauthorized"},
		{"forbidden", 403, "permission denied"},
		{"missing-command", 404, "command not found"},
		{"rate-limited", 429, "too many requests"},
		{"unsupported-plain", 500, "unsupported command: guest-get-osinfo"},
		{"unsupported-message", 500, `{"data":null,"message":"unsupported command: guest-get-osinfo"}`},
		{"unsupported-errors", 500, `{"data":null,"errors":{"message":"unsupported command: guest-get-osinfo"}}`},
		{"missing-os-release", 500, `{"errors":{"message":"guest agent command failed: Failed to open file '/etc/os-release': No such file or directory"}}`},
		{"missing-usr-os-release", 500, "guest agent command failed: Failed to open file '/usr/lib/os-release': No such file or directory"},
		{"missing-os-release-qmp-wrapper", 500, "VM 105 qmp command 'guest-get-osinfo' failed - Failed to open file '/etc/os-release': No such file or directory"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/config") {
					fmt.Fprint(w, `{"data":{}}`)
					return
				}
				if calls.Add(1) == 1 {
					w.WriteHeader(tc.status)
					fmt.Fprint(w, tc.body)
					return
				}
				backupAgentPayload(w, r)
			}))
			defer server.Close()
			c := backupTestClient(t, server.URL)
			if _, err := c.GetVMAgentInfo(context.Background(), "node", 105); err == nil || errors.Is(err, ErrGuestAgentDeferred) || !strings.Contains(err.Error(), fmt.Sprintf("API error %d", tc.status)) {
				t.Fatalf("explicit refusal lost: %v", err)
			}
			if _, err := c.GetVMFSInfo(context.Background(), "node", 105); err != nil {
				t.Fatalf("known refusal poisoned healthy command: %v", err)
			}
			if calls.Load() != 2 {
				t.Errorf("refused/healthy commands = %d", calls.Load())
			}
		})
	}
}

func TestGuestAgentHTTPAmbiguousFailuresDoNotClearAdmission(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"wrong-command", "unsupported command: guest-get-osinfo"},
		{"proxy-prefix", "upstream rejected: unsupported command: guest-get-fsinfo"},
		{"wrong-vm", "VM 106 qmp command 'guest-get-fsinfo' failed - unsupported command: guest-get-fsinfo"},
		{"conflicting-messages", `{"message":"unsupported command: guest-get-fsinfo","errors":{"message":"upstream unavailable"}}`},
		{"duplicate-message", `{"message":"upstream unavailable","message":"unsupported command: guest-get-fsinfo"}`},
		{"escaped-duplicate-message", `{"message":"upstream unavailable","\u006dessage":"unsupported command: guest-get-fsinfo"}`},
		{"duplicate-errors", `{"errors":{"message":"upstream unavailable"},"errors":{"message":"unsupported command: guest-get-fsinfo"}}`},
		{"duplicate-nested-message", `{"errors":{"message":"upstream unavailable","message":"unsupported command: guest-get-fsinfo"}}`},
		{"success-and-error", `{"data":{"result":[]},"message":"unsupported command: guest-get-fsinfo"}`},
		{"unrecognised-detail", `{"errors":{"message":"unsupported command: guest-get-fsinfo","other":"unavailable"}}`},
		{"trailing-value", `{"message":"unsupported command: guest-get-fsinfo"} {}`},
		{"truncated-json", `{"message":"unsupported command: guest-get-fsinfo"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/config") {
					fmt.Fprint(w, `{"data":{}}`)
					return
				}
				calls.Add(1)
				w.WriteHeader(500)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			c := backupTestClient(t, server.URL)
			if _, err := c.GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-completion-unverified" {
				t.Errorf("ambiguous error released admission: %v", err)
			}
			if _, err := c.GetVMAgentInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
				t.Errorf("ambiguous error allowed next command: %v", err)
			}
			if calls.Load() != 1 {
				t.Errorf("wire commands = %d, want one", calls.Load())
			}
		})
	}
}

func TestGuestAgentHTTPTerminalFailuresRemainCommandBound(t *testing.T) {
	cases := []struct{ name, command, body string }{
		{"fs-not-found", "get-fsinfo", "The command guest-get-fsinfo has not been found"},
		{"network-not-supported", "network-get-interfaces", "unsupported command: guest-network-get-interfaces"},
		{"version-not-supported", "info", "unsupported command: guest-info"},
		{"file-read-not-supported", "file-read", "unsupported command: guest-file-read"},
		{"pve-wrapper", "get-fsinfo", "VM 105 qmp command 'guest-get-fsinfo' failed - unsupported command: guest-get-fsinfo"},
		{"not-running", "get-fsinfo", `{"data":null,"message":"QEMU guest agent is not running"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/config") {
					fmt.Fprint(w, `{"data":{}}`)
					return
				}
				if calls.Add(1) == 1 {
					w.WriteHeader(500)
					fmt.Fprint(w, tc.body)
					return
				}
				backupAgentPayload(w, r)
			}))
			defer server.Close()
			c := backupTestClient(t, server.URL)
			if _, err := c.get(context.Background(), "/nodes/node/qemu/105/agent/"+tc.command); err == nil || errors.Is(err, ErrGuestAgentDeferred) {
				t.Fatalf("known terminal failure lost: %v", err)
			}
			if _, err := c.GetVMFSInfo(context.Background(), "node", 105); err != nil {
				t.Fatalf("known completed command blocked healthy command: %v", err)
			}
			if calls.Load() != 2 {
				t.Errorf("refused/healthy commands = %d", calls.Load())
			}
		})
	}
}

func TestGuestAgentHTTPOrdinaryRequestPreservesStatusAndError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(502)
		fmt.Fprint(w, "API error 403: permission denied")
	}))
	defer server.Close()
	c := backupTestClient(t, server.URL)
	_, err := c.get(context.Background(), "/nodes")
	var response *apiResponseError
	if !errors.As(err, &response) || response.statusCode != 502 || response.guestCommandRejected || err.Error() != "API error 502: API error 403: permission denied" {
		t.Fatalf("wire status/ordinary diagnostic lost: %v", err)
	}
	if errors.Is(err, ErrGuestAgentDeferred) {
		t.Fatal("ordinary API acquired guest uncertainty")
	}
}

func TestGuestAgentHTTPUncertaintyExpiresAndKeepsOtherVMsIndependent(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(502)
			fmt.Fprint(w, "upstream unavailable")
			return
		}
		backupAgentPayload(w, r)
	}))
	defer server.Close()
	c := backupTestClient(t, server.URL)
	if _, err := c.GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-completion-unverified" {
		t.Fatal(err)
	}
	if _, err := c.GetVMFSInfo(context.Background(), "node", 106); err != nil {
		t.Fatalf("other VM blocked: %v", err)
	}
	if _, err := c.GetVMAgentInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
		t.Fatal(err)
	}
	// Advance only this synthetic cooldown, not wall time or a native target.
	guestAgentGuards.Lock()
	key := guestAgentGuardKey{endpoint: c.baseURL, vmid: 105}
	entry, ok := guestAgentGuards.entries[key]
	if ok {
		entry.until = time.Now().Add(-time.Second)
		guestAgentGuards.entries[key] = entry
	}
	guestAgentGuards.Unlock()
	if !ok {
		t.Fatal("uncertain VM had no retained admission")
	}
	if _, err := c.GetVMFSInfo(context.Background(), "node", 105); err != nil {
		t.Fatalf("unlocked completed response did not resume: %v", err)
	}
	guestAgentGuards.Lock()
	_, retained := guestAgentGuards.entries[key]
	guestAgentGuards.Unlock()
	if retained {
		t.Fatal("healthy resumption retained cooldown")
	}
	if calls.Load() != 3 {
		t.Errorf("wire commands = %d, want uncertain/other/resumed", calls.Load())
	}
}
