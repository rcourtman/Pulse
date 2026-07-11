package unifiedresources

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	ActionResultV2Version        = 2
	ActionEvidenceVersion        = 1
	ActionEvidenceMaxItems       = 16
	ActionEvidenceMaxRefs        = 16
	ActionEvidenceMaxTextBytes   = 2048
	ActionEvidenceMaxReasonBytes = 256
)

// ActionExecutionStatus is the executor-owned outcome of the requested
// mutation. It is deliberately independent from postcondition verification.
type ActionExecutionStatus string

const (
	ActionExecutionNotRun       ActionExecutionStatus = "not_run"
	ActionExecutionSucceeded    ActionExecutionStatus = "succeeded"
	ActionExecutionFailed       ActionExecutionStatus = "failed"
	ActionExecutionInconclusive ActionExecutionStatus = "inconclusive"
)

// ActionVerificationStatus is the evidence-owned verdict about the intended
// postcondition. A contradicted postcondition does not rewrite execution.
type ActionVerificationStatus string

const (
	ActionVerificationNotAttempted ActionVerificationStatus = "not_attempted"
	ActionVerificationConfirmed    ActionVerificationStatus = "confirmed"
	ActionVerificationContradicted ActionVerificationStatus = "contradicted"
	ActionVerificationInconclusive ActionVerificationStatus = "inconclusive"
)

// ActionEvidenceClass records the trust relationship between the executor and
// observer. A second read through the mutating agent remains agent-attested.
type ActionEvidenceClass string

const (
	ActionEvidenceNone          ActionEvidenceClass = "none"
	ActionEvidenceAgentAttested ActionEvidenceClass = "agent_attested"
	ActionEvidenceIndependent   ActionEvidenceClass = "independent"
)

// ActionEvidenceRef is a bounded durable reference to supporting evidence.
type ActionEvidenceRef struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Digest string `json:"digest"`
}

// ActionEvidence is a redacted, bounded and digest-bound observation. Digest
// is calculated over the canonical redacted envelope with Digest cleared.
type ActionEvidence struct {
	Version             int                 `json:"version"`
	ID                  string              `json:"id"`
	ObserverID          string              `json:"observerId"`
	ObserverKind        string              `json:"observerKind"`
	ObserverTrustDomain string              `json:"observerTrustDomain"`
	ExecutorTrustDomain string              `json:"executorTrustDomain"`
	Method              string              `json:"method"`
	SubjectID           string              `json:"subjectId"`
	ObservedAt          time.Time           `json:"observedAt"`
	ReceivedAt          time.Time           `json:"receivedAt"`
	ReasonCode          string              `json:"reasonCode,omitempty"`
	Summary             string              `json:"summary,omitempty"`
	Refs                []ActionEvidenceRef `json:"refs,omitempty"`
	Digest              string              `json:"digest"`
}

type ActionExecutionTruth struct {
	Status     ActionExecutionStatus `json:"status"`
	ReasonCode string                `json:"reasonCode,omitempty"`
	Summary    string                `json:"summary,omitempty"`
}

type ActionVerificationTruth struct {
	Status        ActionVerificationStatus `json:"status"`
	EvidenceClass ActionEvidenceClass      `json:"evidenceClass"`
	ReasonCode    string                   `json:"reasonCode,omitempty"`
	Summary       string                   `json:"summary,omitempty"`
	Evidence      []ActionEvidence         `json:"evidence,omitempty"`
}

type ActionCompensationSupport string

const (
	ActionCompensationUnavailable ActionCompensationSupport = "unavailable"
	ActionCompensationDeclared    ActionCompensationSupport = "declared"
)

type ActionCompensationStatus string

const (
	ActionCompensationNotAvailable ActionCompensationStatus = "not_available"
	ActionCompensationNotNeeded    ActionCompensationStatus = "not_needed"
	ActionCompensationNotAttempted ActionCompensationStatus = "not_attempted"
	ActionCompensationRunning      ActionCompensationStatus = "running"
	ActionCompensationSucceeded    ActionCompensationStatus = "succeeded"
	ActionCompensationFailed       ActionCompensationStatus = "failed"
	ActionCompensationInconclusive ActionCompensationStatus = "inconclusive"
)

