package main

import (
	"bytes"
	"context"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentstate"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
)

func TestRunInvalidSavedAuthorityRefusesConsumersAndRecovers(t *testing.T) {
	for _, shape := range []string{"oversize", "unavailable-link"} {
		for _, role := range []string{"implicit-token", "selected-token", "runtime-token", "saved-id", "selected-id"} {
			t.Run(shape+"/"+role, func(t *testing.T) {
				stateDir := t.TempDir()
				files := map[string][]byte{
					"token": []byte("synthetic-saved-token\n"), "runtime.token": []byte("synthetic-saved-runtime\n"),
					"agent-id": []byte("synthetic-saved-id\n"), "connection.env": []byte("synthetic-connection-marker\n"),
				}
				var mu sync.Mutex
				var requests [][2]string
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					requests = append(requests, [2]string{r.URL.Path, r.Header.Get("X-API-Token")})
					mu.Unlock()
					if r.URL.Path == "/api/agents/agent/lookup" {
						http.NotFound(w, r)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"success":true,"config":{"commandsEnabled":true}}`)
				}))
				defer server.Close()
				files["saved-ca.pem"] = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
				for name, data := range files {
					if err := os.WriteFile(filepath.Join(stateDir, name), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
				path := filepath.Join(stateDir, "token")
				if role == "runtime-token" {
					path = filepath.Join(stateDir, "runtime.token")
				} else if role == "saved-id" {
					path = filepath.Join(stateDir, "agent-id")
				} else if strings.HasPrefix(role, "selected-") {
					path = filepath.Join(t.TempDir(), "synthetic-private-selected-file")
				}
				oversize := bytes.Repeat([]byte("x"), agentstate.MaxFileBytes+1)
				missingTarget := filepath.Join(t.TempDir(), "unavailable-authority")
				if shape == "unavailable-link" {
					if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
						t.Fatal(err)
					}
					if err := os.Symlink(missingTarget, path); err != nil {
						if runtime.GOOS == "windows" {
							t.Skip("Windows symlink privilege unavailable")
						}
						t.Fatal(err)
					}
				} else if err := os.WriteFile(path, oversize, 0600); err != nil {
					t.Fatal(err)
				}
				before, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				start := func() (*identityStartupEvidence, error) {
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
					defer cancel()
					evidence := spyIdentityStartup(t, cancel)
					enrollAgent = hostagent.Enroll
					// The real parser sees only this fixture's owned state, never an
					// installed default state directory or a collector runtime.
					directToken, tokenFile, idFile := "", "", ""
					env := func(key string) string {
						if role == "selected-token" && key == "PULSE_TOKEN" {
							return "synthetic-unselected-environment"
						}
						return ""
					}
					switch role {
					case "selected-token":
						tokenFile = path
					case "runtime-token":
						directToken = "synthetic-bootstrap"
					case "saved-id", "selected-id":
						directToken = "synthetic-direct-token"
					}
					if role == "selected-id" {
						idFile = path
					}
					err := run(ctx, []string{"--url", server.URL, "--state-dir", stateDir, "--cacert", filepath.Join(stateDir, "saved-ca.pem"),
						"--token", directToken, "--token-file", tokenFile, "--agent-id-file", idFile, fmt.Sprintf("--enroll=%t", role == "runtime-token"),
						"--hostname", "synthetic-bounds-host", "--enable-host", "--enable-docker", "--enable-kubernetes",
						"--disable-auto-update", "--health-addr", "", "--command-authority", "monitoring-only"}, env)
					return evidence, err
				}
				evidence, err := start()
				if err == nil || !strings.Contains(err.Error(), "64 KiB") || !strings.Contains(err.Error(), "regular file") || len(evidence.calls) != 0 {
					t.Fatalf("invalid authority reached startup consumers: calls=%v error=%v", evidence.calls, err)
				}
				for _, private := range []string{path, stateDir, "synthetic-", strings.Repeat("x", 100)} {
					if strings.Contains(err.Error(), private) {
						t.Error("saved-authority refusal disclosed private input")
					}
				}
				mu.Lock()
				count := len(requests)
				mu.Unlock()
				if count != 0 {
					t.Fatal("invalid saved authority reached an authenticated request")
				}
				after, statErr := os.Lstat(path)
				if statErr != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
					t.Fatal("refusal changed the invalid saved entry")
				}
				if shape == "unavailable-link" {
					if target, err := os.Readlink(path); err != nil || target != missingTarget {
						t.Fatal("refusal replaced the saved link")
					}
					if _, err := os.Stat(missingTarget); !os.IsNotExist(err) {
						t.Fatal("refusal created replacement authority at the missing target")
					}
					if !strings.Contains(err.Error(), "restore any unavailable file or directory link target") {
						t.Fatal("refusal omitted linked-storage recovery guidance")
					}
				} else if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, oversize) {
					t.Fatal("refusal changed oversized saved authority")
				}
				for name, want := range files {
					p := filepath.Join(stateDir, name)
					if p == path {
						continue
					}
					if got, err := os.ReadFile(p); err != nil || !bytes.Equal(got, want) {
						t.Fatalf("refusal changed saved %s", name)
					}
				}
				// Replace only the malformed fixture with its intended valid file.
				recovered := "synthetic-recovered-token"
				wantID, wantToken := "synthetic-saved-id", recovered
				if strings.HasSuffix(role, "id") {
					recovered, wantID, wantToken = "synthetic-recovered-id", "synthetic-recovered-id", "synthetic-direct-token"
				}
				restorePath := path
				if shape == "unavailable-link" {
					restorePath = missingTarget
				}
				if err := os.WriteFile(restorePath, []byte(" \t"+recovered+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				evidence, err = start()
				if err != nil || fmt.Sprint(evidence.calls) != "[service updater host docker kubernetes]" {
					t.Fatalf("repaired saved state did not recover startup: calls=%v error=%v", evidence.calls, err)
				}
				if evidence.host.AgentID != wantID || evidence.docker.AgentID != wantID || evidence.kube.AgentID != wantID || evidence.host.APIToken != wantToken || evidence.docker.APIToken != wantToken || evidence.kube.APIToken != wantToken || evidence.updater.APIToken != wantToken {
					t.Fatal("recovery substituted another identity or credential")
				}
				caPath := filepath.Join(stateDir, "saved-ca.pem")
				if evidence.host.InsecureSkipVerify || evidence.docker.InsecureSkipVerify || evidence.kube.InsecureSkipVerify || evidence.updater.InsecureSkipVerify || evidence.host.CACertPath != caPath || evidence.docker.CACertPath != caPath || evidence.kube.CACertPath != caPath || evidence.updater.CACertPath != caPath || evidence.host.EnableCommands || evidence.host.CommandAuthorityProfile != hostagent.CommandAuthorityMonitoringOnly {
					t.Fatal("recovery weakened verified TLS or local command ceilings")
				}
				mu.Lock()
				defer mu.Unlock()
				if len(requests) != 2 || requests[0] != [2]string{"/api/agents/agent/lookup", wantToken} || requests[1] != [2]string{"/api/agents/agent/" + wantID + "/config", wantToken} {
					t.Fatal("recovery did not use the intended authority for both config reads")
				}
			})
		}
	}
}

func TestSavedAgentIDBoundarySurvivesRepeatedPublication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-id")
	want := strings.Repeat("x", agentstate.MaxFileBytes)
	for attempt := 0; attempt < 2; attempt++ {
		if err := writeAgentIDFile(path, want); err != nil {
			t.Fatal(err)
		}
		got, err := readAgentIDFile(path)
		if err != nil || got != want {
			t.Fatalf("boundary-size identity failed round trip: %v", err)
		}
	}
}

func TestResolveTokenRejectsOversizedInjectedAuthority(t *testing.T) {
	for _, selected := range []string{"", "synthetic-selected-file"} {
		t.Run(fmt.Sprintf("selected-%t", selected != ""), func(t *testing.T) {
			token, err := resolveTokenInternal("", selected, "", t.TempDir(), func(string) ([]byte, error) {
				return bytes.Repeat([]byte("x"), agentstate.MaxFileBytes+1), nil
			})
			if err == nil || token != "" || !strings.Contains(err.Error(), "64 KiB") {
				t.Fatal("injected reader released oversized authority")
			}
		})
	}
}
