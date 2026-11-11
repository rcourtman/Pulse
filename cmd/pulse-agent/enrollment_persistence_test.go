package main

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
)

// Use the real enrollment implementation and private filesystem writer, then
// prove its error reaches the actual startup's consumers before any is built.
func TestRunEnrollmentPersistenceFailureStopsAuthenticatedConsumers(t *testing.T) {
	for _, file := range []string{"runtime.token", "agent-id", "selected-agent-id"} {
		t.Run(file, func(t *testing.T) {
			stateDir := t.TempDir()
			selectedID := ""
			if file == "selected-agent-id" {
				selectedID = filepath.Join(stateDir, "selected-agent-id")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			evidence := spyIdentityStartup(t, cancel)
			enrollAgent = hostagent.Enroll
			oldGate := enrollBeforeModuleStartup
			enrollBeforeModuleStartup = true
			t.Cleanup(func() { enrollBeforeModuleStartup = oldGate })
			var posts, downstream atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/agents/agent/enroll" {
					downstream.Add(1)
					http.NotFound(w, r)
					return
				}
				if posts.Add(1) != 1 || r.Header.Get("X-API-Token") != "synthetic-bootstrap" {
					t.Error("enrollment was retried or changed its bootstrap authority")
					http.Error(w, "already consumed", http.StatusConflict)
					return
				}
				path := filepath.Join(stateDir, file)
				if file != "runtime.token" {
					if err := os.Rename(path, filepath.Join(stateDir, "previous-agent-id")); err != nil {
						t.Error(err)
						http.Error(w, "fixture setup failed", 500)
						return
					}
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Error(err)
					http.Error(w, "fixture setup failed", 500)
					return
				}
				if err := os.WriteFile(filepath.Join(path, "keep"), []byte("synthetic-saved-state"), 0600); err != nil {
					t.Error(err)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{"runtimeToken": "synthetic-runtime", "agentId": "synthetic-canonical"})
			}))
			defer server.Close()
			caPath := filepath.Join(stateDir, "saved-ca.pem")
			ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
			if err := os.WriteFile(caPath, ca, 0600); err != nil {
				t.Fatal(err)
			}
			err := run(ctx, []string{"--url", server.URL, "--token", "synthetic-bootstrap", "--enroll", "--state-dir", stateDir,
				"--hostname", "synthetic-state-host", "--agent-id", "synthetic-original-id", "--agent-id-file", selectedID, "--cacert", caPath, "--enable-host", "--enable-docker", "--enable-kubernetes", "--health-addr", ""}, func(string) string { return "" })
			want := "cannot confirm saved runtime token"
			if file != "runtime.token" {
				want = "cannot confirm saved agent ID"
			}
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("startup did not return its enrollment persistence failure: %v", err)
			}
			for _, private := range []string{stateDir, "synthetic-bootstrap", "synthetic-runtime"} {
				if strings.Contains(err.Error(), private) {
					t.Error("startup error disclosed private enrollment input")
				}
			}
			if posts.Load() != 1 || downstream.Load() != 0 || len(evidence.calls) != 0 {
				t.Fatalf("startup after failed save: exchanges=%d downstream=%d constructors=%v", posts.Load(), downstream.Load(), evidence.calls)
			}
			got, err := os.ReadFile(filepath.Join(stateDir, file, "keep"))
			if err != nil || string(got) != "synthetic-saved-state" {
				t.Error("startup replaced the obstructed saved state")
			}
			got, err = os.ReadFile(caPath)
			if err != nil || string(got) != string(ca) {
				t.Error("startup modified its saved TLS trust")
			}
			if file != "runtime.token" {
				got, err := os.ReadFile(filepath.Join(stateDir, "runtime.token"))
				if err != nil || string(got) != "synthetic-runtime" {
					t.Error("later identity failure lost the newly saved runtime credential")
				}
			}
		})
	}
}
