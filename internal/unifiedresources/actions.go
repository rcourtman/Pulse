package unifiedresources

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ActionState tracks the lifecycle of bounded capability execution.
type ActionState string

const (
	ActionStatePlanned   ActionState = "planned"
	ActionStatePending   ActionState = "pending_approval"
	ActionStateApproved  ActionState = "approved"
	ActionStateRejected  ActionState = "rejected"
	ActionStateExpired   ActionState = "expired"
	ActionStateExecuting ActionState = "executing"
	ActionStateCompleted ActionState = "completed"
	ActionStateFailed    ActionState = "failed"
)

// ActionRequest is the payload from an agent or human requesting a capability execution.
type ActionRequest struct {
	RequestID      string         `json:"requestId"` // Caller idempotency key / external correlation
	ResourceID     string         `json:"resourceId"`
	CapabilityName string         `json:"capabilityName"`
	Params         map[string]any `json:"params,omitempty"`
	Reason         string         `json:"reason"`
	// RequestedBy remains on the wire for compatibility and presentation, but
	// canonical planning derives it from Actor.SubjectID. Public callers never
	// own this value.
	RequestedBy string      `json:"requestedBy"`
	Actor       ActionActor `json:"actor"`
}

// ActionActor is the immutable server-owned identity bound to a governed
// action. SubjectID is the durable principal; CredentialID identifies the
// authenticated credential without persisting a secret.
type ActionActor struct {
	SubjectID    string          `json:"subjectId"`
	Kind         ActionActorKind `json:"kind"`
	CredentialID string          `json:"credentialId"`
	OrgID        string          `json:"orgId"`
}

type ActionActorKind string

const (
	ActionActorUser     ActionActorKind = "user"
	ActionActorAPIToken ActionActorKind = "api_token"
	ActionActorService  ActionActorKind = "service"
	ActionActorPolicy   ActionActorKind = "policy"
)

func NormalizeActionActor(actor ActionActor) ActionActor {
	actor.SubjectID = strings.TrimSpace(actor.SubjectID)
	actor.CredentialID = strings.TrimSpace(actor.CredentialID)
	actor.OrgID = strings.TrimSpace(actor.OrgID)
	return actor
}

func ValidateActionActor(actor ActionActor) error {
	actor = NormalizeActionActor(actor)
	if actor.SubjectID == "" || actor.CredentialID == "" || actor.OrgID == "" {
		return errors.New("action actor subject, credential, and organization are required")
	}
	switch actor.Kind {
	case ActionActorUser, ActionActorAPIToken, ActionActorService, ActionActorPolicy:
		return nil
	default:
		return fmt.Errorf("unsupported action actor kind %q", actor.Kind)
	}
}

func ActionActorsEqual(left, right ActionActor) bool {
	left = NormalizeActionActor(left)
	right = NormalizeActionActor(right)
	return left == right
}

const ActionApprovalRequirementVersion = 1

// ApprovalRequirement is the capability-owned approval floor captured at
// planning time. Tenant policy may strengthen this structure in a future
// resolver, but it must never lower Floor.
type ApprovalRequirement struct {
	Version           int                 `json:"version"`
	Floor             ActionApprovalLevel `json:"floor"`
	Quorum            int                 `json:"quorum"`
	DisallowRequester bool                `json:"disallowRequester"`
}

func ApprovalRequirementForFloor(floor ActionApprovalLevel) ApprovalRequirement {
	return ApprovalRequirement{
		Version: ActionApprovalRequirementVersion,
		Floor:   floor,
		Quorum:  1,
	}
}

func NormalizeApprovalRequirement(requirement ApprovalRequirement, legacyFloor ActionApprovalLevel) ApprovalRequirement {
	if requirement.Floor == "" {
		requirement.Floor = legacyFloor
	}
	if requirement.Quorum < 1 {
		requirement.Quorum = 1
	}
	return requirement
}

func ValidateApprovalRequirement(requirement ApprovalRequirement, capabilityFloor ActionApprovalLevel) error {
	requirement = NormalizeApprovalRequirement(requirement, capabilityFloor)
	if requirement.Version != ActionApprovalRequirementVersion || requirement.Quorum < 1 {
		return errors.New("approval requirement version and quorum are invalid")
	}
	if capabilityFloor == ApprovalDryRun {
		if requirement.Floor != ApprovalDryRun {
			return errors.New("dry-run-only capability floor cannot be lowered")
		}
		return nil
	}
	if requirement.Floor == ApprovalDryRun {
		return nil
	}
	rank := func(level ActionApprovalLevel) int {
		switch level {
		case ApprovalNone:
			return 0
		case ApprovalAdmin:
			return 1
		case ApprovalMultiFactor:
			return 2
		default:
			return -1
		}
	}
	if rank(requirement.Floor) < rank(capabilityFloor) || rank(requirement.Floor) < 0 {
		return errors.New("approval requirement cannot lower the capability floor")
	}
	return nil
}

// ApprovalOutcome represents the decision on a requested capability.
type ApprovalOutcome string

const (
	OutcomeApproved ApprovalOutcome = "approved"
	OutcomeRejected ApprovalOutcome = "rejected"
)

// ApprovalMethod tracks how the decision was collected.
type ApprovalMethod string

const (
	MethodUI           ApprovalMethod = "ui"
	MethodAPI          ApprovalMethod = "api"
	MethodMFAChallenge ApprovalMethod = "mfa_challenge"
	MethodPolicy       ApprovalMethod = "policy"
	MethodSession      ApprovalMethod = "session"
	MethodAPIToken     ApprovalMethod = "api_token"
	MethodWebAuthnUV   ApprovalMethod = "webauthn_uv"
	MethodDeviceKeyUV  ApprovalMethod = "device_key_uv"
)

// ApprovalEvidence is the server-checked decision binding. Cryptographic
// methods are never trusted from this structure alone: actionlifecycle calls
// its installed verifier to validate and atomically consume the challenge.
type ApprovalEvidence struct {
	Version     int             `json:"version"`
	Method      ApprovalMethod  `json:"method"`
	Actor       ActionActor     `json:"actor"`
	OrgID       string          `json:"orgId"`
	ActionID    string          `json:"actionId"`
	PlanHash    string          `json:"planHash"`
	Outcome     ApprovalOutcome `json:"outcome"`
	ChallengeID string          `json:"challengeId,omitempty"`
	IssuedAt    time.Time       `json:"issuedAt"`
	ExpiresAt   time.Time       `json:"expiresAt,omitempty"`
}

// ActionDecision is the transport-independent input to the canonical human
// decision boundary. Actor and Evidence are supplied by a trusted adapter,
// never decoded as public identity authority.
type ActionDecision struct {
	Actor    ActionActor      `json:"actor"`
	Outcome  ApprovalOutcome  `json:"outcome"`
	Reason   string           `json:"reason,omitempty"`
	Evidence ApprovalEvidence `json:"evidence"`
}

// ActionApprovalRecord captures a specific approval or rejection event.
type ActionApprovalRecord struct {
	Actor        string            `json:"actor"`     // Who approved/rejected it
	Method       ApprovalMethod    `json:"method"`    // e.g. "ui", "api", "mfa_challenge"
	Timestamp    time.Time         `json:"timestamp"` // When the decision was made
	Outcome      ApprovalOutcome   `json:"outcome"`   // "approved" or "rejected"
	Reason       string            `json:"reason,omitempty"`
	ActorBinding ActionActor       `json:"actorBinding,omitempty"`
	Evidence     *ApprovalEvidence `json:"evidence,omitempty"`
	// PolicyLease is present only for a server-owned policy authorization.
	// Human decisions never carry this field. Policy approval and execution
	// admission are persisted atomically, so the lease cannot become reusable.
	PolicyLease *ActionPolicyAuthorizationLease `json:"policyLease,omitempty"`
}

