package monitoring

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

const recentGuestAgentEvidenceMaxAge = 10 * time.Minute

func hasRecentGuestAgentEvidence(prev *models.VM, now time.Time) bool {
	observedAt := guestAgentEvidenceTime(prev)
	return !observedAt.IsZero() && !observedAt.After(now) && now.Sub(observedAt) <= recentGuestAgentEvidenceMaxAge
}

func guestAgentEvidenceTime(prev *models.VM) time.Time {
	if prev == nil || prev.Type != "qemu" {
		return time.Time{}
	}
	if prev.GuestAgentEvidence.Explicit || !prev.GuestAgentEvidence.ObservedAt.IsZero() {
		return prev.GuestAgentEvidence.ObservedAt
	}
	// Import a legacy direct snapshot's receipt once. Modern producers mark
	// evidence explicitly, including its absence, before preserving identity.
	// Its subsequent LastSeen can therefore never refresh this original time.
	if prev.AgentVersion != "" ||
		prev.GuestAgentStatus == "available" ||
		len(prev.IPAddresses) > 0 ||
		len(prev.NetworkInterfaces) > 0 ||
		prev.OSName != "" ||
		prev.OSVersion != "" {
		return prev.LastSeen
	}

	if len(prev.Disks) > 0 && !strings.HasPrefix(prev.DiskStatusReason, "prev-") {
		return prev.LastSeen
	}
	return time.Time{}
}

func retainGuestAgentEvidence(prev *models.VM, now time.Time) models.GuestAgentEvidence {
	evidence := models.GuestAgentEvidence{Explicit: true}
	renewGuestAgentEvidence(&evidence, guestAgentEvidenceTime(prev), now)
	return evidence
}

func renewGuestAgentEvidence(evidence *models.GuestAgentEvidence, observedAt, now time.Time) {
	if !observedAt.IsZero() && !observedAt.After(now) && observedAt.After(evidence.ObservedAt) {
		evidence.ObservedAt = observedAt
	}
}

func shouldQueryGuestAgent(vmStatus *proxmox.VMStatus, prev *models.VM, now time.Time) bool {
	if vmStatus != nil {
		return vmStatus.Lock == "" && vmStatus.Agent.IsAvailable()
	}
	return hasRecentGuestAgentEvidence(prev, now)
}
