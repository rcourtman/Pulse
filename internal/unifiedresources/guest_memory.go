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

// GuestMemoryKind names the guest whose accounting a Proxmox memory source
// describes. The same source name means different things for QEMU and LXC.
type GuestMemoryKind uint8

const (
	// GuestMemoryOtherPlatform keeps a non-Proxmox platform's selected reading.
	GuestMemoryOtherPlatform GuestMemoryKind = iota
	// GuestMemoryProxmoxVM is a QEMU guest, or a Proxmox guest of unknown type.
	GuestMemoryProxmoxVM
	// GuestMemoryProxmoxContainer is an LXC (including OCI) container.
	GuestMemoryProxmoxContainer
)

// QualifyGuestMemory qualifies a reading without knowing the Proxmox guest
// type, so it uses the conservative QEMU interpretation. Callers that know the
// guest is a container use QualifyGuestMemoryForKind.
func QualifyGuestMemory(percent float64, observation models.MemoryObservation, available, proxmoxGuest bool, now time.Time) GuestMemoryEvidence {
	kind := GuestMemoryOtherPlatform
	if proxmoxGuest {
		kind = GuestMemoryProxmoxVM
	}
	return QualifyGuestMemoryForKind(percent, observation, available, kind, now)
}

// QualifyGuestMemoryForKind shares the existing Patrol interpretation with its
// query and observer inputs. State is authored by collection; consumers do not
// renew the original observation or invent a separate collection lease.
func QualifyGuestMemoryForKind(percent float64, observation models.MemoryObservation, available bool, kind GuestMemoryKind, now time.Time) GuestMemoryEvidence {
	e := GuestMemoryEvidence{Available: available && !math.IsNaN(percent) && !math.IsInf(percent, 0) && percent >= 0 && percent <= 100}
	if !e.Available || observation.State == "unavailable" {
		e.Available = false
		e.State = "unavailable"
		return e
	}
	// Other platforms need not implement Proxmox's memory observation contract.
	if kind == GuestMemoryOtherPlatform && observation == (models.MemoryObservation{}) {
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
	case "cluster-resources":
		e.Source = observation.Source
		// An LXC's listing mem is its cgroup usage minus page cache: Proxmox
		// computes memory.current - memory.stat file (cgroup v2) or
		// usage_in_bytes - total_cache (v1) in PVE::CGroup::get_memory_stat,
		// and /cluster/resources reports that vmstatus value. A QEMU listing is
		// the host's view of guest RAM, which counts the guest's own cache.
		if kind == GuestMemoryProxmoxContainer {
			e.PressureKnown = e.State == "current"
		} else {
			e.MayIncludeCache = true
		}
	case "status-mem", "status-freemem", "derived-total-minus-used":
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
		return QualifyGuestMemoryForKind(0, models.MemoryObservation{}, false, GuestMemoryProxmoxVM, now)
	}
	_, hasProxmox := r.SourceStatus[SourceProxmox]
	m := r.Metrics.Memory
	kind := GuestMemoryKindFor(r.Proxmox != nil || hasProxmox || m.Source == SourceProxmox, r.Type == ResourceTypeSystemContainer)
	return QualifyGuestMemoryForKind(m.Percent, m.Observation, true, kind, now)
}

// GuestMemoryKindFor keeps non-Proxmox guests on their own contract and tells
// Proxmox containers apart from QEMU guests.
func GuestMemoryKindFor(proxmoxGuest, container bool) GuestMemoryKind {
	switch {
	case !proxmoxGuest:
		return GuestMemoryOtherPlatform
	case container:
		return GuestMemoryProxmoxContainer
	default:
		return GuestMemoryProxmoxVM
	}
}
