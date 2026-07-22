package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/deploy"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// This file raises branch/function coverage for five currently-uncovered
// pure helper functions in the internal/api package:
//   - filterPVEBackups        (pve_backups.go)
//   - filterReplicationJobs    (replication.go)
//   - cephDiscoveryTarget      (resources.go)
//   - capMetricPointSeriesByIndex (router.go)
//   - deriveInstallJobStatus   (deploy_handlers.go)
//
// Conventions mirror internal/api/deploy_handlers_test.go (package api,
// httptest.NewRequest for query-param driven helpers, table-driven subtests).

// --- filterPVEBackups -------------------------------------------------------

func branchcov0722PMSamplePVEBackups() models.PVEBackups {
	return models.PVEBackups{
		BackupTasks: []models.BackupTask{
			{ID: "t1", Instance: "pve-a", Node: "node1", VMID: 100},
			{ID: "t2", Instance: "pve-b", Node: "node2", VMID: 200},
		},
		StorageBackups: []models.StorageBackup{
			{ID: "s1", Instance: "pve-a", Node: "node1", VMID: 100},
			{ID: "s2", Instance: "pve-b", Node: "node2", VMID: 200},
		},
		GuestSnapshots: []models.GuestSnapshot{
			{ID: "g1", Instance: "pve-a", Node: "node1", VMID: 100},
			{ID: "g2", Instance: "pve-b", Node: "node2", VMID: 200},
		},
	}
}

func TestBranchcov0722PM_FilterPVEBackups(t *testing.T) {
	backups := branchcov0722PMSamplePVEBackups()

	checkIDs := func(label string, got []string, want []string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: got %d entries (%v), want %d (%v)", label, len(got), got, len(want), want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s[%d]: got %q, want %q", label, i, got[i], want[i])
			}
		}
	}

	tests := []struct {
		name      string
		query     string
		wantTasks []string // expected BackupTask IDs
		wantStore []string // expected StorageBackup IDs
		wantSnaps []string // expected GuestSnapshot IDs
	}{
		{
			name:      "no_filter_passes_all_through",
			query:     "/x",
			wantTasks: []string{"t1", "t2"},
			wantStore: []string{"s1", "s2"},
			wantSnaps: []string{"g1", "g2"},
		},
		{
			name:      "instance_match_pve_a",
			query:     "/x?instance=pve-a",
			wantTasks: []string{"t1"},
			wantStore: []string{"s1"},
			wantSnaps: []string{"g1"},
		},
		{
			name:      "instance_case_insensitive",
			query:     "/x?instance=PVE-A",
			wantTasks: []string{"t1"},
			wantStore: []string{"s1"},
			wantSnaps: []string{"g1"},
		},
		{
			name:      "instance_non_matching_yields_empty",
			query:     "/x?instance=does-not-exist",
			wantTasks: nil,
			wantStore: nil,
			wantSnaps: nil,
		},
		{
			name:      "node_match_node2",
			query:     "/x?node=node2",
			wantTasks: []string{"t2"},
			wantStore: []string{"s2"},
			wantSnaps: []string{"g2"},
		},
		{
			name:      "vmid_match_200",
			query:     "/x?vmid=200",
			wantTasks: []string{"t2"},
			wantStore: []string{"s2"},
			wantSnaps: []string{"g2"},
		},
		{
			// Non-numeric vmid can never equal strconv.Itoa(VMID), so every
			// entry is filtered out — this is the "invalid filter value" arm.
			name:      "vmid_invalid_non_numeric_yields_empty",
			query:     "/x?vmid=not-a-number",
			wantTasks: nil,
			wantStore: nil,
			wantSnaps: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.query, nil)
			got := filterPVEBackups(backups, req)

			taskIDs := make([]string, len(got.BackupTasks))
			for i := range got.BackupTasks {
				taskIDs[i] = got.BackupTasks[i].ID
			}
			storeIDs := make([]string, len(got.StorageBackups))
			for i := range got.StorageBackups {
				storeIDs[i] = got.StorageBackups[i].ID
			}
			snapIDs := make([]string, len(got.GuestSnapshots))
			for i := range got.GuestSnapshots {
				snapIDs[i] = got.GuestSnapshots[i].ID
			}
			checkIDs("backupTasks", taskIDs, tc.wantTasks)
			checkIDs("storageBackups", storeIDs, tc.wantStore)
			checkIDs("guestSnapshots", snapIDs, tc.wantSnaps)
		})
	}
}

