package proxmox

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGuestAgentEndpointKeyIdentity(t *testing.T) {
	for _, tc := range []struct{ first, second string }{
		{"https://PVE.Example.invalid/api2/json", "https://pve.example.invalid/api2/json"},
		{"https://pve.example.invalid:443/api2/json", "https://pve.example.invalid/api2/json"},
		{"http://pve.example.invalid:00080/api2/json", "http://pve.example.invalid/api2/json"},
		{"https://pve.example.invalid:08006/api2/json", "https://pve.example.invalid:8006/api2/json"},
		{"https://[2001:0DB8:0:0:0:0:0:1]:443/api2/json", "https://[2001:db8::1]/api2/json"},
		{"https://[fe80:0:0:0:0:0:0:1%25ZoneA]/api2/json", "https://[fe80::1%25ZoneA]/api2/json"},
	} {
		t.Run(tc.first, func(t *testing.T) {
			first, ok1 := guestAgentEndpointKey(tc.first)
			second, ok2 := guestAgentEndpointKey(tc.second)
			if !ok1 || !ok2 || first != second {
				t.Fatalf("equivalent key differs: %q/%q valid=%t/%t", first, second, ok1, ok2)
			}
			if again, ok := guestAgentEndpointKey(first); !ok || again != first {
				t.Fatal("coordination key is not idempotent")
			}
		})
	}
	for _, tc := range []struct{ first, second string }{
		{"https://pve.example.invalid/api2/json", "http://pve.example.invalid/api2/json"},
		{"https://pve.example.invalid/api2/json", "https://pve.example.invalid:8006/api2/json"},
		{"https://pve.example.invalid/api2/json", "https://other.example.invalid/api2/json"},
		{"https://pve.example.invalid/tenantA/api2/json", "https://pve.example.invalid/tenanta/api2/json"},
		{"https://pve.example.invalid/tenantA/api2/json", "https://pve.example.invalid/tenantB/api2/json"},
		{"https://[fe80::1%25ZoneA]/api2/json", "https://[fe80::1%25zonea]/api2/json"},
	} {
		t.Run("distinct-"+tc.second, func(t *testing.T) {
			first, ok1 := guestAgentEndpointKey(tc.first)
			second, ok2 := guestAgentEndpointKey(tc.second)
			if !ok1 || !ok2 || first == second {
				t.Fatal("independent endpoint identity was collapsed")
			}
		})
	}
	for _, endpoint := range []string{"", "pve.example.invalid", "ftp://pve.invalid/api2/json", "https://user:password@pve.invalid/api2/json", "https://pve.invalid:0/api2/json", "https://pve.invalid:65536/api2/json", "https://pve.invalid/api2/json?secret=value", "https://pve.invalid/api2/json#fragment"} {
		if _, ok := guestAgentEndpointKey(endpoint); ok {
			t.Error("invalid endpoint admitted")
		}
		if release, err := acquireGuestAgent(endpoint, 105); release != nil || GuestAgentDeferredReason(err) != "invalid-guest-key" {
			t.Errorf("invalid endpoint did not fail closed: %v", err)
		}
	}
}

// Preserve the real request authority/path and the normal *http.Transport
// no-replay policy, but dial only this test's guest-local loopback fixture.
func equivalentEndpointTestClient(t *testing.T, endpoint string, server *httptest.Server) *Client {
	t.Helper()
	c := backupTestClient(t, endpoint)
	transport := c.httpClient.Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	c.httpClient.Transport = transport
	c.httpClient.Timeout = 5 * time.Second
	t.Cleanup(c.httpClient.CloseIdleConnections)
	return c
}

func TestGuestAgentEquivalentEndpointsFenceInflight(t *testing.T) {
	for _, tc := range []struct{ name, first, second string }{
		{"dns-case", "http://PVE.Case.invalid/tenant", "http://pve.case.invalid/tenant"},
		{"default-port", "http://pve.port.invalid/tenant", "http://pve.port.invalid:00080/tenant"},
		{"ipv6", "http://[2001:db8::101]/tenant", "http://[2001:0DB8:0:0:0:0:0:101]:80/tenant"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entered, finish := make(chan struct{}), make(chan struct{})
			finished := false
			var commands, configs atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasPrefix(r.URL.Path, "/tenant/api2/json/") {
					t.Error("coordination changed request base path")
				}
				if strings.HasSuffix(r.URL.Path, "/config") {
					configs.Add(1)
					fmt.Fprint(w, `{"data":{}}`)
					return
				}
				if commands.Add(1) == 1 {
					// Admission must outlive headers and an incomplete body.
					fmt.Fprint(w, `{"data":{"result":`)
					w.(http.Flusher).Flush()
					close(entered)
					<-finish
					fmt.Fprint(w, `[]}}`)
					return
				}
				backupAgentPayload(w, r)
			}))
			defer server.Close()
			first := equivalentEndpointTestClient(t, tc.first, server)
			second := equivalentEndpointTestClient(t, tc.second, server)
			firstURL, secondURL := first.baseURL, second.baseURL
			done := make(chan error, 1)
			go func() { _, err := first.GetVMFSInfo(context.Background(), "node", 105); done <- err }()
			<-entered
			defer func() {
				if !finished {
					close(finish)
					<-done
				}
			}()
			for name, read := range backupAgentReads() {
				if err := read(context.Background(), second, 105); GuestAgentDeferredReason(err) != "agent-busy" {
					t.Errorf("equivalent %s bypassed in-flight body: %v", name, err)
				}
			}
			if commands.Load() != 1 || configs.Load() != 1 {
				t.Errorf("same VM sent follow-up command/config reads: commands=%d configs=%d", commands.Load(), configs.Load())
			}
			if _, err := second.GetVMAgentInfo(context.Background(), "node", 106); err != nil {
				t.Errorf("independent VM blocked: %v", err)
			}
			close(finish)
			finished = true
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			for name, read := range backupAgentReads() {
				if err := read(context.Background(), second, 105); err != nil {
					t.Errorf("completed VM did not resume %s: %v", name, err)
				}
			}
			if first.baseURL != firstURL || second.baseURL != secondURL || firstURL == secondURL {
				t.Fatal("canonical coordination changed the configured wire URL")
			}
			t.Logf("wire commands=%d; equivalent busy reads=6, independent VM and completed resumption preserved", commands.Load())
		})
	}
}

