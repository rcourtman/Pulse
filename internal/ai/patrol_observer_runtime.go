package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rs/zerolog/log"
)

const (
	patrolObserverRuntimeFormat          = "pulse-resource-state/v1"
	patrolObserverSweepInterval          = 5 * time.Second
	patrolObserverMinSampleInterval      = 10 * time.Second
	patrolObserverMaxSampleInterval      = 5 * time.Minute
	patrolObserverMinLease               = 2 * time.Minute
	patrolObserverMaxLease               = 30 * time.Minute
	patrolObserverMaxConsecutiveFailures = 10
	patrolObserverWakeRetryInterval      = 15 * time.Minute
)

type patrolResourceStateProbe struct {
	Runtime                      string `json:"runtime"`
	Path                         string `json:"path"`
	Operator                     string `json:"operator"`
	Value                        string `json:"value"`
	SampleIntervalSeconds        int    `json:"sample_interval_seconds"`
	WakeAfterConsecutiveFailures int    `json:"wake_after_consecutive_failures"`
}

type patrolObserverValidationError struct{ code string }

func (e *patrolObserverValidationError) Error() string { return e.code }

type patrolObserverExecution struct {
	nextDue             time.Time
	consecutiveFailures int
	wakeEmitted         bool
	lastWakeAt          time.Time
	deliveryConfirmed   bool
}

type patrolObserverRuntime struct {
	mu         sync.Mutex
	executions map[string]patrolObserverExecution
}

func newPatrolObserverRuntime() *patrolObserverRuntime {
	return &patrolObserverRuntime{executions: make(map[string]patrolObserverExecution)}
}

func (p *PatrolService) objectiveObserverLoop(ctx context.Context) {
	ticker := time.NewTicker(patrolObserverSweepInterval)
	defer ticker.Stop()
	p.processObjectiveObservers(time.Now().UTC())
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case now := <-ticker.C:
			p.processObjectiveObservers(now.UTC())
		}
	}
}

func (p *PatrolService) processObjectiveObservers(now time.Time) {
	if p == nil {
		return
	}
	p.mu.RLock()
	store := p.objectiveStore
	runtime := p.observerRuntime
	p.mu.RUnlock()
	if store == nil || runtime == nil {
		return
	}
	activeExecutions := make(map[string]struct{})
	healthUpdates := make([]patrolObserverHealthUpdate, 0)
	for _, objective := range store.List(false, now) {
		if objective.Status != PatrolObjectiveActive || objective.Observer == nil {
			continue
		}
		activeExecutions[fmt.Sprintf("%s/%d", objective.Observer.ID, objective.Observer.Version)] = struct{}{}
		if update := p.reconcileObjectiveObserver(store, runtime, objective, now); update != nil {
			healthUpdates = append(healthUpdates, *update)
		}
	}
	runtime.retain(activeExecutions)
	if _, err := store.RefreshObserverHealthBatch(healthUpdates, now); err != nil {
		log.Warn().Err(err).Int("lease_count", len(healthUpdates)).Msg("Patrol observer health lease batch failed")
	}
}

func (r *patrolObserverRuntime) retain(active map[string]struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key := range r.executions {
		if _, ok := active[key]; !ok {
			delete(r.executions, key)
		}
	}
}

func (p *PatrolService) reconcileObjectiveObserver(store *PatrolObjectiveStore, runtime *patrolObserverRuntime, objective PatrolObjective, now time.Time) *patrolObserverHealthUpdate {
	artifact, ok := store.GetObserverArtifact(objective.ID)
	if !ok {
		p.recordObserverValidationFailure(store, objective, "observer_artifact_missing", now)
		return nil
	}
	probe, validationErr := validatePatrolObserverArtifact(objective, artifact)
	if validationErr != nil {
		p.recordObserverValidationFailure(store, objective, validationErr.code, now)
		return nil
	}

	for objective.Observer != nil && (objective.Observer.State == PatrolObserverProposed || objective.Observer.State == PatrolObserverRejected || objective.Observer.State == PatrolObserverValidated || objective.Observer.State == PatrolObserverDegraded) {
		next := *clonePatrolObserver(objective.Observer)
		next.Artifact = &artifact
		switch objective.Observer.State {
		case PatrolObserverProposed:
			next.State = PatrolObserverValidated
			next.FailureCode = ""
		case PatrolObserverRejected:
			next.State = PatrolObserverValidated
			next.FailureCode = ""
		case PatrolObserverValidated:
			next.State = PatrolObserverInstalled
			next.ValidUntil = nil
			next.LastEvidenceAt = nil
		case PatrolObserverDegraded:
			next.State = PatrolObserverInstalled
			next.FailureCode = ""
			next.ValidUntil = nil
			next.LastEvidenceAt = nil
		}
		updated, err := store.RecordObserver(objective.ID, objective.Revision, next, "patrol:observer-runtime", now)
		if err != nil {
			if !errors.Is(err, ErrPatrolObjectiveConflict) {
				log.Warn().Err(err).Str("objective_id", objective.ID).Msg("Patrol observer lifecycle transition failed")
			}
			return nil
		}
		objective = updated
	}
	if objective.Observer == nil || (objective.Observer.State != PatrolObserverInstalled && objective.Observer.State != PatrolObserverDegraded) {
		return nil
	}
	return p.evaluateObjectiveObserver(runtime, objective, probe, now)
}