// --- filterReplicationJobs --------------------------------------------------

func TestBranchcov0722PM_FilterReplicationJobs(t *testing.T) {
	jobs := []models.ReplicationJob{
		{ID: "j1", Instance: "pve-a", GuestID: 100, GuestName: "vm-100", Guest: "guest100"},
		{ID: "j2", Instance: "pve-b", GuestID: 200, GuestName: "vm-200", Guest: "guest200"},
	}

	tests := []struct {
		name  string
		query string
		want  []string // expected job IDs, in order
	}{
		{name: "no_filter_returns_copy_of_all", query: "/x", want: []string{"j1", "j2"}},
		{name: "platform_proxmox_valid_token", query: "/x?platform=proxmox", want: []string{"j1", "j2"}},
		{name: "platform_proxmox_pve_valid_token", query: "/x?platform=proxmox-pve", want: []string{"j1", "j2"}},
		{name: "platform_pve_valid_token", query: "/x?platform=pve", want: []string{"j1", "j2"}},
		{name: "platform_unknown_returns_empty", query: "/x?platform=vmware", want: nil},
		{name: "platform_unknown_uppercase_returns_empty", query: "/x?platform=Unknown", want: nil},
		{name: "instance_match", query: "/x?instance=pve-a", want: []string{"j1"}},
		{name: "instance_case_insensitive", query: "/x?instance=PVE-B", want: []string{"j2"}},
		{name: "instance_non_matching", query: "/x?instance=nope", want: nil},
		{name: "guest_match_by_guest_id", query: "/x?guest=100", want: []string{"j1"}},
		{name: "guest_match_by_guest_name", query: "/x?guest=vm-200", want: []string{"j2"}},
		{name: "guest_match_by_guest_field", query: "/x?guest=guest100", want: []string{"j1"}},
		{name: "guest_non_matching", query: "/x?guest=999", want: nil},
		{name: "platform_and_instance_combined", query: "/x?platform=pve&instance=pve-b", want: []string{"j2"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.query, nil)
			got := filterReplicationJobs(jobs, req)

			if len(got) != len(tc.want) {
				t.Fatalf("got %d jobs, want %d (ids=%v)", len(got), len(tc.want), jobIDs(got))
			}
			for i := range got {
				if got[i].ID != tc.want[i] {
					t.Errorf("job[%d]: got %q, want %q", i, got[i].ID, tc.want[i])
				}
			}
		})
	}
}

func jobIDs(jobs []models.ReplicationJob) []string {
	out := make([]string, len(jobs))
	for i := range jobs {
		out[i] = jobs[i].ID
	}
	return out
}

// --- cephDiscoveryTarget ----------------------------------------------------

