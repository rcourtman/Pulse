package qualification

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// This file is a white-box table-test suite targeting currently-undercovered
// branches of:
//   - Manifest.Validate (manifest.go)
//   - validateExistingFindingPrerequisite (runner.go)
//   - validateCollectedDockerPredicate (runner.go)
//   - patrolScopeResourceIDs (runner.go)
//
// Every helper and identifier introduced here is prefixed qualw0716 so it
// cannot collide with sibling test files in package qualification.

// qualw0716ValidRemediation returns a manifest that is a fully-valid
// remediation-track scenario. It is the base used by the remediation error-arm
// table; each row mutates a fresh copy.
func qualw0716ValidRemediation() Manifest {
	m := validTestManifest()
	m.Track = TrackRemediation
	m.Patrol.Mode = "approval"
	m.Investigation = &InvestigationSpec{
		MinEvidenceIDs:         1,
		RequiredSummaryTerms:   []string{"stopped"},
		RequireCompletedStatus: true,
	}
	m.Remediation = &RemediationSpec{
		ActionTarget:         "target",
		ExpectedCapabilities: []string{"restart"},
		Decision:             "observe",
		ActionTimeout:        "1m",
	}
	return m
}

// qualw0716AssertErrContains fails t when err is nil or lacks substring.
func qualw0716AssertErrContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain substring %q", err.Error(), want)
	}
}

