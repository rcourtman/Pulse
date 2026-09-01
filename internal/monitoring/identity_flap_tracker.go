package monitoring

import (
	"sort"
	"strings"
	"time"
)

// identityFlapTracker is the shared revisit-detection core behind the
// Docker-host and host-agent identity conflict trackers. It watches the
// stream of (hostname, secondary) identity pairs folded into a single
// resource identity and detects when two distinct machines are behind it.
//
// The signal is a *revisit*: a value switches away from the previous one and
// back to one already seen inside the window. A genuine rename transitions
// exactly once and never revisits the old value, so it does not trip the
// detector; clones that share an identity alternate every report cycle and
// trip it immediately. The window also controls how long a detected conflict
// stays visible after the flapping stops.
//
// What the secondary field means (machine ID, report IP) is the adapter's
// business; this core only tracks values.
type identityFlapTracker struct {
	window time.Duration

	hostnames   map[string]time.Time
	secondaries map[string]time.Time

	lastHostname  string
	lastSecondary string

	conflictSince    time.Time
	conflictLastSeen time.Time
}

// identityConflict is the tracker's domain-neutral verdict; adapters
// translate it into their platform's conflict model.
type identityConflict struct {
	hostnames []string
	// secondaries is nil unless the secondary values themselves diverge; when
	// clones share one machine ID / report IP the hostname list already
	// carries the story.
	secondaries []string
	firstSeen   time.Time
	lastSeen    time.Time
}

func newIdentityFlapTracker(window time.Duration) *identityFlapTracker {
	return &identityFlapTracker{
		window:      window,
		hostnames:   make(map[string]time.Time),
		secondaries: make(map[string]time.Time),
	}
}

// observe records one report's identity fields and returns the active
// conflict, or nil when the identity looks healthy.
func (t *identityFlapTracker) observe(hostname, secondary string, now time.Time) *identityConflict {
	hostname = strings.TrimSpace(hostname)
	secondary = strings.TrimSpace(secondary)

	pruneOlderThan(t.hostnames, now.Add(-t.window))
	pruneOlderThan(t.secondaries, now.Add(-t.window))

	revisit := false
	if hostname != "" {
		if _, seen := t.hostnames[hostname]; seen && t.lastHostname != "" && t.lastHostname != hostname {
			revisit = true
		}
		t.hostnames[hostname] = now
		t.lastHostname = hostname
	}
	if secondary != "" {
		if _, seen := t.secondaries[secondary]; seen && t.lastSecondary != "" && t.lastSecondary != secondary {
			revisit = true
		}
		t.secondaries[secondary] = now
		t.lastSecondary = secondary
	}

	if revisit {
		if t.conflictSince.IsZero() || now.Sub(t.conflictLastSeen) > t.window {
			t.conflictSince = now
		}
		t.conflictLastSeen = now
	}

	if t.conflictLastSeen.IsZero() || now.Sub(t.conflictLastSeen) > t.window {
		t.conflictSince = time.Time{}
		t.conflictLastSeen = time.Time{}
		return nil
	}

	conflict := &identityConflict{
		hostnames: sortedKeys(t.hostnames),
		firstSeen: t.conflictSince,
		lastSeen:  t.conflictLastSeen,
	}
	if len(t.secondaries) > 1 {
		conflict.secondaries = sortedKeys(t.secondaries)
	}
	return conflict
}

// secondaryRevisit reports whether observing the given secondary value now
// would be a revisit: the value was already seen inside the window and the
// stream has since moved to a different value. This is the read-only
// counterpart of observe for callers that must decide identity adoption
// before the report is recorded.
func (t *identityFlapTracker) secondaryRevisit(secondary string, now time.Time) bool {
	secondary = strings.TrimSpace(secondary)
	if secondary == "" || t.lastSecondary == "" || t.lastSecondary == secondary {
		return false
	}
	seenAt, seen := t.secondaries[secondary]
	return seen && !seenAt.Before(now.Add(-t.window))
}

// dockerMachineIDRevisit reports whether folding a report with this machine ID
// into the given Docker host identity would alternate back to a machine the
// identity has already been seen on: proof that two live machines are behind
// one record (the Docker analog of #1753), as opposed to one recreated
// container whose machine ID changed exactly once and never returns. Callers
// must not hold m.mu.
func (m *Monitor) dockerMachineIDRevisit(identifier, machineID string, now time.Time) bool {
	if strings.TrimSpace(identifier) == "" {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	tracker, ok := m.dockerIdentityFlaps[identifier]
	if !ok {
		return false
	}
	return tracker.secondaryRevisit(machineID, now)
}

// observeIdentityFlap feeds one report's identity fields into the flap
// tracker for the resolved identifier, lazily creating the tracker map and
// entry, and returns the active conflict, if any. Callers must not hold m.mu.
func (m *Monitor) observeIdentityFlap(trackers *map[string]*identityFlapTracker, window time.Duration, identifier, hostname, secondary string, now time.Time) *identityConflict {
	if strings.TrimSpace(identifier) == "" {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if *trackers == nil {
		*trackers = make(map[string]*identityFlapTracker)
	}
	tracker, ok := (*trackers)[identifier]
	if !ok {
		tracker = newIdentityFlapTracker(window)
		(*trackers)[identifier] = tracker
	}
	return tracker.observe(hostname, secondary, now)
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
