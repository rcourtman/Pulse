package ai

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestPatrolGuestMemoryReadingBoundaries(t *testing.T) {
	at := time.Now().UTC().Add(-time.Minute)
	obs := models.MemoryObservation{State: "current", Source: "available-field", ObservedAt: at}
	for _, percent := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1, 101} {
		reading := readPatrolGuestMemory(percent, obs, true, unifiedresources.GuestMemoryProxmoxVM)
		if reading.pressureKnown || reading.available || !strings.HasPrefix(reading.display(), "N/A") {
			t.Errorf("invalid percent became live pressure: %+v", reading)
		}
	}
	for _, source := range []string{"available-field", "derived-free-buffers-cached", "guest-agent-meminfo", "guest-agent-meminfo-derived", "agent"} {
		obs.Source = source
		if r := readPatrolGuestMemory(96, obs, true, unifiedresources.GuestMemoryProxmoxVM); !r.pressureKnown {
			t.Errorf("current cache-aware positive rejected: %s", source)
		}
	}
	for _, state := range []string{"", "future", "last-known", "unavailable"} {
		obs.State = state
		if r := readPatrolGuestMemory(96, obs, true, unifiedresources.GuestMemoryProxmoxVM); r.pressureKnown {
			t.Errorf("non-current state became current: %s", state)
		}
	}
	// Proxmox's listing mem means different things per guest type: a QEMU
	// guest's counts its own page cache, an LXC's has page cache subtracted.
	obs = models.MemoryObservation{State: "current", Source: "cluster-resources", ObservedAt: at}
	if r := readPatrolGuestMemory(97, obs, true, unifiedresources.GuestMemoryProxmoxVM); r.pressureKnown || !r.mayIncludeCache {
		t.Errorf("QEMU cluster-resources memory became pressure: %+v", r)
	}
	ct := readPatrolGuestMemory(97, obs, true, unifiedresources.GuestMemoryProxmoxContainer)
	if !ct.pressureKnown || ct.mayIncludeCache || !strings.Contains(ct.display(), "cache-aware guest usage") {
		t.Errorf("LXC cluster-resources memory lost its pressure meaning: %+v %s", ct, ct.display())
	}
	for _, source := range []string{"status-mem", "status-freemem", "derived-total-minus-used"} {
		obs.Source = source
		if r := readPatrolGuestMemory(97, obs, true, unifiedresources.GuestMemoryProxmoxContainer); r.pressureKnown {
			t.Errorf("container kind widened another cache-inclusive source: %s", source)
		}
	}
	obs.Source = "cluster-resources"
	obs.State = "last-known"
	if r := readPatrolGuestMemory(97, obs, true, unifiedresources.GuestMemoryProxmoxContainer); r.pressureKnown {
		t.Error("retained LXC listing reading became current pressure")
	}
	obs = models.MemoryObservation{State: "current", Source: "available-field", ObservedAt: time.Unix(0, 0)}
	if r := readPatrolGuestMemory(96, obs, true, unifiedresources.GuestMemoryProxmoxVM); r.pressureKnown || !r.observedAt.IsZero() {
		t.Fatal("invalid observation origin became current")
	}
	if status := patrolGuestRuntimeStatus("paused", unifiedresources.StatusOffline); status != "paused" {
		t.Fatal("native pause became canonical collection status")
	}
	if status := patrolGuestRuntimeStatus("running", unifiedresources.StatusWarning); status != "running" {
		t.Fatal("collection warning changed native runtime state")
	}
}

func TestPatrolGuestMemorySeedScopeAndOptOut(t *testing.T) {
	at := time.Now().UTC().Add(-time.Minute)
	snapshot := models.StateSnapshot{VMs: []models.VM{
		{ID: "included", Name: "included", VMID: 100, Type: "qemu", Status: "running", Memory: patrolMemoryFixture(96, "current", "status-mem", at)},
		{ID: "excluded", Name: "excluded", VMID: 101, Type: "qemu", Status: "running", Memory: patrolMemoryFixture(96, "current", "status-mem", at)},
	}}
	snap := newPatrolRuntimeState(snapshot)
	if section := patrolGuestMemoryEvidenceSection(snap, nil, PatrolConfig{AnalyzeGuests: false}); section != "" {
		t.Fatal("guest analysis opt-out acquired memory evidence")
	}
	section := patrolGuestMemoryEvidenceSection(snap, map[string]bool{"included": true}, PatrolConfig{AnalyzeGuests: true})
	if !strings.Contains(section, "included") || strings.Contains(section, "excluded") {
		t.Fatalf("memory evidence escaped its existing scope: %s", section)
	}
}
