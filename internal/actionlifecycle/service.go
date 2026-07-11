// Package actionlifecycle is the transport-independent action lifecycle
// service: planning, approval decisions, and execution for typed resource
// actions. The REST handlers in internal/api and any in-process broker
// (e.g. Patrol investigation proposals) must route through this one
// service so every caller gets identical resource lookup, availability
// checks, plan hashing, audit persistence, remediation locks, plan-drift
// detection, execution, and terminal verification. No caller may dispatch
// a resource mutation around it.
package actionlifecycle

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionplanner"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Executor runs a previously planned and approved action through the
// canonical execution contract.
type Executor interface {
	ExecuteAction(ctx context.Context, record unified.ActionAuditRecord) (*unified.ExecutionResult, error)
}

// DispatchReconciler queries a transport by durable attempt identity. It must
// never send or re-send the mutation. found is true only for an authenticated,
// correlated transport response.
type DispatchReconciler interface {
	ReconcileActionDispatch(ctx context.Context, record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt) (result *unified.ExecutionResult, receipt unified.ActionDispatchReceipt, found bool, err error)
}

// AvailabilityChecker lets an executor contribute live readiness checks
// before Pulse advertises or persists an executable action plan.
type AvailabilityChecker interface {
	CheckActionAvailable(ctx context.Context, req unified.ActionRequest, resource unified.Resource) unified.ResourceActionReadiness
}

// Store is the narrow persistence surface the lifecycle needs. It is a
// structural subset of unified.ResourceStore so the canonical store
// satisfies it without adaptation.
type Store interface {
	CreateActionAudit(record unified.ActionAuditRecord, initialEvents []unified.ActionLifecycleEvent) (unified.ActionAuditRecord, bool, error)
	GetActionAudit(actionID string) (unified.ActionAuditRecord, bool, error)
	RecordActionDecision(record unified.ActionAuditRecord, event unified.ActionLifecycleEvent) error
	RecordActionExpiry(record unified.ActionAuditRecord, event unified.ActionLifecycleEvent) error
	RecordActionExecutionStart(record unified.ActionAuditRecord, event unified.ActionLifecycleEvent) error
	RecordActionPolicyExecutionStart(record unified.ActionAuditRecord, approvalEvent, executionEvent unified.ActionLifecycleEvent) error
	RecordActionExecutionAdmission(record unified.ActionAuditRecord, event unified.ActionLifecycleEvent, attempt unified.ActionDispatchAttempt) error
	RecordActionPolicyExecutionAdmission(record unified.ActionAuditRecord, approvalEvent, executionEvent unified.ActionLifecycleEvent, attempt unified.ActionDispatchAttempt) error
	RecordActionExecutionResult(record unified.ActionAuditRecord, event unified.ActionLifecycleEvent) error
	RecordActionExecutionRefusal(record unified.ActionAuditRecord, event unified.ActionLifecycleEvent) error
	RecordActionLifecycleEvent(event unified.ActionLifecycleEvent) error
	GetActionLifecycleEvents(actionID string, since time.Time, limit int) ([]unified.ActionLifecycleEvent, error)
	GetActionDispatchAttempt(actionID string) (unified.ActionDispatchAttempt, bool, error)
	GetActionDispatchReceipt(attemptID string) (unified.ActionDispatchReceipt, bool, error)
	ClaimActionDispatch(actionID, owner string, now time.Time, lease time.Duration) (unified.ActionDispatchAttempt, bool, error)
	MarkActionDispatchStarted(attemptID, owner string, now time.Time) (unified.ActionDispatchAttempt, error)
	RecordActionDispatchReceipt(receipt unified.ActionDispatchReceipt) (unified.ActionDispatchAttempt, error)
	RecordActionDispatchCompletion(receipt unified.ActionDispatchReceipt, record unified.ActionAuditRecord, event unified.ActionLifecycleEvent) error
	RecoverActionDispatch(actionID string, now time.Time) (unified.ActionDispatchAttempt, bool, error)
	ExpireActionAudits(now time.Time, limit int) ([]unified.ActionAuditRecord, error)
	GetActionAuditsByStates(states []unified.ActionState, limit int) ([]unified.ActionAuditRecord, error)
	GetPendingActionAudits(limit int) ([]unified.ActionAuditRecord, error)
	GetResourceOperatorState(canonicalID string) (unified.ResourceOperatorState, bool, error)
}

// Service wires the lifecycle over per-org registry and store lookups. All
// dependencies are resolved per call so late-bound wiring (executors and
// publishers set after construction) stays current.
type Service struct {
	Registry            func(orgID string) (*unified.ResourceRegistry, error)
	Store               func(orgID string) (Store, error)
	Executor            Executor
	EmergencyStop       func(orgID string) (bool, error)
	DecisionAuthorizer  DecisionAuthorizer
	ExecutionAuthorizer ExecutionAuthorizer
	StepUpVerifier      StepUpVerifier
	// OnActionCompleted receives every terminal (completed/failed) audit
	// record, including refused-before-dispatch failures, so SSE bridges
	// and reconcilers observe the full lifecycle regardless of transport.
	OnActionCompleted func(unified.ActionAuditRecord)
	// OnActionTransition receives the org ID and audit record after every
	// successfully persisted state transition: plan creation, approval
	// decisions, and terminal execution outcomes (including refusals).
	// Persistence always happens before publication, so a subscriber that
	// reconciles origin surfaces (e.g. Patrol finding outcomes) never
	// observes a state the store could still lose. The org ID keys the
	// subscriber's per-tenant stores; without it a multi-tenant
	// reconciler could apply a transition to the wrong tenant. Terminal
	// records additionally flow through OnActionCompleted after this hook.
	OnActionTransition func(orgID string, record unified.ActionAuditRecord)
	Now                func() time.Time
	PolicyAdmission    *PolicyAdmissionCoordinator
}

type ActionDetail struct {
	Audit   unified.ActionAuditRecord      `json:"audit"`
	Events  []unified.ActionLifecycleEvent `json:"events"`
	Attempt *unified.ActionDispatchAttempt `json:"attempt,omitempty"`
	Receipt *unified.ActionDispatchReceipt `json:"receipt,omitempty"`
}

type ActionListView string

const (
	ActionListPending ActionListView = "pending"
	ActionListSettled ActionListView = "settled"
)

type PolicyAdmissionCoordinator struct{ mu sync.RWMutex }

var dispatchWorkerSequence atomic.Uint64

func (s *Service) admissionCoordinator() *PolicyAdmissionCoordinator {
	if s.PolicyAdmission == nil {
		s.PolicyAdmission = &PolicyAdmissionCoordinator{}
	}
	return s.PolicyAdmission
}

// PolicyAuthorizer resolves complete, current automatic authority while the
// lifecycle holds the admission read lock.
type PolicyAuthorizer func(ctx context.Context, record unified.ActionAuditRecord, now time.Time) (unified.ActionPolicyAuthorizationLease, string, error)

// WithPolicyMutation serializes a policy write against automatic admission.
// Writers must persist their change before returning.
func (s *Service) WithPolicyMutation(write func() error) error {
	if s == nil || write == nil {
		return errors.New("policy mutation unavailable")
	}
	coordinator := s.admissionCoordinator()
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	return write()
}

