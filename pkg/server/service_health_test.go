package server

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog"
)

// Ordinary agent-only configuration has no nodes.enc. A later startup must not
// mistake that installation for first provisioning and replace its access.
func TestStartupImportRetainsNonProxmoxAccess(t *testing.T) {
	payload, passphrase := newAutoImportExport(t)
	dir := t.TempDir()
	setAutoImportInputs(t, dir, payload, passphrase)
	target := config.NewConfigPersistence(dir)
	if err := target.SaveAPITokens([]config.APITokenRecord{{
		ID: "synthetic-existing-record", Hash: auth.HashAPIToken("synthetic-existing-credential"),
		Scopes: []string{config.ScopeAgentReport},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sessions.json"), []byte("synthetic-existing-session"), 0600); err != nil {
		t.Fatal(err)
	}
	// Use the production loader without reinitialising the global test logger.
	loaded, err := config.LoadWithoutLoggingInit()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.PVEInstances)+len(loaded.PBSInstances)+len(loaded.PMGInstances) != 0 {
		t.Fatal("fixture is not an ordinary non-Proxmox installation")
	}
	if _, err := os.Lstat(filepath.Join(dir, "nodes.enc")); !os.IsNotExist(err) {
		t.Fatal("fixture unexpectedly has the old bootstrap sentinel")
	}
	before := snapshotAutoImportDestination(t, dir)
	if ShouldAutoImport() {
		t.Error("ordinary restart was admitted as fresh provisioning")
	}
	if err := PerformAutoImport(); err == nil {
		t.Error("startup import replaced existing access")
	}
	if after := snapshotAutoImportDestination(t, dir); !reflect.DeepEqual(before, after) {
		t.Error("startup import changed the retained installation")
	}
	loaded, err = config.LoadWithoutLoggingInit()
	if err != nil {
		t.Fatal(err)
	}
	if record, ok := loaded.ValidateAPIToken("synthetic-existing-credential"); !ok || record.ID != "synthetic-existing-record" {
		t.Error("retained agent credential no longer validates")
	}

	// The authenticated Recovery path uses ImportConfig directly. Bootstrap
	// admission must not disable an administrator's deliberate transfer.
	if err := target.ImportConfig(payload, passphrase); err != nil {
		t.Fatal(err)
	}
	records, err := target.LoadAPITokens()
	if err != nil || len(records) != 1 || records[0].ID != "synthetic-import-record" {
		t.Error("explicit configuration transfer no longer replaces included records")
	}
	for _, name := range []string{".encryption.key", "sessions.json"} {
		if after := snapshotAutoImportDestination(t, dir); after[name] != before[name] {
			t.Errorf("explicit transfer changed excluded local state %s", name)
		}
	}
}

func TestServiceHealthProbeCoversAPIUIAndFrontendAssets(t *testing.T) {
	tests := []struct {
		name         string
		handler      http.Handler
		wantHealthy  bool
		wantCategory string
	}{
		{
			name: "healthy",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/health":
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"status":"healthy"}`)
				case "/":
					w.Header().Set("Content-Type", "text/html")
					fmt.Fprint(w, `<html><head><link rel="stylesheet" href="/assets/app.css"></head><body><script src="/assets/app.js"></script></body></html>`)
				case "/assets/app.css":
					fmt.Fprint(w, "body{}")
				case "/assets/app.js":
					fmt.Fprint(w, "console.log('ok')")
				default:
					http.NotFound(w, r)
				}
			}),
			wantHealthy: true,
		},
		{
			name: "api unhealthy",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/health" {
					http.Error(w, `{"status":"unhealthy"}`, http.StatusServiceUnavailable)
					return
				}
				http.NotFound(w, r)
			}),
			wantCategory: telemetry.ServiceHealthFailureAPIStatus,
		},
		{
			name: "ui unavailable",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/health" {
					fmt.Fprint(w, `{"status":"healthy"}`)
					return
				}
				http.NotFound(w, r)
			}),
			wantCategory: telemetry.ServiceHealthFailureUIStatus,
		},
		{
			name: "frontend asset unavailable",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/health":
					fmt.Fprint(w, `{"status":"healthy"}`)
				case "/":
					fmt.Fprint(w, `<html><body><script src="/assets/missing.js"></script></body></html>`)
				default:
					http.NotFound(w, r)
				}
			}),
			wantCategory: telemetry.ServiceHealthFailureFrontendAssets,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("listen: %v", err)
			}
			server := &http.Server{Handler: test.handler, ReadHeaderTimeout: time.Second}
			done := make(chan struct{})
			go func() {
				defer close(done)
				_ = server.Serve(listener)
			}()
			t.Cleanup(func() {
				_ = server.Close()
				<-done
			})

			got := newServiceHealthProbe(listener, false)()
			if !got.Observed || got.Healthy != test.wantHealthy || got.FailureCategory != test.wantCategory {
				t.Fatalf("service-health observation = %#v, want healthy=%v category=%q", got, test.wantHealthy, test.wantCategory)
			}
		})
	}
}

func TestServiceHealthProbeReportsConnectivityWithoutListener(t *testing.T) {
	for _, listener := range []net.Listener{
		nil,
		serviceHealthAddrListener{addr: &net.UnixAddr{Name: "/tmp/unused", Net: "unix"}},
		serviceHealthAddrListener{addr: &net.TCPAddr{IP: net.IPv4zero}},
	} {
		got := newServiceHealthProbe(listener, false)()
		if !got.Observed || got.Healthy || got.FailureCategory != telemetry.ServiceHealthFailureAPIConnectivity {
			t.Fatalf("invalid listener service-health observation = %#v", got)
		}
	}
}

func TestFrontendAssetPathsStayLocalAndBounded(t *testing.T) {
	index := []byte(`<html><head><link href="https://cdn.example/app.css"><link href="/assets/app.css?v=1"></head><body><script src="/assets/app.js"></script><script src="/not-assets/private.js"></script></body></html>`)
	got := frontendAssetPaths(index)
	if len(got) != 2 || got[0] != "/assets/app.css?v=1" || got[1] != "/assets/app.js" {
		t.Fatalf("frontend asset paths = %#v", got)
	}
}

// syncBuffer is a goroutine-safe bytes.Buffer for capturing watchdog log output
// written from another goroutine while the test polls it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// A bound-but-not-serving listener is otherwise undiagnosable: nothing accepts
// connections and no startup log line is emitted while a synchronous step
// stalls. The watchdog must name the last completed phase and dump the local
// goroutine stacks so a recurrence can be read from the field log.
func TestStartupWatchdogLogsPhaseAndStack(t *testing.T) {
	var buf syncBuffer
	phase := &startupPhase{}
	phase.mark("config watcher started")

	stop := startStartupWatchdog(zerolog.New(&buf), phase, 20*time.Millisecond)
	defer stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(buf.String(), "startup is stalled") {
		time.Sleep(5 * time.Millisecond)
	}

	logged := buf.String()
	if !strings.Contains(logged, "startup is stalled") {
		t.Fatalf("startup watchdog did not fire: %q", logged)
	}
	if !strings.Contains(logged, "config watcher started") {
		t.Fatalf("startup watchdog did not name the last completed phase: %q", logged)
	}
	if !strings.Contains(logged, "goroutine") {
		t.Fatalf("startup watchdog did not include a local goroutine stack: %q", logged)
	}
}