// ActionCompensationTruth describes declared recovery and its outcome without
// rewriting the primary execution or verification truth.
type ActionCompensationTruth struct {
	Support       ActionCompensationSupport `json:"support"`
	Strategy      string                    `json:"strategy,omitempty"`
	Trigger       string                    `json:"trigger,omitempty"`
	Status        ActionCompensationStatus  `json:"status"`
	ReasonCode    string                    `json:"reasonCode,omitempty"`
	Summary       string                    `json:"summary,omitempty"`
	AttemptID     string                    `json:"attemptId,omitempty"`
	StepID        string                    `json:"stepId,omitempty"`
	StartedAt     *time.Time                `json:"startedAt,omitempty"`
	CompletedAt   *time.Time                `json:"completedAt,omitempty"`
	Evidence      []ActionEvidence          `json:"evidence,omitempty"`
	Execution     *ActionExecutionTruth     `json:"execution,omitempty"`
	Verification  *ActionVerificationTruth  `json:"verification,omitempty"`
	RestoredState *ActionRestoredStateTruth `json:"restoredState,omitempty"`
}

// ActionRestoredStateTruth binds a successful compensation to the exact
// durable state identity that independent verification observed.
type ActionRestoredStateTruth struct {
	SubjectID      string    `json:"subjectId"`
	ExpectedDigest string    `json:"expectedDigest"`
	ObservedDigest string    `json:"observedDigest"`
	ObservedAt     time.Time `json:"observedAt"`
}

// ActionResultV2 is the sole canonical terminal truth contract. Legacy result
// booleans and VerificationOutcome are compatibility projections derived from
// this structure during the migration window.
type ActionResultV2 struct {
	Version      int                     `json:"version"`
	Execution    ActionExecutionTruth    `json:"execution"`
	Verification ActionVerificationTruth `json:"verification"`
	Compensation ActionCompensationTruth `json:"compensation"`
}

var (
	ErrInvalidActionResultV2    = errors.New("invalid action result v2")
	ErrInvalidActionEvidence    = errors.New("invalid action evidence")
	ErrFalseIndependentEvidence = errors.New("independent evidence must use a distinct trust domain")
	ErrActionEvidenceBounds     = errors.New("action evidence exceeds bounded contract")
	ErrExecutorResultContract   = errors.New("executor result contract violation")
)

func validActionExecutionStatus(status ActionExecutionStatus) bool {
	switch status {
	case ActionExecutionNotRun, ActionExecutionSucceeded, ActionExecutionFailed, ActionExecutionInconclusive:
		return true
	default:
		return false
	}
}

func validActionVerificationStatus(status ActionVerificationStatus) bool {
	switch status {
	case ActionVerificationNotAttempted, ActionVerificationConfirmed, ActionVerificationContradicted, ActionVerificationInconclusive:
		return true
	default:
		return false
	}
}

func validActionEvidenceClass(class ActionEvidenceClass) bool {
	switch class {
	case ActionEvidenceNone, ActionEvidenceAgentAttested, ActionEvidenceIndependent:
		return true
	default:
		return false
	}
}

func boundedAuditText(value string, limit int) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) > limit {
		return "", ErrActionEvidenceBounds
	}
	value = strings.TrimSpace(RedactAuditText(value))
	if len(value) > limit {
		return "", ErrActionEvidenceBounds
	}
	return value, nil
}

