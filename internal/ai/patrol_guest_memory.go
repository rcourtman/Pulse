package ai

import (
	"fmt"
	"math"
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
	percent       float64
	available     bool
	pressureKnown bool
	state         string
	source        string
	observedAt    time.Time
}

func readPatrolGuestMemory(percent float64, observation models.MemoryObservation, available, proxmoxGuest bool) patrolGuestMemoryReading {
	r := patrolGuestMemoryReading{percent: percent, available: available && !math.IsNaN(percent) && !math.IsInf(percent, 0) && percent >= 0 && percent <= 100}
	if !r.available || observation.State == "unavailable" {
		r.available = false
		r.state = "unavailable"
		return r
	}
	// Other platforms need not implement Proxmox's memory evidence contract.
	// Preserve their existing readings, including a selected measured zero.
	if !proxmoxGuest && observation == (models.MemoryObservation{}) {
		r.pressureKnown = true
		return r
	}
	r.state = "unknown"
	if observation.ObservedAt.After(time.Unix(0, 0)) && !observation.ObservedAt.After(time.Now()) {
		r.observedAt = observation.ObservedAt
	}
	if observation.State == "last-known" {
		r.state = "last-known"
	} else if observation.State == "current" && !r.observedAt.IsZero() {
		r.state = "current"
	}
	// Only known source names are put in the seed. A future/unknown source is
	// not an assertion of either cache awareness or current guest pressure.
	switch observation.Source {
	case "available-field", "derived-free-buffers-cached", "guest-agent-meminfo", "guest-agent-meminfo-derived", "agent":
		r.source = observation.Source
		r.pressureKnown = r.state == "current"
	case "status-mem", "status-freemem", "cluster-resources", "derived-total-minus-used", "previous-snapshot":
		r.source = observation.Source
	}
	return r
}

func (r patrolGuestMemoryReading) display() string {
	if !r.available {
		return "N/A (guest memory unavailable; not evidence of recovery)"
	}
	if r.state == "" {
		return fmt.Sprintf("%.0f%%", r.percent)
	}
	source := r.source
	if source == "" {
		source = "unknown"
	}
	observed := "time unknown"
	if !r.observedAt.IsZero() {
		observed = "observed " + r.observedAt.UTC().Format(time.RFC3339)
	}
	qualification := "cache-aware guest usage"
	if !r.pressureKnown {
		qualification = "guest pressure unknown"
		if r.source == "status-mem" || r.source == "status-freemem" || r.source == "cluster-resources" || r.source == "derived-total-minus-used" {
			qualification += "; may include reclaimable cache"
		}
	}
	return fmt.Sprintf("%.0f%% (%s; source %s; %s; %s)", r.percent, r.state, source, observed, qualification)
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