// ActionPolicyAuthorizationLease binds automatic authorization to every
// policy input that was current at the executing transition.
type ActionPolicyAuthorizationLease struct {
	Version                 int                          `json:"version"`
	OrgID                   string                       `json:"orgId"`
	ActionID                string                       `json:"actionId"`
	ResourceID              string                       `json:"resourceId"`
	CapabilityName          string                       `json:"capabilityName"`
	PlanHash                string                       `json:"planHash"`
	CapabilityPolicyVersion string                       `json:"capabilityPolicyVersion"`
	AutoAuthorization       ActionAutoAuthorizationClass `json:"autoAuthorization"`
	ApprovalPolicy          ActionApprovalLevel          `json:"approvalPolicy"`
	TenantPolicyVersion     string                       `json:"tenantPolicyVersion"`
	EffectiveAutonomyLevel  string                       `json:"effectiveAutonomyLevel"`
	LicenseAllowsAutoFix    bool                         `json:"licenseAllowsAutoFix"`
	FullModeUnlocked        bool                         `json:"fullModeUnlocked"`
	EmergencyStop           bool                         `json:"emergencyStop"`
	ResourcePolicyVersion   string                       `json:"resourcePolicyVersion"`
	CapabilityNames         []string                     `json:"capabilityNames"`
	Window                  *AutoRemediationWindow       `json:"window,omitempty"`
	NeverAutoRemediate      bool                         `json:"neverAutoRemediate"`
	IssuedAt                time.Time                    `json:"issuedAt"`
	ExpiresAt               time.Time                    `json:"expiresAt"`
	Digest                  string                       `json:"digest"`
}

// ActionPolicyAuthorizationDigest returns the canonical SHA-256 binding for
// a lease. Digest itself is excluded from the encoded payload.
func ActionPolicyAuthorizationDigest(lease ActionPolicyAuthorizationLease) string {
	lease.Digest = ""
	payload, _ := json.Marshal(lease)
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("sha256:%x", sum[:])
}

// ValidateActionPolicyAuthorizationLease fails closed on incomplete,
// malformed, expired, or tampered automatic authority.
func ValidateActionPolicyAuthorizationLease(lease ActionPolicyAuthorizationLease, record ActionAuditRecord, now time.Time) error {
	if lease.Version != 1 || strings.TrimSpace(lease.OrgID) == "" ||
		lease.ActionID != record.ID || lease.ResourceID != CanonicalResourceID(record.Request.ResourceID) ||
		lease.CapabilityName != strings.TrimSpace(record.Request.CapabilityName) ||
		lease.PlanHash != record.Plan.PlanHash || lease.CapabilityPolicyVersion != record.Plan.PolicyVersion ||
		strings.TrimSpace(lease.TenantPolicyVersion) == "" || strings.TrimSpace(lease.ResourcePolicyVersion) == "" {
		return ErrActionPolicyAuthorizationInvalid
	}
	if lease.EmergencyStop || lease.NeverAutoRemediate || !lease.LicenseAllowsAutoFix {
		return ErrActionPolicyAuthorizationRevoked
	}
	if lease.IssuedAt.IsZero() || lease.ExpiresAt.IsZero() || !now.Before(lease.ExpiresAt) {
		return ErrActionPolicyAuthorizationExpired
	}
	if lease.Digest == "" || lease.Digest != ActionPolicyAuthorizationDigest(lease) {
		return ErrActionPolicyAuthorizationInvalid
	}
	return nil
}

// ActionPreflight is the deterministic pre-execution readout shown before an
// action is approved or executed. It is intentionally explicit when no provider
// dry-run exists, so action audits can distinguish "not available" from
// "not recorded".
type ActionPreflight struct {
	Target            string    `json:"target,omitempty"`
	CurrentState      string    `json:"currentState,omitempty"`
	IntendedChange    string    `json:"intendedChange,omitempty"`
	DryRunAvailable   bool      `json:"dryRunAvailable"`
	DryRunSummary     string    `json:"dryRunSummary,omitempty"`
	SafetyChecks      []string  `json:"safetyChecks,omitempty"`
	VerificationSteps []string  `json:"verificationSteps,omitempty"`
	GeneratedAt       time.Time `json:"generatedAt,omitempty"`
}

// ActionPlan is the deterministic response Pulse gives back before execution.
type ActionPlan struct {
	ActionID             string              `json:"actionId"` // Internal durable identity
	RequestID            string              `json:"requestId"`
	Allowed              bool                `json:"allowed"`
	RequiresApproval     bool                `json:"requiresApproval"`
	ApprovalPolicy       ActionApprovalLevel `json:"approvalPolicy"`
	ApprovalRequirement  ApprovalRequirement `json:"approvalRequirement"`
	PredictedBlastRadius []string            `json:"predictedBlastRadius,omitempty"` // Correlated related resources
	RollbackAvailable    bool                `json:"rollbackAvailable"`
	Message              string              `json:"message,omitempty"`

	// Stale-plan protection
	PlannedAt       time.Time                      `json:"plannedAt"`
	ExpiresAt       time.Time                      `json:"expiresAt"`
	ResourceVersion string                         `json:"resourceVersion"` // Hash of the resource state at planning time
	PolicyVersion   string                         `json:"policyVersion"`   // Version of the capability/policy when planned
	PolicyDecision  ActionPolicyDecisionProvenance `json:"policyDecision"`
	PlanHash        string                         `json:"planHash"` // Hash verifying params and resource state haven't drifted
	Preflight       *ActionPreflight               `json:"preflight,omitempty"`
}

// ExecutionResult captures the output of the native capability driver.
type ExecutionResult struct {
	Success        bool                      `json:"success"`
	Output         string                    `json:"output,omitempty"`
	ErrorMessage   string                    `json:"errorMessage,omitempty"`
	Verification   *ActionVerificationResult `json:"verification,omitempty"`
	ActionResultV2 *ActionResultV2           `json:"actionResultV2,omitempty"`
}

// ActionVerificationResult records the outcome of a post-execution
// read-after-write check. The broker derives a class-specific verification
// command (e.g. `systemctl is-active <unit>` after a service-restart action)
// and runs it via the same agent path used for the original dispatch. The
// result is persisted alongside ExecutionResult so the audit history shows
// not only what the action did but whether the read-back confirmed the
// intended state. Verification is best-effort: if the agent is unreachable
// or no verification command is derivable for the action class, Ran is
// false and the rest of the fields are empty rather than fabricated.
type ActionVerificationResult struct {
	Ran     bool      `json:"ran"`
	Command string    `json:"command,omitempty"`
	Output  string    `json:"output,omitempty"`
	Success bool      `json:"success"`
	RanAt   time.Time `json:"ranAt,omitempty"`
	Note    string    `json:"note,omitempty"`
}

// VerificationStatus classifies the post-execution read-after-write outcome
// for an action.
//
// The progression is read as: did Pulse confirm the intended state after the
// write?
//   - VerificationUnknown:    no verification has been attempted yet, or the
//     record predates the verifier substrate. Default for older audits on
//     read so the contract surface stays additive.
//   - VerificationVerified:   a postcondition was evaluated and matched.
//   - VerificationUnverified: a postcondition was attempted but could not
//     conclude (e.g. agent unreachable inside the window). Distinct from
//     failure so operators are not misled into thinking the action broke.
//   - VerificationFailed:     a postcondition was evaluated and did not
//     match - the write claims to have succeeded but the world disagrees.
type VerificationStatus string