// ActionEvidenceDigest returns the stable SHA-256 digest of the canonical,
// redacted evidence envelope.
func ActionEvidenceDigest(evidence ActionEvidence) (string, error) {
	evidence.Digest = ""
	normalized, err := normalizeActionEvidenceFields(evidence)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("marshal canonical action evidence: %w", err)
	}
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func normalizeActionEvidenceFields(evidence ActionEvidence) (ActionEvidence, error) {
	if evidence.Version == 0 {
		evidence.Version = ActionEvidenceVersion
	}
	if evidence.Version != ActionEvidenceVersion {
		return ActionEvidence{}, fmt.Errorf("%w: unsupported version %d", ErrInvalidActionEvidence, evidence.Version)
	}
	var err error
	for _, field := range []*string{&evidence.ID, &evidence.ObserverID, &evidence.ObserverKind, &evidence.ObserverTrustDomain, &evidence.ExecutorTrustDomain, &evidence.Method, &evidence.SubjectID} {
		*field, err = boundedAuditText(*field, ActionEvidenceMaxTextBytes)
		if err != nil {
			return ActionEvidence{}, err
		}
	}
	evidence.ReasonCode, err = boundedAuditText(evidence.ReasonCode, ActionEvidenceMaxReasonBytes)
	if err != nil {
		return ActionEvidence{}, err
	}
	evidence.Summary, err = boundedAuditText(evidence.Summary, ActionEvidenceMaxTextBytes)
	if err != nil {
		return ActionEvidence{}, err
	}
	if len(evidence.Refs) > ActionEvidenceMaxRefs {
		return ActionEvidence{}, ErrActionEvidenceBounds
	}
	for i := range evidence.Refs {
		evidence.Refs[i].ID, err = boundedAuditText(evidence.Refs[i].ID, ActionEvidenceMaxTextBytes)
		if err != nil {
			return ActionEvidence{}, err
		}
		evidence.Refs[i].Kind, err = boundedAuditText(evidence.Refs[i].Kind, ActionEvidenceMaxReasonBytes)
		if err != nil {
			return ActionEvidence{}, err
		}
		evidence.Refs[i].Digest = strings.TrimSpace(evidence.Refs[i].Digest)
		if !validSHA256Digest(evidence.Refs[i].Digest) {
			return ActionEvidence{}, fmt.Errorf("%w: invalid evidence reference digest", ErrInvalidActionEvidence)
		}
	}
	if evidence.ObservedAt.IsZero() || evidence.ReceivedAt.IsZero() {
		return ActionEvidence{}, fmt.Errorf("%w: observation timestamps required", ErrInvalidActionEvidence)
	}
	evidence.ObservedAt = evidence.ObservedAt.UTC()
	evidence.ReceivedAt = evidence.ReceivedAt.UTC()
	if evidence.ReceivedAt.Before(evidence.ObservedAt) {
		return ActionEvidence{}, fmt.Errorf("%w: receivedAt predates observedAt", ErrInvalidActionEvidence)
	}
	if evidence.ID == "" || evidence.ObserverID == "" || evidence.ObserverKind == "" || evidence.ObserverTrustDomain == "" || evidence.ExecutorTrustDomain == "" || evidence.Method == "" || evidence.SubjectID == "" {
		return ActionEvidence{}, fmt.Errorf("%w: evidence identity, observer kind, trust domains, method, and subject required", ErrInvalidActionEvidence)
	}
	return evidence, nil
}

func validSHA256Digest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil
}

func NormalizeActionEvidence(evidence ActionEvidence) (ActionEvidence, error) {
	providedDigest := strings.TrimSpace(evidence.Digest)
	evidence.Digest = ""
	normalized, err := normalizeActionEvidenceFields(evidence)
	if err != nil {
		return ActionEvidence{}, err
	}
	digest, err := ActionEvidenceDigest(normalized)
	if err != nil {
		return ActionEvidence{}, err
	}
	if providedDigest != "" && providedDigest != digest {
		return ActionEvidence{}, fmt.Errorf("%w: digest mismatch", ErrInvalidActionEvidence)
	}
	normalized.Digest = digest
	return normalized, nil
}

func normalizeExecutionTruth(truth ActionExecutionTruth) (ActionExecutionTruth, error) {
	if !validActionExecutionStatus(truth.Status) {
		return ActionExecutionTruth{}, fmt.Errorf("%w: unsupported execution status %q", ErrInvalidActionResultV2, truth.Status)
	}
	var err error
	truth.ReasonCode, err = boundedAuditText(truth.ReasonCode, ActionEvidenceMaxReasonBytes)
	if err != nil {
		return ActionExecutionTruth{}, err
	}
	truth.Summary, err = boundedAuditText(truth.Summary, ActionEvidenceMaxTextBytes)
	if err != nil {
		return ActionExecutionTruth{}, err
	}
	if (truth.Status == ActionExecutionFailed || truth.Status == ActionExecutionInconclusive || truth.Status == ActionExecutionNotRun) && truth.ReasonCode == "" {
		return ActionExecutionTruth{}, fmt.Errorf("%w: non-success execution requires reasonCode", ErrInvalidActionResultV2)
	}
	return truth, nil
}

