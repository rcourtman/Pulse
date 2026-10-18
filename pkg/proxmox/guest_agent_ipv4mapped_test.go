package proxmox

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGuestAgentIPv4MappedIdentity(t *testing.T) {
	for _, pair := range [][2]string{
		{"https://192.0.2.10/api2/json", "https://[::ffff:192.0.2.10]:443/api2/json"},
		{"http://192.0.2.10:8006/tenant/api2/json", "http://[0:0:0:0:0:FFFF:c000:020a]:08006/tenant/api2/json"},
	} {
		a, okA := guestAgentEndpointKey(pair[0])
		b, okB := guestAgentEndpointKey(pair[1])
		if !okA || !okB || a != b {
			t.Errorf("same numeric IPv4 destination has different admission keys: %q / %q", a, b)
		}
		if again, ok := guestAgentEndpointKey(b); !ok || again != b {
			t.Errorf("coordination key is not idempotent: %q / %q", b, again)
		}
	}
	for _, second := range []string{
		"http://[::ffff:192.0.2.10]/tenant/api2/json",
		"https://[::ffff:192.0.2.11]/tenant/api2/json",
		"https://[::ffff:192.0.2.10]:8006/tenant/api2/json",
		"https://[::ffff:192.0.2.10]/Tenant/api2/json",
		"https://[::ffff:192.0.2.10%25ZoneA]/tenant/api2/json",
		"https://[::192.0.2.10]/tenant/api2/json",
		"https://[64:ff9b::c000:20a]/tenant/api2/json",
		"https://pve.example.invalid/tenant/api2/json",
	} {
		a, okA := guestAgentEndpointKey("https://192.0.2.10/tenant/api2/json")
		b, okB := guestAgentEndpointKey(second)
		if !okA || !okB || a == b {
			t.Errorf("independent origin/path/zone collapsed: %q / %q", a, b)
		}
	}
	a, _ := guestAgentEndpointKey("https://[::ffff:192.0.2.10%25ZoneA]/api2/json")
	b, _ := guestAgentEndpointKey("https://[::ffff:192.0.2.10%25zonea]/api2/json")
	if a == b {
		t.Fatal("mapped address lost its case-sensitive zone")
	}
}

// Dial the actual numeric endpoints, not an overridden DialContext. Both URL
// spellings must reach this one IPv4 listener in the offline Linux proof VM.
func mappedEndpointServer(t *testing.T, handler http.Handler) (*httptest.Server, string, string) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	port := listener.Addr().(*net.TCPAddr).Port
	return server, fmt.Sprintf("http://127.0.0.1:%d", port), fmt.Sprintf("http://[::ffff:127.0.0.1]:%d", port)
}

func mappedEndpointClient(t *testing.T, endpoint string) *Client {
	t.Helper()
	client := backupTestClient(t, endpoint)
	client.httpClient.Timeout = 5 * time.Second
	// No external proxy may turn the numeric loopback proof into another route.
	client.httpClient.Transport.(*http.Transport).Proxy = nil
	t.Cleanup(client.httpClient.CloseIdleConnections)
	return client
}

func mappedEndpointReadNames() []string {
	names := make([]string, 0, len(backupAgentReads()))
	for name := range backupAgentReads() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func TestGuestAgentIPv4MappedInflight(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("mapped-first-%t", reverse), func(t *testing.T) {
			isolateGuestAgentCapacity(t)
			entered, finish := make(chan struct{}), make(chan struct{})
			var commands, configs atomic.Int32
			_, ipv4, mapped := mappedEndpointServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/config") {
					configs.Add(1)
					fmt.Fprint(w, `{"data":{}}`)
					return
				}
				if commands.Add(1) == 1 {
					fmt.Fprint(w, `{"data":{"result":`)
					w.(http.Flusher).Flush()
					close(entered)
					<-finish
					fmt.Fprint(w, `[]}}`)
					return
				}
				backupAgentPayload(w, r)
			}))
			if reverse {
				ipv4, mapped = mapped, ipv4
			}
			first, second := mappedEndpointClient(t, ipv4), mappedEndpointClient(t, mapped)
			firstURL, secondURL := first.baseURL, second.baseURL
			done := make(chan error, 1)
			go func() { _, err := first.GetVMFSInfo(context.Background(), "node", 105); done <- err }()
			finished := false
			t.Cleanup(func() {
				if !finished {
					close(finish)
					<-done
				}
			})
			select {
			case <-entered:
			case err := <-done:
				finished = true
				close(finish)
				t.Fatalf("first numeric endpoint did not reach the listener: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("first numeric endpoint did not enter the listener")
			}
			reads := backupAgentReads()
			for _, name := range mappedEndpointReadNames() {
				if err := reads[name](context.Background(), second, 105); GuestAgentDeferredReason(err) != "agent-busy" {
					t.Errorf("%s bypassed the same guest's in-flight body: %v", name, err)
				}
			}
			if commands.Load() != 1 || configs.Load() != 1 {
				t.Errorf("in-flight channel dispatched more work: commands=%d configs=%d, want 1/1", commands.Load(), configs.Load())
			}
			if _, err := second.GetVMAgentInfo(context.Background(), "node", 106); err != nil {
				t.Errorf("independent VM blocked: %v", err)
			}
			close(finish)
			finished = true
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			for _, name := range mappedEndpointReadNames() {
				if err := reads[name](context.Background(), second, 105); err != nil {
					t.Errorf("known completion did not resume %s: %v", name, err)
				}
			}
			if first.baseURL != firstURL || second.baseURL != secondURL || firstURL == secondURL {
				t.Fatal("coordination rewrote a configured request URL")
			}
			t.Logf("same IPv4 listener, unchanged URLs, six busy refusals, other VM and six resumed commands; commands=%d", commands.Load())
		})
	}
}

