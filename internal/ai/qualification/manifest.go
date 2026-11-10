// Package qualification implements independent-ground-truth qualification for
// Pulse Patrol. Unlike the smoke evals in internal/ai/eval, expected faults are
// declared by scenario manifests and confirmed by an out-of-band lab oracle.
package qualification

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const SchemaVersion = "patrol.qual/v1"

var safeID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,95}$`)

type Track string

const (
	TrackWatch         Track = "watch"
	TrackInvestigation Track = "investigation"
	TrackRemediation   Track = "remediation"
	// TrackResponse scores what Patrol does with a real fault from the moment
	// it appears: whether it woke by itself, what it concluded, whether a
	// governed action fixed it. It produces an outcome grade for a scorecard,
	// not a launch gate.
	TrackResponse Track = "response"
)

const (
	resourceKindContainer      = "container"
	resourceKindHostFilesystem = "host_filesystem"
)

// Manifest is the reviewed, model-independent statement of a qualification
// scenario. It intentionally contains no expected Patrol tool names.
type Manifest struct {
	SchemaVersion    string             `json:"schema_version"`
	ID               string             `json:"id"`
	Version          int                `json:"version"`
	Title            string             `json:"title"`
	Description      string             `json:"description"`
	Owner            string             `json:"owner"`
	Track            Track              `json:"track"`
	Risk             string             `json:"risk"`
	Tags             []string           `json:"tags,omitempty"`
	Lab              LabSpec            `json:"lab"`
	Resources        []ResourceSpec     `json:"resources"`
	Baseline         []Predicate        `json:"baseline"`
	Faults           []FaultSpec        `json:"faults"`
	NegativeControls []NegativeControl  `json:"negative_controls,omitempty"`
	Collection       CollectionSpec     `json:"collection"`
	Patrol           PatrolSpec         `json:"patrol"`
	Investigation    *InvestigationSpec `json:"investigation,omitempty"`
	Remediation      *RemediationSpec   `json:"remediation,omitempty"`
	Engagement       *EngagementSpec    `json:"engagement,omitempty"`
	Outcome          *OutcomeSpec       `json:"outcome,omitempty"`
	Security         SecuritySpec       `json:"security"`
	Budgets          BudgetSpec         `json:"budgets"`
	Repeat           RepeatSpec         `json:"repeat"`
	Gates            GateSpec           `json:"gates"`
	Teardown         TeardownSpec       `json:"teardown"`
	Metadata         map[string]string  `json:"metadata,omitempty"`
}

type LabSpec struct {
	Driver       string `json:"driver"`
	Profile      string `json:"profile"`
	Image        string `json:"image"`
	AllowPull    bool   `json:"allow_pull,omitempty"`
	SharedHostOK bool   `json:"shared_host_ok,omitempty"`
}

type ResourceSpec struct {
	Alias string `json:"alias"`
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	// Fixture names a driver-owned service fixture (see fixtures.go). The
	// image, entrypoint, health check and mounts then come from the reviewed
	// fixture, so the running container shows what a real service would show.
	Fixture string `json:"fixture,omitempty"`
	// Env is the container environment. It is scenario-owned ground truth: a
	// misconfiguration fault changes one of these values.
	Env map[string]string `json:"env,omitempty"`
	// Volume is a lab-created named volume shared by every resource that
	// declares the same name. The mount target belongs to the fixture.
	Volume string `json:"volume,omitempty"`
	// MountPath is the dedicated host filesystem of a host_filesystem
	// resource. The operator has to name the same path on the command line.
	MountPath      string            `json:"mount_path,omitempty"`
	Image          string            `json:"image,omitempty"`
	Command        []string          `json:"command,omitempty"`
	Restart        string            `json:"restart,omitempty"`
	Healthcheck    []string          `json:"healthcheck,omitempty"`
	HealthEvery    string            `json:"health_every,omitempty"`
	FaultVolume    bool              `json:"fault_volume,omitempty"`
	ScratchStorage bool              `json:"scratch_storage,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
}

type FaultSpec struct {
	ID               string          `json:"id"`
	CausalGroup      string          `json:"causal_group"`
	Target           string          `json:"target"`
	Injector         InjectorSpec    `json:"injector"`
	Oracle           []Predicate     `json:"oracle"`
	Expected         ExpectedFinding `json:"expected_finding"`
	RevertOracle     []Predicate     `json:"revert_oracle,omitempty"`
	DetectWithin     string          `json:"detect_within,omitempty"`
	Required         bool            `json:"required"`
	RelatedResources []string        `json:"related_resources,omitempty"`
	AllowedCoTags    []string        `json:"allowed_cofinding_tags,omitempty"`
}

type InjectorSpec struct {
	Kind     string `json:"kind"`
	Resource string `json:"resource"`
	Value    string `json:"value,omitempty"`
}

type Predicate struct {
	Probe    string          `json:"probe"`
	Target   string          `json:"target"`
	Operator string          `json:"operator"`
	Value    json.RawMessage `json:"value"`
	Timeout  string          `json:"timeout,omitempty"`
}

type ExpectedFinding struct {
	Resource           string   `json:"resource"`
	ResourceTypes      []string `json:"resource_types"`
	Categories         []string `json:"categories"`
	Severities         []string `json:"severities"`
	RequiredEvidence   []string `json:"required_evidence,omitempty"`
	AllowedAdvice      []string `json:"allowed_advice,omitempty"`
	ForbiddenAdvice    []string `json:"forbidden_advice,omitempty"`
	MaxPrimaryFindings int      `json:"max_primary_findings"`
}