func normalizeVerificationTruth(truth ActionVerificationTruth) (ActionVerificationTruth, error) {
	if !validActionVerificationStatus(truth.Status) || !validActionEvidenceClass(truth.EvidenceClass) {
		return ActionVerificationTruth{}, fmt.Errorf("%w: unsupported verification status or evidence class", ErrInvalidActionResultV2)
	}
	var err error
	truth.ReasonCode, err = boundedAuditText(truth.ReasonCode, ActionEvidenceMaxReasonBytes)
	if err != nil {
		return ActionVerificationTruth{}, err
	}
	truth.Summary, err = boundedAuditText(truth.Summary, ActionEvidenceMaxTextBytes)
	if err != nil {
		return ActionVerificationTruth{}, err
	}
	if len(truth.Evidence) > ActionEvidenceMaxItems {
		return ActionVerificationTruth{}, ErrActionEvidenceBounds
	}
	if truth.Status == ActionVerificationNotAttempted {
		if truth.EvidenceClass != ActionEvidenceNone || len(truth.Evidence) != 0 {
			return ActionVerificationTruth{}, fmt.Errorf("%w: not-attempted verification cannot carry evidence", ErrInvalidActionResultV2)
		}
		return truth, nil
	}
	if truth.Status == ActionVerificationInconclusive && truth.ReasonCode == "" {
		return ActionVerificationTruth{}, fmt.Errorf("%w: inconclusive verification requires reasonCode", ErrInvalidActionResultV2)
	}
	if (truth.Status == ActionVerificationConfirmed || truth.Status == ActionVerificationContradicted) && (truth.EvidenceClass == ActionEvidenceNone || len(truth.Evidence) == 0) {
		return ActionVerificationTruth{}, fmt.Errorf("%w: conclusive verification requires evidence", ErrInvalidActionResultV2)
	}
	if truth.EvidenceClass == ActionEvidenceNone && len(truth.Evidence) != 0 {
		return ActionVerificationTruth{}, fmt.Errorf("%w: evidence class none cannot carry evidence", ErrInvalidActionResultV2)
	}
	if truth.EvidenceClass != ActionEvidenceNone && len(truth.Evidence) == 0 {
		return ActionVerificationTruth{}, fmt.Errorf("%w: declared evidence class requires evidence", ErrInvalidActionResultV2)
	}
	for i := range truth.Evidence {
		truth.Evidence[i], err = NormalizeActionEvidence(truth.Evidence[i])
		if err != nil {
			return ActionVerificationTruth{}, err
		}
		if truth.EvidenceClass == ActionEvidenceIndependent && truth.Evidence[i].ObserverTrustDomain == truth.Evidence[i].ExecutorTrustDomain {
			return ActionVerificationTruth{}, ErrFalseIndependentEvidence
		}
	}
	return truth, nil
}

