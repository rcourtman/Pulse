package recovery

import (
	"strings"
	"testing"
	"time"
)

func TestDeriveIndex(t *testing.T) {
	tests := []struct {
		name     string
		point    RecoveryPoint
		expected PointIndex
	}{
		{
			name:  "empty point",
			point: RecoveryPoint{},
			expected: PointIndex{
				SubjectLabel:  "",
				SubjectType:   "",
				IsWorkload:    false,
				ClusterLabel:  "",
				NodeHostLabel: "",
			},
		},
		{
			name: "Proxmox VM with subject ref",
			point: RecoveryPoint{
				ID:       "snap-123",
				Provider: ProviderProxmoxPVE,
				SubjectRef: &ExternalRef{
					Type:      "proxmox-vm",
					Namespace: "pve-cluster",
					Name:      "web-server",
					ID:        "vm-100",
				},
				Details: map[string]any{
					"node":     "pve1",
					"vmid":     "100",
					"instance": "pve-cluster",
				},
			},
			expected: PointIndex{
				SubjectLabel:    "web-server",
				SubjectType:     "proxmox-vm",
				IsWorkload:      true,
				ClusterLabel:    "pve-cluster",
				NodeHostLabel:   "pve1",
				EntityIDLabel:   "100",
				RepositoryLabel: "",
				DetailsSummary:  "",
			},
		},
		{
			name: "Proxmox LXC with resource ID",
			point: RecoveryPoint{
				ID:                "snap-456",
				Provider:          ProviderProxmoxPVE,
				SubjectResourceID: "unified-resource-123",
				SubjectRef: &ExternalRef{
					Type:  "proxmox-lxc",
					Name:  "database",
					Class: "pve2",
				},
				Details: map[string]any{
					"instance": "prod-cluster",
				},
			},
			expected: PointIndex{
				SubjectLabel:    "database",
				SubjectType:     "proxmox-lxc",
				IsWorkload:      true,
				ClusterLabel:    "prod-cluster",
				NodeHostLabel:   "pve2",
				EntityIDLabel:   "",
				RepositoryLabel: "",
				DetailsSummary:  "",
			},
		},
		{
			name: "Kubernetes with namespace",
			point: RecoveryPoint{
				ID:       "backup-k8s-1",
				Provider: ProviderKubernetes,
				SubjectRef: &ExternalRef{
					Type:      "k8s-pvc",
					Namespace: "production",
					Name:      "data-volume",
					UID:       "abc-123-def",
				},
				Details: map[string]any{
					"k8sClusterName": "prod-eks",
					"namespace":      "production",
				},
			},
			expected: PointIndex{
				SubjectLabel:    "production/data-volume",
				SubjectType:     "k8s-pvc",
				IsWorkload:      true,
				ClusterLabel:    "prod-eks",
				NodeHostLabel:   "",
				NamespaceLabel:  "production",
				EntityIDLabel:   "abc-123-def",
				RepositoryLabel: "",
				DetailsSummary:  "",
			},
		},
		{
			name: "PBS with repository ref",
			point: RecoveryPoint{
				ID:       "pbs-backup-1",
				Provider: ProviderProxmoxPBS,
				SubjectRef: &ExternalRef{
					Type: "proxmox-vm-backup",
					Name: "vm-100",
				},
				RepositoryRef: &ExternalRef{
					Type:  "proxmox-pbs-datastore",
					Name:  "backup-store",
					Class: "local",
				},
				Details: map[string]any{
					"volid": "local:backup/vm-100/2024-01-15_00:00:00",
				},
			},
			expected: PointIndex{
				SubjectLabel:    "vm-100",
				SubjectType:     "proxmox-vm-backup",
				IsWorkload:      true,
				ClusterLabel:    "",
				NodeHostLabel:   "",
				NamespaceLabel:  "local",
				EntityIDLabel:   "",
				RepositoryLabel: "backup-store (local)",
				DetailsSummary:  "local:backup/vm-100/2024-01-15_00:00:00",
			},
		},
		{
			name: "TrueNAS with hostname",
			point: RecoveryPoint{
				ID:       "truenas-snap-1",
				Provider: ProviderTrueNAS,
				SubjectRef: &ExternalRef{
					Type: "truenas-dataset",
					Name: "pool/data",
				},
				Details: map[string]any{
					"hostname":      "truenas-01",
					"snapshot":      "pool/data@auto-2024-01-15",
					"targetDataset": "pool/data",
				},
			},
			expected: PointIndex{
				SubjectLabel:    "pool/data",
				SubjectType:     "truenas-dataset",
				IsWorkload:      false,
				ClusterLabel:    "truenas-01",
				NodeHostLabel:   "truenas-01",
				NamespaceLabel:  "",
				EntityIDLabel:   "",
				RepositoryLabel: "pool/data",
				DetailsSummary:  "pool/data@auto-2024-01-15",
			},
		},
		{
			name: "Docker container",
			point: RecoveryPoint{
				ID:       "docker-snap-1",
				Provider: ProviderDocker,
				SubjectRef: &ExternalRef{
					Type: "docker-container",
					Name: "nginx",
				},
				Details: map[string]any{
					"node": "docker-host-1",
				},
			},
			expected: PointIndex{
				SubjectLabel:    "nginx",
				SubjectType:     "docker-container",
				IsWorkload:      true,
				ClusterLabel:    "",
				NodeHostLabel:   "docker-host-1",
				NamespaceLabel:  "",
				EntityIDLabel:   "",
				RepositoryLabel: "",
				DetailsSummary:  "",
			},
		},
		{
			name: "K8s pod",
			point: RecoveryPoint{
				ID:       "k8s-pod-1",
				Provider: ProviderKubernetes,
				SubjectRef: &ExternalRef{
					Type: "k8s-pod",
					Name: "web-pod-0",
				},
				Details: map[string]any{
					"namespace": "default",
				},
			},
			expected: PointIndex{
				SubjectLabel:    "web-pod-0",
				SubjectType:     "k8s-pod",
				IsWorkload:      true,
				ClusterLabel:    "",
				NodeHostLabel:   "",
				NamespaceLabel:  "default",
				EntityIDLabel:   "",
				RepositoryLabel: "",
				DetailsSummary:  "",
			},
		},
		{
			name: "Velero backup with namespace and name",
			point: RecoveryPoint{
				ID:       "velero-backup-1",
				Provider: ProviderKubernetes,
				SubjectRef: &ExternalRef{
					Type: "velero-backup",
				},
				Details: map[string]any{
					"veleroName": "backup-2024-01-15",
					"veleroNs":   "production",
				},
			},
			expected: PointIndex{
				SubjectLabel:    "velero-backup-1",
				SubjectType:     "velero-backup",
				IsWorkload:      false,
				ClusterLabel:    "",
				NodeHostLabel:   "",
				NamespaceLabel:  "",
				EntityIDLabel:   "",
				RepositoryLabel: "",
				DetailsSummary:  "production/backup-2024-01-15",
			},
		},
		{
			name: "Point with only resource ID",
			point: RecoveryPoint{
				ID:                "point-1",
				Provider:          ProviderHostAgent,
				SubjectResourceID: "unified-resource-abc",
			},
			expected: PointIndex{
				SubjectLabel:    "unified-resource-abc",
				SubjectType:     "",
				IsWorkload:      true,
				ClusterLabel:    "",
				NodeHostLabel:   "",
				NamespaceLabel:  "",
				EntityIDLabel:   "",
				RepositoryLabel: "",
				DetailsSummary:  "",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := DeriveIndex(tc.point)
			if result.SubjectLabel != tc.expected.SubjectLabel {
				t.Errorf("SubjectLabel = %q, want %q", result.SubjectLabel, tc.expected.SubjectLabel)
			}
			if result.SubjectType != tc.expected.SubjectType {
				t.Errorf("SubjectType = %q, want %q", result.SubjectType, tc.expected.SubjectType)
			}
			if result.IsWorkload != tc.expected.IsWorkload {
				t.Errorf("IsWorkload = %v, want %v", result.IsWorkload, tc.expected.IsWorkload)
			}
			if result.ClusterLabel != tc.expected.ClusterLabel {
				t.Errorf("ClusterLabel = %q, want %q", result.ClusterLabel, tc.expected.ClusterLabel)
			}
			if result.NodeHostLabel != tc.expected.NodeHostLabel {
				t.Errorf("NodeHostLabel = %q, want %q", result.NodeHostLabel, tc.expected.NodeHostLabel)
			}
			if result.NamespaceLabel != tc.expected.NamespaceLabel {
				t.Errorf("NamespaceLabel = %q, want %q", result.NamespaceLabel, tc.expected.NamespaceLabel)
			}
			if result.EntityIDLabel != tc.expected.EntityIDLabel {
				t.Errorf("EntityIDLabel = %q, want %q", result.EntityIDLabel, tc.expected.EntityIDLabel)
			}
			if result.RepositoryLabel != tc.expected.RepositoryLabel {
				t.Errorf("RepositoryLabel = %q, want %q", result.RepositoryLabel, tc.expected.RepositoryLabel)
			}
			if result.DetailsSummary != tc.expected.DetailsSummary {
				t.Errorf("DetailsSummary = %q, want %q", result.DetailsSummary, tc.expected.DetailsSummary)
			}
		})
	}
}

