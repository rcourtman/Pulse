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

func TestGuestAgentConfigResponseMustBeComplete(t *testing.T) {
	for _, test := range []struct {
		name  string
		write func(http.ResponseWriter)
	}{
		{"truncated", func(w http.ResponseWriter) {
			w.Header().Set("Content-Length", "1000")
			fmt.Fprint(w, `{"data":{}}`)
		}},
		{"trailing-json", func(w http.ResponseWriter) {
			fmt.Fprint(w, `{"data":{}}{"data":{"lock":"backup"}}`)
		}},
		{"trailing-garbage", func(w http.ResponseWriter) {
			fmt.Fprint(w, `{"data":{}}incomplete-response`)
		}},
		{"chunked-over-limit", func(w http.ResponseWriter) {
			fmt.Fprint(w, `{"data":{}}`)
			w.(http.Flusher).Flush() // No declared length: enforce the read boundary.
			fmt.Fprint(w, strings.Repeat(" ", int(maxResponseBodyBytes)))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for name, read := range backupAgentReads() {
				for _, postCommand := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/post=%t", name, postCommand), func(t *testing.T) {
						var configs, commands atomic.Int32
						var healthy atomic.Bool
						server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							if strings.HasSuffix(r.URL.Path, "/config") {
								if configs.Add(1) == 1 && postCommand || healthy.Load() {
									fmt.Fprint(w, `{"data":{}}`)
								} else {
									test.write(w)
								}
								return
							}
							commands.Add(1)
							backupAgentPayload(w, r)
						}))
						defer server.Close()
						// Retained VM fences outlive a closed listener. Recycled
						// ports must not share uncertainty between independent cases.
						c := backupTestClient(t, guestAgentFixtureEndpoint(t, server.URL))
						if err := read(context.Background(), c, 105); GuestAgentDeferredReason(err) != "lock-unverified" {
							t.Errorf("incomplete config was accepted: %v", err)
						}
						wantCommands := int32(0)
						if postCommand {
							wantCommands = 1
						}
						if got := commands.Load(); got != wantCommands {
							t.Errorf("guest commands = %d, want %d", got, wantCommands)
						}
						// This config failure says nothing about the completed guest
						// command. A later ordinary read must recheck the lock and resume.
						healthy.Store(true)
						if err := read(context.Background(), c, 105); err != nil {
							t.Fatalf("complete unlocked config did not resume: %v", err)
						}
						if got := commands.Load(); got != wantCommands+1 {
							t.Errorf("resumed guest commands = %d, want %d", got, wantCommands+1)
						}
					})
				}
			}
		})
	}
}

func TestGuestAgentConfigPrefixDoesNotAdmitCommand(t *testing.T) {
	var commands atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			w.(http.Flusher).Flush()
			<-r.Context().Done() // The body never completes, despite a valid prefix.
			return
		}
		commands.Add(1)
		backupAgentPayload(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := backupTestClient(t, guestAgentFixtureEndpoint(t, server.URL)).GetVMAgentInfo(ctx, "node", 105); GuestAgentDeferredReason(err) != "lock-unverified" {
		t.Errorf("stalled config admitted a command: %v", err)
	}
	if got := commands.Load(); got != 0 {
		t.Errorf("commands before config completion = %d, want zero", got)
	}
}

func TestGuestAgentAdmissionIncludesPostConfigBody(t *testing.T) {
	var configs, commands atomic.Int32
	entered := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/config") {
			n := configs.Add(1)
			fmt.Fprint(w, `{"data":{}}`)
			if n == 2 {
				w.(http.Flusher).Flush()
				close(entered)
				<-r.Context().Done()
			}
			return
		}
		commands.Add(1)
		backupAgentPayload(w, r)
	}))
	defer server.Close()
	host := guestAgentFixtureEndpoint(t, server.URL)
	c1, c2 := backupTestClient(t, host), backupTestClient(t, host)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		value, err := c1.GetVMFSInfo(ctx, "node", 105)
		if value != nil {
			t.Error("filesystem payload escaped the incomplete post-command lock check")
		}
		done <- err
	}()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("post-command config check never started: %v", err)
	}
	if _, err := c2.GetVMNetworkInterfaces(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-busy" {
		t.Errorf("second client bypassed incomplete config admission: %v", err)
	}
	if err := <-done; GuestAgentDeferredReason(err) != "lock-unverified" {
		t.Errorf("post-command config prefix accepted: %v", err)
	}
	if got := commands.Load(); got != 1 {
		t.Errorf("commands during incomplete post-check = %d, want one", got)
	}
	if _, err := c2.GetVMFSInfo(context.Background(), "node", 105); err != nil {
		t.Fatalf("admission not released after failed lock check: %v", err)
	}
}

func TestGuestAgentLockResponseFixtureIsolation(t *testing.T) {
	var commands, configs atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/config") {
			configs.Add(1)
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
		commands.Add(1)
		fmt.Fprint(w, `{"data":null}`)
	}))
	defer server.Close()
	var hosts []string
	// Use exactly the same listener/VMID to make port reuse deterministic. Each
	// independent subtest must get its own scope, but two clients in that case
	// must still share the original uncertainty fence without another request.
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			host := guestAgentFixtureEndpoint(t, server.URL)
			hosts = append(hosts, host)
			if _, err := backupTestClient(t, host).GetVMNetworkInterfaces(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-completion-unverified" {
				t.Fatalf("independent fixture inherited another case's fence: %v", err)
			}
			if _, err := backupTestClient(t, host).GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
				t.Fatalf("same-case client lost its retained uncertainty fence: %v", err)
			}
		})
	}
	for _, host := range hosts {
		if _, err := backupTestClient(t, host).GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
			t.Fatalf("admitting another fixture erased the existing fence: %v", err)
		}
	}
	if commands.Load() != 2 || configs.Load() != 2 {
		t.Fatalf("fixture command/config counts = %d/%d, want exactly 2/2", commands.Load(), configs.Load())
	}
}

func TestVMConfigCompleteWhitespaceAtResponseLimit(t *testing.T) {
	const config = `{"data":{"name":"fixture","agent":1}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, config)
		w.(http.Flusher).Flush()
		fmt.Fprint(w, strings.Repeat(" ", int(maxResponseBodyBytes)-len(config)))
	}))
	defer server.Close()
	// This checks the complete 8 MiB response boundary, not throughput. The
	// shared one-second client fixture can time out while race-instrumented
	// body collection/decoding is still completing. Keep a bounded hang guard
	// here without changing any production timeout or lock/size requirement.
	client, err := NewClient(ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.GetVMConfig(context.Background(), "node", 105)
	if err != nil || got["name"] != "fixture" || got["agent"] != float64(1) {
		t.Fatalf("complete bounded config rejected or changed: %#v %v", got, err)
	}
}
