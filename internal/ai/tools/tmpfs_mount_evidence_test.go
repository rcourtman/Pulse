package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestQueryPreservesMountConfigurationEvidence(t *testing.T) {
	for _, provider := range []bool{false, true} {
		name := "typed read state"
		if provider {
			name = "canonical provider"
		}
		t.Run(name, func(t *testing.T) {
			snapshot := commandEvidenceSnapshot()
			snapshot.DockerHosts[0].Containers[0].Mounts = []models.DockerContainerMount{
				{Type: "tmpfs", Destination: "/var/lib/service-cache", Mode: "rw,noexec,nosuid,nodev,size=8388608", RW: true},
				{Type: "tmpfs", Destination: "/readonly-cache", Mode: "ro,noexec", RW: false},
				{Type: "bind", Source: "/host/data", Destination: "/data", Mode: "", RW: false},
			}
			registry := unifiedresources.NewRegistry(nil)
			registry.IngestSnapshot(snapshot)
			cfg := ExecutorConfig{ReadState: registry, ControlLevel: ControlLevelReadOnly}
			if provider {
				cfg.UnifiedResourceProvider = &registryUnifiedQueryProvider{registry}
			}
			executor := NewPulseToolExecutor(cfg)
			list, err := executor.executeQuery(context.Background(), map[string]interface{}{"action": "list", "type": "docker-hosts"})
			if err != nil || list.IsError {
				t.Fatalf("list: %v %+v", err, list)
			}
			var hosts map[string]any
			if err := json.Unmarshal([]byte(list.Content[0].Text), &hosts); err != nil {
				t.Fatal(err)
			}
			id := hosts["docker_hosts"].([]any)[0].(map[string]any)["containers"].([]any)[0].(map[string]any)["id"]
			if !provider {
				// The typed compatibility path currently accepts provider IDs/names.
				id = snapshot.DockerHosts[0].Containers[0].Name
			}
			args := map[string]interface{}{"action": "get", "resource_type": "app-container", "resource_id": id}
			result, err := executor.executeQuery(context.Background(), args)
			if err != nil || result.IsError {
				t.Fatalf("get: %v %+v", err, result)
			}
			var decoded map[string]any
			if err := json.Unmarshal([]byte(result.Content[0].Text), &decoded); err != nil {
				t.Fatal(err)
			}
			mounts, ok := decoded["mounts"].([]any)
			if !ok {
				t.Fatalf("missing mount projection: %+v", decoded)
			}
			if len(mounts) != 3 {
				t.Fatalf("lost mounts: %+v", mounts)
			}
			for i, want := range snapshot.DockerHosts[0].Containers[0].Mounts {
				got := mounts[i].(map[string]any)
				if got["type"] != want.Type || got["source"] != want.Source || got["destination"] != want.Destination || got["rw"] != want.RW || (want.Mode != "" && got["mode"] != want.Mode) {
					t.Fatalf("mount provenance/access changed: %+v, want %+v", got, want)
				}
			}
			if _, exists := decoded["disk"]; exists {
				t.Fatalf("mount configuration invented capacity: %+v", decoded["disk"])
			}
			capture, _ := json.Marshal(map[string]any{"case": name, "input": args, "output": decoded})
			t.Logf("MOUNT_EVIDENCE %s", capture)
			if provider {
				resources := registry.ListByType(unifiedresources.ResourceTypeAppContainer)
				for _, host := range registry.ListByType(unifiedresources.ResourceTypeAgent) {
					if host.Docker != nil {
						resources = append(resources, host)
					}
				}
				encoded, err := json.Marshal(resources)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("MOUNT_RESOURCES %s", encoded)
			}
		})
	}
}