func TestPointIndex_ToDisplay(t *testing.T) {
	tests := []struct {
		name     string
		index    PointIndex
		expected *RecoveryPointDisplay
	}{
		{
			name:     "all empty returns nil",
			index:    PointIndex{},
			expected: nil,
		},
		{
			name: "only subject label returns display",
			index: PointIndex{
				SubjectLabel: "test-vm",
			},
			expected: &RecoveryPointDisplay{
				SubjectLabel: "test-vm",
			},
		},
		{
			name: "full index returns full display",
			index: PointIndex{
				SubjectLabel:    "web-server",
				SubjectType:     "proxmox-vm",
				IsWorkload:      true,
				ClusterLabel:    "pve-cluster",
				NodeHostLabel:   "pve1",
				NamespaceLabel:  "",
				EntityIDLabel:   "100",
				RepositoryLabel: "backup-store",
				DetailsSummary:  "snapshot-2024-01-15",
			},
			expected: &RecoveryPointDisplay{
				SubjectLabel:    "web-server",
				SubjectType:     "proxmox-vm",
				IsWorkload:      true,
				ClusterLabel:    "pve-cluster",
				NodeHostLabel:   "pve1",
				NamespaceLabel:  "",
				EntityIDLabel:   "100",
				RepositoryLabel: "backup-store",
				DetailsSummary:  "snapshot-2024-01-15",
			},
		},
		{
			name: "whitespace-only fields treated as empty",
			index: PointIndex{
				SubjectLabel:  "  ",
				SubjectType:   "  ",
				ClusterLabel:  "\t",
				NodeHostLabel: "\n",
				IsWorkload:    false,
			},
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.index.ToDisplay()
			if tc.expected == nil {
				if result != nil {
					t.Errorf("ToDisplay() = %v, want nil", result)
				}
				return
			}
			if result == nil {
				t.Errorf("ToDisplay() = nil, want %v", tc.expected)
				return
			}
			if result.SubjectLabel != tc.expected.SubjectLabel {
				t.Errorf("SubjectLabel = %q, want %q", result.SubjectLabel, tc.expected.SubjectLabel)
			}
			if result.SubjectType != tc.expected.SubjectType {
				t.Errorf("SubjectType = %q, want %q", result.SubjectType, tc.expected.SubjectType)
			}
			if result.IsWorkload != tc.expected.IsWorkload {
				t.Errorf("IsWorkload = %v, want %v", result.IsWorkload, tc.expected.IsWorkload)
			}
			if result.ClusterLabel != tc.expected.ClusterLabel {
				t.Errorf("ClusterLabel = %q, want %q", result.ClusterLabel, tc.expected.ClusterLabel)
			}
			if result.NodeHostLabel != tc.expected.NodeHostLabel {
				t.Errorf("NodeHostLabel = %q, want %q", result.NodeHostLabel, tc.expected.NodeHostLabel)
			}
		})
	}
}

