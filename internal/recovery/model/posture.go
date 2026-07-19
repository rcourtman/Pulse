package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

type ProtectionState string

const (
	ProtectionStateProtected   ProtectionState = "protected"
	ProtectionStateAttention   ProtectionState = "attention"
	ProtectionStateUnprotected ProtectionState = "unprotected"
	ProtectionStateUnknown     ProtectionState = "unknown"
)

func (state ProtectionState) Valid() bool {
	switch state {
	case ProtectionStateProtected,
		ProtectionStateAttention,
		ProtectionStateUnprotected,
		ProtectionStateUnknown:
		return true
	default:
		return false
	}
}

type ProtectionFreshness string

const (
	ProtectionFreshnessCurrent ProtectionFreshness = "current"
	ProtectionFreshnessStale   ProtectionFreshness = "stale"
	ProtectionFreshnessUnknown ProtectionFreshness = "unknown"
)

func (freshness ProtectionFreshness) Valid() bool {
	switch freshness {
	case ProtectionFreshnessCurrent,
		ProtectionFreshnessStale,
		ProtectionFreshnessUnknown:
		return true
	default:
		return false
	}
}

type ProtectionVerification string

const (
	ProtectionVerificationVerified   ProtectionVerification = "verified"
	ProtectionVerificationUnverified ProtectionVerification = "unverified"
	ProtectionVerificationStale      ProtectionVerification = "stale"
	ProtectionVerificationUnknown    ProtectionVerification = "unknown"
)

func (verification ProtectionVerification) Valid() bool {
	switch verification {
	case ProtectionVerificationVerified,
		ProtectionVerificationUnverified,
		ProtectionVerificationStale,
		ProtectionVerificationUnknown:
		return true
	default:
		return false
	}
}

type ProtectionCoverage string

const (
	ProtectionCoverageComplete ProtectionCoverage = "complete"
	ProtectionCoveragePartial  ProtectionCoverage = "partial"
	ProtectionCoverageNone     ProtectionCoverage = "none"
	ProtectionCoverageUnknown  ProtectionCoverage = "unknown"
)

func (coverage ProtectionCoverage) Valid() bool {
	switch coverage {
	case ProtectionCoverageComplete,
		ProtectionCoveragePartial,
		ProtectionCoverageNone,
		ProtectionCoverageUnknown:
		return true
	default:
		return false
	}
}

type ProtectionHistoryCompleteness string

const (
	ProtectionHistoryComplete    ProtectionHistoryCompleteness = "complete"
	ProtectionHistoryPartial     ProtectionHistoryCompleteness = "partial"
	ProtectionHistoryUnavailable ProtectionHistoryCompleteness = "unavailable"
	ProtectionHistoryUnknown     ProtectionHistoryCompleteness = "unknown"
)

func (completeness ProtectionHistoryCompleteness) Valid() bool {
	switch completeness {
	case ProtectionHistoryComplete,
		ProtectionHistoryPartial,
		ProtectionHistoryUnavailable,
		ProtectionHistoryUnknown:
		return true
	default:
		return false
	}
}

type ProtectionProviderState struct {
	Provider             Provider                             `json:"provider"`
	Source               string                               `json:"source"`
	Scope                string                               `json:"scope"`
	JobState             Outcome                              `json:"jobState"`
	HistoryCompleteness  ProtectionHistoryCompleteness        `json:"historyCompleteness"`
	Permissions          operationaltrust.EvidencePermissions `json:"permissions"`
	LastAttemptAt        *time.Time                           `json:"lastAttemptAt,omitempty"`
	LastSuccessAt        *time.Time                           `json:"lastSuccessAt,omitempty"`
	LastVerifiedAt       *time.Time                           `json:"lastVerifiedAt,omitempty"`
	EvidenceIDs          []string                             `json:"evidenceIds"`
	VerificationExpected bool                                 `json:"verificationExpected,omitempty"`
}

func (state ProtectionProviderState) Clone() ProtectionProviderState {
	clone := state
	clone.LastAttemptAt = cloneTime(state.LastAttemptAt)
	clone.LastSuccessAt = cloneTime(state.LastSuccessAt)
	clone.LastVerifiedAt = cloneTime(state.LastVerifiedAt)
	clone.EvidenceIDs = append([]string(nil), state.EvidenceIDs...)
	return clone
}

func (state ProtectionProviderState) Validate() error {
	if strings.TrimSpace(string(state.Provider)) == "" {
		return errors.New("protection provider is required")
	}
	if strings.TrimSpace(state.Source) == "" {
		return errors.New("protection provider source is required")
	}
	if strings.TrimSpace(state.Scope) == "" {
		return errors.New("protection provider scope is required")
	}
	if !validOutcome(state.JobState) {
		return fmt.Errorf("protection provider job state %q is invalid", state.JobState)
	}
	if !state.HistoryCompleteness.Valid() {
		return fmt.Errorf(
			"protection provider history completeness %q is invalid",
			state.HistoryCompleteness,
		)
	}
	if !validEvidencePermissions(state.Permissions) {
		return fmt.Errorf("protection provider permissions %q are invalid", state.Permissions)
	}
	if !sortedUniqueStrings(state.EvidenceIDs) {
		return errors.New("protection provider evidence ids must be sorted and unique")
	}
	return nil
}