type NegativeControl struct {
	Resource string `json:"resource"`
	Reason   string `json:"reason"`
}

type CollectionSpec struct {
	Sources            []string `json:"sources"`
	ConvergenceTimeout string   `json:"convergence_timeout"`
	PollInterval       string   `json:"poll_interval"`
	RequireExactName   bool     `json:"require_exact_name"`
}

type PatrolSpec struct {
	Mode   string `json:"mode"`
	Scoped bool   `json:"scoped"`
	// ScopeResources optionally narrows the initial Patrol trigger to
	// reviewed resource aliases. Collection and the out-of-band oracle still
	// observe the whole lab, so related resources remain independently
	// scoreable even when they are outside the trigger anchor.
	ScopeResources                []string `json:"scope_resources,omitempty"`
	RunTimeout                    string   `json:"run_timeout"`
	InvestigationTimeout          string   `json:"investigation_timeout,omitempty"`
	RequireRealModel              bool     `json:"require_real_model"`
	RequireToolCallEvidence       bool     `json:"require_tool_call_evidence"`
	RequireExistingReconfirmation bool     `json:"require_existing_reconfirmation,omitempty"`
}

// InvestigationSpec declares independently reviewable expectations for the
// Pro investigation output. These are semantic expectations, never expected
// tool names, so the model remains free to choose its own evidence path.
type InvestigationSpec struct {
	MinEvidenceCalls int `json:"min_evidence_calls,omitempty"`
	// MinEvidenceIDs is the report-v1 replay compatibility name. New
	// manifests use MinEvidenceCalls; both score the same scenario-owned
	// evidence-call quantity.
	MinEvidenceIDs int `json:"min_evidence_ids,omitempty"`
	// RequiredSummaryTerms are facts with one canonical accepted expression.
	// RequiredSummaryTermGroups are scenario-owned semantic alternatives; at
	// least one non-empty term in every group must appear. This avoids teaching
	// models a magic word when normal collection exposes an equivalent state.
	RequiredSummaryTerms      []string   `json:"required_summary_terms,omitempty"`
	RequiredSummaryTermGroups [][]string `json:"required_summary_term_groups,omitempty"`
	ForbiddenSummaryTerms     []string   `json:"forbidden_summary_terms,omitempty"`
	// RootCauseResources and AffectedResources are scenario-owned aliases.
	// They are matched against named response sections, never inferred from
	// whichever tools the model chose to call.
	RootCauseResources []string `json:"root_cause_resources,omitempty"`
	AffectedResources  []string `json:"affected_resources,omitempty"`
	MaxToolsUsed       int      `json:"max_tools_used,omitempty"`
	// MaxEvidenceCalls gates actual evidence-call volume. MaxToolsUsed is a
	// separate diversity/load signal counting distinct tool names.
	MaxEvidenceCalls       int  `json:"max_evidence_calls,omitempty"`
	RequireCompletedStatus bool `json:"require_completed_status"`
}

func (s InvestigationSpec) minimumEvidenceCalls() int {
	if s.MinEvidenceCalls > 0 {
		return s.MinEvidenceCalls
	}
	return s.MinEvidenceIDs
}

// RemediationSpec governs the optional decision/execution portion of a Pro
// scenario. The runner binds the action to the scenario finding, exact
// collected resource, expected capability, persisted action ID, and plan hash
// before it can record a decision or execute anything. The
// await_autonomous decision is deliberately different: the runner records no
// decision and sends no execute request; it only waits for Patrol's governed
// action lifecycle to settle and verifies the independent postconditions.
type RemediationSpec struct {
	ActionTarget                 string      `json:"action_target"`
	ExpectedCapabilities         []string    `json:"expected_capabilities"`
	Decision                     string      `json:"decision"`
	DecisionReason               string      `json:"decision_reason"`
	ActionTimeout                string      `json:"action_timeout"`
	RequireExactOrigin           bool        `json:"require_exact_origin"`
	RequireLifecycleVerification bool        `json:"require_lifecycle_verification"`
	AllowedVerificationStatuses  []string    `json:"allowed_verification_statuses,omitempty"`
	Postconditions               []Predicate `json:"postconditions"`
}

type SecuritySpec struct {
	ForbiddenToolNames    []string `json:"forbidden_tool_names,omitempty"`
	ForbiddenOutputMarker []string `json:"forbidden_output_markers,omitempty"`
	RequireFaultIntact    bool     `json:"require_fault_intact_after_patrol"`
	RequireNoMutation     bool     `json:"require_no_unexpected_mutation"`
}

type BudgetSpec struct {
	CollectionLatencyP95 string  `json:"collection_latency_p95"`
	PatrolLatencyP95     string  `json:"patrol_latency_p95"`
	EndToEndLatencyP95   string  `json:"end_to_end_latency_p95"`
	InputTokensP95       int     `json:"input_tokens_p95"`
	OutputTokensP95      int     `json:"output_tokens_p95"`
	CostUSDP95           float64 `json:"cost_usd_p95"`
	MaxToolCalls         int     `json:"max_tool_calls"`
	MaxDuplicateCalls    int     `json:"max_duplicate_calls"`
}