func TestSubjectKeyForPoint(t *testing.T) {
	tests := []struct {
		name     string
		point    RecoveryPoint
		expected string
	}{
		{
			name: "with subject resource ID",
			point: RecoveryPoint{
				Provider:          ProviderProxmoxPVE,
				SubjectResourceID: "unified-resource-123",
			},
			expected: "res:unified-resource-123",
		},
		{
			name: "with external ref only",
			point: RecoveryPoint{
				Provider: ProviderKubernetes,
				SubjectRef: &ExternalRef{
					Type:      "k8s-pvc",
					Namespace: "production",
					Name:      "data-volume",
					UID:       "uid-123",
				},
			},
			expected: "ext:",
		},
		{
			name:     "empty point",
			point:    RecoveryPoint{},
			expected: "",
		},
		{
			name: "subject resource ID with whitespace",
			point: RecoveryPoint{
				Provider:          ProviderHostAgent,
				SubjectResourceID: "  resource-456  ",
			},
			expected: "res:resource-456",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SubjectKeyForPoint(tc.point)
			if result != tc.expected && (result == "" || !strings.HasPrefix(tc.expected, "ext:")) {
				if result != tc.expected {
					t.Errorf("SubjectKeyForPoint() = %q, want %q", result, tc.expected)
				}
			}
		})
	}
}