const (
	VerificationUnknown    VerificationStatus = "unknown"
	VerificationVerified   VerificationStatus = "verified"
	VerificationUnverified VerificationStatus = "unverified"
	VerificationFailed     VerificationStatus = "failed"
)

// VerificationOutcome is the durable verification projection persisted on
// the action audit record. EvidenceSummary carries an optional one-line
// human-readable note about what evidence (or its absence) drove the
// status - e.g. "systemctl ActiveState=active observed within 12s".
type VerificationOutcome struct {
	Status          VerificationStatus `json:"status"`
	EvidenceSummary string             `json:"evidenceSummary,omitempty"`
}

// IsValidVerificationStatus reports whether the given status is one of the
// closed enum values.
func IsValidVerificationStatus(status VerificationStatus) bool {
	switch status {
	case VerificationUnknown, VerificationVerified, VerificationUnverified, VerificationFailed:
		return true
	default:
		return false
	}
}

// NormalizeVerificationOutcome coerces empty or unrecognized statuses to
// VerificationUnknown so older audit records read without breaking the
// closed enum contract, and trims EvidenceSummary.
func NormalizeVerificationOutcome(outcome VerificationOutcome) VerificationOutcome {
	outcome.EvidenceSummary = strings.TrimSpace(outcome.EvidenceSummary)
	if outcome.Status == "" || !IsValidVerificationStatus(outcome.Status) {
		outcome.Status = VerificationUnknown
	}
	return outcome
}

// VerificationOutcomeFromExecutionResult derives the durable lifecycle
// classification from the executor's read-after-write result at the single
// completion boundary. Executors report evidence; they do not get to write a
// separate audit classification that can drift from it.
func VerificationOutcomeFromExecutionResult(result *ExecutionResult) VerificationOutcome {
	if result == nil || !result.Success || result.Verification == nil {
		return VerificationOutcome{Status: VerificationUnknown}
	}
	note := strings.TrimSpace(result.Verification.Note)
	verification := NormalizeActionVerificationResult(result.Verification)
	if verification == nil || !verification.Ran {
		return VerificationOutcome{Status: VerificationUnverified, EvidenceSummary: note}
	}
	if verification.Success {
		return VerificationOutcome{Status: VerificationVerified, EvidenceSummary: strings.TrimSpace(verification.Note)}
	}
	return VerificationOutcome{Status: VerificationFailed, EvidenceSummary: strings.TrimSpace(verification.Note)}
}

// NormalizeActionVerificationResult applies the canonical verification field
// hygiene used by stored audit records and every action-audit projection.
func NormalizeActionVerificationResult(result *ActionVerificationResult) *ActionVerificationResult {
	if result == nil {
		return nil
	}
	normalized := *result
	normalized.Command = strings.TrimSpace(normalized.Command)
	normalized.Output = strings.TrimSpace(normalized.Output)
	normalized.Note = strings.TrimSpace(normalized.Note)
	if normalized.RanAt.IsZero() {
		normalized.RanAt = time.Time{}
	} else {
		normalized.RanAt = normalized.RanAt.UTC()
	}
	if !normalized.Ran {
		normalized.Command = ""
		normalized.Output = ""
		normalized.Note = ""
		normalized.Success = false
		normalized.RanAt = time.Time{}
	}
	return &normalized
}

func cloneActionVerificationResult(result *ActionVerificationResult) *ActionVerificationResult {
	if result == nil {
		return nil
	}
	clone := *result
	return &clone
}

// CanonicalActionVerification returns the top-level verification projection for
// an audit record, falling back to legacy result.verification records while
// older persisted rows are still being read.
func CanonicalActionVerification(record ActionAuditRecord) *ActionVerificationResult {
	if record.Result != nil && record.Result.ActionResultV2 != nil {
		canonical := CanonicalActionResultV2(record)
		projected := LegacyActionVerificationFromV2(canonical)
		if projected == nil || !projected.Ran {
			return NormalizeActionVerificationResult(projected)
		}
		for _, candidate := range []*ActionVerificationResult{record.Verification, record.Result.Verification} {
			compatibility := NormalizeActionVerificationResult(candidate)
			if compatibility != nil && compatibility.Ran && compatibility.Success == projected.Success {
				return compatibility
			}
		}
		return NormalizeActionVerificationResult(projected)
	}
	if result := NormalizeActionVerificationResult(record.Verification); result != nil {
		return result
	}
	if record.Result == nil {
		return nil
	}
	return NormalizeActionVerificationResult(record.Result.Verification)
}

// ActionAuditRecord tracks the full end-to-end lifecycle of a tool invocation.
type ActionAuditRecord struct {
	ID                  string                    `json:"id"`
	CreatedAt           time.Time                 `json:"createdAt"`
	UpdatedAt           time.Time                 `json:"updatedAt"`
	State               ActionState               `json:"state"`
	DecisionRevision    uint64                    `json:"decisionRevision"`
	Request             ActionRequest             `json:"request"`
	Plan                ActionPlan                `json:"plan"`
	Origin              *ActionOrigin             `json:"origin,omitempty"`
	Approvals           []ActionApprovalRecord    `json:"approvals,omitempty"`
	Result              *ExecutionResult          `json:"result,omitempty"`
	Verification        *ActionVerificationResult `json:"verification,omitempty"`
	VerificationOutcome VerificationOutcome       `json:"verificationOutcome"`
}

// ActionOrigin records which internal surface proposed the action and the
// correlation identifiers that surface needs to reconcile decisions and
// terminal outcomes back onto its own records (e.g. a Patrol finding).
// Origin is broker-owned metadata: it is set by in-process planning callers
// only and is never accepted from the public plan request body.
type ActionOrigin struct {
	Surface             string   `json:"surface"`
	FindingID           string   `json:"findingId,omitempty"`
	InvestigationID     string   `json:"investigationId,omitempty"`
	ProposalID          string   `json:"proposalId,omitempty"`
	OperationalRecordID string   `json:"operationalRecordId,omitempty"`
	EvidenceIDs         []string `json:"evidenceIds,omitempty"`
}

// NormalizeActionOrigin trims origin fields and collapses an all-empty
// origin to nil so absent metadata never persists as an empty object.
func NormalizeActionOrigin(origin *ActionOrigin) *ActionOrigin {
	if origin == nil {
		return nil
	}
	normalized := ActionOrigin{
		Surface:             strings.TrimSpace(origin.Surface),
		FindingID:           strings.TrimSpace(origin.FindingID),
		InvestigationID:     strings.TrimSpace(origin.InvestigationID),
		ProposalID:          strings.TrimSpace(origin.ProposalID),
		OperationalRecordID: strings.TrimSpace(origin.OperationalRecordID),
		EvidenceIDs:         uniqueStrings(origin.EvidenceIDs),
	}
	sort.Strings(normalized.EvidenceIDs)
	if normalized.Surface == "" &&
		normalized.FindingID == "" &&
		normalized.InvestigationID == "" &&
		normalized.ProposalID == "" &&
		normalized.OperationalRecordID == "" &&
		len(normalized.EvidenceIDs) == 0 {
		return nil
	}
	return &normalized
}

type ActionLifecycleEventKind string

const (
	ActionLifecycleEventTransition ActionLifecycleEventKind = "transition"
	ActionLifecycleEventDecision   ActionLifecycleEventKind = "decision"
	ActionLifecycleEventLegacy     ActionLifecycleEventKind = "legacy"
)

// ActionLifecycleEvent is an append-only action audit fact. State transitions
// remain unique by action/state; human decisions are independently unique by
// action/decision revision and carry their complete server-bound approval.
type ActionLifecycleEvent struct {
	ActionID         string                   `json:"actionId"`
	Timestamp        time.Time                `json:"timestamp"`
	State            ActionState              `json:"state"`
	Kind             ActionLifecycleEventKind `json:"kind"`
	DecisionRevision uint64                   `json:"decisionRevision,omitempty"`
	Decision         *ActionApprovalRecord    `json:"decision,omitempty"`
	Actor            string                   `json:"actor,omitempty"`
	Message          string                   `json:"message,omitempty"`
}

