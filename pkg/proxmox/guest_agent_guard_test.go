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

func TestGuestAgentClusterAliasesAndVMIsolation(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/nodes"):
			fmt.Fprint(w, `{"data":[]}`)
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, `{"data":{}}`)
		case strings.Contains(r.URL.Path, "/agent/"):
			close(entered)
			<-release
			backupAgentPayload(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer first.Close()
	var calls atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/nodes") {
			fmt.Fprint(w, `{"data":[]}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
		calls.Add(1)
		backupAgentPayload(w, r)
	})
	second := httptest.NewServer(handler)
	defer second.Close()
	independent := httptest.NewServer(handler)
	defer independent.Close()
	NewClusterClient("aliases", ClientConfig{Host: first.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second}, []string{first.URL, second.URL}, nil)
	c1, c2, c3 := backupTestClient(t, first.URL), backupTestClient(t, second.URL), backupTestClient(t, independent.URL)
	done := make(chan error, 1)
	go func() { _, err := c1.GetVMFSInfo(context.Background(), "node", 105); done <- err }()
	<-entered
	defer func() {
		close(release)
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	if _, err := c2.GetVMAgentInfo(context.Background(), "migrated-node", 105); !errors.Is(err, ErrGuestAgentDeferred) {
		t.Errorf("alias bypassed busy VM: %v", err)
	}
	if _, err := c2.GetVMAgentInfo(context.Background(), "node", 106); err != nil {
		t.Errorf("different VM blocked: %v", err)
	}
	if _, err := c3.GetVMAgentInfo(context.Background(), "node", 105); err != nil {
		t.Errorf("independent source blocked: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("healthy isolated calls = %d", calls.Load())
	}
}

func TestGuestAgentLockAppearingInFlightDiscardsPayload(t *testing.T) {
	var locked atomic.Bool
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/config") {
			if locked.Load() {
				fmt.Fprint(w, `{"data":{"lock":"backup"}}`)
			} else {
				fmt.Fprint(w, `{"data":{}}`)
			}
			return
		}
		calls.Add(1)
		locked.Store(true)
		backupAgentPayload(w, r)
	}))
	defer server.Close()
	c := backupTestClient(t, server.URL)
	if value, err := c.GetVMFSInfo(context.Background(), "node", 105); value != nil || GuestAgentDeferredReason(err) != "vm-locked" {
		t.Fatalf("in-flight backup payload published: %#v %v", value, err)
	}
	if _, err := c.GetVMAgentInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "vm-locked" {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("another command queued: %d", calls.Load())
	}
}

func TestGuestAgentIncompleteBodyBlocksWholeVM(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
		calls.Add(1)
		w.Header().Set("Content-Length", "1000")
		fmt.Fprint(w, `{"data":{"result":[]}}`)
	}))
	defer server.Close()
	c := backupTestClient(t, server.URL)
	if _, err := c.GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-response-incomplete" {
		t.Fatalf("incomplete body not deferred: %v", err)
	}
	for name, read := range backupAgentReads() {
		t.Run(name, func(t *testing.T) {
			if err := read(context.Background(), c, 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
				t.Fatal(err)
			}
		})
	}
	if calls.Load() != 1 {
		t.Fatalf("commands after incomplete body = %d", calls.Load())
	}
}

func TestGuestAgentClusterDoesNotReplayUncertainCall(t *testing.T) {
	var calls atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/nodes"):
			fmt.Fprint(w, `{"data":[]}`)
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, `{"data":{}}`)
		default:
			calls.Add(1)
			http.Error(w, "QEMU guest command timed out", 500)
		}
	})
	a, b := httptest.NewServer(handler), httptest.NewServer(handler)
	defer a.Close()
	defer b.Close()
	cc := NewClusterClient("timeouts", ClientConfig{Host: a.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second}, []string{a.URL, b.URL}, nil)
	if _, err := cc.GetVMFSInfo(context.Background(), "node", 105); !errors.Is(err, ErrGuestAgentDeferred) {
		t.Fatalf("uncertain completion lost: %v", err)
	}
	if _, err := cc.GetVMAgentInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
		t.Fatalf("another cluster command sent: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("uncertain call replayed: %d", calls.Load())
	}
	for endpoint, healthy := range cc.GetHealthStatus() {
		if !healthy {
			t.Errorf("VM timeout poisoned endpoint %s", endpoint)
		}
	}
}

func TestGuestAgentCooldownExpiresWithoutSleepOrReplay(t *testing.T) {
	endpoint := "http://fixture.invalid/api2/json"
	release, err := acquireGuestAgent(endpoint, 105)
	if err != nil {
		t.Fatal(err)
	}
	release(true)
	if _, err := acquireGuestAgent(endpoint, 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
		t.Fatal(err)
	}
	guestAgentGuards.Lock()
	key := guestAgentGuardKey{endpoint: endpoint, vmid: 105}
	entry := guestAgentGuards.entries[key]
	entry.until = time.Now().Add(-time.Second)
	guestAgentGuards.entries[key] = entry
	guestAgentGuards.Unlock()
	release, err = acquireGuestAgent(endpoint, 105)
	if err != nil {
		t.Fatal(err)
	}
	release(false)
	guestAgentGuards.Lock()
	_, retained := guestAgentGuards.entries[key]
	guestAgentGuards.Unlock()
	if retained {
		t.Fatal("healthy completed operation retained estate state")
	}
}

func TestGuestAgentPasswordSessionDoesNotReplayRefusedCommand(t *testing.T) {
	for _, refused := range []bool{false, true} {
		t.Run(fmt.Sprintf("refused-%t", refused), func(t *testing.T) {
			var authCalls, agentCalls, nodeCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/access/ticket"):
					authCalls.Add(1)
					fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
				case strings.HasSuffix(r.URL.Path, "/config"):
					fmt.Fprint(w, `{"data":{}}`)
				case strings.Contains(r.URL.Path, "/agent/"):
					if agentCalls.Add(1) == 1 && refused {
						http.Error(w, "session refused", http.StatusUnauthorized)
						return
					}
					backupAgentPayload(w, r)
				case strings.HasSuffix(r.URL.Path, "/nodes"):
					if nodeCalls.Add(1) == 1 {
						http.Error(w, "expired ordinary API session", http.StatusUnauthorized)
						return
					}
					fmt.Fprint(w, `{"data":[]}`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client, err := NewClient(ClientConfig{Host: server.URL, User: "fixture@pam", Password: "fixture-password", Timeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.GetVMFSInfo(context.Background(), "node", 105)
			if refused && (err == nil || !strings.Contains(err.Error(), "401")) {
				t.Fatalf("guest refusal lost: %v", err)
			}
			if !refused && err != nil {
				t.Fatal(err)
			}
			if agentCalls.Load() != 1 || authCalls.Load() != 1 {
				t.Fatalf("guest replay: commands=%d authentications=%d", agentCalls.Load(), authCalls.Load())
			}
			if _, err := client.GetNodes(context.Background()); err != nil {
				t.Fatalf("ordinary API session recovery broken: %v", err)
			}
			if agentCalls.Load() != 1 || nodeCalls.Load() != 2 || authCalls.Load() != 2 {
				t.Fatalf("ordinary recovery replayed guest or did not recover: guest=%d nodes=%d auth=%d", agentCalls.Load(), nodeCalls.Load(), authCalls.Load())
			}
		})
	}
}
