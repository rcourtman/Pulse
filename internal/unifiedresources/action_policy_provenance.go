package unifiedresources

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const ActionPolicyDecisionVersion = 1

type ActionPolicyDecisionStatus string

const (
	ActionPolicyDecisionResolved      ActionPolicyDecisionStatus = "resolved"
	ActionPolicyDecisionLegacyUnknown ActionPolicyDecisionStatus = "legacy_unknown"
)

type ActionPolicyAuthorityKind string

const (
	ActionPolicyAuthorityCapability ActionPolicyAuthorityKind = "capability_registry"
	ActionPolicyAuthorityTenant     ActionPolicyAuthorityKind = "tenant_patrol_policy"
	ActionPolicyAuthorityResource   ActionPolicyAuthorityKind = "resource_operator_policy"
)

type ActionPolicyAuthorityStatus string

const (
	ActionPolicyAuthorityConsulted   ActionPolicyAuthorityStatus = "consulted"
	ActionPolicyAuthorityUnavailable ActionPolicyAuthorityStatus = "unavailable"
	ActionPolicyAuthorityNotFound    ActionPolicyAuthorityStatus = "not_found"
)

type ActionPolicyReasonCode string

const (
	PolicyReasonCapabilityApprovalNone  ActionPolicyReasonCode = "capability_approval_none"
	PolicyReasonCapabilityApprovalAdmin ActionPolicyReasonCode = "capability_approval_admin"
	PolicyReasonCapabilityApprovalMFA   ActionPolicyReasonCode = "capability_approval_mfa"
	PolicyReasonCapabilityDryRun        ActionPolicyReasonCode = "capability_dry_run_only"
	PolicyReasonCapabilityAutoNever     ActionPolicyReasonCode = "capability_auto_never"
	PolicyReasonCapabilityAutoLowRisk   ActionPolicyReasonCode = "capability_auto_low_risk"
	PolicyReasonCapabilityAutoElevated  ActionPolicyReasonCode = "capability_auto_elevated"
	PolicyReasonTenantUnavailable       ActionPolicyReasonCode = "tenant_policy_unavailable"
	PolicyReasonTenantEmergencyStop     ActionPolicyReasonCode = "tenant_emergency_stop"
	PolicyReasonTenantModeMonitor       ActionPolicyReasonCode = "tenant_mode_monitor"
	PolicyReasonTenantModeAssisted      ActionPolicyReasonCode = "tenant_mode_assisted"
	PolicyReasonTenantModeFull          ActionPolicyReasonCode = "tenant_mode_full"
	PolicyReasonTenantModeUnknown       ActionPolicyReasonCode = "tenant_mode_unknown"
	PolicyReasonTenantFullLocked        ActionPolicyReasonCode = "tenant_full_mode_locked"
	PolicyReasonTenantFullUnlocked      ActionPolicyReasonCode = "tenant_full_mode_unlocked"
	PolicyReasonResourceUnavailable     ActionPolicyReasonCode = "resource_policy_unavailable"
	PolicyReasonResourceMissing         ActionPolicyReasonCode = "resource_policy_missing"
	PolicyReasonResourceNeverAuto       ActionPolicyReasonCode = "resource_never_auto_remediate"
	PolicyReasonResourceCapabilityAllow ActionPolicyReasonCode = "resource_capability_allowed"
	PolicyReasonResourceCapabilityDeny  ActionPolicyReasonCode = "resource_capability_not_allowed"
	PolicyReasonResourceWindowOpen      ActionPolicyReasonCode = "resource_window_open"
	PolicyReasonResourceWindowClosed    ActionPolicyReasonCode = "resource_window_closed"
)

type ActionPolicyDecisionScope struct {
	OrgID          string `json:"orgId"`
	ResourceID     string `json:"resourceId"`
	CapabilityName string `json:"capabilityName"`
}

type ActionPolicyAuthorityFactor struct {
	Kind          ActionPolicyAuthorityKind   `json:"kind"`
	SourceID      string                      `json:"sourceId"`
	Revision      string                      `json:"revision,omitempty"`
	Status        ActionPolicyAuthorityStatus `json:"status"`
	Scope         ActionPolicyDecisionScope   `json:"scope"`
	ApprovalFloor ActionApprovalLevel         `json:"approvalFloor,omitempty"`
	ReasonCodes   []ActionPolicyReasonCode    `json:"reasonCodes"`
}

