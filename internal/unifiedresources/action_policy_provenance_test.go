package unifiedresources

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func policyProvenanceTestRecord(t *testing.T, id string) ActionAuditRecord {
	t.Helper()
	now := time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC)
	actor := ActionActor{SubjectID: "operator@example.com", Kind: ActionActorUser, CredentialID: "session:test", OrgID: "default"}
	requirement := ApprovalRequirementForFloor(ApprovalAdmin)
	scope := ActionPolicyDecisionScope{OrgID: "default", ResourceID: "vm:42", CapabilityName: "restart"}
	authorities := []ActionPolicyAuthorityFactor{
		{Kind: ActionPolicyAuthorityCapability, SourceID: "capability-registry:restart", Revision: "policy:sha256:0123456789abcdef01234567", Status: ActionPolicyAuthorityConsulted, ApprovalFloor: ApprovalAdmin, ReasonCodes: []ActionPolicyReasonCode{PolicyReasonCapabilityApprovalAdmin, PolicyReasonCapabilityAutoLowRisk}},
		{Kind: ActionPolicyAuthorityTenant, SourceID: "patrol-tenant-policy", Revision: "tenant-policy:sha256:0123456789abcdef01234567", Status: ActionPolicyAuthorityConsulted, ReasonCodes: []ActionPolicyReasonCode{PolicyReasonTenantModeAssisted, PolicyReasonTenantFullLocked}},
		{Kind: ActionPolicyAuthorityResource, SourceID: "resource-operator-policy:vm:42", Revision: "resource-policy:sha256:0123456789abcdef01234567", Status: ActionPolicyAuthorityConsulted, ReasonCodes: []ActionPolicyReasonCode{PolicyReasonResourceCapabilityAllow, PolicyReasonResourceWindowOpen}},
	}
	provenance, err := BuildActionPolicyDecisionProvenance(id, scope, authorities, requirement, true, true)
	if err != nil {
		t.Fatalf("BuildActionPolicyDecisionProvenance: %v", err)
	}
	return ActionAuditRecord{
		ID: id, CreatedAt: now, UpdatedAt: now, State: ActionStatePending,
		Request: ActionRequest{RequestID: "req-" + id, ResourceID: "vm:42", CapabilityName: "restart", Reason: "recover service", RequestedBy: actor.SubjectID, Actor: actor},
		Plan:    ActionPlan{ActionID: id, RequestID: "req-" + id, Allowed: true, RequiresApproval: true, ApprovalPolicy: ApprovalAdmin, ApprovalRequirement: requirement, PlannedAt: now, ExpiresAt: now.Add(time.Hour), ResourceVersion: "resource:sha256:0123456789abcdef01234567", PolicyVersion: "policy:sha256:0123456789abcdef01234567", PolicyDecision: provenance, PlanHash: "sha256:" + id},
	}
}

func policyProvenanceInitialEvents(record ActionAuditRecord) []ActionLifecycleEvent {
	return []ActionLifecycleEvent{
		{ActionID: record.ID, Timestamp: record.CreatedAt, State: ActionStatePlanned, Actor: record.Request.RequestedBy, Message: "Action plan created."},
		{ActionID: record.ID, Timestamp: record.CreatedAt, State: ActionStatePending, Actor: record.Request.RequestedBy, Message: "Action is waiting for approval before execution."},
	}
}

func resignPolicyDecision(record *ActionAuditRecord) {
	record.Plan.PolicyDecision.DecisionID = ActionPolicyDecisionDigest(record.Plan.PolicyDecision)
}

