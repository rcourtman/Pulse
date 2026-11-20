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
func (m *Monitor) retainUnreadBackupTasks(instance string, nodes []proxmox.BackupTaskHistoryNode, tasks []models.BackupTask, listedJobs map[string]string) []models.BackupTask {
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
		if task.Instance != instance || !unread[task.Node] || seen[task.ID] || counts[task.Node] >= limits[task.Node] {
			continue
		}
		// A listed parent uses this poll's log result or its own log-cache
		// fallback. Inventory carry-forward must not undo an explicit empty
		// log by reintroducing old children of that already accepted job.
		if len(listedJobs) != 0 && task.Type == "vzdump" {
			id := task.ID
			if task.VMID > 0 {
				id = vzdumpParentTaskID(task)
			}
			if _, listed := listedJobs[id]; listed {
				continue
			}
		}
		tasks = append(tasks, task)
		seen[task.ID] = true
		counts[task.Node]++
	}
	return tasks
}

type pveArchiveWriter struct {
	node   string
	parent string
	vmid   int
	start  time.Time
}

// Use the accepted task inventory before applying the display cap. A writer
// missing from an incomplete node read may still own an archive. While its
// parent is still running, a log with no result for this guest says nothing
// about that writer, even if the log can replace live guest details. Overlay
// evidence for the exact task below. This cache affects only storage correlation.
func (m *Monitor) recordPVEArchiveWriters(instance string, nodes []proxmox.BackupTaskHistoryNode, tasks, jobs []models.BackupTask) {
	unreadNodes := make(map[string]bool)
	for _, node := range nodes {
		if !node.Complete && node.Reason != "access_denied" {
			unreadNodes[node.Node] = true
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	writers := make(map[string]pveArchiveWriter)
	previous := m.pveArchiveWriters[instance]
	if len(previous) > 0 {
		jobRunning := make(map[string]bool, len(jobs))
		for _, job := range jobs {
			jobRunning[job.ID] = job.EndTime.IsZero()
		}
		for id, writer := range previous {
			if running, listed := jobRunning[writer.parent]; listed {
				if !running {
					continue
				}
			} else if !unreadNodes[writer.node] {
				continue
			}
			writers[id] = writer
		}
	}
	for _, task := range tasks {
		if task.Instance != instance || task.Type != "vzdump" || task.VMID <= 0 {
			continue
		}
		delete(writers, task.ID)
		status := strings.ToLower(strings.TrimSpace(task.Status))
		if task.EndTime.IsZero() && (status == "" || status == "running") {
			writers[task.ID] = pveArchiveWriter{node: task.Node, parent: vzdumpParentTaskID(task), vmid: task.VMID, start: task.StartTime}
		}
	}
	if len(writers) == 0 {
		delete(m.pveArchiveWriters, instance)
		return
	}
	if m.pveArchiveWriters == nil {
		m.pveArchiveWriters = make(map[string]map[string]pveArchiveWriter)
	}
	m.pveArchiveWriters[instance] = writers
}

// An OK for another task, even if its time matches a single listed file,
// gives no archive identity. Only compare writer scope and the file's origin:
// an unrelated local disk's independently listed archive remains eligible.
func (m *Monitor) pveArchiveWriterStarts(instance string) map[int]map[string]time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	starts := make(map[int]map[string]time.Time)
	for _, writer := range m.pveArchiveWriters[instance] {
		byNode := starts[writer.vmid]
		if byNode == nil {
			byNode = make(map[string]time.Time)
			starts[writer.vmid] = byNode
		}
		if prior, ok := byNode[writer.node]; !ok || writer.start.Before(prior) {
			byNode[writer.node] = writer.start
		}
	}
	return starts
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

// After publication admission, give already accepted task evidence time to
// reach history even when a later log uses up the provider-read budget. Keep
// the original deadline if it leaves more time, but don't extend any provider
// read or let the write outlive the connection/runtime that owns the poll.
func (m *Monitor) persistAcceptedBackupTaskHistory(readCtx context.Context, points []recovery.RecoveryPoint, observation model.BackupHistoryCollection) {
	owner, _ := readCtx.Value(pveBackupPollContextKey{}).(*pveBackupPoll)
	if owner == nil {
		m.persistBackupTaskHistory(readCtx, points, observation)
		return
	}
	const historyWriteGrace = 20 * time.Second
	deadline, _ := readCtx.Deadline()
	if minimum := time.Now().Add(historyWriteGrace); deadline.Before(minimum) {
		deadline = minimum
	}
	writeCtx, cancel := context.WithDeadline(owner.ctx, deadline)
	defer cancel()
	m.persistBackupTaskHistory(writeCtx, points, observation)
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
