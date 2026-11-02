package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	proxmoxrecoverymapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

// BackupJobsResponse is the payload for `GET /api/backups/jobs`: PVE backup
// job outcomes for the last 24 hours and for each of the last 14 local days,
// summarized by recovery.BuildBackupHealth from retained task history (the
// same authority as the scheduled backup health report). The live
// /api/backups/pve task list only spans the latest collection window, so it
// cannot answer "is this normal compared with the last two weeks".
type BackupJobsResponse struct {
	Data BackupJobsPayload `json:"data"`
}

type BackupJobsPayload struct {
	// complete, partial, not_observed or unavailable, over the 14 days.
	HistoryState string           `json:"historyState"`
	Timezone     string           `json:"timezone"`
	Last24h      BackupJobsWindow `json:"last24h"`
	Days         []BackupJobsDay  `json:"days"`
}

type BackupJobsCounts struct {
	Success int `json:"success"`
	Failed  int `json:"failed"`
	Warning int `json:"warning"`
	Running int `json:"running"`
	Unknown int `json:"unknown"`
}

type BackupJobsWindow struct {
	Start        time.Time           `json:"start"`
	End          time.Time           `json:"end"`
	HistoryState string              `json:"historyState"`
	Counts       BackupJobsCounts    `json:"counts"`
	Failures     []BackupJobsFailure `json:"failures"`
}

// BackupJobsFailure names the workload only. Provider error text stays in
// the task drill-down, outside this summary (BACKUP_TASK_TIMELINE_CONTRACT.md).
type BackupJobsFailure struct {
	WorkloadID   string    `json:"workloadId"`
	WorkloadName string    `json:"workloadName"`
	Instance     string    `json:"instance"`
	At           time.Time `json:"at"`
	Outcome      string    `json:"outcome"`
}

type BackupJobsDay struct {
	Date         string           `json:"date"`
	HistoryState string           `json:"historyState"`
	Counts       BackupJobsCounts `json:"counts"`
}

const backupJobsHistoryDays = 14

func (r *Router) handleBackupJobs(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed",
			"Only GET method is allowed", nil)
		return
	}
	monitor := r.getTenantMonitor(req.Context())
	if monitor == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "no_monitor",
			"Monitor not available", nil)
		return
	}

	// Day boundaries follow the viewer's IANA zone; anything unreadable is UTC.
	location := time.UTC
	if name := strings.TrimSpace(req.URL.Query().Get("timezone")); name != "" {
		if loaded, err := time.LoadLocation(name); err == nil {
			location = loaded
		}
	}
	now := time.Now().UTC()
	from := backupJobsHistoryStart(now, location)

	resources, _ := monitor.UnifiedResourceSnapshot()
	all, err := backupHealthWorkloads(reportingEnrichmentSnapshot{Resources: resources}, config.ReportScheduleScope{})
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "workloads_unavailable",
			"Backup job workloads are unavailable", nil)
		return
	}
	workloads := pveBackupJobWorkloads(all)

	var points []recovery.RecoveryPoint
	var coverage model.BackupHistoryCoverage
	available := true
	if mock.IsMockEnabled() {
		points, coverage = mockBackupJobHistory(monitor.PVEBackupsSnapshot().BackupTasks, workloads, from, now)
	} else if r.recoveryHandlers == nil {
		available = false
	} else if store, storeErr := r.recoveryHandlers.storeForOrg(GetOrgID(req.Context())); storeErr != nil {
		available = false
		log.Warn().Err(storeErr).Msg("Backup job history store unavailable")
	} else if points, coverage, err = store.BackupHealthHistory(req.Context(), from, now); err != nil {
		available = false
		log.Warn().Err(err).Msg("Backup job history unavailable")
	}

	payload := buildBackupJobsPayload(workloads, points, coverage, available, from, now, location)

	if err := utils.WriteJSONResponse(w, BackupJobsResponse{Data: payload}); err != nil {
		log.Error().Err(err).Msg("Failed to encode backup jobs response")
	}
}

// backupJobsHistoryStart is local midnight 13 days before today, so the
// window holds 14 whole local days ending now.
func backupJobsHistoryStart(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location).
		AddDate(0, 0, -(backupJobsHistoryDays - 1)).UTC()
}