func TestGuestAgentEquivalentEndpointsRetainUncertainty(t *testing.T) {
	var commands atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
		commands.Add(1)
		if strings.Contains(r.URL.Path, "/qemu/105/") {
			w.Header().Set("Content-Length", "1000")
			fmt.Fprint(w, `{"data":{"result":[]}}`)
			return
		}
		backupAgentPayload(w, r)
	}))
	defer server.Close()
	first := equivalentEndpointTestClient(t, "http://PVE.Uncertain.invalid", server)
	second := equivalentEndpointTestClient(t, "http://pve.uncertain.invalid:80", server)
	if _, err := first.GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-response-incomplete" {
		t.Fatal(err)
	}
	for name, read := range backupAgentReads() {
		if err := read(context.Background(), second, 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
			t.Errorf("equivalent %s escaped uncertainty: %v", name, err)
		}
	}
	if got := commands.Load(); got != 1 {
		t.Errorf("uncertain command caused %d wire commands, want one", got)
	}
	if _, err := second.GetVMAgentInfo(context.Background(), "node", 106); err != nil {
		t.Fatal(err)
	}
	// Other endpoint identity remains independent of this VM's uncertainty.
	other := equivalentEndpointTestClient(t, "http://different.uncertain.invalid:80", server)
	if _, err := other.GetVMAgentInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-response-incomplete" {
		t.Fatalf("independent endpoint could not send its own VM command: %v", err)
	}
}

func TestGuestAgentEquivalentEndpointAliasRegistration(t *testing.T) {
	// Register while a standalone spelling already has an in-flight command.
	first := "https://NODE.Alias.invalid:443/tenant/api2/json"
	release, err := acquireGuestAgent(first, 105)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { release(false) }()
	registerGuestAgentEndpoints("https://node.alias.invalid/tenant", []string{"https://OTHER.Alias.invalid:443/tenant"})
	registerGuestAgentEndpoints("https://other.alias.invalid/tenant", []string{"https://third.alias.invalid/tenant"})
	for _, endpoint := range []string{"https://other.alias.invalid/tenant/api2/json", "https://THIRD.alias.invalid:443/tenant/api2/json"} {
		if next, err := acquireGuestAgent(endpoint, 105); GuestAgentDeferredReason(err) != "agent-busy" {
			if next != nil {
				next(false)
			}
			t.Errorf("new/transitive alias bypassed active VM: %v", err)
		}
	}
	for _, endpoint := range []string{"https://node.alias.invalid/tenantB/api2/json", "https://node.alias.invalid:8006/tenant/api2/json", "http://node.alias.invalid/tenant/api2/json"} {
		next, err := acquireGuestAgent(endpoint, 105)
		if err != nil {
			t.Errorf("distinct source blocked: %v", err)
			continue
		}
		next(false)
	}
	// A known completion clears every equivalent key, not just its spelling.
	release(false)
	release = func(bool) {}
	next, err := acquireGuestAgent("https://THIRD.alias.invalid:443/tenant/api2/json", 105)
	if err != nil {
		t.Fatal(err)
	}
	next(true)
	if again, err := acquireGuestAgent(first, 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
		if again != nil {
			again(false)
		}
		t.Fatalf("alias cooldown did not reach original spelling: %v", err)
	}
	// Advance only the existing test-owned cooldown, not a guest or wall clock.
	guestAgentGuards.Lock()
	for _, endpoint := range guestAgentGuards.aliases["https://node.alias.invalid/tenant/api2/json"] {
		key := guestAgentGuardKey{endpoint: endpoint, vmid: 105}
		entry := guestAgentGuards.entries[key]
		entry.until = time.Now().Add(-time.Second)
		guestAgentGuards.entries[key] = entry
	}
	guestAgentGuards.Unlock()
	next, err = acquireGuestAgent(first, 105)
	if err != nil {
		t.Fatal(err)
	}
	next(false)
}
