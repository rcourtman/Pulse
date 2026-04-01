package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestHasRecentGuestAgentEvidenceAcceptsUnifiedReadStateStatus(t *testing.T) {
	now := time.Now()

	prev := &models.VM{
		Type:         "qemu",
		Status:       "online",
		IPAddresses:  []string{"192.168.1.50"},
		AgentVersion: "8.1.0",
		LastSeen:     now.Add(-time.Minute),
	}

	if !hasRecentGuestAgentEvidence(prev, now) {
		t.Fatal("expected unified read-state VM snapshot to count as recent guest-agent evidence")
	}
}

func TestHasRecentGuestAgentEvidenceRejectsStaleSnapshots(t *testing.T) {
	now := time.Now()

	prev := &models.VM{
		Type:         "qemu",
		Status:       "online",
		IPAddresses:  []string{"192.168.1.50"},
		AgentVersion: "8.1.0",
		LastSeen:     now.Add(-recentGuestAgentEvidenceMaxAge - time.Second),
	}

	if hasRecentGuestAgentEvidence(prev, now) {
		t.Fatal("expected stale guest-agent evidence to be rejected")
	}
}
