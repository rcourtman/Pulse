package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// A server that consumes a command and loses its reply models uncertain
// completion, not an idle socket that closed before receiving any work.
func TestGuestAgentWireDoesNotReplayLostReply(t *testing.T) {
	for _, startsBackup := range []bool{false, true} {
		for name, read := range backupAgentReads() {
			t.Run(fmt.Sprintf("%s/backup=%t", name, startsBackup), func(t *testing.T) {
				var commands, configs atomic.Int32
				var locked atomic.Bool
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasSuffix(r.URL.Path, "/config") {
						configs.Add(1)
						if locked.Load() {
							fmt.Fprint(w, `{"data":{"lock":"backup"}}`)
						} else {
							fmt.Fprint(w, `{"data":{}}`)
						}
						return
					}
					if strings.Contains(r.URL.Path, "/agent/") {
						if commands.Add(1) == 1 {
							locked.Store(startsBackup)
							conn, _, err := w.(http.Hijacker).Hijack()
							if err != nil {
								t.Error(err)
								return
							}
							// The request has reached the endpoint before the connection is lost.
							conn.Close()
							return
						}
						backupAgentPayload(w, r)
						return
					}
					http.NotFound(w, r)
				}))
				defer server.Close()
				client := backupTestClient(t, server.URL)
				if err := read(context.Background(), client, 105); GuestAgentDeferredReason(err) != "agent-timeout" {
					t.Errorf("lost command reply was not deferred: %v", err)
				}
				if got := commands.Load(); got != 1 {
					t.Errorf("commands after lost reply = %d, want exactly one", got)
				}
				if got := configs.Load(); got != 1 {
					t.Errorf("config reads after uncertain command = %d, want only pre-dispatch verification", got)
				}
				// A different method must not follow the uncertain command either.
				if _, err := client.GetVMAgentInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
					t.Errorf("subsequent command escaped VM cooldown: %v", err)
				}
				if got := commands.Load(); got != 1 {
					t.Errorf("commands after cooldown read = %d, want one", got)
				}
			})
		}
	}
}

func TestGuestAgentWireDoesNotFollowRedirect(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		for name, read := range backupAgentReads() {
			t.Run(fmt.Sprintf("%s/status=%d", name, status), func(t *testing.T) {
				var first, redirected atomic.Int32
				destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					redirected.Add(1)
					backupAgentPayload(w, r)
				}))
				defer destination.Close()
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasSuffix(r.URL.Path, "/config") {
						fmt.Fprint(w, `{"data":{}}`)
						return
					}
					first.Add(1)
					http.Redirect(w, r, destination.URL+r.URL.Path, status)
				}))
				defer server.Close()
				client := backupTestClient(t, server.URL)
				if err := read(context.Background(), client, 105); GuestAgentDeferredReason(err) != "agent-redirect" {
					t.Errorf("redirect response was not deferred: %v", err)
				}
				if got := redirected.Load(); got != 0 {
					t.Errorf("unverified redirected commands = %d, want zero", got)
				}
				if got := first.Load(); got != 1 {
					t.Errorf("initial commands = %d, want one", got)
				}
				if _, err := client.GetVMAgentInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
					t.Errorf("redirect completion uncertainty escaped VM cooldown: %v", err)
				}
				if got := first.Load(); got != 1 {
					t.Errorf("command replay after redirect = %d, want one", got)
				}
				if got := redirected.Load(); got != 0 {
					t.Errorf("redirect destination reached during cooldown = %d", got)
				}
			})
		}
	}
}

func TestGuestAgentTransportLeavesOrdinaryRecoveryIntact(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 2 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			conn.Close()
			return
		}
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()
	client := backupTestClient(t, server.URL)
	// Warm the ordinary transport. Its idempotent resource reads still recover
	// a lost reply on that reused connection; only QGA commands are single-use.
	for i := 0; i < 2; i++ {
		if _, err := client.GetNodes(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("ordinary wire reads = %d, want warm read plus recovered pair", got)
	}
}

func TestGuestAgentTransportLeavesOrdinaryRedirectIntact(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/api2/json/nodes" {
			http.Redirect(w, r, "/redirected/nodes", http.StatusTemporaryRedirect)
			return
		}
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()
	if _, err := backupTestClient(t, server.URL).GetNodes(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("ordinary redirect reads = %d, want two", got)
	}
}
