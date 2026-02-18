package kubernetesmapper

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/idgen"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
)

func outcomeFromReadyError(ready *bool, errText string) recovery.Outcome {
	if strings.TrimSpace(errText) != "" {
		return recovery.OutcomeFailed
	}
	if ready != nil && *ready {
		return recovery.OutcomeSuccess
	}
	if ready != nil && !*ready {
		return recovery.OutcomeRunning
	}
	return recovery.OutcomeUnknown
}

func mapVeleroPhase(phase string) recovery.Outcome {
	normalized := strings.ToLower(strings.TrimSpace(phase))
	switch normalized {
	case "completed", "complete":
		return recovery.OutcomeSuccess
	case "partiallyfailed", "partially_failed", "partial":
		return recovery.OutcomeWarning
	case "failed", "error":
		return recovery.OutcomeFailed
	case "inprogress", "in_progress", "running":
		return recovery.OutcomeRunning
	default:
		return recovery.OutcomeUnknown
	}
}

// FromKubernetesRecoveryReport maps agent-reported Kubernetes recovery artifacts
// (VolumeSnapshots and/or Velero backups) into canonical recovery points.
func FromKubernetesRecoveryReport(cluster agentsk8s.ClusterInfo, report *agentsk8s.RecoveryReport) []recovery.RecoveryPoint {
	if report == nil {
		return nil
	}

	points := make([]recovery.RecoveryPoint, 0, len(report.VolumeSnapshots)+len(report.VeleroBackups))

	for _, snap := range report.VolumeSnapshots {
		outcome := outcomeFromReadyError(snap.ReadyToUse, snap.Error)
		startedAt := snap.CreationTime
		completedAt := snap.CompletionTime
		// If kube only provides creation time for ready snapshots, treat it as "completed".
		if completedAt == nil && startedAt != nil && outcome == recovery.OutcomeSuccess {
			completedAt = startedAt
		}

		id := idgen.StableID(
			string(recovery.ProviderKubernetes),
			string(recovery.KindSnapshot),
			snap.Namespace,
			snap.Name,
			snap.UID,
			idgen.TimeKey(completedAt, startedAt),
		)

		var details = map[string]any{
			"k8sClusterId":   strings.TrimSpace(cluster.ID),
			"k8sClusterName": strings.TrimSpace(cluster.Name),
			"snapshotUid":    strings.TrimSpace(snap.UID),
			"snapshotName":   strings.TrimSpace(snap.Name),
			"snapshotNs":     strings.TrimSpace(snap.Namespace),
		}
		if strings.TrimSpace(snap.ContentName) != "" {
			details["snapshotContentName"] = strings.TrimSpace(snap.ContentName)
		}

		subject := &recovery.ExternalRef{
			Type:      "k8s-pvc",
			Namespace: strings.TrimSpace(snap.Namespace),
			Name:      strings.TrimSpace(snap.SourcePVC),
			UID:       strings.TrimSpace(snap.SourcePVCUID),
		}
		if subject.Name == "" && subject.UID == "" {
			subject = &recovery.ExternalRef{
				Type:      "k8s-volume-snapshot",
				Namespace: strings.TrimSpace(snap.Namespace),
				Name:      strings.TrimSpace(snap.Name),
				UID:       strings.TrimSpace(snap.UID),
			}
		}

		var repo *recovery.ExternalRef
		if strings.TrimSpace(snap.SnapshotClass) != "" {
			repo = &recovery.ExternalRef{
				Type:  "k8s-volume-snapshot-class",
				Name:  strings.TrimSpace(snap.SnapshotClass),
				Class: strings.TrimSpace(snap.SnapshotClass),
			}
		}

		points = append(points, recovery.RecoveryPoint{
			ID:            id,
			Provider:      recovery.ProviderKubernetes,
			Kind:          recovery.KindSnapshot,
			Mode:          recovery.ModeSnapshot,
			Outcome:       outcome,
			StartedAt:     startedAt,
			CompletedAt:   completedAt,
			SizeBytes:     snap.RestoreSizeBytes,
			SubjectRef:    subject,
			RepositoryRef: repo,
			Details:       details,
		})
	}

	for _, b := range report.VeleroBackups {
		outcome := mapVeleroPhase(b.Phase)
		startedAt := b.StartedAt
		completedAt := b.CompletedAt
		if completedAt == nil && startedAt != nil && outcome == recovery.OutcomeSuccess {
			completedAt = startedAt
		}

		id := idgen.StableID(
			string(recovery.ProviderKubernetes),
			string(recovery.KindBackup),
			b.Namespace,
			b.Name,
			b.UID,
			idgen.TimeKey(completedAt, startedAt),
		)

		details := map[string]any{
			"k8sClusterId":   strings.TrimSpace(cluster.ID),
			"k8sClusterName": strings.TrimSpace(cluster.Name),
			"veleroUid":      strings.TrimSpace(b.UID),
			"veleroName":     strings.TrimSpace(b.Name),
			"veleroNs":       strings.TrimSpace(b.Namespace),
			"phase":          strings.TrimSpace(b.Phase),
		}
		if strings.TrimSpace(b.StorageLocation) != "" {
			details["storageLocation"] = strings.TrimSpace(b.StorageLocation)
		}

		points = append(points, recovery.RecoveryPoint{
			ID:          id,
			Provider:    recovery.ProviderKubernetes,
			Kind:        recovery.KindBackup,
			Mode:        recovery.ModeRemote,
			Outcome:     outcome,
			StartedAt:   startedAt,
			CompletedAt: completedAt,
			SubjectRef: &recovery.ExternalRef{
				Type: "k8s-cluster",
				ID:   strings.TrimSpace(cluster.ID),
				Name: strings.TrimSpace(cluster.Name),
			},
			RepositoryRef: veleroRepoRef(b),
			Details:       details,
		})
	}

	return points
}

func veleroRepoRef(b agentsk8s.VeleroBackup) *recovery.ExternalRef {
	if strings.TrimSpace(b.StorageLocation) == "" {
		return nil
	}
	return &recovery.ExternalRef{
		Type: "velero-backup-storage-location",
		Name: strings.TrimSpace(b.StorageLocation),
	}
}
