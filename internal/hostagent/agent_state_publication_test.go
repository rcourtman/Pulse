package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"
	gohost "github.com/shirou/gopsutil/v4/host"
)

// Keep the production filesystem implementation while making host discovery
// deterministic and proving an already-enrolled restart needs none of it.
type enrollmentStateCollector struct {
	*defaultCollector
	hostReads atomic.Int32
}

func (c *enrollmentStateCollector) HostInfo(context.Context) (*gohost.InfoStat, error) {
	if c.hostReads.Add(1) != 1 {
		return nil, fmt.Errorf("host discovery must not run on an enrolled restart")
	}
	return &gohost.InfoStat{Hostname: "synthetic-state-host", OS: "linux", Platform: "linux", KernelArch: "x86_64"}, nil
}

func TestEnrollmentPublishesPrivateStateWithoutFollowingLinks(t *testing.T) {
	for _, file := range []string{runtimeTokenFile, "agent-id"} {
		for _, shape := range []string{"absent", "regular", "symlink", "hardlink"} {
			t.Run(file+"/"+shape, func(t *testing.T) {
				stateDir := filepath.Join(t.TempDir(), "custom-instance")
				if err := os.Mkdir(stateDir, 0755); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(stateDir, file)
				victim := filepath.Join(t.TempDir(), "unrelated-file")
				// An empty runtime token must still enrol. A linked identity may
				// contain the previous canonical ID, which must not be overwritten.
				old := []byte("")
				if file == "agent-id" {
					old = []byte("synthetic-unrelated-identity")
				}
				if err := os.WriteFile(victim, old, 0644); err != nil {
					t.Fatal(err)
				}
				victimBefore, err := os.Stat(victim)
				if err != nil {
					t.Fatal(err)
				}
				switch shape {
				case "regular":
					if err := os.WriteFile(path, old, 0666); err != nil {
						t.Fatal(err)
					}
				case "symlink":
					if err := os.Symlink(victim, path); err != nil {
						t.Fatal(err)
					}
				case "hardlink":
					if err := os.Link(victim, path); err != nil {
						t.Fatal(err)
					}
				}
				var requests atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					if r.Method != http.MethodPost || r.URL.Path != enrollEndpoint || r.Header.Get("X-API-Token") != "synthetic-bootstrap" {
						t.Error("enrollment request lost its original route or credential")
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(enrollResponse{RuntimeToken: "synthetic-runtime", AgentID: "synthetic-canonical"})
				}))
				defer server.Close()
				collector := &enrollmentStateCollector{defaultCollector: &defaultCollector{}}
				cfg := EnrollmentConfig{PulseURL: server.URL, APIToken: "synthetic-bootstrap", StateDir: stateDir,
					HostnameOverride: "synthetic-state-host", Collector: collector}
				result, err := Enroll(context.Background(), cfg)
				if err != nil || result.APIToken != "synthetic-runtime" || result.AgentID != "synthetic-canonical" {
					t.Fatalf("enrollment did not retain its runtime credential and canonical identity: %v", err)
				}
				for name, want := range map[string]string{runtimeTokenFile: "synthetic-runtime", "agent-id": "synthetic-canonical"} {
					assertPrivateStateFile(t, filepath.Join(stateDir, name), want)
				}
				victimAfter, err := os.Stat(victim)
				if err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(victim)
				if err != nil || !bytes.Equal(got, old) || victimAfter.Mode() != victimBefore.Mode() || !victimAfter.ModTime().Equal(victimBefore.ModTime()) {
					t.Error("enrollment wrote credential/identity data through a link or changed the unrelated file")
				}
				// A fresh process uses the newly published runtime credential,
				// not the consumed bootstrap, and cannot re-enrol or discover hosts.
				cfg.APIToken = "synthetic-consumed-bootstrap"
				restarted, err := Enroll(context.Background(), cfg)
				if err != nil || restarted.APIToken != "synthetic-runtime" || requests.Load() != 1 || collector.hostReads.Load() != 1 {
					t.Fatalf("saved-state restart did not reuse the exact runtime credential without another request: %v", err)
				}
				entries, err := os.ReadDir(stateDir)
				if err != nil || len(entries) != 2 {
					t.Fatal("state publication left temporary files or unexpected state")
				}
				info, err := os.Stat(stateDir)
				if err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0700 {
					t.Fatal("state directory lost its private permissions")
				}
			})
		}
	}
}

