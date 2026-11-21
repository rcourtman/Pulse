package recovery

import (
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
)

// BackupTaskRunningMaxAge is how long a PVE task reading supports Running
// in a backup health rollup. Retaining the row does not renew that evidence.
const BackupTaskRunningMaxAge = 5 * time.Minute

// BuildBackupHealth summarizes execution evidence for an authoritative current
// workload roster. It never promotes restore artifacts into execution outcomes,
// guesses across ambiguous source identities, or treats absence as success.
func BuildBackupHealth(workloads []model.BackupHealthWorkload, points []RecoveryPoint, coverage model.BackupHistoryCoverage, from, to time.Time, location *time.Location) model.BackupHealthSummary {
	if location == nil {
		location = time.UTC
	}
	result := model.BackupHealthSummary{
		Start:        from,
		End:          to,
		Timezone:     location.String(),
		HistoryState: "complete",
		Workloads:    make([]model.BackupHealthWorkloadResult, 0, len(workloads)),
		Days:         []model.BackupHealthDay{},
		Failures:     []model.BackupHealthFailure{},
	}
	byID := map[string]int{}
	byNative := map[backupHealthNativeKey]backupHealthNativeMatch{}
	instances := indexBackupHealthCoverage(workloads, coverage, from, to)
	for _, workload := range workloads {
		index := len(result.Workloads)
		instance := instances[workload.Instance]
		state, reason := instance.state, instance.reason
		if workload.Instance == "" || workload.VMID <= 0 {
			state, reason = "unsupported", "Backup task outcomes are not available for this workload's provider."
		}
		result.Workloads = append(result.Workloads, model.BackupHealthWorkloadResult{BackupHealthWorkload: workload, HistoryState: state, HistoryReason: reason})
		byID[workload.ID] = index
		key := backupHealthNativeKey{instance: workload.Instance, vmid: workload.VMID}
		match, exists := byNative[key]
		if exists {
			match.ambiguous = true
		} else {
			match.index = index
		}
		byNative[key] = match
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
		for _, instance := range instances {
			s, _ := backupHistoryState(instance.coverage, start, end)
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
	for pointIndex, point := range points {
		if point.Kind != KindOther || point.Provider != ProviderProxmoxPVE || !strings.HasPrefix(point.ID, "pve-task:") || seen[point.ID] {
			continue
		}
		seen[point.ID] = true
		if point.StartedAt == nil || !point.StartedAt.Before(to) {
			continue
		}
		instance := recoveryDetailString(point, "instance")
		instanceCoverage := instances[instance]
		if instanceCoverage == nil {
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
				instanceCoverage.lastUnattributedTask = pointIndex + 1
			}
			continue
		}
		index, matched := byID[point.SubjectResourceID]
		if matched && result.Workloads[index].Instance != instance {
			continue
		}
		if !matched {
			key := backupHealthNativeKey{instance: instance, vmid: recoveryDetailInt(point, "vmid")}
			match, found := byNative[key]
			if !found || match.ambiguous {
				if match.ambiguous && (point.CompletedAt == nil || !point.CompletedAt.Before(from)) {
					match.lastAmbiguousTask = pointIndex + 1
					byNative[key] = match
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
			index = match.index
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
			if backupTaskRunningAt(point, to) {
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
	for i := range result.Workloads {
		row := &result.Workloads[i]
		// Apply attribution gaps once per row. The last qualifying point in
		// input order still supplies the reason; zero means no such point.
		unattributed := instances[row.Instance].lastUnattributedTask
		ambiguous := byNative[backupHealthNativeKey{instance: row.Instance, vmid: row.VMID}].lastAmbiguousTask
		switch {
		case ambiguous > unattributed:
			row.HistoryState = "partial"
			row.HistoryReason = "Some backup task identities could not be uniquely matched to workloads."
		case unattributed > 0:
			row.HistoryState = "partial"
			row.HistoryReason = "Some multi-guest backup jobs could not be fully attributed to workloads."
		}
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

func backupTaskRunningAt(point RecoveryPoint, cutoff time.Time) bool {
	if point.Outcome != OutcomeRunning || point.Evidence == nil {
		return false
	}
	if validUntil := point.Evidence.ValidUntil; validUntil != nil {
		return !cutoff.After(*validUntil)
	}
	// Older task rows (and callers without an observation clock) have no
	// explicit expiry. Preserve their existing ingestion-time cutoff.
	return !point.Evidence.IngestedAt.Before(cutoff.Add(-BackupTaskRunningMaxAge))
}

type backupHealthInstanceCoverage struct {
	coverage             model.BackupHistoryCoverage
	state                string
	reason               string
	lastUnattributedTask int
}

type backupHealthNativeKey struct {
	instance string
	vmid     int
}

type backupHealthNativeMatch struct {
	index             int
	ambiguous         bool
	lastAmbiguousTask int
}

// A report can scope a few workloads out of an entire fleet. Partition just
// the in-scope PVE coverage once and evaluate the full period once per
// connection, not once per guest. Keep the input order within each connection
// so the first incomplete source and its interval sequence still win.
func indexBackupHealthCoverage(workloads []model.BackupHealthWorkload, coverage model.BackupHistoryCoverage, from, to time.Time) map[string]*backupHealthInstanceCoverage {
	if len(workloads) == 0 {
		return nil
	}
	instances := make(map[string]*backupHealthInstanceCoverage)
	for _, workload := range workloads {
		if instances[workload.Instance] == nil {
			instances[workload.Instance] = &backupHealthInstanceCoverage{}
		}
	}
	for _, source := range coverage.Sources {
		if instance := instances[source.Instance]; instance != nil && source.Provider == ProviderProxmoxPVE {
			instance.coverage.Sources = append(instance.coverage.Sources, source)
		}
	}
	for _, interval := range coverage.Intervals {
		if instance := instances[interval.Instance]; instance != nil && interval.Provider == ProviderProxmoxPVE {
			instance.coverage.Intervals = append(instance.coverage.Intervals, interval)
		}
	}
	for _, instance := range instances {
		instance.state, instance.reason = backupHistoryState(instance.coverage, from, to)
	}
	return instances
}

// coverage contains only PVE rows for a single connection. Each node still
// has to cover the requested window with its own intervals.
func backupHistoryState(coverage model.BackupHistoryCoverage, from, to time.Time) (string, string) {
	if len(coverage.Sources) == 0 {
		return "not_observed", "No durable backup task collection has been recorded for this source."
	}
	for _, source := range coverage.Sources {
		cursor := from
		for _, interval := range coverage.Intervals {
			if interval.Node != source.Node || interval.End.Before(cursor) {
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
	return "complete", "Available task history was collected throughout this period. Provider retention remains external to Pulse."
}
