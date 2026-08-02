package monitoring

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// hostIdentityConflictWindow bounds how far apart two reports can be and
// still count as evidence that distinct machines share one identity. It also
// controls how long a detected conflict stays visible after the flapping
// stops (e.g. one of the clones was fixed or shut down). Sized to cover a few
// report cycles even for agents on multi-minute intervals.
const hostIdentityConflictWindow = 15 * time.Minute

// trackHostAgentIdentity feeds one report's identity fields into the flap
// tracker for the resolved host identifier and returns the active conflict,
// if any. The secondary identity field is the report IP: MSP-style template
// deployments often reuse hostnames too (pve01 at two sites), leaving the
// address as the only field that betrays the clone. Callers must not hold
// m.mu.
func (m *Monitor) trackHostAgentIdentity(identifier, hostname, reportIP string, now time.Time) *models.HostIdentityConflict {
	conflict := m.observeIdentityFlap(&m.hostIdentityFlaps, hostIdentityConflictWindow, identifier, hostname, reportIP, now)
	if conflict == nil {
		return nil
	}
	return &models.HostIdentityConflict{
		Hostnames: conflict.hostnames,
		ReportIPs: conflict.secondaries,
		FirstSeen: conflict.firstSeen,
		LastSeen:  conflict.lastSeen,
	}
}

// clearHostAgentIdentityTrackingLocked drops flap state for a host identity,
// e.g. when the host is deliberately removed. Callers must hold m.mu.
func (m *Monitor) clearHostAgentIdentityTrackingLocked(hostID string) {
	delete(m.hostIdentityFlaps, hostID)
}
