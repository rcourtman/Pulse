package hostagent

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentstate"
	"github.com/rs/zerolog"
)

func TestEnrollmentInvalidSavedAuthorityNeverReenrolls(t *testing.T) {
	for _, entry := range []string{"load", "pre-module", "legacy-loop", "legacy-run"} {
		for _, shape := range []string{"oversize", "directory"} {
			t.Run(entry+"/"+shape, func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, runtimeTokenFile)
				marker := path
				want := bytes.Repeat([]byte("x"), agentstate.MaxFileBytes+1)
				if shape == "directory" {
					if err := os.Mkdir(path, 0700); err != nil {
						t.Fatal(err)
					}
					marker = filepath.Join(path, "keep")
					want = []byte("synthetic-preserved-state")
				}
				if err := os.WriteFile(marker, want, 0600); err != nil {
					t.Fatal(err)
				}
				before, err := os.Stat(marker)
				if err != nil {
					t.Fatal(err)
				}
				for name, data := range map[string]string{"agent-id": "synthetic-original-id", "ca.pem": "synthetic-saved-trust"} {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
						t.Fatal(err)
					}
				}
				var posts atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					posts.Add(1)
					http.Error(w, "must not re-enroll", http.StatusConflict)
				}))
				defer server.Close()
				collector := &enrollmentStateCollector{defaultCollector: &defaultCollector{}}
				var logs bytes.Buffer
				logger := zerolog.New(&logs)
				cfg := EnrollmentConfig{PulseURL: server.URL, APIToken: "synthetic-bootstrap", StateDir: dir, Collector: collector, Logger: &logger}
				var result EnrollmentResult
				if entry == "load" {
					result.APIToken, err = LoadRuntimeToken(dir)
				} else if entry == "pre-module" {
					result, err = Enroll(context.Background(), cfg)
				} else {
					a := &Agent{cfg: Config{APIToken: cfg.APIToken, AgentID: "synthetic-original-id", Enroll: true},
						agentID: "synthetic-original-id", stateDir: dir, collector: collector, logger: logger, trimmedPulseURL: server.URL, httpClient: server.Client()}
					if entry == "legacy-run" {
						err = a.Run(context.Background())
					} else {
						err = a.runEnrollmentLoop(context.Background())
					}
					if a.cfg.APIToken != cfg.APIToken || a.cfg.AgentID != "synthetic-original-id" || a.agentID != "synthetic-original-id" {
						t.Error("invalid saved authority changed live credentials or identity")
					}
				}
				if err == nil || !strings.Contains(err.Error(), "cannot read saved runtime token") || !strings.Contains(err.Error(), "regular file of at most 64 KiB") || result != (EnrollmentResult{}) || posts.Load() != 0 || collector.hostReads.Load() != 0 {
					t.Fatalf("invalid saved authority was not terminal before discovery/exchange: result=%t posts=%d reads=%d error=%v", result != (EnrollmentResult{}), posts.Load(), collector.hostReads.Load(), err)
				}
				for _, private := range []string{dir, "synthetic-", strings.Repeat("x", 100)} {
					if strings.Contains(err.Error()+logs.String(), private) {
						t.Error("saved-state refusal disclosed private input")
					}
				}
				after, statErr := os.Stat(marker)
				got, readErr := os.ReadFile(marker)
				if readErr != nil || statErr != nil || !bytes.Equal(got, want) || !os.SameFile(before, after) || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
					t.Fatal("refusal modified saved runtime state")
				}
				for name, want := range map[string]string{"agent-id": "synthetic-original-id", "ca.pem": "synthetic-saved-trust"} {
					if got, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(got) != want {
						t.Error("refusal modified identity or trust")
					}
				}
				// The fixture owns these files. Repair just its invalid token
				// destination, then prove both enrollment paths reuse the intended
				// saved token without host discovery or another exchange.
				if shape == "directory" {
					if err := os.Remove(marker); err != nil {
						t.Fatal(err)
					}
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(path, []byte(" synthetic-restored-runtime\n"), 0600); err != nil {
					t.Fatal(err)
				}
				restarted, err := Enroll(context.Background(), cfg)
				if err != nil || restarted.APIToken != "synthetic-restored-runtime" || restarted.AgentID != "" || posts.Load() != 0 || collector.hostReads.Load() != 0 {
					t.Fatalf("repaired authority did not restart without enrollment: %v", err)
				}
				a := &Agent{cfg: Config{APIToken: cfg.APIToken}, stateDir: dir, collector: collector, logger: logger}
				if err := a.runEnrollmentLoop(context.Background()); err != nil || a.cfg.APIToken != "synthetic-restored-runtime" || posts.Load() != 0 || collector.hostReads.Load() != 0 {
					t.Fatalf("legacy restart did not retain recovered authority: %v", err)
				}
			})
		}
	}
}

func TestEnrollmentBoundsLegacyCollectorAuthority(t *testing.T) {
	for _, size := range []int{agentstate.MaxFileBytes, agentstate.MaxFileBytes + 1} {
		t.Run(fmt.Sprintf("bytes-%d", size), func(t *testing.T) {
			collector := &mockCollector{readFileFn: func(string) ([]byte, error) { return bytes.Repeat([]byte("x"), size), nil }}
			token, err := persistedRuntimeToken(collector, t.TempDir())
			if size > agentstate.MaxFileBytes {
				if err == nil || token != "" {
					t.Fatal("legacy collector released oversized saved authority")
				}
			} else if err != nil || len(token) != size {
				t.Fatalf("legacy collector lost complete boundary-size authority: %v", err)
			}
		})
	}
}

