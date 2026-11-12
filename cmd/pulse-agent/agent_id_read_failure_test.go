package main

import (
	"context"
	"errors"
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
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
	"github.com/rcourtman/pulse-go-rewrite/internal/dockeragent"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
	"github.com/rcourtman/pulse-go-rewrite/internal/kubernetesagent"
	"github.com/rs/zerolog"
)

type identityStartupEvidence struct {
	calls   []string
	host    hostagent.Config
	docker  dockeragent.Config
	kube    kubernetesagent.Config
	updater agentupdate.Config
}

// Keep the real flag parser, saved-file reader and remote config transport, but
// never run a collector, updater or enrollment request on the test host.
func spyIdentityStartup(t *testing.T, cancel context.CancelFunc) *identityStartupEvidence {
	t.Helper()
	t.Setenv("PULSE_AGENT_HELPER_SOCKET", "")
	oldEnroll, oldUpdater, oldHost := enrollAgent, newUpdater, newHostAgent
	oldDocker, oldKube, oldService := newDockerAgent, newKubeAgent, runAsWindowsServiceFunc
	t.Cleanup(func() {
		enrollAgent, newUpdater, newHostAgent = oldEnroll, oldUpdater, oldHost
		newDockerAgent, newKubeAgent, runAsWindowsServiceFunc = oldDocker, oldKube, oldService
	})
	evidence := &identityStartupEvidence{}
	enrollAgent = func(_ context.Context, cfg hostagent.EnrollmentConfig) (hostagent.EnrollmentResult, error) {
		evidence.calls = append(evidence.calls, "enroll")
		return hostagent.EnrollmentResult{APIToken: cfg.APIToken}, nil
	}
	runAsWindowsServiceFunc = func(Config, zerolog.Logger) (bool, error) {
		evidence.calls = append(evidence.calls, "service")
		return false, nil
	}
	newUpdater = func(cfg agentupdate.Config) *agentupdate.Updater {
		evidence.calls = append(evidence.calls, "updater")
		evidence.updater = cfg
		cfg.Disabled = true
		return agentupdate.New(cfg)
	}
	newHostAgent = func(cfg hostagent.Config) (Runnable, error) {
		evidence.calls = append(evidence.calls, "host")
		evidence.host = cfg
		return &mockRunnable{}, nil
	}
	newDockerAgent = func(cfg dockeragent.Config) (RunnableCloser, error) {
		evidence.calls = append(evidence.calls, "docker")
		evidence.docker = cfg
		return &mockRunnableCloser{}, nil
	}
	newKubeAgent = func(cfg kubernetesagent.Config) (Runnable, error) {
		evidence.calls = append(evidence.calls, "kubernetes")
		evidence.kube = cfg
		cancel()
		return &mockRunnable{}, nil
	}
	return evidence
}