func normalizeCompensationTruth(truth ActionCompensationTruth) (ActionCompensationTruth, error) {
	var err error
	truth.Strategy, err = boundedAuditText(truth.Strategy, ActionEvidenceMaxReasonBytes)
	if err != nil {
		return ActionCompensationTruth{}, err
	}
	truth.Trigger, err = boundedAuditText(truth.Trigger, ActionEvidenceMaxReasonBytes)
	if err != nil {
		return ActionCompensationTruth{}, err
	}
	truth.ReasonCode, err = boundedAuditText(truth.ReasonCode, ActionEvidenceMaxReasonBytes)
	if err != nil {
		return ActionCompensationTruth{}, err
	}
	truth.Summary, err = boundedAuditText(truth.Summary, ActionEvidenceMaxTextBytes)
	if err != nil {
		return ActionCompensationTruth{}, err
	}
	truth.AttemptID, err = boundedAuditText(truth.AttemptID, ActionEvidenceMaxTextBytes)
	if err != nil {
		return ActionCompensationTruth{}, err
	}
	truth.StepID, err = boundedAuditText(truth.StepID, ActionEvidenceMaxTextBytes)
	if err != nil {
		return ActionCompensationTruth{}, err
	}
	if truth.StartedAt != nil {
		startedAt := truth.StartedAt.UTC()
		truth.StartedAt = &startedAt
	}
	if truth.CompletedAt != nil {
		completedAt := truth.CompletedAt.UTC()
		truth.CompletedAt = &completedAt
	}
	if len(truth.Evidence) > ActionEvidenceMaxItems {
		return ActionCompensationTruth{}, ErrActionEvidenceBounds
	}
	for i := range truth.Evidence {
		truth.Evidence[i], err = NormalizeActionEvidence(truth.Evidence[i])
		if err != nil {
			return ActionCompensationTruth{}, err
		}
	}
	if truth.Support == ActionCompensationUnavailable {
		if truth.Status != ActionCompensationNotAvailable || truth.Strategy != "" || truth.Trigger != "" || compensationHasRuntimeOutcome(truth) {
			return ActionCompensationTruth{}, fmt.Errorf("%w: unavailable compensation cannot carry runtime outcome", ErrInvalidActionResultV2)
		}
		return truth, nil
	}
	if truth.Support != ActionCompensationDeclared || truth.Strategy == "" || truth.Trigger == "" {
		return ActionCompensationTruth{}, fmt.Errorf("%w: declared compensation strategy and trigger required", ErrInvalidActionResultV2)
	}
	switch truth.Status {
	case ActionCompensationNotNeeded, ActionCompensationNotAttempted, ActionCompensationRunning, ActionCompensationSucceeded, ActionCompensationFailed, ActionCompensationInconclusive:
	default:
		return ActionCompensationTruth{}, fmt.Errorf("%w: unsupported compensation status %q", ErrInvalidActionResultV2, truth.Status)
	}
	if truth.Status == ActionCompensationNotNeeded || truth.Status == ActionCompensationNotAttempted {
		if compensationHasRuntimeOutcome(truth) {
			return ActionCompensationTruth{}, fmt.Errorf("%w: unattempted compensation cannot carry runtime outcome", ErrInvalidActionResultV2)
		}
		return truth, nil
	}
	if truth.AttemptID == "" || truth.StepID == "" || truth.StartedAt == nil {
		return ActionCompensationTruth{}, fmt.Errorf("%w: compensation runtime requires attempt, step, and start identity", ErrInvalidActionResultV2)
	}
	if truth.Status == ActionCompensationRunning {
		if truth.CompletedAt != nil || truth.Execution != nil || truth.Verification != nil || truth.RestoredState != nil {
			return ActionCompensationTruth{}, fmt.Errorf("%w: running compensation cannot carry terminal outcome", ErrInvalidActionResultV2)
		}
		return truth, nil
	}
	if truth.CompletedAt == nil || truth.CompletedAt.Before(*truth.StartedAt) {
		return ActionCompensationTruth{}, fmt.Errorf("%w: terminal compensation requires coherent completion time", ErrInvalidActionResultV2)
	}
	if (truth.Status == ActionCompensationFailed || truth.Status == ActionCompensationInconclusive) && truth.ReasonCode == "" {
		return ActionCompensationTruth{}, fmt.Errorf("%w: failed or inconclusive compensation requires reasonCode", ErrInvalidActionResultV2)
	}
	if truth.Execution != nil {
		normalized, err := normalizeExecutionTruth(*truth.Execution)
		if err != nil {
			return ActionCompensationTruth{}, err
		}
		truth.Execution = &normalized
	}
	if truth.Verification != nil {
		normalized, err := normalizeVerificationTruth(*truth.Verification)
		if err != nil {
			return ActionCompensationTruth{}, err
		}
		truth.Verification = &normalized
	}
	if truth.Execution == nil {
		return ActionCompensationTruth{}, fmt.Errorf("%w: terminal compensation requires execution truth", ErrInvalidActionResultV2)
	}
	if truth.RestoredState != nil {
		restored, err := normalizeRestoredStateTruth(*truth.RestoredState)
		if err != nil {
			return ActionCompensationTruth{}, err
		}
		truth.RestoredState = &restored
	}
	if truth.Status == ActionCompensationSucceeded {
		if truth.Execution.Status != ActionExecutionSucceeded || truth.Verification == nil || truth.Verification.Status != ActionVerificationConfirmed || truth.RestoredState == nil {
			return ActionCompensationTruth{}, fmt.Errorf("%w: compensation success requires successful execution and confirmed restoration", ErrInvalidActionResultV2)
		}
	} else if truth.Status == ActionCompensationFailed {
		if truth.Execution.Status != ActionExecutionFailed && (truth.Verification == nil || truth.Verification.Status != ActionVerificationContradicted) {
			return ActionCompensationTruth{}, fmt.Errorf("%w: failed compensation requires failed execution or contradicted restoration", ErrInvalidActionResultV2)
		}
	} else if truth.Status == ActionCompensationInconclusive {
		if truth.Execution.Status != ActionExecutionInconclusive && (truth.Verification == nil || truth.Verification.Status != ActionVerificationInconclusive) {
			return ActionCompensationTruth{}, fmt.Errorf("%w: inconclusive compensation requires an inconclusive axis", ErrInvalidActionResultV2)
		}
	}
	return truth, nil
}

func compensationHasRuntimeOutcome(truth ActionCompensationTruth) bool {
	return truth.AttemptID != "" || truth.StepID != "" || truth.StartedAt != nil || truth.CompletedAt != nil || len(truth.Evidence) != 0 || truth.Execution != nil || truth.Verification != nil || truth.RestoredState != nil
}

func normalizeRestoredStateTruth(truth ActionRestoredStateTruth) (ActionRestoredStateTruth, error) {
	var err error
	truth.SubjectID, err = boundedAuditText(truth.SubjectID, ActionEvidenceMaxTextBytes)
	if err != nil {
		return ActionRestoredStateTruth{}, err
	}
	truth.ExpectedDigest = strings.TrimSpace(truth.ExpectedDigest)
	truth.ObservedDigest = strings.TrimSpace(truth.ObservedDigest)
	if truth.SubjectID == "" || !validSHA256Digest(truth.ExpectedDigest) || truth.ExpectedDigest != truth.ObservedDigest || truth.ObservedAt.IsZero() {
		return ActionRestoredStateTruth{}, fmt.Errorf("%w: restored state requires matching durable digests and timestamp", ErrInvalidActionResultV2)
	}
	truth.ObservedAt = truth.ObservedAt.UTC()
	return truth, nil
}