func buildBackupJobsPayload(
	workloads []model.BackupHealthWorkload,
	points []recovery.RecoveryPoint,
	coverage model.BackupHistoryCoverage,
	available bool,
	from, now time.Time,
	location *time.Location,
) BackupJobsPayload {
	to := backupJobsObservedThrough(workloads, coverage, now)
	lastFrom := now.Add(-24 * time.Hour)
	days := recovery.BuildBackupHealth(workloads, backupJobPointsSince(points, from), coverage, from, to, location)
	last := recovery.BuildBackupHealth(workloads, backupJobPointsSince(points, lastFrom), coverage, lastFrom, to, location)

	payload := BackupJobsPayload{
		HistoryState: days.HistoryState,
		Timezone:     location.String(),
		Last24h: BackupJobsWindow{
			Start:        last.Start,
			End:          last.End,
			HistoryState: last.HistoryState,
			Counts:       backupJobsCounts(last.Counts),
			Failures:     make([]BackupJobsFailure, 0, len(last.Failures)),
		},
		Days: make([]BackupJobsDay, 0, len(days.Days)),
	}
	for _, failure := range last.Failures {
		payload.Last24h.Failures = append(payload.Last24h.Failures, BackupJobsFailure{
			WorkloadID:   failure.WorkloadID,
			WorkloadName: failure.WorkloadName,
			Instance:     failure.Instance,
			At:           failure.At,
			Outcome:      string(failure.Outcome),
		})
	}
	for _, day := range days.Days {
		payload.Days = append(payload.Days, BackupJobsDay{
			Date:         day.Date,
			HistoryState: day.HistoryState,
			Counts:       backupJobsCounts(day.Counts),
		})
	}
	if !available {
		// No successful backups are assumed when history cannot be read.
		payload.HistoryState = "unavailable"
		payload.Last24h.HistoryState = "unavailable"
		for i := range payload.Days {
			payload.Days[i].HistoryState = "unavailable"
		}
	}

	return payload
}

// pveBackupJobWorkloads keeps Proxmox VE guests only. A VM from another
// platform has no PVE instance; BuildBackupHealth would rate its empty
// instance as an unobserved source and mark every day partial.
func pveBackupJobWorkloads(workloads []model.BackupHealthWorkload) []model.BackupHealthWorkload {
	out := make([]model.BackupHealthWorkload, 0, len(workloads))
	for _, workload := range workloads {
		if workload.Instance != "" && workload.VMID > 0 {
			out = append(out, workload)
		}
	}
	return out
}

// Collection is recorded per completed poll, so coverage always ends a poll
// interval before the request. A window that ran to "now" would read partial
// on every healthy install. When every rated instance was observed within
// backupJobsCollectionGrace, the windows end at the oldest of those latest
// observations instead; anything staler keeps "now" and reads partial.
const backupJobsCollectionGrace = 30 * time.Minute

func backupJobsObservedThrough(workloads []model.BackupHealthWorkload, coverage model.BackupHistoryCoverage, now time.Time) time.Time {
	latest := map[string]time.Time{}
	for _, interval := range coverage.Intervals {
		if interval.Provider != recovery.ProviderProxmoxPVE {
			continue
		}
		if interval.End.After(latest[interval.Instance]) {
			latest[interval.Instance] = interval.End
		}
	}
	through := now
	for _, workload := range workloads {
		end, ok := latest[workload.Instance]
		if !ok || now.Sub(end) > backupJobsCollectionGrace {
			return now
		}
		if end.Before(through) {
			through = end
		}
	}
	return through
}

// backupJobPointsSince keeps tasks that started or finished inside a window.
// BuildBackupHealth clamps an unresolved older task to the window start, so
// one abandoned task would otherwise read "unknown" in every window.
func backupJobPointsSince(points []recovery.RecoveryPoint, start time.Time) []recovery.RecoveryPoint {
	out := make([]recovery.RecoveryPoint, 0, len(points))
	for _, point := range points {
		started := point.StartedAt != nil && !point.StartedAt.Before(start)
		finished := point.CompletedAt != nil && !point.CompletedAt.Before(start)
		if started || finished {
			out = append(out, point)
		}
	}
	return out
}

func backupJobsCounts(counts model.BackupOutcomeCounts) BackupJobsCounts {
	return BackupJobsCounts{
		Success: counts.Success,
		Failed:  counts.Failed,
		Warning: counts.Warning,
		Running: counts.Running,
		Unknown: counts.Unknown,
	}
}

// Mock mode has no retained store. Its fixture tasks are the whole history,
// so map them as the monitor would and treat every rated instance as observed
// for the full window, as a polled real instance would be.
func mockBackupJobHistory(tasks []models.BackupTask, workloads []model.BackupHealthWorkload, from, to time.Time) ([]recovery.RecoveryPoint, model.BackupHistoryCoverage) {
	points, err := proxmoxrecoverymapper.FromPVEBackupTasksWithEvidence(tasks, nil, to)
	if err != nil {
		return nil, model.BackupHistoryCoverage{}
	}
	coverage := model.BackupHistoryCoverage{}
	seen := map[string]bool{}
	instances := make([]string, 0, len(tasks)+len(workloads))
	for _, task := range tasks {
		instances = append(instances, task.Instance)
	}
	for _, workload := range workloads {
		instances = append(instances, workload.Instance)
	}
	for _, instance := range instances {
		if instance == "" || seen[instance] {
			continue
		}
		seen[instance] = true
		coverage.Sources = append(coverage.Sources, model.BackupHistorySource{
			Provider: recovery.ProviderProxmoxPVE, Instance: instance,
			FirstObservedAt: from, LastObservedAt: to, Complete: true,
		})
		coverage.Intervals = append(coverage.Intervals, model.BackupHistoryInterval{
			Provider: recovery.ProviderProxmoxPVE, Instance: instance, Start: from, End: to,
		})
	}
	return points, coverage
}

func (r *Router) registerBackupJobsRoutes() {
	r.mux.HandleFunc("/api/backups/jobs", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleBackupJobs)))
}
