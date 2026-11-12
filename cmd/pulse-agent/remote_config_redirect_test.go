package main

import (
	"bytes"
	"context"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
	"github.com/rcourtman/pulse-go-rewrite/internal/remoteconfig"
	"github.com/rs/zerolog"
)

const remoteRedirectStartupSecret = "synthetic-startup-destination-not-for-diagnostics"
const remoteRedirectStartupToken = "synthetic-startup-credential-not-for-diagnostics"

func TestRunRemoteConfigRedirectKeepsLocalAuthorityAndDiagnostics(t *testing.T) {
	originalLevel := zerolog.GlobalLevel()
	t.Cleanup(func() { zerolog.SetGlobalLevel(originalLevel) })
	t.Setenv("PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED", "")
	for _, stage := range []string{"lookup", "config"} {
		for _, shape := range []string{"absolute", "malformed"} {
			t.Run(stage+"/"+shape, func(t *testing.T) {
				dir := t.TempDir()
				var targetHits, unexpected, source atomic.Int32
				var restored atomic.Bool
				target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetHits.Add(1); w.WriteHeader(http.StatusNoContent) }))
				defer target.Close()
				location := target.URL + "/" + remoteRedirectStartupSecret + "?token=" + remoteRedirectStartupToken
				if shape == "malformed" {
					location = "http://[" + remoteRedirectStartupSecret + "?token=" + remoteRedirectStartupToken
				}
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					source.Add(1)
					if r.Header.Get("Authorization") != "Bearer "+remoteRedirectStartupToken || r.Header.Get("X-API-Token") != remoteRedirectStartupToken {
						t.Error("startup changed saved credential")
					}
					lookup := r.URL.Path == "/api/agents/agent/lookup"
					if !lookup && r.URL.Path != "/api/agents/agent/synthetic-saved-id/config" {
						unexpected.Add(1)
						http.NotFound(w, r)
						return
					}
					if !restored.Load() && (stage == "lookup" && lookup || stage == "config" && !lookup) {
						w.Header().Set("Location", location)
						w.WriteHeader(http.StatusTemporaryRedirect)
						_, _ = io.WriteString(w, remoteRedirectStartupSecret)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					if lookup {
						_, _ = io.WriteString(w, `{"success":true,"agent":{"id":"synthetic-saved-id"}}`)
						return
					}
					// A later valid response still cannot promote a monitoring-only launch.
					_, _ = io.WriteString(w, `{"success":true,"agentId":"synthetic-saved-id","config":{"commandsEnabled":true,"settings":{}}}`)
				}))
				defer server.Close()
				saved := map[string][]byte{"agent-id": []byte("synthetic-saved-id\n"), "token": []byte(remoteRedirectStartupToken + "\n"), "connection.env": []byte("synthetic-saved-connection\n"), "ca.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})}
				for name, data := range saved {
					if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
				start := func() *identityStartupEvidence {
					t.Helper()
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					evidence := spyIdentityStartup(t, cancel)
					err := run(ctx, []string{"--url", server.URL, "--token-file", filepath.Join(dir, "token"), "--state-dir", dir, "--agent-id-file", filepath.Join(dir, "agent-id"), "--hostname", "synthetic-host", "--cacert", filepath.Join(dir, "ca.pem"), "--enable-host", "--enable-docker", "--enable-kubernetes", "--disable-auto-update", "--health-addr", "", "--command-authority", "monitoring-only", "--log-level", "warn", "--log-file", filepath.Join(dir, "agent.log")}, func(string) string { return "" })
					if err != nil {
						t.Fatalf("startup did not preserve local fallback: %v", err)
					}
					for _, marker := range evidence.calls {
						if marker == "enroll" {
							t.Error("remote-config refusal reached enrollment")
						}
					}
					if evidence.host.APIToken != remoteRedirectStartupToken || evidence.docker.APIToken != remoteRedirectStartupToken || evidence.kube.APIToken != remoteRedirectStartupToken || evidence.updater.APIToken != remoteRedirectStartupToken || evidence.host.AgentID != "synthetic-saved-id" || evidence.docker.AgentID != "synthetic-saved-id" || evidence.kube.AgentID != "synthetic-saved-id" || evidence.host.EnableCommands || evidence.host.CommandAuthorityProfile != hostagent.CommandAuthorityMonitoringOnly || evidence.host.InsecureSkipVerify || evidence.updater.InsecureSkipVerify {
						t.Error("startup/recovery changed identity, credential, TLS or local command ceiling")
					}
					for name, data := range saved {
						got, err := os.ReadFile(filepath.Join(dir, name))
						if err != nil || !bytes.Equal(got, data) {
							t.Error("startup/recovery changed saved authority/trust")
						}
					}
					if _, err := os.Stat(filepath.Join(dir, "runtime.token")); !os.IsNotExist(err) {
						t.Error("startup/recovery created enrollment state")
					}
					return evidence
				}
				start()
				logs, err := os.ReadFile(filepath.Join(dir, "agent.log"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(logs), "HTTP 307 redirect") || !strings.Contains(string(logs), "final Pulse URL") || !strings.Contains(string(logs), "fetch_failed") {
					t.Error("startup lost actionable refusal diagnostic")
				}
				if strings.Contains(string(logs), remoteRedirectStartupSecret) || strings.Contains(string(logs), remoteRedirectStartupToken) {
					t.Error("startup log disclosed redirect destination/credential")
				}
				want := int32(1)
				if stage == "config" {
					want = 2
				}
				if source.Load() != want || targetHits.Load() != 0 || unexpected.Load() != 0 {
					t.Error("startup refusal retried or followed redirect")
				}
				restored.Store(true)
				start()
				if source.Load() != want+2 || targetHits.Load() != 0 || unexpected.Load() != 0 {
					t.Error("same-credential startup recovery changed routing")
				}
			})
		}
	}
}

