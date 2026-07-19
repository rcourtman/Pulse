package recovery

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

const (
	DefaultProtectionFreshnessWindow    = 7 * 24 * time.Hour
	DefaultProtectionVerificationWindow = 7 * 24 * time.Hour
)

var DefaultProtectionPosturePolicy = ProtectionPosturePolicy{
	FreshnessWindow:     DefaultProtectionFreshnessWindow,
	VerificationWindow:  DefaultProtectionVerificationWindow,
	RequireVerification: false,
}

func ProviderScopeForPoint(point RecoveryPoint) string {
	if scope := strings.TrimSpace(point.ProviderScope); scope != "" {
		return scope
	}
	switch point.Provider {
	case ProviderProxmoxPBS:
		if point.RepositoryRef != nil {
			if scope := strings.TrimSpace(point.RepositoryRef.Namespace); scope != "" {
				return scope
			}
		}
	case ProviderProxmoxPVE:
		if scope := recoveryDetailString(point, "instance"); scope != "" {
			return scope
		}
	case ProviderKubernetes:
		if scope := recoveryDetailString(point, "k8sClusterId"); scope != "" {
			return scope
		}
	case ProviderTrueNAS:
		if scope := recoveryDetailString(point, "connectionId"); scope != "" {
			return scope
		}
	}
	if point.SubjectRef != nil {
		if scope := strings.TrimSpace(point.SubjectRef.Namespace); scope != "" {
			return scope
		}
	}
	return "provider-default"
}

func NewRecoveryPointEvidence(
	point RecoveryPoint,
	collector string,
	ingestedAt time.Time,
) (*operationaltrust.EvidenceEnvelope, error) {
	collector = strings.TrimSpace(collector)
	if collector == "" {
		return nil, fmt.Errorf("recovery evidence collector is required")
	}
	observedAt := recoveryPointObservedAt(point)
	if observedAt.IsZero() {
		return nil, fmt.Errorf("recovery point observation time is required")
	}
	if ingestedAt.IsZero() {
		return nil, fmt.Errorf("recovery point ingestion time is required")
	}

	scope := ProviderScopeForPoint(point)
	source := operationaltrust.EvidenceSource{
		Provider:  string(point.Provider),
		Collector: collector,
		Instance:  scope,
	}
	subject := operationaltrust.EvidenceSubject{}
	if resourceID := strings.TrimSpace(point.SubjectResourceID); resourceID != "" {
		subject.ResourceID = resourceID
	} else {
		subject.ProviderRef = SubjectKeyForPoint(point)
		subject.ProviderScope = scope
	}
	evidenceID, err := operationaltrust.NewEvidenceID(
		source,
		subject,
		observedAt,
		point.ID,
	)
	if err != nil {
		return nil, err
	}

	envelope := &operationaltrust.EvidenceEnvelope{
		ID:           evidenceID,
		Source:       source,
		Subject:      subject,
		ObservedAt:   observedAt,
		IngestedAt:   ingestedAt.UTC(),
		Completeness: operationaltrust.EvidenceComplete,
		Confidence:   operationaltrust.EvidenceConfirmed,
		Permissions:  operationaltrust.EvidencePermissionsSufficient,
		PayloadRef: &operationaltrust.EvidencePayloadRef{
			Kind: "recovery-point",
			ID:   strings.TrimSpace(point.ID),
		},
	}
	if subject.ResourceID != "" && point.Provider == ProviderProxmoxPBS {
		index := DeriveIndex(point)
		matched := map[string]string{
			"providerScope": scope,
		}
		if value := strings.TrimSpace(index.ItemType); value != "" {
			matched["itemType"] = value
		}
		if value := strings.TrimSpace(index.EntityIDLabel); value != "" {
			matched["entityId"] = value
		}
		envelope.Confidence = operationaltrust.EvidenceInferred
		envelope.Reason = &operationaltrust.EvidenceReason{
			Code:    "provider_identity_correlation",
			Message: "The PBS subject was linked through a unique provider-scoped guest match.",
		}
		envelope.Correlation = &operationaltrust.IdentityCorrelation{
			Rule:           "provider_scoped_guest_identity",
			MatchedFields:  matched,
			CandidateCount: 1,
		}
	}
	if err := envelope.Validate(); err != nil {
		return nil, err
	}
	return envelope, nil
}