func TestGuestAgentIPv4MappedUncertainty(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, incomplete := range []bool{false, true} {
			t.Run(fmt.Sprintf("mapped-first-%t/incomplete-%t", reverse, incomplete), func(t *testing.T) {
				isolateGuestAgentCapacity(t)
				var commands, configs atomic.Int32
				_, ipv4, mapped := mappedEndpointServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch {
					case strings.HasSuffix(r.URL.Path, "/nodes"):
						fmt.Fprint(w, `{"data":[]}`)
					case strings.HasSuffix(r.URL.Path, "/status/current"):
						fmt.Fprint(w, `{"data":{"status":"running","cpu":0.25,"diskread":4096}}`)
					case strings.HasSuffix(r.URL.Path, "/config"):
						configs.Add(1)
						fmt.Fprint(w, `{"data":{}}`)
					default:
						commands.Add(1)
						if strings.Contains(r.URL.Path, "/qemu/105/") {
							if incomplete {
								w.Header().Set("Content-Length", "1000")
								fmt.Fprint(w, `{"data":{"result":[]}}`)
							} else {
								w.WriteHeader(http.StatusInternalServerError)
								fmt.Fprint(w, "guest completion unknown")
							}
							return
						}
						backupAgentPayload(w, r)
					}
				}))
				if reverse {
					ipv4, mapped = mapped, ipv4
				}
				first := mappedEndpointClient(t, ipv4)
				want := "agent-completion-unverified"
				if incomplete {
					want = "agent-response-incomplete"
				}
				if _, err := first.GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != want {
					t.Fatalf("original uncertainty not retained: %v", err)
				}
				key, _ := guestAgentEndpointKey(first.baseURL)
				guestAgentGuards.Lock()
				until := guestAgentGuards.entries[guestAgentGuardKey{endpoint: key, vmid: 105}].until
				guestAgentGuards.Unlock()
				before := configs.Load()
				second := mappedEndpointClient(t, mapped)
				reads := backupAgentReads()
				for _, name := range mappedEndpointReadNames() {
					if err := reads[name](context.Background(), second, 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
						t.Errorf("%s bypassed uncertainty through the other numeric spelling: %v", name, err)
					}
				}
				// A separately configured one-endpoint cluster must not need a
				// user-declared alias to observe the same numeric channel's fence.
				cc := NewClusterClient("mapped", ClientConfig{Host: mapped, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second}, []string{mapped}, nil)
				if _, err := cc.GetVMFSInfo(context.Background(), "migrated-node", 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
					t.Errorf("fresh cluster/migrated node bypassed uncertainty: %v", err)
				}
				if commands.Load() != 1 || configs.Load() != before {
					t.Errorf("uncertain channel dispatched follow-up work: commands=%d configs=%d, want 1/%d", commands.Load(), configs.Load(), before)
				}
				guestAgentGuards.Lock()
				retained := guestAgentGuards.entries[guestAgentGuardKey{endpoint: key, vmid: 105}].until
				guestAgentGuards.Unlock()
				if until.IsZero() || retained != until {
					t.Error("follow-up reads changed the original uncertainty deadline")
				}
				if _, err := second.GetVMAgentInfo(context.Background(), "node", 106); err != nil {
					t.Errorf("other VM lost admission: %v", err)
				}
				if status, err := second.GetVMStatus(context.Background(), "node", 105); err != nil || status.CPU != .25 || status.DiskRead != 4096 {
					t.Errorf("ordinary non-QGA status changed: %+v %v", status, err)
				}
				t.Logf("same IPv4 listener; one uncertain command, six method refusals and fresh cluster refusal, original deadline preserved; commands=%d", commands.Load())
			})
		}
	}
}
