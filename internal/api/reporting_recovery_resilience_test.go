package api

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func TestReportingHandlers_ListBackupsForReport_ToleratesMalformedPersistedMetadata(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }

	recoveryHandler, dbPath := newRecoveryHandlerWithPersistedPoint(t, recovery.RecoveryPoint{
		ID:                "report-point-bad-json",
		Provider:          recovery.ProviderProxmoxPBS,
		Kind:              recovery.KindBackup,
		Mode:              recovery.ModeRemote,
		Outcome:           recovery.OutcomeSuccess,
		SubjectResourceID: "vm-123",
		SubjectRef: &recovery.ExternalRef{
			Type: "proxmox-vm",
			Name: "Archive VM",
		},
		RepositoryRef: &recovery.ExternalRef{
			Type: "pbs-datastore",
			Name: "fast-store",
		},
		CompletedAt: timePtr(time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC)),
		Verified:    boolPtr(true),
		Immutable:   boolPtr(true),
		Details: map[string]any{
			"storage": "fast-store",
			"volid":   "vm/123/2026-02-20T09:00:00Z",
		},
	})
	corruptRecoveryRowJSON(t, dbPath, "report-point-bad-json", true, true, true)

	handler := NewReportingHandlers(nil, recoveryHandler.manager)
	start := time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 20, 23, 59, 59, 0, time.UTC)

	backups := handler.listBackupsForReport(context.Background(), "default", "vm-123", start, end)
	if len(backups) != 1 {
		t.Fatalf("expected exactly 1 backup, got %d", len(backups))
	}
	if backups[0].Type != "pbs" {
		t.Fatalf("backup type = %q, want %q", backups[0].Type, "pbs")
	}
	if backups[0].Storage != "" {
		t.Fatalf("backup storage = %q, want empty after malformed details degradation", backups[0].Storage)
	}
	if backups[0].VolID != "" {
		t.Fatalf("backup volid = %q, want empty after malformed details degradation", backups[0].VolID)
	}
	if !backups[0].Verified {
		t.Fatal("expected verified flag to survive malformed metadata degradation")
	}
	if !backups[0].Protected {
		t.Fatal("expected protected flag to survive malformed metadata degradation")
	}
	if !backups[0].Timestamp.Equal(time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("backup timestamp = %s, want %s", backups[0].Timestamp, time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC))
	}
}
