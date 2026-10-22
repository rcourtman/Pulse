package alerts

import (
	"math"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Snapshot age and size are independent observations. Optional/malformed data
// cannot establish recovery of a previously observed breach in the other one.
// Held values retain their original observation time and never grow with time.
type snapshotAlertEvidence struct {
	createdAt      time.Time
	ageDays        float64
	ageObservedAt  time.Time
	ageHeld        bool
	sizeBytes      int64
	sizeObservedAt time.Time
	sizeHeld       bool
}

func snapshotMetadataTime(metadata map[string]interface{}, key string) time.Time {
	switch value := metadata[key].(type) {
	case time.Time:
		return value
	case string: // Restored active-alert JSON.
		parsed, _ := time.Parse(time.RFC3339Nano, value)
		return parsed
	default:
		return time.Time{}
	}
}

func snapshotObservationTime(previous *Alert, key string) time.Time {
	observed := snapshotMetadataTime(previous.Metadata, key)
	if observed.IsZero() {
		// Older persisted alerts predate per-metric observation times.
		observed = previous.LastSeen
	}
	// A clock correction can put an already accepted observation ahead of
	// this evaluation. Keep its original time, rather than turning the clock
	// change into recovery or relabelling the old value as a fresh reading.
	return observed
}

func resolveSnapshotAlertEvidence(snapshot models.GuestSnapshot, previous *Alert, now time.Time) snapshotAlertEvidence {
	var evidence snapshotAlertEvidence
	// The ordinary Proxmox mapper represents absent snaptime as Unix epoch,
	// rather than Go's zero time. Neither establishes a snapshot's age.
	ageKnown := snapshot.Time.After(time.Unix(0, 0)) && !snapshot.Time.After(now)
	if ageKnown {
		evidence.createdAt = snapshot.Time
		evidence.ageDays = now.Sub(snapshot.Time).Hours() / 24
		evidence.ageObservedAt = now
	}
	if snapshot.SizeBytes > 0 {
		evidence.sizeBytes = snapshot.SizeBytes
		evidence.sizeObservedAt = now
	}
	if previous == nil {
		return evidence
	}
	oldCreatedAt := snapshotMetadataTime(previous.Metadata, "snapshotCreatedAt")
	if ageKnown && !snapshot.Time.Equal(oldCreatedAt) {
		// A reused snapshot name with a new birth time is a replacement, not
		// permission to transfer the old snapshot's missing size evidence.
		return evidence
	}
	if !ageKnown && oldCreatedAt.After(time.Unix(0, 0)) {
		age, ok := metadataFloatValue(previous.Metadata, "snapshotAgeDays")
		observed := snapshotObservationTime(previous, "snapshotAgeObservedAt")
		if ok && age >= 0 && !math.IsNaN(age) && !math.IsInf(age, 0) && !observed.IsZero() {
			evidence.createdAt, evidence.ageDays = oldCreatedAt, age
			evidence.ageObservedAt, evidence.ageHeld = observed, true
		}
	}
	if evidence.sizeBytes == 0 {
		size, ok := metadataFloatValue(previous.Metadata, "snapshotSizeBytes")
		observed := snapshotObservationTime(previous, "snapshotSizeObservedAt")
		if ok && size > 0 && size < float64(math.MaxInt64) && !observed.IsZero() {
			evidence.sizeBytes = int64(size)
			evidence.sizeObservedAt, evidence.sizeHeld = observed, true
		}
	}
	return evidence
}