func NewProtectionProviderObservation(
	provider Provider,
	source string,
	scope string,
	jobState Outcome,
	historyCompleteness ProtectionHistoryCompleteness,
	permissions operationaltrust.EvidencePermissions,
	verificationExpected bool,
	observedAt time.Time,
	ingestedAt time.Time,
	reason *operationaltrust.EvidenceReason,
) (ProtectionProviderObservation, error) {
	source = strings.TrimSpace(source)
	scope = strings.TrimSpace(scope)
	evidenceSource := operationaltrust.EvidenceSource{
		Provider:  strings.TrimSpace(string(provider)),
		Collector: source,
		Instance:  scope,
	}
	evidenceSubject := operationaltrust.EvidenceSubject{
		ProviderRef:   strings.TrimSpace(string(provider)) + ":" + scope,
		ProviderScope: scope,
	}
	evidenceID, err := operationaltrust.NewEvidenceID(
		evidenceSource,
		evidenceSubject,
		observedAt,
		fmt.Sprintf(
			"%s:%s:%s:%s",
			jobState,
			historyCompleteness,
			permissions,
			scope,
		),
	)
	if err != nil {
		return ProtectionProviderObservation{}, err
	}

	completeness := operationaltrust.EvidenceUnavailable
	switch historyCompleteness {
	case ProtectionHistoryComplete:
		completeness = operationaltrust.EvidenceComplete
	case ProtectionHistoryPartial:
		completeness = operationaltrust.EvidencePartial
	}
	confidence := operationaltrust.EvidenceUnknown
	if historyCompleteness == ProtectionHistoryComplete &&
		permissions == operationaltrust.EvidencePermissionsSufficient {
		confidence = operationaltrust.EvidenceConfirmed
	}
	if (completeness != operationaltrust.EvidenceComplete ||
		confidence != operationaltrust.EvidenceConfirmed ||
		permissions != operationaltrust.EvidencePermissionsSufficient) &&
		reason == nil {
		reason = &operationaltrust.EvidenceReason{
			Code:    "provider_history_limited",
			Message: "Provider history could not support a complete protection assertion.",
		}
	}

	observation := ProtectionProviderObservation{
		ID:                   evidenceID,
		Provider:             provider,
		Source:               source,
		Scope:                scope,
		JobState:             jobState,
		HistoryCompleteness:  historyCompleteness,
		Permissions:          permissions,
		VerificationExpected: verificationExpected,
		ObservedAt:           observedAt.UTC(),
		IngestedAt:           ingestedAt.UTC(),
		Evidence: operationaltrust.EvidenceEnvelope{
			ID:           evidenceID,
			Source:       evidenceSource,
			Subject:      evidenceSubject,
			ObservedAt:   observedAt.UTC(),
			IngestedAt:   ingestedAt.UTC(),
			Completeness: completeness,
			Confidence:   confidence,
			Reason:       reason,
			Permissions:  permissions,
			PayloadRef: &operationaltrust.EvidencePayloadRef{
				Kind: "protection-provider-observation",
				ID:   evidenceID,
			},
		},
	}
	if err := observation.Validate(); err != nil {
		return ProtectionProviderObservation{}, err
	}
	return observation, nil
}

