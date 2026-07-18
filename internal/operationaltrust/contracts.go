// Package operationaltrust defines the shared evidence, lifecycle, and
// notification-link contracts used by Pulse's operational read models.
package operationaltrust

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type EvidenceCompleteness string

const (
	EvidenceComplete    EvidenceCompleteness = "complete"
	EvidencePartial     EvidenceCompleteness = "partial"
	EvidenceUnavailable EvidenceCompleteness = "unavailable"
)

func (value EvidenceCompleteness) valid() bool {
	switch value {
	case EvidenceComplete, EvidencePartial, EvidenceUnavailable:
		return true
	default:
		return false
	}
}

type EvidenceConfidence string

const (
	EvidenceConfirmed EvidenceConfidence = "confirmed"
	EvidenceInferred  EvidenceConfidence = "inferred"
	EvidenceUnknown   EvidenceConfidence = "unknown"
)

func (value EvidenceConfidence) valid() bool {
	switch value {
	case EvidenceConfirmed, EvidenceInferred, EvidenceUnknown:
		return true
	default:
		return false
	}
}

type EvidencePermissions string

const (
	EvidencePermissionsSufficient EvidencePermissions = "sufficient"
	EvidencePermissionsPartial    EvidencePermissions = "partial"
	EvidencePermissionsDenied     EvidencePermissions = "denied"
	EvidencePermissionsUnknown    EvidencePermissions = "unknown"
)

func (value EvidencePermissions) valid() bool {
	switch value {
	case EvidencePermissionsSufficient,
		EvidencePermissionsPartial,
		EvidencePermissionsDenied,
		EvidencePermissionsUnknown:
		return true
	default:
		return false
	}
}

type EvidenceFreshness string

const (
	EvidenceFresh            EvidenceFreshness = "fresh"
	EvidenceStale            EvidenceFreshness = "stale"
	EvidenceFreshnessUnknown EvidenceFreshness = "unknown"
)

type EvidenceSource struct {
	Provider  string `json:"provider"`
	Collector string `json:"collector"`
	Instance  string `json:"instance,omitempty"`
}

func (source EvidenceSource) Validate() error {
	if strings.TrimSpace(source.Provider) == "" {
		return errors.New("evidence source provider is required")
	}
	if strings.TrimSpace(source.Collector) == "" {
		return errors.New("evidence source collector is required")
	}
	return nil
}

type EvidenceSubject struct {
	ResourceID    string `json:"resourceId,omitempty"`
	ProviderRef   string `json:"providerRef,omitempty"`
	ProviderScope string `json:"providerScope,omitempty"`
}

func (subject EvidenceSubject) Validate() error {
	hasResource := strings.TrimSpace(subject.ResourceID) != ""
	hasProviderRef := strings.TrimSpace(subject.ProviderRef) != ""
	if hasResource == hasProviderRef {
		return errors.New("evidence subject requires exactly one resource id or provider reference")
	}
	if hasProviderRef && strings.TrimSpace(subject.ProviderScope) == "" {
		return errors.New("unresolved provider evidence requires provider scope")
	}
	return nil
}

type EvidenceReason struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

func (reason EvidenceReason) Validate() error {
	if strings.TrimSpace(reason.Code) == "" {
		return errors.New("evidence reason code is required")
	}
	return nil
}

type EvidencePayloadRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

func (ref EvidencePayloadRef) Validate() error {
	if strings.TrimSpace(ref.Kind) == "" {
		return errors.New("evidence payload reference kind is required")
	}
	if strings.TrimSpace(ref.ID) == "" {
		return errors.New("evidence payload reference id is required")
	}
	return nil
}

type IdentityCorrelation struct {
	Rule           string            `json:"rule"`
	MatchedFields  map[string]string `json:"matchedFields"`
	CandidateCount int               `json:"candidateCount"`
}

func (correlation IdentityCorrelation) Validate() error {
	if strings.TrimSpace(correlation.Rule) == "" {
		return errors.New("identity correlation rule is required")
	}
	if len(correlation.MatchedFields) == 0 {
		return errors.New("identity correlation matched fields are required")
	}
	if correlation.CandidateCount != 1 {
		return fmt.Errorf(
			"identity correlation requires exactly one candidate, got %d",
			correlation.CandidateCount,
		)
	}
	for key, value := range correlation.MatchedFields {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			return errors.New("identity correlation matched fields must be non-empty")
		}
	}
	return nil
}

