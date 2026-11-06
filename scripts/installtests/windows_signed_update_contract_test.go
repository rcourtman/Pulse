package installtests

import (
	"os"
	"strings"
	"testing"

	host "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	packet "github.com/rcourtman/pulse-go-rewrite/scripts/installtests/windowsupdatepacket"
	"gopkg.in/yaml.v3"
)

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
		if strings.Contains(step.Run, "^TestWindowsSignedAutomaticUpdate$") {
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