type ProtectionPosture struct {
	SubjectResourceID     string                    `json:"subjectResourceId"`
	State                 ProtectionState           `json:"state"`
	LastAttemptAt         *time.Time                `json:"lastAttemptAt,omitempty"`
	LastSuccessfulPointAt *time.Time                `json:"lastSuccessfulPointAt,omitempty"`
	LastVerifiedAt        *time.Time                `json:"lastVerifiedAt,omitempty"`
	Freshness             ProtectionFreshness       `json:"freshness"`
	Verification          ProtectionVerification    `json:"verification"`
	Coverage              ProtectionCoverage        `json:"coverage"`
	ProviderStates        []ProtectionProviderState `json:"providerStates"`
	RepositoryResourceIDs []string                  `json:"repositoryResourceIds"`
	EvidenceIDs           []string                  `json:"evidenceIds"`
	Explanation           string                    `json:"explanation"`
	EvaluatedAt           time.Time                 `json:"evaluatedAt"`
}

func (posture ProtectionPosture) Clone() ProtectionPosture {
	clone := posture
	clone.LastAttemptAt = cloneTime(posture.LastAttemptAt)
	clone.LastSuccessfulPointAt = cloneTime(posture.LastSuccessfulPointAt)
	clone.LastVerifiedAt = cloneTime(posture.LastVerifiedAt)
	clone.ProviderStates = make([]ProtectionProviderState, len(posture.ProviderStates))
	for i := range posture.ProviderStates {
		clone.ProviderStates[i] = posture.ProviderStates[i].Clone()
	}
	clone.RepositoryResourceIDs = append([]string(nil), posture.RepositoryResourceIDs...)
	clone.EvidenceIDs = append([]string(nil), posture.EvidenceIDs...)
	return clone
}

func (posture ProtectionPosture) Validate() error {
	if strings.TrimSpace(posture.SubjectResourceID) == "" {
		return errors.New("protection posture subject resource id is required")
	}
	if !posture.State.Valid() {
		return fmt.Errorf("protection posture state %q is invalid", posture.State)
	}
	if !posture.Freshness.Valid() {
		return fmt.Errorf("protection posture freshness %q is invalid", posture.Freshness)
	}
	if !posture.Verification.Valid() {
		return fmt.Errorf("protection posture verification %q is invalid", posture.Verification)
	}
	if !posture.Coverage.Valid() {
		return fmt.Errorf("protection posture coverage %q is invalid", posture.Coverage)
	}
	if posture.EvaluatedAt.IsZero() {
		return errors.New("protection posture evaluation time is required")
	}
	if strings.TrimSpace(posture.Explanation) == "" {
		return errors.New("protection posture explanation is required")
	}
	if !sortedUniqueStrings(posture.RepositoryResourceIDs) {
		return errors.New("protection repository resource ids must be sorted and unique")
	}
	if !sortedUniqueStrings(posture.EvidenceIDs) {
		return errors.New("protection evidence ids must be sorted and unique")
	}
	for i := range posture.ProviderStates {
		if err := posture.ProviderStates[i].Validate(); err != nil {
			return fmt.Errorf("protection provider state %d: %w", i, err)
		}
		if i > 0 && compareProviderStates(
			posture.ProviderStates[i-1],
			posture.ProviderStates[i],
		) >= 0 {
			return errors.New("protection provider states must be sorted and unique")
		}
	}
	return nil
}

type ProtectionProviderObservation struct {
	ID                   string                               `json:"id"`
	Provider             Provider                             `json:"provider"`
	Source               string                               `json:"source"`
	Scope                string                               `json:"scope"`
	JobState             Outcome                              `json:"jobState"`
	HistoryCompleteness  ProtectionHistoryCompleteness        `json:"historyCompleteness"`
	Permissions          operationaltrust.EvidencePermissions `json:"permissions"`
	VerificationExpected bool                                 `json:"verificationExpected,omitempty"`
	ObservedAt           time.Time                            `json:"observedAt"`
	IngestedAt           time.Time                            `json:"ingestedAt"`
	Evidence             operationaltrust.EvidenceEnvelope    `json:"evidence"`
}

func (observation ProtectionProviderObservation) Clone() ProtectionProviderObservation {
	clone := observation
	clone.Evidence = observation.Evidence.Clone()
	return clone
}

