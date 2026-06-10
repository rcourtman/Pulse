package api

import (
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// reportAvailabilityChangeLimit bounds how many timeline rows a single
// report subject pulls. Production resources record a handful of state
// transitions per month; a resource that exceeds this is flapping hard
// enough that the truncated window still tells the honest story.
const reportAvailabilityChangeLimit = 20000

// availabilityState classifies a recorded resource state for uptime math.
type availabilityState int

const (
	availabilityStateUnobserved availabilityState = iota
	availabilityStateUp
	availabilityStateDown
)

// classifyAvailabilityState maps the canonical resource state vocabulary
// (online / warning / offline / unknown plus the synthetic "absent" emitted
// when a resource enters or leaves the registry) onto uptime semantics.
// Warning counts as up: the resource is reachable and serving, just
// unhealthy. Absent and unknown are unobserved - a monitoring gap is not an
// outage. Unrecognized future states default to up because a client-facing
// stability report claiming false downtime is worse than missing an exotic
// down state.
func classifyAvailabilityState(state string) availabilityState {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "", "absent", "unknown":
		return availabilityStateUnobserved
	case "offline":
		return availabilityStateDown
	default:
		return availabilityStateUp
	}
}

func availabilityChangeTime(change unifiedresources.ResourceChange) time.Time {
	if change.OccurredAt != nil && !change.OccurredAt.IsZero() {
		return *change.OccurredAt
	}
	return change.ObservedAt
}

// computeReportAvailability derives an availability summary for one report
// subject from its recorded state timeline. The walk reconstructs the state
// for every moment of [start, end]: the state before the first in-window
// transition is that transition's From; with no transitions at all the
// resource sat in currentState for the whole window (any change would have
// been journaled).
func computeReportAvailability(changes []unifiedresources.ResourceChange, currentState string, start, end time.Time) *reporting.AvailabilityInfo {
	if !end.After(start) {
		return nil
	}

	transitions := make([]unifiedresources.ResourceChange, 0, len(changes))
	for _, change := range changes {
		if change.Kind != unifiedresources.ChangeStateTransition {
			continue
		}
		at := availabilityChangeTime(change)
		if at.Before(start) || at.After(end) {
			continue
		}
		transitions = append(transitions, change)
	}
	sort.SliceStable(transitions, func(i, j int) bool {
		return availabilityChangeTime(transitions[i]).Before(availabilityChangeTime(transitions[j]))
	})

	initialState := currentState
	if len(transitions) > 0 {
		initialState = transitions[0].From
	}

	var up, down time.Duration
	var downIncidents int
	var longestOutage, currentOutage time.Duration

	accumulate := func(state availabilityState, d time.Duration) {
		if d <= 0 {
			return
		}
		switch state {
		case availabilityStateUp:
			up += d
		case availabilityStateDown:
			down += d
			currentOutage += d
		}
		if state != availabilityStateDown {
			if currentOutage > longestOutage {
				longestOutage = currentOutage
			}
			currentOutage = 0
		}
	}

	cursor := start
	state := classifyAvailabilityState(initialState)
	for _, change := range transitions {
		at := availabilityChangeTime(change)
		accumulate(state, at.Sub(cursor))
		next := classifyAvailabilityState(change.To)
		if next == availabilityStateDown && state != availabilityStateDown {
			downIncidents++
		}
		state = next
		cursor = at
	}
	accumulate(state, end.Sub(cursor))
	if currentOutage > longestOutage {
		longestOutage = currentOutage
	}

	window := end.Sub(start)
	observed := up + down
	info := &reporting.AvailabilityInfo{
		ObservedPercent: 100 * float64(observed) / float64(window),
		TotalDowntime:   down,
		LongestOutage:   longestOutage,
		DownIncidents:   downIncidents,
	}
	if observed > 0 {
		info.UptimePercent = 100 * float64(up) / float64(observed)
	}
	return info
}

// resolveReportAvailability attaches the availability summary for the report
// subject. The change timeline is keyed by the canonical unified resource ID
// (not the metrics-target ID), so this runs against req.ResourceID.
func (h *ReportingHandlers) resolveReportAvailability(orgID string, req *reporting.MetricReportRequest, snapshot reportingEnrichmentSnapshot, start, end time.Time) {
	if h == nil || req == nil || h.mtMonitor == nil {
		return
	}

	currentState := ""
	for i := range snapshot.Resources {
		if snapshot.Resources[i].ID == req.ResourceID {
			currentState = string(snapshot.Resources[i].Status)
			break
		}
	}
	if currentState == "" {
		// Unknown to the unified registry: no timeline to report against.
		return
	}

	monitor, err := h.mtMonitor.GetMonitor(orgID)
	if err != nil || monitor == nil {
		return
	}
	changes := monitor.RecentResourceChanges(req.ResourceID, start, reportAvailabilityChangeLimit)
	req.Availability = computeReportAvailability(changes, currentState, start, end)
}