func TestRunUnreadableAgentIDStopsBeforeAuthenticatedStartup(t *testing.T) {
	for _, location := range []string{"state-directory", "selected-file"} {
		for _, shape := range []string{"directory", "unreadable", "link-loop", "non-directory-ancestor"} {
			if location == "state-directory" && shape == "non-directory-ancestor" {
				// A non-directory state directory fails credential loading first.
				continue
			}
			for _, enroll := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/enroll-%t", location, shape, enroll), func(t *testing.T) {
					stateDir := t.TempDir()
					path := filepath.Join(stateDir, "agent-id")
					if location == "selected-file" {
						path = filepath.Join(t.TempDir(), "synthetic-private-identity")
					}
					const marker = "synthetic-original-identity\n"
					markerPath := path
					switch shape {
					case "directory":
						if err := os.Mkdir(path, 0700); err != nil {
							t.Fatal(err)
						}
						markerPath = filepath.Join(path, "keep")
						if err := os.WriteFile(markerPath, []byte(marker), 0600); err != nil {
							t.Fatal(err)
						}
					case "unreadable":
						if runtime.GOOS == "windows" {
							t.Skip("Unix permission-based read obstruction")
						}
						if err := os.WriteFile(path, []byte(marker), 0000); err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { _ = os.Chmod(path, 0600) })
					case "link-loop":
						if err := os.Symlink(filepath.Base(path), path); err != nil {
							if runtime.GOOS == "windows" {
								t.Skip("Windows symlink privilege unavailable")
							}
							t.Fatal(err)
						}
					case "non-directory-ancestor":
						if err := os.WriteFile(path, []byte(marker), 0600); err != nil {
							t.Fatal(err)
						}
						markerPath = path
						path = filepath.Join(path, "agent-id")
					}
					if _, err := os.ReadFile(path); err == nil || errors.Is(err, os.ErrNotExist) {
						t.Skip("filesystem did not classify this as a non-absence read failure")
					}
					before, err := os.Lstat(markerPath)
					if err != nil {
						t.Fatal(err)
					}
					files := map[string]string{"token": "synthetic-manual-token\n", "runtime.token": "synthetic-runtime-token\n", "saved-ca.pem": "synthetic-trust-marker\n"}
					for name, data := range files {
						if err := os.WriteFile(filepath.Join(stateDir, name), []byte(data), 0600); err != nil {
							t.Fatal(err)
						}
					}
					var requests atomic.Int64
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						requests.Add(1)
						w.WriteHeader(http.StatusUnauthorized)
					}))
					defer server.Close()
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					evidence := spyIdentityStartup(t, cancel)
					selectedPath := ""
					if location == "selected-file" {
						selectedPath = path
					}
					err = run(ctx, []string{
						"--url", server.URL, "--state-dir", stateDir, "--agent-id-file", selectedPath,
						fmt.Sprintf("--enroll=%t", enroll), "--enable-host", "--enable-docker", "--enable-kubernetes",
						"--disable-auto-update", "--health-addr", "", "--command-authority", "monitoring-only",
					}, func(string) string { return "" })
					if err == nil || !strings.Contains(err.Error(), "cannot read saved agent ID") || !strings.Contains(err.Error(), "readable by the agent service") {
						t.Errorf("unknown saved identity did not stop startup with actionable guidance: %v", err)
					}
					for _, private := range []string{stateDir, path, "synthetic-"} {
						if strings.Contains(fmt.Sprint(err), private) {
							t.Error("saved-identity error disclosed private configuration")
						}
					}
					if len(evidence.calls) != 0 || requests.Load() != 0 {
						t.Errorf("unknown identity reached consumers: %v, requests=%d", evidence.calls, requests.Load())
					}
					after, err := os.Lstat(markerPath)
					if err != nil || before.Mode() != after.Mode() {
						t.Fatal("startup changed the saved identity's file type or mode")
					}
					if shape == "link-loop" {
						if target, err := os.Readlink(path); err != nil || target != filepath.Base(path) {
							t.Fatal("startup replaced the obstructed identity link")
						}
					} else {
						if shape == "unreadable" {
							if err := os.Chmod(path, 0600); err != nil {
								t.Fatal(err)
							}
						}
						if got, err := os.ReadFile(markerPath); err != nil || string(got) != marker {
							t.Fatal("startup overwrote the saved identity")
						}
					}
					for name, data := range files {
						file := filepath.Join(stateDir, name)
						got, err := os.ReadFile(file)
						info, statErr := os.Stat(file)
						if err != nil || string(got) != data || statErr != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
							t.Fatalf("startup changed saved %s", name)
						}
					}
				})
			}
		}
	}
}