// PlanOptions carries broker-owned planning metadata that must never be
// accepted from a public transport request body.
type PlanOptions struct {
	// Actor is trusted server context. Public transports derive it from the
	// authenticated request and internal brokers stamp their fixed identity.
	Actor unified.ActionActor
	// ApprovalRequirement is an optional trusted, policy-resolved
	// strengthening of the capability floor. Public transports cannot set it.
	ApprovalRequirement *unified.ApprovalRequirement
	// Origin identifies the internal proposing surface and its
	// correlation IDs. Nil for operator/API-initiated plans.
	Origin *unified.ActionOrigin
}

type DecisionAuthorizer interface {
	AuthorizeDecision(ctx context.Context, orgID string, record unified.ActionAuditRecord, decision unified.ActionDecision) error
}

type DecisionAuthorizerFunc func(context.Context, string, unified.ActionAuditRecord, unified.ActionDecision) error

func (f DecisionAuthorizerFunc) AuthorizeDecision(ctx context.Context, orgID string, record unified.ActionAuditRecord, decision unified.ActionDecision) error {
	return f(ctx, orgID, record, decision)
}

type ExecutionAuthorizer interface {
	AuthorizeExecution(ctx context.Context, orgID string, record unified.ActionAuditRecord, actor unified.ActionActor) error
}

type ExecutionAuthorizerFunc func(context.Context, string, unified.ActionAuditRecord, unified.ActionActor) error

func (f ExecutionAuthorizerFunc) AuthorizeExecution(ctx context.Context, orgID string, record unified.ActionAuditRecord, actor unified.ActionActor) error {
	return f(ctx, orgID, record, actor)
}

// StepUpVerifier verifies and atomically consumes a cryptographic challenge.
// No default verifier exists: MFA approvals fail closed until a durable
// server-owned verifier is installed.
type StepUpVerifier interface {
	VerifyAndConsume(ctx context.Context, record unified.ActionAuditRecord, decision unified.ActionDecision) error
}

type StepUpVerifierFunc func(context.Context, unified.ActionAuditRecord, unified.ActionDecision) error

func (f StepUpVerifierFunc) VerifyAndConsume(ctx context.Context, record unified.ActionAuditRecord, decision unified.ActionDecision) error {
	return f(ctx, record, decision)
}

// Sentinel errors for dependency failures. Callers map these to their
// transport's unavailability semantics.
var (
	ErrRegistryUnavailable               = errors.New("resource registry unavailable")
	ErrStoreUnavailable                  = errors.New("action audit store unavailable")
	ErrExecutorUnavailable               = errors.New("no action executor is configured")
	ErrDecisionAuthorizationUnavailable  = errors.New("action decision authorization unavailable")
	ErrExecutionAuthorizationUnavailable = errors.New("action execution authorization unavailable")
	ErrActionAuthorizationDenied         = errors.New("action authorization denied")
	ErrApprovalEvidenceInvalid           = errors.New("approval evidence is not bound to this actor, organization, action, plan, and outcome")
	ErrApprovalStepUpUnavailable         = errors.New("cryptographic step-up approval is unavailable")
	ErrApprovalActorNotHuman             = errors.New("detached or service actors cannot satisfy human approval")
	ErrApprovalSeparationRequired        = errors.New("requester cannot approve this action")
	ErrDecisionReplayConflict            = errors.New("action decision replay conflicts with the persisted decision")
)

// ResourceNotFoundError reports that the requested resource is not present
// in the org's registry.
type ResourceNotFoundError struct{ ResourceID string }

func (e *ResourceNotFoundError) Error() string {
	return fmt.Sprintf("resource %q not found", e.ResourceID)
}

// ActionNotFoundError reports that no audit record exists for the action ID.
type ActionNotFoundError struct{ ActionID string }

func (e *ActionNotFoundError) Error() string {
	return fmt.Sprintf("action %q not found", e.ActionID)
}

// CapabilityNotFoundError reports that the resource does not advertise the
// requested capability. It unwraps to actionplanner.ErrCapabilityNotFound.
type CapabilityNotFoundError struct {
	ResourceID     string
	CapabilityName string
}

func (e *CapabilityNotFoundError) Error() string {
	return fmt.Sprintf("capability %q not found on resource %q", e.CapabilityName, e.ResourceID)
}

func (e *CapabilityNotFoundError) Unwrap() error { return actionplanner.ErrCapabilityNotFound }

// AvailabilityRefusedError reports that the executor's live readiness check
// refused the action before a plan was persisted.
type AvailabilityRefusedError struct {
	ResourceID     string
	CapabilityName string
	Readiness      unified.ResourceActionReadiness
}

func (e *AvailabilityRefusedError) Error() string {
	reason := strings.TrimSpace(e.Readiness.Reason)
	if reason == "" {
		reason = "action execution is unavailable"
	}
	return fmt.Sprintf("action %s on %s unavailable: %s", e.CapabilityName, e.ResourceID, reason)
}

// PersistError wraps a storage write failure at a named lifecycle stage.
type PersistError struct {
	Op  string
	Err error
}

func (e *PersistError) Error() string { return fmt.Sprintf("persist %s: %v", e.Op, e.Err) }
func (e *PersistError) Unwrap() error { return e.Err }

// QueryError wraps a storage read failure.
type QueryError struct {
	Op  string
	Err error
}

func (e *QueryError) Error() string { return fmt.Sprintf("query %s: %v", e.Op, e.Err) }
func (e *QueryError) Unwrap() error { return e.Err }

// FreshnessCheckError wraps an infrastructure failure while revalidating
// plan freshness. Plan drift itself is reported as unified.ErrActionPlanDrift.
type FreshnessCheckError struct{ Err error }

func (e *FreshnessCheckError) Error() string { return fmt.Sprintf("plan freshness check: %v", e.Err) }
func (e *FreshnessCheckError) Unwrap() error { return e.Err }

// PolicyCheckError wraps an infrastructure failure while evaluating
// execution policy. A remediation lock itself is reported as
// unified.ErrResourceRemediationLocked.
type PolicyCheckError struct{ Err error }

func (e *PolicyCheckError) Error() string { return fmt.Sprintf("execution policy check: %v", e.Err) }
func (e *PolicyCheckError) Unwrap() error { return e.Err }

func (s *Service) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) registry(orgID string) (*unified.ResourceRegistry, error) {
	if s == nil || s.Registry == nil {
		return nil, ErrRegistryUnavailable
	}
	registry, err := s.Registry(orgID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRegistryUnavailable, err)
	}
	if registry == nil {
		return nil, ErrRegistryUnavailable
	}
	return registry, nil
}

func (s *Service) store(orgID string) (Store, error) {
	if s == nil || s.Store == nil {
		return nil, ErrStoreUnavailable
	}
	store, err := s.Store(orgID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStoreUnavailable, err)
	}
	if store == nil {
		return nil, ErrStoreUnavailable
	}
	return store, nil
}

// NormalizeRequest trims and canonicalizes an action request before audit
// persistence so persisted requests hash and replan deterministically.
func NormalizeRequest(req unified.ActionRequest) unified.ActionRequest {
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.ResourceID = unified.CanonicalResourceID(req.ResourceID)
	req.CapabilityName = strings.TrimSpace(req.CapabilityName)
	req.Reason = strings.TrimSpace(req.Reason)
	req.RequestedBy = strings.TrimSpace(req.RequestedBy)
	if req.Params == nil {
		req.Params = map[string]any{}
	}
	return req
}

