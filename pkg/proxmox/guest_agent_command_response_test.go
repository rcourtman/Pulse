package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func commandReplyTestClient(t *testing.T, host string) *Client {
	t.Helper()
	c := backupTestClient(t, host)
	// This fixture checks request counts and admission, not listener latency.
	c.httpClient.Timeout = 15 * time.Second
	t.Cleanup(c.httpClient.CloseIdleConnections)
	return c
}

func guestReplyFamily(reader string) string {
	switch reader {
	case "os":
		return "os"
	case "version":
		return "version"
	case "filesystem":
		return "filesystem"
	case "network":
		return "network"
	default:
		return "file"
	}
}

// These are valid, synthetic replies for another existing command, not a
// native command-ID collision replay. A generic type/envelope check accepts
// them; the actual HTTP reader must retain uncertainty before releasing work.
func TestGuestAgentForeignReplyDefersEveryReader(t *testing.T) {
	replies := map[string]string{
		"os":         `{"data":{"id":"linux","name":"Linux"}}`,
		"version":    `{"data":{"result":{"version":"1.0","supported_commands":[]}}}`,
		"filesystem": `{"data":{"result":[{"name":"root","mountpoint":"/","total-bytes":1024,"used-bytes":512}]}}`,
		"network":    `{"data":{"result":[{"name":"eth0","hardware-address":"02:00:00:00:00:01","ip-addresses":[]}]}}`,
		"file":       `{"data":{"content":"MemTotal: 1024 kB\nMemAvailable: 512 kB\n","truncated":false}}`,
		"error":      `{"data":{"error":{"class":"GenericError","desc":"synthetic error"}}}`,
	}
	for reader, read := range backupAgentReads() {
		for family, body := range replies {
			if guestReplyFamily(reader) == family {
				continue
			}
			t.Run(reader+"/"+family, func(t *testing.T) {
				if !guestAgentSuccessResponse([]byte(body)) {
					t.Fatal("control is not a valid generic reply")
				}
				var commands, configs atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch {
					case strings.HasSuffix(r.URL.Path, "/config"):
						configs.Add(1)
						fmt.Fprint(w, `{"data":{}}`)
					case strings.HasSuffix(r.URL.Path, "/status/current"):
						fmt.Fprint(w, `{"data":{"status":"running","cpu":0.25,"diskread":4096}}`)
					default:
						if commands.Add(1) == 1 {
							fmt.Fprint(w, body)
							return
						}
						backupAgentPayload(w, r)
					}
				}))
				defer server.Close()
				// Independent cases must not inherit uncertainty if the kernel
				// reuses a listener port. Within a case all clients share a key.
				host := server.URL + "/command-reply-fixture/" + reader + "/" + family
				c := commandReplyTestClient(t, host)
				if err := read(context.Background(), c, 105); GuestAgentDeferredReason(err) != "agent-completion-unverified" {
					t.Fatalf("foreign %s reply released %s admission: %v", family, reader, err)
				}
				if commands.Load() != 1 || configs.Load() != 1 {
					t.Fatal("foreign reply triggered a postflight lock read or replay")
				}
				diagnostic := commandReplyTestClient(t, host)
				for later, next := range backupAgentReads() {
					if err := next(context.Background(), diagnostic, 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
						t.Errorf("next %s escaped foreign-reply uncertainty: %v", later, err)
					}
				}
				if commands.Load() != 1 || configs.Load() != 1 {
					t.Fatal("foreign reply permitted another command or config request")
				}
				status, err := c.GetVMStatus(context.Background(), "node", 105)
				if err != nil || status.CPU != 0.25 || status.DiskRead != 4096 {
					t.Errorf("independent PVE counters stopped: %+v / %v", status, err)
				}
				if _, err := diagnostic.GetVMAgentInfo(context.Background(), "node", 106); err != nil {
					t.Errorf("another VM was fenced: %v", err)
				}
				t.Logf("foreign=%s reader=%s; one original command, six blocked follow-ups, live PVE and independent VM preserved", family, reader)
			})
		}
	}
}