func BuildProtectionPostureFromPointsAt(
	subjectResourceID string,
	points []RecoveryPoint,
	observations []ProtectionProviderObservation,
	policy ProtectionPosturePolicy,
	now time.Time,
) ProtectionPosture {
	subjectResourceID = strings.TrimSpace(subjectResourceID)
	type aggregate struct {
		summary     ProtectionProviderSummary
		lastEventAt time.Time
	}
	byProviderScope := make(map[string]*aggregate)

	latestObservations := make(map[string]ProtectionProviderObservation)
	for _, observation := range observations {
		if err := observation.Validate(); err != nil {
			continue
		}
		key := providerScopeKey(observation.Provider, observation.Scope)
		current, exists := latestObservations[key]
		if !exists ||
			observation.ObservedAt.After(current.ObservedAt) ||
			(observation.ObservedAt.Equal(current.ObservedAt) && observation.ID > current.ID) {
			latestObservations[key] = observation.Clone()
		}
	}

	for _, point := range points {
		if strings.TrimSpace(point.SubjectResourceID) != subjectResourceID {
			continue
		}
		scope := ProviderScopeForPoint(point)
		key := providerScopeKey(point.Provider, scope)
		agg := byProviderScope[key]
		if agg == nil {
			agg = &aggregate{
				summary: ProtectionProviderSummary{
					Provider:            point.Provider,
					Source:              "legacy-recovery-point",
					Scope:               scope,
					JobState:            OutcomeUnknown,
					HistoryCompleteness: ProtectionHistoryUnknown,
					Permissions:         operationaltrust.EvidencePermissionsUnknown,
				},
			}
			byProviderScope[key] = agg
		}

		eventAt := recoveryPointObservedAt(point)
		if !eventAt.IsZero() {
			if agg.summary.LastAttemptAt == nil || eventAt.After(*agg.summary.LastAttemptAt) {
				value := eventAt.UTC()
				agg.summary.LastAttemptAt = &value
			}
			if agg.lastEventAt.IsZero() || eventAt.After(agg.lastEventAt) {
				agg.lastEventAt = eventAt
				if validRecoveryOutcome(point.Outcome) {
					agg.summary.JobState = point.Outcome
				} else {
					agg.summary.JobState = OutcomeUnknown
				}
			}
		}

		switch point.Kind {
		case KindBackup:
			agg.summary.BackupPointCount++
			if point.Outcome == OutcomeSuccess && !eventAt.IsZero() {
				if agg.summary.LastSuccessAt == nil || eventAt.After(*agg.summary.LastSuccessAt) {
					value := eventAt.UTC()
					agg.summary.LastSuccessAt = &value
				}
			}
			if point.Outcome == OutcomeSuccess &&
				point.Verified != nil &&
				*point.Verified &&
				!eventAt.IsZero() {
				if agg.summary.LastVerifiedAt == nil || eventAt.After(*agg.summary.LastVerifiedAt) {
					value := eventAt.UTC()
					agg.summary.LastVerifiedAt = &value
				}
			}
		case KindSnapshot:
			agg.summary.SnapshotPointCount++
		}
		if value := strings.TrimSpace(point.RepositoryResourceID); value != "" {
			agg.summary.RepositoryResourceIDs = append(
				agg.summary.RepositoryResourceIDs,
				value,
			)
		}
		if point.Evidence != nil {
			agg.summary.EvidenceIDs = append(agg.summary.EvidenceIDs, point.Evidence.ID)
			if source := strings.TrimSpace(point.Evidence.Source.Collector); source != "" {
				agg.summary.Source = source
			}
		}
	}

	for key, agg := range byProviderScope {
		observation, ok := latestObservations[key]
		if !ok {
			continue
		}
		agg.summary.Source = observation.Source
		agg.summary.HistoryCompleteness = observation.HistoryCompleteness
		agg.summary.Permissions = observation.Permissions
		agg.summary.VerificationExpected = observation.VerificationExpected
		agg.summary.EvidenceIDs = append(agg.summary.EvidenceIDs, observation.Evidence.ID)
		if !observation.ObservedAt.Before(agg.lastEventAt) &&
			observation.JobState != OutcomeUnknown {
			agg.summary.JobState = observation.JobState
		}
	}

	summaries := make([]ProtectionProviderSummary, 0, len(byProviderScope))
	for _, agg := range byProviderScope {
		agg.summary.RepositoryResourceIDs = sortedUnique(agg.summary.RepositoryResourceIDs)
		agg.summary.EvidenceIDs = sortedUnique(agg.summary.EvidenceIDs)
		summaries = append(summaries, agg.summary)
	}
	return DeriveProtectionPostureAt(subjectResourceID, summaries, policy, now)
}