type RepeatSpec struct {
	Development   int `json:"development"`
	Nightly       int `json:"nightly"`
	Qualification int `json:"qualification"`
}

type GateSpec struct {
	MinRecall                 float64 `json:"min_recall"`
	MaxFalsePositives         int     `json:"max_false_positives"`
	MinResourceAccuracy       float64 `json:"min_resource_accuracy"`
	MinCategoryAccuracy       float64 `json:"min_category_accuracy"`
	MinSeverityAccuracy       float64 `json:"min_severity_accuracy"`
	MinEvidenceGrounding      float64 `json:"min_evidence_grounding"`
	MaxFindingsPerCausalGroup float64 `json:"max_findings_per_causal_group"`
}

type TeardownSpec struct {
	Predicates           []Predicate `json:"predicates"`
	RequireSecondNoop    bool        `json:"require_second_cleanup_noop"`
	RequireInventorySame bool        `json:"require_inventory_unchanged"`
}

// EngagementSpec bounds how long a response scenario waits for Patrol to start
// by itself before the harness asks for a run. It never names the trigger the
// scenario expects: how the run started is an observation, read from the
// Patrol run record that first reported the finding.
type EngagementSpec struct {
	// AlertTimeout bounds the wait for Pulse to raise an alert on a scenario
	// resource after the fault is confirmed.
	AlertTimeout string `json:"alert_timeout"`
	// NaturalWakeTimeout bounds the wait for a finding nobody asked for.
	NaturalWakeTimeout string `json:"natural_wake_timeout"`
	// InvestigationStartTimeout bounds the wait for the investigation to
	// start by itself before the harness asks for it.
	InvestigationStartTimeout string `json:"investigation_start_timeout"`
	// Fallback is what the harness asks for once the natural wait is over.
	// scoped_run is the same request as Run Patrol on the affected resources.
	// When Pulse cannot resolve that selection the harness asks from the
	// firing alert, as its Investigate button does, and last for a full run.
	Fallback string `json:"fallback"`
}

// OutcomeSpec is the scenario-owned grading rubric of a response scenario.
// Every grade is decided from structured state and these terms. Nobody reads
// the conclusion to grade it.
type OutcomeSpec struct {
	// Expect is what a maintenance person would do: fixed (an action exists
	// and should be taken), handed_over (no action exists, the exact cause and
	// next step is the product) or quiet (nothing is wrong).
	Expect string `json:"expect"`
	// DecisiveTerms are the facts the stored conclusion needs before it is a
	// hand-over a person can act on. Each group is a set of accepted
	// spellings and every group has to match.
	DecisiveTerms [][]string `json:"decisive_terms,omitempty"`
	// AreaTerms mark a conclusion that found something the alert did not
	// already say, without reaching the decisive fact. Leave it empty when
	// the scenario has no such middle ground.
	AreaTerms [][]string `json:"area_terms,omitempty"`
	// MisdirectionTerms are statements the lab knows to be false. A
	// conclusion that contains one sends the user the wrong way and cannot be
	// a hand-over.
	MisdirectionTerms []string `json:"misdirection_terms,omitempty"`
}

const (
	OutcomeExpectFixed      = "fixed"
	OutcomeExpectHandedOver = "handed_over"
	OutcomeExpectQuiet      = "quiet"
)

var hostFilesystemPath = regexp.MustCompile(`^/(mnt|srv|media)/[a-z0-9][a-z0-9._-]{1,63}$`)

func (m Manifest) validateResource(resource ResourceSpec) []error {
	var errs []error
	switch resource.Kind {
	case resourceKindContainer:
		if resource.MountPath != "" {
			errs = append(errs, fmt.Errorf("resource %q: mount_path belongs to host_filesystem resources", resource.Alias))
		}
		if resource.Fixture != "" {
			if _, ok := fixtureByKey(resource.Fixture); !ok {
				errs = append(errs, fmt.Errorf("resource %q uses unknown fixture %q", resource.Alias, resource.Fixture))
			}
			if resource.Image != "" || len(resource.Command) > 0 || len(resource.Healthcheck) > 0 || resource.HealthEvery != "" || resource.FaultVolume || resource.ScratchStorage {
				errs = append(errs, fmt.Errorf("resource %q: a fixture owns its image, command, health check and mounts", resource.Alias))
			}
		} else {
			if len(resource.Env) > 0 || resource.Volume != "" {
				errs = append(errs, fmt.Errorf("resource %q: env and volume require a fixture", resource.Alias))
			}
			if resource.Image == "" && m.Lab.Image == "" {
				errs = append(errs, fmt.Errorf("resource %q has no image", resource.Alias))
			}
		}
		if resource.Volume != "" && !safeID.MatchString(resource.Volume) {
			errs = append(errs, fmt.Errorf("resource %q has an invalid volume name", resource.Alias))
		}
		// A fixture stands in for a real service, with a real image name and
		// usually an exact container name. Both are only safe on a Docker
		// daemon nothing else uses.
		if resource.Fixture != "" && m.Lab.SharedHostOK {
			errs = append(errs, fmt.Errorf("resource %q: fixtures need a private Docker daemon, set lab.shared_host_ok to false", resource.Alias))
		}
	case resourceKindHostFilesystem:
		if !hostFilesystemPath.MatchString(resource.MountPath) {
			errs = append(errs, fmt.Errorf("resource %q: mount_path must be a dedicated mount directly under /mnt, /srv or /media", resource.Alias))
		}
		if m.Lab.SharedHostOK {
			errs = append(errs, fmt.Errorf("resource %q: a host filesystem fixture needs a private lab host, set lab.shared_host_ok to false", resource.Alias))
		}
		if resource.Fixture != "" || resource.Image != "" || len(resource.Command) > 0 || len(resource.Healthcheck) > 0 || len(resource.Env) > 0 ||
			resource.Volume != "" || resource.Restart != "" || resource.FaultVolume || resource.ScratchStorage || len(resource.Labels) > 0 || resource.Name != "" {
			errs = append(errs, fmt.Errorf("resource %q: a host_filesystem resource declares only alias, kind and mount_path", resource.Alias))
		}
	default:
		errs = append(errs, fmt.Errorf("resource %q has unsupported kind %q", resource.Alias, resource.Kind))
	}
	return errs
}

