package monitoring

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestOptionalAPTDenialExpiryRecoveryAndIsolation(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		t.Run(kind, func(t *testing.T) {
			f, client := optionalDenialClient(t, kind, http.StatusForbidden)
			original := time.Now().Add(-2 * pendingUpdatesCacheTTL)
			m := &Monitor{nodePendingUpdatesCache: map[string]pendingUpdatesCache{"lab-node": {count: 7, checkedAt: original}}}
			poll := func(instance, name, id, status string) models.Node {
				n := models.Node{}
				m.applyNodePendingUpdates(context.Background(), instance, client, proxmox.Node{Node: name}, id, status, &n)
				return n
			}
			first := poll("lab", "node", "lab-node", "online")
			denied := m.nodePendingUpdatesCache["lab-node"]
			if first.PendingUpdatesStatus != "stale" || denied.deniedAt.IsZero() || !denied.checkedAt.Equal(original) || denied.count != 7 {
				t.Fatalf("denial lost original evidence: %+v / %+v", first, denied)
			}
			f.status.Store(http.StatusOK)
			poll("lab", "node", "lab-node", "online")
			offline := poll("lab", "node", "lab-node", "offline")
			if f.aptCalls.Load() != 1 || m.nodePendingUpdatesCache["lab-node"] != denied || offline.PendingUpdatesStatus != "not_checked" || offline.PendingUpdatesReason != "node_offline" {
				t.Error("backoff/offline poll renewed or discarded denial")
			}
			for _, target := range [][3]string{{"lab", "other", "lab-other"}, {"peer", "node", "peer-node"}} {
				n := poll(target[0], target[1], target[2], "online")
				if n.PendingUpdatesStatus != "checked" || n.PendingUpdates != 0 || n.PendingUpdatesCheckedAt.IsZero() {
					t.Error("denial leaked to another connection/node")
				}
			}
			denied.deniedAt = time.Now().Add(-pendingUpdatesCacheTTL)
			m.nodePendingUpdatesCache["lab-node"] = denied
			recovered := poll("lab", "node", "lab-node", "online")
			cached := m.nodePendingUpdatesCache["lab-node"]
			if f.aptCalls.Load() != 4 || recovered.PendingUpdatesStatus != "checked" || recovered.PendingUpdates != 0 || recovered.PendingUpdatesReason != "" || !recovered.PendingUpdatesCheckedAt.After(original) || !cached.deniedAt.IsZero() {
				t.Errorf("expiry did not recover confirmed zero: %+v / %+v", recovered, cached)
			}
			poll("lab", "node", "lab-node", "online")
			if f.aptCalls.Load() != 4 || m.nodePendingUpdatesCache["lab-node"] != cached {
				t.Error("successful cache cadence/clock changed")
			}
		})
	}
}

func TestOptionalAPTRepeatedDenialReprobesWithoutRenewingSuccess(t *testing.T) {
	f, client := optionalDenialClient(t, "client", http.StatusForbidden)
	original := time.Now().Add(-2 * pendingUpdatesCacheTTL)
	m := &Monitor{nodePendingUpdatesCache: map[string]pendingUpdatesCache{"lab-node": {count: 0, checkedAt: original}}}
	poll := func() models.Node {
		n := models.Node{}
		m.applyNodePendingUpdates(context.Background(), "lab", client, proxmox.Node{Node: "node"}, "lab-node", "online", &n)
		return n
	}
	poll()
	entry := m.nodePendingUpdatesCache["lab-node"]
	entry.deniedAt = time.Now().Add(-pendingUpdatesCacheTTL)
	m.nodePendingUpdatesCache["lab-node"] = entry
	n := poll()
	renewed := m.nodePendingUpdatesCache["lab-node"]
	if f.aptCalls.Load() != 2 || !renewed.deniedAt.After(entry.deniedAt) || !renewed.checkedAt.Equal(original) || n.PendingUpdatesStatus != "stale" || !n.PendingUpdatesCheckedAt.Equal(original) {
		t.Error("repeated denial changed successful origin or failed bounded reprobe")
	}
	poll()
	if f.aptCalls.Load() != 2 {
		t.Error("reprobe failure was not backed off again")
	}
}

