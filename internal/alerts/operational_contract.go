package alerts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

const legacyAlertEvidenceReason = "legacy_alert_projection"
const legacyAlertRecoveryEvidenceReason = "legacy_alert_recovery_projection"
const maxOperationalEvidencePerAlert = 64
const maxOperationalTransitionsPerAlert = 128

func ensureOperationalContract(alert *Alert, ingestedAt time.Time) {
	if alert == nil {
		return
	}
	if ingestedAt.IsZero() {
		ingestedAt = time.Now()
	}
	backfillCanonicalIdentity(alert)

	if len(alert.Evidence) == 0 {
		if envelope, ok := legacyAlertEvidenceEnvelope(alert, ingestedAt); ok {
			alert.Evidence = []operationaltrust.EvidenceEnvelope{envelope}
		}
	}
	if strings.TrimSpace(alert.ResourceID) == "" {
		return
	}

	evidenceIDs := operationalEvidenceIDs(alert.Evidence)
	state := operationalStateForAlert(alert)
	severity := operationalSeverityForAlert(alert)
	firstObservedAt := alert.StartTime
	if firstObservedAt.IsZero() {
		firstObservedAt = alert.LastSeen
	}
	if firstObservedAt.IsZero() {
		firstObservedAt = ingestedAt
	}
	lastObservedAt := alert.LastSeen
	if lastObservedAt.IsZero() || lastObservedAt.Before(firstObservedAt) {
		lastObservedAt = firstObservedAt
	}
	causeKey := strings.TrimSpace(alert.CanonicalState)
	if causeKey == "" {
		causeKey = strings.TrimSpace(alert.ID)
	}
	specID := strings.TrimSpace(alert.CanonicalSpecID)
	if specID == "" {
		specID = "legacy-alert:" + strings.TrimSpace(alert.Type)
	}
	recordID := causeKey
	if recordID == "" {
		recordID = specID + ":" + strings.TrimSpace(alert.ResourceID)
	}

	record := alert.OperationalRecord
	previousState := operationaltrust.OperationalObserving
	hadRecord := record != nil
	if hadRecord {
		previousState = record.State
	}
	if record == nil {
		record = &operationaltrust.OperationalRecord{
			ID:                 recordID,
			CanonicalSpecID:    specID,
			SubjectResourceID:  strings.TrimSpace(alert.ResourceID),
			FirstObservedAt:    firstObservedAt,
			StateChangedAt:     firstObservedAt,
			CauseKey:           causeKey,
			RelatedResourceIDs: []string{},
		}
		alert.OperationalRecord = record
	}
	record.ID = recordID
	record.CanonicalSpecID = specID
	record.SubjectResourceID = strings.TrimSpace(alert.ResourceID)
	record.State = state
	record.Severity = severity
	record.FirstObservedAt = firstObservedAt
	record.LastObservedAt = lastObservedAt
	record.EvidenceIDs = evidenceIDs
	record.CauseKey = causeKey
	record.ImpactSummary = strings.TrimSpace(alert.Message)
	if strings.TrimSpace(record.RecommendedNextStep) == "" {
		record.RecommendedNextStep = operationalRecommendedNextStep(alert)
	}
	if state != operationaltrust.OperationalResolved {
		record.ResolvedAt = nil
	}
	if record.RelatedResourceIDs == nil {
		record.RelatedResourceIDs = []string{}
	}

	if alert.Acknowledged {
		acknowledgedAt := lastObservedAt
		if alert.AckTime != nil && !alert.AckTime.IsZero() {
			acknowledgedAt = *alert.AckTime
		}
		record.StateChangedAt = acknowledgedAt
		if hadRecord && previousState != operationaltrust.OperationalAcknowledged &&
			acknowledgedAt.Before(lastObservedAt) {
			record.StateChangedAt = ingestedAt
		}
		record.Acknowledgement = &operationaltrust.Acknowledgement{
			At: acknowledgedAt,
			By: strings.TrimSpace(alert.AckUser),
		}
		if record.Acknowledgement.By == "" {
			record.Acknowledgement.By = "unknown"
		}
	} else {
		record.Acknowledgement = nil
		if hadRecord && previousState != state {
			record.StateChangedAt = ingestedAt
		} else if record.StateChangedAt.IsZero() ||
			record.StateChangedAt.Before(firstObservedAt) ||
			record.StateChangedAt.After(lastObservedAt) {
			record.StateChangedAt = firstObservedAt
		}
	}

	if alert.LatestTransition == nil ||
		alert.LatestTransition.OperationalRecordID != record.ID ||
		alert.LatestTransition.To != record.State {
		alert.LatestTransition = buildCurrentTransition(record, previousState)
	}
	if alert.LatestTransition != nil {
		alert.Transitions = appendOperationalTransition(
			alert.Transitions,
			alert.LatestTransition.Clone(),
		)
	}
}