func (observation ProtectionProviderObservation) Validate() error {
	if strings.TrimSpace(observation.ID) == "" {
		return errors.New("protection provider observation id is required")
	}
	if strings.TrimSpace(string(observation.Provider)) == "" {
		return errors.New("protection provider observation provider is required")
	}
	if strings.TrimSpace(observation.Source) == "" {
		return errors.New("protection provider observation source is required")
	}
	if strings.TrimSpace(observation.Scope) == "" {
		return errors.New("protection provider observation scope is required")
	}
	if !validOutcome(observation.JobState) {
		return fmt.Errorf(
			"protection provider observation job state %q is invalid",
			observation.JobState,
		)
	}
	if !observation.HistoryCompleteness.Valid() {
		return fmt.Errorf(
			"protection provider observation history completeness %q is invalid",
			observation.HistoryCompleteness,
		)
	}
	if !validEvidencePermissions(observation.Permissions) {
		return fmt.Errorf(
			"protection provider observation permissions %q are invalid",
			observation.Permissions,
		)
	}
	if observation.ObservedAt.IsZero() || observation.IngestedAt.IsZero() {
		return errors.New("protection provider observation times are required")
	}
	if !observation.Evidence.ObservedAt.Equal(observation.ObservedAt) {
		return errors.New("provider observation and evidence observation times must match")
	}
	if !observation.Evidence.IngestedAt.Equal(observation.IngestedAt) {
		return errors.New("provider observation and evidence ingestion times must match")
	}
	if observation.Evidence.ID != observation.ID {
		return errors.New("provider observation id must match its evidence id")
	}
	if err := observation.Evidence.Validate(); err != nil {
		return fmt.Errorf("provider observation evidence: %w", err)
	}
	return nil
}

type ProtectionPosturePolicy struct {
	FreshnessWindow     time.Duration `json:"-"`
	VerificationWindow  time.Duration `json:"-"`
	RequireVerification bool          `json:"requireVerification"`
}

func (policy ProtectionPosturePolicy) Validate() error {
	if policy.FreshnessWindow <= 0 {
		return errors.New("protection freshness window must be positive")
	}
	if policy.VerificationWindow <= 0 {
		return errors.New("protection verification window must be positive")
	}
	return nil
}

type ProtectionPosturePolicyPayload struct {
	FreshnessWindowSeconds    int64 `json:"freshnessWindowSeconds"`
	VerificationWindowSeconds int64 `json:"verificationWindowSeconds"`
	RequireVerification       bool  `json:"requireVerification"`
}

func (policy ProtectionPosturePolicy) Payload() ProtectionPosturePolicyPayload {
	return ProtectionPosturePolicyPayload{
		FreshnessWindowSeconds:    int64(policy.FreshnessWindow / time.Second),
		VerificationWindowSeconds: int64(policy.VerificationWindow / time.Second),
		RequireVerification:       policy.RequireVerification,
	}
}

type ProtectionPostureQuery struct {
	SubjectResourceIDs []string
	State              ProtectionState
	Page               int
	Limit              int
}

type ProtectionProviderSummary struct {
	Provider              Provider
	Source                string
	Scope                 string
	JobState              Outcome
	HistoryCompleteness   ProtectionHistoryCompleteness
	Permissions           operationaltrust.EvidencePermissions
	VerificationExpected  bool
	LastAttemptAt         *time.Time
	LastSuccessAt         *time.Time
	LastVerifiedAt        *time.Time
	BackupPointCount      int
	SnapshotPointCount    int
	RepositoryResourceIDs []string
	EvidenceIDs           []string
}

func (summary ProtectionProviderSummary) normalize() ProtectionProviderSummary {
	summary.Provider = Provider(strings.TrimSpace(string(summary.Provider)))
	summary.Source = strings.TrimSpace(summary.Source)
	summary.Scope = strings.TrimSpace(summary.Scope)
	summary.RepositoryResourceIDs = normalizeSortedStrings(summary.RepositoryResourceIDs)
	summary.EvidenceIDs = normalizeSortedStrings(summary.EvidenceIDs)
	if !validOutcome(summary.JobState) {
		summary.JobState = OutcomeUnknown
	}
	if !summary.HistoryCompleteness.Valid() {
		summary.HistoryCompleteness = ProtectionHistoryUnknown
	}
	if !validEvidencePermissions(summary.Permissions) {
		summary.Permissions = operationaltrust.EvidencePermissionsUnknown
	}
	return summary
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

func validOutcome(outcome Outcome) bool {
	switch outcome {
	case OutcomeSuccess, OutcomeWarning, OutcomeFailed, OutcomeRunning, OutcomeUnknown:
		return true
	default:
		return false
	}
}

func validEvidencePermissions(value operationaltrust.EvidencePermissions) bool {
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

func normalizeSortedStrings(values []string) []string {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			unique[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(unique))
	for value := range unique {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func sortedUniqueStrings(values []string) bool {
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
		if i > 0 && values[i-1] >= value {
			return false
		}
	}
	return true
}

func compareProviderStates(a, b ProtectionProviderState) int {
	aKey := strings.Join([]string{string(a.Provider), a.Scope, a.Source}, "\x00")
	bKey := strings.Join([]string{string(b.Provider), b.Scope, b.Source}, "\x00")
	return strings.Compare(aKey, bKey)
}