// Plan validates the request against the org's live resource registry,
// produces a typed plan through the canonical planner, runs the executor's
// availability check, and persists the plan-stage audit trail. Approval
// requirements come from the capability's declared policy, never from the
// caller.
func (s *Service) Plan(ctx context.Context, orgID string, req unified.ActionRequest, actor unified.ActionActor) (unified.ActionPlan, error) {
	return s.PlanWithOptions(ctx, orgID, req, PlanOptions{Actor: actor})
}

// PlanWithOptions is Plan plus broker-owned metadata. In-process proposing
// surfaces use it to stamp the action's origin; the HTTP adapter always
// calls plain Plan so a public request can never claim a first-party origin.
func (s *Service) PlanWithOptions(ctx context.Context, orgID string, req unified.ActionRequest, opts PlanOptions) (unified.ActionPlan, error) {
	orgID = strings.TrimSpace(orgID)
	opts.Actor = unified.NormalizeActionActor(opts.Actor)
	if err := unified.ValidateActionActor(opts.Actor); err != nil {
		return unified.ActionPlan{}, &actionplanner.ValidationError{Field: "actor", Message: err.Error()}
	}
	if opts.Actor.OrgID != orgID {
		return unified.ActionPlan{}, &actionplanner.ValidationError{Field: "actor.orgId", Message: "actor organization does not match request organization"}
	}
	req.Actor = opts.Actor
	req.RequestedBy = opts.Actor.SubjectID
	req.ResourceID = unified.CanonicalResourceID(req.ResourceID)
	if req.ResourceID == "" {
		return unified.ActionPlan{}, &actionplanner.ValidationError{Field: "resourceId", Message: "resource id is required"}
	}

	registry, err := s.registry(orgID)
	if err != nil {
		return unified.ActionPlan{}, err
	}
	resource, ok := registry.Get(req.ResourceID)
	if !ok || resource == nil {
		return unified.ActionPlan{}, &ResourceNotFoundError{ResourceID: req.ResourceID}
	}

	planner := actionplanner.Planner{}
	var plan unified.ActionPlan
	if opts.ApprovalRequirement != nil {
		plan, err = planner.PlanWithRequirement(req, *resource, *opts.ApprovalRequirement)
	} else {
		plan, err = planner.Plan(req, *resource)
	}
	if err != nil {
		if errors.Is(err, actionplanner.ErrCapabilityNotFound) {
			return unified.ActionPlan{}, &CapabilityNotFoundError{
				ResourceID:     req.ResourceID,
				CapabilityName: strings.TrimSpace(req.CapabilityName),
			}
		}
		return unified.ActionPlan{}, err
	}

	req = NormalizeRequest(req)
	if checker, ok := s.Executor.(AvailabilityChecker); ok {
		if readiness := checker.CheckActionAvailable(ctx, req, *resource); readiness.Name != "" && !readiness.Available {
			return unified.ActionPlan{}, &AvailabilityRefusedError{
				ResourceID:     req.ResourceID,
				CapabilityName: req.CapabilityName,
				Readiness:      readiness,
			}
		}
	}

	store, err := s.store(orgID)
	if err != nil {
		return unified.ActionPlan{}, err
	}
	record, created, err := persistPlanAudit(store, req, plan, opts.Origin)
	if err != nil {
		return unified.ActionPlan{}, &PersistError{Op: "action plan audit", Err: err}
	}
	if !created {
		return record.Plan, nil
	}
	s.publishTransition(orgID, record)
	return plan, nil
}

// Get returns the authoritative audit record for an action.
func (s *Service) Get(orgID, actionID string) (unified.ActionAuditRecord, bool, error) {
	store, err := s.store(orgID)
	if err != nil {
		return unified.ActionAuditRecord{}, false, err
	}
	record, found, err := store.GetActionAudit(strings.TrimSpace(actionID))
	if err != nil {
		return unified.ActionAuditRecord{}, false, &QueryError{Op: "action audit", Err: err}
	}
	if !found {
		return record, false, nil
	}
	record, err = s.materializeExpiry(store, orgID, record)
	if err != nil {
		return unified.ActionAuditRecord{}, false, err
	}
	return record, true, nil
}

// Detail returns the authoritative lifecycle record together with its durable
// transport correlation. It never infers result truth from dispatch state.
func (s *Service) Detail(orgID, actionID string) (ActionDetail, bool, error) {
	record, found, err := s.Get(orgID, actionID)
	if err != nil || !found {
		return ActionDetail{}, found, err
	}
	store, err := s.store(orgID)
	if err != nil {
		return ActionDetail{}, false, err
	}
	events, err := store.GetActionLifecycleEvents(record.ID, time.Time{}, 500)
	if err != nil {
		return ActionDetail{}, false, &QueryError{Op: "action lifecycle events", Err: err}
	}
	detail := ActionDetail{Audit: record, Events: events}
	if attempt, ok, queryErr := store.GetActionDispatchAttempt(record.ID); queryErr != nil {
		return ActionDetail{}, false, &QueryError{Op: "action dispatch attempt", Err: queryErr}
	} else if ok {
		detail.Attempt = &attempt
		if receipt, receiptOK, receiptErr := store.GetActionDispatchReceipt(attempt.ID); receiptErr != nil {
			return ActionDetail{}, false, &QueryError{Op: "action dispatch receipt", Err: receiptErr}
		} else if receiptOK {
			detail.Receipt = &receipt
		}
	}
	return detail, true, nil
}

// List returns the canonical pending or settled inbox projection.
func (s *Service) List(orgID string, view ActionListView, limit int) ([]unified.ActionAuditRecord, error) {
	store, err := s.store(orgID)
	if err != nil {
		return nil, err
	}
	if _, err := store.ExpireActionAudits(s.now(), 500); err != nil {
		return nil, &PersistError{Op: "action expiry sweep", Err: err}
	}
	var states []unified.ActionState
	switch view {
	case ActionListPending:
		states = []unified.ActionState{unified.ActionStatePlanned, unified.ActionStatePending, unified.ActionStateApproved, unified.ActionStateExecuting}
	case ActionListSettled:
		states = []unified.ActionState{unified.ActionStateRejected, unified.ActionStateExpired, unified.ActionStateCompleted, unified.ActionStateFailed}
	default:
		return nil, fmt.Errorf("unsupported action list view %q", view)
	}
	records, err := store.GetActionAuditsByStates(states, limit)
	if err != nil {
		return nil, &QueryError{Op: "action inbox", Err: err}
	}
	return records, nil
}

// DecisionQueue preserves the legacy oldest-first pending-approval projection
// while sharing the canonical expiry sweep.
func (s *Service) DecisionQueue(orgID string, limit int) ([]unified.ActionAuditRecord, error) {
	store, err := s.store(orgID)
	if err != nil {
		return nil, err
	}
	if _, err := store.ExpireActionAudits(s.now(), 500); err != nil {
		return nil, &PersistError{Op: "action expiry sweep", Err: err}
	}
	records, err := store.GetPendingActionAudits(limit)
	if err != nil {
		return nil, &QueryError{Op: "pending action queue", Err: err}
	}
	return records, nil
}

