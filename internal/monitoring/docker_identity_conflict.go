package monitoring

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// dockerIdentityConflictWindow bounds how far apart two reports can be and
// still count as evidence that distinct machines share one identity. It also
// controls how long a detected conflict stays visible after the flapping
// stops (e.g. one of the clones was fixed or shut down). Sized to cover a few
// report cycles even for agents on multi-minute intervals.
const dockerIdentityConflictWindow = 15 * time.Minute

// trackDockerHostIdentity feeds one report's identity fields into the flap
// tracker for the resolved host identifier and returns the active conflict,
// if any. The secondary identity field is the reported machine ID: cloned
// VMs that share /etc/machine-id alternate hostnames every report cycle and
// trip the revisit detector immediately. Callers must not hold m.mu.
func (m *Monitor) trackDockerHostIdentity(identifier, hostname, machineID string, now time.Time) *models.DockerHostIdentityConflict {
	conflict := m.observeIdentityFlap(&m.dockerIdentityFlaps, dockerIdentityConflictWindow, identifier, hostname, machineID, now)
	if conflict == nil {
		return nil
	}
	return &models.DockerHostIdentityConflict{
		Hostnames:  conflict.hostnames,
		MachineIDs: conflict.secondaries,
		FirstSeen:  conflict.firstSeen,
		LastSeen:   conflict.lastSeen,
	}
}

// clearDockerHostIdentityTrackingLocked drops flap state for a host identity,
// e.g. when the host is deliberately removed. Callers must hold m.mu.
func (m *Monitor) clearDockerHostIdentityTrackingLocked(hostID string) {
	delete(m.dockerIdentityFlaps, hostID)
}
