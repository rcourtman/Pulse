package recovery

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
)

// BuildBackupHealth summarizes execution evidence for an authoritative current
// workload roster. It never promotes restore artifacts into execution outcomes,
// guesses across ambiguous source identities, or treats absence as success.
func BuildBackupHealth(workloads []model.BackupHealthWorkload, points []RecoveryPoint, coverage model.BackupHistoryCoverage, from, to time.Time, location *time.Location) model.BackupHealthSummary {
	if location == nil {
		location = time.UTC
	}
	result := model.BackupHealthSummary{Start: from, End: to, Timezone: location.String(), HistoryState: "complete", Workloads: []model.BackupHealthWorkloadResult{}, Days: []model.BackupHealthDay{}, Failures: []model.BackupHealthFailure{}}
	byID := map[string]int{}
	byNative := map[string][]int{}
	instances := map[string]bool{}
	for _, workload := range workloads {
		index := len(result.Workloads)
		state, reason := backupHistoryState(coverage, workload.Instance, from, to)
		if workload.Instance == "" || workload.VMID <= 0 {
			state, reason = "unsupported", "Backup task outcomes are not available for this workload's provider."
		}
		result.Workloads = append(result.Workloads, model.BackupHealthWorkloadResult{BackupHealthWorkload: workload, HistoryState: state, HistoryReason: reason})
		byID[workload.ID] = index
		key := fmt.Sprintf("%s\x00%d", workload.Instance, workload.VMID)
		byNative[key] = append(byNative[key], index)
		instances[workload.Instance] = true
	}
	dayIndexes := map[string]int{}
	local := from.In(location)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	for day.Before(to) {
		next := day.AddDate(0, 0, 1)
		start, end := day, next
		if start.Before(from) {
			start = from
		}
		if end.After(to) {
			end = to
		}
		state := "not_observed"
		allComplete, observed := len(workloads) > 0, false
		for instance := range instances {
			s, _ := backupHistoryState(coverage, instance, start, end)
			if s != "not_observed" {
				observed = true
			}
			if s != "complete" {
				allComplete = false
			}
		}
		if allComplete {
			state = "complete"
		} else if observed {
			state = "partial"
		}
		label := day.Format("2006-01-02")
		dayIndexes[label] = len(result.Days)
		result.Days = append(result.Days, model.BackupHealthDay{Date: label, HistoryState: state})
		day = next
	}
	seen := map[string]bool{}
	for _, point := range points {
		if point.Kind != KindOther || point.Provider != ProviderProxmoxPVE || !strings.HasPrefix(point.ID, "pve-task:") || seen[point.ID] {
			continue
		}
		seen[point.ID] = true
		if point.StartedAt == nil || !point.StartedAt.Before(to) {
			continue
		}
		instance := recoveryDetailString(point, "instance")
		if !instances[instance] {
			continue
		}
		// A multi-guest job lacking a complete task log makes client-level
		// attribution incomplete without disclosing another client's job details.
		if recoveryDetailInt(point, "vmid") == 0 && (point.CompletedAt == nil || !point.CompletedAt.Before(from)) {
			qualified, _ := point.Details["guestTaskDetailsComplete"].(bool)
			if !qualified {
				at := to.Add(-time.Nanosecond)
				if point.CompletedAt != nil && point.CompletedAt.Before(to) {
					at = *point.CompletedAt
				}
				if dayIndex, ok := dayIndexes[at.In(location).Format("2006-01-02")]; ok {
					result.Days[dayIndex].HistoryState = "partial"
				}
				for i := range result.Workloads {
					if result.Workloads[i].Instance == instance {
						result.Workloads[i].HistoryState = "partial"
						result.Workloads[i].HistoryReason = "Some multi-guest backup jobs could not be fully attributed to workloads."
					}
				}
			}
			continue
		}
		index, matched := byID[point.SubjectResourceID]
		if matched && result.Workloads[index].Instance != instance {
			continue
		}
		if !matched {
			candidates := byNative[fmt.Sprintf("%s\x00%d", instance, recoveryDetailInt(point, "vmid"))]
			if len(candidates) != 1 {
				if len(candidates) > 1 && (point.CompletedAt == nil || !point.CompletedAt.Before(from)) {
					for _, candidate := range candidates {
						result.Workloads[candidate].HistoryState = "partial"
						result.Workloads[candidate].HistoryReason = "Some backup task identities could not be uniquely matched to workloads."
					}
					at := to.Add(-time.Nanosecond)
					if point.CompletedAt != nil && point.CompletedAt.Before(to) {
						at = *point.CompletedAt
					}
					if dayIndex, ok := dayIndexes[at.In(location).Format("2006-01-02")]; ok {
						result.Days[dayIndex].HistoryState = "partial"
					}
				}
				continue
			}
			index = candidates[0]
		}
		row := &result.Workloads[index]
		if point.Outcome == OutcomeSuccess && point.CompletedAt != nil && point.CompletedAt.Before(to) {
			if row.LastSuccessAt == nil || point.CompletedAt.After(*row.LastSuccessAt) {
				value := *point.CompletedAt
				row.LastSuccessAt = &value
			}
		}
		at := *point.StartedAt
		outcome := point.Outcome
		if point.CompletedAt != nil {
			at = *point.CompletedAt
			if !at.Before(to) {
				outcome = OutcomeRunning
				at = to.Add(-time.Nanosecond)
			}
		} else {
			// An unresolved task from an old poll is not evidence that it is
			// still running at the report cutoff.
			outcome = OutcomeUnknown
			if point.Outcome == OutcomeRunning && point.Evidence != nil && !point.Evidence.IngestedAt.Before(to.Add(-5*time.Minute)) {
				outcome = OutcomeRunning
			}
			if at.Before(from) {
				at = from
			}
		}
		if at.Before(from) || !at.Before(to) {
			continue
		}
		row.Counts.Add(outcome)
		result.Counts.Add(outcome)
		if dayIndex, ok := dayIndexes[at.In(location).Format("2006-01-02")]; ok {
			result.Days[dayIndex].Counts.Add(outcome)
		}
		if outcome == OutcomeFailed || outcome == OutcomeWarning {
			detail := recoveryDetailString(point, "error")
			if detail == "" {
				detail = recoveryDetailString(point, "status")
			}
			result.Failures = append(result.Failures, model.BackupHealthFailure{WorkloadID: row.ID, WorkloadName: row.Name, Instance: row.Instance, TaskID: point.ID, Detail: detail, At: at, Outcome: outcome})
		}
	}
	for _, row := range result.Workloads {
		if row.Counts.Total() == 0 {
			result.MissingWorkloads++
		}
		if row.HistoryState != "complete" {
			result.HistoryState = "partial"
		}
	}
	if len(result.Workloads) == 0 {
		result.HistoryState = "not_observed"
	}
	if result.Counts.Total() == 0 && len(result.Workloads) > 0 {
		allMissing := true
		for _, row := range result.Workloads {
			if row.HistoryState != "not_observed" && row.HistoryState != "unsupported" {
				allMissing = false
			}
		}
		if allMissing {
			result.HistoryState = "not_observed"
		}
	}
	sort.SliceStable(result.Workloads, func(i, j int) bool {
		a, b := result.Workloads[i], result.Workloads[j]
		if a.Instance != b.Instance {
			return a.Instance < b.Instance
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ID < b.ID
	})
	sort.SliceStable(result.Failures, func(i, j int) bool { return result.Failures[i].At.After(result.Failures[j].At) })
	return result
}

func recoveryDetailInt(point RecoveryPoint, key string) int {
	switch value := point.Details[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	}
	return 0
}

func backupHistoryState(coverage model.BackupHistoryCoverage, instance string, from, to time.Time) (string, string) {
	found := false
	for _, source := range coverage.Sources {
		if source.Provider != ProviderProxmoxPVE || source.Instance != instance {
			continue
		}
		found = true
		cursor := from
		for _, interval := range coverage.Intervals {
			if interval.Provider != source.Provider || interval.Instance != source.Instance || interval.Node != source.Node || interval.End.Before(cursor) {
				continue
			}
			if interval.Start.After(cursor) {
				break
			}
			if interval.End.After(cursor) {
				cursor = interval.End
			}
		}
		if cursor.Before(to) {
			reason := "Pulse did not continuously observe this source throughout the reporting period."
			if source.Reason != "" {
				reason += " Latest collection: " + strings.ReplaceAll(source.Reason, "_", " ") + "."
			}
			return "partial", reason
		}
	}
	if !found {
		return "not_observed", "No durable backup task collection has been recorded for this source."
	}
	return "complete", "Available task history was collected throughout this period. Provider retention remains external to Pulse."
}
