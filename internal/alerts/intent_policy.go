package alerts

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrAlertIntentPolicyRevisionConflict = errors.New("alert_intent_policy_revision_conflict")

type OperatorIntentContext struct {
	IntentionallyOffline bool       `json:"intentionallyOffline"`
	MaintenanceStartAt   *time.Time `json:"maintenanceStartAt,omitempty"`
	MaintenanceEndAt     *time.Time `json:"maintenanceEndAt,omitempty"`
	MaintenanceReason    string     `json:"maintenanceReason,omitempty"`
}

func (c OperatorIntentContext) MaintenanceActiveAt(now time.Time) bool {
	return c.MaintenanceStartAt != nil && c.MaintenanceEndAt != nil &&
		!now.Before(*c.MaintenanceStartAt) && now.Before(*c.MaintenanceEndAt)
}

type BackupIntentContext struct {
	Active     bool      `json:"active"`
	ObservedAt time.Time `json:"observedAt,omitempty"`
	Evidence   string    `json:"evidence,omitempty"`
}

type OperatorIntentContextResolver func(resourceID string, now time.Time) (OperatorIntentContext, bool)
type BackupIntentContextResolver func(resourceID, instance, node string, vmid int, now time.Time) (BackupIntentContext, bool)
type ResourceIntentIdentityResolver func(resourceID string) (canonicalID string, found bool)

type IntentPendingState struct {
	TrackingKey             string     `json:"trackingKey"`
	ResourceID              string     `json:"resourceId"`
	ResourceType            string     `json:"resourceType"`
	Signal                  string     `json:"signal"`
	FirstMatchedAt          time.Time  `json:"firstMatchedAt"`
	LastObservedAt          time.Time  `json:"lastObservedAt"`
	ElapsedNanos            int64      `json:"elapsedNanos,omitempty"`
	BackupActive            bool       `json:"backupActive,omitempty"`
	BackupEndedAt           *time.Time `json:"backupEndedAt,omitempty"`
	BackupEndedElapsedNanos *int64     `json:"backupEndedElapsedNanos,omitempty"`
	BackupEvidence          string     `json:"backupEvidence,omitempty"`
}

type EffectiveAlertIntentPolicy struct {
	GraceSeconds       int                        `json:"graceSeconds"`
	HonorOperatorState bool                       `json:"honorOperatorState"`
	BackupOffline      *BackupOfflineIntentPolicy `json:"backupOffline,omitempty"`
	Sources            map[string]string          `json:"sources"`
	Explicit           bool                       `json:"explicit"`
}

type AlertIntentPolicyPreviewRequest struct {
	ResourceID       string     `json:"resourceId"`
	ResourceType     string     `json:"resourceType"`
	Signal           string     `json:"signal"`
	ConditionActive  bool       `json:"conditionActive"`
	FirstMatchedAt   *time.Time `json:"firstMatchedAt,omitempty"`
	BackupActive     *bool      `json:"backupActive,omitempty"`
	BackupObservedAt *time.Time `json:"backupObservedAt,omitempty"`
}

