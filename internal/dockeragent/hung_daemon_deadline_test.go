package dockeragent

// Regression pins for the 2026-07-17 live-exercise stall: collectStorageUsage
// sat 6+ minutes inside moby DiskUsage even though dockerCallWithRetry wraps
// every call in context.WithTimeout. Investigation could not reproduce a
// deadline being swallowed anywhere in the moby client stack; these tests pin
// that a context deadline aborts the real moby client (request.go + otelhttp
// transport + API-version negotiation) against a hung unix-socket daemon, in
// both hang shapes, so a future moby/otelhttp upgrade cannot silently regress
// it:
//
//   - hang before response headers (daemon computing /system/df forever)
//   - hang mid-body (daemon streams partial JSON then stalls)

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moby/moby/client"
)

// hungSocketServer listens on a unix socket, accepts connections, discards
// whatever the client writes, and never responds.
func startHungSocketServer(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "pulsedu")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	sockPath := filepath.Join(dir, "docker.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 4096)
				for {
					if _, err := c.Read(buf); err != nil {
						return
					}
					// Read requests forever, never write a byte back.
				}
			}(conn)
		}
	}()

	return sockPath
}

// startStallingHTTPServer answers /_ping so API-version negotiation succeeds,
// then for /system/df writes response headers plus a partial JSON body and
// stalls forever.
func startStallingHTTPServer(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "pulsedu")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	sockPath := filepath.Join(dir, "docker.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}

	hang := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Api-Version", "1.51")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Any versioned endpoint (e.g. /v1.51/system/df): stream a partial
		// body, then stall.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"LayersSize": 1, "Images": [`)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-hang
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		close(hang)
		_ = srv.Close()
	})

	return sockPath
}

func newTestMobyClient(t *testing.T, sockPath string) dockerClient {
	t.Helper()

	cli, err := newMobyDockerClient(
		client.WithHost("unix://"+sockPath),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		t.Fatalf("new moby client: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })
	return cli
}

// diskUsageViaProductionPath mirrors collectStorageUsage's exact wrapper and
// options.
func diskUsageViaProductionPath(ctx context.Context, cli dockerClient, timeout time.Duration) error {
	_, err := dockerCallWithRetryAttempts(ctx, timeout, 1, func(callCtx context.Context) (client.DiskUsageResult, error) {
		return cli.DiskUsage(callCtx, dockerDiskUsageOptions{
			Containers: true,
			Images:     true,
			BuildCache: true,
			Volumes:    true,
			Verbose:    true,
		})
	})
	return err
}

func TestDiskUsageDeadlineAbortsHungSocketBeforeHeaders(t *testing.T) {
	sockPath := startHungSocketServer(t)
	cli := newTestMobyClient(t, sockPath)

	const timeout = 2 * time.Second
	start := time.Now()
	err := diskUsageViaProductionPath(context.Background(), cli, timeout)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected error from hung daemon socket, got nil after %v", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v (after %v)", err, elapsed)
	}
	if elapsed > timeout+3*time.Second {
		t.Fatalf("deadline did not abort the call promptly: elapsed %v for %v timeout", elapsed, timeout)
	}
	t.Logf("pre-header hang aborted after %v with: %v", elapsed, err)
}

func TestDiskUsageDeadlineAbortsHungSocketMidBody(t *testing.T) {
	sockPath := startStallingHTTPServer(t)
	cli := newTestMobyClient(t, sockPath)

	const timeout = 2 * time.Second
	start := time.Now()
	err := diskUsageViaProductionPath(context.Background(), cli, timeout)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected error from mid-body stall, got nil after %v", elapsed)
	}
	if elapsed > timeout+3*time.Second {
		t.Fatalf("deadline did not abort the mid-body stall promptly: elapsed %v for %v timeout (err=%v)", elapsed, timeout, err)
	}
	t.Logf("mid-body stall aborted after %v with: %v", elapsed, err)
}