func operationalRecommendedNextStep(alert *Alert) string {
	if alert == nil {
		return ""
	}
	if action, ok := alert.Metadata["incidentAction"].(string); ok {
		if action = strings.TrimSpace(action); action != "" {
			return action
		}
	}
	return "Open the affected resource and verify its current state before making changes."
}

func mergeOperationalRecurrence(
	target *Alert,
	previous *Alert,
	observedAt time.Time,
) bool {
	if target == nil ||
		previous == nil ||
		previous.OperationalRecord == nil ||
		previous.OperationalRecord.State != operationaltrust.OperationalResolved {
		return false
	}

	currentEvidence := make([]operationaltrust.EvidenceEnvelope, len(target.Evidence))
	for index := range target.Evidence {
		currentEvidence[index] = target.Evidence[index].Clone()
	}
	acknowledged := target.Acknowledged
	ackUser := target.AckUser
	var originalAckTime *time.Time
	if target.AckTime != nil {
		value := *target.AckTime
		originalAckTime = &value
	}
	record := previous.OperationalRecord.Clone()
	target.OperationalRecord = &record
	if previous.LatestTransition != nil {
		transition := previous.LatestTransition.Clone()
		target.LatestTransition = &transition
	}
	target.Transitions = nil
	for _, transition := range previous.Transitions {
		target.Transitions = appendOperationalTransition(
			target.Transitions,
			transition.Clone(),
		)
	}
	target.Evidence = nil
	for _, envelope := range previous.Evidence {
		target.Evidence = appendOperationalEvidence(
			target.Evidence,
			envelope.Clone(),
		)
	}
	for _, envelope := range currentEvidence {
		target.Evidence = appendOperationalEvidence(target.Evidence, envelope)
	}
	target.Acknowledged = false
	target.AckTime = nil
	target.AckUser = ""
	ensureOperationalContract(target, observedAt)
	reopened := target.LatestTransition != nil &&
		target.LatestTransition.From == operationaltrust.OperationalResolved &&
		target.LatestTransition.To == operationaltrust.OperationalOpen
	if !reopened || !acknowledged {
		return reopened
	}

	reappliedAt := observedAt
	target.Acknowledged = true
	target.AckTime = &reappliedAt
	target.AckUser = ackUser
	ensureOperationalContract(target, observedAt)
	target.AckTime = originalAckTime
	if target.OperationalRecord != nil &&
		target.OperationalRecord.Acknowledgement != nil &&
		originalAckTime != nil {
		target.OperationalRecord.Acknowledgement.At = *originalAckTime
	}
	return true
}

func newResolvedAlert(
	alert *Alert,
	resolvedAt time.Time,
	recoveryEvidence *operationaltrust.EvidenceEnvelope,
) *ResolvedAlert {
	if alert == nil {
		return nil
	}
	if resolvedAt.IsZero() {
		resolvedAt = time.Now()
	}
	if recoveryEvidence == nil {
		if envelope, ok := legacyAlertRecoveryEvidenceEnvelope(alert, resolvedAt); ok {
			recoveryEvidence = &envelope
		}
	}
	markOperationalResolved(alert, resolvedAt, recoveryEvidence)
	return &ResolvedAlert{
		Alert:        alert,
		ResolvedTime: resolvedAt,
	}
}