func Test_w0716_qual_Validate_ErrorArms(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Manifest)
		wantSub string
	}{
		{"schema version mismatch", func(m *Manifest) { m.SchemaVersion = "wrong" }, "schema_version must be"},
		{"id too short", func(m *Manifest) { m.ID = "x" }, "id must be 3-96"},
		{"version not positive", func(m *Manifest) { m.Version = 0 }, "version must be positive"},
		{"title missing", func(m *Manifest) { m.Title = "" }, "title, description, and owner are required"},
		{"owner missing", func(m *Manifest) { m.Owner = "   " }, "title, description, and owner are required"},
		{"unsupported track", func(m *Manifest) { m.Track = "bogus" }, `unsupported track "bogus"`},
		{"unsupported lab driver", func(m *Manifest) { m.Lab.Driver = "vagrant" }, "unsupported lab driver"},
		{"no resources", func(m *Manifest) { m.Resources = nil }, "at least one disposable resource"},
		{"resource alias invalid", func(m *Manifest) { m.Resources[0].Alias = "X" }, "resources[0].alias is invalid"},
		{"duplicate resource alias", func(m *Manifest) {
			m.Resources = append(m.Resources, ResourceSpec{Alias: "target", Kind: "container", Name: "pulse-qual-${run_id}-target-2"})
		}, `duplicate resource alias "target"`},
		{"unsupported resource kind", func(m *Manifest) { m.Resources[0].Kind = "vm" }, `unsupported kind "vm"`},
		{"resource has no image", func(m *Manifest) {
			m.Lab.Image = ""
			m.Resources[0].Image = ""
		}, `resource "target" has no image`},
		{"fault id invalid", func(m *Manifest) { m.Faults[0].ID = "X" }, "faults[0].id is invalid"},
		{"duplicate fault id", func(m *Manifest) {
			m.Faults = append(m.Faults, FaultSpec{
				ID: "fault", CausalGroup: "fault2", Target: "target", Required: true,
				Injector: InjectorSpec{Kind: "stop", Resource: "target"},
				Oracle:   []Predicate{{Probe: "docker.running", Target: "target", Operator: "eq", Value: json.RawMessage("false")}},
				Expected: ExpectedFinding{Resource: "target", ResourceTypes: []string{"app"}, Categories: []string{"reliability"}, Severities: []string{"warning"}, MaxPrimaryFindings: 1},
			})
		}, `duplicate fault id "fault"`},
		{"fault target unknown", func(m *Manifest) { m.Faults[0].Target = "ghost" }, "targets unknown resource"},
		{"fault injector resource unknown", func(m *Manifest) { m.Faults[0].Injector.Resource = "ghost" }, "injector targets unknown resource"},
		{"unsupported injector", func(m *Manifest) { m.Faults[0].Injector.Kind = "bomb" }, `unsupported injector "bomb"`},
		{"fault missing causal group", func(m *Manifest) { m.Faults[0].CausalGroup = "" }, "has no causal_group"},
		{"fault related resource unknown", func(m *Manifest) { m.Faults[0].RelatedResources = []string{"ghost"} }, "references unknown related resource"},
		{"fault related duplicates target", func(m *Manifest) { m.Faults[0].RelatedResources = []string{"target"} }, "related resource duplicates its target"},
		{"fault not required", func(m *Manifest) { m.Faults[0].Required = false }, "must be required for qualification"},
		{"fault incomplete expected finding", func(m *Manifest) { m.Faults[0].Expected.Severities = nil }, "incomplete expected finding semantics"},
		{"fault expected finding unknown resource", func(m *Manifest) { m.Faults[0].Expected.Resource = "ghost" }, "expected finding references unknown resource"},
		{"fault expected finding must equal target or related", func(m *Manifest) {
			m.Resources = append(m.Resources, ResourceSpec{Alias: "decoy", Kind: "container", Name: "pulse-qual-${run_id}-decoy"})
			m.Faults[0].Expected.Resource = "decoy"
		}, "must equal target \"target\" or a declared related resource"},
		{"fault detect_within unparseable", func(m *Manifest) { m.Faults[0].DetectWithin = "bad" }, "detect_within:"},
		{"fault max_primary_findings zero", func(m *Manifest) { m.Faults[0].Expected.MaxPrimaryFindings = 0 }, "max_primary_findings must be positive"},
		{"baseline predicates required", func(m *Manifest) { m.Baseline = nil }, "baseline predicates are required"},
		{"negative control unknown resource", func(m *Manifest) {
			m.NegativeControls = []NegativeControl{{Resource: "ghost", Reason: "none"}}
		}, "negative control references unknown resource"},
		{"scope resources requires scoped flag", func(m *Manifest) {
			m.Patrol.ScopeResources = []string{"target"}
		}, "scope_resources requires patrol.scoped=true"},
		{"scope resources duplicates", func(m *Manifest) {
			m.Patrol.Scoped = true
			m.Patrol.ScopeResources = []string{"target", "target"}
		}, "scope_resources duplicates resource"},
		{"durations unparseable", func(m *Manifest) { m.Collection.ConvergenceTimeout = "bad" }, "collection.convergence_timeout:"},
		{"unsupported patrol mode", func(m *Manifest) { m.Patrol.Mode = "bogus" }, `unsupported Patrol mode "bogus"`},
		{"collection sources empty", func(m *Manifest) { m.Collection.Sources = nil }, "collection.sources must not be empty"},
		{"require exact name false", func(m *Manifest) { m.Collection.RequireExactName = false }, "collection.require_exact_name must be true"},
		{"require real model false", func(m *Manifest) { m.Patrol.RequireRealModel = false }, "require_real_model must be true"},
		{"require tool call evidence false", func(m *Manifest) { m.Patrol.RequireToolCallEvidence = false }, "require_tool_call_evidence must be true"},
		{"require no mutation false", func(m *Manifest) { m.Security.RequireNoMutation = false }, "require_no_unexpected_mutation must be true"},
		{"require fault intact false with faults", func(m *Manifest) { m.Security.RequireFaultIntact = false }, "require_fault_intact_after_patrol must be true when faults are declared"},
		{"teardown second noop false", func(m *Manifest) { m.Teardown.RequireSecondNoop = false }, "teardown must require a second cleanup no-op"},
		{"repeat counts not positive", func(m *Manifest) { m.Repeat.Development = 0 }, "repeat counts must all be positive"},
		{"repeat counts out of order", func(m *Manifest) {
			m.Repeat.Development = 5
			m.Repeat.Nightly = 2
		}, "repeat counts must satisfy qualification >= nightly >= development"},
		{"repeat qualification below wilson floor", func(m *Manifest) { m.Repeat.Qualification = 5 }, "cannot satisfy the 0.850 Wilson lower-bound gate"},
		{"replay lab cannot require real model", func(m *Manifest) { m.Lab.Driver = "replay" }, "replay-only lab cannot require a real model"},
		{"watch track must not declare investigation", func(m *Manifest) {
			m.Investigation = &InvestigationSpec{MinEvidenceIDs: 1, RequiredSummaryTerms: []string{"x"}}
		}, "Watch scenarios must not declare investigation"},
		{"only remediation track may declare remediation", func(m *Manifest) {
			m.Remediation = &RemediationSpec{ActionTarget: "target", ExpectedCapabilities: []string{"restart"}, Decision: "observe", ActionTimeout: "1m"}
		}, "only remediation-track scenarios may declare remediation expectations"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			manifest := validTestManifest()
			tc.mutate(&manifest)
			qualw0716AssertErrContains(t, manifest.Validate(), tc.wantSub)
		})
	}
}

