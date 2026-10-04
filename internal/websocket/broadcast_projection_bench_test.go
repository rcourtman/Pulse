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

// The fixture models a broadcast after completed ingest. Registry reads/clones,
// identity, health, projection, encoding, deltas and client queues are real;
// persistence, agent ingest, network I/O and installed CPU are not measured.
type broadcastReadStore struct {
	*unifiedresources.MonitorAdapter
}

func (*broadcastReadStore) TryReplaceRegistryForRead(models.StateSnapshot, time.Duration, func() map[unifiedresources.DataSource][]unifiedresources.IngestRecord) bool {
	return false
}

func BenchmarkBroadcastProjection1000Resources(b *testing.B) {
	now := time.Now().UTC()
	seed := models.StateSnapshot{LastUpdate: now}
	for i := 0; i < 1000; i++ {
		seed.VMs = append(seed.VMs, models.VM{
			ID: fmt.Sprintf("lab:pve-%d:%d", i%8, i+100), VMID: i + 100,
			Name: fmt.Sprintf("vm-%d", i), Node: fmt.Sprintf("pve-%d", i%8),
			Instance: "lab", Type: "qemu", Status: "running", CPU: 0.1,
			Memory: models.Memory{Total: 4 << 30, Used: 1 << 30}, LastSeen: now,
		})
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(seed)
	monitor := &monitoring.Monitor{}
	monitor.SetResourceStore(&broadcastReadStore{unifiedresources.NewMonitorAdapter(registry)})
	for _, recipients := range []int{0, 1, 4} {
		b.Run(fmt.Sprintf("viewers-%d", recipients), func(b *testing.B) {
			sequence := int64(0)
			getter := func(string) interface{} {
				state := monitor.BuildFrontendState()
				sequence++
				state.LastUpdate = sequence
				// Change telemetry on a row, not identity/static metadata.
				state.Resources[0].CPU.Current = float64(sequence % 100)
				state.Resources[0].LastSeen = sequence
				return state
			}
			run, err := websocket.NewBroadcastProjectionProbeForTest(getter, recipients)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			var bytes int
			for i := 0; i < b.N; i++ {
				bytes, err = run()
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(bytes), "wire-B/op")
		})
	}
}
