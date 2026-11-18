package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// A source mutation and a deployed application have different observers. The
// receipt stays immutable; the application view joins newer inventory and
// service probes whenever the operator revisits the deployment.
func composeExecutionResult(facts agentexec.DockerContainerUpdateResultPayload) (*unified.ExecutionResult, error) {
	execution := unified.ActionExecutionTruth{Status: unified.ActionExecutionNotRun, ReasonCode: actionPreflightReasonCode(facts.ReasonCode), Summary: "The agent stopped before changing the registered deployment source."}
	if facts.MutationStarted {
		execution = unified.ActionExecutionTruth{Status: unified.ActionExecutionInconclusive, ReasonCode: "possible_partial_effect", Summary: "The source operation may have taken effect. Inspect the registered source before retrying."}
	}
	if facts.MutationCompleted {
		summary := "Compose replaced the reviewed service and the agent observed its approved image digest."
		if facts.SourceStage == "git_proposed" {
			summary = fmt.Sprintf("Published Git branch %s at %s for the deployment owner to review.", facts.ProposalBranch, facts.ProposalCommit)
		}
		execution = unified.ActionExecutionTruth{Status: unified.ActionExecutionSucceeded, Summary: summary}
	}
	if facts.Error != "" {
		execution.Summary += " " + facts.Error
	}
	verification := unified.ActionVerificationTruth{Status: unified.ActionVerificationInconclusive, EvidenceClass: unified.ActionEvidenceNone, ReasonCode: "compose_rollout_observation_pending", Summary: "Open the Compose application to compare the running service image and HTTP checks against this change."}
	recovery := "For a local source change, inspect the registered file and the adjacent .pulse-before-" + facts.ActionID + " copy before restoring the service. Application data requires its own backup."
	if strings.HasPrefix(facts.SourceStage, "git_") {
		recovery = "Close the proposal or revert the merged Git change through the deployment owner. Application data requires its own backup."
	}
	if !facts.MutationStarted {
		recovery = "The registered deployment source was unchanged."
	}
	projected, _, err := unified.ApplyActionResultV2(&unified.ExecutionResult{Output: execution.Summary}, unified.ActionResultV2{Version: unified.ActionResultV2Version, Execution: execution, Verification: verification, Compensation: unified.ActionCompensationTruth{Support: unified.ActionCompensationUnavailable, Status: unified.ActionCompensationNotAvailable, Summary: recovery}})
	if err != nil {
		return nil, err
	}
	projected.Verification = unified.LegacyActionVerificationFromV2(*projected.ActionResultV2)
	return projected, nil
}

type composeDeploymentObservation struct {
	Image      string     `json:"image"`
	Endpoint   string     `json:"endpoint"`
	ObservedAt *time.Time `json:"observedAt,omitempty"`
}
type composeProjectInventory struct {
	services map[string][]unified.Resource
	images   map[string]unified.Resource
}

func composeInventoryForProject(registry *unified.ResourceRegistry, runtime, project string) composeProjectInventory {
	p := composeProjectInventory{services: map[string][]unified.Resource{}, images: map[string]unified.Resource{}}
	if registry == nil {
		return p
	}
	for _, ct := range registry.ListByType(unified.ResourceTypeAppContainer) {
		d := ct.Docker
		if d != nil && d.HostSourceID == runtime && d.ComposeProject == project && d.ComposeService != "" {
			p.services[d.ComposeService] = append(p.services[d.ComposeService], ct)
		}
	}
	for _, image := range registry.ListByType(unified.ResourceTypeDockerImage) {
		if d := image.Docker; d != nil && d.HostSourceID == runtime && d.ImageID != "" {
			p.images[d.ImageID] = image
		}
	}
	return p
}

func composeInventoryFresh(r unified.Resource, after, now time.Time) bool {
	source, ok := r.SourceStatus[unified.SourceDocker]
	return ok && source.Status == "online" && source.LastSeen.After(after) && !source.LastSeen.After(now.Add(5*time.Second))
}

func (p composeProjectInventory) observe(a unified.ActionAuditRecord, now time.Time) *composeDeploymentObservation {
	if a.Result == nil || a.Result.ActionResultV2 == nil || a.Result.ActionResultV2.Execution.Status != unified.ActionExecutionSucceeded {
		return nil
	}
	service, _ := a.Request.Params["composeService"].(string)
	ref, _ := a.Request.Params["afterImage"].(string)
	_, want, ok := strings.Cut(ref, "@")
	if !ok || service == "" {
		return nil
	}
	out := &composeDeploymentObservation{Image: "waiting", Endpoint: "waiting"}
	cts := p.services[service]
	if len(cts) != 1 || !composeInventoryFresh(cts[0], a.UpdatedAt, now) {
		return out
	}
	ct := cts[0]
	d := ct.Docker
	if d == nil || d.ContainerState != "running" {
		out.Image = "not_running"
		return out
	}
	image, ok := p.images[d.ImageID]
	if !ok || image.Docker == nil || !composeInventoryFresh(image, a.UpdatedAt, now) || len(image.Docker.RepoDigests) == 0 {
		return out
	}
	out.Image = "different"
	for _, ref := range image.Docker.RepoDigests {
		if _, digest, ok := strings.Cut(ref, "@"); ok && digest == want {
			out.Image = "matched"
			break
		}
	}
	if out.Image != "matched" {
		return out
	}
	stamp := ct.SourceStatus[unified.SourceDocker].LastSeen.UTC()
	out.ObservedAt = &stamp
	checks := ct.AvailabilityChecks
	if len(checks) == 0 && ct.Availability != nil {
		checks = []unified.AvailabilityData{*ct.Availability}
	}
	after := a.UpdatedAt
	if d.StartedAt != nil && d.StartedAt.After(after) {
		after = *d.StartedAt
	}
	found, pending := false, false
	for _, check := range checks {
		if !check.Enabled || check.CorrelationState != "attached" || (check.Protocol != "http" && check.Protocol != "https") {
			continue
		}
		found = true
		if check.LastChecked == nil || !check.LastChecked.After(after) || check.LastChecked.After(now.Add(5*time.Second)) {
			pending = true
			continue
		}
		cadence := time.Duration(check.PollIntervalSeconds) * time.Second
		if cadence <= 0 {
			cadence = time.Minute
		}
		expires := check.LastChecked.Add(2 * cadence)
		if check.Evidence != nil && check.Evidence.ValidUntil != nil {
			expires = *check.Evidence.ValidUntil
		}
		if now.After(expires) || check.Disagreement {
			pending = true
			continue
		}
		if !check.Available || check.ApplicationOutcome == "failed" {
			out.Endpoint = "failed"
			return out
		}
		if check.ApplicationOutcome != "passed" && check.ApplicationStatusCode == 0 {
			pending = true
		}
	}
	switch {
	case !found:
		out.Endpoint = "unconfigured"
	case !pending:
		out.Endpoint = "passed"
	}
	return out
}
