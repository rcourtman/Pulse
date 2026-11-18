package mock

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	u "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const demoComposeProject = "document-ingest"
const demoComposeCheck = "mock-availability-document-ingest"

// Keep the application on one online Docker host, including in Docker-only estates.
func applyDemoComposeProject(host *models.DockerHost, now time.Time) bool {
	if host.Runtime != "docker" || len(host.Containers) < 2 {
		return false
	}
	host.FailureLogsEnabled = true
	for _, service := range []string{"api", "worker"} {
		name := "document-ingest-" + service
		image := "ghcr.io/pulse-demo/" + name + ":1.4"
		started := now.Add(-30 * time.Minute)
		ct := models.DockerContainer{ID: mockStableHexString(64, host.ID, demoComposeProject, service), Name: name, Image: image, ImageDigest: "sha256:" + mockStableHexString(64, host.ID, image, "image"), State: "running", Status: "Up", Health: "healthy", CreatedAt: started, StartedAt: &started, UptimeSeconds: 1800, ComposeSourceRegistered: true, Labels: map[string]string{"com.docker.compose.project": demoComposeProject, "com.docker.compose.service": service, "io.pulse.demo.compose": demoComposeProject}}
		if service == "worker" {
			ct.Health = "unhealthy"
			ct.FailureLog = &models.DockerContainerFailureLog{Reason: "unhealthy", CapturedAt: now.Add(-2 * time.Minute), Lines: []string{"ingest: cannot write document index: read-only filesystem", "worker: queue paused until index storage is writable"}}
		} else {
			ct.Ports = []models.DockerContainerPort{{PrivatePort: 8080, PublicPort: 8080, Protocol: "tcp", IP: "0.0.0.0"}}
		}
		host.Containers = append(host.Containers, ct)
	}
	return true
}

func demoComposeAPI(resources []u.Resource) *u.Resource {
	for i := range resources {
		d := resources[i].Docker
		if resources[i].Type == u.ResourceTypeAppContainer && d != nil && d.ComposeProject == demoComposeProject && d.ComposeService == "api" {
			return &resources[i]
		}
	}
	return nil
}

func demoComposeEndpoint(api u.Resource, now time.Time) AvailabilityFixture {
	return AvailabilityFixture{Target: normalizeAvailabilityTargetFixture(AvailabilityTargetFixture{ID: demoComposeCheck, Name: "Document ingest readiness", TargetKind: mockAvailabilityTargetService, LinkedResourceID: api.ID, Address: "http://document-ingest.demo.pulse.local", Protocol: "http", Port: 8080, Path: "/health", Enabled: true, PollIntervalSecs: 30, FailureThreshold: 2}), LastChecked: now.Add(-5 * time.Second), LastSuccess: now.Add(-3 * time.Minute), LastError: "HTTP 503", ConsecutiveFailures: 5}
}

func demoComposeDeployment(api u.Resource, now time.Time) ActionFixture {
	d := api.Docker
	digest := "sha256:" + mockStableHexString(64, d.HostSourceID, normalizeMockDockerImageReference(d.Image), "digest")
	planned := now.Add(-20 * time.Minute)
	finished := now.Add(-15 * time.Minute)
	a := newMockActionFixture(mockActionFixtureSpec{ID: "demo-document-ingest-deploy", ResourceID: api.ID, CapabilityName: "update", Reason: "Pin the reviewed API image in the document-ingest deployment source.", RequestedBy: "demo-operator", PlannedAt: planned, ExpiresAt: now.Add(40 * time.Minute), ApprovalFloor: u.ApprovalAdmin, AutoClass: u.PolicyReasonCapabilityAutoNever, CurrentState: "Document ingest uses its registered Compose source.", IntendedChange: "Propose the reviewed API digest on the deployment repository.", SafetyChecks: []string{"The registered source revision matches the review"}, Verification: []string{"Compare runtime image inventory and the HTTP readiness check"}})
	a = approveMockAction(a, "operator@example.com", u.OutcomeApproved, planned.Add(time.Minute))
	a.Audit.Request.Params = map[string]any{"composeRuntime": d.HostSourceID, "composeProject": demoComposeProject, "composeService": "api", "sourceHash": mockActionRevision("sha256:", demoComposeProject), "afterImage": d.Image + "@" + digest}
	a.Audit.State = u.ActionStateCompleted
	a.Audit.UpdatedAt = finished
	a.Audit.Result = &u.ExecutionResult{ActionResultV2: &u.ActionResultV2{Version: u.ActionResultV2Version, Execution: u.ActionExecutionTruth{Status: u.ActionExecutionSucceeded, Summary: "Published Git branch pulse/service-update/demo-document-ingest-deploy for review."}, Verification: u.ActionVerificationTruth{Status: u.ActionVerificationInconclusive, EvidenceClass: u.ActionEvidenceNone, ReasonCode: "compose_rollout_observation_pending", Summary: "The application shows the current image and HTTP observations."}, Compensation: u.ActionCompensationTruth{Support: u.ActionCompensationUnavailable, Status: u.ActionCompensationNotAvailable, Summary: "Revert the merged change in Git. Index data has a separate backup."}}}
	a.Events = append(a.Events, mockActionTransition(a.Audit, u.ActionStateExecuting, finished.Add(-time.Second), "Publishing the reviewed source change."), mockActionTransition(a.Audit, u.ActionStateCompleted, finished, "Git proposal published."))
	return mustNormalizeMockActionFixture(a)
}
