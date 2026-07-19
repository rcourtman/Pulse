package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type attentionActionTestExecutor struct {
	result *unified.ExecutionResult
	calls  int
}

func (e *attentionActionTestExecutor) ExecuteAction(
	context.Context,
	unified.ActionAuditRecord,
) (*unified.ExecutionResult, error) {
	e.calls++
	return e.result, nil
}

func (*attentionActionTestExecutor) CheckActionAvailable(
	_ context.Context,
	request unified.ActionRequest,
	_ unified.Resource,
) unified.ResourceActionReadiness {
	return unified.ResourceActionReadiness{Name: request.CapabilityName, Available: true}
}

func TestAttentionActionPlanBindsCanonicalRecordEvidenceAndReplaysIdempotently(t *testing.T) {
	now := time.Now().UTC()
	alert := attentionHandlerAlert("docker-health-record", operationaltrust.OperationalOpen, now)
	resourceID := "docker:host-1/container-1"
	alert.Type = ai.AttentionDockerHealthKind
	alert.ResourceID = resourceID
	alert.ResourceName = "API container"
	alert.Metadata = map[string]interface{}{"resourceType": string(unified.ResourceTypeAppContainer)}
	alert.OperationalRecord.SubjectResourceID = resourceID
	alert.Evidence[0].Subject.ResourceID = resourceID

	resources := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	resources.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{{
			ID:        resourceID,
			Type:      unified.ResourceTypeAppContainer,
			Name:      "API container",
			Status:    unified.StatusWarning,
			LastSeen:  now,
			UpdatedAt: now,
			Capabilities: []unified.ResourceCapability{{
				Name:                 ai.AttentionDockerRestartCapability,
				Type:                 unified.CapabilityTypeCommon,
				Description:          "Restart this Docker container.",
				MinimumApprovalLevel: unified.ApprovalAdmin,
				AutoAuthorization:    unified.AutoAuthorizeLowRisk,
				InternalHandler:      ai.AttentionDockerLifecycleHandler,
			}},
		}},
	})
	executor := &attentionActionTestExecutor{result: &unified.ExecutionResult{
		Success: true,
		Output:  "Container restart completed.",
		Verification: &unified.ActionVerificationResult{
			Ran:     true,
			Success: true,
			Note:    "Fresh container readback observed the same container running.",
		},
	}}
	resources.SetActionExecutor(executor)
	authority := testActionAuthority()
	resolved := false
	handler := &AttentionHandlers{
		readAlerts: func(context.Context) ([]alerts.Alert, []alerts.Alert, error) {
			if resolved {
				return nil, []alerts.Alert{alert}, nil
			}
			return []alerts.Alert{alert}, nil, nil
		},
	}
	handler.SetActionDependencies(resources, authority)

	detailRequest := actionHandlerTestRequest(
		httptest.NewRequest(
			http.MethodGet,
			"/api/ai/patrol/attention/docker-health-record",
			nil,
		),
		"operator@example.com",
	)
	detailResponse := httptest.NewRecorder()
	handler.HandleAttention(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", detailResponse.Code, detailResponse.Body.String())
	}
	var detail ai.AttentionItemDetail
	if err := json.Unmarshal(detailResponse.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if len(detail.Item.AvailableActions) != 1 ||
		detail.Item.AvailableActions[0].Capability != ai.AttentionDockerRestartCapability ||
		detail.Item.AvailableActions[0].ActionID != "" {
		t.Fatalf("initial offers = %+v", detail.Item.AvailableActions)
	}

	var first unified.ActionPlan
	for attempt := 0; attempt < 2; attempt++ {
		request := actionHandlerTestRequest(
			httptest.NewRequest(
				http.MethodPost,
				"/api/ai/patrol/attention/docker-health-record/actions/restart/plan",
				nil,
			),
			"operator@example.com",
		)
		response := httptest.NewRecorder()
		handler.HandleAttention(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("attempt %d status=%d body=%s", attempt, response.Code, response.Body.String())
		}
		var plan unified.ActionPlan
		if err := json.Unmarshal(response.Body.Bytes(), &plan); err != nil {
			t.Fatalf("decode plan: %v", err)
		}
		if attempt == 0 {
			first = plan
		} else if plan.ActionID != first.ActionID || plan.PlanHash != first.PlanHash {
			t.Fatalf("replay plan=%+v first=%+v", plan, first)
		}
	}
	if !first.RequiresApproval {
		t.Fatalf("plan = %+v, want explicit approval", first)
	}

	store, err := resources.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}
	reader, ok := store.(unified.OperationalActionAuditOriginReader)
	if !ok {
		t.Fatalf("%T lacks operational action lookup", store)
	}
	audit, found, err := reader.GetLatestActionAuditByOperationalRecord(
		operationalTrustActionOriginSurface,
		"docker-health-record",
	)
	if err != nil || !found {
		t.Fatalf("origin audit found=%t err=%v", found, err)
	}
	if audit.ID != first.ActionID ||
		audit.Origin == nil ||
		audit.Origin.OperationalRecordID != "docker-health-record" ||
		len(audit.Origin.EvidenceIDs) != 1 ||
		audit.Origin.EvidenceIDs[0] != alert.Evidence[0].ID {
		t.Fatalf("audit origin = %+v", audit.Origin)
	}

	decisionRequest := actionHandlerTestRequest(
		httptest.NewRequest(
			http.MethodPost,
			"/api/actions/"+first.ActionID+"/decision",
			bytes.NewBufferString(`{"outcome":"approved","planHash":"`+first.PlanHash+`"}`),
		),
		"operator@example.com",
	)
	decisionRequest.SetPathValue("id", first.ActionID)
	decisionResponse := httptest.NewRecorder()
	resources.HandleDecideAction(decisionResponse, decisionRequest)
	if decisionResponse.Code != http.StatusOK {
		t.Fatalf("decision status=%d body=%s", decisionResponse.Code, decisionResponse.Body.String())
	}
	for attempt := 0; attempt < 2; attempt++ {
		executeRequest := actionHandlerTestRequest(
			httptest.NewRequest(
				http.MethodPost,
				"/api/actions/"+first.ActionID+"/execute",
				bytes.NewBufferString(`{"planHash":"`+first.PlanHash+`"}`),
			),
			"operator@example.com",
		)
		executeRequest.SetPathValue("id", first.ActionID)
		executeResponse := httptest.NewRecorder()
		resources.HandleExecuteAction(executeResponse, executeRequest)
		if executeResponse.Code != http.StatusOK {
			t.Fatalf("execute attempt %d status=%d body=%s", attempt, executeResponse.Code, executeResponse.Body.String())
		}
	}
	if executor.calls != 1 {
		t.Fatalf("executor calls=%d, want exactly one", executor.calls)
	}

	detailResponse = httptest.NewRecorder()
	handler.HandleAttention(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("reloaded detail status=%d body=%s", detailResponse.Code, detailResponse.Body.String())
	}
	if err := json.Unmarshal(detailResponse.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode reloaded detail: %v", err)
	}
	if len(detail.Item.AvailableActions) != 1 ||
		detail.Item.AvailableActions[0].ActionID != first.ActionID ||
		detail.Item.VerificationState != ai.AttentionVerificationSucceeded ||
		detail.Item.State != operationaltrust.OperationalOpen {
		t.Fatalf("reloaded item = %+v", detail.Item)
	}

	// A successful command/readback never closes the operational record by
	// itself. Only a fresh detector-owned recovery observation moves the item
	// to resolved.
	recoveredAt := time.Now().UTC().Add(time.Second)
	alert.OperationalRecord.State = operationaltrust.OperationalResolved
	alert.OperationalRecord.StateChangedAt = recoveredAt
	alert.OperationalRecord.LastObservedAt = recoveredAt
	alert.OperationalRecord.ResolvedAt = &recoveredAt
	alert.Evidence[0].ObservedAt = recoveredAt
	alert.Evidence[0].IngestedAt = recoveredAt
	validUntil := recoveredAt.Add(time.Hour)
	alert.Evidence[0].ValidUntil = &validUntil
	resolved = true
	detailResponse = httptest.NewRecorder()
	handler.HandleAttention(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("resolved detail status=%d body=%s", detailResponse.Code, detailResponse.Body.String())
	}
	if err := json.Unmarshal(detailResponse.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode resolved detail: %v", err)
	}
	if detail.Item.State != operationaltrust.OperationalResolved ||
		detail.Item.VerificationState != ai.AttentionVerificationSucceeded {
		t.Fatalf("resolved item = %+v", detail.Item)
	}
}

