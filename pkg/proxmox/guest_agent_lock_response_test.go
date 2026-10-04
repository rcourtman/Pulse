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
						c := backupTestClient(t, server.URL)
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
	if _, err := backupTestClient(t, server.URL).GetVMAgentInfo(ctx, "node", 105); GuestAgentDeferredReason(err) != "lock-unverified" {
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
	c1, c2 := backupTestClient(t, server.URL), backupTestClient(t, server.URL)
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

func TestVMConfigCompleteWhitespaceAtResponseLimit(t *testing.T) {
	const config = `{"data":{"name":"fixture","agent":1}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, config)
		w.(http.Flusher).Flush()
		fmt.Fprint(w, strings.Repeat(" ", int(maxResponseBodyBytes)-len(config)))
	}))
	defer server.Close()
	got, err := backupTestClient(t, server.URL).GetVMConfig(context.Background(), "node", 105)
	if err != nil || got["name"] != "fixture" || got["agent"] != float64(1) {
		t.Fatalf("complete bounded config rejected or changed: %#v %v", got, err)
	}
}