// ResourceOperatorState returns the authoritative per-resource policy used by
// execution and Patrol policy authorization.
func (s *Service) ResourceOperatorState(orgID, resourceID string) (unified.ResourceOperatorState, bool, error) {
	store, err := s.store(orgID)
	if err != nil {
		return unified.ResourceOperatorState{}, false, err
	}
	state, found, err := store.GetResourceOperatorState(unified.CanonicalResourceID(resourceID))
	if err != nil {
		return unified.ResourceOperatorState{}, false, &QueryError{Op: "resource operator state", Err: err}
	}
	return state, found, nil
}

// Capabilities returns the resource's advertised capability declarations
// through the same registry resolution and typed errors as planning, so a
// proposing surface can inspect capability names and parameter schemas
// without guessing planner input or duplicating registry lookup.
func (s *Service) Capabilities(ctx context.Context, orgID, resourceID string) ([]unified.ResourceCapability, error) {
	_ = ctx
	resourceID = unified.CanonicalResourceID(resourceID)
	if resourceID == "" {
		return nil, &actionplanner.ValidationError{Field: "resourceId", Message: "resource id is required"}
	}
	registry, err := s.registry(orgID)
	if err != nil {
		return nil, err
	}
	resource, ok := registry.Get(resourceID)
	if !ok || resource == nil {
		return nil, &ResourceNotFoundError{ResourceID: resourceID}
	}
	capabilities := make([]unified.ResourceCapability, len(resource.Capabilities))
	copy(capabilities, resource.Capabilities)
	return capabilities, nil
}

// PersistPlanAudit records the planned action's audit record and its
// initial lifecycle events, deduplicating states that were already
// recorded for the same action ID (idempotent replans).
func PersistPlanAudit(store Store, req unified.ActionRequest, plan unified.ActionPlan) error {
	_, _, err := persistPlanAudit(store, req, plan, nil)
	return err
}

func persistPlanAudit(store Store, req unified.ActionRequest, plan unified.ActionPlan, origin *unified.ActionOrigin) (unified.ActionAuditRecord, bool, error) {
	state := PlannedActionState(plan)
	record := unified.ActionAuditRecord{
		ID:        plan.ActionID,
		CreatedAt: plan.PlannedAt,
		UpdatedAt: plan.PlannedAt,
		State:     state,
		Request:   req,
		Plan:      plan,
		Origin:    unified.NormalizeActionOrigin(origin),
	}
	events := []unified.ActionLifecycleEvent{
		{
			ActionID:  plan.ActionID,
			Timestamp: plan.PlannedAt,
			State:     unified.ActionStatePlanned,
			Actor:     req.RequestedBy,
			Message:   "Action plan created.",
		},
	}
	if state != unified.ActionStatePlanned {
		events = append(events, unified.ActionLifecycleEvent{
			ActionID:  plan.ActionID,
			Timestamp: plan.PlannedAt,
			State:     state,
			Actor:     req.RequestedBy,
			Message:   "Action is waiting for approval before execution.",
		})
	}
	return store.CreateActionAudit(record, events)
}

// PlannedActionState is the initial audit state for a fresh plan: pending
// when the capability policy requires approval, planned otherwise.
func PlannedActionState(plan unified.ActionPlan) unified.ActionState {
	if plan.RequiresApproval {
		return unified.ActionStatePending
	}
	return unified.ActionStatePlanned
}

// Decide is the single human decision boundary. Trusted adapters provide a
// server-derived actor and evidence binding; authorization and approval-floor
// enforcement happen here before the append-only decision is persisted.
func (s *Service) Decide(ctx context.Context, orgID, actionID string, decision unified.ActionDecision) (unified.ActionAuditRecord, error) {
	actionID = strings.TrimSpace(actionID)
	if actionID == "" {
		return unified.ActionAuditRecord{}, &ActionNotFoundError{ActionID: actionID}
	}
	store, err := s.store(orgID)
	if err != nil {
		return unified.ActionAuditRecord{}, err
	}
	record, ok, err := store.GetActionAudit(actionID)
	if err != nil {
		return unified.ActionAuditRecord{}, &QueryError{Op: "action audit", Err: err}
	}
	if !ok {
		return unified.ActionAuditRecord{}, &ActionNotFoundError{ActionID: actionID}
	}
	record, err = s.materializeExpiry(store, orgID, record)
	if err != nil {
		return unified.ActionAuditRecord{}, err
	}
	if record.State == unified.ActionStateExpired {
		return record, unified.ErrActionPlanExpired
	}
	decision.Actor = unified.NormalizeActionActor(decision.Actor)
	decision.Reason = strings.TrimSpace(decision.Reason)
	decision.Evidence.Actor = unified.NormalizeActionActor(decision.Evidence.Actor)
	if exact, conflict := decisionReplay(record, decision); exact {
		return record, nil
	} else if conflict {
		return unified.ActionAuditRecord{}, ErrDecisionReplayConflict
	}
	if err := unified.ValidateHumanActionBinding(record, orgID); err != nil {
		return unified.ActionAuditRecord{}, err
	}
	if err := validateDecisionBinding(record, orgID, decision); err != nil {
		return unified.ActionAuditRecord{}, err
	}
	if s.DecisionAuthorizer == nil {
		return unified.ActionAuditRecord{}, ErrDecisionAuthorizationUnavailable
	}
	if err := s.DecisionAuthorizer.AuthorizeDecision(ctx, orgID, record, decision); err != nil {
		return unified.ActionAuditRecord{}, err
	}
	if err := s.validateApprovalFloor(ctx, record, decision, true); err != nil {
		return unified.ActionAuditRecord{}, err
	}
	approval := unified.ActionApprovalRecord{
		Actor:        decision.Actor.SubjectID,
		ActorBinding: decision.Actor,
		Method:       decision.Evidence.Method,
		Outcome:      decision.Outcome,
		Reason:       decision.Reason,
		Evidence:     &decision.Evidence,
	}
	if record.State != unified.ActionStatePending {
		return unified.ActionAuditRecord{}, unified.ErrActionNotPending
	}

	now := s.now()
	if approval.Timestamp.IsZero() {
		approval.Timestamp = now
	}
	for attempt := 0; attempt < 16; attempt++ {
		updated, event, err := unified.ApplyActionDecision(record, approval, now)
		if err != nil {
			return unified.ActionAuditRecord{}, err
		}
		if err := store.RecordActionDecision(updated, event); err != nil {
			if errors.Is(err, unified.ErrActionDecisionRevisionConflict) {
				current, found, queryErr := store.GetActionAudit(actionID)
				if queryErr != nil {
					return unified.ActionAuditRecord{}, &QueryError{Op: "action decision retry", Err: queryErr}
				}
				if !found {
					return unified.ActionAuditRecord{}, &ActionNotFoundError{ActionID: actionID}
				}
				if exact, conflict := decisionReplay(current, decision); exact {
					return current, nil
				} else if conflict {
					return unified.ActionAuditRecord{}, ErrDecisionReplayConflict
				}
				if current.State != unified.ActionStatePending {
					return unified.ActionAuditRecord{}, unified.ErrActionNotPending
				}
				if err := unified.ValidateHumanActionBinding(current, orgID); err != nil {
					return unified.ActionAuditRecord{}, err
				}
				if err := validateDecisionBinding(current, orgID, decision); err != nil {
					return unified.ActionAuditRecord{}, err
				}
				if err := s.DecisionAuthorizer.AuthorizeDecision(ctx, orgID, current, decision); err != nil {
					return unified.ActionAuditRecord{}, err
				}
				if err := s.validateApprovalFloor(ctx, current, decision, false); err != nil {
					return unified.ActionAuditRecord{}, err
				}
				record = current
				continue
			}
			if errors.Is(err, unified.ErrActionNotPending) {
				current, found, queryErr := store.GetActionAudit(actionID)
				if queryErr == nil && found {
					if exact, conflict := decisionReplay(current, decision); exact {
						return current, nil
					} else if conflict {
						return unified.ActionAuditRecord{}, ErrDecisionReplayConflict
					}
				}
				return unified.ActionAuditRecord{}, err
			}
			return unified.ActionAuditRecord{}, &PersistError{Op: "action decision", Err: err}
		}
		s.publishTransition(orgID, updated)
		return updated, nil
	}
	return unified.ActionAuditRecord{}, &PersistError{Op: "action decision", Err: unified.ErrActionDecisionRevisionConflict}
}

