package kubernetesmapper

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
)

func TestFromKubernetesRecoveryReport_NilReport(t *testing.T) {
	cluster := agentsk8s.ClusterInfo{ID: "cluster-1", Name: "prod-eks"}
	result := FromKubernetesRecoveryReport(cluster, nil)
	if result != nil {
		t.Errorf("FromKubernetesRecoveryReport(nil) = %v, want nil", result)
	}
}

func TestFromKubernetesRecoveryReport_EmptyReport(t *testing.T) {
	cluster := agentsk8s.ClusterInfo{ID: "cluster-1", Name: "prod-eks"}
	report := &agentsk8s.RecoveryReport{}

	result := FromKubernetesRecoveryReport(cluster, report)

	if len(result) != 0 {
		t.Errorf("expected 0 points, got %d", len(result))
	}
}

func TestFromKubernetesRecoveryReport_VolumeSnapshot(t *testing.T) {
	cluster := agentsk8s.ClusterInfo{ID: "cluster-1", Name: "prod-eks"}

	ready := true
	creationTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	snapshot := agentsk8s.VolumeSnapshot{
		UID:              "snap-uid-123",
		Name:             "my-snapshot",
		Namespace:        "default",
		SourcePVC:        "my-pvc",
		SourcePVCUID:     "pvc-uid-456",
		SnapshotClass:    "csi-hostpath-sc",
		ReadyToUse:       &ready,
		RestoreSizeBytes: ptrInt64(1024),
		CreationTime:     &creationTime,
		CompletionTime:   &creationTime,
	}

	report := &agentsk8s.RecoveryReport{
		VolumeSnapshots: []agentsk8s.VolumeSnapshot{snapshot},
	}

	result := FromKubernetesRecoveryReport(cluster, report)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.Provider != recovery.ProviderKubernetes {
		t.Errorf("Provider = %v, want %v", p.Provider, recovery.ProviderKubernetes)
	}
	if p.Kind != recovery.KindSnapshot {
		t.Errorf("Kind = %v, want %v", p.Kind, recovery.KindSnapshot)
	}
	if p.Mode != recovery.ModeSnapshot {
		t.Errorf("Mode = %v, want %v", p.Mode, recovery.ModeSnapshot)
	}
	if p.Outcome != recovery.OutcomeSuccess {
		t.Errorf("Outcome = %v, want %v", p.Outcome, recovery.OutcomeSuccess)
	}
	if p.SubjectRef == nil {
		t.Error("expected SubjectRef to be set")
	} else {
		if p.SubjectRef.Type != "k8s-pvc" {
			t.Errorf("SubjectRef.Type = %q, want 'k8s-pvc'", p.SubjectRef.Type)
		}
		if p.SubjectRef.Name != "my-pvc" {
			t.Errorf("SubjectRef.Name = %q, want 'my-pvc'", p.SubjectRef.Name)
		}
	}
}

func TestFromKubernetesRecoveryReport_VeleroBackup(t *testing.T) {
	cluster := agentsk8s.ClusterInfo{ID: "cluster-1", Name: "prod-eks"}

	startedAt := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	completedAt := time.Date(2024, 1, 15, 10, 5, 0, 0, time.UTC)
	backup := agentsk8s.VeleroBackup{
		UID:             "backup-uid-123",
		Name:            "my-backup",
		Namespace:       "velero",
		Phase:           "Completed",
		StartedAt:       &startedAt,
		CompletedAt:     &completedAt,
		StorageLocation: "default",
	}

	report := &agentsk8s.RecoveryReport{
		VeleroBackups: []agentsk8s.VeleroBackup{backup},
	}

	result := FromKubernetesRecoveryReport(cluster, report)

	if len(result) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result))
	}

	p := result[0]
	if p.Provider != recovery.ProviderKubernetes {
		t.Errorf("Provider = %v, want %v", p.Provider, recovery.ProviderKubernetes)
	}
	if p.Kind != recovery.KindBackup {
		t.Errorf("Kind = %v, want %v", p.Kind, recovery.KindBackup)
	}
	if p.Mode != recovery.ModeRemote {
		t.Errorf("Mode = %v, want %v", p.Mode, recovery.ModeRemote)
	}
	if p.Outcome != recovery.OutcomeSuccess {
		t.Errorf("Outcome = %v, want %v", p.Outcome, recovery.OutcomeSuccess)
	}
	if p.SubjectRef == nil {
		t.Error("expected SubjectRef to be set")
	}
}

func TestFromKubernetesRecoveryReport_MultipleSnapshotsAndBackups(t *testing.T) {
	cluster := agentsk8s.ClusterInfo{ID: "cluster-1", Name: "prod-eks"}

	// Create multiple snapshots
	snap1Ready := true
	snap1Time := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	snapshots := []agentsk8s.VolumeSnapshot{
		{
			UID:          "snap-1",
			Name:         "snap-1",
			Namespace:    "default",
			ReadyToUse:   &snap1Ready,
			CreationTime: &snap1Time,
		},
		{
			UID:          "snap-2",
			Name:         "snap-2",
			Namespace:    "production",
			ReadyToUse:   &snap1Ready,
			CreationTime: &snap1Time,
		},
	}

	// Create backup
	backupTime := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)
	backups := []agentsk8s.VeleroBackup{
		{
			UID:       "backup-1",
			Name:      "backup-1",
			Namespace: "velero",
			Phase:     "Completed",
			StartedAt: &backupTime,
		},
	}

	report := &agentsk8s.RecoveryReport{
		VolumeSnapshots: snapshots,
		VeleroBackups:   backups,
	}

	result := FromKubernetesRecoveryReport(cluster, report)

	if len(result) != 3 {
		t.Fatalf("expected 3 points, got %d", len(result))
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}
