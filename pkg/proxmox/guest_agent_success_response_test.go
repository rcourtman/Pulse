package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// HTTP 200 and a finished body do not establish a completed guest reply if
// the envelope is missing, ambiguous, or only a valid prefix. All six readers
// must retain uncertainty before another client/method can queue QGA work.
func TestGuestAgentInvalidSuccessDefersEveryRead(t *testing.T) {
	for name, body := range map[string]string{
		"empty":             "",
		"html":              "<html>upstream unavailable</html>",
		"truncated-json":    `{"data":{"result":`,
		"trailing-value":    `{"data":{"result":[]}} {"data":{"result":[]}}`,
		"trailing-garbage":  `{"data":{"result":[]}} unavailable`,
		"missing-data":      `{}`,
		"null-data":         `{"data":null}`,
		"scalar-data":       `{"data":"upstream unavailable"}`,
		"duplicate-data":    `{"data":null,"data":{"result":[]}}`,
		"escaped-duplicate": `{"data":null,"\u0064ata":{"result":[]}}`,
		"case-conflict":     `{"Data":null,"data":{"result":[]}}`,
		"case-only":         `{"Data":{"result":[]}}`,
		"competing-error":   `{"errors":{"message":"upstream unavailable"},"data":{"result":[]}}`,
		"duplicate-result":  `{"data":{"result":null,"result":[]}}`,
		"result-case":       `{"data":{"Result":null,"result":[]}}`,
		"content-case":      `{"data":{"Content":"stale","content":"fresh"}}`,
		"duplicate-content": `{"data":{"content":"stale","content":"fresh"}}`,
		"non-utf8":          "{\"data\":{\"content\":\"\xff\"}}",
	} {
		t.Run(name, func(t *testing.T) {
			for reader, first := range backupAgentReads() {
				t.Run(reader, func(t *testing.T) {
					var calls, configs atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						switch {
						case strings.HasSuffix(r.URL.Path, "/config"):
							configs.Add(1)
							fmt.Fprint(w, `{"data":{}}`)
						case strings.HasSuffix(r.URL.Path, "/status/current"):
							fmt.Fprint(w, `{"data":{"status":"running","cpu":0.25,"diskread":4096}}`)
						default:
							if calls.Add(1) == 1 {
								fmt.Fprint(w, body)
								return
							}
							backupAgentPayload(w, r)
						}
					}))
					defer server.Close()
					c := backupTestClient(t, server.URL)
					if err := first(context.Background(), c, 105); GuestAgentDeferredReason(err) != "agent-completion-unverified" {
						t.Errorf("unverified success released admission: %v", err)
					}
					before := configs.Load()
					// The next complete reply would be healthy; it must not be
					// requested while the earlier command remains uncertain.
					diagnostic := backupTestClient(t, server.URL)
					for later, read := range backupAgentReads() {
						if err := read(context.Background(), diagnostic, 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
							t.Errorf("next %s escaped cooldown: %v", later, err)
						}
					}
					if calls.Load() != 1 || configs.Load() != before {
						t.Errorf("uncertain reply caused follow-up commands/config reads: %d/%d -> %d", calls.Load(), before, configs.Load())
					}
					status, err := c.GetVMStatus(context.Background(), "node", 105)
					if err != nil || status.CPU != 0.25 || status.DiskRead != 4096 {
						t.Errorf("live non-agent counters lost: %+v %v", status, err)
					}
				})
			}
		})
	}
}