func TestAttentionActionOfferFailsClosedWithoutFreshEvidenceOrExecutorReadiness(t *testing.T) {
	now := time.Now().UTC()
	detail := ai.AttentionItemDetail{
		Item: ai.AttentionItem{
			ID:                  "record",
			OperationalRecordID: "record",
			SubjectResourceID:   "docker:host/container",
			SubjectResourceType: string(unified.ResourceTypeAppContainer),
			Kind:                ai.AttentionDockerHealthKind,
			State:               operationaltrust.OperationalOpen,
		},
		Evidence: []operationaltrust.EvidenceEnvelope{{
			ID:           "evidence",
			Subject:      operationaltrust.EvidenceSubject{ResourceID: "docker:host/container"},
			ObservedAt:   now.Add(-time.Hour),
			IngestedAt:   now.Add(-time.Hour),
			Completeness: operationaltrust.EvidenceComplete,
			Confidence:   operationaltrust.EvidenceConfirmed,
			Permissions:  operationaltrust.EvidencePermissionsSufficient,
		}},
	}
	resource := unified.Resource{
		ID:   detail.Item.SubjectResourceID,
		Type: unified.ResourceTypeAppContainer,
		Capabilities: []unified.ResourceCapability{{
			Name:                 ai.AttentionDockerRestartCapability,
			MinimumApprovalLevel: unified.ApprovalAdmin,
			InternalHandler:      ai.AttentionDockerLifecycleHandler,
		}},
	}
	if offer, reason := ai.ProjectAttentionAction(
		&detail,
		ai.AttentionActionCandidate{
			Resource: &resource,
			Readiness: unified.ResourceActionReadiness{
				Name:      ai.AttentionDockerRestartCapability,
				Available: true,
			},
			Authorized: true,
		},
		now,
	); reason != ai.AttentionActionEvidenceStale || offer.Capability != "" {
		t.Fatalf("stale offer=%+v reason=%q", offer, reason)
	}
}
