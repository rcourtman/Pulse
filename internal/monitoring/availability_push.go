package monitoring

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

const (
	availabilityPushEvalInterval      = 15 * time.Second
	maxAvailabilityPushMessageRunes   = 256
	maxAvailabilityPushDurationMillis = 24 * 60 * 60 * 1000
)

type AvailabilityPushEvent string

const (
	AvailabilityPushEventSuccess AvailabilityPushEvent = "up"
	AvailabilityPushEventStart   AvailabilityPushEvent = "start"
	AvailabilityPushEventFail    AvailabilityPushEvent = "fail"
)

type AvailabilityPushPingResult struct {
	Accepted       bool      `json:"ok"`
	Ignored        bool      `json:"ignored,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	TargetID       string    `json:"targetId,omitempty"`
	Event          string    `json:"event,omitempty"`
	ReceivedAt     time.Time `json:"receivedAt,omitempty"`
	DurationMillis int64     `json:"durationMillis,omitempty"`
}

type availabilityPushState struct {
	FirstSeenAt        time.Time
	LastPingAt         time.Time
	LastStartAt        time.Time
	LastExplicitFailAt time.Time
	LastMissedDeadline time.Time
	LastDurationMillis int64
	LastMessage        string
	Running            bool
}

// RecordAvailabilityPushPing ingests an authenticated inbound heartbeat ping
// (up, start, or fail) for a push availability check.
func (m *Monitor) RecordAvailabilityPushPing(
	target config.AvailabilityTarget,
	event AvailabilityPushEvent,
	message string,
	durationOverrideMs *int64,
	now time.Time,
) AvailabilityPushPingResult {
	target = config.NormalizeAvailabilityTarget(target)
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if !target.Enabled {
		return AvailabilityPushPingResult{
			Accepted:   true,
			Ignored:    true,
			Reason:     "paused",
			TargetID:   target.ID,
			Event:      string(event),
			ReceivedAt: now,
		}
	}
	cleanedMessage := sanitizePushMessage(message)

	var durationMs int64
	m.mu.Lock()
	if m.availabilityPushStates == nil {
		m.availabilityPushStates = make(map[string]availabilityPushState)
	}
	state := m.availabilityPushStates[target.ID]
	if state.FirstSeenAt.IsZero() {
		state.FirstSeenAt = now
	}

	switch event {
	case AvailabilityPushEventStart:
		state.Running = true
		state.LastStartAt = now
		if cleanedMessage != "" {
			state.LastMessage = cleanedMessage
		}
	case AvailabilityPushEventFail:
		state.Running = false
		state.LastExplicitFailAt = now
		state.LastMissedDeadline = time.Time{}
		state.LastMessage = cleanedMessage
	default:
		event = AvailabilityPushEventSuccess
		if durationOverrideMs != nil && *durationOverrideMs >= 0 {
			durationMs = *durationOverrideMs
			if durationMs > maxAvailabilityPushDurationMillis {
				durationMs = maxAvailabilityPushDurationMillis
			}
		} else if state.Running && !state.LastStartAt.IsZero() && !now.Before(state.LastStartAt) {
			durationMs = now.Sub(state.LastStartAt).Milliseconds()
			if durationMs > maxAvailabilityPushDurationMillis {
				durationMs = maxAvailabilityPushDurationMillis
			}
		}
		state.Running = false
		state.LastPingAt = now
		state.LastExplicitFailAt = time.Time{}
		state.LastMissedDeadline = time.Time{}
		state.LastDurationMillis = durationMs
		state.LastMessage = cleanedMessage
	}
	m.availabilityPushStates[target.ID] = state
	m.mu.Unlock()

	switch event {
	case AvailabilityPushEventSuccess:
		m.applyAvailabilityObservationDetailed(
			target,
			uuid.NewString(),
			now,
			time.Duration(durationMs)*time.Millisecond,
			AvailabilityProbeReachable,
			AvailabilityProbeReachable,
			nil,
			nil,
			nil,
			"",
			time.Time{},
		)
	case AvailabilityPushEventFail:
		errText := "Push check reported failure"
		if cleanedMessage != "" {
			errText = errText + ": " + cleanedMessage
		}
		m.applyAvailabilityObservationDetailed(
			target,
			uuid.NewString(),
			now,
			0,
			AvailabilityProbeUnreachable,
			AvailabilityProbeUnreachable,
			nil,
			errors.New(errText),
			nil,
			"",
			time.Time{},
		)
	}

	if m.state != nil {
		m.updateResourceStore(m.currentStateWithScope())
	}

	return AvailabilityPushPingResult{
		Accepted:       true,
		TargetID:       target.ID,
		Event:          string(event),
		ReceivedAt:     now,
		DurationMillis: durationMs,
	}
}

// evaluatePushAvailabilityTarget checks whether an armed push target has missed
// its scheduled interval or cron deadline plus grace window.
func (m *Monitor) evaluatePushAvailabilityTarget(target config.AvailabilityTarget, now time.Time) {
	if m == nil {
		return
	}
	target = config.NormalizeAvailabilityTarget(target)
	if !target.Enabled || target.Protocol != config.AvailabilityProbePush || target.Push == nil {
		return
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	m.mu.Lock()
	if m.availabilityPushStates == nil {
		m.availabilityPushStates = make(map[string]availabilityPushState)
	}
	state, exists := m.availabilityPushStates[target.ID]
	if !exists || state.FirstSeenAt.IsZero() {
		state.FirstSeenAt = now
		m.availabilityPushStates[target.ID] = state
	}
	m.mu.Unlock()

	// Newly created checks wait for their first ping or /start before arming
	// missed-heartbeat alerts, avoiding false alarms during initial job setup.
	if state.LastPingAt.IsZero() && state.LastStartAt.IsZero() && state.LastExplicitFailAt.IsZero() {
		return
	}
	// Preserve an active explicit /fail state until a recovery ping arrives.
	if !state.LastExplicitFailAt.IsZero() && !state.LastPingAt.After(state.LastExplicitFailAt) {
		return
	}

	deadline, missed := pushTargetMissedDeadline(target, state, now)
	if !missed {
		return
	}

	m.mu.Lock()
	currentState := m.availabilityPushStates[target.ID]
	if currentState.LastMissedDeadline.Equal(deadline) {
		m.mu.Unlock()
		return
	}
	currentState.LastMissedDeadline = deadline
	currentState.Running = false
	m.availabilityPushStates[target.ID] = currentState
	m.mu.Unlock()

	overdue := now.Sub(deadline)
	errText := fmt.Sprintf("Missed expected push heartbeat (overdue by %s)", formatPushOverdue(overdue))
	m.applyAvailabilityObservationDetailed(
		target,
		uuid.NewString(),
		now,
		0,
		AvailabilityProbeUnreachable,
		AvailabilityProbeUnreachable,
		nil,
		errors.New(errText),
		nil,
		"",
		time.Time{},
	)
	if m.state != nil {
		m.updateResourceStore(m.currentStateWithScope())
	}
}

func pushTargetMissedDeadline(target config.AvailabilityTarget, state availabilityPushState, now time.Time) (time.Time, bool) {
	if target.Push == nil {
		return time.Time{}, false
	}
	grace := time.Duration(target.Push.GraceSeconds) * time.Second
	if grace < 0 {
		grace = 0
	}

	switch target.Push.ScheduleType {
	case config.AvailabilityPushScheduleCron:
		sched, err := config.ParseAvailabilityCron(target.Push.CronExpression)
		if err != nil {
			return time.Time{}, false
		}
		cutoff := now.Add(-grace)
		prevScheduled, ok := sched.Prev(cutoff)
		if !ok {
			return time.Time{}, false
		}
		floor := state.LastPingAt
		if floor.IsZero() {
			floor = state.LastStartAt
		}
		if floor.IsZero() {
			floor = state.FirstSeenAt
		}
		if !prevScheduled.After(floor) {
			return time.Time{}, false
		}
		deadline := prevScheduled.Add(grace)
		return deadline, now.After(deadline)
	default:
		interval := time.Duration(target.Push.IntervalSeconds) * time.Second
		if interval <= 0 {
			interval = time.Duration(config.DefaultAvailabilityPushIntervalSecs) * time.Second
		}
		ref := state.LastPingAt
		if ref.IsZero() {
			ref = state.LastStartAt
		}
		if ref.IsZero() {
			return time.Time{}, false
		}
		deadline := ref.Add(interval + grace)
		return deadline, now.After(deadline)
	}
}

func formatPushOverdue(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		mins := int(d / time.Minute)
		secs := int((d % time.Minute) / time.Second)
		if secs == 0 {
			return fmt.Sprintf("%dm", mins)
		}
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
	hours := int(d / time.Hour)
	mins := int((d % time.Hour) / time.Minute)
	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}

func sanitizePushMessage(raw string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, raw)
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	runes := []rune(cleaned)
	if len(runes) > maxAvailabilityPushMessageRunes {
		return string(runes[:maxAvailabilityPushMessageRunes])
	}
	return cleaned
}