type EvidenceEnvelope struct {
	ID           string               `json:"id"`
	Source       EvidenceSource       `json:"source"`
	Subject      EvidenceSubject      `json:"subject"`
	ObservedAt   time.Time            `json:"observedAt"`
	IngestedAt   time.Time            `json:"ingestedAt"`
	ValidUntil   *time.Time           `json:"validUntil,omitempty"`
	Completeness EvidenceCompleteness `json:"completeness"`
	Confidence   EvidenceConfidence   `json:"confidence"`
	Reason       *EvidenceReason      `json:"reason,omitempty"`
	Permissions  EvidencePermissions  `json:"permissions"`
	PayloadRef   *EvidencePayloadRef  `json:"payloadRef,omitempty"`
	Correlation  *IdentityCorrelation `json:"correlation,omitempty"`
}

func (envelope EvidenceEnvelope) Clone() EvidenceEnvelope {
	clone := envelope
	if envelope.ValidUntil != nil {
		value := *envelope.ValidUntil
		clone.ValidUntil = &value
	}
	if envelope.Reason != nil {
		value := *envelope.Reason
		clone.Reason = &value
	}
	if envelope.PayloadRef != nil {
		value := *envelope.PayloadRef
		clone.PayloadRef = &value
	}
	if envelope.Correlation != nil {
		value := *envelope.Correlation
		value.MatchedFields = cloneStringMap(envelope.Correlation.MatchedFields)
		clone.Correlation = &value
	}
	return clone
}

func NewEvidenceID(
	source EvidenceSource,
	subject EvidenceSubject,
	observedAt time.Time,
	sourceObservationID string,
) (string, error) {
	if err := source.Validate(); err != nil {
		return "", err
	}
	if err := subject.Validate(); err != nil {
		return "", err
	}
	if observedAt.IsZero() {
		return "", errors.New("evidence observed time is required")
	}
	if strings.TrimSpace(sourceObservationID) == "" {
		return "", errors.New("source observation id is required")
	}
	return stableID(
		"evidence",
		source.Provider,
		source.Collector,
		source.Instance,
		subject.ResourceID,
		subject.ProviderScope,
		subject.ProviderRef,
		observedAt.UTC().Format(time.RFC3339Nano),
		sourceObservationID,
	), nil
}

func (envelope EvidenceEnvelope) Validate() error {
	if strings.TrimSpace(envelope.ID) == "" {
		return errors.New("evidence id is required")
	}
	if err := envelope.Source.Validate(); err != nil {
		return err
	}
	if err := envelope.Subject.Validate(); err != nil {
		return err
	}
	if envelope.ObservedAt.IsZero() {
		return errors.New("evidence observed time is required")
	}
	if envelope.IngestedAt.IsZero() {
		return errors.New("evidence ingested time is required")
	}
	if envelope.ValidUntil != nil && envelope.ValidUntil.Before(envelope.ObservedAt) {
		return errors.New("evidence validity cannot end before observation")
	}
	if !envelope.Completeness.valid() {
		return fmt.Errorf("evidence completeness %q is invalid", envelope.Completeness)
	}
	if !envelope.Confidence.valid() {
		return fmt.Errorf("evidence confidence %q is invalid", envelope.Confidence)
	}
	if !envelope.Permissions.valid() {
		return fmt.Errorf("evidence permissions %q are invalid", envelope.Permissions)
	}
	needsReason := envelope.Completeness != EvidenceComplete ||
		envelope.Confidence != EvidenceConfirmed ||
		envelope.Permissions != EvidencePermissionsSufficient
	if needsReason && envelope.Reason == nil {
		return errors.New("limited evidence requires a typed reason")
	}
	if envelope.Reason != nil {
		if err := envelope.Reason.Validate(); err != nil {
			return err
		}
	}
	if envelope.PayloadRef != nil {
		if err := envelope.PayloadRef.Validate(); err != nil {
			return err
		}
	}
	if envelope.Correlation != nil {
		if err := envelope.Correlation.Validate(); err != nil {
			return err
		}
	}
	if envelope.Confidence == EvidenceInferred && envelope.Correlation == nil {
		return errors.New("inferred evidence requires identity correlation")
	}
	if envelope.Subject.ResourceID == "" && envelope.Correlation != nil {
		return errors.New("unresolved evidence cannot claim a resolved identity correlation")
	}
	return nil
}