func Test_w0716_qual_Validate_InvestigationArms(t *testing.T) {
	// Base is an investigation-track manifest with a valid Investigation spec.
	base := func() Manifest {
		m := validTestManifest()
		m.Track = TrackInvestigation
		m.Investigation = &InvestigationSpec{
			MinEvidenceIDs:       1,
			RequiredSummaryTerms: []string{"stopped"},
		}
		return m
	}

	cases := []struct {
		name    string
		mutate  func(*Manifest)
		wantSub string
	}{
		{"investigation required for pro track", func(m *Manifest) { m.Investigation = nil }, "investigation expectations are required for Pro tracks"},
		{"investigation min evidence calls must be positive", func(m *Manifest) {
			m.Investigation.MinEvidenceIDs = 0
			m.Investigation.RequiredSummaryTerms = []string{"x"}
		}, "investigation.min_evidence_calls must be positive"},
		{"investigation summary expectations required", func(m *Manifest) {
			m.Investigation.MinEvidenceIDs = 1
			m.Investigation.RequiredSummaryTerms = nil
			m.Investigation.RequiredSummaryTermGroups = nil
		}, "investigation summary expectations must include"},
		{"investigation term group empty", func(m *Manifest) {
			m.Investigation.RequiredSummaryTermGroups = [][]string{{}}
		}, "required_summary_term_groups[0] must not be empty"},
		{"investigation term group element empty", func(m *Manifest) {
			m.Investigation.RequiredSummaryTermGroups = [][]string{{"ok", "  "}}
		}, "required_summary_term_groups[0][1] must not be empty"},
		{"investigation resource expectation unknown", func(m *Manifest) {
			m.Investigation.RootCauseResources = []string{"ghost"}
		}, "investigation resource expectation references unknown resource"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			manifest := base()
			tc.mutate(&manifest)
			qualw0716AssertErrContains(t, manifest.Validate(), tc.wantSub)
		})
	}
}

func Test_w0716_qual_Validate_RemediationArms(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Manifest)
		wantSub string
	}{
		{"remediation required for remediation track", func(m *Manifest) { m.Remediation = nil }, "remediation expectations are required for remediation track"},
		{"remediation action target unknown", func(m *Manifest) { m.Remediation.ActionTarget = "ghost" }, "remediation.action_target references unknown resource"},
		{"remediation expected capabilities empty", func(m *Manifest) { m.Remediation.ExpectedCapabilities = nil }, "remediation.expected_capabilities must not be empty"},
		{"remediation decision unsupported", func(m *Manifest) { m.Remediation.Decision = "maybe" }, `unsupported remediation decision "maybe"`},
		{"remediation action timeout invalid", func(m *Manifest) { m.Remediation.ActionTimeout = "bad" }, "remediation.action_timeout:"},
		{"remediation non-observe requires postconditions", func(m *Manifest) {
			m.Remediation.Decision = "reject"
			m.Remediation.DecisionReason = "why"
		}, "remediation decisions require independent postconditions"},
		{"remediation non-observe requires a reason", func(m *Manifest) {
			m.Remediation.Decision = "reject"
			m.Remediation.Postconditions = []Predicate{{Probe: "docker.running", Target: "target", Operator: "eq", Value: json.RawMessage("true")}}
		}, "remediation decisions require a reason"},
		{"remediation lifecycle verification needs statuses", func(m *Manifest) {
			m.Remediation.RequireLifecycleVerification = true
		}, "required lifecycle verification needs allowed statuses"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			manifest := qualw0716ValidRemediation()
			tc.mutate(&manifest)
			qualw0716AssertErrContains(t, manifest.Validate(), tc.wantSub)
		})
	}
}