func markOperationalResolved(
	alert *Alert,
	resolvedAt time.Time,
	recoveryEvidence *operationaltrust.EvidenceEnvelope,
) {
	if alert == nil {
		return
	}
	ensureOperationalContract(alert, resolvedAt)
	record := alert.OperationalRecord
	if record == nil || recoveryEvidence == nil || recoveryEvidence.Validate() != nil {
		return
	}
	alert.Evidence = appendOperationalEvidence(alert.Evidence, recoveryEvidence.Clone())
	recoveryEvidenceIDs := []string{recoveryEvidence.ID}
	from := record.State
	record.State = operationaltrust.OperationalResolved
	record.StateChangedAt = resolvedAt
	record.ResolvedAt = &resolvedAt
	record.EvidenceIDs = operationalEvidenceIDs(alert.Evidence)
	id, err := operationaltrust.NewTransitionID(
		record.ID,
		from,
		operationaltrust.OperationalResolved,
		resolvedAt,
		operationaltrust.TransitionRecoveryEvidence,
		record.CauseKey,
		recoveryEvidenceIDs,
	)
	if err != nil {
		return
	}
	alert.LatestTransition = &operationaltrust.LifecycleTransition{
		ID:                  id,
		OperationalRecordID: record.ID,
		From:                from,
		To:                  operationaltrust.OperationalResolved,
		At:                  resolvedAt,
		Cause:               operationaltrust.TransitionRecoveryEvidence,
		CauseKey:            record.CauseKey,
		EvidenceIDs:         recoveryEvidenceIDs,
	}
	alert.Transitions = appendOperationalTransition(
		alert.Transitions,
		alert.LatestTransition.Clone(),
	)
}

func (m *Manager) newResolvedAlert(
	alert *Alert,
	resolvedAt time.Time,
	recoveryEvidence *operationaltrust.EvidenceEnvelope,
) *ResolvedAlert {
	resolved := newResolvedAlert(alert, resolvedAt, recoveryEvidence)
	if resolved != nil && m != nil && m.historyManager != nil {
		m.historyManager.UpdateAlertOperationalContractForAlert(alert)
	}
	return resolved
}

func legacyAlertEvidenceEnvelope(
	alert *Alert,
	ingestedAt time.Time,
) (operationaltrust.EvidenceEnvelope, bool) {
	observedAt := alert.LastSeen
	if observedAt.IsZero() {
		observedAt = alert.StartTime
	}
	if observedAt.IsZero() {
		observedAt = ingestedAt
	}
	source := operationaltrust.EvidenceSource{
		Provider:  "pulse",
		Collector: "legacy-alert-adapter",
		Instance:  strings.TrimSpace(alert.Instance),
	}
	subject := operationaltrust.EvidenceSubject{
		ResourceID: strings.TrimSpace(alert.ResourceID),
	}
	if subject.ResourceID == "" {
		subject.ProviderRef = strings.TrimSpace(alert.ID)
		if subject.ProviderRef == "" {
			return operationaltrust.EvidenceEnvelope{}, false
		}
		subject.ProviderScope = strings.TrimSpace(alert.Instance)
		if subject.ProviderScope == "" {
			subject.ProviderScope = "pulse"
		}
	}
	sourceObservationID := strings.TrimSpace(alert.ID)
	if sourceObservationID == "" {
		sourceObservationID = strings.TrimSpace(alert.CanonicalState)
	}
	if sourceObservationID == "" {
		return operationaltrust.EvidenceEnvelope{}, false
	}
	id, err := operationaltrust.NewEvidenceID(source, subject, observedAt, sourceObservationID)
	if err != nil {
		return operationaltrust.EvidenceEnvelope{}, false
	}
	envelope := operationaltrust.EvidenceEnvelope{
		ID:           id,
		Source:       source,
		Subject:      subject,
		ObservedAt:   observedAt,
		IngestedAt:   ingestedAt,
		Completeness: operationaltrust.EvidencePartial,
		Confidence:   operationaltrust.EvidenceUnknown,
		Permissions:  operationaltrust.EvidencePermissionsUnknown,
		Reason: &operationaltrust.EvidenceReason{
			Code: legacyAlertEvidenceReason,
		},
	}
	return envelope, envelope.Validate() == nil
}

