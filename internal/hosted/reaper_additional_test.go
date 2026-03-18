package hosted

import (
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestNewReaperDefaultsToMinuteInterval(t *testing.T) {
	r := NewReaper(&mockOrgLister{}, &mockOrgDeleter{}, 0, false)
	if r.scanInterval != time.Minute {
		t.Fatalf("expected default scan interval %v, got %v", time.Minute, r.scanInterval)
	}
}

func TestReaperScanOnceHandlesNilReceiver(t *testing.T) {
	var r *Reaper
	if got := r.ScanOnce(); got != nil {
		t.Fatalf("expected nil result for nil receiver, got %v", got)
	}
}

func TestReaperScanOnceReturnsDryRunResult(t *testing.T) {
	fixedTime := time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC)
	requestedAt := fixedTime.Add(-31 * 24 * time.Hour)

	lister := &mockOrgLister{
		orgs: []*models.Organization{
			{
				ID:                  "org-scan-once",
				Status:              models.OrgStatusPendingDeletion,
				DeletionRequestedAt: &requestedAt,
				RetentionDays:       30,
			},
		},
	}

	r := NewReaper(lister, &mockOrgDeleter{}, time.Minute, false)
	r.now = func() time.Time { return fixedTime }

	results := r.ScanOnce()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "dry_run" {
		t.Fatalf("expected dry_run action, got %q", results[0].Action)
	}
}

func TestReaperScanReturnsNilForMissingDependenciesOrListError(t *testing.T) {
	failingLister := &mockOrgLister{err: errors.New("list failed")}
	testCases := []struct {
		name   string
		reaper *Reaper
	}{
		{
			name:   "missing lister",
			reaper: &Reaper{now: time.Now},
		},
		{
			name:   "missing clock",
			reaper: &Reaper{lister: &mockOrgLister{}},
		},
		{
			name:   "lister error",
			reaper: &Reaper{lister: failingLister, now: time.Now},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.reaper.scan(); got != nil {
				t.Fatalf("expected nil scan result, got %v", got)
			}
		})
	}
}

func TestReaperLiveModeReportsNilDeleterError(t *testing.T) {
	fixedTime := time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC)
	requestedAt := fixedTime.Add(-31 * 24 * time.Hour)

	lister := &mockOrgLister{
		orgs: []*models.Organization{
			{
				ID:                  "org-nil-deleter",
				Status:              models.OrgStatusPendingDeletion,
				DeletionRequestedAt: &requestedAt,
				RetentionDays:       30,
			},
		},
	}

	r := NewReaper(lister, nil, time.Minute, true)
	r.now = func() time.Time { return fixedTime }

	results := r.scan()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "deleted" {
		t.Fatalf("expected deleted action, got %q", results[0].Action)
	}
	if results[0].Error == nil || results[0].Error.Error() != "org deleter is nil" {
		t.Fatalf("expected org deleter is nil error, got %v", results[0].Error)
	}
}

func TestReaperLiveModePropagatesDeleteError(t *testing.T) {
	fixedTime := time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC)
	requestedAt := fixedTime.Add(-31 * 24 * time.Hour)
	deleteErr := errors.New("delete failed")

	lister := &mockOrgLister{
		orgs: []*models.Organization{
			{
				ID:                  "org-delete-error",
				Status:              models.OrgStatusPendingDeletion,
				DeletionRequestedAt: &requestedAt,
				RetentionDays:       30,
			},
		},
	}

	deleter := &mockOrgDeleter{err: deleteErr}
	r := NewReaper(lister, deleter, time.Minute, true)
	r.now = func() time.Time { return fixedTime }

	results := r.scan()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !errors.Is(results[0].Error, deleteErr) {
		t.Fatalf("expected delete error %v, got %v", deleteErr, results[0].Error)
	}
	if deleter.calls != 1 {
		t.Fatalf("expected one delete call, got %d", deleter.calls)
	}
}