func decisionReplay(record unified.ActionAuditRecord, decision unified.ActionDecision) (exact, conflict bool) {
	for _, approval := range record.Approvals {
		actor := unified.NormalizeActionActor(approval.ActorBinding)
		if !unified.ActionActorsEqual(actor, decision.Actor) {
			continue
		}
		if approval.Outcome != decision.Outcome || strings.TrimSpace(approval.Reason) != strings.TrimSpace(decision.Reason) || approval.Evidence == nil {
			return false, true
		}
		persisted := *approval.Evidence
		persisted.Actor = unified.NormalizeActionActor(persisted.Actor)
		requested := decision.Evidence
		requested.Actor = unified.NormalizeActionActor(requested.Actor)
		if decisionEvidenceReplayEqual(persisted, requested) {
			return true, false
		}
		return false, true
	}
	return false, false
}

func decisionEvidenceReplayEqual(persisted, requested unified.ApprovalEvidence) bool {
	if persisted.Version != requested.Version || persisted.Method != requested.Method ||
		!unified.ActionActorsEqual(persisted.Actor, requested.Actor) || persisted.OrgID != requested.OrgID ||
		persisted.ActionID != requested.ActionID || persisted.PlanHash != requested.PlanHash || persisted.Outcome != requested.Outcome {
		return false
	}
	switch persisted.Method {
	case unified.MethodWebAuthnUV, unified.MethodDeviceKeyUV:
		return strings.TrimSpace(persisted.ChallengeID) != "" && persisted.ChallengeID == requested.ChallengeID
	case unified.MethodSession, unified.MethodAPIToken:
		return true
	default:
		return false
	}
}

func validateDecisionBinding(record unified.ActionAuditRecord, orgID string, decision unified.ActionDecision) error {
	if err := unified.ValidateActionActor(decision.Actor); err != nil ||
		(decision.Actor.Kind != unified.ActionActorUser && decision.Actor.Kind != unified.ActionActorAPIToken) ||
		decision.Actor.OrgID != strings.TrimSpace(orgID) {
		return ErrApprovalActorNotHuman
	}
	requirement := unified.NormalizeApprovalRequirement(record.Plan.ApprovalRequirement, record.Plan.ApprovalPolicy)
	if requirement.DisallowRequester && strings.EqualFold(decision.Actor.SubjectID, record.Request.Actor.SubjectID) {
		return ErrApprovalSeparationRequired
	}
	evidence := decision.Evidence
	evidence.Actor = unified.NormalizeActionActor(evidence.Actor)
	if evidence.Version != 1 || !unified.ActionActorsEqual(evidence.Actor, decision.Actor) || evidence.OrgID != strings.TrimSpace(orgID) ||
		evidence.ActionID != record.ID || evidence.PlanHash != record.Plan.PlanHash || evidence.Outcome != decision.Outcome || evidence.IssuedAt.IsZero() {
		return ErrApprovalEvidenceInvalid
	}
	return nil
}

func (s *Service) validateApprovalFloor(ctx context.Context, record unified.ActionAuditRecord, decision unified.ActionDecision, consumeStepUp bool) error {
	if decision.Outcome == unified.OutcomeRejected {
		return nil
	}
	requirement := unified.NormalizeApprovalRequirement(record.Plan.ApprovalRequirement, record.Plan.ApprovalPolicy)
	switch requirement.Floor {
	case unified.ApprovalDryRun:
		return unified.ErrActionDryRunOnly
	case unified.ApprovalMultiFactor:
		if decision.Actor.Kind == unified.ActionActorAPIToken {
			return ErrApprovalStepUpUnavailable
		}
		if decision.Evidence.Method != unified.MethodWebAuthnUV && decision.Evidence.Method != unified.MethodDeviceKeyUV {
			return ErrApprovalStepUpUnavailable
		}
		if strings.TrimSpace(decision.Evidence.ChallengeID) == "" || decision.Evidence.ExpiresAt.IsZero() || !s.now().Before(decision.Evidence.ExpiresAt) {
			return ErrApprovalEvidenceInvalid
		}
		if s.StepUpVerifier == nil {
			return ErrApprovalStepUpUnavailable
		}
		if !consumeStepUp {
			return nil
		}
		return s.StepUpVerifier.VerifyAndConsume(ctx, record, decision)
	case unified.ApprovalAdmin, unified.ApprovalNone:
		if decision.Evidence.Method != unified.MethodSession && decision.Evidence.Method != unified.MethodAPIToken {
			return ErrApprovalEvidenceInvalid
		}
		return nil
	default:
		return unified.ErrActionReplanRequired
	}
}