// ActionEngine defines the enforced runtime loop for capabilities.
type ActionEngine interface {
	PlanAction(req ActionRequest) (*ActionPlan, error)
	ApproveAction(actionID string, approval ActionApprovalRecord) error
	ExecuteAction(actionID string) (*ExecutionResult, error)
}

var (
	ErrActionNotPending  = errors.New("action is not pending approval")
	ErrActionNotApproved = errors.New("action is not approved for execution")
	// ErrActionPlanDrift is returned when the payload presented at execute
	// time does not match the plan hash recorded at approval time. The hash
	// is the contract for "the operator approved exactly this action"; if
	// it does not match, the broker must refuse execution rather than run a
	// drifted plan under a stale approval.
	ErrActionPlanDrift = errors.New("approved plan hash does not match execution payload; refusing to run drifted plan")
	// ErrResourceRemediationLocked is returned when the operator has set
	// NeverAutoRemediate=true on the target resource. The action broker
	// must refuse the dispatch even when the approval ID resolves and
	// the plan hash matches; the operator's per-resource intent
	// outranks the per-action approval. Persists a Failed audit
	// record with `resource_remediation_locked:` prefix on the
	// ErrorMessage so the audit timeline shows every refused dispatch.
	ErrResourceRemediationLocked        = errors.New("resource is operator-locked against automated remediation")
	ErrActionNotExecuting               = errors.New("action is not executing")
	ErrActionAlreadyExecuting           = errors.New("action is already executing")
	ErrActionExecutionFinal             = errors.New("action execution is already final")
	ErrActionPlanExpired                = errors.New("action plan expired")
	ErrActionPlanNotExpired             = errors.New("action plan has not expired")
	ErrActionDryRunOnly                 = errors.New("action plan is dry-run only")
	ErrActionExecutionRefusal           = errors.New("action execution refusal is not a permanent terminal refusal")
	ErrInvalidApprovalOutcome           = errors.New("invalid approval outcome")
	ErrActionAuditAlreadyExists         = errors.New("action audit already exists")
	ErrActionIdentityConflict           = errors.New("action audit identity conflicts with the persisted record")
	ErrActionDecisionRevisionConflict   = errors.New("action decision revision conflicts with the persisted record")
	ErrActionPolicyAuthorizationInvalid = errors.New("policy_authorization_invalid")
	ErrActionPolicyAuthorizationExpired = errors.New("policy_authorization_expired")
	ErrActionPolicyAuthorizationRevoked = errors.New("policy_authorization_revoked")
	ErrActionEmergencyStop              = errors.New("action_emergency_stop")
	ErrActionReplanRequired             = errors.New("action_replan_required")
	ErrDuplicateApprovalActor           = errors.New("duplicate approval actor")
)

// BeginPolicyActionExecution atomically composes the policy approval and
// executing record. Stores persist both lifecycle events in one CAS write.
func BeginPolicyActionExecution(record ActionAuditRecord, approval ActionApprovalRecord, lease ActionPolicyAuthorizationLease, now time.Time) (ActionAuditRecord, ActionLifecycleEvent, ActionLifecycleEvent, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if record.State != ActionStatePending && record.State != ActionStatePlanned {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ActionLifecycleEvent{}, ErrActionNotApproved
	}
	if err := ValidateActionExecutionStart(record, now); err != nil && !errors.Is(err, ErrActionNotApproved) {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ActionLifecycleEvent{}, err
	}
	if err := ValidateActionPolicyAuthorizationLease(lease, record, now); err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ActionLifecycleEvent{}, err
	}
	approval.Actor = strings.TrimSpace(approval.Actor)
	approval.Method = MethodPolicy
	approval.Outcome = OutcomeApproved
	approval.Timestamp = now
	approval.PolicyLease = &lease
	record.Approvals = append(record.Approvals, approval)
	record.State = ActionStateExecuting
	record.UpdatedAt = now
	normalized, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ActionLifecycleEvent{}, err
	}
	approvedEvent := ActionLifecycleEvent{ActionID: record.ID, Timestamp: now, State: ActionStateApproved, Actor: approval.Actor, Message: "Action authorized by current Patrol policy."}
	startedEvent := ActionLifecycleEvent{ActionID: record.ID, Timestamp: now, State: ActionStateExecuting, Actor: approval.Actor, Message: "Policy-authorized action execution started."}
	return normalized, approvedEvent, startedEvent, nil
}

// ActionAuditIdentityMatches reports whether a replay addresses the same
// immutable governed action. Lifecycle state, timestamps, approvals, results,
// and verification are deliberately excluded because the persisted record is
// authoritative for those mutable fields.
func ActionAuditIdentityMatches(existing, replay ActionAuditRecord) bool {
	existing, existingErr := NormalizeActionAuditRecord(existing)
	replay, replayErr := NormalizeActionAuditRecord(replay)
	if existingErr != nil || replayErr != nil {
		return false
	}
	return existing.ID == replay.ID &&
		existing.Plan.ActionID == replay.Plan.ActionID &&
		existing.Plan.PlanHash == replay.Plan.PlanHash &&
		canonicalActionIdentityJSONEqual(existing.Request, replay.Request) &&
		canonicalActionIdentityJSONEqual(existing.Origin, replay.Origin)
}

func canonicalActionIdentityJSONEqual(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

// ApplyActionDecision records an explicit approval or rejection against a
// pending governed action without starting execution. Execution remains a
// separate contract so approvals cannot become an implicit control bypass.
func ApplyActionDecision(record ActionAuditRecord, approval ActionApprovalRecord, now time.Time) (ActionAuditRecord, ActionLifecycleEvent, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	approval.ActorBinding = NormalizeActionActor(approval.ActorBinding)
	if approval.ActorBinding.SubjectID != "" {
		approval.Actor = approval.ActorBinding.SubjectID
	}
	approval.Actor = strings.TrimSpace(approval.Actor)
	approval.Reason = strings.TrimSpace(approval.Reason)
	if approval.Method == "" {
		approval.Method = MethodAPI
	}
	if approval.Timestamp.IsZero() {
		approval.Timestamp = now
	} else {
		approval.Timestamp = approval.Timestamp.UTC()
	}

	for _, existing := range record.Approvals {
		if approval.Actor != "" && strings.EqualFold(strings.TrimSpace(existing.Actor), approval.Actor) {
			return ActionAuditRecord{}, ActionLifecycleEvent{}, ErrDuplicateApprovalActor
		}
	}

	var nextState ActionState
	var message string
	switch approval.Outcome {
	case OutcomeApproved:
		requirement := NormalizeApprovalRequirement(record.Plan.ApprovalRequirement, record.Plan.ApprovalPolicy)
		approvedActors := map[string]struct{}{}
		for _, existing := range record.Approvals {
			if existing.Outcome == OutcomeApproved && strings.TrimSpace(existing.Actor) != "" {
				approvedActors[strings.ToLower(strings.TrimSpace(existing.Actor))] = struct{}{}
			}
		}
		approvedActors[strings.ToLower(approval.Actor)] = struct{}{}
		if len(approvedActors) < requirement.Quorum {
			nextState = ActionStatePending
			message = fmt.Sprintf("Approval recorded; %d of %d distinct approvals collected.", len(approvedActors), requirement.Quorum)
		} else {
			nextState = ActionStateApproved
			message = "Action approved. Execution remains pending a separate execution contract."
		}
	case OutcomeRejected:
		nextState = ActionStateRejected
		message = "Action rejected before execution."
	default:
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ErrInvalidApprovalOutcome
	}
	if record.State != ActionStatePending {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ErrActionNotPending
	}
	if !record.Plan.ExpiresAt.IsZero() && !now.Before(record.Plan.ExpiresAt) {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ErrActionPlanExpired
	}

	record.State = nextState
	record.UpdatedAt = approval.Timestamp
	record.Approvals = append(record.Approvals, approval)
	record.DecisionRevision++
	normalized, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, err
	}
	event := ActionLifecycleEvent{
		ActionID:         normalized.ID,
		Timestamp:        approval.Timestamp,
		State:            nextState,
		Kind:             ActionLifecycleEventDecision,
		DecisionRevision: normalized.DecisionRevision,
		Decision:         &approval,
		Actor:            approval.Actor,
		Message:          message,
	}
	normalizedEvent, err := NormalizeActionLifecycleEvent(event)
	if err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, err
	}
	return normalized, normalizedEvent, nil
}

