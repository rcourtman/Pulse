package qualification

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

type RunnerConfig struct {
	Manifest             Manifest
	Lab                  *DockerLab
	Client               *PulseClient
	ArtifactRoot         string
	RunID                string
	ModelOverride        string
	GitSHA               string
	GitDirty             bool
	ExpectedPulseVersion string
	ChallengeNonce       string
	// AuthorizeRemediation is a second, independent operator gate. Live lab
	// authorization alone never permits an action decision or execution.
	AuthorizeRemediation bool
}

type QualificationRunner struct {
	config RunnerConfig
}

func NewRunner(config RunnerConfig) (*QualificationRunner, error) {
	if err := config.Manifest.Validate(); err != nil {
		return nil, err
	}
	if config.Manifest.Lab.Driver == "docker" && config.Lab == nil {
		return nil, errors.New("Docker lab is required")
	}
	if config.Client == nil {
		return nil, errors.New("Pulse client is required")
	}
	if config.Manifest.Track == TrackRemediation && config.Manifest.Remediation != nil &&
		config.Manifest.Remediation.Decision != "observe" && !config.AuthorizeRemediation {
		return nil, errors.New("remediation decision or execution requires explicit authorization")
	}
	if config.RunID == "" {
		config.RunID = newRunID()
	}
	config.ChallengeNonce = strings.TrimSpace(config.ChallengeNonce)
	if err := ValidateContributionChallenge(config.ChallengeNonce); err != nil {
		return nil, err
	}
	if !safeID.MatchString(config.RunID) {
		return nil, errors.New("invalid run id")
	}
	if config.ArtifactRoot == "" {
		config.ArtifactRoot = filepath.Join("tmp", "patrol-qualification")
	}
	return &QualificationRunner{config: config}, nil
}

func newRunID() string {
	var suffix [4]byte
	_, _ = rand.Read(suffix[:])
	return "q-" + time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(suffix[:])
}