func TestSubjectKey(t *testing.T) {
	tests := []struct {
		name              string
		provider          Provider
		subjectResourceID string
		subjectRef        *ExternalRef
		expectResourceKey bool
	}{
		{
			name:              "with resource ID",
			provider:          ProviderProxmoxPVE,
			subjectResourceID: "res-123",
			subjectRef:        nil,
			expectResourceKey: true,
		},
		{
			name:              "empty resource ID returns external key",
			provider:          ProviderKubernetes,
			subjectResourceID: "",
			subjectRef: &ExternalRef{
				Type: "k8s-pvc",
			},
			expectResourceKey: false,
		},
		{
			name:              "whitespace-only resource ID",
			provider:          ProviderHostAgent,
			subjectResourceID: "   ",
			subjectRef:        nil,
			expectResourceKey: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SubjectKey(tc.provider, tc.subjectResourceID, tc.subjectRef)
			if tc.expectResourceKey {
				if !strings.HasPrefix(result, "res:") {
					t.Errorf("SubjectKey() = %q, expected resource key", result)
				}
			} else {
				if result != "" && !strings.HasPrefix(result, "ext:") {
					t.Errorf("SubjectKey() = %q, expected external key or empty", result)
				}
			}
		})
	}
}

func TestRepositoryKey(t *testing.T) {
	tests := []struct {
		name                 string
		provider             Provider
		repositoryResourceID string
		repositoryRef        *ExternalRef
		expectResourceKey    bool
	}{
		{
			name:                 "with repository resource ID",
			provider:             ProviderProxmoxPBS,
			repositoryResourceID: "repo-123",
			repositoryRef:        nil,
			expectResourceKey:    true,
		},
		{
			name:                 "nil ref returns external key",
			provider:             ProviderProxmoxPBS,
			repositoryResourceID: "",
			repositoryRef:        nil,
			expectResourceKey:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := RepositoryKey(tc.provider, tc.repositoryResourceID, tc.repositoryRef)
			if tc.expectResourceKey {
				if !strings.HasPrefix(result, "res:") {
					t.Errorf("RepositoryKey() = %q, expected resource key", result)
				}
			}
		})
	}
}