// ActionDecisionAppendCommand is the canonical pure store command for one
// append-only human decision. Both persistence implementations validate and
// execute this same command before applying their atomic CAS mechanics.
type ActionDecisionAppendCommand struct {
	Record          ActionAuditRecord
	DecisionEvent   ActionLifecycleEvent
	TransitionEvent *ActionLifecycleEvent
}

// PrepareActionDecisionAppend validates an append against the authoritative
// current audit and derives the optional lifecycle transition. It validates
// captured evidence structure and policy-floor compatibility, but deliberately
// does not perform cryptographic verification or current membership/RBAC.
func PrepareActionDecisionAppend(current, proposed ActionAuditRecord, event ActionLifecycleEvent) (ActionDecisionAppendCommand, error) {
	current, err := NormalizeActionAuditRecord(current)
	if err != nil {
		return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
	}
	proposed, err = NormalizeActionAuditRecord(proposed)
	if err != nil {
		return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
	}
	event, err = NormalizeActionLifecycleEvent(event)
	if err != nil {
		return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
	}
	if current.State != ActionStatePending || !ActionAuditIdentityMatches(current, proposed) ||
		current.DecisionRevision != uint64(len(current.Approvals)) ||
		proposed.DecisionRevision != current.DecisionRevision+1 ||
		len(proposed.Approvals) != len(current.Approvals)+1 ||
		proposed.DecisionRevision != uint64(len(proposed.Approvals)) || event.Decision == nil ||
		event.ActionID != proposed.ID || event.State != proposed.State || event.DecisionRevision != proposed.DecisionRevision {
		return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
	}
	for index := range current.Approvals {
		if !canonicalActionIdentityJSONEqual(current.Approvals[index], proposed.Approvals[index]) {
			return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
		}
	}
	if err := ValidateActionActor(proposed.Request.Actor); err != nil {
		return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
	}
	requirement := NormalizeApprovalRequirement(proposed.Plan.ApprovalRequirement, proposed.Plan.ApprovalPolicy)
	if err := ValidateApprovalRequirement(requirement, proposed.Plan.ApprovalPolicy); err != nil {
		return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
	}
	approvedActors := map[string]struct{}{}
	seenSubjects := map[string]struct{}{}
	for index := range proposed.Approvals {
		approval := proposed.Approvals[index]
		approval.ActorBinding = NormalizeActionActor(approval.ActorBinding)
		if err := ValidateActionActor(approval.ActorBinding); err != nil ||
			(approval.ActorBinding.Kind != ActionActorUser && approval.ActorBinding.Kind != ActionActorAPIToken) ||
			approval.Actor != approval.ActorBinding.SubjectID || approval.Evidence == nil || approval.Timestamp.IsZero() {
			return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
		}
		subjectKey := strings.ToLower(approval.ActorBinding.SubjectID)
		if _, duplicate := seenSubjects[subjectKey]; duplicate {
			return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
		}
		seenSubjects[subjectKey] = struct{}{}
		if requirement.DisallowRequester && strings.EqualFold(approval.ActorBinding.SubjectID, proposed.Request.Actor.SubjectID) {
			return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
		}
		evidence := *approval.Evidence
		evidence.Actor = NormalizeActionActor(evidence.Actor)
		if evidence.Version != 1 || evidence.IssuedAt.IsZero() ||
			!ActionActorsEqual(evidence.Actor, approval.ActorBinding) || evidence.OrgID != proposed.Request.Actor.OrgID ||
			evidence.ActionID != proposed.ID || evidence.PlanHash != proposed.Plan.PlanHash ||
			evidence.Outcome != approval.Outcome || evidence.Method != approval.Method ||
			evidence.IssuedAt.After(approval.Timestamp) ||
			(!evidence.ExpiresAt.IsZero() && (evidence.ExpiresAt.Before(evidence.IssuedAt) || approval.Timestamp.After(evidence.ExpiresAt))) {
			return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
		}
		switch approval.Method {
		case MethodSession:
			if approval.ActorBinding.Kind != ActionActorUser || strings.TrimSpace(evidence.ChallengeID) != "" {
				return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
			}
		case MethodAPIToken:
			if approval.ActorBinding.Kind != ActionActorAPIToken || strings.TrimSpace(evidence.ChallengeID) != "" {
				return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
			}
		case MethodWebAuthnUV, MethodDeviceKeyUV:
			if approval.ActorBinding.Kind != ActionActorUser || strings.TrimSpace(evidence.ChallengeID) == "" || evidence.ExpiresAt.IsZero() || !evidence.ExpiresAt.After(evidence.IssuedAt) {
				return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
			}
		default:
			return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
		}
		if approval.Outcome == OutcomeApproved {
			switch requirement.Floor {
			case ApprovalDryRun:
				return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
			case ApprovalMultiFactor:
				if approval.Method != MethodWebAuthnUV && approval.Method != MethodDeviceKeyUV {
					return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
				}
			case ApprovalAdmin, ApprovalNone:
				if approval.Method != MethodSession && approval.Method != MethodAPIToken {
					return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
				}
			default:
				return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
			}
		}
		if index < len(proposed.Approvals)-1 && approval.Outcome != OutcomeApproved {
			return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
		}
		if approval.Outcome == OutcomeApproved {
			approvedActors[subjectKey] = struct{}{}
		}
	}
	last := proposed.Approvals[len(proposed.Approvals)-1]
	if !canonicalActionIdentityJSONEqual(last, *event.Decision) || event.Actor != last.Actor || !event.Timestamp.Equal(last.Timestamp) {
		return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
	}
	derivedState := ActionStateRejected
	expectedMessage := "Action rejected before execution."
	if last.Outcome == OutcomeApproved {
		if len(approvedActors) < requirement.Quorum {
			derivedState = ActionStatePending
			expectedMessage = fmt.Sprintf("Approval recorded; %d of %d distinct approvals collected.", len(approvedActors), requirement.Quorum)
		} else {
			derivedState = ActionStateApproved
			expectedMessage = "Action approved. Execution remains pending a separate execution contract."
		}
	} else if last.Outcome != OutcomeRejected {
		return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
	}
	if proposed.State != derivedState || event.State != derivedState || event.Message != expectedMessage {
		return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
	}
	command := ActionDecisionAppendCommand{Record: proposed, DecisionEvent: event}
	if derivedState != ActionStatePending {
		transition := event
		transition.Kind = ActionLifecycleEventTransition
		transition.DecisionRevision = 0
		transition.Decision = nil
		transition, err = NormalizeActionLifecycleEvent(transition)
		if err != nil {
			return ActionDecisionAppendCommand{}, ErrActionDecisionRevisionConflict
		}
		command.TransitionEvent = &transition
	}
	return command, nil
}