func (m Manifest) validateResponse(aliases map[string]struct{}, kinds map[string]string) []error {
	if m.Track != TrackResponse {
		var errs []error
		if m.Engagement != nil || m.Outcome != nil {
			errs = append(errs, errors.New("only response scenarios may declare engagement or outcome"))
		}
		for _, kind := range kinds {
			if kind == resourceKindHostFilesystem {
				errs = append(errs, errors.New("host_filesystem resources are limited to response scenarios"))
				break
			}
		}
		return errs
	}
	var errs []error
	if m.Investigation != nil {
		errs = append(errs, errors.New("response scenarios describe the conclusion in outcome, not in investigation"))
	}
	if m.Engagement == nil {
		errs = append(errs, errors.New("response scenarios require engagement"))
	} else {
		for name, raw := range map[string]string{
			"engagement.alert_timeout":               m.Engagement.AlertTimeout,
			"engagement.natural_wake_timeout":        m.Engagement.NaturalWakeTimeout,
			"engagement.investigation_start_timeout": m.Engagement.InvestigationStartTimeout,
		} {
			if _, err := positiveDuration(raw); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", name, err))
			}
		}
		if m.Engagement.Fallback != "scoped_run" {
			errs = append(errs, fmt.Errorf("unsupported engagement.fallback %q", m.Engagement.Fallback))
		}
	}
	if m.Patrol.InvestigationTimeout == "" {
		errs = append(errs, errors.New("response scenarios require patrol.investigation_timeout"))
	} else if _, err := positiveDuration(m.Patrol.InvestigationTimeout); err != nil {
		errs = append(errs, fmt.Errorf("patrol.investigation_timeout: %w", err))
	}
	if !m.Patrol.Scoped {
		errs = append(errs, errors.New("response scenarios require patrol.scoped=true for the fallback request"))
	}
	if m.Outcome == nil {
		return append(errs, errors.New("response scenarios require outcome"))
	}
	checkGroups := func(label string, groups [][]string) {
		for groupIndex, group := range groups {
			if len(group) == 0 {
				errs = append(errs, fmt.Errorf("outcome.%s[%d] must not be empty", label, groupIndex))
			}
			for termIndex, term := range group {
				if strings.TrimSpace(term) == "" {
					errs = append(errs, fmt.Errorf("outcome.%s[%d][%d] must not be empty", label, groupIndex, termIndex))
				}
			}
		}
	}
	checkGroups("decisive_terms", m.Outcome.DecisiveTerms)
	checkGroups("area_terms", m.Outcome.AreaTerms)
	for index, term := range m.Outcome.MisdirectionTerms {
		if strings.TrimSpace(term) == "" {
			errs = append(errs, fmt.Errorf("outcome.misdirection_terms[%d] must not be empty", index))
		}
	}
	switch m.Outcome.Expect {
	case OutcomeExpectQuiet:
		if len(m.Faults) != 0 || len(m.NegativeControls) == 0 {
			errs = append(errs, errors.New("a quiet response scenario declares no fault and at least one negative control"))
		}
		if m.Remediation != nil || len(m.Outcome.DecisiveTerms) > 0 || len(m.Outcome.AreaTerms) > 0 {
			errs = append(errs, errors.New("a quiet response scenario has nothing to fix or hand over"))
		}
	case OutcomeExpectFixed, OutcomeExpectHandedOver:
		if len(m.Faults) == 0 {
			errs = append(errs, errors.New("a response scenario that expects work declares at least one fault"))
		}
		if len(m.Outcome.DecisiveTerms) == 0 {
			errs = append(errs, errors.New("outcome.decisive_terms is required: without it a hand-over cannot be told from a restated alert"))
		}
		if m.Outcome.Expect == OutcomeExpectFixed {
			if m.Remediation == nil || m.Remediation.Decision != "approve_execute" {
				errs = append(errs, errors.New("outcome.expect=fixed requires remediation with decision approve_execute"))
			}
		} else if m.Remediation != nil {
			errs = append(errs, errors.New("outcome.expect=handed_over means no action exists: remove remediation"))
		}
	default:
		errs = append(errs, fmt.Errorf("unsupported outcome.expect %q", m.Outcome.Expect))
	}
	_ = aliases
	return errs
}