func (p *PatrolService) recordObserverValidationFailure(store *PatrolObjectiveStore, objective PatrolObjective, code string, now time.Time) {
	if objective.Observer == nil || objective.Observer.State == PatrolObserverDisabled || objective.Observer.FailureCode == code {
		return
	}
	artifact, _ := store.GetObserverArtifact(objective.ID)
	next := *clonePatrolObserver(objective.Observer)
	next.FailureCode = code
	if next.State == PatrolObserverProposed || next.State == PatrolObserverRejected {
		next.State = PatrolObserverRejected
	} else {
		next.State = PatrolObserverDegraded
		next.ValidUntil = nil
	}
	if artifact.Format != "" {
		next.Artifact = &artifact
	}
	if _, err := store.RecordObserver(objective.ID, objective.Revision, next, "patrol:observer-validator", now); err != nil && !errors.Is(err, ErrPatrolObjectiveConflict) {
		log.Warn().Err(err).Str("objective_id", objective.ID).Str("failure_code", code).Msg("Patrol observer validation failure could not be recorded")
	}
}

func validatePatrolObserverArtifact(objective PatrolObjective, artifact PatrolObserverArtifact) (patrolResourceStateProbe, *patrolObserverValidationError) {
	fail := func(code string) (patrolResourceStateProbe, *patrolObserverValidationError) {
		return patrolResourceStateProbe{}, &patrolObserverValidationError{code: code}
	}
	if objective.Observer == nil || !objective.Observer.ReadOnly {
		return fail("observer_not_read_only")
	}
	if len(objective.Scope.ResourceIDs) == 0 {
		return fail("observer_scope_required")
	}
	if len(objective.Observer.TriggerKinds) != 1 || objective.Observer.TriggerKinds[0] != PatrolObserverTriggerInterval {
		return fail("observer_trigger_unsupported")
	}
	var requirements map[string]json.RawMessage
	if err := decodeStrictJSONObject(artifact.Requirements, &requirements); err != nil {
		return fail("observer_requirements_invalid")
	}
	if len(requirements) != 0 {
		return fail("observer_requirements_unsupported")
	}
	var probe patrolResourceStateProbe
	if err := decodeStrictJSONObject(artifact.Probe, &probe); err != nil {
		return fail("observer_probe_invalid")
	}
	if probe.Runtime != patrolObserverRuntimeFormat {
		return fail("observer_runtime_unsupported")
	}
	if probe.Path != "status" {
		return fail("observer_path_unsupported")
	}
	if probe.Operator != "equals" && probe.Operator != "not_equals" {
		return fail("observer_operator_unsupported")
	}
	switch unifiedresources.ResourceStatus(strings.ToLower(strings.TrimSpace(probe.Value))) {
	case unifiedresources.StatusOnline, unifiedresources.StatusOffline, unifiedresources.StatusWarning, unifiedresources.StatusUnknown:
		probe.Value = strings.ToLower(strings.TrimSpace(probe.Value))
	default:
		return fail("observer_value_unsupported")
	}
	interval := time.Duration(probe.SampleIntervalSeconds) * time.Second
	if interval < patrolObserverMinSampleInterval || interval > patrolObserverMaxSampleInterval {
		return fail("observer_interval_out_of_bounds")
	}
	if probe.WakeAfterConsecutiveFailures < 1 || probe.WakeAfterConsecutiveFailures > patrolObserverMaxConsecutiveFailures {
		return fail("observer_failure_window_out_of_bounds")
	}
	return probe, nil
}

func decodeStrictJSONObject(data []byte, target interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("expected one JSON object")
	}
	return nil
}

