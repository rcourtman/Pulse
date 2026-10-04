package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func backupTestClient(t *testing.T, host string) *Client {
	t.Helper()
	c, err := NewClient(ClientConfig{Host: host, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func backupAgentReads() map[string]func(context.Context, *Client, int) error {
	return map[string]func(context.Context, *Client, int) error{
		"os": func(ctx context.Context, c *Client, id int) error {
			_, err := c.GetVMAgentInfo(ctx, "node", id)
			return err
		},
		"version": func(ctx context.Context, c *Client, id int) error {
			_, err := c.GetVMAgentVersion(ctx, "node", id)
			return err
		},
		"filesystem": func(ctx context.Context, c *Client, id int) error {
			_, err := c.GetVMFSInfo(ctx, "node", id)
			return err
		},
		"network": func(ctx context.Context, c *Client, id int) error {
			_, err := c.GetVMNetworkInterfaces(ctx, "node", id)
			return err
		},
		"meminfo": func(ctx context.Context, c *Client, id int) error {
			_, err := c.GetVMMemoryAvailabilityFromAgent(ctx, "node", id)
			return err
		},
		"legacy-memory": func(ctx context.Context, c *Client, id int) error {
			_, err := c.GetVMMemAvailableFromAgent(ctx, "node", id)
			return err
		},
	}
}

func backupAgentPayload(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "file-read"):
		fmt.Fprint(w, `{"data":{"content":"MemTotal: 1024 kB\nMemAvailable: 512 kB\n"}}`)
	case strings.HasSuffix(r.URL.Path, "info") && !strings.HasSuffix(r.URL.Path, "fsinfo") && !strings.HasSuffix(r.URL.Path, "osinfo"):
		fmt.Fprint(w, `{"data":{"result":{"version":"1.0"}}}`)
	case strings.HasSuffix(r.URL.Path, "get-osinfo"):
		fmt.Fprint(w, `{"data":{"id":"linux"}}`)
	default:
		fmt.Fprint(w, `{"data":{"result":[]}}`)
	}
}

func TestGuestAgentBackupLockBlocksEveryReadAndResumes(t *testing.T) {
	var locked atomic.Bool
	locked.Store(true)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/config") {
			if locked.Load() {
				fmt.Fprint(w, `{"data":{"lock":"backup"}}`)
			} else {
				fmt.Fprint(w, `{"data":{}}`)
			}
			return
		}
		if strings.Contains(r.URL.Path, "/agent/") {
			calls.Add(1)
			backupAgentPayload(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	c := backupTestClient(t, server.URL)
	for name, read := range backupAgentReads() {
		t.Run(name, func(t *testing.T) {
			if err := read(context.Background(), c, 105); err == nil {
				t.Fatal("backup-locked guest was queried")
			}
		})
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("agent requests while locked = %d, want zero", got)
	}
	locked.Store(false)
	for name, read := range backupAgentReads() {
		t.Run("resumed-"+name, func(t *testing.T) {
			if err := read(context.Background(), c, 105); err != nil {
				t.Fatal(err)
			}
		})
	}
	if got := calls.Load(); got != 6 {
		t.Errorf("agent requests after unlock = %d, want six", got)
	}
}

func TestGuestAgentUnknownLockStateFailsClosed(t *testing.T) {
	for _, body := range []string{`{"data":null}`, `{}`, `{"data":{"lock":true}}`, `{"data":{"lock":"snapshot"}}`} {
		t.Run(body, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/config") {
					fmt.Fprint(w, body)
					return
				}
				calls.Add(1)
				backupAgentPayload(w, r)
			}))
			defer server.Close()
			_, err := backupTestClient(t, server.URL).GetVMFSInfo(context.Background(), "node", 105)
			if err == nil || calls.Load() != 0 {
				t.Fatalf("unverified lock state queried agent: err=%v calls=%d", err, calls.Load())
			}
		})
	}
}

func TestGuestAgentConcurrentDiagnosticsDoNotOverlap(t *testing.T) {
	var active, peak atomic.Int32
	entered := make(chan struct{}, 10)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
		n := active.Add(1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		entered <- struct{}{}
		<-release
		active.Add(-1)
		backupAgentPayload(w, r)
	}))
	defer server.Close()
	// Separate Client instances model a diagnostic alongside ordinary polling.
	c1, c2 := backupTestClient(t, server.URL), backupTestClient(t, server.URL)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _, _ = c1.GetVMFSInfo(context.Background(), "node", 105) }()
	<-entered
	wg.Add(1)
	done := make(chan struct{})
	go func() {
		defer wg.Done()
		_, _ = c2.GetVMNetworkInterfaces(context.Background(), "node", 105)
		close(done)
	}()
	select {
	case <-done:
	case <-entered:
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	wg.Wait()
	if got := peak.Load(); got != 1 {
		t.Fatalf("simultaneous guest-agent requests = %d, want one", got)
	}
}

func TestGuestAgentTimeoutDefersOtherCommands(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
		calls.Add(1)
		if strings.HasSuffix(r.URL.Path, "get-fsinfo") {
			<-r.Context().Done()
			return
		}
		backupAgentPayload(w, r)
	}))
	defer server.Close()
	c := backupTestClient(t, server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	if _, err := c.GetVMFSInfo(ctx, "node", 105); err == nil {
		t.Fatal("missing timeout")
	}
	if _, err := c.GetVMAgentInfo(context.Background(), "node", 105); err == nil {
		t.Error("another command was sent immediately after timeout")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("guest-agent requests = %d, want one", got)
	}
}