// ActionPolicyDecisionProvenance is immutable descriptive evidence of the
// bounded policy authorities consulted while a plan was created. It is not an
// authorization lease and must never be accepted as dispatch authority.
type ActionPolicyDecisionProvenance struct {
	Version             int                           `json:"version"`
	Status              ActionPolicyDecisionStatus    `json:"status"`
	DecisionID          string                        `json:"decisionId,omitempty"`
	ActionID            string                        `json:"actionId,omitempty"`
	Scope               ActionPolicyDecisionScope     `json:"scope"`
	Authorities         []ActionPolicyAuthorityFactor `json:"authorities"`
	ApprovalRequirement ApprovalRequirement           `json:"approvalRequirement"`
	// PlanningAllowed means the capability request was structurally plannable.
	// It is never current execution authority; dispatch requires a fresh lease.
	PlanningAllowed  bool `json:"planningAllowed"`
	RequiresApproval bool `json:"requiresApproval"`
}

func (provenance ActionPolicyDecisionProvenance) MarshalJSON() ([]byte, error) {
	if provenance.Version == 0 && provenance.Status == "" {
		provenance.Status = ActionPolicyDecisionLegacyUnknown
	}
	type wire ActionPolicyDecisionProvenance
	return json.Marshal(wire(provenance))
}

func LegacyUnknownActionPolicyDecision() ActionPolicyDecisionProvenance {
	return ActionPolicyDecisionProvenance{Status: ActionPolicyDecisionLegacyUnknown}
}

func IsLegacyUnknownActionPolicyDecision(provenance ActionPolicyDecisionProvenance) bool {
	return provenance.Version == 0 && (provenance.Status == "" || provenance.Status == ActionPolicyDecisionLegacyUnknown) &&
		provenance.DecisionID == "" && provenance.ActionID == "" && provenance.Scope == (ActionPolicyDecisionScope{}) &&
		len(provenance.Authorities) == 0 && provenance.ApprovalRequirement == (ApprovalRequirement{}) &&
		!provenance.PlanningAllowed && !provenance.RequiresApproval
}

func NormalizeActionPolicyDecisionScope(scope ActionPolicyDecisionScope) ActionPolicyDecisionScope {
	scope.OrgID = strings.TrimSpace(scope.OrgID)
	scope.ResourceID = CanonicalResourceID(scope.ResourceID)
	scope.CapabilityName = strings.TrimSpace(scope.CapabilityName)
	return scope
}

func ActionPolicyDecisionDigest(provenance ActionPolicyDecisionProvenance) string {
	provenance.DecisionID = ""
	payload, _ := json.Marshal(provenance)
	sum := sha256.Sum256(payload)
	return "policy-decision:sha256:" + hex.EncodeToString(sum[:])
}

func BuildActionPolicyDecisionProvenance(actionID string, scope ActionPolicyDecisionScope, authorities []ActionPolicyAuthorityFactor, requirement ApprovalRequirement, allowed, requiresApproval bool) (ActionPolicyDecisionProvenance, error) {
	scope = NormalizeActionPolicyDecisionScope(scope)
	provenance := ActionPolicyDecisionProvenance{
		Version: ActionPolicyDecisionVersion, Status: ActionPolicyDecisionResolved,
		ActionID: strings.TrimSpace(actionID), Scope: scope,
		Authorities:         append([]ActionPolicyAuthorityFactor(nil), authorities...),
		ApprovalRequirement: requirement, PlanningAllowed: allowed, RequiresApproval: requiresApproval,
	}
	for i := range provenance.Authorities {
		provenance.Authorities[i].Scope = scope
		provenance.Authorities[i].ReasonCodes = append([]ActionPolicyReasonCode(nil), provenance.Authorities[i].ReasonCodes...)
	}
	provenance.DecisionID = ActionPolicyDecisionDigest(provenance)
	if err := ValidateActionPolicyDecisionProvenance(provenance); err != nil {
		return ActionPolicyDecisionProvenance{}, err
	}
	return provenance, nil
}

