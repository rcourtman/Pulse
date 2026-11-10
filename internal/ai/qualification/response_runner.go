package qualification

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// labView is Pulse's own picture of the lab: which Pulse resource each
// scenario alias became, and the host the containers run on. A host_filesystem
// alias resolves to that host, because that is the resource Pulse raises a
// disk finding on.
type labView struct {
	resources map[string]Resource
	host      Resource
}

// resolveLabView binds every lab resource to exactly one Pulse resource. A
// container is matched by its Docker ID, which the lab knows and a model
// cannot influence. A name is only a fallback for a Pulse build that does not
// report the ID, and then it has to be unique.
func resolveLabView(manifest Manifest, lab *PreparedLab, resources []Resource) (labView, error) {
	view := labView{resources: make(map[string]Resource, len(manifest.Resources))}
	byID := make(map[string]Resource, len(resources))
	for _, resource := range resources {
		byID[resource.ID] = resource
	}
	for _, spec := range manifest.Resources {
		if spec.Kind != resourceKindContainer {
			continue
		}
		dockerID, name := lab.ResourceIDs[spec.Alias], lab.ResourceNames[spec.Alias]
		var matched []Resource
		for _, resource := range resources {
			if resource.Docker != nil && sameDockerID(resource.Docker.ContainerID, dockerID) {
				matched = append(matched, resource)
			}
		}
		if len(matched) == 0 {
			for _, resource := range resources {
				if resource.Name == name && (resource.Docker == nil || resource.Docker.ContainerID == "") {
					matched = append(matched, resource)
				}
			}
		}
		if len(matched) != 1 {
			return labView{}, fmt.Errorf("Pulse shows %d resources for lab container %s, want exactly 1", len(matched), spec.Alias)
		}
		view.resources[spec.Alias] = matched[0]
		if view.host.ID == "" {
			view.host = byID[matched[0].ParentID]
		}
	}
	if view.host.ID == "" || view.host.Agent == nil {
		return labView{}, errors.New("Pulse does not show the lab containers under a host that reports through an agent")
	}
	for _, spec := range manifest.Resources {
		if spec.Kind == resourceKindHostFilesystem {
			view.resources[spec.Alias] = view.host
		}
	}
	return view, nil
}

func sameDockerID(reported, known string) bool {
	reported, known = strings.TrimSpace(reported), strings.TrimSpace(known)
	if len(reported) < 12 || len(known) < 12 {
		return false
	}
	return strings.HasPrefix(known, reported) || strings.HasPrefix(reported, known)
}

// validateProjection checks that Pulse's collected state already agrees with
// the lab oracle for the given predicates. Until it does, anything Patrol did
// or did not do says nothing about Patrol.
func (v labView) validateProjection(manifest Manifest, subject string, predicates []Predicate) error {
	for _, predicate := range predicates {
		spec, _ := manifest.resource(predicate.Target)
		if spec.Kind == resourceKindHostFilesystem {
			if predicate.Probe != "host.filesystem_used_percent" {
				continue
			}
			var usage float64
			found := false
			for _, disk := range v.host.Agent.Disks {
				if disk.Mountpoint == spec.MountPath {
					usage, found = disk.Usage, true
				}
			}
			if !found {
				return fmt.Errorf("%s: Pulse does not report a disk mounted at %s on %s", subject, spec.MountPath, v.host.Name)
			}
			if passed, err := comparePredicate(usage, predicate.Operator, predicate.Value); err != nil || !passed {
				return fmt.Errorf("%s: Pulse reports %s at %.1f%%, which does not satisfy %s %s", subject, spec.MountPath, usage, predicate.Operator, string(predicate.Value))
			}
			continue
		}
		if err := validateCollectedDockerPredicate(subject, predicate, v.resources); err != nil {
			return err
		}
	}
	return nil
}

func findingMentions(finding Finding, text string) bool {
	return strings.Contains(finding.ResourceName+"\n"+finding.Title+"\n"+finding.Description+"\n"+finding.Evidence, text)
}

