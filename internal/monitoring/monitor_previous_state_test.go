package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

func TestPreviousProxmoxMemoryKeepsPercentagePoints(t *testing.T) {
	for _, tc := range []struct {
		name string
		used int64
		want float64
	}{
		{"measured zero", 0, 0},
		{"small", 88, 0.88},
		{"ordinary", 6250, 62.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC()
			const total int64 = 10_000
			memory := models.Memory{Used: tc.used, Total: total, Free: total - tc.used, Usage: tc.want}
			registry := unifiedresources.NewRegistry(nil)
			registry.IngestSnapshot(models.StateSnapshot{
				Nodes: []models.Node{{
					ID: "pve-node", Name: "node", Instance: "pve", Status: "online", LastSeen: now, Memory: memory,
				}},
				Containers: []models.Container{{
					ID: "pve-node-101", Node: "node", Instance: "pve", VMID: 101, Type: "lxc",
					Status: "running", LastSeen: now, Memory: memory,
				}},
			})
			nodes, containers := registry.Nodes(), registry.Containers()
			require.Len(t, nodes, 1)
			require.Len(t, containers, 1)

			node := previousNodeFromView(nodes[0])
			container := previousContainerFromView(containers[0])
			for _, carried := range []models.Memory{node.Memory, container.Memory} {
				require.Equal(t, total, carried.Total)
				require.Equal(t, tc.used, carried.Used)
				require.InDelta(t, tc.want, carried.Usage, 1e-9)
			}

			next := unifiedresources.NewRegistry(nil)
			next.IngestSnapshot(models.StateSnapshot{Nodes: []models.Node{node}, Containers: []models.Container{container}})
			require.Len(t, next.Nodes(), 1)
			require.Len(t, next.Containers(), 1)
			require.InDelta(t, tc.want, next.Nodes()[0].MemoryPercent(), 1e-9)
			require.InDelta(t, tc.want, next.Containers()[0].MemoryPercent(), 1e-9)
		})
	}
}
