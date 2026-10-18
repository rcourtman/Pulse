package monitoring

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// An explicit read-only proof input lets the browser receipt consume the real
// hostagent/OpenSSH output, not a handwritten substitute. Ordinary suite runs
// still exercise the same projection with synthetic report shapes.
func TestClusterSensorProducerConsumer(t *testing.T) {
	reports := map[string][]agentshost.ClusterNodeSensors{
		"id_rsa": clusterSensorReport("peer", 55), "id_ed25519": clusterSensorReport("peer", 55),
		"denied": nil, "recovered": clusterSensorReport("peer", 55), "partial": clusterSensorReport("peer", 55),
	}
	if input := os.Getenv("PULSE_PEER_SENSOR_REPORTS"); input != "" {
		data, err := os.ReadFile(input)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &reports); err != nil {
			t.Fatal(err)
		}
	}
	if len(reports) != 5 {
		t.Fatal("expected all five producer states")
	}
	resources := make(map[string]models.ResourceFrontend)
	for _, name := range []string{"id_rsa", "id_ed25519", "denied", "recovered", "partial", "stale", "foreign", "local"} {
		t.Run(name, func(t *testing.T) {
			node := readStateTestNode("fixture", "peer")
			node.Type = "node"
			node.Host = "peer"
			node.CPU = 0.1
			node.Memory = models.Memory{Total: 8 << 30, Used: 2 << 30, Free: 6 << 30, Usage: 25}
			node.Disk = models.Disk{Total: 100 << 30, Used: 20 << 30, Free: 80 << 30, Usage: 20}
			hosts := []models.Host{reportingTestHost("reporter", "source", "fixture-source", 0)}
			if name == "local" {
				hosts = append(hosts, reportingTestHost("local-agent", "peer", node.ID, 70))
			}
			m := readStateMonitor([]models.Node{readStateTestNode("fixture", "source"), node}, hosts)
			data, ok := reports[name]
			if name == "stale" || name == "foreign" || name == "local" {
				data, ok = reports["id_rsa"]
			}
			if !ok {
				t.Fatal("missing producer state")
			}
			observedAt := time.Now()
			if name == "stale" {
				observedAt = observedAt.Add(-3 * time.Minute)
			}
			m.applyClusterSensors("reporter", data, observedAt)
			if name == "foreign" {
				node.Instance, node.ID = "other", "other-peer"
			}
			node.Temperature = m.getHostAgentTemperatureForNode(node)
			missing := name == "denied" || name == "stale" || name == "foreign"
			if missing {
				if node.Temperature != nil {
					t.Fatalf("missing/stale/foreign producer manufactured a reading: %+v", node.Temperature)
				}
			} else {
				want := 55.0
				if name == "local" {
					want = 70
				}
				if node.Temperature == nil || !node.Temperature.HasCPU || node.Temperature.CPUPackage != want {
					t.Fatalf("wrong consuming temperature, want %g: %+v", want, node.Temperature)
				}
			}
			adapter := unifiedresources.NewMonitorAdapter(nil)
			adapter.PopulateFromSnapshot(models.StateSnapshot{Nodes: []models.Node{node}})
			resource, _, ok := adapter.GetByReference(node.ID)
			if !ok {
				t.Fatal("node missing from unified resource projection")
			}
			proxmox, err := json.Marshal(resource.Proxmox)
			if err != nil {
				t.Fatal(err)
			}
			resources[name] = models.ConvertResourceToFrontend(models.ResourceConvertInput{
				ID: resource.ID, Type: string(resource.Type), Name: resource.Name,
				Status: string(resource.Status), Temperature: resource.Temperature,
				Proxmox: proxmox, LastSeenUnix: resource.LastSeen.UnixMilli(),
			})
		})
	}
	// A failed report does not refresh the age of the last successful peer
	// reading. Existing server freshness is preserved, not replaced by zero.
	t.Run("failure does not renew cached evidence", func(t *testing.T) {
		node := readStateTestNode("fixture", "peer")
		m := readStateMonitor([]models.Node{readStateTestNode("fixture", "source"), node}, []models.Host{reportingTestHost("reporter", "source", "fixture-source", 0)})
		lastGood := time.Now().Add(-30 * time.Second)
		m.applyClusterSensors("reporter", reports["id_rsa"], lastGood)
		m.applyClusterSensors("reporter", reports["denied"], time.Now())
		if got := m.getHostAgentTemperatureForNode(node); got == nil || got.CPUPackage != 55 {
			t.Fatalf("recent last-good evidence was discarded: %+v", got)
		}
		if !m.clusterSensorsCache["reporter\x00peer"].updatedAt.Equal(lastGood) {
			t.Fatal("failed report renewed the cache timestamp")
		}
	})
	data, err := json.Marshal(resources)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("PEER_SENSOR_RESOURCES=%s", data)
}