func (r *QualificationRunner) Run(ctx context.Context) (report RunReport, terminalErr error) {
	manifest := r.config.Manifest
	digest, err := manifest.Digest()
	if err != nil {
		return RunReport{}, err
	}
	report = RunReport{
		SchemaVersion: ReportSchemaVersion,
		RunID:         r.config.RunID,
		GeneratedAt:   time.Now().UTC(),
		Manifest:      manifest,
		Environment:   r.initialEnvironment(),
		Collected:     make(map[string]CollectedTruth),
		Investigation: make(map[string]aicontracts.InvestigationSession),
	}
	artifactDir := filepath.Join(r.config.ArtifactRoot, r.config.RunID)
	var prepared *PreparedLab
	var restoreModel func(context.Context) error
	finish := func() {
		if restoreModel != nil {
			restoreCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if restoreErr := restoreModel(restoreCtx); restoreErr != nil {
				report.Errors = append(report.Errors, "restore Patrol model: "+restoreErr.Error())
			}
		}
		if prepared != nil {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			report.Teardown = r.config.Lab.Cleanup(cleanupCtx, manifest, prepared)
			if !report.Teardown.Passed {
				report.Errors = append(report.Errors, "lab teardown verification failed")
			}
		}
		report.Passed = report.Score.Passed && report.Teardown.Passed && len(report.Errors) == 0
		if writeErr := WriteReport(artifactDir, report); writeErr != nil {
			terminalErr = errors.Join(terminalErr, fmt.Errorf("write report: %w", writeErr))
		}
	}
	defer finish()

	if err := r.phase(&report, "preflight", func() error {
		settings, settingsErr := r.config.Client.Settings(ctx)
		if settingsErr != nil {
			return settingsErr
		}
		status, statusErr := r.config.Client.Status(ctx)
		if statusErr != nil {
			return statusErr
		}
		version, versionErr := r.config.Client.Version(ctx)
		if versionErr != nil {
			return versionErr
		}
		if strings.TrimSpace(version.Version) == "" {
			return errors.New("Pulse runtime version endpoint returned no version identity")
		}
		if expected := strings.TrimSpace(r.config.ExpectedPulseVersion); expected != "" && version.Version != expected {
			return fmt.Errorf("Pulse runtime version %q does not match required version %q", version.Version, expected)
		}
		if manifest.Patrol.RequireRealModel {
			if !settings.Enabled || !settings.PatrolEnabled || !status.Readiness.Ready {
				return fmt.Errorf("Patrol real-model readiness failed: %s", status.Readiness.Summary)
			}
			if status.Readiness.Provider == "" || status.Readiness.Model == "" || strings.EqualFold(status.Readiness.Provider, "demo") {
				return errors.New("Patrol readiness did not identify a real provider and model")
			}
		}
		autonomy, autonomyErr := r.config.Client.Autonomy(ctx)
		if autonomyErr != nil {
			return autonomyErr
		}
		if expected := strings.TrimSpace(manifest.Patrol.Mode); expected != "" && !strings.EqualFold(expected, autonomy.Effective()) {
			return fmt.Errorf("scenario requires Patrol mode %q, effective mode is %q", expected, autonomy.Effective())
		}
		var overrideErr error
		restoreModel, overrideErr = r.config.Client.OverridePatrolModel(ctx, r.config.ModelOverride)
		if overrideErr != nil {
			return overrideErr
		}
		if r.config.ModelOverride != "" {
			settings, settingsErr = r.config.Client.Settings(ctx)
			if settingsErr != nil {
				return settingsErr
			}
			status, statusErr = r.config.Client.Status(ctx)
			if statusErr != nil {
				return statusErr
			}
			if !status.Readiness.Ready || strings.EqualFold(status.Readiness.Provider, "demo") {
				return fmt.Errorf("overridden Patrol model is not ready: %s", status.Readiness.Summary)
			}
		}
		model := settings.EffectivePatrolModel()
		provider := strings.ToLower(strings.TrimSpace(status.Readiness.Provider))
		if provider == "" {
			provider, _ = splitModel(model)
		}
		report.Environment = Environment{
			GitSHA: r.config.GitSHA, GitDirty: r.config.GitDirty,
			PulseVersion: version.Version,
			PulseBaseURL: r.config.Client.config.BaseURL,
			DockerTarget: dockerTargetLabel(r.config.Lab.target),
			Model:        model, Provider: provider, InferenceRoute: inferenceRouteForProvider(provider), ChallengeNonce: r.config.ChallengeNonce,
			CapturedAt: time.Now().UTC(),
		}
		return nil
	}); err != nil {
		report.Errors = append(report.Errors, err.Error())
		terminalErr = err
		return report, terminalErr
	}

	if err := r.phase(&report, "provision_and_baseline", func() error {
		var prepareErr error
		prepared, prepareErr = r.config.Lab.Prepare(ctx, manifest, r.config.RunID)
		report.PreparedLab = prepared
		if prepareErr != nil {
			return prepareErr
		}
		baseline, observeErr := r.config.Lab.Observe(ctx, manifest, prepared, manifest.Baseline)
		if observeErr != nil {
			return observeErr
		}
		report.GroundTruth = GroundTruth{
			SchemaVersion: "patrol.qualification.ground-truth/v2",
			ManifestID:    manifest.ID, ManifestDigest: digest, RunID: r.config.RunID,
			CreatedAt: time.Now().UTC(), Baseline: baseline,
			Resources: make(map[string]CollectedTruth),
		}
		return nil
	}); err != nil {
		report.Errors = append(report.Errors, err.Error())
		terminalErr = err
		return report, terminalErr
	}

	faultStarted := time.Now().UTC()
	if err := r.phase(&report, "inject_and_confirm_ground_truth", func() error {
		for _, fault := range manifest.Faults {
			if err := r.config.Lab.ApplyFault(ctx, manifest, prepared, fault); err != nil {
				return fmt.Errorf("apply fault %s: %w", fault.ID, err)
			}
			observations, err := r.config.Lab.Observe(ctx, manifest, prepared, fault.Oracle)
			truth := FaultTruth{
				ID: fault.ID, CausalGroup: fault.CausalGroup, TargetAlias: fault.Target,
				TargetName: prepared.ResourceNames[fault.Target], Active: err == nil && allObservationsPassed(observations),
				ConfirmedAt: time.Now().UTC(), Observations: observations,
			}
			report.GroundTruth.Faults = append(report.GroundTruth.Faults, truth)
			if err != nil {
				return fmt.Errorf("confirm fault %s: %w", fault.ID, err)
			}
		}
		for _, control := range manifest.NegativeControls {
			report.GroundTruth.Negative = append(report.GroundTruth.Negative, NegativeTruth{Alias: control.Resource, Name: prepared.ResourceNames[control.Resource], Reason: control.Reason})
		}
		return nil
	}); err != nil {
		report.Errors = append(report.Errors, err.Error())
		terminalErr = err
		return report, terminalErr
	}

	var collected map[string]Resource
	if err := r.phase(&report, "normal_collection_convergence", func() error {
		timeout, _ := positiveDuration(manifest.Collection.ConvergenceTimeout)
		poll, _ := positiveDuration(manifest.Collection.PollInterval)
		var collectionErr error
		collected, collectionErr = r.config.Client.WaitForResourcesMatching(ctx, prepared.ResourceNames, timeout, poll, func(resources map[string]Resource) error {
			return validateCollectedScenarioProjection(manifest, resources)
		})
		if collectionErr != nil {
			return collectionErr
		}
		for alias, resource := range collected {
			truth := CollectedTruth{Alias: alias, Name: resource.Name, ResourceID: resource.ID, ResourceType: resource.Type, Status: resource.Status, ObservedAt: time.Now().UTC()}
			report.Collected[alias] = truth
			report.GroundTruth.Resources[alias] = truth
		}
		for i := range report.GroundTruth.Faults {
			truth := &report.GroundTruth.Faults[i]
			resource := collected[truth.TargetAlias]
			truth.ResourceID, truth.ResourceType = resource.ID, resource.Type
			fault := findFault(manifest.Faults, truth.ID)
			expected := collected[fault.Expected.Resource]
			truth.ExpectedResourceAlias = fault.Expected.Resource
			truth.ExpectedResourceName = expected.Name
			truth.ExpectedResourceID = expected.ID
			truth.ExpectedResourceType = expected.Type
			for _, alias := range fault.RelatedResources {
				truth.RelatedResourceIDs = append(truth.RelatedResourceIDs, collected[alias].ID)
			}
		}
		for i := range report.GroundTruth.Negative {
			truth := &report.GroundTruth.Negative[i]
			truth.ResourceID = collected[truth.Alias].ID
		}
		return nil
	}); err != nil {
		report.Errors = append(report.Errors, err.Error())
		terminalErr = err
		return report, terminalErr
	}

	var beforeFindings, afterFindings []Finding
	triggeredAt := time.Now().UTC()
	if err := r.phase(&report, "real_model_patrol", func() error {
		var err error
		beforeFindings, err = r.config.Client.Findings(ctx)
		if err != nil {
			return err
		}
		resourceIDs := make([]string, 0, len(collected))
		if manifest.Patrol.Scoped {
			for _, resource := range collected {
				resourceIDs = append(resourceIDs, resource.ID)
			}
			sort.Strings(resourceIDs)
		}
		if manifest.Patrol.RequireExistingReconfirmation {
			warmupTriggeredAt := time.Now().UTC()
			runTimeout, _ := positiveDuration(manifest.Patrol.RunTimeout)
			warmup, warmupErr := r.config.Client.TriggerAndWait(ctx, resourceIDs, "", runTimeout)
			if warmupErr != nil {
				return fmt.Errorf("existing-finding prerequisite Patrol run: %w", warmupErr)
			}
			afterWarmup, findingsErr := r.config.Client.Findings(ctx)
			if findingsErr != nil {
				return findingsErr
			}
			warmupFindings := filterRunFindings(beforeFindings, afterWarmup, warmup, collected, warmupTriggeredAt)
			if len(warmupFindings) == 0 {
				return errors.New("existing-finding prerequisite run did not create a run-owned finding")
			}
			report.PrerequisitePatrolRun = &warmup
			beforeFindings = afterWarmup
		}
		triggeredAt = time.Now().UTC()
		runTimeout, _ := positiveDuration(manifest.Patrol.RunTimeout)
		report.PatrolRun, err = r.config.Client.TriggerAndWait(ctx, resourceIDs, "", runTimeout)
		if err != nil {
			return err
		}
		if manifest.Patrol.RequireToolCallEvidence && len(report.PatrolRun.ToolCalls) == 0 {
			return errors.New("completed Patrol run has no persisted tool-call evidence")
		}
		if manifest.Patrol.RequireExistingReconfirmation {
			if report.PatrolRun.ExistingFindings == 0 {
				return errors.New("qualification run did not reconfirm an existing finding")
			}
			if len(report.PatrolRun.FindingAssessments) == 0 {
				return errors.New("qualification run reconfirmed an existing finding without a persisted assessment")
			}
		}
		afterFindings, err = r.config.Client.Findings(ctx)
		if err != nil {
			return err
		}
		report.Findings = filterRunFindings(beforeFindings, afterFindings, report.PatrolRun, collected, triggeredAt)
		return nil
	}); err != nil {
		report.Errors = append(report.Errors, err.Error())
		terminalErr = err
		return report, terminalErr
	}

	faultsIntact, noMutation := true, true
	if err := r.phase(&report, "post_patrol_safety_oracle", func() error {
		for _, fault := range manifest.Faults {
			observations, err := r.config.Lab.Observe(ctx, manifest, prepared, fault.Oracle)
			report.PostPatrol = append(report.PostPatrol, observations...)
			if err != nil {
				faultsIntact = false
				if manifest.Security.RequireFaultIntact {
					return fmt.Errorf("fault %s did not remain intact: %w", fault.ID, err)
				}
			}
		}
		current, err := r.config.Lab.inventory(ctx)
		if err != nil {
			return err
		}
		noMutation = inventoryEqual(current, prepared.ExpectedInventory)
		if manifest.Security.RequireNoMutation && !noMutation {
			return errors.New("Docker inventory changed during Watch/investigation run")
		}
		return nil
	}); err != nil {
		report.Errors = append(report.Errors, err.Error())
	}

	if manifest.Track == TrackInvestigation || manifest.Track == TrackRemediation {
		if err := r.phase(&report, "investigation", func() error {
			timeout := 10 * time.Minute
			if manifest.Patrol.InvestigationTimeout != "" {
				timeout, _ = positiveDuration(manifest.Patrol.InvestigationTimeout)
			}
			if len(report.Findings) == 0 {
				return errors.New("investigation track produced no finding to investigate")
			}
			for _, finding := range report.Findings {
				investigation, err := r.config.Client.WaitForInvestigation(ctx, finding.ID, timeout)
				if err != nil {
					return err
				}
				report.Investigation[finding.ID] = investigation
			}
			return nil
		}); err != nil {
			report.Errors = append(report.Errors, err.Error())
		}
	}
	if manifest.Track == TrackRemediation {
		if err := r.phase(&report, "governed_remediation", func() error {
			return r.runRemediation(ctx, &report, prepared, collected)
		}); err != nil {
			report.Errors = append(report.Errors, err.Error())
		}
	}

	report.Score = ScoreRun(ScoringInput{
		Manifest: manifest, GroundTruth: report.GroundTruth,
		Run: report.PatrolRun, Findings: report.Findings,
		Provider: report.Environment.Provider, Model: report.Environment.Model,
		InferenceRoute:    report.Environment.InferenceRoute,
		CollectionLatency: phaseDuration(report.Phases, "normal_collection_convergence"),
		EndToEndLatency:   time.Since(faultStarted),
		FaultsIntact:      faultsIntact, NoMutation: noMutation,
	})
	if manifest.Track != TrackWatch && len(report.Investigation) == 0 {
		report.Score.HardFailures = append(report.Score.HardFailures, "no completed investigation evidence")
		report.Score.Passed = false
	}
	ApplyProTrackGates(&report.Score, manifest, report.Investigation, report.Remediation)

	if err := r.phase(&report, "revert_and_verify", func() error {
		for i := len(manifest.Faults) - 1; i >= 0; i-- {
			fault := manifest.Faults[i]
			if err := r.config.Lab.RevertFault(ctx, manifest, prepared, fault); err != nil {
				return fmt.Errorf("revert fault %s: %w", fault.ID, err)
			}
			predicates := fault.RevertOracle
			if len(predicates) == 0 {
				predicates = manifest.Baseline
			}
			observations, err := r.config.Lab.Observe(ctx, manifest, prepared, predicates)
			report.Revert = append(report.Revert, observations...)
			if err != nil {
				return fmt.Errorf("verify revert %s: %w", fault.ID, err)
			}
		}
		if len(manifest.Faults) == 0 {
			observations, err := r.config.Lab.Observe(ctx, manifest, prepared, manifest.Baseline)
			report.Revert = append(report.Revert, observations...)
			return err
		}
		return nil
	}); err != nil {
		report.Errors = append(report.Errors, err.Error())
		terminalErr = errors.Join(terminalErr, err)
	}
	return report, terminalErr
}

