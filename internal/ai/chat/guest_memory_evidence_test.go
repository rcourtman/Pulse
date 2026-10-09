package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestAssistantGuestMemoryContextAndFacts(t *testing.T) {
	at := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	for _, kind := range []string{"vm", "system-container"} {
		for _, tc := range []struct {
			name, state, source, text string
			percent                   float64
			unavailable, known        bool
		}{
			{"healthy", "current", "guest-agent-meminfo", "cache-aware guest usage", 24, false, true},
			{"measured-zero", "current", "agent", "0% (current", 0, false, true},
			{"pressure", "current", "available-field", "cache-aware guest usage", 96, false, true},
			{"cache", "current", "status-mem", "may include reclaimable cache", 96, false, false},
			{"retained", "last-known", "agent", "guest pressure unknown", 24, false, false},
			{"missing", "unavailable", "unavailable", "N/A (guest memory unavailable; not evidence of recovery)", 0, true, false},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				memory := models.Memory{Total: 20 << 30, Used: int64(float64(20<<30) * tc.percent / 100), Usage: tc.percent, UsageUnavailable: tc.unavailable,
					Observation: models.MemoryObservation{State: tc.state, Source: tc.source, ObservedAt: at}}
				snapshot := models.StateSnapshot{}
				if kind == "vm" {
					snapshot.VMs = []models.VM{{ID: "guest", Name: "guest", Instance: "fixture", Node: "node", VMID: 100, Status: "running", LastSeen: time.Now(), Memory: memory}}
				} else {
					snapshot.Containers = []models.Container{{ID: "guest", Name: "guest", Instance: "fixture", Node: "node", VMID: 100, Status: "running", LastSeen: time.Now(), Memory: memory}}
				}
				registry := unifiedresources.NewRegistry(nil)
				registry.IngestSnapshot(snapshot)
				raw, err := marshalAssistantInventoryTopologyContextFromReadState(registry)
				if err != nil {
					t.Fatal(err)
				}
				var decoded map[string]interface{}
				if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
					t.Fatal(err)
				}
				family := "vms"
				if kind != "vm" {
					family = "containers"
				}
				row := decoded["proxmox"].(map[string]interface{})["nodes"].([]interface{})[0].(map[string]interface{})[family].([]interface{})[0].(map[string]interface{})
				e, ok := row["memory_evidence"].(map[string]interface{})
				if !ok || e["available"] != !tc.unavailable || e["pressure_known"] != tc.known {
					t.Errorf("Assistant seed lost qualification: %s", raw)
				}
				if !tc.unavailable && row["memory_percent"] != tc.percent || tc.unavailable && row["memory_percent"] != nil {
					t.Errorf("Assistant seed confused absent/measured memory: %+v", row)
				}
				id := ""
				if kind == "vm" {
					id = registry.VMs()[0].ID()
				} else {
					id = registry.Containers()[0].ID()
				}
				exec := tools.NewPulseToolExecutor(tools.ExecutorConfig{ReadState: registry})
				input := map[string]interface{}{"action": "get", "resource_type": kind, "resource_id": id}
				result, err := exec.ExecuteTool(context.Background(), agentcapabilities.PulseQueryToolName, input)
				if err != nil || result.IsError {
					t.Fatalf("actual resource query failed: %v %+v", err, result)
				}
				facts := extractQueryGetFacts(input, result.Content[0].Text)
				if len(facts) == 0 || !strings.Contains(facts[0].Value, tc.text) {
					t.Errorf("condensed facts removed memory qualification: %+v", facts)
				}
				if !tc.unavailable && (len(facts) == 0 || !strings.Contains(facts[0].Value, at.Format(time.RFC3339))) {
					t.Errorf("facts renewed original memory time: %+v", facts)
				}
			})
		}
	}
}