func DeriveProtectionPostureAt(
	subjectResourceID string,
	summaries []ProtectionProviderSummary,
	policy ProtectionPosturePolicy,
	now time.Time,
) ProtectionPosture {
	subjectResourceID = strings.TrimSpace(subjectResourceID)
	if err := policy.Validate(); err != nil {
		operationaltrust.GetMetrics().ObserveProtectionEvaluationFailure("invalid_policy")
		policy = DefaultProtectionPosturePolicy
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	posture := ProtectionPosture{
		SubjectResourceID:     subjectResourceID,
		State:                 ProtectionStateUnknown,
		Freshness:             ProtectionFreshnessUnknown,
		Verification:          ProtectionVerificationUnknown,
		Coverage:              ProtectionCoverageUnknown,
		ProviderStates:        []ProtectionProviderState{},
		RepositoryResourceIDs: []string{},
		EvidenceIDs:           []string{},
		Explanation:           "Pulse has no complete provider history linked to this resource, so protection is unknown.",
		EvaluatedAt:           now,
	}

	hasUnknownBlock := false
	hasPartial := false
	hasCompleteHistory := false
	hasQualifyingSuccess := false
	hasSupportedQualifyingSuccess := false
	hasInvalidatingFailure := false
	hasSnapshotsOnly := false
	verificationExpected := policy.RequireVerification
	supportedVerificationExpected := policy.RequireVerification
	var supportedLastSuccessAt *time.Time
	var supportedLastVerifiedAt *time.Time

	for _, summary := range summaries {
		if strings.TrimSpace(string(summary.Provider)) == "" ||
			strings.TrimSpace(summary.Scope) == "" {
			continue
		}
		if !validRecoveryOutcome(summary.JobState) {
			summary.JobState = OutcomeUnknown
		}
		if !summary.HistoryCompleteness.Valid() {
			summary.HistoryCompleteness = ProtectionHistoryUnknown
		}
		if !validProtectionPermissions(summary.Permissions) {
			summary.Permissions = operationaltrust.EvidencePermissionsUnknown
		}
		summary.Source = strings.TrimSpace(summary.Source)
		if summary.Source == "" {
			summary.Source = "legacy-recovery-point"
		}
		summary.RepositoryResourceIDs = sortedUnique(summary.RepositoryResourceIDs)
		summary.EvidenceIDs = sortedUnique(summary.EvidenceIDs)

		state := ProtectionProviderState{
			Provider:             summary.Provider,
			Source:               summary.Source,
			Scope:                strings.TrimSpace(summary.Scope),
			JobState:             summary.JobState,
			HistoryCompleteness:  summary.HistoryCompleteness,
			Permissions:          summary.Permissions,
			LastAttemptAt:        cloneProtectionTime(summary.LastAttemptAt),
			LastSuccessAt:        cloneProtectionTime(summary.LastSuccessAt),
			LastVerifiedAt:       cloneProtectionTime(summary.LastVerifiedAt),
			EvidenceIDs:          append([]string(nil), summary.EvidenceIDs...),
			VerificationExpected: summary.VerificationExpected,
		}
		posture.ProviderStates = append(posture.ProviderStates, state)
		posture.RepositoryResourceIDs = append(
			posture.RepositoryResourceIDs,
			summary.RepositoryResourceIDs...,
		)
		posture.EvidenceIDs = append(posture.EvidenceIDs, summary.EvidenceIDs...)

		posture.LastAttemptAt = latestProtectionTime(posture.LastAttemptAt, summary.LastAttemptAt)
		posture.LastSuccessfulPointAt = latestProtectionTime(
			posture.LastSuccessfulPointAt,
			summary.LastSuccessAt,
		)
		posture.LastVerifiedAt = latestProtectionTime(posture.LastVerifiedAt, summary.LastVerifiedAt)

		switch summary.HistoryCompleteness {
		case ProtectionHistoryComplete:
			hasCompleteHistory = true
		case ProtectionHistoryPartial:
			hasPartial = true
		case ProtectionHistoryUnavailable, ProtectionHistoryUnknown:
			hasUnknownBlock = true
		}
		switch summary.Permissions {
		case operationaltrust.EvidencePermissionsSufficient:
		case operationaltrust.EvidencePermissionsPartial:
			hasPartial = true
		case operationaltrust.EvidencePermissionsDenied,
			operationaltrust.EvidencePermissionsUnknown:
			hasUnknownBlock = true
		}

		if summary.LastSuccessAt != nil {
			hasQualifyingSuccess = true
		}
		if summary.LastSuccessAt != nil &&
			summary.HistoryCompleteness == ProtectionHistoryComplete &&
			summary.Permissions == operationaltrust.EvidencePermissionsSufficient {
			hasSupportedQualifyingSuccess = true
			supportedLastSuccessAt = latestProtectionTime(
				supportedLastSuccessAt,
				summary.LastSuccessAt,
			)
			supportedLastVerifiedAt = latestProtectionTime(
				supportedLastVerifiedAt,
				summary.LastVerifiedAt,
			)
			if summary.VerificationExpected {
				supportedVerificationExpected = true
			}
		}
		if summary.SnapshotPointCount > 0 && summary.BackupPointCount == 0 {
			hasSnapshotsOnly = true
		}
		if summary.VerificationExpected {
			verificationExpected = true
		}
		if summary.JobState == OutcomeFailed &&
			summary.LastAttemptAt != nil &&
			(summary.LastSuccessAt == nil ||
				!summary.LastAttemptAt.Before(*summary.LastSuccessAt)) {
			hasInvalidatingFailure = true
		}
	}

	sort.Slice(posture.ProviderStates, func(i, j int) bool {
		return providerStateSortKey(posture.ProviderStates[i]) <
			providerStateSortKey(posture.ProviderStates[j])
	})
	posture.RepositoryResourceIDs = sortedUnique(posture.RepositoryResourceIDs)
	posture.EvidenceIDs = sortedUnique(posture.EvidenceIDs)

	stateLastSuccessAt := posture.LastSuccessfulPointAt
	stateLastVerifiedAt := posture.LastVerifiedAt
	stateVerificationExpected := verificationExpected
	if hasSupportedQualifyingSuccess {
		stateLastSuccessAt = supportedLastSuccessAt
		stateLastVerifiedAt = supportedLastVerifiedAt
		stateVerificationExpected = supportedVerificationExpected
	}
	if stateLastSuccessAt != nil {
		if now.Sub(*stateLastSuccessAt) <= policy.FreshnessWindow {
			posture.Freshness = ProtectionFreshnessCurrent
		} else {
			posture.Freshness = ProtectionFreshnessStale
		}
	}
	if stateLastSuccessAt != nil {
		switch {
		case stateLastVerifiedAt != nil &&
			now.Sub(*stateLastVerifiedAt) <= policy.VerificationWindow:
			posture.Verification = ProtectionVerificationVerified
		case stateLastVerifiedAt != nil:
			posture.Verification = ProtectionVerificationStale
		case stateVerificationExpected:
			posture.Verification = ProtectionVerificationUnverified
		}
	}

	switch {
	case len(posture.ProviderStates) == 0:
		posture.Coverage = ProtectionCoverageUnknown
	case hasUnknownBlock:
		posture.Coverage = ProtectionCoverageUnknown
	case hasPartial:
		posture.Coverage = ProtectionCoveragePartial
	case hasCompleteHistory && !hasQualifyingSuccess:
		posture.Coverage = ProtectionCoverageNone
	default:
		posture.Coverage = ProtectionCoverageComplete
	}

	switch {
	case len(posture.ProviderStates) == 0:
		// Keep the initialized unknown explanation.
	case hasSupportedQualifyingSuccess &&
		posture.Freshness == ProtectionFreshnessCurrent &&
		!hasInvalidatingFailure &&
		!hasPartial &&
		(!stateVerificationExpected || posture.Verification == ProtectionVerificationVerified):
		posture.State = ProtectionStateProtected
		if posture.Verification == ProtectionVerificationVerified {
			posture.Explanation = "A current subject-linked backup is available and has recent verification evidence."
		} else {
			posture.Explanation = "A current subject-linked backup is available from complete provider history."
		}
		if hasUnknownBlock {
			posture.Explanation += " Another linked provider has unavailable history, but it does not invalidate the confirmed recovery point."
		}
	case hasSupportedQualifyingSuccess:
		posture.State = ProtectionStateAttention
		switch {
		case hasInvalidatingFailure:
			posture.Explanation = "A backup exists, but a newer provider failure needs attention before Pulse can call this resource protected."
		case posture.Freshness == ProtectionFreshnessStale:
			posture.Explanation = "The strongest subject-linked backup is older than the configured freshness window."
		case stateVerificationExpected && posture.Verification != ProtectionVerificationVerified:
			posture.Explanation = "A current backup exists, but its verification evidence is missing or stale."
		default:
			posture.Explanation = "Recovery evidence exists, but provider history or permissions are incomplete."
		}
	case hasUnknownBlock:
		posture.State = ProtectionStateUnknown
		posture.Explanation = "Provider history or permissions are unavailable, so Pulse cannot make a stronger protection claim."
	case hasQualifyingSuccess || hasPartial:
		posture.State = ProtectionStateAttention
		posture.Explanation = "Recovery evidence exists, but provider history or permissions are incomplete."
	case hasCompleteHistory:
		posture.State = ProtectionStateUnprotected
		if hasSnapshotsOnly {
			posture.Explanation = "Provider history is complete, but only snapshots are present; snapshots alone do not prove independent recovery."
		} else {
			posture.Explanation = "Provider history is complete, but no qualifying subject-linked backup exists."
		}
	default:
		posture.State = ProtectionStateUnknown
	}

	operationaltrust.GetMetrics().ObserveProtectionEvaluation(string(posture.State))
	return posture
}

func recoveryPointObservedAt(point RecoveryPoint) time.Time {
	if point.CompletedAt != nil && !point.CompletedAt.IsZero() {
		return point.CompletedAt.UTC()
	}
	if point.StartedAt != nil && !point.StartedAt.IsZero() {
		return point.StartedAt.UTC()
	}
	return time.Time{}
}

func recoveryDetailString(point RecoveryPoint, key string) string {
	if point.Details == nil {
		return ""
	}
	value, _ := point.Details[key].(string)
	return strings.TrimSpace(value)
}

func providerScopeKey(provider Provider, scope string) string {
	return strings.TrimSpace(string(provider)) + "\x00" + strings.TrimSpace(scope)
}

func providerStateSortKey(state ProtectionProviderState) string {
	return strings.Join([]string{
		strings.TrimSpace(string(state.Provider)),
		strings.TrimSpace(state.Scope),
		strings.TrimSpace(state.Source),
	}, "\x00")
}

func sortedUnique(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func latestProtectionTime(current, candidate *time.Time) *time.Time {
	if candidate == nil {
		return cloneProtectionTime(current)
	}
	if current == nil || candidate.After(*current) {
		return cloneProtectionTime(candidate)
	}
	return cloneProtectionTime(current)
}

func cloneProtectionTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	clone := value.UTC()
	return &clone
}

func validRecoveryOutcome(value Outcome) bool {
	switch value {
	case OutcomeSuccess, OutcomeWarning, OutcomeFailed, OutcomeRunning, OutcomeUnknown:
		return true
	default:
		return false
	}
}

func validProtectionPermissions(value operationaltrust.EvidencePermissions) bool {
	switch value {
	case operationaltrust.EvidencePermissionsSufficient,
		operationaltrust.EvidencePermissionsPartial,
		operationaltrust.EvidencePermissionsDenied,
		operationaltrust.EvidencePermissionsUnknown:
		return true
	default:
		return false
	}
}