func NormalizeActionResultV2(result ActionResultV2) (ActionResultV2, error) {
	if result.Version == 0 {
		result.Version = ActionResultV2Version
	}
	if result.Version != ActionResultV2Version {
		return ActionResultV2{}, fmt.Errorf("%w: unsupported version %d", ErrInvalidActionResultV2, result.Version)
	}
	var err error
	result.Execution, err = normalizeExecutionTruth(result.Execution)
	if err != nil {
		return ActionResultV2{}, err
	}
	result.Verification, err = normalizeVerificationTruth(result.Verification)
	if err != nil {
		return ActionResultV2{}, err
	}
	result.Compensation, err = normalizeCompensationTruth(result.Compensation)
	if err != nil {
		return ActionResultV2{}, err
	}
	return result, nil
}

func defaultCompensationTruth(rollbackAvailable bool) ActionCompensationTruth {
	if rollbackAvailable {
		return ActionCompensationTruth{Support: ActionCompensationDeclared, Strategy: "legacy_declared", Trigger: "legacy_rollback_declared", Status: ActionCompensationNotAttempted}
	}
	return ActionCompensationTruth{Support: ActionCompensationUnavailable, Status: ActionCompensationNotAvailable}
}

func legacyVerificationEvidence(verification *ActionVerificationResult, outcome VerificationOutcome, subjectID string, observedAt time.Time) ActionEvidence {
	summary := strings.TrimSpace(outcome.EvidenceSummary)
	method := "legacy_verification_outcome"
	if verification != nil {
		method = "agent_readback"
		if strings.TrimSpace(verification.Note) != "" {
			summary = verification.Note
		} else if strings.TrimSpace(verification.Output) != "" {
			summary = verification.Output
		}
		if !verification.RanAt.IsZero() {
			observedAt = verification.RanAt
		}
	}
	if observedAt.IsZero() {
		observedAt = time.Unix(1, 0).UTC()
	}
	evidence := ActionEvidence{
		Version: ActionEvidenceVersion, ID: "legacy-verification", ObserverID: "legacy-agent",
		ObserverKind:        "agent",
		ObserverTrustDomain: "legacy-agent", ExecutorTrustDomain: "legacy-agent", Method: method,
		SubjectID: strings.TrimSpace(subjectID), ObservedAt: observedAt.UTC(), ReceivedAt: observedAt.UTC(), Summary: summary,
	}
	if evidence.SubjectID == "" {
		evidence.SubjectID = "legacy-action"
	}
	normalized, err := NormalizeActionEvidence(evidence)
	if err == nil {
		return normalized
	}
	return evidence
}

func legacyReadbackContradiction(result *ExecutionResult) bool {
	if result == nil || result.Success || result.Verification == nil || !result.Verification.Ran || result.Verification.Success {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(result.ErrorMessage))
	return strings.Contains(message, "completed but verification did not confirm")
}