// outcomeResources lists the aliases whose findings count as Patrol looking
// at the scenario: every fault target, the resource the finding is expected
// on, the related resources, and for a quiet scenario the negative controls.
func (m Manifest) outcomeResources() []string {
	seen := make(map[string]struct{})
	var aliases []string
	add := func(alias string) {
		if alias == "" {
			return
		}
		if _, ok := seen[alias]; ok {
			return
		}
		seen[alias] = struct{}{}
		aliases = append(aliases, alias)
	}
	for _, fault := range m.Faults {
		add(fault.Target)
		add(fault.Expected.Resource)
		for _, related := range fault.RelatedResources {
			add(related)
		}
	}
	if len(m.Faults) == 0 {
		for _, control := range m.NegativeControls {
			add(control.Resource)
		}
	}
	sort.Strings(aliases)
	return aliases
}

func (m Manifest) resource(alias string) (ResourceSpec, bool) {
	for _, resource := range m.Resources {
		if resource.Alias == alias {
			return resource, true
		}
	}
	return ResourceSpec{}, false
}

type Catalog struct {
	Manifests []Manifest
	ByID      map[string]Manifest
}

func LoadManifest(path string) (Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var manifest Manifest
	if err := dec.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode %s: %w", path, err)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Manifest{}, fmt.Errorf("decode %s: trailing JSON value", path)
		}
		return Manifest{}, fmt.Errorf("decode %s trailing data: %w", path, err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, fmt.Errorf("validate %s: %w", path, err)
	}
	return manifest, nil
}

func LoadCatalog(dir string) (Catalog, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Catalog{}, err
	}
	catalog := Catalog{ByID: make(map[string]Manifest)}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || strings.HasSuffix(entry.Name(), ".schema.json") {
			continue
		}
		manifest, err := LoadManifest(filepath.Join(dir, entry.Name()))
		if err != nil {
			return Catalog{}, err
		}
		if _, exists := catalog.ByID[manifest.ID]; exists {
			return Catalog{}, fmt.Errorf("duplicate scenario id %q", manifest.ID)
		}
		catalog.ByID[manifest.ID] = manifest
		catalog.Manifests = append(catalog.Manifests, manifest)
	}
	sort.Slice(catalog.Manifests, func(i, j int) bool { return catalog.Manifests[i].ID < catalog.Manifests[j].ID })
	if len(catalog.Manifests) == 0 {
		return Catalog{}, errors.New("catalog contains no scenario manifests")
	}
	return catalog, nil
}

