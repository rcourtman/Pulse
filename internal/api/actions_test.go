package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestHandlePlanActionReturnsCanonicalPlan(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			{
				ID:        "vm:42",
				Type:      unified.ResourceTypeVM,
				Name:      "web-42",
				Status:    unified.StatusWarning,
				LastSeen:  now,
				UpdatedAt: now,
				Sources:   []unified.DataSource{unified.SourceProxmox},
				Capabilities: []unified.ResourceCapability{
					{
						Name:                 "restart",
						Type:                 unified.CapabilityTypeCommon,
						Description:          "Restart the VM",
						MinimumApprovalLevel: unified.ApprovalAdmin,
						InternalHandler:      "proxmox.vm.restart",
						Params: []unified.CapabilityParam{
							{Name: "mode", Type: "string", Required: true, Enum: []string{"graceful", "force"}},
						},
					},
				},
				Relationships: []unified.ResourceRelationship{
					{
						SourceID: "vm:42",
						TargetID: "node-1",
						Type:     unified.RelRunsOn,
						Active:   true,
					},
				},
			},
		},
	})
	body := bytes.NewBufferString(`{
		"requestId":"agent-run-123",
		"resourceId":"vm:42",
		"capabilityName":"restart",
		"params":{"mode":"graceful"},
		"reason":"Recover after confirmed outage",
		"requestedBy":"agent:oncall-helper"
	}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/actions/plan", body)
	h.HandlePlanAction(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "InternalHandler") || strings.Contains(rec.Body.String(), "proxmox.vm.restart") {
		t.Fatalf("response leaked internal execution handler: %s", rec.Body.String())
	}

	var plan unified.ActionPlan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !plan.Allowed {
		t.Fatalf("Allowed = false, want true")
	}
	if !plan.RequiresApproval {
		t.Fatalf("RequiresApproval = false, want true")
	}
	if plan.ApprovalPolicy != unified.ApprovalAdmin {
		t.Fatalf("ApprovalPolicy = %q, want %q", plan.ApprovalPolicy, unified.ApprovalAdmin)
	}
	if plan.ActionID == "" || !strings.HasPrefix(plan.PlanHash, "sha256:") {
		t.Fatalf("missing action identity/hash: actionID=%q planHash=%q", plan.ActionID, plan.PlanHash)
	}
	if plan.Preflight == nil || plan.Preflight.Target != "vm:42" {
		t.Fatalf("Preflight = %#v, want target vm:42", plan.Preflight)
	}
	if len(plan.PredictedBlastRadius) != 2 || plan.PredictedBlastRadius[0] != "vm:42" || plan.PredictedBlastRadius[1] != "node-1" {
		t.Fatalf("PredictedBlastRadius = %#v", plan.PredictedBlastRadius)
	}
}

func TestHandlePlanActionRejectsMissingCapability(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unified.Resource{
			{ID: "vm:42", Type: unified.ResourceTypeVM, Name: "web-42", Status: unified.StatusOnline, LastSeen: now, UpdatedAt: now},
		},
	})
	body := bytes.NewBufferString(`{
		"requestId":"agent-run-123",
		"resourceId":"vm:42",
		"capabilityName":"restart",
		"reason":"Recover after confirmed outage",
		"requestedBy":"agent:oncall-helper"
	}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/actions/plan", body)
	h.HandlePlanAction(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"capability_not_found"`) {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}