func TestGuestAgentInvalidSuccessSharesAliasesAndResumes(t *testing.T) {
	var calls, otherCalls atomic.Int32
	var healthy atomic.Bool
	handler := func(counter *atomic.Int32, broken bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/config"):
				fmt.Fprint(w, `{"data":{}}`)
			case strings.HasSuffix(r.URL.Path, "/nodes"):
				fmt.Fprint(w, `{"data":[]}`)
			default:
				counter.Add(1)
				if broken && !healthy.Load() {
					fmt.Fprint(w, `{"data":null,"data":{"result":[]}}`)
					return
				}
				backupAgentPayload(w, r)
			}
		}
	}
	a := httptest.NewServer(handler(&calls, true))
	b := httptest.NewServer(handler(&calls, false))
	independent := httptest.NewServer(handler(&otherCalls, false))
	defer a.Close()
	defer b.Close()
	defer independent.Close()
	cc := NewClusterClient("success-envelope", ClientConfig{Host: a.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture"}, []string{a.URL, b.URL}, nil)
	if _, err := backupTestClient(t, a.URL).GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-completion-unverified" {
		t.Fatal(err)
	}
	if _, err := cc.GetVMAgentInfo(context.Background(), "migrated", 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
		t.Fatalf("cluster alias replayed uncertain success: %v", err)
	}
	if _, err := backupTestClient(t, b.URL).GetVMFSInfo(context.Background(), "migrated", 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
		t.Fatalf("fresh client/alias bypassed uncertainty: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("cluster wire commands after uncertainty = %d", calls.Load())
	}
	for _, c := range []*Client{backupTestClient(t, b.URL), backupTestClient(t, independent.URL)} {
		if _, err := c.GetVMFSInfo(context.Background(), "node", 106); err != nil {
			t.Fatalf("independent guest blocked: %v", err)
		}
	}
	if _, err := backupTestClient(t, independent.URL).GetVMFSInfo(context.Background(), "node", 105); err != nil {
		t.Fatalf("independent source blocked: %v", err)
	}
	// Expire only this fixture's actual shared cooldown. No sleep, replay of
	// the uncertain request, or replacement of the guard/transport is needed.
	guestAgentGuards.Lock()
	for _, endpoint := range []string{a.URL, b.URL} {
		key := guestAgentGuardKey{endpoint: endpoint + "/api2/json", vmid: 105}
		entry := guestAgentGuards.entries[key]
		entry.until = time.Now().Add(-time.Second)
		guestAgentGuards.entries[key] = entry
	}
	guestAgentGuards.Unlock()
	healthy.Store(true)
	for name, read := range backupAgentReads() {
		if err := read(context.Background(), backupTestClient(t, a.URL), 105); err != nil {
			t.Errorf("fresh %s after cooldown: %v", name, err)
		}
	}
	if calls.Load() != 8 || otherCalls.Load() != 2 {
		t.Fatalf("isolated/resumed commands = %d/%d, want 8/2", calls.Load(), otherCalls.Load())
	}
}

func TestGuestAgentUnexpectedSuccessStatusDefersEveryRead(t *testing.T) {
	for _, status := range []int{201, 202, 204, 206} {
		t.Run(fmt.Sprintf("status-%d", status), func(t *testing.T) {
			for name, first := range backupAgentReads() {
				t.Run(name, func(t *testing.T) {
					var calls atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if strings.HasSuffix(r.URL.Path, "/config") {
							fmt.Fprint(w, `{"data":{}}`)
							return
						}
						if calls.Add(1) == 1 {
							w.WriteHeader(status)
							backupAgentPayload(w, r)
							return
						}
						backupAgentPayload(w, r)
					}))
					defer server.Close()
					c := backupTestClient(t, server.URL)
					if err := first(context.Background(), c, 105); GuestAgentDeferredReason(err) != "agent-completion-unverified" {
						t.Fatalf("unexpected status became completed reply: %v", err)
					}
					for name, next := range backupAgentReads() {
						if err := next(context.Background(), c, 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
							t.Errorf("next %s escaped cooldown: %v", name, err)
						}
					}
					if calls.Load() != 1 {
						t.Fatalf("unexpected success caused another command: %d", calls.Load())
					}
				})
			}
		})
	}
}

func TestGuestAgentSuccessEnvelopePreservesSupportedPayloads(t *testing.T) {
	for name, body := range map[string]string{
		"empty-dictionary": `{"data":{}}`,
		"null-result":      `{"data":{"result":null}}`,
		"empty-result":     `{"data":{"result":[]}}`,
		"object-result":    `{"data":{"result":{"mountpoint":"/","total-bytes":1024,"used-bytes":512}}}`,
		"partial-rows":     `{"data":{"result":[null,{"name":"root","mountpoint":"/","used-bytes":512}]}}`,
		"os-info":          `{"data":{"id":"linux","name":"Linux"}}`,
		"version":          `{"data":{"result":{"qemu-ga":{"version":"9.2"}}}}`,
		"file-read":        `{"data":{"content":"MemTotal: 1024 kB\nMemAvailable: 512 kB\n","truncated":false}}`,
		"truncated-file":   `{"data":{"content":"partial","truncated":true}}`,
		"unicode":          `{"data":{"name":"café"}}`,
		"whitespace":       " \n{\"data\":{\"result\":[]}}\r\n\t ",
	} {
		t.Run(name, func(t *testing.T) {
			if !guestAgentSuccessResponse([]byte(body)) {
				t.Fatal("valid envelope rejected before method-specific decoding")
			}
		})
	}
}