// ExpireAction materializes plan expiry as an explicit monotonic lifecycle
// state. Expiry is admission truth, not an execution-result classification.
func ExpireAction(record ActionAuditRecord, actor string, now time.Time) (ActionAuditRecord, ActionLifecycleEvent, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if record.State == ActionStateExpired {
		return record, ActionLifecycleEvent{}, nil
	}
	switch record.State {
	case ActionStatePlanned, ActionStatePending, ActionStateApproved:
	default:
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ErrActionExecutionFinal
	}
	if record.Plan.ExpiresAt.IsZero() || now.Before(record.Plan.ExpiresAt) {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ErrActionPlanNotExpired
	}
	record.State = ActionStateExpired
	record.UpdatedAt = now
	normalized, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, err
	}
	event, err := NormalizeActionLifecycleEvent(ActionLifecycleEvent{
		ActionID:  normalized.ID,
		Timestamp: now,
		State:     ActionStateExpired,
		Actor:     strings.TrimSpace(actor),
		Message:   "Action plan expired before dispatch.",
	})
	return normalized, event, err
}

// BeginActionExecution moves an explicitly executable action into executing.
// Approval remains separate from execution: approval-required plans must be
// approved first, while approval-free plans may start from planned only through
// this explicit execution contract.
func BeginActionExecution(record ActionAuditRecord, actor string, now time.Time) (ActionAuditRecord, ActionLifecycleEvent, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if err := ValidateActionExecutionStart(record, now); err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, err
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "api:authenticated"
	}

	record.State = ActionStateExecuting
	record.UpdatedAt = now
	normalized, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, err
	}
	event := ActionLifecycleEvent{
		ActionID:  normalized.ID,
		Timestamp: now,
		State:     ActionStateExecuting,
		Actor:     actor,
		Message:   "Action execution started.",
	}
	normalizedEvent, err := NormalizeActionLifecycleEvent(event)
	if err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, err
	}
	return normalized, normalizedEvent, nil
}

// CompleteActionExecution records the terminal result for an action that has
// already entered executing state.
func CompleteActionExecution(record ActionAuditRecord, result *ExecutionResult, actor string, now time.Time) (ActionAuditRecord, ActionLifecycleEvent, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if record.State != ActionStateExecuting {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ErrActionNotExecuting
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "api:authenticated"
	}
	if result == nil {
		result = ExecutorContractViolationResult("executor_nil_result", "Executor returned no result.", record.Plan.RollbackAvailable)
	}
	result.Output = strings.TrimSpace(result.Output)
	result.ErrorMessage = strings.TrimSpace(result.ErrorMessage)
	canonical := CanonicalActionResultV2(ActionAuditRecord{ID: record.ID, UpdatedAt: now, Plan: record.Plan, Result: result, VerificationOutcome: VerificationOutcomeFromExecutionResult(result)})
	var err error
	result, record.VerificationOutcome, err = ApplyActionResultV2(result, canonical)
	if err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, err
	}

	nextState := ActionStateCompleted
	message := "Action execution completed."
	if !result.Success {
		nextState = ActionStateFailed
		message = result.ErrorMessage
		if message == "" {
			message = "Action execution failed."
		}
	}

	record.State = nextState
	record.UpdatedAt = now
	record.Result = result
	normalized, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, err
	}
	event := ActionLifecycleEvent{
		ActionID:  normalized.ID,
		Timestamp: now,
		State:     nextState,
		Actor:     actor,
		Message:   message,
	}
	normalizedEvent, err := NormalizeActionLifecycleEvent(event)
	if err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, err
	}
	return normalized, normalizedEvent, nil
}

// RefuseActionExecution records a permanent pre-dispatch refusal as the same
// terminal failed audit shape used by runtime execution failures.
func RefuseActionExecution(record ActionAuditRecord, reason error, actor string, now time.Time) (ActionAuditRecord, ActionLifecycleEvent, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	code, message, ok := permanentActionExecutionRefusalMessage(reason)
	if !ok {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, fmt.Errorf("%w: %v", ErrActionExecutionRefusal, reason)
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "api:authenticated"
	}

	record.State = ActionStateFailed
	record.UpdatedAt = now
	record.Result = KnownNoEffectResult(code, message, record.Plan.RollbackAvailable)
	normalized, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, err
	}
	event := ActionLifecycleEvent{
		ActionID:  normalized.ID,
		Timestamp: now,
		State:     ActionStateFailed,
		Actor:     actor,
		Message:   normalized.Result.ErrorMessage,
	}
	normalizedEvent, err := NormalizeActionLifecycleEvent(event)
	if err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, err
	}
	return normalized, normalizedEvent, nil
}

// IsPermanentActionExecutionRefusal reports whether err represents a
// non-dispatchable execution attempt that should terminally fail the audit.
func IsPermanentActionExecutionRefusal(err error) bool {
	_, _, ok := permanentActionExecutionRefusalMessage(err)
	return ok
}

// permanentActionExecutionRefusalMessage returns the stable machine reason
// code and the human refusal message for a permanent pre-dispatch refusal.
// The code is persisted as the canonical execution reason code, so telemetry
// and audit consumers can distinguish refusal causes without message parsing.
func permanentActionExecutionRefusalMessage(reason error) (string, string, bool) {
	switch {
	case errors.Is(reason, ErrActionPlanDrift):
		return "plan_drift", "plan_drift: action plan no longer matches the current resource contract; re-plan before executing", true
	case errors.Is(reason, ErrActionPlanExpired):
		return "action_plan_expired", "action_plan_expired: action plan has expired; re-plan before executing", true
	case errors.Is(reason, ErrActionDryRunOnly):
		return "action_dry_run_only", "action_dry_run_only: action plan is dry-run only and cannot be executed", true
	case errors.Is(reason, ErrResourceRemediationLocked):
		return "resource_remediation_locked", "resource_remediation_locked: resource is operator-locked against automated remediation", true
	case errors.Is(reason, ErrActionPolicyAuthorizationExpired):
		return "policy_authorization_expired", "policy_authorization_expired: automatic authority expired before dispatch", true
	case errors.Is(reason, ErrActionPolicyAuthorizationInvalid):
		return "policy_authorization_invalid", "policy_authorization_invalid: automatic authority is missing, unreadable, or malformed", true
	case errors.Is(reason, ErrActionPolicyAuthorizationRevoked):
		return "policy_authorization_revoked", "policy_authorization_revoked: automatic authority changed before dispatch", true
	case errors.Is(reason, ErrActionEmergencyStop):
		return "action_emergency_stop", "action_emergency_stop: action dispatch is stopped by the operator", true
	case errors.Is(reason, ErrActionReplanRequired):
		return "action_replan_required", "action_replan_required: legacy action authority is unbound; re-plan before deciding or executing", true
	default:
		return "", "", false
	}
}

// ValidateActionExecutionStart checks whether the current persisted state may
// enter execution without mutating the record.
func ValidateActionExecutionStart(record ActionAuditRecord, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	switch record.State {
	case ActionStateExecuting:
		return ErrActionAlreadyExecuting
	case ActionStateExpired, ActionStateCompleted, ActionStateFailed:
		return ErrActionExecutionFinal
	}
	if !record.Plan.ExpiresAt.IsZero() && !now.Before(record.Plan.ExpiresAt) {
		return ErrActionPlanExpired
	}
	if record.Plan.ApprovalPolicy == ApprovalDryRun {
		return ErrActionDryRunOnly
	}
	switch record.State {
	case ActionStateApproved:
		return nil
	case ActionStatePlanned:
		if record.Plan.Allowed && !record.Plan.RequiresApproval {
			return nil
		}
		return ErrActionNotApproved
	case ActionStatePending, ActionStateRejected, ActionStateExpired:
		return ErrActionNotApproved
	default:
		return fmt.Errorf("unsupported action state %q", record.State)
	}
}

