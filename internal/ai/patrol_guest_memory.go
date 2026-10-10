package ai

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func patrolGuestRuntimeStatus(runtime string, status unifiedresources.ResourceStatus) string {
	if runtime != "" {
		return runtime
	}
	// Canonical online is compatible with a running guest; warning/offline
	// collection state is not evidence of a different native power state.
	if status == unifiedresources.StatusOnline {
		return "running"
	}
	return string(status)
}

func patrolGuestMemoryEvidenceSection(snap patrolRuntimeState, scopedSet map[string]bool, cfg PatrolConfig) string {
	if !cfg.AnalyzeGuests {
		return ""
	}
	if gaps := patrolGuestMemoryGaps(patrolGuestInventoryRows(snap, scopedSet, nil)); gaps != "" {
		return "# Guest memory evidence\n" + gaps + "\n\n"
	}
	return ""
}

// A hypervisor's cache-inclusive footprint is not guest memory pressure. Keep
// it visible as evidence, but do not use it (or a retained/undated reading) to
// manufacture a pressure flag, forecast, anomaly or verified recovery.
type patrolGuestMemoryReading struct {
	percent         float64
	available       bool
	pressureKnown   bool
	state           string
	source          string
	observedAt      time.Time
	mayIncludeCache bool
}

func readPatrolGuestMemory(percent float64, observation models.MemoryObservation, available, proxmoxGuest bool) patrolGuestMemoryReading {
	e := unifiedresources.QualifyGuestMemory(percent, observation, available, proxmoxGuest, time.Now())
	return patrolGuestMemoryReading{percent: percent, available: e.Available, pressureKnown: e.PressureKnown,
		state: e.State, source: e.Source, observedAt: e.ObservedAt, mayIncludeCache: e.MayIncludeCache}
}

func (r patrolGuestMemoryReading) display() string {
	return (unifiedresources.GuestMemoryEvidence{Available: r.available, PressureKnown: r.pressureKnown,
		State: r.state, Source: r.source, ObservedAt: r.observedAt, MayIncludeCache: r.mayIncludeCache}).Format(r.percent)
}

func patrolGuestMemoryGaps(rows []patrolGuestInventoryRow) string {
	count := 0
	var examples []string
	for _, row := range rows {
		if row.memory.pressureKnown || row.status != "running" {
			continue
		}
		count++
		if len(examples) < 5 {
			examples = append(examples, row.name+": "+row.memory.display())
		}
	}
	if count == 0 {
		return ""
	}
	return fmt.Sprintf("Guest memory pressure unknown for %d running guests (not evidence of health or recovery): %s", count, strings.Join(examples, "; "))
}