// Execute runs an approved action to a terminal audit state. Every refusal
// path fails closed: expired plans, unapproved or already-final actions,
// plan drift against the live resource contract, and operator remediation
// locks are all persisted as refused executions (never silently dropped)
// and published to the completion hook. There is no bypass that reaches
// the executor without passing every gate.
func (s *Service) Execute(ctx context.Context, orgID, actionID string, actor unified.ActionActor, reason string) (unified.ActionAuditRecord, error) {
	actionID = strings.TrimSpace(actionID)
	if actionID == "" {
		return unified.ActionAuditRecord{}, &ActionNotFoundError{ActionID: actionID}
	}
	store, err := s.store(orgID)
	if err != nil {
		return unified.ActionAuditRecord{}, err
	}
	record, ok, err := store.GetActionAudit(actionID)
	if err != nil {
		return unified.ActionAuditRecord{}, &QueryError{Op: "action audit", Err: err}
	}
	if !ok {
		return unified.ActionAuditRecord{}, &ActionNotFoundError{ActionID: actionID}
	}
	actor = unified.NormalizeActionActor(actor)
	if err := unified.ValidateActionActor(actor); err != nil ||
		(actor.Kind != unified.ActionActorUser && actor.Kind != unified.ActionActorAPIToken) ||
		actor.OrgID != strings.TrimSpace(orgID) {
		return unified.ActionAuditRecord{}, ErrApprovalActorNotHuman
	}
	if err := unified.ValidateHumanActionBinding(record, orgID); err != nil {
		return unified.ActionAuditRecord{}, err
	}
	if s.ExecutionAuthorizer == nil {
		return unified.ActionAuditRecord{}, ErrExecutionAuthorizationUnavailable
	}
	if err := s.ExecutionAuthorizer.AuthorizeExecution(ctx, orgID, record, actor); err != nil {
		return unified.ActionAuditRecord{}, err
	}
	actorID := actor.SubjectID
	if record.State == unified.ActionStateExecuting {
		return s.dispatchCommitted(ctx, orgID, store, record, actorID)
	}
	if record.State == unified.ActionStateCompleted || record.State == unified.ActionStateFailed {
		return record, nil
	}

	now := s.now()
	record, err = s.materializeExpiry(store, orgID, record)
	if err != nil {
		return unified.ActionAuditRecord{}, err
	}
	if record.State == unified.ActionStateExpired {
		return record, unified.ErrActionPlanExpired
	}
	if stopped, stopErr := s.emergencyStopped(orgID); stopErr != nil || stopped {
		if stopErr != nil {
			return unified.ActionAuditRecord{}, &PolicyCheckError{Err: stopErr}
		}
		failed, persistErr := RecordRefusedExecution(store, record, actorID, now, unified.ErrActionEmergencyStop)
		if persistErr != nil {
			return unified.ActionAuditRecord{}, &PersistError{Op: "emergency-stop refusal", Err: persistErr}
		}
		s.publishTransition(orgID, failed)
		s.publishCompleted(failed)
		return failed, unified.ErrActionEmergencyStop
	}
	if err := unified.ValidateActionExecutionStart(record, now); err != nil {
		if unified.IsPermanentActionExecutionRefusal(err) {
			failed, persistErr := RecordRefusedExecution(store, record, actorID, now, err)
			if persistErr != nil {
				return unified.ActionAuditRecord{}, &PersistError{Op: "refused action execution", Err: persistErr}
			}
			s.publishTransition(orgID, failed)
			s.publishCompleted(failed)
			return failed, err
		}
		return unified.ActionAuditRecord{}, err
	}
	if s.Executor == nil {
		return unified.ActionAuditRecord{}, ErrExecutorUnavailable
	}
	if err := s.ValidatePlanFresh(orgID, record); err != nil {
		if errors.Is(err, unified.ErrActionPlanDrift) {
			failed, persistErr := RecordRefusedExecution(store, record, actorID, now, err)
			if persistErr != nil {
				return unified.ActionAuditRecord{}, &PersistError{Op: "refused action execution", Err: persistErr}
			}
			s.publishTransition(orgID, failed)
			s.publishCompleted(failed)
			return failed, err
		}
		return unified.ActionAuditRecord{}, &FreshnessCheckError{Err: err}
	}
	if err := validateExecutionPolicy(store, record); err != nil {
		if unified.IsPermanentActionExecutionRefusal(err) {
			failed, persistErr := RecordRefusedExecution(store, record, actorID, now, err)
			if persistErr != nil {
				return unified.ActionAuditRecord{}, &PersistError{Op: "refused action execution", Err: persistErr}
			}
			s.publishTransition(orgID, failed)
			s.publishCompleted(failed)
			return failed, err
		}
		return unified.ActionAuditRecord{}, &PolicyCheckError{Err: err}
	}

	started, startEvent, err := unified.BeginActionExecution(record, actorID, now)
	if err != nil {
		return unified.ActionAuditRecord{}, err
	}
	if reason != "" {
		startEvent.Message = "Action execution started: " + reason
	}
	attempt, err := unified.NewActionDispatchAttempt(started.ID, now)
	if err != nil {
		return unified.ActionAuditRecord{}, err
	}
	if err := store.RecordActionExecutionAdmission(started, startEvent, attempt); err != nil {
		if errors.Is(err, unified.ErrActionAlreadyExecuting) || errors.Is(err, unified.ErrActionExecutionFinal) {
			current, found, queryErr := store.GetActionAudit(actionID)
			if queryErr == nil && found {
				if current.State == unified.ActionStateExecuting {
					return s.dispatchCommitted(ctx, orgID, store, current, actorID)
				}
				return current, nil
			}
		}
		return unified.ActionAuditRecord{}, &PersistError{Op: "action execution start", Err: err}
	}
	s.publishTransition(orgID, started)
	return s.dispatchCommitted(ctx, orgID, store, started, actorID)
}

// ExecuteUnderPolicy is the only automatic dispatch boundary. It revalidates
// the complete policy under the admission lock and commits policy approval and
// executing atomically before the executor can be called.
func (s *Service) ExecuteUnderPolicy(ctx context.Context, orgID, actionID, actor string, authorize PolicyAuthorizer) (unified.ActionAuditRecord, error) {
	coordinator := s.admissionCoordinator()
	coordinator.mu.RLock()
	started, store, admitted, admissionErr := s.beginPolicyExecution(ctx, orgID, actionID, actor, authorize)
	coordinator.mu.RUnlock()
	if !admitted {
		return started, admissionErr
	}
	return s.dispatchCommitted(ctx, orgID, store, started, actor)
}