func (m Manifest) Validate() error {
	var errs []error
	if m.SchemaVersion != SchemaVersion {
		errs = append(errs, fmt.Errorf("schema_version must be %q", SchemaVersion))
	}
	if !safeID.MatchString(m.ID) {
		errs = append(errs, errors.New("id must be 3-96 lowercase identifier characters"))
	}
	if m.Version < 1 {
		errs = append(errs, errors.New("version must be positive"))
	}
	if strings.TrimSpace(m.Title) == "" || strings.TrimSpace(m.Description) == "" || strings.TrimSpace(m.Owner) == "" {
		errs = append(errs, errors.New("title, description, and owner are required"))
	}
	if m.Track != TrackWatch && m.Track != TrackInvestigation && m.Track != TrackRemediation && m.Track != TrackResponse {
		errs = append(errs, fmt.Errorf("unsupported track %q", m.Track))
	}
	if m.Lab.Driver != "docker" && m.Lab.Driver != "replay" {
		errs = append(errs, fmt.Errorf("unsupported lab driver %q", m.Lab.Driver))
	}
	aliases := make(map[string]struct{}, len(m.Resources))
	scratchResources := make(map[string]bool, len(m.Resources))
	resourceKinds := make(map[string]string, len(m.Resources))
	resourceByAlias := make(map[string]ResourceSpec, len(m.Resources))
	if len(m.Resources) == 0 {
		errs = append(errs, errors.New("at least one disposable resource is required"))
	}
	for i, resource := range m.Resources {
		if !safeID.MatchString(resource.Alias) {
			errs = append(errs, fmt.Errorf("resources[%d].alias is invalid", i))
		}
		if _, exists := aliases[resource.Alias]; exists {
			errs = append(errs, fmt.Errorf("duplicate resource alias %q", resource.Alias))
		}
		aliases[resource.Alias] = struct{}{}
		scratchResources[resource.Alias] = resource.ScratchStorage
		resourceKinds[resource.Alias] = resource.Kind
		resourceByAlias[resource.Alias] = resource
		errs = append(errs, m.validateResource(resource)...)
	}
	checkPredicates := func(label string, predicates []Predicate) []error {
		problems := validatePredicates(label, predicates, aliases)
		for _, predicate := range predicates {
			if predicate.Probe == "docker.scratch_available_bytes" && !scratchResources[predicate.Target] {
				problems = append(problems, fmt.Errorf("%s scratch probe requires scratch_storage on %q", label, predicate.Target))
			}
			kind, known := resourceKinds[predicate.Target]
			if !known {
				continue
			}
			hostProbe := strings.HasPrefix(predicate.Probe, "host.")
			dockerProbe := strings.HasPrefix(predicate.Probe, "docker.")
			if hostProbe && kind != resourceKindHostFilesystem {
				problems = append(problems, fmt.Errorf("%s probe %s requires a host_filesystem resource, %q is a %s", label, predicate.Probe, predicate.Target, kind))
			}
			if dockerProbe && kind != resourceKindContainer {
				problems = append(problems, fmt.Errorf("%s probe %s requires a container resource, %q is a %s", label, predicate.Probe, predicate.Target, kind))
			}
		}
		return problems
	}
	faultIDs := make(map[string]struct{}, len(m.Faults))
	for i, fault := range m.Faults {
		if !safeID.MatchString(fault.ID) {
			errs = append(errs, fmt.Errorf("faults[%d].id is invalid", i))
		}
		if _, exists := faultIDs[fault.ID]; exists {
			errs = append(errs, fmt.Errorf("duplicate fault id %q", fault.ID))
		}
		faultIDs[fault.ID] = struct{}{}
		if _, ok := aliases[fault.Target]; !ok {
			errs = append(errs, fmt.Errorf("fault %q targets unknown resource %q", fault.ID, fault.Target))
		}
		if _, ok := aliases[fault.Injector.Resource]; !ok {
			errs = append(errs, fmt.Errorf("fault %q injector targets unknown resource %q", fault.ID, fault.Injector.Resource))
		}
		switch fault.Injector.Kind {
		case "fill_scratch_storage":
			if !scratchResources[fault.Injector.Resource] {
				errs = append(errs, fmt.Errorf("fault %q requires scratch_storage", fault.ID))
			}
			if fault.Injector.Value != "" {
				errs = append(errs, fmt.Errorf("fault %q uses a fixed storage bound and accepts no value", fault.ID))
			}
		case "marker_enable", "health_process_stop", "stop", "disconnect_network", "kill":
			if kind, ok := resourceKinds[fault.Injector.Resource]; ok && kind != resourceKindContainer {
				errs = append(errs, fmt.Errorf("fault %q injector %q requires a container resource", fault.ID, fault.Injector.Kind))
			}
		case "service_process_exit", "crash_once":
			// Both run one fixed command the fixture declares. A fixture
			// without that command cannot take the fault.
			fixture, ok := fixtureByKey(resourceByAlias[fault.Injector.Resource].Fixture)
			if !ok || (fault.Injector.Kind == "service_process_exit" && len(fixture.ServiceProcessExit) == 0) ||
				(fault.Injector.Kind == "crash_once" && len(fixture.CrashOnce) == 0) {
				errs = append(errs, fmt.Errorf("fault %q injector %q is not supported by the fixture of resource %q", fault.ID, fault.Injector.Kind, fault.Injector.Resource))
			}
			if fault.Injector.Value != "" {
				errs = append(errs, fmt.Errorf("fault %q injector %q accepts no value", fault.ID, fault.Injector.Kind))
			}
		case "recreate_with_env":
			key, _, ok := strings.Cut(fault.Injector.Value, "=")
			if _, declared := resourceByAlias[fault.Injector.Resource].Env[key]; !ok || !declared {
				errs = append(errs, fmt.Errorf("fault %q recreate_with_env value must be KEY=VALUE for an env key the resource already declares", fault.ID))
			}
			if resourceByAlias[fault.Injector.Resource].Fixture == "" {
				errs = append(errs, fmt.Errorf("fault %q recreate_with_env requires a fixture resource", fault.ID))
			}
		case "fill_host_filesystem":
			if resourceKinds[fault.Injector.Resource] != resourceKindHostFilesystem {
				errs = append(errs, fmt.Errorf("fault %q fill_host_filesystem requires a host_filesystem resource", fault.ID))
			}
			if fault.Injector.Value != "" {
				errs = append(errs, fmt.Errorf("fault %q uses a driver-owned fill layout and accepts no value", fault.ID))
			}
		default:
			errs = append(errs, fmt.Errorf("fault %q has unsupported injector %q", fault.ID, fault.Injector.Kind))
		}
		if fault.CausalGroup == "" {
			errs = append(errs, fmt.Errorf("fault %q has no causal_group", fault.ID))
		}
		for _, related := range fault.RelatedResources {
			if _, ok := aliases[related]; !ok {
				errs = append(errs, fmt.Errorf("fault %q references unknown related resource %q", fault.ID, related))
			}
			if related == fault.Target {
				errs = append(errs, fmt.Errorf("fault %q related resource duplicates its target %q", fault.ID, related))
			}
		}
		if len(fault.Oracle) == 0 {
			errs = append(errs, fmt.Errorf("fault %q has no independent oracle", fault.ID))
		}
		if !fault.Required {
			errs = append(errs, fmt.Errorf("fault %q must be required for qualification", fault.ID))
		}
		if len(fault.Expected.ResourceTypes) == 0 || len(fault.Expected.Categories) == 0 || len(fault.Expected.Severities) == 0 {
			errs = append(errs, fmt.Errorf("fault %q has incomplete expected finding semantics", fault.ID))
		}
		if _, ok := aliases[fault.Expected.Resource]; !ok {
			errs = append(errs, fmt.Errorf("fault %q expected finding references unknown resource %q", fault.ID, fault.Expected.Resource))
		} else if fault.Expected.Resource != fault.Target && !contains(fault.RelatedResources, fault.Expected.Resource) {
			errs = append(errs, fmt.Errorf("fault %q expected finding resource %q must equal target %q or a declared related resource", fault.ID, fault.Expected.Resource, fault.Target))
		}
		if fault.DetectWithin != "" {
			if _, err := positiveDuration(fault.DetectWithin); err != nil {
				errs = append(errs, fmt.Errorf("fault %q detect_within: %w", fault.ID, err))
			}
		}
		errs = append(errs, checkPredicates("fault "+fault.ID+" oracle", fault.Oracle)...)
		errs = append(errs, checkPredicates("fault "+fault.ID+" revert_oracle", fault.RevertOracle)...)
		if fault.Expected.MaxPrimaryFindings < 1 {
			errs = append(errs, fmt.Errorf("fault %q max_primary_findings must be positive", fault.ID))
		}
	}
	if len(m.Baseline) == 0 {
		errs = append(errs, errors.New("baseline predicates are required"))
	}
	errs = append(errs, checkPredicates("baseline", m.Baseline)...)
	errs = append(errs, checkPredicates("teardown", m.Teardown.Predicates)...)
	for _, control := range m.NegativeControls {
		if _, ok := aliases[control.Resource]; !ok {
			errs = append(errs, fmt.Errorf("negative control references unknown resource %q", control.Resource))
		}
	}
	if len(m.Patrol.ScopeResources) > 0 && !m.Patrol.Scoped {
		errs = append(errs, errors.New("patrol.scope_resources requires patrol.scoped=true"))
	}
	seenScopeResources := make(map[string]struct{}, len(m.Patrol.ScopeResources))
	for _, alias := range m.Patrol.ScopeResources {
		if _, ok := aliases[alias]; !ok {
			errs = append(errs, fmt.Errorf("patrol.scope_resources references unknown resource %q", alias))
		}
		if _, duplicate := seenScopeResources[alias]; duplicate {
			errs = append(errs, fmt.Errorf("patrol.scope_resources duplicates resource %q", alias))
		}
		seenScopeResources[alias] = struct{}{}
	}
	if m.Track == TrackInvestigation || m.Track == TrackRemediation {
		if m.Investigation == nil {
			errs = append(errs, errors.New("investigation expectations are required for Pro tracks"))
		} else {
			if m.Investigation.minimumEvidenceCalls() < 1 {
				errs = append(errs, errors.New("investigation.min_evidence_calls must be positive"))
			}
			if len(m.Investigation.RequiredSummaryTerms) == 0 && len(m.Investigation.RequiredSummaryTermGroups) == 0 {
				errs = append(errs, errors.New("investigation summary expectations must include required_summary_terms or required_summary_term_groups"))
			}
			for groupIndex, group := range m.Investigation.RequiredSummaryTermGroups {
				if len(group) == 0 {
					errs = append(errs, fmt.Errorf("investigation.required_summary_term_groups[%d] must not be empty", groupIndex))
					continue
				}
				for termIndex, term := range group {
					if strings.TrimSpace(term) == "" {
						errs = append(errs, fmt.Errorf("investigation.required_summary_term_groups[%d][%d] must not be empty", groupIndex, termIndex))
					}
				}
			}
			for _, alias := range append(append([]string(nil), m.Investigation.RootCauseResources...), m.Investigation.AffectedResources...) {
				if _, ok := aliases[alias]; !ok {
					errs = append(errs, fmt.Errorf("investigation resource expectation references unknown resource %q", alias))
				}
			}
		}
	} else if m.Investigation != nil && m.Track != TrackResponse {
		errs = append(errs, errors.New("Watch scenarios must not declare investigation expectations"))
	}
	errs = append(errs, m.validateResponse(aliases, resourceKinds)...)
	if m.Track == TrackRemediation || (m.Track == TrackResponse && m.Remediation != nil) {
		if m.Remediation == nil {
			errs = append(errs, errors.New("remediation expectations are required for remediation track"))
		} else {
			if _, ok := aliases[m.Remediation.ActionTarget]; !ok {
				errs = append(errs, fmt.Errorf("remediation.action_target references unknown resource %q", m.Remediation.ActionTarget))
			}
			if len(m.Remediation.ExpectedCapabilities) == 0 {
				errs = append(errs, errors.New("remediation.expected_capabilities must not be empty"))
			}
			switch m.Remediation.Decision {
			case "observe", "reject", "approve_execute", "await_autonomous":
			default:
				errs = append(errs, fmt.Errorf("unsupported remediation decision %q", m.Remediation.Decision))
			}
			if _, err := positiveDuration(m.Remediation.ActionTimeout); err != nil {
				errs = append(errs, fmt.Errorf("remediation.action_timeout: %w", err))
			}
			if m.Remediation.Decision != "observe" && len(m.Remediation.Postconditions) == 0 {
				errs = append(errs, errors.New("remediation decisions require independent postconditions"))
			}
			if m.Remediation.Decision != "observe" && strings.TrimSpace(m.Remediation.DecisionReason) == "" {
				errs = append(errs, errors.New("remediation decisions require a reason"))
			}
			if m.Remediation.RequireLifecycleVerification && len(m.Remediation.AllowedVerificationStatuses) == 0 {
				errs = append(errs, errors.New("required lifecycle verification needs allowed statuses"))
			}
			errs = append(errs, checkPredicates("remediation postconditions", m.Remediation.Postconditions)...)
		}
	} else if m.Remediation != nil {
		errs = append(errs, errors.New("only remediation-track scenarios may declare remediation expectations"))
	}
	for name, raw := range map[string]string{
		"collection.convergence_timeout": m.Collection.ConvergenceTimeout,
		"collection.poll_interval":       m.Collection.PollInterval,
		"patrol.run_timeout":             m.Patrol.RunTimeout,
		"budgets.collection_latency_p95": m.Budgets.CollectionLatencyP95,
		"budgets.patrol_latency_p95":     m.Budgets.PatrolLatencyP95,
		"budgets.end_to_end_latency_p95": m.Budgets.EndToEndLatencyP95,
	} {
		if _, err := positiveDuration(raw); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}
	if m.Patrol.Mode != "monitor" && m.Patrol.Mode != "approval" && m.Patrol.Mode != "autonomous" {
		errs = append(errs, fmt.Errorf("unsupported Patrol mode %q", m.Patrol.Mode))
	}
	if len(m.Collection.Sources) == 0 {
		errs = append(errs, errors.New("collection.sources must not be empty"))
	}
	if !m.Collection.RequireExactName {
		errs = append(errs, errors.New("collection.require_exact_name must be true for qualification"))
	}
	if !m.Patrol.RequireRealModel {
		errs = append(errs, errors.New("patrol.require_real_model must be true for qualification"))
	}
	if !m.Patrol.RequireToolCallEvidence && !m.permitsToolFreeAllClear() && m.Track != TrackResponse {
		errs = append(errs, errors.New("tool-free Patrol qualification is limited to fault-free Watch scenarios with a declared negative control"))
	}
	if !m.Security.RequireNoMutation {
		errs = append(errs, errors.New("security.require_no_unexpected_mutation must be true for qualification"))
	}
	if len(m.Faults) > 0 && !m.Security.RequireFaultIntact &&
		!(m.Track == TrackRemediation && m.Patrol.Mode == "autonomous" && m.Remediation != nil && m.Remediation.Decision == "await_autonomous") {
		errs = append(errs, errors.New("security.require_fault_intact_after_patrol must be true when faults are declared"))
	}
	if !m.Teardown.RequireSecondNoop || !m.Teardown.RequireInventorySame {
		errs = append(errs, errors.New("teardown must require a second cleanup no-op and unchanged inventory"))
	}
	if m.Repeat.Development < 1 || m.Repeat.Nightly < 1 || m.Repeat.Qualification < 1 {
		errs = append(errs, errors.New("repeat counts must all be positive"))
	}
	if m.Repeat.Qualification < m.Repeat.Nightly || m.Repeat.Nightly < m.Repeat.Development {
		errs = append(errs, errors.New("repeat counts must satisfy qualification >= nightly >= development"))
	}
	if m.Repeat.Qualification > 0 {
		perfectLower := WilsonInterval(m.Repeat.Qualification, m.Repeat.Qualification).Lower
		if perfectLower < minimumQualificationWilsonLower {
			errs = append(errs, fmt.Errorf(
				"repeat.qualification=%d cannot satisfy the %.3f Wilson lower-bound gate even with a perfect run; at least %d repeats are required",
				m.Repeat.Qualification,
				minimumQualificationWilsonLower,
				minimumPerfectQualificationRuns(),
			))
		}
	}
	if m.Patrol.RequireRealModel && m.Lab.Driver == "replay" {
		errs = append(errs, errors.New("a replay-only lab cannot require a real model"))
	}
	return errors.Join(errs...)
}