func TestAgentStatePublicationDoesNotTruncateOpenPredecessor(t *testing.T) {
	for _, file := range []string{runtimeTokenFile, "agent-id"} {
		t.Run(file, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, file)
			if err := os.WriteFile(path, []byte("synthetic-previous-state"), 0644); err != nil {
				t.Fatal(err)
			}
			previous, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer previous.Close()
			a := &Agent{stateDir: dir, collector: NewDefaultCollector(), logger: zerolog.Nop()}
			publishStateFixture(a, file, "synthetic-replacement")
			assertPrivateStateFile(t, path, "synthetic-replacement")
			old, err := io.ReadAll(previous)
			if err != nil || string(old) != "synthetic-previous-state" {
				t.Fatal("publication truncated or rewrote an already-open predecessor")
			}
		})
	}
}

func TestAgentStatePublicationFailureKeepsDestination(t *testing.T) {
	for _, file := range []string{runtimeTokenFile, "agent-id"} {
		t.Run(file, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, file)
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			keep := filepath.Join(path, "keep")
			if err := os.WriteFile(keep, []byte("synthetic-saved-state"), 0600); err != nil {
				t.Fatal(err)
			}
			var logs bytes.Buffer
			a := &Agent{stateDir: dir, collector: NewDefaultCollector(), logger: zerolog.New(&logs)}
			publishStateFixture(a, file, "synthetic-replacement")
			got, err := os.ReadFile(keep)
			if err != nil || string(got) != "synthetic-saved-state" {
				t.Fatal("failed publication removed the existing destination")
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 || entries[0].Name() != file {
				t.Fatal("failed publication left temporary files")
			}
			if !strings.Contains(logs.String(), "Failed to persist") || strings.Contains(logs.String(), "synthetic-replacement") {
				t.Fatal("publication failure was not reported safely")
			}
		})
	}
}

func TestAgentStateConcurrentPublication(t *testing.T) {
	for _, file := range []string{runtimeTokenFile, "agent-id"} {
		t.Run(file, func(t *testing.T) {
			dir := t.TempDir()
			values := make(map[string]bool)
			var writers sync.WaitGroup
			start := make(chan struct{})
			for i := 0; i < 8; i++ {
				value := fmt.Sprintf("synthetic-writer-%d-", i) + strings.Repeat(fmt.Sprint(i), 8192+i)
				values[value] = true
				writers.Add(1)
				go func() {
					defer writers.Done()
					<-start
					a := &Agent{stateDir: dir, collector: NewDefaultCollector(), logger: zerolog.Nop()}
					publishStateFixture(a, file, value)
				}()
			}
			close(start)
			writers.Wait()
			data, err := os.ReadFile(filepath.Join(dir, file))
			if err != nil || !values[string(data)] {
				t.Fatal("concurrent writers published missing, mixed or partial state")
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatal("concurrent writers left temporary files")
			}
		})
	}
}

func publishStateFixture(a *Agent, file, value string) {
	if file == runtimeTokenFile {
		a.persistRuntimeToken(value)
	} else {
		a.persistAgentID(value)
	}
}

func assertPrivateStateFile(t *testing.T, path, want string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Error("state was not published as an owner-only regular file")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Error("published state differs from the exact admitted value")
	}
}

// An opted-in writer's refusal is terminal for that save. Never fall back to a
// truncating write after the secure writer reports an error.
type refusingPrivateStateCollector struct {
	SystemCollector
	attempts int
}

func (c *refusingPrivateStateCollector) WritePrivateStateFile(string, []byte) error {
	c.attempts++
	return os.ErrPermission
}

func TestAgentStatePublicationDoesNotBypassWriterFailure(t *testing.T) {
	for _, file := range []string{runtimeTokenFile, "agent-id"} {
		t.Run(file, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, file)
			if err := os.WriteFile(path, []byte("synthetic-saved-state"), 0600); err != nil {
				t.Fatal(err)
			}
			collector := &refusingPrivateStateCollector{SystemCollector: NewDefaultCollector()}
			var logs bytes.Buffer
			a := &Agent{stateDir: dir, collector: collector, logger: zerolog.New(&logs)}
			publishStateFixture(a, file, "synthetic-replacement")
			got, err := os.ReadFile(path)
			if err != nil || string(got) != "synthetic-saved-state" || collector.attempts != 1 {
				t.Fatal("secure writer failure was bypassed or replaced saved state")
			}
			if !strings.Contains(logs.String(), "Failed to persist") || strings.Contains(logs.String(), "synthetic-replacement") {
				t.Fatal("secure writer failure was not reported safely")
			}
		})
	}
}