// ValidateHumanActionBinding rejects legacy nonterminal records that predate
// the immutable actor/requirement contract. Terminal history remains readable,
// while pending or approved work must be re-planned under current authority.
func ValidateHumanActionBinding(record ActionAuditRecord, orgID string) error {
	record.Request.Actor = NormalizeActionActor(record.Request.Actor)
	requirement := NormalizeApprovalRequirement(record.Plan.ApprovalRequirement, record.Plan.ApprovalPolicy)
	if ValidateActionActor(record.Request.Actor) != nil || record.Request.Actor.OrgID != strings.TrimSpace(orgID) ||
		requirement.Version != ActionApprovalRequirementVersion || requirement.Floor != record.Plan.ApprovalPolicy {
		return ErrActionReplanRequired
	}
	if record.Plan.RequiresApproval && (record.State == ActionStateApproved || record.State == ActionStateExecuting) {
		actors := map[string]struct{}{}
		for _, approval := range record.Approvals {
			if approval.Outcome != OutcomeApproved || approval.Evidence == nil {
				continue
			}
			actor := NormalizeActionActor(approval.ActorBinding)
			evidence := *approval.Evidence
			evidence.Actor = NormalizeActionActor(evidence.Actor)
			if ValidateActionActor(actor) != nil || !ActionActorsEqual(actor, evidence.Actor) ||
				evidence.Version != 1 || evidence.OrgID != strings.TrimSpace(orgID) || evidence.ActionID != record.ID ||
				evidence.PlanHash != record.Plan.PlanHash || evidence.Outcome != OutcomeApproved || evidence.IssuedAt.IsZero() {
				continue
			}
			if requirement.Floor == ApprovalMultiFactor {
				if evidence.Method != MethodWebAuthnUV && evidence.Method != MethodDeviceKeyUV {
					continue
				}
			} else if evidence.Method != MethodSession && evidence.Method != MethodAPIToken {
				continue
			}
			actors[strings.ToLower(actor.SubjectID)] = struct{}{}
		}
		if len(actors) < requirement.Quorum {
			return ErrActionReplanRequired
		}
	}
	return nil
}

// NormalizeActionAuditRecord applies the canonical action-governance floor
// before a record is persisted. It keeps older callers usable by filling safe
// deterministic defaults, but rejects records that cannot identify the action,
// state, resource, capability, or requester.
func NormalizeActionAuditRecord(record ActionAuditRecord) (ActionAuditRecord, error) {
	record.Plan.PolicyDecision.Authorities = append([]ActionPolicyAuthorityFactor(nil), record.Plan.PolicyDecision.Authorities...)
	for i := range record.Plan.PolicyDecision.Authorities {
		record.Plan.PolicyDecision.Authorities[i].ReasonCodes = append([]ActionPolicyReasonCode(nil), record.Plan.PolicyDecision.Authorities[i].ReasonCodes...)
	}
	if record.Plan.Preflight != nil {
		preflight := *record.Plan.Preflight
		preflight.SafetyChecks = append([]string(nil), record.Plan.Preflight.SafetyChecks...)
		preflight.VerificationSteps = append([]string(nil), record.Plan.Preflight.VerificationSteps...)
		record.Plan.Preflight = &preflight
	}
	record.Approvals = append([]ActionApprovalRecord(nil), record.Approvals...)
	for i := range record.Approvals {
		record.Approvals[i].ActorBinding = NormalizeActionActor(record.Approvals[i].ActorBinding)
		if record.Approvals[i].Evidence != nil {
			evidence := *record.Approvals[i].Evidence
			evidence.Actor = NormalizeActionActor(evidence.Actor)
			record.Approvals[i].Evidence = &evidence
		}
		if record.Approvals[i].PolicyLease != nil {
			lease := *record.Approvals[i].PolicyLease
			lease.CapabilityNames = append([]string(nil), lease.CapabilityNames...)
			if lease.Window != nil {
				window := *lease.Window
				lease.Window = &window
			}
			record.Approvals[i].PolicyLease = &lease
		}
	}
	record.ID = strings.TrimSpace(record.ID)
	record.Plan.ActionID = strings.TrimSpace(record.Plan.ActionID)
	if record.ID == "" {
		record.ID = record.Plan.ActionID
	}
	if record.ID == "" {
		return ActionAuditRecord{}, fmt.Errorf("action audit id required")
	}
	if record.Plan.ActionID == "" {
		record.Plan.ActionID = record.ID
	}
	if record.Plan.ActionID != record.ID {
		return ActionAuditRecord{}, fmt.Errorf("action audit id %q does not match plan action id %q", record.ID, record.Plan.ActionID)
	}
	if !isValidActionState(record.State) {
		return ActionAuditRecord{}, fmt.Errorf("unsupported action state %q", record.State)
	}

	record.Request.RequestID = strings.TrimSpace(record.Request.RequestID)
	record.Plan.RequestID = strings.TrimSpace(record.Plan.RequestID)
	if record.Request.RequestID == "" {
		record.Request.RequestID = record.Plan.RequestID
	}
	if record.Request.RequestID == "" {
		record.Request.RequestID = record.ID
	}
	if record.Plan.RequestID == "" {
		record.Plan.RequestID = record.Request.RequestID
	}
	if record.Plan.RequestID != record.Request.RequestID {
		return ActionAuditRecord{}, fmt.Errorf("action request id %q does not match plan request id %q", record.Request.RequestID, record.Plan.RequestID)
	}

	record.Request.ResourceID = CanonicalResourceID(record.Request.ResourceID)
	record.Request.CapabilityName = strings.TrimSpace(record.Request.CapabilityName)
	record.Request.Reason = strings.TrimSpace(record.Request.Reason)
	record.Request.RequestedBy = strings.TrimSpace(record.Request.RequestedBy)
	record.Request.Actor = NormalizeActionActor(record.Request.Actor)
	if record.Request.Actor.SubjectID != "" {
		record.Request.RequestedBy = record.Request.Actor.SubjectID
	}
	record.Origin = NormalizeActionOrigin(record.Origin)
	if record.Request.ResourceID == "" {
		return ActionAuditRecord{}, fmt.Errorf("action request resource id required")
	}
	if record.Request.CapabilityName == "" {
		return ActionAuditRecord{}, fmt.Errorf("action request capability name required")
	}
	if record.Request.RequestedBy == "" {
		return ActionAuditRecord{}, fmt.Errorf("action request requestedBy required")
	}

	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	} else {
		record.CreatedAt = record.CreatedAt.UTC()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	} else {
		record.UpdatedAt = record.UpdatedAt.UTC()
	}

	if record.Plan.PlannedAt.IsZero() {
		record.Plan.PlannedAt = record.CreatedAt
	} else {
		record.Plan.PlannedAt = record.Plan.PlannedAt.UTC()
	}
	if record.Plan.ExpiresAt.IsZero() {
		record.Plan.ExpiresAt = record.Plan.PlannedAt.Add(5 * time.Minute)
	} else {
		record.Plan.ExpiresAt = record.Plan.ExpiresAt.UTC()
	}
	if record.Plan.ApprovalPolicy == "" {
		if record.Plan.RequiresApproval {
			record.Plan.ApprovalPolicy = ApprovalAdmin
		} else {
			record.Plan.ApprovalPolicy = ApprovalNone
		}
	}
	record.Plan.ApprovalRequirement = NormalizeApprovalRequirement(record.Plan.ApprovalRequirement, record.Plan.ApprovalPolicy)
	if record.Plan.ApprovalRequirement.Version != 0 && record.Plan.ApprovalRequirement.Version != ActionApprovalRequirementVersion {
		return ActionAuditRecord{}, fmt.Errorf("unsupported approval requirement version %d", record.Plan.ApprovalRequirement.Version)
	}
	if record.Plan.PolicyDecision.Version == 0 && record.Plan.PolicyDecision.Status == "" {
		record.Plan.PolicyDecision = LegacyUnknownActionPolicyDecision()
	}
	if err := ValidateActionPlanPolicyDecision(record.Plan, record.Request); err != nil {
		return ActionAuditRecord{}, fmt.Errorf("invalid action policy decision: %w", err)
	}
	record.Plan.Preflight = NormalizeActionPreflight(record.Plan.Preflight, record.Request, record.Plan)

	if record.Result != nil {
		result := *record.Result
		result.Output = strings.TrimSpace(result.Output)
		result.ErrorMessage = strings.TrimSpace(result.ErrorMessage)
		if result.Verification == nil {
			result.Verification = cloneActionVerificationResult(record.Verification)
		}
		canonical := CanonicalActionResultV2(ActionAuditRecord{ID: record.ID, UpdatedAt: record.UpdatedAt, Plan: record.Plan, Result: &result, VerificationOutcome: record.VerificationOutcome})
		result.Verification = NormalizeActionVerificationResult(result.Verification)
		result.ActionResultV2 = &canonical
		record.Result = &result
	}
	record.Verification = NormalizeActionVerificationResult(record.Verification)
	if record.Verification == nil && record.Result != nil {
		record.Verification = cloneActionVerificationResult(record.Result.Verification)
	}
	if record.Verification != nil && record.Result != nil {
		record.Result.Verification = cloneActionVerificationResult(record.Verification)
	}
	record.VerificationOutcome = NormalizeVerificationOutcome(record.VerificationOutcome)
	if record.Result != nil {
		result, legacy, err := ApplyActionResultV2(record.Result, CanonicalActionResultV2(record))
		if err != nil {
			return ActionAuditRecord{}, err
		}
		record.Result = result
		record.VerificationOutcome = legacy
	}

	for i := range record.Approvals {
		record.Approvals[i].Actor = strings.TrimSpace(record.Approvals[i].Actor)
		record.Approvals[i].Reason = strings.TrimSpace(record.Approvals[i].Reason)
		if record.Approvals[i].Method == "" {
			record.Approvals[i].Method = MethodAPI
		}
		if record.Approvals[i].Outcome == "" {
			record.Approvals[i].Outcome = OutcomeApproved
		}
		if record.Approvals[i].Timestamp.IsZero() {
			record.Approvals[i].Timestamp = record.UpdatedAt
		} else {
			record.Approvals[i].Timestamp = record.Approvals[i].Timestamp.UTC()
		}
	}

	return record, nil
}