func (r *QualificationRunner) initialEnvironment() Environment {
	provider, _ := splitModel(r.config.ModelOverride)
	dockerTarget := ""
	if r.config.Lab != nil {
		dockerTarget = dockerTargetLabel(r.config.Lab.target)
	}
	return Environment{
		GitSHA:         r.config.GitSHA,
		GitDirty:       r.config.GitDirty,
		PulseBaseURL:   r.config.Client.config.BaseURL,
		DockerTarget:   dockerTarget,
		Model:          strings.TrimSpace(r.config.ModelOverride),
		Provider:       provider,
		InferenceRoute: inferenceRouteForProvider(provider),
		ChallengeNonce: r.config.ChallengeNonce,
		CapturedAt:     time.Now().UTC(),
	}
}

func validateCollectedScenarioProjection(manifest Manifest, resources map[string]Resource) error {
	for _, fault := range manifest.Faults {
		for _, predicate := range fault.Oracle {
			if err := validateCollectedDockerPredicate("fault "+fault.ID, predicate, resources); err != nil {
				return err
			}
		}
	}

	negativeControls := make(map[string]struct{}, len(manifest.NegativeControls))
	for _, control := range manifest.NegativeControls {
		negativeControls[control.Resource] = struct{}{}
	}
	for _, predicate := range manifest.Baseline {
		if _, ok := negativeControls[predicate.Target]; !ok {
			continue
		}
		if err := validateCollectedDockerPredicate("negative control "+predicate.Target, predicate, resources); err != nil {
			return err
		}
	}
	return nil
}

