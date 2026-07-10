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
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionplanner"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Executor runs a previously planned and approved action through the
// canonical execution contract.
type Executor interface {
	ExecuteAction(ctx context.Context, record unified.ActionAuditRecord) (*unified.ExecutionResult, error)
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
	RecordActionAudit(record unified.ActionAuditRecord) error
	GetActionAudit(actionID string) (unified.ActionAuditRecord, bool, error)
	RecordActionDecision(record unified.ActionAuditRecord, event unified.ActionLifecycleEvent) error
	RecordActionExecutionStart(record unified.ActionAuditRecord, event unified.ActionLifecycleEvent) error
	RecordActionExecutionResult(record unified.ActionAuditRecord, event unified.ActionLifecycleEvent) error
	RecordActionLifecycleEvent(event unified.ActionLifecycleEvent) error
	GetActionLifecycleEvents(actionID string, since time.Time, limit int) ([]unified.ActionLifecycleEvent, error)
	GetResourceOperatorState(canonicalID string) (unified.ResourceOperatorState, bool, error)
}

// Service wires the lifecycle over per-org registry and store lookups. All
// dependencies are resolved per call so late-bound wiring (executors and
// publishers set after construction) stays current.
type Service struct {
	Registry func(orgID string) (*unified.ResourceRegistry, error)
	Store    func(orgID string) (Store, error)
	Executor Executor
	// OnActionCompleted receives every terminal (completed/failed) audit
	// record, including refused-before-dispatch failures, so SSE bridges
	// and reconcilers observe the full lifecycle regardless of transport.
	OnActionCompleted func(unified.ActionAuditRecord)
	Now               func() time.Time
}

// Sentinel errors for dependency failures. Callers map these to their
// transport's unavailability semantics.
var (
	ErrRegistryUnavailable = errors.New("resource registry unavailable")
	ErrStoreUnavailable    = errors.New("action audit store unavailable")
	ErrExecutorUnavailable = errors.New("no action executor is configured")
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
func (s *Service) Plan(ctx context.Context, orgID string, req unified.ActionRequest) (unified.ActionPlan, error) {
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

	plan, err := (actionplanner.Planner{}).Plan(req, *resource)
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
	if err := PersistPlanAudit(store, req, plan); err != nil {
		return unified.ActionPlan{}, &PersistError{Op: "action plan audit", Err: err}
	}
	return plan, nil
}

// PersistPlanAudit records the planned action's audit record and its
// initial lifecycle events, deduplicating states that were already
// recorded for the same action ID (idempotent replans).
func PersistPlanAudit(store Store, req unified.ActionRequest, plan unified.ActionPlan) error {
	state := PlannedActionState(plan)
	record := unified.ActionAuditRecord{
		ID:        plan.ActionID,
		CreatedAt: plan.PlannedAt,
		UpdatedAt: plan.PlannedAt,
		State:     state,
		Request:   req,
		Plan:      plan,
	}
	if err := store.RecordActionAudit(record); err != nil {
		return err
	}

	existingEvents, err := store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 100)
	if err != nil {
		return err
	}
	seenStates := map[unified.ActionState]bool{}
	for _, event := range existingEvents {
		seenStates[event.State] = true
	}

	if !seenStates[unified.ActionStatePlanned] {
		if err := store.RecordActionLifecycleEvent(unified.ActionLifecycleEvent{
			ActionID:  plan.ActionID,
			Timestamp: plan.PlannedAt,
			State:     unified.ActionStatePlanned,
			Actor:     req.RequestedBy,
			Message:   "Action plan created.",
		}); err != nil {
			return err
		}
	}
	if state != unified.ActionStatePlanned && !seenStates[state] {
		if err := store.RecordActionLifecycleEvent(unified.ActionLifecycleEvent{
			ActionID:  plan.ActionID,
			Timestamp: plan.PlannedAt,
			State:     state,
			Actor:     req.RequestedBy,
			Message:   "Action is waiting for approval before execution.",
		}); err != nil {
			return err
		}
	}
	return nil
}

