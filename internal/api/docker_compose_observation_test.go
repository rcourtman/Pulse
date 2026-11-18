package api

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	u "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestComposeDeploymentRequiresFreshImageAndHTTP(t *testing.T) {
	now := time.Now().UTC()
	observed := now.Add(-time.Second)
	deployed := now.Add(-time.Minute)
	digest := "sha256:" + strings.Repeat("1", 64)
	receipt, err := composeExecutionResult(agentexec.DockerContainerUpdateResultPayload{ActionID: "review", OperationVersion: 2, SourceStage: "git_proposed", MutationStarted: true, MutationCompleted: true, ProposalBranch: "pulse/service-update/review", ProposalCommit: strings.Repeat("2", 40)})
	if err != nil || receipt.ActionResultV2.Verification.Status != u.ActionVerificationInconclusive || !strings.Contains(receipt.ActionResultV2.Execution.Summary, "pulse/service-update/review") {
		t.Fatalf("proposal truth: %#v, %v", receipt, err)
	}
	a := u.ActionAuditRecord{UpdatedAt: deployed, Request: u.ActionRequest{Params: map[string]any{"afterImage": "example/web:1@" + digest, "composeService": "web"}}, Result: receipt}
	fresh := map[u.DataSource]u.SourceStatus{u.SourceDocker: {Status: "online", LastSeen: observed}}
	ct := u.Resource{Docker: &u.DockerData{ImageID: "config", ContainerState: "running"}, SourceStatus: fresh}
	image := u.Resource{SourceStatus: fresh, Docker: &u.DockerData{RepoDigests: []string{"example/web@" + digest}}}
	inv := composeProjectInventory{services: map[string][]u.Resource{"web": {ct}}, images: map[string]u.Resource{"config": image}}
	check := u.AvailabilityData{Enabled: true, Available: true, Protocol: "http", ApplicationStatusCode: 200, CorrelationState: "attached", LastChecked: &observed}
	run := func(name, wantImage, wantHTTP string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			o := inv.observe(a, now)
			if o == nil || o.Image != wantImage || o.Endpoint != wantHTTP {
				t.Fatalf("observation %#v", o)
			}
		})
	}
	run("no endpoint", "matched", "unconfigured")
	inv.services["web"][0].Availability = &check
	run("verified", "matched", "passed")
	check.Available = false
	check.ApplicationStatusCode = 503
	run("failing after deploy", "matched", "failed")
	check.LastChecked = &deployed
	run("earlier check excluded", "matched", "waiting")
	image.Docker.RepoDigests = []string{"example/web@sha256:old"}
	run("different manifest", "different", "waiting")
	inv.services["web"] = append(inv.services["web"], ct)
	run("ambiguous replica", "waiting", "waiting")
}
