package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

// Keep all real private-file publication operations except the deliberately
// failed operation. The response arms the fault after the saved-token reads.
type failingEnrollmentSaveCollector struct {
	*defaultCollector
	file, operation string
	phase           string
	armed           atomic.Bool
	failures        int
	writes          []string
}

func (c *failingEnrollmentSaveCollector) HostInfo(context.Context) (*gohost.InfoStat, error) {
	return &gohost.InfoStat{Hostname: "synthetic-enrollment-host", OS: "linux", Platform: "linux", KernelArch: "x86_64"}, nil
}

func (c *failingEnrollmentSaveCollector) fail(operation string) error {
	if c.armed.Load() && c.phase == c.file && operation == c.operation {
		c.failures++
		return errors.New("synthetic-private-path synthetic-bootstrap synthetic-runtime synthetic-storage-detail")
	}
	return nil
}

func (c *failingEnrollmentSaveCollector) MkdirAll(path string, mode os.FileMode) error {
	c.phase = runtimeTokenFile
	if len(c.writes) > 0 {
		c.phase = "agent-id"
	}
	if err := c.fail("mkdir"); err != nil {
		return err
	}
	return c.defaultCollector.MkdirAll(path, mode)
}

func (c *failingEnrollmentSaveCollector) Chmod(path string, mode os.FileMode) error {
	operation := "directory-mode"
	if filepath.Base(path) == runtimeTokenFile || filepath.Base(path) == "agent-id" {
		operation = "file-mode"
	}
	if err := c.fail(operation); err != nil {
		return err
	}
	return c.defaultCollector.Chmod(path, mode)
}

func (c *failingEnrollmentSaveCollector) WritePrivateStateFile(path string, data []byte) error {
	if err := c.fail("publish"); err != nil {
		return err
	}
	c.writes = append(c.writes, filepath.Base(path))
	return c.defaultCollector.WritePrivateStateFile(path, data)
}

func TestEnrollmentSaveFailureDoesNotReleaseAuthority(t *testing.T) {
	for _, entry := range []string{"pre-module", "legacy-loop", "legacy-run"} {
		for _, file := range []string{runtimeTokenFile, "agent-id"} {
			for _, operation := range []string{"mkdir", "directory-mode", "publish", "file-mode"} {
				t.Run(entry+"/"+file+"/"+operation, func(t *testing.T) {
					dir := t.TempDir()
					if err := os.WriteFile(filepath.Join(dir, "agent-id"), []byte("synthetic-original-id"), 0600); err != nil {
						t.Fatal(err)
					}
					trust := filepath.Join(dir, "saved-ca.pem")
					if err := os.WriteFile(trust, []byte("synthetic-trust"), 0600); err != nil {
						t.Fatal(err)
					}
					trustBefore, err := os.Stat(trust)
					if err != nil {
						t.Fatal(err)
					}
					collector := &failingEnrollmentSaveCollector{defaultCollector: &defaultCollector{}, file: file, operation: operation}
					var posts atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if posts.Add(1) != 1 {
							t.Error("local persistence failure retried the completed exchange")
							http.Error(w, "already consumed", http.StatusConflict)
							return
						}
						if r.Method != http.MethodPost || r.URL.Path != enrollEndpoint || r.Header.Get("X-API-Token") != "synthetic-bootstrap" {
							t.Error("enrollment changed its route or credential")
						}
						collector.armed.Store(true)
						w.Header().Set("Content-Type", "application/json")
						_ = json.NewEncoder(w).Encode(enrollResponse{RuntimeToken: "synthetic-runtime", AgentID: "synthetic-canonical"})
					}))
					defer server.Close()
					var logs bytes.Buffer
					logger := zerolog.New(&logs)
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					if entry == "pre-module" {
						result, err := Enroll(ctx, EnrollmentConfig{PulseURL: server.URL, APIToken: "synthetic-bootstrap", StateDir: dir, Collector: collector, Logger: &logger})
						assertEnrollmentSaveError(t, err, file, logs.String())
						if result != (EnrollmentResult{}) {
							t.Fatal("incomplete enrollment returned usable authority")
						}
					} else {
						a := &Agent{cfg: Config{APIToken: "synthetic-bootstrap", AgentID: "synthetic-original-id", Enroll: true}, agentID: "synthetic-original-id",
							stateDir: dir, collector: collector, logger: logger, trimmedPulseURL: server.URL, httpClient: server.Client()}
						var err error
						if entry == "legacy-run" {
							err = a.Run(ctx)
						} else {
							err = a.runEnrollmentLoop(ctx)
						}
						assertEnrollmentSaveError(t, err, file, logs.String())
						if a.cfg.APIToken != "synthetic-bootstrap" || a.cfg.AgentID != "synthetic-original-id" || a.agentID != "synthetic-original-id" {
							t.Error("incomplete enrollment published replacement in-memory authority")
						}
					}
					if posts.Load() != 1 || collector.failures != 1 {
						t.Fatalf("exchanges/failures = %d/%d, want one terminal save failure", posts.Load(), collector.failures)
					}
					if file == "agent-id" || operation == "file-mode" {
						assertPrivateStateFile(t, filepath.Join(dir, runtimeTokenFile), "synthetic-runtime")
					}
					if file == runtimeTokenFile || operation != "file-mode" {
						assertPrivateStateFile(t, filepath.Join(dir, "agent-id"), "synthetic-original-id")
					} else {
						// A failed post-publication permission confirmation is not absence.
						assertPrivateStateFile(t, filepath.Join(dir, "agent-id"), "synthetic-canonical")
					}
					got, readErr := os.ReadFile(trust)
					trustAfter, statErr := os.Stat(trust)
					if readErr != nil || statErr != nil || string(got) != "synthetic-trust" || trustAfter.Mode() != trustBefore.Mode() || !trustAfter.ModTime().Equal(trustBefore.ModTime()) {
						t.Error("failed enrollment changed saved trust")
					}
					entries, err := os.ReadDir(dir)
					if err != nil {
						t.Fatal(err)
					}
					for _, e := range entries {
						if strings.HasPrefix(e.Name(), ".") {
							t.Error("failed enrollment left a private publication temporary")
						}
					}
				})
			}
		}
	}
}