func (s *Service) beginPolicyExecution(ctx context.Context, orgID, actionID, actor string, authorize PolicyAuthorizer) (unified.ActionAuditRecord, Store, bool, error) {
	actionID = strings.TrimSpace(actionID)
	store, err := s.store(orgID)
	if err != nil {
		return unified.ActionAuditRecord{}, nil, false, err
	}
	record, found, err := store.GetActionAudit(actionID)
	if err != nil {
		return unified.ActionAuditRecord{}, store, false, &QueryError{Op: "action audit", Err: err}
	}
	if !found {
		return unified.ActionAuditRecord{}, store, false, &ActionNotFoundError{ActionID: actionID}
	}
	if record.State == unified.ActionStateExecuting {
		return record, store, true, nil
	}
	if record.State == unified.ActionStateCompleted || record.State == unified.ActionStateFailed {
		return record, store, false, nil
	}
	now := s.now()
	record, err = s.materializeExpiry(store, orgID, record)
	if err != nil {
		return unified.ActionAuditRecord{}, store, false, err
	}
	if record.State == unified.ActionStateExpired {
		return record, store, false, unified.ErrActionPlanExpired
	}
	if record.State == unified.ActionStateApproved {
		for _, approval := range record.Approvals {
			if approval.Method == unified.MethodPolicy && approval.PolicyLease == nil {
				failed, refuseErr := s.refusePolicyAdmission(store, orgID, record, actor, now, unified.ErrActionPolicyAuthorizationInvalid)
				return failed, store, false, refuseErr
			}
		}
	}
	if stopped, stopErr := s.emergencyStopped(orgID); stopErr != nil || stopped {
		reason := error(unified.ErrActionEmergencyStop)
		if stopErr != nil {
			reason = fmt.Errorf("%w: %v", unified.ErrActionPolicyAuthorizationInvalid, stopErr)
		}
		failed, refuseErr := s.refusePolicyAdmission(store, orgID, record, actor, now, reason)
		return failed, store, false, refuseErr
	}
	if s.Executor == nil {
		return unified.ActionAuditRecord{}, store, false, ErrExecutorUnavailable
	}
	if err := s.ValidatePlanFresh(orgID, record); err != nil {
		failed, refuseErr := s.refusePolicyAdmission(store, orgID, record, actor, now, fmt.Errorf("%w: %v", unified.ErrActionPlanDrift, err))
		return failed, store, false, refuseErr
	}
	if err := validateExecutionPolicy(store, record); err != nil {
		if !unified.IsPermanentActionExecutionRefusal(err) {
			err = fmt.Errorf("%w: %v", unified.ErrActionPolicyAuthorizationInvalid, err)
		}
		failed, refuseErr := s.refusePolicyAdmission(store, orgID, record, actor, now, err)
		return failed, store, false, refuseErr
	}
	if authorize == nil {
		failed, refuseErr := s.refusePolicyAdmission(store, orgID, record, actor, now, unified.ErrActionPolicyAuthorizationInvalid)
		return failed, store, false, refuseErr
	}
	lease, reason, err := authorize(ctx, record, now)
	if err != nil {
		if !errors.Is(err, unified.ErrActionPolicyAuthorizationExpired) && !errors.Is(err, unified.ErrActionPolicyAuthorizationRevoked) {
			err = fmt.Errorf("%w: %v", unified.ErrActionPolicyAuthorizationInvalid, err)
		}
		failed, refuseErr := s.refusePolicyAdmission(store, orgID, record, actor, now, err)
		return failed, store, false, refuseErr
	}
	if strings.TrimSpace(lease.OrgID) != strings.TrimSpace(orgID) || lease.ApprovalPolicy != record.Plan.ApprovalPolicy || lease.AutoAuthorization == unified.AutoAuthorizeNever || lease.ApprovalPolicy == unified.ApprovalDryRun || lease.ApprovalPolicy == unified.ApprovalMultiFactor {
		failed, refuseErr := s.refusePolicyAdmission(store, orgID, record, actor, now, unified.ErrActionPolicyAuthorizationInvalid)
		return failed, store, false, refuseErr
	}
	started, approvedEvent, startEvent, err := unified.BeginPolicyActionExecution(record, unified.ActionApprovalRecord{Actor: actor, Reason: reason}, lease, now)
	if err != nil {
		failed, refuseErr := s.refusePolicyAdmission(store, orgID, record, actor, now, err)
		return failed, store, false, refuseErr
	}
	attempt, err := unified.NewActionDispatchAttempt(started.ID, now)
	if err != nil {
		return unified.ActionAuditRecord{}, store, false, err
	}
	if err := store.RecordActionPolicyExecutionAdmission(started, approvedEvent, startEvent, attempt); err != nil {
		if errors.Is(err, unified.ErrActionAlreadyExecuting) || errors.Is(err, unified.ErrActionExecutionFinal) {
			current, ok, queryErr := store.GetActionAudit(actionID)
			if queryErr == nil && ok {
				return current, store, false, nil
			}
		}
		return unified.ActionAuditRecord{}, store, false, &PersistError{Op: "policy action execution start", Err: err}
	}
	s.publishTransition(orgID, started)
	return started, store, true, nil
}

func (s *Service) materializeExpiry(store Store, orgID string, record unified.ActionAuditRecord) (unified.ActionAuditRecord, error) {
	if record.State == unified.ActionStateExpired || record.State == unified.ActionStateExecuting || record.State == unified.ActionStateCompleted || record.State == unified.ActionStateFailed || record.State == unified.ActionStateRejected {
		return record, nil
	}
	now := s.now()
	if record.Plan.ExpiresAt.IsZero() || now.Before(record.Plan.ExpiresAt) {
		return record, nil
	}
	expired, event, err := unified.ExpireAction(record, "system:expiry", now)
	if err != nil {
		return unified.ActionAuditRecord{}, err
	}
	if err := store.RecordActionExpiry(expired, event); err != nil {
		current, found, queryErr := store.GetActionAudit(record.ID)
		if queryErr == nil && found {
			return current, nil
		}
		return unified.ActionAuditRecord{}, &PersistError{Op: "action expiry", Err: err}
	}
	s.publishTransition(orgID, expired)
	return expired, nil
}

func (s *Service) dispatchCommitted(ctx context.Context, orgID string, store Store, record unified.ActionAuditRecord, actor string) (unified.ActionAuditRecord, error) {
	attempt, found, err := store.GetActionDispatchAttempt(record.ID)
	if err != nil {
		return unified.ActionAuditRecord{}, &QueryError{Op: "action dispatch attempt", Err: err}
	}
	if !found {
		// Legacy executing rows predate durable dispatch authority. They must
		// remain observable, but can never be readmitted or blindly resent.
		return record, nil
	}
	if attempt.State == unified.ActionDispatchReceiptPending || attempt.State == unified.ActionDispatchReceiptRecorded {
		return s.reconcileCommitted(ctx, orgID, store, record, attempt, actor)
	}
	owner := fmt.Sprintf("action-lifecycle:%s:%d", attempt.ID, dispatchWorkerSequence.Add(1))
	attempt, claimed, err := store.ClaimActionDispatch(record.ID, owner, s.now(), 30*time.Second)
	if err != nil {
		return unified.ActionAuditRecord{}, &PersistError{Op: "action dispatch claim", Err: err}
	}
	if !claimed {
		return record, nil
	}
	attempt, err = store.MarkActionDispatchStarted(attempt.ID, owner, s.now())
	if err != nil {
		return unified.ActionAuditRecord{}, &PersistError{Op: "action dispatch start", Err: err}
	}
	result, execErr := s.Executor.ExecuteAction(withDispatchAttempt(ctx, attempt), record)
	if execErr != nil {
		if result != nil {
			return record, fmt.Errorf("%w: executor returned both result and error", unified.ErrExecutorResultContract)
		}
		// A timeout, disconnect, cancellation, or generic executor error is not
		// proof that the transport answered. Preserve receipt_pending so a
		// reconciler can query by attempt ID without resending.
		return record, execErr
	}
	receipt := unified.ActionDispatchReceipt{
		AttemptID: attempt.ID, ActionID: record.ID, TransportRequestID: attempt.ID, ReceivedAt: s.now(),
	}
	return s.completeCorrelatedDispatch(orgID, store, record, receipt, result, actor)
}

func (s *Service) reconcileCommitted(ctx context.Context, orgID string, store Store, record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt, actor string) (unified.ActionAuditRecord, error) {
	reconciler, ok := s.Executor.(DispatchReconciler)
	if !ok {
		return record, nil
	}
	result, receipt, found, err := reconciler.ReconcileActionDispatch(ctx, record, attempt)
	if err != nil {
		if result != nil {
			return record, fmt.Errorf("%w: reconciler returned both result and error", unified.ErrExecutorResultContract)
		}
		return record, err
	}
	if !found {
		return record, err
	}
	if receipt.AttemptID == "" {
		receipt = unified.ActionDispatchReceipt{AttemptID: attempt.ID, ActionID: record.ID, TransportRequestID: attempt.ID, ReceivedAt: s.now()}
	}
	if receipt.AttemptID != attempt.ID || receipt.ActionID != record.ID {
		return unified.ActionAuditRecord{}, unified.ErrActionDispatchReceiptConflict
	}
	return s.completeCorrelatedDispatch(orgID, store, record, receipt, result, actor)
}

