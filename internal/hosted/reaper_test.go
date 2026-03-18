package hosted

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type mockOrgLister struct {
	orgs  []*models.Organization
	err   error
	calls int
}

func (m *mockOrgLister) ListOrganizations() ([]*models.Organization, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return m.orgs, nil
}

type mockOrgDeleter struct {
	err        error
	calls      int
	deletedOrg []string
}

func (m *mockOrgDeleter) DeleteOrganization(orgID string) error {
	m.calls++
	m.deletedOrg = append(m.deletedOrg, orgID)
	return m.err
}

func TestReaperDetectsExpiredOrg(t *testing.T) {
	fixedTime := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)
	requestedAt := fixedTime.Add(-31 * 24 * time.Hour)

	lister := &mockOrgLister{
		orgs: []*models.Organization{
			{
				ID:                  "org-expired",
				Status:              models.OrgStatusPendingDeletion,
				DeletionRequestedAt: &requestedAt,
				RetentionDays:       30,
			},
		},
	}
	deleter := &mockOrgDeleter{}
	r := NewReaper(lister, deleter, time.Hour, false)
	r.now = func() time.Time { return fixedTime }

	results := r.scan()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ReapActionDryRun {
		t.Fatalf("expected action %q, got %q", ReapActionDryRun, results[0].Action)
	}
	if deleter.calls != 0 {
		t.Fatalf("expected no delete calls in dry-run mode, got %d", deleter.calls)
	}
}

func TestReaperSkipsNonExpiredOrg(t *testing.T) {
	fixedTime := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)
	requestedAt := fixedTime.Add(-15 * 24 * time.Hour)

	lister := &mockOrgLister{
		orgs: []*models.Organization{
			{
				ID:                  "org-not-expired",
				Status:              models.OrgStatusPendingDeletion,
				DeletionRequestedAt: &requestedAt,
				RetentionDays:       30,
			},
		},
	}
	deleter := &mockOrgDeleter{}
	r := NewReaper(lister, deleter, time.Hour, false)
	r.now = func() time.Time { return fixedTime }

	results := r.scan()
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestReaperSkipsDefaultOrg(t *testing.T) {
	fixedTime := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)
	requestedAt := fixedTime.Add(-40 * 24 * time.Hour)

	lister := &mockOrgLister{
		orgs: []*models.Organization{
			{
				ID:                  "default",
				Status:              models.OrgStatusPendingDeletion,
				DeletionRequestedAt: &requestedAt,
				RetentionDays:       30,
			},
		},
	}
	deleter := &mockOrgDeleter{}
	r := NewReaper(lister, deleter, time.Hour, true)
	r.now = func() time.Time { return fixedTime }

	results := r.scan()
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
	if deleter.calls != 0 {
		t.Fatalf("expected no delete calls for default org, got %d", deleter.calls)
	}
}

func TestReaperSkipsInvalidOrgID(t *testing.T) {
	fixedTime := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)
	requestedAt := fixedTime.Add(-40 * 24 * time.Hour)

	lister := &mockOrgLister{
		orgs: []*models.Organization{
			{
				ID:                  "bad/org-id",
				Status:              models.OrgStatusPendingDeletion,
				DeletionRequestedAt: &requestedAt,
				RetentionDays:       30,
			},
		},
	}
	deleter := &mockOrgDeleter{}
	r := NewReaper(lister, deleter, time.Hour, true)
	r.now = func() time.Time { return fixedTime }

	results := r.scan()
	if len(results) != 0 {
		t.Fatalf("expected 0 results for invalid org ID, got %d", len(results))
	}
	if deleter.calls != 0 {
		t.Fatalf("expected no delete calls for invalid org ID, got %d", deleter.calls)
	}
}