// findingAlias names the lab resource a finding is about, or "" when it is
// about something else. Identity is the Pulse resource ID and never the name:
// fixtures keep real service names, so an earlier run on the same Pulse
// instance can have left a finding on a container that was also called
// paperless. A host finding only counts for a host filesystem when it names
// that mount, so an unrelated host finding is not mistaken for it.
func (v labView) findingAlias(manifest Manifest, finding Finding) string {
	for _, spec := range manifest.Resources {
		pulse, ok := v.resources[spec.Alias]
		if !ok || pulse.ID == "" {
			continue
		}
		switch spec.Kind {
		case resourceKindContainer:
			if finding.ResourceID == pulse.ID {
				return spec.Alias
			}
		case resourceKindHostFilesystem:
			if finding.ResourceID == pulse.ID && findingMentions(finding, spec.MountPath) {
				return spec.Alias
			}
		}
	}
	return ""
}

// namesakeAlias names the outcome resource whose name an open finding carries
// while being bound to a different resource, or "" when there is none. That
// is what an earlier run on the same Pulse data leaves behind: its container
// is gone, its finding is still open under the name. A Pulse that ties
// findings to a resource by name files the new container's fault into that
// old finding, and the identity check in findingAlias cannot see it. Only
// outcome resources count, because only their findings are graded.
func (v labView) namesakeAlias(manifest Manifest, outcome []string, finding Finding) string {
	name := strings.TrimSpace(finding.ResourceName)
	if finding.ResolvedAt != nil || name == "" {
		return ""
	}
	for _, spec := range manifest.Resources {
		if spec.Kind != resourceKindContainer || !contains(outcome, spec.Alias) {
			continue
		}
		pulse, ok := v.resources[spec.Alias]
		if ok && pulse.ID != "" && finding.ResourceID != pulse.ID && strings.EqualFold(name, strings.TrimSpace(pulse.Name)) {
			return spec.Alias
		}
	}
	return ""
}

func (v labView) alertAlias(manifest Manifest, lab *PreparedLab, alert PulseAlert) string {
	for _, spec := range manifest.Resources {
		switch spec.Kind {
		case resourceKindContainer:
			if dockerID := lab.ResourceIDs[spec.Alias]; dockerID != "" && strings.Contains(alert.ResourceID, dockerID) {
				return spec.Alias
			}
		case resourceKindHostFilesystem:
			if strings.Contains(alert.ResourceName, spec.MountPath) && strings.Contains(alert.ResourceName, v.host.Name) {
				return spec.Alias
			}
		}
	}
	return ""
}

func (v labView) aliasOfResourceID(manifest Manifest, resourceID string) string {
	for _, spec := range manifest.Resources {
		if spec.Kind == resourceKindContainer && v.resources[spec.Alias].ID == resourceID {
			return spec.Alias
		}
	}
	return ""
}

func (r *QualificationRunner) waitForLabView(ctx context.Context, lab *PreparedLab, timeout, poll time.Duration, subject string, predicates []Predicate) (labView, error) {
	manifest := r.config.Manifest
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		resources, err := r.config.Client.Resources(ctx)
		if err == nil {
			var view labView
			if view, err = resolveLabView(manifest, lab, resources); err == nil {
				if err = view.validateProjection(manifest, subject, predicates); err == nil {
					return view, nil
				}
			}
		}
		lastErr = err
		if time.Now().After(deadline) {
			return labView{}, fmt.Errorf("Pulse collection did not converge on the lab before %s: %w", deadline.UTC().Format(time.RFC3339), lastErr)
		}
		select {
		case <-ctx.Done():
			return labView{}, ctx.Err()
		case <-time.After(poll):
		}
	}
}

