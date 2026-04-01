package monitoring

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

const recentGuestAgentEvidenceMaxAge = 10 * time.Minute

func hasRecentGuestAgentEvidence(prev *models.VM, now time.Time) bool {
	// Previous unified read-state snapshots normalize guest status to resource health
	// (for example "online"), so continuity depends on recent guest-agent evidence
	// rather than on replaying an exact prior runtime status string.
	if prev == nil || prev.Type != "qemu" {
		return false
	}
	if prev.LastSeen.IsZero() || now.Sub(prev.LastSeen) > recentGuestAgentEvidenceMaxAge {
		return false
	}

	if prev.AgentVersion != "" ||
		len(prev.IPAddresses) > 0 ||
		len(prev.NetworkInterfaces) > 0 ||
		prev.OSName != "" ||
		prev.OSVersion != "" {
		return true
	}

	if len(prev.Disks) > 0 && !strings.HasPrefix(prev.DiskStatusReason, "prev-") {
		return true
	}

	return false
}

func shouldQueryGuestAgent(vmStatus *proxmox.VMStatus, prev *models.VM, now time.Time) bool {
	if vmStatus != nil {
		return vmStatus.Agent.Value > 0
	}
	return hasRecentGuestAgentEvidence(prev, now)
}
