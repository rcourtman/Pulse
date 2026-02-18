package truenasmapper

import (
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/idgen"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
)

const (
	maxTrueNASSnapshotsPerIngest    = 500
	maxTrueNASReplicationsPerIngest = 200
)

func outcomeFromTrueNASReplication(state string, errText string) recovery.Outcome {
	if strings.TrimSpace(errText) != "" {
		return recovery.OutcomeFailed
	}
	normalized := strings.ToLower(strings.TrimSpace(state))
	switch normalized {
	case "success", "succeeded", "ok", "complete", "completed":
		return recovery.OutcomeSuccess
	case "warning", "partial", "partiallyfailed", "partially_failed":
		return recovery.OutcomeWarning
	case "failed", "error", "aborted":
		return recovery.OutcomeFailed
	case "running", "inprogress", "in_progress", "active":
		return recovery.OutcomeRunning
	default:
		return recovery.OutcomeUnknown
	}
}

// FromTrueNASSnapshot maps TrueNAS snapshot data into canonical recovery points.
func FromTrueNASSnapshot(connectionID string, snapshot *truenas.FixtureSnapshot) []recovery.RecoveryPoint {
	if snapshot == nil {
		return nil
	}

	conn := strings.TrimSpace(connectionID)
	host := strings.TrimSpace(snapshot.System.Hostname)
	collectedAt := snapshot.CollectedAt
	if collectedAt.IsZero() {
		collectedAt = time.Now().UTC()
	}

	points := make([]recovery.RecoveryPoint, 0, len(snapshot.ZFSSnapshots)+len(snapshot.ReplicationTasks))

	zfsSnaps := append([]truenas.ZFSSnapshot(nil), snapshot.ZFSSnapshots...)
	sort.SliceStable(zfsSnaps, func(i, j int) bool {
		a := zfsSnaps[i].CreatedAt
		b := zfsSnaps[j].CreatedAt
		if a == nil && b == nil {
			return zfsSnaps[i].FullName > zfsSnaps[j].FullName
		}
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		return a.After(*b)
	})
	if len(zfsSnaps) > maxTrueNASSnapshotsPerIngest {
		zfsSnaps = zfsSnaps[:maxTrueNASSnapshotsPerIngest]
	}

	for _, s := range zfsSnaps {
		full := strings.TrimSpace(s.FullName)
		if full == "" {
			full = strings.TrimSpace(s.ID)
		}
		dataset := strings.TrimSpace(s.Dataset)
		name := strings.TrimSpace(s.Name)
		if dataset == "" || name == "" {
			ds, snap := splitSnapshotName(full)
			if dataset == "" {
				dataset = ds
			}
			if name == "" {
				name = snap
			}
		}

		completedAt := s.CreatedAt
		startedAt := completedAt
		if completedAt == nil {
			t := collectedAt
			completedAt = &t
		}

		id := idgen.StableID(string(recovery.ProviderTrueNAS), conn, "zfs-snapshot", full, idgen.TimeKey(completedAt, startedAt))

		points = append(points, recovery.RecoveryPoint{
			ID:          id,
			Provider:    recovery.ProviderTrueNAS,
			Kind:        recovery.KindSnapshot,
			Mode:        recovery.ModeSnapshot,
			Outcome:     recovery.OutcomeSuccess,
			StartedAt:   startedAt,
			CompletedAt: completedAt,
			SizeBytes:   s.UsedBytes,
			SubjectRef: &recovery.ExternalRef{
				Type: "truenas-dataset",
				Name: dataset,
				ID:   dataset,
			},
			Details: map[string]any{
				"connectionId": conn,
				"hostname":     host,
				"dataset":      dataset,
				"snapshot":     name,
				"fullName":     full,
			},
		})
	}

	repTasks := append([]truenas.ReplicationTask(nil), snapshot.ReplicationTasks...)
	// Prefer tasks with a last run to be first.
	sort.SliceStable(repTasks, func(i, j int) bool {
		a := repTasks[i].LastRun
		b := repTasks[j].LastRun
		if a == nil && b == nil {
			return repTasks[i].Name < repTasks[j].Name
		}
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		return a.After(*b)
	})
	if len(repTasks) > maxTrueNASReplicationsPerIngest {
		repTasks = repTasks[:maxTrueNASReplicationsPerIngest]
	}

	for _, task := range repTasks {
		taskID := strings.TrimSpace(task.ID)
		taskName := strings.TrimSpace(task.Name)
		if taskName == "" {
			taskName = taskID
		}

		completedAt := task.LastRun
		startedAt := completedAt

		outcome := outcomeFromTrueNASReplication(task.LastState, task.LastError)
		if completedAt == nil {
			// If we don't know when it last ran, treat it as unknown rather than "success".
			outcome = recovery.OutcomeUnknown
		}

		id := idgen.StableID(string(recovery.ProviderTrueNAS), conn, "replication-task", taskID, taskName, idgen.TimeKey(completedAt, startedAt))

		subject := &recovery.ExternalRef{
			Type: "truenas-replication-task",
			ID:   taskID,
			Name: taskName,
		}
		if len(task.SourceDatasets) > 0 {
			subject = &recovery.ExternalRef{
				Type: "truenas-dataset",
				Name: task.SourceDatasets[0],
				ID:   task.SourceDatasets[0],
			}
		}

		var repo *recovery.ExternalRef
		if strings.TrimSpace(task.TargetDataset) != "" {
			repo = &recovery.ExternalRef{
				Type: "truenas-dataset",
				Name: strings.TrimSpace(task.TargetDataset),
				ID:   strings.TrimSpace(task.TargetDataset),
			}
		}

		points = append(points, recovery.RecoveryPoint{
			ID:            id,
			Provider:      recovery.ProviderTrueNAS,
			Kind:          recovery.KindBackup,
			Mode:          recovery.ModeRemote,
			Outcome:       outcome,
			StartedAt:     startedAt,
			CompletedAt:   completedAt,
			SubjectRef:    subject,
			RepositoryRef: repo,
			Details: map[string]any{
				"connectionId":   conn,
				"hostname":       host,
				"taskId":         taskID,
				"taskName":       taskName,
				"direction":      strings.TrimSpace(task.Direction),
				"sourceDatasets": append([]string(nil), task.SourceDatasets...),
				"targetDataset":  strings.TrimSpace(task.TargetDataset),
				"lastState":      strings.TrimSpace(task.LastState),
				"lastError":      strings.TrimSpace(task.LastError),
				"lastSnapshot":   strings.TrimSpace(task.LastSnapshot),
			},
		})
	}

	return points
}

func splitSnapshotName(full string) (dataset string, snapshot string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "", ""
	}
	parts := strings.SplitN(full, "@", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}