func TestReaperSkipsActiveOrg(t *testing.T) {
	fixedTime := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)
	requestedAt := fixedTime.Add(-40 * 24 * time.Hour)

	lister := &mockOrgLister{
		orgs: []*models.Organization{
			{
				ID:                  "org-active",
				Status:              models.OrgStatusActive,
				DeletionRequestedAt: &requestedAt,
				RetentionDays:       30,
			},
		},
	}
	deleter := &mockOrgDeleter{}
	r := NewReaper(lister, deleter, time.Hour, true)
	r.now = func() time.Time { return fixedTime }

	results := r.scan()
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
	if deleter.calls != 0 {
		t.Fatalf("expected no delete calls for active org, got %d", deleter.calls)
	}
}

func TestReaperLiveModeDeletes(t *testing.T) {
	fixedTime := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)
	requestedAt := fixedTime.Add(-31 * 24 * time.Hour)

	lister := &mockOrgLister{
		orgs: []*models.Organization{
			{
				ID:                  "org-live-delete",
				Status:              models.OrgStatusPendingDeletion,
				DeletionRequestedAt: &requestedAt,
				RetentionDays:       30,
			},
		},
	}
	deleter := &mockOrgDeleter{}
	r := NewReaper(lister, deleter, time.Hour, true)
	r.now = func() time.Time { return fixedTime }

	results := r.scan()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ReapActionDeleted {
		t.Fatalf("expected action %q, got %q", ReapActionDeleted, results[0].Action)
	}
	if results[0].Error != nil {
		t.Fatalf("expected nil result error, got %v", results[0].Error)
	}
	if deleter.calls != 1 {
		t.Fatalf("expected one delete call, got %d", deleter.calls)
	}
	if len(deleter.deletedOrg) != 1 || deleter.deletedOrg[0] != "org-live-delete" {
		t.Fatalf("expected delete call for org-live-delete, got %v", deleter.deletedOrg)
	}
}

type recordingDeleter struct {
	calls []string
	err   error
}

func (d *recordingDeleter) DeleteOrganization(orgID string) error {
	d.calls = append(d.calls, "delete:"+orgID)
	return d.err
}

func TestReaperOnBeforeDeleteHookCalled(t *testing.T) {
	fixedTime := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)
	requestedAt := fixedTime.Add(-31 * 24 * time.Hour)

	lister := &mockOrgLister{
		orgs: []*models.Organization{
			{
				ID:                  "org-hook-called",
				Status:              models.OrgStatusPendingDeletion,
				DeletionRequestedAt: &requestedAt,
				RetentionDays:       30,
			},
		},
	}
	deleter := &recordingDeleter{}
	r := NewReaper(lister, deleter, time.Hour, true)
	r.now = func() time.Time { return fixedTime }

	var order []string
	r.OnBeforeDelete = func(orgID string) error {
		order = append(order, "hook:"+orgID)
		return nil
	}

	results := r.scan()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error != nil {
		t.Fatalf("expected nil result error, got %v", results[0].Error)
	}

	if len(deleter.calls) != 1 || deleter.calls[0] != "delete:org-hook-called" {
		t.Fatalf("expected deleter called for org-hook-called, got %v", deleter.calls)
	}
	if len(order) != 1 || order[0] != "hook:org-hook-called" {
		t.Fatalf("expected hook called for org-hook-called, got %v", order)
	}

	combined := append(append([]string{}, order...), deleter.calls...)
	if len(combined) != 2 || combined[0] != "hook:org-hook-called" || combined[1] != "delete:org-hook-called" {
		t.Fatalf("expected hook called before delete, got %v", combined)
	}
}

func TestReaperOnBeforeDeleteHookErrorSkipsDelete(t *testing.T) {
	fixedTime := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)
	requestedAt := fixedTime.Add(-31 * 24 * time.Hour)

	lister := &mockOrgLister{
		orgs: []*models.Organization{
			{
				ID:                  "org-hook-error",
				Status:              models.OrgStatusPendingDeletion,
				DeletionRequestedAt: &requestedAt,
				RetentionDays:       30,
			},
		},
	}
	deleter := &recordingDeleter{}
	r := NewReaper(lister, deleter, time.Hour, true)
	r.now = func() time.Time { return fixedTime }
	r.OnBeforeDelete = func(orgID string) error {
		return errors.New("hook error")
	}

	results := r.scan()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == nil {
		t.Fatal("expected result error, got nil")
	}
	if len(deleter.calls) != 0 {
		t.Fatalf("expected no delete calls when hook errors, got %v", deleter.calls)
	}
}

