package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func boolPtr(value bool) *bool { return &value }

// Docker reports exit code 0 for a container that is running or was never
// started. Only a container that has ended, or is being restarted, has an exit
// code that says anything.
func TestContainerExitFactsOnlyForAContainerThatEnded(t *testing.T) {
	for _, tc := range []struct {
		state    string
		exitCode int
		oom      *bool
		wantCode *int
		wantOOM  *bool
	}{
		{"exited", 0, boolPtr(false), intPtr(0), boolPtr(false)},
		{" Exited ", 139, nil, intPtr(139), nil},
		{"dead", 137, boolPtr(true), intPtr(137), boolPtr(true)},
		{"restarting", 1, nil, intPtr(1), nil},
		{"running", 0, boolPtr(false), nil, nil},
		{"created", 0, nil, nil, nil},
		{"paused", 0, nil, nil, nil},
		{"", 3, nil, nil, nil},
	} {
		code, oom := containerExitFacts(tc.state, tc.exitCode, tc.oom)
		if (code == nil) != (tc.wantCode == nil) || (code != nil && *code != *tc.wantCode) {
			t.Errorf("state %q: exit code = %v, want %v", tc.state, deref(code), deref(tc.wantCode))
		}
		if (oom == nil) != (tc.wantOOM == nil) || (oom != nil && *oom != *tc.wantOOM) {
			t.Errorf("state %q: oom = %v, want %v", tc.state, oom, tc.wantOOM)
		}
		if oom != nil && oom == tc.oom {
			t.Errorf("state %q: the out-of-memory flag is shared with the source", tc.state)
		}
	}
}

func intPtr(value int) *int { return &value }

func deref(value *int) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

// Patrol's investigator reads a container through these payloads. On a default
// agent it has no other way to learn how a stopped container ended, and the
// difference between exit code 0 and 139 is the difference between a clean
// stop and a crash. On 12 November 2026 a conclusion about a cleanly stopped
// service told the owner to find out the exit code on the host.
func TestContainerPayloadsCarryTheExitCodeOfAStoppedContainer(t *testing.T) {
	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{{
			ID: "host1", Hostname: "docker01", DisplayName: "docker01",
			Containers: []models.DockerContainer{
				{ID: "c-clean", Name: "gitea", State: "exited", Image: "gitea/gitea:1.22.3", ExitCode: 0, OOMKilled: boolPtr(false)},
				{ID: "c-crash", Name: "immich_machine_learning", State: "exited", Image: "immich-ml", ExitCode: 139},
				{ID: "c-loop", Name: "vaultwarden", State: "restarting", Image: "vaultwarden/server", ExitCode: 1},
				{ID: "c-up", Name: "paperless", State: "running", Image: "paperless"},
			},
		}},
	}
	executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})

	result, err := executor.executeGetTopology(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("topology: %v", err)
	}
	var topology TopologyResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &topology); err != nil {
		t.Fatalf("decode topology: %v", err)
	}
	if len(topology.Docker.Hosts) != 1 {
		t.Fatalf("topology hosts = %+v", topology.Docker.Hosts)
	}
	got := map[string]DockerContainerSummary{}
	for _, container := range topology.Docker.Hosts[0].Containers {
		got[container.Name] = container
	}
	if c := got["gitea"]; c.ExitCode == nil || *c.ExitCode != 0 || c.OOMKilled == nil || *c.OOMKilled {
		t.Fatalf("a clean stop lost its exit code or out-of-memory answer: %+v", c)
	}
	if c := got["immich_machine_learning"]; c.ExitCode == nil || *c.ExitCode != 139 || c.OOMKilled != nil {
		t.Fatalf("a crash lost its exit code, or gained an out-of-memory answer nobody gave: %+v", c)
	}
	if c := got["vaultwarden"]; c.ExitCode == nil || *c.ExitCode != 1 {
		t.Fatalf("a restarting container lost its last exit code: %+v", c)
	}
	if c := got["paperless"]; c.ExitCode != nil || c.OOMKilled != nil {
		t.Fatalf("a running container carries an exit code that means nothing: %+v", c)
	}
	// The key is absent for a running container, not present as 0.
	raw := result.Content[0].Text
	if strings.Count(raw, `"exit_code"`) != 3 {
		t.Fatalf("topology carries %d exit codes, want 3:\n%s", strings.Count(raw, `"exit_code"`), raw)
	}

	for name, want := range map[string]*int{"gitea": intPtr(0), "immich_machine_learning": intPtr(139), "paperless": nil} {
		result, err := executor.executeGetResource(context.Background(), map[string]interface{}{
			"resource_type": "app-container",
			"resource_id":   name,
		})
		if err != nil {
			t.Fatalf("get %s: %v", name, err)
		}
		var response ResourceResponse
		if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
			t.Fatalf("decode %s: %v\n%s", name, err, result.Content[0].Text)
		}
		if (response.ExitCode == nil) != (want == nil) || (want != nil && *response.ExitCode != *want) {
			t.Fatalf("%s: exit code = %v, want %v (%s)", name, deref(response.ExitCode), deref(want), result.Content[0].Text)
		}
	}
}

// The resource-scoped health read goes through the unified resource, the path
// a running server takes for an investigation.
func TestResourceHealthCarriesTheExitCodeOfAStoppedContainer(t *testing.T) {
	const canonicalID = "app-container-67d0edb8f133784a"
	resource := func(state string, exitCode int) *stubUnifiedResourceProvider {
		return &stubUnifiedResourceProvider{resources: []unifiedresources.Resource{{
			ID: canonicalID, Type: unifiedresources.ResourceTypeAppContainer, Technology: "docker", Name: "gitea",
			Status: unifiedresources.StatusOffline, ParentName: "docker01",
			Docker: &unifiedresources.DockerData{ContainerID: strings.Repeat("6", 64), ContainerState: state, ExitCode: exitCode, OOMKilled: boolPtr(false)},
		}}}
	}
	read := func(provider *stubUnifiedResourceProvider) ResourceResponse {
		executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}, UnifiedResourceProvider: provider})
		result, err := executor.executeQuery(context.Background(), map[string]interface{}{"action": "health", "resource_id": canonicalID})
		if err != nil {
			t.Fatalf("resource-scoped health: %v", err)
		}
		var response ResourceResponse
		if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return response
	}

	stopped := read(resource("exited", 0))
	if stopped.ExitCode == nil || *stopped.ExitCode != 0 || stopped.OOMKilled == nil || *stopped.OOMKilled {
		t.Fatalf("a cleanly stopped container reads %+v, want exit code 0 and no out-of-memory kill", stopped)
	}
	if running := read(resource("running", 0)); running.ExitCode != nil || running.OOMKilled != nil {
		t.Fatalf("a running container carries exit facts: %+v", running)
	}
}