func TestBranchcov0722PM_CephDiscoveryTarget(t *testing.T) {
	tests := []struct {
		name     string
		resource unified.Resource
		wantNil  bool
		want     *unified.DiscoveryTarget
	}{
		{
			name:     "non_ceph_resource_nil_ceph_returns_nil",
			resource: unified.Resource{ID: "r1", Name: "host1"},
			wantNil:  true,
		},
		{
			name:     "ceph_nil_and_no_name_or_id_returns_nil",
			resource: unified.Resource{Ceph: &unified.CephMeta{}},
			wantNil:  true,
		},
		{
			name: "populated_ceph_uses_fsid_as_agent_id",
			resource: unified.Resource{
				ID:   "ceph-cluster-1",
				Name: "ceph-prod",
				Ceph: &unified.CephMeta{FSID: "fsid-aaa", HealthStatus: "HEALTH_OK"},
			},
			want: &unified.DiscoveryTarget{
				ResourceType: "ceph",
				AgentID:      "fsid-aaa",
				ResourceID:   "ceph-cluster-1",
				Hostname:     "ceph-prod",
			},
		},
		{
			name: "ceph_without_fsid_falls_back_to_name",
			resource: unified.Resource{
				ID:   "ceph-cluster-2",
				Name: "ceph-staging",
				Ceph: &unified.CephMeta{HealthStatus: "HEALTH_WARN"},
			},
			want: &unified.DiscoveryTarget{
				ResourceType: "ceph",
				AgentID:      "ceph-staging",
				ResourceID:   "ceph-cluster-2",
				Hostname:     "ceph-staging",
			},
		},
		{
			name: "ceph_without_fsid_or_name_falls_back_to_id",
			resource: unified.Resource{
				ID:   "res-id-only",
				Ceph: &unified.CephMeta{HealthStatus: "HEALTH_ERR"},
			},
			want: &unified.DiscoveryTarget{
				ResourceType: "ceph",
				AgentID:      "res-id-only",
				ResourceID:   "res-id-only",
				Hostname:     "", // resource.Name is empty
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cephDiscoveryTarget(tc.resource)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil target, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected non-nil target, got nil")
			}
			if got.ResourceType != tc.want.ResourceType {
				t.Errorf("ResourceType: got %q, want %q", got.ResourceType, tc.want.ResourceType)
			}
			if got.AgentID != tc.want.AgentID {
				t.Errorf("AgentID: got %q, want %q", got.AgentID, tc.want.AgentID)
			}
			if got.ResourceID != tc.want.ResourceID {
				t.Errorf("ResourceID: got %q, want %q", got.ResourceID, tc.want.ResourceID)
			}
			if got.Hostname != tc.want.Hostname {
				t.Errorf("Hostname: got %q, want %q", got.Hostname, tc.want.Hostname)
			}
		})
	}
}

// --- capMetricPointSeriesByIndex --------------------------------------------

func TestBranchcov0722PM_CapMetricPointSeriesByIndex(t *testing.T) {
	// 10 distinct points: timestamp == index, value == index.
	points := make([]MetricPoint, 10)
	for i := range points {
		points[i] = MetricPoint{Timestamp: int64(i), Value: float64(i)}
	}

	tests := []struct {
		name      string
		points    []MetricPoint
		maxPoints int
		wantLen   int
		wantFirst int64 // expected first result timestamp
		wantLast  int64 // expected last result timestamp
	}{
		{
			name:      "len_le_maxpoints_returned_unchanged",
			points:    points,
			maxPoints: 15,
			wantLen:   10,
			wantFirst: 0,
			wantLast:  9,
		},
		{
			name:      "maxpoints_le_zero_returned_unchanged",
			points:    points,
			maxPoints: 0,
			wantLen:   10,
			wantFirst: 0,
			wantLast:  9,
		},
		{
			name:      "negative_maxpoints_returned_unchanged",
			points:    points,
			maxPoints: -3,
			wantLen:   10,
			wantFirst: 0,
			wantLast:  9,
		},
		{
			name:      "empty_input_returned_unchanged",
			points:    nil,
			maxPoints: 5,
			wantLen:   0,
			wantFirst: 0,
			wantLast:  0,
		},
		{
			name:      "len_gt_maxpoints_samples_keeping_first_and_last",
			points:    points,
			maxPoints: 3,
			wantLen:   3,
			wantFirst: 0, // points[0]
			wantLast:  9, // points[9]
		},
		{
			name:      "maxpoints_one_returns_only_last",
			points:    points,
			maxPoints: 1,
			wantLen:   1,
			wantFirst: 9, // only element is points[len-1]
			wantLast:  9,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := capMetricPointSeriesByIndex(tc.points, tc.maxPoints)
			if len(got) != tc.wantLen {
				t.Fatalf("len: got %d, want %d", len(got), tc.wantLen)
			}
			if tc.wantLen == 0 {
				return
			}
			if got[0].Timestamp != tc.wantFirst {
				t.Errorf("first timestamp: got %d, want %d", got[0].Timestamp, tc.wantFirst)
			}
			if got[len(got)-1].Timestamp != tc.wantLast {
				t.Errorf("last timestamp: got %d, want %d", got[len(got)-1].Timestamp, tc.wantLast)
			}
			// For the maxPoints=3 case, assert the exact sampled middle point
			// (index 5) so the sampling math — not just the endpoints — is verified.
			if tc.name == "len_gt_maxpoints_samples_keeping_first_and_last" {
				if got[1].Timestamp != 5 || got[1].Value != 5 {
					t.Errorf("middle sampled point: got {ts=%d val=%v}, want {ts=5 val=5}", got[1].Timestamp, got[1].Value)
				}
			}
			if tc.name == "maxpoints_one_returns_only_last" {
				if got[0].Value != 9 {
					t.Errorf("single point value: got %v, want 9", got[0].Value)
				}
			}
		})
	}
}

