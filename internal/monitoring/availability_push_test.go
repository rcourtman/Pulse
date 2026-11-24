package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func newPushTestMonitor() *Monitor {
	return &Monitor{
		availabilityStatuses:   make(map[string]AvailabilityProbeStatus),
		availabilityByLocation: make(map[string]map[string]AvailabilityProbeStatus),
		availabilityPushStates: make(map[string]availabilityPushState),
		pollStatusMap:          make(map[string]*pollStatus),
		failureCounts:          make(map[string]int),
		lastOutcome:            make(map[string]taskOutcome),
	}
}

func TestMonitorPushAvailabilityLifecycleIntervalAndStartDuration(t *testing.T) {
	m := newPushTestMonitor()
	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID:       "push-backup",
		Name:     "Nightly backup",
		Protocol: config.AvailabilityProbePush,
		Enabled:  true,
		Push: &config.AvailabilityPushConfig{
			Token:           "abcdefghijklmnopqrstuvwxyz123456",
			ScheduleType:    config.AvailabilityPushScheduleInterval,
			IntervalSeconds: 3600,
			GraceSeconds:    300,
		},
	})

	base := time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC)

	// Before any ping arrives, evaluation should not create a false failure.
	m.evaluatePushAvailabilityTarget(target, base)
	if _, ok := m.availabilityStatuses[target.ID]; ok {
		t.Fatalf("expected no failure status before first ping")
	}

	// Job starts at 02:00 and succeeds 2500ms later.
	startRes := m.RecordAvailabilityPushPing(target, AvailabilityPushEventStart, "starting backup", nil, base)
	if !startRes.Accepted || startRes.Ignored {
		t.Fatalf("unexpected start ping result: %+v", startRes)
	}
	doneAt := base.Add(2500 * time.Millisecond)
	okRes := m.RecordAvailabilityPushPing(target, AvailabilityPushEventSuccess, "backup complete", nil, doneAt)
	if !okRes.Accepted || okRes.DurationMillis != 2500 {
		t.Fatalf("unexpected success ping result: %+v", okRes)
	}

	status := m.availabilityStatuses[target.ID]
	if !status.Available || status.Outcome != string(AvailabilityProbeReachable) || status.LatencyMillis != 2500 {
		t.Fatalf("expected reachable status with 2500ms duration, got %+v", status)
	}

	// Within interval + grace (1h 4m after doneAt): still healthy.
	m.evaluatePushAvailabilityTarget(target, doneAt.Add(64*time.Minute))
	if !m.availabilityStatuses[target.ID].Available {
		t.Fatalf("expected target to remain available within grace window")
	}

	// Past interval + grace (1h 6m after doneAt): transitions to unreachable + incident.
	overdueAt := doneAt.Add(66 * time.Minute)
	m.evaluatePushAvailabilityTarget(target, overdueAt)
	missed := m.availabilityStatuses[target.ID]
	if missed.Available || missed.Outcome != string(AvailabilityProbeUnreachable) {
		t.Fatalf("expected unreachable status after missed deadline, got %+v", missed)
	}
	resource, _ := availabilityResourceFromTarget(target, missed, "default", overdueAt)
	if resource.Status != unifiedresources.StatusOffline {
		t.Fatalf("resource.Status = %q, want offline", resource.Status)
	}
	if len(resource.Incidents) != 1 || resource.Incidents[0].Code != "availability_unreachable" {
		t.Fatalf("expected availability_unreachable incident, got %+v", resource.Incidents)
	}

	// Explicit /fail also transitions to unreachable immediately.
	m.RecordAvailabilityPushPing(target, AvailabilityPushEventSuccess, "recovered", nil, overdueAt.Add(time.Minute))
	if !m.availabilityStatuses[target.ID].Available {
		t.Fatalf("expected recovery ping to restore Available=true")
	}
	m.RecordAvailabilityPushPing(target, AvailabilityPushEventFail, "disk full", nil, overdueAt.Add(2*time.Minute))
	failed := m.availabilityStatuses[target.ID]
	if failed.Available || failed.LastError != "Push check reported failure: disk full" {
		t.Fatalf("unexpected explicit fail status: %+v", failed)
	}
}

func TestMonitorPushAvailabilityCronSchedule(t *testing.T) {
	m := newPushTestMonitor()
	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID:       "push-cron",
		Name:     "03:00 DB dump",
		Protocol: config.AvailabilityProbePush,
		Enabled:  true,
		Push: &config.AvailabilityPushConfig{
			Token:          "abcdefghijklmnopqrstuvwxyz123456",
			ScheduleType:   config.AvailabilityPushScheduleCron,
			CronExpression: "0 3 * * *",
			GraceSeconds:   300, // 5m grace -> deadline 03:05 UTC
		},
	})

	// Initial healthy ping at 03:02 UTC on Nov 23.
	m.RecordAvailabilityPushPing(target, AvailabilityPushEventSuccess, "", nil, time.Date(2026, 11, 23, 3, 2, 0, 0, time.UTC))
	if !m.availabilityStatuses[target.ID].Available {
		t.Fatalf("expected healthy after Nov 23 ping")
	}

	// At 03:04 UTC on Nov 24 (within 5m grace): still healthy.
	m.evaluatePushAvailabilityTarget(target, time.Date(2026, 11, 24, 3, 4, 0, 0, time.UTC))
	if !m.availabilityStatuses[target.ID].Available {
		t.Fatalf("expected healthy during grace window at 03:04 UTC")
	}

	// At 03:06 UTC on Nov 24 (past 03:05 UTC deadline): missed!
	m.evaluatePushAvailabilityTarget(target, time.Date(2026, 11, 24, 3, 6, 0, 0, time.UTC))
	if m.availabilityStatuses[target.ID].Available {
		t.Fatalf("expected unreachable after 03:05 UTC cron deadline passed")
	}
}

func TestMonitorPushAvailabilityPausedTargetIgnoresPings(t *testing.T) {
	m := newPushTestMonitor()
	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID:       "push-paused",
		Name:     "Paused job",
		Protocol: config.AvailabilityProbePush,
		Enabled:  false,
		Push: &config.AvailabilityPushConfig{
			Token:           "abcdefghijklmnopqrstuvwxyz123456",
			ScheduleType:    config.AvailabilityPushScheduleInterval,
			IntervalSeconds: 3600,
			GraceSeconds:    300,
		},
	})
	res := m.RecordAvailabilityPushPing(target, AvailabilityPushEventFail, "ignored", nil, time.Now().UTC())
	if !res.Accepted || !res.Ignored || res.Reason != "paused" {
		t.Fatalf("expected paused ping to be accepted+ignored, got %+v", res)
	}
	if _, exists := m.availabilityStatuses[target.ID]; exists {
		t.Fatalf("paused target should not mutate availabilityStatuses")
	}
}