type remoteRefreshLog struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	once   sync.Once
	failed chan struct{}
}

func (w *remoteRefreshLog) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buffer.Write(p)
	if bytes.Contains(p, []byte(`"action":"refresh_failed"`)) {
		w.once.Do(func() { close(w.failed) })
	}
	return n, err
}
func (w *remoteRefreshLog) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.String()
}

type remoteRefreshApply func(map[string]interface{}, *bool)

func (f remoteRefreshApply) ApplyRemoteConfig(settings map[string]interface{}, commands *bool) {
	f(settings, commands)
}

func TestRemoteConfigRefreshRedirectPreservesAppliedStateThenRecovers(t *testing.T) {
	t.Setenv("PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED", "")
	old := remoteConfigRefreshInterval
	remoteConfigRefreshInterval = 5 * time.Millisecond
	t.Cleanup(func() { remoteConfigRefreshInterval = old })
	for _, stage := range []string{"lookup", "config"} {
		t.Run(stage, func(t *testing.T) {
			var restored atomic.Bool
			var targets, unexpected atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targets.Add(1) }))
			defer target.Close()
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer "+remoteRedirectStartupToken {
					t.Error("refresh changed credential")
				}
				lookup := r.URL.Path == "/api/agents/agent/lookup"
				if !lookup && r.URL.Path != "/api/agents/agent/synthetic-saved-id/config" {
					unexpected.Add(1)
					http.NotFound(w, r)
					return
				}
				if !restored.Load() && (stage == "lookup" && lookup || stage == "config" && !lookup) {
					w.Header().Set("Location", target.URL+"/"+remoteRedirectStartupSecret+"?token="+remoteRedirectStartupToken)
					w.WriteHeader(http.StatusFound)
					_, _ = io.WriteString(w, `{"config":{"commandsEnabled":true,"settings":{"interval":"1ms"}}}`)
					return
				}
				if lookup {
					_, _ = io.WriteString(w, `{"success":true,"agent":{"id":"synthetic-saved-id"}}`)
					return
				}
				_, _ = io.WriteString(w, `{"success":true,"agentId":"synthetic-saved-id","config":{"commandsEnabled":false,"settings":{"interval":"60s"}}}`)
			}))
			defer server.Close()
			dir := t.TempDir()
			ca := filepath.Join(dir, "ca.pem")
			if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
				t.Fatal(err)
			}
			writer := &remoteRefreshLog{failed: make(chan struct{})}
			logger := zerolog.New(writer)
			client := remoteconfig.New(remoteconfig.Config{PulseURL: server.URL, APIToken: remoteRedirectStartupToken, AgentID: "synthetic-saved-id", Hostname: "synthetic-host", CACertPath: ca, Logger: logger})
			defer client.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			applied := make(chan struct{}, 1)
			var calls atomic.Int32
			applier := remoteRefreshApply(func(settings map[string]interface{}, commands *bool) {
				calls.Add(1)
				if !restored.Load() || commands == nil || *commands || settings["interval"] != "60s" {
					t.Error("refresh applied refused or changed configuration")
				}
				select {
				case applied <- struct{}{}:
				default:
				}
				cancel()
			})
			done := make(chan struct{})
			go func() { defer close(done); runRemoteConfigLoop(ctx, client, []RemoteConfigApplier{applier}, &logger) }()
			select {
			case <-writer.failed:
			case <-ctx.Done():
				<-done
				t.Fatal("refresh refusal was not observed")
			}
			if calls.Load() != 0 {
				t.Error("refused refresh overwrote applied settings")
			}
			restored.Store(true)
			select {
			case <-applied:
			case <-ctx.Done():
				t.Error("refresh did not recover with same authority")
			}
			cancel()
			<-done
			logs := writer.String()
			if calls.Load() != 1 || targets.Load() != 0 || unexpected.Load() != 0 {
				t.Error("refresh lost single-apply or no-follow behavior")
			}
			if !strings.Contains(logs, "HTTP 302 redirect") || !strings.Contains(logs, "final Pulse URL") {
				t.Error("refresh lost refusal guidance")
			}
			if strings.Contains(logs, remoteRedirectStartupSecret) || strings.Contains(logs, remoteRedirectStartupToken) {
				t.Error("periodic refresh log disclosed redirect destination/credential")
			}
		})
	}
}