func Test_w0716_qual_Validate_ValidManifestsPass(t *testing.T) {
	t.Run("watch base manifest passes", func(t *testing.T) {
		if err := validTestManifest().Validate(); err != nil {
			t.Fatalf("watch base manifest rejected: %v", err)
		}
	})

	t.Run("investigation manifest passes", func(t *testing.T) {
		m := validTestManifest()
		m.Track = TrackInvestigation
		m.Investigation = &InvestigationSpec{MinEvidenceIDs: 2, RequiredSummaryTerms: []string{"stopped"}}
		if err := m.Validate(); err != nil {
			t.Fatalf("investigation manifest rejected: %v", err)
		}
	})

	t.Run("remediation observe manifest passes", func(t *testing.T) {
		if err := qualw0716ValidRemediation().Validate(); err != nil {
			t.Fatalf("remediation manifest rejected: %v", err)
		}
	})

	t.Run("remediation approve_execute with postconditions passes", func(t *testing.T) {
		m := qualw0716ValidRemediation()
		m.Remediation.Decision = "approve_execute"
		m.Remediation.DecisionReason = "restart to recover"
		m.Remediation.Postconditions = []Predicate{{Probe: "docker.running", Target: "target", Operator: "eq", Value: json.RawMessage("true")}}
		if err := m.Validate(); err != nil {
			t.Fatalf("remediation approve_execute manifest rejected: %v", err)
		}
	})
}