func TestActionPolicyDecisionSemanticMatrixRejectsWithoutStoreMutation(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*ActionAuditRecord)
	}{
		{"unsupported_version", func(r *ActionAuditRecord) { r.Plan.PolicyDecision.Version = 99 }},
		{"fabricated_legacy_authority", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.Version = 0
			r.Plan.PolicyDecision.Status = ActionPolicyDecisionLegacyUnknown
		}},
		{"cross_org_scope", func(r *ActionAuditRecord) { r.Plan.PolicyDecision.Scope.OrgID = "other"; resignPolicyDecision(r) }},
		{"duplicate_authority", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.Authorities = append(r.Plan.PolicyDecision.Authorities, r.Plan.PolicyDecision.Authorities[2])
			resignPolicyDecision(r)
		}},
		{"unbounded_authorities", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.Authorities = append(r.Plan.PolicyDecision.Authorities, r.Plan.PolicyDecision.Authorities[1], r.Plan.PolicyDecision.Authorities[2])
			resignPolicyDecision(r)
		}},
		{"capability_two_approval_reasons", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.Authorities[0].ReasonCodes = append(r.Plan.PolicyDecision.Authorities[0].ReasonCodes, PolicyReasonCapabilityApprovalMFA)
			resignPolicyDecision(r)
		}},
		{"capability_wrong_floor_reason", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.Authorities[0].ReasonCodes[0] = PolicyReasonCapabilityApprovalMFA
			resignPolicyDecision(r)
		}},
		{"capability_two_auto_reasons", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.Authorities[0].ReasonCodes = append(r.Plan.PolicyDecision.Authorities[0].ReasonCodes, PolicyReasonCapabilityAutoElevated)
			resignPolicyDecision(r)
		}},
		{"tenant_two_modes", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.Authorities[1].ReasonCodes = append(r.Plan.PolicyDecision.Authorities[1].ReasonCodes, PolicyReasonTenantModeFull)
			resignPolicyDecision(r)
		}},
		{"tenant_locked_and_unlocked", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.Authorities[1].ReasonCodes = append(r.Plan.PolicyDecision.Authorities[1].ReasonCodes, PolicyReasonTenantFullUnlocked)
			resignPolicyDecision(r)
		}},
		{"tenant_unlocked_while_assisted", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.Authorities[1].ReasonCodes[1] = PolicyReasonTenantFullUnlocked
			resignPolicyDecision(r)
		}},
		{"resource_allow_and_deny", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.Authorities[2].ReasonCodes = append(r.Plan.PolicyDecision.Authorities[2].ReasonCodes, PolicyReasonResourceCapabilityDeny)
			resignPolicyDecision(r)
		}},
		{"resource_denied_with_window", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.Authorities[2].ReasonCodes[0] = PolicyReasonResourceCapabilityDeny
			resignPolicyDecision(r)
		}},
		{"resource_window_open_and_closed", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.Authorities[2].ReasonCodes = append(r.Plan.PolicyDecision.Authorities[2].ReasonCodes, PolicyReasonResourceWindowClosed)
			resignPolicyDecision(r)
		}},
		{"consulted_resource_missing", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.Authorities[2].ReasonCodes[0] = PolicyReasonResourceMissing
			resignPolicyDecision(r)
		}},
		{"unavailable_with_revision", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.Authorities[1].Status = ActionPolicyAuthorityUnavailable
			r.Plan.PolicyDecision.Authorities[1].ReasonCodes = []ActionPolicyReasonCode{PolicyReasonTenantUnavailable}
			resignPolicyDecision(r)
		}},
		{"unavailable_with_extra_reason", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.Authorities[1].Status = ActionPolicyAuthorityUnavailable
			r.Plan.PolicyDecision.Authorities[1].Revision = ""
			r.Plan.PolicyDecision.Authorities[1].ReasonCodes = []ActionPolicyReasonCode{PolicyReasonTenantUnavailable, PolicyReasonTenantModeAssisted}
			resignPolicyDecision(r)
		}},
		{"planning_allowed_false", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.PlanningAllowed = false
			r.Plan.Allowed = false
			resignPolicyDecision(r)
		}},
		{"admin_without_approval", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.RequiresApproval = false
			r.Plan.RequiresApproval = false
			resignPolicyDecision(r)
		}},
		{"plan_requirement_mismatch", func(r *ActionAuditRecord) {
			r.Plan.PolicyDecision.ApprovalRequirement.Floor = ApprovalMultiFactor
			resignPolicyDecision(r)
		}},
	}

	type auditStore interface {
		CreateActionAudit(ActionAuditRecord, []ActionLifecycleEvent) (ActionAuditRecord, bool, error)
		GetActionAudit(string) (ActionAuditRecord, bool, error)
		GetActionLifecycleEvents(string, time.Time, int) ([]ActionLifecycleEvent, error)
	}
	factories := []struct {
		name string
		new  func(*testing.T) auditStore
	}{
		{"memory", func(*testing.T) auditStore { return NewMemoryStore() }},
		{"sqlite", func(t *testing.T) auditStore {
			store, err := NewSQLiteResourceStore(t.TempDir(), "default")
			if err != nil {
				t.Fatal(err)
			}
			return store
		}},
	}
	for _, factory := range factories {
		for _, test := range mutations {
			t.Run(factory.name+"/"+test.name, func(t *testing.T) {
				store := factory.new(t)
				record := policyProvenanceTestRecord(t, "act_"+factory.name+"_"+test.name)
				test.mutate(&record)
				if _, _, err := store.CreateActionAudit(record, policyProvenanceInitialEvents(record)); err == nil {
					t.Fatal("malicious policy provenance was accepted")
				}
				if _, found, err := store.GetActionAudit(record.ID); err != nil || found {
					t.Fatalf("record mutated: found=%v err=%v", found, err)
				}
				events, err := store.GetActionLifecycleEvents(record.ID, time.Time{}, 10)
				if err != nil || len(events) != 0 {
					t.Fatalf("events mutated: events=%#v err=%v", events, err)
				}
				if sqlite, ok := store.(*SQLiteResourceStore); ok {
					reopened, err := NewSQLiteResourceStore(filepath.Dir(filepath.Dir(sqlite.dbPath)), "default")
					if err != nil {
						t.Fatal(err)
					}
					if _, found, err := reopened.GetActionAudit(record.ID); err != nil || found {
						t.Fatalf("reopen mutated record: found=%v err=%v", found, err)
					}
					events, err := reopened.GetActionLifecycleEvents(record.ID, time.Time{}, 10)
					if err != nil || len(events) != 0 {
						t.Fatalf("reopen mutated events: %#v err=%v", events, err)
					}
				}
			})
		}
	}
}