// permitsToolFreeAllClear identifies the one qualification shape where the
// absence of tool calls is the desired product outcome. A faulted,
// investigation, remediation, or existing-finding scenario still needs
// persisted tool evidence. A Watch negative control instead proves that the
// real model can inspect a healthy scoped resource and correctly do nothing.
func (m Manifest) permitsToolFreeAllClear() bool {
	return m.Track == TrackWatch &&
		len(m.Faults) == 0 &&
		len(m.NegativeControls) > 0 &&
		!m.Patrol.RequireExistingReconfirmation
}

func validatePredicates(label string, predicates []Predicate, aliases map[string]struct{}) []error {
	var errs []error
	for index, predicate := range predicates {
		if _, ok := aliases[predicate.Target]; !ok {
			errs = append(errs, fmt.Errorf("%s[%d] targets unknown resource %q", label, index, predicate.Target))
		}
		switch predicate.Probe {
		case "inventory.same_as_pre", "docker.exists", "docker.status", "docker.running", "docker.health", "docker.restart_count", "docker.exit_code", "docker.network_attached", "docker.scratch_available_bytes", "host.filesystem_used_percent":
		default:
			errs = append(errs, fmt.Errorf("%s[%d] has unsupported probe %q", label, index, predicate.Probe))
		}
		switch predicate.Operator {
		case "eq", "not_eq", "gte", "lte", "gt", "lt", "in":
		default:
			errs = append(errs, fmt.Errorf("%s[%d] has unsupported operator %q", label, index, predicate.Operator))
		}
		if len(predicate.Value) == 0 || !json.Valid(predicate.Value) {
			errs = append(errs, fmt.Errorf("%s[%d] has invalid value", label, index))
		}
		if predicate.Timeout != "" {
			if _, err := positiveDuration(predicate.Timeout); err != nil {
				errs = append(errs, fmt.Errorf("%s[%d] timeout: %w", label, index, err))
			}
		}
	}
	return errs
}

func (m Manifest) Digest() (string, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func positiveDuration(value string) (time.Duration, error) {
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, errors.New("duration must be positive")
	}
	return d, nil
}
