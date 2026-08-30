package hostagent

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/availabilityprobe"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

// availabilitySettingsKey is the remote-config key carrying the availability
// targets the server assigned to this agent.
const availabilitySettingsKey = "availabilityTargets"

// availabilityModuleName is how an active probe assignment is surfaced in
// AgentInfo.Modules, alongside host/docker/kubernetes.
const availabilityModuleName = "availability"

const (
	availabilityModuleStateStarting = "starting"
	availabilityModuleStateRunning  = "running"
)

// availabilityPendingCapacity bounds the results waiting for a report. It
// mirrors the report buffer's spirit: an agent that cannot reach Pulse keeps
// the newest observations and drops the oldest rather than growing without
// limit.
const availabilityPendingCapacity = 200

// availabilityMinInterval and availabilityMaxInterval mirror the server-side
// clamp so an assignment cannot make the agent probe faster (or rarer) than
// the poller would have locally.
const (
	availabilityMinInterval = 10 * time.Second
	availabilityMaxInterval = time.Hour
)

// availabilityErrorLimit bounds the failure text a single check contributes to
// a report. The server only shows the latest error, so an unbounded message
// buys nothing and inflates every report.
const availabilityErrorLimit = 240

// pendingAvailabilityResult tags each queued result with a monotonic sequence.
// Delivery is confirmed by sequence rather than by count so a queue overflow
// while a report is in flight cannot drop an observation that was never sent.
type pendingAvailabilityResult struct {
	sequence uint64
	result   agentshost.AvailabilityProbeResult
}

// availabilityProbeModule runs the availability checks the server assigned to
// this agent and queues their results for the next host report. It owns its
// scheduling entirely: the server sends assignments, never a schedule.
type availabilityProbeModule struct {
	logger        zerolog.Logger
	now           func() time.Time
	probe         func(context.Context, config.AvailabilityTarget) (availabilityprobe.Outcome, error)
	detailedProbe func(context.Context, config.AvailabilityTarget) (availabilityprobe.ProbeResult, error)

	mu        sync.Mutex
	targets   []config.AvailabilityTarget
	updatedAt time.Time
	running   bool
	sequence  uint64
	inFlight  uint64

	pending *utils.Queue[pendingAvailabilityResult]
	reload  chan struct{}
}

func newAvailabilityProbeModule(logger zerolog.Logger, targets []config.AvailabilityTarget) *availabilityProbeModule {
	module := &availabilityProbeModule{
		logger:        logger.With().Str("module", availabilityModuleName).Logger(),
		now:           time.Now,
		detailedProbe: availabilityprobe.DetailedResult,
		pending:       utils.New[pendingAvailabilityResult](availabilityPendingCapacity),
		reload:        make(chan struct{}, 1),
	}
	module.applyTargets(targets)
	return module
}

// applyTargets replaces the assignment set. It reports whether the schedule
// actually changed: the remote config is re-fetched on a fixed interval and an
// unchanged assignment must not restart (and therefore re-run) every check.
func (m *availabilityProbeModule) applyTargets(targets []config.AvailabilityTarget) bool {
	if m == nil {
		return false
	}
	normalized := normalizeAvailabilityAssignments(targets)

	m.mu.Lock()
	if availabilityAssignmentsEqual(m.targets, normalized) {
		m.mu.Unlock()
		return false
	}
	m.targets = normalized
	m.updatedAt = m.now().UTC()
	m.mu.Unlock()

	select {
	case m.reload <- struct{}{}:
	default:
	}
	return true
}

func normalizeAvailabilityAssignments(targets []config.AvailabilityTarget) []config.AvailabilityTarget {
	normalized := make([]config.AvailabilityTarget, 0, len(targets))
	for _, target := range targets {
		// The ID is checked before normalization: defaults would mint a fresh
		// one, and a target the server cannot recognise is unreportable.
		if strings.TrimSpace(target.ID) == "" {
			continue
		}
		normalized = append(normalized, config.NormalizeAvailabilityTarget(target))
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].ID < normalized[j].ID })
	return normalized
}