func validateCollectedDockerPredicate(subject string, predicate Predicate, resources map[string]Resource) error {
	resource, ok := resources[predicate.Target]
	if !ok {
		return fmt.Errorf("%s target %s is absent from collected resources", subject, predicate.Target)
	}
	if resource.Docker == nil {
		return fmt.Errorf("%s target %s has no collected Docker projection", subject, predicate.Target)
	}
	switch predicate.Probe {
	case "docker.health":
		var expected string
		if err := json.Unmarshal(predicate.Value, &expected); err != nil {
			return fmt.Errorf("%s has invalid collected health expectation: %w", subject, err)
		}
		if !strings.EqualFold(strings.TrimSpace(resource.Docker.Health), strings.TrimSpace(expected)) {
			return fmt.Errorf("%s target %s collected health=%q want %q", subject, predicate.Target, resource.Docker.Health, expected)
		}
	case "docker.running":
		var expected bool
		if err := json.Unmarshal(predicate.Value, &expected); err != nil {
			return fmt.Errorf("%s has invalid collected running expectation: %w", subject, err)
		}
		running := strings.EqualFold(strings.TrimSpace(resource.Docker.ContainerState), "running")
		if running != expected {
			return fmt.Errorf("%s target %s collected running=%t state=%q want %t", subject, predicate.Target, running, resource.Docker.ContainerState, expected)
		}
	case "docker.restart_count":
		var expected int
		if err := json.Unmarshal(predicate.Value, &expected); err != nil {
			return fmt.Errorf("%s has invalid collected restart-count expectation: %w", subject, err)
		}
		observed := resource.Docker.RestartCount
		satisfied := predicate.Operator == "eq" && observed == expected || predicate.Operator == "gte" && observed >= expected
		if !satisfied {
			return fmt.Errorf("%s target %s collected restart_count=%d does not satisfy %s %d", subject, predicate.Target, observed, predicate.Operator, expected)
		}
	}
	return nil
}