func applyCanonicalOperationalEvidence(
	alert *Alert,
	spec alertspecs.ResourceAlertSpec,
	evidence alertspecs.AlertEvidence,
	ingestedAt time.Time,
) bool {
	if alert == nil {
		return false
	}
	envelope, ok := canonicalAlertEvidenceEnvelope(spec, evidence, alert.Instance, ingestedAt)
	if !ok {
		return false
	}
	alert.Evidence = appendOperationalEvidence(alert.Evidence, envelope)
	return true
}

func canonicalAlertEvidenceEnvelope(
	spec alertspecs.ResourceAlertSpec,
	evidence alertspecs.AlertEvidence,
	instance string,
	ingestedAt time.Time,
) (operationaltrust.EvidenceEnvelope, bool) {
	if evidence.Envelope != nil {
		envelope := evidence.Envelope.Clone()
		if envelope.Subject.ResourceID != spec.ResourceID ||
			!envelope.ObservedAt.Equal(evidence.ObservedAt) ||
			envelope.Validate() != nil {
			return operationaltrust.EvidenceEnvelope{}, false
		}
		return envelope, true
	}
	payload, err := json.Marshal(evidence)
	if err != nil {
		return operationaltrust.EvidenceEnvelope{}, false
	}
	digest := sha256.Sum256(payload)
	digestID := hex.EncodeToString(digest[:])
	provider := "pulse"
	collector := "canonical-alert:" + string(spec.Kind)
	if evidence.ProviderIncident != nil {
		if value := strings.TrimSpace(evidence.ProviderIncident.Provider); value != "" {
			provider = value
		}
		if value := strings.TrimSpace(evidence.ProviderIncident.Source); value != "" {
			collector = value
		}
	}
	source := operationaltrust.EvidenceSource{
		Provider:  provider,
		Collector: collector,
		Instance:  strings.TrimSpace(instance),
	}
	subject := operationaltrust.EvidenceSubject{ResourceID: spec.ResourceID}
	id, err := operationaltrust.NewEvidenceID(
		source,
		subject,
		evidence.ObservedAt,
		digestID,
	)
	if err != nil {
		return operationaltrust.EvidenceEnvelope{}, false
	}
	envelope := operationaltrust.EvidenceEnvelope{
		ID:           id,
		Source:       source,
		Subject:      subject,
		ObservedAt:   evidence.ObservedAt,
		IngestedAt:   ingestedAt,
		Completeness: operationaltrust.EvidenceComplete,
		Confidence:   operationaltrust.EvidenceConfirmed,
		Permissions:  operationaltrust.EvidencePermissionsSufficient,
		PayloadRef: &operationaltrust.EvidencePayloadRef{
			Kind: "alert-evidence-digest",
			ID:   digestID,
		},
	}
	return envelope, envelope.Validate() == nil
}

func legacyAlertRecoveryEvidenceEnvelope(
	alert *Alert,
	resolvedAt time.Time,
) (operationaltrust.EvidenceEnvelope, bool) {
	if alert == nil || strings.TrimSpace(alert.ResourceID) == "" {
		return operationaltrust.EvidenceEnvelope{}, false
	}
	source := operationaltrust.EvidenceSource{
		Provider:  "pulse",
		Collector: "legacy-alert-recovery-adapter",
		Instance:  strings.TrimSpace(alert.Instance),
	}
	subject := operationaltrust.EvidenceSubject{
		ResourceID: strings.TrimSpace(alert.ResourceID),
	}
	sourceObservationID := strings.TrimSpace(alert.ID)
	if sourceObservationID == "" {
		sourceObservationID = strings.TrimSpace(alert.CanonicalState)
	}
	if sourceObservationID == "" {
		return operationaltrust.EvidenceEnvelope{}, false
	}
	id, err := operationaltrust.NewEvidenceID(
		source,
		subject,
		resolvedAt,
		sourceObservationID+":resolved",
	)
	if err != nil {
		return operationaltrust.EvidenceEnvelope{}, false
	}
	envelope := operationaltrust.EvidenceEnvelope{
		ID:           id,
		Source:       source,
		Subject:      subject,
		ObservedAt:   resolvedAt,
		IngestedAt:   resolvedAt,
		Completeness: operationaltrust.EvidencePartial,
		Confidence:   operationaltrust.EvidenceUnknown,
		Permissions:  operationaltrust.EvidencePermissionsUnknown,
		Reason: &operationaltrust.EvidenceReason{
			Code: legacyAlertRecoveryEvidenceReason,
		},
	}
	return envelope, envelope.Validate() == nil
}