func TestRollupResourceID(t *testing.T) {
	tests := []struct {
		name       string
		subjectKey string
		expected   string
	}{
		{
			name:       "resource key returns ID",
			subjectKey: "res:unified-resource-123",
			expected:   "unified-resource-123",
		},
		{
			name:       "external key returns as-is",
			subjectKey: "ext:abc123def456",
			expected:   "ext:abc123def456",
		},
		{
			name:       "empty string",
			subjectKey: "",
			expected:   "",
		},
		{
			name:       "whitespace trimmed",
			subjectKey: "  res:resource-456  ",
			expected:   "resource-456",
		},
		{
			name:       "malformed key",
			subjectKey: "other-prefix:value",
			expected:   "other-prefix:value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := RollupResourceID(tc.subjectKey)
			if result != tc.expected {
				t.Errorf("RollupResourceID(%q) = %q, want %q", tc.subjectKey, result, tc.expected)
			}
		})
	}
}

func TestBuildRollupsFromPoints(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	twoHoursAgo := now.Add(-2 * time.Hour)
	oneDayAgo := now.Add(-24 * time.Hour)

	tests := []struct {
		name     string
		points   []RecoveryPoint
		expected int
	}{
		{
			name:     "empty points",
			points:   []RecoveryPoint{},
			expected: 0,
		},
		{
			name: "single point with resource ID",
			points: []RecoveryPoint{
				{
					ID:                "p1",
					Provider:          ProviderProxmoxPVE,
					SubjectResourceID: "res-1",
					StartedAt:         &now,
					CompletedAt:       &now,
					Outcome:           OutcomeSuccess,
				},
			},
			expected: 1,
		},
		{
			name: "multiple points same subject",
			points: []RecoveryPoint{
				{
					ID:                "p1",
					Provider:          ProviderProxmoxPVE,
					SubjectResourceID: "res-1",
					StartedAt:         &twoHoursAgo,
					CompletedAt:       &twoHoursAgo,
					Outcome:           OutcomeSuccess,
				},
				{
					ID:                "p2",
					Provider:          ProviderProxmoxPVE,
					SubjectResourceID: "res-1",
					StartedAt:         &now,
					CompletedAt:       &now,
					Outcome:           OutcomeFailed,
				},
			},
			expected: 1,
		},
		{
			name: "different subjects create different rollups",
			points: []RecoveryPoint{
				{
					ID:                "p1",
					Provider:          ProviderProxmoxPVE,
					SubjectResourceID: "res-1",
					StartedAt:         &now,
					CompletedAt:       &now,
					Outcome:           OutcomeSuccess,
				},
				{
					ID:                "p2",
					Provider:          ProviderKubernetes,
					SubjectResourceID: "res-2",
					StartedAt:         &now,
					CompletedAt:       &now,
					Outcome:           OutcomeSuccess,
				},
			},
			expected: 2,
		},
		{
			name: "points without timestamps are skipped",
			points: []RecoveryPoint{
				{
					ID:                "p1",
					Provider:          ProviderProxmoxPVE,
					SubjectResourceID: "res-1",
					Outcome:           OutcomeSuccess,
				},
			},
			expected: 0,
		},
		{
			name: "points without subject key are skipped",
			points: []RecoveryPoint{
				{
					ID:          "p1",
					Provider:    ProviderProxmoxPVE,
					StartedAt:   &now,
					CompletedAt: &now,
					Outcome:     OutcomeSuccess,
				},
			},
			expected: 0,
		},
		{
			name: "tracks last success timestamp",
			points: []RecoveryPoint{
				{
					ID:                "p1",
					Provider:          ProviderProxmoxPVE,
					SubjectResourceID: "res-1",
					StartedAt:         &twoHoursAgo,
					CompletedAt:       &twoHoursAgo,
					Outcome:           OutcomeFailed,
				},
				{
					ID:                "p2",
					Provider:          ProviderProxmoxPVE,
					SubjectResourceID: "res-1",
					StartedAt:         &oneHourAgo,
					CompletedAt:       &oneHourAgo,
					Outcome:           OutcomeSuccess,
				},
			},
			expected: 1,
		},
		{
			name: "external refs group correctly",
			points: []RecoveryPoint{
				{
					ID:       "p1",
					Provider: ProviderKubernetes,
					SubjectRef: &ExternalRef{
						Type:      "k8s-pvc",
						Namespace: "prod",
						Name:      "data",
					},
					StartedAt:   &now,
					CompletedAt: &now,
					Outcome:     OutcomeSuccess,
				},
				{
					ID:       "p2",
					Provider: ProviderKubernetes,
					SubjectRef: &ExternalRef{
						Type:      "k8s-pvc",
						Namespace: "prod",
						Name:      "data",
					},
					StartedAt:   &oneDayAgo,
					CompletedAt: &oneDayAgo,
					Outcome:     OutcomeSuccess,
				},
			},
			expected: 1,
		},
		{
			name: "multiple providers for same subject",
			points: []RecoveryPoint{
				{
					ID:                "p1",
					Provider:          ProviderProxmoxPVE,
					SubjectResourceID: "res-1",
					StartedAt:         &now,
					CompletedAt:       &now,
					Outcome:           OutcomeSuccess,
				},
				{
					ID:                "p2",
					Provider:          ProviderProxmoxPBS,
					SubjectResourceID: "res-1",
					StartedAt:         &now,
					CompletedAt:       &now,
					Outcome:           OutcomeSuccess,
				},
			},
			expected: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := BuildRollupsFromPoints(tc.points)
			if len(result) != tc.expected {
				t.Errorf("BuildRollupsFromPoints() returned %d rollups, want %d", len(result), tc.expected)
			}
		})
	}
}

