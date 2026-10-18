package websocket_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

// Source-native mixed estate, close to #2199's reported resource/Kubernetes
// scale. Completed ingest is outside the read measurement, as in the existing
// broadcast probe. Transport, compression and installed CPU are not measured.
func connectedCostMonitor() *monitoring.Monitor {
	now := time.Unix(1791283200, 0).UTC()
	seed := models.StateSnapshot{LastUpdate: now}
	cluster := models.KubernetesCluster{ID: "cost-cluster", Name: "cost-cluster", Status: "online", LastSeen: now}
	for i := 0; i < 1000; i++ {
		cluster.Pods = append(cluster.Pods, models.KubernetesPod{UID: fmt.Sprintf("pod-%04d", i), Name: fmt.Sprintf(" API-Workload-%04d ", 1000-i), Namespace: "workloads", Phase: "Running", UsageCPUPercent: 12.5, UsageMemoryBytes: 9007199254740993, Labels: map[string]string{"app": "api", "team": "ops", "env": "prod", "description": "unchanged source metadata<&>"}, Containers: []models.KubernetesPodContainer{{Name: "api", Ready: true}}})
	}
	seed.KubernetesClusters = []models.KubernetesCluster{cluster}
	for i := 0; i < 235; i++ {
		seed.VMs = append(seed.VMs, models.VM{ID: fmt.Sprintf("lab:pve-%d:%d", i%8, i+100), VMID: i + 100, Name: fmt.Sprintf(" MixedCase-VM-%04d ", 235-i), Node: fmt.Sprintf("pve-%d", i%8), Instance: "lab", Type: "qemu", Status: "running", CPU: 0.1, Memory: models.Memory{Total: 4 << 30, Used: 1 << 30}, LastSeen: now})
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(seed)
	monitor := &monitoring.Monitor{}
	monitor.SetResourceStore(&broadcastReadStore{unifiedresources.NewMonitorAdapter(registry)})
	return monitor
}
func BenchmarkConnectedDashboardCost(b *testing.B) {
	for _, mode := range []string{"cold", "quiet", "metric", "heartbeat", "churn"} {
		for _, viewers := range []int{0, 1, 4, 17} {
			b.Run(fmt.Sprintf("%s/viewers-%d", mode, viewers), func(b *testing.B) {
				monitor := connectedCostMonitor()
				if count := len(monitor.BuildBroadcastFrontendState().Resources); count != 1236 {
					b.Fatalf("resources=%d, want1236", count)
				}
				sequence := int64(0)
				getter := func(string) interface{} {
					state := monitor.BuildBroadcastFrontendState()
					sequence++
					switch mode {
					case "metric":
						state.LastUpdate = sequence
						state.Resources[0].CPU = &models.ResourceMetricFrontend{Current: float64(sequence % 100)}
					case "heartbeat":
						state.LastUpdate = sequence
						for i := range state.Resources {
							state.Resources[i].LastSeen = sequence
						}
					case "churn":
						state.LastUpdate = sequence
						state.Resources = state.Resources[:len(state.Resources)-int(sequence%2)]
						state.Resources[0].Labels = map[string]string{"changed": fmt.Sprint(sequence)}
						state.Resources[0].DisplayName = fmt.Sprint(sequence)
						state.Resources[0], state.Resources[len(state.Resources)-1] = state.Resources[len(state.Resources)-1], state.Resources[0]
					}
					return state
				}
				run, err := websocket.NewQuietBroadcastProjectionProbeForTest(getter, viewers)
				if err != nil {
					b.Fatal(err)
				}
				// Shared accepted baselines, not independent initial-state snapshots.
				if mode != "quiet" {
					if _, err = run(); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportAllocs()
				b.ResetTimer()
				wire := 0
				for i := 0; i < b.N; i++ {
					if mode == "cold" {
						run, err = websocket.NewQuietBroadcastProjectionProbeForTest(getter, viewers)
						if err != nil {
							b.Fatal(err)
						}
					}
					wire, err = run()
					if err != nil {
						b.Fatal(err)
					}
					if viewers > 0 && mode != "quiet" && mode != "cold" && wire == 0 {
						b.Fatal("live workload queued no frame")
					}
				}
				b.ReportMetric(float64(wire), "wire-B/op")
			})
		}
	}
}