func legacyUnknownEffect(result *ExecutionResult) bool {
	if result == nil || result.Success {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(result.ErrorMessage))
	for _, marker := range []string{"context deadline exceeded", "deadline exceeded", "timed out", "timeout", "connection reset", "connection closed", "disconnected", "disconnect", "broken pipe", "unexpected eof"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// ActionResultV2FromLegacy converts the compatibility execution result and
// verification projection into canonical truth without inventing independence.
func ActionResultV2FromLegacy(result *ExecutionResult, outcome VerificationOutcome, rollbackAvailable bool, subjectID string, completedAt time.Time) ActionResultV2 {
	canonical := ActionResultV2{
		Version:      ActionResultV2Version,
		Execution:    ActionExecutionTruth{Status: ActionExecutionInconclusive, ReasonCode: "legacy_missing_result", Summary: "Legacy action recorded no executor result."},
		Verification: ActionVerificationTruth{Status: ActionVerificationNotAttempted, EvidenceClass: ActionEvidenceNone},
		Compensation: defaultCompensationTruth(rollbackAvailable),
	}
	if result == nil {
		normalized, _ := NormalizeActionResultV2(canonical)
		return normalized
	}
	if result.Success {
		canonical.Execution = ActionExecutionTruth{Status: ActionExecutionSucceeded, Summary: strings.TrimSpace(result.Output)}
	} else if legacyReadbackContradiction(result) {
		canonical.Execution = ActionExecutionTruth{Status: ActionExecutionSucceeded, Summary: strings.TrimSpace(result.Output)}
	} else if legacyUnknownEffect(result) {
		canonical.Execution = ActionExecutionTruth{Status: ActionExecutionInconclusive, ReasonCode: "legacy_unknown_effect", Summary: strings.TrimSpace(result.ErrorMessage)}
	} else {
		canonical.Execution = ActionExecutionTruth{Status: ActionExecutionFailed, ReasonCode: "legacy_execution_failed", Summary: strings.TrimSpace(result.ErrorMessage)}
	}
	rawVerificationNote := ""
	if result.Verification != nil {
		rawVerificationNote = strings.TrimSpace(result.Verification.Note)
	}
	verification := NormalizeActionVerificationResult(result.Verification)
	if verification != nil {
		if !verification.Ran {
			canonical.Verification = ActionVerificationTruth{Status: ActionVerificationInconclusive, EvidenceClass: ActionEvidenceNone, ReasonCode: "legacy_verification_inconclusive", Summary: rawVerificationNote}
		} else {
			status := ActionVerificationContradicted
			if verification.Success {
				status = ActionVerificationConfirmed
			}
			canonical.Verification = ActionVerificationTruth{Status: status, EvidenceClass: ActionEvidenceAgentAttested, Evidence: []ActionEvidence{legacyVerificationEvidence(verification, outcome, subjectID, completedAt)}}
		}
	} else {
		switch NormalizeVerificationOutcome(outcome).Status {
		case VerificationVerified:
			canonical.Verification = ActionVerificationTruth{Status: ActionVerificationConfirmed, EvidenceClass: ActionEvidenceAgentAttested, Evidence: []ActionEvidence{legacyVerificationEvidence(nil, outcome, subjectID, completedAt)}}
		case VerificationFailed:
			canonical.Verification = ActionVerificationTruth{Status: ActionVerificationContradicted, EvidenceClass: ActionEvidenceAgentAttested, Evidence: []ActionEvidence{legacyVerificationEvidence(nil, outcome, subjectID, completedAt)}}
		case VerificationUnverified:
			canonical.Verification = ActionVerificationTruth{Status: ActionVerificationInconclusive, EvidenceClass: ActionEvidenceNone, ReasonCode: "legacy_verification_inconclusive", Summary: outcome.EvidenceSummary}
		}
	}
	normalized, err := NormalizeActionResultV2(canonical)
	if err != nil {
		fallback := ActionResultV2{
			Version:      ActionResultV2Version,
			Execution:    ActionExecutionTruth{Status: ActionExecutionInconclusive, ReasonCode: "legacy_result_invalid", Summary: err.Error()},
			Verification: ActionVerificationTruth{Status: ActionVerificationInconclusive, EvidenceClass: ActionEvidenceNone, ReasonCode: "legacy_result_invalid"},
			Compensation: defaultCompensationTruth(rollbackAvailable),
		}
		normalized, _ = NormalizeActionResultV2(fallback)
	}
	return normalized
}

// CanonicalActionResultV2 returns normalized stored truth or derives it from
// the compatibility fields for older rows.
func CanonicalActionResultV2(record ActionAuditRecord) ActionResultV2 {
	if record.Result != nil && record.Result.ActionResultV2 != nil {
		if normalized, err := NormalizeActionResultV2(*record.Result.ActionResultV2); err == nil {
			return normalized
		}
		return invalidStoredActionResultV2(record.Plan.RollbackAvailable)
	}
	return ActionResultV2FromLegacy(record.Result, record.VerificationOutcome, record.Plan.RollbackAvailable, record.ID, record.UpdatedAt)
}

func invalidStoredActionResultV2(rollbackAvailable bool) ActionResultV2 {
	result := ActionResultV2{
		Version:      ActionResultV2Version,
		Execution:    ActionExecutionTruth{Status: ActionExecutionInconclusive, ReasonCode: "result_v2_invalid", Summary: "Stored action result violated the canonical contract."},
		Verification: ActionVerificationTruth{Status: ActionVerificationInconclusive, EvidenceClass: ActionEvidenceNone, ReasonCode: "result_v2_invalid"},
		Compensation: defaultCompensationTruth(rollbackAvailable),
	}
	normalized, err := NormalizeActionResultV2(result)
	if err != nil {
		panic("canonical invalid-result fallback is invalid: " + err.Error())
	}
	return normalized
}

// ApplyActionResultV2 derives compatibility fields from canonical truth.
func ApplyActionResultV2(result *ExecutionResult, canonical ActionResultV2) (*ExecutionResult, VerificationOutcome, error) {
	normalized, err := NormalizeActionResultV2(canonical)
	if err != nil {
		return nil, VerificationOutcome{}, err
	}
	if result == nil {
		result = &ExecutionResult{}
	} else {
		clone := *result
		result = &clone
	}
	result.ActionResultV2 = &normalized
	result.Success = normalized.Execution.Status == ActionExecutionSucceeded
	if normalized.Execution.Status != ActionExecutionSucceeded && strings.TrimSpace(result.ErrorMessage) == "" {
		result.ErrorMessage = strings.TrimSpace(normalized.Execution.Summary)
		if result.ErrorMessage == "" {
			result.ErrorMessage = normalized.Execution.ReasonCode
		}
	}
	legacy := VerificationOutcome{Status: VerificationUnknown}
	switch normalized.Verification.Status {
	case ActionVerificationConfirmed:
		legacy.Status = VerificationVerified
	case ActionVerificationContradicted:
		legacy.Status = VerificationFailed
	case ActionVerificationInconclusive:
		legacy.Status = VerificationUnverified
	}
	legacy.EvidenceSummary = normalized.Verification.Summary
	if legacy.EvidenceSummary == "" && len(normalized.Verification.Evidence) > 0 {
		legacy.EvidenceSummary = normalized.Verification.Evidence[0].Summary
	}
	return result, legacy, nil
}

// LegacyActionVerificationFromV2 derives the bounded compatibility readback
// projection without allowing legacy fields to override canonical truth.
func LegacyActionVerificationFromV2(canonical ActionResultV2) *ActionVerificationResult {
	switch canonical.Verification.Status {
	case ActionVerificationConfirmed:
		return &ActionVerificationResult{Ran: true, Success: true, Note: canonical.Verification.Summary}
	case ActionVerificationContradicted:
		return &ActionVerificationResult{Ran: true, Success: false, Note: canonical.Verification.Summary}
	case ActionVerificationInconclusive:
		return &ActionVerificationResult{Ran: false, Success: false}
	default:
		return nil
	}
}

// LegacyActionResultProjection derives the one-window compatibility result
// from canonical truth, including fail-closed handling of malformed stored V2.
func LegacyActionResultProjection(record ActionAuditRecord) *ExecutionResult {
	canonical := CanonicalActionResultV2(record)
	projected, _, err := ApplyActionResultV2(record.Result, canonical)
	if err != nil {
		projected, _, _ = ApplyActionResultV2(nil, invalidStoredActionResultV2(record.Plan.RollbackAvailable))
	}
	projected.Verification = LegacyActionVerificationFromV2(canonical)
	return projected
}

// ExecutorContractViolationResult returns durable inconclusive truth for an
// executor tuple that cannot be trusted.
func ExecutorContractViolationResult(reasonCode, summary string, rollbackAvailable bool) *ExecutionResult {
	canonical := ActionResultV2{
		Version:      ActionResultV2Version,
		Execution:    ActionExecutionTruth{Status: ActionExecutionInconclusive, ReasonCode: strings.TrimSpace(reasonCode), Summary: strings.TrimSpace(summary)},
		Verification: ActionVerificationTruth{Status: ActionVerificationInconclusive, EvidenceClass: ActionEvidenceNone, ReasonCode: "execution_outcome_inconclusive"},
		Compensation: defaultCompensationTruth(rollbackAvailable),
	}
	result, _, err := ApplyActionResultV2(nil, canonical)
	if err != nil {
		return &ExecutionResult{Success: false, ErrorMessage: ErrExecutorResultContract.Error()}
	}
	return result
}

// KnownNoEffectResult records a typed refusal that is known to have stopped
// before dispatch. It is failed lifecycle admission with execution not_run,
// not an executor failure and not an unknown-effect attempt.
func KnownNoEffectResult(reasonCode, summary string, rollbackAvailable bool) *ExecutionResult {
	canonical := ActionResultV2{
		Version:      ActionResultV2Version,
		Execution:    ActionExecutionTruth{Status: ActionExecutionNotRun, ReasonCode: strings.TrimSpace(reasonCode), Summary: strings.TrimSpace(summary)},
		Verification: ActionVerificationTruth{Status: ActionVerificationNotAttempted, EvidenceClass: ActionEvidenceNone},
		Compensation: defaultCompensationTruth(rollbackAvailable),
	}
	result, _, err := ApplyActionResultV2(&ExecutionResult{ErrorMessage: strings.TrimSpace(summary)}, canonical)
	if err != nil {
		return ExecutorContractViolationResult("known_no_effect_result_invalid", "Known-no-effect result violated its canonical contract.", rollbackAvailable)
	}
	return result
}