func ValidateActionPlanPolicyDecision(plan ActionPlan, request ActionRequest) error {
	provenance := plan.PolicyDecision
	if provenance.Version == 0 {
		if !IsLegacyUnknownActionPolicyDecision(provenance) {
			return errors.New("legacy action policy decision contains fabricated authority")
		}
		return nil
	}
	if err := ValidateActionPolicyDecisionProvenance(provenance); err != nil {
		return err
	}
	scope := NormalizeActionPolicyDecisionScope(ActionPolicyDecisionScope{
		OrgID: request.Actor.OrgID, ResourceID: request.ResourceID, CapabilityName: request.CapabilityName,
	})
	if provenance.ActionID != plan.ActionID || provenance.Scope != scope ||
		provenance.ApprovalRequirement != plan.ApprovalRequirement ||
		provenance.PlanningAllowed != plan.Allowed || provenance.RequiresApproval != plan.RequiresApproval {
		return errors.New("action policy decision does not match its plan")
	}
	return nil
}

func ValidateActionPolicyDecisionProvenance(provenance ActionPolicyDecisionProvenance) error {
	if provenance.Version != ActionPolicyDecisionVersion || provenance.Status != ActionPolicyDecisionResolved {
		return errors.New("unsupported action policy decision version or status")
	}
	if provenance.ActionID != strings.TrimSpace(provenance.ActionID) || provenance.Scope != NormalizeActionPolicyDecisionScope(provenance.Scope) {
		return errors.New("action policy decision identity and scope must be canonical")
	}
	provenance.Scope = NormalizeActionPolicyDecisionScope(provenance.Scope)
	if provenance.ActionID == "" || provenance.Scope.OrgID == "" || provenance.Scope.ResourceID == "" || provenance.Scope.CapabilityName == "" {
		return errors.New("action policy decision identity and bounded scope are required")
	}
	if len(provenance.Authorities) < 1 || len(provenance.Authorities) > 3 {
		return errors.New("action policy decision authorities are unbounded")
	}
	if err := ValidateApprovalRequirement(provenance.ApprovalRequirement, provenance.Authorities[0].ApprovalFloor); err != nil {
		return fmt.Errorf("action policy decision approval requirement: %w", err)
	}
	switch provenance.ApprovalRequirement.Floor {
	case ApprovalNone:
		if provenance.RequiresApproval || provenance.ApprovalRequirement.Quorum != 1 || provenance.ApprovalRequirement.DisallowRequester {
			return errors.New("no-approval policy decision has contradictory approval requirements")
		}
	case ApprovalDryRun:
		if provenance.RequiresApproval || provenance.ApprovalRequirement.Quorum != 1 || provenance.ApprovalRequirement.DisallowRequester {
			return errors.New("dry-run policy decision has contradictory approval requirements")
		}
	case ApprovalAdmin, ApprovalMultiFactor:
		if !provenance.RequiresApproval {
			return errors.New("approval policy decision must require approval")
		}
	default:
		return errors.New("action policy decision approval floor is unsupported")
	}
	if !provenance.PlanningAllowed {
		return errors.New("persisted action plan policy decision must be allowed")
	}
	seenKinds := map[ActionPolicyAuthorityKind]struct{}{}
	order := map[ActionPolicyAuthorityKind]int{ActionPolicyAuthorityCapability: 0, ActionPolicyAuthorityTenant: 1, ActionPolicyAuthorityResource: 2}
	previous := -1
	for index, factor := range provenance.Authorities {
		if factor.SourceID != strings.TrimSpace(factor.SourceID) || factor.Revision != strings.TrimSpace(factor.Revision) || factor.Scope != NormalizeActionPolicyDecisionScope(factor.Scope) {
			return errors.New("action policy authority fields must be canonical")
		}
		rank, ok := order[factor.Kind]
		if !ok || rank <= previous || factor.Scope != provenance.Scope {
			return errors.New("action policy decision authority order or scope is invalid")
		}
		previous = rank
		if _, duplicate := seenKinds[factor.Kind]; duplicate {
			return errors.New("duplicate action policy decision authority")
		}
		seenKinds[factor.Kind] = struct{}{}
		if index == 0 && factor.Kind != ActionPolicyAuthorityCapability {
			return errors.New("capability registry must be the first policy authority")
		}
		if err := validateActionPolicyAuthorityFactor(factor, provenance.Scope); err != nil {
			return err
		}
	}
	if _, hasResource := seenKinds[ActionPolicyAuthorityResource]; hasResource {
		if _, hasTenant := seenKinds[ActionPolicyAuthorityTenant]; !hasTenant {
			return errors.New("resource policy authority requires tenant policy authority")
		}
	}
	if provenance.DecisionID == "" || provenance.DecisionID != ActionPolicyDecisionDigest(provenance) {
		return errors.New("action policy decision digest is invalid")
	}
	return nil
}