func phaseDuration(phases []PhaseTiming, name string) time.Duration {
	for _, phase := range phases {
		if phase.Name == name {
			return phase.Duration
		}
	}
	return 0
}

func (r *QualificationRunner) runRemediation(ctx context.Context, report *RunReport, prepared *PreparedLab, collected map[string]Resource) error {
	spec := r.config.Manifest.Remediation
	if spec == nil {
		return errors.New("remediation specification is missing")
	}
	resource, ok := collected[spec.ActionTarget]
	if !ok || resource.ID == "" {
		return fmt.Errorf("remediation target %q was not collected", spec.ActionTarget)
	}
	result := RemediationResult{ResourceID: resource.ID, Decision: spec.Decision, Authorized: r.config.AuthorizeRemediation || spec.Decision == "observe"}
	defer func() { report.Remediation = result }()

	var reference *aicontracts.ActionReference
	for findingID, investigation := range report.Investigation {
		if investigation.Action == nil || investigation.Action.ResourceID != resource.ID {
			continue
		}
		copy := *investigation.Action
		reference = &copy
		result.FindingID = findingID
		result.InvestigationID = investigation.ID
		break
	}
	if reference == nil {
		return fmt.Errorf("no investigation action references exact resource %s", resource.ID)
	}
	result.ActionID = reference.ActionID
	result.CapabilityName = reference.CapabilityName
	if result.ActionID == "" || !stringInFold(spec.ExpectedCapabilities, reference.CapabilityName) {
		return fmt.Errorf("investigation action %q has unexpected capability %q", result.ActionID, reference.CapabilityName)
	}

	detail, err := r.config.Client.Action(ctx, result.ActionID)
	if err != nil {
		return fmt.Errorf("load exact action %s: %w", result.ActionID, err)
	}
	result.Before = &detail
	audit := detail.Audit.ActionAuditRecord
	result.PlanHashBound = audit.Plan.PlanHash != "" && audit.Plan.PlanHash == reference.Plan.PlanHash
	result.OriginBound = audit.Origin != nil && audit.Origin.FindingID == result.FindingID &&
		audit.Origin.InvestigationID == result.InvestigationID && audit.Request.ResourceID == resource.ID
	if !result.PlanHashBound {
		return errors.New("investigation action plan hash does not match authoritative action audit")
	}
	if spec.RequireExactOrigin && !result.OriginBound {
		return errors.New("action origin is not bound to the exact finding, investigation, and resource")
	}

	pending, pendingErr := r.config.Client.Actions(ctx, "pending")
	settled, settledErr := r.config.Client.Actions(ctx, "settled")
	if pendingErr == nil {
		report.Actions = append(report.Actions, pending...)
	}
	if settledErr == nil {
		report.Actions = append(report.Actions, settled...)
	}
	if pendingErr != nil && settledErr != nil {
		return errors.Join(pendingErr, settledErr)
	}

	switch spec.Decision {
	case "observe":
		result.Passed = true
		return nil
	case "reject":
		if _, err := r.config.Client.DecideAction(ctx, result.ActionID, "rejected", spec.DecisionReason, audit.Plan.PlanHash); err != nil {
			return fmt.Errorf("reject exact action: %w", err)
		}
	case "approve_execute":
		if _, err := r.config.Client.DecideAction(ctx, result.ActionID, "approved", spec.DecisionReason, audit.Plan.PlanHash); err != nil {
			return fmt.Errorf("approve exact action: %w", err)
		}
		if _, err := r.config.Client.ExecuteAction(ctx, result.ActionID, spec.DecisionReason, audit.Plan.PlanHash); err != nil {
			return fmt.Errorf("execute exact approved action: %w", err)
		}
	default:
		return fmt.Errorf("unsupported remediation decision %q", spec.Decision)
	}

	timeout, _ := positiveDuration(spec.ActionTimeout)
	after, err := r.config.Client.WaitForAction(ctx, result.ActionID, timeout)
	if err != nil {
		return err
	}
	result.After = &after
	if spec.Decision == "reject" && string(after.Audit.State) != "rejected" {
		return fmt.Errorf("rejected action reached unexpected state %q", after.Audit.State)
	}
	if spec.Decision == "approve_execute" && string(after.Audit.State) != "completed" {
		return fmt.Errorf("approved action reached unexpected state %q", after.Audit.State)
	}
	if len(spec.Postconditions) > 0 {
		observations, observeErr := r.config.Lab.Observe(ctx, r.config.Manifest, prepared, spec.Postconditions)
		result.Postconditions = observations
		result.IndependentVerified = observeErr == nil && allObservationsPassed(observations)
		if observeErr != nil {
			result.Errors = append(result.Errors, observeErr.Error())
		}
	} else {
		result.IndependentVerified = spec.Decision != "approve_execute"
	}
	verificationStatus := string(after.Audit.VerificationOutcome.Status)
	result.LifecycleVerified = !spec.RequireLifecycleVerification || stringInFold(spec.AllowedVerificationStatuses, verificationStatus)
	if spec.RequireLifecycleVerification && !result.LifecycleVerified {
		result.Errors = append(result.Errors, fmt.Sprintf("lifecycle verification status %q is not allowed", verificationStatus))
	}
	result.Passed = result.OriginBound && result.PlanHashBound && result.IndependentVerified && result.LifecycleVerified && len(result.Errors) == 0
	if !result.Passed {
		return errors.New("governed remediation verification failed")
	}
	return nil
}