func Test_w0716_qual_ValidateExistingFindingPrerequisite(t *testing.T) {
	twoFaults := []FaultSpec{
		{ID: "f1", Target: "alpha", Required: true, Expected: ExpectedFinding{Resource: "alpha"}},
		{ID: "f2", Target: "alpha", Required: true, Expected: ExpectedFinding{Resource: "alpha"}},
	}

	cases := []struct {
		name      string
		faults    []FaultSpec
		collected map[string]Resource
		warmup    PatrolRun
		findings  []Finding
		wantErr   string
	}{
		{
			name:      "error status fails prerequisite",
			faults:    []FaultSpec{{ID: "f1", Target: "alpha", Required: true, Expected: ExpectedFinding{Resource: "alpha"}}},
			collected: map[string]Resource{"alpha": {ID: "res-alpha"}},
			warmup:    PatrolRun{Status: "error"},
			findings:  []Finding{{ResourceID: "res-alpha"}},
			wantErr:   "prerequisite Patrol run failed",
		},
		{
			name:      "positive error count fails prerequisite even with ok status",
			faults:    []FaultSpec{{ID: "f1", Target: "alpha", Required: true, Expected: ExpectedFinding{Resource: "alpha"}}},
			collected: map[string]Resource{"alpha": {ID: "res-alpha"}},
			warmup:    PatrolRun{Status: "issues_found", ErrorCount: 2},
			findings:  []Finding{{ResourceID: "res-alpha"}},
			wantErr:   "prerequisite Patrol run failed",
		},
		{
			name:      "no run-owned finding created",
			faults:    []FaultSpec{{ID: "f1", Target: "alpha", Required: true, Expected: ExpectedFinding{Resource: "alpha"}}},
			collected: map[string]Resource{"alpha": {ID: "res-alpha"}},
			warmup:    PatrolRun{Status: "issues_found"},
			findings:  nil,
			wantErr:   "did not create a run-owned finding",
		},
		{
			name:      "expected resource not collected",
			faults:    []FaultSpec{{ID: "f1", Target: "alpha", Required: true, Expected: ExpectedFinding{Resource: "alpha"}}},
			collected: map[string]Resource{},
			warmup:    PatrolRun{Status: "issues_found"},
			findings:  []Finding{{ResourceID: "other"}},
			wantErr:   `expected resource "alpha" was not collected`,
		},
		{
			name:      "collected resource has empty id",
			faults:    []FaultSpec{{ID: "f1", Target: "alpha", Required: true, Expected: ExpectedFinding{Resource: "alpha"}}},
			collected: map[string]Resource{"alpha": {ID: ""}},
			warmup:    PatrolRun{Status: "issues_found"},
			findings:  []Finding{{ResourceID: "other"}},
			wantErr:   `expected resource "alpha" was not collected`,
		},
		{
			name:      "no finding for expected resource",
			faults:    []FaultSpec{{ID: "f1", Target: "alpha", Required: true, Expected: ExpectedFinding{Resource: "alpha"}}},
			collected: map[string]Resource{"alpha": {ID: "res-alpha"}},
			warmup:    PatrolRun{Status: "issues_found"},
			findings:  []Finding{{ResourceID: "res-beta"}},
			wantErr:   "created no finding for expected resource",
		},
		{
			name:      "empty expected resource falls back to target and succeeds",
			faults:    []FaultSpec{{ID: "f1", Target: "alpha", Required: true, Expected: ExpectedFinding{Resource: ""}}},
			collected: map[string]Resource{"alpha": {ID: "res-alpha"}},
			warmup:    PatrolRun{Status: "issues_found"},
			findings:  []Finding{{ResourceID: "res-alpha"}},
			wantErr:   "",
		},
		{
			name:      "duplicate alias dedup skips second fault and succeeds",
			faults:    twoFaults,
			collected: map[string]Resource{"alpha": {ID: "res-alpha"}},
			warmup:    PatrolRun{Status: "issues_found"},
			findings:  []Finding{{ResourceID: "res-alpha"}},
			wantErr:   "",
		},
		{
			name: "non-required fault is skipped",
			faults: []FaultSpec{
				{ID: "f1", Target: "alpha", Required: true, Expected: ExpectedFinding{Resource: "alpha"}},
				{ID: "f2", Target: "beta", Required: false, Expected: ExpectedFinding{Resource: "beta"}},
			},
			collected: map[string]Resource{"alpha": {ID: "res-alpha"}},
			warmup:    PatrolRun{Status: "issues_found"},
			findings:  []Finding{{ResourceID: "res-alpha"}},
			wantErr:   "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			manifest := Manifest{Faults: tc.faults}
			err := validateExistingFindingPrerequisite(manifest, tc.collected, tc.warmup, tc.findings)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			qualw0716AssertErrContains(t, err, tc.wantErr)
		})
	}
}

