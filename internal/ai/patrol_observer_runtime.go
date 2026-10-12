package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rs/zerolog/log"
)

const (
	patrolObserverResourceStateFormat    = "pulse-resource-state/v1"
	patrolObserverAvailabilityFormat     = "pulse-availability-state/v1"
	patrolObserverResourceMetricFormat   = "pulse-resource-metric/v1"
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

type patrolAvailabilityStateProbe struct {
	Runtime                      string `json:"runtime"`
	TargetID                     string `json:"target_id"`
	Path                         string `json:"path"`
	Operator                     string `json:"operator"`
	Value                        string `json:"value"`
	SampleIntervalSeconds        int    `json:"sample_interval_seconds"`
	WakeAfterConsecutiveFailures int    `json:"wake_after_consecutive_failures"`
}

type patrolResourceMetricProbe struct {
	Runtime                      string  `json:"runtime"`
	Metric                       string  `json:"metric"`
	Operator                     string  `json:"operator"`
	Threshold                    float64 `json:"threshold"`
	SampleIntervalSeconds        int     `json:"sample_interval_seconds"`
	WakeAfterConsecutiveFailures int     `json:"wake_after_consecutive_failures"`
	MaxEvidenceAgeSeconds        int     `json:"max_evidence_age_seconds"`
}

type patrolValidatedObserverProbe struct {
	runtime                      string
	targetID                     string
	path                         string
	operator                     string
	value                        string
	metric                       string
	threshold                    float64
	maxEvidenceAgeSeconds        int
	sampleIntervalSeconds        int
	wakeAfterConsecutiveFailures int
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
	return &patrolObserverRuntime{
		executions: make(map[string]patrolObserverExecution),
	}
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
	objectives := store.List(false, now)
	p.queueMissingObjectiveCoverage(objectives, now)
	for _, objective := range objectives {
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
	state := p.currentPatrolRuntimeState()
	effectiveObjective := objective
	if len(effectiveObjective.Scope.ResourceIDs) == 0 {
		effectiveObjective.Scope.ResourceIDs = patrolObjectiveEffectiveResourceIDs(state)
	}
	probe, validationErr := validatePatrolObserverArtifact(effectiveObjective, artifact)
	if validationErr != nil {
		p.recordObserverValidationFailure(store, objective, validationErr.code, now)
		return nil
	}
	if bindingErr := p.validatePatrolObserverBinding(effectiveObjective, probe, state); bindingErr != nil {
		p.recordObserverValidationFailure(store, objective, bindingErr.code, now)
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
	effectiveObjective = objective
	if len(effectiveObjective.Scope.ResourceIDs) == 0 {
		effectiveObjective.Scope.ResourceIDs = patrolObjectiveEffectiveResourceIDs(state)
	}
	return p.evaluateObjectiveObserver(runtime, effectiveObjective, probe, state, now)
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

func validatePatrolObserverArtifact(objective PatrolObjective, artifact PatrolObserverArtifact) (patrolValidatedObserverProbe, *patrolObserverValidationError) {
	fail := func(code string) (patrolValidatedObserverProbe, *patrolObserverValidationError) {
		return patrolValidatedObserverProbe{}, &patrolObserverValidationError{code: code}
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
	var envelope struct {
		Runtime string `json:"runtime"`
	}
	if err := json.Unmarshal(artifact.Probe, &envelope); err != nil {
		return fail("observer_probe_invalid")
	}
	var probe patrolValidatedObserverProbe
	switch strings.TrimSpace(envelope.Runtime) {
	case patrolObserverResourceStateFormat:
		var decoded patrolResourceStateProbe
		if err := decodeStrictJSONObject(artifact.Probe, &decoded); err != nil {
			return fail("observer_probe_invalid")
		}
		probe = patrolValidatedObserverProbe{
			runtime: decoded.Runtime, path: decoded.Path, operator: decoded.Operator,
			value: decoded.Value, sampleIntervalSeconds: decoded.SampleIntervalSeconds,
			wakeAfterConsecutiveFailures: decoded.WakeAfterConsecutiveFailures,
		}
		if probe.path != "status" {
			return fail("observer_path_unsupported")
		}
		switch unifiedresources.ResourceStatus(strings.ToLower(strings.TrimSpace(probe.value))) {
		case unifiedresources.StatusOnline, unifiedresources.StatusOffline, unifiedresources.StatusWarning, unifiedresources.StatusUnknown:
			probe.value = strings.ToLower(strings.TrimSpace(probe.value))
		default:
			return fail("observer_value_unsupported")
		}
	case patrolObserverAvailabilityFormat:
		var decoded patrolAvailabilityStateProbe
		if err := decodeStrictJSONObject(artifact.Probe, &decoded); err != nil {
			return fail("observer_probe_invalid")
		}
		probe = patrolValidatedObserverProbe{
			runtime: decoded.Runtime, targetID: strings.TrimSpace(decoded.TargetID), path: decoded.Path,
			operator: decoded.Operator, value: strings.ToLower(strings.TrimSpace(decoded.Value)),
			sampleIntervalSeconds:        decoded.SampleIntervalSeconds,
			wakeAfterConsecutiveFailures: decoded.WakeAfterConsecutiveFailures,
		}
		if probe.targetID == "" {
			return fail("observer_availability_target_required")
		}
		if probe.path != "probe_outcome" {
			return fail("observer_path_unsupported")
		}
		switch probe.value {
		case "reachable", "unreachable", "indeterminate":
		default:
			return fail("observer_value_unsupported")
		}
	case patrolObserverResourceMetricFormat:
		var decoded patrolResourceMetricProbe
		if err := decodeStrictJSONObject(artifact.Probe, &decoded); err != nil {
			return fail("observer_probe_invalid")
		}
		probe = patrolValidatedObserverProbe{
			runtime: decoded.Runtime, metric: strings.ToLower(strings.TrimSpace(decoded.Metric)),
			operator: strings.ToLower(strings.TrimSpace(decoded.Operator)), threshold: decoded.Threshold,
			sampleIntervalSeconds:        decoded.SampleIntervalSeconds,
			wakeAfterConsecutiveFailures: decoded.WakeAfterConsecutiveFailures,
			maxEvidenceAgeSeconds:        decoded.MaxEvidenceAgeSeconds,
		}
		switch probe.metric {
		case "cpu_percent", "memory_percent", "disk_percent", "temperature_celsius":
		default:
			return fail("observer_metric_unsupported")
		}
		if math.IsNaN(probe.threshold) || math.IsInf(probe.threshold, 0) {
			return fail("observer_threshold_invalid")
		}
		if probe.metric == "temperature_celsius" {
			if probe.threshold < -100 || probe.threshold > 300 {
				return fail("observer_threshold_out_of_bounds")
			}
		} else if probe.threshold < 0 || probe.threshold > 100 {
			return fail("observer_threshold_out_of_bounds")
		}
		if probe.maxEvidenceAgeSeconds < probe.sampleIntervalSeconds || probe.maxEvidenceAgeSeconds > 3600 {
			return fail("observer_evidence_age_out_of_bounds")
		}
	default:
		return fail("observer_runtime_unsupported")
	}
	if probe.runtime == patrolObserverResourceMetricFormat {
		switch probe.operator {
		case "less_than", "less_than_or_equals", "greater_than", "greater_than_or_equals":
		default:
			return fail("observer_operator_unsupported")
		}
	} else if probe.operator != "equals" && probe.operator != "not_equals" {
		return fail("observer_operator_unsupported")
	}
	interval := time.Duration(probe.sampleIntervalSeconds) * time.Second
	if interval < patrolObserverMinSampleInterval || interval > patrolObserverMaxSampleInterval {
		return fail("observer_interval_out_of_bounds")
	}
	if probe.wakeAfterConsecutiveFailures < 1 || probe.wakeAfterConsecutiveFailures > patrolObserverMaxConsecutiveFailures {
		return fail("observer_failure_window_out_of_bounds")
	}
	return probe, nil
}

func (p *PatrolService) validatePatrolObserverBinding(objective PatrolObjective, probe patrolValidatedObserverProbe, state patrolRuntimeState) *patrolObserverValidationError {
	switch probe.runtime {
	case patrolObserverAvailabilityFormat:
		check, ownerID, found := patrolAvailabilityCheckByTarget(state, probe.targetID)
		if !found {
			return &patrolObserverValidationError{code: "observer_availability_target_missing"}
		}
		if !check.Enabled {
			return &patrolObserverValidationError{code: "observer_availability_target_disabled"}
		}
		scope := make(map[string]struct{}, len(objective.Scope.ResourceIDs))
		for _, resourceID := range objective.Scope.ResourceIDs {
			scope[canonicalPatrolScopeToken(resourceID)] = struct{}{}
		}
		_, ownerInScope := scope[canonicalPatrolScopeToken(ownerID)]
		_, linkedInScope := scope[canonicalPatrolScopeToken(check.LinkedResourceID)]
		if !ownerInScope && !linkedInScope {
			return &patrolObserverValidationError{code: "observer_availability_target_out_of_scope"}
		}
		return nil
	default:
		return nil
	}
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

func (p *PatrolService) evaluateObjectiveObserver(runtime *patrolObserverRuntime, objective PatrolObjective, probe patrolValidatedObserverProbe, state patrolRuntimeState, now time.Time) *patrolObserverHealthUpdate {
	observer := objective.Observer
	key := fmt.Sprintf("%s/%d", observer.ID, observer.Version)
	runtime.mu.Lock()
	execution := runtime.executions[key]
	if !execution.nextDue.IsZero() && now.Before(execution.nextDue) {
		runtime.mu.Unlock()
		return nil
	}
	interval := time.Duration(probe.sampleIntervalSeconds) * time.Second
	execution.nextDue = now.Add(interval)
	runtime.mu.Unlock()
	failing := make([]string, 0)
	evidenceDetails := make([]string, 0)
	if probe.runtime == patrolObserverAvailabilityFormat {
		check, ownerID, exists := patrolAvailabilityCheckByTarget(state, probe.targetID)
		matched := exists && check.LastChecked != nil && strings.EqualFold(check.ProbeOutcome, probe.value)
		if probe.operator == "not_equals" {
			matched = exists && check.LastChecked != nil && !strings.EqualFold(check.ProbeOutcome, probe.value)
		}
		if !matched {
			failing = append(failing, ownerID)
		}
	} else if probe.runtime == patrolObserverResourceMetricFormat {
		resources := patrolUnifiedResourcesByID(state)
		maxAge := time.Duration(probe.maxEvidenceAgeSeconds) * time.Second
		for _, resourceID := range objective.Scope.ResourceIDs {
			resource, exists := resources[canonicalPatrolScopeToken(resourceID)]
			if !exists {
				failing = append(failing, resourceID)
				evidenceDetails = append(evidenceDetails, resourceID+"=resource_missing")
				continue
			}
			observedAt := resource.LastSeen
			if observedAt.IsZero() {
				observedAt = resource.UpdatedAt
			}
			if probe.metric == "memory_percent" && (resource.Type == unifiedresources.ResourceTypeVM || resource.Type == unifiedresources.ResourceTypeSystemContainer) {
				// The row can be refreshed by a different source. Only the selected
				// guest memory sample owns its freshness and pressure semantics.
				memory := unifiedresources.GuestMemoryEvidenceForResource(&resource, now)
				if !memory.PressureKnown {
					failing = append(failing, resourceID)
					evidenceDetails = append(evidenceDetails, resourceID+"=guest_memory_pressure_unknown")
					continue
				}
				if memory.State != "" {
					observedAt = memory.ObservedAt
				}
			}
			if observedAt.IsZero() || now.Sub(observedAt) > maxAge {
				failing = append(failing, resourceID)
				evidenceDetails = append(evidenceDetails, resourceID+"=metric_stale")
				continue
			}
			value, available := patrolResourceMetricValue(resource, probe.metric)
			if !available {
				failing = append(failing, resourceID)
				evidenceDetails = append(evidenceDetails, resourceID+"=metric_missing")
				continue
			}
			if !patrolMetricPredicate(value, probe.operator, probe.threshold) {
				failing = append(failing, resourceID)
				evidenceDetails = append(evidenceDetails, fmt.Sprintf("%s=%.2f", resourceID, value))
			}
		}
	} else {
		statuses := patrolRuntimeCanonicalStatuses(state)
		for _, resourceID := range objective.Scope.ResourceIDs {
			status, exists := statuses[canonicalPatrolScopeToken(resourceID)]
			matched := exists && string(status) == probe.value
			if probe.operator == "not_equals" {
				matched = exists && string(status) != probe.value
			}
			if !matched {
				failing = append(failing, resourceID)
			}
		}
	}

	return p.finalizeObjectiveObserverSample(runtime, objective, probe, failing, evidenceDetails, now)
}

func (p *PatrolService) finalizeObjectiveObserverSample(runtime *patrolObserverRuntime, objective PatrolObjective, probe patrolValidatedObserverProbe, failing, evidenceDetails []string, now time.Time) *patrolObserverHealthUpdate {
	observer := objective.Observer
	key := fmt.Sprintf("%s/%d", observer.ID, observer.Version)
	interval := time.Duration(probe.sampleIntervalSeconds) * time.Second
	runtime.mu.Lock()
	execution := runtime.executions[key]
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
	shouldWake := len(failing) > 0 && execution.consecutiveFailures >= probe.wakeAfterConsecutiveFailures &&
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
	evidence := patrolObserverEvidence(probe, failing, evidenceDetails, len(objective.Scope.ResourceIDs), execution.consecutiveFailures)
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

func patrolObserverEvidence(probe patrolValidatedObserverProbe, failing, details []string, scopedResources, consecutiveFailures int) string {
	if probe.runtime == patrolObserverAvailabilityFormat {
		return fmt.Sprintf(
			"Canonical availability target %s did not satisfy %s %s after %d consecutive local samples.",
			probe.targetID, probe.operator, probe.value, consecutiveFailures,
		)
	}
	if probe.runtime == patrolObserverResourceMetricFormat {
		detail := ""
		if len(details) > 0 {
			detail = " Evidence: " + strings.Join(details, ", ") + "."
		}
		return fmt.Sprintf(
			"Canonical %s did not satisfy %s %.2f for %d of %d scoped resources after %d consecutive local samples.%s",
			probe.metric, probe.operator, probe.threshold, len(failing), scopedResources, consecutiveFailures, detail,
		)
	}
	return fmt.Sprintf(
		"Canonical resource status did not satisfy %s %s for %d of %d scoped resources after %d consecutive local samples.",
		probe.operator, probe.value, len(failing), scopedResources, consecutiveFailures,
	)
}

func patrolUnifiedResourcesByID(state patrolRuntimeState) map[string]unifiedresources.Resource {
	result := make(map[string]unifiedresources.Resource)
	if state.unifiedResourceProvider == nil {
		return result
	}
	for _, resource := range state.unifiedResourceProvider.GetAll() {
		if token := canonicalPatrolScopeToken(resource.ID); token != "" {
			result[token] = resource
		}
	}
	return result
}

func patrolObjectiveEffectiveResourceIDs(state patrolRuntimeState) []string {
	resources := patrolUnifiedResourcesByID(state)
	result := make([]string, 0, len(resources))
	for _, resource := range resources {
		if id := strings.TrimSpace(resource.ID); id != "" {
			result = append(result, id)
		}
	}
	sort.Strings(result)
	return result
}

func patrolResourceMetricValue(resource unifiedresources.Resource, metric string) (float64, bool) {
	if metric == "temperature_celsius" {
		if resource.Temperature == nil || math.IsNaN(*resource.Temperature) || math.IsInf(*resource.Temperature, 0) {
			return 0, false
		}
		return *resource.Temperature, true
	}
	if resource.Metrics == nil {
		return 0, false
	}
	var value *unifiedresources.MetricValue
	switch metric {
	case "cpu_percent":
		value = resource.Metrics.CPU
	case "memory_percent":
		value = resource.Metrics.Memory
	case "disk_percent":
		value = resource.Metrics.Disk
	}
	if value == nil || math.IsNaN(value.Percent) || math.IsInf(value.Percent, 0) {
		return 0, false
	}
	return value.Percent, true
}

func patrolMetricPredicate(value float64, operator string, threshold float64) bool {
	switch operator {
	case "less_than":
		return value < threshold
	case "less_than_or_equals":
		return value <= threshold
	case "greater_than":
		return value > threshold
	case "greater_than_or_equals":
		return value >= threshold
	default:
		return false
	}
}

func patrolAvailabilityCheckByTarget(state patrolRuntimeState, targetID string) (unifiedresources.AvailabilityData, string, bool) {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" || state.unifiedResourceProvider == nil {
		return unifiedresources.AvailabilityData{}, "", false
	}
	for _, resource := range state.unifiedResourceProvider.GetAll() {
		for _, check := range unifiedresources.AvailabilityChecksForResource(resource) {
			if strings.TrimSpace(check.TargetID) == targetID {
				return check, resource.ID, true
			}
		}
	}
	return unifiedresources.AvailabilityData{}, "", false
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
	if state.readState == nil {
		state = state.withDerivedProviders()
	}
	rs := state.readState
	if rs == nil {
		return result
	}
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