func (p *PatrolService) evaluateObjectiveObserver(runtime *patrolObserverRuntime, objective PatrolObjective, probe patrolResourceStateProbe, now time.Time) *patrolObserverHealthUpdate {
	observer := objective.Observer
	key := fmt.Sprintf("%s/%d", observer.ID, observer.Version)
	runtime.mu.Lock()
	execution := runtime.executions[key]
	if !execution.nextDue.IsZero() && now.Before(execution.nextDue) {
		runtime.mu.Unlock()
		return nil
	}
	interval := time.Duration(probe.SampleIntervalSeconds) * time.Second
	execution.nextDue = now.Add(interval)
	runtime.mu.Unlock()

	statuses := patrolRuntimeCanonicalStatuses(p.currentPatrolRuntimeState())
	failing := make([]string, 0)
	for _, resourceID := range objective.Scope.ResourceIDs {
		status, exists := statuses[canonicalPatrolScopeToken(resourceID)]
		matched := exists && string(status) == probe.Value
		if probe.Operator == "not_equals" {
			matched = exists && string(status) != probe.Value
		}
		if !matched {
			failing = append(failing, resourceID)
		}
	}

	runtime.mu.Lock()
	execution = runtime.executions[key]
	if len(failing) == 0 {
		execution.consecutiveFailures = 0
		execution.wakeEmitted = false
		execution.lastWakeAt = time.Time{}
		execution.deliveryConfirmed = false
	} else {
		execution.consecutiveFailures++
	}
	runtime.executions[key] = execution
	runtime.mu.Unlock()

	if len(failing) > 0 && execution.wakeEmitted && !execution.deliveryConfirmed &&
		p.objectiveWakeDelivered(objective, execution.lastWakeAt) {
		runtime.mu.Lock()
		latest := runtime.executions[key]
		if latest.lastWakeAt.Equal(execution.lastWakeAt) {
			latest.deliveryConfirmed = true
			runtime.executions[key] = latest
			execution = latest
		}
		runtime.mu.Unlock()
	}
	shouldWake := len(failing) > 0 && execution.consecutiveFailures >= probe.WakeAfterConsecutiveFailures &&
		(!execution.wakeEmitted || (!execution.deliveryConfirmed && now.Sub(execution.lastWakeAt) >= patrolObserverWakeRetryInterval))

	leaseDuration := 3 * interval
	if leaseDuration < patrolObserverMinLease {
		leaseDuration = patrolObserverMinLease
	}
	if leaseDuration > patrolObserverMaxLease {
		leaseDuration = patrolObserverMaxLease
	}
	refreshLease := observer.ValidUntil == nil || observer.ValidUntil.Sub(now) <= leaseDuration/2
	var healthUpdate *patrolObserverHealthUpdate
	if refreshLease {
		healthUpdate = &patrolObserverHealthUpdate{
			ObjectiveID: objective.ID,
			ObserverID:  observer.ID,
			Version:     observer.Version,
			ValidUntil:  now.Add(leaseDuration),
			EvidenceAt:  now,
		}
	}
	if !shouldWake {
		return healthUpdate
	}
	sort.Strings(failing)
	evidence := fmt.Sprintf(
		"Canonical resource status did not satisfy %s %s for %d of %d scoped resources after %d consecutive local samples.",
		probe.Operator, probe.Value, len(failing), len(objective.Scope.ResourceIDs), execution.consecutiveFailures,
	)
	scope := PatrolScope{
		ResourceIDs: failing,
		Depth:       PatrolDepthQuick,
		Reason:      TriggerReasonObjectiveEvidence,
		Priority:    triggerPriorityObjective,
		Context:     fmt.Sprintf("Local observer %s for objective %s detected %d of %d scoped resources outside its canonical status predicate.", observer.ID, objective.ID, len(failing), len(objective.Scope.ResourceIDs)),
		ObjectiveContext: &aicontracts.PatrolObjectiveContext{
			ObjectiveID:         objective.ID,
			Revision:            objective.Revision,
			Brief:               objective.Brief,
			Context:             objective.OptionalContext,
			ObserverID:          observer.ID,
			ObserverVersion:     observer.Version,
			ObservedResourceIDs: append([]string(nil), failing...),
			Evidence:            evidence,
			ObservedAt:          now,
		},
	}
	p.mu.RLock()
	tm := p.triggerManager
	p.mu.RUnlock()
	accepted := tm != nil && tm.TriggerPatrol(scope)
	if accepted {
		runtime.mu.Lock()
		execution = runtime.executions[key]
		execution.wakeEmitted = true
		execution.lastWakeAt = now
		execution.deliveryConfirmed = false
		runtime.executions[key] = execution
		runtime.mu.Unlock()
		log.Info().Str("objective_id", objective.ID).Int("affected_resources", len(failing)).Msg("Patrol objective observer queued an evidence-triggered check")
	}
	return healthUpdate
}

