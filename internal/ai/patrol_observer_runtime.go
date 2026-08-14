package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rs/zerolog/log"
)

const (
	patrolObserverResourceStateFormat    = "pulse-resource-state/v1"
	patrolObserverAvailabilityFormat     = "pulse-availability-state/v1"
	patrolObserverResourceMetricFormat   = "pulse-resource-metric/v1"
	patrolObserverHTTPJSONFormat         = "pulse-http-json/v1"
	patrolObserverSweepInterval          = 5 * time.Second
	patrolObserverMinSampleInterval      = 10 * time.Second
	patrolObserverMaxSampleInterval      = 5 * time.Minute
	patrolObserverMinLease               = 2 * time.Minute
	patrolObserverMaxLease               = 30 * time.Minute
	patrolObserverMaxConsecutiveFailures = 10
	patrolObserverWakeRetryInterval      = 15 * time.Minute
	patrolObserverHTTPMaxBodyBytes       = 1024 * 1024
	patrolObserverHTTPMaxConcurrency     = 4
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

type patrolHTTPJSONAuthReference struct {
	HeaderName string `json:"header_name"`
	SecretRef  string `json:"secret_ref"`
}

type patrolHTTPJSONProbe struct {
	Runtime                      string                       `json:"runtime"`
	DiscoveryID                  string                       `json:"discovery_id"`
	RequestPath                  string                       `json:"request_path"`
	JSONPointer                  string                       `json:"json_pointer"`
	Operator                     string                       `json:"operator"`
	Expected                     json.RawMessage              `json:"expected,omitempty"`
	Auth                         *patrolHTTPJSONAuthReference `json:"auth,omitempty"`
	TimeoutSeconds               int                          `json:"timeout_seconds"`
	SampleIntervalSeconds        int                          `json:"sample_interval_seconds"`
	WakeAfterConsecutiveFailures int                          `json:"wake_after_consecutive_failures"`
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
	discoveryID                  string
	requestPath                  string
	jsonPointer                  string
	expected                     json.RawMessage
	auth                         *patrolHTTPJSONAuthReference
	timeoutSeconds               int
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
	httpSlots  chan struct{}
}

func newPatrolObserverRuntime() *patrolObserverRuntime {
	return &patrolObserverRuntime{
		executions: make(map[string]patrolObserverExecution),
		httpSlots:  make(chan struct{}, patrolObserverHTTPMaxConcurrency),
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
	case patrolObserverHTTPJSONFormat:
		var decoded patrolHTTPJSONProbe
		if err := decodeStrictJSONObject(artifact.Probe, &decoded); err != nil {
			return fail("observer_probe_invalid")
		}
		probe = patrolValidatedObserverProbe{
			runtime: decoded.Runtime, discoveryID: strings.TrimSpace(decoded.DiscoveryID),
			requestPath: decoded.RequestPath, jsonPointer: decoded.JSONPointer,
			operator: strings.ToLower(strings.TrimSpace(decoded.Operator)), expected: append(json.RawMessage(nil), decoded.Expected...),
			auth: decoded.Auth, timeoutSeconds: decoded.TimeoutSeconds,
			sampleIntervalSeconds:        decoded.SampleIntervalSeconds,
			wakeAfterConsecutiveFailures: decoded.WakeAfterConsecutiveFailures,
		}
		if probe.discoveryID == "" {
			return fail("observer_discovery_required")
		}
		if err := validatePatrolHTTPRelativePath(probe.requestPath); err != nil {
			return fail("observer_http_path_invalid")
		}
		if err := validatePatrolJSONPointer(probe.jsonPointer); err != nil {
			return fail("observer_json_pointer_invalid")
		}
		if probe.timeoutSeconds < 1 || probe.timeoutSeconds > 5 {
			return fail("observer_http_timeout_out_of_bounds")
		}
		if probe.auth != nil {
			probe.auth.HeaderName = http.CanonicalHeaderKey(strings.TrimSpace(probe.auth.HeaderName))
			probe.auth.SecretRef = strings.TrimSpace(probe.auth.SecretRef)
			if !validPatrolHTTPAuthHeader(probe.auth.HeaderName) || probe.auth.SecretRef == "" {
				return fail("observer_http_auth_invalid")
			}
		}
		if probe.operator == "exists" || probe.operator == "not_exists" {
			if len(probe.expected) != 0 {
				return fail("observer_expected_unsupported")
			}
		} else {
			if len(probe.expected) == 0 || !json.Valid(probe.expected) {
				return fail("observer_expected_invalid")
			}
			var expected any
			decoder := json.NewDecoder(bytes.NewReader(probe.expected))
			decoder.UseNumber()
			if err := decoder.Decode(&expected); err != nil {
				return fail("observer_expected_invalid")
			}
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
	} else if probe.runtime == patrolObserverHTTPJSONFormat {
		switch probe.operator {
		case "exists", "not_exists", "equals", "not_equals", "less_than", "less_than_or_equals", "greater_than", "greater_than_or_equals":
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
	case patrolObserverHTTPJSONFormat:
		discovery, code := p.patrolObserverDiscovery(objective, probe.discoveryID)
		if code != "" {
			return &patrolObserverValidationError{code: code}
		}
		if _, err := patrolHTTPObserverURL(discovery.SuggestedURL, probe.requestPath); err != nil {
			return &patrolObserverValidationError{code: "observer_discovery_url_invalid"}
		}
		if probe.auth != nil {
			if secret := strings.TrimSpace(discovery.UserSecrets[probe.auth.SecretRef]); secret == "" {
				return &patrolObserverValidationError{code: "observer_http_secret_missing"}
			}
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
	if probe.runtime == patrolObserverHTTPJSONFormat {
		select {
		case runtime.httpSlots <- struct{}{}:
			go p.evaluateHTTPJSONObserver(runtime, objective, probe, now)
		default:
			runtime.mu.Lock()
			execution = runtime.executions[key]
			execution.nextDue = now.Add(patrolObserverSweepInterval)
			runtime.executions[key] = execution
			runtime.mu.Unlock()
		}
		return nil
	}

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
	if probe.runtime == patrolObserverHTTPJSONFormat {
		detail := ""
		if len(details) > 0 {
			detail = " Evidence: " + strings.Join(details, ", ") + "."
		}
		return fmt.Sprintf(
			"Read-only discovery API assertion %s %s at JSON pointer %q failed after %d consecutive local samples.%s",
			probe.discoveryID, probe.operator, probe.jsonPointer, consecutiveFailures, detail,
		)
	}
	return fmt.Sprintf(
		"Canonical resource status did not satisfy %s %s for %d of %d scoped resources after %d consecutive local samples.",
		probe.operator, probe.value, len(failing), scopedResources, consecutiveFailures,
	)
}

func (p *PatrolService) evaluateHTTPJSONObserver(runtime *patrolObserverRuntime, objective PatrolObjective, probe patrolValidatedObserverProbe, now time.Time) {
	defer func() { <-runtime.httpSlots }()
	store := p.GetObjectiveStore()
	if store == nil || objective.Observer == nil {
		return
	}
	discovery, code := p.patrolObserverDiscovery(objective, probe.discoveryID)
	if code != "" {
		p.recordObserverValidationFailure(store, objective, code, time.Now().UTC())
		return
	}
	resourceID := patrolDiscoveryScopedResourceID(objective, discovery)
	matched, detail := p.samplePatrolHTTPJSONObserver(discovery, probe)
	failing := []string(nil)
	details := []string(nil)
	if !matched {
		failing = []string{resourceID}
		details = []string{resourceID + "=" + detail}
	}
	current, found := store.Get(objective.ID, time.Now().UTC())
	if !found || current.Observer == nil || current.Revision != objective.Revision || current.Observer.ID != objective.Observer.ID || current.Observer.Version != objective.Observer.Version {
		return
	}
	if len(current.Scope.ResourceIDs) == 0 {
		current.Scope.ResourceIDs = append([]string(nil), objective.Scope.ResourceIDs...)
	}
	healthUpdate := p.finalizeObjectiveObserverSample(runtime, current, probe, failing, details, now)
	if healthUpdate != nil {
		if _, err := store.RefreshObserverHealthBatch([]patrolObserverHealthUpdate{*healthUpdate}, time.Now().UTC()); err != nil {
			log.Warn().Err(err).Str("objective_id", objective.ID).Msg("Patrol HTTP JSON observer health lease failed")
		}
	}
}

func (p *PatrolService) samplePatrolHTTPJSONObserver(discovery *servicediscovery.ResourceDiscovery, probe patrolValidatedObserverProbe) (bool, string) {
	target, err := patrolHTTPObserverURL(discovery.SuggestedURL, probe.requestPath)
	if err != nil {
		return false, "url_invalid"
	}
	timeout := time.Duration(probe.timeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	opts := securityutil.RestrictedOutboundHTTPOptions{
		AllowedSchemes: []string{"http", "https"}, AllowPrivateIPs: true, AllowLoopback: true,
		ResponseHeaderTimeout: timeout,
	}
	validated, err := securityutil.ValidateOutboundFetchURL(ctx, target.String(), opts)
	if err != nil {
		return false, "target_blocked"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, validated.String(), nil)
	if err != nil {
		return false, "request_invalid"
	}
	request.Header.Set("Accept", "application/json")
	if probe.auth != nil {
		secret := strings.TrimSpace(discovery.UserSecrets[probe.auth.SecretRef])
		if secret == "" {
			return false, "secret_missing"
		}
		request.Header.Set(probe.auth.HeaderName, secret)
	}
	response, err := securityutil.NewRestrictedOutboundHTTPClient(timeout, opts).Do(request)
	if err != nil {
		return false, "request_failed"
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return false, fmt.Sprintf("http_status_%d", response.StatusCode)
	}
	limited := io.LimitReader(response.Body, patrolObserverHTTPMaxBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return false, "response_read_failed"
	}
	if len(body) > patrolObserverHTTPMaxBodyBytes {
		return false, "response_too_large"
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return false, "response_not_json"
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return false, "response_not_single_json_value"
	}
	actual, exists := patrolJSONPointerValue(document, probe.jsonPointer)
	matched := patrolJSONPredicate(actual, exists, probe.operator, probe.expected)
	if matched {
		return true, "matched"
	}
	if !exists {
		return false, "pointer_missing"
	}
	return false, "actual_" + patrolJSONEvidenceValue(actual)
}

func (p *PatrolService) patrolObserverDiscovery(objective PatrolObjective, discoveryID string) (*servicediscovery.ResourceDiscovery, string) {
	if p == nil {
		return nil, "observer_discovery_store_unavailable"
	}
	p.mu.RLock()
	store := p.discoveryStore
	p.mu.RUnlock()
	if store == nil {
		return nil, "observer_discovery_store_unavailable"
	}
	discovery, err := store.Get(discoveryID)
	if err != nil || discovery == nil {
		return nil, "observer_discovery_missing"
	}
	if len(servicediscovery.FilterDiscoveriesByResourceIDs([]*servicediscovery.ResourceDiscovery{discovery}, objective.Scope.ResourceIDs)) == 0 {
		return nil, "observer_discovery_out_of_scope"
	}
	if strings.TrimSpace(discovery.SuggestedURL) == "" {
		return nil, "observer_discovery_url_missing"
	}
	return discovery, ""
}

func patrolDiscoveryScopedResourceID(objective PatrolObjective, discovery *servicediscovery.ResourceDiscovery) string {
	for _, resourceID := range objective.Scope.ResourceIDs {
		if len(servicediscovery.FilterDiscoveriesByResourceIDs([]*servicediscovery.ResourceDiscovery{discovery}, []string{resourceID})) > 0 {
			return resourceID
		}
	}
	if discovery != nil && strings.TrimSpace(discovery.ResourceID) != "" {
		return strings.TrimSpace(discovery.ResourceID)
	}
	return objective.Scope.ResourceIDs[0]
}

func validatePatrolHTTPRelativePath(raw string) error {
	if raw == "" || len(raw) > 2048 || !strings.HasPrefix(raw, "/") {
		return errors.New("request path must be a bounded absolute path")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil || parsed.Fragment != "" || strings.HasPrefix(raw, "//") {
		return errors.New("request path cannot carry an origin, user info, or fragment")
	}
	return nil
}

func patrolHTTPObserverURL(baseURL, requestPath string) (*url.URL, error) {
	if err := validatePatrolHTTPRelativePath(requestPath); err != nil {
		return nil, err
	}
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || base.Scheme == "" || base.Host == "" || base.User != nil || base.Fragment != "" {
		return nil, errors.New("discovery URL is not an absolute HTTP origin")
	}
	if !strings.EqualFold(base.Scheme, "http") && !strings.EqualFold(base.Scheme, "https") {
		return nil, errors.New("discovery URL scheme is unsupported")
	}
	relative, _ := url.Parse(requestPath)
	resolved := base.ResolveReference(relative)
	if !strings.EqualFold(resolved.Scheme, base.Scheme) || !strings.EqualFold(resolved.Host, base.Host) {
		return nil, errors.New("resolved observer URL changed origin")
	}
	return resolved, nil
}

func validPatrolHTTPAuthHeader(header string) bool {
	if header == "Authorization" || strings.HasPrefix(header, "X-") {
		for _, r := range header {
			if !(r == '-' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
				return false
			}
		}
		return len(header) <= 128
	}
	return false
}

func validatePatrolJSONPointer(pointer string) error {
	if len(pointer) > 2048 {
		return errors.New("JSON pointer too long")
	}
	if pointer == "" {
		return nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return errors.New("JSON pointer must be empty or begin with slash")
	}
	for _, token := range strings.Split(pointer[1:], "/") {
		for index := 0; index < len(token); index++ {
			if token[index] == '~' && (index+1 >= len(token) || token[index+1] != '0' && token[index+1] != '1') {
				return errors.New("JSON pointer escape is invalid")
			}
		}
	}
	return nil
}

func patrolJSONPointerValue(document any, pointer string) (any, bool) {
	if pointer == "" {
		return document, true
	}
	current := document
	for _, rawToken := range strings.Split(pointer[1:], "/") {
		token := strings.ReplaceAll(strings.ReplaceAll(rawToken, "~1", "/"), "~0", "~")
		switch value := current.(type) {
		case map[string]any:
			var exists bool
			current, exists = value[token]
			if !exists {
				return nil, false
			}
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(value) || token != strconv.Itoa(index) {
				return nil, false
			}
			current = value[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func patrolJSONPredicate(actual any, exists bool, operator string, expectedJSON json.RawMessage) bool {
	switch operator {
	case "exists":
		return exists
	case "not_exists":
		return !exists
	}
	if !exists {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(expectedJSON))
	decoder.UseNumber()
	var expected any
	if err := decoder.Decode(&expected); err != nil {
		return false
	}
	if operator == "equals" || operator == "not_equals" {
		matched := reflect.DeepEqual(actual, expected)
		if operator == "not_equals" {
			return !matched
		}
		return matched
	}
	actualNumber, actualOK := patrolJSONNumber(actual)
	expectedNumber, expectedOK := patrolJSONNumber(expected)
	if !actualOK || !expectedOK {
		return false
	}
	return patrolMetricPredicate(actualNumber, operator, expectedNumber)
}

func patrolJSONNumber(value any) (float64, bool) {
	switch number := value.(type) {
	case json.Number:
		parsed, err := number.Float64()
		return parsed, err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0)
	case float64:
		return number, !math.IsNaN(number) && !math.IsInf(number, 0)
	default:
		return 0, false
	}
}

func patrolJSONEvidenceValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(typed)
	case json.Number:
		return typed.String()
	case string:
		cleaned := strings.Map(func(r rune) rune {
			if r < 0x20 || r == 0x7f {
				return -1
			}
			return r
		}, typed)
		if len(cleaned) > 96 {
			cleaned = cleaned[:96]
		}
		return strconv.Quote(cleaned)
	default:
		return fmt.Sprintf("type_%T", value)
	}
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
