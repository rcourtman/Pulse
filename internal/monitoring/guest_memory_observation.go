package monitoring

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type guestMemoryObservationContext struct {
	source             string
	providerObservedAt time.Time
	deferred           bool
	previous           *GuestMemorySnapshot
	linkedAgent        models.Host
}

// guestMemoryObservation annotates the value already selected by the builder.
// It does not select different numeric evidence, change alert/history policy,
// renew a cache, or run a guest command. In particular disk state is not memory
// provenance: PVE or a linked Pulse agent may still supply an independent read.
func (m *Monitor) guestMemoryObservation(instance, guestType, node string, vmid int, memory models.Memory, evidence guestMemoryObservationContext, now time.Time) models.MemoryObservation {
	source := CanonicalMemorySource(evidence.source)
	if !memory.HasKnownUsage() || source == "unavailable" || source == "powered-off" {
		if source != "powered-off" {
			source = "unavailable"
		}
		return models.MemoryObservation{State: "unavailable", Source: source}
	}
	observation := models.MemoryObservation{State: "current", Source: source}
	switch source {
	case "guest-agent-meminfo", "guest-agent-meminfo-derived", "previous-snapshot":
		// The cache, rather than the repeatedly refreshed diagnostic/poll
		// timestamp, owns the age of retained QGA evidence.
		previous := evidence.previous
		sameGuest := previous != nil && previous.Instance == instance && previous.GuestType == guestType && previous.Node == node && previous.VMID == vmid
		qgaOrigin := source != "previous-snapshot" || (sameGuest &&
			(previous.Memory.Observation.Source == "guest-agent-meminfo" || previous.Memory.Observation.Source == "guest-agent-meminfo-derived" ||
				(previous.Memory.Observation.State == "" && (CanonicalMemorySource(previous.MemorySource) == "guest-agent-meminfo" || CanonicalMemorySource(previous.MemorySource) == "guest-agent-meminfo-derived"))))
		if guestType == "qemu" && qgaOrigin {
			if original, ok := m.cachedGuestMemoryObservation(instance, node, vmid, memory, now); ok {
				observation = original
			}
		}
		if source == "previous-snapshot" {
			observation.State = "last-known"
			if observation.ObservedAt.IsZero() && sameGuest && previous.Memory.Total == memory.Total && previous.Memory.Used == memory.Used {
				// An annotated prior source carries its original observation.
				// A legacy previous-snapshot's RetrievedAt cannot supply it.
				if previous.Memory.Observation.State == "current" || previous.Memory.Observation.State == "last-known" {
					observation = previous.Memory.Observation
					observation.State = "last-known"
				}
			}
		} else if evidence.deferred || (!observation.ObservedAt.IsZero() && now.Sub(observation.ObservedAt) >= vmAgentMemCacheTTL) {
			observation.State = "last-known"
		}
	case "agent":
		observation.ObservedAt = evidence.linkedAgent.LastSeen
		if original := evidence.linkedAgent.Memory.Observation; original.State != "" {
			observation = original
		}
	case "available-field", "derived-free-buffers-cached", "derived-total-minus-used", "status-mem", "status-freemem", "cluster-resources":
		observation.ObservedAt = evidence.providerObservedAt
	default:
		// Unknown provenance cannot be upgraded to a current observation by
		// a new poll. Keep the selected numbers and expose the uncertainty.
		observation.State = "last-known"
	}
	if observation.ObservedAt.IsZero() || observation.ObservedAt.After(now) {
		observation.ObservedAt = time.Time{}
		observation.State = "last-known"
	}
	return observation
}

func (m *Monitor) cachedGuestMemoryObservation(instance, node string, vmid int, memory models.Memory, now time.Time) (models.MemoryObservation, bool) {
	m.rrdCacheMu.RLock()
	entry, ok := m.vmAgentMemCache[guestMemoryCacheKey(instance, node, vmid)]
	m.rrdCacheMu.RUnlock()
	if !ok || entry.negative || entry.fetchedAt.IsZero() || entry.fetchedAt.After(now) || now.Sub(entry.fetchedAt) > vmAgentMemCleanupMaxAge || memory.Total <= 0 || entry.info.EffectiveAvailable > uint64(memory.Total) || memory.Used != memory.Total-int64(entry.info.EffectiveAvailable) {
		return models.MemoryObservation{}, false
	}
	expected := models.Memory{Free: int64(entry.info.EffectiveAvailable)}
	splitReclaimableMemory(&expected, entry.info.Free)
	if memory.Free != expected.Free || memory.Cache != expected.Cache {
		return models.MemoryObservation{}, false
	}
	source := "guest-agent-meminfo"
	switch entry.info.Source {
	case "meminfo-available":
	case "meminfo-derived":
		source = "guest-agent-meminfo-derived"
	default:
		return models.MemoryObservation{}, false
	}
	return models.MemoryObservation{State: "current", Source: source, ObservedAt: entry.fetchedAt}, true
}
