package unifiedresources

import (
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

// PBSJobFailedIncidentCode marks a datastore whose sync, verify, prune or
// garbage-collection job failed on its newest run.
const PBSJobFailedIncidentCode = "pbs_job_failed"

// A failed sync leaves the offsite copy stale, a failed verify leaves restores
// unproven, and failed GC or prune let the store fill faster than it should.
// None of them says a guest's primary backup failed, so they belong to the
// datastore and never to guest protection (#1815). Backup jobs are already
// answered per guest by protection posture.
var pbsDatastoreJobFamilyLabels = map[string]string{
	"sync":    "Sync job",
	"verify":  "Verify job",
	"prune":   "Prune job",
	"garbage": "Garbage collection",
}

// pbsDatastoreJobIncidents turns the datastore's failed PBS jobs into incidents
// on that datastore. The judgment is the poller's own (posture "critical" is a
// failed last run); this only picks which runs speak for the store. Storeless
// task history cannot be pinned to one datastore, and a late, warned or
// unknown job is not a failure, so neither raises an incident. The newest run
// of each job speaks for it, matching the Backups page Jobs column.
func pbsDatastoreJobIncidents(instance models.PBSInstance, datastore string) []ResourceIncident {
	store := strings.TrimSpace(datastore)
	if store == "" || len(instance.JobHealthEvidence) == 0 {
		return nil
	}

	newest := make(map[string]models.PBSJobHealthEvidence)
	for _, entry := range instance.JobHealthEvidence {
		family := strings.ToLower(strings.TrimSpace(entry.Family))
		if _, ok := pbsDatastoreJobFamilyLabels[family]; !ok || !entry.Enabled {
			continue
		}
		if strings.TrimSpace(entry.Store) != store {
			continue
		}
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			id = store
		}
		// Job ids are unique only within a family: a sync and a verify job may
		// share one, and neither may hide the other.
		key := family + ":" + id
		if previous, ok := newest[key]; ok && pbsJobRunTime(entry).Before(pbsJobRunTime(previous)) {
			continue
		}
		newest[key] = entry
	}

	keys := make([]string, 0, len(newest))
	for key, entry := range newest {
		if strings.EqualFold(strings.TrimSpace(entry.Posture), "critical") {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)

	incidents := make([]ResourceIncident, 0, len(keys))
	for _, key := range keys {
		entry := newest[key]
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			id = store
		}
		startedAt := pbsJobRunTime(entry)
		if startedAt.IsZero() {
			startedAt = instance.LastSeen
		}
		incidents = append(incidents, ResourceIncident{
			Provider:  "pulse",
			NativeID:  "pbs-datastore:" + store + ":job:" + key,
			Code:      PBSJobFailedIncidentCode,
			Severity:  storagehealth.RiskWarning,
			Source:    string(SourcePBS),
			Summary:   pbsJobFailedSummary(entry, id, store),
			StartedAt: startedAt,
			// A PBS outage drops the datastore from a poll; with confirmed
			// recovery that reads as unknown, so the alert neither clears nor
			// notifies again when PBS returns.
			RecoveryConfirmationsRequired: 2,
		})
	}
	return incidents
}

// The summary is the alert message, so it names the job and, for a sync, the
// remote it copies from. It holds no times: incidents merge on their summary.
func pbsJobFailedSummary(entry models.PBSJobHealthEvidence, id, store string) string {
	family := strings.ToLower(strings.TrimSpace(entry.Family))
	label := pbsDatastoreJobFamilyLabels[family]
	subject := label
	if family != "garbage" {
		subject = label + " " + id
	}
	summary := subject + " failed on datastore " + store
	if remote := strings.TrimSpace(entry.Remote); family == "sync" && remote != "" {
		summary += " (copying from " + remote + ")"
	}
	return summary
}

func pbsJobRunTime(entry models.PBSJobHealthEvidence) time.Time {
	if !entry.Freshness.LastRunEndTime.IsZero() {
		return entry.Freshness.LastRunEndTime
	}
	if entry.LastRunEndtime > 0 {
		return time.Unix(entry.LastRunEndtime, 0).UTC()
	}
	if entry.TaskEndTime > 0 {
		return time.Unix(entry.TaskEndTime, 0).UTC()
	}
	return time.Time{}
}