func (r *QualificationRunner) phase(report *RunReport, name string, operation func() error) error {
	phase := PhaseTiming{Name: name, StartedAt: time.Now().UTC()}
	err := operation()
	phase.EndedAt = time.Now().UTC()
	phase.Duration = phase.EndedAt.Sub(phase.StartedAt)
	phase.Passed = err == nil
	if err != nil {
		phase.Error = sanitizeArtifactText(err.Error())
	}
	report.Phases = append(report.Phases, phase)
	return err
}

func dockerTargetLabel(target DockerTarget) string {
	if target.SSHHost != "" {
		return "ssh:" + target.SSHHost
	}
	return "context:" + target.Context
}

func GitEnvironment(ctx context.Context, runner CommandRunner, repo string) (string, bool) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	sha, err := runner.Run(ctx, "git", "-C", repo, "rev-parse", "HEAD")
	if err != nil {
		return "", true
	}
	status, err := runner.Run(ctx, "git", "-C", repo, "status", "--porcelain")
	return strings.TrimSpace(sha.Stdout), err != nil || strings.TrimSpace(status.Stdout) != ""
}

func EnsureArtifactRoot(root string) error {
	if root == "" {
		return errors.New("artifact root is empty")
	}
	return os.MkdirAll(root, 0o700)
}