func assertEnrollmentSaveError(t *testing.T, err error, file, logs string) {
	t.Helper()
	if err == nil {
		t.Fatal("enrollment acknowledged an incomplete save")
	}
	want := "cannot confirm saved runtime token"
	guidance := "bootstrap token may already be consumed"
	if file == "agent-id" {
		want = "cannot confirm saved agent ID"
		guidance = "preserve runtime.token"
	}
	if !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), guidance) {
		t.Errorf("missing persistence stage or safe recovery guidance: %v", err)
	}
	all := err.Error() + logs
	for _, secret := range []string{"synthetic-private-path", "synthetic-bootstrap", "synthetic-runtime", "synthetic-storage-detail", "Enrollment succeeded, using runtime token"} {
		if strings.Contains(all, secret) {
			t.Error("incomplete enrollment disclosed private input or announced success")
		}
	}
}

func TestEnrollmentRealPublicationFailureIsTerminal(t *testing.T) {
	for _, file := range []string{runtimeTokenFile, "agent-id"} {
		t.Run(file, func(t *testing.T) {
			dir := t.TempDir()
			var posts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				posts.Add(1)
				path := filepath.Join(dir, file)
				if err := os.Mkdir(path, 0700); err != nil {
					t.Error(err)
					http.Error(w, "fixture setup failed", 500)
					return
				}
				if err := os.WriteFile(filepath.Join(path, "keep"), []byte("synthetic-preserved-state"), 0600); err != nil {
					t.Error(err)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(enrollResponse{RuntimeToken: "synthetic-runtime", AgentID: "synthetic-canonical"})
			}))
			defer server.Close()
			var logs bytes.Buffer
			logger := zerolog.New(&logs)
			collector := &enrollmentStateCollector{defaultCollector: &defaultCollector{}}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			result, err := Enroll(ctx, EnrollmentConfig{PulseURL: server.URL, APIToken: "synthetic-bootstrap", StateDir: dir, Collector: collector, Logger: &logger})
			assertEnrollmentSaveError(t, err, file, logs.String())
			if result != (EnrollmentResult{}) || posts.Load() != 1 {
				t.Fatal("failed real save released authority or repeated the exchange")
			}
			got, err := os.ReadFile(filepath.Join(dir, file, "keep"))
			if err != nil || string(got) != "synthetic-preserved-state" {
				t.Error("failed publication destroyed preexisting destination")
			}
			if file == "agent-id" {
				assertPrivateStateFile(t, filepath.Join(dir, runtimeTokenFile), "synthetic-runtime")
			}
		})
	}
}

func TestEnrollmentSavedTokenDoesNotRequireAnotherSave(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, runtimeTokenFile), []byte("synthetic-saved-runtime"), 0600); err != nil {
		t.Fatal(err)
	}
	collector := &refusingPrivateStateCollector{SystemCollector: NewDefaultCollector()}
	cfg := EnrollmentConfig{PulseURL: "https://unused.example.test", APIToken: "synthetic-consumed-bootstrap", StateDir: dir, Collector: collector}
	result, err := Enroll(context.Background(), cfg)
	if err != nil || result.APIToken != "synthetic-saved-runtime" || result.AgentID != "" || collector.attempts != 0 {
		t.Fatalf("saved authority depended on an unnecessary rewrite: %v", err)
	}
}

func TestEnrollmentPersistenceNeedsConfiguredStateDirectory(t *testing.T) {
	a := &Agent{collector: &mockCollector{}, logger: zerolog.Nop()}
	if err := a.persistRuntimeToken("synthetic-runtime"); err == nil {
		t.Error("missing credential destination was accepted as a saved token")
	}
	if err := a.persistAgentID("synthetic-id"); err == nil {
		t.Error("missing identity destination was accepted as a saved identity")
	}
}
