package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

func TestPreviousGuestContextKeepsScopedCarryInViewOrder(t *testing.T) {
	now := time.Now()
	vm := func(site, name string, id int) models.VM {
		return models.VM{
			ID: makeGuestID(site, "node", id), VMID: id, Instance: site, Node: "node",
			Name: name, Status: "running", Type: "qemu", LastSeen: now,
			IPAddresses: []string{"192.0.2.20"}, OSName: "Fixture Linux",
		}
	}
	ct := func(site, name string, id int, oci bool) models.Container {
		kind := "lxc"
		if oci {
			kind = "oci"
		}
		return models.Container{
			ID: makeGuestID(site, "node", id), VMID: id, Instance: site, Node: "node",
			Name: name, Status: "running", Type: kind, IsOCI: oci, LastSeen: now,
			IPAddresses: []string{"192.0.2.30"}, HasDocker: true, DockerCheckedAt: now,
		}
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		VMs: []models.VM{
			vm("subject", "zulu", 101), vm("subject", "alpha", 102), vm("subject", "echo", 103),
			vm("peer", "zulu", 101), vm("peer", "alpha", 102),
		},
		Containers: []models.Container{
			ct("subject", "zulu", 201, false), ct("subject", "alpha", 202, true), ct("subject", "echo", 203, false),
			ct("peer", "zulu", 201, true), ct("peer", "alpha", 202, false),
		},
	})
	m := &Monitor{state: models.NewState(), resourceStore: unifiedresources.NewMonitorAdapter(registry)}

	got := m.previousGuestContextForInstance("subject")
	assertGuestIDs(t, got.vms, []string{"subject:node:102", "subject:node:103", "subject:node:101"})
	assertGuestIDs(t, got.containers, []string{"subject:node:202", "subject:node:203", "subject:node:201"})
	require.Equal(t, len(got.vms), cap(got.vms), "do not reserve peer VM rows")
	require.Equal(t, len(got.containers), cap(got.containers), "do not reserve peer container rows")
	require.Len(t, got.vmsByID, len(got.vms))
	require.Len(t, got.containersByID, len(got.containers))
	for _, row := range got.vms {
		require.Equal(t, row, got.vmsByID[row.ID])
		require.Equal(t, "Fixture Linux", row.OSName)
		require.Equal(t, []string{"192.0.2.20"}, row.IPAddresses)
	}
	for _, row := range got.containers {
		require.Equal(t, row, got.containersByID[row.ID])
		require.True(t, row.HasDocker)
		require.True(t, now.Equal(row.DockerCheckedAt))
	}
	require.Equal(t, map[int]bool{202: true}, got.containerOCIByVMID)

	got.vms[0].IPAddresses[0] = "198.51.100.1"
	got.containers[0].IPAddresses[0] = "198.51.100.2"
	next := m.previousGuestContextForInstance("subject")
	require.Equal(t, []string{"192.0.2.20"}, next.vms[0].IPAddresses)
	require.Equal(t, []string{"192.0.2.30"}, next.containers[0].IPAddresses)
	empty := m.previousGuestContextForInstance("missing")
	require.Empty(t, empty.vmsByID)
	require.Empty(t, empty.containersByID)
	require.Zero(t, cap(empty.vms))
	require.Zero(t, cap(empty.containers))
}

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
