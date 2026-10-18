package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rs/zerolog/log"
)

type reportingActivityRecorder interface {
	RecordReportingActivity(config.ReportingActivityEvent) error
}

func (h *ReportingHandlers) recordReportingActivity(event config.ReportingActivityEvent) {
	if h == nil || mock.IsMockEnabled() {
		return
	}
	recorder, ok := h.settingsStore.(reportingActivityRecorder)
	if !ok {
		return
	}
	if err := recorder.RecordReportingActivity(event); err != nil {
		// No report, scope, recipient, schedule or error content enters this log.
		log.Debug().Msg("Unable to retain aggregate reporting activity")
	}
}

func (h *ReportingHandlers) recordReportingDownload(r *http.Request, kind string, count int, err error, expected int) {
	if r == nil || r.Method == http.MethodHead || r.Context().Err() != nil || err != nil || count != expected {
		return
	}
	h.recordReportingActivity(config.ReportingActivityEvent{At: time.Now().UTC(), Kind: kind, Event: "generated"})
}

func reportingActivityDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func reportingActivityPeriod(schedule config.ReportSchedule, start, end time.Time, now time.Time) string {
	if start.IsZero() || end.IsZero() {
		anchor := now
		if schedule.RunWindowEnd != nil {
			anchor = *schedule.RunWindowEnd
		}
		var err error
		start, end, err = reportScheduleWindow(schedule, anchor, schedule.RunManual)
		if err != nil {
			return ""
		}
	}
	location, err := reportScheduleLocation(schedule.Cadence.Timezone)
	if err != nil || !end.After(start) {
		return ""
	}
	// Day boundaries describe the reporting window. A retry or same-day rerun
	// cannot become repeat use merely because its clock time changed.
	return reportingActivityDigest(start.In(location).Format(time.DateOnly) + "\x00" + end.In(location).Format(time.DateOnly))
}

func (h *ReportingHandlers) recordReportingSuccessfulPeriod(ctx context.Context, schedule config.ReportSchedule, period string, at time.Time) {
	if schedule.ID == "" || period == "" || schedule.LastRunStatus != config.ReportScheduleLastRunOK {
		return
	}
	key := reportingActivityDigest(GetOrgID(ctx) + "\x00" + schedule.Kind + "\x00" + schedule.ID)
	h.recordReportingActivity(config.ReportingActivityEvent{At: at, Kind: schedule.Kind, Event: "successful_period", ScheduleKey: key, PeriodKey: period})
}

func (h *ReportingHandlers) recordReportingDeliveryFailure(kind string, status string, at time.Time) {
	event := "delivery_failed"
	if status == "unconfirmed" {
		event = "delivery_unconfirmed"
	}
	h.recordReportingActivity(config.ReportingActivityEvent{At: at, Kind: kind, Event: event})
}