func availabilityAssignmentsEqual(left, right []config.AvailabilityTarget) bool {
	return reflect.DeepEqual(left, right)
}

func (m *availabilityProbeModule) assignments() []config.AvailabilityTarget {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]config.AvailabilityTarget(nil), m.targets...)
}

// Run supervises the probe schedule until the context is cancelled. Each
// assignment change stops the current workers and rebuilds them, which keeps a
// removed target from ever running one more time.
func (m *availabilityProbeModule) Run(ctx context.Context) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
	}()

	for {
		// Consume a reload queued before this pass: the assignment about to be
		// read already contains it, and acting on it again would restart every
		// worker and re-run each check immediately.
		select {
		case <-m.reload:
		default:
		}

		runCtx, cancel := context.WithCancel(ctx)
		var wg sync.WaitGroup
		for _, target := range m.assignments() {
			if !target.Enabled {
				continue
			}
			wg.Add(1)
			go func(target config.AvailabilityTarget) {
				defer wg.Done()
				m.runTarget(runCtx, target)
			}(target)
		}

		select {
		case <-ctx.Done():
			cancel()
			wg.Wait()
			return
		case <-m.reload:
			cancel()
			wg.Wait()
		}
	}
}

func (m *availabilityProbeModule) runTarget(ctx context.Context, target config.AvailabilityTarget) {
	interval := availabilityProbeInterval(target)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Probe immediately so a fresh assignment reports before the server's
	// staleness window opens instead of after a full interval.
	m.check(ctx, target)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.check(ctx, target)
		}
	}
}

func availabilityProbeInterval(target config.AvailabilityTarget) time.Duration {
	interval := time.Duration(target.EffectivePollIntervalSecs()) * time.Second
	if interval < availabilityMinInterval {
		return availabilityMinInterval
	}
	if interval > availabilityMaxInterval {
		return availabilityMaxInterval
	}
	return interval
}

func (m *availabilityProbeModule) check(ctx context.Context, target config.AvailabilityTarget) {
	timeout := time.Duration(target.EffectiveTimeoutMillis()) * time.Millisecond
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	start := m.now()
	probeResult, err := m.runProbe(probeCtx, target)
	latency := m.now().Sub(start)
	cancel()

	if ctx.Err() != nil {
		// A shutdown or reassignment cancelled the check mid-flight. The
		// failure it produced describes the agent, not the target.
		return
	}
	if latency < 0 {
		latency = 0
	}

	result := agentshost.AvailabilityProbeResult{
		ObservationID:    uuid.NewString(),
		TargetID:         target.ID,
		ConfigRevision:   target.ConfigRevision,
		Outcome:          string(probeResult.Outcome),
		TransportOutcome: string(probeResult.TransportOutcome),
		LatencyMillis:    latency.Milliseconds(),
		CheckedAt:        m.now().UTC(),
		Certificate:      probeResult.Certificate.Clone(),
	}
	if probeResult.Application != nil {
		result.ApplicationOutcome = string(probeResult.Application.Outcome)
		result.ApplicationStatusCode = probeResult.Application.StatusCode
		result.ApplicationFailureCode = probeResult.Application.FailureCode
	}
	if err != nil {
		message := strings.TrimSpace(err.Error())
		if len(message) > availabilityErrorLimit {
			message = message[:availabilityErrorLimit]
		}
		result.Error = message
	}

	m.enqueue(result)

	m.logger.Debug().
		Str("targetID", target.ID).
		Str("outcome", result.Outcome).
		Int64("latencyMillis", result.LatencyMillis).
		Msg("Completed assigned availability check")
}

func (m *availabilityProbeModule) runProbe(ctx context.Context, target config.AvailabilityTarget) (availabilityprobe.ProbeResult, error) {
	if m.probe != nil {
		outcome, err := m.probe(ctx, target)
		return availabilityprobe.ProbeResult{Outcome: outcome}, err
	}
	if m.detailedProbe != nil {
		return m.detailedProbe(ctx, target)
	}
	return availabilityprobe.DetailedResult(ctx, target)
}