// --- deriveInstallJobStatus -------------------------------------------------

func TestBranchcov0722PM_DeriveInstallJobStatus(t *testing.T) {
	tests := []struct {
		name    string
		targets []deploy.TargetStatus
		want    deploy.JobStatus
	}{
		{name: "empty_targets_succeeded", targets: nil, want: deploy.JobSucceeded},
		{name: "all_succeeded", targets: []deploy.TargetStatus{deploy.TargetSucceeded, deploy.TargetSucceeded}, want: deploy.JobSucceeded},
		{name: "all_verifying_counts_as_succeeded", targets: []deploy.TargetStatus{deploy.TargetVerifying}, want: deploy.JobSucceeded},
		{name: "all_enrolling_counts_as_succeeded", targets: []deploy.TargetStatus{deploy.TargetEnrolling}, want: deploy.JobSucceeded},
		{name: "all_failed_permanent", targets: []deploy.TargetStatus{deploy.TargetFailedPermanent, deploy.TargetFailedPermanent}, want: deploy.JobFailed},
		{name: "all_failed_retryable", targets: []deploy.TargetStatus{deploy.TargetFailedRetryable}, want: deploy.JobFailed},
		{name: "skipped_and_canceled_count_as_failed", targets: []deploy.TargetStatus{deploy.TargetSkippedAgent, deploy.TargetCanceled}, want: deploy.JobFailed},
		{name: "pending_default_branch_counts_as_failed", targets: []deploy.TargetStatus{deploy.TargetPending, deploy.TargetPending}, want: deploy.JobFailed},
		{name: "installing_default_branch_counts_as_failed", targets: []deploy.TargetStatus{deploy.TargetInstalling}, want: deploy.JobFailed},
		{name: "mixed_succeeded_and_failed_partial", targets: []deploy.TargetStatus{deploy.TargetSucceeded, deploy.TargetFailedPermanent}, want: deploy.JobPartialSuccess},
		{name: "succeeded_plus_pending_partial", targets: []deploy.TargetStatus{deploy.TargetSucceeded, deploy.TargetPending}, want: deploy.JobPartialSuccess},
		{name: "enrolling_plus_failed_partial", targets: []deploy.TargetStatus{deploy.TargetEnrolling, deploy.TargetFailedRetryable}, want: deploy.JobPartialSuccess},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			targets := make([]deploy.Target, len(tc.targets))
			for i, s := range tc.targets {
				targets[i] = deploy.Target{ID: "tgt", Status: s}
			}
			got := deriveInstallJobStatus(targets)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