func (p *PatrolService) objectiveWakeDelivered(objective PatrolObjective, lastWakeAt time.Time) bool {
	if p == nil || lastWakeAt.IsZero() || objective.Observer == nil || p.runHistoryStore == nil {
		return false
	}
	for _, run := range p.runHistoryStore.GetAll() {
		if run.CompletedAt.Before(lastWakeAt) || run.ErrorCount > 0 || strings.EqualFold(strings.TrimSpace(run.Status), "error") {
			continue
		}
		context := run.ObjectiveContext
		if context == nil {
			continue
		}
		if context.ObjectiveID == objective.ID && context.Revision == objective.Revision &&
			context.ObserverID == objective.Observer.ID && context.ObserverVersion == objective.Observer.Version {
			return true
		}
	}
	return false
}

func patrolRuntimeCanonicalStatuses(state patrolRuntimeState) map[string]unifiedresources.ResourceStatus {
	result := make(map[string]unifiedresources.ResourceStatus)
	add := func(status unifiedresources.ResourceStatus, ids ...string) {
		for _, id := range ids {
			if token := canonicalPatrolScopeToken(id); token != "" {
				result[token] = normalizePatrolResourceStatus(string(status))
			}
		}
	}
	if rs := state.readState; rs != nil {
		for _, v := range rs.VMs() {
			add(v.Status(), v.ID(), v.SourceID())
		}
		for _, v := range rs.Containers() {
			add(v.Status(), v.ID(), v.SourceID())
		}
		for _, v := range rs.Nodes() {
			add(v.Status(), v.ID(), v.SourceID())
		}
		for _, v := range rs.Hosts() {
			add(v.Status(), v.ID())
		}
		for _, v := range rs.DockerHosts() {
			add(v.Status(), v.ID(), v.HostSourceID())
		}
		for _, v := range rs.DockerContainers() {
			add(v.Status(), v.ID(), v.ContainerID())
		}
		for _, v := range rs.StoragePools() {
			add(v.Status(), v.ID(), v.SourceID())
		}
		for _, v := range rs.PhysicalDisks() {
			add(v.Status(), v.ID())
		}
		for _, v := range rs.PBSInstances() {
			add(v.Status(), v.ID())
		}
		for _, v := range rs.PMGInstances() {
			add(v.Status(), v.ID())
		}
		for _, v := range rs.K8sClusters() {
			add(v.Status(), v.ID())
		}
		return result
	}
	for _, v := range state.VMs {
		add(normalizePatrolResourceStatus(v.Status), v.ID)
	}
	for _, v := range state.Containers {
		add(normalizePatrolResourceStatus(v.Status), v.ID)
	}
	for _, v := range state.Nodes {
		add(normalizePatrolResourceStatus(v.Status), v.ID)
	}
	for _, v := range state.Hosts {
		add(normalizePatrolResourceStatus(v.Status), v.ID)
	}
	for _, v := range state.DockerHosts {
		add(normalizePatrolResourceStatus(v.Status), v.ID)
		for _, container := range v.Containers {
			add(normalizePatrolResourceStatus(container.State), container.ID)
		}
	}
	for _, v := range state.Storage {
		add(normalizePatrolResourceStatus(v.Status), v.ID)
	}
	for _, v := range state.PBSInstances {
		add(normalizePatrolResourceStatus(v.Status), v.ID)
	}
	for _, v := range state.PMGInstances {
		add(normalizePatrolResourceStatus(v.Status), v.ID)
	}
	for _, v := range state.KubernetesClusters {
		add(normalizePatrolResourceStatus(v.Status), v.ID)
	}
	return result
}

func normalizePatrolResourceStatus(value string) unifiedresources.ResourceStatus {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "online", "running", "active", "healthy", "up", "available", "ready":
		return unifiedresources.StatusOnline
	case "offline", "stopped", "inactive", "failed", "error", "down", "unavailable":
		return unifiedresources.StatusOffline
	case "warning", "degraded", "unhealthy":
		return unifiedresources.StatusWarning
	default:
		return unifiedresources.StatusUnknown
	}
}