func validateActionPolicyAuthorityFactor(factor ActionPolicyAuthorityFactor, scope ActionPolicyDecisionScope) error {
	expectedSource := map[ActionPolicyAuthorityKind]string{
		ActionPolicyAuthorityCapability: "capability-registry:" + scope.CapabilityName,
		ActionPolicyAuthorityTenant:     "patrol-tenant-policy",
		ActionPolicyAuthorityResource:   "resource-operator-policy:" + scope.ResourceID,
	}[factor.Kind]
	if factor.SourceID != expectedSource || len(factor.ReasonCodes) == 0 || len(factor.ReasonCodes) > 8 {
		return errors.New("action policy authority identity or reasons are invalid")
	}
	if factor.Status == ActionPolicyAuthorityConsulted {
		prefix := map[ActionPolicyAuthorityKind]string{
			ActionPolicyAuthorityCapability: "policy:sha256:",
			ActionPolicyAuthorityTenant:     "tenant-policy:sha256:",
			ActionPolicyAuthorityResource:   "resource-policy:sha256:",
		}[factor.Kind]
		if !validPolicyRevision(factor.Revision, prefix) {
			return errors.New("action policy authority revision is invalid")
		}
	} else if (factor.Status != ActionPolicyAuthorityUnavailable && factor.Status != ActionPolicyAuthorityNotFound) || factor.Revision != "" {
		return errors.New("action policy authority status is invalid")
	}
	if factor.Kind == ActionPolicyAuthorityCapability {
		if factor.Status != ActionPolicyAuthorityConsulted || factor.ApprovalFloor == "" {
			return errors.New("capability policy authority must capture its approval floor")
		}
	} else if factor.ApprovalFloor != "" {
		return errors.New("non-capability policy authority cannot declare an approval floor")
	}
	seen := map[ActionPolicyReasonCode]struct{}{}
	for _, reason := range factor.ReasonCodes {
		if _, ok := allowedPolicyReasons[factor.Kind][reason]; !ok {
			return fmt.Errorf("unsupported action policy reason %q", reason)
		}
		if _, duplicate := seen[reason]; duplicate {
			return errors.New("duplicate action policy reason")
		}
		seen[reason] = struct{}{}
	}
	if factor.Status == ActionPolicyAuthorityUnavailable || factor.Status == ActionPolicyAuthorityNotFound {
		expected := map[ActionPolicyAuthorityKind]ActionPolicyReasonCode{
			ActionPolicyAuthorityTenant:   PolicyReasonTenantUnavailable,
			ActionPolicyAuthorityResource: PolicyReasonResourceUnavailable,
		}[factor.Kind]
		if factor.Status == ActionPolicyAuthorityNotFound {
			if factor.Kind != ActionPolicyAuthorityResource {
				return errors.New("only resource policy authority may be not found")
			}
			expected = PolicyReasonResourceMissing
		}
		if len(seen) != 1 {
			return errors.New("unavailable policy authority has contradictory reasons")
		}
		if _, ok := seen[expected]; !ok {
			return errors.New("unavailable policy authority reason is invalid")
		}
		return nil
	}
	switch factor.Kind {
	case ActionPolicyAuthorityCapability:
		if countPolicyReasons(seen, PolicyReasonCapabilityApprovalNone, PolicyReasonCapabilityApprovalAdmin, PolicyReasonCapabilityApprovalMFA, PolicyReasonCapabilityDryRun) != 1 ||
			countPolicyReasons(seen, PolicyReasonCapabilityAutoNever, PolicyReasonCapabilityAutoLowRisk, PolicyReasonCapabilityAutoElevated) != 1 {
			return errors.New("capability policy authority reasons are contradictory")
		}
		expectedApproval := map[ActionApprovalLevel]ActionPolicyReasonCode{
			ApprovalNone: PolicyReasonCapabilityApprovalNone, ApprovalAdmin: PolicyReasonCapabilityApprovalAdmin,
			ApprovalMultiFactor: PolicyReasonCapabilityApprovalMFA, ApprovalDryRun: PolicyReasonCapabilityDryRun,
		}[factor.ApprovalFloor]
		if _, ok := seen[expectedApproval]; !ok {
			return errors.New("capability approval reason does not match its floor")
		}
	case ActionPolicyAuthorityTenant:
		if countPolicyReasons(seen, PolicyReasonTenantModeMonitor, PolicyReasonTenantModeAssisted, PolicyReasonTenantModeFull, PolicyReasonTenantModeUnknown) != 1 ||
			countPolicyReasons(seen, PolicyReasonTenantFullLocked, PolicyReasonTenantFullUnlocked) != 1 ||
			countPolicyReasons(seen, PolicyReasonTenantUnavailable) != 0 {
			return errors.New("tenant policy authority reasons are contradictory")
		}
		if _, unlocked := seen[PolicyReasonTenantFullUnlocked]; unlocked {
			if _, full := seen[PolicyReasonTenantModeFull]; !full {
				return errors.New("tenant full-mode unlock contradicts effective mode")
			}
		}
	case ActionPolicyAuthorityResource:
		if countPolicyReasons(seen, PolicyReasonResourceCapabilityAllow, PolicyReasonResourceCapabilityDeny) != 1 ||
			countPolicyReasons(seen, PolicyReasonResourceWindowOpen, PolicyReasonResourceWindowClosed) > 1 ||
			countPolicyReasons(seen, PolicyReasonResourceUnavailable, PolicyReasonResourceMissing) != 0 {
			return errors.New("resource policy authority reasons are contradictory")
		}
		if _, denied := seen[PolicyReasonResourceCapabilityDeny]; denied && countPolicyReasons(seen, PolicyReasonResourceWindowOpen, PolicyReasonResourceWindowClosed) != 0 {
			return errors.New("resource window result cannot qualify a denied capability")
		}
	}
	return nil
}