func Test_w0716_qual_ValidateCollectedDockerPredicate(t *testing.T) {
	resources := map[string]Resource{
		"up":   {ID: "r-up", Docker: &DockerResource{ContainerState: "running", Health: "healthy", RestartCount: 3}},
		"down": {ID: "r-down", Docker: &DockerResource{ContainerState: "exited", Health: "unhealthy", RestartCount: 0}},
		"bare": {ID: "r-bare"},
	}

	cases := []struct {
		name      string
		predicate Predicate
		resources map[string]Resource
		wantErr   string
	}{
		{"target absent", Predicate{Probe: "docker.running", Target: "missing", Operator: "eq", Value: json.RawMessage("true")}, resources, "absent from collected resources"},
		{"no docker projection", Predicate{Probe: "docker.running", Target: "bare", Operator: "eq", Value: json.RawMessage("true")}, resources, "has no collected Docker projection"},
		{"health value parse error", Predicate{Probe: "docker.health", Target: "up", Operator: "eq", Value: json.RawMessage("not-a-string")}, resources, "invalid collected health expectation"},
		{"health mismatch", Predicate{Probe: "docker.health", Target: "up", Operator: "eq", Value: json.RawMessage(`"unhealthy"`)}, resources, "collected health="},
		{"health match", Predicate{Probe: "docker.health", Target: "up", Operator: "eq", Value: json.RawMessage(`"healthy"`)}, resources, ""},
		{"health match case insensitive", Predicate{Probe: "docker.health", Target: "up", Operator: "eq", Value: json.RawMessage(`"  HEALTHY  "`)}, resources, ""},
		{"running value parse error", Predicate{Probe: "docker.running", Target: "up", Operator: "eq", Value: json.RawMessage("not-a-bool")}, resources, "invalid collected running expectation"},
		{"running mismatch", Predicate{Probe: "docker.running", Target: "up", Operator: "eq", Value: json.RawMessage("false")}, resources, "collected running=true"},
		{"running match true", Predicate{Probe: "docker.running", Target: "up", Operator: "eq", Value: json.RawMessage("true")}, resources, ""},
		{"running match false", Predicate{Probe: "docker.running", Target: "down", Operator: "eq", Value: json.RawMessage("false")}, resources, ""},
		{"restart count value parse error", Predicate{Probe: "docker.restart_count", Target: "up", Operator: "eq", Value: json.RawMessage("not-an-int")}, resources, "invalid collected restart-count expectation"},
		{"restart count eq fail", Predicate{Probe: "docker.restart_count", Target: "up", Operator: "eq", Value: json.RawMessage("5")}, resources, "does not satisfy eq 5"},
		{"restart count eq success", Predicate{Probe: "docker.restart_count", Target: "up", Operator: "eq", Value: json.RawMessage("3")}, resources, ""},
		{"restart count gte success", Predicate{Probe: "docker.restart_count", Target: "up", Operator: "gte", Value: json.RawMessage("2")}, resources, ""},
		{"restart count gte fail", Predicate{Probe: "docker.restart_count", Target: "up", Operator: "gte", Value: json.RawMessage("10")}, resources, "does not satisfy gte 10"},
		{"restart count unhandled operator fails", Predicate{Probe: "docker.restart_count", Target: "up", Operator: "gt", Value: json.RawMessage("1")}, resources, "does not satisfy gt 1"},
		{"unhandled probe falls through with nil", Predicate{Probe: "docker.exists", Target: "up", Operator: "eq", Value: json.RawMessage("true")}, resources, ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateCollectedDockerPredicate("subject", tc.predicate, tc.resources)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			qualw0716AssertErrContains(t, err, tc.wantErr)
		})
	}
}

func Test_w0716_qual_PatrolScopeResourceIDs(t *testing.T) {
	collected := map[string]Resource{
		"alpha": {ID: "id-alpha"},
		"beta":  {ID: "id-beta"},
		"gamma": {ID: "id-gamma"},
	}

	cases := []struct {
		name   string
		scoped bool
		scope  []string
		want   []string
	}{
		{"unscoped returns nil", false, nil, nil},
		{"unscoped ignores scope resources", false, []string{"alpha"}, nil},
		{"scoped with explicit subset sorted", true, []string{"gamma", "alpha"}, []string{"id-alpha", "id-gamma"}},
		{"scoped with empty scope returns all sorted", true, nil, []string{"id-alpha", "id-beta", "id-gamma"}},
		{"scoped all resources sorted", true, []string{"beta", "alpha", "gamma"}, []string{"id-alpha", "id-beta", "id-gamma"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			manifest := Manifest{Patrol: PatrolSpec{Scoped: tc.scoped, ScopeResources: tc.scope}}
			got := patrolScopeResourceIDs(manifest, collected)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("patrolScopeResourceIDs = %v, want %v", got, tc.want)
			}
		})
	}
}
