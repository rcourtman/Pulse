package monitoring

import (
	"context"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

// A short HTTP response alone does not establish that a multi-guest log was
// complete or understood. Require contiguous lines from the start, a terminal
// marker and at least one attributed guest before qualifying the parent job.
func backupGuestLogComplete(job models.BackupTask, lines []proxmox.TaskLogLine, guests []models.BackupTask) bool {
	if job.EndTime.IsZero() || len(lines) == 0 || len(lines) >= 20000 || len(guests) == 0 {
		return false
	}
	for i, line := range lines {
		if line.LineNumber != int64(i+1) {
			return false
		}
	}
	last := strings.TrimSpace(lines[len(lines)-1].Text)
	return last == "TASK OK" || strings.HasPrefix(last, "TASK ERROR:") || strings.HasPrefix(last, "TASK WARNINGS:")
}

type backupTaskHistoryClient interface {
	GetBackupTaskHistory(context.Context, time.Time) (proxmox.BackupTaskHistory, error)
}

func (m *Monitor) readBackupTaskHistory(ctx context.Context, instance string, client PVEClientInterface) ([]proxmox.Task, model.BackupHistoryCollection, error) {
	observation := model.BackupHistoryCollection{Provider: recovery.ProviderProxmoxPVE, Instance: instance}
	m.mu.RLock()
	interval := time.Duration(0)
	if m.config != nil {
		interval = m.config.BackupPollingInterval
	}
	if interval <= 0 && m.config != nil {
		cycles := m.config.BackupPollingCycles
		if cycles <= 0 {
			cycles = 10
		}
		interval = m.config.PVEPollingInterval * time.Duration(cycles)
	}
	m.mu.RUnlock()
	if interval < time.Minute {
		interval = time.Minute
	}
	observation.MaxGap = 2*interval + 3*time.Minute
	if source, ok := client.(backupTaskHistoryClient); ok {
		since := time.Now().UTC().Add(-90 * 24 * time.Hour)
		m.mu.RLock()
		manager, orgID := m.recoveryManager, m.orgID
		m.mu.RUnlock()
		if manager != nil {
			if orgID == "" {
				orgID = "default"
			}
			store, err := manager.StoreForOrg(orgID)
			if err == nil {
				if start, err := store.BackupHistoryCollectionStart(ctx, recovery.ProviderProxmoxPVE, instance, time.Now().UTC()); err == nil {
					since = start
				}
			}
		}
		history, err := source.GetBackupTaskHistory(ctx, since)
		observation.ObservedAt = time.Now().UTC()
		observation.Complete = err == nil && len(history.Nodes) > 0
		for _, node := range history.Nodes {
			if !node.Complete {
				observation.Complete = false
				// A capped backfill can advance the next incremental read, but
				// must never hide another node's failure or missing authority.
				if observation.Reason == "" || observation.Reason == "history_truncated" {
					observation.Reason = node.Reason
				}
			}
		}
		if err != nil {
			observation.Reason = "collection_failed"
		}
		if len(history.Nodes) == 0 && err == nil {
			observation.Reason = "no_nodes_observed"
		}
		return history.Tasks, observation, err
	}
	// Older provider adapters remain useful for live monitoring but cannot
	// establish report completeness without collection-quality metadata.
	tasks, err := client.GetBackupTasks(ctx)
	observation.ObservedAt = time.Now().UTC()
	observation.Reason = "collection_quality_unavailable"
	return tasks, observation, err
}

func (m *Monitor) persistBackupTaskHistory(ctx context.Context, points []recovery.RecoveryPoint, observation model.BackupHistoryCollection) {
	m.mu.RLock()
	manager, orgID := m.recoveryManager, m.orgID
	m.mu.RUnlock()
	if manager == nil {
		return
	}
	if orgID == "" {
		orgID = "default"
	}
	store, err := manager.StoreForOrg(orgID)
	if err == nil {
		err = store.RecordBackupHistory(ctx, points, []model.BackupHistoryCollection{observation})
	}
	if err != nil {
		log.Error().Err(err).Str("instance", observation.Instance).Msg("Failed to persist backup execution history; report coverage remains unconfirmed")
	}
}

func (m *Monitor) qualifyBackupGuestTaskDetails(instance string, jobs []models.BackupTask, upids map[string]string, points []recovery.RecoveryPoint) {
	complete := make(map[string]bool, len(jobs))
	m.mu.RLock()
	for _, job := range jobs {
		entry, ok := m.vzdumpJobTaskCache[instance+"|"+upids[job.ID]]
		complete["pve-task:"+job.ID] = ok && entry.historyComplete
	}
	m.mu.RUnlock()
	for i := range points {
		if qualified, ok := complete[points[i].ID]; ok {
			points[i].Details["guestTaskDetailsComplete"] = qualified
		}
	}
}
