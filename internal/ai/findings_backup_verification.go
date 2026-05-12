// findings_backup_verification.go emits the "this protected resource hasn't
// had its backup verified recently" signal. The detector reads a
// recovery.ProtectionRollup, decides whether the subject is stale relative
// to its verification window, and builds a backup-category Finding suitable
// for the existing patrol-finding intake path (FindingsStore.Add or
// PatrolService.recordFinding).
//
// Severity is Watch by default and escalates to Warning once the last
// successful backup is itself older than one stale window — that's the
// MVP's "multiple consecutive stale windows" heuristic without depending on
// a separate historical counter.
package ai

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

// BackupVerificationStaleFindingKey is the stable, deduped key for the
// backup_verification_stale signal. The finding store keys the dedup set
// on (resourceID, category, key), so this constant must not drift.
const BackupVerificationStaleFindingKey = "backup_verification_stale"

// backupVerificationStaleWarningMultiplier marks how many staleness windows
// the last successful backup must be older than before the finding escalates
// to Warning severity.
const backupVerificationStaleWarningMultiplier = 2

// BuildBackupVerificationStaleFinding inspects a rollup and returns a Finding
// representing the stale-verification signal, or nil if the rollup is not
// stale. The caller is expected to feed the result into the standard patrol
// finding intake (FindingsStore.Add). Returns nil for nil rollups.
func BuildBackupVerificationStaleFinding(rollup *recovery.ProtectionRollup, now time.Time) *Finding {
	if rollup == nil || rollup.VerifyIntent != recovery.VerifyIntentStale {
		return nil
	}

	resourceID := strings.TrimSpace(rollup.SubjectResourceID)
	if resourceID == "" {
		// Fall back to the rollup key for external refs; the dedup key must
		// be stable across patrol runs so the same subject keeps the same ID.
		resourceID = strings.TrimSpace(rollup.RollupID)
	}
	if resourceID == "" {
		// Without a stable identifier we cannot dedup; refuse to emit rather
		// than risk surfacing duplicate findings.
		return nil
	}

	resourceName := backupVerificationDisplayLabel(rollup, resourceID)
	resourceType := backupVerificationResourceType(rollup)

	severity := FindingSeverityWatch
	consecutiveWindows := backupVerificationConsecutiveStaleWindows(rollup, now)
	if consecutiveWindows >= backupVerificationStaleWarningMultiplier {
		severity = FindingSeverityWarning
	}

	id := generateFindingID(resourceID, string(FindingCategoryBackup), BackupVerificationStaleFindingKey)

	finding := &Finding{
		ID:           id,
		Key:          BackupVerificationStaleFindingKey,
		Severity:     severity,
		Category:     FindingCategoryBackup,
		ResourceID:   resourceID,
		ResourceName: resourceName,
		ResourceType: resourceType,
		Title:        fmt.Sprintf("Backup not verified recently on %s", resourceName),
		Description: fmt.Sprintf(
			"A successful backup exists for %s but no verification has landed in the last %s. Restore confidence drops the longer this goes unchecked.",
			resourceName, backupVerificationWindowDescription(),
		),
		Impact:         "If the most recent backup is corrupt or otherwise unrestorable, you will not learn until a real restore is attempted.",
		Recommendation: "Run a verification job against the latest backup, or schedule a recurring verify so the staleness window does not lapse again.",
		Evidence:       backupVerificationEvidence(rollup, now),
		Source:         "patrol-rule",
	}

	return finding
}

func backupVerificationDisplayLabel(rollup *recovery.ProtectionRollup, fallback string) string {
	if rollup.Display != nil {
		if label := strings.TrimSpace(rollup.Display.SubjectLabel); label != "" {
			return label
		}
	}
	if rollup.SubjectRef != nil {
		if name := strings.TrimSpace(rollup.SubjectRef.Name); name != "" {
			return name
		}
	}
	return fallback
}

func backupVerificationResourceType(rollup *recovery.ProtectionRollup) string {
	if rollup.Display != nil {
		if t := strings.TrimSpace(rollup.Display.SubjectType); t != "" {
			return t
		}
	}
	if rollup.SubjectRef != nil {
		if t := strings.TrimSpace(rollup.SubjectRef.Type); t != "" {
			return t
		}
	}
	return "backup-subject"
}

// backupVerificationConsecutiveStaleWindows estimates how many staleness
// windows the last successful backup has aged through. A return value >= 2
// signals the rollup has been stale across multiple windows and the
// emitter should escalate to Warning.
func backupVerificationConsecutiveStaleWindows(rollup *recovery.ProtectionRollup, now time.Time) int {
	windowMs := recovery.BackupVerifyStaleWindow.Milliseconds()
	if windowMs <= 0 {
		return 0
	}

	// Prefer the last successful backup timestamp as the anchor: the rollup
	// is stale relative to that point. If there is no success on file, fall
	// back to the last verification (which, being outside the window, also
	// counts as historical evidence).
	var anchorMs int64
	if rollup.LastSuccessAt != nil {
		anchorMs = rollup.LastSuccessAt.UTC().UnixMilli()
	} else if rollup.LastVerifiedAt != nil {
		anchorMs = rollup.LastVerifiedAt.UTC().UnixMilli()
	}
	if anchorMs <= 0 {
		return 0
	}

	ageMs := now.UTC().UnixMilli() - anchorMs
	if ageMs <= 0 {
		return 0
	}
	return int(ageMs / windowMs)
}

func backupVerificationWindowDescription() string {
	days := int(recovery.BackupVerifyStaleWindow / (24 * time.Hour))
	if days <= 0 {
		return recovery.BackupVerifyStaleWindow.String()
	}
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

func backupVerificationEvidence(rollup *recovery.ProtectionRollup, now time.Time) string {
	parts := make([]string, 0, 3)
	if rollup.LastSuccessAt != nil {
		parts = append(parts, fmt.Sprintf("last successful backup %s ago", durationCoarse(now.Sub(*rollup.LastSuccessAt))))
	}
	if rollup.LastVerifiedAt != nil {
		parts = append(parts, fmt.Sprintf("last verified %s ago", durationCoarse(now.Sub(*rollup.LastVerifiedAt))))
	} else {
		parts = append(parts, "no recorded verification on this rollup")
	}
	parts = append(parts, fmt.Sprintf("staleness window %s", backupVerificationWindowDescription()))
	return strings.Join(parts, "; ")
}

func durationCoarse(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	days := int(d / (24 * time.Hour))
	if days >= 1 {
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	}
	hours := int(d / time.Hour)
	if hours >= 1 {
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	mins := int(d / time.Minute)
	if mins <= 1 {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", mins)
}
