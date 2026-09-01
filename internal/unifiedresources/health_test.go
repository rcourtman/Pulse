package unifiedresources

import (
	"testing"
	"time"
)

func TestEvaluateResourceHealthPrecedenceAndVerdicts(t *testing.T) {
	now := time.Date(2026, 8, 31, 20, 0, 0, 0, time.UTC)
	staleStatus := map[DataSource]SourceStatus{
		SourceAgent: {Status: "stale", LastSeen: now.Add(-10 * time.Minute)},
	}
	tests := []struct {
		name     string
		resource Resource
		alerts   []ResourceHealthAlert
		want     ResourceHealthVerdict
		code     string
	}{
		{
			name:     "critical alert wins over stale telemetry",
			resource: Resource{ID: "vm-1", Type: ResourceTypeVM, Status: StatusOnline, LastSeen: now.Add(-10 * time.Minute), SourceStatus: staleStatus},
			alerts:   []ResourceHealthAlert{{ResourceID: "vm-1", Level: "critical", Type: "cpu"}},
			want:     HealthCritical, code: "critical_alert",
		},
		{
			name:     "warning alert wins over powered off workload",
			resource: Resource{ID: "vm-2", Type: ResourceTypeVM, Status: StatusOffline, Proxmox: &ProxmoxData{RuntimeStatus: "stopped"}},
			alerts:   []ResourceHealthAlert{{ResourceID: "vm-2", Level: "warning", Type: "backup"}},
			want:     HealthAttention, code: "warning_alert",
		},
		{
			name:     "confirmed availability failure is critical",
			resource: Resource{ID: "check-1", Type: ResourceTypeNetworkEndpoint, Status: StatusOnline, Availability: &AvailabilityData{Enabled: true, Available: false, ConsecutiveFailures: 3, FailureThreshold: 3}},
			want:     HealthCritical, code: "availability_failed",
		},
		{
			name:     "offline infrastructure is critical",
			resource: Resource{ID: "host-1", Type: ResourceTypeAgent, Status: StatusOffline},
			want:     HealthCritical, code: "offline",
		},
		{
			name:     "old recorded backup needs attention",
			resource: Resource{ID: "vm-3", Type: ResourceTypeVM, Status: StatusOnline, Proxmox: &ProxmoxData{RuntimeStatus: "running", LastBackup: now.Add(-9 * 24 * time.Hour)}},
			want:     HealthAttention, code: "backup_stale",
		},
		{
			name:     "stale telemetry is explicit",
			resource: Resource{ID: "host-2", Type: ResourceTypeAgent, Status: StatusWarning, LastSeen: now.Add(-10 * time.Minute), SourceStatus: staleStatus},
			want:     HealthStale, code: "telemetry_stale",
		},
		{
			name:     "powered off workload is neutral",
			resource: Resource{ID: "vm-4", Type: ResourceTypeVM, Status: StatusOffline, Proxmox: &ProxmoxData{RuntimeStatus: "stopped"}},
			want:     HealthOff, code: "powered_off",
		},
		{
			name:     "healthy resource is verified",
			resource: Resource{ID: "host-3", Type: ResourceTypeAgent, Status: StatusOnline, LastSeen: now},
			want:     HealthOK,
		},
		{
			name:     "positive status without telemetry cannot be healthy",
			resource: Resource{ID: "host-missing", Type: ResourceTypeAgent, Status: StatusOnline},
			want:     HealthUnknown, code: "telemetry_missing",
		},
		{
			name:     "informational alert does not invent attention",
			resource: Resource{ID: "host-info", Type: ResourceTypeAgent, Status: StatusOnline, LastSeen: now},
			alerts:   []ResourceHealthAlert{{ResourceID: "host-info", Level: "info", Type: "notice"}},
			want:     HealthOK,
		},
		{
			name:     "missing telemetry is unknown",
			resource: Resource{ID: "host-4", Type: ResourceTypeAgent, Status: StatusUnknown, SourceStatus: map[DataSource]SourceStatus{SourceAgent: {Status: "unknown"}}},
			want:     HealthUnknown, code: "telemetry_missing",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := EvaluateResourceHealth(test.resource, test.alerts, now)
			if got.Verdict != test.want {
				t.Fatalf("verdict = %q, want %q (%+v)", got.Verdict, test.want, got)
			}
			if test.code == "" {
				if len(got.Reasons) != 0 {
					t.Fatalf("healthy verdict has reasons: %+v", got.Reasons)
				}
				return
			}
			if len(got.Reasons) == 0 || got.Reasons[0].Code != test.code {
				t.Fatalf("reason = %+v, want code %q", got.Reasons, test.code)
			}
		})
	}
}

func TestEvaluateResourceHealthMatchesCanonicalAliases(t *testing.T) {
	resource := Resource{
		ID: "current", Type: ResourceTypeVM, Status: StatusOnline,
		Canonical: &CanonicalIdentity{PrimaryID: "primary", SupersededIDs: []string{"old"}},
	}
	got := EvaluateResourceHealth(resource, []ResourceHealthAlert{{ResourceID: "old", Level: "critical"}}, time.Now())
	if got.Verdict != HealthCritical {
		t.Fatalf("alias alert verdict = %q, want critical", got.Verdict)
	}
}
