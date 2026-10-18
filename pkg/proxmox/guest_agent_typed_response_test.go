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

func invalidGuestAgentTypedReplies() map[string]string {
	return map[string]string{
		"string-result":     `{"data":{"result":"wrong command id"}}`,
		"empty-text-result": `{"data":{"result":""}}`,
		"true-result":       `{"data":{"result":true}}`,
		"false-result":      `{"data":{"result":false}}`,
		"number-result":     `{"data":{"result":17}}`,
		"zero-result":       `{"data":{"result":0}}`,
		"float-result":      `{"data":{"result":1.5}}`,
		"object-content":    `{"data":{"content":{"error":"wrong command id"}}}`,
		"array-content":     `{"data":{"content":[]}}`,
		"boolean-content":   `{"data":{"content":false}}`,
		"number-content":    `{"data":{"content":0}}`,
		"text-truncated":    `{"data":{"content":"MemTotal: 1024 kB\nMemAvailable: 512 kB\n","truncated":"false"}}`,
		"number-truncated":  `{"data":{"truncated":0}}`,
		"object-truncated":  `{"data":{"truncated":{}}}`,
		"array-truncated":   `{"data":{"truncated":[]}}`,
		"space-result":      "{\"data\":{\"result\": \n 17 \t}}",
	}
}

// Exercise the actual guarded HTTP methods, not just the envelope helper.
// No command, diagnostic or postflight lock read may follow an impossible
// typed reply until its existing uncertainty deadline has expired.
func TestGuestAgentTypedSuccessDefersEveryRead(t *testing.T) {
	for name, body := range invalidGuestAgentTypedReplies() {
		t.Run(name, func(t *testing.T) {
			if guestAgentSuccessResponse([]byte(body)) {
				t.Fatal("impossible guest reply accepted as a successful envelope")
			}
			for reader, first := range backupAgentReads() {
				t.Run(reader, func(t *testing.T) {
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
					// Uncertainty outlives this listener. A recycled port is not
					// the same synthetic guest, but every client in this case is.
					endpoint := typedSuccessFixtureEndpoint(server.URL)
					c := backupTestClient(t, endpoint)
					if err := first(context.Background(), c, 105); GuestAgentDeferredReason(err) != "agent-completion-unverified" {
						t.Fatalf("typed reply released admission: %v", err)
					}
					if commands.Load() != 1 || configs.Load() != 1 {
						t.Fatal("reply was treated as complete and received a postflight lock read")
					}
					diagnostic := backupTestClient(t, endpoint)
					for later, read := range backupAgentReads() {
						if err := read(context.Background(), diagnostic, 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
							t.Errorf("next %s escaped uncertainty: %v", later, err)
						}
					}
					if commands.Load() != 1 || configs.Load() != 1 {
						t.Fatal("uncertain reply dispatched more guest commands/config reads")
					}
					status, err := c.GetVMStatus(context.Background(), "node", 105)
					if err != nil || status.CPU != 0.25 || status.DiskRead != 4096 {
						t.Errorf("independent non-QGA counters changed: %+v %v", status, err)
					}
				})
			}
		})
	}
}

var typedSuccessFixtureIDs atomic.Uint64

// Allocate once per independent fixture, then retain the endpoint for all its
// clients. The sequence also distinguishes repeated -count invocations; no
// existing guard is cleared, expired or weakened to make a fixture admissible.
func typedSuccessFixtureEndpoint(serverURL string) string {
	return fmt.Sprintf("%s/typed-success-fixture/%d", serverURL, typedSuccessFixtureIDs.Add(1))
}

// Reuse a listener deliberately instead of waiting for ephemeral-port reuse.
// Reusing the old base path must retain its fence; independent fixtures must
// each exercise their first invalid reply and keep every later reader fenced.
func TestGuestAgentTypedSuccessFixtureIsolation(t *testing.T) {
	var commands, configs atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/config") {
			configs.Add(1)
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
		commands.Add(1)
		fmt.Fprint(w, `{"data":{"result":17}}`)
	}))
	defer server.Close()
	endpoints := make([]string, 0, 3)
	for index := 0; index < 3; index++ {
		endpoint := typedSuccessFixtureEndpoint(server.URL)
		endpoints = append(endpoints, endpoint)
		if _, err := backupTestClient(t, endpoint).GetVMNetworkInterfaces(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-completion-unverified" {
			t.Fatalf("independent fixture %d never reached its invalid reply: %v", index, err)
		}
		// A fresh client on a reused path is still the original channel,
		// exactly as it would be after the OS recycled a raw listener URL.
		if _, err := backupTestClient(t, endpoint).GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
			t.Fatalf("reused fixture %d lost its uncertainty fence: %v", index, err)
		}
		if commands.Load() != int32(index+1) || configs.Load() != int32(index+1) {
			t.Fatal("fixture independence admitted a postflight or replayed command")
		}
	}
	for _, endpoint := range endpoints {
		for name, read := range backupAgentReads() {
			if err := read(context.Background(), backupTestClient(t, endpoint), 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
				t.Errorf("another fixture erased the original %s fence: %v", name, err)
			}
		}
	}
	if commands.Load() != 3 || configs.Load() != 3 {
		t.Fatal("retained fences dispatched follow-up guest commands or lock reads")
	}
	t.Log("one listener; three independent invalid replies; all six readers retain every original fence")
}