func (envelope EvidenceEnvelope) FreshnessAt(at time.Time) EvidenceFreshness {
	if at.IsZero() || envelope.ObservedAt.IsZero() || envelope.ValidUntil == nil {
		return EvidenceFreshnessUnknown
	}
	if at.After(*envelope.ValidUntil) {
		return EvidenceStale
	}
	return EvidenceFresh
}

type OperationalState string

const (
	OperationalObserving    OperationalState = "observing"
	OperationalOpen         OperationalState = "open"
	OperationalAcknowledged OperationalState = "acknowledged"
	OperationalSuppressed   OperationalState = "suppressed"
	OperationalResolving    OperationalState = "resolving"
	OperationalResolved     OperationalState = "resolved"
	OperationalStale        OperationalState = "stale"
	OperationalUnknown      OperationalState = "unknown"
)

func (state OperationalState) valid() bool {
	switch state {
	case OperationalObserving,
		OperationalOpen,
		OperationalAcknowledged,
		OperationalSuppressed,
		OperationalResolving,
		OperationalResolved,
		OperationalStale,
		OperationalUnknown:
		return true
	default:
		return false
	}
}

type OperationalSeverity string

const (
	SeverityInfo     OperationalSeverity = "info"
	SeverityWarning  OperationalSeverity = "warning"
	SeverityCritical OperationalSeverity = "critical"
	SeverityUnknown  OperationalSeverity = "unknown"
)

func (severity OperationalSeverity) valid() bool {
	switch severity {
	case SeverityInfo, SeverityWarning, SeverityCritical, SeverityUnknown:
		return true
	default:
		return false
	}
}

type Acknowledgement struct {
	At   time.Time `json:"at"`
	By   string    `json:"by"`
	Note string    `json:"note,omitempty"`
}

func (acknowledgement Acknowledgement) Validate() error {
	if acknowledgement.At.IsZero() {
		return errors.New("acknowledgement time is required")
	}
	if strings.TrimSpace(acknowledgement.By) == "" {
		return errors.New("acknowledgement actor is required")
	}
	return nil
}