// PlannedActionState is the initial audit state for a fresh plan: pending
// when the capability policy requires approval, planned otherwise.
func PlannedActionState(plan unified.ActionPlan) unified.ActionState {
	if plan.RequiresApproval {
		return unified.ActionStatePending
	}
	return unified.ActionStatePlanned
}

// Decide applies an approval outcome to a pending action. The caller
// supplies the approval's actor, method, outcome, and reason; the service
// stamps the decision time when unset and persists the resulting state
// transition and lifecycle event atomically through the store contract.
func (s *Service) Decide(ctx context.Context, orgID, actionID string, approval unified.ActionApprovalRecord) (unified.ActionAuditRecord, error) {
	_ = ctx
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

	now := s.now()
	if approval.Timestamp.IsZero() {
		approval.Timestamp = now
	}
	updated, event, err := unified.ApplyActionDecision(record, approval, now)
	if err != nil {
		return unified.ActionAuditRecord{}, err
	}
	if err := store.RecordActionDecision(updated, event); err != nil {
		if errors.Is(err, unified.ErrActionNotPending) {
			return unified.ActionAuditRecord{}, err
		}
		return unified.ActionAuditRecord{}, &PersistError{Op: "action decision", Err: err}
	}
	return updated, nil
}

// Execute runs an approved action to a terminal audit state. Every refusal
// path fails closed: expired plans, unapproved or already-final actions,
// plan drift against the live resource contract, and operator remediation
// locks are all persisted as refused executions (never silently dropped)
// and published to the completion hook. There is no bypass that reaches
// the executor without passing every gate.
func (s *Service) Execute(ctx context.Context, orgID, actionID, actor, reason string) (unified.ActionAuditRecord, error) {
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

	now := s.now()
	if err := unified.ValidateActionExecutionStart(record, now); err != nil {
		if unified.IsPermanentActionExecutionRefusal(err) {
			failed, persistErr := RecordRefusedExecution(store, record, actor, now, err)
			if persistErr != nil {
				return unified.ActionAuditRecord{}, &PersistError{Op: "refused action execution", Err: persistErr}
			}
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
			failed, persistErr := RecordRefusedExecution(store, record, actor, now, err)
			if persistErr != nil {
				return unified.ActionAuditRecord{}, &PersistError{Op: "refused action execution", Err: persistErr}
			}
			s.publishCompleted(failed)
			return failed, err
		}
		return unified.ActionAuditRecord{}, &FreshnessCheckError{Err: err}
	}
	if err := validateExecutionPolicy(store, record); err != nil {
		if unified.IsPermanentActionExecutionRefusal(err) {
			failed, persistErr := RecordRefusedExecution(store, record, actor, now, err)
			if persistErr != nil {
				return unified.ActionAuditRecord{}, &PersistError{Op: "refused action execution", Err: persistErr}
			}
			s.publishCompleted(failed)
			return failed, err
		}
		return unified.ActionAuditRecord{}, &PolicyCheckError{Err: err}
	}

	started, startEvent, err := unified.BeginActionExecution(record, actor, now)
	if err != nil {
		return unified.ActionAuditRecord{}, err
	}
	if reason != "" {
		startEvent.Message = "Action execution started: " + reason
	}
	if err := store.RecordActionExecutionStart(started, startEvent); err != nil {
		return unified.ActionAuditRecord{}, &PersistError{Op: "action execution start", Err: err}
	}

	result, execErr := s.Executor.ExecuteAction(ctx, started)
	if execErr != nil {
		result = &unified.ExecutionResult{Success: false, ErrorMessage: execErr.Error()}
	}
	completed, doneEvent, err := unified.CompleteActionExecution(started, result, actor, s.now())
	if err != nil {
		return unified.ActionAuditRecord{}, err
	}
	if err := store.RecordActionExecutionResult(completed, doneEvent); err != nil {
		return unified.ActionAuditRecord{}, &PersistError{Op: "action execution result", Err: err}
	}
	s.publishCompleted(completed)
	return completed, nil
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
	if err := store.RecordActionAudit(failed); err != nil {
		return unified.ActionAuditRecord{}, err
	}
	if err := store.RecordActionLifecycleEvent(event); err != nil {
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