func TestOptionalMemoryDenialExpiryRecoveryAndIsolation(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		t.Run(kind, func(t *testing.T) {
			f, client := optionalDenialClient(t, kind, http.StatusForbidden)
			key := guestMemoryCacheKey("lab", "node", 105)
			original := agentMemCacheEntry{info: memoryLifecycleSample(4096), fetchedAt: time.Now().Add(-2 * vmAgentMemCacheTTL)}
			m := &Monitor{guestMetadataCache: map[string]guestMetadataCacheEntry{guestMetadataCacheKey("lab", "node", 105): {osName: "Linux"}}, vmAgentMemCache: map[string]agentMemCacheEntry{key: original}}
			_, err := m.getVMAgentMemoryAvailability(context.Background(), client, "lab", "node", 105)
			denied := m.vmAgentMemCache[key]
			if status, ok := proxmox.APIErrorStatus(err); !ok || status != http.StatusForbidden || !denied.negative || !denied.permissionDenied || denied.info != (proxmox.LinuxMemoryAvailability{}) {
				t.Fatalf("denial became success or lost wire status: %+v / %v", denied, err)
			}
			f.status.Store(http.StatusOK)
			for _, target := range []struct {
				instance, node string
				vmid           int
			}{{"lab", "node", 106}, {"lab", "other", 105}, {"peer", "node", 105}} {
				info, err := m.getVMAgentMemoryAvailability(context.Background(), client, target.instance, target.node, target.vmid)
				if err != nil || info.EffectiveAvailable != 6144*1024 {
					t.Errorf("denial leaked scope: %+v / %v", info, err)
				}
			}
			// The long retry overrides known-Linux cadence, not the success origin.
			denied.fetchedAt = time.Now().Add(-vmAgentMemNegativeTTL)
			m.vmAgentMemCache[key] = denied
			info, err := m.getVMAgentMemoryAvailability(context.Background(), client, "lab", "node", 105)
			recovered := m.vmAgentMemCache[key]
			if err != nil || info.EffectiveAvailable != 6144*1024 || recovered.negative || recovered.permissionDenied || !recovered.fetchedAt.After(original.fetchedAt) || f.memoryCalls.Load() != 5 {
				t.Fatalf("expiry failed to recover: %+v / %v / %+v", info, err, recovered)
			}
			_, _ = m.getVMAgentMemoryAvailability(context.Background(), client, "lab", "node", 105)
			if f.memoryCalls.Load() != 5 || m.vmAgentMemCache[key] != recovered {
				t.Error("success cache cadence/origin changed")
			}
			m.clearGuestMemoryCache("lab", "node", 105)
			if _, ok := m.vmAgentMemCache[key]; ok {
				t.Error("lifecycle did not retire memory state")
			}
		})
	}
}

func TestOptionalMemoryDenialInvalidationKeepsOwnershipFence(t *testing.T) {
	_, wire := optionalDenialClient(t, "client", http.StatusForbidden)
	_, denial := wire.(guestAgentMemoryAvailabilityClient).GetVMMemoryAvailabilityFromAgent(context.Background(), "node", 105)
	for _, disposition := range []string{"guest", "connection", "unavailable", "stopped"} {
		t.Run(disposition, func(t *testing.T) {
			m := &Monitor{}
			_, _, done, release := startBlockedGuestMemory(t, m, "lab", proxmox.LinuxMemoryAvailability{}, denial)
			defer release()
			invalidateMemoryLifecycle(m, "lab", disposition)
			next := &guestMemoryAgentTestClient{stubPVEClient: &stubPVEClient{}}
			if _, err := m.getVMAgentMemoryAvailability(context.Background(), next, "lab", "node", 105); err != proxmox.ErrGuestAgentDeferred || next.memCalls != 0 {
				t.Error("retirement released unfinished read")
			}
			release()
			got := readGuestMemoryReply(t, done)
			if got.err != proxmox.ErrGuestAgentDeferred || len(m.vmAgentMemCache) != 0 || !reflect.DeepEqual(got.info, proxmox.LinuxMemoryAvailability{}) {
				t.Error("late obsolete denial repopulated replacement cache")
			}
		})
	}
}