type Suppression struct {
	At        time.Time  `json:"at"`
	By        string     `json:"by"`
	Reason    string     `json:"reason"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

func (suppression Suppression) Validate() error {
	if suppression.At.IsZero() {
		return errors.New("suppression time is required")
	}
	if strings.TrimSpace(suppression.By) == "" {
		return errors.New("suppression actor is required")
	}
	if strings.TrimSpace(suppression.Reason) == "" {
		return errors.New("suppression reason is required")
	}
	if suppression.ExpiresAt != nil && !suppression.ExpiresAt.After(suppression.At) {
		return errors.New("suppression expiry must follow suppression time")
	}
	return nil
}

type OperationalRecord struct {
	ID                  string              `json:"id"`
	CanonicalSpecID     string              `json:"canonicalSpecId"`
	SubjectResourceID   string              `json:"subjectResourceId"`
	State               OperationalState    `json:"state"`
	Severity            OperationalSeverity `json:"severity"`
	FirstObservedAt     time.Time           `json:"firstObservedAt"`
	LastObservedAt      time.Time           `json:"lastObservedAt"`
	StateChangedAt      time.Time           `json:"stateChangedAt"`
	ResolvedAt          *time.Time          `json:"resolvedAt,omitempty"`
	Acknowledgement     *Acknowledgement    `json:"acknowledgement,omitempty"`
	Suppression         *Suppression        `json:"suppression,omitempty"`
	EvidenceIDs         []string            `json:"evidenceIds"`
	CauseKey            string              `json:"causeKey"`
	RelatedResourceIDs  []string            `json:"relatedResourceIds"`
	ImpactSummary       string              `json:"impactSummary,omitempty"`
	RecommendedNextStep string              `json:"recommendedNextStep,omitempty"`
}

func (record OperationalRecord) Clone() OperationalRecord {
	clone := record
	if record.ResolvedAt != nil {
		value := *record.ResolvedAt
		clone.ResolvedAt = &value
	}
	if record.Acknowledgement != nil {
		value := *record.Acknowledgement
		clone.Acknowledgement = &value
	}
	if record.Suppression != nil {
		value := *record.Suppression
		if record.Suppression.ExpiresAt != nil {
			expiresAt := *record.Suppression.ExpiresAt
			value.ExpiresAt = &expiresAt
		}
		clone.Suppression = &value
	}
	clone.EvidenceIDs = append([]string(nil), record.EvidenceIDs...)
	clone.RelatedResourceIDs = append([]string(nil), record.RelatedResourceIDs...)
	return clone
}

func (record OperationalRecord) Validate() error {
	if strings.TrimSpace(record.ID) == "" {
		return errors.New("operational record id is required")
	}
	if strings.TrimSpace(record.CanonicalSpecID) == "" {
		return errors.New("operational canonical spec id is required")
	}
	if strings.TrimSpace(record.SubjectResourceID) == "" {
		return errors.New("operational subject resource id is required")
	}
	if !record.State.valid() {
		return fmt.Errorf("operational state %q is invalid", record.State)
	}
	if !record.Severity.valid() {
		return fmt.Errorf("operational severity %q is invalid", record.Severity)
	}
	if record.FirstObservedAt.IsZero() ||
		record.LastObservedAt.IsZero() ||
		record.StateChangedAt.IsZero() {
		return errors.New("operational observation and state-change times are required")
	}
	if record.LastObservedAt.Before(record.FirstObservedAt) {
		return errors.New("operational last observation precedes first observation")
	}
	if strings.TrimSpace(record.CauseKey) == "" {
		return errors.New("operational cause key is required")
	}
	if record.Acknowledgement != nil {
		if err := record.Acknowledgement.Validate(); err != nil {
			return err
		}
	}
	if record.Suppression != nil {
		if err := record.Suppression.Validate(); err != nil {
			return err
		}
	}
	if record.State == OperationalAcknowledged && record.Acknowledgement == nil {
		return errors.New("acknowledged operational state requires acknowledgement")
	}
	if record.State == OperationalSuppressed && record.Suppression == nil {
		return errors.New("suppressed operational state requires suppression")
	}
	if record.State == OperationalResolved {
		if record.ResolvedAt == nil || record.ResolvedAt.IsZero() {
			return errors.New("resolved operational state requires resolution time")
		}
		if len(canonicalIDs(record.EvidenceIDs)) == 0 {
			return errors.New("resolved operational state requires decisive evidence")
		}
	} else if record.ResolvedAt != nil {
		return errors.New("non-resolved operational state cannot carry resolution time")
	}
	return nil
}

type TransitionCause string

const (
	TransitionDetectorDecision   TransitionCause = "detector_decision"
	TransitionAcknowledgement    TransitionCause = "acknowledgement"
	TransitionUnacknowledgement  TransitionCause = "unacknowledgement"
	TransitionSuppression        TransitionCause = "suppression"
	TransitionSuppressionExpired TransitionCause = "suppression_expired"
	TransitionRecoveryEvidence   TransitionCause = "recovery_evidence"
	TransitionCollectionStale    TransitionCause = "collection_stale"
	TransitionCollectionUnknown  TransitionCause = "collection_unknown"
)

func (cause TransitionCause) valid() bool {
	switch cause {
	case TransitionDetectorDecision,
		TransitionAcknowledgement,
		TransitionUnacknowledgement,
		TransitionSuppression,
		TransitionSuppressionExpired,
		TransitionRecoveryEvidence,
		TransitionCollectionStale,
		TransitionCollectionUnknown:
		return true
	default:
		return false
	}
}

type LifecycleTransition struct {
	ID                  string           `json:"id"`
	OperationalRecordID string           `json:"operationalRecordId"`
	From                OperationalState `json:"from"`
	To                  OperationalState `json:"to"`
	At                  time.Time        `json:"at"`
	Cause               TransitionCause  `json:"cause"`
	CauseKey            string           `json:"causeKey"`
	EvidenceIDs         []string         `json:"evidenceIds"`
	Reason              string           `json:"reason,omitempty"`
}

func (transition LifecycleTransition) Clone() LifecycleTransition {
	clone := transition
	clone.EvidenceIDs = append([]string(nil), transition.EvidenceIDs...)
	return clone
}

func NewTransitionID(
	recordID string,
	from OperationalState,
	to OperationalState,
	at time.Time,
	cause TransitionCause,
	causeKey string,
	evidenceIDs []string,
) (string, error) {
	transition := LifecycleTransition{
		OperationalRecordID: recordID,
		From:                from,
		To:                  to,
		At:                  at,
		Cause:               cause,
		CauseKey:            causeKey,
		EvidenceIDs:         canonicalIDs(evidenceIDs),
	}
	if err := transition.validateWithoutID(); err != nil {
		return "", err
	}
	parts := []string{
		recordID,
		string(from),
		string(to),
		at.UTC().Format(time.RFC3339Nano),
		string(cause),
		causeKey,
	}
	parts = append(parts, transition.EvidenceIDs...)
	return stableID("transition", parts...), nil
}

func (transition LifecycleTransition) Validate() error {
	if strings.TrimSpace(transition.ID) == "" {
		return errors.New("lifecycle transition id is required")
	}
	return transition.validateWithoutID()
}

func (transition LifecycleTransition) validateWithoutID() error {
	if strings.TrimSpace(transition.OperationalRecordID) == "" {
		return errors.New("lifecycle operational record id is required")
	}
	if !transition.From.valid() {
		return fmt.Errorf("lifecycle from state %q is invalid", transition.From)
	}
	if !transition.To.valid() {
		return fmt.Errorf("lifecycle to state %q is invalid", transition.To)
	}
	if transition.From == transition.To {
		return errors.New("lifecycle transition must change state")
	}
	if transition.At.IsZero() {
		return errors.New("lifecycle transition time is required")
	}
	if !transition.Cause.valid() {
		return fmt.Errorf("lifecycle transition cause %q is invalid", transition.Cause)
	}
	if strings.TrimSpace(transition.CauseKey) == "" {
		return errors.New("lifecycle transition cause key is required")
	}
	switch transition.Cause {
	case TransitionAcknowledgement:
		if transition.To != OperationalAcknowledged {
			return errors.New("acknowledgement transition must enter acknowledged state")
		}
	case TransitionUnacknowledgement:
		if transition.From != OperationalAcknowledged || transition.To != OperationalOpen {
			return errors.New("unacknowledgement transition must return acknowledged state to open")
		}
	case TransitionSuppression:
		if transition.To != OperationalSuppressed {
			return errors.New("suppression transition must enter suppressed state")
		}
	case TransitionSuppressionExpired:
		if transition.From != OperationalSuppressed || transition.To != OperationalOpen {
			return errors.New("suppression expiry must return suppressed state to open")
		}
	case TransitionRecoveryEvidence:
		if transition.To != OperationalResolving && transition.To != OperationalResolved {
			return errors.New("recovery evidence transition must enter resolving or resolved state")
		}
		if len(canonicalIDs(transition.EvidenceIDs)) == 0 {
			return errors.New("recovery transition requires evidence")
		}
	case TransitionCollectionStale:
		if transition.To != OperationalStale {
			return errors.New("collection-stale transition must enter stale state")
		}
	case TransitionCollectionUnknown:
		if transition.To != OperationalUnknown {
			return errors.New("collection-unknown transition must enter unknown state")
		}
	case TransitionDetectorDecision:
		if transition.To != OperationalOpen {
			return errors.New("detector decision transition must enter open state")
		}
		if len(canonicalIDs(transition.EvidenceIDs)) == 0 {
			return errors.New("detector decision requires evidence")
		}
	}
	return nil
}

type NotificationDeliveryState string

const (
	NotificationQueued     NotificationDeliveryState = "queued"
	NotificationDelivering NotificationDeliveryState = "delivering"
	NotificationDelivered  NotificationDeliveryState = "delivered"
	NotificationRetrying   NotificationDeliveryState = "retrying"
	NotificationCancelled  NotificationDeliveryState = "cancelled"
	NotificationFailed     NotificationDeliveryState = "failed"
	NotificationDeadLetter NotificationDeliveryState = "dead_letter"
)

func (state NotificationDeliveryState) valid() bool {
	switch state {
	case NotificationQueued,
		NotificationDelivering,
		NotificationDelivered,
		NotificationRetrying,
		NotificationCancelled,
		NotificationFailed,
		NotificationDeadLetter:
		return true
	default:
		return false
	}
}

type NotificationLink struct {
	NotificationID      string                    `json:"notificationId"`
	OperationalRecordID string                    `json:"operationalRecordId"`
	TransitionID        string                    `json:"transitionId"`
	LifecycleState      OperationalState          `json:"lifecycleState"`
	CauseKey            string                    `json:"causeKey"`
	DestinationID       string                    `json:"destinationId"`
	DeliveryState       NotificationDeliveryState `json:"deliveryState"`
	AttemptedAt         *time.Time                `json:"attemptedAt,omitempty"`
	CompletedAt         *time.Time                `json:"completedAt,omitempty"`
}

func (link NotificationLink) Clone() NotificationLink {
	clone := link
	if link.AttemptedAt != nil {
		value := *link.AttemptedAt
		clone.AttemptedAt = &value
	}
	if link.CompletedAt != nil {
		value := *link.CompletedAt
		clone.CompletedAt = &value
	}
	return clone
}

func NewNotificationID(
	destinationID string,
	deliveryType string,
	createdAt time.Time,
	transitionIDs []string,
) (string, error) {
	if strings.TrimSpace(destinationID) == "" {
		return "", errors.New("notification destination id is required")
	}
	if strings.TrimSpace(deliveryType) == "" {
		return "", errors.New("notification delivery type is required")
	}
	if createdAt.IsZero() {
		return "", errors.New("notification creation time is required")
	}
	ids := canonicalIDs(transitionIDs)
	if len(ids) == 0 {
		return "", errors.New("notification transition ids are required")
	}
	parts := []string{
		destinationID,
		deliveryType,
		createdAt.UTC().Format(time.RFC3339Nano),
	}
	parts = append(parts, ids...)
	return stableID("notification", parts...), nil
}

func (link NotificationLink) Validate() error {
	if strings.TrimSpace(link.NotificationID) == "" {
		return errors.New("notification id is required")
	}
	if strings.TrimSpace(link.OperationalRecordID) == "" {
		return errors.New("notification operational record id is required")
	}
	if strings.TrimSpace(link.TransitionID) == "" {
		return errors.New("notification transition id is required")
	}
	if !link.LifecycleState.valid() {
		return fmt.Errorf("notification lifecycle state %q is invalid", link.LifecycleState)
	}
	if strings.TrimSpace(link.CauseKey) == "" {
		return errors.New("notification cause key is required")
	}
	if strings.TrimSpace(link.DestinationID) == "" {
		return errors.New("notification destination id is required")
	}
	if !link.DeliveryState.valid() {
		return fmt.Errorf("notification delivery state %q is invalid", link.DeliveryState)
	}
	if link.CompletedAt != nil && link.AttemptedAt == nil {
		return errors.New("completed notification requires attempted time")
	}
	if link.AttemptedAt != nil &&
		link.CompletedAt != nil &&
		link.CompletedAt.Before(*link.AttemptedAt) {
		return errors.New("notification completion precedes attempt")
	}
	return nil
}

func canonicalIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func stableID(prefix string, parts ...string) string {
	var payload strings.Builder
	for _, part := range parts {
		payload.WriteString(strconv.Itoa(len(part)))
		payload.WriteByte(':')
		payload.WriteString(part)
		payload.WriteByte('|')
	}
	sum := sha256.Sum256([]byte(payload.String()))
	return prefix + "_" + hex.EncodeToString(sum[:16])
}
