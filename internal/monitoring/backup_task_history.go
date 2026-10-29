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
// marker and an explicit result for every attributed guest before qualifying
// the parent job. An ended parent cannot fill in a missing guest verdict.
func backupGuestLogComplete(job models.BackupTask, lines []proxmox.TaskLogLine, guests []models.BackupTask) bool {
	if job.EndTime.IsZero() || len(lines) == 0 || len(lines) >= 20000 || len(guests) == 0 {
		return false
	}
	for i, line := range lines {
		if line.LineNumber != int64(i+1) {
			return false
		}
	}
	for _, guest := range guests {
		if guest.Status != "OK" && guest.Status != "error" {
			return false
		}
	}
	last := strings.TrimSpace(lines[len(lines)-1].Text)
	return last == "TASK OK" || strings.HasPrefix(last, "TASK ERROR:") || strings.HasPrefix(last, "TASK WARNINGS:")
}

type backupTaskHistoryClient interface {
	GetBackupTaskHistory(context.Context, time.Time) (proxmox.BackupTaskHistory, error)
}

func (m *Monitor) readBackupTaskHistory(ctx context.Context, instance string, client PVEClientInterface) (proxmox.BackupTaskHistory, model.BackupHistoryCollection, error) {
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
		return history, observation, err
	}
	// Older provider adapters remain useful for live monitoring but cannot
	// establish report completeness without collection-quality metadata.
	tasks, err := client.GetBackupTasks(ctx)
	observation.ObservedAt = time.Now().UTC()
	observation.Reason = "collection_quality_unavailable"
	return proxmox.BackupTaskHistory{Tasks: tasks}, observation, err
}

// Unread is not an authoritative empty inventory. Carry forward only the last
// accepted live observations from that connection's failed/offline nodes. New
// accepted pages win by exact ID; retained rows never re-enter durable ingestion
// or acquire a new ObservedAt. A valid empty read, removed node or explicit
// access denial withdraws its old live rows instead of resurrecting them.
func (m *Monitor) retainUnreadBackupTasks(instance string, nodes []proxmox.BackupTaskHistoryNode, tasks []models.BackupTask) []models.BackupTask {
	unread := make(map[string]bool)
	for _, node := range nodes {
		if node.Reason == "collection_failed" || node.Reason == "node_offline" {
			unread[node.Node] = true
		}
	}
	if len(unread) == 0 {
		return tasks
	}
	seen := make(map[string]bool, len(tasks))
	counts := make(map[string]int)
	limits := make(map[string]int)
	for _, task := range tasks {
		seen[task.ID] = true
		counts[task.Node]++
	}
	previous := m.state.GetBackupTasks()
	for _, task := range previous {
		if task.Instance == instance && unread[task.Node] {
			limits[task.Node]++
		}
	}
	// Share the existing 200-provider-task live window. Synthesized guest
	// children can make a previously accepted payload larger; preserve that
	// payload's bound, but never accumulate a fresh window of carried rows on
	// every partially read poll. Accepted new rows always have priority.
	for node := range unread {
		limits[node] = max(200, limits[node], counts[node])
	}
	for _, task := range previous {
		if task.Instance == instance && unread[task.Node] && !seen[task.ID] && counts[task.Node] < limits[task.Node] {
			tasks = append(tasks, task)
			seen[task.ID] = true
			counts[task.Node]++
		}
	}
	return tasks
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