func TestBuildRollupsFromPoints_LastOutcome(t *testing.T) {
	now := time.Now()
	laterTime := now.Add(1 * time.Second)

	points := []RecoveryPoint{
		{
			ID:                "p1",
			Provider:          ProviderProxmoxPVE,
			SubjectResourceID: "res-1",
			StartedAt:         &now,
			CompletedAt:       &now,
			Outcome:           OutcomeSuccess,
		},
		{
			ID:                "p2",
			Provider:          ProviderProxmoxPVE,
			SubjectResourceID: "res-1",
			StartedAt:         &laterTime,
			CompletedAt:       &laterTime,
			Outcome:           "",
		},
	}

	rollups := BuildRollupsFromPoints(points)
	if len(rollups) != 1 {
		t.Fatalf("expected 1 rollup, got %d", len(rollups))
	}

	if rollups[0].LastOutcome != OutcomeUnknown {
		t.Errorf("LastOutcome = %v, want %v", rollups[0].LastOutcome, OutcomeUnknown)
	}
}

func TestBuildRollupsFromPoints_EmptyOutcomeTreatedAsUnknown(t *testing.T) {
	now := time.Now()

	points := []RecoveryPoint{
		{
			ID:                "p1",
			Provider:          ProviderProxmoxPVE,
			SubjectResourceID: "res-1",
			StartedAt:         &now,
			CompletedAt:       &now,
			Outcome:           "",
		},
	}

	rollups := BuildRollupsFromPoints(points)
	if len(rollups) != 1 {
		t.Fatalf("expected 1 rollup, got %d", len(rollups))
	}

	if rollups[0].LastOutcome != OutcomeUnknown {
		t.Errorf("LastOutcome = %v, want %v", rollups[0].LastOutcome, OutcomeUnknown)
	}
}

func TestBuildRollupsFromPoints_SortOrder(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	points := []RecoveryPoint{
		{
			ID:                "p1",
			Provider:          ProviderProxmoxPVE,
			SubjectResourceID: "res-1",
			StartedAt:         &now,
			CompletedAt:       &now,
			Outcome:           OutcomeSuccess,
		},
		{
			ID:                "p2",
			Provider:          ProviderProxmoxPVE,
			SubjectResourceID: "res-2",
			StartedAt:         &oneHourAgo,
			CompletedAt:       &oneHourAgo,
			Outcome:           OutcomeSuccess,
		},
	}

	rollups := BuildRollupsFromPoints(points)
	if len(rollups) != 2 {
		t.Fatalf("expected 2 rollups, got %d", len(rollups))
	}

	if rollups[0].LastAttemptAt.Before(*rollups[1].LastAttemptAt) {
		t.Error("expected rollups sorted by last attempt descending")
	}
}