// NormalizeActionLifecycleEvent applies the same action-governance identity and
// state checks to append-only lifecycle events.
func NormalizeActionLifecycleEvent(event ActionLifecycleEvent) (ActionLifecycleEvent, error) {
	event.ActionID = strings.TrimSpace(event.ActionID)
	if event.ActionID == "" {
		return ActionLifecycleEvent{}, fmt.Errorf("action lifecycle event action id required")
	}
	if !isValidActionState(event.State) {
		return ActionLifecycleEvent{}, fmt.Errorf("unsupported action lifecycle state %q", event.State)
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	} else {
		event.Timestamp = event.Timestamp.UTC()
	}
	event.Actor = strings.TrimSpace(event.Actor)
	event.Message = strings.TrimSpace(event.Message)
	if event.Kind == "" {
		if event.DecisionRevision > 0 || event.Decision != nil {
			event.Kind = ActionLifecycleEventDecision
		} else {
			event.Kind = ActionLifecycleEventTransition
		}
	}
	switch event.Kind {
	case ActionLifecycleEventTransition:
		if event.DecisionRevision != 0 || event.Decision != nil {
			return ActionLifecycleEvent{}, fmt.Errorf("state transition event cannot carry decision identity")
		}
	case ActionLifecycleEventDecision:
		if event.DecisionRevision == 0 || event.Decision == nil {
			return ActionLifecycleEvent{}, fmt.Errorf("decision event revision and approval are required")
		}
		decision := *event.Decision
		decision.ActorBinding = NormalizeActionActor(decision.ActorBinding)
		decision.Actor = strings.TrimSpace(decision.Actor)
		decision.Reason = strings.TrimSpace(decision.Reason)
		if decision.Actor == "" || decision.Actor != decision.ActorBinding.SubjectID || decision.Evidence == nil ||
			decision.Outcome == "" || decision.Method == "" || decision.Timestamp.IsZero() {
			return ActionLifecycleEvent{}, fmt.Errorf("decision event approval binding is incomplete")
		}
		decision.Timestamp = decision.Timestamp.UTC()
		evidence := *decision.Evidence
		evidence.Actor = NormalizeActionActor(evidence.Actor)
		if !ActionActorsEqual(evidence.Actor, decision.ActorBinding) || evidence.ActionID != event.ActionID ||
			evidence.Outcome != decision.Outcome || evidence.Method != decision.Method {
			return ActionLifecycleEvent{}, fmt.Errorf("decision event evidence binding does not match approval")
		}
		decision.Evidence = &evidence
		event.Decision = &decision
		event.Actor = decision.Actor
		event.Timestamp = decision.Timestamp
	default:
		return ActionLifecycleEvent{}, fmt.Errorf("unsupported action lifecycle event kind %q", event.Kind)
	}
	return event, nil
}

// NormalizeActionPreflight ensures persisted action plans always state whether
// a dry-run was available and what post-execution verification should inspect.
func NormalizeActionPreflight(preflight *ActionPreflight, request ActionRequest, plan ActionPlan) *ActionPreflight {
	if preflight == nil {
		preflight = &ActionPreflight{}
	}
	preflight.Target = strings.TrimSpace(preflight.Target)
	if preflight.Target == "" {
		preflight.Target = request.ResourceID
	}
	preflight.CurrentState = strings.TrimSpace(preflight.CurrentState)
	preflight.IntendedChange = strings.TrimSpace(preflight.IntendedChange)
	if preflight.IntendedChange == "" {
		preflight.IntendedChange = strings.TrimSpace(plan.Message)
	}
	if preflight.IntendedChange == "" {
		preflight.IntendedChange = strings.TrimSpace(request.Reason)
	}
	preflight.DryRunSummary = strings.TrimSpace(preflight.DryRunSummary)
	if preflight.DryRunSummary == "" {
		if preflight.DryRunAvailable {
			preflight.DryRunSummary = "Provider-supported dry run is available for this action."
		} else {
			preflight.DryRunSummary = "No provider-supported dry run is available for this action."
		}
	}
	if len(preflight.SafetyChecks) == 0 {
		preflight.SafetyChecks = []string{"Action is recorded in the unified action audit."}
	}
	if len(preflight.VerificationSteps) == 0 {
		preflight.VerificationSteps = []string{"Review the action result and lifecycle events after execution."}
	}
	if preflight.GeneratedAt.IsZero() {
		preflight.GeneratedAt = plan.PlannedAt
	} else {
		preflight.GeneratedAt = preflight.GeneratedAt.UTC()
	}
	return preflight
}

func isValidActionState(state ActionState) bool {
	switch state {
	case ActionStatePlanned, ActionStatePending, ActionStateApproved, ActionStateRejected, ActionStateExpired, ActionStateExecuting, ActionStateCompleted, ActionStateFailed:
		return true
	default:
		return false
	}
}
