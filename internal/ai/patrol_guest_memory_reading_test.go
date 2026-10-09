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
		reading := readPatrolGuestMemory(percent, obs, true, true)
		if reading.pressureKnown || reading.available || !strings.HasPrefix(reading.display(), "N/A") {
			t.Errorf("invalid percent became live pressure: %+v", reading)
		}
	}
	for _, source := range []string{"available-field", "derived-free-buffers-cached", "guest-agent-meminfo", "guest-agent-meminfo-derived", "agent"} {
		obs.Source = source
		if r := readPatrolGuestMemory(96, obs, true, true); !r.pressureKnown {
			t.Errorf("current cache-aware positive rejected: %s", source)
		}
	}
	for _, state := range []string{"", "future", "last-known", "unavailable"} {
		obs.State = state
		if r := readPatrolGuestMemory(96, obs, true, true); r.pressureKnown {
			t.Errorf("non-current state became current: %s", state)
		}
	}
	obs.State = "current"
	obs.ObservedAt = time.Unix(0, 0)
	if r := readPatrolGuestMemory(96, obs, true, true); r.pressureKnown || !r.observedAt.IsZero() {
		t.Fatal("invalid observation origin became current")
	}
	if status := patrolGuestRuntimeStatus("paused", unifiedresources.StatusOffline); status != "paused" {
		t.Fatal("native pause became canonical collection status")
	}
	if status := patrolGuestRuntimeStatus("running", unifiedresources.StatusWarning); status != "running" {
		t.Fatal("collection warning changed native runtime state")
	}
}
