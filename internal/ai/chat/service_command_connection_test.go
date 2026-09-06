package chat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestAssistantInventoryDoesNotInventCommandConnectionObservations(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Nodes:       []models.Node{{ID: "node-one", Name: "node-one", Status: "online"}},
		DockerHosts: []models.DockerHost{{ID: "host-one", Hostname: "observed-host", Status: "online", Containers: []models.DockerContainer{{ID: "app-one", Name: "observed-app", State: "running"}}}},
	})
	raw, err := marshalAssistantInventoryTopologyContextFromReadState(registry)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"agent_connected", "command_agent_connected", "can_execute", "nodes_with_agents", "docker_hosts_with_agents", "nodes_with_command_agents", "docker_hosts_with_command_agents"} {
		if strings.Contains(raw, `"`+field+`"`) {
			t.Fatalf("inventory seed invented %s without observing command connections: %s", field, raw)
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatal(err)
	}
	host := decoded["docker"].(map[string]any)["hosts"].([]any)[0].(map[string]any)
	if host["hostname"] != "observed-host" || host["container_count"] != float64(1) {
		t.Fatalf("monitoring inventory was lost: %+v", host)
	}
}