func TestGuestAgentForeignReplySharesUnrenewedFenceAndResumes(t *testing.T) {
	var commands atomic.Int32
	var healthy atomic.Bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/nodes"):
			fmt.Fprint(w, `{"data":[]}`)
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, `{"data":{}}`)
		default:
			commands.Add(1)
			if strings.Contains(r.URL.Path, "/qemu/105/") && !healthy.Load() {
				fmt.Fprint(w, `{"data":{"result":{"qemu-ga":{"version":"1.0"}}}}`)
				return
			}
			backupAgentPayload(w, r)
		}
	})
	a, b, other := httptest.NewServer(handler), httptest.NewServer(handler), httptest.NewServer(handler)
	defer a.Close()
	defer b.Close()
	defer other.Close()
	hostA, hostB, hostOther := a.URL+"/alias-shape", b.URL+"/alias-shape", other.URL+"/other-shape"
	cc := NewClusterClient("command-reply-fixture", ClientConfig{Host: hostA, TokenName: "fixture@pve!pulse", TokenValue: "fixture"}, []string{hostA, hostB}, nil)
	if _, err := commandReplyTestClient(t, hostA).GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-completion-unverified" {
		t.Fatal(err)
	}
	guestAgentGuards.Lock()
	original := make(map[guestAgentGuardKey]guestAgentGuardEntry)
	for key, entry := range guestAgentGuards.entries {
		original[key] = entry
	}
	guestAgentGuards.Unlock()
	if _, err := cc.GetVMNetworkInterfaces(context.Background(), "migrated", 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
		t.Fatalf("cluster alias bypassed uncertainty: %v", err)
	}
	if _, err := commandReplyTestClient(t, hostB).GetVMAgentVersion(context.Background(), "migrated", 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
		t.Fatalf("fresh alias client bypassed uncertainty: %v", err)
	}
	guestAgentGuards.Lock()
	unchanged := reflect.DeepEqual(original, guestAgentGuards.entries)
	guestAgentGuards.Unlock()
	if commands.Load() != 1 || !unchanged {
		t.Fatal("follow-up dispatched work or renewed the original fence")
	}
	if _, err := commandReplyTestClient(t, hostB).GetVMFSInfo(context.Background(), "node", 106); err != nil {
		t.Fatal(err)
	}
	if _, err := commandReplyTestClient(t, hostOther).GetVMAgentVersion(context.Background(), "node", 105); err != nil {
		t.Fatalf("independent source blocked: %v", err)
	}
	// Expire only this fixture's original transport fence. No clock sleep,
	// re-enactment, guard reset or live/native operation is performed.
	guestAgentGuards.Lock()
	for key, entry := range guestAgentGuards.entries {
		if key.vmid == 105 && (key.endpoint == hostA+"/api2/json" || key.endpoint == hostB+"/api2/json") {
			entry.until = time.Now().Add(-time.Second)
			guestAgentGuards.entries[key] = entry
		}
	}
	guestAgentGuards.Unlock()
	healthy.Store(true)
	for reader, read := range backupAgentReads() {
		if err := read(context.Background(), commandReplyTestClient(t, hostB), 105); err != nil {
			t.Errorf("completed compatible %s read did not resume: %v", reader, err)
		}
	}
}

func TestGuestAgentDuplicateForeignEvidenceCannotBeErased(t *testing.T) {
	for name, result := range map[string]string{
		"duplicate": `{"name":"eth0","ip-addresses":[],"ip-addresses":null}`,
		"escaped":   `{"name":"eth0","ip-addresses":[],"\u0069p-addresses":null}`,
		"case":      `{"name":"eth0","IP-Addresses":[],"ip-addresses":null}`,
		"array-row": `[{"name":"eth0","ip-addresses":[],"ip-addresses":null}]`,
	} {
		t.Run(name, func(t *testing.T) {
			var commands atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/config") {
					fmt.Fprint(w, `{"data":{}}`)
					return
				}
				commands.Add(1)
				fmt.Fprintf(w, `{"data":{"result":%s}}`, result)
			}))
			defer server.Close()
			c := commandReplyTestClient(t, server.URL+"/duplicate-foreign/"+name)
			if _, err := c.GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-completion-unverified" {
				t.Fatalf("later null erased foreign interface evidence: %v", err)
			}
			if _, err := c.GetVMAgentInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-cooldown" || commands.Load() != 1 {
				t.Fatalf("foreign duplicate permitted another command: %v / %d", err, commands.Load())
			}
		})
	}
}

func TestGuestAgentCommandReplyPreservesCompatibleEmptyAndPartialData(t *testing.T) {
	replies := map[string][]string{
		"os":         {`{"data":{"name":"Linux","version":"fixture","future":{"enabled":true}}}`, `{"data":{"result":{"id":"linux","version-id":"1"}}}`},
		"version":    {`{"data":{"result":{"version":{"version":"1.0"},"supported_commands":[]}}}`, `{"data":{"result":{"qemu-ga":{"version":"1.0"}}}}`},
		"filesystem": {`{"data":{"result":{"mountpoint":"/","total-bytes":1024,"used-bytes":0},"future":true}}`, `{"data":{"result":[null,{"mountpoint":"/","used-bytes":512}]}}`},
		"network":    {`{"data":{"result":{"name":"eth0","hardware-address":"02:00:00:00:00:01"}}}`, `{"data":{"result":[null,{"name":"eth0"}],"future":true}}`},
		"file":       {`{"data":{"content":"MemTotal: 1024 kB\nMemAvailable: 0 kB\n","truncated":false}}`, `{"data":{"content":"","truncated":true}}`},
	}
	for reader, read := range backupAgentReads() {
		compatible := append([]string{`{"data":{}}`, `{"data":{"result":null,"content":null,"truncated":null}}`}, replies[guestReplyFamily(reader)]...)
		for index, body := range compatible {
			t.Run(fmt.Sprintf("%s/%d", reader, index), func(t *testing.T) {
				var commands atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasSuffix(r.URL.Path, "/config") {
						fmt.Fprint(w, `{"data":{}}`)
						return
					}
					if commands.Add(1) == 1 {
						fmt.Fprint(w, body)
						return
					}
					backupAgentPayload(w, r)
				}))
				defer server.Close()
				c := commandReplyTestClient(t, guestAgentFixtureEndpoint(t, server.URL))
				if err := read(context.Background(), c, 105); GuestAgentDeferredReason(err) != "" {
					t.Fatalf("compatible reply paused %s: %v", reader, err)
				}
				if _, err := c.GetVMAgentInfo(context.Background(), "node", 105); err != nil || commands.Load() != 2 {
					t.Fatalf("completed empty/partial measurement blocked OS discovery: %v", err)
				}
			})
		}
	}
}
