package monitoring

import (
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// dockerIdentityConflictWindow bounds how far apart two reports can be and
// still count as evidence that distinct machines share one identity. It also
// controls how long a detected conflict stays visible after the flapping
// stops (e.g. one of the clones was fixed or shut down). Sized to cover a few
// report cycles even for agents on multi-minute intervals.
const dockerIdentityConflictWindow = 15 * time.Minute

// dockerIdentityFlapTracker watches the stream of reports folded into a single
// Docker host identity and detects when two distinct machines are behind it.
//
// The signal is a *revisit*: the reported hostname (or machine ID) switches
// away from the previous value and back to one already seen inside the
// window. A genuine hostname rename transitions exactly once and never
// revisits the old value, so it does not trip the detector; cloned VMs that
// share /etc/machine-id alternate every report cycle and trip it immediately.
type dockerIdentityFlapTracker struct {
	hostnames  map[string]time.Time
	machineIDs map[string]time.Time

	lastHostname  string
	lastMachineID string

	conflictSince    time.Time
	conflictLastSeen time.Time
}

func newDockerIdentityFlapTracker() *dockerIdentityFlapTracker {
	return &dockerIdentityFlapTracker{
		hostnames:  make(map[string]time.Time),
		machineIDs: make(map[string]time.Time),
	}
}

// observe records one report's identity fields and returns the active
// conflict, or nil when the identity looks healthy.
func (t *dockerIdentityFlapTracker) observe(hostname, machineID string, now time.Time) *models.DockerHostIdentityConflict {
	hostname = strings.TrimSpace(hostname)
	machineID = strings.TrimSpace(machineID)

	pruneOlderThan(t.hostnames, now.Add(-dockerIdentityConflictWindow))
	pruneOlderThan(t.machineIDs, now.Add(-dockerIdentityConflictWindow))

	revisit := false
	if hostname != "" {
		if _, seen := t.hostnames[hostname]; seen && t.lastHostname != "" && t.lastHostname != hostname {
			revisit = true
		}
		t.hostnames[hostname] = now
		t.lastHostname = hostname
	}
	if machineID != "" {
		if _, seen := t.machineIDs[machineID]; seen && t.lastMachineID != "" && t.lastMachineID != machineID {
			revisit = true
		}
		t.machineIDs[machineID] = now
		t.lastMachineID = machineID
	}

	if revisit {
		if t.conflictSince.IsZero() || now.Sub(t.conflictLastSeen) > dockerIdentityConflictWindow {
			t.conflictSince = now
		}
		t.conflictLastSeen = now
	}

	if t.conflictLastSeen.IsZero() || now.Sub(t.conflictLastSeen) > dockerIdentityConflictWindow {
		t.conflictSince = time.Time{}
		t.conflictLastSeen = time.Time{}
		return nil
	}

	conflict := &models.DockerHostIdentityConflict{
		Hostnames: sortedKeys(t.hostnames),
		FirstSeen: t.conflictSince,
		LastSeen:  t.conflictLastSeen,
	}
	// Machine IDs are only interesting when they themselves diverge; in the
	// common cloned-VM case there is exactly one shared machine ID and the
	// hostname list carries the story.
	if len(t.machineIDs) > 1 {
		conflict.MachineIDs = sortedKeys(t.machineIDs)
	}
	return conflict
}

func pruneOlderThan(entries map[string]time.Time, cutoff time.Time) {
	for key, seenAt := range entries {
		if seenAt.Before(cutoff) {
			delete(entries, key)
		}
	}
}

func sortedKeys(entries map[string]time.Time) []string {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// trackDockerHostIdentity feeds one report's identity fields into the flap
// tracker for the resolved host identifier and returns the active conflict,
// if any. Callers must not hold m.mu.
func (m *Monitor) trackDockerHostIdentity(identifier, hostname, machineID string, now time.Time) *models.DockerHostIdentityConflict {
	if strings.TrimSpace(identifier) == "" {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dockerIdentityFlaps == nil {
		m.dockerIdentityFlaps = make(map[string]*dockerIdentityFlapTracker)
	}
	tracker, ok := m.dockerIdentityFlaps[identifier]
	if !ok {
		tracker = newDockerIdentityFlapTracker()
		m.dockerIdentityFlaps[identifier] = tracker
	}
	return tracker.observe(hostname, machineID, now)
}

// clearDockerHostIdentityTracking drops flap state for a host identity, e.g.
// when the host is deliberately removed. Callers must hold m.mu.
func (m *Monitor) clearDockerHostIdentityTrackingLocked(hostID string) {
	delete(m.dockerIdentityFlaps, hostID)
}