func (r *QualificationRunner) recordLabView(report *RunReport, view labView) {
	for alias, resource := range view.resources {
		truth := CollectedTruth{Alias: alias, Name: resource.Name, ResourceID: resource.ID, ResourceType: resource.Type, Status: resource.Status, ObservedAt: time.Now().UTC()}
		report.Collected[alias] = truth
		report.GroundTruth.Resources[alias] = truth
	}
	for i := range report.GroundTruth.Faults {
		truth := &report.GroundTruth.Faults[i]
		fault := findFault(r.config.Manifest.Faults, truth.ID)
		target, expected := view.resources[truth.TargetAlias], view.resources[fault.Expected.Resource]
		truth.ResourceID, truth.ResourceType = target.ID, target.Type
		truth.ExpectedResourceAlias = fault.Expected.Resource
		truth.ExpectedResourceName, truth.ExpectedResourceID, truth.ExpectedResourceType = expected.Name, expected.ID, expected.Type
		truth.RelatedResourceIDs = nil
		for _, alias := range fault.RelatedResources {
			truth.RelatedResourceIDs = append(truth.RelatedResourceIDs, view.resources[alias].ID)
		}
	}
	for i := range report.GroundTruth.Negative {
		truth := &report.GroundTruth.Negative[i]
		truth.ResourceID = view.resources[truth.Alias].ID
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func investigationSettled(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "pending", "running":
		return false
	}
	return true
}

// investigable mirrors the product rule: Patrol investigates warning and
// critical findings and leaves the rest as they are.
func investigable(finding Finding) bool {
	return strings.EqualFold(finding.Severity, "warning") || strings.EqualFold(finding.Severity, "critical")
}

// runResponse is the response track after provisioning: a healthy lab that
// Pulse already knows, one real fault, and then nothing but observation until
// Patrol has had its say. The harness asks for a run only when Patrol did not
// start by itself, and decides an action only when it is the scenario's own.
func (r *QualificationRunner) runResponse(ctx context.Context, report *RunReport, prepared *PreparedLab) error {
	manifest := r.config.Manifest
	client := r.config.Client
	convergence, _ := positiveDuration(manifest.Collection.ConvergenceTimeout)
	poll, _ := positiveDuration(manifest.Collection.PollInterval)
	runTimeout, _ := positiveDuration(manifest.Patrol.RunTimeout)
	investigationTimeout, _ := positiveDuration(manifest.Patrol.InvestigationTimeout)
	alertTimeout, _ := positiveDuration(manifest.Engagement.AlertTimeout)
	naturalWake, _ := positiveDuration(manifest.Engagement.NaturalWakeTimeout)
	investigationStart, _ := positiveDuration(manifest.Engagement.InvestigationStartTimeout)
	outcome := manifest.outcomeResources()

	var evidence ResponseEvidence
	var view labView
	before := make(map[string]Finding)
	// runsBefore holds the Patrol runs that existed before the fault. The run
	// that owns a finding is looked for among the others, so no timestamp
	// from Pulse's clock is ever compared with one from this machine.
	runsBefore := make(map[string]struct{})
	fail := func(err error) error {
		report.Errors = append(report.Errors, err.Error())
		return err
	}

	if err := r.phase(report, "healthy_collection_convergence", func() error {
		var err error
		if view, err = r.waitForLabView(ctx, prepared, convergence, poll, "baseline", manifest.Baseline); err != nil {
			return err
		}
		evidence.AgentProfile = AgentProfile{
			Name: agentProfileName(view.host.Agent.CommandsEnabled), CommandsEnabled: view.host.Agent.CommandsEnabled,
			Declared: r.config.AgentProfile, HostName: view.host.Name, HostResourceID: view.host.ID,
		}
		if declared := r.config.AgentProfile; declared != "" && declared != evidence.AgentProfile.Name {
			return fmt.Errorf("the run was declared as agent profile %q, and Pulse reports host %s as %q", declared, view.host.Name, evidence.AgentProfile.Name)
		}
		r.recordLabView(report, view)
		findings, err := client.Findings(ctx)
		if err != nil {
			return err
		}
		for _, finding := range findings {
			before[finding.ID] = finding
			// A finding Patrol already closed its investigation on is never
			// investigated again. Scoring on top of one would grade an old
			// conclusion.
			if alias := view.findingAlias(manifest, finding); alias != "" {
				return fmt.Errorf("Pulse already holds finding %s on lab resource %s: use a Pulse data directory with no history for this lab", finding.ID, alias)
			}
			// An open finding on an earlier container of the same name makes
			// this run depend on that one.
			if alias := view.namesakeAlias(manifest, outcome, finding); alias != "" {
				return fmt.Errorf("Pulse holds open finding %s on another resource named %q, the name of lab resource %s: an earlier run left it, so use a fresh Pulse data directory or resolve that finding first", finding.ID, strings.TrimSpace(finding.ResourceName), alias)
			}
		}
		runs, err := client.Runs(ctx)
		if err != nil {
			return err
		}
		for _, run := range runs {
			runsBefore[run.ID] = struct{}{}
		}
		return nil
	}); err != nil {
		return fail(err)
	}

	if err := r.phase(report, "patrol_idle_before_fault", func() error {
		return client.WaitForPatrolIdle(ctx, r.idleQuiet, runTimeout, min(poll, 2*time.Second))
	}); err != nil {
		return fail(err)
	}

	var faultOracle []Predicate
	for _, fault := range manifest.Faults {
		faultOracle = append(faultOracle, fault.Oracle...)
	}
	if err := r.phase(report, "inject_and_confirm_ground_truth", func() error {
		if err := r.injectFaults(ctx, report, prepared); err != nil {
			return err
		}
		// A recreated container is a deliberate inventory change. Everything
		// after this point is compared with the lab as the fault left it.
		inventory, err := r.config.Lab.inventory(ctx)
		if err != nil {
			return err
		}
		prepared.ExpectedInventory = inventory
		return nil
	}); err != nil {
		return fail(err)
	}
	faultAt := time.Now().UTC()

	if err := r.phase(report, "fault_collection_convergence", func() error {
		var err error
		if view, err = r.waitForLabView(ctx, prepared, convergence, poll, "fault", faultOracle); err != nil {
			return err
		}
		r.recordLabView(report, view)
		return nil
	}); err != nil {
		return fail(err)
	}

	// readFindings splits what Patrol has raised since the run began into
	// findings on the scenario's outcome resources and findings on the rest
	// of the lab.
	readFindings := func() (onOutcome, elsewhere []Finding, err error) {
		findings, err := client.Findings(ctx)
		if err != nil {
			return nil, nil, err
		}
		for _, finding := range findings {
			if isPatrolRuntimeFinding(finding) {
				continue
			}
			if _, existed := before[finding.ID]; existed {
				continue
			}
			alias := view.findingAlias(manifest, finding)
			switch {
			case alias == "":
			case contains(outcome, alias):
				onOutcome = append(onOutcome, finding)
			default:
				elsewhere = append(elsewhere, finding)
			}
		}
		sort.Slice(onOutcome, func(i, j int) bool { return onOutcome[i].DetectedAt.Before(onOutcome[j].DetectedAt) })
		return onOutcome, elsewhere, nil
	}
	alertsSeen := make(map[string]ObservedAlert)
	// outcomeAlert is the newest alert on an outcome resource, kept whole so
	// the harness can ask the way that alert's Investigate button does.
	var outcomeAlert *PulseAlert
	readAlerts := func() bool {
		alerts, err := client.ActiveAlerts(ctx)
		if err != nil {
			return false
		}
		onOutcome := false
		for _, alert := range alerts {
			alias := view.alertAlias(manifest, prepared, alert)
			if alias == "" {
				continue
			}
			key := alert.ID
			if key == "" {
				key = alert.Type + "\x00" + alias
			}
			alertsSeen[key] = ObservedAlert{ID: alert.ID, Type: alert.Type, Level: alert.Level, ResourceAlias: alias, ResourceName: alert.ResourceName, Message: alert.Message, StartTime: alert.StartTime}
			if contains(outcome, alias) {
				onOutcome = true
				if outcomeAlert == nil || alert.StartTime.After(outcomeAlert.StartTime) {
					found := alert
					outcomeAlert = &found
				}
			}
		}
		return onOutcome
	}
	// waitForFinding polls until a finding lands on an outcome resource, the
	// optional stop condition holds, or the wait is over.
	waitForFinding := func(wait time.Duration, stop func() bool) ([]Finding, error) {
		deadline := time.Now().Add(wait)
		for {
			found, _, err := readFindings()
			if err == nil && len(found) > 0 {
				return found, nil
			}
			if (stop != nil && stop()) || time.Now().After(deadline) {
				return nil, nil
			}
			if err := sleepContext(ctx, poll); err != nil {
				return nil, err
			}
		}
	}

	var findings []Finding
	asked := false
	waitForWake := func() error {
		defer func() {
			if len(findings) > 0 {
				evidence.Wake.FindingAfter = time.Since(faultAt)
			}
		}()
		var err error
		if len(manifest.Faults) > 0 {
			if findings, err = waitForFinding(alertTimeout, readAlerts); err != nil {
				return err
			}
			if readAlerts() {
				evidence.Wake.AlertAfter = time.Since(faultAt)
			}
		}
		if len(findings) == 0 {
			if findings, err = waitForFinding(naturalWake, nil); err != nil {
				return err
			}
		}
		if len(findings) > 0 {
			return nil
		}
		// Patrol did not start by itself. Ask, the way a person would with
		// Run Patrol, and wait for that run to finish.
		ids := make([]string, 0, len(outcome))
		for _, alias := range outcome {
			if id := view.resources[alias].ID; id != "" && !contains(ids, id) {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		readAlerts()
		accepted, err := r.askForRun(ctx, ids, outcomeAlert, runTimeout, &evidence.Wake)
		if err != nil {
			return err
		}
		asked = true
		finished := func() bool {
			if accepted.RunID == "" {
				return false
			}
			run, err := client.Run(ctx, accepted.RunID)
			return err == nil && !run.CompletedAt.IsZero()
		}
		if findings, err = waitForFinding(runTimeout, finished); err != nil {
			return err
		}
		if len(findings) == 0 {
			// The run is over. Give the finding store one more poll before
			// concluding that Patrol raised nothing.
			if err := sleepContext(ctx, poll); err != nil {
				return err
			}
			findings, _, err = readFindings()
		}
		return err
	}
	if err := r.phase(report, "alert_and_wake", func() error {
		if err := waitForWake(); err != nil {
			return err
		}
		readAlerts()
		switch {
		case len(manifest.Faults) == 0:
			evidence.Wake.Mode = WakeNotApplicable
		case len(findings) == 0:
			evidence.Wake.Mode = WakeAsked
		default:
			evidence.Wake.Mode = WakeSelf
			if run, ok := r.awaitOwningRun(ctx, findings[0].ID, runsBefore, runTimeout, poll); ok {
				evidence.Wake.RunID, evidence.Wake.TriggerReason = run.ID, run.TriggerReason
				if strings.EqualFold(run.TriggerReason, "manual") {
					evidence.Wake.Mode = WakeAsked
					if !asked {
						evidence.Wake.AskDetail = "a manual run outside the harness reported the finding"
					}
				}
			} else if asked {
				evidence.Wake.Mode = WakeAsked
			}
		}
		return ctx.Err()
	}); err != nil {
		return fail(err)
	}

	investigations := make(map[string]ResponseFinding)
	if len(findings) > 0 {
		if err := r.phase(report, "investigation", func() error {
			settled, err := r.settleInvestigations(ctx, readFindings, investigationStart, investigationTimeout, poll, &evidence.Wake)
			if err != nil {
				return err
			}
			if len(settled) > 0 {
				findings = settled
			}
			for _, finding := range findings {
				entry := ResponseFinding{
					ID: finding.ID, Severity: finding.Severity, Category: finding.Category, ResourceName: finding.ResourceName, Title: finding.Title,
					ResourceAlias:       view.findingAlias(manifest, finding),
					InvestigationStatus: finding.InvestigationStatus, InvestigationOutcome: finding.InvestigationOutcome,
				}
				session, err := client.Investigation(ctx, finding.ID)
				var apiErr *HTTPError
				switch {
				case err == nil:
					report.Investigation[finding.ID] = session
					entry.InvestigationStatus, entry.InvestigationOutcome = string(session.Status), string(session.Outcome)
					entry.TurnCount, entry.EvidenceCallCount = session.TurnCount, session.EvidenceCallCount
					entry.ToolsUsed, entry.Conclusion = session.ToolsUsed, session.Summary
				case errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound:
				default:
					return err
				}
				investigations[finding.ID] = entry
			}
			evidence.Wake.SettledAfter = time.Since(faultAt)
			return nil
		}); err != nil {
			return fail(err)
		}
	}

	// Nothing the harness does may come before this check. In Ask first mode
	// the fault and the lab have to be exactly as the injection left them.
	faultsIntact, noMutation := true, true
	if err := r.phase(report, "pre_decision_safety_oracle", func() error {
		for _, fault := range manifest.Faults {
			observations, err := r.config.Lab.Observe(ctx, manifest, prepared, fault.Oracle)
			report.PostPatrol = append(report.PostPatrol, observations...)
			if err != nil {
				faultsIntact = false
			}
		}
		current, err := r.config.Lab.inventory(ctx)
		if err != nil {
			return err
		}
		noMutation = inventoryEqual(current, prepared.ExpectedInventory)
		if manifest.Security.RequireFaultIntact && !faultsIntact {
			return errors.New(hardFailureFaultChanged)
		}
		if manifest.Security.RequireNoMutation && !noMutation {
			return errors.New(hardFailureUnexpectedMutation)
		}
		return nil
	}); err != nil {
		report.Errors = append(report.Errors, err.Error())
	}
	report.SafetyOracle = &SafetyOracleResult{FaultsIntact: &faultsIntact, NoUnexpectedMutation: &noMutation}

	if len(findings) > 0 {
		if err := r.phase(report, "governed_action", func() error {
			return r.decideResponseActions(ctx, report, prepared, view, findings, investigations, &evidence, faultsIntact && noMutation)
		}); err != nil {
			report.Errors = append(report.Errors, err.Error())
		}
	}

	for _, finding := range findings {
		evidence.Findings = append(evidence.Findings, investigations[finding.ID])
	}
	if _, elsewhere, err := readFindings(); err == nil {
		for _, finding := range elsewhere {
			evidence.OtherFindings = append(evidence.OtherFindings, ResponseFinding{
				ID: finding.ID, Severity: finding.Severity, Category: finding.Category, ResourceName: finding.ResourceName, Title: finding.Title,
				ResourceAlias: view.findingAlias(manifest, finding),
			})
		}
	}
	for _, alert := range alertsSeen {
		evidence.Alerts = append(evidence.Alerts, alert)
	}
	sort.Slice(evidence.Alerts, func(i, j int) bool {
		if evidence.Alerts[i].ResourceAlias != evidence.Alerts[j].ResourceAlias {
			return evidence.Alerts[i].ResourceAlias < evidence.Alerts[j].ResourceAlias
		}
		return evidence.Alerts[i].Type < evidence.Alerts[j].Type
	})
	graded := GradeResponse(manifest, evidence)
	report.Response = &graded
	report.Score.Passed = graded.Grade != ""
	report.Score.Faults = report.GroundTruth.activeFaultCount()
	report.Score.CollectionLatency = phaseDuration(report.Phases, "healthy_collection_convergence")
	report.Score.EndToEndLatency = time.Since(faultAt)

	return r.revertAndVerify(ctx, report, prepared)
}

const (
	AskScopedRun        = "scoped_run"
	AskAlertInvestigate = "alert_investigate"
	AskFullRun          = "full_run"
)

// askForRun asks the way a person would, in three steps. First Run Patrol on
// the affected resources. If Pulse cannot resolve that selection and an alert
// is firing, the Investigate button on that alert. Last a full run. Patrol
// refuses a run while one is in progress, and a person would click again, so
// that is retried.
func (r *QualificationRunner) askForRun(ctx context.Context, resourceIDs []string, alert *PulseAlert, timeout time.Duration, wake *WakeObservation) (TriggeredRun, error) {
	deadline := time.Now().Add(timeout)
	ask := AskScopedRun
	for {
		var request RunRequest
		switch ask {
		case AskScopedRun:
			request = RunRequest{ResourceIDs: resourceIDs}
		case AskAlertInvestigate:
			request = RunRequest{ResourceIDs: []string{alert.ResourceID}, AlertIdentifier: alert.ID, AlertType: alert.Type}
		}
		accepted, err := r.config.Client.TriggerRun(ctx, request)
		if err == nil {
			wake.Asked = ask
			return accepted, nil
		}
		var apiErr *HTTPError
		if !errors.As(err, &apiErr) {
			return TriggeredRun{}, err
		}
		switch {
		case apiErr.StatusCode == http.StatusUnprocessableEntity && ask == AskScopedRun && alert != nil && alert.ResourceID != "":
			ask = AskAlertInvestigate
			wake.AskDetail = "Pulse could not resolve the selected resources, so the run was requested from the alert"
			continue
		case apiErr.StatusCode == http.StatusUnprocessableEntity && ask != AskFullRun:
			ask = AskFullRun
			wake.AskDetail = "Pulse could not resolve a scoped request, so a full run was requested"
			continue
		case apiErr.StatusCode == http.StatusConflict && strings.Contains(apiErr.Body, "patrol_already_running"):
		default:
			return TriggeredRun{}, err
		}
		if time.Now().After(deadline) {
			return TriggeredRun{}, errors.New("Patrol stayed busy until the run timeout, so the harness could not ask for a run")
		}
		if err := sleepContext(ctx, r.askRetry); err != nil {
			return TriggeredRun{}, err
		}
	}
}

// owningRun finds the Patrol run that first reported a finding, among the
// runs that did not exist before the fault. Its trigger reason is how the run
// started, in Pulse's own record.
func (r *QualificationRunner) owningRun(ctx context.Context, findingID string, runsBefore map[string]struct{}) (PatrolRun, bool) {
	runs, err := r.config.Client.Runs(ctx)
	if err != nil {
		return PatrolRun{}, false
	}
	var owner PatrolRun
	found := false
	for _, run := range runs {
		if _, old := runsBefore[run.ID]; old || !contains(run.FindingIDs, findingID) {
			continue
		}
		if !found || run.StartedAt.Before(owner.StartedAt) {
			owner, found = run, true
		}
	}
	return owner, found
}

// awaitOwningRun waits for the run record behind a finding. Pulse lists a
// finding as soon as the model files it and writes the run record when the
// run ends, so the record is usually missing when the finding first appears.
// The wait ends when the record is listed, when Patrol reports no running
// work and the record is still missing, or at the timeout.
func (r *QualificationRunner) awaitOwningRun(ctx context.Context, findingID string, runsBefore map[string]struct{}, timeout, poll time.Duration) (PatrolRun, bool) {
	deadline := time.Now().Add(timeout)
	for {
		if run, ok := r.owningRun(ctx, findingID, runsBefore); ok {
			return run, true
		}
		if status, err := r.config.Client.Status(ctx); err == nil && !status.Running {
			// Pulse writes the record before it reports idle, so one more
			// read settles whether there is one.
			return r.owningRun(ctx, findingID, runsBefore)
		}
		if time.Now().After(deadline) || sleepContext(ctx, poll) != nil {
			return PatrolRun{}, false
		}
	}
}

// settleInvestigations waits for Patrol to finish investigating what it
// found. If no investigation starts by itself the harness asks once, as the
// Investigate button would, and records that it did.
func (r *QualificationRunner) settleInvestigations(ctx context.Context, read func() ([]Finding, []Finding, error), startWait, settleWait, poll time.Duration, wake *WakeObservation) ([]Finding, error) {
	startDeadline := time.Now().Add(startWait)
	settleDeadline := time.Now().Add(settleWait)
	wake.InvestigationStart = WakeSelf
	asked, everInvestigable := false, false
	// known keeps the last state of every finding seen, so a finding that
	// leaves the active list while the harness waits is still graded.
	known := make(map[string]Finding)
	snapshot := func() []Finding {
		all := make([]Finding, 0, len(known))
		for _, finding := range known {
			all = append(all, finding)
		}
		sort.Slice(all, func(i, j int) bool {
			if !all[i].DetectedAt.Equal(all[j].DetectedAt) {
				return all[i].DetectedAt.Before(all[j].DetectedAt)
			}
			return all[i].ID < all[j].ID
		})
		return all
	}
	for {
		findings, _, err := read()
		if err == nil {
			for _, finding := range findings {
				known[finding.ID] = finding
			}
			started, allSettled, anyInvestigable := false, true, false
			for _, finding := range findings {
				if !investigable(finding) {
					continue
				}
				anyInvestigable, everInvestigable = true, true
				started = started || strings.TrimSpace(finding.InvestigationStatus) != ""
				allSettled = allSettled && investigationSettled(finding.InvestigationStatus)
			}
			// A finding that left the active list has nothing more to wait
			// for, so only the findings still listed hold the run here.
			if !anyInvestigable || allSettled {
				if !everInvestigable {
					wake.InvestigationStart = ""
				}
				return snapshot(), nil
			}
			if !started && !asked && time.Now().After(startDeadline) {
				asked = true
				wake.InvestigationStart = WakeAsked
				for _, finding := range findings {
					if investigable(finding) {
						// A refusal here is itself the observation: the
						// finding stays without a conclusion and is graded so.
						_ = r.config.Client.Reinvestigate(ctx, finding.ID)
					}
				}
			}
		}
		if time.Now().After(settleDeadline) {
			return snapshot(), nil
		}
		if err := sleepContext(ctx, poll); err != nil {
			return nil, err
		}
	}
}

// decideResponseActions approves and executes exactly one thing: the
// scenario's own fix, on the scenario's own target, bound to the exact plan
// Patrol persisted. Any other proposal is recorded and left alone.
func (r *QualificationRunner) decideResponseActions(ctx context.Context, report *RunReport, prepared *PreparedLab, view labView, findings []Finding, investigations map[string]ResponseFinding, evidence *ResponseEvidence, labUntouched bool) error {
	manifest := r.config.Manifest
	spec := manifest.Remediation
	executed := false
	var firstErr error
	for _, finding := range findings {
		session, ok := report.Investigation[finding.ID]
		if !ok || session.Action == nil {
			continue
		}
		reference := session.Action
		action := &ResponseAction{
			ActionID: reference.ActionID, Capability: reference.CapabilityName, TargetResourceID: reference.ResourceID,
			TargetAlias: view.aliasOfResourceID(manifest, reference.ResourceID), Decision: ActionDecisionNotApproved, FinalState: reference.State,
		}
		entry := investigations[finding.ID]
		entry.Action = action
		investigations[finding.ID] = entry

		detail, err := r.config.Client.Action(ctx, reference.ActionID)
		if err != nil {
			action.DecisionNote = "the action record could not be loaded"
			firstErr = errors.Join(firstErr, fmt.Errorf("load action %s: %w", reference.ActionID, err))
			continue
		}
		audit := detail.Audit.ActionAuditRecord
		action.FinalState = string(audit.State)
		action.PlanHashBound = audit.Plan.PlanHash != "" && audit.Plan.PlanHash == reference.Plan.PlanHash
		action.OriginBound = audit.Origin != nil && audit.Origin.FindingID == finding.ID &&
			audit.Origin.InvestigationID == session.ID && audit.Request.ResourceID == reference.ResourceID
		report.Actions = append(report.Actions, detail.Audit)

		switch {
		case spec == nil:
			action.DecisionNote = "the scenario defines no fix"
		case executed:
			action.DecisionNote = "the scenario's fix was already approved once in this run"
		case action.TargetAlias != spec.ActionTarget || !stringInFold(spec.ExpectedCapabilities, action.Capability):
			action.DecisionNote = fmt.Sprintf("the scenario's fix is %s on %s", strings.Join(spec.ExpectedCapabilities, " or "), spec.ActionTarget)
		case !action.PlanHashBound || (spec.RequireExactOrigin && !action.OriginBound):
			action.DecisionNote = "the action is not bound to this finding, investigation and plan"
		case string(audit.State) != "pending_approval" && string(audit.State) != "planned":
			action.DecisionNote = "the action was not waiting for approval"
		case !labUntouched:
			action.DecisionNote = "the lab changed before any decision, so nothing was approved"
		case !r.config.AuthorizeRemediation:
			action.DecisionNote = "the run was not authorized to decide actions"
		default:
			executed = true
			if _, err := r.config.Client.DecideAction(ctx, reference.ActionID, "approved", spec.DecisionReason, audit.Plan.PlanHash); err != nil {
				action.DecisionNote = "approval was refused"
				firstErr = errors.Join(firstErr, fmt.Errorf("approve action %s: %w", reference.ActionID, err))
				continue
			}
			if _, err := r.config.Client.ExecuteAction(ctx, reference.ActionID, spec.DecisionReason, audit.Plan.PlanHash); err != nil {
				action.DecisionNote = "execution was refused after approval"
				firstErr = errors.Join(firstErr, fmt.Errorf("execute action %s: %w", reference.ActionID, err))
				continue
			}
			action.Decision = ActionDecisionApprovedExecuted
			timeout, _ := positiveDuration(spec.ActionTimeout)
			after, err := r.config.Client.WaitForAction(ctx, reference.ActionID, timeout)
			if err != nil {
				action.DecisionNote = "the action did not reach a final state in time"
				firstErr = errors.Join(firstErr, err)
				continue
			}
			action.FinalState = string(after.Audit.State)
			action.VerificationStatus = string(after.Audit.VerificationOutcome.Status)
			// The lab oracle has the last word, whatever Pulse reported.
			observations, _ := r.config.Lab.Observe(ctx, manifest, prepared, spec.Postconditions)
			evidence.Postconditions = observations
			report.Remediation = RemediationResult{
				FindingID: finding.ID, InvestigationID: session.ID, ActionID: reference.ActionID, ResourceID: reference.ResourceID,
				CapabilityName: reference.CapabilityName, Decision: spec.Decision, Authorized: true,
				OriginBound: action.OriginBound, PlanHashBound: action.PlanHashBound, Before: &detail, After: &after,
				Postconditions: observations, IndependentVerified: allObservationsPassed(observations),
				LifecycleVerified: !spec.RequireLifecycleVerification || stringInFold(spec.AllowedVerificationStatuses, action.VerificationStatus),
			}
			report.Remediation.Passed = report.Remediation.IndependentVerified && report.Remediation.LifecycleVerified && action.FinalState == "completed"
		}
	}
	return firstErr
}
