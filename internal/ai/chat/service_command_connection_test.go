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

func TestAssistantInventoryKeepsSameNamedNodesApart(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "site-a-pmx1", Instance: "site-a", Name: "pmx1", Status: "online"},
			{ID: "site-b-pmx1", Instance: "site-b", Name: "pmx1", Status: "offline"},
			{ID: "site-b-pmx2", Instance: "site-b", Name: "pmx2", Status: "online"},
		},
		VMs: []models.VM{
			{ID: "site-a:pmx1:101", Instance: "site-a", Node: "pmx1", VMID: 101, Name: "alpha", Type: "qemu", Status: "running"},
			{ID: "site-b:pmx1:101", Instance: "site-b", Node: "pmx1", VMID: 101, Name: "bravo", Type: "qemu", Status: "running"},
		},
	})
	raw, err := marshalAssistantInventoryTopologyContextFromReadState(registry)
	if err != nil {
		t.Fatal(err)
	}
	context, ok := parseAssistantInventoryTopologyContext(raw)
	if !ok || len(context.Proxmox.Nodes) != 3 {
		t.Fatalf("inventory lost a node: %s", raw)
	}
	for i, want := range []struct{ label, instance, vm string }{
		{"Node pmx1 (site-a)", "site-a", "alpha"},
		{"Node pmx1 (site-b)", "site-b", "bravo"},
		{"Node pmx2", "", ""},
	} {
		node := context.Proxmox.Nodes[i]
		if node.AnswerLabel != want.label || node.Instance != want.instance {
			t.Fatalf("node %d is %q on %q, want %q on %q", i, node.AnswerLabel, node.Instance, want.label, want.instance)
		}
		if want.vm == "" {
			if len(node.VMs) != 0 {
				t.Fatalf("%s took guests: %+v", want.label, node.VMs)
			}
			continue
		}
		if len(node.VMs) != 1 || node.VMs[0].Name != want.vm {
			t.Fatalf("%s holds %+v, want %s", want.label, node.VMs, want.vm)
		}
	}
}