type AlertIntentPolicyPreviewContext struct {
	Kind       string     `json:"kind"`
	Active     bool       `json:"active"`
	Evidence   string     `json:"evidence,omitempty"`
	ObservedAt *time.Time `json:"observedAt,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
}

type AlertIntentPolicyPreview struct {
	ResourceID     string                            `json:"resourceId"`
	ResourceType   string                            `json:"resourceType"`
	Signal         string                            `json:"signal"`
	Status         string                            `json:"status"`
	Reason         string                            `json:"reason"`
	Effective      EffectiveAlertIntentPolicy        `json:"effective"`
	FirstMatchedAt *time.Time                        `json:"firstMatchedAt,omitempty"`
	EligibleAt     *time.Time                        `json:"eligibleAt,omitempty"`
	HardCapAt      *time.Time                        `json:"hardCapAt,omitempty"`
	RemainingSecs  int64                             `json:"remainingSeconds,omitempty"`
	Contexts       []AlertIntentPolicyPreviewContext `json:"contexts"`
	Warnings       []string                          `json:"warnings"`
}

type intentDecision struct {
	ShouldActivate bool
	Pending        bool
	Suppressed     bool
	Reason         string
	EligibleAt     time.Time
	HardCapAt      time.Time
	StateChanged   bool
	Effective      EffectiveAlertIntentPolicy
}

func (m *Manager) SetOperatorIntentContextResolver(resolver OperatorIntentContextResolver) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.operatorIntentResolver = resolver
	m.mu.Unlock()
}

func (m *Manager) SetBackupIntentContextResolver(resolver BackupIntentContextResolver) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.backupIntentResolver = resolver
	m.mu.Unlock()
}

func (m *Manager) SetResourceIntentIdentityResolver(resolver ResourceIntentIdentityResolver) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.resourceIntentResolver = resolver
	m.mu.Unlock()
}

// LoadIntentPolicies installs a persisted policy document without changing its
// revision. API writes should use UpdateIntentPolicies.
func (m *Manager) LoadIntentPolicies(document AlertIntentPolicyDocument) error {
	if err := ValidateAlertIntentPolicyDocument(document); err != nil {
		return err
	}
	if m == nil {
		return errors.New("alert manager is nil")
	}
	m.mu.Lock()
	m.intentPolicies = NormalizeAlertIntentPolicyDocument(document)
	m.mu.Unlock()
	return nil
}

func (m *Manager) GetIntentPolicies() AlertIntentPolicyDocument {
	if m == nil {
		return NewAlertIntentPolicyDocument()
	}
	m.mu.RLock()
	document := NormalizeAlertIntentPolicyDocument(m.intentPolicies)
	m.mu.RUnlock()
	return document
}

func (m *Manager) UpdateIntentPolicies(document AlertIntentPolicyDocument) (AlertIntentPolicyDocument, error) {
	if err := ValidateAlertIntentPolicyDocument(document); err != nil {
		return AlertIntentPolicyDocument{}, err
	}
	if m == nil {
		return AlertIntentPolicyDocument{}, errors.New("alert manager is nil")
	}

	now := m.policyNow().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()
	if document.Revision != m.intentPolicies.Revision {
		return AlertIntentPolicyDocument{}, fmt.Errorf("%w: got %d want %d", ErrAlertIntentPolicyRevisionConflict, document.Revision, m.intentPolicies.Revision)
	}
	normalized := NormalizeAlertIntentPolicyDocument(document)
	normalized.Revision = m.intentPolicies.Revision + 1
	normalized.UpdatedAt = &now
	m.intentPolicies = normalized
	return NormalizeAlertIntentPolicyDocument(normalized), nil
}

func (m *Manager) resolveEffectiveIntentPolicyNoLock(resourceID, resourceType, signal string) EffectiveAlertIntentPolicy {
	resourceID = strings.TrimSpace(resourceID)
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	signal = strings.ToLower(strings.TrimSpace(signal))
	effective := EffectiveAlertIntentPolicy{Sources: make(map[string]string)}

	if strings.HasPrefix(signal, "metric.") {
		metric := strings.TrimPrefix(signal, "metric.")
		if delay, source, ok := m.getLegacyTimeThresholdWithSource(resourceType, metric); ok {
			effective.GraceSeconds = delay
			effective.Sources["graceSeconds"] = source
		}
	}

	apply := func(rule AlertIntentRule, source string) {
		if rule.GraceSeconds != nil {
			effective.GraceSeconds = *rule.GraceSeconds
			effective.Sources["graceSeconds"] = source
			effective.Explicit = true
		}
		if rule.HonorOperatorState != nil {
			effective.HonorOperatorState = *rule.HonorOperatorState
			effective.Sources["honorOperatorState"] = source
			effective.Explicit = true
		}
		if rule.BackupOffline != nil {
			backup := *rule.BackupOffline
			effective.BackupOffline = &backup
			effective.Sources["backupOffline"] = source
			effective.Explicit = true
		}
	}
	applyRules := func(rules map[string]AlertIntentRule, source string) {
		if rule, ok := rules[string(AlertIntentSignalDefault)]; ok {
			apply(rule, source+".*")
		}
		if rule, ok := rules[signal]; ok {
			apply(rule, source+"."+signal)
		}
	}

	applyRules(m.intentPolicies.Defaults, "defaults")
	typeKeys := CanonicalResourceTypeKeys(resourceType)
	for i := len(typeKeys) - 1; i >= 0; i-- {
		typeKey := typeKeys[i]
		if rules, ok := m.intentPolicies.ResourceTypes[typeKey]; ok {
			applyRules(rules, "resourceTypes."+typeKey)
		}
	}
	resourceIDs := make([]string, 0, 2)
	if m.resourceIntentResolver != nil {
		if canonicalID, ok := m.resourceIntentResolver(resourceID); ok {
			canonicalID = strings.TrimSpace(canonicalID)
			if canonicalID != "" && canonicalID != resourceID {
				resourceIDs = append(resourceIDs, canonicalID)
			}
		}
	}
	resourceIDs = append(resourceIDs, resourceID)
	for _, candidateID := range resourceIDs {
		if rules, ok := m.intentPolicies.Resources[candidateID]; ok {
			applyRules(rules, "resources."+candidateID)
		}
	}
	if _, ok := effective.Sources["graceSeconds"]; !ok {
		effective.Sources["graceSeconds"] = "factory"
	}
	if _, ok := effective.Sources["honorOperatorState"]; !ok {
		effective.Sources["honorOperatorState"] = "factory"
	}
	return effective
}

func (m *Manager) ResolveEffectiveIntentPolicy(resourceID, resourceType, signal string) EffectiveAlertIntentPolicy {
	if m == nil {
		return EffectiveAlertIntentPolicy{Sources: map[string]string{"graceSeconds": "factory", "honorOperatorState": "factory"}}
	}
	m.mu.RLock()
	effective := m.resolveEffectiveIntentPolicyNoLock(resourceID, resourceType, signal)
	m.mu.RUnlock()
	return effective
}

func (m *Manager) intentTickNoLock() time.Duration {
	if m.intentClock == nil {
		return 0
	}
	return m.intentClock()
}

func (m *Manager) clearIntentPendingNoLock(trackingKey string) bool {
	_, pending := m.intentPending[trackingKey]
	_, ticking := m.intentRuntimeTicks[trackingKey]
	delete(m.intentPending, trackingKey)
	delete(m.intentRuntimeTicks, trackingKey)
	return pending || ticking
}

func (m *Manager) advanceIntentElapsedNoLock(trackingKey string, state *IntentPendingState, tick time.Duration) {
	if state == nil {
		return
	}
	if previous, ok := m.intentRuntimeTicks[trackingKey]; ok && tick >= previous {
		delta := tick - previous
		if delta > 0 && state.ElapsedNanos <= int64(^uint64(0)>>1)-int64(delta) {
			state.ElapsedNanos += int64(delta)
		}
	}
	m.intentRuntimeTicks[trackingKey] = tick
}

func durationUntil(total, elapsed time.Duration) time.Duration {
	if total <= elapsed {
		return 0
	}
	return total - elapsed
}

func absoluteDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func intentTimePointer(value time.Time) *time.Time {
	return &value
}

func (m *Manager) evaluateIntentNoLock(resourceID, resourceType, signal, trackingKey string, _ time.Time, conditionActive bool, backup BackupIntentContext) intentDecision {
	effective := m.resolveEffectiveIntentPolicyNoLock(resourceID, resourceType, signal)
	decision := intentDecision{Effective: effective}
	if !conditionActive {
		if m.clearIntentPendingNoLock(trackingKey) {
			decision.StateChanged = true
		}
		decision.Reason = "condition_clear"
		return decision
	}
	if !effective.Explicit {
		if m.clearIntentPendingNoLock(trackingKey) {
			decision.StateChanged = true
		}
		decision.ShouldActivate = true
		return decision
	}
	observedAt := m.policyNow().UTC()
	tick := m.intentTickNoLock()
	state, exists := m.intentPending[trackingKey]
	if !exists || state.FirstMatchedAt.IsZero() {
		state = IntentPendingState{
			TrackingKey: trackingKey, ResourceID: resourceID, ResourceType: resourceType,
			Signal: signal, FirstMatchedAt: observedAt,
		}
		m.intentRuntimeTicks[trackingKey] = tick
		decision.StateChanged = true
	} else {
		previousElapsed := state.ElapsedNanos
		m.advanceIntentElapsedNoLock(trackingKey, &state, tick)
		if state.ElapsedNanos != previousElapsed {
			decision.StateChanged = true
		}
	}
	state.LastObservedAt = observedAt
	elapsed := time.Duration(state.ElapsedNanos)
	projectedFirstMatchedAt := observedAt.Add(-elapsed)
	if absoluteDuration(state.FirstMatchedAt.Add(elapsed).Sub(observedAt)) > time.Minute {
		state.FirstMatchedAt = projectedFirstMatchedAt
		if state.BackupEndedElapsedNanos != nil {
			backupEndedElapsed := time.Duration(*state.BackupEndedElapsedNanos)
			state.BackupEndedAt = intentTimePointer(observedAt.Add(-(elapsed - backupEndedElapsed)))
		}
		decision.StateChanged = true
	}

	if effective.HonorOperatorState && m.operatorIntentResolver != nil {
		if operator, ok := m.operatorIntentResolver(resourceID, observedAt); ok {
			if operator.MaintenanceActiveAt(observedAt) {
				m.intentPending[trackingKey] = state
				decision.Pending = true
				decision.Suppressed = true
				decision.Reason = "operator_maintenance"
				if operator.MaintenanceEndAt != nil {
					decision.EligibleAt = *operator.MaintenanceEndAt
				}
				return decision
			}
			if signal == string(AlertIntentSignalOffline) && operator.IntentionallyOffline {
				m.intentPending[trackingKey] = state
				decision.Pending = true
				decision.Suppressed = true
				decision.Reason = "operator_intentionally_offline"
				return decision
			}
		}
	}

	eligibleElapsed := time.Duration(effective.GraceSeconds) * time.Second
	if effective.BackupOffline != nil && effective.BackupOffline.Enabled && signal == string(AlertIntentSignalOffline) {
		if backup.Active {
			if !state.BackupActive || state.BackupEvidence != backup.Evidence {
				decision.StateChanged = true
			}
			state.BackupActive = true
			state.BackupEndedAt = nil
			state.BackupEndedElapsedNanos = nil
			state.BackupEvidence = backup.Evidence
		} else if state.BackupActive {
			endedAt := observedAt
			endedElapsed := state.ElapsedNanos
			state.BackupActive = false
			state.BackupEndedAt = &endedAt
			state.BackupEndedElapsedNanos = &endedElapsed
			decision.StateChanged = true
		}

		capElapsed := time.Duration(effective.BackupOffline.MaxDeferralSeconds) * time.Second
		capAt := observedAt.Add(durationUntil(capElapsed, elapsed))
		decision.HardCapAt = capAt
		if state.BackupActive && elapsed < capElapsed {
			m.intentPending[trackingKey] = state
			decision.Pending = true
			decision.Suppressed = true
			decision.Reason = "backup_active"
			decision.EligibleAt = capAt
			return decision
		}
		if state.BackupEndedElapsedNanos != nil {
			postEligibleElapsed := time.Duration(*state.BackupEndedElapsedNanos) + time.Duration(effective.BackupOffline.PostGraceSeconds)*time.Second
			if postEligibleElapsed > eligibleElapsed {
				eligibleElapsed = postEligibleElapsed
			}
		}
		if eligibleElapsed > capElapsed {
			eligibleElapsed = capElapsed
		}
		if elapsed >= capElapsed {
			decision.Reason = "backup_grace_cap_exceeded"
		}
	}

	m.intentPending[trackingKey] = state
	eligibleAt := observedAt.Add(durationUntil(eligibleElapsed, elapsed))
	decision.EligibleAt = eligibleAt
	if elapsed < eligibleElapsed {
		decision.Pending = true
		if decision.Reason == "" {
			decision.Reason = "grace_period"
		}
		return decision
	}
	decision.ShouldActivate = true
	if decision.Reason == "" {
		decision.Reason = "eligible"
	}
	return decision
}

func (m *Manager) PreviewIntentPolicy(request AlertIntentPolicyPreviewRequest) (AlertIntentPolicyPreview, error) {
	request.ResourceID = strings.TrimSpace(request.ResourceID)
	request.ResourceType = strings.ToLower(strings.TrimSpace(request.ResourceType))
	request.Signal = strings.ToLower(strings.TrimSpace(request.Signal))
	if request.ResourceID == "" || request.ResourceType == "" || request.Signal == "" {
		return AlertIntentPolicyPreview{}, errors.New("resourceId, resourceType, and signal are required")
	}
	if !ValidAlertIntentSignal(request.Signal) {
		return AlertIntentPolicyPreview{}, fmt.Errorf("unsupported alert intent signal %q", request.Signal)
	}
	now := m.policyNow().UTC()
	trackingKey := "preview:" + request.ResourceID + ":" + request.Signal
	backup := BackupIntentContext{}
	if request.BackupActive != nil {
		backup.Active = *request.BackupActive
		backup.Evidence = "preview"
		if request.BackupObservedAt != nil {
			backup.ObservedAt = request.BackupObservedAt.UTC()
		}
	}

	m.mu.Lock()
	previous, hadPrevious := m.intentPending[trackingKey]
	previousTick, hadPreviousTick := m.intentRuntimeTicks[trackingKey]
	if request.FirstMatchedAt != nil {
		firstMatchedAt := request.FirstMatchedAt.UTC()
		elapsed := now.Sub(firstMatchedAt)
		if elapsed < 0 {
			elapsed = 0
		}
		m.intentPending[trackingKey] = IntentPendingState{
			TrackingKey: trackingKey, ResourceID: request.ResourceID, ResourceType: request.ResourceType,
			Signal: request.Signal, FirstMatchedAt: firstMatchedAt, LastObservedAt: now, ElapsedNanos: int64(elapsed),
		}
		m.intentRuntimeTicks[trackingKey] = m.intentTickNoLock()
	}
	decision := m.evaluateIntentNoLock(request.ResourceID, request.ResourceType, request.Signal, trackingKey, now, request.ConditionActive, backup)
	state, stateExists := m.intentPending[trackingKey]
	operator := OperatorIntentContext{}
	operatorFound := false
	if decision.Effective.HonorOperatorState && m.operatorIntentResolver != nil {
		operator, operatorFound = m.operatorIntentResolver(request.ResourceID, now)
	}
	if hadPrevious {
		m.intentPending[trackingKey] = previous
	} else {
		delete(m.intentPending, trackingKey)
	}
	if hadPreviousTick {
		m.intentRuntimeTicks[trackingKey] = previousTick
	} else {
		delete(m.intentRuntimeTicks, trackingKey)
	}
	m.mu.Unlock()

	preview := AlertIntentPolicyPreview{
		ResourceID: request.ResourceID, ResourceType: request.ResourceType, Signal: request.Signal,
		Effective: decision.Effective, Contexts: []AlertIntentPolicyPreviewContext{}, Warnings: []string{},
	}
	if stateExists && !state.FirstMatchedAt.IsZero() {
		first := state.FirstMatchedAt
		preview.FirstMatchedAt = &first
	}
	if !decision.EligibleAt.IsZero() {
		eligible := decision.EligibleAt
		preview.EligibleAt = &eligible
		if eligible.After(now) {
			preview.RemainingSecs = int64(eligible.Sub(now).Seconds())
		}
	}
	if !decision.HardCapAt.IsZero() {
		capAt := decision.HardCapAt
		preview.HardCapAt = &capAt
	}
	switch {
	case !request.ConditionActive:
		preview.Status, preview.Reason = "clear", "condition_clear"
	case decision.Suppressed:
		preview.Status, preview.Reason = "expected_transient", decision.Reason
	case decision.Pending:
		preview.Status, preview.Reason = "pending_grace", decision.Reason
	default:
		preview.Status, preview.Reason = "would_activate", decision.Reason
	}
	if request.BackupActive != nil {
		ctx := AlertIntentPolicyPreviewContext{Kind: "backup", Active: *request.BackupActive, Evidence: backup.Evidence}
		if !backup.ObservedAt.IsZero() {
			observedAt := backup.ObservedAt
			ctx.ObservedAt = &observedAt
		}
		preview.Contexts = append(preview.Contexts, ctx)
	}
	if operatorFound {
		active := operator.IntentionallyOffline || operator.MaintenanceActiveAt(now)
		ctx := AlertIntentPolicyPreviewContext{Kind: "operator_state", Active: active, Evidence: operator.MaintenanceReason}
		if operator.MaintenanceEndAt != nil {
			expiresAt := operator.MaintenanceEndAt.UTC()
			ctx.ExpiresAt = &expiresAt
		}
		preview.Contexts = append(preview.Contexts, ctx)
	}
	if request.Signal == string(AlertIntentSignalAvailability) {
		preview.Warnings = append(preview.Warnings, "Availability probe failure thresholds are evaluated before alert grace.")
	}
	return preview, nil
}
