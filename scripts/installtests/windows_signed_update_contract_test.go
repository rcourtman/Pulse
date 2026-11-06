package installtests

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
	host "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	packet "github.com/rcourtman/pulse-go-rewrite/scripts/installtests/windowsupdatepacket"
	"gopkg.in/yaml.v3"
)

func TestWindowsSignedUpdateTransportUsesActualUpdaterRoute(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "canonical"
		if legacy {
			name = "old-unserved-route"
		}
		t.Run(name, func(t *testing.T) {
			ready := make(chan struct{})
			close(ready)
			transport := &signedWindowsTransport{versionReady: ready}
			peer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Counterexample: the previous shim never served the actual
				// updater route. Do not retry or accept that version-check error.
				if legacy && r.URL.Path == "/api/agent/version" {
					http.NotFound(w, r)
					return
				}
				transport.ServeHTTP(w, r)
			}))
			defer peer.Close()
			ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: peer.Certificate().Raw})
			path := filepath.Join(t.TempDir(), "fixture-ca.pem")
			if err := os.WriteFile(path, ca, 0600); err != nil {
				t.Fatal(err)
			}
			// Same-version checks use the REAL updater without replacing any
			// binary. The native test owns Windows replacement and SCM proof.
			updater := agentupdate.New(agentupdate.Config{PulseURL: peer.URL, APIToken: signedWindowsToken, CurrentVersion: packet.After, CACertPath: path})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			updater.CheckAndUpdate(ctx)
			status := updater.Snapshot()
			transport.mu.Lock()
			defer transport.mu.Unlock()
			if legacy {
				if status.State != agentupdate.UpdateStateError || status.LastError == "" || transport.versionChecks != 0 {
					t.Fatal("unserved actual version route was accepted")
				}
			} else if status.State != agentupdate.UpdateStateIdle || status.LastError != "" || status.LastCheckedAt == nil || transport.versionChecks != 1 {
				t.Fatalf("actual updater did not complete its authenticated verified-TLS version check: state=%s, checks=%d", status.State, transport.versionChecks)
			}
			if transport.downloads != 0 || transport.unexpected != 0 {
				t.Fatal("same-version transport check reached another operation")
			}
		})
	}
}

func TestWindowsSignedUpdatePowerShellExitContract(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows PowerShell 5.1 exit semantics require Windows")
	}
	for _, test := range []struct {
		name, script string
		success      bool
	}{
		{"normal", "Write-Output 'complete'", true},
		{"handled-absence-return", "try { throw 'synthetic handled absence' } catch { return }", true},
		{"unhandled-failure", "throw 'synthetic unhandled observation failure'", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", signedWindowsPSCommand(test.script))
			_, err := cmd.CombinedOutput()
			if (err == nil) != test.success || ctx.Err() != nil {
				t.Fatalf("PowerShell exit contract differs: expected success=%t, error=%v", test.success, err)
			}
		})
	}
}

func TestWindowsSignedUpdateRequiresActualConsumerResult(t *testing.T) {
	for _, test := range []struct {
		name, refusal string
		report        host.AgentInfo
		accepted      bool
	}{
		{"valid-replacement", "", host.AgentInfo{Version: packet.After, UpdatedFrom: packet.Before}, true},
		{"old-report", "", host.AgentInfo{Version: packet.Before, UpdatedFrom: packet.Before}, false},
		{"missing-provenance", "", host.AgentInfo{Version: packet.After}, false},
		{"wrong-provenance", "", host.AgentInfo{Version: packet.After, UpdatedFrom: "v1.0.0"}, false},
		{"actual-refusal", "signature verification failed", host.AgentInfo{Version: packet.Before, Update: &host.UpdateStatus{State: "error", LastError: "signature verification failed against all trusted keys"}}, true},
		{"missing-refusal", "signature verification failed", host.AgentInfo{Version: packet.Before}, false},
		{"unrelated-error", "signature verification failed", host.AgentInfo{Version: packet.Before, Update: &host.UpdateStatus{State: "error", LastError: "TLS failed"}}, false},
		{"false-success", "signature verification failed", host.AgentInfo{Version: packet.Before, Update: &host.UpdateStatus{State: "idle", LastError: "signature verification failed"}}, false},
		{"mutated-after-refusal", "signature verification failed", host.AgentInfo{Version: packet.After, Update: &host.UpdateStatus{State: "error", LastError: "signature verification failed"}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if signedWindowsResult(test.refusal, test.report) != test.accepted {
				t.Fatal("accepted missing or unrelated updater result")
			}
		})
	}
}

func TestWindowsSignedUpdateWorkflowBindsBothSourceStages(t *testing.T) {
	content, err := os.ReadFile(repoFile(".github", "workflows", "unified-agent-native.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On   map[string]struct{ Paths []string }
		Jobs map[string]struct {
			Runner string `yaml:"runs-on"`
			Needs  string
			Steps  []struct {
				Name, Run string
				Continue  bool `yaml:"continue-on-error"`
				Env       map[string]string
				With      map[string]string
			}
		}
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatal(err)
	}
	for _, event := range []string{"push", "pull_request"} {
		paths := strings.Join(workflow.On[event].Paths, "\n")
		for _, required := range []string{"internal/api/unified_agent*.go", "internal/api/windows_update_packet_test.go", "internal/agentupdate/**", "scripts/installtests/**"} {
			if !strings.Contains(paths, required) {
				t.Fatalf("%s misses %s", event, required)
			}
		}
	}
	producer := workflow.Jobs["windows-update-producer"]
	consumer := workflow.Jobs["windows-signed-autoupdate"]
	if producer.Runner != "ubuntu-24.04" || consumer.Runner != "windows-2025" || consumer.Needs != "windows-update-producer" {
		t.Fatal("missing fixed producer/native consumer ordering")
	}
	foundProducer, foundConsumer := false, false
	for _, step := range producer.Steps {
		if strings.Contains(step.Run, "^TestWindowsSignedUpdatePacket$") {
			foundProducer = step.Env["EXPECTED_SOURCE_SHA"] == "${{ github.sha }}" && strings.Contains(step.Run, `test "$(git rev-parse HEAD)" = "$EXPECTED_SOURCE_SHA"`) && !step.Continue
		}
	}
	for _, step := range consumer.Steps {
		if strings.Contains(step.Run, "^TestWindowsSigned(AutomaticUpdate|UpdatePowerShellExitContract)$") {
			foundConsumer = step.Env["EXPECTED_SOURCE_SHA"] == "${{ github.sha }}" && step.Env["PULSE_WINDOWS_SIGNED_UPDATE"] == "1" && strings.Contains(step.Run, "$LASTEXITCODE -ne 0") && strings.Contains(step.Run, "$receipt.owned_scm_cleanup -ne $true") && !step.Continue
		}
	}
	if !foundProducer || !foundConsumer {
		t.Fatal("missing required exact-source producer or native terminal acceptance")
	}
	for _, job := range []string{"windows-update-producer", "windows-signed-autoupdate"} {
		found := false
		for _, step := range workflow.Jobs[job].Steps {
			found = found || step.With["name"] == "windows-update-packet-${{ github.run_id }}-${{ github.run_attempt }}"
		}
		if !found {
			t.Fatal("production responses can come from another run/attempt")
		}
	}
}