// enqueue queues one completed observation for the next report. The queue is
// bounded and drops the oldest entry when full, so a long outage costs the
// earliest observations rather than unbounded memory.
func (m *availabilityProbeModule) enqueue(result agentshost.AvailabilityProbeResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sequence++
	pending := pendingAvailabilityResult{sequence: m.sequence, result: result}
	m.pending.Push(pending)
}

// snapshotForReport returns every queued result without removing it and marks
// the batch as in flight. Nothing leaves the queue until the primary
// destination has accepted it.
func (m *availabilityProbeModule) snapshotForReport() []agentshost.AvailabilityProbeResult {
	if m == nil {
		return nil
	}
	items := m.pending.Items()
	if len(items) == 0 {
		m.mu.Lock()
		m.inFlight = 0
		m.mu.Unlock()
		return nil
	}
	results := make([]agentshost.AvailabilityProbeResult, 0, len(items))
	for _, item := range items {
		results = append(results, item.result)
	}

	m.mu.Lock()
	m.inFlight = items[len(items)-1].sequence
	m.mu.Unlock()
	return results
}

// commitDelivered drops the results the primary destination accepted. Results
// queued while the report was in flight carry a higher sequence and survive
// for the next report, so no observation is delivered twice or lost.
func (m *availabilityProbeModule) commitDelivered() {
	if m == nil {
		return
	}
	m.mu.Lock()
	delivered := m.inFlight
	m.inFlight = 0
	m.mu.Unlock()
	if delivered == 0 {
		return
	}
	for {
		item, ok := m.pending.Peek()
		if !ok || item.sequence > delivered {
			return
		}
		m.pending.Pop()
	}
}

// discardInFlight forgets the in-flight marker when a report is buffered
// instead of sent, so a later success cannot retire results the server never
// received.
func (m *availabilityProbeModule) discardInFlight() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.inFlight = 0
	m.mu.Unlock()
}

// moduleStatus surfaces the probe module in AgentInfo.Modules only while the
// server has something assigned to this agent. An agent with no assignment has
// no module to report on.
func (m *availabilityProbeModule) moduleStatus() (agentshost.ModuleStatus, bool) {
	if m == nil {
		return agentshost.ModuleStatus{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.targets) == 0 {
		return agentshost.ModuleStatus{}, false
	}
	state := availabilityModuleStateStarting
	if m.running {
		state = availabilityModuleStateRunning
	}
	return agentshost.ModuleStatus{
		Name:      availabilityModuleName,
		Enabled:   true,
		State:     state,
		UpdatedAt: m.updatedAt,
	}, true
}

// AvailabilityTargetsFromSetting decodes the availability assignments carried
// by a remote-config setting value. The payload is re-marshalled through JSON
// so the agent shares the server's field names and ignores keys it does not
// know.
func AvailabilityTargetsFromSetting(value interface{}) ([]config.AvailabilityTarget, error) {
	if value == nil {
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode availability targets: %w", err)
	}
	var targets []config.AvailabilityTarget
	if err := json.Unmarshal(raw, &targets); err != nil {
		return nil, fmt.Errorf("decode availability targets: %w", err)
	}
	return normalizeAvailabilityAssignments(targets), nil
}

// availabilityTargetsFromSettings reports the assignments in a remote-config
// payload. The server omits the key entirely when it has nothing assigned to
// this agent, so a missing key means "no assignments" rather than "unchanged".
func availabilityTargetsFromSettings(settings map[string]interface{}) ([]config.AvailabilityTarget, error) {
	if settings == nil {
		return nil, nil
	}
	value, ok := settings[availabilitySettingsKey]
	if !ok {
		return nil, nil
	}
	return AvailabilityTargetsFromSetting(value)
}