func TestActionPolicyDecisionRoundTripsMemorySQLiteAndLegacyReopen(t *testing.T) {
	record := policyProvenanceTestRecord(t, "act_policy_roundtrip")
	memory := NewMemoryStore()
	if _, created, err := memory.CreateActionAudit(record, policyProvenanceInitialEvents(record)); err != nil || !created {
		t.Fatalf("memory create: created=%v err=%v", created, err)
	}
	got, found, err := memory.GetActionAudit(record.ID)
	if err != nil || !found || !reflect.DeepEqual(got.Plan.PolicyDecision, record.Plan.PolicyDecision) {
		t.Fatalf("memory round trip: found=%v err=%v provenance=%#v", found, err, got.Plan.PolicyDecision)
	}
	got.Plan.PolicyDecision.Authorities[0].ReasonCodes[0] = PolicyReasonCapabilityApprovalMFA
	again, found, err := memory.GetActionAudit(record.ID)
	if err != nil || !found || !reflect.DeepEqual(again.Plan.PolicyDecision, record.Plan.PolicyDecision) {
		t.Fatalf("memory read mutated immutable provenance: found=%v err=%v provenance=%#v", found, err, again.Plan.PolicyDecision)
	}

	dir := t.TempDir()
	sqlite, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	if _, created, err := sqlite.CreateActionAudit(record, policyProvenanceInitialEvents(record)); err != nil || !created {
		t.Fatalf("sqlite create: created=%v err=%v", created, err)
	}
	reopened, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	got, found, err = reopened.GetActionAudit(record.ID)
	if err != nil || !found || !reflect.DeepEqual(got.Plan.PolicyDecision, record.Plan.PolicyDecision) {
		t.Fatalf("sqlite reopen: found=%v err=%v provenance=%#v", found, err, got.Plan.PolicyDecision)
	}

	legacy := record
	legacy.ID = "act_policy_legacy"
	legacy.Request.RequestID = "req-act_policy_legacy"
	legacy.Plan.ActionID = legacy.ID
	legacy.Plan.RequestID = legacy.Request.RequestID
	legacy.Plan.PlanHash = "sha256:legacy"
	legacy.Plan.PolicyDecision = ActionPolicyDecisionProvenance{}
	if _, created, err := reopened.CreateActionAudit(legacy, policyProvenanceInitialEvents(legacy)); err != nil || !created {
		t.Fatalf("legacy create: created=%v err=%v", created, err)
	}
	encoded, err := json.Marshal(legacy.Plan)
	if err != nil {
		t.Fatal(err)
	}
	var legacyWire map[string]any
	if err := json.Unmarshal(encoded, &legacyWire); err != nil {
		t.Fatal(err)
	}
	delete(legacyWire, "policyDecision")
	encoded, err = json.Marshal(legacyWire)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.db.Exec(`UPDATE action_audits SET plan_json = ? WHERE id = ?`, string(encoded), legacy.ID); err != nil {
		t.Fatal(err)
	}
	reopened, err = NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	got, found, err = reopened.GetActionAudit(legacy.ID)
	if err != nil || !found || got.Plan.PolicyDecision.Status != ActionPolicyDecisionLegacyUnknown || got.Plan.PolicyDecision.Version != 0 {
		t.Fatalf("legacy reopen: found=%v err=%v provenance=%#v", found, err, got.Plan.PolicyDecision)
	}
}

func TestActionPolicyDecisionCannotAuthorizeDispatch(t *testing.T) {
	record := policyProvenanceTestRecord(t, "act_policy_not_authority")
	record.State = ActionStatePlanned
	record.Plan.RequiresApproval = false
	record.Plan.ApprovalPolicy = ApprovalNone
	record.Plan.ApprovalRequirement = ApprovalRequirementForFloor(ApprovalNone)
	record.Plan.PolicyDecision.ApprovalRequirement = record.Plan.ApprovalRequirement
	record.Plan.PolicyDecision.RequiresApproval = false
	record.Plan.PolicyDecision.Authorities[0].ApprovalFloor = ApprovalNone
	record.Plan.PolicyDecision.Authorities[0].ReasonCodes[0] = PolicyReasonCapabilityApprovalNone
	resignPolicyDecision(&record)
	approval := ActionApprovalRecord{Actor: "pulse_patrol_policy", Method: MethodPolicy, Outcome: OutcomeApproved, Timestamp: record.CreatedAt}
	if _, _, _, err := BeginPolicyActionExecution(record, approval, ActionPolicyAuthorizationLease{}, record.CreatedAt); !errors.Is(err, ErrActionPolicyAuthorizationInvalid) {
		t.Fatalf("plan-time provenance substituted for dispatch lease: %v", err)
	}
}