func operationalStateForAlert(alert *Alert) operationaltrust.OperationalState {
	if alert != nil && alert.Acknowledged {
		return operationaltrust.OperationalAcknowledged
	}
	return operationaltrust.OperationalOpen
}

func operationalSeverityForAlert(alert *Alert) operationaltrust.OperationalSeverity {
	if alert == nil {
		return operationaltrust.SeverityUnknown
	}
	switch alert.Level {
	case AlertLevelCritical:
		return operationaltrust.SeverityCritical
	case AlertLevelWarning:
		return operationaltrust.SeverityWarning
	default:
		return operationaltrust.SeverityUnknown
	}
}

func operationalEvidenceIDs(envelopes []operationaltrust.EvidenceEnvelope) []string {
	seen := make(map[string]struct{}, len(envelopes))
	result := make([]string, 0, len(envelopes))
	for _, envelope := range envelopes {
		id := strings.TrimSpace(envelope.ID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func appendOperationalEvidence(
	envelopes []operationaltrust.EvidenceEnvelope,
	envelope operationaltrust.EvidenceEnvelope,
) []operationaltrust.EvidenceEnvelope {
	for index := range envelopes {
		if envelopes[index].ID == envelope.ID {
			envelopes[index] = envelope
			return envelopes
		}
	}
	envelopes = append(envelopes, envelope)
	if len(envelopes) > maxOperationalEvidencePerAlert {
		envelopes = append(
			[]operationaltrust.EvidenceEnvelope(nil),
			envelopes[len(envelopes)-maxOperationalEvidencePerAlert:]...,
		)
	}
	return envelopes
}

func appendOperationalTransition(
	transitions []operationaltrust.LifecycleTransition,
	transition operationaltrust.LifecycleTransition,
) []operationaltrust.LifecycleTransition {
	for index := range transitions {
		if transitions[index].ID == transition.ID {
			transitions[index] = transition
			return transitions
		}
	}
	transitions = append(transitions, transition)
	if len(transitions) > maxOperationalTransitionsPerAlert {
		transitions = append(
			[]operationaltrust.LifecycleTransition(nil),
			transitions[len(transitions)-maxOperationalTransitionsPerAlert:]...,
		)
	}
	return transitions
}

func buildCurrentTransition(
	record *operationaltrust.OperationalRecord,
	previousState operationaltrust.OperationalState,
) *operationaltrust.LifecycleTransition {
	if record == nil {
		return nil
	}
	from := previousState
	cause := operationaltrust.TransitionDetectorDecision
	at := record.FirstObservedAt
	if record.State == operationaltrust.OperationalAcknowledged {
		cause = operationaltrust.TransitionAcknowledgement
		at = record.StateChangedAt
	} else if previousState == operationaltrust.OperationalAcknowledged &&
		record.State == operationaltrust.OperationalOpen {
		cause = operationaltrust.TransitionUnacknowledgement
		at = record.StateChangedAt
	}
	if from == record.State {
		return nil
	}
	id, err := operationaltrust.NewTransitionID(
		record.ID,
		from,
		record.State,
		at,
		cause,
		record.CauseKey,
		record.EvidenceIDs,
	)
	if err != nil {
		return nil
	}
	return &operationaltrust.LifecycleTransition{
		ID:                  id,
		OperationalRecordID: record.ID,
		From:                from,
		To:                  record.State,
		At:                  at,
		Cause:               cause,
		CauseKey:            record.CauseKey,
		EvidenceIDs:         append([]string(nil), record.EvidenceIDs...),
	}
}