func TestGuestAgentTypedSuccessSharesUnrenewedFenceAndResumes(t *testing.T) {
	isolateGuestAgentRegistry(t)
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
				fmt.Fprint(w, `{"data":{"result":"wrong command id"}}`)
				return
			}
			backupAgentPayload(w, r)
		}
	})
	a, b, independent := httptest.NewServer(handler), httptest.NewServer(handler), httptest.NewServer(handler)
	defer a.Close()
	defer b.Close()
	defer independent.Close()
	cc := NewClusterClient("typed-success", ClientConfig{Host: a.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture"}, []string{a.URL, b.URL}, nil)
	if _, err := cc.GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-completion-unverified" || commands.Load() != 1 {
		t.Fatalf("cluster failed over or cleared uncertain reply: %v commands=%d", err, commands.Load())
	}
	guestAgentGuards.Lock()
	original := make(map[guestAgentGuardKey]guestAgentGuardEntry)
	for key, entry := range guestAgentGuards.entries {
		original[key] = entry
	}
	guestAgentGuards.Unlock()
	for _, endpoint := range []string{a.URL, b.URL} {
		for name, read := range backupAgentReads() {
			if err := read(context.Background(), backupTestClient(t, endpoint), 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
				t.Errorf("fresh alias %s escaped uncertainty: %v", name, err)
			}
		}
	}
	guestAgentGuards.Lock()
	unchanged := reflect.DeepEqual(original, guestAgentGuards.entries)
	guestAgentGuards.Unlock()
	if !unchanged || commands.Load() != 1 {
		t.Fatal("deferral dispatched work or renewed the original cooldown")
	}
	if _, err := cc.GetVMAgentInfo(context.Background(), "node", 106); err != nil {
		t.Fatalf("another VM blocked: %v", err)
	}
	// The unrelated source may issue its own command; it receives its own
	// uncertainty, not the original cluster's pre-dispatch cooldown.
	if _, err := backupTestClient(t, independent.URL).GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-completion-unverified" || commands.Load() != 3 {
		t.Fatalf("independent source inherited another cluster's fence: %v", err)
	}
	// Advance only the existing fixture-owned deadlines, not a native guest
	// or the process clock. Recovery uses new valid observations.
	guestAgentGuards.Lock()
	for key, entry := range guestAgentGuards.entries {
		entry.until = time.Now().Add(-time.Second)
		guestAgentGuards.entries[key] = entry
	}
	guestAgentGuards.Unlock()
	healthy.Store(true)
	for name, read := range backupAgentReads() {
		if err := read(context.Background(), backupTestClient(t, b.URL), 105); err != nil {
			t.Errorf("fresh %s after expiry did not resume: %v", name, err)
		}
	}
	if commands.Load() != 9 {
		t.Fatalf("independent and resumed commands=%d, want9", commands.Load())
	}
}

func TestGuestAgentTypedSuccessPreservesFieldAndMeasurementSemantics(t *testing.T) {
	for name, body := range map[string]string{
		"null-fields":      `{"data":{"result":null,"content":null,"truncated":null}}`,
		"empty-fields":     `{"data":{"result":[],"content":"","truncated":false}}`,
		"empty-object":     `{"data":{"result":{}}}`,
		"partial-rows":     `{"data":{"result":[null,{"name":"root","mountpoint":"/","used-bytes":512}]}}`,
		"escaped-content":  `{"data":{"content":"caf\u00e9\n","truncated":true}}`,
		"unknown-metadata": `{"data":{"future-version":{"enabled":true},"result":[]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if !guestAgentSuccessResponse([]byte(body)) {
				t.Fatal("compatible typed reply rejected")
			}
		})
	}
	for name, body := range map[string]string{
		"empty":     `{"data":{"content":""}}`,
		"null":      `{"data":{"content":null,"truncated":null}}`,
		"truncated": `{"data":{"content":"MemTotal: 1024 kB\nMemFree: 100 kB\nBuffers: 100 kB\nCached: 100 kB\n","truncated":true}}`,
	} {
		t.Run("completed-file-"+name, func(t *testing.T) {
			var commands atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/config") {
					fmt.Fprint(w, `{"data":{}}`)
					return
				}
				commands.Add(1)
				if strings.HasSuffix(r.URL.Path, "/file-read") {
					fmt.Fprint(w, body)
					return
				}
				backupAgentPayload(w, r)
			}))
			defer server.Close()
			c := backupTestClient(t, typedSuccessFixtureEndpoint(server.URL))
			info, err := c.GetVMMemoryAvailabilityFromAgent(context.Background(), "node", 105)
			if err == nil || GuestAgentDeferredReason(err) != "" || info != (LinuxMemoryAvailability{}) {
				t.Fatalf("completed but unusable memory became telemetry or uncertainty: %+v %v", info, err)
			}
			if _, err := c.GetVMAgentInfo(context.Background(), "node", 105); err != nil || commands.Load() != 2 {
				t.Fatalf("measurement rejection unnecessarily paused OS recovery: %v", err)
			}
		})
	}
}