func TestReaperDryRunDoesNotDelete(t *testing.T) {
	fixedTime := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)
	requestedAt := fixedTime.Add(-31 * 24 * time.Hour)

	lister := &mockOrgLister{
		orgs: []*models.Organization{
			{
				ID:                  "org-dry-run",
				Status:              models.OrgStatusPendingDeletion,
				DeletionRequestedAt: &requestedAt,
				RetentionDays:       30,
			},
		},
	}
	deleter := &mockOrgDeleter{}
	r := NewReaper(lister, deleter, time.Hour, false)
	r.now = func() time.Time { return fixedTime }

	results := r.scan()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ReapActionDryRun {
		t.Fatalf("expected action %q, got %q", ReapActionDryRun, results[0].Action)
	}
	if deleter.calls != 0 {
		t.Fatalf("expected no delete calls in dry-run mode, got %d", deleter.calls)
	}
}

func TestReaperGracefulShutdown(t *testing.T) {
	lister := &mockOrgLister{}
	deleter := &mockOrgDeleter{err: errors.New("unexpected delete call")}
	r := NewReaper(lister, deleter, time.Hour, true)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- r.Run(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil error on graceful shutdown, got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("expected Run to exit promptly after context cancellation")
	}
}

func TestReaperRunNormalizesInvalidScanInterval(t *testing.T) {
	testCases := []struct {
		name     string
		interval time.Duration
	}{
		{name: "zero_interval", interval: 0},
		{name: "negative_interval", interval: -1 * time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &Reaper{scanInterval: tc.interval}

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("Run panicked for scan interval %s: %v", tc.interval, recovered)
				}
			}()

			if err := r.Run(ctx); err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}

			if r.scanInterval != defaultReaperScanInterval {
				t.Fatalf("expected scan interval %s, got %s", defaultReaperScanInterval, r.scanInterval)
			}
		})
	}
}

func TestReaperClampRetentionDays(t *testing.T) {
	testCases := []struct {
		name    string
		input   int
		want    int
		clamped bool
	}{
		{name: "in_range", input: 30, want: 30, clamped: false},
		{name: "upper_bound", input: maxRetentionDays, want: maxRetentionDays, clamped: false},
		{name: "lower_bound", input: minRetentionDays, want: minRetentionDays, clamped: false},
		{name: "above_upper_bound", input: maxRetentionDays + 1, want: maxRetentionDays, clamped: true},
		{name: "below_lower_bound", input: minRetentionDays - 1, want: minRetentionDays, clamped: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, clamped := clampRetentionDays(tc.input)
			if got != tc.want {
				t.Fatalf("expected normalized retention %d, got %d", tc.want, got)
			}
			if clamped != tc.clamped {
				t.Fatalf("expected clamped=%t, got %t", tc.clamped, clamped)
			}
		})
	}
}

func TestReaperScanClampsRetentionOverflow(t *testing.T) {
	fixedTime := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)
	requestedAt := fixedTime.Add(-1 * time.Hour)

	lister := &mockOrgLister{
		orgs: []*models.Organization{
			{
				ID:                  "org-overflow-retention",
				Status:              models.OrgStatusPendingDeletion,
				DeletionRequestedAt: &requestedAt,
				RetentionDays:       maxRetentionDays + 1,
			},
		},
	}
	deleter := &mockOrgDeleter{}
	r := NewReaper(lister, deleter, time.Hour, false)
	r.now = func() time.Time { return fixedTime }

	results := r.scan()
	if len(results) != 0 {
		t.Fatalf("expected no expired orgs when retention overflows and is clamped, got %d", len(results))
	}
	if deleter.calls != 0 {
		t.Fatalf("expected no delete calls in dry-run mode, got %d", deleter.calls)
	}
}
