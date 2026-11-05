package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"
)

// Relocating the configured state directory must not consume bootstrap
// authority without saving its replacement. Use the real filesystem writer
// and a server that refuses a second exchange, not an injected successful save.
func TestEnrollmentRestartsWithLinkedStateDirectory(t *testing.T) {
	for _, layout := range []string{"absolute", "relative", "chain", "ancestor", "missing-child"} {
		for _, destination := range []string{"absent", "symlink", "hardlink"} {
			t.Run(layout+"/"+destination, func(t *testing.T) {
				root := t.TempDir()
				relocated := filepath.Join(root, "relocated")
				if err := os.Mkdir(relocated, 0755); err != nil {
					t.Fatal(err)
				}
				alias := filepath.Join(root, "state")
				linkTarget := relocated
				switch layout {
				case "relative":
					linkTarget = "relocated"
				case "chain":
					makeStateSymlink(t, "relocated", filepath.Join(root, "middle"))
					linkTarget = "middle"
				}
				makeStateSymlink(t, linkTarget, alias)
				stateDir, realDir := alias, relocated
				if layout == "ancestor" || layout == "missing-child" {
					stateDir = filepath.Join(alias, "nested", "instance")
					realDir = filepath.Join(relocated, "nested", "instance")
					if layout == "ancestor" {
						if err := os.MkdirAll(realDir, 0755); err != nil {
							t.Fatal(err)
						}
					}
				}

				// Empty linked runtime state still needs enrollment. The ID's
				// nonempty predecessor must not be overwritten through either
				// type of destination link, even inside a linked directory.
				old := map[string]string{runtimeTokenFile: "", "agent-id": "synthetic-previous-id"}
				victims := make(map[string]os.FileInfo)
				if destination != "absent" {
					if err := os.MkdirAll(realDir, 0755); err != nil {
						t.Fatal(err)
					}
					for name, value := range old {
						victim := filepath.Join(root, "unrelated-"+name)
						if err := os.WriteFile(victim, []byte(value), 0644); err != nil {
							t.Fatal(err)
						}
						info, err := os.Stat(victim)
						if err != nil {
							t.Fatal(err)
						}
						victims[victim] = info
						path := filepath.Join(realDir, name)
						if destination == "symlink" {
							makeStateSymlink(t, victim, path)
						} else if err := os.Link(victim, path); err != nil {
							t.Fatal(err)
						}
					}
				}

				var requests atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if requests.Add(1) != 1 {
						http.Error(w, "bootstrap already consumed", http.StatusConflict)
						return
					}
					if r.Method != http.MethodPost || r.URL.Path != enrollEndpoint || r.Header.Get("X-API-Token") != "synthetic-bootstrap" {
						t.Error("enrollment changed its route or authority")
						http.Error(w, "unexpected enrollment", http.StatusUnauthorized)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(enrollResponse{RuntimeToken: "synthetic-runtime", AgentID: "synthetic-canonical"})
				}))
				defer server.Close()
				var logs bytes.Buffer
				logger := zerolog.New(&logs)
				collector := &enrollmentStateCollector{defaultCollector: &defaultCollector{}}
				cfg := EnrollmentConfig{PulseURL: server.URL, APIToken: "synthetic-bootstrap", StateDir: stateDir,
					HostnameOverride: "synthetic-state-host", Logger: &logger, Collector: collector}
				result, err := Enroll(context.Background(), cfg)
				if err != nil || result.APIToken != "synthetic-runtime" || result.AgentID != "synthetic-canonical" {
					t.Fatalf("enrollment lost its runtime authority or canonical identity: %v", err)
				}
				// A new enrollment call constructs a fresh Agent. A fresh
				// collector refuses host discovery, and the fixture server
				// refuses a second POST with the consumed bootstrap token.
				restartCollector := &enrollmentStateCollector{defaultCollector: &defaultCollector{}}
				restartCollector.hostReads.Store(1)
				cfg.Collector = restartCollector
				restarted, restartErr := Enroll(context.Background(), cfg)
				if restartErr != nil || restarted.APIToken != "synthetic-runtime" || requests.Load() != 1 || restartCollector.hostReads.Load() != 1 {
					t.Fatalf("restart failed to reuse saved runtime authority without another exchange or host discovery: %v", restartErr)
				}
				for name, want := range map[string]string{runtimeTokenFile: "synthetic-runtime", "agent-id": "synthetic-canonical"} {
					assertPrivateStateFile(t, filepath.Join(realDir, name), want)
					assertPrivateStateFile(t, filepath.Join(stateDir, name), want)
				}
				for path, before := range victims {
					after, err := os.Stat(path)
					if err != nil {
						t.Fatal(err)
					}
					data, err := os.ReadFile(path)
					name := strings.TrimPrefix(filepath.Base(path), "unrelated-")
					if err != nil || string(data) != old[name] || after.Mode() != before.Mode() || !after.ModTime().Equal(before.ModTime()) {
						t.Error("state publication modified an unrelated linked file")
					}
				}
				if target, err := os.Readlink(alias); err != nil || target != linkTarget {
					t.Error("state publication replaced or changed the configured directory link")
				}
				if info, err := os.Stat(realDir); err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0700 {
					t.Error("relocated state directory lost private permissions")
				}
				if entries, err := os.ReadDir(realDir); err != nil || len(entries) != 2 {
					t.Error("state publication left temporary or missing files")
				}
				if strings.Contains(logs.String(), "Failed to persist") || strings.Contains(logs.String(), "synthetic-runtime") || strings.Contains(logs.String(), "synthetic-bootstrap") {
					t.Error("enrollment logged a persistence failure or credential")
				}
			})
		}
	}
}

func makeStateSymlink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("creating a Windows symlink requires an available native privilege: %v", err)
		}
		t.Fatal(err)
	}
}
