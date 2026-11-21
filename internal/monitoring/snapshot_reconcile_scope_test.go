package monitoring

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/require"
)

func showSnapshotFixtureGuests(m *Monitor, kind string, showFirst bool) {
	var vms []models.VM
	var containers []models.Container
	for _, guest := range []struct {
		node string
		id   int
		name string
	}{{"node-a", 101, "Ledger VM"}, {"node-b", 202, "Worker VM"}} {
		if guest.id == 101 && !showFirst {
			continue
		}
		id := makeGuestID("site", guest.node, guest.id)
		if kind == "qemu" {
			vms = append(vms, models.VM{ID: id, Instance: "site", Node: guest.node, VMID: guest.id, Name: guest.name, Type: kind, Status: "running"})
		} else {
			containers = append(containers, models.Container{ID: id, Instance: "site", Node: guest.node, VMID: guest.id, Name: guest.name, Type: kind, Status: "running"})
		}
	}
	m.state.UpdateGuestsForInstance("site", vms, containers)
}

// A warm cache can carry the unknown sibling. After restart, only the durable
// evidence still knows it exists. Use the same actual snapshot HTTP reader
// and reconcile path in both cases.
func TestPVESnapshotColdSubsetDoesNotReconcileUnqueriedGuest(t *testing.T) {
	for _, kind := range []string{"qemu", "lxc"} {
		t.Run(kind, func(t *testing.T) {
			h := newSnapshotInventoryHTTPFixture(t, kind)
			showSnapshotFixtureGuests(h.f.m, kind, true)
			h.poll(t, "site")
			h.poll(t, "other-site")
			before := h.f.points(t)
			const unreadID = "pve-snapshot:site-node-a-101-weekly"
			const emptyID = "pve-snapshot:site-node-b-202-weekly"
			const otherID = "pve-snapshot:other-site-node-a-101-weekly"
			require.Len(t, before, 3)
			require.Contains(t, before, unreadID)
			require.Contains(t, before, emptyID)
			require.Contains(t, before, otherID)

			h.f.coldState(false)
			showSnapshotFixtureGuests(h.f.m, kind, false)
			h.set("node-b", `{"data":[]}`)
			h.f.m.pollGuestSnapshots(context.Background(), "site", h.client)
			h.f.wait(t)
			require.Equal(t, 4, h.requests, "one actual HTTP read for the single visible guest")
			now := h.f.points(t)
			require.Equal(t, before[unreadID], now[unreadID], "an unqueried guest's exact record must survive")
			require.Equal(t, before[otherID], now[otherID], "the namesake is untouched")
			require.NotContains(t, now, emptyID, "the queried guest's explicit empty list still removes its stale record")
			require.Empty(t, snapshotCollectionByID(h.f.m), "retained history is not fabricated live snapshot inventory")

			showSnapshotFixtureGuests(h.f.m, kind, true)
			h.set("node-a", `{"data":null}`)
			h.poll(t, "site")
			require.Equal(t, now, h.f.points(t), "returning guest plus unread snapshot listing cannot renew or delete prior history")

			h.set("node-a", `{"data":[]}`)
			h.poll(t, "site")
			after := h.f.points(t)
			require.Len(t, after, 1)
			require.Equal(t, before[otherID], after[otherID], "an explicit empty listing removes only this guest's stored row")
		})
	}
}