func (s *Service) completeCorrelatedDispatch(orgID string, store Store, record unified.ActionAuditRecord, receipt unified.ActionDispatchReceipt, result *unified.ExecutionResult, actor string) (unified.ActionAuditRecord, error) {
	completed, doneEvent, err := unified.CompleteActionExecution(record, result, actor, s.now())
	if err != nil {
		return unified.ActionAuditRecord{}, err
	}
	if err := store.RecordActionDispatchCompletion(receipt, completed, doneEvent); err != nil {
		if errors.Is(err, unified.ErrActionExecutionFinal) {
			if current, found, queryErr := store.GetActionAudit(record.ID); queryErr == nil && found {
				return current, nil
			}
		}
		return unified.ActionAuditRecord{}, &PersistError{Op: "correlated action dispatch completion", Err: err}
	}
	s.publishTransition(orgID, completed)
	s.publishCompleted(completed)
	return completed, nil
}

// RecordDispatchReceipt persists a late authenticated callback correlation.
// It does not infer or mutate terminal execution truth.
func (s *Service) RecordDispatchReceipt(orgID string, receipt unified.ActionDispatchReceipt) (unified.ActionDispatchAttempt, error) {
	store, err := s.store(orgID)
	if err != nil {
		return unified.ActionDispatchAttempt{}, err
	}
	attempt, err := store.RecordActionDispatchReceipt(receipt)
	if err != nil {
		return unified.ActionDispatchAttempt{}, &PersistError{Op: "late action dispatch receipt", Err: err}
	}
	return attempt, nil
}

// RecoverExecutingActions drives restart recovery without blind re-execution.
// Queued or pre-send expired claims may dispatch once; post-start attempts are
// reconciled by durable attempt identity only.
func (s *Service) RecoverExecutingActions(ctx context.Context, orgID, actor string, limit int) ([]unified.ActionAuditRecord, error) {
	store, err := s.store(orgID)
	if err != nil {
		return nil, err
	}
	records, err := store.GetActionAuditsByStates([]unified.ActionState{unified.ActionStateExecuting}, limit)
	if err != nil {
		return nil, &QueryError{Op: "executing action recovery", Err: err}
	}
	recovered := make([]unified.ActionAuditRecord, 0, len(records))
	for _, record := range records {
		current, recoverErr := s.dispatchCommitted(ctx, orgID, store, record, actor)
		if recoverErr != nil {
			continue
		}
		recovered = append(recovered, current)
	}
	return recovered, nil
}

func (s *Service) emergencyStopped(orgID string) (bool, error) {
	if s == nil || s.EmergencyStop == nil {
		return false, nil
	}
	return s.EmergencyStop(orgID)
}

func (s *Service) refusePolicyAdmission(store Store, orgID string, record unified.ActionAuditRecord, actor string, now time.Time, reason error) (unified.ActionAuditRecord, error) {
	failed, persistErr := RecordRefusedExecution(store, record, actor, now, reason)
	if persistErr != nil {
		return unified.ActionAuditRecord{}, &PersistError{Op: "policy admission refusal", Err: persistErr}
	}
	s.publishTransition(orgID, failed)
	s.publishCompleted(failed)
	return failed, reason
}

// ValidatePlanFresh replans the persisted request against the live
// resource contract and refuses execution when the action identity, plan
// hash, resource version, or capability policy no longer match. Execute
// runs it before every dispatch; brokers may also call it as a standalone
// preflight before requesting a decision.
func (s *Service) ValidatePlanFresh(orgID string, record unified.ActionAuditRecord) error {
	normalized, err := unified.NormalizeActionAuditRecord(record)
	if err != nil {
		return fmt.Errorf("%w: %v", unified.ErrActionPlanDrift, err)
	}
	registry, err := s.registry(orgID)
	if err != nil {
		return err
	}
	resource, ok := registry.Get(normalized.Request.ResourceID)
	if !ok || resource == nil {
		return fmt.Errorf("%w: resource %q is no longer present", unified.ErrActionPlanDrift, normalized.Request.ResourceID)
	}
	currentPlan, err := (actionplanner.Planner{Now: func() time.Time {
		return normalized.Plan.PlannedAt
	}}).Plan(normalized.Request, *resource)
	if err != nil {
		return fmt.Errorf("%w: %v", unified.ErrActionPlanDrift, err)
	}
	if currentPlan.ActionID != normalized.Plan.ActionID {
		return fmt.Errorf("%w: action identity changed", unified.ErrActionPlanDrift)
	}
	if currentPlan.PlanHash != normalized.Plan.PlanHash {
		return fmt.Errorf("%w: plan hash changed", unified.ErrActionPlanDrift)
	}
	if currentPlan.ResourceVersion != normalized.Plan.ResourceVersion {
		return fmt.Errorf("%w: resource version changed", unified.ErrActionPlanDrift)
	}
	if currentPlan.PolicyVersion != normalized.Plan.PolicyVersion {
		return fmt.Errorf("%w: capability policy changed", unified.ErrActionPlanDrift)
	}
	return nil
}

// validateExecutionPolicy enforces operator-set per-resource policy at the
// dispatch decision point, currently the NeverAutoRemediate lock.
func validateExecutionPolicy(store Store, record unified.ActionAuditRecord) error {
	if store == nil {
		return errors.New("action audit store unavailable")
	}
	normalized, err := unified.NormalizeActionAuditRecord(record)
	if err != nil {
		return err
	}
	state, found, err := store.GetResourceOperatorState(normalized.Request.ResourceID)
	if err != nil || !found {
		return err
	}
	if state.NeverAutoRemediate {
		return unified.ErrResourceRemediationLocked
	}
	return nil
}

// RecordRefusedExecution persists a refused-before-dispatch execution as a
// terminal failed audit record with its lifecycle event so refusals are
// never silently dropped from the action history.
func RecordRefusedExecution(store Store, record unified.ActionAuditRecord, actor string, now time.Time, reason error) (unified.ActionAuditRecord, error) {
	failed, event, err := unified.RefuseActionExecution(record, reason, actor, now)
	if err != nil {
		return unified.ActionAuditRecord{}, err
	}
	if store == nil {
		return unified.ActionAuditRecord{}, errors.New("action audit store unavailable")
	}
	if err := store.RecordActionExecutionRefusal(failed, event); err != nil {
		return unified.ActionAuditRecord{}, err
	}
	return failed, nil
}

func (s *Service) publishCompleted(record unified.ActionAuditRecord) {
	if s == nil || s.OnActionCompleted == nil {
		return
	}
	if record.State != unified.ActionStateCompleted && record.State != unified.ActionStateFailed {
		return
	}
	s.OnActionCompleted(record)
}

// publishTransition notifies the persisted-state subscriber. Callers invoke
// it only after the corresponding store write succeeded.
func (s *Service) publishTransition(orgID string, record unified.ActionAuditRecord) {
	if s == nil || s.OnActionTransition == nil {
		return
	}
	s.OnActionTransition(orgID, record)
}
