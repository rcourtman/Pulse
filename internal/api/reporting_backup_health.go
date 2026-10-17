package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
	"github.com/rs/zerolog/log"
)

func backupHealthWorkloads(snapshot reportingEnrichmentSnapshot, scope config.ReportScheduleScope) ([]model.BackupHealthWorkload, error) {
	selected := map[string]bool{}
	for _, resource := range scope.Resources {
		kind := reporting.CanonicalResourceType(resource.ResourceType)
		if kind != "vm" && kind != "system-container" {
			return nil, fmt.Errorf("Backup health reports support VMs and containers")
		}
		selected[resource.ResourceID] = true
	}
	tags := map[string]struct{}{}
	for _, tag := range scope.Tags {
		tags[strings.ToLower(strings.TrimSpace(tag))] = struct{}{}
	}
	all := len(selected) == 0 && len(tags) == 0
	workloads := []model.BackupHealthWorkload{}
	for i := range snapshot.Resources {
		resource := &snapshot.Resources[i]
		if resource.Type != unifiedresources.ResourceTypeVM && resource.Type != unifiedresources.ResourceTypeSystemContainer {
			continue
		}
		if !all && !selected[resource.ID] && !resourceHasAnyReportScheduleTag(*resource, tags) {
			continue
		}
		workload := model.BackupHealthWorkload{ID: resource.ID, Name: resource.Name, Type: string(resource.Type)}
		if workload.Name == "" {
			workload.Name = resource.ID
		}
		if resource.Type == unifiedresources.ResourceTypeVM {
			view := unifiedresources.NewVMView(resource)
			workload.Instance, workload.Node, workload.VMID = view.Instance(), view.Node(), view.VMID()
		} else {
			view := unifiedresources.NewContainerView(resource)
			workload.Instance, workload.Node, workload.VMID = view.Instance(), view.Node(), view.VMID()
		}
		workloads = append(workloads, workload)
	}
	return workloads, nil
}

func (h *ReportingHandlers) generateBackupHealthReport(ctx context.Context, schedule config.ReportSchedule, now time.Time, manual bool) (generatedMultiReport, error) {
	snapshot, ok := h.getReportingEnrichmentSnapshot(ctx, GetOrgID(ctx))
	if !ok {
		return generatedMultiReport{}, fmt.Errorf("Backup report scope is unavailable. Check this organization's infrastructure connections.")
	}
	workloads, err := backupHealthWorkloads(snapshot, schedule.Scope)
	if err != nil {
		return generatedMultiReport{}, err
	}
	windowEnd := now
	if schedule.RunWindowEnd != nil {
		windowEnd = *schedule.RunWindowEnd
	}
	start, end, err := reportScheduleWindow(schedule, windowEnd, manual)
	if err != nil {
		return generatedMultiReport{}, err
	}
	location, err := time.LoadLocation(schedule.Cadence.Timezone)
	if err != nil {
		return generatedMultiReport{}, err
	}
	// Business backup summaries use completed calendar periods. Waiting until
	// the next day's configured delivery time lets normal collection observe
	// the cutoff rather than calling every fresh weekly report incomplete.
	if schedule.Cadence.Type == config.ReportScheduleCadenceWeekly {
		localEnd := end.In(location)
		end = time.Date(localEnd.Year(), localEnd.Month(), localEnd.Day(), 0, 0, 0, 0, location).UTC()
		start = end.In(location).AddDate(0, 0, -7).UTC()
	}
	var points []recovery.RecoveryPoint
	var coverage model.BackupHistoryCoverage
	historyAvailable := h.recoveryManager != nil
	if historyAvailable {
		store, readErr := h.recoveryManager.StoreForOrg(GetOrgID(ctx))
		if readErr == nil {
			points, coverage, readErr = store.BackupHealthHistory(ctx, start, end)
		}
		if readErr != nil {
			historyAvailable = false
			log.Warn().Err(readErr).Msg("Backup report history unavailable")
		}
	}
	summary := recovery.BuildBackupHealth(workloads, points, coverage, start, end, location)
	notice := "Counts describe recorded attempts only. Incomplete coverage and workloads without attempts need review."
	if !historyAvailable {
		summary.HistoryState = "unavailable"
		for i := range summary.Workloads {
			summary.Workloads[i].HistoryState = "unavailable"
			summary.Workloads[i].HistoryReason = "Pulse could not read retained backup execution history."
		}
		for i := range summary.Days {
			summary.Days[i].HistoryState = "unavailable"
		}
		notice = "Pulse could not read retained backup execution history. Backup outcomes are unavailable; no successful backups are assumed."
	} else if summary.HistoryState == "complete" {
		notice = "Available task history was observed throughout this reporting period. Workloads without recorded attempts still need review."
	}
	data := reporting.BackupHealthData{Title: schedule.Name, GeneratedAt: now, Brand: h.resolveReportBranding(ctx).EffectiveBrand(), Summary: summary, HistoryNotice: notice}
	if schedule.Format != "pdf" && schedule.Format != "csv" {
		return generatedMultiReport{}, fmt.Errorf("Backup health report format must be PDF or CSV")
	}
	generator := resolveBackupHealthReportGenerator()
	if generator == nil {
		return generatedMultiReport{}, fmt.Errorf("Backup health report rendering is unavailable. Update to a Pulse Pro runtime with backup health reporting.")
	}
	output, contentType, err := generator(data, reporting.ReportFormat(schedule.Format))
	if err != nil {
		return generatedMultiReport{}, err
	}
	message := "Backup data is incomplete. The report identifies missing evidence."
	switch summary.HistoryState {
	case "complete":
		message = fmt.Sprintf("Backup task history observed throughout the period; %d workloads have no recorded attempts.", summary.MissingWorkloads)
	case "not_observed":
		message = "No backup task observation history is available for this scope."
	case "unavailable":
		message = "Backup history could not be read. The report shows unavailable outcomes."
	}
	return generatedMultiReport{Data: output, ContentType: contentType, Format: reporting.ReportFormat(schedule.Format), Filename: fmt.Sprintf("backup-health-%s.%s", now.UTC().Format("20060102-150405"), schedule.Format), Start: start, End: end, ResourceCount: len(workloads), DataState: summary.HistoryState, DataMessage: message}, nil
}
