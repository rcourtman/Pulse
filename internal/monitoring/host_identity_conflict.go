package monitoring

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// hostIdentityConflictWindow bounds how far apart two reports can be and
// still count as evidence that distinct machines share one identity. It also
// controls how long a detected conflict stays visible after the flapping
// stops (e.g. one of the clones was fixed or shut down). Sized to cover a few
// report cycles even for agents on multi-minute intervals.
const hostIdentityConflictWindow = 15 * time.Minute

// hostIdentityFlapTracker watches the stream of reports folded into a single
// host agent identity and detects when two distinct machines are behind it.
//
// The signal is a *revisit*: the reported hostname (or report IP) switches
// away from the previous value and back to one already seen inside the
// window. A genuine hostname rename transitions exactly once and never
// revisits the old value, so it does not trip the detector; template-cloned
// machines that share /etc/machine-id alternate every report cycle and trip
// it immediately. The report IP is tracked alongside the hostname because
// MSP-style template deployments often reuse hostnames too (pve01 at two
// sites), leaving the address as the only field that betrays the clone.
type hostIdentityFlapTracker struct {
	hostnames map[string]time.Time
	reportIPs map[string]time.Time

	lastHostname string
	lastReportIP string

	conflictSince    time.Time
	conflictLastSeen time.Time
}

func newHostIdentityFlapTracker() *hostIdentityFlapTracker {
	return &hostIdentityFlapTracker{
		hostnames: make(map[string]time.Time),
		reportIPs: make(map[string]time.Time),
	}
}

// observe records one report's identity fields and returns the active
// conflict, or nil when the identity looks healthy.
func (t *hostIdentityFlapTracker) observe(hostname, reportIP string, now time.Time) *models.HostIdentityConflict {
	hostname = strings.TrimSpace(hostname)
	reportIP = strings.TrimSpace(reportIP)

	pruneOlderThan(t.hostnames, now.Add(-hostIdentityConflictWindow))
	pruneOlderThan(t.reportIPs, now.Add(-hostIdentityConflictWindow))

	revisit := false
	if hostname != "" {
		if _, seen := t.hostnames[hostname]; seen && t.lastHostname != "" && t.lastHostname != hostname {
			revisit = true
		}
		t.hostnames[hostname] = now
		t.lastHostname = hostname
	}
	if reportIP != "" {
		if _, seen := t.reportIPs[reportIP]; seen && t.lastReportIP != "" && t.lastReportIP != reportIP {
			revisit = true
		}
		t.reportIPs[reportIP] = now
		t.lastReportIP = reportIP
	}

	if revisit {
		if t.conflictSince.IsZero() || now.Sub(t.conflictLastSeen) > hostIdentityConflictWindow {
			t.conflictSince = now
		}
		t.conflictLastSeen = now
	}

	if t.conflictLastSeen.IsZero() || now.Sub(t.conflictLastSeen) > hostIdentityConflictWindow {
		t.conflictSince = time.Time{}
		t.conflictLastSeen = time.Time{}
		return nil
	}

	conflict := &models.HostIdentityConflict{
		Hostnames: sortedKeys(t.hostnames),
		FirstSeen: t.conflictSince,
		LastSeen:  t.conflictLastSeen,
	}
	// Report IPs are only interesting when they themselves diverge; when the
	// clones alternate hostnames the hostname list already carries the story.
	if len(t.reportIPs) > 1 {
		conflict.ReportIPs = sortedKeys(t.reportIPs)
	}
	return conflict
}

// trackHostAgentIdentity feeds one report's identity fields into the flap
// tracker for the resolved host identifier and returns the active conflict,
// if any. Callers must not hold m.mu.
func (m *Monitor) trackHostAgentIdentity(identifier, hostname, reportIP string, now time.Time) *models.HostIdentityConflict {
	if strings.TrimSpace(identifier) == "" {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.hostIdentityFlaps == nil {
		m.hostIdentityFlaps = make(map[string]*hostIdentityFlapTracker)
	}
	tracker, ok := m.hostIdentityFlaps[identifier]
	if !ok {
		tracker = newHostIdentityFlapTracker()
		m.hostIdentityFlaps[identifier] = tracker
	}
	return tracker.observe(hostname, reportIP, now)
}

// clearHostAgentIdentityTrackingLocked drops flap state for a host identity,
// e.g. when the host is deliberately removed. Callers must hold m.mu.
func (m *Monitor) clearHostAgentIdentityTrackingLocked(hostID string) {
	delete(m.hostIdentityFlaps, hostID)
}
