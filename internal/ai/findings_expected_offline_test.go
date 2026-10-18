package ai

import (
	"testing"
	"time"
)

// Expected-offline is availability-specific in every runtime that reads the
// canonical monitoring mode: the alert runtime suppresses only offline and
// availability signals, and Patrol must silence only availability findings.
// Muted is the posture that silences everything.

func expectedOfflineTestStore(projection ResourceOperatorStateProjection) *FindingsStore {
	store := NewFindingsStore()
	store.SetResourceOperatorStateProvider(
		ResourceOperatorStateProviderFunc(func(_ string, _ time.Time) (ResourceOperatorStateProjection, bool) {
			return projection, true
		}),
	)
	return store
}

func operatorStateDismissCause(f *Finding) string {
	if f == nil {
		return ""
	}
	return findOperatorStateDismissCause(f)
}

func expectedOfflineTestFindings() (availability *Finding, others []*Finding) {
	availability = &Finding{
		ID: "f-down", ResourceID: "vm:standby", Severity: FindingSeverityWarning,
		Category: FindingCategoryReliability, Title: `VM "standby" is offline`,
	}
	others = []*Finding{
		{
			ID: "f-disk", ResourceID: "vm:standby", Severity: FindingSeverityWarning,
			Category: FindingCategoryCapacity, Title: `Disk usage on "standby" is at 95%`,
		},
		{
			ID: "f-memory", ResourceID: "vm:standby", Severity: FindingSeverityWarning,
			Category: FindingCategoryPerformance, Title: `Memory usage on "standby" is at 93%`,
		},
		{
			ID: "f-smart", ResourceID: "vm:standby", Severity: FindingSeverityCritical,
			Key: "smart-failure", Category: FindingCategoryReliability,
			Title: `Disk sda reports 8 offline uncorrectable sectors`,
		},
	}
	return availability, others
}

func TestFindingsStore_Add_ExpectedOfflineSuppressesOnlyAvailabilityFindings(t *testing.T) {
	for _, tc := range []struct {
		name       string
		projection ResourceOperatorStateProjection
		wantCause  string
	}{
		{"monitoring mode", ResourceOperatorStateProjection{MonitoringMode: "expected_offline", IntentionallyOffline: true}, "expected_offline"},
		{"legacy flag only", ResourceOperatorStateProjection{IntentionallyOffline: true}, "intentionally_offline"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := expectedOfflineTestStore(tc.projection)
			availability, others := expectedOfflineTestFindings()

			store.Add(availability)
			down := store.Get(availability.ID)
			if down == nil || down.DismissedReason != "expected_behavior" || down.IsActive() {
				t.Fatalf("availability finding on an expected-offline resource must be auto-dismissed; got %+v", down)
			}
			if cause := operatorStateDismissCause(down); cause != tc.wantCause {
				t.Fatalf("availability finding operator_state_cause = %q, want %q", cause, tc.wantCause)
			}

			for _, f := range others {
				store.Add(f)
				got := store.Get(f.ID)
				if got == nil {
					t.Fatalf("%s: finding must be persisted", f.ID)
				}
				if got.DismissedReason != "" || got.AcknowledgedAt != nil || !got.IsActive() {
					t.Errorf("%s (%s) must stay visible on an expected-offline resource; got DismissedReason=%q AcknowledgedAt=%v", f.ID, f.Category, got.DismissedReason, got.AcknowledgedAt)
				}
				if cause := operatorStateDismissCause(got); cause != "" {
					t.Errorf("%s must carry no operator-state dismissal; got cause %q", f.ID, cause)
				}
			}
		})
	}
}

// Negative control: the same non-availability findings are silenced when the
// resource is muted, so the visibility above comes from expected-offline's
// scope and not from a provider the store ignored.
func TestFindingsStore_Add_MutedSuppressesEveryFindingCategory(t *testing.T) {
	store := expectedOfflineTestStore(ResourceOperatorStateProjection{MonitoringMode: "muted"})
	availability, others := expectedOfflineTestFindings()
	for _, f := range append([]*Finding{availability}, others...) {
		store.Add(f)
		got := store.Get(f.ID)
		if got == nil || got.DismissedReason != "expected_behavior" || got.IsActive() {
			t.Fatalf("%s must be auto-dismissed on a muted resource; got %+v", f.ID, got)
		}
		if cause := operatorStateDismissCause(got); cause != "monitoring_muted" {
			t.Errorf("%s operator_state_cause = %q, want monitoring_muted", f.ID, cause)
		}
	}
}

// Moving a resource from muted to expected-offline wakes the non-availability
// findings muted silenced, while the availability finding stays quiet.
func TestFindingsStore_Add_MutedToExpectedOfflineWakesOnlyNonAvailabilityFindings(t *testing.T) {
	projection := ResourceOperatorStateProjection{MonitoringMode: "muted"}
	store := NewFindingsStore()
	store.SetResourceOperatorStateProvider(
		ResourceOperatorStateProviderFunc(func(_ string, _ time.Time) (ResourceOperatorStateProjection, bool) {
			return projection, true
		}),
	)
	availability, others := expectedOfflineTestFindings()
	disk := others[0]
	store.Add(copyFindingForTest(availability))
	store.Add(copyFindingForTest(disk))

	projection = ResourceOperatorStateProjection{MonitoringMode: "expected_offline", IntentionallyOffline: true}
	store.Add(copyFindingForTest(availability))
	store.Add(copyFindingForTest(disk))

	if down := store.Get(availability.ID); down == nil || down.DismissedReason != "expected_behavior" {
		t.Fatalf("availability finding must stay dismissed under expected-offline; got %+v", down)
	}
	woken := store.Get(disk.ID)
	if woken == nil || woken.DismissedReason != "" || !woken.IsActive() {
		t.Fatalf("disk finding must wake once only expected-offline applies; got %+v", woken)
	}
	lifted := false
	for _, ev := range woken.Lifecycle {
		if ev.Type == "suppression_lifted" && ev.Metadata["previous_cause"] == "monitoring_muted" {
			lifted = true
		}
	}
	if !lifted {
		t.Error("expected a suppression_lifted lifecycle event naming monitoring_muted")
	}
}

func copyFindingForTest(f *Finding) *Finding {
	c := *f
	c.Lifecycle = nil
	return &c
}