func TestEnrollmentUnavailableLinkedStateRefusesAndRestoresWithoutExchange(t *testing.T) {
	for _, entry := range []string{"load", "pre-module", "legacy-loop", "legacy-run"} {
		for _, shape := range []string{"token-link", "state-directory-link"} {
			t.Run(entry+"/"+shape, func(t *testing.T) {
				savedDir := t.TempDir()
				files := map[string]string{runtimeTokenFile: "synthetic-retained-runtime\n", "agent-id": "synthetic-retained-id", "ca.pem": "synthetic-retained-trust"}
				for name, data := range files {
					if err := os.WriteFile(filepath.Join(savedDir, name), []byte(data), 0600); err != nil {
						t.Fatal(err)
					}
				}
				stateDir := savedDir
				linkPath := filepath.Join(stateDir, runtimeTokenFile)
				missing := filepath.Join(t.TempDir(), "unavailable-state")
				runtimePath := filepath.Join(savedDir, runtimeTokenFile)
				restore := func() error { return os.Rename(savedDir, missing) }
				if shape == "token-link" {
					runtimePath = filepath.Join(t.TempDir(), "retained-runtime")
					if err := os.Rename(filepath.Join(savedDir, runtimeTokenFile), runtimePath); err != nil {
						t.Fatal(err)
					}
					restore = func() error { return os.Rename(runtimePath, missing) }
				} else {
					stateDir = filepath.Join(t.TempDir(), "relocated")
					linkPath = stateDir
					// Let Windows create a directory link before the target is
					// deliberately made unavailable, not a file-typed link.
					if err := os.Mkdir(missing, 0700); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(missing, linkPath); err != nil {
					if runtime.GOOS == "windows" {
						t.Skip("Windows symlink privilege unavailable")
					}
					t.Fatal(err)
				}
				if shape == "state-directory-link" {
					if err := os.Remove(missing); err != nil {
						t.Fatal(err)
					}
				}
				before, err := os.Lstat(linkPath)
				if err != nil {
					t.Fatal(err)
				}
				var posts atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					posts.Add(1)
					http.Error(w, "must not re-enroll", http.StatusConflict)
				}))
				defer server.Close()
				collector := &enrollmentStateCollector{defaultCollector: &defaultCollector{}}
				var logs bytes.Buffer
				logger := zerolog.New(&logs)
				cfg := EnrollmentConfig{PulseURL: server.URL, APIToken: "synthetic-bootstrap", StateDir: stateDir, Collector: collector, Logger: &logger}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				var result EnrollmentResult
				if entry == "load" {
					result.APIToken, err = LoadRuntimeToken(stateDir)
				} else if entry == "pre-module" {
					result, err = Enroll(ctx, cfg)
				} else {
					a := &Agent{cfg: Config{APIToken: cfg.APIToken, AgentID: "synthetic-retained-id", Enroll: true}, agentID: "synthetic-retained-id",
						stateDir: stateDir, collector: collector, logger: logger, trimmedPulseURL: server.URL, httpClient: server.Client()}
					if entry == "legacy-run" {
						err = a.Run(ctx)
					} else {
						err = a.runEnrollmentLoop(ctx)
					}
					if a.cfg.APIToken != cfg.APIToken || a.agentID != "synthetic-retained-id" || a.cfg.AgentID != "synthetic-retained-id" {
						t.Error("unavailable saved state replaced live authority")
					}
				}
				if err == nil || result != (EnrollmentResult{}) || posts.Load() != 0 || collector.hostReads.Load() != 0 || !strings.Contains(err.Error(), "restore any unavailable file or directory link target") {
					t.Fatalf("unavailable linked state reached discovery/exchange: posts=%d reads=%d error=%v", posts.Load(), collector.hostReads.Load(), err)
				}
				for _, private := range []string{savedDir, stateDir, missing, "synthetic-"} {
					if strings.Contains(err.Error()+logs.String(), private) {
						t.Error("linked-state refusal disclosed private input")
					}
				}
				after, statErr := os.Lstat(linkPath)
				target, linkErr := os.Readlink(linkPath)
				if statErr != nil || linkErr != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) || target != missing {
					t.Fatal("refusal changed the saved link")
				}
				if _, err := os.Stat(missing); !os.IsNotExist(err) {
					t.Fatal("refusal created state at the unavailable target")
				}
				for name, want := range files {
					path := filepath.Join(savedDir, name)
					if name == runtimeTokenFile {
						path = runtimePath
					}
					if got, err := os.ReadFile(path); err != nil || string(got) != want {
						t.Fatal("refusal changed retained state")
					}
				}
				if err := restore(); err != nil {
					t.Fatal(err)
				}
				restarted, err := Enroll(ctx, cfg)
				if err != nil || restarted.APIToken != "synthetic-retained-runtime" || restarted.AgentID != "" || posts.Load() != 0 || collector.hostReads.Load() != 0 {
					t.Fatalf("restored target did not reuse retained authority: %v", err)
				}
				a := &Agent{cfg: Config{APIToken: cfg.APIToken}, stateDir: stateDir, collector: collector, logger: logger}
				if err := a.runEnrollmentLoop(ctx); err != nil || a.cfg.APIToken != "synthetic-retained-runtime" || posts.Load() != 0 || collector.hostReads.Load() != 0 {
					t.Fatalf("legacy restart did not reuse retained authority: %v", err)
				}
			})
		}
	}
}