func TestRunKnownAgentIdentityStatesReachConsumers(t *testing.T) {
	for _, state := range []string{"saved", "missing", "empty", "whitespace", "explicit-flag", "explicit-environment", "flag-over-environment"} {
		t.Run(state, func(t *testing.T) {
			stateDir := t.TempDir()
			path := filepath.Join(stateDir, "agent-id")
			want := ""
			initial := ""
			mode := os.FileMode(0600)
			if state == "saved" {
				initial, want = "  synthetic-saved-id\n", "synthetic-saved-id"
			} else if state == "whitespace" {
				initial = " \n\t"
			} else if strings.Contains(state, "explicit") || state == "flag-over-environment" {
				initial, want = "synthetic-unselected-id\n", "synthetic-selected-id"
				if runtime.GOOS != "windows" {
					// An explicit ID bypasses unreadable saved state on Unix, where
					// directory permissions still permit atomic replacement. On
					// Windows mode0000 sets read-only, not unreadable, and prevents
					// replacement. Keep all precedence/restart assertions there
					// with a writable destination instead of changing its attributes.
					mode = 0000
				}
			}
			if state != "missing" {
				if err := os.WriteFile(path, []byte(initial), mode); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(path, 0600) })
			}
			var requests [][2]string
			var requestMu sync.Mutex
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestMu.Lock()
				requests = append(requests, [2]string{r.URL.Path, r.Header.Get("X-API-Token")})
				requestMu.Unlock()
				if r.URL.Path == "/api/agents/agent/lookup" {
					// No hostname remap: config must retain the ID startup selected.
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"success":true,"config":{}}`)
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			evidence := spyIdentityStartup(t, cancel)
			env := func(key string) string {
				if key == "PULSE_AGENT_ID" {
					if state == "explicit-environment" {
						return want
					}
					if state == "flag-over-environment" {
						return "synthetic-unselected-environment-id"
					}
				}
				return ""
			}
			var err error
			// Literal temporary-state arguments also satisfy the repository's
			// static guard against seeding a real service state directory.
			if state == "explicit-flag" || state == "flag-over-environment" {
				err = run(ctx, []string{
					"--url", server.URL, "--state-dir", stateDir, "--token", "synthetic-primary-token",
					"--agent-id", want, "--hostname", "synthetic-fixture-host",
					"--enable-host", "--enable-docker", "--enable-kubernetes", "--disable-auto-update",
					"--health-addr", "", "--command-authority", "monitoring-only",
				}, env)
			} else {
				err = run(ctx, []string{
					"--url", server.URL, "--state-dir", stateDir, "--token", "synthetic-primary-token",
					"--hostname", "synthetic-fixture-host", "--enable-host", "--enable-docker", "--enable-kubernetes",
					"--disable-auto-update", "--health-addr", "", "--command-authority", "monitoring-only",
				}, env)
			}
			if err != nil {
				t.Fatalf("known identity startup failed: %v", err)
			}
			id := evidence.host.AgentID
			if id == "" || want != "" && id != want {
				t.Fatalf("startup identity = %q, selected = %q", id, want)
			}
			if fmt.Sprint(evidence.calls) != "[service updater host docker kubernetes]" || evidence.docker.AgentID != id || evidence.kube.AgentID != id {
				t.Fatalf("consumers received inconsistent identity: %v", evidence.calls)
			}
			requestMu.Lock()
			defer requestMu.Unlock()
			if len(requests) != 2 || requests[0] != [2]string{"/api/agents/agent/lookup", "synthetic-primary-token"} || requests[1] != [2]string{"/api/agents/agent/" + id + "/config", "synthetic-primary-token"} {
				t.Fatal("remote config did not consume the same selected identity and credential")
			}
			if evidence.host.APIToken != "synthetic-primary-token" || evidence.docker.APIToken != "synthetic-primary-token" || evidence.kube.APIToken != "synthetic-primary-token" || evidence.updater.APIToken != "synthetic-primary-token" || evidence.host.EnableCommands || evidence.host.CommandAuthorityProfile != hostagent.CommandAuthorityMonitoringOnly || evidence.host.InsecureSkipVerify || evidence.docker.InsecureSkipVerify || evidence.kube.InsecureSkipVerify || evidence.updater.InsecureSkipVerify {
				t.Fatal("identity admission changed credentials, command ceilings or TLS verification")
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != id+"\n" {
				t.Fatal("selected identity was not preserved for restart")
			}
		})
	}
}

func TestRunLocalProbesDoNotRequireReadableAgentID(t *testing.T) {
	for _, probe := range []string{"--self-test", "--validate-url"} {
		t.Run(probe, func(t *testing.T) {
			stateDir := t.TempDir()
			path := filepath.Join(stateDir, "agent-id")
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(path, "keep")
			if err := os.WriteFile(marker, []byte("synthetic-preserved-id\n"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			evidence := spyIdentityStartup(t, cancel)
			if err := run(ctx, []string{probe, "--url", "https://pulse.example.test", "--state-dir", stateDir}, func(string) string { return "" }); err != nil {
				t.Fatalf("local probe depends on saved identity: %v", err)
			}
			if len(evidence.calls) != 0 {
				t.Fatalf("local probe reached runtime consumers: %v", evidence.calls)
			}
			if got, err := os.ReadFile(marker); err != nil || string(got) != "synthetic-preserved-id\n" {
				t.Fatal("local probe changed saved identity")
			}
		})
	}
}