func countPolicyReasons(seen map[ActionPolicyReasonCode]struct{}, values ...ActionPolicyReasonCode) int {
	count := 0
	for _, value := range values {
		if _, ok := seen[value]; ok {
			count++
		}
	}
	return count
}

func validPolicyRevision(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	digest := strings.TrimPrefix(value, prefix)
	if len(digest) != 24 {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}

var allowedPolicyReasons = map[ActionPolicyAuthorityKind]map[ActionPolicyReasonCode]struct{}{
	ActionPolicyAuthorityCapability: reasonSet(PolicyReasonCapabilityApprovalNone, PolicyReasonCapabilityApprovalAdmin, PolicyReasonCapabilityApprovalMFA, PolicyReasonCapabilityDryRun, PolicyReasonCapabilityAutoNever, PolicyReasonCapabilityAutoLowRisk, PolicyReasonCapabilityAutoElevated),
	ActionPolicyAuthorityTenant:     reasonSet(PolicyReasonTenantUnavailable, PolicyReasonTenantEmergencyStop, PolicyReasonTenantModeMonitor, PolicyReasonTenantModeAssisted, PolicyReasonTenantModeFull, PolicyReasonTenantModeUnknown, PolicyReasonTenantFullLocked, PolicyReasonTenantFullUnlocked),
	ActionPolicyAuthorityResource:   reasonSet(PolicyReasonResourceUnavailable, PolicyReasonResourceMissing, PolicyReasonResourceNeverAuto, PolicyReasonResourceCapabilityAllow, PolicyReasonResourceCapabilityDeny, PolicyReasonResourceWindowOpen, PolicyReasonResourceWindowClosed),
}

func reasonSet(values ...ActionPolicyReasonCode) map[ActionPolicyReasonCode]struct{} {
	out := make(map[ActionPolicyReasonCode]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}
