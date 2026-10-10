package unifiedresources

import (
	"fmt"
	"math"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// GuestMemoryEvidence qualifies the selected numeric reading. It is not
// permission to obtain another reading or evidence that a guest has recovered.
// Available distinguishes a measured zero from an absent/invalid percentage.
type GuestMemoryEvidence struct {
	Available       bool      `json:"available"`
	PressureKnown   bool      `json:"pressure_known"`
	State           string    `json:"state,omitempty"`
	Source          string    `json:"source,omitempty"`
	ObservedAt      time.Time `json:"observed_at,omitzero"`
	MayIncludeCache bool      `json:"may_include_reclaimable_cache,omitempty"`
}

// QualifyGuestMemory shares the existing Patrol interpretation with its query
// and observer inputs. State is authored by collection; consumers do not renew
// the original observation or invent a separate collection lease.
func QualifyGuestMemory(percent float64, observation models.MemoryObservation, available, proxmoxGuest bool, now time.Time) GuestMemoryEvidence {
	e := GuestMemoryEvidence{Available: available && !math.IsNaN(percent) && !math.IsInf(percent, 0) && percent >= 0 && percent <= 100}
	if !e.Available || observation.State == "unavailable" {
		e.Available = false
		e.State = "unavailable"
		return e
	}
	// Other platforms need not implement Proxmox's memory observation contract.
	if !proxmoxGuest && observation == (models.MemoryObservation{}) {
		e.PressureKnown = true
		return e
	}
	e.State = "unknown"
	if observation.ObservedAt.After(time.Unix(0, 0)) && !observation.ObservedAt.After(now) {
		e.ObservedAt = observation.ObservedAt
	}
	if observation.State == "last-known" {
		e.State = "last-known"
	} else if observation.State == "current" && !e.ObservedAt.IsZero() {
		e.State = "current"
	}
	// Source names are closed facts, not arbitrary upstream text in AI context.
	switch observation.Source {
	case "available-field", "derived-free-buffers-cached", "guest-agent-meminfo", "guest-agent-meminfo-derived", "agent":
		e.Source = observation.Source
		e.PressureKnown = e.State == "current"
	case "status-mem", "status-freemem", "cluster-resources", "derived-total-minus-used":
		e.Source = observation.Source
		e.MayIncludeCache = true
	case "previous-snapshot":
		e.Source = observation.Source
	}
	return e
}

// Format keeps qualification when a tool result is condensed into model facts.
func (e GuestMemoryEvidence) Format(percent float64) string {
	if !e.Available {
		return "N/A (guest memory unavailable; not evidence of recovery)"
	}
	if e.State == "" {
		return fmt.Sprintf("%.0f%%", percent)
	}
	source := e.Source
	if source == "" {
		source = "unknown"
	}
	observed := "time unknown"
	if !e.ObservedAt.IsZero() {
		observed = "observed " + e.ObservedAt.UTC().Format(time.RFC3339)
	}
	qualification := "cache-aware guest usage"
	if !e.PressureKnown {
		qualification = "guest pressure unknown"
		if e.MayIncludeCache {
			qualification += "; may include reclaimable cache"
		}
	}
	return fmt.Sprintf("%.0f%% (%s; source %s; %s; %s)", percent, e.State, source, observed, qualification)
}

// GuestMemoryEvidenceForResource reads only the selected metric. A conflicting
// raw platform facet or resource LastSeen must never lend it provenance.
func GuestMemoryEvidenceForResource(r *Resource, now time.Time) GuestMemoryEvidence {
	if r == nil || r.Metrics == nil || r.Metrics.Memory == nil {
		return QualifyGuestMemory(0, models.MemoryObservation{}, false, true, now)
	}
	_, hasProxmox := r.SourceStatus[SourceProxmox]
	m := r.Metrics.Memory
	return QualifyGuestMemory(m.Percent, m.Observation, true, r.Proxmox != nil || hasProxmox || m.Source == SourceProxmox, now)
}
