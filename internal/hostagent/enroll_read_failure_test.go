package hostagent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	gohost "github.com/shirou/gopsutil/v4/host"
)

// Exercise both enrollment entry points with real saved files. Only the read
// obstruction is synthetic; the HTTP request and state writer are production.
type obstructedRuntimeCollector struct {
	*defaultCollector
	obstruction error
	absentFirst bool
	reads       int
	hostReads   int
}

func (c *obstructedRuntimeCollector) ReadFile(path string) ([]byte, error) {
	if filepath.Base(path) == runtimeTokenFile {
		c.reads++
		if c.absentFirst && c.reads == 1 {
			return nil, os.ErrNotExist
		}
		if c.obstruction != nil {
			return []byte("synthetic-partial-credential"), c.obstruction
		}
	}
	return c.defaultCollector.ReadFile(path)
}

func (c *obstructedRuntimeCollector) HostInfo(context.Context) (*gohost.InfoStat, error) {
	c.hostReads++
	return &gohost.InfoStat{Hostname: "synthetic-saved-host", Platform: "linux", KernelArch: "x86_64"}, nil
}

func TestEnrollmentRejectsUnknownRuntimeState(t *testing.T) {
	for _, entry := range []string{"pre-module", "second-read", "legacy-loop", "legacy-run"} {
		for _, failure := range []struct {
			name string
			err  error
		}{
			{"permission", os.ErrPermission},
			{"io", errors.New("synthetic-private-io-detail")},
			{"wrapped-permission", fmt.Errorf("synthetic-private-wrapper: %w", os.ErrPermission)},
			{"path-error", &os.PathError{Op: "read-synthetic-private-operation", Path: "synthetic-private-path", Err: os.ErrPermission}},
		} {
			t.Run(entry+"/"+failure.name, func(t *testing.T) {
				dir := t.TempDir()
				old := map[string][]byte{runtimeTokenFile: []byte("synthetic-original-runtime\n"), "agent-id": []byte("synthetic-original-id"), "saved-ca.pem": []byte("synthetic-trust-marker")}
				before := make(map[string]os.FileInfo)
				for name, data := range old {
					path := filepath.Join(dir, name)
					if err := os.WriteFile(path, data, 0600); err != nil {
						t.Fatal(err)
					}
					info, err := os.Stat(path)
					if err != nil {
						t.Fatal(err)
					}
					before[name] = info
				}
				var posts atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					posts.Add(1)
					if r.Method != http.MethodPost || r.URL.Path != enrollEndpoint || r.Header.Get("X-API-Token") != "synthetic-bootstrap" {
						t.Error("unexpected enrollment route or authority")
					}
					if entry == "legacy-run" {
						http.Error(w, "bootstrap consumed", http.StatusConflict)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"runtimeToken":"synthetic-replacement-runtime","agentId":"synthetic-replacement-id"}`))
				}))
				defer server.Close()
				collector := &obstructedRuntimeCollector{defaultCollector: &defaultCollector{}, obstruction: failure.err, absentFirst: entry == "second-read"}
				var logs bytes.Buffer
				logger := zerolog.New(&logs)
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				cfg := EnrollmentConfig{PulseURL: server.URL, APIToken: "synthetic-bootstrap", StateDir: dir, Collector: collector, Logger: &logger}
				var err error
				if strings.HasPrefix(entry, "legacy-") {
					a := &Agent{cfg: Config{APIToken: cfg.APIToken, AgentID: "synthetic-original-id", Enroll: true}, agentID: "synthetic-original-id", stateDir: dir, collector: collector, logger: logger, httpClient: server.Client(), trimmedPulseURL: server.URL}
					if entry == "legacy-run" {
						err = a.Run(ctx)
					} else {
						err = a.runEnrollmentLoop(ctx)
					}
					if a.cfg.APIToken != cfg.APIToken || a.agentID != "synthetic-original-id" || a.cfg.AgentID != "synthetic-original-id" {
						t.Error("unknown runtime state replaced in-memory authority or identity")
					}
				} else {
					var result EnrollmentResult
					result, err = Enroll(ctx, cfg)
					if result.APIToken != "" || result.AgentID != "" {
						t.Error("unknown runtime state returned replacement authority or identity")
					}
				}
				if err == nil || !strings.Contains(err.Error(), "cannot read saved runtime token") || !strings.Contains(err.Error(), "readable by the agent service") {
					t.Errorf("unknown saved credential needs actionable read failure; got error=%v", err)
				}
				if posts.Load() != 0 {
					t.Errorf("unknown saved state caused %d enrollment POST(s)", posts.Load())
				}
				expectedReads := 1
				expectedHostReads := 0
				if entry == "second-read" {
					expectedReads = 2
					expectedHostReads = 1
				}
				if collector.reads != expectedReads || collector.hostReads != expectedHostReads {
					t.Errorf("read/discovery counts = %d/%d, want %d/%d", collector.reads, collector.hostReads, expectedReads, expectedHostReads)
				}
				for name, want := range old {
					path := filepath.Join(dir, name)
					got, readErr := os.ReadFile(path)
					info, statErr := os.Stat(path)
					if readErr != nil || statErr != nil || !bytes.Equal(got, want) || info.Mode() != before[name].Mode() || !info.ModTime().Equal(before[name].ModTime()) {
						t.Errorf("unknown saved state changed %s bytes or metadata", name)
					}
				}
				if entries, readErr := os.ReadDir(dir); readErr != nil || len(entries) != len(old) {
					t.Error("unknown saved state changed directory entries")
				}
				diagnostic := logs.String() + fmt.Sprint(err)
				for _, private := range []string{"synthetic-partial-credential", "synthetic-private-io-detail", "synthetic-private-wrapper", "synthetic-private-operation", "synthetic-private-path", "synthetic-bootstrap", "synthetic-original-runtime", dir} {
					if strings.Contains(diagnostic, private) {
						t.Error("runtime read failure disclosed credential, path or underlying error detail")
					}
				}
				// Removing only the obstruction must recover the original authority, with
				// no host discovery or replacement enrollment and no new identity.
				collector.obstruction = nil
				collector.absentFirst = false
				hostReads := collector.hostReads
				restarted, restartErr := Enroll(ctx, cfg)
				if restartErr != nil || restarted.APIToken != "synthetic-original-runtime" || restarted.AgentID != "" || posts.Load() != 0 || collector.hostReads != hostReads {
					t.Errorf("restart did not recover original saved authority without enrollment: err=%v posts=%d", restartErr, posts.Load())
				}
			})
		}
	}
}

func TestEnrollmentPreservesKnownRuntimeStates(t *testing.T) {
	for _, entry := range []string{"pre-module", "legacy-loop"} {
		for _, state := range []string{"missing", "empty", "whitespace", "saved"} {
			t.Run(entry+"/"+state, func(t *testing.T) {
				dir := t.TempDir()
				values := map[string]string{"empty": "", "whitespace": " \t\r\n", "saved": " \tsynthetic-known-runtime\r\n"}
				if data, ok := values[state]; ok {
					if err := os.WriteFile(filepath.Join(dir, runtimeTokenFile), []byte(data), 0600); err != nil {
						t.Fatal(err)
					}
				}
				var posts atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					posts.Add(1)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"runtimeToken":"synthetic-first-runtime","agentId":"synthetic-first-id"}`))
				}))
				defer server.Close()
				collector := &obstructedRuntimeCollector{defaultCollector: &defaultCollector{}}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				var token string
				var err error
				if entry == "pre-module" {
					var result EnrollmentResult
					result, err = Enroll(ctx, EnrollmentConfig{PulseURL: server.URL, APIToken: "synthetic-bootstrap", StateDir: dir, Collector: collector})
					token = result.APIToken
				} else {
					a := &Agent{cfg: Config{APIToken: "synthetic-bootstrap"}, stateDir: dir, collector: collector, logger: zerolog.Nop(), httpClient: server.Client(), trimmedPulseURL: server.URL}
					err = a.runEnrollmentLoop(ctx)
					token = a.cfg.APIToken
				}
				wantToken := "synthetic-first-runtime"
				wantPosts := int32(1)
				if state == "saved" {
					wantToken = "synthetic-known-runtime"
					wantPosts = 0
				}
				if err != nil || token != wantToken || posts.Load() != wantPosts {
					t.Errorf("known state lost existing enrollment behavior: err=%v posts=%d", err, posts.Load())
				}
			})
		}
	}
}
