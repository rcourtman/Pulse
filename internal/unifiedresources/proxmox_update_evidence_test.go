package unifiedresources

import (
	"testing"
	"time"
)

func TestMergeProxmoxDataTreatsUpdateEvidenceAsAuthoritative(t *testing.T) {
	oldCheckedAt := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	newCheckedAt := oldCheckedAt.Add(time.Hour)
	existing := &ProxmoxData{
		PendingUpdates:          9,
		PendingUpdatesCheckedAt: &oldCheckedAt,
		PendingUpdatesStatus:    "checked",
	}
	incoming := &ProxmoxData{
		PendingUpdates:          0,
		PendingUpdatesCheckedAt: &newCheckedAt,
		PendingUpdatesStatus:    "checked",
	}

	merged := mergeProxmoxData(existing, incoming)
	if merged.PendingUpdates != 0 || merged.PendingUpdatesStatus != "checked" || merged.PendingUpdatesCheckedAt == nil || !merged.PendingUpdatesCheckedAt.Equal(newCheckedAt) {
		t.Fatalf("expected confirmed zero to replace old positive evidence, got %+v", merged)
	}

	incoming.PendingUpdatesStatus = "unavailable"
	incoming.PendingUpdatesReason = "permission_denied"
	incoming.PendingUpdatesCheckedAt = nil
	merged = mergeProxmoxData(merged, incoming)
	if merged.PendingUpdatesStatus != "unavailable" || merged.PendingUpdatesReason != "permission_denied" || merged.PendingUpdatesCheckedAt != nil {
		t.Fatalf("expected unavailable evidence to clear inherited check metadata, got %+v", merged)
	}
}
