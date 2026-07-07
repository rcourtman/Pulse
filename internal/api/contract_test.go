package api

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/actionplanner"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/correlation"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	authpkg "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/cloudauth"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	licensetestsupport "github.com/rcourtman/pulse-go-rewrite/pkg/licensing/testsupport"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
	"github.com/rs/zerolog"
	tmock "github.com/stretchr/testify/mock"
)

type resourceContractSnapshot struct {
	ID   string
	Name string
	Type string
}

func TestContract_AlertDeliveryDiagnosisRouteIsReadOnlyMonitoringRead(t *testing.T) {
	source, err := os.ReadFile("alerts.go")
	if err != nil {
		t.Fatalf("read alerts.go: %v", err)
	}
	src := string(source)

	required := []string{
		`case path == "delivery-diagnosis" && r.Method == http.MethodGet:`,
		`ensureScope(w, r, config.ScopeMonitoringRead)`,
		`h.GetAlertDeliveryDiagnosis(w, r)`,
		`manager.DiagnoseAlertDelivery(alertIdentifier)`,
	}
	for _, snippet := range required {
		if !strings.Contains(src, snippet) {
			t.Fatalf("alert delivery diagnosis route contract missing %q", snippet)
		}
	}
}

func TestContract_AlertDeliveryDiagnosisPayloadShape(t *testing.T) {
	replayAt := time.Date(2026, 6, 29, 12, 30, 0, 0, time.UTC)
	diagnosis := alerts.AlertDeliveryDiagnosis{
		AlertIdentifier:         "canonical:a1",
		AlertID:                 "a1",
		TrackingKey:             "node/a1/cpu",
		Status:                  alerts.AlertDeliveryStatusDeferred,
		Reason:                  alerts.AlertDeliveryReasonQuietHours + ":performance",
		Message:                 "Alert delivery is deferred by quiet-hours policy and will be replayed later.",
		AlertType:               "cpu",
		Level:                   alerts.AlertLevelCritical,
		ResourceID:              "node-1",
		ResourceName:            "node-1",
		NotificationsEnabled:    true,
		ActivationState:         string(alerts.ActivationActive),
		CooldownMinutes:         5,
		MaxAlertsHour:           10,
		RecentAlertsInHour:      2,
		FlappingActive:          false,
		FlappingHistoryInWindow: 1,
		FlappingThreshold:       3,
		FlappingWindowSeconds:   300,
		QuietHoursReplayAt:      &replayAt,
	}

	payload, err := json.Marshal(diagnosis)
	if err != nil {
		t.Fatalf("marshal alert delivery diagnosis: %v", err)
	}
	body := string(payload)
	for _, field := range []string{
		`"alertIdentifier":"canonical:a1"`,
		`"trackingKey":"node/a1/cpu"`,
		`"status":"deferred"`,
		`"reason":"quiet_hours:performance"`,
		`"notificationsEnabled":true`,
		`"cooldownMinutes":5`,
		`"recentAlertsInHour":2`,
		`"flappingHistoryInWindow":1`,
		`"quietHoursReplayAt":"2026-06-29T12:30:00Z"`,
	} {
		if !strings.Contains(body, field) {
			t.Fatalf("alert delivery diagnosis payload missing %s in %s", field, body)
		}
	}
}

func TestContract_ReportSchedulePayloadShape(t *testing.T) {
	nextRun := time.Date(2026, 7, 8, 6, 0, 0, 0, time.UTC)
	schedule := config.ReportSchedule{
		ID:      "monthly-acme",
		Name:    "Acme monthly report",
		Enabled: true,
		Cadence: config.ReportScheduleCadence{
			Type:       config.ReportScheduleCadenceMonthly,
			DayOfMonth: 1,
			Time:       "06:00",
			Timezone:   "Europe/London",
		},
		Scope: config.ReportScheduleScope{
			Resources: []config.ReportScheduleResource{{
				ResourceType: "agent",
				ResourceID:   "agent-1",
				Name:         "pve-a",
			}},
			Tags: []string{"client:acme"},
		},
		Format: config.ReportScheduleFormatPDF,
		Delivery: config.ReportScheduleDelivery{
			Method:     config.ReportScheduleDeliveryEmail,
			To:         []string{"ops@example.com"},
			Attach:     true,
			SaveToDisk: true,
		},
		RetentionCount: 12,
		LastRunStatus:  config.ReportScheduleLastRunOK,
		NextRunAt:      &nextRun,
		CreatedAt:      time.Date(2026, 7, 7, 6, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 7, 7, 6, 30, 0, 0, time.UTC),
	}

	payload, err := json.Marshal(config.ReportScheduleStore{Schedules: []config.ReportSchedule{schedule}})
	if err != nil {
		t.Fatalf("marshal report schedule contract: %v", err)
	}
	body := string(payload)
	for _, field := range []string{
		`"schedules":[`,
		`"id":"monthly-acme"`,
		`"enabled":true`,
		`"type":"monthly"`,
		`"day_of_month":1`,
		`"timezone":"Europe/London"`,
		`"resourceType":"agent"`,
		`"resourceId":"agent-1"`,
		`"tags":["client:acme"]`,
		`"format":"pdf"`,
		`"method":"email"`,
		`"to":["ops@example.com"]`,
		`"attach":true`,
		`"save_to_disk":true`,
		`"retention_count":12`,
		`"last_run_status":"ok"`,
		`"next_run_at":"2026-07-08T06:00:00Z"`,
	} {
		if !strings.Contains(body, field) {
			t.Fatalf("report schedule payload missing %s in %s", field, body)
		}
	}
}

func TestContract_PMGInstancesEndpointUsesUnifiedReadStatePayload(t *testing.T) {
	now := time.Now().UTC()
	source := models.PMGInstance{
		ID:       "pmg-contract-1",
		Name:     "gateway-contract",
		Host:     "https://pmg-contract.example.com:8006",
		GuestURL: "https://pmg-contract.example.com/quarantine",
		Status:   "online",
		Version:  "8.2",
		Nodes: []models.PMGNodeStatus{{
			Name:   "pmg-node-contract",
			Status: "online",
			QueueStatus: &models.PMGQueueStatus{
				Incoming:  4,
				Total:     10,
				OldestAge: 600,
				UpdatedAt: now,
			},
		}},
		MailStats: &models.PMGMailStats{
			CountTotal:      1000,
			JunkIn:          10,
			PregreetRejects: 13,
			UpdatedAt:       now,
		},
		MailCount:        []models.PMGMailCountPoint{{Timestamp: now, Count: 1000, Timeframe: "hour", Index: 1}},
		DomainStats:      []models.PMGDomainStat{{Domain: "example.com", MailCount: 100, Bytes: 2048}},
		DomainStatsAsOf:  now,
		ConnectionHealth: "connected",
		LastSeen:         now,
		LastUpdated:      now,
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{PMGInstances: []models.PMGInstance{source}})
	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "resourceStore", unifiedresources.NewMonitorAdapter(registry))

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/pmg/instances?id=pmg-contract-1", nil)
	rec := httptest.NewRecorder()
	router.handleListPMGInstances(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp PMGInstancesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode PMG contract response: %v", err)
	}
	if resp.Meta.Total != 1 || len(resp.Data) != 1 {
		t.Fatalf("expected one PMG instance, got total=%d len=%d", resp.Meta.Total, len(resp.Data))
	}
	got := resp.Data[0]
	if got.ID != source.ID || got.Host != source.Host || got.GuestURL != source.GuestURL {
		t.Fatalf("PMG identity and URL contract not preserved: %+v", got)
	}
	if len(got.Nodes) != 1 || got.Nodes[0].QueueStatus == nil || got.Nodes[0].QueueStatus.OldestAge != 600 {
		t.Fatalf("PMG queue contract not preserved: %+v", got.Nodes)
	}
	if got.MailStats == nil || got.MailStats.CountTotal != 1000 || got.MailStats.JunkIn != 10 || got.MailStats.PregreetRejects != 13 {
		t.Fatalf("PMG mail stats contract not preserved: %+v", got.MailStats)
	}
	if len(got.MailCount) != 1 || got.MailCount[0].Index != 1 || len(got.DomainStats) != 1 || got.DomainStats[0].Bytes != 2048 {
		t.Fatalf("PMG historical/domain contract not preserved: mail=%+v domains=%+v", got.MailCount, got.DomainStats)
	}
}

func TestContract_MockAvailabilityTargetsUseSavedTargetPayloads(t *testing.T) {
	previousMock := mock.IsMockEnabled()
	if err := mock.SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() { _ = mock.SetEnabled(previousMock) })

	targets := mockAvailabilityTargetResponses()
	if len(targets) < 4 {
		t.Fatalf("expected mock availability target payloads, got %d", len(targets))
	}

	seen := map[string]availabilityTargetResponse{}
	for _, target := range targets {
		if target.ID == "" {
			t.Fatal("mock availability target payload must keep a saved target id")
		}
		if target.Status == nil {
			t.Fatalf("mock availability target %q must include probe status", target.ID)
		}
		if target.Status.TargetID != target.ID {
			t.Fatalf("mock availability target %q status target id drifted to %q", target.ID, target.Status.TargetID)
		}
		seen[target.ID] = target
	}

	mqtt, ok := seen["mock-availability-mqtt-meter"]
	if !ok {
		t.Fatal("mock MQTT availability target missing from API payload contract")
	}
	if mqtt.Protocol != config.AvailabilityProbeTCP || mqtt.Port != 1883 {
		t.Fatalf("mock MQTT availability target must remain a saved TCP:1883 target, got protocol=%q port=%v", mqtt.Protocol, mqtt.Port)
	}

	response, ok := mockAvailabilityTestResponse("mock-availability-mqtt-meter")
	if !ok {
		t.Fatal("saved mock availability test response missing")
	}
	if !response.Success || response.LatencyMillis <= 0 || response.Error != "" {
		t.Fatalf("mock MQTT saved-test response must be successful with latency and no error: %+v", response)
	}
}

func TestContract_MockDiscoveryEndpointsUseCanonicalPayloads(t *testing.T) {
	previousMock := mock.IsMockEnabled()
	if err := mock.SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() { _ = mock.SetEnabled(previousMock) })

	handlers := NewDiscoveryHandlers(nil, nil)
	listReq := httptest.NewRequest(http.MethodGet, "/api/discovery", nil)
	listRec := httptest.NewRecorder()
	handlers.HandleListDiscoveries(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("/api/discovery status = %d, body=%s", listRec.Code, listRec.Body.String())
	}

	var listBody struct {
		Discoveries []map[string]any `json:"discoveries"`
		Total       int              `json:"total"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode discovery list: %v", err)
	}
	if listBody.Total == 0 || len(listBody.Discoveries) == 0 {
		t.Fatal("expected mock discovery list to expose canonical discovery summaries")
	}

	var dockerDetail map[string]any
	for _, summary := range listBody.Discoveries {
		if summary["resource_type"] != "docker" {
			continue
		}
		resourceType, _ := summary["resource_type"].(string)
		targetID, _ := summary["target_id"].(string)
		resourceID, _ := summary["resource_id"].(string)
		if resourceType == "" || targetID == "" || resourceID == "" {
			t.Fatalf("mock discovery summary lost canonical identity: %#v", summary)
		}

		detailReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/discovery/%s/%s/%s", resourceType, targetID, resourceID), nil)
		detailRec := httptest.NewRecorder()
		handlers.HandleGetDiscovery(detailRec, detailReq)
		if detailRec.Code != http.StatusOK {
			t.Fatalf("discovery detail status = %d, body=%s", detailRec.Code, detailRec.Body.String())
		}

		var detail map[string]any
		if err := json.NewDecoder(detailRec.Body).Decode(&detail); err != nil {
			t.Fatalf("decode discovery detail: %v", err)
		}
		if !discoveryContractValueEmpty(detail["suggested_url"]) {
			dockerDetail = detail
			break
		}
	}
	if dockerDetail == nil {
		t.Fatalf("expected mock discovery summaries to include Docker workload URL context: %#v", listBody.Discoveries)
	}
	for _, key := range []string{"service_name", "service_version", "config_paths", "ports", "suggested_url", "cli_access_version"} {
		value, ok := dockerDetail[key]
		if !ok || discoveryContractValueEmpty(value) {
			t.Fatalf("mock discovery detail missing %s: %#v", key, dockerDetail)
		}
	}
	if _, ok := dockerDetail["raw_command_output"]; ok {
		t.Fatalf("mock discovery detail exposed raw command output: %#v", dockerDetail)
	}
}

func discoveryContractValueEmpty(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case float64:
		return typed == 0
	default:
		return false
	}
}

func TestPatrolRemediationCommercialCopyUsesSafeRemediationWording(t *testing.T) {
	files := []string{"ai_handlers.go", "router_routes_ai_relay.go"}
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := string(source)
		if strings.Contains(text, "Pulse Patrol Auto-Fix requires Pulse Pro") ||
			strings.Contains(text, "Auto-Fix requires Pulse Pro") {
			t.Fatalf("%s must not expose legacy Auto-Fix license copy", file)
		}
	}
}

func TestContract_PatrolAutonomyCommunityMonitorUpdatePayload(t *testing.T) {
	rawToken := "patrol-autonomy-contract-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	body := `{"autonomy_level":"monitor","full_mode_unlocked":true,"investigation_budget":2,"investigation_timeout_sec":30}`
	req := httptest.NewRequest(http.MethodPut, "/api/ai/patrol/autonomy", strings.NewReader(body))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected monitor autonomy update to be allowed, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Success  bool                   `json:"success"`
		Settings PatrolAutonomyResponse `json:"settings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode monitor update response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response, got %+v", resp)
	}
	if resp.Settings.AutonomyLevel != config.PatrolAutonomyMonitor ||
		resp.Settings.FullModeUnlocked ||
		resp.Settings.InvestigationBudget != 5 ||
		resp.Settings.InvestigationTimeoutSec != 60 {
		t.Fatalf("unexpected monitor update settings: %+v", resp.Settings)
	}

	premiumBody := `{"autonomy_level":"approval","investigation_budget":10,"investigation_timeout_sec":120}`
	premiumReq := httptest.NewRequest(http.MethodPut, "/api/ai/patrol/autonomy", strings.NewReader(premiumBody))
	premiumReq.Header.Set("X-API-Token", rawToken)
	premiumRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(premiumRec, premiumReq)

	if premiumRec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected premium autonomy to require license, got %d: %s", premiumRec.Code, premiumRec.Body.String())
	}
	if !strings.Contains(premiumRec.Body.String(), "limited to Monitor") {
		t.Fatalf("expected premium autonomy payload to explain Community monitor limit, got %s", premiumRec.Body.String())
	}
}

func TestContract_AssistantFindingContextUsesModelOnlyHandoff(t *testing.T) {
	handlerSource, err := os.ReadFile(filepath.Clean("ai_handler.go"))
	if err != nil {
		t.Fatalf("read ai_handler.go: %v", err)
	}
	settingsHandlerSource, err := os.ReadFile(filepath.Clean("ai_handlers.go"))
	if err != nil {
		t.Fatalf("read ai_handlers.go: %v", err)
	}
	chatServiceSource, err := os.ReadFile(filepath.Clean("../ai/chat/service.go"))
	if err != nil {
		t.Fatalf("read chat service: %v", err)
	}
	chatSessionSource, err := os.ReadFile(filepath.Clean("../ai/chat/session.go"))
	if err != nil {
		t.Fatalf("read chat session store: %v", err)
	}
	chatTypesSource, err := os.ReadFile(filepath.Clean("../ai/chat/types.go"))
	if err != nil {
		t.Fatalf("read chat types: %v", err)
	}
	patrolHandoffSource, err := os.ReadFile(filepath.Clean("../ai/patrol_assistant_handoff.go"))
	if err != nil {
		t.Fatalf("read Patrol Assistant handoff model: %v", err)
	}
	toolsQuerySource, err := os.ReadFile(filepath.Clean("../ai/tools/tools_query.go"))
	if err != nil {
		t.Fatalf("read AI tools query runtime: %v", err)
	}

	handlerText := string(handlerSource)
	for _, required := range []string{
		"requestSuppliedSessionID := requestSessionID != \"\"",
		`if findingID == "" && requestSuppliedSessionID`,
		`svc.GetModelHandoffFindingID(ctx, requestSessionID)`,
		`svc.GetModelHandoffMetadata(ctx, requestSessionID)`,
		`if requestSuppliedSessionID`,
		`svc.ClearModelHandoffContext(ctx, requestSessionID)`,
		`HandoffActions   []chat.HandoffAction   ` + "`json:\"handoff_actions,omitempty\"`",
		`HandoffMetadata  chat.HandoffMetadata   ` + "`json:\"handoff_metadata,omitempty\"`",
		"handoffActions := normalizeChatRequestHandoffActions(req.HandoffActions)",
		"handoffMetadata := chat.NormalizeHandoffMetadata(req.HandoffMetadata)",
		"storedMetadata = chat.NormalizeHandoffMetadata(storedMetadata)",
		`if storedMetadata.Kind == "patrol_run"`,
		"handoffMetadata = storedMetadata",
		"h.getPatrolRunForHandoff(ctx, handoffMetadata.RunID)",
		"airuntime.BuildPatrolRunAssistantHandoff(run)",
		"chatRequestHandoffActionLimit",
		"requestHandoffContext := handoffContext",
		"requestHandoffResources := handoffResources",
		"requestHandoffActions := handoffActions",
		"handoffActions = mergeUnifiedFindingRequestHandoffActions(f, buildUnifiedFindingHandoffActions(f, orgID), requestHandoffActions)",
		"handoffContext = mergeUnifiedFindingRequestHandoffContext(buildUnifiedFindingChatContext(f, store, handoffActions), requestHandoffContext, f.ID)",
		"handoffResources = mergeUnifiedFindingRequestHandoffResources(f, buildUnifiedFindingHandoffResources(f, store), requestHandoffResources)",
		"safePatrolFindingRequestHandoffContext",
		"Product Handoff Boundary",
		"livePatrolApprovalForFinding(f.ID, orgID)",
		"chat.HydrateHandoffActionFromApproval(&action, liveApproval)",
		"ApprovalRequestedAt",
		"ApprovalExpiresAt",
		"func chatAutonomousModeForFindingHandoff(requested *bool, findingID, handoffContext string, handoffResources []chat.HandoffResource, handoffActions []chat.HandoffAction, handoffMetadata chat.HandoffMetadata) *bool",
		`if strings.TrimSpace(findingID) != ""`,
		"chatApprovalRequiredAutonomousMode()",
		"AutonomousMode:       chatAutonomousModeForFindingHandoff(req.AutonomousMode, findingID, handoffContext, handoffResources, handoffActions, handoffMetadata)",
		`appendChatContextLine(&b, "Finding Status", unifiedFindingChatStatus(f, time.Now()))`,
		`appendChatContextLine(&b, "Finding Detected At", f.DetectedAt.Format(time.RFC3339))`,
		`appendChatContextLine(&b, "Finding Last Seen At", f.LastSeenAt.Format(time.RFC3339))`,
		`appendChatContextLine(&b, "Finding Resolved At", f.ResolvedAt.Format(time.RFC3339))`,
		`appendChatContextLine(&b, "Finding Snoozed Until", f.SnoozedUntil.Format(time.RFC3339))`,
		`appendChatContextLine(&b, "Finding Dismissed Reason", f.DismissedReason)`,
		`appendChatContextLine(&b, "Finding Suppressed", "true")`,
		`appendChatContextLine(&b, "Finding Times Raised", strconv.Itoa(f.TimesRaised))`,
		`appendChatContextLine(&b, "AI Context", f.AIContext)`,
		`appendChatContextLine(&b, "Root Cause ID", f.RootCauseID)`,
		`appendStringListChatContext(&b, "Correlated Finding", f.CorrelatedIDs)`,
		"appendUnifiedFindingRelatedContext(&b, f, lookup)",
		"[Related Finding Context]",
		"Related Finding Boundary",
		"formatUnifiedFindingBriefingRecency(f)",
		"latest lifecycle",
		`appendChatContextLine(&b, "Remediation ID", f.RemediationID)`,
		`appendChatContextLine(&b, "Last Investigated At", f.LastInvestigatedAt.Format(time.RFC3339))`,
		`appendChatContextLine(&b, "Investigation Attempts", strconv.Itoa(f.InvestigationAttempts))`,
		`appendChatContextLine(&b, "Loop State", f.LoopState)`,
		`appendChatContextLine(&b, "Regression Count", strconv.Itoa(f.RegressionCount))`,
		`appendChatContextLine(&b, "Last Regression At", f.LastRegressionAt.Format(time.RFC3339))`,
		"appendUnifiedFindingLifecycleEventContext(&b, f.Lifecycle)",
		"formatUnifiedFindingLatestLifecycleEvent(f.Lifecycle)",
		"[Finding Lifecycle Context]",
		"Lifecycle Boundary",
		"appendUnifiedFindingModelBriefingContext(&b, f, handoffActions)",
		"[Finding Briefing]",
		`appendChatContextLine(b, "Recency", formatUnifiedFindingBriefingRecencyFacts(f))`,
		"formatUnifiedFindingBriefingRecencyFacts(f)",
		`appendChatContextLine(b, "Evidence Snapshot", evidence)`,
		`appendChatContextLine(b, "Verification", verification)`,
		"formatInvestigationRecordEvidenceBriefing",
		"formatBriefingStringList",
		`appendChatContextLine(b, "Governed Action Context", actionContext)`,
		"unifiedFindingBriefingActionContext(f, handoffActions)",
		"unifiedFindingPrimaryHandoffAction(f, handoffActions)",
		"Latest Lifecycle Event",
		"Model Boundary",
		`return "suppressed"`,
		"Prompt:",
		"HandoffContext:       handoffContext",
		"HandoffResources:     handoffResources",
		"HandoffActions:       handoffActions",
		"HandoffMetadata:      handoffMetadata",
		"SuppressSessionEvent: true",
	} {
		if !strings.Contains(handlerText, required) {
			t.Fatalf("ai_handler.go must preserve model-only finding handoff contract: missing %q", required)
		}
	}
	if strings.Contains(handlerText, "prompt = findingCtx +") {
		t.Fatal("ai_handler.go must not prepend finding context into the persisted prompt")
	}
	for _, forbidden := range []string{
		"Recommended Next Step",
		"unifiedFindingBriefingNextStep",
	} {
		if strings.Contains(handlerText, forbidden) {
			t.Fatalf("ai_handler.go must not synthesize product-authored handoff recommendations: found %q", forbidden)
		}
	}

	settingsHandlerText := string(settingsHandlerSource)
	for _, required := range []string{
		"actionAuditStore: chatService.GetActionAuditStore()",
		"tools.AttachApprovalActionPlan(req, time.Now().UTC())",
		"tools.RecordPendingApprovalAction(a.actionAuditStore, req)",
	} {
		if !strings.Contains(settingsHandlerText, required) {
			t.Fatalf("ai_handlers.go must preserve Patrol queued-fix action-audit seeding: missing %q", required)
		}
	}

	patrolHandoffText := string(patrolHandoffSource)
	for _, required := range []string{
		"func BuildPatrolRunAssistantHandoff(run PatrolRunRecord) PatrolRunAssistantHandoff",
		"[Patrol Run Context]",
		"Source: Pulse Patrol run history",
		"patrolRunAssistantHandoffResources",
		"sanitizePatrolRunAnalysis",
		"Model Boundary",
	} {
		if !strings.Contains(patrolHandoffText, required) {
			t.Fatalf("Patrol runtime must own safe Assistant run handoff context: missing %q", required)
		}
	}

	chatServiceText := string(chatServiceSource)
	for _, required := range []string{
		"handoffContext := strings.TrimSpace(req.HandoffContext)",
		"handoffFindingID := strings.TrimSpace(req.FindingID)",
		"handoffMetadata := NormalizeHandoffMetadata(req.HandoffMetadata)",
		"handoffMetadataCarriesEnvelope",
		"handoffMetadata.Kind == sessionHandoffKindResourceContext",
		"hasRequestHandoffEnvelope",
		"sessions.SetModelHandoffEnvelope(session.ID, handoffFindingID, handoffContext, handoffResources, handoffActions, handoffMetadata)",
		"ClearModelHandoffContext",
		"handoffResources := normalizeHandoffResources(req.HandoffResources)",
		"GetModelHandoffFindingID",
		"GetModelHandoffMetadata",
		"sessions.GetModelHandoffEnvelope(session.ID)",
		"handoffContext = storedContext",
		"handoffResources = storedResources",
		"sessions.SetModelHandoffActions(session.ID, handoffActions)",
		"handoffActions = storedActions",
		"GetActionAuditStore",
		"refreshHandoffActionStatus(handoffActions, s.orgID, s.actionAuditStore)",
		"HydrateHandoffActionFromApproval(&actions[idx], req)",
		"func HydrateHandoffActionFromApproval(action *HandoffAction, req *approval.ApprovalRequest)",
		"approval.RequesterForRequest(req)",
		"action.ActionRequestedBy = requestedBy",
		"mergeHandoffResourcePolicyContext(handoffContext, handoffResources, handoffResourceProvider)",
		"mergeHandoffResourceStateContext(handoffContext, handoffResources, handoffResourceProvider)",
		"mergeHandoffResourceRelationshipContext(handoffContext, handoffResources, handoffResourceProvider)",
		"mergeHandoffResourceTimelineContext(handoffContext, handoffResources, handoffResourceProvider, s.actionAuditStore, time.Now())",
		"handoffContext = mergeHandoffActionContext(handoffContext, handoffActions)",
		"sanitizeHandoffContextForResourcePolicy(handoffContext, handoffResources, handoffResourceProvider)",
		"Action Reference",
		"Action State",
		"Action Plan Expires At",
		"Action Dry Run Summary",
		"Action Result",
		"s.hydrateHandoffResources(session.ID, handoffResources, sessions, unifiedResourceProvider)",
		"injectHandoffContextIntoLatestUserMessage(messages, handoffContext)",
		"Resource Policy Context",
		"Policy Boundary",
		"Resource State Context",
		"labelPrefix+\" Status\"",
		"labelPrefix+\" Capabilities\"",
		"Resource State Boundary",
		"Resource Relationship Context",
		"Relationship Boundary",
		"Timeline Boundary",
		"Approval Status",
		"Action Boundary",
		"User message: ",
	} {
		if !strings.Contains(chatServiceText, required) {
			t.Fatalf("chat service must inject handoff context into model-only turn: missing %q", required)
		}
	}

	chatSessionText := string(chatSessionSource)
	if !regexp.MustCompile(`ModelContext\s+\*sessionModelContext`).MatchString(chatSessionText) {
		t.Fatalf("chat session store must persist model-only handoff metadata outside messages: missing %q", "ModelContext *sessionModelContext")
	}
	for _, required := range []string{
		"SetModelHandoffFindingID",
		"SetModelHandoffEnvelope",
		"GetModelHandoffFindingID",
		"GetModelHandoffMetadata",
		"ClearModelHandoffContext",
		"HandoffFindingID string",
		"SetModelHandoffContext",
		"GetModelHandoffContext",
		"GetModelHandoffEnvelope",
		"SetModelHandoffResources",
		"GetModelHandoffResources",
		"HandoffResources []HandoffResource",
		"SetModelHandoffActions",
		"GetModelHandoffActions",
		"HandoffActions   []HandoffAction",
		"SetModelHandoffMetadata",
		"HandoffMetadata  HandoffMetadata",
	} {
		if !strings.Contains(chatSessionText, required) {
			t.Fatalf("chat session store must persist model-only handoff metadata outside messages: missing %q", required)
		}
	}

	chatTypesText := string(chatTypesSource)
	for _, required := range []string{
		"type HandoffResource struct",
		"HandoffResources []HandoffResource",
		"type HandoffAction struct",
		"HandoffActions   []HandoffAction",
		"type HandoffMetadata struct",
		"HandoffMetadata  HandoffMetadata",
		"ApprovalStatus",
		"ActionID",
		"ActionState",
		"ActionRequestedBy",
		"ActionPlanExpiresAt",
		"ActionDryRunSummary",
		"ActionResult",
	} {
		if !strings.Contains(chatTypesText, required) {
			t.Fatalf("chat request must carry structured handoff resource scope outside messages: missing %q", required)
		}
	}

	toolsQueryText := string(toolsQuerySource)
	if !strings.Contains(toolsQueryText, "func CanonicalHandoffResourceRegistration(provider UnifiedResourceProvider") {
		t.Fatal("AI tools runtime must own canonical handoff resource registration")
	}
	if !strings.Contains(toolsQueryText, "func CanonicalHandoffUnifiedResource(provider UnifiedResourceProvider") {
		t.Fatal("AI tools runtime must own canonical handoff unified resource resolution")
	}
}

func TestContract_AssistantMessageHistoryUsesClientSafeProjection(t *testing.T) {
	handlerSource, err := os.ReadFile(filepath.Clean("ai_handler.go"))
	if err != nil {
		t.Fatalf("read ai_handler.go: %v", err)
	}
	serviceSource, err := os.ReadFile(filepath.Clean("../ai/chat/service.go"))
	if err != nil {
		t.Fatalf("read chat service: %v", err)
	}
	typesSource, err := os.ReadFile(filepath.Clean("../ai/chat/types.go"))
	if err != nil {
		t.Fatalf("read chat types: %v", err)
	}

	handlerText := string(handlerSource)
	for _, required := range []string{
		"func (h *AIHandler) HandleMessages(w http.ResponseWriter, r *http.Request, sessionID string)",
		"messages[i] = messages[i].ClientSafe()",
		"json.NewEncoder(w).Encode(messages)",
	} {
		if !strings.Contains(handlerText, required) {
			t.Fatalf("Assistant message-history handler must encode client-safe transcript projection: missing %q", required)
		}
	}

	serviceText := string(serviceSource)
	for _, required := range []string{
		"func (s *Service) GetMessages(ctx context.Context, sessionID string) ([]Message, error)",
		"messages[i] = messages[i].ClientSafe()",
	} {
		if !strings.Contains(serviceText, required) {
			t.Fatalf("chat service message-history reads must return client-safe transcript projection: missing %q", required)
		}
	}

	typesText := string(typesSource)
	for _, required := range []string{
		"func (m Message) ClientSafe() Message",
		"m = m.NormalizeCollections()",
		`if strings.EqualFold(m.Role, "assistant")`,
		"m.Content = cleanToolCallArtifacts(m.Content)",
		`m.ReasoningContent = ""`,
		"return m",
	} {
		if !strings.Contains(typesText, required) {
			t.Fatalf("chat message client-safe projection must strip hidden reasoning and raw tool-call prose: missing %q", required)
		}
	}
}

func TestContract_InvestigateAlertIsApprovalBound(t *testing.T) {
	source, err := os.ReadFile(filepath.Clean("ai_handlers.go"))
	if err != nil {
		t.Fatalf("read ai_handlers.go: %v", err)
	}
	text := string(source)

	if !strings.Contains(text, "autonomousMode := false") ||
		!strings.Contains(text, "AutonomousMode:         &autonomousMode") ||
		!strings.Contains(text, "RequireCommandApproval: true") {
		t.Fatal("investigate-alert API handoff must force request-scoped approval and command approval")
	}
}

func TestContract_ProxmoxSetupScriptUsesPrivilegeSeparatedTokenACLs(t *testing.T) {
	storagePerms := `
pveum aclmod /storage -user pulse-monitor@pve -role PVEDatastoreAdmin
if [ "$TOKEN_CREATED" = true ]; then
    pveum aclmod /storage -token "$PULSE_TOKEN_ID" -role PVEDatastoreAdmin
fi`
	pveArtifact := buildSetupScriptInstallArtifact(
		"https://pulse.example",
		"pve",
		"https://pve.example:8006",
		"https://pulse.example",
		true,
		"setup-token-123",
		0,
	)
	pveScript := renderSetupScript("pve", setupScriptRenderContext{
		ServerName:       "pve-example",
		PulseURL:         "https://pulse.example",
		ServerHost:       "https://pve.example:8006",
		SetupToken:       "setup-token-123",
		TokenName:        "pulse-example",
		TokenMatchPrefix: "pulse-example",
		StoragePerms:     storagePerms,
		SensorsPublicKey: "ssh-ed25519 AAAATEST pulse@test",
		Artifact:         pveArtifact,
	})

	for _, required := range []string{
		`TOKEN_OUTPUT=$(pveum user token add pulse-monitor@pve "$TOKEN_NAME" --privsep 1 2>&1)`,
		`pveum aclmod / -user pulse-monitor@pve -role PVEAuditor`,
		`pveum aclmod / -token "$PULSE_TOKEN_ID" -role PVEAuditor`,
		`pveum aclmod / -user pulse-monitor@pve -role PulseMonitor`,
		`pveum aclmod / -token "$PULSE_TOKEN_ID" -role PulseMonitor`,
		`pveum aclmod /storage -user pulse-monitor@pve -role PVEDatastoreAdmin`,
		`pveum aclmod /storage -token "$PULSE_TOKEN_ID" -role PVEDatastoreAdmin`,
		`smoke_test_pve_token() {`,
		`Authorization: PVEAPIToken=$PULSE_TOKEN_ID=$TOKEN_VALUE`,
		`${HOST_URL%/}/api2/json/nodes`,
		`if smoke_test_pve_token; then`,
	} {
		if !strings.Contains(pveScript, required) {
			t.Fatalf("PVE setup script must contain %q", required)
		}
	}
	if strings.Contains(pveScript, "--privsep 0") {
		t.Fatal("PVE setup script must not create full-privilege API tokens")
	}

	pbsArtifact := buildSetupScriptInstallArtifact(
		"https://pulse.example",
		"pbs",
		"https://pbs.example:8007",
		"https://pulse.example",
		false,
		"setup-token-123",
		0,
	)
	pbsScript := renderSetupScript("pbs", setupScriptRenderContext{
		ServerName:       "pbs-example",
		PulseURL:         "https://pulse.example",
		ServerHost:       "https://pbs.example:8007",
		SetupToken:       "setup-token-123",
		TokenName:        "pulse-example",
		TokenMatchPrefix: "pulse-example",
		Artifact:         pbsArtifact,
	})

	for _, required := range []string{
		`proxmox-backup-manager acl update / Audit --auth-id pulse-monitor@pbs`,
		`proxmox-backup-manager acl update / Audit --auth-id "$PULSE_TOKEN_ID"`,
	} {
		if !strings.Contains(pbsScript, required) {
			t.Fatalf("PBS setup script must contain %q", required)
		}
	}
	for _, forbidden := range []string{
		`Admin --auth-id pulse-monitor@pbs`,
		`Admin --auth-id "$PULSE_TOKEN_ID"`,
	} {
		if strings.Contains(pbsScript, forbidden) {
			t.Fatalf("PBS setup script must not grant admin ACL %q", forbidden)
		}
	}
}

func TestContract_NodeConfigUpdateTracksOptionalConnectionFieldsAndRedactedSecrets(t *testing.T) {
	var preserveReq NodeConfigRequest
	if err := json.Unmarshal([]byte(`{"name":"cluster","tokenName":"pulse-monitor@pve!pulse-pve","tokenValue":"********"}`), &preserveReq); err != nil {
		t.Fatalf("decode preserve request: %v", err)
	}
	preserveReq.normalizeTokenAliases()
	if preserveReq.hasGuestURLField() {
		t.Fatal("omitted guestURL must preserve the stored connection URL")
	}
	if preserveReq.hasFingerprintField() {
		t.Fatal("omitted fingerprint must preserve the stored TLS fingerprint")
	}
	if preserveReq.TokenValue != "" {
		t.Fatalf("redacted token placeholder must not become a stored token secret, got %q", preserveReq.TokenValue)
	}

	var clearReq NodeConfigRequest
	if err := json.Unmarshal([]byte(`{"guestURL":"","fingerprint":""}`), &clearReq); err != nil {
		t.Fatalf("decode clear request: %v", err)
	}
	if !clearReq.hasGuestURLField() {
		t.Fatal("explicit empty guestURL must remain observable so clients can clear it intentionally")
	}
	if !clearReq.hasFingerprintField() {
		t.Fatal("explicit empty fingerprint must remain observable so clients can clear it intentionally")
	}
}

func TestContract_HostedMagicLinkStablePrincipalProof(t *testing.T) {
	source, err := os.ReadFile(filepath.Clean("magic_link_handlers.go"))
	if err != nil {
		t.Fatalf("read magic_link_handlers.go: %v", err)
	}
	text := string(source)
	for _, required := range []string{
		"resolveMagicLinkPrincipal",
		"CreateSession(sessionToken, sessionDuration, userAgent, clientIP, principal.UserID)",
		"TrackUserSession(principal.UserID, sessionToken)",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("magic_link_handlers.go must contain %q", required)
		}
	}
	for _, forbidden := range []string{
		"CreateSession(sessionToken, sessionDuration, userAgent, clientIP, token.Email)",
		"TrackUserSession(token.Email, sessionToken)",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("magic_link_handlers.go must not contain legacy email-principal pattern %q", forbidden)
		}
	}

	invariantDoc, err := os.ReadFile(filepath.Clean("../../docs/release-control/v6/internal/IDENTITY_INVARIANTS.md"))
	if err != nil {
		t.Fatalf("read identity invariant contract: %v", err)
	}
	if !strings.Contains(string(invariantDoc), "Email is contact metadata") {
		t.Fatal("identity invariant contract must define email as contact metadata")
	}

	modelSource, err := os.ReadFile(filepath.Clean("../models/organization.go"))
	if err != nil {
		t.Fatalf("read organization model: %v", err)
	}
	if !strings.Contains(string(modelSource), "ResolvePrincipalByEmail") {
		t.Fatal("organization model must expose email-to-stable-principal resolution")
	}
	if strings.Contains(string(modelSource), "userID = email") {
		t.Fatal("organization model must not synthesize magic-link principals from email")
	}
}

func TestContract_CheckoutMagicLinkDeliveryRequiresStoredPrincipalProof(t *testing.T) {
	source, err := os.ReadFile(filepath.Clean("payments_webhook_handlers.go"))
	if err != nil {
		t.Fatalf("read payments_webhook_handlers.go: %v", err)
	}
	text := string(source)
	for _, required := range []string{
		"ResolvePrincipalByEmail(email)",
		"models.IsValidOrganizationRole(role)",
		"GenerateToken(email, orgID)",
		"SendMagicLink(email, orgID, token, baseURL)",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("payments_webhook_handlers.go must contain %q", required)
		}
	}
	for _, forbidden := range []string{
		"strings.EqualFold(org.OwnerEmail, email)",
		"strings.EqualFold(org.OwnerUserID, email)",
		"strings.EqualFold(m.Email, email)",
		"strings.EqualFold(m.UserID, email)",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("payments_webhook_handlers.go must not contain legacy contact-email membership pattern %q", forbidden)
		}
	}

	identityDoc, err := os.ReadFile(filepath.Clean("../../docs/release-control/v6/internal/IDENTITY_INVARIANTS.md"))
	if err != nil {
		t.Fatalf("read identity invariant contract: %v", err)
	}
	if !strings.Contains(string(identityDoc), "Post-checkout magic-link delivery") {
		t.Fatal("identity invariant contract must cover checkout magic-link delivery")
	}
}

func TestContract_OrganizationRuntimeAccessUsesStrictUserIDProof(t *testing.T) {
	modelSource, err := os.ReadFile(filepath.Clean("../models/organization.go"))
	if err != nil {
		t.Fatalf("read organization model: %v", err)
	}
	modelText := string(modelSource)
	for _, required := range []string{
		"HasMemberUserID",
		"GetMemberRoleByUserID",
		"IsOwnerUserID",
		"CanUserIDAccess",
		"CanUserIDManage",
	} {
		if !strings.Contains(modelText, required) {
			t.Fatalf("organization model must contain strict user ID helper %q", required)
		}
	}

	requiredByFile := map[string][]string{
		"authorization.go": {
			"org.CanUserIDAccess(userID)",
		},
		"org_handlers.go": {
			"org.CanUserIDAccess(username)",
			"org.CanUserIDManage(username)",
			"org.IsOwnerUserID(username)",
			"org.GetMemberRoleByUserID(username)",
		},
		"security_setup_fix.go": {
			"org.CanUserIDManage(sessionUser)",
		},
		"cloud_org_admin_auth.go": {
			"org.IsOwnerUserID(userID)",
		},
	}
	for file, required := range requiredByFile {
		source, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := string(source)
		for _, needle := range required {
			if !strings.Contains(text, needle) {
				t.Fatalf("%s must contain strict runtime authorization proof %q", file, needle)
			}
		}
		for _, forbidden := range []string{
			".CanUserAccess(username)",
			".CanUserAccess(userID)",
			".CanUserManage(username)",
			".CanUserManage(sessionUser)",
			".IsOwner(username)",
			".GetMemberRole(username)",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s must not use legacy email-aware runtime accessor %q", file, forbidden)
			}
		}
	}

	identityDoc, err := os.ReadFile(filepath.Clean("../../docs/release-control/v6/internal/IDENTITY_INVARIANTS.md"))
	if err != nil {
		t.Fatalf("read identity invariant contract: %v", err)
	}
	identityText := string(identityDoc)
	if !strings.Contains(identityText, "strict user-ID") || !strings.Contains(identityText, "OwnerUserID") {
		t.Fatal("identity invariant contract must require strict user-ID membership checks for live authorization")
	}
}

func TestContract_HostedHandoffRequiresStableSubjectProof(t *testing.T) {
	directSource, err := os.ReadFile(filepath.Clean("cloud_handoff.go"))
	if err != nil {
		t.Fatalf("read cloud_handoff.go: %v", err)
	}
	exchangeSource, err := os.ReadFile(filepath.Clean("cloud_handoff_handlers.go"))
	if err != nil {
		t.Fatalf("read cloud_handoff_handlers.go: %v", err)
	}

	for file, source := range map[string]string{
		"cloud_handoff.go":          string(directSource),
		"cloud_handoff_handlers.go": string(exchangeSource),
	} {
		for _, required := range []string{
			"isEmailShapedHandoffUserID(userID)",
			"CreateSession(sessionToken, sessionDuration, userAgent, clientIP, authz.UserID)",
			"TrackUserSession(authz.UserID, sessionToken)",
		} {
			if !strings.Contains(source, required) {
				t.Fatalf("%s must contain %q", file, required)
			}
		}
		for _, forbidden := range []string{
			"userID = email",
			"email = normalizeHandoffEmail(userID)",
			"CreateSession(sessionToken, sessionDuration, userAgent, clientIP, claims.Email)",
			"TrackUserSession(claims.Email, sessionToken)",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s must not contain legacy email-principal pattern %q", file, forbidden)
			}
		}
	}
}

func TestContract_SSOStablePrincipalProof(t *testing.T) {
	ssoIdentity, err := os.ReadFile(filepath.Clean("auth_principal_identity.go"))
	if err != nil {
		t.Fatalf("read auth_principal_identity.go: %v", err)
	}
	oidcSource, err := os.ReadFile(filepath.Clean("oidc_handlers.go"))
	if err != nil {
		t.Fatalf("read oidc_handlers.go: %v", err)
	}
	samlSource, err := os.ReadFile(filepath.Clean("saml_handlers.go"))
	if err != nil {
		t.Fatalf("read saml_handlers.go: %v", err)
	}

	for file, text := range map[string]string{
		"auth_principal_identity.go": string(ssoIdentity),
		"oidc_handlers.go":           string(oidcSource),
		"saml_handlers.go":           string(samlSource),
	} {
		for _, required := range []string{
			"stableSSOPrincipal",
			"applySSORoleAssignments",
		} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s must contain %q", file, required)
			}
		}
	}
	if !strings.Contains(string(oidcSource), "establishOIDCSession(w, req, principal, username, oidcTokens)") {
		t.Fatal("oidc_handlers.go must establish OIDC sessions with a stable principal and separate display username")
	}
	if !strings.Contains(string(samlSource), "establishSAMLSession(w, req, principal, result.Username, samlSession)") {
		t.Fatal("saml_handlers.go must establish SAML sessions with a stable principal and separate display username")
	}
	for _, forbidden := range []string{
		"establishOIDCSession(w, req, username, oidcTokens)",
		"UpdateUserRoles(username, rolesToAssign)",
	} {
		if strings.Contains(string(oidcSource), forbidden) {
			t.Fatalf("oidc_handlers.go must not contain legacy SSO principal pattern %q", forbidden)
		}
	}
	for _, forbidden := range []string{
		"establishSAMLSession(w, req, username, samlSession)",
		"UpdateUserRoles(result.Username, rolesToAssign)",
	} {
		if strings.Contains(string(samlSource), forbidden) {
			t.Fatalf("saml_handlers.go must not contain legacy SSO principal pattern %q", forbidden)
		}
	}
}

func TestContract_APITokenOwnerBindingCoversSharedMintingPaths(t *testing.T) {
	files := map[string][]string{
		"security_tokens.go": {
			"setAPITokenOwnerUserID",
			"apiTokenOwnerUserIDForRequest",
			"reserved token metadata key",
		},
		"agent_install_command_shared.go": {
			"OwnerUserID string",
			"setAPITokenOwnerUserID(record, opts.OwnerUserID)",
			"mergeAPITokenMetadata(record, opts.Metadata)",
		},
		"deploy_handlers.go": {
			"setAPITokenOwnerUserID(record, ownerUserID)",
			"setAPITokenOwnerUserID(runtimeRecord, apiTokenOwnerUserID(*bootstrapToken))",
			"apiTokenOwnerUserIDForRequest(h.config, r)",
		},
		"router.go": {
			"setAPITokenOwnerUserID(record, apiTokenOwnerUserIDForRequest(r.config, req))",
		},
		"security_setup_fix.go": {
			"setAPITokenOwnerUserID(tokenRecord, setupRequest.Username)",
			"setAPITokenOwnerUserID(tokenRecord, apiTokenOwnerUserIDForRequest(r.config, rq))",
		},
	}

	for file, needles := range files {
		source, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := string(source)
		for _, needle := range needles {
			if !strings.Contains(text, needle) {
				t.Fatalf("%s must contain %q", file, needle)
			}
		}
	}
}

func TestContractAISettingsClampsPaidRuntimeControlsToEntitlements(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.OllamaBaseURL = "http://127.0.0.1:11434"
	aiCfg.ControlLevel = config.ControlLevelAutonomous
	aiCfg.PatrolAutoFix = true
	aiCfg.AlertTriggeredAnalysis = true
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("save ai config: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)
	handler.defaultAIService.SetLicenseChecker(stubLicenseChecker{allow: false})

	req := newLoopbackRequest(http.MethodGet, "/api/settings/ai", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/settings/ai status = %d, body %s", rec.Code, rec.Body.String())
	}

	var resp AISettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ControlLevel != config.ControlLevelControlled {
		t.Fatalf("control level = %q, want %q", resp.ControlLevel, config.ControlLevelControlled)
	}
	if resp.PatrolAutoFix {
		t.Fatal("patrol auto-remediation must be entitlement-clamped")
	}
	if resp.AlertTriggeredAnalysis {
		t.Fatal("alert-triggered analysis must be entitlement-clamped")
	}
}

func TestContractAnthropicOAuthSetupEndpointsFailClosedPayload(t *testing.T) {
	handler := &AISettingsHandler{}

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		call   func(http.ResponseWriter, *http.Request)
	}{
		{
			name:   "start",
			method: http.MethodGet,
			path:   "/api/ai/oauth/start",
			call:   handler.HandleOAuthStart,
		},
		{
			name:   "exchange",
			method: http.MethodPost,
			path:   "/api/ai/oauth/exchange",
			body:   `{"code":"code","state":"state"}`,
			call:   handler.HandleOAuthExchange,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newLoopbackRequest(tt.method, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			tt.call(rec, req)

			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusNotImplemented, rec.Body.String())
			}

			var resp struct {
				Success bool   `json:"success"`
				Error   string `json:"error"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Success {
				t.Fatal("unsupported OAuth response must not report success")
			}
			if resp.Error != "unsupported_anthropic_oauth" {
				t.Fatalf("error=%q, want unsupported_anthropic_oauth", resp.Error)
			}
			if !strings.Contains(resp.Message, "Configure an Anthropic API key") {
				t.Fatalf("message must direct operators to API-key setup, got %q", resp.Message)
			}
		})
	}
}

func sortedVMChartKeys(values map[string]VMChartData) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type contractCapturingStreamingProvider struct {
	lastRequest providers.ChatRequest
}

type contractSupplementalUsageProvider struct {
	settled bool
	readyAt time.Time
	records []unifiedresources.IngestRecord
}

func (p *contractSupplementalUsageProvider) SupplementalRecords(*monitoring.Monitor, string) []unifiedresources.IngestRecord {
	out := make([]unifiedresources.IngestRecord, len(p.records))
	copy(out, p.records)
	return out
}

func (p *contractSupplementalUsageProvider) SnapshotOwnedSources() []unifiedresources.DataSource {
	return []unifiedresources.DataSource{unifiedresources.SourceTrueNAS}
}

func (p *contractSupplementalUsageProvider) SupplementalInventoryReadyAt(*monitoring.Monitor, string) (time.Time, bool) {
	return p.readyAt, p.settled
}

func (p *contractSupplementalUsageProvider) settle(count int) {
	now := time.Now().UTC()
	p.readyAt = now
	p.settled = true
	p.records = make([]unifiedresources.IngestRecord, 0, count)
	for i := 0; i < count; i++ {
		host := fmt.Sprintf("contract-truenas-%02d.lab.local", i+1)
		p.records = append(p.records, unifiedresources.IngestRecord{
			SourceID: fmt.Sprintf("system:contract-truenas-%02d", i+1),
			Resource: unifiedresources.Resource{
				ID:        fmt.Sprintf("contract-truenas-%02d", i+1),
				Type:      unifiedresources.ResourceTypeAgent,
				Name:      host,
				Status:    unifiedresources.StatusOnline,
				LastSeen:  now,
				UpdatedAt: now,
				Identity: unifiedresources.ResourceIdentity{
					Hostnames: []string{host},
				},
				TrueNAS: &unifiedresources.TrueNASData{
					Hostname: host,
				},
			},
		})
	}
}

func (p *contractCapturingStreamingProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	p.lastRequest = req
	return &providers.ChatResponse{Content: "ok", Model: req.Model}, nil
}

func (p *contractCapturingStreamingProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	p.lastRequest = req
	callback(providers.StreamEvent{Type: "content", Data: providers.ContentEvent{Text: "hello"}})
	callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{StopReason: "end_turn"}})
	return nil
}

func (p *contractCapturingStreamingProvider) TestConnection(ctx context.Context) error { return nil }
func (p *contractCapturingStreamingProvider) Name() string                             { return "contract-capture" }
func (p *contractCapturingStreamingProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}
func (p *contractCapturingStreamingProvider) SupportsThinking(model string) bool { return false }

func TestContract_WebSocketTrustedProxyHostedOrigin(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "127.0.0.1/32")
	resetTrustedProxyCIDRsForTests()

	rawToken := "contract-ws-origin-forwarded-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)

	server, cleanup := newWebSocketRouter(t, []string{}, record)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?org_id=default"
	headers := http.Header{}
	headers.Set("X-API-Token", rawToken)
	headers.Set("Origin", "https://tenant.example.com")
	headers.Set("X-Forwarded-Proto", "https")
	headers.Set("X-Forwarded-Host", "tenant.example.com")

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("expected websocket connection behind trusted proxy, got %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		t.Fatalf("expected 101 switching protocols, got %v", resp)
	}
	conn.Close()
}

func TestContract_AIRelayTargetHostnameEquivalence(t *testing.T) {
	server := agentexec.NewServer(func(string, string, string) bool { return true })
	ts := newIPv4HTTPServer(t, http.HandlerFunc(server.HandleWebSocket))
	defer ts.Close()

	conn := registerAgent(t, ts.URL, "agent-1", "prox97.seftic.local")
	defer conn.Close()

	adapter := &agentCommandAdapter{handler: &AISettingsHandler{agentServer: server}}

	if got := adapter.FindAgentForTarget("prox97"); got != "agent-1" {
		t.Fatalf("FindAgentForTarget(%q) = %q, want agent-1", "prox97", got)
	}
	if got := adapter.FindAgentForTarget("prox97.seftic.local"); got != "agent-1" {
		t.Fatalf("FindAgentForTarget(%q) = %q, want agent-1", "prox97.seftic.local", got)
	}
	if got := adapter.FindAgentForTarget("prox97.other.local"); got != "" {
		t.Fatalf("FindAgentForTarget(%q) = %q, want empty for distinct FQDN", "prox97.other.local", got)
	}
}

func TestContract_WireAIChatDependencies_WiresTrueNASAppActionProvider(t *testing.T) {
	router := &Router{
		trueNASPoller: monitoring.NewTrueNASPoller(nil, 0, nil),
	}
	service := &capturingAIService{}

	router.wireAIChatDependenciesForService(context.Background(), service)

	if service.appContainerActionProvider == nil {
		t.Fatal("expected TrueNAS app action provider to be wired into AI chat dependencies")
	}
	if service.appContainerReadProvider == nil {
		t.Fatal("expected TrueNAS app read provider to be wired into AI chat dependencies")
	}
	if service.appContainerConfigProvider == nil {
		t.Fatal("expected TrueNAS app config provider to be wired into AI chat dependencies")
	}
}

func TestContract_AITransportResourceTypeUsesSharedActionResourceVocabulary(t *testing.T) {
	for _, input := range []string{"truenas", "SYSTEM-CONTAINER", "app-container", "host"} {
		got := normalizeAITransportResourceType(input)
		want := agentcapabilities.NormalizeActionResourceType(input)
		if got != want {
			t.Fatalf("normalizeAITransportResourceType(%q) = %q, want shared action resource type %q", input, got, want)
		}
	}
	if got := normalizeAITransportResourceType("truenas"); got != agentcapabilities.ActionTargetTypeAgent {
		t.Fatalf("truenas resource type = %q, want %q", got, agentcapabilities.ActionTargetTypeAgent)
	}
}

func TestContract_ChatServiceAdapterPatrolForwardsExecutionID(t *testing.T) {
	cfg := chat.Config{
		DataDir: t.TempDir(),
		AIConfig: &config.AIConfig{
			Enabled:     true,
			ChatModel:   "stub:model",
			PatrolModel: "stub:model",
		},
	}
	service := chat.NewService(cfg)
	provider := &contractCapturingStreamingProvider{}
	setUnexportedField(t, service, "providerFactory", func(string) (providers.StreamingProvider, error) {
		return provider, nil
	})
	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = service.Stop(context.Background()) })

	adapter := &chatServiceAdapter{svc: service}
	resp, err := adapter.ExecutePatrolStream(context.Background(), ai.PatrolExecuteRequest{
		Prompt:      "patrol",
		ExecutionID: "patrol-run-contract",
	}, func(ai.ChatStreamEvent) {})
	if err != nil {
		t.Fatalf("ExecutePatrolStream: %v", err)
	}
	if resp == nil || resp.Content == "" {
		t.Fatalf("expected patrol response content, got %#v", resp)
	}
	if provider.lastRequest.ExecutionID != "patrol-run-contract" {
		t.Fatalf("execution_id=%q want patrol-run-contract", provider.lastRequest.ExecutionID)
	}
}

func TestContract_AIQuickstartPayloadFieldsAreRetired(t *testing.T) {
	settingsBody, err := json.Marshal(AISettingsResponse{
		Enabled:    true,
		Configured: false,
	})
	if err != nil {
		t.Fatalf("marshal AI settings response: %v", err)
	}
	if bytes.Contains(settingsBody, []byte(`quickstart`)) {
		t.Fatalf("expected AI settings payload not to expose retired quickstart fields, got %s", settingsBody)
	}

	statusBody, err := json.Marshal(PatrolStatusResponse{
		RuntimeState: ai.PatrolRuntimeStateActive,
		Healthy:      true,
	})
	if err != nil {
		t.Fatalf("marshal patrol status response: %v", err)
	}
	if bytes.Contains(statusBody, []byte(`quickstart`)) {
		t.Fatalf("expected patrol status payload not to expose retired quickstart fields, got %s", statusBody)
	}
}

func TestContract_AISettingsUpdateRequiresProviderBeforeEnable(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", strings.NewReader(`{
		"enabled": true
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if strings.Contains(strings.ToLower(rec.Body.String()), "quickstart") {
		t.Fatalf("provider-required response must not mention retired quickstart, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Please configure a provider") {
		t.Fatalf("expected provider-required response, got %s", rec.Body.String())
	}
}

func TestContract_AISettingsUpdateProviderResolutionJSONSnapshot(t *testing.T) {
	ollama := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{
					{"name": "llama3:latest"},
					{"name": "mistral:latest"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ollama.Close()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", strings.NewReader(fmt.Sprintf(`{
		"enabled": true,
		"ollama_base_url": %q
	}`, ollama.URL)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	want := fmt.Sprintf(`{
		"enabled":true,
		"model":"ollama:llama3:latest",
		"configured":true,
		"custom_context":"",
		"auth_method":"api_key",
		"oauth_connected":false,
		"patrol_interval_minutes":360,
		"patrol_enabled":true,
		"patrol_auto_fix":false,
		"alert_triggered_analysis":true,
		"patrol_event_triggers_enabled":true,
		"patrol_alert_triggers_enabled":true,
		"patrol_anomaly_triggers_enabled":true,
		"patrol_alert_trigger_min_severity":"critical",
		"patrol_alert_trigger_types":[],
		"use_proactive_thresholds":false,
		"available_models":[],
		"anthropic_configured":false,
		"openai_configured":false,
		"openrouter_configured":false,
		"deepseek_configured":false,
		"gemini_configured":false,
		"zai_configured":false,
		"groq_configured":false,
		"mistral_configured":false,
		"cerebras_configured":false,
		"together_configured":false,
		"fireworks_configured":false,
		"ollama_configured":true,
		"ollama_base_url":%q,
		"ollama_password_set":false,
		"ollama_keep_alive":"30s",
		"configured_providers":["ollama"],
		"control_level":"read_only",
		"protected_guests":[],
		"discovery_enabled":false,
		"patrol_readiness":{
			"status":"warning",
			"ready":true,
			"cause":"model_tool_support_unverified",
			"summary":"Ollama connectivity alone does not prove tool support. Use an Ollama model that returns tool_calls for Patrol verification.",
			"provider":"ollama",
			"model":"ollama:llama3:latest",
			"checks":[{
				"id":"configuration",
				"status":"warning",
				"cause":"model_tool_support_unverified",
				"label":"Patrol control",
				"message":"Ollama connectivity alone does not prove tool support. Use an Ollama model that returns tool_calls for Patrol verification."
			}]
		}
	}`, ollama.URL)

	// patrol_preflight is now emitted on update responses with dynamic
	// timestamps and durations that vary per run; strip it from the
	// snapshot comparison and rely on dedicated preflight tests for its
	// shape.
	assertJSONSnapshot(t, rec.Body.Bytes(), want, "patrol_preflight", "providers")
}

func TestContract_PatrolStatusDoesNotSurfaceQuickstartActivation(t *testing.T) {
	handler, patrol, _, _ := setupAIHandlerWithPatrol(t)
	persistence := handler.defaultPersistence
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	if err := handler.defaultAIService.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	setUnexportedField(t, patrol, "lastBlockedReason", "Quickstart credits exhausted. Connect your API key to continue using AI Patrol.")
	setUnexportedField(t, patrol, "lastBlockedAt", time.Now().UTC())

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/status", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetPatrolStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp PatrolStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode patrol status: %v", err)
	}
	if resp.RuntimeState == ai.PatrolRuntimeStateBlocked {
		t.Fatalf("runtime_state=%q, want non-blocked for retired quickstart", resp.RuntimeState)
	}
	if !resp.Enabled {
		t.Fatal("expected patrol status to remain enabled while activation is required")
	}
	if !resp.Healthy {
		t.Fatal("expected retired quickstart activation state to stay healthy on the public status surface")
	}
	if strings.Contains(strings.ToLower(resp.BlockedReason), "quickstart") {
		t.Fatalf("blocked_reason must not expose retired quickstart copy, got %q", resp.BlockedReason)
	}
}

func TestContract_PatrolStatusMissingAssistantServiceUsesNativeSurfaceIdentity(t *testing.T) {
	handler := &AISettingsHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/status", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetPatrolStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp PatrolStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode patrol status: %v", err)
	}
	if resp.Readiness == nil {
		t.Fatal("expected readiness payload when Assistant service is missing")
	}
	if resp.Readiness.Summary != "Pulse Assistant runtime service is not available." {
		t.Fatalf("readiness summary = %q", resp.Readiness.Summary)
	}

	var serviceCheck *PatrolReadinessCheck
	for i := range resp.Readiness.Checks {
		if resp.Readiness.Checks[i].ID == "service" {
			serviceCheck = &resp.Readiness.Checks[i]
			break
		}
	}
	if serviceCheck == nil {
		t.Fatalf("service readiness check missing from %+v", resp.Readiness.Checks)
	}
	if serviceCheck.Label != "Assistant runtime service" {
		t.Fatalf("service readiness label = %q", serviceCheck.Label)
	}
	if serviceCheck.Message != "Pulse Assistant runtime service is not available." {
		t.Fatalf("service readiness message = %q", serviceCheck.Message)
	}
	if strings.Contains(rec.Body.String(), "Pulse AI") {
		t.Fatalf("patrol status payload must not expose legacy Pulse AI runtime identity: %s", rec.Body.String())
	}
}

func TestContract_AISettingsDoesNotExposeQuickstartActivation(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/ai", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp AISettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode ai settings: %v", err)
	}
	if resp.Configured {
		t.Fatal("expected AI settings to remain unconfigured without BYOK/local provider")
	}
	if bytes.Contains(bytes.ToLower(rec.Body.Bytes()), []byte("quickstart")) {
		t.Fatalf("AI settings payload must not expose retired quickstart fields, got %s", rec.Body.Bytes())
	}
}

func TestContract_AISettingsBYOKOverrideDoesNotExposeQuickstartInventoryJSONSnapshot(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "openai:gpt-4o"
	aiCfg.OpenAIAPIKey = "sk-openai-test"
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/ai", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	const want = `{
		"enabled":true,
		"model":"openai:gpt-4o",
		"configured":true,
		"custom_context":"",
		"auth_method":"api_key",
		"oauth_connected":false,
		"patrol_interval_minutes":360,
		"patrol_enabled":true,
		"patrol_auto_fix":false,
		"alert_triggered_analysis":true,
		"patrol_event_triggers_enabled":true,
		"patrol_alert_triggers_enabled":true,
		"patrol_anomaly_triggers_enabled":true,
		"patrol_alert_trigger_min_severity":"critical",
		"patrol_alert_trigger_types":[],
		"use_proactive_thresholds":false,
		"available_models":[],
		"anthropic_configured":false,
		"openai_configured":true,
		"openrouter_configured":false,
		"deepseek_configured":false,
		"gemini_configured":false,
		"zai_configured":false,
		"groq_configured":false,
		"mistral_configured":false,
		"cerebras_configured":false,
		"together_configured":false,
		"fireworks_configured":false,
		"ollama_configured":false,
		"ollama_base_url":"http://localhost:11434",
		"ollama_password_set":false,
		"ollama_keep_alive":"30s",
		"configured_providers":["openai"],
		"control_level":"read_only",
		"protected_guests":[],
		"discovery_enabled":false,
		"patrol_readiness":{"status":"not_ready","ready":false,"cause":"service_unavailable","summary":"Pulse Patrol service is not available.","checks":[{"id":"service","status":"not_ready","cause":"service_unavailable","label":"Patrol service","message":"Pulse Patrol service is not available.","action":"restart_service"}]}
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want, "providers")
}

func TestContract_AISettingsProviderRegistryMetadata(t *testing.T) {
	settings := config.NewDefaultAIConfig()
	settings.OpenAIAPIKey = "sk-test"
	settings.GroqAPIKey = "gsk-test"
	settings.OllamaBaseURL = config.DefaultOllamaBaseURL

	defs := aiProviderDefinitionResponses(settings)
	gotIDs := make([]string, 0, len(defs))
	byID := make(map[string]AIProviderDefinitionResponse, len(defs))
	for _, def := range defs {
		gotIDs = append(gotIDs, def.ID)
		byID[def.ID] = def
	}

	wantIDs := []string{
		"anthropic",
		"openai",
		"openrouter",
		"deepseek",
		"gemini",
		"zai",
		"groq",
		"mistral",
		"cerebras",
		"together",
		"fireworks",
		"ollama",
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("provider ids = %#v, want %#v", gotIDs, wantIDs)
	}
	if _, ok := byID["quickstart"]; ok {
		t.Fatal("retired quickstart provider must not be user-configurable")
	}

	cases := map[string]struct {
		displayName         string
		apiKeyField         string
		clearKeyField       string
		configuredField     string
		defaultBaseURL      string
		modelsDevProviderID string
		configured          bool
	}{
		"zai": {
			displayName:         "Z.ai",
			apiKeyField:         "zai_api_key",
			clearKeyField:       "clear_zai_key",
			configuredField:     "zai_configured",
			defaultBaseURL:      "https://api.z.ai/api/paas/v4",
			modelsDevProviderID: "zai",
		},
		"groq": {
			displayName:         "Groq",
			apiKeyField:         "groq_api_key",
			clearKeyField:       "clear_groq_key",
			configuredField:     "groq_configured",
			defaultBaseURL:      "https://api.groq.com/openai/v1",
			modelsDevProviderID: "groq",
			configured:          true,
		},
		"mistral": {
			displayName:         "Mistral",
			apiKeyField:         "mistral_api_key",
			clearKeyField:       "clear_mistral_key",
			configuredField:     "mistral_configured",
			defaultBaseURL:      "https://api.mistral.ai/v1",
			modelsDevProviderID: "mistral",
		},
		"cerebras": {
			displayName:         "Cerebras",
			apiKeyField:         "cerebras_api_key",
			clearKeyField:       "clear_cerebras_key",
			configuredField:     "cerebras_configured",
			defaultBaseURL:      "https://api.cerebras.ai/v1",
			modelsDevProviderID: "cerebras",
		},
		"together": {
			displayName:         "Together AI",
			apiKeyField:         "together_api_key",
			clearKeyField:       "clear_together_key",
			configuredField:     "together_configured",
			defaultBaseURL:      "https://api.together.xyz/v1",
			modelsDevProviderID: "togetherai",
		},
		"fireworks": {
			displayName:         "Fireworks AI",
			apiKeyField:         "fireworks_api_key",
			clearKeyField:       "clear_fireworks_key",
			configuredField:     "fireworks_configured",
			defaultBaseURL:      "https://api.fireworks.ai/inference/v1",
			modelsDevProviderID: "fireworks-ai",
		},
	}
	for id, want := range cases {
		got, ok := byID[id]
		if !ok {
			t.Fatalf("provider %q missing from registry response", id)
		}
		if got.DisplayName != want.displayName ||
			got.Protocol != string(config.AIProviderProtocolOpenAICompatible) ||
			got.APIKeyField != want.apiKeyField ||
			got.ClearKeyField != want.clearKeyField ||
			got.ConfiguredField != want.configuredField ||
			got.DefaultBaseURL != want.defaultBaseURL ||
			got.ModelsDevProviderID != want.modelsDevProviderID ||
			!got.RequiresAPIKey ||
			!got.UserConfigurable ||
			got.Configured != want.configured {
			t.Fatalf("provider %s metadata drifted: %#v", id, got)
		}
	}

	if !byID["openai"].Configured {
		t.Fatal("OpenAI provider metadata should reflect configured API key")
	}
	if byID["ollama"].RequiresAPIKey {
		t.Fatal("Ollama provider metadata must not require an API key")
	}
	if !byID["ollama"].Configured {
		t.Fatal("Ollama provider metadata should reflect the default local endpoint")
	}
}

func TestContract_ChartMetricPointsPreserveMillisecondPrecision(t *testing.T) {
	pointTime := time.Date(2026, time.March, 31, 12, 0, 0, 987_000_000, time.UTC)

	converted := monitorPointsToAPI([]monitoring.MetricPoint{{
		Timestamp: pointTime,
		Value:     42,
	}})
	if len(converted) != 1 {
		t.Fatalf("expected one converted point, got %d", len(converted))
	}
	if converted[0].Timestamp != pointTime.UnixMilli() {
		t.Fatalf("expected millisecond timestamp %d, got %d", pointTime.UnixMilli(), converted[0].Timestamp)
	}
}

func TestContract_StorageChartsUseCanonicalMetricsTargetIDs(t *testing.T) {
	monitor, state, metricsHistory := newTestMonitor(t)
	now := time.Now().UTC().Add(-5 * time.Minute).Truncate(time.Millisecond)

	metricsHistory.AddStorageMetric("vc-1:datastore:datastore-202", "usage", 0.25, now)
	metricsHistory.AddStorageMetric("vc-1:datastore:datastore-202", "used", 3.57*1024*1024*1024*1024, now)
	metricsHistory.AddStorageMetric("vc-1:datastore:datastore-202", "avail", 11.03*1024*1024*1024*1024, now)
	metricsHistory.AddStorageMetric("pool:archive", "usage", 0.36, now)
	metricsHistory.AddStorageMetric("pool:archive", "used", 5.49*1024*1024*1024*1024, now)
	metricsHistory.AddStorageMetric("pool:archive", "avail", 4.51*1024*1024*1024*1024, now)

	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateSnapshotAndSupplemental(state.GetSnapshot(), map[unifiedresources.DataSource][]unifiedresources.IngestRecord{
		unifiedresources.SourceVMware: {
			{
				SourceID: "vc-1:datastore:datastore-202",
				Resource: unifiedresources.Resource{
					ID:       "storage-vmware-1",
					Type:     unifiedresources.ResourceTypeStorage,
					Name:     "archive-tier",
					Status:   unifiedresources.StatusOnline,
					LastSeen: now,
					Storage: &unifiedresources.StorageMeta{
						Type:     "datastore",
						Platform: "vmware",
						Nodes:    []string{"esxi-01.lab.local"},
					},
					VMware: &unifiedresources.VMwareData{
						ConnectionID:    "vc-1",
						EntityType:      "datastore",
						ManagedObjectID: "datastore-202",
						RuntimeHostName: "esxi-01.lab.local",
					},
				},
			},
		},
		unifiedresources.SourceTrueNAS: {
			{
				SourceID: "pool:archive",
				Resource: unifiedresources.Resource{
					ID:       "storage-truenas-1",
					Type:     unifiedresources.ResourceTypeStorage,
					Name:     "archive",
					Status:   unifiedresources.StatusOnline,
					LastSeen: now,
					Storage: &unifiedresources.StorageMeta{
						Type:     "zfs-pool",
						Platform: "truenas",
					},
					TrueNAS: &unifiedresources.TrueNASData{
						Hostname: "truenas-main",
					},
				},
			},
		},
	})
	setTestUnexportedField(t, monitor, "resourceStore", monitoring.ResourceStoreInterface(adapter))

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/storage-charts?range=60", nil)
	rec := httptest.NewRecorder()
	router.handleStorageCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var decoded StorageChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal storage charts response: %v", err)
	}
	if _, ok := decoded.Pools["vc-1:datastore:datastore-202"]; !ok {
		t.Fatalf("expected VMware datastore chart keyed by canonical metrics target, got %v", decoded.Pools)
	}
	if _, ok := decoded.Pools["pool:archive"]; !ok {
		t.Fatalf("expected TrueNAS pool chart keyed by canonical metrics target, got %v", decoded.Pools)
	}
	if _, ok := decoded.Pools["storage-vmware-1"]; ok {
		t.Fatalf("expected raw VMware resource id to stay out of chart payload, got %v", decoded.Pools)
	}
	if _, ok := decoded.Pools["storage-truenas-1"]; ok {
		t.Fatalf("expected raw TrueNAS resource id to stay out of chart payload, got %v", decoded.Pools)
	}
	vmwarePool := decoded.Pools["vc-1:datastore:datastore-202"]
	if len(vmwarePool.Used) != 1 || len(vmwarePool.Avail) != 1 {
		t.Fatalf("expected canonical storage summary payload to preserve used/avail series, got %+v", vmwarePool)
	}
	if vmwarePool.Used[0].Timestamp != now.UnixMilli() || vmwarePool.Avail[0].Timestamp != now.UnixMilli() {
		t.Fatalf(
			"expected storage summary series to preserve millisecond timestamps, got used=%v avail=%v",
			vmwarePool.Used,
			vmwarePool.Avail,
		)
	}
}

func TestContract_StorageSummaryChartsStayAggregateOnly(t *testing.T) {
	monitor, state, metricsHistory := newTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	state.Storage = []models.Storage{
		{ID: "store-1", Name: "Store One"},
		{ID: "store-2", Name: "Store Two"},
	}
	metricsHistory.AddStorageMetric("store-1", "used", 400, now)
	metricsHistory.AddStorageMetric("store-1", "avail", 600, now)
	metricsHistory.AddStorageMetric("store-1", "total", 1000, now)
	metricsHistory.AddStorageMetric("store-2", "used", 100, now)
	metricsHistory.AddStorageMetric("store-2", "avail", 900, now)
	metricsHistory.AddStorageMetric("store-2", "total", 1000, now)
	syncTestResourceStore(t, monitor, state)

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/charts/storage-summary?range=24h", nil)
	rec := httptest.NewRecorder()

	router.handleStorageSummaryCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var decoded StorageSummaryTrendResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal storage summary response: %v", err)
	}
	if len(decoded.Capacity) != 1 {
		t.Fatalf("expected one aggregate capacity point, got %+v", decoded.Capacity)
	}
	if decoded.Capacity[0].Value != 25 {
		t.Fatalf("expected aggregate capacity of 25%%, got %+v", decoded.Capacity[0])
	}
	if decoded.Stats.PointCounts.Total != 1 || decoded.Stats.PointCounts.Storage != 1 {
		t.Fatalf("expected aggregate-only point counts, got %+v", decoded.Stats.PointCounts)
	}
}

func TestContract_TrueNASConnectionsDisabledMessageIsExplicit(t *testing.T) {
	setTrueNASFeatureForTest(t, false)
	handler, _, _ := newTrueNASHandlersForTest(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/truenas/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when TrueNAS integration is disabled, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "explicitly disabled") {
		t.Fatalf("expected explicit disable message, got %s", rec.Body.String())
	}
}

func TestContract_TrueNASSavedConnectionTestsUpdateRuntimeSummary(t *testing.T) {
	setTrueNASFeatureForTest(t, true)

	connection := config.TrueNASInstance{
		ID:                 "conn-1",
		Name:               "tower",
		Host:               "truenas.local",
		Port:               443,
		APIKey:             "super-secret",
		UseHTTPS:           true,
		InsecureSkipVerify: false,
		Enabled:            true,
	}
	handler, persistence, _ := newTrueNASHandlersForTest(t, nil)
	if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{connection}); err != nil {
		t.Fatalf("seed truenas config: %v", err)
	}
	poller := monitoring.NewTrueNASPoller(nil, time.Minute, nil)
	handler.getPoller = func(context.Context) *monitoring.TrueNASPoller { return poller }
	handler.newClient = func(cfg truenas.ClientConfig) (trueNASClient, error) {
		return &fakeTrueNASClient{}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/truenas/connections/conn-1/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	summary := poller.ConnectionSummaries("default", []config.TrueNASInstance{connection})[connection.ID]
	if summary.Poll == nil || summary.Poll.LastSuccessAt == nil {
		t.Fatalf("expected saved manual test to refresh poll summary, got %+v", summary.Poll)
	}
}

func TestContract_VMwareConnectionsDisabledMessageIsExplicit(t *testing.T) {
	setVMwareFeatureForTest(t, false)
	handler, _ := newVMwareHandlersForTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/vmware/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when VMware integration is disabled, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "explicitly disabled") {
		t.Fatalf("expected explicit disable message, got %s", rec.Body.String())
	}
}

func TestContract_VMwareSavedConnectionTestsUpdateRuntimeSummary(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	connection := config.VMwareVCenterInstance{
		ID:       "conn-1",
		Name:     "lab-vcenter",
		Host:     "vcsa.lab.local",
		Port:     443,
		Username: "administrator@vsphere.local",
		Password: "super-secret",
		Enabled:  true,
	}
	handler, persistence := newVMwareHandlersForTest(t)
	poller := monitoring.NewVMwarePoller(nil, time.Minute)
	handler.getPoller = func(context.Context) *monitoring.VMwarePoller { return poller }
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{connection}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}
	handler.newClient = func(cfg vmware.ClientConfig) (vmwareClient, error) {
		return &fakeVMwareClient{
			testConnection: func(context.Context) (*vmware.InventorySummary, error) {
				return &vmware.InventorySummary{Hosts: 3, VMs: 20, Datastores: 4, Networks: 6, VIRelease: "8.0.3"}, nil
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/conn-1/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	summary := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{connection})[connection.ID]
	if summary.Poll == nil || summary.Poll.LastSuccessAt == nil {
		t.Fatalf("expected saved manual test to refresh runtime summary, got %+v", summary.Poll)
	}
	if summary.Observed == nil || summary.Observed.VMs != 20 || summary.Observed.Networks != 6 {
		t.Fatalf("expected saved manual test to refresh observed summary, got %+v", summary.Observed)
	}
}

func TestContract_VMwareConnectionListCarriesObservedSummary(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	connection := config.VMwareVCenterInstance{
		ID:       "conn-1",
		Name:     "lab-vcenter",
		Host:     "vcsa.lab.local",
		Port:     443,
		Username: "administrator@vsphere.local",
		Password: "super-secret",
		Enabled:  true,
	}
	handler, persistence := newVMwareHandlersForTest(t)
	poller := monitoring.NewVMwarePoller(nil, time.Minute)
	handler.getPoller = func(context.Context) *monitoring.VMwarePoller { return poller }
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{connection}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}

	collectedAt := time.Date(2026, 3, 30, 18, 0, 0, 0, time.UTC)
	poller.RecordConnectionTestSuccess("default", connection.ID, &vmware.InventorySummary{
		Hosts:      4,
		VMs:        24,
		Datastores: 6,
		Networks:   8,
		VIRelease:  "8.0.3",
	}, collectedAt)

	req := httptest.NewRequest(http.MethodGet, "/api/vmware/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var responses []vmwareConnectionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &responses); err != nil {
		t.Fatalf("unmarshal vmware list response: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 vmware connection response, got %d", len(responses))
	}
	if responses[0].Password != "********" {
		t.Fatalf("expected redacted password in vmware list response, got %q", responses[0].Password)
	}
	if responses[0].Poll == nil || responses[0].Poll.LastSuccessAt == nil {
		t.Fatalf("expected poll summary in vmware list response, got %+v", responses[0])
	}
	if responses[0].Observed == nil {
		t.Fatalf("expected observed summary in vmware list response, got %+v", responses[0])
	}
	if got := responses[0].Observed.Hosts; got != 4 {
		t.Fatalf("observed hosts = %d, want 4", got)
	}
	if got := responses[0].Observed.VMs; got != 24 {
		t.Fatalf("observed vms = %d, want 24", got)
	}
	if got := responses[0].Observed.Datastores; got != 6 {
		t.Fatalf("observed datastores = %d, want 6", got)
	}
	if got := responses[0].Observed.Networks; got != 8 {
		t.Fatalf("observed networks = %d, want 8", got)
	}
	if got := responses[0].Observed.VIRelease; got != "8.0.3" {
		t.Fatalf("observed viRelease = %q, want 8.0.3", got)
	}
	if responses[0].Observed.CollectedAt == nil || !responses[0].Observed.CollectedAt.Equal(collectedAt) {
		t.Fatalf("observed collectedAt = %+v, want %s", responses[0].Observed.CollectedAt, collectedAt.Format(time.RFC3339))
	}
}

type mixedCadenceMetricSample struct {
	timestamp    time.Time
	percentValue float64
	rateValue    float64
}

func mixedCadenceLongRangeMetricSamples(now time.Time) []mixedCadenceMetricSample {
	windowStart := now.Add(-7 * 24 * time.Hour)
	samples := make([]mixedCadenceMetricSample, 0, 120)
	appendRange := func(start, end time.Time, step time.Duration, percentValue, rateValue float64) {
		for ts := start; ts.Before(end); ts = ts.Add(step) {
			samples = append(samples, mixedCadenceMetricSample{
				timestamp:    ts,
				percentValue: percentValue,
				rateValue:    rateValue,
			})
		}
	}

	appendRange(windowStart, now.Add(-24*time.Hour), 3*time.Hour, 20, 50)
	appendRange(now.Add(-24*time.Hour), now.Add(-2*time.Hour), 30*time.Minute, 40, 75)
	appendRange(now.Add(-2*time.Hour), now, 5*time.Minute, 60, 100)
	samples = append(samples, mixedCadenceMetricSample{
		timestamp:    now,
		percentValue: 75,
		rateValue:    125,
	})
	return samples
}

func TestContract_InfrastructureChartsNormalizeLongRangeMixedCadence(t *testing.T) {
	store := newTestMetricsStore(t)
	monitor, state, _ := newTestMonitor(t)
	setTestUnexportedField(t, monitor, "metricsStore", store)

	state.Nodes = []models.Node{{
		ID:       "node-contract-1",
		Name:     "node-contract-1",
		Instance: "pve1",
		Status:   "online",
		CPU:      0.75,
		Memory:   models.Memory{Usage: 42.0},
		Disk:     models.Disk{Usage: 55.0},
	}}
	syncTestResourceStore(t, monitor, state)

	now := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Minute)
	samples := mixedCadenceLongRangeMetricSamples(now)
	seed := make([]metrics.WriteMetric, 0, len(samples)*3)
	appendMetric := func(ts time.Time, value float64) {
		for _, metricType := range []string{"cpu", "memory", "disk"} {
			seed = append(seed, metrics.WriteMetric{
				ResourceType: "node",
				ResourceID:   "node-contract-1",
				MetricType:   metricType,
				Value:        value,
				Timestamp:    ts,
				Tier:         metrics.TierMinute,
			})
		}
	}
	for _, sample := range samples {
		appendMetric(sample.timestamp, sample.percentValue)
	}
	store.WriteBatchSync(seed)

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/charts/infrastructure?range=7d", nil)
	rec := httptest.NewRecorder()
	router.handleInfrastructureCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var decoded InfrastructureChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal infrastructure charts response: %v", err)
	}

	cpuSeries := decoded.NodeData["node-contract-1"]["cpu"]
	if len(cpuSeries) == 0 {
		t.Fatal("expected normalized cpu series")
	}
	if len(cpuSeries) > infrastructureSummaryMaxSeriesPoints {
		t.Fatalf("expected cpu series <= %d points, got %d", infrastructureSummaryMaxSeriesPoints, len(cpuSeries))
	}
	if cpuSeries[len(cpuSeries)-1].Timestamp != now.UnixMilli() {
		t.Fatalf("expected latest cpu timestamp %d, got %d", now.UnixMilli(), cpuSeries[len(cpuSeries)-1].Timestamp)
	}
	if cpuSeries[len(cpuSeries)-1].Value != 75 {
		t.Fatalf("expected latest cpu value 75, got %.2f", cpuSeries[len(cpuSeries)-1].Value)
	}

	recentWindowStart := now.Add(-24 * time.Hour).UnixMilli()
	recentCount := 0
	for _, point := range cpuSeries {
		if point.Timestamp >= recentWindowStart {
			recentCount++
		}
	}
	if recentCount > 20 {
		t.Fatalf("expected day-proportional recent summary buckets, got %d recent cpu points", recentCount)
	}
}

func TestContract_InfrastructureChartsHonorExplicitMetricFilters(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	state.Nodes = []models.Node{{
		ID:     "node-contract-1",
		Name:   "node-contract-1",
		Status: "online",
		CPU:    0.1,
		Memory: models.Memory{Usage: 12.0},
		Disk:   models.Disk{Usage: 34.0},
	}}
	state.DockerHosts = []models.DockerHost{{
		ID:       "docker-host-contract-1",
		Runtime:  "docker",
		CPUUsage: 23.0,
		Memory:   models.Memory{Usage: 45.0},
		Disks:    []models.Disk{{Usage: 67.0}},
		Status:   "online",
	}}
	state.Hosts = []models.Host{{
		ID:       "agent-contract-1",
		Hostname: "agent-contract-1",
		CPUUsage: 11.0,
		Memory:   models.Memory{Usage: 22.0},
		Disks:    []models.Disk{{Usage: 33.0}},
		Status:   "online",
	}}
	syncTestResourceStore(t, monitor, state)

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/charts/infrastructure?range=5m&metrics=cpu,memory",
		nil,
	)
	rec := httptest.NewRecorder()

	router.handleInfrastructureCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var decoded InfrastructureChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal infrastructure charts response: %v", err)
	}

	if decoded.Stats.PointCounts.Nodes != 2 {
		t.Fatalf("expected stats.pointCounts.nodes=2, got %d", decoded.Stats.PointCounts.Nodes)
	}
	if decoded.Stats.PointCounts.DockerHosts != 2 {
		t.Fatalf(
			"expected stats.pointCounts.dockerHosts=2, got %d",
			decoded.Stats.PointCounts.DockerHosts,
		)
	}
	if decoded.Stats.PointCounts.Agents != 2 {
		t.Fatalf("expected stats.pointCounts.agents=2, got %d", decoded.Stats.PointCounts.Agents)
	}
	if _, ok := decoded.NodeData["node-contract-1"]["disk"]; ok {
		t.Fatal("expected node disk series to be filtered out of infrastructure summary payload")
	}
	if got := len(decoded.NodeData["node-contract-1"]); got != 2 {
		t.Fatalf("expected node payload to contain only requested metrics, got %d entries", got)
	}
	if _, ok := decoded.DockerHostData["docker-host-contract-1"]["disk"]; ok {
		t.Fatal("expected docker-host disk series to be filtered out of infrastructure summary payload")
	}
	if got := len(decoded.DockerHostData["docker-host-contract-1"]); got != 2 {
		t.Fatalf("expected docker-host payload to contain only requested metrics, got %d entries", got)
	}
	if _, ok := decoded.AgentData["agent-contract-1"]["disk"]; ok {
		t.Fatal("expected agent disk series to be filtered out of infrastructure summary payload")
	}
	if got := len(decoded.AgentData["agent-contract-1"]); got != 2 {
		t.Fatalf("expected agent payload to contain only requested metrics, got %d entries", got)
	}
}

func TestContract_MockChartRoutesUseCanonicalMockUnifiedReadStateForVMwareHosts(t *testing.T) {
	setMockModeForTest(t, true)

	fixtures := vmware.DefaultFixtures()
	if len(fixtures.Hosts) == 0 {
		t.Fatal("expected default VMware fixtures to include at least one host")
	}
	expectedHostID := vmware.SourceID(fixtures.ConnectionID, "host", fixtures.Hosts[0].Host)

	monitor, state, _ := newTestMonitor(t)
	state.Hosts = []models.Host{{
		ID:       "live-store-host-1",
		Hostname: "live-store-host-1",
		CPUUsage: 11.0,
		Memory:   models.Memory{Usage: 22.0},
		Disks:    []models.Disk{{Usage: 33.0}},
		Status:   "online",
	}}
	syncTestResourceStore(t, monitor, state)

	router := &Router{monitor: monitor}
	for _, path := range []string{"/api/charts?range=5m", "/api/charts/infrastructure?range=5m"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()

		switch path {
		case "/api/charts?range=5m":
			router.handleCharts(rec, req)
		case "/api/charts/infrastructure?range=5m":
			router.handleInfrastructureCharts(rec, req)
		default:
			t.Fatalf("unexpected path %q", path)
		}

		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", path, rec.Code, rec.Body.String())
		}

		agentData := map[string]VMChartData{}
		if path == "/api/charts?range=5m" {
			var decoded ChartResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
				t.Fatalf("decode %s: %v", path, err)
			}
			agentData = decoded.AgentData
		} else {
			var decoded InfrastructureChartsResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
				t.Fatalf("decode %s: %v", path, err)
			}
			agentData = decoded.AgentData
		}

		series, ok := agentData[expectedHostID]
		if !ok {
			t.Fatalf("%s missing VMware host %q; got keys=%v", path, expectedHostID, sortedVMChartKeys(agentData))
		}
		if len(series["cpu"]) == 0 {
			t.Fatalf("%s expected VMware host %q cpu series", path, expectedHostID)
		}
		if _, ok := agentData["live-store-host-1"]; ok {
			t.Fatalf("%s unexpectedly used live store-only host instead of canonical mock snapshot; keys=%v", path, sortedVMChartKeys(agentData))
		}
	}
}

func TestContract_WorkloadChartsCapLongRangeMixedCadenceByTime(t *testing.T) {
	store := newTestMetricsStore(t)
	monitor, state, _ := newTestMonitor(t)
	setTestUnexportedField(t, monitor, "metricsStore", store)

	state.Nodes = []models.Node{{
		ID:       "node-contract-1",
		Name:     "node-contract-1",
		Instance: "pve1",
		Status:   "online",
	}}
	state.VMs = []models.VM{{
		ID:         "vm-contract-1",
		VMID:       101,
		Name:       "vm-contract-1",
		Node:       "node-contract-1",
		Instance:   "pve1",
		Status:     "running",
		CPU:        0.75,
		Memory:     models.Memory{Usage: 42.0},
		Disk:       models.Disk{Usage: 55.0},
		NetworkIn:  128,
		NetworkOut: 256,
		DiskRead:   512,
		DiskWrite:  256,
	}}
	syncTestResourceStore(t, monitor, state)

	readState := monitor.GetUnifiedReadStateOrSnapshot()
	vms := readState.VMs()
	if len(vms) != 1 {
		t.Fatalf("expected 1 vm view, got %d", len(vms))
	}
	sourceID := strings.TrimSpace(vms[0].SourceID())
	if sourceID == "" {
		t.Fatal("expected vm source ID")
	}

	now := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Minute)
	samples := mixedCadenceLongRangeMetricSamples(now)
	seed := make([]metrics.WriteMetric, 0, len(samples)*7)
	appendMetric := func(ts time.Time, percentValue, rateValue float64) {
		for _, metricType := range []string{"cpu", "memory", "disk"} {
			seed = append(seed, metrics.WriteMetric{
				ResourceType: "vm",
				ResourceID:   sourceID,
				MetricType:   metricType,
				Value:        percentValue,
				Timestamp:    ts,
				Tier:         metrics.TierMinute,
			})
		}
		for _, metricType := range []string{"diskread", "diskwrite", "netin", "netout"} {
			seed = append(seed, metrics.WriteMetric{
				ResourceType: "vm",
				ResourceID:   sourceID,
				MetricType:   metricType,
				Value:        rateValue,
				Timestamp:    ts,
				Tier:         metrics.TierMinute,
			})
		}
	}
	for _, sample := range samples {
		appendMetric(sample.timestamp, sample.percentValue, sample.rateValue)
	}
	store.WriteBatchSync(seed)

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/charts/workloads?range=7d&maxPoints=30", nil)
	rec := httptest.NewRecorder()
	router.handleWorkloadCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var decoded WorkloadChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal workload charts response: %v", err)
	}

	if len(decoded.ChartData) != 1 {
		t.Fatalf("expected 1 workload chart entry, got %d", len(decoded.ChartData))
	}
	var cpuSeries []MetricPoint
	for _, chartData := range decoded.ChartData {
		cpuSeries = chartData["cpu"]
		break
	}
	if len(cpuSeries) == 0 {
		t.Fatal("expected capped cpu series")
	}
	if len(cpuSeries) > 30 {
		t.Fatalf("expected cpu series <= 30 points, got %d", len(cpuSeries))
	}
	if cpuSeries[len(cpuSeries)-1].Timestamp != now.UnixMilli() {
		t.Fatalf("expected latest cpu timestamp %d, got %d", now.UnixMilli(), cpuSeries[len(cpuSeries)-1].Timestamp)
	}

	recentWindowStart := now.Add(-24 * time.Hour).UnixMilli()
	recentCount := 0
	for _, point := range cpuSeries {
		if point.Timestamp >= recentWindowStart {
			recentCount++
		}
	}
	if recentCount > 8 {
		t.Fatalf("expected day-proportional capped workload points, got %d recent cpu points", recentCount)
	}
}

func TestContract_WorkloadChartsUseCanonicalWorkloadIDsForProviderBackedVMs(t *testing.T) {
	monitor, state, history := newTestMonitor(t)
	now := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Minute)
	metricID := "vc-1:vm:vm-201"

	history.AddGuestMetric(metricID, "cpu", 51, now.Add(-10*time.Minute))
	history.AddGuestMetric(metricID, "memory", 64, now.Add(-5*time.Minute))
	history.AddGuestMetric(metricID, "disk", 43, now.Add(-3*time.Minute))
	history.AddGuestMetric(metricID, "netin", 1200, now.Add(-2*time.Minute))
	history.AddGuestMetric(metricID, "netout", 800, now.Add(-2*time.Minute))

	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateSnapshotAndSupplemental(state.GetSnapshot(), map[unifiedresources.DataSource][]unifiedresources.IngestRecord{
		unifiedresources.SourceVMware: {
			{
				SourceID: metricID,
				Resource: unifiedresources.Resource{
					ID:       "vm-vmware-contract",
					Type:     unifiedresources.ResourceTypeVM,
					Name:     "warehouse-api-01",
					Status:   unifiedresources.StatusOnline,
					LastSeen: now,
					MetricsTarget: &unifiedresources.MetricsTarget{
						ResourceType: "vm",
						ResourceID:   metricID,
					},
					VMware: &unifiedresources.VMwareData{
						ConnectionID:    "vc-1",
						EntityType:      "vm",
						ManagedObjectID: "vm-201",
					},
				},
			},
		},
	})
	setTestUnexportedField(t, monitor, "resourceStore", monitoring.ResourceStoreInterface(adapter))

	readState := monitor.GetUnifiedReadStateOrSnapshot()
	if readState == nil || len(readState.VMs()) != 1 || readState.VMs()[0] == nil {
		t.Fatalf("expected one provider-backed VM in read state, got %+v", readState)
	}
	resourceID, _, ok := vmChartRequest(readState.VMs()[0])
	if !ok {
		t.Fatal("expected canonical vm chart request")
	}

	router := &Router{monitor: monitor}

	workloadReq := httptest.NewRequest(http.MethodGet, "/api/charts/workloads?range=1h", nil)
	workloadRec := httptest.NewRecorder()
	router.handleWorkloadCharts(workloadRec, workloadReq)

	if workloadRec.Code != http.StatusOK {
		t.Fatalf("expected workload charts 200, got %d: %s", workloadRec.Code, workloadRec.Body.String())
	}

	var workloadDecoded WorkloadChartsResponse
	if err := json.Unmarshal(workloadRec.Body.Bytes(), &workloadDecoded); err != nil {
		t.Fatalf("unmarshal workload charts response: %v", err)
	}
	if _, ok := workloadDecoded.ChartData[resourceID]; !ok {
		t.Fatalf("expected workload charts keyed by canonical workload id %q, got %v", resourceID, workloadDecoded.ChartData)
	}
	if _, ok := workloadDecoded.ChartData[metricID]; ok {
		t.Fatalf("expected provider metrics target %q to stay out of workload chart response keys", metricID)
	}
	if workloadDecoded.GuestTypes[resourceID] != "vm" {
		t.Fatalf("expected vm guest type for %q, got %q", resourceID, workloadDecoded.GuestTypes[resourceID])
	}

	summaryReq := httptest.NewRequest(http.MethodGet, "/api/charts/workloads-summary?range=1h", nil)
	summaryRec := httptest.NewRecorder()
	router.handleWorkloadsSummaryCharts(summaryRec, summaryReq)

	if summaryRec.Code != http.StatusOK {
		t.Fatalf("expected workloads summary 200, got %d: %s", summaryRec.Code, summaryRec.Body.String())
	}

	var summaryDecoded WorkloadsSummaryChartsResponse
	if err := json.Unmarshal(summaryRec.Body.Bytes(), &summaryDecoded); err != nil {
		t.Fatalf("unmarshal workloads summary response: %v", err)
	}
	if summaryDecoded.GuestCounts.Total != 1 || summaryDecoded.GuestCounts.Running != 1 {
		t.Fatalf("expected stable provider-backed guest counts, got %+v", summaryDecoded.GuestCounts)
	}
	if len(summaryDecoded.TopContributors.CPU) == 0 {
		t.Fatal("expected provider-backed cpu top contributor")
	}
	if summaryDecoded.TopContributors.CPU[0].ID != resourceID {
		t.Fatalf("expected workloads summary contributor id %q, got %+v", resourceID, summaryDecoded.TopContributors.CPU[0])
	}
	if summaryDecoded.TopContributors.CPU[0].ID == metricID {
		t.Fatalf("expected workloads summary contributor id to avoid raw metrics target %q", metricID)
	}
}

func TestContract_WorkloadsSummaryChartsNormalizeLongRangeMixedCadence(t *testing.T) {
	store := newTestMetricsStore(t)
	monitor, state, _ := newTestMonitor(t)
	setTestUnexportedField(t, monitor, "metricsStore", store)

	state.Nodes = []models.Node{{
		ID:       "node-contract-1",
		Name:     "node-contract-1",
		Instance: "pve1",
		Status:   "online",
	}}
	state.VMs = []models.VM{{
		ID:         "vm-contract-1",
		VMID:       101,
		Name:       "vm-contract-1",
		Node:       "node-contract-1",
		Instance:   "pve1",
		Status:     "running",
		CPU:        0.75,
		Memory:     models.Memory{Usage: 42.0},
		Disk:       models.Disk{Usage: 55.0},
		NetworkIn:  128,
		NetworkOut: 256,
	}}
	syncTestResourceStore(t, monitor, state)

	readState := monitor.GetUnifiedReadStateOrSnapshot()
	vms := readState.VMs()
	if len(vms) != 1 {
		t.Fatalf("expected 1 vm view, got %d", len(vms))
	}
	sourceID := strings.TrimSpace(vms[0].SourceID())
	if sourceID == "" {
		t.Fatal("expected vm source ID")
	}

	now := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Minute)
	samples := mixedCadenceLongRangeMetricSamples(now)
	seed := make([]metrics.WriteMetric, 0, len(samples)*5)
	appendMetric := func(ts time.Time, percentValue, rateValue float64) {
		for _, metricType := range []string{"cpu", "memory", "disk"} {
			seed = append(seed, metrics.WriteMetric{
				ResourceType: "vm",
				ResourceID:   sourceID,
				MetricType:   metricType,
				Value:        percentValue,
				Timestamp:    ts,
				Tier:         metrics.TierMinute,
			})
		}
		for _, metricType := range []string{"netin", "netout"} {
			seed = append(seed, metrics.WriteMetric{
				ResourceType: "vm",
				ResourceID:   sourceID,
				MetricType:   metricType,
				Value:        rateValue,
				Timestamp:    ts,
				Tier:         metrics.TierMinute,
			})
		}
	}
	for _, sample := range samples {
		appendMetric(sample.timestamp, sample.percentValue, sample.rateValue)
	}
	store.WriteBatchSync(seed)

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/charts/workloads-summary?range=7d", nil)
	rec := httptest.NewRecorder()
	router.handleWorkloadsSummaryCharts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var decoded WorkloadsSummaryChartsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal workloads summary charts response: %v", err)
	}

	if len(decoded.CPU.P50) == 0 {
		t.Fatal("expected normalized workload summary p50 series")
	}
	if len(decoded.CPU.P50) > workloadsSummaryMaxSeriesPoints {
		t.Fatalf("expected workload summary p50 <= %d points, got %d", workloadsSummaryMaxSeriesPoints, len(decoded.CPU.P50))
	}
	if len(decoded.CPU.P95) > workloadsSummaryMaxSeriesPoints {
		t.Fatalf("expected workload summary p95 <= %d points, got %d", workloadsSummaryMaxSeriesPoints, len(decoded.CPU.P95))
	}
	if decoded.CPU.P50[len(decoded.CPU.P50)-1].Timestamp != now.UnixMilli() {
		t.Fatalf("expected latest workload summary timestamp %d, got %d", now.UnixMilli(), decoded.CPU.P50[len(decoded.CPU.P50)-1].Timestamp)
	}

	recentWindowStart := now.Add(-24 * time.Hour).UnixMilli()
	recentCount := 0
	for _, point := range decoded.CPU.P50 {
		if point.Timestamp >= recentWindowStart {
			recentCount++
		}
	}
	if recentCount > 20 {
		t.Fatalf("expected day-proportional summary buckets, got %d recent p50 points", recentCount)
	}
}

func TestContract_WorkloadChartMetricBudgetGuardrailsRemainCanonical(t *testing.T) {
	data, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("failed to read router.go: %v", err)
	}
	source := string(data)

	requiredSnippets := []string{
		`var workloadSummaryMetricOrder = []string{`,
		`"cpu",`,
		`"memory",`,
		`"disk",`,
		`"netin",`,
		`"netout",`,
		`var workloadChartsBatchWG sync.WaitGroup`,
		`workloadChartsBatchWG.Add(4)`,
		`podBatchMetrics = monitor.GetGuestMetricsForChartBatch("k8s", podRequests, duration, workloadSummaryMetricOrder...)`,
		`var workloadsSummaryBatchWG sync.WaitGroup`,
		`workloadsSummaryBatchWG.Add(4)`,
		`vmBatchMetrics = monitor.GetGuestMetricsForChartBatch("vm", vmRequests, duration, workloadSummaryMetricOrder...)`,
		`containerBatchMetrics = monitor.GetGuestMetricsForChartBatch("container", containerRequests, duration, workloadSummaryMetricOrder...)`,
		`dockerContainerBatchMetrics = monitor.GetGuestMetricsForChartBatch("dockerContainer", dockerContainerRequests, duration, workloadSummaryMetricOrder...)`,
		`summaryChartsCacheTTL = 5 * time.Second`,
		`infrastructureChartsCacheKey(req *http.Request, timeRange string, requestedMetricNames []string) string`,
		`cachedInfrastructureChartsPayload`,
		`cacheInfrastructureChartsPayload`,
		`workloadsSummaryChartsCacheKey`,
		`cachedWorkloadsSummaryChartsPayload`,
		`cacheWorkloadsSummaryChartsPayload`,
		`type workloadSummaryMetricBucket struct`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("router.go must contain %q", snippet)
		}
	}
}

func TestContract_GenerateStyledMockSeries_UsesTimestampBasedCurve(t *testing.T) {
	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC).UnixMilli()

	coarse := generateStyledMockSeries(
		now,
		time.Hour,
		7,
		51.9,
		"dockerContainer",
		"orion-2-f54579833f9c",
		"memory",
	)
	fine := generateStyledMockSeries(
		now,
		time.Hour,
		13,
		51.9,
		"dockerContainer",
		"orion-2-f54579833f9c",
		"memory",
	)

	if len(coarse) != 7 || len(fine) != 13 {
		t.Fatalf("unexpected synthetic series lengths coarse=%d fine=%d", len(coarse), len(fine))
	}

	for i, point := range coarse {
		fineIndex := i * 2
		if fine[fineIndex].Timestamp != point.Timestamp {
			t.Fatalf(
				"expected shared timestamp at coarse[%d]=%d to match fine[%d]=%d",
				i,
				point.Timestamp,
				fineIndex,
				fine[fineIndex].Timestamp,
			)
		}
		if fine[fineIndex].Value != point.Value {
			t.Fatalf(
				"expected shared timestamp value at coarse[%d]=%f to match fine[%d]=%f",
				i,
				point.Value,
				fineIndex,
				fine[fineIndex].Value,
			)
		}
	}
}

func TestContract_PlatformMockToggleRebindsRuntimeConnectionsAndResources(t *testing.T) {
	setMockModeForTest(t, false)

	cfg := &config.Config{DataPath: t.TempDir()}
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("new monitor: %v", err)
	}
	if err := monitor.SetMockMode(false); err != nil {
		t.Fatalf("set monitor mock mode: %v", err)
	}

	router := NewRouter(cfg, monitor, nil, nil, nil, "1.0.0")
	t.Cleanup(func() {
		router.shutdownBackgroundWorkers()
	})

	toggleReq := httptest.NewRequest(http.MethodPost, "/api/system/mock-mode", strings.NewReader(`{"enabled":true}`))
	toggleRec := httptest.NewRecorder()
	router.configHandlers.HandleUpdateMockMode(toggleRec, toggleReq)
	if toggleRec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d, body=%s", toggleRec.Code, toggleRec.Body.String())
	}

	truenasListRec := httptest.NewRecorder()
	truenasListReq := httptest.NewRequest(http.MethodGet, "/api/truenas/connections", nil)
	router.trueNASHandlers.HandleList(truenasListRec, truenasListReq)
	if truenasListRec.Code != http.StatusOK {
		t.Fatalf("truenas connections status = %d, body=%s", truenasListRec.Code, truenasListRec.Body.String())
	}
	var truenasConnections []trueNASConnectionResponse
	if err := json.Unmarshal(truenasListRec.Body.Bytes(), &truenasConnections); err != nil {
		t.Fatalf("decode truenas connections: %v", err)
	}
	if len(truenasConnections) != 1 || truenasConnections[0].ID != "truenas-mock-1" {
		t.Fatalf("expected mock truenas connection, got %#v", truenasConnections)
	}

	vmwareListRec := httptest.NewRecorder()
	vmwareListReq := httptest.NewRequest(http.MethodGet, "/api/vmware/connections", nil)
	router.vmwareHandlers.HandleList(vmwareListRec, vmwareListReq)
	if vmwareListRec.Code != http.StatusOK {
		t.Fatalf("vmware connections status = %d, body=%s", vmwareListRec.Code, vmwareListRec.Body.String())
	}
	var vmwareConnections []vmwareConnectionResponse
	if err := json.Unmarshal(vmwareListRec.Body.Bytes(), &vmwareConnections); err != nil {
		t.Fatalf("decode vmware connections: %v", err)
	}
	if len(vmwareConnections) != 1 || vmwareConnections[0].ID != "vc-mock-1" {
		t.Fatalf("expected mock vmware connection, got %#v", vmwareConnections)
	}

	assertResourceSource := func(path string, wantSource unifiedresources.DataSource) {
		t.Helper()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.resourceHandlers.HandleListResources(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", path, rec.Code, rec.Body.String())
		}

		var resp ResourcesResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode %s response: %v", path, err)
		}
		for _, resource := range resp.Data {
			for _, source := range resource.Sources {
				if source == wantSource {
					return
				}
			}
		}
		t.Fatalf("expected %s response to include source %q, got %#v", path, wantSource, resp.Data)
	}

	assertResourceSource("/api/resources?source=truenas", unifiedresources.SourceTrueNAS)
	assertResourceSource("/api/resources?source=vmware-vsphere", unifiedresources.SourceVMware)

	assertResourceCount := func(path string, want int) {
		t.Helper()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.resourceHandlers.HandleListResources(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", path, rec.Code, rec.Body.String())
		}

		var resp ResourcesResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode %s response: %v", path, err)
		}
		if len(resp.Data) != want {
			t.Fatalf("%s returned %d resources, want %d", path, len(resp.Data), want)
		}
	}

	assertResourceCount("/api/resources?source=truenas&type=app-container", len(truenas.DefaultFixtures().Apps))
	assertResourceCount("/api/resources?source=truenas&type=network-share", len(truenas.DefaultFixtures().Shares))
	assertResourceCount("/api/resources?source=vmware-vsphere&type=storage", len(vmware.DefaultFixtures().Datastores))
	assertResourceCount("/api/resources?source=vmware-vsphere&type=network", len(vmware.DefaultFixtures().Networks))
}

func TestContract_PlatformMockConnectionListsUseSharedFixtureMetadata(t *testing.T) {
	setTrueNASFeatureForTest(t, true)
	setVMwareFeatureForTest(t, true)

	setMockModeForTest(t, true)

	t.Run("truenas", func(t *testing.T) {
		fixture := mock.DefaultTrueNASConnectionFixture()
		if fixture.CollectedAt.IsZero() {
			t.Fatal("expected canonical truenas mock fixture timestamp")
		}

		handler, _, _ := newTrueNASHandlersForTest(t, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/truenas/connections", nil)
		rec := httptest.NewRecorder()
		handler.HandleList(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var responses []trueNASConnectionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &responses); err != nil {
			t.Fatalf("decode truenas mock list response: %v", err)
		}
		if len(responses) != 1 {
			t.Fatalf("expected 1 mock truenas connection, got %d", len(responses))
		}

		response := responses[0]
		if response.ID != fixture.ID || response.Name != fixture.Name || response.Host != fixture.Host || response.Port != fixture.Port {
			t.Fatalf("unexpected truenas mock connection metadata: got %+v want fixture %+v", response.TrueNASInstance, fixture)
		}
		if response.APIKey != "********" {
			t.Fatalf("expected redacted truenas api key, got %q", response.APIKey)
		}
		if response.Poll == nil || response.Poll.IntervalSeconds != fixture.PollIntervalSeconds {
			t.Fatalf("expected truenas poll interval %d, got %+v", fixture.PollIntervalSeconds, response.Poll)
		}
		if response.Poll.LastSuccessAt == nil || !response.Poll.LastSuccessAt.Equal(fixture.CollectedAt) {
			t.Fatalf("expected truenas last success at %s, got %+v", fixture.CollectedAt.Format(time.RFC3339), response.Poll)
		}
		if response.Observed == nil {
			t.Fatal("expected truenas observed summary")
		}
		if response.Observed.Host != fixture.Host ||
			response.Observed.ResourceID != fixture.ResourceID ||
			response.Observed.Systems != fixture.Systems ||
			response.Observed.StoragePools != fixture.StoragePools ||
			response.Observed.Datasets != fixture.Datasets ||
			response.Observed.Apps != fixture.Apps ||
			response.Observed.Disks != fixture.Disks ||
			response.Observed.RecoveryArtifacts != fixture.RecoveryArtifacts {
			t.Fatalf("unexpected truenas observed summary: got %+v want fixture %+v", response.Observed, fixture)
		}
		if response.Observed.CollectedAt == nil || !response.Observed.CollectedAt.Equal(fixture.CollectedAt) {
			t.Fatalf("expected truenas observed collectedAt %s, got %+v", fixture.CollectedAt.Format(time.RFC3339), response.Observed)
		}
	})

	t.Run("vmware", func(t *testing.T) {
		fixture := mock.DefaultVMwareConnectionFixture()
		if fixture.CollectedAt.IsZero() {
			t.Fatal("expected canonical vmware mock fixture timestamp")
		}

		handler, _ := newVMwareHandlersForTest(t)
		req := httptest.NewRequest(http.MethodGet, "/api/vmware/connections", nil)
		rec := httptest.NewRecorder()
		handler.HandleList(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var responses []vmwareConnectionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &responses); err != nil {
			t.Fatalf("decode vmware mock list response: %v", err)
		}
		if len(responses) != 1 {
			t.Fatalf("expected 1 mock vmware connection, got %d", len(responses))
		}

		response := responses[0]
		if response.ID != fixture.ID || response.Name != fixture.Name || response.Host != fixture.Host || response.Port != fixture.Port || response.Username != fixture.Username {
			t.Fatalf("unexpected vmware mock connection metadata: got %+v want fixture %+v", response.VMwareVCenterInstance, fixture)
		}
		if response.Password != "********" {
			t.Fatalf("expected redacted vmware password, got %q", response.Password)
		}
		if response.Poll == nil || response.Poll.IntervalSeconds != fixture.PollIntervalSeconds {
			t.Fatalf("expected vmware poll interval %d, got %+v", fixture.PollIntervalSeconds, response.Poll)
		}
		if response.Poll.LastSuccessAt == nil || !response.Poll.LastSuccessAt.Equal(fixture.CollectedAt) {
			t.Fatalf("expected vmware last success at %s, got %+v", fixture.CollectedAt.Format(time.RFC3339), response.Poll)
		}
		if response.Observed == nil {
			t.Fatal("expected vmware observed summary")
		}
		if response.Observed.Hosts != fixture.Hosts ||
			response.Observed.VMs != fixture.VMs ||
			response.Observed.Datastores != fixture.Datastores ||
			response.Observed.VIRelease != fixture.VIRelease {
			t.Fatalf("unexpected vmware observed summary: got %+v want fixture %+v", response.Observed, fixture)
		}
		if response.Observed.CollectedAt == nil || !response.Observed.CollectedAt.Equal(fixture.CollectedAt) {
			t.Fatalf("expected vmware observed collectedAt %s, got %+v", fixture.CollectedAt.Format(time.RFC3339), response.Observed)
		}
	})
}

func TestContract_VMwareConnectionListCarriesDegradedObservedSummary(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	connection := config.VMwareVCenterInstance{
		ID:       "conn-1",
		Name:     "lab-vcenter",
		Host:     "vcsa.lab.local",
		Port:     443,
		Username: "administrator@vsphere.local",
		Password: "super-secret",
		Enabled:  true,
	}
	handler, persistence := newVMwareHandlersForTest(t)
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{connection}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}

	collectedAt := time.Date(2026, 3, 31, 18, 15, 0, 0, time.UTC)
	handler.statusMu.Lock()
	handler.statuses = map[string]vmwareConnectionRuntimeStatus{
		connection.ID: {
			Poll: &monitoring.VMwareConnectionPollStatus{
				IntervalSeconds: 60,
				LastSuccessAt:   &collectedAt,
			},
			Observed: &monitoring.VMwareConnectionObservedSummary{
				CollectedAt: &collectedAt,
				Hosts:       4,
				VMs:         24,
				Datastores:  6,
				VIRelease:   "8.0.3",
				Degraded:    true,
				IssueCount:  3,
				Issues: []monitoring.VMwareConnectionObservedIssue{
					{
						Stage:       "signals",
						Category:    "permission",
						Message:     "VMware permissions are insufficient for HostSystem overall status",
						Occurrences: 2,
					},
					{
						Stage:       "topology",
						Category:    "unavailable",
						Message:     "VMware vm guest identity is temporarily unavailable",
						Occurrences: 1,
					},
				},
			},
		},
	}
	handler.statusMu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/vmware/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var responses []vmwareConnectionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &responses); err != nil {
		t.Fatalf("unmarshal vmware degraded list response: %v", err)
	}
	if len(responses) != 1 || responses[0].Observed == nil {
		t.Fatalf("expected 1 vmware connection with degraded observed summary, got %+v", responses)
	}
	if !responses[0].Observed.Degraded {
		t.Fatalf("expected degraded observed summary, got %+v", responses[0].Observed)
	}
	if responses[0].Observed.IssueCount != 3 {
		t.Fatalf("observed issueCount = %d, want 3", responses[0].Observed.IssueCount)
	}
	if len(responses[0].Observed.Issues) != 2 {
		t.Fatalf("observed issues len = %d, want 2", len(responses[0].Observed.Issues))
	}
	if responses[0].Observed.Issues[0].Stage != "signals" || responses[0].Observed.Issues[0].Occurrences != 2 {
		t.Fatalf("unexpected first observed issue: %+v", responses[0].Observed.Issues[0])
	}
	if responses[0].Observed.Issues[1].Stage != "topology" || responses[0].Observed.Issues[1].Occurrences != 1 {
		t.Fatalf("unexpected second observed issue: %+v", responses[0].Observed.Issues[1])
	}
}

func TestContract_SSOTestRejectsMetadataURLWithUserinfo(t *testing.T) {
	called := make(chan struct{}, 1)
	metadataServer := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case called <- struct{}{}:
		default:
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testSAMLMetadata))
	}))
	defer metadataServer.Close()

	metadataURL, err := url.Parse(metadataServer.URL)
	if err != nil {
		t.Fatalf("parse metadata server url: %v", err)
	}
	metadataURL.User = url.UserPassword("user", "pass")

	reqBody := SSOTestRequest{
		Type: "saml",
		SAML: &SAMLTestConfig{
			IDPMetadataURL: metadataURL.String(),
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var resp SSOTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Success {
		t.Fatalf("expected failed response, got success: %+v", resp)
	}

	select {
	case <-called:
		t.Fatal("expected SAML metadata URL with userinfo to be rejected before outbound fetch")
	default:
	}
}

func TestContract_SSOTestRejectsManualSLOURLWithUserinfo(t *testing.T) {
	reqBody := SSOTestRequest{
		Type: "saml",
		SAML: &SAMLTestConfig{
			IDPSSOURL: "https://idp.example.com/sso",
			IDPSLOURL: "https://user:pass@idp.example.com/slo",
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var resp SSOTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Success {
		t.Fatalf("expected failed response, got success: %+v", resp)
	}
	if resp.Message != "Invalid SLO URL format" {
		t.Fatalf("expected invalid SLO URL message, got %+v", resp)
	}
}

func TestContract_SSOTestOIDCDiscoveryKeepsIssuerBasePath(t *testing.T) {
	var issuerURL string
	discoveryServer := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/realms/pulse/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"issuer": %q,
			"authorization_endpoint": %q,
			"token_endpoint": %q,
			"userinfo_endpoint": %q,
			"jwks_uri": %q,
			"scopes_supported": ["openid", "profile", "email"]
		}`, issuerURL, issuerURL+"/protocol/openid-connect/auth", issuerURL+"/protocol/openid-connect/token", issuerURL+"/protocol/openid-connect/userinfo", issuerURL+"/protocol/openid-connect/certs")
	}))
	defer discoveryServer.Close()
	issuerURL = discoveryServer.URL + "/auth/realms/pulse"

	reqBody := SSOTestRequest{
		Type: "oidc",
		OIDC: &OIDCTestConfig{
			IssuerURL: issuerURL,
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp SSOTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response, got %+v", resp)
	}
	if resp.Details == nil {
		t.Fatal("expected OIDC details in response")
	}
	if resp.Details.EntityID != issuerURL {
		t.Fatalf("issuer=%q, want %q", resp.Details.EntityID, issuerURL)
	}
}

func TestContract_SSOProviderDetailRoundTripKeepsEditableOIDCConfig(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	router := &Router{
		config: &config.Config{
			DataPath:  tmp,
			PublicURL: "https://pulse.example.com",
		},
		persistence: persistence,
		ssoConfig:   config.NewSSOConfig(),
		samlManager: NewSAMLServiceManager("https://pulse.example.com"),
		oidcManager: NewOIDCServiceManager(),
	}
	if err := persistence.SaveSSOConfig(router.ssoConfig); err != nil {
		t.Fatalf("save initial sso config: %v", err)
	}

	provider := config.SSOProvider{
		ID:                "corp-oidc",
		Name:              "Corporate OIDC",
		Type:              config.SSOProviderTypeOIDC,
		Enabled:           true,
		AllowedGroups:     []string{"admins", "operators"},
		AllowedDomains:    []string{"example.com"},
		AllowedEmails:     []string{"owner@example.com"},
		GroupsClaim:       "groups",
		GroupRoleMappings: map[string]string{"admins": "admin"},
		OIDC: &config.OIDCProviderConfig{
			IssuerURL:    "https://idp.example.com/realms/pulse",
			ClientID:     "pulse-client",
			ClientSecret: "super-secret",
			RedirectURL:  "https://pulse.example.com/api/oidc/corp-oidc/callback",
			LogoutURL:    "https://idp.example.com/logout",
			Scopes:       []string{"openid", "profile", "email", "groups"},
		},
	}
	if err := router.ssoConfig.AddProvider(provider); err != nil {
		t.Fatalf("add provider: %v", err)
	}
	if err := router.saveSSOConfig(); err != nil {
		t.Fatalf("save provider: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/security/sso/providers/corp-oidc", nil)
	rec := httptest.NewRecorder()
	router.handleSSOProvider(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "super-secret") {
		t.Fatal("provider detail response exposed raw OIDC client secret")
	}

	var detail SSOProviderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("unmarshal detail response: %v", err)
	}
	if detail.OIDC == nil {
		t.Fatal("provider detail response missing nested OIDC edit config")
	}
	if detail.OIDC.RedirectURL != provider.OIDC.RedirectURL {
		t.Fatalf("redirect URL = %q, want %q", detail.OIDC.RedirectURL, provider.OIDC.RedirectURL)
	}
	if detail.OIDC.LogoutURL != provider.OIDC.LogoutURL {
		t.Fatalf("logout URL = %q, want %q", detail.OIDC.LogoutURL, provider.OIDC.LogoutURL)
	}
	if !reflect.DeepEqual(detail.OIDC.Scopes, provider.OIDC.Scopes) {
		t.Fatalf("scopes = %#v, want %#v", detail.OIDC.Scopes, provider.OIDC.Scopes)
	}
	if detail.GroupsClaim != "groups" || !reflect.DeepEqual(detail.GroupRoleMappings, provider.GroupRoleMappings) {
		t.Fatalf("group mapping detail lost: claim=%q mappings=%#v", detail.GroupsClaim, detail.GroupRoleMappings)
	}

	body, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal detail response: %v", err)
	}
	req = httptest.NewRequest(http.MethodPut, "/api/security/sso/providers/corp-oidc", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	router.handleSSOProvider(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	loaded, err := persistence.LoadSSOConfig()
	if err != nil {
		t.Fatalf("load stored sso config: %v", err)
	}
	stored := loaded.GetProvider("corp-oidc")
	if stored == nil || stored.OIDC == nil {
		t.Fatalf("stored provider missing OIDC config: %#v", stored)
	}
	if stored.OIDC.ClientSecret != "super-secret" || !stored.OIDC.ClientSecretSet {
		t.Fatal("OIDC edit round-trip did not preserve existing client secret marker")
	}
	if stored.OIDC.RedirectURL != provider.OIDC.RedirectURL || stored.OIDC.LogoutURL != provider.OIDC.LogoutURL || !reflect.DeepEqual(stored.OIDC.Scopes, provider.OIDC.Scopes) {
		t.Fatalf("OIDC edit fields were not preserved: %#v", stored.OIDC)
	}
	if !reflect.DeepEqual(stored.AllowedGroups, provider.AllowedGroups) || !reflect.DeepEqual(stored.AllowedDomains, provider.AllowedDomains) || !reflect.DeepEqual(stored.AllowedEmails, provider.AllowedEmails) {
		t.Fatalf("SSO restrictions were not preserved: %#v", stored)
	}
	if stored.GroupsClaim != provider.GroupsClaim || !reflect.DeepEqual(stored.GroupRoleMappings, provider.GroupRoleMappings) {
		t.Fatalf("SSO role mappings were not preserved: %#v", stored)
	}
}

func TestContract_SSOTestRejectsCrossOriginSAMLMetadataRedirect(t *testing.T) {
	targetCalled := make(chan struct{}, 1)
	target := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case targetCalled <- struct{}{}:
		default:
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testSAMLMetadata))
	}))
	defer target.Close()

	origin := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	defer origin.Close()

	reqBody := SSOTestRequest{
		Type: "saml",
		SAML: &SAMLTestConfig{
			IDPMetadataURL: origin.URL + "/metadata",
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	select {
	case <-targetCalled:
		t.Fatal("expected cross-origin SAML redirect to be rejected before fetching the target origin")
	default:
	}
}

func TestContract_SSOTestRejectsCrossOriginOIDCDiscoveryRedirect(t *testing.T) {
	targetCalled := make(chan struct{}, 1)
	var targetURL string
	target := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		select {
		case targetCalled <- struct{}{}:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"issuer": %q,
			"authorization_endpoint": %q,
			"token_endpoint": %q,
			"userinfo_endpoint": %q,
			"jwks_uri": %q,
			"scopes_supported": ["openid", "profile", "email"]
		}`, targetURL, targetURL+"/auth", targetURL+"/token", targetURL+"/userinfo", targetURL+"/jwks")
	}))
	defer target.Close()
	targetURL = target.URL

	origin := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	defer origin.Close()

	reqBody := SSOTestRequest{
		Type: "oidc",
		OIDC: &OIDCTestConfig{
			IssuerURL: origin.URL,
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	setTestIP(req)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleTestSSOProvider(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	select {
	case <-targetCalled:
		t.Fatal("expected cross-origin OIDC redirect to be rejected before fetching the target origin")
	default:
	}
}

func TestContract_SSOLocalRedirectTargetsStayCanonical(t *testing.T) {
	if got := buildLocalRedirectTarget("https://evil.example.com/pwn", map[string]string{
		"oidc": "error",
	}); got != "/?oidc=error" {
		t.Fatalf("absolute redirect target = %q, want %q", got, "/?oidc=error")
	}

	if got := buildLocalRedirectTarget("/login?foo=bar#section", map[string]string{
		"saml": "success",
	}); got != "/login?foo=bar&saml=success#section" {
		t.Fatalf("local redirect target = %q, want %q", got, "/login?foo=bar&saml=success#section")
	}
}

func TestContract_SAMLLoginRejectsUnsupportedMethods(t *testing.T) {
	router := newSAMLRouter(t, testSAMLProvider("okta", true))
	req := httptest.NewRequest(http.MethodPut, "/api/saml/okta/login", nil)
	rec := httptest.NewRecorder()

	router.handleSAMLLogin(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
	}
}

func TestContract_FindingJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC)
	lastSeen := now.Add(5 * time.Minute)
	resolvedAt := now.Add(10 * time.Minute)
	ackAt := now.Add(11 * time.Minute)
	snoozedUntil := now.Add(12 * time.Minute)
	lastInvestigated := now.Add(15 * time.Minute)
	lastRegression := now.Add(30 * time.Minute)

	payload := ai.Finding{
		ID:                     "finding-1",
		Key:                    "cpu-high",
		Severity:               ai.FindingSeverityCritical,
		Category:               ai.FindingCategoryPerformance,
		ResourceID:             "vm-100",
		ResourceName:           "web-server",
		ResourceType:           "vm",
		Node:                   "pve-1",
		Title:                  "High CPU usage",
		Description:            "CPU sustained above 95%",
		Recommendation:         "Investigate processes and load",
		Evidence:               "cpu=96%",
		Source:                 "ai-analysis",
		DetectedAt:             now,
		LastSeenAt:             lastSeen,
		ResolvedAt:             &resolvedAt,
		AutoResolved:           true,
		ResolveReason:          "No longer detected",
		AcknowledgedAt:         &ackAt,
		SnoozedUntil:           &snoozedUntil,
		AlertIdentifier:        "alert-1",
		DismissedReason:        "expected_behavior",
		UserNote:               "Runs nightly batch",
		TimesRaised:            4,
		Suppressed:             true,
		InvestigationSessionID: "inv-session-1",
		InvestigationStatus:    "completed",
		InvestigationOutcome:   "fix_queued",
		LastInvestigatedAt:     &lastInvestigated,
		InvestigationAttempts:  2,
		InvestigationRecord: &aicontracts.InvestigationRecord{
			ID:        "investigation-1",
			FindingID: "finding-1",
			Subject: aicontracts.InvestigationRecordSubject{
				ResourceID:   "vm-100",
				ResourceName: "web-server",
				ResourceType: "vm",
				Node:         "pve-1",
			},
			Trigger: aicontracts.InvestigationRecordTrigger{
				FindingKey:  "cpu-high",
				Source:      "ai-analysis",
				Severity:    "critical",
				Category:    "performance",
				Title:       "High CPU usage",
				DetectedAt:  now,
				Description: "CPU sustained above 95%",
			},
			Status:            aicontracts.InvestigationStatusCompleted,
			Outcome:           aicontracts.OutcomeFixQueued,
			Confidence:        aicontracts.InvestigationRecordConfidenceMedium,
			Evidence:          []aicontracts.InvestigationRecordEvidence{{Kind: "finding_evidence", Summary: "cpu=96%"}},
			Conclusion:        "CPU sustained above 95%",
			RecommendedAction: "Investigate processes and load",
			Verification:      []string{},
			Rollback:          []string{},
			ToolsUsed:         []string{"ssh.exec"},
			StartedAt:         now,
			ApprovalID:        "approval-1",
		},
		LoopState: "remediation_planned",
		Lifecycle: []ai.FindingLifecycleEvent{
			{
				At:      now,
				Type:    "state_change",
				Message: "Moved to investigating",
				From:    "detected",
				To:      "investigating",
				Metadata: map[string]string{
					"from": "detected",
					"to":   "investigating",
				},
			},
		},
		RegressionCount:  1,
		LastRegressionAt: &lastRegression,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal finding: %v", err)
	}

	const want = `{
		"id":"finding-1",
		"key":"cpu-high",
		"severity":"critical",
		"category":"performance",
		"resource_id":"vm-100",
		"resource_name":"web-server",
		"resource_type":"vm",
		"node":"pve-1",
		"title":"High CPU usage",
		"description":"CPU sustained above 95%",
		"recommendation":"Investigate processes and load",
		"evidence":"cpu=96%",
		"source":"ai-analysis",
		"detected_at":"2026-02-08T13:14:15Z",
		"last_seen_at":"2026-02-08T13:19:15Z",
		"resolved_at":"2026-02-08T13:24:15Z",
		"auto_resolved":true,
		"resolve_reason":"No longer detected",
		"acknowledged_at":"2026-02-08T13:25:15Z",
		"snoozed_until":"2026-02-08T13:26:15Z",
		"alert_identifier":"alert-1",
		"dismissed_reason":"expected_behavior",
		"user_note":"Runs nightly batch",
		"times_raised":4,
		"suppressed":true,
		"investigation_session_id":"inv-session-1",
		"investigation_status":"completed",
		"investigation_outcome":"fix_queued",
		"last_investigated_at":"2026-02-08T13:29:15Z",
		"investigation_attempts":2,
		"investigation_record":{"id":"investigation-1","finding_id":"finding-1","subject":{"resource_id":"vm-100","resource_name":"web-server","resource_type":"vm","node":"pve-1"},"trigger":{"finding_key":"cpu-high","source":"ai-analysis","severity":"critical","category":"performance","title":"High CPU usage","detected_at":"2026-02-08T13:14:15Z","description":"CPU sustained above 95%"},"status":"completed","outcome":"fix_queued","confidence":"medium","evidence":[{"kind":"finding_evidence","summary":"cpu=96%"}],"conclusion":"CPU sustained above 95%","recommended_action":"Investigate processes and load","verification":[],"rollback":[],"tools_used":["ssh.exec"],"started_at":"2026-02-08T13:14:15Z","approval_id":"approval-1"},
		"loop_state":"remediation_planned",
		"lifecycle":[{"at":"2026-02-08T13:14:15Z","type":"state_change","message":"Moved to investigating","from":"detected","to":"investigating","metadata":{"from":"detected","to":"investigating"}}],
		"regression_count":1,
		"last_regression_at":"2026-02-08T13:44:15Z"
	}`

	assertJSONSnapshot(t, got, want)
}

// TestContract_PatrolStatusTrustJSONSnapshot pins the canonical JSON shape
// for the trust snapshot block carried on the patrol-status response. The
// PatrolStatusResponse.Trust field surfaces FindingsTrustSummary on the
// operator-facing Patrol page so trust signals (fix verified, dismissed as
// noise, etc.) are scannable without expanding individual findings.
func TestContract_PatrolStatusTrustJSONSnapshot(t *testing.T) {
	trust := ai.FindingsTrustSummary{
		Tracked:              4,
		CurrentlyActive:      1,
		Resolved:             2,
		AutoResolved:         1,
		FixVerified:          1,
		FixFailed:            0,
		DismissedAsNoise:     1,
		DismissedAsExpected:  0,
		DismissedAsLater:     0,
		Suppressed:           0,
		RegressedAtLeastOnce: 1,
	}
	got, err := json.Marshal(trust)
	if err != nil {
		t.Fatalf("marshal trust summary: %v", err)
	}
	want := `{"tracked":4,"currently_active":1,"resolved":2,"auto_resolved":1,"fix_verified":1,"fix_failed":0,"dismissed_as_noise":1,"dismissed_as_expected":0,"dismissed_as_later":0,"suppressed":0,"regressed_at_least_once":1}`
	if string(got) != want {
		t.Fatalf("trust summary JSON drift:\n got: %s\nwant: %s", got, want)
	}
}

// TestContract_UnifiedFindingPreviousResolvedFixSummaryJSONSnapshot pins the
// canonical JSON shape for the operational-memory field that preserves the
// prior successful fix summary across regressions. The Finding to
// UnifiedFinding conversion in the API router must emit this field under
// the canonical "previous_resolved_fix_summary" key so Assistant context
// and operator surfaces can reason about "what worked last time."
func TestContract_UnifiedFindingPreviousResolvedFixSummaryJSONSnapshot(t *testing.T) {
	finding := unified.UnifiedFinding{
		ID:                         "uf-regress",
		Source:                     unified.SourceAIPatrol,
		Severity:                   unified.SeverityWarning,
		Category:                   unified.CategoryReliability,
		ResourceID:                 "vm-100",
		Title:                      "Service stalled",
		Description:                "Service stopped responding again",
		PreviousResolvedFixSummary: "Restart the workload service after backup window clears",
		DetectedAt:                 time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC),
		LastSeenAt:                 time.Date(2026, 5, 8, 12, 5, 0, 0, time.UTC),
	}
	got, err := json.Marshal(finding)
	if err != nil {
		t.Fatalf("marshal unified finding: %v", err)
	}
	if !strings.Contains(string(got), `"previous_resolved_fix_summary":"Restart the workload service after backup window clears"`) {
		t.Fatalf("expected previous_resolved_fix_summary field in canonical UnifiedFinding payload, got %s", got)
	}
}

// TestContract_UnifiedFindingImpactJSONSnapshot pins the canonical JSON shape
// for UnifiedFinding payloads carrying the operator-facing Impact field. The
// AI engine populates Finding.Impact at detection time and the Finding to
// UnifiedFinding conversion in the API router must preserve that string under
// the canonical "impact" key so the operator-visible card shows
// consequence-if-ignored copy.
func TestContract_UnifiedFindingImpactJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC)
	finding := unified.UnifiedFinding{
		ID:             "uf-impact",
		Source:         unified.SourceAIPatrol,
		Severity:       unified.SeverityWarning,
		Category:       unified.CategoryReliability,
		ResourceID:     "patrol-runtime",
		ResourceName:   "Pulse Patrol Service",
		ResourceType:   "service",
		Title:          "Pulse Patrol: Provider connection issue",
		Description:    "Pulse Patrol could not maintain a healthy connection to the configured provider during analysis.",
		Impact:         "While Patrol cannot analyze, alerts continue to fire without evidence or recommended actions, and AI Intelligence summaries cannot refresh.",
		Recommendation: "Check provider reachability, base URL, firewall or proxy rules, and provider availability, then rerun Patrol.",
		DetectedAt:     now,
		LastSeenAt:     now,
	}
	got, err := json.Marshal(finding)
	if err != nil {
		t.Fatalf("marshal unified finding: %v", err)
	}
	if !strings.Contains(string(got), `"impact":"While Patrol cannot analyze`) {
		t.Fatalf("expected impact field in canonical UnifiedFinding payload, got %s", got)
	}
}

func TestContract_ApprovalJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC)
	expires := now.Add(5 * time.Minute)
	decided := now.Add(2 * time.Minute)

	payload := approval.ApprovalRequest{
		ID:          "approval-1",
		ExecutionID: "exec-1",
		ToolID:      "tool-1",
		Command:     "rm -rf /tmp/cache",
		TargetType:  "agent",
		TargetID:    "host-1",
		TargetName:  "alpha",
		Context:     "Cleanup temporary cache",
		RequestedBy: approval.RequesterPulsePatrol,
		RiskLevel:   approval.RiskHigh,
		Status:      approval.StatusApproved,
		RequestedAt: now,
		ExpiresAt:   expires,
		DecidedAt:   &decided,
		DecidedBy:   "admin",
		DenyReason:  "not needed",
		CommandHash: "sha256:abc",
		Consumed:    true,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal approval: %v", err)
	}

	const want = `{
		"id":"approval-1",
		"executionId":"exec-1",
		"toolId":"tool-1",
		"command":"rm -rf /tmp/cache",
			"targetType":"agent",
		"targetId":"host-1",
		"targetName":"alpha",
		"context":"Cleanup temporary cache",
		"requestedBy":"pulse_patrol",
		"riskLevel":"high",
		"status":"approved",
		"requestedAt":"2026-02-08T13:14:15Z",
		"expiresAt":"2026-02-08T13:19:15Z",
		"decidedAt":"2026-02-08T13:16:15Z",
		"decidedBy":"admin",
		"denyReason":"not needed",
		"commandHash":"sha256:abc",
		"consumed":true
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ApprovalListResponseJSONSnapshot(t *testing.T) {
	previousStore := approval.GetStore()
	approval.SetStore(nil)
	t.Cleanup(func() { approval.SetStore(previousStore) })

	handler := newTestAISettingsHandler(&config.Config{DataPath: t.TempDir()}, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/approvals", nil)
	rec := httptest.NewRecorder()
	handler.HandleListApprovals(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("approval list status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	const want = `{
		"approvals":[],
		"stats":{
			"approved":0,
			"denied":0,
			"executions":0,
			"expired":0,
			"pending":0
		}
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
}

func TestContract_HostedSignupResponseJSONSnapshot(t *testing.T) {
	payload := hostedSignupResponse{
		Message: hostedSignupAcceptedMessage,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal hosted signup response: %v", err)
	}
	if strings.Contains(string(got), "user_id") || strings.Contains(string(got), "org_id") {
		t.Fatalf("hosted signup accepted response must not expose identity fields: %s", got)
	}

	const want = `{
		"message":"If that email can finish signup, you'll receive a Pulse Account sign-in link shortly."
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_BillingStateQuickstartJSONSnapshot(t *testing.T) {
	grantedAt := time.Date(2026, 3, 25, 14, 30, 0, 0, time.UTC).Unix()

	payload := billingState{
		Capabilities:               []string{"ai_autofix", "ai_patrol"},
		Limits:                     map[string]int64{"max_monitored_systems": 25},
		MetersEnabled:              []string{},
		PlanVersion:                "cloud_starter",
		SubscriptionState:          subscriptionStateActiveValue,
		QuickstartCreditsGranted:   true,
		QuickstartCreditsUsed:      3,
		QuickstartCreditsGrantedAt: &grantedAt,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal billing state: %v", err)
	}

	const want = `{
		"capabilities":["ai_autofix","ai_patrol"],
		"limits":{"max_monitored_systems":25},
		"meters_enabled":[],
		"plan_version":"cloud_starter",
		"subscription_state":"active",
		"quickstart_credits_granted":true,
		"quickstart_credits_used":3,
		"quickstart_credits_granted_at":1774449000
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_HostedAISettingsDoesNotAutoBootstrapQuickstartJSONSnapshot(t *testing.T) {
	t.Setenv("PULSE_HOSTED_MODE", "true")

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	persistence, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("GetPersistence(default): %v", err)
	}

	seedHostedAIBillingState(t, mtp, "default")

	handler := NewAISettingsHandler(mtp, nil, nil)
	handler.defaultConfig = &config.Config{DataPath: baseDir}

	req := httptest.NewRequest(http.MethodGet, "/api/settings/ai", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if persistence.HasAIConfig() {
		t.Fatal("expected hosted AI settings contract not to persist implicit quickstart bootstrap")
	}

	const want = `{
		"enabled":false,
		"model":"",
		"configured":false,
		"custom_context":"",
		"auth_method":"api_key",
		"oauth_connected":false,
		"patrol_interval_minutes":360,
		"patrol_enabled":true,
		"patrol_auto_fix":false,
		"alert_triggered_analysis":true,
		"patrol_event_triggers_enabled":true,
		"patrol_alert_triggers_enabled":true,
		"patrol_anomaly_triggers_enabled":true,
		"patrol_alert_trigger_min_severity":"critical",
		"patrol_alert_trigger_types":[],
		"use_proactive_thresholds":false,
		"available_models":[],
		"anthropic_configured":false,
		"openai_configured":false,
		"openrouter_configured":false,
		"deepseek_configured":false,
		"gemini_configured":false,
		"zai_configured":false,
		"groq_configured":false,
		"mistral_configured":false,
		"cerebras_configured":false,
		"together_configured":false,
		"fireworks_configured":false,
		"ollama_configured":false,
		"ollama_base_url":"http://localhost:11434",
		"ollama_password_set":false,
		"ollama_keep_alive":"30s",
		"configured_providers":[],
		"control_level":"read_only",
		"protected_guests":[],
		"discovery_enabled":false,
		"patrol_readiness":{"status":"not_ready","ready":false,"cause":"service_unavailable","summary":"Pulse Patrol service is not available.","checks":[{"id":"service","status":"not_ready","cause":"service_unavailable","label":"Patrol service","message":"Pulse Patrol service is not available.","action":"restart_service"}]}
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want, "providers")
}

func TestContract_AISettingsRetiredQuickstartAliasJSONSnapshot(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "quickstart:minimax-2.5m"
	aiCfg.ChatModel = "quickstart:minimax-2.5m"
	aiCfg.PatrolModel = "quickstart:minimax-2.5m"
	aiCfg.DiscoveryModel = "quickstart:minimax-2.5m"
	aiCfg.AutoFixModel = "quickstart:minimax-2.5m"
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/ai", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	const want = `{
		"enabled":true,
		"model":"",
		"configured":false,
		"custom_context":"",
		"auth_method":"api_key",
		"oauth_connected":false,
		"patrol_interval_minutes":360,
		"patrol_enabled":true,
		"patrol_auto_fix":false,
		"alert_triggered_analysis":true,
		"patrol_event_triggers_enabled":true,
		"patrol_alert_triggers_enabled":true,
		"patrol_anomaly_triggers_enabled":true,
		"patrol_alert_trigger_min_severity":"critical",
		"patrol_alert_trigger_types":[],
		"use_proactive_thresholds":false,
		"available_models":[],
		"anthropic_configured":false,
		"openai_configured":false,
		"openrouter_configured":false,
		"deepseek_configured":false,
		"gemini_configured":false,
		"zai_configured":false,
		"groq_configured":false,
		"mistral_configured":false,
		"cerebras_configured":false,
		"together_configured":false,
		"fireworks_configured":false,
		"ollama_configured":false,
		"ollama_base_url":"http://localhost:11434",
		"ollama_password_set":false,
		"ollama_keep_alive":"30s",
		"configured_providers":[],
		"control_level":"read_only",
		"protected_guests":[],
		"discovery_enabled":false,
		"patrol_readiness":{"status":"not_ready","ready":false,"cause":"service_unavailable","summary":"Pulse Patrol service is not available.","checks":[{"id":"service","status":"not_ready","cause":"service_unavailable","label":"Patrol service","message":"Pulse Patrol service is not available.","action":"restart_service"}]}
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want, "providers")
	if bytes.Contains(bytes.ToLower(rec.Body.Bytes()), []byte("quickstart")) {
		t.Fatalf("expected AI settings payload to suppress legacy hosted quickstart aliases, got %s", rec.Body.Bytes())
	}
}

func TestContract_AISettingsOllamaAuthJSONSnapshot(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "ollama:llama3"
	aiCfg.PatrolModel = "ollama:llama3"
	aiCfg.OllamaBaseURL = "http://ollama.example:11434"
	aiCfg.OllamaUsername = "unai"
	aiCfg.OllamaPassword = "secret"
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/ai", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	const want = `{
		"enabled":true,
		"model":"ollama:llama3",
		"patrol_model":"ollama:llama3",
		"configured":true,
		"custom_context":"",
		"auth_method":"api_key",
		"oauth_connected":false,
		"patrol_interval_minutes":360,
		"patrol_enabled":true,
		"patrol_auto_fix":false,
		"alert_triggered_analysis":true,
		"patrol_event_triggers_enabled":true,
		"patrol_alert_triggers_enabled":true,
		"patrol_anomaly_triggers_enabled":true,
		"patrol_alert_trigger_min_severity":"critical",
		"patrol_alert_trigger_types":[],
		"use_proactive_thresholds":false,
		"available_models":[],
		"anthropic_configured":false,
		"openai_configured":false,
		"openrouter_configured":false,
		"deepseek_configured":false,
		"gemini_configured":false,
		"zai_configured":false,
		"groq_configured":false,
		"mistral_configured":false,
		"cerebras_configured":false,
		"together_configured":false,
		"fireworks_configured":false,
		"ollama_configured":true,
		"ollama_base_url":"http://ollama.example:11434",
		"ollama_username":"unai",
		"ollama_password_set":true,
		"ollama_keep_alive":"30s",
		"configured_providers":["ollama"],
		"control_level":"read_only",
		"protected_guests":[],
		"discovery_enabled":false,
		"patrol_readiness":{"status":"not_ready","ready":false,"cause":"service_unavailable","summary":"Pulse Patrol service is not available.","checks":[{"id":"service","status":"not_ready","cause":"service_unavailable","label":"Patrol service","message":"Pulse Patrol service is not available.","action":"restart_service"}]}
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want, "providers")
}

func TestContract_VMInventoryExportCSVHeaders(t *testing.T) {
	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/reports/inventory/vms/export?format=csv", nil)
	rec := httptest.NewRecorder()

	handler.HandleExportVMInventory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Fatalf("expected csv content type, got %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment; filename=\"vm-inventory-") {
		t.Fatalf("expected VM inventory attachment filename, got %q", got)
	}

	const want = "Resource ID,Instance,Node,Pool,VMID,VM Name,Status,CPU Cores,Memory Allocated Bytes,Disk Allocated Bytes,Disk Used Bytes,Disk Status Reason\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("unexpected VM inventory CSV header row:\nwant %q\ngot  %q", want, got)
	}
}

func TestContract_ReportingCatalogJSONSnapshot(t *testing.T) {
	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/reports/catalog", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetReportingCatalog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected json content type, got %q", got)
	}

	const want = `{
		"id":"advanced_reporting",
		"title":"Detailed Reporting",
		"description":"Generate performance reports and current-state exports across infrastructure and workloads.",
		"lockedState":{
			"title":"Advanced Reporting",
			"description":"AI-narrated performance reports for a single resource or your full fleet, with an executive summary, outlier callouts, and period-over-period changes. PDF and CSV formats, plus current-state VM inventory exports. AI narration uses Pulse Assistant when configured; reports fall back to a deterministic summary otherwise. Available on paid self-hosted and hosted plans."
		},
		"guidance":{
			"title":"Advanced Insights",
			"description":"Performance reports include an AI-narrated executive summary, fleet outliers, and period-over-period comparison. They draw from the historical metrics store and Patrol findings within the window. The VM inventory export captures current runtime state for spreadsheet-friendly fleet reviews. Use reports for retrospective trends and the inventory export for current allocation snapshots."
		},
		"performanceReport":{
			"id":"performance_reports",
			"title":"Performance Reports",
			"description":"Generate PDF summaries or CSV metric exports from historical monitoring data for one or more selected resources.",
			"singleResourceEndpoint":"/api/admin/reports/generate",
			"multiResourceEndpoint":"/api/admin/reports/generate-multi",
			"singleFilenamePrefix":"report",
			"singleFilenameSubject":"resource_id",
			"multiFilenamePrefix":"fleet-report",
			"filenameDateStyle":"utc_yyyymmdd",
			"formats":[
				{
					"value":"pdf",
					"label":"PDF Report"
				},
				{
					"value":"csv",
					"label":"CSV Data"
				}
			],
			"defaultFormat":"pdf",
			"ranges":[
				{
					"key":"24h",
					"label":"Last 24 Hours",
					"description":"Current-day operational summary for short-term regressions.",
					"windowHours":24
				},
				{
					"key":"7d",
					"label":"Last 7 Days",
					"description":"Weekly trend window for recent performance changes.",
					"windowHours":168
				},
				{
					"key":"30d",
					"label":"Last 30 Days",
					"description":"Monthly review window for sustained capacity or reliability shifts.",
					"windowHours":720
				}
			],
			"defaultRange":"24h",
			"multiResourceMax":50,
			"supportsMetricFilter":true,
			"supportsCustomTitle":true
		},
		"vmInventoryExport":{
			"id":"vm_inventory",
			"title":"VM Inventory Export",
			"description":"Export the current fleet-wide VM inventory as CSV using the canonical runtime model. Includes VM identity, placement, CPU, memory allocation, disk allocation, and disk usage columns.",
			"format":"csv",
			"exportEndpoint":"/api/admin/reports/inventory/vms/export",
			"filenamePrefix":"vm-inventory",
			"filenameDateStyle":"utc_yyyymmdd",
			"columns":[
				{
					"key":"resource_id",
					"label":"Resource ID",
					"description":"Canonical Pulse resource ID for the VM."
				},
				{
					"key":"instance",
					"label":"Instance",
					"description":"Configured Proxmox instance or cluster name."
				},
				{
					"key":"node",
					"label":"Node",
					"description":"Proxmox node currently hosting the VM."
				},
				{
					"key":"pool",
					"label":"Pool",
					"description":"Canonical Proxmox pool membership when the platform reports one."
				},
				{
					"key":"vmid",
					"label":"VMID",
					"description":"Numeric Proxmox VM identifier."
				},
				{
					"key":"vm_name",
					"label":"VM Name",
					"description":"Current VM display name from the runtime model."
				},
				{
					"key":"status",
					"label":"Status",
					"description":"Canonical runtime status for the VM."
				},
				{
					"key":"cpu_cores",
					"label":"CPU Cores",
					"description":"Allocated virtual CPU core count."
				},
				{
					"key":"memory_allocated_bytes",
					"label":"Memory Allocated Bytes",
					"description":"Configured memory allocation in bytes."
				},
				{
					"key":"disk_allocated_bytes",
					"label":"Disk Allocated Bytes",
					"description":"Total allocated disk capacity in bytes across the VM."
				},
				{
					"key":"disk_used_bytes",
					"label":"Disk Used Bytes",
					"description":"Current used disk bytes from the canonical runtime disk view."
				},
				{
					"key":"disk_status_reason",
					"label":"Disk Status Reason",
					"description":"Reason disk usage is partial or unavailable when the runtime cannot provide a full guest view."
				}
			]
		}
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
}

func TestContract_PerformanceReportTransportUsesCatalogDefinition(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reporting?resourceType=node&resourceId=node-1&metricType=+cpu+&title=+Node+report+",
		nil,
	)
	rec := httptest.NewRecorder()

	handler.HandleGenerateReport(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	definition := reporting.DescribePerformanceReport()
	if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, fmt.Sprintf("attachment; filename=\"%s-node-1-", definition.SingleFilenamePrefix)) {
		t.Fatalf("expected canonical performance-report attachment filename, got %q", got)
	}
	if engine.lastReq.Format != definition.DefaultFormat {
		t.Fatalf("expected default format %q, got %q", definition.DefaultFormat, engine.lastReq.Format)
	}
	if engine.lastReq.MetricType != "cpu" || engine.lastReq.Title != "Node report" {
		t.Fatalf("expected trimmed canonical optional fields, got %+v", engine.lastReq)
	}
}

func TestContract_ReportingRequestCarriesEntitledReportBranding(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	service := pkglicensing.NewService()
	service.SetCurrentForTesting(&pkglicensing.License{
		Claims: pkglicensing.Claims{
			LicenseID: "lic_contract_report_branding",
			Email:     "brand@example.test",
			Tier:      pkglicensing.TierEnterprise,
		},
		ValidatedAt: time.Now(),
	})
	SetLicenseServiceProvider(reportBrandLicenseProvider{service: service})
	t.Cleanup(func() { SetLicenseServiceProvider(nil) })

	t.Setenv("PULSE_REPORT_PROVIDER_BRAND_DISPLAY_NAME", "Provider Default")
	handler := NewReportingHandlers(nil, nil)
	handler.SetSystemSettingsStore(reportBrandSettingsStore{settings: &config.SystemSettings{
		ReportBranding: &config.ReportBrandSettings{DisplayName: "Client One"},
	}})

	req := httptest.NewRequest(http.MethodGet, "/api/reporting?resourceType=node&resourceId=node-1", nil)
	rec := httptest.NewRecorder()
	handler.HandleGenerateReport(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !engine.lastReq.Branding.Entitled {
		t.Fatal("expected reporting request to carry white_label entitlement state")
	}
	if got := engine.lastReq.Branding.ProviderDefault.DisplayName; got != "Provider Default" {
		t.Fatalf("provider brand = %q, want Provider Default", got)
	}
	if got := engine.lastReq.Branding.WorkspaceOverride.DisplayName; got != "Client One" {
		t.Fatalf("workspace brand = %q, want Client One", got)
	}
	if got := engine.lastReq.Branding.EffectiveBrand(); got == nil || got.DisplayName != "Client One" {
		t.Fatalf("effective brand should prefer workspace override, got %+v", got)
	}
}

func TestContract_ReportBrandingSettingsRejectWorkspaceLogoPath(t *testing.T) {
	err := validateReportBrandingSettings(map[string]interface{}{
		"displayName": "Client One",
		"logoPath":    "/etc/pulse/secrets/handoff.key",
	})
	if err == nil || !strings.Contains(err.Error(), "reportBranding.logoPath is not supported") {
		t.Fatalf("expected workspace logoPath to be rejected, got %v", err)
	}

	err = validateReportBrandingSettings(map[string]interface{}{
		"displayName": "Client One",
		"logoBase64":  "iVBORw0KGgo=",
		"logoFormat":  "png",
	})
	if err != nil {
		t.Fatalf("expected inline report logo settings to remain valid, got %v", err)
	}
}

func TestContract_ReportingCatalogRouteAccessibleWithoutReportingFeature(t *testing.T) {
	rawToken := "reporting-catalog-contract-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/reports/catalog", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for ungated reporting catalog route, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("expected json content type, got %q", got)
	}
}

func TestContract_LocalCommercialAnalyticsRoutesAreNotRegistered(t *testing.T) {
	rawToken := "retired-commercial-analytics-contract-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/upgrade-metrics/events"},
		{method: http.MethodGet, path: "/api/upgrade-metrics/stats"},
		{method: http.MethodGet, path: "/api/upgrade-metrics/health"},
		{method: http.MethodGet, path: "/api/upgrade-metrics/config"},
		{method: http.MethodPut, path: "/api/upgrade-metrics/config"},
		{method: http.MethodGet, path: "/api/admin/upgrade-metrics-funnel"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-API-Token", rawToken)
			rec := httptest.NewRecorder()

			router.Handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("%s %s status=%d, want %d body=%s", tc.method, tc.path, rec.Code, http.StatusNotFound, rec.Body.String())
			}
		})
	}
}

func TestContract_PerformanceReportTransportUsesCatalogDefaultRange(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	end := time.Date(2026, 3, 25, 15, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reporting?resourceType=node&resourceId=node-1&end="+url.QueryEscape(end.Format(time.RFC3339)),
		nil,
	)
	rec := httptest.NewRecorder()

	handler.HandleGenerateReport(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	definition := reporting.DescribePerformanceReport()
	if got := engine.lastReq.Start; !got.Equal(end.Add(-definition.DefaultRangeDuration())) {
		t.Fatalf("expected canonical default start time, got %s", got)
	}
	if !engine.lastReq.End.Equal(end) {
		t.Fatalf("expected canonical end time, got %s", engine.lastReq.End)
	}
}

func TestContract_PerformanceReportTransportRejectsInvalidTimeRange(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reporting?resourceType=node&resourceId=node-1&start=2026-03-25T12:00:00Z&end=2026-03-25T11:00:00Z",
		nil,
	)
	rec := httptest.NewRecorder()

	handler.HandleGenerateReport(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode invalid-time-range response: %v", err)
	}
	if payload.Code != "invalid_time_range" {
		t.Fatalf("expected invalid_time_range code, got %q", payload.Code)
	}
	if payload.Error != "end must be after start" {
		t.Fatalf("expected canonical invalid_time_range message, got %q", payload.Error)
	}
}

func TestContract_PerformanceReportTransportRejectsOversizedMultiBody(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	padding := strings.Repeat("x", reportingMultiReportBodyMax)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/reporting/generate-multi",
		strings.NewReader(fmt.Sprintf(`{"resources":[{"resourceType":"vm","resourceId":"vm-1"}],"format":"pdf","title":"%s"}`, padding)),
	)
	rec := httptest.NewRecorder()

	handler.HandleGenerateMultiReport(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode oversized-body response: %v", err)
	}
	if payload.Code != "body_too_large" {
		t.Fatalf("expected body_too_large code, got %q", payload.Code)
	}
	if payload.Error != "Request body must be 1MB or less" {
		t.Fatalf("expected canonical oversized-body message, got %q", payload.Error)
	}
}

func TestContract_PerformanceReportTransportRejectsInvalidOptionalFieldWithStableCode(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reporting?resourceType=node&resourceId=node-1&metricType="+url.QueryEscape("cpu usage"),
		nil,
	)
	rec := httptest.NewRecorder()

	handler.HandleGenerateReport(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode invalid optional-field response: %v", err)
	}
	if payload.Code != "invalid_metric_type" {
		t.Fatalf("expected invalid_metric_type code, got %q", payload.Code)
	}
	if payload.Error != "metricType must match [a-zA-Z0-9._:-]+ and be <= 64 chars" {
		t.Fatalf("expected canonical invalid_metric_type message, got %q", payload.Error)
	}
}

func TestContract_ReportingInvalidFormatErrorsUseCatalogDefinitions(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handler := NewReportingHandlers(nil, nil)

	reportReq := httptest.NewRequest(
		http.MethodGet,
		"/api/reporting?format=xlsx&resourceType=node&resourceId=node-1",
		nil,
	)
	reportRec := httptest.NewRecorder()
	handler.HandleGenerateReport(reportRec, reportReq)
	if reportRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid performance report format, got %d body=%s", reportRec.Code, reportRec.Body.String())
	}
	var reportPayload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(reportRec.Body).Decode(&reportPayload); err != nil {
		t.Fatalf("decode performance invalid-format response: %v", err)
	}
	if reportPayload.Error != reporting.DescribePerformanceReport().InvalidFormatError() {
		t.Fatalf("expected canonical performance invalid-format error, got %q", reportPayload.Error)
	}

	inventoryReq := httptest.NewRequest(
		http.MethodGet,
		"/api/admin/reports/inventory/vms/export?format=pdf",
		nil,
	)
	inventoryRec := httptest.NewRecorder()
	handler.HandleExportVMInventory(inventoryRec, inventoryReq)
	if inventoryRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid inventory format, got %d body=%s", inventoryRec.Code, inventoryRec.Body.String())
	}
	var inventoryPayload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(inventoryRec.Body).Decode(&inventoryPayload); err != nil {
		t.Fatalf("decode inventory invalid-format response: %v", err)
	}
	if inventoryPayload.Error != reporting.DescribeVMInventoryExport().InvalidFormatError() {
		t.Fatalf("expected canonical inventory invalid-format error, got %q", inventoryPayload.Error)
	}
}

func TestContract_HostedTenantAISettingsDoesNotAutoBootstrapQuickstartJSONSnapshot(t *testing.T) {
	t.Setenv("PULSE_HOSTED_MODE", "true")

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	persistence, err := mtp.GetPersistence("t-tenant")
	if err != nil {
		t.Fatalf("GetPersistence(t-tenant): %v", err)
	}

	seedHostedAIBillingState(t, mtp, "default")

	handler := NewAISettingsHandler(mtp, nil, nil)
	handler.defaultConfig = &config.Config{DataPath: baseDir}

	req := httptest.NewRequest(http.MethodGet, "/api/settings/ai", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "t-tenant"))
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if persistence.HasAIConfig() {
		t.Fatal("expected hosted tenant AI settings contract not to persist implicit quickstart bootstrap")
	}

	const want = `{
		"enabled":false,
		"model":"",
		"configured":false,
		"custom_context":"",
		"auth_method":"api_key",
		"oauth_connected":false,
		"patrol_interval_minutes":360,
		"patrol_enabled":true,
		"patrol_auto_fix":false,
		"alert_triggered_analysis":true,
		"patrol_event_triggers_enabled":true,
		"patrol_alert_triggers_enabled":true,
		"patrol_anomaly_triggers_enabled":true,
		"patrol_alert_trigger_min_severity":"critical",
		"patrol_alert_trigger_types":[],
		"use_proactive_thresholds":false,
		"available_models":[],
		"anthropic_configured":false,
		"openai_configured":false,
		"openrouter_configured":false,
		"deepseek_configured":false,
		"gemini_configured":false,
		"zai_configured":false,
		"groq_configured":false,
		"mistral_configured":false,
		"cerebras_configured":false,
		"together_configured":false,
		"fireworks_configured":false,
		"ollama_configured":false,
		"ollama_base_url":"http://localhost:11434",
		"ollama_password_set":false,
		"ollama_keep_alive":"30s",
		"configured_providers":[],
		"control_level":"read_only",
		"protected_guests":[],
		"discovery_enabled":false,
		"patrol_readiness":{"status":"not_ready","ready":false,"cause":"service_unavailable","summary":"Pulse Patrol service is not available.","checks":[{"id":"service","status":"not_ready","cause":"service_unavailable","label":"Patrol service","message":"Pulse Patrol service is not available.","action":"restart_service"}]}
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want, "providers")
}

func TestContract_StripeWebhookHandlersUseCanonicalRuntimeDataDir(t *testing.T) {
	envDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", envDir)

	persistence := config.NewMultiTenantPersistence(envDir)
	billingStore := config.NewFileBillingStore(envDir)
	rbacProvider := NewTenantRBACProvider(envDir)

	withExplicitDir := NewStripeWebhookHandlers(billingStore, persistence, rbacProvider, nil, nil, true, envDir)
	if got := filepath.Dir(withExplicitDir.deduper.dir); got != filepath.Join(envDir, "stripe") {
		t.Fatalf("explicit dedupe dir root = %q, want %q", got, filepath.Join(envDir, "stripe"))
	}
	if got := filepath.Dir(withExplicitDir.index.dir); got != filepath.Join(envDir, "stripe") {
		t.Fatalf("explicit customer index dir root = %q, want %q", got, filepath.Join(envDir, "stripe"))
	}

	withEnvFallback := NewStripeWebhookHandlers(billingStore, persistence, rbacProvider, nil, nil, true, "")
	if got := filepath.Dir(withEnvFallback.deduper.dir); got != filepath.Join(envDir, "stripe") {
		t.Fatalf("env fallback dedupe dir root = %q, want %q", got, filepath.Join(envDir, "stripe"))
	}
	if got := filepath.Dir(withEnvFallback.index.dir); got != filepath.Join(envDir, "stripe") {
		t.Fatalf("env fallback customer index dir root = %q, want %q", got, filepath.Join(envDir, "stripe"))
	}
}

func TestContract_NotificationWebhookTestResponseJSONSnapshot(t *testing.T) {
	payload := map[string]interface{}{
		"success":  true,
		"status":   200,
		"response": "OK",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal webhook test response: %v", err)
	}

	const want = `{
		"response":"OK",
		"status":200,
		"success":true
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_NotificationPushoverWebhookResponseJSONSnapshot(t *testing.T) {
	payload := map[string]interface{}{
		"id":      "hook-1",
		"name":    "Pushover",
		"url":     "https://api.pushover.net/1/messages.json",
		"service": "pushover",
		"customFields": map[string]string{
			"token": "app-token",
			"user":  "user-key",
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal pushover webhook response: %v", err)
	}

	const want = `{
		"customFields":{"token":"app-token","user":"user-key"},
		"id":"hook-1",
		"name":"Pushover",
		"service":"pushover",
		"url":"https://api.pushover.net/1/messages.json"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceIntelligenceIncludesRecentChanges(t *testing.T) {
	svc := newEnabledAIService(t)
	canonicalStore := unifiedresources.NewMemoryStore()
	observedAt := time.Now().Add(-15 * time.Minute)
	if err := canonicalStore.RecordChange(unifiedresources.ResourceChange{
		ID:         "change-contract",
		ObservedAt: observedAt,
		ResourceID: "vm-100",
		Kind:       unifiedresources.ChangeRestart,
		SourceType: unifiedresources.SourcePlatformEvent,
		Reason:     "guest restarted",
	}); err != nil {
		t.Fatalf("record canonical change: %v", err)
	}
	correlationDetector := correlation.NewDetector(correlation.Config{
		MinOccurrences:    1,
		CorrelationWindow: 2 * time.Hour,
		RetentionWindow:   24 * time.Hour,
	})
	correlationBase := observedAt.Add(-10 * time.Minute)
	correlationDetector.RecordEvent(correlation.Event{
		ResourceID:   "storage-1",
		ResourceName: "storage-1",
		ResourceType: "storage",
		EventType:    correlation.EventDiskFull,
		Timestamp:    correlationBase,
	})
	correlationDetector.RecordEvent(correlation.Event{
		ResourceID:   "vm-100",
		ResourceName: "vm-100",
		ResourceType: "vm",
		EventType:    correlation.EventRestart,
		Timestamp:    correlationBase.Add(1 * time.Minute),
	})
	correlationDetector.RecordEvent(correlation.Event{
		ResourceID:   "storage-1",
		ResourceName: "storage-1",
		ResourceType: "storage",
		EventType:    correlation.EventDiskFull,
		Timestamp:    correlationBase.Add(2 * time.Minute),
	})
	correlationDetector.RecordEvent(correlation.Event{
		ResourceID:   "vm-100",
		ResourceName: "vm-100",
		ResourceType: "vm",
		EventType:    correlation.EventRestart,
		Timestamp:    correlationBase.Add(3 * time.Minute),
	})
	svc.SetUnifiedResourceProvider(&stubUnifiedResourceProvider{
		resources: []unifiedresources.Resource{
			{ID: "public-1", Type: unifiedresources.ResourceTypeVM, Tags: []string{"public"}},
			{ID: "internal-1", Type: unifiedresources.ResourceTypeAgent, Agent: &unifiedresources.AgentData{Hostname: "agent-1"}},
		},
	})
	setUnexportedField(t, svc.GetPatrolService(), "correlationDetector", correlationDetector)
	setUnexportedField(t, svc, "resourceExportStore", canonicalStore)
	setUnexportedField(t, svc.GetPatrolService(), "aiService", svc)

	handlers := &AISettingsHandler{defaultAIService: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence?resource_id=vm-100", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetIntelligence(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	recentChanges, ok := payload["recent_changes"].([]interface{})
	if !ok {
		t.Fatalf("expected recent_changes array in response, got %T", payload["recent_changes"])
	}
	if len(recentChanges) != 1 {
		t.Fatalf("expected 1 recent change, got %d", len(recentChanges))
	}
	if _, ok := payload["policy_posture"]; ok {
		t.Fatal("did not expect policy_posture in resource intelligence response")
	}
	dependencies, ok := payload["dependencies"].([]interface{})
	if !ok {
		t.Fatalf("expected dependencies array in response, got %T", payload["dependencies"])
	}
	if len(dependencies) == 0 {
		t.Fatal("expected at least one dependency in response")
	}
	correlations, ok := payload["correlations"].([]interface{})
	if !ok {
		t.Fatalf("expected correlations array in response, got %T", payload["correlations"])
	}
	if len(correlations) == 0 {
		t.Fatal("expected at least one correlation in response")
	}
	firstCorrelation, ok := correlations[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected correlation object, got %T", correlations[0])
	}
	if firstCorrelation["event_pattern"] == "" {
		t.Fatal("expected correlation event_pattern in response")
	}
	if firstCorrelation["avg_delay"] == nil {
		t.Fatal("expected correlation avg_delay in response")
	}
	if firstCorrelation["confidence"] == nil {
		t.Fatal("expected correlation confidence in response")
	}
}

func TestContract_IntelligenceSummaryIncludesRecentChanges(t *testing.T) {
	svc := newEnabledAIService(t)
	canonicalStore := unifiedresources.NewMemoryStore()
	if err := canonicalStore.RecordChange(unifiedresources.ResourceChange{
		ID:         "change-summary",
		ObservedAt: time.Now().Add(-15 * time.Minute),
		ResourceID: "vm-100",
		Kind:       unifiedresources.ChangeRestart,
		SourceType: unifiedresources.SourcePlatformEvent,
		Reason:     "guest restarted",
	}); err != nil {
		t.Fatalf("record canonical change: %v", err)
	}
	svc.SetUnifiedResourceProvider(&stubUnifiedResourceProvider{
		resources: []unifiedresources.Resource{
			{
				ID:   "public-1",
				Name: "public-vm",
				Type: unifiedresources.ResourceTypeVM,
				Tags: []string{"public"},
			},
			{
				ID:   "restricted-1",
				Name: "mail-gw",
				Type: unifiedresources.ResourceTypePMG,
				PMG:  &unifiedresources.PMGData{Hostname: "mail.internal"},
			},
		},
	})
	setUnexportedField(t, svc, "resourceExportStore", canonicalStore)
	setUnexportedField(t, svc.GetPatrolService(), "aiService", svc)

	handlers := &AISettingsHandler{defaultAIService: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetIntelligence(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	recentChanges, ok := payload["recent_changes"].([]interface{})
	if !ok {
		t.Fatalf("expected recent_changes array in response, got %T", payload["recent_changes"])
	}
	if len(recentChanges) != 1 {
		t.Fatalf("expected 1 recent change, got %d", len(recentChanges))
	}
	policyPosture, ok := payload["policy_posture"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected policy_posture object in response, got %T", payload["policy_posture"])
	}
	if got := int(policyPosture["total_resources"].(float64)); got != 2 {
		t.Fatalf("expected total_resources=2, got %d", got)
	}
}

func TestContract_RecentChangesEndpointUsesCanonicalTimeline(t *testing.T) {
	svc := newEnabledAIService(t)
	canonicalStore := unifiedresources.NewMemoryStore()
	if err := canonicalStore.RecordChange(unifiedresources.ResourceChange{
		ID:            "change-canonical",
		ObservedAt:    time.Now().Add(-25 * time.Minute),
		ResourceID:    "vm-canonical",
		Kind:          unifiedresources.ChangeRestart,
		From:          "running",
		To:            "restarting",
		SourceType:    unifiedresources.SourcePlatformEvent,
		SourceAdapter: unifiedresources.AdapterProxmox,
		Reason:        "guest restarted after maintenance",
	}); err != nil {
		t.Fatalf("record canonical change: %v", err)
	}
	svc.SetUnifiedResourceProvider(&stubUnifiedResourceProvider{
		resources: []unifiedresources.Resource{
			{
				ID:   "vm-canonical",
				Name: "canonical-vm",
				Type: unifiedresources.ResourceTypeVM,
			},
		},
	})
	setUnexportedField(t, svc, "resourceExportStore", canonicalStore)
	setUnexportedField(t, svc.GetPatrolService(), "aiService", svc)

	handlers := &AISettingsHandler{defaultAIService: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/changes?hours=1", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetRecentChanges(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	changes, ok := payload["changes"].([]interface{})
	if !ok {
		t.Fatalf("expected changes array in response, got %T", payload["changes"])
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 recent change, got %d", len(changes))
	}
	change, ok := changes[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected object change, got %#v", changes[0])
	}
	if change["resource_name"] != "canonical-vm" {
		t.Fatalf("expected canonical resource name, got %#v", change["resource_name"])
	}
	if change["resource_type"] != string(unifiedresources.ResourceTypeVM) {
		t.Fatalf("expected resource type vm, got %#v", change["resource_type"])
	}
	if change["change_type"] != string(unifiedresources.ChangeRestart) {
		t.Fatalf("expected canonical change type, got %#v", change["change_type"])
	}
	if desc, ok := change["description"].(string); !ok || !strings.Contains(desc, "Restart") {
		t.Fatalf("expected canonical change description, got %#v", change["description"])
	}
}

func TestContract_AIIntelligenceCorrelationsJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 17, 30, 0, 0, time.UTC)
	payload := map[string]any{
		"correlations": []map[string]any{
			{
				"source_id":     "node-1",
				"source_name":   "node-1",
				"source_type":   "node",
				"target_id":     "vm-1",
				"target_name":   "vm-1",
				"target_type":   "vm",
				"event_pattern": "high_cpu -> restart",
				"occurrences":   1,
				"avg_delay":     "1m0s",
				"confidence":    0.1,
				"last_seen":     now,
				"description":   "When node-1 experiences high_cpu, vm-1 often follows within 1m0s",
			},
		},
		"count": 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal correlations response: %v", err)
	}

	const want = `{
		"correlations":[
			{
				"avg_delay":"1m0s",
				"confidence":0.1,
				"description":"When node-1 experiences high_cpu, vm-1 often follows within 1m0s",
				"event_pattern":"high_cpu -\u003e restart",
				"last_seen":"2026-03-18T17:30:00Z",
				"occurrences":1,
				"source_id":"node-1",
				"source_name":"node-1",
				"source_type":"node",
				"target_id":"vm-1",
				"target_name":"vm-1",
				"target_type":"vm"
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_MonitoredSystemLedgerJSONSnapshot(t *testing.T) {
	payload := MonitoredSystemLedgerResponse{
		Systems: []MonitoredSystemLedgerEntry{
			{
				Name:   "Tower",
				Type:   "host",
				Status: "warning",
				StatusExplanation: MonitoredSystemLedgerStatusExplanation{
					Summary: "At least one included source is stale, so Pulse marks this monitored system as warning.",
					Reasons: []MonitoredSystemLedgerStatusReason{
						{
							Kind:       "source-stale",
							Name:       "Tower",
							Type:       "host",
							Source:     "agent",
							Status:     "stale",
							ReportedAt: "2026-03-18T17:25:00Z",
							Summary:    "Agent data for Tower is stale (last reported 2026-03-18T17:25:00Z).",
						},
					},
				},
				LatestIncludedSignal: MonitoredSystemLedgerLatestSignal{
					Name:   "Tower",
					Type:   "host",
					Source: "agent",
					At:     "2026-03-18T17:30:00Z",
				},
				Source: "agent",
				Explanation: MonitoredSystemLedgerExplanation{
					Summary: "Counts as one monitored system because Pulse sees one top-level host view from agent.",
					Reasons: []MonitoredSystemLedgerExplanationReason{
						{
							Kind:    "standalone",
							Signal:  "single-top-level-view",
							Summary: "No overlapping top-level source matched this system.",
						},
					},
					Surfaces: []MonitoredSystemLedgerExplanationSurface{
						{
							Name:   "Tower",
							Type:   "host",
							Source: "agent",
						},
					},
				},
			},
		},
		Total: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal monitored system ledger response: %v", err)
	}

	const want = `{
		"systems":[
			{
				"name":"Tower",
				"type":"host",
				"status":"warning",
				"status_explanation":{
					"summary":"At least one included source is stale, so Pulse marks this monitored system as warning.",
					"reasons":[
						{
							"kind":"source-stale",
							"name":"Tower",
							"type":"host",
							"source":"agent",
							"status":"stale",
							"reported_at":"2026-03-18T17:25:00Z",
							"summary":"Agent data for Tower is stale (last reported 2026-03-18T17:25:00Z)."
						}
					]
				},
				"latest_included_signal":{
					"name":"Tower",
					"type":"host",
					"source":"agent",
					"at":"2026-03-18T17:30:00Z"
				},
				"source":"agent",
				"explanation":{
					"summary":"Counts as one monitored system because Pulse sees one top-level host view from agent.",
					"reasons":[
						{
							"kind":"standalone",
							"signal":"single-top-level-view",
							"summary":"No overlapping top-level source matched this system."
						}
					],
					"surfaces":[
						{
							"name":"Tower",
							"type":"host",
							"source":"agent"
						}
					]
				}
			}
		],
			"total":1
		}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_MonitoredSystemLedgerPreviewJSONSnapshot(t *testing.T) {
	payload := MonitoredSystemLedgerPreviewResponse{
		CurrentCount:    1,
		ProjectedCount:  1,
		AdditionalCount: 0,
		Effect:          "attaches_existing",
		CurrentSystems: []MonitoredSystemLedgerEntry{
			{
				Name:   "Tower",
				Type:   "host",
				Status: "online",
				StatusExplanation: MonitoredSystemLedgerStatusExplanation{
					Summary: "All included top-level collection paths currently report online status.",
					Reasons: []MonitoredSystemLedgerStatusReason{},
				},
				LatestIncludedSignal: MonitoredSystemLedgerLatestSignal{
					Name:   "Tower",
					Type:   "host",
					Source: "agent",
					At:     "2026-03-18T17:30:00Z",
				},
				Source: "agent",
				Explanation: MonitoredSystemLedgerExplanation{
					Summary: "Counts as one monitored system because Pulse sees one top-level host view from agent.",
					Reasons: []MonitoredSystemLedgerExplanationReason{
						{
							Kind:    "standalone",
							Signal:  "single-top-level-view",
							Summary: "No overlapping top-level source matched this system.",
						},
					},
					Surfaces: []MonitoredSystemLedgerExplanationSurface{
						{Name: "Tower", Type: "host", Source: "agent"},
					},
				},
			},
		},
		ProjectedSystems: []MonitoredSystemLedgerEntry{
			{
				Name:   "tower",
				Type:   "proxmox-node",
				Status: "online",
				StatusExplanation: MonitoredSystemLedgerStatusExplanation{
					Summary: "All included top-level collection paths currently report online status.",
					Reasons: []MonitoredSystemLedgerStatusReason{},
				},
				LatestIncludedSignal: MonitoredSystemLedgerLatestSignal{
					Name:   "tower",
					Type:   "proxmox-node",
					Source: "proxmox",
					At:     "2026-03-18T17:35:00Z",
				},
				Source: "multiple",
				Explanation: MonitoredSystemLedgerExplanation{
					Summary: "Counts as one monitored system because Pulse merged 2 top-level views into one canonical system using shared machine identity.",
					Reasons: []MonitoredSystemLedgerExplanationReason{
						{
							Kind:    "grouped",
							Signal:  "identity-match",
							Summary: "Pulse matched these top-level views by shared canonical machine identity.",
						},
					},
					Surfaces: []MonitoredSystemLedgerExplanationSurface{
						{Name: "Tower", Type: "host", Source: "agent"},
						{Name: "tower", Type: "proxmox-node", Source: "proxmox"},
					},
				},
			},
		},
		CurrentSystem: &MonitoredSystemLedgerEntry{
			Name:   "Tower",
			Type:   "host",
			Status: "online",
			StatusExplanation: MonitoredSystemLedgerStatusExplanation{
				Summary: "All included top-level collection paths currently report online status.",
				Reasons: []MonitoredSystemLedgerStatusReason{},
			},
			LatestIncludedSignal: MonitoredSystemLedgerLatestSignal{
				Name:   "Tower",
				Type:   "host",
				Source: "agent",
				At:     "2026-03-18T17:30:00Z",
			},
			Source: "agent",
			Explanation: MonitoredSystemLedgerExplanation{
				Summary: "Counts as one monitored system because Pulse sees one top-level host view from agent.",
				Reasons: []MonitoredSystemLedgerExplanationReason{
					{
						Kind:    "standalone",
						Signal:  "single-top-level-view",
						Summary: "No overlapping top-level source matched this system.",
					},
				},
				Surfaces: []MonitoredSystemLedgerExplanationSurface{
					{Name: "Tower", Type: "host", Source: "agent"},
				},
			},
		},
		ProjectedSystem: &MonitoredSystemLedgerEntry{
			Name:   "tower",
			Type:   "proxmox-node",
			Status: "online",
			StatusExplanation: MonitoredSystemLedgerStatusExplanation{
				Summary: "All included top-level collection paths currently report online status.",
				Reasons: []MonitoredSystemLedgerStatusReason{},
			},
			LatestIncludedSignal: MonitoredSystemLedgerLatestSignal{
				Name:   "tower",
				Type:   "proxmox-node",
				Source: "proxmox",
				At:     "2026-03-18T17:35:00Z",
			},
			Source: "multiple",
			Explanation: MonitoredSystemLedgerExplanation{
				Summary: "Counts as one monitored system because Pulse merged 2 top-level views into one canonical system using shared machine identity.",
				Reasons: []MonitoredSystemLedgerExplanationReason{
					{
						Kind:    "grouped",
						Signal:  "identity-match",
						Summary: "Pulse matched these top-level views by shared canonical machine identity.",
					},
				},
				Surfaces: []MonitoredSystemLedgerExplanationSurface{
					{Name: "Tower", Type: "host", Source: "agent"},
					{Name: "tower", Type: "proxmox-node", Source: "proxmox"},
				},
			},
		},
	}

	got, err := json.Marshal(payload.NormalizeCollections())
	if err != nil {
		t.Fatalf("marshal monitored system ledger preview response: %v", err)
	}

	const want = `{
			"current_count":1,
			"projected_count":1,
			"additional_count":0,
		"effect":"attaches_existing",
		"current_systems":[
			{
				"name":"Tower",
				"type":"host",
				"status":"online",
				"status_explanation":{
					"summary":"All included top-level collection paths currently report online status.",
					"reasons":[]
				},
				"latest_included_signal":{
					"name":"Tower",
					"type":"host",
					"source":"agent",
					"at":"2026-03-18T17:30:00Z"
				},
				"source":"agent",
				"explanation":{
					"summary":"Counts as one monitored system because Pulse sees one top-level host view from agent.",
					"reasons":[
						{
							"kind":"standalone",
							"signal":"single-top-level-view",
							"summary":"No overlapping top-level source matched this system."
						}
					],
					"surfaces":[
						{
							"name":"Tower",
							"type":"host",
							"source":"agent"
						}
					]
				}
			}
		],
		"projected_systems":[
			{
				"name":"tower",
				"type":"proxmox-node",
				"status":"online",
				"status_explanation":{
					"summary":"All included top-level collection paths currently report online status.",
					"reasons":[]
				},
				"latest_included_signal":{
					"name":"tower",
					"type":"proxmox-node",
					"source":"proxmox",
					"at":"2026-03-18T17:35:00Z"
				},
				"source":"multiple",
				"explanation":{
					"summary":"Counts as one monitored system because Pulse merged 2 top-level views into one canonical system using shared machine identity.",
					"reasons":[
						{
							"kind":"grouped",
							"signal":"identity-match",
							"summary":"Pulse matched these top-level views by shared canonical machine identity."
						}
					],
					"surfaces":[
						{
							"name":"Tower",
							"type":"host",
							"source":"agent"
						},
						{
							"name":"tower",
							"type":"proxmox-node",
							"source":"proxmox"
						}
					]
				}
			}
		],
		"current_system":{
			"name":"Tower",
			"type":"host",
			"status":"online",
			"status_explanation":{
				"summary":"All included top-level collection paths currently report online status.",
				"reasons":[]
			},
			"latest_included_signal":{
				"name":"Tower",
				"type":"host",
				"source":"agent",
				"at":"2026-03-18T17:30:00Z"
			},
			"source":"agent",
			"explanation":{
				"summary":"Counts as one monitored system because Pulse sees one top-level host view from agent.",
				"reasons":[
					{
						"kind":"standalone",
						"signal":"single-top-level-view",
						"summary":"No overlapping top-level source matched this system."
					}
				],
				"surfaces":[
					{
						"name":"Tower",
						"type":"host",
						"source":"agent"
					}
				]
			}
		},
		"projected_system":{
			"name":"tower",
			"type":"proxmox-node",
			"status":"online",
			"status_explanation":{
				"summary":"All included top-level collection paths currently report online status.",
				"reasons":[]
			},
			"latest_included_signal":{
				"name":"tower",
				"type":"proxmox-node",
				"source":"proxmox",
				"at":"2026-03-18T17:35:00Z"
			},
			"source":"multiple",
			"explanation":{
				"summary":"Counts as one monitored system because Pulse merged 2 top-level views into one canonical system using shared machine identity.",
				"reasons":[
					{
						"kind":"grouped",
						"signal":"identity-match",
						"summary":"Pulse matched these top-level views by shared canonical machine identity."
					}
				],
				"surfaces":[
					{
						"name":"Tower",
						"type":"host",
						"source":"agent"
					},
					{
						"name":"tower",
						"type":"proxmox-node",
						"source":"proxmox"
					}
				]
			}
		}
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_MonitoredSystemLedgerDoesNotEmitCompatibilityAliases(t *testing.T) {
	entry := monitoredSystemLedgerEntry(unifiedresources.MonitoredSystemRecord{
		Name:   "Tower",
		Type:   "host",
		Status: unifiedresources.StatusWarning,
		StatusExplanation: unifiedresources.MonitoredSystemStatusExplanation{
			Summary: "At least one included source is stale, so Pulse marks this monitored system as warning.",
			Reasons: []unifiedresources.MonitoredSystemStatusReason{},
		},
		LastSeen: time.Date(2026, 3, 18, 17, 35, 0, 0, time.UTC),
		LatestIncludedSignal: unifiedresources.MonitoredSystemLatestSignal{
			Name:   "tower.local",
			Type:   "docker-host",
			Source: "docker",
			At:     time.Date(2026, 3, 18, 17, 30, 0, 0, time.UTC),
		},
		Source: "multiple",
		Explanation: unifiedresources.MonitoredSystemGroupingExplanation{
			Summary:  "Counts as one monitored system because Pulse merged 2 top-level views into one canonical system using shared machine identity.",
			Reasons:  []unifiedresources.MonitoredSystemGroupingReason{},
			Surfaces: []unifiedresources.MonitoredSystemGroupingSurface{},
		},
	})

	payload := MonitoredSystemLedgerResponse{
		Systems: []MonitoredSystemLedgerEntry{entry},
		Total:   1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal monitored system ledger response: %v", err)
	}

	const want = `{
		"systems":[
			{
				"name":"Tower",
				"type":"host",
				"status":"warning",
				"status_explanation":{
					"summary":"At least one included source is stale, so Pulse marks this monitored system as warning.",
					"reasons":[]
				},
				"latest_included_signal":{
					"name":"tower.local",
					"type":"docker-host",
					"source":"docker",
					"at":"2026-03-18T17:30:00Z"
				},
				"source":"multiple",
				"explanation":{
					"summary":"Counts as one monitored system because Pulse merged 2 top-level views into one canonical system using shared machine identity.",
					"reasons":[],
					"surfaces":[]
				}
			}
			],
			"total":1
		}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResolveAuthEnvPathUsesCanonicalRuntimeDataDir(t *testing.T) {
	envDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", envDir)

	explicitDir := t.TempDir()
	if got := resolveAuthEnvPath(explicitDir); got != filepath.Join(explicitDir, ".env") {
		t.Fatalf("resolveAuthEnvPath(explicit) = %q, want %q", got, filepath.Join(explicitDir, ".env"))
	}

	if got := resolveAuthEnvPath(""); got != filepath.Join(envDir, ".env") {
		t.Fatalf("resolveAuthEnvPath(env fallback) = %q, want %q", got, filepath.Join(envDir, ".env"))
	}
}

func TestContract_ResolveAuthEnvWritePathsDeduplicatesCanonicalFallback(t *testing.T) {
	envDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", envDir)

	paths := resolveAuthEnvWritePaths("", "")
	if len(paths) != 1 {
		t.Fatalf("resolveAuthEnvWritePaths() len = %d, want 1", len(paths))
	}
	if want := filepath.Join(envDir, ".env"); paths[0] != want {
		t.Fatalf("resolveAuthEnvWritePaths()[0] = %q, want %q", paths[0], want)
	}
}

func TestContract_WriteAuthEnvFileFallsBackToDataPath(t *testing.T) {
	configPathFile := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(configPathFile, []byte("blocked"), 0600); err != nil {
		t.Fatalf("write blocked config path file: %v", err)
	}
	dataDir := t.TempDir()

	writtenPath, err := writeAuthEnvFile(configPathFile, dataDir, []byte("PULSE_AUTH_USER='pulse'\n"))
	if err != nil {
		t.Fatalf("writeAuthEnvFile() error = %v", err)
	}

	wantPath := filepath.Join(dataDir, ".env")
	if writtenPath != wantPath {
		t.Fatalf("writeAuthEnvFile() path = %q, want %q", writtenPath, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("stat fallback auth env: %v", err)
	}
}

func TestContract_RecoveryTokenPersistenceJSONSnapshot(t *testing.T) {
	payload := []*RecoveryToken{
		{
			TokenHash: recoveryTokenHash("raw-recovery-token"),
			CreatedAt: time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC),
			ExpiresAt: time.Date(2026, 2, 8, 14, 14, 15, 0, time.UTC),
			Used:      true,
			UsedAt:    time.Date(2026, 2, 8, 13, 24, 15, 0, time.UTC),
			IP:        "192.168.1.10",
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal recovery token persistence: %v", err)
	}

	const want = `[
		{
			"token_hash":"59b5880d54ca8c991c09269834d59ea09ab4f467fd4d580a932cd70c5b993fa4",
			"created_at":"2026-02-08T13:14:15Z",
			"expires_at":"2026-02-08T14:14:15Z",
			"used":true,
			"used_at":"2026-02-08T13:24:15Z",
			"ip":"192.168.1.10"
		}
	]`

	assertJSONSnapshot(t, got, want)
}

func TestContract_PersistentAuthStoresRequireExplicitInitialization(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)

	assertPanics := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Fatalf("%s should require explicit initialization", name)
			}
		}()
		fn()
	}

	assertPanics("session store", func() { _ = GetSessionStore() })
	assertPanics("csrf store", func() { _ = GetCSRFStore() })
	assertPanics("recovery token store", func() { _ = GetRecoveryTokenStore() })
}

func TestContract_HostedSessionAuthPrecedesAnonymousFallback(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)

	InitSessionStore(t.TempDir())

	store := GetSessionStore()
	sessionToken := generateSessionToken()
	store.CreateSession(sessionToken, 24*time.Hour, "contract-test", "127.0.0.1", "hosted-owner@example.com")
	record, err := config.NewAPITokenRecord("hosted-contract-token.12345678", "hosted-contract", []string{config.ScopeSettingsWrite})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{*record},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/security/tokens/relay-mobile", nil)
	req.AddCookie(&http.Cookie{
		Name:  "pulse_session",
		Value: sessionToken,
	})
	rec := httptest.NewRecorder()

	if !CheckAuth(cfg, rec, req) {
		t.Fatal("CheckAuth() = false, want true for valid hosted browser session")
	}
	if got := rec.Header().Get("X-Authenticated-User"); got != "hosted-owner@example.com" {
		t.Fatalf("X-Authenticated-User = %q, want hosted-owner@example.com", got)
	}
	if got := rec.Header().Get("X-Auth-Method"); got != "session" {
		t.Fatalf("X-Auth-Method = %q, want session", got)
	}
}

func TestContract_SecureHostedSessionRequiresHostPrefixedCookie(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)

	InitSessionStore(t.TempDir())

	store := GetSessionStore()
	sessionToken := generateSessionToken()
	store.CreateSession(sessionToken, 24*time.Hour, "contract-test", "127.0.0.1", "hosted-owner@example.com")
	record, err := config.NewAPITokenRecord("hosted-secure-session-token.12345678", "hosted-secure-session", []string{config.ScopeSettingsWrite})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{*record},
	}

	t.Run("legacy cookie is rejected on secure requests", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "https://pulse.example.test/api/security/tokens/relay-mobile", nil)
		req.TLS = &tls.ConnectionState{}
		req.AddCookie(&http.Cookie{
			Name:  cookieNameSession,
			Value: sessionToken,
		})
		rec := httptest.NewRecorder()

		if CheckAuth(cfg, rec, req) {
			t.Fatal("CheckAuth() = true, want false for legacy session cookie on secure request")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d when secure request only presents legacy cookie", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("host-prefixed cookie remains valid on secure requests", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "https://pulse.example.test/api/security/tokens/relay-mobile", nil)
		req.TLS = &tls.ConnectionState{}
		req.AddCookie(&http.Cookie{
			Name:  cookieNameSessionSecure,
			Value: sessionToken,
		})
		rec := httptest.NewRecorder()

		if !CheckAuth(cfg, rec, req) {
			t.Fatal("CheckAuth() = false, want true for __Host- session cookie on secure request")
		}
		if got := rec.Header().Get("X-Authenticated-User"); got != "hosted-owner@example.com" {
			t.Fatalf("X-Authenticated-User = %q, want hosted-owner@example.com", got)
		}
	})
}

func TestContract_UniversalRateLimitStateIsScopedPerRouterConfig(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	first := UniversalRateLimitMiddlewareWithConfig(newEndpointRateLimitConfig(), handler)
	second := UniversalRateLimitMiddlewareWithConfig(newEndpointRateLimitConfig(), handler)

	makeRequest := func(target http.Handler) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		req.RemoteAddr = "198.51.100.25:12345"
		rec := httptest.NewRecorder()
		target.ServeHTTP(rec, req)
		return rec
	}

	for i := 0; i < 10; i++ {
		if rec := makeRequest(first); rec.Code != http.StatusOK {
			t.Fatalf("first router request %d status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	if rec := makeRequest(first); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("first router overflow status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	if rec := makeRequest(second); rec.Code != http.StatusOK {
		t.Fatalf("second router first request status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestContract_RouterShutdownClosesOwnedPersistentAuthStoresAfterGlobalRebind(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)

	routerOne := NewRouter(&config.Config{DataPath: t.TempDir()}, nil, nil, nil, nil, "1.0.0")
	routerTwo := NewRouter(&config.Config{DataPath: t.TempDir()}, nil, nil, nil, nil, "1.0.0")
	t.Cleanup(routerTwo.shutdownBackgroundWorkers)

	if routerOne.sessionStore == nil || routerOne.csrfStore == nil {
		t.Fatal("routerOne should capture initialized persistent auth stores")
	}
	if routerOne.recoveryTokenStore == nil {
		t.Fatal("routerOne should capture initialized recovery token store")
	}
	if routerTwo.sessionStore == nil || routerTwo.csrfStore == nil {
		t.Fatal("routerTwo should capture initialized persistent auth stores")
	}
	if routerTwo.recoveryTokenStore == nil {
		t.Fatal("routerTwo should capture initialized recovery token store")
	}
	if routerOne.sessionStore == routerTwo.sessionStore {
		t.Fatal("router instances should not share the same session store after rebind")
	}
	if routerOne.csrfStore == routerTwo.csrfStore {
		t.Fatal("router instances should not share the same csrf store after rebind")
	}
	if routerOne.recoveryTokenStore == routerTwo.recoveryTokenStore {
		t.Fatal("router instances should not share the same recovery token store after rebind")
	}

	routerOne.shutdownBackgroundWorkers()

	select {
	case <-routerOne.sessionStore.workerDone:
	default:
		t.Fatal("routerOne session store worker should be closed after router shutdown")
	}

	select {
	case <-routerOne.csrfStore.workerDone:
	default:
		t.Fatal("routerOne csrf store worker should be closed after router shutdown")
	}

	select {
	case <-routerTwo.sessionStore.workerDone:
		t.Fatal("routerTwo session store should remain active when routerOne shuts down")
	default:
	}

	select {
	case <-routerTwo.csrfStore.workerDone:
		t.Fatal("routerTwo csrf store should remain active when routerOne shuts down")
	default:
	}

	select {
	case <-routerOne.recoveryTokenStore.stopCleanup:
	default:
		t.Fatal("routerOne recovery token store should be closed after router shutdown")
	}

	select {
	case <-routerTwo.recoveryTokenStore.stopCleanup:
		t.Fatal("routerTwo recovery token store should remain active when routerOne shuts down")
	default:
	}
}

func TestContract_HostedOrgManagerSessionCanMintRelayMobileToken(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	dataDir := t.TempDir()
	hashed, err := authpkg.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		AuthUser:   "platform-admin",
		AuthPass:   hashed,
	}

	mtp := config.NewMultiTenantPersistence(dataDir)
	org := &models.Organization{
		ID:          "org-a",
		DisplayName: "Org A",
		OwnerUserID: "operator-owner",
		Members: []models.OrganizationMember{
			{UserID: "legacy-owner", Role: models.OrgRoleOwner, AddedAt: time.Now()},
			{UserID: "operator-owner", Role: models.OrgRoleOwner, AddedAt: time.Now()},
		},
	}
	if err := mtp.SaveOrganization(org); err != nil {
		t.Fatalf("save organization: %v", err)
	}

	router := newMultiTenantRouter(t, cfg)
	setLicenseTierForHandlersForTests(t, router.licenseHandlers, "org-a", pkglicensing.TierRelay)

	sessionToken := "relay-owner-session-" + strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339Nano), ":", "-")
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "legacy-owner")

	req := httptest.NewRequest(http.MethodPost, "/api/security/tokens/relay-mobile", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", generateCSRFToken(sessionToken))
	req.Header.Set("X-Pulse-Org-ID", "org-a")
	req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})

	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("relay mobile token route status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Token  string      `json:"token"`
		Record apiTokenDTO `json:"record"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode relay mobile response: %v", err)
	}
	if payload.Token == "" {
		t.Fatal("expected relay mobile token in response")
	}
	if payload.Record.OwnerUserID != "legacy-owner" {
		t.Fatalf("ownerUserId = %q, want legacy-owner", payload.Record.OwnerUserID)
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected one stored token, got %d", len(cfg.APITokens))
	}
	if cfg.APITokens[0].OrgID != "org-a" {
		t.Fatalf("stored token orgId = %q, want org-a", cfg.APITokens[0].OrgID)
	}
}

func TestContract_RelayMobileScopeCanReadOnboardingDeepLink(t *testing.T) {
	rawToken := "relay-mobile-onboarding-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeRelayMobileAccess}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/onboarding/deep-link", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code == http.StatusForbidden && strings.Contains(rec.Body.String(), "missing_scope") {
		t.Fatalf("relay mobile scope should satisfy onboarding deep-link gating, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestContract_OnboardingDeepLinkIgnoresQueryAuthToken(t *testing.T) {
	rawToken := "relay-mobile-onboarding-query-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeRelayMobileAccess}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/onboarding/deep-link?auth_token="+url.QueryEscape(rawToken), nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d when auth token is only provided in query string", rec.Code, http.StatusUnauthorized)
	}
}

func TestContract_SetupTokenRoutesRejectQueryAuthToken(t *testing.T) {
	cfg := newTestConfigWithTokens(t, newTokenRecord(t, "settings-write-token", []string{config.ScopeSettingsWrite}, nil))
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	t.Setenv("HOME", t.TempDir())

	token := "fedcba9876543210fedcba9876543210"
	tokenHash := authpkg.HashAPIToken(token)
	router.configHandlers.codeMutex.Lock()
	router.configHandlers.setupTokens[tokenHash] = &SetupTokenRecord{ExpiresAt: time.Now().Add(time.Minute)}
	router.configHandlers.codeMutex.Unlock()

	t.Run("verify-temperature-ssh requires header token transport", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/system/verify-temperature-ssh?auth_token="+token, strings.NewReader(`{"nodes":""}`))
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d when setup token is only in query string", rec.Code, http.StatusUnauthorized)
		}

		req = httptest.NewRequest(http.MethodPost, "/api/system/verify-temperature-ssh", strings.NewReader(`{"nodes":""}`))
		req.Header.Set("X-Setup-Token", token)
		rec = httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d when setup token is provided in header", rec.Code, http.StatusOK)
		}
	})

	t.Run("ssh-config requires header token transport", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/system/ssh-config?auth_token="+token, strings.NewReader("Host example\nHostname example\n"))
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d when setup token is only in query string", rec.Code, http.StatusUnauthorized)
		}

		req = httptest.NewRequest(http.MethodPost, "/api/system/ssh-config", strings.NewReader("Host example\nHostname example\n"))
		req.Header.Set("X-Setup-Token", token)
		rec = httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d when setup token is provided in header", rec.Code, http.StatusOK)
		}
	})
}

func TestContract_RelayMobileScopeCannotReadApprovalDetail(t *testing.T) {
	rawToken := "relay-mobile-approval-detail-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeRelayMobileAccess}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/ai/approvals/approval-1", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("relay mobile scope should not satisfy approval detail gating, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), config.ScopeAIExecute) {
		t.Fatalf("relay mobile approval detail rejection should mention %q, got %s", config.ScopeAIExecute, rec.Body.String())
	}
}

func TestContract_UnifiedAgentReportResponseJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"success":   true,
		"agentId":   "agent-123",
		"lastSeen":  "2026-02-08T13:14:15Z",
		"platform":  "linux",
		"osName":    "Debian GNU/Linux",
		"osVersion": "12",
		"config": map[string]any{
			"commandsEnabled": true,
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal unified agent report response: %v", err)
	}

	const want = `{
		"agentId":"agent-123",
		"config":{"commandsEnabled":true},
		"lastSeen":"2026-02-08T13:14:15Z",
		"osName":"Debian GNU/Linux",
		"osVersion":"12",
		"platform":"linux",
		"success":true
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_UnifiedAgentLookupFailsClosedOnAmbiguousHostname(t *testing.T) {
	handler := newUnifiedAgentHandlerForTests(t,
		models.Host{ID: "host-1", Hostname: "webserver.corp.example.com", DisplayName: "Web One"},
		models.Host{ID: "host-2", Hostname: "webserver.other.example.com", DisplayName: "Web Two"},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent/lookup?hostname=webserver", nil)
	rec := httptest.NewRecorder()

	handler.HandleLookup(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected not found status for ambiguous hostname lookup, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode ambiguous lookup error response: %v", err)
	}
	if resp.Code != "agent_not_found" {
		t.Fatalf("expected agent_not_found code, got %q", resp.Code)
	}
}

func TestContract_HostsShareResolvedIdentityTreatsLoopbackAliasAsSameNode(t *testing.T) {
	if !hostsShareResolvedIdentity("https://localhost:7655", "https://127.0.0.1:7655") {
		t.Fatal("expected localhost and loopback IP to resolve as the same host identity")
	}
	if hostsShareResolvedIdentity("https://192.0.2.10:7655", "https://192.0.2.11:7655") {
		t.Fatal("expected different IP endpoints to remain distinct host identities")
	}
}

func TestContract_DiagnosticsDockerPrepareTokenInstallCommandUsesLifecycleTransport(t *testing.T) {
	baseURL := "https://pulse.example.com/base"
	got := buildContainerRuntimeAgentInstallCommand(baseURL, "token-123")

	if !strings.Contains(got, posixShellQuote(baseURL+"/install.sh")) {
		t.Fatalf("install command missing normalized install script URL: %s", got)
	}
	if !strings.Contains(got, "--enable-host=false") {
		t.Fatalf("install command missing canonical host-disable flag: %s", got)
	}
	if strings.Contains(got, "--disable-host") {
		t.Fatalf("install command preserved stale disable-host flag: %s", got)
	}
	if !strings.Contains(got, `| { if [ "$(id -u)" -eq 0 ]; then bash -s --`) {
		t.Fatalf("install command missing governed root-or-sudo wrapper: %s", got)
	}
	if strings.Contains(got, "curl -fsSL "+posixShellQuote(baseURL+"/install.sh")+" | sudo bash -s --") {
		t.Fatalf("install command preserved raw sudo pipe instead of governed wrapper: %s", got)
	}
}

func TestContract_DiagnosticsDockerPrepareTokenOptionalAuthInstallCommandOmitsToken(t *testing.T) {
	got := buildContainerRuntimeAgentInstallCommand("http://pulse.example.com:7655/", "")

	if strings.Contains(got, "--token") {
		t.Fatalf("optional-auth install command preserved token flag: %s", got)
	}
	if !strings.Contains(got, "--insecure") {
		t.Fatalf("optional-auth install command missing insecure flag for plain HTTP Pulse URL: %s", got)
	}
}

func TestContract_SetupScriptURLCommandUsesFailFastQuotedTransport(t *testing.T) {
	url := "https://pulse.example.com/api/setup-script?type=pve&host=pve1.local"
	got := buildSetupScriptCommand(url, "token-123")

	if !strings.Contains(got, "curl -fsSL "+posixShellQuote(url)+" | ") {
		t.Fatalf("setup-script command missing canonical fail-fast transport: %s", got)
	}
	if !strings.Contains(got, `if [ "$(id -u)" -eq 0 ]; then PULSE_SETUP_TOKEN=`+posixShellQuote("token-123")+` bash`) {
		t.Fatalf("setup-script command missing direct-root execution path: %s", got)
	}
	if !strings.Contains(got, `elif command -v sudo >/dev/null 2>&1; then sudo env PULSE_SETUP_TOKEN=`+posixShellQuote("token-123")+` bash`) {
		t.Fatalf("setup-script command missing sudo execution path: %s", got)
	}
	if strings.Contains(got, "curl -sSL ") {
		t.Fatalf("setup-script command preserved stale non-fail-fast curl transport: %s", got)
	}
}

func TestContract_SetupScriptEmbedsFailFastGuidance(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pve&host=http://sentinel-host:8006&pulse_url=http://sentinel-url:7656", nil)
	rec := httptest.NewRecorder()

	handlers.HandleSetupScript(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	script := rec.Body.String()
	if !strings.Contains(script, `PULSE_BOOTSTRAP_COMMAND_WITH_ENV='curl -fsSL '"'"'http://sentinel-url:7656/api/setup-script?host=http%3A%2F%2Fsentinel-host%3A8006&pulse_url=http%3A%2F%2Fsentinel-url%3A7656&type=pve'"'"' | `) {
		t.Fatalf("setup script missing canonical bootstrap command owner: %s", script)
	}
	if strings.Contains(script, `PULSE_BOOTSTRAP_COMMAND_WITH_ENV='curl -fsSL '"'"'http://sentinel-url:7656/api/setup-script?host=http%3A%2F%2Fsentinel-host%3A8006&pulse_url=http%3A%2F%2Fsentinel-url%3A7656&type=pve'"'"' | { if [ "$(id -u)" -eq 0 ]; then PULSE_SETUP_TOKEN=`) {
		t.Fatalf("setup script bootstrap command should defer setup token to runtime hydration, got: %s", script)
	}
	if !strings.Contains(script, `echo "  $PULSE_BOOTSTRAP_COMMAND_WITH_ENV"`) {
		t.Fatalf("setup script missing bootstrap-command retry guidance: %s", script)
	}
	if !strings.Contains(script, `SETUP_SCRIPT_URL="http://sentinel-url:7656/api/setup-script?host=http%3A%2F%2Fsentinel-host%3A8006&pulse_url=http%3A%2F%2Fsentinel-url%3A7656&type=pve"`) {
		t.Fatalf("setup script missing canonical encoded retry URL: %s", script)
	}
	if !strings.Contains(script, `PULSE_SETUP_TOKEN="${PULSE_SETUP_TOKEN:-`) {
		t.Fatalf("setup script missing canonical PVE setup-token initialization before rerun guidance: %s", script)
	}
	if !strings.Contains(script, `echo "Root privileges required. Run as root (su -) and retry."`) {
		t.Fatalf("setup script missing canonical root requirement guidance: %s", script)
	}
	if !strings.Contains(script, `echo "This setup flow must run on the Proxmox host so Pulse can create"`) {
		t.Fatalf("setup script missing canonical off-host rerun guidance: %s", script)
	}
	if strings.Contains(script, `echo "  curl -sSL \"$SETUP_SCRIPT_URL\" | bash"`) || strings.Contains(script, `echo "   curl -sSL \"$PULSE_URL/api/setup-script?type=pve&host=YOUR_PVE_URL&pulse_url=$PULSE_URL\" | bash"`) {
		t.Fatalf("setup script preserved stale non-fail-fast guidance: %s", script)
	}
	if strings.Contains(script, `echo "Manual setup steps:"`) || strings.Contains(script, `echo "  2. In Pulse: Settings → Nodes → Add Node (enter token from above)"`) {
		t.Fatalf("setup script preserved stale off-host manual token flow: %s", script)
	}
	if !strings.Contains(script, `done <<< "$OLD_TOKENS_PVE"`) {
		t.Fatalf("setup script missing explicit pve old-token cleanup loop: %s", script)
	}
	if !strings.Contains(script, `done <<< "$OLD_TOKENS_PAM"`) {
		t.Fatalf("setup script missing explicit pam old-token cleanup loop: %s", script)
	}
	if strings.Contains(script, `done <<< "$OLD_TOKENS"`) {
		t.Fatalf("setup script preserved stale undefined old-token cleanup variable: %s", script)
	}
	if !strings.Contains(script, `pveum user token remove pulse-monitor@pve "$TOKEN"`) {
		t.Fatalf("setup script missing pve token cleanup command: %s", script)
	}
	if !strings.Contains(script, `pveum user token remove pulse-monitor@pam "$TOKEN"`) {
		t.Fatalf("setup script missing pam token cleanup command: %s", script)
	}
	if !strings.Contains(script, `TOKEN_MATCH_PREFIX="pulse-sentinel-url"`) {
		t.Fatalf("setup script missing canonical token-match prefix for cleanup discovery: %s", script)
	}
	if !strings.Contains(script, `grep -E "^${TOKEN_MATCH_PREFIX}(-[0-9]+)?$"`) {
		t.Fatalf("setup script missing canonical cleanup token discovery matcher: %s", script)
	}
	if !strings.Contains(script, `pulse_pve_token_exists() {`) ||
		!strings.Contains(script, `grep -Fx "$TOKEN_NAME" >/dev/null 2>&1`) ||
		!strings.Contains(script, `if pulse_pve_token_exists; then`) {
		t.Fatalf("setup script missing exact PVE token rotation detection: %s", script)
	}
	if strings.Contains(script, `PULSE_IP_PATTERN=`) {
		t.Fatalf("setup script preserved stale ip-pattern cleanup discovery: %s", script)
	}
	if strings.Contains(script, `grep -q "$TOKEN_NAME"`) {
		t.Fatalf("setup script preserved stale broad PVE token rotation detection: %s", script)
	}
	if !strings.Contains(script, `resolve_authorized_keys_path() {`) {
		t.Fatalf("setup script missing authorized_keys symlink resolver: %s", script)
	}
	if !strings.Contains(script, `resolved="$(readlink -f "$auth_keys" 2>/dev/null || true)"`) {
		t.Fatalf("setup script missing canonical authorized_keys target resolution: %s", script)
	}
	if !strings.Contains(script, `install_authorized_keys_file() {`) {
		t.Fatalf("setup script missing shared authorized_keys install helper: %s", script)
	}
	if strings.Count(script, `AUTH_KEYS="$(resolve_authorized_keys_path)"`) < 2 {
		t.Fatalf("setup script install and uninstall paths must both resolve authorized_keys symlinks: %s", script)
	}
	if !strings.Contains(script, `grep -vF '# pulse-' "$AUTH_KEYS" > "$TMP_AUTH_KEYS" 2>/dev/null`) {
		t.Fatalf("setup script uninstall path must remove all Pulse-managed SSH keys through the resolved authorized_keys target: %s", script)
	}
	if !strings.Contains(script, `grep -vF "# pulse-" "$AUTH_KEYS" > "$TMP_AUTH_KEYS" 2>/dev/null`) {
		t.Fatalf("setup script install path must filter existing Pulse-managed SSH keys through the resolved authorized_keys target: %s", script)
	}
	if strings.Contains(script, `grep -vF '# pulse-managed-key'`) {
		t.Fatalf("setup script preserved stale managed-key-only cleanup: %s", script)
	}
	if strings.Contains(script, `/root/.ssh/authorized_keys.tmp`) || strings.Contains(script, `echo "$SSH_SENSORS_KEY_ENTRY" >> /root/.ssh/authorized_keys`) {
		t.Fatalf("setup script preserved symlink-breaking authorized_keys rewrite: %s", script)
	}
	if !strings.Contains(script, `PULSE_SENSORS_WRAPPER="/usr/local/sbin/pulse-sensors"`) {
		t.Fatalf("setup script missing canonical Pulse sensor wrapper path: %s", script)
	}
	if !strings.Contains(script, `SSH_SENSORS_KEY_ENTRY="command=\"$PULSE_SENSORS_WRAPPER\",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty $SSH_SENSORS_PUBLIC_KEY # pulse-sensors"`) {
		t.Fatalf("setup script must force temperature SSH keys to the Pulse sensor wrapper: %s", script)
	}
	if !strings.Contains(script, `"smart": collect_smart(),`) ||
		!strings.Contains(script, `apt-get install -y smartmontools`) ||
		!strings.Contains(script, `def inferred_smart_device_types(device):`) ||
		!strings.Contains(script, `for dtype in inferred_smart_device_types(device):`) ||
		!strings.Contains(script, `attempt_index == len(attempts) - 1`) {
		t.Fatalf("setup script missing SMART temperature wrapper contract: %s", script)
	}
	if strings.Contains(script, `SSH_SENSORS_KEY_ENTRY="command=\"sensors -j\"`) {
		t.Fatalf("setup script preserved stale raw sensors forced command: %s", script)
	}
	if strings.Contains(script, `echo "Please run this script as root"`) {
		t.Fatalf("setup script preserved stale root-only guidance: %s", script)
	}
	if !strings.Contains(script, `grep -Eq '"status"[[:space:]]*:[[:space:]]*"success"'`) {
		t.Fatalf("setup script missing secure success detection: %s", script)
	}
	if strings.Contains(script, `grep -q "success"`) {
		t.Fatalf("setup script preserved broad success substring detection: %s", script)
	}
	if !strings.Contains(script, `curl -fsS -X POST "$PULSE_URL/api/auto-register"`) {
		t.Fatalf("setup script missing fail-fast auto-register transport: %s", script)
	}
	if !strings.Contains(script, `curl -fsS -X POST "$PULSE_URL/api/auto-unregister"`) {
		t.Fatalf("setup script missing fail-fast auto-unregister transport: %s", script)
	}
	if !strings.Contains(script, `"source":"script"`) {
		t.Fatalf("setup script missing canonical /api/auto-register source marker: %s", script)
	}
	if !strings.Contains(script, `echo "  • Removing Pulse connection from server..."`) {
		t.Fatalf("setup script missing canonical server-side teardown guidance: %s", script)
	}
	if !strings.Contains(script, `REGISTER_RC=$?`) {
		t.Fatalf("setup script missing explicit auto-register curl exit-code handling: %s", script)
	}
	if !strings.Contains(script, `echo "⚠️  Auto-registration skipped: token value unavailable"`) {
		t.Fatalf("setup script missing fail-closed token-value-unavailable guidance: %s", script)
	}
	if strings.Contains(script, `curl -s -X POST "$PULSE_URL/api/auto-register"`) {
		t.Fatalf("setup script preserved stale non-fail-fast auto-register transport: %s", script)
	}
	if !strings.Contains(script, `echo "The provided Pulse setup token was invalid or expired"`) {
		t.Fatalf("setup script missing invalid setup-token guidance: %s", script)
	}
	if !strings.Contains(script, `echo "Get a fresh setup token from Pulse Settings → Nodes and rerun this script."`) {
		t.Fatalf("setup script missing fresh setup-token rerun guidance: %s", script)
	}
	if !strings.Contains(script, `SETUP_TOKEN_INVALID=true`) {
		t.Fatalf("setup script missing PVE auth-failure state tracking: %s", script)
	}
	if !strings.Contains(script, `echo "Pulse setup token authentication failed."`) {
		t.Fatalf("setup script missing PVE auth-failure completion guidance: %s", script)
	}
	if !strings.Contains(script, `if [ "$AUTO_REG_SUCCESS" != true ] && [ "$SETUP_TOKEN_INVALID" != true ]; then`) {
		t.Fatalf("setup script missing PVE auth-failure footer guard: %s", script)
	}
	if !strings.Contains(script, `echo "📝 Use the token details below in Pulse Settings → Nodes to finish registration."`) {
		t.Fatalf("setup script missing canonical auto-register failure continuation guidance: %s", script)
	}
	if strings.Contains(script, `echo "To enable auto-registration, add your API token to the setup URL"`) {
		t.Fatalf("setup script preserved stale API-token auth guidance: %s", script)
	}
	if strings.Contains(script, `echo "The provided API token was invalid"`) {
		t.Fatalf("setup script preserved stale invalid API-token guidance: %s", script)
	}
	if strings.Contains(script, `echo "To enable auto-registration, rerun with a valid Pulse setup token"`) {
		t.Fatalf("setup script preserved stale split setup-token auth guidance: %s", script)
	}
	if strings.Contains(script, `echo "📝 For manual setup:"`) {
		t.Fatalf("setup script preserved stale numbered manual-setup fallback: %s", script)
	}
	if strings.Contains(script, `echo "   2. Add this node manually in Pulse Settings"`) {
		t.Fatalf("setup script preserved stale auto-register failure continuation guidance: %s", script)
	}
	if !strings.Contains(script, `echo "Pulse monitoring token setup completed."`) {
		t.Fatalf("setup script missing truthful manual completion messaging: %s", script)
	}
	if !strings.Contains(script, `echo "Pulse monitoring token setup failed."`) {
		t.Fatalf("setup script missing token-create failure completion messaging: %s", script)
	}
	if !strings.Contains(script, `echo "Fix the token creation error above and rerun this script on the node."`) {
		t.Fatalf("setup script missing immediate token-create failure rerun guidance: %s", script)
	}
	if !strings.Contains(script, `echo "Resolve the token creation error shown above and rerun this script on the node."`) {
		t.Fatalf("setup script missing token-create failure rerun guidance: %s", script)
	}
	if !strings.Contains(script, `echo "   Resolve the token output issue above and rerun this script on the node."`) {
		t.Fatalf("setup script missing token-extract failure rerun guidance: %s", script)
	}
	if !strings.Contains(script, `echo "Successfully registered with Pulse monitoring."`) {
		t.Fatalf("setup script missing canonical success messaging: %s", script)
	}
	if !strings.Contains(script, `echo "  Token Value: [See token output above]"`) {
		t.Fatalf("setup script missing canonical token placeholder guidance: %s", script)
	}
	if !strings.Contains(script, `echo "Finish registration in Pulse using the manual setup details below."`) {
		t.Fatalf("setup script missing truthful manual registration guidance: %s", script)
	}
	if !strings.Contains(script, `echo "Add this server to Pulse with:"`) {
		t.Fatalf("setup script missing canonical manual-add heading: %s", script)
	}
	if !strings.Contains(script, `echo "Use these details in Pulse Settings → Nodes to finish registration."`) {
		t.Fatalf("setup script missing canonical manual-add continuation guidance: %s", script)
	}
	if !strings.Contains(script, `echo "⚠️  Auto-registration failed. Finish registration manually in Pulse Settings → Nodes."`) {
		t.Fatalf("setup script missing canonical auto-register failure summary: %s", script)
	}
	if !strings.Contains(script, `echo "  Host URL: $SERVER_HOST"`) {
		t.Fatalf("setup script missing canonical manual host continuity: %s", script)
	}
	if strings.Contains(script, `echo "Manual setup instructions:"`) {
		t.Fatalf("setup script preserved stale manual setup heading: %s", script)
	}
	if strings.Contains(script, `echo "Node registered successfully"`) || strings.Contains(script, `echo "Node successfully registered with Pulse monitoring."`) || strings.Contains(script, `echo "✅ Successfully registered with Pulse!"`) || strings.Contains(script, `echo "Server successfully registered with Pulse monitoring."`) {
		t.Fatalf("setup script preserved stale success copy variants: %s", script)
	}
	if strings.Contains(script, `echo "   Token Value: [See above]"`) || strings.Contains(script, `echo "  Token Value: [Check the output above for the token or instructions]"`) {
		t.Fatalf("setup script preserved stale token placeholder guidance: %s", script)
	}
	if strings.Contains(script, `echo "⚠️  Auto-registration failed. Manual configuration may be needed."`) {
		t.Fatalf("setup script preserved stale auto-register failure summary: %s", script)
	}
	if strings.Contains(script, `PULSE_REG_TOKEN=your-token ./setup.sh`) {
		t.Fatalf("setup script preserved stale rerun token guidance: %s", script)
	}
	if strings.Contains(script, `echo "Manual registration may be required."`) {
		t.Fatalf("setup script preserved stale manual-registration token failure guidance: %s", script)
	}
	if strings.Contains(script, `echo "  Host URL: YOUR_PROXMOX_HOST:8006"`) {
		t.Fatalf("setup script preserved stale placeholder manual host guidance: %s", script)
	}
	if !strings.Contains(script, `echo "Pulse monitoring token setup could not be completed."`) {
		t.Fatalf("setup script missing token-extract failure completion messaging: %s", script)
	}
	if !strings.Contains(script, `echo "Resolve the token output issue shown above and rerun this script on the node."`) {
		t.Fatalf("setup script missing token-extract completion rerun guidance: %s", script)
	}
	if !strings.Contains(script, `if [ "$TOKEN_READY" = true ]; then
    if smoke_test_pve_token; then
        attempt_auto_registration
    else
        AUTO_REG_SUCCESS=false
    fi
else
    AUTO_REG_SUCCESS=false
fi`) {
		t.Fatalf("setup script does not skip PVE auto-registration when no usable or smoke-tested token is ready: %s", script)
	}
	if !strings.Contains(script, `if [ "$TOKEN_READY" = true ]; then
        echo "Add this server to Pulse with:"`) {
		t.Fatalf("setup script does not gate PVE manual token details on usable token extraction: %s", script)
	}
	if strings.Contains(script, `elif [ "$TOKEN_READY" != true ]; then
    echo "Pulse monitoring token setup completed."`) {
		t.Fatalf("setup script lets PVE token-extract failure fall through to completed token setup: %s", script)
	}

	pbsReq := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pbs&host=https://sentinel-pbs:8007&pulse_url=http://sentinel-url:7656", nil)
	pbsRec := httptest.NewRecorder()

	handlers.HandleSetupScript(pbsRec, pbsReq)

	if pbsRec.Code != http.StatusOK {
		t.Fatalf("pbs status = %d, want %d", pbsRec.Code, http.StatusOK)
	}

	pbsScript := pbsRec.Body.String()
	if !strings.Contains(pbsScript, `echo "  Host URL: $HOST_URL"`) {
		t.Fatalf("setup script missing canonical PBS manual host continuity: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "Pulse monitoring token setup failed."`) {
		t.Fatalf("setup script missing PBS token-create failure completion messaging: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "Pulse monitoring token setup could not be completed."`) {
		t.Fatalf("setup script missing PBS token-extract failure completion messaging: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "⚠️  Auto-registration skipped: no setup token provided"`) {
		t.Fatalf("setup script missing PBS setup-token-skip guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `PULSE_SETUP_TOKEN="${PULSE_SETUP_TOKEN:-`) {
		t.Fatalf("setup script missing canonical PBS setup-token initialization before rerun guidance: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `echo "⚠️  Auto-registration skipped: no setup token provided"
                    AUTO_REG_SUCCESS=false
                    REGISTER_RESPONSE=""
                    REGISTER_RC=1`) {
		t.Fatalf("setup script still forces fake PBS request-failure state after missing setup-token skip: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `echo "⚠️  Auto-registration skipped: token value unavailable"
                AUTO_REG_SUCCESS=false
                REGISTER_RESPONSE=""
                REGISTER_RC=1`) {
		t.Fatalf("setup script still forces fake PBS request-failure state after token-value skip: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `if [ "$REGISTER_ATTEMPTED" != true ]; then`) {
		t.Fatalf("setup script does not distinguish skipped PBS auto-registration paths from attempted requests: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "The provided Pulse setup token was invalid or expired"`) {
		t.Fatalf("setup script missing invalid PBS setup-token guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "Get a fresh setup token from Pulse Settings → Nodes and rerun this script."`) {
		t.Fatalf("setup script missing fresh PBS setup-token rerun guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `SETUP_TOKEN_INVALID=true`) {
		t.Fatalf("setup script missing PBS auth-failure state tracking: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "Pulse setup token authentication failed."`) {
		t.Fatalf("setup script missing PBS auth-failure completion guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `if [ "$AUTO_REG_SUCCESS" != true ] && [ "$SETUP_TOKEN_INVALID" != true ]; then`) {
		t.Fatalf("setup script missing PBS auth-failure footer guard: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "Fix the token creation error above and rerun this script on the node."`) {
		t.Fatalf("setup script missing PBS immediate token-create failure rerun guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "Resolve the token creation error shown above and rerun this script on the node."`) {
		t.Fatalf("setup script missing PBS token-create failure rerun guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "   Resolve the token output issue above and rerun this script on the node."`) {
		t.Fatalf("setup script missing PBS token-extract failure rerun guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "Resolve the token output issue shown above and rerun this script on the node."`) {
		t.Fatalf("setup script missing PBS token-extract completion rerun guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `HOST_URL="https://sentinel-pbs:8007"`) {
		t.Fatalf("setup script missing canonical PBS host binding: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `TOKEN_MATCH_PREFIX="pulse-sentinel-url"`) {
		t.Fatalf("pbs setup script missing canonical token-match prefix for cleanup discovery: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `grep -oE "${TOKEN_MATCH_PREFIX}(-[0-9]+)?" | sort -u || true`) {
		t.Fatalf("pbs setup script missing canonical cleanup token discovery matcher: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `awk '{print $1}' | grep -Fx "$TOKEN_NAME" >/dev/null 2>&1`) {
		t.Fatalf("pbs setup script missing exact token rotation detection: %s", pbsScript)
	}
	tokenCreateIndex := strings.Index(pbsScript, `TOKEN_CREATE_RC=$?`)
	bannerIndex := strings.Index(pbsScript, `echo "IMPORTANT: Copy the token value below - it's only shown once!"`)
	successBranchIndex := strings.Index(pbsScript, "else\n    TOKEN_CREATED=true")
	if tokenCreateIndex == -1 || bannerIndex == -1 || successBranchIndex == -1 {
		t.Fatalf("pbs setup script missing token-create truth markers: %s", pbsScript)
	}
	if bannerIndex < tokenCreateIndex {
		t.Fatalf("pbs setup script prints token-copy banner before token creation result is known: %s", pbsScript)
	}
	if bannerIndex < successBranchIndex {
		t.Fatalf("pbs setup script prints token-copy banner outside the successful token-create branch: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `PULSE_IP_PATTERN=`) {
		t.Fatalf("pbs setup script preserved stale ip-pattern cleanup discovery: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `grep -q "$TOKEN_NAME"`) {
		t.Fatalf("pbs setup script preserved stale broad token rotation detection: %s", pbsScript)
	}
	if strings.Index(pbsScript, `HOST_URL="https://sentinel-pbs:8007"`) > strings.Index(pbsScript, `if [ -z "$PULSE_SETUP_TOKEN" ]; then`) {
		t.Fatalf("setup script binds PBS host too late for manual fallback continuity: %s", pbsScript)
	}
	if strings.Index(pbsScript, `HOST_URL="https://sentinel-pbs:8007"`) > strings.Index(pbsScript, `if [ "$TOKEN_CREATE_RC" -ne 0 ]; then`) {
		t.Fatalf("setup script binds PBS host too late for token-create failure continuity: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `if [ "$TOKEN_READY" = true ]; then
        echo "Add this server to Pulse with:"`) {
		t.Fatalf("setup script does not gate PBS manual token details on usable token extraction: %s", pbsScript)
	}
	attemptBannerIndex := strings.Index(pbsScript, `echo "🔄 Attempting auto-registration with Pulse..."`)
	authTokenGateIndex := strings.Index(pbsScript, `if [ -n "$PULSE_SETUP_TOKEN" ]; then`)
	tokenSkipIndex := strings.Index(pbsScript, `echo "⚠️  Auto-registration skipped: token value unavailable"`)
	if attemptBannerIndex == -1 || authTokenGateIndex == -1 || tokenSkipIndex == -1 {
		t.Fatalf("setup script missing PBS auto-registration truth markers: %s", pbsScript)
	}
	if attemptBannerIndex < authTokenGateIndex {
		t.Fatalf("setup script prints PBS auto-registration attempt banner before the real request path: %s", pbsScript)
	}
	if attemptBannerIndex < tokenSkipIndex {
		t.Fatalf("setup script prints PBS auto-registration attempt banner before token-unavailable skip handling: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `elif [ "$TOKEN_READY" != true ]; then
    echo "Pulse monitoring token setup completed."`) {
		t.Fatalf("setup script lets PBS token-extract failure fall through to completed token setup: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `echo "  Host URL: https://$SERVER_IP:8007"`) {
		t.Fatalf("setup script preserved stale PBS runtime-IP host guidance: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `echo "Manual registration may be required."`) {
		t.Fatalf("setup script preserved stale PBS manual-registration token failure guidance: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `echo "To enable auto-registration, rerun with a valid Pulse setup token"`) {
		t.Fatalf("setup script preserved stale split PBS setup-token auth guidance: %s", pbsScript)
	}
	if !strings.Contains(pbsScript, `echo "⚠️  Auto-registration failed. Finish registration manually in Pulse Settings → Nodes."`) {
		t.Fatalf("setup script missing canonical PBS auto-register failure summary: %s", pbsScript)
	}
	if strings.Count(pbsScript, `echo "📝 Use the token details below in Pulse Settings → Nodes to finish registration."`) < 2 {
		t.Fatalf("setup script missing canonical PBS request-failure/manual-response continuity: %s", pbsScript)
	}
	if strings.Contains(pbsScript, `echo "⚠️  Auto-registration failed. Manual configuration may be needed."`) {
		t.Fatalf("setup script preserved stale PBS auto-register failure summary: %s", pbsScript)
	}
}

func TestContract_SetupScriptRequiresCanonicalHost(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve", nil)
	rec := httptest.NewRecorder()

	handlers.HandleSetupScript(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "Missing required parameter: host" {
		t.Fatalf("body = %q, want canonical missing host guidance", got)
	}
}

func TestContract_SetupScriptUsesCanonicalTypeAndHostValidation(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	invalidTypeReq := httptest.NewRequest(
		http.MethodGet,
		"/api/setup-script?type=pmg&host=https://node.example.internal:8006&pulse_url=https://pulse.example.com:7655",
		nil,
	)
	invalidTypeRec := httptest.NewRecorder()
	handlers.HandleSetupScript(invalidTypeRec, invalidTypeReq)

	if invalidTypeRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid type status = %d, want %d", invalidTypeRec.Code, http.StatusBadRequest)
	}
	if got := strings.TrimSpace(invalidTypeRec.Body.String()); got != "type must be 'pve' or 'pbs'" {
		t.Fatalf("invalid type body = %q, want canonical type guidance", got)
	}

	normalizedHostReq := httptest.NewRequest(
		http.MethodGet,
		"/api/setup-script?type=pve&host=https://pve-node.example.internal&pulse_url=https://pulse.example.com:7655",
		nil,
	)
	normalizedHostRec := httptest.NewRecorder()
	handlers.HandleSetupScript(normalizedHostRec, normalizedHostReq)

	if normalizedHostRec.Code != http.StatusOK {
		t.Fatalf("normalized host status = %d, want %d: %s", normalizedHostRec.Code, http.StatusOK, normalizedHostRec.Body.String())
	}
	body := normalizedHostRec.Body.String()
	if !strings.Contains(body, `SERVER_HOST="https://pve-node.example.internal:8006"`) {
		t.Fatalf("normalized host body missing canonical host, got: %s", truncate(body, 500))
	}
	if !strings.Contains(body, `SETUP_SCRIPT_URL="https://pulse.example.com:7655/api/setup-script?host=https%3A%2F%2Fpve-node.example.internal%3A8006&pulse_url=https%3A%2F%2Fpulse.example.com%3A7655&type=pve"`) {
		t.Fatalf("normalized host body missing canonical rerun URL, got: %s", truncate(body, 700))
	}
}

func TestContract_SetupScriptRequiresCanonicalPulseURL(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve&host=https://pve.local:8006", nil)
	rec := httptest.NewRecorder()

	handlers.HandleSetupScript(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "Missing required parameter: pulse_url" {
		t.Fatalf("body = %q, want canonical missing pulse_url guidance", got)
	}
}

func TestContract_SetupScriptUsesCanonicalShellDownloadHeaders(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pbs&host=https://sentinel-pbs:8007&pulse_url=http://sentinel-url:7656", nil)
	rec := httptest.NewRecorder()

	handlers.HandleSetupScript(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/x-shellscript; charset=utf-8" {
		t.Fatalf("setup-script content type = %q, want %q", got, "text/x-shellscript; charset=utf-8")
	}
	if got := rec.Header().Get("Content-Disposition"); got != "attachment; filename=\"pulse-setup-pbs.sh\"" {
		t.Fatalf("setup-script content disposition = %q, want %q", got, "attachment; filename=\"pulse-setup-pbs.sh\"")
	}
}

func TestContract_SetupScriptDerivesRenderedServerNameFromCanonicalHost(t *testing.T) {
	handlers := newTestConfigHandlers(t, &config.Config{
		DataPath:   t.TempDir(),
		ConfigPath: t.TempDir(),
	})

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/setup-script?type=pve&host=https://derived-pve.example.internal:8006&pulse_url=https://pulse.example.com:7655",
		nil,
	)
	rec := httptest.NewRecorder()

	handlers.HandleSetupScript(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	script := rec.Body.String()
	if !strings.Contains(script, "# Pulse Monitoring Setup Script for derived-pve.example.internal") {
		t.Fatalf("setup script missing derived canonical host label: %s", script)
	}
	if strings.Contains(script, "# Pulse Monitoring Setup Script for your-server") {
		t.Fatalf("setup script preserved placeholder server label for canonical host: %s", script)
	}
}

func TestContract_AssignProfileRejectsMissingProfile(t *testing.T) {
	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init default persistence: %v", err)
	}

	handler := NewConfigProfileHandler(mtp)
	body := bytes.NewBufferString(`{"agent_id":"agent-1","profile_id":"missing-profile"}`)
	req := httptest.NewRequest(http.MethodPost, "/assignments", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "Profile not found" {
		t.Fatalf("body = %q, want %q", got, "Profile not found")
	}
}

func TestContract_ResolveLoopbackAwarePublicBaseURLPreservesConfiguredHTTPS(t *testing.T) {
	cfg := &config.Config{
		PublicURL:    "https://public.example.com/base/",
		FrontendPort: 7655,
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "127.0.0.1:7655"

	if got := resolveLoopbackAwarePublicBaseURL(req, cfg); got != "https://public.example.com/base" {
		t.Fatalf("baseURL = %q, want %q", got, "https://public.example.com/base")
	}
}

func TestContract_CanonicalPulseMonitorTokenNamePrefersPulseURL(t *testing.T) {
	got := buildPulseMonitorTokenName("https://public.example.com/base", "127.0.0.1:7655")
	if got != "pulse-public-example-com" {
		t.Fatalf("tokenName = %q, want %q", got, "pulse-public-example-com")
	}
}

func TestContract_FilterRecoveryPointsForRollupsIncludesNormalizedFilters(t *testing.T) {
	verified := true
	unverified := false
	points := []recovery.RecoveryPoint{
		{
			ID:                "point-1",
			Provider:          recovery.ProviderKubernetes,
			Kind:              recovery.KindBackup,
			Mode:              recovery.ModeRemote,
			Outcome:           recovery.OutcomeSuccess,
			SubjectResourceID: "pod-1",
			Verified:          &verified,
			Display: &recovery.RecoveryPointDisplay{
				SubjectLabel:    "pod-1",
				SubjectType:     "pod",
				ItemType:        "pod",
				ClusterLabel:    "prod-cluster",
				NodeHostLabel:   "worker-1",
				NamespaceLabel:  "default",
				RepositoryLabel: "repo-a",
				IsWorkload:      true,
			},
		},
		{
			ID:                "point-2",
			Provider:          recovery.ProviderKubernetes,
			Kind:              recovery.KindBackup,
			Mode:              recovery.ModeRemote,
			Outcome:           recovery.OutcomeFailed,
			SubjectResourceID: "pod-2",
			Verified:          &unverified,
			Display: &recovery.RecoveryPointDisplay{
				SubjectLabel:   "pod-2",
				SubjectType:    "pod",
				ItemType:       "pod",
				ClusterLabel:   "other-cluster",
				NodeHostLabel:  "worker-2",
				NamespaceLabel: "kube-system",
				IsWorkload:     true,
			},
		},
	}

	filtered := filterRecoveryPointsForRollups(points, recovery.ListPointsOptions{
		Query:          "repo-a",
		ItemType:       "pod",
		ClusterLabel:   "prod-cluster",
		NodeHostLabel:  "worker-1",
		NamespaceLabel: "default",
		Verification:   "verified",
		WorkloadOnly:   true,
	})

	if len(filtered) != 1 {
		t.Fatalf("len(filtered) = %d, want 1", len(filtered))
	}
	if got := filtered[0].SubjectResourceID; got != "pod-1" {
		t.Fatalf("subjectResourceID = %q, want %q", got, "pod-1")
	}
}

func TestContract_ParseRecoveryPlatformQueryPrefersCanonicalPlatformAlias(t *testing.T) {
	t.Parallel()

	if got := parseRecoveryPlatformQuery(url.Values{
		"platform": []string{" truenas "},
		"provider": []string{"proxmox-pve"},
	}); got != recovery.Provider("truenas") {
		t.Fatalf("parseRecoveryPlatformQuery(platform first) = %q, want %q", got, "truenas")
	}

	if got := parseRecoveryPlatformQuery(url.Values{
		"provider": []string{" proxmox-pbs "},
	}); got != recovery.Provider("proxmox-pbs") {
		t.Fatalf("parseRecoveryPlatformQuery(provider fallback) = %q, want %q", got, "proxmox-pbs")
	}
}

func TestContract_RecoveryPointPayloadUsesCanonicalPlatformField(t *testing.T) {
	payload := buildRecoveryPointPayload(recovery.RecoveryPoint{
		ID:       "point-1",
		Provider: recovery.Provider("truenas"),
		Kind:     recovery.Kind("snapshot"),
		Mode:     recovery.Mode("snapshot"),
		Outcome:  recovery.Outcome("success"),
	})

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal recovery point payload: %v", err)
	}

	const want = `{
		"id":"point-1",
		"platform":"truenas",
		"provider":"truenas",
		"kind":"snapshot",
		"mode":"snapshot",
		"outcome":"success"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_RecoveryPointsMockPathReturnsCanonicalProviderBackedFixtures(t *testing.T) {
	previousEnabled := mock.IsMockEnabled()
	previousConfig := mock.GetConfig()
	t.Cleanup(func() {
		if err := mock.SetEnabled(false); err != nil {
			t.Errorf("disable mock mode: %v", err)
		}
		mock.SetMockConfig(previousConfig)
		if previousEnabled {
			if err := mock.SetEnabled(true); err != nil {
				t.Errorf("restore mock mode: %v", err)
			}
			mock.SetMockConfig(previousConfig)
		}
	})

	t.Setenv("PULSE_MOCK_NODES", "1")
	t.Setenv("PULSE_MOCK_VMS_PER_NODE", "0")
	t.Setenv("PULSE_MOCK_LXCS_PER_NODE", "0")
	t.Setenv("PULSE_MOCK_DOCKER_HOSTS", "0")
	t.Setenv("PULSE_MOCK_DOCKER_CONTAINERS", "0")
	t.Setenv("PULSE_MOCK_GENERIC_HOSTS", "0")
	t.Setenv("PULSE_MOCK_K8S_CLUSTERS", "0")
	t.Setenv("PULSE_MOCK_K8S_NODES", "0")
	t.Setenv("PULSE_MOCK_K8S_PODS", "0")
	t.Setenv("PULSE_MOCK_K8S_DEPLOYMENTS", "0")

	if err := mock.SetEnabled(false); err != nil {
		t.Fatalf("disable mock mode: %v", err)
	}
	if err := mock.SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/recovery/points?platform=truenas&limit=10", nil)
	rec := httptest.NewRecorder()

	NewRecoveryHandlers(nil).HandleListPoints(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp recoveryPointsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal recovery points response: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("expected provider-backed mock recovery points in response")
	}
	if resp.Data[0].Platform != recovery.Provider("truenas") {
		t.Fatalf("platform = %q, want %q", resp.Data[0].Platform, "truenas")
	}
	if resp.Data[0].Display == nil {
		t.Fatal("expected normalized recovery display payload on mock points response")
	}
}

func TestContract_RecoveryRollupPayloadUsesCanonicalPlatformsField(t *testing.T) {
	payload := buildRecoveryRollupPayload(recovery.ProtectionRollup{
		RollupID:    "rollup-1",
		LastOutcome: recovery.Outcome("success"),
		Providers: []recovery.Provider{
			recovery.Provider("proxmox-pbs"),
			recovery.Provider("truenas"),
		},
	})

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal recovery rollup payload: %v", err)
	}

	const want = `{
		"rollupId":"rollup-1",
		"lastOutcome":"success",
		"platforms":["proxmox-pbs","truenas"],
		"providers":["proxmox-pbs","truenas"]
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_BillingStateJSONSnapshot(t *testing.T) {
	payload := entitlements.BillingState{
		Capabilities:         []string{"relay", "mobile_app"},
		Limits:               map[string]int64{"max_monitored_systems": 10},
		MetersEnabled:        []string{"api_requests"},
		PlanVersion:          "cloud_starter",
		SubscriptionState:    entitlements.SubStateActive,
		StripeCustomerID:     "cus_123",
		StripeSubscriptionID: "sub_123",
		StripePriceID:        "price_123",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal billing state: %v", err)
	}

	const want = `{
		"capabilities":["relay","mobile_app"],
		"limits":{"max_monitored_systems":10},
		"meters_enabled":["api_requests"],
		"plan_version":"cloud_starter",
		"subscription_state":"active",
		"stripe_customer_id":"cus_123",
		"stripe_subscription_id":"sub_123",
		"stripe_price_id":"price_123"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_HostedTenantEntitlementsFallbackToDefaultBillingState(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init default persistence: %v", err)
	}
	if _, err := mtp.GetPersistence("t-tenant"); err != nil {
		t.Fatalf("init tenant persistence: %v", err)
	}

	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState("default", &entitlements.BillingState{
		Capabilities:      []string{pkglicensing.FeatureRelay, pkglicensing.FeatureRBAC},
		Limits:            map[string]int64{"max_monitored_systems": 50},
		PlanVersion:       "msp_starter",
		SubscriptionState: entitlements.SubStateActive,
	}); err != nil {
		t.Fatalf("save default billing state: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, true)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "t-tenant")
	req := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handlers.HandleEntitlements(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("entitlements status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload EntitlementPayload
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode entitlements payload: %v", err)
	}
	if payload.SubscriptionState != string(pkglicensing.SubStateActive) {
		t.Fatalf("subscription_state=%q, want %q", payload.SubscriptionState, pkglicensing.SubStateActive)
	}
	if !sliceContainsString(payload.Capabilities, pkglicensing.FeatureRelay) {
		t.Fatalf("expected hosted tenant payload to include %q from default hosted billing state", pkglicensing.FeatureRelay)
	}
	for _, limit := range payload.Limits {
		if limit.Key == pkglicensing.MaxMonitoredSystemsLicenseGateKey {
			t.Fatalf("expected retired max_monitored_systems limit to be omitted, got %+v", payload.Limits)
		}
	}
}

func TestContract_HostedTenantEntitlementRefreshFallsBackToDefaultBillingState(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init default persistence: %v", err)
	}
	if _, err := mtp.GetPersistence("t-tenant"); err != nil {
		t.Fatalf("init tenant persistence: %v", err)
	}

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	refreshServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/entitlements/refresh" {
			http.NotFound(w, r)
			return
		}
		var req hostedTrialLeaseRefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode refresh request: %v", err)
		}
		if req.OrgID != "default" {
			t.Fatalf("req.OrgID=%q, want %q", req.OrgID, "default")
		}
		if req.InstanceHost != "pulse.example.com" {
			t.Fatalf("req.InstanceHost=%q, want %q", req.InstanceHost, "pulse.example.com")
		}
		if req.EntitlementRefreshToken != "etr_hosted_default" {
			t.Fatalf("req.EntitlementRefreshToken=%q, want %q", req.EntitlementRefreshToken, "etr_hosted_default")
		}

		entitlementJWT, err := pkglicensing.SignEntitlementLeaseToken(priv, pkglicensing.EntitlementLeaseClaims{
			OrgID:             "default",
			InstanceHost:      "pulse.example.com",
			PlanVersion:       "msp_starter",
			SubscriptionState: pkglicensing.SubStateActive,
			Capabilities: []string{
				pkglicensing.FeatureRelay,
				pkglicensing.FeatureAIAutoFix,
			},
		})
		if err != nil {
			t.Fatalf("SignEntitlementLeaseToken: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hostedTrialLeaseRefreshResponse{
			EntitlementJWT: entitlementJWT,
		})
	}))
	defer refreshServer.Close()

	store := config.NewFileBillingStore(baseDir)
	expiredLease, err := pkglicensing.SignEntitlementLeaseToken(priv, pkglicensing.EntitlementLeaseClaims{
		OrgID:             "default",
		InstanceHost:      "pulse.example.com",
		PlanVersion:       "msp_starter",
		SubscriptionState: pkglicensing.SubStateActive,
		Capabilities:      []string{pkglicensing.FeatureRelay},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	})
	if err != nil {
		t.Fatalf("SignEntitlementLeaseToken(expired): %v", err)
	}
	if err := store.SaveBillingState("default", &entitlements.BillingState{
		EntitlementJWT:          expiredLease,
		EntitlementRefreshToken: "etr_hosted_default",
	}); err != nil {
		t.Fatalf("save default billing state: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, true, &config.Config{
		PublicURL:         "https://pulse.example.com",
		ProTrialSignupURL: refreshServer.URL + "/start-pro-trial",
	})

	refreshed, permanent, err := handlers.refreshHostedEntitlementLeaseOnce("t-tenant", nil)
	if err != nil {
		t.Fatalf("refreshHostedEntitlementLeaseOnce: %v", err)
	}
	if !refreshed || permanent {
		t.Fatalf("refreshed=%v permanent=%v, want refreshed=true permanent=false", refreshed, permanent)
	}

	state, err := store.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState(default): %v", err)
	}
	if state == nil {
		t.Fatal("expected default billing state after hosted tenant refresh")
	}
	if state.SubscriptionState != entitlements.SubStateActive {
		t.Fatalf("subscription_state=%q, want %q", state.SubscriptionState, entitlements.SubStateActive)
	}
	if state.PlanVersion != "msp_starter" {
		t.Fatalf("plan_version=%q, want %q", state.PlanVersion, "msp_starter")
	}
	if !sliceContainsString(state.Capabilities, pkglicensing.FeatureAIAutoFix) {
		t.Fatalf("expected default hosted billing state to include %q after tenant refresh, got %v", pkglicensing.FeatureAIAutoFix, state.Capabilities)
	}
}

func TestContract_HostedEntitlementVerifierBridgeUsesCompatibilityEnvAlias(t *testing.T) {
	if pkglicensing.HostedEntitlementPublicKeyEnvVar != pkglicensing.TrialActivationPublicKeyEnvVar {
		t.Fatalf("hosted entitlement verifier env=%q, want legacy env alias %q", pkglicensing.HostedEntitlementPublicKeyEnvVar, pkglicensing.TrialActivationPublicKeyEnvVar)
	}

	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv("PULSE_HOSTED_MODE", "true")
	t.Setenv(pkglicensing.HostedEntitlementPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	got, err := trialActivationPublicKeyFromLicensing()
	if err != nil {
		t.Fatalf("trialActivationPublicKeyFromLicensing: %v", err)
	}
	if !bytes.Equal(got, pub) {
		t.Fatal("hosted entitlement verifier bridge did not resolve the compatibility env key")
	}
}

func TestContract_EntitlementPayloadMonitoredSystemUsageJSONSnapshot(t *testing.T) {
	payload := buildEntitlementPayloadWithUsage(&licenseStatus{
		Valid:    true,
		Tier:     pkglicensing.TierPro,
		Features: append([]string(nil), pkglicensing.TierFeatures[pkglicensing.TierPro]...),
	}, string(pkglicensing.SubStateActive), entitlementUsageSnapshot{
		MonitoredSystems:          7,
		MonitoredSystemsAvailable: true,
		LegacyConnections: legacyConnectionCountsModel{
			ProxmoxNodes:       2,
			DockerHosts:        1,
			KubernetesClusters: 1,
		},
	}, nil)

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal entitlement payload: %v", err)
	}

	const want = `{
		"capabilities":["update_alerts","sso","advanced_sso","ai_patrol","relay","mobile_app","push_notifications","long_term_metrics","ai_alerts","ai_autofix","kubernetes_ai","agent_profiles","rbac","audit_logging","advanced_reporting"],
		"limits":[],
		"subscription_state":"active",
		"upgrade_reasons":[],
		"tier":"pro",
		"hosted_mode":false,
		"valid":true,
		"is_lifetime":false,
		"days_remaining":0,
		"trial_eligible":false,
		"max_history_days":90,
			"legacy_connections":{"proxmox_nodes":2,"docker_hosts":1,"kubernetes_clusters":1},
			"has_migration_gap":false
		}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_SelfHostedCommunityEntitlementsJSONSnapshot(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	handlers := NewLicenseHandlers(mtp, false)

	req := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).
		WithContext(context.WithValue(context.Background(), OrgIDContextKey, "default"))
	rec := httptest.NewRecorder()
	handlers.HandleEntitlements(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("entitlements status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload EntitlementPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode entitlements: %v", err)
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal entitlements payload: %v", err)
	}

	const want = `{
		"capabilities":["update_alerts","sso","advanced_sso","ai_patrol"],
		"limits":[],
		"subscription_state":"active",
		"upgrade_reasons":[
			{"key":"mobile_app","reason":"Get Relay so Pulse Mobile can pair with this instance for secure handoff and notifications.","action_url":"https://pulserelay.pro/pricing?utm_source=pulse\u0026utm_medium=app\u0026utm_campaign=upgrade\u0026feature=mobile_app"},
			{"key":"push_notifications","reason":"Get Relay so important alerts reach you immediately on mobile instead of waiting for you to reopen Pulse.","action_url":"https://pulserelay.pro/pricing?utm_source=pulse\u0026utm_medium=app\u0026utm_campaign=upgrade\u0026feature=push_notifications"},
			{"key":"relay","reason":"Get Relay so Pulse stays reachable securely from anywhere instead of only on the local dashboard.","action_url":"https://pulserelay.pro/pricing?utm_source=pulse\u0026utm_medium=app\u0026utm_campaign=upgrade\u0026feature=relay"},
			{"key":"long_term_metrics","reason":"Get Relay for 14 days of history, or Pro for 90 days, so you can see what changed before and after an incident.","action_url":"https://pulserelay.pro/pricing?utm_source=pulse\u0026utm_medium=app\u0026utm_campaign=upgrade\u0026feature=long_term_metrics"},
			{"key":"ai_autofix","reason":"Upgrade to Pro so Patrol can investigate issues, handle safe fixes within Patrol mode, and verify the outcome.","action_url":"https://pulserelay.pro/pricing?utm_source=pulse\u0026utm_medium=app\u0026utm_campaign=upgrade\u0026feature=ai_autofix"},
			{"key":"ai_alerts","reason":"Upgrade to Pro so Patrol can investigate issues instead of handing you a stack of symptoms.","action_url":"https://pulserelay.pro/pricing?utm_source=pulse\u0026utm_medium=app\u0026utm_campaign=upgrade\u0026feature=ai_alerts"},
			{"key":"rbac","reason":"Upgrade to Pro when more than one operator needs safe access boundaries around infrastructure changes.","action_url":"https://pulserelay.pro/pricing?utm_source=pulse\u0026utm_medium=app\u0026utm_campaign=upgrade\u0026feature=rbac"},
			{"key":"agent_profiles","reason":"Upgrade to Pro to standardize agent behavior across systems without reconfiguring every install by hand.","action_url":"https://pulserelay.pro/pricing?utm_source=pulse\u0026utm_medium=app\u0026utm_campaign=upgrade\u0026feature=agent_profiles"},
			{"key":"audit_logging","reason":"Upgrade to Pro to keep a trustworthy action trail for incident review, accountability, and compliance.","action_url":"https://pulserelay.pro/pricing?utm_source=pulse\u0026utm_medium=app\u0026utm_campaign=upgrade\u0026feature=audit_logging"},
			{"key":"advanced_reporting","reason":"Upgrade to Pro to turn live infrastructure state into shareable reports without manual screenshot work.","action_url":"https://pulserelay.pro/pricing?utm_source=pulse\u0026utm_medium=app\u0026utm_campaign=upgrade\u0026feature=advanced_reporting"}
		],
		"tier":"free",
		"hosted_mode":false,
		"valid":false,
		"is_lifetime":false,
		"days_remaining":0,
		"trial_eligible":false,
		"max_history_days":7,
		"overflow_days_remaining":14,
			"legacy_connections":{"proxmox_nodes":0,"docker_hosts":0,"kubernetes_clusters":0},
			"has_migration_gap":false,
			"runtime":{"build":"community","label":"Pulse Community runtime"}
		}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_SelfHostedCommunityRuntimeCapabilitiesJSONSnapshot(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	handlers := NewLicenseHandlers(mtp, false)

	req := httptest.NewRequest(http.MethodGet, "/api/license/runtime-capabilities", nil).
		WithContext(context.WithValue(context.Background(), OrgIDContextKey, "default"))
	rec := httptest.NewRecorder()
	handlers.HandleRuntimeCapabilities(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"runtime capabilities status=%d, want %d: %s",
			rec.Code,
			http.StatusOK,
			rec.Body.String(),
		)
	}

	var payload RuntimeCapabilitiesPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode runtime capabilities: %v", err)
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal runtime capabilities payload: %v", err)
	}

	const want = `{
		"capabilities":["update_alerts","sso","advanced_sso","ai_patrol"],
			"limits":[],
			"hosted_mode":false,
			"max_history_days":7,
			"runtime":{"build":"community","label":"Pulse Community runtime"}
		}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_LicenseHandlersPassRuntimeIdentityToActivationService(t *testing.T) {
	source, err := os.ReadFile("licensing_handlers.go")
	if err != nil {
		t.Fatalf("read licensing_handlers.go: %v", err)
	}
	text := string(source)
	if count := strings.Count(text, "SetRuntimeIdentity(h.currentRuntimeIdentity())"); count < 2 {
		t.Fatalf("expected cached and new tenant services to receive runtime identity, count=%d", count)
	}
}

func TestContract_EntitlementPayloadMonitoredSystemUsageUnavailableJSONSnapshot(t *testing.T) {
	payload := buildEntitlementPayloadWithUsage(&licenseStatus{
		Valid:       true,
		Tier:        pkglicensing.TierCloud,
		PlanVersion: "cloud_starter",
		Features:    append([]string(nil), pkglicensing.TierFeatures[pkglicensing.TierPro]...),
	}, string(pkglicensing.SubStateActive), entitlementUsageSnapshot{
		MonitoredSystemsUnavailableReason: "supplemental_inventory_unsettled",
	}, nil)

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal entitlement payload: %v", err)
	}

	const want = `{
		"capabilities":["update_alerts","sso","advanced_sso","ai_patrol","relay","mobile_app","push_notifications","long_term_metrics","ai_alerts","ai_autofix","kubernetes_ai","agent_profiles","rbac","audit_logging","advanced_reporting"],
		"limits":[],
		"subscription_state":"active",
		"upgrade_reasons":[],
		"plan_version":"cloud_starter",
		"tier":"cloud",
		"hosted_mode":false,
		"valid":true,
		"is_lifetime":false,
		"days_remaining":0,
		"trial_eligible":false,
		"max_history_days":90,
		"legacy_connections":{"proxmox_nodes":0,"docker_hosts":0,"kubernetes_clusters":0},
		"has_migration_gap":false
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_EntitlementPayloadLifetimeJSONSnapshot(t *testing.T) {
	payload := buildEntitlementPayloadWithUsage(&licenseStatus{
		Valid:       true,
		Tier:        pkglicensing.TierLifetime,
		PlanVersion: "v5_lifetime_grandfathered",
		IsLifetime:  true,
		Features:    append([]string(nil), pkglicensing.TierFeatures[pkglicensing.TierLifetime]...),
		MaxGuests:   0,
	}, string(pkglicensing.SubStateActive), entitlementUsageSnapshot{
		MonitoredSystems:          15,
		MonitoredSystemsAvailable: true,
		LegacyConnections: legacyConnectionCountsModel{
			ProxmoxNodes: 1,
			DockerHosts:  1,
		},
	}, nil)

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal entitlement payload: %v", err)
	}

	const want = `{
		"capabilities":["update_alerts","sso","advanced_sso","ai_patrol","relay","mobile_app","push_notifications","long_term_metrics","ai_alerts","ai_autofix","kubernetes_ai","agent_profiles","rbac","audit_logging","advanced_reporting"],
		"limits":[],
		"subscription_state":"active",
		"upgrade_reasons":[],
		"plan_version":"v5_lifetime_grandfathered",
		"tier":"lifetime",
		"hosted_mode":false,
		"valid":true,
		"is_lifetime":true,
		"days_remaining":0,
		"trial_eligible":false,
		"max_history_days":90,
			"legacy_connections":{"proxmox_nodes":1,"docker_hosts":1,"kubernetes_clusters":0},
			"has_migration_gap":false
		}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_EntitlementUsageSnapshotWaitsForSettledSupplementalInventory(t *testing.T) {
	provider := &contractSupplementalUsageProvider{}
	monitor := &monitoring.Monitor{}
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(nil))
	monitor.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, provider)

	handlers := &LicenseHandlers{monitor: monitor}

	usage := handlers.entitlementUsageSnapshot(context.Background())
	if usage.MonitoredSystemsAvailable {
		t.Fatalf("expected unsettled supplemental inventory to keep usage unavailable, got %+v", usage)
	}

	provider.settle(1)
	usage = handlers.entitlementUsageSnapshot(context.Background())
	if usage.MonitoredSystemsAvailable {
		t.Fatalf("expected stale store freshness to keep usage unavailable, got %+v", usage)
	}

	monitor.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, provider)
	usage = handlers.entitlementUsageSnapshot(context.Background())
	if !usage.MonitoredSystemsAvailable {
		t.Fatalf("expected usage to become available after canonical store rebuild, got %+v", usage)
	}
	if usage.MonitoredSystems != 1 {
		t.Fatalf("MonitoredSystems=%d, want 1", usage.MonitoredSystems)
	}
}

func TestContract_MissingLicensePublicKeyActivationErrorGuidesLocalBuilds(t *testing.T) {
	msg := userFriendlyActivationError(fmt.Errorf("validate license: %w: signature verification required", pkglicensing.ErrNoPublicKey))
	lower := strings.ToLower(msg)

	for _, want := range []string{
		"official pulse release tarball",
		"published docker image",
		"same license key",
	} {
		if !strings.Contains(lower, want) {
			t.Fatalf("activation error %q does not contain %q", msg, want)
		}
	}

	for _, forbidden := range []string{
		"temporarily unavailable",
		"try again later",
		"validate license:",
		"signature verification",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("activation error %q contains misleading/internal text %q", msg, forbidden)
		}
	}
}

func TestContract_LegacyMigrationFallbackStaysUncappedJSONSnapshot(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")
	const expectedClientVersion = "6.0.0-rc.1"

	grantJWT, grantPublicKey, err := licensetestsupport.GenerateGrantJWTForTesting(pkglicensing.GrantClaims{
		LicenseID: "lic_contract_floor",
		Tier:      "pro",
		PlanKey:   "legacy_migration_fallback",
		State:     "active",
		Features:  []string{"relay"},
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
		Email:     "contract-floor@example.com",
	})
	if err != nil {
		t.Fatalf("generate grant jwt: %v", err)
	}
	pkglicensing.SetPublicKey(grantPublicKey)
	t.Cleanup(func() { pkglicensing.SetPublicKey(nil) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/licenses/exchange" {
			t.Fatalf("path = %q, want /v1/licenses/exchange", r.URL.Path)
		}
		var exchangeReq pkglicensing.ExchangeLegacyLicenseRequest
		if err := json.NewDecoder(r.Body).Decode(&exchangeReq); err != nil {
			t.Fatalf("decode exchange request: %v", err)
		}
		if exchangeReq.ClientVersion != expectedClientVersion {
			t.Fatalf("exchange client version=%q, want %q", exchangeReq.ClientVersion, expectedClientVersion)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(pkglicensing.ActivateInstallationResponse{
			License: pkglicensing.ActivateResponseLicense{
				LicenseID: "lic_contract_floor",
				State:     "active",
				Tier:      "pro",
				Features:  []string{"relay"},
			},
			Installation: pkglicensing.ActivateResponseInstallation{
				InstallationID:    "inst_contract_floor",
				InstallationToken: "pit_live_contract_floor",
				Status:            "active",
			},
			Grant: pkglicensing.GrantEnvelope{
				JWT:       grantJWT,
				JTI:       "grant_contract_floor",
				ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()
	t.Setenv("PULSE_LICENSE_SERVER_URL", server.URL)

	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	cp, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("init default persistence: %v", err)
	}
	persistence, err := pkglicensing.NewPersistence(cp.GetConfigDir())
	if err != nil {
		t.Fatalf("new persistence: %v", err)
	}
	legacyJWT, err := licensetestsupport.GenerateLicenseForTesting("contract-floor@example.com", pkglicensing.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("generate test license: %v", err)
	}
	if err := persistence.Save(legacyJWT); err != nil {
		t.Fatalf("save legacy jwt: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, false)
	handlers.SetRuntimeVersion(expectedClientVersion)
	handlers.SetMonitors(buildGrandfatherFloorMonitor(23), nil)
	t.Cleanup(handlers.StopAllBackgroundLoops)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")

	statusReq := httptest.NewRequest(http.MethodGet, "/api/license/status", nil).WithContext(ctx)
	statusRec := httptest.NewRecorder()
	handlers.HandleLicenseStatus(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", statusRec.Code, http.StatusOK, statusRec.Body.String())
	}

	entReq := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
	entRec := httptest.NewRecorder()
	handlers.HandleEntitlements(entRec, entReq)
	if entRec.Code != http.StatusOK {
		t.Fatalf("entitlements status=%d, want %d: %s", entRec.Code, http.StatusOK, entRec.Body.String())
	}

	var status pkglicensing.LicenseStatus
	if err := json.Unmarshal(statusRec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	var payload EntitlementPayload
	if err := json.Unmarshal(entRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode entitlements: %v", err)
	}
	got, err := json.Marshal(struct {
		Status struct {
			Tier        pkglicensing.Tier `json:"tier"`
			PlanVersion string            `json:"plan_version"`
			Valid       bool              `json:"valid"`
		} `json:"status"`
		Entitlements struct {
			Tier              string                     `json:"tier"`
			PlanVersion       string                     `json:"plan_version"`
			SubscriptionState string                     `json:"subscription_state"`
			Limits            []pkglicensing.LimitStatus `json:"limits"`
		} `json:"entitlements"`
	}{
		Status: struct {
			Tier        pkglicensing.Tier `json:"tier"`
			PlanVersion string            `json:"plan_version"`
			Valid       bool              `json:"valid"`
		}{
			Tier:        status.Tier,
			PlanVersion: status.PlanVersion,
			Valid:       status.Valid,
		},
		Entitlements: struct {
			Tier              string                     `json:"tier"`
			PlanVersion       string                     `json:"plan_version"`
			SubscriptionState string                     `json:"subscription_state"`
			Limits            []pkglicensing.LimitStatus `json:"limits"`
		}{
			Tier:              payload.Tier,
			PlanVersion:       payload.PlanVersion,
			SubscriptionState: payload.SubscriptionState,
			Limits:            payload.Limits,
		},
	})
	if err != nil {
		t.Fatalf("marshal snapshot payload: %v", err)
	}

	const want = `{
		"status":{
			"tier":"pro",
			"plan_version":"legacy_migration_fallback",
			"valid":true
		},
		"entitlements":{
			"tier":"pro",
			"plan_version":"legacy_migration_fallback",
			"subscription_state":"active",
			"limits":[]
		}
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_HostedBillingStateFallbackJSONSnapshot(t *testing.T) {
	baseDir := t.TempDir()
	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState("default", &entitlements.BillingState{
		Capabilities:         []string{pkglicensing.FeatureRelay, pkglicensing.FeatureRBAC},
		Limits:               map[string]int64{"max_monitored_systems": 50},
		MetersEnabled:        []string{},
		PlanVersion:          "msp_starter",
		SubscriptionState:    entitlements.SubStateActive,
		StripeCustomerID:     "cus_hosted",
		StripeSubscriptionID: "sub_hosted",
		StripePriceID:        "price_hosted",
	}); err != nil {
		t.Fatalf("save default billing state: %v", err)
	}

	handlers := NewBillingStateHandlers(store, true)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/orgs/t-tenant/billing-state", nil)
	req.SetPathValue("id", "t-tenant")
	rec := httptest.NewRecorder()

	handlers.HandleGetBillingState(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	const want = `{
		"capabilities":["relay","rbac"],
			"limits":{},
		"meters_enabled":[],
		"plan_version":"msp_starter",
		"subscription_state":"active",
		"stripe_customer_id":"cus_hosted",
		"stripe_subscription_id":"sub_hosted",
		"stripe_price_id":"price_hosted"
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
}

func TestContract_MonitoredSystemUsageUnavailableIncludesReason(t *testing.T) {
	body := bytes.NewBufferString(`{
		"candidate":{
			"source":"proxmox",
			"name":"tower",
			"hostname":"tower.local",
			"host_url":"https://tower.local:8006"
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/license/monitored-system-ledger/preview", body)
	rec := httptest.NewRecorder()

	router := &Router{}
	router.handleMonitoredSystemLedgerPreview(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}

	var payload APIError
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode unavailable payload: %v", err)
	}
	payload = payload.NormalizeCollections()
	if payload.Code != "monitored_system_usage_unavailable" {
		t.Fatalf("code=%q, want monitored_system_usage_unavailable", payload.Code)
	}
	if got := payload.Details["reason"]; got != monitoring.MonitoredSystemUsageUnavailableMonitorState {
		t.Fatalf("details.reason=%q, want %q", got, monitoring.MonitoredSystemUsageUnavailableMonitorState)
	}
}

func TestContract_HostReportAdmissionPreservesRestartContinuityAtLimit(t *testing.T) {
	setMaxMonitoredSystemsLicenseForTests(t, 1)

	cfg := &config.Config{DataPath: t.TempDir()}
	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:      "agent-1",
			Version: "6.0.0-rc.1",
		},
		Host: agentshost.HostInfo{
			ID:        "machine-1",
			MachineID: "machine-1",
			Hostname:  "host-1.local",
			Platform:  "linux",
		},
		Timestamp: time.Now().UTC(),
	}

	postReport := func(t *testing.T, handler *UnifiedAgentHandlers, input agentshost.Report) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(input)
		if err != nil {
			t.Fatalf("marshal host report: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/report", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleReport(rec, req)
		return rec
	}

	handler, _ := newUnifiedAgentHandlers(t, cfg)
	initialRec := postReport(t, handler, report)
	if initialRec.Code != http.StatusOK {
		t.Fatalf("initial host report should pass, got %d: %s", initialRec.Code, initialRec.Body.String())
	}

	restartedHandler, _ := newUnifiedAgentHandlers(t, cfg)
	restartReport := report
	restartReport.Timestamp = report.Timestamp.Add(30 * time.Second)
	restartRec := postReport(t, restartedHandler, restartReport)
	if restartRec.Code != http.StatusOK {
		t.Fatalf("restart continuity report should stay admitted, got %d: %s", restartRec.Code, restartRec.Body.String())
	}

	newHostReport := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:      "agent-2",
			Version: "6.0.0-rc.1",
		},
		Host: agentshost.HostInfo{
			ID:        "machine-2",
			MachineID: "machine-2",
			Hostname:  "host-2.local",
			Platform:  "linux",
		},
		Timestamp: report.Timestamp.Add(time.Minute),
	}
	newHostRec := postReport(t, restartedHandler, newHostReport)
	if newHostRec.Code != http.StatusOK {
		t.Fatalf("new host should be admitted because monitored-system caps are retired, got %d: %s", newHostRec.Code, newHostRec.Body.String())
	}
}

func TestContract_PlatformConnectionWritesIgnoreUsageUnavailableWithCapsRetired(t *testing.T) {
	t.Run("truenas add", func(t *testing.T) {
		setTrueNASFeatureForTest(t, true)
		setMockModeForTest(t, false)
		setMaxMonitoredSystemsLicenseForTests(t, 1)

		handler, _, monitor := newTrueNASHandlersForTest(t, nil)
		bindUnavailableSupplementalUsageProviderForTest(
			t,
			monitor,
			unifiedresources.SourceTrueNAS,
			monitoring.MonitoredSystemUsageUnavailableSupplementalInventoryUnsettled,
		)

		body := marshalTrueNASRequest(t, map[string]any{
			"name":   "tower",
			"host":   "tower.local",
			"apiKey": "super-secret",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/truenas/connections", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleAdd(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 with monitored-system caps retired, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("vmware add", func(t *testing.T) {
		setVMwareFeatureForTest(t, true)
		setMockModeForTest(t, false)
		setMaxMonitoredSystemsLicenseForTests(t, 1)

		handler, _ := newVMwareHandlersForTest(t)
		monitor, _, _ := newTestMonitor(t)
		handler.getMonitor = func(context.Context) *monitoring.Monitor { return monitor }
		bindUnavailableSupplementalUsageProviderForTest(
			t,
			monitor,
			unifiedresources.SourceVMware,
			monitoring.MonitoredSystemUsageUnavailableSupplementalInventoryUnsettled,
		)

		previewRecordsCalled := false
		handler.previewRecords = func(context.Context, config.VMwareVCenterInstance) ([]unifiedresources.IngestRecord, error) {
			previewRecordsCalled = true
			return nil, nil
		}

		body := marshalVMwareRequest(t, map[string]any{
			"name":     "lab-vcenter",
			"host":     "vcsa.lab.local",
			"port":     443,
			"username": "administrator@vsphere.local",
			"password": "super-secret",
			"enabled":  true,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleAdd(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 with monitored-system caps retired, got %d: %s", rec.Code, rec.Body.String())
		}
		if previewRecordsCalled {
			t.Fatal("expected VMware write not to preview external inventory when monitored-system caps are retired")
		}
	})
}

func TestContract_DisabledPlatformConnectionWritesBypassUnavailableUsageGate(t *testing.T) {
	t.Run("truenas add", func(t *testing.T) {
		setTrueNASFeatureForTest(t, true)
		setMockModeForTest(t, false)
		setMaxMonitoredSystemsLicenseForTests(t, 1)

		handler, _, monitor := newTrueNASHandlersForTest(t, nil)
		bindUnavailableSupplementalUsageProviderForTest(
			t,
			monitor,
			unifiedresources.SourceTrueNAS,
			monitoring.MonitoredSystemUsageUnavailableSupplementalInventoryUnsettled,
		)

		body := marshalTrueNASRequest(t, map[string]any{
			"name":    "tower",
			"host":    "tower.local",
			"apiKey":  "super-secret",
			"enabled": false,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/truenas/connections", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleAdd(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
		}
	})

	t.Run("truenas update", func(t *testing.T) {
		setTrueNASFeatureForTest(t, true)
		setMockModeForTest(t, false)
		setMaxMonitoredSystemsLicenseForTests(t, 1)

		handler, persistence, monitor := newTrueNASHandlersForTest(t, nil)
		bindUnavailableSupplementalUsageProviderForTest(
			t,
			monitor,
			unifiedresources.SourceTrueNAS,
			monitoring.MonitoredSystemUsageUnavailableSupplementalInventoryRebuildPending,
		)
		if err := persistence.SaveTrueNASConfig([]config.TrueNASInstance{{
			ID:       "alpha",
			Name:     "archive",
			Host:     "archive.local",
			APIKey:   "super-secret",
			UseHTTPS: true,
			Enabled:  true,
		}}); err != nil {
			t.Fatalf("seed truenas config: %v", err)
		}

		body := marshalTrueNASRequest(t, map[string]any{
			"name":     "archive",
			"host":     "archive.local",
			"apiKey":   "********",
			"useHttps": true,
			"enabled":  false,
		})
		req := httptest.NewRequest(http.MethodPut, "/api/truenas/connections/alpha", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleUpdate(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})

	t.Run("vmware add", func(t *testing.T) {
		setVMwareFeatureForTest(t, true)
		setMockModeForTest(t, false)
		setMaxMonitoredSystemsLicenseForTests(t, 1)

		handler, _ := newVMwareHandlersForTest(t)
		monitor, _, _ := newTestMonitor(t)
		handler.getMonitor = func(context.Context) *monitoring.Monitor { return monitor }
		bindUnavailableSupplementalUsageProviderForTest(
			t,
			monitor,
			unifiedresources.SourceVMware,
			monitoring.MonitoredSystemUsageUnavailableSupplementalInventoryUnsettled,
		)

		previewRecordsCalled := false
		handler.previewRecords = func(context.Context, config.VMwareVCenterInstance) ([]unifiedresources.IngestRecord, error) {
			previewRecordsCalled = true
			return nil, nil
		}

		body := marshalVMwareRequest(t, map[string]any{
			"name":     "lab-vcenter",
			"host":     "vcsa.lab.local",
			"port":     443,
			"username": "administrator@vsphere.local",
			"password": "super-secret",
			"enabled":  false,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleAdd(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
		}
		if previewRecordsCalled {
			t.Fatal("expected disabled VMware add to bypass external inventory preview")
		}
	})

	t.Run("vmware update", func(t *testing.T) {
		setVMwareFeatureForTest(t, true)
		setMockModeForTest(t, false)
		setMaxMonitoredSystemsLicenseForTests(t, 1)

		handler, persistence := newVMwareHandlersForTest(t)
		monitor, _, _ := newTestMonitor(t)
		handler.getMonitor = func(context.Context) *monitoring.Monitor { return monitor }
		bindUnavailableSupplementalUsageProviderForTest(
			t,
			monitor,
			unifiedresources.SourceVMware,
			monitoring.MonitoredSystemUsageUnavailableSupplementalInventoryRebuildPending,
		)
		if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{{
			ID:                 "alpha",
			Name:               "vc-a",
			Host:               "vc-a.lab.local",
			Port:               443,
			Username:           "administrator@vsphere.local",
			Password:           "super-secret",
			InsecureSkipVerify: true,
			Enabled:            true,
		}}); err != nil {
			t.Fatalf("seed vmware config: %v", err)
		}

		previewRecordsCalled := false
		handler.previewRecords = func(context.Context, config.VMwareVCenterInstance) ([]unifiedresources.IngestRecord, error) {
			previewRecordsCalled = true
			return nil, nil
		}

		body := marshalVMwareRequest(t, map[string]any{
			"name":               "vc-a",
			"host":               "vc-b.lab.local",
			"port":               443,
			"username":           "administrator@vsphere.local",
			"password":           "********",
			"insecureSkipVerify": true,
			"enabled":            false,
		})
		req := httptest.NewRequest(http.MethodPut, "/api/vmware/connections/alpha", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleUpdate(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if previewRecordsCalled {
			t.Fatal("expected disabled VMware update to bypass external inventory preview")
		}
	})
}

func TestContract_PlatformConnectionPreviewPreservesCanonicalEnabledDefaults(t *testing.T) {
	t.Run("truenas new preview defaults omitted enabled to active", func(t *testing.T) {
		setTrueNASFeatureForTest(t, true)

		handler, _, monitor := newTrueNASHandlersForTest(t, nil)
		registry := unifiedresources.NewRegistry(nil)
		registry.IngestRecords(unifiedresources.SourceAgent, []unifiedresources.IngestRecord{
			{
				SourceID: "host-1",
				Resource: unifiedresources.Resource{
					ID:     "host-1",
					Type:   unifiedresources.ResourceTypeAgent,
					Name:   "tower.local",
					Status: unifiedresources.StatusOnline,
					Agent: &unifiedresources.AgentData{
						AgentID:   "agent-1",
						Hostname:  "tower.local",
						MachineID: "machine-1",
					},
					Identity: unifiedresources.ResourceIdentity{
						MachineID: "machine-1",
						Hostnames: []string{"tower.local"},
					},
				},
			},
		})
		setUnexportedField(t, monitor, "resourceStore", monitoring.ResourceStoreInterface(unifiedresources.NewMonitorAdapter(registry)))

		body := marshalTrueNASRequest(t, map[string]any{
			"name":   "tower",
			"host":   "tower.local",
			"apiKey": "super-secret",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/truenas/connections/preview", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandlePreviewConnection(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var preview MonitoredSystemLedgerPreviewResponse
		if err := json.NewDecoder(rec.Body).Decode(&preview); err != nil {
			t.Fatalf("decode preview response: %v", err)
		}
		if preview.Effect != "attaches_existing" {
			t.Fatalf("Effect = %q, want attaches_existing", preview.Effect)
		}
	})

	t.Run("vmware saved preview preserves stored enabled when omitted", func(t *testing.T) {
		setVMwareFeatureForTest(t, true)

		handler, persistence := newVMwareHandlersForTest(t)
		monitor, _, _ := newTestMonitor(t)
		handler.getMonitor = func(context.Context) *monitoring.Monitor { return monitor }
		if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{
			{
				ID:       "conn-1",
				Name:     "lab-vcenter",
				Host:     "vcsa.lab.local",
				Username: "administrator@vsphere.local",
				Password: "super-secret",
				Enabled:  true,
			},
		}); err != nil {
			t.Fatalf("seed vmware config: %v", err)
		}

		previewRecordsCalled := false
		handler.previewRecords = func(context.Context, config.VMwareVCenterInstance) ([]unifiedresources.IngestRecord, error) {
			previewRecordsCalled = true
			return []unifiedresources.IngestRecord{
				{
					SourceID: "vc-1:host:host-101",
					Resource: unifiedresources.Resource{
						Type:   unifiedresources.ResourceTypeAgent,
						Name:   "esxi-01.lab.local",
						Status: unifiedresources.StatusOnline,
						VMware: &unifiedresources.VMwareData{
							ConnectionID:    "conn-1",
							ConnectionName:  "lab-vcenter",
							ManagedObjectID: "host-101",
							EntityType:      "host",
							HostUUID:        "uuid-host-1",
						},
					},
					Identity: unifiedresources.ResourceIdentity{
						DMIUUID:   "uuid-host-1",
						Hostnames: []string{"esxi-01.lab.local"},
					},
				},
			}, nil
		}

		body := marshalVMwareRequest(t, map[string]any{
			"host":     "edited.lab.local",
			"username": "operator@vsphere.local",
			"password": "********",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/conn-1/preview", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandlePreviewSavedConnection(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !previewRecordsCalled {
			t.Fatal("expected saved VMware preview with omitted enabled to stay on the active preview path")
		}
	})
}

func TestContract_DemoModeCommercialSurfacePolicy(t *testing.T) {
	t.Run("hidden routes return not found", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("hidden commercial route should not reach downstream handler: %s %s", r.Method, r.URL.Path)
		})
		handler := DemoModeMiddleware(&config.Config{DemoMode: true}, next)

		testCases := []struct {
			method string
			path   string
		}{
			{method: http.MethodGet, path: "/api/license/status"},
			{method: http.MethodGet, path: "/api/license/features"},
			{method: http.MethodGet, path: "/api/license/commercial-posture"},
			{method: http.MethodGet, path: "/api/license/entitlements"},
			{method: http.MethodPost, path: "/api/license/activate"},
			{method: http.MethodPost, path: "/api/license/clear"},
			{method: http.MethodGet, path: "/api/license/monitored-system-ledger"},
			{method: http.MethodPost, path: "/api/license/monitored-system-ledger/explain"},
			{method: http.MethodPost, path: "/api/license/monitored-system-ledger/preview"},
			{method: http.MethodPost, path: "/api/truenas/connections/preview"},
			{method: http.MethodPost, path: "/api/truenas/connections/conn-1/preview"},
			{method: http.MethodPost, path: "/api/vmware/connections/preview"},
			{method: http.MethodPost, path: "/api/vmware/connections/conn-1/preview"},
			{method: http.MethodGet, path: "/api/admin/orgs/t-tenant/billing-state"},
			{method: http.MethodPut, path: "/api/admin/orgs/t-tenant/billing-state"},
			{method: http.MethodGet, path: "/api/diagnostics"},
			{method: http.MethodPost, path: "/api/diagnostics/docker/prepare-token"},
			{method: http.MethodGet, path: "/api/logs/stream"},
			{method: http.MethodGet, path: "/api/logs/download"},
			{method: http.MethodGet, path: "/api/logs/level"},
			{method: http.MethodPost, path: "/api/logs/level"},
			{method: http.MethodGet, path: "/api/admin/users"},
			{method: http.MethodGet, path: "/api/admin/users/"},
			{method: http.MethodHead, path: "/api/admin/users"},
			{method: http.MethodGet, path: "/api/discover"},
			{method: http.MethodGet, path: "/api/discover/"},
			{method: http.MethodHead, path: "/api/discover"},
			{method: http.MethodGet, path: licensePurchaseStartPath},
		}

		for _, tc := range testCases {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("%s %s status=%d, want %d: %s", tc.method, tc.path, rec.Code, http.StatusNotFound, rec.Body.String())
			}
		}
	})

	t.Run("runtime capabilities stay available but sanitize public demo limit details", func(t *testing.T) {
		t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

		handlers := createTestHandler(t)
		handlers.SetConfig(&config.Config{DemoMode: true})
		licenseKey, err := licensetestsupport.GenerateLicenseForTesting("contract-demo@example.com", pkglicensing.TierPro, 24*time.Hour)
		if err != nil {
			t.Fatalf("GenerateLicenseForTesting: %v", err)
		}
		if _, err := handlers.Service(context.Background()).Activate(licenseKey); err != nil {
			t.Fatalf("Activate() error = %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/license/runtime-capabilities", nil)
		rec := httptest.NewRecorder()
		handlers.HandleRuntimeCapabilities(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var payload RuntimeCapabilitiesPayload
		if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.Capabilities) == 0 {
			t.Fatalf("expected sanitized runtime capabilities to preserve capabilities, got %+v", payload)
		}
		if sliceContainsString(payload.Capabilities, pkglicensing.FeatureDemoFixtures) {
			t.Fatalf("public demo runtime capabilities leaked internal feature %q: %v", pkglicensing.FeatureDemoFixtures, payload.Capabilities)
		}
		for _, limit := range payload.Limits {
			if limit.Limit != 0 || limit.Current != 0 || limit.State != "ok" {
				t.Fatalf("sanitized limit=%+v, want limit=0 current=0 state=ok", limit)
			}
		}
	})
}

func TestContract_ReleaseDemoFixtureRuntimeGuardrailsRemainCanonical(t *testing.T) {
	requiredSnippets := map[string][]string{
		"licensing_handlers.go": {
			"func (h *LicenseHandlers) syncReleaseDemoFixtureRuntime(orgID string, service *licenseService) {",
			"if !shouldEnforceReleaseDemoFixtureRuntime() {",
			"authorized := h.demoFixturesAuthorized(service)",
			"mock.SetReleaseFixturesAuthorized(authorized)",
			"if !wantsMockFixturesFromEnv() {",
		},
		"release_demo_fixtures_dev.go": {
			"//go:build !release",
			"return false",
		},
		"release_demo_fixtures_release.go": {
			"//go:build release",
			"return true",
		},
	}

	for file, snippets := range requiredSnippets {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}
		source := string(data)
		for _, snippet := range snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must contain %q", file, snippet)
			}
		}
	}
}

func TestContract_SelfHostedPurchaseHandoffJSONSnapshot(t *testing.T) {
	handler := createTestHandler(t)
	handler.SetConfig(&config.Config{PublicURL: "https://pulse.example.com"})
	var capturedReq struct {
		Feature           string `json:"feature"`
		SuccessURL        string `json:"success_url"`
		CancelURL         string `json:"cancel_url"`
		PurchaseReturnJTI string `json:"purchase_return_jti"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/checkout/portal-handoff" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&capturedReq); err != nil {
			t.Fatalf("decode checkout portal handoff request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"portal_handoff_id": "cph_placeholder",
			"feature":           capturedReq.Feature,
		})
	}))
	defer server.Close()
	t.Setenv("PULSE_LICENSE_SERVER_URL", server.URL)

	req := httptest.NewRequest(
		http.MethodGet,
		"https://pulse.example.com"+licensePurchaseStartPath+
			"?feature=max_monitored_systems&utm_content=legacy-bookmark",
		nil,
	)
	rec := httptest.NewRecorder()

	handler.HandleCheckoutStart(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}

	location := rec.Header().Get("Location")
	redirectURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse redirect location: %v", err)
	}
	if redirectURL.Scheme != "https" || redirectURL.Host != "cloud.pulserelay.pro" || redirectURL.Path != "/portal" {
		t.Fatalf("redirect location=%q, want Pulse Account portal", location)
	}
	if got := redirectURL.Query().Get("feature"); got != "" {
		t.Fatalf("feature=%q, want omitted portal query", got)
	}
	if got := redirectURL.Query().Get("return_url"); got != "" {
		t.Fatalf("return_url=%q, want omitted portal query", got)
	}
	if got := redirectURL.Query().Get("purchase_return_token"); got != "" {
		t.Fatalf("purchase_return_token=%q, want omitted portal query", got)
	}
	if got := redirectURL.Query().Get("purchase_handoff_url"); got != "" {
		t.Fatalf("purchase_handoff_url=%q, want omitted portal query", got)
	}
	if got := redirectURL.Query().Get("portal_handoff_id"); got != "cph_placeholder" {
		t.Fatalf("portal_handoff_id=%q, want cph_placeholder", got)
	}

	activationURL, err := url.Parse(capturedReq.SuccessURL)
	if err != nil {
		t.Fatalf("parse success_url: %v", err)
	}
	returnToken := strings.TrimSpace(activationURL.Query().Get(licensePurchaseReturnTokenField))
	if returnToken == "" {
		t.Fatal("expected signed purchase_return_token in success_url")
	}
	for _, entitlementField := range []string{
		"entitlements",
		"runtime",
		"commercial_posture",
		"subscription_state",
		"tier",
		"plan_version",
	} {
		if got := activationURL.Query().Get(entitlementField); got != "" {
			t.Fatalf("success_url query %s=%q, want omitted checkout UX metadata only", entitlementField, got)
		}
	}
	signingKey, err := handler.purchaseReturnSigningKey()
	if err != nil {
		t.Fatalf("purchaseReturnSigningKey: %v", err)
	}
	claims, err := verifyPurchaseReturnTokenFromLicensing(returnToken, signingKey, "pulse.example.com", time.Now().UTC())
	if err != nil {
		t.Fatalf("verifyPurchaseReturnTokenFromLicensing: %v", err)
	}
	if claims.Feature != "self_hosted_plan" {
		t.Fatalf("claims.Feature=%q, want self_hosted_plan", claims.Feature)
	}
	if claims.OrgID != "default" {
		t.Fatalf("claims.OrgID=%q, want default", claims.OrgID)
	}
	if capturedReq.PurchaseReturnJTI != claims.ID {
		t.Fatalf("purchase_return_jti=%q, want %q", capturedReq.PurchaseReturnJTI, claims.ID)
	}
	activationQuery := activationURL.Query()
	activationQuery.Set(licensePurchaseReturnTokenField, "placeholder")
	activationURL.RawQuery = activationQuery.Encode()

	got, err := json.Marshal(struct {
		PortalUtmContent   string `json:"portal_utm_content"`
		PortalHandoffID    string `json:"portal_handoff_id"`
		IntentFeature      string `json:"intent_feature"`
		PurchaseReturnJTI  string `json:"purchase_return_jti"`
		SuccessURLTemplate string `json:"success_url_template"`
		CancelURL          string `json:"cancel_url"`
	}{
		PortalUtmContent:   redirectURL.Query().Get("utm_content"),
		PortalHandoffID:    redirectURL.Query().Get("portal_handoff_id"),
		IntentFeature:      capturedReq.Feature,
		PurchaseReturnJTI:  "placeholder",
		SuccessURLTemplate: activationURL.String(),
		CancelURL:          capturedReq.CancelURL,
	})
	if err != nil {
		t.Fatalf("marshal normalized handoff snapshot: %v", err)
	}

	const want = `{
		"portal_utm_content":"legacy-bookmark",
		"portal_handoff_id":"cph_placeholder",
		"intent_feature":"self_hosted_plan",
		"purchase_return_jti":"placeholder",
		"success_url_template":"https://pulse.example.com/auth/license-purchase-activate?purchase_return_token=placeholder\u0026session_id=%7BCHECKOUT_SESSION_ID%7D",
		"cancel_url":"https://pulse.example.com/settings/system/billing/plan?intent=self_hosted_plan\u0026purchase=cancelled"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_SelfHostedPurchaseHandoffUnavailableJSONSnapshot(t *testing.T) {
	handler := createTestHandler(t)
	handler.SetConfig(&config.Config{PublicURL: "https://pulse.example.com"})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/checkout/portal-handoff" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		http.Error(w, "portal handoff unavailable", http.StatusBadGateway)
	}))
	defer server.Close()
	t.Setenv("PULSE_LICENSE_SERVER_URL", server.URL)

	req := httptest.NewRequest(
		http.MethodGet,
		"https://pulse.example.com"+licensePurchaseStartPath+"?feature=max_monitored_systems",
		nil,
	)
	rec := httptest.NewRecorder()

	handler.HandleCheckoutStart(rec, req)

	got, err := json.MarshalIndent(struct {
		StatusCode          int  `json:"status_code"`
		TitlePresent        bool `json:"title_present"`
		RetryMessagePresent bool `json:"retry_message_present"`
		OwnedRedirectPath   bool `json:"owned_redirect_path"`
	}{
		StatusCode:          rec.Code,
		TitlePresent:        strings.Contains(rec.Body.String(), "Pulse Account unavailable"),
		RetryMessagePresent: strings.Contains(rec.Body.String(), "Retry from this instance in a moment"),
		OwnedRedirectPath:   strings.Contains(rec.Body.String(), "/settings/system/billing/plan?intent=self_hosted_plan&purchase=unavailable"),
	}, "", "\t")
	if err != nil {
		t.Fatalf("marshal unavailable handoff snapshot: %v", err)
	}

	const want = `{
		"status_code": 503,
		"title_present": true,
		"retry_message_present": true,
		"owned_redirect_path": true
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_SelfHostedPurchaseActivationBridgeCopyStaysPlanOwned(t *testing.T) {
	continueRec := httptest.NewRecorder()
	writeLicensePurchaseActivationContinuePage(
		continueRec,
		"session_contract",
		"token_contract",
		"self_hosted_plan",
		"portal_handoff_contract",
	)

	successRec := httptest.NewRecorder()
	writeLicensePurchaseActivationSuccessPage(successRec, "self_hosted_plan")

	failureRec := httptest.NewRecorder()
	writeLicensePurchaseActivationFailurePage(
		failureRec,
		http.StatusConflict,
		"self_hosted_plan",
		selfHostedBillingPurchaseExpired,
		"Reopen the upgrade flow from Plans in Pulse.",
	)

	got, err := json.MarshalIndent(struct {
		ContinueTitle       bool `json:"continue_title"`
		ContinueAction      bool `json:"continue_action"`
		ContinueLegacyBrand bool `json:"continue_legacy_brand"`
		SuccessTitle        bool `json:"success_title"`
		SuccessReturnLabel  bool `json:"success_return_label"`
		SuccessLegacyBrand  bool `json:"success_legacy_brand"`
		FailureTitle        bool `json:"failure_title"`
		FailureOpenLabel    bool `json:"failure_open_label"`
		FailureGuidance     bool `json:"failure_guidance"`
		FailureLegacyBrand  bool `json:"failure_legacy_brand"`
	}{
		ContinueTitle:       strings.Contains(continueRec.Body.String(), "Finalizing plan upgrade"),
		ContinueAction:      strings.Contains(continueRec.Body.String(), "Continue to Plans"),
		ContinueLegacyBrand: strings.Contains(continueRec.Body.String(), "Pulse Pro"),
		SuccessTitle:        strings.Contains(successRec.Body.String(), "Plan activated"),
		SuccessReturnLabel:  strings.Contains(successRec.Body.String(), "Return to Plans"),
		SuccessLegacyBrand:  strings.Contains(successRec.Body.String(), "Pulse Pro"),
		FailureTitle:        strings.Contains(failureRec.Body.String(), "Plan activation needs attention"),
		FailureOpenLabel:    strings.Contains(failureRec.Body.String(), "Open Plans"),
		FailureGuidance:     strings.Contains(failureRec.Body.String(), "Reopen the upgrade flow from Plans in Pulse."),
		FailureLegacyBrand:  strings.Contains(failureRec.Body.String(), "Pulse Pro"),
	}, "", "\t")
	if err != nil {
		t.Fatalf("marshal activation bridge snapshot: %v", err)
	}

	const want = `{
		"continue_title": true,
		"continue_action": true,
		"continue_legacy_brand": false,
		"success_title": true,
		"success_return_label": true,
		"success_legacy_brand": false,
		"failure_title": true,
		"failure_open_label": true,
		"failure_guidance": true,
		"failure_legacy_brand": false
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_SelfHostedPurchaseHandoffRejectsInsecureNonLoopbackCallbackURL(t *testing.T) {
	handler := createTestHandler(t)

	req := httptest.NewRequest(
		http.MethodGet,
		"http://192.168.1.25:7655"+licensePurchaseStartPath+"?feature=max_monitored_systems",
		nil,
	)
	req.Host = "192.168.1.25:7655"
	req.RemoteAddr = "192.168.1.50:54321"
	rec := httptest.NewRecorder()

	handler.HandleCheckoutStart(rec, req)

	got, err := json.MarshalIndent(struct {
		StatusCode           int  `json:"status_code"`
		TitlePresent         bool `json:"title_present"`
		SecureFailureMessage bool `json:"secure_failure_message"`
		OwnedRedirectPath    bool `json:"owned_redirect_path"`
	}{
		StatusCode:           rec.Code,
		TitlePresent:         strings.Contains(rec.Body.String(), "Pulse Account unavailable"),
		SecureFailureMessage: strings.Contains(rec.Body.String(), "Pulse could not open Pulse Account from this instance right now"),
		OwnedRedirectPath:    strings.Contains(rec.Body.String(), "/settings/system/billing/plan?intent=self_hosted_plan&purchase=unavailable"),
	}, "", "\t")
	if err != nil {
		t.Fatalf("marshal insecure handoff snapshot: %v", err)
	}

	const want = `{
		"status_code": 503,
		"title_present": true,
		"secure_failure_message": true,
		"owned_redirect_path": true
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_HandoffExchangeJSONSnapshot(t *testing.T) {
	key := []byte("test-handoff-key")
	configDir := t.TempDir()
	secretsDir := filepath.Join(configDir, "secrets")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, "handoff.key"), key, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	handler := HandleHandoffExchange(configDir)
	tenantID := "tenant-contract"
	saveHandoffTestOrganization(t, configDir, &models.Organization{
		ID:          tenantID,
		DisplayName: "Contract Tenant",
		Status:      models.OrgStatusActive,
		CreatedAt:   time.Now().UTC(),
		OwnerUserID: "operator.owner+mixed@pulserelay.pro",
	})
	t.Setenv("PULSE_TENANT_ID", "")
	token := signHandoffToken(t, key, cloudHandoffClaims{
		AccountID: "acct-contract",
		Email:     "Operator.Owner+Mixed@PulseRelay.Pro",
		Role:      "owner",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "jti-contract",
			Subject:   "user-contract",
			Issuer:    cloudHandoffIssuer,
			Audience:  jwt.ClaimStrings{tenantID},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/cloud/handoff/exchange?token="+token+"&format=json", nil)
	req.Host = tenantID + ".cloud.pulserelay.pro"
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Forwarded-Host", req.Host)
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()

	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	const want = `{
		"account_id":"acct-contract",
		"email":"operator.owner+mixed@pulserelay.pro",
		"exp":"placeholder",
		"jti":"jti-contract",
		"ok":true,
		"role":"owner",
		"tenant_id":"tenant-contract",
		"user_id":"user-contract"
	}`

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode handoff payload: %v", err)
	}
	if _, ok := payload["exp"].(string); !ok {
		t.Fatalf("exp missing or not a string: %+v", payload)
	}
	payload["exp"] = "placeholder"
	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal normalized handoff payload: %v", err)
	}

	assertJSONSnapshot(t, got, want)
}

func TestContract_TenantAIServiceAvoidsSnapshotProviderBridge(t *testing.T) {
	tmp := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tmp)

	defaultMonitor, _, _ := newTestMonitor(t)
	tenantAdapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	tenantMonitor := &monitoring.Monitor{}
	tenantMonitor.SetResourceStore(tenantAdapter)

	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{
		"default":  defaultMonitor,
		"tenant-1": tenantMonitor,
	})

	handler := NewAISettingsHandler(mtp, mtm, nil)
	handler.SetStateProvider(defaultMonitor)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")
	svc := handler.GetAIService(ctx)
	if svc == nil {
		t.Fatal("expected tenant AI service")
	}
	if svc.GetStateProvider() != nil {
		t.Fatal("expected tenant AI service to avoid snapshot provider bridge")
	}
	if svc.GetPatrolService() == nil {
		t.Fatal("expected tenant patrol service to initialize from canonical providers")
	}
}

func TestContract_TenantMonitorAdapterUsesMonitorPollingCadence(t *testing.T) {
	router := &Router{
		config: &config.Config{
			PVEPollingInterval: 15 * time.Second,
		},
		monitorResourceAdapters: make(map[string]*unifiedresources.MonitorAdapter),
	}
	tenantMonitor := &monitoring.Monitor{}
	tenantMonitor.SetOrgID("tenant-1")
	setUnexportedField(t, tenantMonitor, "config", &config.Config{
		PVEPollingInterval: 5 * time.Minute,
	})

	adapter := router.monitorAdapterForMonitor(tenantMonitor)
	if adapter == nil {
		t.Fatal("expected tenant monitor adapter")
	}

	seen := time.Now().UTC().Add(-90 * time.Second).Truncate(time.Millisecond)
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		VMs: []models.VM{{
			ID:       "cluster-a:pve-a:101",
			Name:     "db",
			Node:     "pve-a",
			Instance: "cluster-a",
			VMID:     101,
			Status:   "running",
			Type:     "qemu",
			LastSeen: seen,
		}},
	})

	vms := adapter.VMs()
	if len(vms) != 1 {
		t.Fatalf("VM count = %d, want 1", len(vms))
	}
	if vms[0].Status() != unifiedresources.StatusOnline {
		t.Fatalf("VM status = %q, want online", vms[0].Status())
	}
}

func TestContract_HandoffExchangeTargetPathIsSignedAndLocalOnly(t *testing.T) {
	source, err := os.ReadFile(filepath.Clean("cloud_handoff_handlers.go"))
	if err != nil {
		t.Fatalf("read cloud_handoff_handlers.go: %v", err)
	}
	text := string(source)

	for _, required := range []string{
		"TargetPath string `json:\"target_path,omitempty\"`",
		"sanitizeCloudHandoffTargetPath(claims.TargetPath)",
		"http.Redirect(w, r, targetPath, http.StatusTemporaryRedirect)",
		"strings.HasPrefix(raw, \"//\")",
		"parsed.IsAbs() || parsed.Host != \"\"",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("cloud handoff exchange must preserve signed local target-path contract: missing %q", required)
		}
	}

	for _, forbidden := range []string{
		"r.FormValue(\"target_path\")",
		"r.URL.Query().Get(\"target_path\")",
		"r.FormValue(\"return_to\")",
		"r.URL.Query().Get(\"return_to\")",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("cloud handoff exchange must not accept unsigned redirect targets: found %q", forbidden)
		}
	}
}

func TestContract_HostedCloudHandoffUsesExistingTenantOrganizationMembership(t *testing.T) {
	key := []byte("test-handoff-key")
	configDir := t.TempDir()
	resetSessionStoreForTests()
	t.Cleanup(resetSessionStoreForTests)
	resetCSRFStoreForTests()
	t.Cleanup(resetCSRFStoreForTests)
	InitSessionStore(configDir)
	InitCSRFStore(configDir)

	secretsDir := filepath.Join(configDir, "secrets")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatalf("mkdir secrets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, "handoff.key"), key, 0o600); err != nil {
		t.Fatalf("write handoff key: %v", err)
	}

	tenantID := "tenant-contract-membership"
	mtp := saveHandoffTestOrganization(t, configDir, &models.Organization{
		ID:          tenantID,
		DisplayName: "Contract Membership",
		Status:      models.OrgStatusActive,
		CreatedAt:   time.Now().UTC(),
		OwnerUserID: "legacy-owner@example.com",
		Members: []models.OrganizationMember{
			{UserID: "legacy-owner@example.com", Role: models.OrgRoleOwner, AddedAt: time.Now().UTC()},
			{UserID: "courtmanr@gmail.com", Role: models.OrgRoleOwner, AddedAt: time.Now().UTC()},
		},
	})

	token := signHandoffToken(t, key, cloudHandoffClaims{
		AccountID: "acct-contract-membership",
		Email:     "courtmanr@gmail.com",
		Role:      "owner",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "jti-contract-membership",
			Subject:   "user-contract-membership",
			Issuer:    cloudHandoffIssuer,
			Audience:  jwt.ClaimStrings{tenantID},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	form := url.Values{}
	form.Set("token", token)
	t.Setenv("PULSE_HOSTED_MODE", "true")
	t.Setenv("PULSE_TENANT_ID", "")
	t.Setenv("PULSE_PUBLIC_URL", "")
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/handoff/exchange?format=json", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Host = tenantID + ".cloud.pulserelay.pro"
	req.RemoteAddr = "198.51.100.20:1234"
	rec := httptest.NewRecorder()

	HandleHandoffExchange(configDir).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	org, err := mtp.LoadOrganization(tenantID)
	if err != nil {
		t.Fatalf("load organization: %v", err)
	}
	if org.OwnerUserID != "legacy-owner@example.com" {
		t.Fatalf("ownerUserID=%q, want %q", org.OwnerUserID, "legacy-owner@example.com")
	}
	if got := org.GetMemberRole("user-contract-membership"); got != models.OrgRoleOwner {
		t.Fatalf("stable member role=%q, want %q", got, models.OrgRoleOwner)
	}
	if got := org.Members[1].Email; got != "courtmanr@gmail.com" {
		t.Fatalf("canonicalized member email=%q, want %q", got, "courtmanr@gmail.com")
	}
}

func TestContract_HostedDirectCloudHandoffUsesExistingTenantMembership(t *testing.T) {
	key := []byte("test-direct-handoff-key")
	configDir := t.TempDir()
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)

	if err := os.WriteFile(filepath.Join(configDir, cloudauth.HandoffKeyFile), key, 0o600); err != nil {
		t.Fatalf("write direct handoff key: %v", err)
	}

	tenantID := "tenant-direct-contract"
	mtp := saveHandoffTestOrganization(t, configDir, &models.Organization{
		ID:          tenantID,
		DisplayName: "Direct Contract Membership",
		Status:      models.OrgStatusActive,
		CreatedAt:   time.Now().UTC(),
		OwnerUserID: "legacy-owner@example.com",
		Members: []models.OrganizationMember{
			{UserID: "legacy-owner@example.com", Role: models.OrgRoleOwner, AddedAt: time.Now().UTC()},
			{UserID: "courtmanr@gmail.com", Role: models.OrgRoleOwner, AddedAt: time.Now().UTC()},
		},
	})

	token, err := cloudauth.SignWithClaims(key, cloudauth.Claims{
		Email:     "courtmanr@gmail.com",
		TenantID:  tenantID,
		AccountID: "acct-direct-contract",
		UserID:    "user-direct-contract",
		Role:      "owner",
	}, time.Hour)
	if err != nil {
		t.Fatalf("sign direct handoff claims: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/cloud-handoff?token="+url.QueryEscape(token), nil)
	rec := httptest.NewRecorder()

	HandleCloudHandoff(configDir).ServeHTTP(rec, req)
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusTemporaryRedirect, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/" {
		t.Fatalf("redirect=%q, want %q", got, "/")
	}

	org, err := mtp.LoadOrganization(tenantID)
	if err != nil {
		t.Fatalf("load organization: %v", err)
	}
	if org.OwnerUserID != "legacy-owner@example.com" {
		t.Fatalf("ownerUserID=%q, want %q", org.OwnerUserID, "legacy-owner@example.com")
	}
	if got := org.GetMemberRole("user-direct-contract"); got != models.OrgRoleOwner {
		t.Fatalf("stable member role=%q, want %q", got, models.OrgRoleOwner)
	}
	if got := org.Members[1].Email; got != "courtmanr@gmail.com" {
		t.Fatalf("canonicalized member email=%q, want %q", got, "courtmanr@gmail.com")
	}
}

func TestContract_APITokenDeleteRejectsScopeEscalation(t *testing.T) {
	router := &Router{
		config: &config.Config{
			APITokens: []config.APITokenRecord{
				{
					ID:        "broad-token",
					Name:      "broad",
					Hash:      "hash-broad",
					CreatedAt: time.Now().Add(-time.Hour),
					Scopes:    []string{config.ScopeWildcard},
					OrgID:     "default",
				},
			},
		},
	}

	caller, err := config.NewAPITokenRecord(
		"limited-caller-token-123.12345678",
		"limited",
		[]string{config.ScopeSettingsWrite},
	)
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/security/tokens/broad-token", nil)
	req = req.WithContext(authpkg.WithAPIToken(req.Context(), caller))
	rec := httptest.NewRecorder()
	router.handleDeleteAPIToken(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), `Cannot delete token with scope "*"`) {
		t.Fatalf("expected delete scope-escalation contract message, got %q", rec.Body.String())
	}
	if len(router.config.APITokens) != 1 || router.config.APITokens[0].ID != "broad-token" {
		t.Fatalf("expected broader token to remain configured, got %+v", router.config.APITokens)
	}
}

func TestContract_OrganizationInvitationTransportJSONSnapshot(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)

	createReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs", bytes.NewBufferString(`{"id":"acme","displayName":"Acme"}`)),
		"alice",
	)
	createRec := httptest.NewRecorder()
	h.HandleCreateOrg(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create org: status=%d body=%s", createRec.Code, createRec.Body.String())
	}

	inviteReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/members", bytes.NewBufferString(`{"userId":"bob","role":"viewer"}`)),
		"alice",
	)
	inviteReq.SetPathValue("id", "acme")
	inviteRec := httptest.NewRecorder()
	h.HandleInviteMember(inviteRec, inviteReq)
	if inviteRec.Code != http.StatusAccepted {
		t.Fatalf("invite member: status=%d body=%s", inviteRec.Code, inviteRec.Body.String())
	}

	var invitePayload organizationAccessMutationResponse
	if err := json.Unmarshal(inviteRec.Body.Bytes(), &invitePayload); err != nil {
		t.Fatalf("decode invite payload: %v", err)
	}
	if invitePayload.Kind != "invitation" || invitePayload.Invitation == nil {
		t.Fatalf("unexpected invite payload: %+v", invitePayload)
	}
	if invitePayload.Invitation.InvitedAt.IsZero() {
		t.Fatalf("expected invite payload to include invitedAt")
	}

	inboxReq := withUser(httptest.NewRequest(http.MethodGet, "/api/org-invitations", nil), "bob")
	inboxRec := httptest.NewRecorder()
	h.HandleListMyInvitations(inboxRec, inboxReq)
	if inboxRec.Code != http.StatusOK {
		t.Fatalf("list my invitations: status=%d body=%s", inboxRec.Code, inboxRec.Body.String())
	}

	var inboxPayload []organizationUserInvitationResponse
	if err := json.Unmarshal(inboxRec.Body.Bytes(), &inboxPayload); err != nil {
		t.Fatalf("decode invitation inbox: %v", err)
	}
	if len(inboxPayload) != 1 {
		t.Fatalf("expected 1 invitation, got %d", len(inboxPayload))
	}
	if inboxPayload[0].InvitedAt.IsZero() {
		t.Fatalf("expected inbox payload to include invitedAt")
	}

	acceptReq := withUser(httptest.NewRequest(http.MethodPost, "/api/org-invitations/acme/accept", nil), "bob")
	acceptReq.SetPathValue("id", "acme")
	acceptRec := httptest.NewRecorder()
	h.HandleAcceptMyInvitation(acceptRec, acceptReq)
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("accept invitation: status=%d body=%s", acceptRec.Code, acceptRec.Body.String())
	}

	var acceptPayload organizationAccessMutationResponse
	if err := json.Unmarshal(acceptRec.Body.Bytes(), &acceptPayload); err != nil {
		t.Fatalf("decode accept payload: %v", err)
	}
	if acceptPayload.Kind != "member" || acceptPayload.Member == nil {
		t.Fatalf("unexpected accept payload: %+v", acceptPayload)
	}
	if acceptPayload.Member.AddedAt.IsZero() {
		t.Fatalf("expected accept payload to include addedAt")
	}

	got, err := json.Marshal(struct {
		Invite struct {
			Kind       string `json:"kind"`
			Invitation struct {
				UserID    string `json:"userId"`
				Role      string `json:"role"`
				InvitedAt string `json:"invitedAt"`
				InvitedBy string `json:"invitedBy"`
			} `json:"invitation"`
		} `json:"invite"`
		Inbox []struct {
			UserID         string `json:"userId"`
			Role           string `json:"role"`
			InvitedAt      string `json:"invitedAt"`
			InvitedBy      string `json:"invitedBy"`
			OrgID          string `json:"orgId"`
			OrgDisplayName string `json:"orgDisplayName"`
		} `json:"inbox"`
		Accept struct {
			Kind   string `json:"kind"`
			Member struct {
				UserID  string `json:"userId"`
				Role    string `json:"role"`
				AddedAt string `json:"addedAt"`
				AddedBy string `json:"addedBy"`
			} `json:"member"`
		} `json:"accept"`
	}{
		Invite: struct {
			Kind       string `json:"kind"`
			Invitation struct {
				UserID    string `json:"userId"`
				Role      string `json:"role"`
				InvitedAt string `json:"invitedAt"`
				InvitedBy string `json:"invitedBy"`
			} `json:"invitation"`
		}{
			Kind: invitePayload.Kind,
			Invitation: struct {
				UserID    string `json:"userId"`
				Role      string `json:"role"`
				InvitedAt string `json:"invitedAt"`
				InvitedBy string `json:"invitedBy"`
			}{
				UserID:    invitePayload.Invitation.UserID,
				Role:      string(invitePayload.Invitation.Role),
				InvitedAt: "placeholder",
				InvitedBy: invitePayload.Invitation.InvitedBy,
			},
		},
		Inbox: []struct {
			UserID         string `json:"userId"`
			Role           string `json:"role"`
			InvitedAt      string `json:"invitedAt"`
			InvitedBy      string `json:"invitedBy"`
			OrgID          string `json:"orgId"`
			OrgDisplayName string `json:"orgDisplayName"`
		}{
			{
				UserID:         inboxPayload[0].UserID,
				Role:           string(inboxPayload[0].Role),
				InvitedAt:      "placeholder",
				InvitedBy:      inboxPayload[0].InvitedBy,
				OrgID:          inboxPayload[0].OrgID,
				OrgDisplayName: inboxPayload[0].OrgDisplayName,
			},
		},
		Accept: struct {
			Kind   string `json:"kind"`
			Member struct {
				UserID  string `json:"userId"`
				Role    string `json:"role"`
				AddedAt string `json:"addedAt"`
				AddedBy string `json:"addedBy"`
			} `json:"member"`
		}{
			Kind: acceptPayload.Kind,
			Member: struct {
				UserID  string `json:"userId"`
				Role    string `json:"role"`
				AddedAt string `json:"addedAt"`
				AddedBy string `json:"addedBy"`
			}{
				UserID:  acceptPayload.Member.UserID,
				Role:    string(acceptPayload.Member.Role),
				AddedAt: "placeholder",
				AddedBy: acceptPayload.Member.AddedBy,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal normalized invitation transport snapshot: %v", err)
	}

	const want = `{
		"invite":{
			"kind":"invitation",
			"invitation":{"userId":"bob","role":"viewer","invitedAt":"placeholder","invitedBy":"alice"}
		},
		"inbox":[
			{"userId":"bob","role":"viewer","invitedAt":"placeholder","invitedBy":"alice","orgId":"acme","orgDisplayName":"Acme"}
		],
		"accept":{
			"kind":"member",
			"member":{"userId":"bob","role":"viewer","addedAt":"placeholder","addedBy":"alice"}
		}
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_OwnerTransferRequiresFreshSession(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	configDir := t.TempDir()
	resetSessionStoreForTests()
	t.Cleanup(resetSessionStoreForTests)
	InitSessionStore(configDir)

	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)

	createReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs", bytes.NewBufferString(`{"id":"acme","displayName":"Acme"}`)),
		"alice",
	)
	createRec := httptest.NewRecorder()
	h.HandleCreateOrg(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create org: status=%d body=%s", createRec.Code, createRec.Body.String())
	}

	inviteMemberForTest(t, h, "acme", "alice", "bob", "viewer", http.StatusAccepted)
	acceptInvitationForTest(t, h, "acme", "bob")

	sessionToken := generateSessionToken()
	store := GetSessionStore()
	store.CreateSession(sessionToken, time.Hour, "browser", "127.0.0.1", "alice")
	store.mu.Lock()
	store.sessions[sessionHash(sessionToken)].CreatedAt = time.Now().Add(-privilegedBrowserSessionMaxAge - time.Minute)
	store.mu.Unlock()

	transferReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/members", bytes.NewBufferString(`{"userId":"bob","role":"owner"}`)),
		"alice",
	)
	transferReq.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})
	transferReq.SetPathValue("id", "acme")
	transferRec := httptest.NewRecorder()
	h.HandleInviteMember(transferRec, transferReq)
	if transferRec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d: %s", transferRec.Code, http.StatusUnauthorized, transferRec.Body.String())
	}

	var payload APIError
	if err := json.Unmarshal(transferRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if payload.Code != "fresh_session_required" {
		t.Fatalf("code=%q, want fresh_session_required", payload.Code)
	}
	if payload.ErrorMessage != "Sign in again to transfer ownership" {
		t.Fatalf("error=%q, want %q", payload.ErrorMessage, "Sign in again to transfer ownership")
	}
	if payload.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status_code=%d, want %d", payload.StatusCode, http.StatusUnauthorized)
	}
}

func TestContract_OnboardingQRResponseJSONSnapshot(t *testing.T) {
	payload := onboardingQRResponse{
		Schema:      onboardingSchemaVersion,
		InstanceURL: "https://pulse.example.test",
		InstanceID:  "relay_abc123",
		Relay: onboardingRelayDetails{
			Enabled:             true,
			URL:                 "wss://relay.example.test/ws/app",
			IdentityFingerprint: "AA:BB:CC",
			IdentityPublicKey:   "base64-key",
		},
		AuthToken: "token-123",
		DeepLink:  "pulse://connect?schema=pulse-mobile-onboarding-v1&instance_url=https%3A%2F%2Fpulse.example.test&instance_id=relay_abc123&relay_url=wss%3A%2F%2Frelay.example.test%2Fws%2Fapp&auth_token=token-123&identity_fingerprint=AA%3ABB%3ACC&identity_public_key=base64-key",
	}.normalizeCollections()

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal onboarding qr response: %v", err)
	}

	const want = `{
		"schema":"pulse-mobile-onboarding-v1",
		"instance_url":"https://pulse.example.test",
		"instance_id":"relay_abc123",
		"relay":{"enabled":true,"url":"wss://relay.example.test/ws/app","identity_fingerprint":"AA:BB:CC","identity_public_key":"base64-key"},
		"auth_token":"token-123",
		"deep_link":"pulse://connect?schema=pulse-mobile-onboarding-v1\u0026instance_url=https%3A%2F%2Fpulse.example.test\u0026instance_id=relay_abc123\u0026relay_url=wss%3A%2F%2Frelay.example.test%2Fws%2Fapp\u0026auth_token=token-123\u0026identity_fingerprint=AA%3ABB%3ACC\u0026identity_public_key=base64-key",
		"diagnostics":[]
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_OnboardingQRResponseOmitsEmptyInstanceURLJSONSnapshot(t *testing.T) {
	payload := onboardingQRResponse{
		Schema:     onboardingSchemaVersion,
		InstanceID: "relay_abc123",
		Relay: onboardingRelayDetails{
			Enabled: true,
			URL:     "wss://relay.example.test/ws/app",
		},
		AuthToken: "token-123",
		DeepLink:  "pulse://connect?schema=pulse-mobile-onboarding-v1&instance_id=relay_abc123&relay_url=wss%3A%2F%2Frelay.example.test%2Fws%2Fapp&auth_token=token-123",
		Diagnostics: []onboardingDiagnostic{
			{
				Code:     "instance_url_not_https",
				Severity: "warning",
				Field:    "instance_url",
				Expected: "https://...",
				Received: "http://pulse.local.test",
				Message:  "Pulse web link is not included in this pairing code because the resolved Pulse URL is not HTTPS. Pairing can continue, but opening Pulse web from the phone requires an HTTPS Pulse URL.",
			},
		},
	}.normalizeCollections()

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal onboarding qr response without instance_url: %v", err)
	}

	const want = `{
		"schema":"pulse-mobile-onboarding-v1",
		"instance_id":"relay_abc123",
		"relay":{"enabled":true,"url":"wss://relay.example.test/ws/app"},
		"auth_token":"token-123",
		"deep_link":"pulse://connect?schema=pulse-mobile-onboarding-v1\u0026instance_id=relay_abc123\u0026relay_url=wss%3A%2F%2Frelay.example.test%2Fws%2Fapp\u0026auth_token=token-123",
		"diagnostics":[{"code":"instance_url_not_https","severity":"warning","message":"Pulse web link is not included in this pairing code because the resolved Pulse URL is not HTTPS. Pairing can continue, but opening Pulse web from the phone requires an HTTPS Pulse URL.","field":"instance_url","expected":"https://...","received":"http://pulse.local.test"}]
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_OnboardingNotReadyResponseJSONSnapshot(t *testing.T) {
	payload := onboardingNotReadyResponse{
		Code:    onboardingNotReadyCode,
		Error:   onboardingNotReadyMessage,
		Message: onboardingNotReadyMessage,
		Diagnostics: []onboardingDiagnostic{
			{
				Code:     "relay_registration_unavailable",
				Severity: "error",
				Field:    "instance_id",
				Message:  "Remote Access is enabled, but this Pulse instance is not connected to the relay yet. Wait for the status to show Connected before generating a mobile pairing code.",
			},
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal onboarding not-ready response: %v", err)
	}

	const want = `{
		"code":"onboarding_not_ready",
		"error":"Pulse Mobile pairing is not ready yet.",
		"message":"Pulse Mobile pairing is not ready yet.",
		"diagnostics":[{"code":"relay_registration_unavailable","severity":"error","message":"Remote Access is enabled, but this Pulse instance is not connected to the relay yet. Wait for the status to show Connected before generating a mobile pairing code.","field":"instance_id"}]
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_HostedRelayConfigResponseJSONSnapshot(t *testing.T) {
	router, _, instanceHost := newHostedRelayRuntimeTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/relay", nil)
	rec := httptest.NewRecorder()
	router.handleGetRelayConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	cfg, err := router.loadRelayConfigForRuntime(context.Background())
	if err != nil {
		t.Fatalf("loadRelayConfigForRuntime() error = %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, instanceHost) {
		t.Fatalf("relay config response leaked hosted instance secret %q: %s", instanceHost, body)
	}
	if strings.Contains(body, cfg.IdentityPrivateKey) {
		t.Fatalf("relay config response leaked identity private key: %s", body)
	}

	want := fmt.Sprintf(`{
		"enabled":true,
		"server_url":"%s",
		"identity_public_key":"%s",
		"identity_fingerprint":"%s"
	}`, relay.DefaultServerURL, cfg.IdentityPublicKey, cfg.IdentityFingerprint)

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
}

func TestContract_DiagnosticsInfoJSONSnapshot(t *testing.T) {
	payload := EmptyDiagnosticsInfo()

	got, err := json.Marshal(payload.NormalizeCollections())
	if err != nil {
		t.Fatalf("marshal diagnostics info: %v", err)
	}

	const want = `{
		"version":"",
		"runtime":"",
		"uptime":0,
		"nodes":[],
		"pbs":[],
		"system":{"os":"","arch":"","goVersion":"","numCPU":0,"numGoroutine":0,"memoryMB":0},
		"errors":[],
		"nodeSnapshots":[],
		"guestSnapshots":[],
		"memorySources":[],
		"memorySourceBreakdown":[]
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_UpdatePlanManualFallbackJSONSnapshot(t *testing.T) {
	payload := updates.UpdatePlan{
		CanAutoUpdate:   false,
		RequiresRoot:    false,
		RollbackSupport: true,
		EstimatedTime:   "5-10 minutes",
		Instructions: []string{
			"Check out or build Pulse 6.0.0-rc.1 in your development workspace.",
			"Stop the current development instance.",
			"Restart Pulse with the rebuilt binary or release artifact against the existing data directory.",
		},
		Prerequisites: []string{
			"A local development workspace for Pulse",
			"Build tooling for the target version",
			"A backup of the active data directory before replacing the binary",
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal update plan: %v", err)
	}

	const want = `{
		"canAutoUpdate":false,
		"instructions":["Check out or build Pulse 6.0.0-rc.1 in your development workspace.","Stop the current development instance.","Restart Pulse with the rebuilt binary or release artifact against the existing data directory."],
		"prerequisites":["A local development workspace for Pulse","Build tooling for the target version","A backup of the active data directory before replacing the binary"],
		"estimatedTime":"5-10 minutes",
		"requiresRoot":false,
		"rollbackSupport":true
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_EmptyUpdatePlanJSONSnapshot(t *testing.T) {
	payload := updates.EmptyUpdatePlan()

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal empty update plan: %v", err)
	}

	const want = `{
		"canAutoUpdate":false,
		"instructions":[],
		"prerequisites":[],
		"requiresRoot":false,
		"rollbackSupport":false
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_APITokenDTOJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC)
	lastUsed := now.Add(30 * time.Minute)
	expires := now.Add(24 * time.Hour)

	payload := apiTokenDTO{
		ID:          "token-1",
		Name:        "Deploy token",
		Prefix:      "pulse_",
		Suffix:      "1234",
		CreatedAt:   now,
		LastUsedAt:  &lastUsed,
		ExpiresAt:   &expires,
		Scopes:      []string{"monitoring:read", "settings:write"},
		OwnerUserID: "owner@example.com",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal API token dto: %v", err)
	}

	const want = `{
		"id":"token-1",
		"name":"Deploy token",
		"prefix":"pulse_",
		"suffix":"1234",
		"createdAt":"2026-02-08T13:14:15Z",
		"lastUsedAt":"2026-02-08T13:44:15Z",
		"expiresAt":"2026-02-09T13:14:15Z",
		"scopes":["monitoring:read","settings:write"],
		"ownerUserId":"owner@example.com"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_APITokenScopeAliasNormalization(t *testing.T) {
	raw := []string{"host-agent:report", "host-agent:config:read", "host-agent:manage", "host-agent:enroll"}
	got, err := normalizeRequestedScopes(&raw)
	if err != nil {
		t.Fatalf("normalize requested scopes: %v", err)
	}

	want := []string{
		config.ScopeAgentConfigRead,
		config.ScopeAgentEnroll,
		config.ScopeAgentManage,
		config.ScopeAgentReport,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized scopes = %#v, want %#v", got, want)
	}

	for _, legacy := range raw {
		if strings.HasPrefix(legacy, "agent:") {
			t.Fatalf("expected legacy alias input, got canonical scope %q", legacy)
		}
	}
}

func TestContract_HostedSubscriptionRequiredErrorJSONSnapshot(t *testing.T) {
	rec := httptest.NewRecorder()

	writeHostedSubscriptionRequiredError(rec)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusPaymentRequired)
	}

	const want = `{
		"error":"subscription_required",
		"message":"Your Cloud subscription is not active. Please check your billing status."
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
}

func TestContract_AgentBinaryReleaseAssetURL(t *testing.T) {
	router := &Router{serverVersion: "v6.0.0-rc.1"}

	got, err := router.agentBinaryReleaseAssetURL("linux-arm64")
	if err != nil {
		t.Fatalf("agent binary release asset URL: %v", err)
	}

	const want = "https://github.com/rcourtman/Pulse/releases/download/v6.0.0-rc.1/pulse-agent-linux-arm64"
	if got != want {
		t.Fatalf("agent binary release asset URL = %q, want %q", got, want)
	}
}

func TestContract_AgentBinaryReleaseAssetURLRejectsDevPrereleaseBuild(t *testing.T) {
	router := &Router{serverVersion: "v6.0.0-dev"}

	if _, err := router.agentBinaryReleaseAssetURL("linux-arm64"); err == nil {
		t.Fatalf("expected dev prerelease build to reject release asset lookup")
	}
}

func TestContract_ProxmoxInstallCommandIncludesInsecureForPlainHTTP(t *testing.T) {
	got := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            "http://pulse.example.com:7655/",
		Token:              "token-123",
		InstallType:        "pve",
		IncludeInstallType: true,
	})

	if !strings.Contains(got, "--url "+posixShellQuote("http://pulse.example.com:7655")) {
		t.Fatalf("install command missing canonical base URL: %s", got)
	}
	if !strings.Contains(got, "--insecure") {
		t.Fatalf("install command missing insecure flag for plain HTTP Pulse URL: %s", got)
	}
	if !strings.Contains(got, `--token-file "$token_file"`) {
		t.Fatalf("install command missing token-file transport: %s", got)
	}
}

func TestContract_ProxmoxInstallCommandUsesPrivilegeEscalationWrapper(t *testing.T) {
	got := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            "https://pulse.example.com/",
		Token:              "token-123",
		InstallType:        "pve",
		IncludeInstallType: true,
	})

	if !strings.Contains(got, `| { if [ "$(id -u)" -eq 0 ]; then bash -s --`) {
		t.Fatalf("install command missing root-or-sudo wrapper: %s", got)
	}
	if !strings.Contains(got, `sudo bash -s --`) {
		t.Fatalf("install command missing sudo fallback: %s", got)
	}
	if strings.Contains(got, "| bash -s -- --url") {
		t.Fatalf("install command preserved raw bash pipe instead of governed wrapper: %s", got)
	}
	if !strings.Contains(got, `rm -f "$token_file"`) {
		t.Fatalf("install command missing ephemeral token cleanup: %s", got)
	}
}

func TestContract_OptionalAuthProxmoxInstallCommandOmitsToken(t *testing.T) {
	got := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            "https://pulse.example.com/",
		Token:              "",
		InstallType:        "pve",
		IncludeInstallType: true,
	})

	if strings.Contains(got, "--token") {
		t.Fatalf("optional-auth install command preserved token flag: %s", got)
	}
	if !strings.Contains(got, "--url "+posixShellQuote("https://pulse.example.com")) {
		t.Fatalf("optional-auth install command missing canonical base URL: %s", got)
	}
}

func TestContract_ProxmoxInstallCommandNormalizesTrailingSlashBaseURL(t *testing.T) {
	got := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            "https://pulse.example.com/base///",
		Token:              "token-123",
		InstallType:        "pbs",
		IncludeInstallType: true,
	})

	if !strings.Contains(got, posixShellQuote("https://pulse.example.com/base/install.sh")) {
		t.Fatalf("install command missing normalized install script URL: %s", got)
	}
	if !strings.Contains(got, "--url "+posixShellQuote("https://pulse.example.com/base")) {
		t.Fatalf("install command missing normalized base URL: %s", got)
	}
	if !strings.Contains(got, `--token-file "$token_file"`) {
		t.Fatalf("install command missing token-file transport: %s", got)
	}
	if strings.Contains(got, "//install.sh") {
		t.Fatalf("install command preserved double-slash install path: %s", got)
	}
}

func TestContract_DownloadUnifiedAgentReleaseAssetIncludesSignatureHeader(t *testing.T) {
	router, tempDir := setupUnifiedAgentRouter(t)
	router.serverVersion = "v6.0.0"

	binContent := validTestUnifiedAgentBinary("linux-amd64")
	binPath := filepath.Join(tempDir, "bin", "pulse-agent-linux-amd64")
	if err := os.WriteFile(binPath, binContent, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	if err := os.WriteFile(binPath+".sig", []byte("signed-agent"), 0o644); err != nil {
		t.Fatalf("write signature: %v", err)
	}
	if err := os.WriteFile(binPath+".sshsig", []byte("signed-agent-ssh"), 0o644); err != nil {
		t.Fatalf("write ssh signature: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/install/agent?arch=linux-amd64", nil)
	w := httptest.NewRecorder()
	router.handleDownloadUnifiedAgent(w, req)

	if got := w.Header().Get(signatureHeaderName); got != "signed-agent" {
		t.Fatalf("agent download signature header = %q, want %q", got, "signed-agent")
	}
	if got := w.Header().Get(sshSignatureHeaderName); got != encodedTestSSHSignature("signed-agent-ssh") {
		t.Fatalf("agent download SSH signature header = %q, want %q", got, encodedTestSSHSignature("signed-agent-ssh"))
	}
}

func TestContract_DownloadInstallScriptReleaseAssetIncludesSignatureHeader(t *testing.T) {
	router, tempDir := setupUnifiedAgentRouter(t)
	router.serverVersion = "v6.0.0"

	scriptPath := filepath.Join(tempDir, "scripts", "install.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatalf("write install script: %v", err)
	}
	if err := os.WriteFile(scriptPath+".sig", []byte("signed-install-script"), 0o644); err != nil {
		t.Fatalf("write install signature: %v", err)
	}
	if err := os.WriteFile(scriptPath+".sshsig", []byte("signed-install-script-ssh"), 0o644); err != nil {
		t.Fatalf("write install SSH signature: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/install.sh", nil)
	w := httptest.NewRecorder()
	router.handleDownloadUnifiedInstallScript(w, req)

	if got := w.Header().Get(signatureHeaderName); got != "signed-install-script" {
		t.Fatalf("install script signature header = %q, want %q", got, "signed-install-script")
	}
	if got := w.Header().Get(sshSignatureHeaderName); got != encodedTestSSHSignature("signed-install-script-ssh") {
		t.Fatalf("install script SSH signature header = %q, want %q", got, encodedTestSSHSignature("signed-install-script-ssh"))
	}
}

func TestContract_SystemSettingsResponseJSONSnapshot(t *testing.T) {
	payload := EmptySystemSettingsResponse()
	payload.SystemSettings = config.SystemSettings{
		PVEPollingInterval:           30,
		PBSPollingInterval:           60,
		PMGPollingInterval:           60,
		BackupPollingInterval:        3600,
		UpdateChannel:                "rc",
		AutoUpdateEnabled:            false,
		AutoUpdateCheckInterval:      24,
		AutoUpdateTime:               "03:00",
		DiscoveryEnabled:             true,
		DiscoverySubnet:              "10.0.0.0/24",
		DiscoveryConfig:              config.DefaultDiscoveryConfig(),
		Theme:                        "dark",
		TemperatureMonitoringEnabled: true,
		DisableDockerUpdateActions:   true,
	}
	payload.EnvOverrides = map[string]bool{
		"PULSE_TELEMETRY": true,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal system settings response: %v", err)
	}

	const want = `{
		"pvePollingInterval":30,
		"pbsPollingInterval":60,
		"pmgPollingInterval":60,
		"backupPollingInterval":3600,
		"updateChannel":"rc",
		"autoUpdateEnabled":false,
		"autoUpdateCheckInterval":24,
		"autoUpdateTime":"03:00",
		"discoveryEnabled":true,
		"discoverySubnet":"10.0.0.0/24",
		"discoveryConfig":{
			"environment_override":"auto",
			"subnet_blocklist":["169.254.0.0/16"],
			"max_hosts_per_scan":1024,
			"max_concurrent":50,
			"enable_reverse_dns":true,
			"scan_gateways":true,
			"dial_timeout_ms":1000,
			"http_timeout_ms":2000
		},
		"theme":"dark",
		"fullWidthMode":false,
		"allowEmbedding":false,
		"temperatureMonitoringEnabled":true,
		"hideLocalLogin":false,
		"disableDockerUpdateActions":true,
		"envOverrides":{"PULSE_TELEMETRY":true}
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_CachedDiscoveryResponseJSONSnapshot(t *testing.T) {
	response := map[string]interface{}{
		"servers": []map[string]interface{}{
			{
				"ip":   "10.0.0.1",
				"port": 8006,
				"type": "pve",
			},
		},
		"errors": []string{
			"Docker bridge network [10.0.0.2:8007]: request timed out",
		},
		"structured_errors": []map[string]interface{}{
			{
				"ip":         "10.0.0.2",
				"port":       8007,
				"phase":      "docker_bridge_network",
				"error_type": "timeout",
				"message":    "request timed out",
				"timestamp":  "2023-11-14T22:13:20Z",
			},
		},
		"environment": nil,
		"cached":      true,
		"updated":     int64(1700000010),
		"age":         float64(0),
	}

	got, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal cached discovery response: %v", err)
	}

	const want = `{
		"age":0,
		"cached":true,
		"environment":null,
		"errors":["Docker bridge network [10.0.0.2:8007]: request timed out"],
		"servers":[{"ip":"10.0.0.1","port":8006,"type":"pve"}],
		"structured_errors":[{"error_type":"timeout","ip":"10.0.0.2","message":"request timed out","phase":"docker_bridge_network","port":8007,"timestamp":"2023-11-14T22:13:20Z"}],
		"updated":1700000010
	}`

	var wantValue interface{}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("unmarshal wanted discovery response: %v", err)
	}

	var gotValue interface{}
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("unmarshal got discovery response: %v", err)
	}

	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("cached discovery response mismatch\nwant: %s\ngot:  %s", want, string(got))
	}
}

func TestContract_DiscoveryCommandScanRoutesRequireSettingsWrite(t *testing.T) {
	monitoringToken := "discovery-command-monitoring-token-123.12345678"
	settingsToken := "discovery-command-settings-token-123.12345678"
	monitoringRecord := newTokenRecord(t, monitoringToken, []string{config.ScopeMonitoringWrite}, nil)
	settingsRecord := newTokenRecord(t, settingsToken, []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, monitoringRecord, settingsRecord)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/discovery/run",
		"/api/discovery/agent/host-1/resource-1",
		"/api/discovery/vm/host-1/resource-1",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("X-API-Token", monitoringToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected monitoring:write token to be forbidden for command-scan route %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeSettingsWrite) {
			t.Fatalf("expected command-scan route %s to require %q, got %q", path, config.ScopeSettingsWrite, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("X-API-Token", settingsToken)
		rec = httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code == http.StatusForbidden && strings.Contains(rec.Body.String(), config.ScopeSettingsWrite) {
			t.Fatalf("expected settings:write token to pass route scope for command-scan route %s, got %d: %s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestContract_WorkloadDiscoveryTriggerLeavesHostnameForServiceResolution(t *testing.T) {
	source, err := os.ReadFile(filepath.Clean("discovery_handlers.go"))
	if err != nil {
		t.Fatalf("read discovery handlers: %v", err)
	}
	text := string(source)

	for _, required := range []string{
		"Hostname:     reqBody.Hostname",
		"resourceType == servicediscovery.ResourceTypeAgent",
		"req.Hostname = targetID",
		"Workloads should resolve their own display name from state",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("discovery trigger lost workload hostname contract marker %q", required)
		}
	}
	if strings.Contains(text, "if req.Hostname == \"\" {\n\t\treq.Hostname = targetID") {
		t.Fatal("discovery trigger must not use the route target as a generic workload hostname fallback")
	}
}

func TestContract_AutoRegisterRequestJSONSnapshot(t *testing.T) {
	payload := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-homelab",
		TokenValue: "secret-token",
		ServerName: "pve-node-1",
		AuthToken:  "setup-token-123",
		Source:     "agent",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal auto-register request: %v", err)
	}

	const want = `{
		"type":"pve",
		"host":"https://pve.local:8006",
		"tokenId":"pulse-monitor@pve!pulse-homelab",
		"tokenValue":"secret-token",
		"serverName":"pve-node-1",
		"authToken":"setup-token-123",
		"source":"agent"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_AutoRegisterScriptRequestJSONSnapshot(t *testing.T) {
	payload := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-homelab",
		TokenValue: "secret-token",
		ServerName: "pve-node-1",
		AuthToken:  "setup-token-123",
		Source:     "script",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal script auto-register request: %v", err)
	}

	const want = `{
		"type":"pve",
		"host":"https://pve.local:8006",
		"tokenId":"pulse-monitor@pve!pulse-homelab",
		"tokenValue":"secret-token",
		"serverName":"pve-node-1",
		"authToken":"setup-token-123",
		"source":"script"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_AutoRegisterCheckRequestJSONSnapshot(t *testing.T) {
	payload := AutoRegisterRequest{
		Type:              "pve",
		Host:              "https://pve.local:8006",
		CandidateHosts:    []string{"https://pve.local:8006", "https://10.0.0.5:8006"},
		ServerName:        "pve-node-1",
		AuthToken:         "setup-token-123",
		Source:            "agent",
		CheckRegistration: true,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal auto-register check request: %v", err)
	}

	const want = `{
		"type":"pve",
		"host":"https://pve.local:8006",
		"candidateHosts":["https://pve.local:8006","https://10.0.0.5:8006"],
		"serverName":"pve-node-1",
		"authToken":"setup-token-123",
		"source":"agent",
		"checkRegistration":true
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_AutoUnregisterRequestJSONSnapshot(t *testing.T) {
	payload := AutoUnregisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-homelab",
		ServerName: "pve-node-1",
		AuthToken:  "setup-token-123",
		Source:     "script",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal auto-unregister request: %v", err)
	}

	const want = `{
		"type":"pve",
		"host":"https://pve.local:8006",
		"tokenId":"pulse-monitor@pve!pulse-homelab",
		"serverName":"pve-node-1",
		"authToken":"setup-token-123",
		"source":"script"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_AutoRegisterScriptRequestRequiresExplicitSourceMarker(t *testing.T) {
	payload := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-homelab",
		TokenValue: "secret-token",
		ServerName: "pve-node-1",
		AuthToken:  "setup-token-123",
		Source:     "script",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal script auto-register request: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("decode script auto-register request: %v", err)
	}
	if decoded["source"] != "script" {
		t.Fatalf("source = %#v, want explicit script marker", decoded["source"])
	}
}

func TestContract_CanonicalAutoRegisterRequiresExplicitServerName(t *testing.T) {
	payload := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-homelab",
		TokenValue: "secret-token",
		AuthToken:  "setup-token-123",
		Source:     "script",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal serverName-free auto-register request: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("decode serverName-free auto-register request: %v", err)
	}
	if _, ok := decoded["serverName"]; ok {
		t.Fatalf("serverName = %#v, want omitted when caller does not send it", decoded["serverName"])
	}
}

func TestContract_CanonicalAutoRegisterSetupTokenAuthFailureText(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	handler := newTestConfigHandlers(t, cfg)

	requestBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-homelab",
		TokenValue: "secret-token",
		ServerName: "pve-node-1",
		Source:     "script",
	}

	missingAuthJSON, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal missing-auth auto-register request: %v", err)
	}

	missingReq := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(missingAuthJSON))
	missingRec := httptest.NewRecorder()
	handler.HandleAutoRegister(missingRec, missingReq)
	if missingRec.Code != http.StatusUnauthorized {
		t.Fatalf("missing-auth status = %d, want 401", missingRec.Code)
	}
	if got := missingRec.Body.String(); got != "Pulse setup token required\n" {
		t.Fatalf("missing-auth body = %q, want canonical missing-setup-token guidance", got)
	}

	requestBody.AuthToken = "invalid-setup-token"
	invalidAuthJSON, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal invalid-auth auto-register request: %v", err)
	}

	invalidReq := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(invalidAuthJSON))
	invalidRec := httptest.NewRecorder()
	handler.HandleAutoRegister(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid-auth status = %d, want 401", invalidRec.Code)
	}
	if got := invalidRec.Body.String(); got != "Invalid or expired setup token\n" {
		t.Fatalf("invalid-auth body = %q, want canonical setup-token auth failure text", got)
	}

	const validSetupToken = "setup-token-123"
	tokenHash := authpkg.HashAPIToken(validSetupToken)
	handler.codeMutex.Lock()
	handler.setupTokens[tokenHash] = &SetupTokenRecord{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	requestBody.AuthToken = validSetupToken
	requestBody.TokenValue = ""
	mismatchedJSON, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal mismatched-completion auto-register request: %v", err)
	}

	mismatchedReq := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(mismatchedJSON))
	mismatchedRec := httptest.NewRecorder()
	handler.HandleAutoRegister(mismatchedRec, mismatchedReq)
	if mismatchedRec.Code != http.StatusBadRequest {
		t.Fatalf("mismatched-completion status = %d, want 400", mismatchedRec.Code)
	}
	if got := mismatchedRec.Body.String(); got != "tokenId and tokenValue must be provided together\n" {
		t.Fatalf("mismatched-completion body = %q, want canonical token-pair guidance", got)
	}
}

func TestContract_BootstrapTokenPersistenceJSONSnapshot(t *testing.T) {
	tempDir := t.TempDir()

	token, created, path, err := loadOrCreateBootstrapToken(tempDir)
	if err != nil {
		t.Fatalf("loadOrCreateBootstrapToken() error = %v", err)
	}
	if !created {
		t.Fatal("expected bootstrap token to be created")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted bootstrap token: %v", err)
	}
	snapshot := string(data)
	if !strings.Contains(snapshot, `"version":2`) {
		t.Fatalf("bootstrap token snapshot missing version: %s", snapshot)
	}
	if !strings.Contains(snapshot, `"token_ciphertext":"`) {
		t.Fatalf("bootstrap token snapshot missing ciphertext field: %s", snapshot)
	}
	if !strings.Contains(snapshot, `"token_hash":"`) {
		t.Fatalf("bootstrap token snapshot missing token hash field: %s", snapshot)
	}
	if strings.Contains(snapshot, token) {
		t.Fatalf("bootstrap token snapshot leaked raw token: %s", snapshot)
	}
}

func TestContract_InitializeBootstrapTokenLogsPathNotSecret(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{DataPath: tempDir}

	var logBuf bytes.Buffer
	logger := zerolog.New(&logBuf)
	router := &Router{config: cfg, eventLogger: &logger}

	router.initializeBootstrapToken()

	token, created, tokenPath, err := loadOrCreateBootstrapToken(tempDir)
	if err != nil {
		t.Fatalf("loadOrCreateBootstrapToken() error = %v", err)
	}
	if created {
		t.Fatal("expected bootstrap token to already exist after initialization")
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, tokenPath) {
		t.Fatalf("bootstrap token log missing path %q in %q", tokenPath, logOutput)
	}
	if !strings.Contains(logOutput, "pulse bootstrap-token") {
		t.Fatalf("bootstrap token log missing local reveal guidance: %q", logOutput)
	}
	if strings.Contains(logOutput, token) {
		t.Fatalf("bootstrap token leaked into logs: %q", logOutput)
	}
}

func TestContract_QuickSecuritySetupBootstrapRetrievalGuidance(t *testing.T) {
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)

	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	router := &Router{
		config:      cfg,
		persistence: config.NewConfigPersistence(cfg.DataPath),
	}
	router.initializeBootstrapToken()
	InitPersistentAuthStores(tempDir)

	handler := handleQuickSecuritySetupFixed(router)
	body := `{"username":"bootstrap","password":"StrongPass!1","apiToken":"` + strings.Repeat("aa", 32) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()

	authLimiter.Reset("127.0.0.1")
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("quick setup status = %d, want 401 (%s)", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); !strings.Contains(got, "pulse bootstrap-token") {
		t.Fatalf("quick setup guidance = %q, want pulse bootstrap-token retrieval guidance", got)
	}
	if got := rec.Body.String(); strings.Contains(got, ".bootstrap_token") {
		t.Fatalf("quick setup guidance = %q, want no raw .bootstrap_token scraping guidance", got)
	}
}

// A valid bootstrap token authorizes quick setup from any origin — the token
// is the security boundary, and it's only readable by callers with filesystem
// access to the Pulse data directory. The loopback-only path remains for the
// no-token fallback (legacy console flow).
func TestContract_QuickSecuritySetupValidBootstrapTokenAuthorizesAnyOrigin(t *testing.T) {
	body := `{"username":"bootstrap","password":"StrongPass!1","apiToken":"` + strings.Repeat("aa", 32) + `"}`

	newHandler := func(t *testing.T) (http.HandlerFunc, string) {
		t.Helper()
		tempDir := t.TempDir()
		cfg := &config.Config{
			DataPath:   tempDir,
			ConfigPath: tempDir,
		}
		router := &Router{
			config:      cfg,
			persistence: config.NewConfigPersistence(cfg.DataPath),
		}
		router.initializeBootstrapToken()
		InitPersistentAuthStores(tempDir)

		token, _, _, err := loadOrCreateBootstrapToken(tempDir)
		if err != nil {
			t.Fatalf("loadOrCreateBootstrapToken: %v", err)
		}
		return handleQuickSecuritySetupFixed(router), token
	}

	t.Run("remote IP with valid bootstrap token succeeds", func(t *testing.T) {
		handler, token := newHandler(t)

		req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(body))
		req.RemoteAddr = "198.51.100.41:54321"
		req.Header.Set(bootstrapTokenHeader, token)
		rec := httptest.NewRecorder()

		authLimiter.Reset("198.51.100.41")
		handler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("remote quick setup with bootstrap token status = %d, want 200 (%s)", rec.Code, rec.Body.String())
		}

		foundSessionCookie := false
		for _, cookie := range rec.Result().Cookies() {
			if cookie.Name == cookieNameSession || cookie.Name == cookieNameSessionSecure {
				foundSessionCookie = true
				break
			}
		}
		if !foundSessionCookie {
			t.Fatal("expected remote quick setup with bootstrap token to issue a session cookie")
		}
	})

	t.Run("remote IP without bootstrap token is rejected with bootstrap guidance", func(t *testing.T) {
		handler, _ := newHandler(t)

		req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(body))
		req.RemoteAddr = "198.51.100.41:54321"
		rec := httptest.NewRecorder()

		authLimiter.Reset("198.51.100.41")
		handler(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("remote quick setup without bootstrap token status = %d, want 403 (%s)", rec.Code, rec.Body.String())
		}
		if got := rec.Body.String(); !strings.Contains(strings.ToLower(got), "bootstrap") {
			t.Fatalf("remote no-token rejection guidance = %q, want bootstrap-token message", got)
		}
	})

	t.Run("loopback IP with valid bootstrap token succeeds", func(t *testing.T) {
		handler, token := newHandler(t)

		req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:54321"
		req.Header.Set(bootstrapTokenHeader, token)
		rec := httptest.NewRecorder()

		authLimiter.Reset("127.0.0.1")
		handler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("loopback quick setup status = %d, want 200 (%s)", rec.Code, rec.Body.String())
		}
	})
}

func TestContract_ResetFirstRunSecurityResponseJSONSnapshot(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("NODE_ENV", "")

	record := newTokenRecord(t, "contract-reset-first-run-token-123.12345678", []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed-password"

	envPath, err := writeAuthEnvFile(cfg.ConfigPath, cfg.DataPath, []byte("PULSE_AUTH_USER='admin'\n"))
	if err != nil {
		t.Fatalf("writeAuthEnvFile: %v", err)
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	req := httptest.NewRequest(http.MethodPost, "/api/security/dev/reset-first-run", nil)
	req.Header.Set("X-API-Token", "contract-reset-first-run-token-123.12345678")
	rec := httptest.NewRecorder()

	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("reset-first-run status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	var payload firstRunResetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode reset-first-run response: %v", err)
	}
	if strings.TrimSpace(payload.BootstrapToken) == "" {
		t.Fatal("reset-first-run response missing bootstrapToken")
	}
	if got, want := payload.BootstrapTokenPath, filepath.Join(cfg.DataPath, bootstrapTokenFilename); got != want {
		t.Fatalf("reset-first-run bootstrapTokenPath = %q, want %q", got, want)
	}
	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Fatalf("reset-first-run should remove auth env file, stat err = %v", err)
	}

	got, err := json.Marshal(firstRunResetResponse{
		BootstrapToken:     "bootstrap-token-placeholder",
		BootstrapTokenPath: "bootstrap-token-path-placeholder",
	})
	if err != nil {
		t.Fatalf("marshal reset-first-run response snapshot: %v", err)
	}

	const want = `{
		"bootstrapToken":"bootstrap-token-placeholder",
		"bootstrapTokenPath":"bootstrap-token-path-placeholder"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResetFirstRunSecurityClearsEnvBackedStatus(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("NODE_ENV", "")
	t.Setenv("PULSE_AUTH_USER", "admin")
	t.Setenv("PULSE_AUTH_PASS", "hashed-password")

	record := newTokenRecord(t, "contract-reset-first-run-token-456.12345678", []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed-password"

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	resetReq := httptest.NewRequest(http.MethodPost, "/api/security/dev/reset-first-run", nil)
	resetReq.Header.Set("X-API-Token", "contract-reset-first-run-token-456.12345678")
	resetRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(resetRec, resetReq)
	if resetRec.Code != http.StatusOK {
		t.Fatalf("reset-first-run status = %d, want 200 (%s)", resetRec.Code, resetRec.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	statusRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("security status = %d, want 200 (%s)", statusRec.Code, statusRec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(statusRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode security status payload: %v", err)
	}
	if got, _ := payload["hasAuthentication"].(bool); got {
		t.Fatalf("hasAuthentication = %v, want false", payload["hasAuthentication"])
	}
	if got, _ := payload["bootstrapTokenPath"].(string); strings.TrimSpace(got) == "" {
		t.Fatalf("bootstrapTokenPath = %v, want non-empty", payload["bootstrapTokenPath"])
	}
}

func TestContract_SetupScriptURLResponseJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"type":              "pve",
		"host":              "https://pve.local:8006",
		"url":               "https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006&pulse_url=https%3A%2F%2Fpulse.example&type=pve",
		"downloadURL":       "https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006&pulse_url=https%3A%2F%2Fpulse.example&setup_token=setup-token-123&type=pve",
		"scriptFileName":    "pulse-setup-pve.sh",
		"command":           "curl -fsSL 'https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006&pulse_url=https%3A%2F%2Fpulse.example&type=pve' | { if [ \"$(id -u)\" -eq 0 ]; then PULSE_SETUP_TOKEN='setup-token-123' bash; elif command -v sudo >/dev/null 2>&1; then sudo env PULSE_SETUP_TOKEN='setup-token-123' bash; else echo \"Root privileges required. Run as root (su -) and retry.\" >&2; exit 1; fi; }",
		"commandWithEnv":    "curl -fsSL 'https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006&pulse_url=https%3A%2F%2Fpulse.example&type=pve' | { if [ \"$(id -u)\" -eq 0 ]; then PULSE_SETUP_TOKEN='setup-token-123' bash; elif command -v sudo >/dev/null 2>&1; then sudo env PULSE_SETUP_TOKEN='setup-token-123' bash; else echo \"Root privileges required. Run as root (su -) and retry.\" >&2; exit 1; fi; }",
		"commandWithoutEnv": "curl -fsSL 'https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006&pulse_url=https%3A%2F%2Fpulse.example&type=pve' | { if [ \"$(id -u)\" -eq 0 ]; then bash; elif command -v sudo >/dev/null 2>&1; then sudo bash; else echo \"Root privileges required. Run as root (su -) and retry.\" >&2; exit 1; fi; }",
		"expires":           int64(1900000000),
		"setupToken":        "setup-token-123",
		"tokenHint":         "set…123",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal setup-script-url response: %v", err)
	}

	const want = `{
		"command":"curl -fsSL 'https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006\u0026pulse_url=https%3A%2F%2Fpulse.example\u0026type=pve' | { if [ \"$(id -u)\" -eq 0 ]; then PULSE_SETUP_TOKEN='setup-token-123' bash; elif command -v sudo \u003e/dev/null 2\u003e\u00261; then sudo env PULSE_SETUP_TOKEN='setup-token-123' bash; else echo \"Root privileges required. Run as root (su -) and retry.\" \u003e\u00262; exit 1; fi; }",
		"commandWithEnv":"curl -fsSL 'https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006\u0026pulse_url=https%3A%2F%2Fpulse.example\u0026type=pve' | { if [ \"$(id -u)\" -eq 0 ]; then PULSE_SETUP_TOKEN='setup-token-123' bash; elif command -v sudo \u003e/dev/null 2\u003e\u00261; then sudo env PULSE_SETUP_TOKEN='setup-token-123' bash; else echo \"Root privileges required. Run as root (su -) and retry.\" \u003e\u00262; exit 1; fi; }",
		"commandWithoutEnv":"curl -fsSL 'https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006\u0026pulse_url=https%3A%2F%2Fpulse.example\u0026type=pve' | { if [ \"$(id -u)\" -eq 0 ]; then bash; elif command -v sudo \u003e/dev/null 2\u003e\u00261; then sudo bash; else echo \"Root privileges required. Run as root (su -) and retry.\" \u003e\u00262; exit 1; fi; }",
		"downloadURL":"https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006\u0026pulse_url=https%3A%2F%2Fpulse.example\u0026setup_token=setup-token-123\u0026type=pve",
		"expires":1900000000,
		"host":"https://pve.local:8006",
		"scriptFileName":"pulse-setup-pve.sh",
		"setupToken":"setup-token-123",
		"tokenHint":"set…123",
		"type":"pve",
		"url":"https://pulse.example/api/setup-script?host=https%3A%2F%2Fpve.local%3A8006\u0026pulse_url=https%3A%2F%2Fpulse.example\u0026type=pve"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_SecurityStatusIncludesSessionCapabilitiesDemoMode(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.DemoMode = true

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("security status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode security status payload: %v", err)
	}

	sessionCapabilities, ok := payload["sessionCapabilities"].(map[string]any)
	if !ok {
		t.Fatalf("sessionCapabilities = %#v, want object", payload["sessionCapabilities"])
	}
	if got, _ := sessionCapabilities["demoMode"].(bool); !got {
		t.Fatalf("sessionCapabilities.demoMode = %v, want true", sessionCapabilities["demoMode"])
	}

	presentationPolicy, ok := payload["presentationPolicy"].(map[string]any)
	if !ok {
		t.Fatalf("presentationPolicy = %#v, want object", payload["presentationPolicy"])
	}
	for key, want := range map[string]bool{
		"demoMode":       true,
		"readOnly":       true,
		"hideCommercial": true,
		"hideUpgrade":    true,
	} {
		if got, _ := presentationPolicy[key].(bool); got != want {
			t.Fatalf("presentationPolicy.%s = %v, want %v", key, presentationPolicy[key], want)
		}
	}
}

func TestContract_SecurityStatusPresentationPolicyDefaultsHideUpgradeOutsideHosted(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.DemoMode = false

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("security status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode security status payload: %v", err)
	}

	presentationPolicy, ok := payload["presentationPolicy"].(map[string]any)
	if !ok {
		t.Fatalf("presentationPolicy = %#v, want object", payload["presentationPolicy"])
	}
	for key, want := range map[string]bool{
		"demoMode":       false,
		"readOnly":       false,
		"hideCommercial": false,
		"hideUpgrade":    true,
	} {
		if got, _ := presentationPolicy[key].(bool); got != want {
			t.Fatalf("presentationPolicy.%s = %v, want %v", key, presentationPolicy[key], want)
		}
	}
}

func TestContract_SecurityStatusSplitsAuditLogCapabilityFromSettingsRead(t *testing.T) {
	prevAuthorizer := authpkg.GetAuthorizer()
	authpkg.SetAuthorizer(&allowRulesAuthorizer{
		rules: map[string]bool{
			"read:audit_logs":  true,
			"admin:audit_logs": true,
		},
	})
	defer authpkg.SetAuthorizer(prevAuthorizer)

	cases := []struct {
		name                  string
		scopes                []string
		wantAuditLog          bool
		wantAuditWebhooksRead bool
	}{
		{
			name:                  "settings read stays limited to audit webhooks",
			scopes:                []string{config.ScopeSettingsRead},
			wantAuditLog:          false,
			wantAuditWebhooksRead: true,
		},
		{
			name:                  "audit read unlocks audit log capability",
			scopes:                []string{config.ScopeSettingsRead, config.ScopeAuditRead},
			wantAuditLog:          true,
			wantAuditWebhooksRead: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rawToken := fmt.Sprintf("contract-audit-scope-%s.12345678", strings.ReplaceAll(tc.name, " ", "-"))
			record := newTokenRecord(t, rawToken, tc.scopes, nil)
			cfg := newTestConfigWithTokens(t, record)
			router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

			req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
			req.Header.Set("X-API-Token", rawToken)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("security status = %d, want 200 (%s)", rec.Code, rec.Body.String())
			}

			var payload struct {
				SettingsCapabilities securityStatusSettingsCapabilities `json:"settingsCapabilities"`
				TokenScopes          []string                           `json:"tokenScopes"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode security status payload: %v", err)
			}

			if payload.SettingsCapabilities.AuditLog != tc.wantAuditLog {
				t.Fatalf("settingsCapabilities.auditLog = %v, want %v", payload.SettingsCapabilities.AuditLog, tc.wantAuditLog)
			}
			if payload.SettingsCapabilities.AuditWebhooksRead != tc.wantAuditWebhooksRead {
				t.Fatalf(
					"settingsCapabilities.auditWebhooksRead = %v, want %v",
					payload.SettingsCapabilities.AuditWebhooksRead,
					tc.wantAuditWebhooksRead,
				)
			}
			if !reflect.DeepEqual(payload.TokenScopes, tc.scopes) {
				t.Fatalf("tokenScopes = %#v, want %#v", payload.TokenScopes, tc.scopes)
			}
		})
	}
}

func TestContract_SetupScriptURLRejectsNonCanonicalRequestJSON(t *testing.T) {
	handler := newTestConfigHandlers(t, &config.Config{DataPath: t.TempDir()})

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/setup-script-url",
		bytes.NewBufferString(`{"type":"pve","host":"pve.local","setupToken":"unexpected"}`),
	)
	rec := httptest.NewRecorder()

	handler.HandleSetupScriptURL(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := rec.Body.String(); got != "Invalid request\n" {
		t.Fatalf("body = %q, want canonical invalid-request guidance", got)
	}
}

func TestContract_SetupBootstrapRejectsPBSBackupPerms(t *testing.T) {
	handler := newTestConfigHandlers(t, &config.Config{DataPath: t.TempDir()})

	setupScriptReq := httptest.NewRequest(
		http.MethodGet,
		"/api/setup-script?type=pbs&host=https://pbs.local:8007&pulse_url=https://pulse.example&backup_perms=true",
		nil,
	)
	setupScriptRec := httptest.NewRecorder()
	handler.HandleSetupScript(setupScriptRec, setupScriptReq)
	if setupScriptRec.Code != http.StatusBadRequest {
		t.Fatalf("setup-script status = %d, want 400", setupScriptRec.Code)
	}
	if got := setupScriptRec.Body.String(); got != "backup_perms is only supported for type 'pve'\n" {
		t.Fatalf("setup-script body = %q, want canonical backup-perms guidance", got)
	}

	setupURLReq := httptest.NewRequest(
		http.MethodPost,
		"/api/setup-script-url",
		bytes.NewBufferString(`{"type":"pbs","host":"pbs.local","backupPerms":true}`),
	)
	setupURLRec := httptest.NewRecorder()
	handler.HandleSetupScriptURL(setupURLRec, setupURLReq)
	if setupURLRec.Code != http.StatusBadRequest {
		t.Fatalf("setup-script-url status = %d, want 400", setupURLRec.Code)
	}
	if got := setupURLRec.Body.String(); got != "backupPerms is only supported for type 'pve'\n" {
		t.Fatalf("setup-script-url body = %q, want canonical backup-perms guidance", got)
	}
}

func TestContract_CanonicalAutoRegisterSourceContract(t *testing.T) {
	if !isCanonicalAutoRegisterSource("agent") {
		t.Fatalf("agent source should be accepted")
	}
	if !isCanonicalAutoRegisterSource("script") {
		t.Fatalf("script source should be accepted")
	}
	if isCanonicalAutoRegisterSource("manual") {
		t.Fatalf("manual source should be rejected")
	}
}

func TestContract_CanonicalAutoRegisterTypeContract(t *testing.T) {
	if !isCanonicalAutoRegisterType("pve") {
		t.Fatalf("pve type should be accepted")
	}
	if !isCanonicalAutoRegisterType("pbs") {
		t.Fatalf("pbs type should be accepted")
	}
	if isCanonicalAutoRegisterType("pmg") {
		t.Fatalf("pmg type should be rejected")
	}
}

func TestContract_CanonicalAutoRegisterTokenIDContract(t *testing.T) {
	if !isCanonicalAutoRegisterTokenID("pve", "pulse-monitor@pve!pulse-homelab") {
		t.Fatalf("canonical pve token id should be accepted")
	}
	if !isCanonicalAutoRegisterTokenID("pbs", "pulse-monitor@pbs!pulse-backup") {
		t.Fatalf("canonical pbs token id should be accepted")
	}
	if isCanonicalAutoRegisterTokenID("pve", "pulse-monitor@pve!token") {
		t.Fatalf("non-pulse-managed pve token suffix should be rejected")
	}
	if isCanonicalAutoRegisterTokenID("pve", "pulse@pve!token") {
		t.Fatalf("non-canonical token id should be rejected")
	}
	if isCanonicalAutoRegisterTokenID("pve", "pulse-monitor@pbs!pulse-backup") {
		t.Fatalf("cross-type token id should be rejected")
	}
	if isCanonicalAutoRegisterTokenID("pve", "pulse-monitor@pve!") {
		t.Fatalf("empty canonical token suffix should be rejected")
	}
	if isCanonicalAutoRegisterTokenID("pve", "pulse-monitor@pve!pulse-") {
		t.Fatalf("empty pulse-managed token slug should be rejected")
	}
}

func TestContract_CanonicalAutoRegisterMatchMessageContract(t *testing.T) {
	if got := canonicalAutoRegisterMatchMessage("resolved host identity"); got != "Canonical auto-register matched existing node by resolved host identity" {
		t.Fatalf("resolved-host message = %q", got)
	}
	if got := canonicalAutoRegisterMatchMessage("DHCP continuity token identity"); got != "Canonical auto-register matched existing node by DHCP continuity token identity" {
		t.Fatalf("dhcp message = %q", got)
	}
	if got := canonicalAutoRegisterMatchMessage("host; updated token in-place"); got != "Canonical auto-register matched existing node by host; updated token in-place" {
		t.Fatalf("host-update message = %q", got)
	}
	if strings.Contains(canonicalAutoRegisterMatchMessage("resolved host identity"), "Secure auto-register") {
		t.Fatalf("canonical match message must not preserve secure auto-register wording")
	}
}

func TestContract_CanonicalAutoRegisterCompletionPayloadMessageContract(t *testing.T) {
	if got := canonicalAutoRegisterCompletionPayloadMessage(); got != "Incomplete canonical auto-register token completion payload" {
		t.Fatalf("completion-payload message = %q", got)
	}
	if strings.Contains(canonicalAutoRegisterCompletionPayloadMessage(), "secure token completion") {
		t.Fatalf("canonical completion-payload message must not preserve secure wording")
	}
}

func TestContract_CanonicalAutoRegisterMissingFieldsMessageContract(t *testing.T) {
	if got := canonicalAutoRegisterMissingFieldsMessage("", "", false, ""); got != "Missing required canonical auto-register fields: type, host, tokenId/tokenValue, serverName" {
		t.Fatalf("all-missing message = %q", got)
	}
	if got := canonicalAutoRegisterMissingFieldsMessage("pve", "https://pve.local:8006", true, ""); got != "Missing required canonical auto-register fields: serverName" {
		t.Fatalf("serverName-only message = %q", got)
	}
}

func TestContract_CanonicalAutoUnregisterMissingFieldsMessageContract(t *testing.T) {
	if got := canonicalAutoUnregisterMissingFieldsMessage("", "", ""); got != "Missing required canonical auto-unregister fields: type, host, serverName" {
		t.Fatalf("all-missing auto-unregister message = %q", got)
	}
	if got := canonicalAutoUnregisterMissingFieldsMessage("pve", "https://pve.local:8006", ""); got != "Missing required canonical auto-unregister fields: serverName" {
		t.Fatalf("serverName-only auto-unregister message = %q", got)
	}
}

func TestContract_CanonicalAutoRegisterDirectValidationContract(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		Source:     "script",
		TokenID:    "pulse-monitor@pve!pulse-homelab",
		TokenValue: "secret-token",
	}

	missingServerJSON, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal missing-serverName canonical request: %v", err)
	}

	missingServerReq := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(missingServerJSON))
	missingServerRec := httptest.NewRecorder()
	handler.handleCanonicalAutoRegister(missingServerRec, missingServerReq, &reqBody, "127.0.0.1")
	if missingServerRec.Code != http.StatusBadRequest {
		t.Fatalf("missing-serverName status = %d, want 400", missingServerRec.Code)
	}
	if got := missingServerRec.Body.String(); got != "Missing required canonical auto-register fields: serverName\n" {
		t.Fatalf("missing-serverName body = %q, want canonical missing-field guidance", got)
	}

	reqBody.ServerName = "pve-node-1"
	reqBody.TokenValue = ""
	mismatchedReq := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	mismatchedRec := httptest.NewRecorder()
	handler.handleCanonicalAutoRegister(mismatchedRec, mismatchedReq, &reqBody, "127.0.0.1")
	if mismatchedRec.Code != http.StatusBadRequest {
		t.Fatalf("mismatched-completion status = %d, want 400", mismatchedRec.Code)
	}
	if got := mismatchedRec.Body.String(); got != "tokenId and tokenValue must be provided together\n" {
		t.Fatalf("mismatched-completion body = %q, want canonical token-pair guidance", got)
	}
}

func TestContract_AutoRegisterResponseJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"status":     "success",
		"message":    "Node pve-node-1 registered successfully at https://pve.local:8006",
		"action":     "use_token",
		"type":       "pve",
		"source":     "script",
		"host":       "https://pve.local:8006",
		"nodeId":     "pve-node-1",
		"nodeName":   "pve-node-1",
		"tokenId":    "pulse-monitor@pve!pulse-homelab",
		"tokenValue": "secret-token",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal auto-register response: %v", err)
	}

	const want = `{
		"action":"use_token",
		"host":"https://pve.local:8006",
		"message":"Node pve-node-1 registered successfully at https://pve.local:8006",
		"nodeId":"pve-node-1",
		"nodeName":"pve-node-1",
		"source":"script",
		"status":"success",
		"tokenId":"pulse-monitor@pve!pulse-homelab",
		"tokenValue":"secret-token",
		"type":"pve"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_AutoUnregisterResponseJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"status":   "success",
		"message":  "Node pve-node-1 removed successfully from https://pve.local:8006",
		"action":   "remove_connection",
		"type":     "pve",
		"source":   "script",
		"host":     "https://pve.local:8006",
		"nodeId":   "pve-node-1",
		"nodeName": "pve-node-1",
		"removed":  true,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal auto-unregister response: %v", err)
	}

	const want = `{
		"action":"remove_connection",
		"host":"https://pve.local:8006",
		"message":"Node pve-node-1 removed successfully from https://pve.local:8006",
		"nodeId":"pve-node-1",
		"nodeName":"pve-node-1",
		"removed":true,
		"source":"script",
		"status":"success",
		"type":"pve"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_AutoUnregisterNoopResponseJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"status":   "success",
		"message":  "No matching node is currently configured for https://pve.local:8006",
		"action":   "noop",
		"type":     "pve",
		"source":   "script",
		"host":     "https://pve.local:8006",
		"nodeId":   "pve-node-1",
		"nodeName": "pve-node-1",
		"removed":  false,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal auto-unregister noop response: %v", err)
	}

	const want = `{
		"action":"noop",
		"host":"https://pve.local:8006",
		"message":"No matching node is currently configured for https://pve.local:8006",
		"nodeId":"pve-node-1",
		"nodeName":"pve-node-1",
		"removed":false,
		"source":"script",
		"status":"success",
		"type":"pve"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_AutoRegisterWebSocketEventJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"type":      "pve",
		"source":    "script",
		"host":      "https://pve.local:8006",
		"name":      "pve-node-1",
		"nodeId":    "pve-node-1",
		"nodeName":  "pve-node-1",
		"tokenId":   "pulse-monitor@pve!pulse-homelab",
		"hasToken":  true,
		"verifySSL": true,
		"status":    "connected",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal auto-register websocket event: %v", err)
	}

	const want = `{
		"hasToken":true,
		"host":"https://pve.local:8006",
		"name":"pve-node-1",
		"nodeId":"pve-node-1",
		"nodeName":"pve-node-1",
		"source":"script",
		"status":"connected",
		"tokenId":"pulse-monitor@pve!pulse-homelab",
		"type":"pve",
		"verifySSL":true
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_CanonicalAutoRegisterEventJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"type":      "pbs",
		"source":    "agent",
		"host":      "https://pbs.local:8007",
		"name":      "backup-node (2)",
		"nodeId":    "backup-node (2)",
		"nodeName":  "backup-node (2)",
		"tokenId":   "pulse-monitor@pbs!pulse-backup",
		"hasToken":  true,
		"verifySSL": true,
		"status":    "connected",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal canonical /api/auto-register websocket event: %v", err)
	}

	const want = `{
		"hasToken":true,
		"host":"https://pbs.local:8007",
		"name":"backup-node (2)",
		"nodeId":"backup-node (2)",
		"nodeName":"backup-node (2)",
		"source":"agent",
		"status":"connected",
		"tokenId":"pulse-monitor@pbs!pulse-backup",
		"type":"pbs",
		"verifySSL":true
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_CanonicalAutoRegisterReusedTokenResponseJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"status":     "success",
		"message":    "Node pve-node-1 registered successfully at https://pve.local:8006",
		"action":     "use_token",
		"type":       "pve",
		"source":     "script",
		"host":       "https://pve.local:8006",
		"nodeId":     "pve-node-1",
		"nodeName":   "pve-node-1",
		"tokenId":    "pulse-monitor@pve!pulse-existing-node",
		"tokenValue": "existing-token",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal canonical /api/auto-register response: %v", err)
	}

	const want = `{
		"action":"use_token",
		"host":"https://pve.local:8006",
		"message":"Node pve-node-1 registered successfully at https://pve.local:8006",
		"nodeId":"pve-node-1",
		"nodeName":"pve-node-1",
		"source":"script",
		"status":"success",
		"tokenId":"pulse-monitor@pve!pulse-existing-node",
		"tokenValue":"existing-token",
		"type":"pve"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_CanonicalAutoRegisterCallerProvidedTokenResponseJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"status":     "success",
		"message":    "Node pve-node-1 registered successfully at https://pve.local:8006",
		"action":     "use_token",
		"type":       "pve",
		"source":     "agent",
		"host":       "https://pve.local:8006",
		"nodeId":     "pve-node-1",
		"nodeName":   "pve-node-1",
		"tokenId":    "pulse-monitor@pve!pulse-server",
		"tokenValue": "created-locally",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal canonical /api/auto-register caller-provided token response: %v", err)
	}

	const want = `{
		"action":"use_token",
		"host":"https://pve.local:8006",
		"message":"Node pve-node-1 registered successfully at https://pve.local:8006",
		"nodeId":"pve-node-1",
		"nodeName":"pve-node-1",
		"source":"agent",
		"status":"success",
		"tokenId":"pulse-monitor@pve!pulse-server",
		"tokenValue":"created-locally",
		"type":"pve"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_CanonicalAutoRegisterRotatedTokenResponseJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"status":     "success",
		"message":    "Node pve-node-1 registered successfully at https://pve.local:8006",
		"action":     "use_token",
		"type":       "pve",
		"source":     "agent",
		"host":       "https://pve.local:8006",
		"nodeId":     "pve-node-1",
		"nodeName":   "pve-node-1",
		"tokenId":    "pulse-monitor@pve!pulse-existing-node",
		"tokenValue": "rotated-token",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal canonical /api/auto-register rotated token response: %v", err)
	}

	const want = `{
		"action":"use_token",
		"host":"https://pve.local:8006",
		"message":"Node pve-node-1 registered successfully at https://pve.local:8006",
		"nodeId":"pve-node-1",
		"nodeName":"pve-node-1",
		"source":"agent",
		"status":"success",
		"tokenId":"pulse-monitor@pve!pulse-existing-node",
		"tokenValue":"rotated-token",
		"type":"pve"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_CanonicalAutoRegisterResponseUsesCanonicalStoredNodeIdentity(t *testing.T) {
	payload := map[string]any{
		"status":     "success",
		"message":    "Node pve-node-1 (2) registered successfully at https://pve.local:8006",
		"action":     "use_token",
		"type":       "pve",
		"source":     "agent",
		"host":       "https://pve.local:8006",
		"nodeId":     "pve-node-1 (2)",
		"nodeName":   "pve-node-1 (2)",
		"tokenId":    "pulse-monitor@pve!pulse-existing-node",
		"tokenValue": "existing-token",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal canonical /api/auto-register disambiguated response: %v", err)
	}

	const want = `{
		"action":"use_token",
		"host":"https://pve.local:8006",
		"message":"Node pve-node-1 (2) registered successfully at https://pve.local:8006",
		"nodeId":"pve-node-1 (2)",
		"nodeName":"pve-node-1 (2)",
		"source":"agent",
		"status":"success",
		"tokenId":"pulse-monitor@pve!pulse-existing-node",
		"tokenValue":"existing-token",
		"type":"pve"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_MetricsHistoryLiveFallbackJSONSnapshot(t *testing.T) {
	state := models.NewState()
	state.UpdateVMsForInstance("pve1", []models.VM{{
		ID:       "pve1:node1:101",
		VMID:     101,
		Name:     "vm-101",
		Node:     "node1",
		Instance: "pve1",
		Status:   "running",
		Type:     "qemu",
		CPU:      0.42,
		Memory: models.Memory{
			Usage: 55.0,
		},
		Disk: models.Disk{
			Usage: 33.0,
		},
	}})

	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", state)
	setUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))

	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("failed to init persistence: %v", err)
	}

	router := &Router{
		monitor:         monitor,
		licenseHandlers: NewLicenseHandlers(mtp, false),
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/metrics-store/history?resourceType=vm&resourceId=pve1:node1:101&metric=cpu&start=2026-03-11T00:00:00Z&end=2026-03-12T00:00:00Z",
		nil,
	)
	rec := httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal metrics history response: %v", err)
	}

	points, ok := payload["points"].([]any)
	if !ok || len(points) != 1 {
		t.Fatalf("unexpected points payload: %#v", payload["points"])
	}
	point, ok := points[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected point payload: %#v", points[0])
	}
	point["timestamp"] = float64(1700000000000)
	payload["range"] = "24h"
	payload["start"] = float64(1741651200000)
	payload["end"] = float64(1741737600000)

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal normalized metrics history response: %v", err)
	}

	const want = `{
		"end":1741737600000,
		"metric":"cpu",
		"points":[
			{
				"max":42,
				"min":42,
				"timestamp":1700000000000,
				"value":42
			}
		],
		"range":"24h",
		"resourceId":"pve1:node1:101",
		"resourceType":"vm",
		"source":"live",
		"start":1741651200000
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_MetricsHistoryProxmoxNodeCPUFallbackConvertsRatioToPercent(t *testing.T) {
	state := models.NewState()
	state.UpdateNodes([]models.Node{{
		ID:       "pve1:node1",
		Name:     "node1",
		Instance: "pve1",
		Status:   "online",
		CPU:      0.37,
	}})

	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", state)
	setUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))

	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("failed to init persistence: %v", err)
	}

	router := &Router{
		monitor:         monitor,
		licenseHandlers: NewLicenseHandlers(mtp, false),
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/metrics-store/history?resourceType=node&resourceId=pve1:node1&metric=cpu&range=24h",
		nil,
	)
	rec := httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp metricsHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal metrics history response: %v", err)
	}
	if resp.Source != "live" {
		t.Fatalf("source = %q, want live", resp.Source)
	}
	if len(resp.Points) != 1 {
		t.Fatalf("points = %d, want 1", len(resp.Points))
	}
	if math.Abs(resp.Points[0].Value-37) > 0.001 {
		t.Fatalf("cpu fallback value = %f, want 37", resp.Points[0].Value)
	}
}

func TestContract_MetricsHistoryDockerContainerCPUFallbackUsesCapacityPercent(t *testing.T) {
	state := models.NewState()
	state.UpsertDockerHost(models.DockerHost{
		ID:       "docker-host-contract-cpu",
		Hostname: "docker-host-contract-cpu",
		CPUs:     4,
		Containers: []models.DockerContainer{{
			ID:            "container-contract-cpu",
			Name:          "api",
			State:         "running",
			CPUPercent:    240,
			MemoryPercent: 50,
		}},
	})

	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", state)
	setUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))

	router := &Router{monitor: monitor}
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/metrics-store/history?resourceType=app-container&resourceId=container-contract-cpu&metric=cpu&range=5m",
		nil,
	)
	rec := httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp metricsHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal metrics history response: %v", err)
	}
	if resp.ResourceType != "app-container" || resp.ResourceId != "container-contract-cpu" || resp.Metric != "cpu" {
		t.Fatalf("unexpected metrics history identity: %+v", resp)
	}
	if resp.Source != "live" {
		t.Fatalf("expected live fallback source, got %q", resp.Source)
	}
	if len(resp.Points) != 1 || resp.Points[0].Value != 60 {
		t.Fatalf("Docker container CPU fallback points = %+v, want one normalized value 60", resp.Points)
	}
}

func TestContract_MetricsHistoryAgentTemperatureLiveFallback(t *testing.T) {
	state := models.NewState()
	state.UpsertHost(models.Host{
		ID:       "agent-pve-node-1",
		Hostname: "pve-node-1",
		Status:   "online",
		Sensors: models.HostSensorSummary{
			TemperatureCelsius: map[string]float64{
				"cpu_package": 62.5,
			},
		},
	})

	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", state)
	setUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))

	router := &Router{monitor: monitor}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/metrics-store/history?resourceType=agent&resourceId=agent-pve-node-1&metric=temperature&range=5m",
		nil,
	)
	rec := httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp metricsHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.ResourceType != "agent" {
		t.Fatalf("expected resourceType agent, got %q", resp.ResourceType)
	}
	if resp.ResourceId != "agent-pve-node-1" {
		t.Fatalf("expected resource id agent-pve-node-1, got %q", resp.ResourceId)
	}
	if resp.Metric != "temperature" {
		t.Fatalf("expected metric temperature, got %q", resp.Metric)
	}
	if resp.Source != "live" {
		t.Fatalf("expected source live, got %q", resp.Source)
	}
	if len(resp.Points) != 1 || resp.Points[0].Value != 62.5 {
		t.Fatalf("expected one live temperature point 62.5, got %+v", resp.Points)
	}
}

func TestContract_MetricsHistoryCanonicalizesLegacyKubernetesPodIDs(t *testing.T) {
	mh := monitoring.NewMetricsHistory(1000, time.Hour)
	now := time.Now().UTC().Truncate(time.Second)
	legacyID := "cluster-1:pod:pod-1"
	canonicalID := "k8s:" + legacyID
	for i, value := range []float64{21, 34, 29} {
		mh.AddGuestMetric(canonicalID, "cpu", value, now.Add(time.Duration(i-2)*10*time.Minute))
	}

	router := &Router{
		monitor: &monitoring.Monitor{},
	}
	setUnexportedField(t, router.monitor, "metricsHistory", mh)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/metrics-store/history?resourceType=k8s&resourceId="+legacyID+"&metric=cpu&range=30m",
		nil,
	)
	rec := httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp metricsHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.ResourceType != "k8s" {
		t.Fatalf("expected resourceType k8s, got %q", resp.ResourceType)
	}
	if resp.ResourceId != canonicalID {
		t.Fatalf("expected canonical pod resource id %q, got %q", canonicalID, resp.ResourceId)
	}
	if resp.Metric != "cpu" {
		t.Fatalf("expected metric cpu, got %q", resp.Metric)
	}
	if resp.Range != "30m" {
		t.Fatalf("expected range 30m, got %q", resp.Range)
	}
	if resp.Source != "memory" {
		t.Fatalf("expected source memory, got %q", resp.Source)
	}
	if len(resp.Points) != 3 {
		t.Fatalf("expected 3 cpu points, got %d", len(resp.Points))
	}
}

func TestContract_MetricsHistoryPhysicalDiskIOLiveWindowUsesCanonicalDiskTarget(t *testing.T) {
	mh := monitoring.NewMetricsHistory(1000, time.Hour)
	now := time.Now().UTC().Truncate(time.Second)
	resourceID := "SERIAL884006359727"
	for i, value := range []float64{1.5, 2.25, 3.0} {
		mh.AddDiskMetric(resourceID, "diskread", value*1024*1024, now.Add(time.Duration(i-2)*10*time.Minute))
	}

	router := &Router{
		monitor: &monitoring.Monitor{},
	}
	setUnexportedField(t, router.monitor, "metricsHistory", mh)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/metrics-store/history?resourceType=disk&resourceId="+resourceID+"&metric=diskread&range=30m",
		nil,
	)
	rec := httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp metricsHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.ResourceType != "disk" {
		t.Fatalf("expected resourceType disk, got %q", resp.ResourceType)
	}
	if resp.ResourceId != resourceID {
		t.Fatalf("expected canonical disk resource id %q, got %q", resourceID, resp.ResourceId)
	}
	if resp.Metric != "diskread" {
		t.Fatalf("expected metric diskread, got %q", resp.Metric)
	}
	if resp.Range != "30m" {
		t.Fatalf("expected range 30m, got %q", resp.Range)
	}
	if resp.Source != "memory" {
		t.Fatalf("expected source memory, got %q", resp.Source)
	}
	if len(resp.Points) != 3 {
		t.Fatalf("expected 3 diskread points, got %d", len(resp.Points))
	}
}

func TestContract_PatrolStatusResponseJSONSnapshot(t *testing.T) {
	lastPatrolAt := time.Date(2026, 3, 12, 9, 30, 0, 0, time.UTC)
	lastActivityAt := lastPatrolAt.Add(5 * time.Minute)
	nextPatrolAt := lastPatrolAt.Add(6 * time.Hour)
	blockedAt := lastPatrolAt.Add(15 * time.Minute)

	payload := PatrolStatusResponse{
		RuntimeState:   ai.PatrolRuntimeStateBlocked,
		Running:        false,
		Enabled:        true,
		LastPatrolAt:   &lastPatrolAt,
		LastActivityAt: &lastActivityAt,
		TriggerStatus: &ai.TriggerStatus{
			Running:                     true,
			PendingTriggers:             3,
			CurrentInterval:             300000,
			RecentEvents:                6,
			IsBusyMode:                  true,
			AlertTriggersEnabled:        true,
			AnomalyTriggersEnabled:      true,
			EventTriggersBlocked:        true,
			EventTriggersBlockedReason:  ai.EventTriggerBlockReasonBackgroundAutomationDisabled,
			EventTriggersBlockedMessage: "Automatic Patrol checks from alerts and anomalies are paused by the local development safety guard. Manual Patrol still works.",
		},
		NextPatrolAt:     &nextPatrolAt,
		LastDurationMs:   12345,
		ResourcesChecked: 18,
		FindingsCount:    3,
		ErrorCount:       1,
		Healthy:          false,
		IntervalMs:       21600000,
		FixedCount:       2,
		BlockedReason:    "Awaiting AI provider configuration",
		BlockedCause:     string(ai.PatrolFailureCauseProviderNotConfigured),
		BlockedAt:        &blockedAt,
		LicenseRequired:  true,
		LicenseStatus:    "none",
		UpgradeURL:       "https://pulserelay.pro/upgrade?feature=ai_autofix",
		Readiness: &PatrolReadinessResponse{
			Status:   patrolReadinessNotReady,
			Ready:    false,
			Cause:    string(ai.PatrolFailureCauseModelUnsupportedTools),
			Summary:  "The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.",
			Provider: "ollama",
			Model:    "ollama:deepseek-r1:7b-llama-distill-q4_K_M",
			Checks: []PatrolReadinessCheck{
				{
					ID:      "tools",
					Status:  patrolReadinessNotReady,
					Cause:   string(ai.PatrolFailureCauseModelUnsupportedTools),
					Label:   "Patrol tools",
					Message: "The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.",
					Action:  "open_provider_settings",
				},
			},
		},
	}
	payload.Summary.Critical = 1
	payload.Summary.Warning = 2
	payload.Summary.Watch = 0
	payload.Summary.Info = 4

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal patrol status response: %v", err)
	}

	const want = `{
		"runtime_state":"blocked",
		"running":false,
		"enabled":true,
		"last_patrol_at":"2026-03-12T09:30:00Z",
		"last_activity_at":"2026-03-12T09:35:00Z",
		"trigger_status":{"running":true,"pending_triggers":3,"current_interval_ms":300000,"recent_events":6,"is_busy_mode":true,"alert_triggers_enabled":true,"anomaly_triggers_enabled":true,"event_triggers_blocked":true,"event_triggers_blocked_reason":"background_automation_disabled","event_triggers_blocked_message":"Automatic Patrol checks from alerts and anomalies are paused by the local development safety guard. Manual Patrol still works."},
		"next_patrol_at":"2026-03-12T15:30:00Z",
		"last_duration_ms":12345,
		"resources_checked":18,
		"findings_count":3,
		"error_count":1,
		"healthy":false,
		"interval_ms":21600000,
		"fixed_count":2,
		"blocked_reason":"Awaiting AI provider configuration",
		"blocked_cause":"provider_not_configured",
		"blocked_at":"2026-03-12T09:45:00Z",
		"license_required":true,
		"license_status":"none",
		"upgrade_url":"https://pulserelay.pro/upgrade?feature=ai_autofix",
		"summary":{"critical":1,"warning":2,"watch":0,"info":4},
		"readiness":{"status":"not_ready","ready":false,"cause":"model_unsupported_tools","summary":"The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.","provider":"ollama","model":"ollama:deepseek-r1:7b-llama-distill-q4_K_M","checks":[{"id":"tools","status":"not_ready","cause":"model_unsupported_tools","label":"Patrol tools","message":"The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.","action":"open_provider_settings"}]}
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_PatrolRunRecordJSONSnapshot(t *testing.T) {
	startedAt := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	completedAt := startedAt.Add(90 * time.Second)

	payload := ai.PatrolRunRecord{
		ID:                        "run-1",
		StartedAt:                 startedAt,
		CompletedAt:               completedAt,
		DurationMs:                90000,
		Type:                      "scoped",
		TriggerReason:             "alert_fired",
		ScopeResourceIDs:          []string{"seed-resource"},
		EffectiveScopeResourceIDs: []string{},
		ScopeResourceTypes:        []string{"vm"},
		ResourcesChecked:          4,
		NodesChecked:              0,
		GuestsChecked:             2,
		DockerChecked:             0,
		StorageChecked:            0,
		HostsChecked:              0,
		TrueNASChecked:            1,
		PBSChecked:                0,
		PMGChecked:                1,
		KubernetesChecked:         1,
		NewFindings:               0,
		ExistingFindings:          2,
		RejectedFindings:          1,
		ResolvedFindings:          1,
		AutoFixCount:              0,
		FindingsSummary:           "All clear",
		FindingIDs:                []string{},
		ErrorCount:                0,
		Status:                    "healthy",
		TriageFlags:               3,
		TriageSkippedLLM:          true,
		ToolCallCount:             0,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal patrol run record: %v", err)
	}

	const want = `{
		"id":"run-1",
		"started_at":"2026-03-12T10:00:00Z",
		"completed_at":"2026-03-12T10:01:30Z",
		"duration_ms":90000,
		"type":"scoped",
		"trigger_reason":"alert_fired",
		"scope_resource_ids":["seed-resource"],
		"effective_scope_resource_ids":[],
		"scope_resource_types":["vm"],
		"resources_checked":4,
		"nodes_checked":0,
		"guests_checked":2,
		"docker_checked":0,
		"storage_checked":0,
		"hosts_checked":0,
		"truenas_checked":1,
		"pbs_checked":0,
		"pmg_checked":1,
		"kubernetes_checked":1,
		"new_findings":0,
		"existing_findings":2,
		"rejected_findings":1,
		"resolved_findings":1,
		"findings_summary":"All clear",
		"finding_ids":[],
		"error_count":0,
		"status":"healthy",
		"triage_flags":3,
		"triage_skipped_llm":true,
		"tool_call_count":0
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ChatStreamEventJSONSnapshots(t *testing.T) {
	cases := []struct {
		name  string
		event chat.StreamEvent
		want  string
	}{
		{
			name: "session",
			event: mustStreamEvent(t, "session", chat.SessionData{
				ID: "session-1",
			}),
			want: `{"type":"session","data":{"id":"session-1"}}`,
		},
		{
			name: "content",
			event: mustStreamEvent(t, "content", chat.ContentData{
				Text: "hello",
			}),
			want: `{"type":"content","data":{"text":"hello"}}`,
		},
		{
			name: "workflow_state_provider_start",
			event: mustStreamEvent(t, "workflow_state", chat.WorkflowStateData{
				Phase:    "provider_start",
				Message:  "Waiting for assistant.",
				State:    "investigating",
				Provider: "openrouter",
				Model:    "openrouter:qwen/qwen3.7-plus",
			}),
			want: `{"type":"workflow_state","data":{"phase":"provider_start","message":"Waiting for assistant.","state":"investigating","provider":"openrouter","model":"openrouter:qwen/qwen3.7-plus"}}`,
		},
		{
			name: "workflow_state_provider_retry",
			event: mustStreamEvent(t, "workflow_state", chat.WorkflowStateData{
				Phase:        "provider_retry",
				Message:      "Selected route connection failed before any output; retrying.",
				State:        "investigating",
				Attempt:      2,
				MaxAttempts:  2,
				RetryAfterMS: 200,
			}),
			want: `{"type":"workflow_state","data":{"phase":"provider_retry","message":"Selected route connection failed before any output; retrying.","state":"investigating","attempt":2,"max_attempts":2,"retry_after_ms":200}}`,
		},
		{
			name: "workflow_state_stream_idle",
			event: mustStreamEvent(t, "workflow_state", chat.WorkflowStateData{
				Phase:   "stream_idle",
				Message: chatStreamIdleProgressMessage,
			}),
			want: `{"type":"workflow_state","data":{"phase":"stream_idle","message":"Assistant is still working; waiting for the next stream event."}}`,
		},
		{
			name: "tool_start",
			event: mustStreamEvent(t, "tool_start", chat.ToolStartData{
				ID:       "tool-1",
				Name:     "pulse_read",
				Input:    `{"path":"/tmp/x.log"}`,
				RawInput: `{"path":"/tmp/x.log"}`,
			}),
			want: `{"type":"tool_start","data":{"id":"tool-1","name":"pulse_read","input":"{\"path\":\"/tmp/x.log\"}","raw_input":"{\"path\":\"/tmp/x.log\"}"}}`,
		},
		{
			name: "tool_progress",
			event: mustStreamEvent(t, "tool_progress", chat.ToolProgressData{
				ID:       "tool-1",
				Name:     "pulse_read",
				Input:    `{"path":"/tmp/x.log"}`,
				RawInput: `{"path":"/tmp/x.log"}`,
				Phase:    "running",
				Message:  "Reading target.",
			}),
			want: `{"type":"tool_progress","data":{"id":"tool-1","name":"pulse_read","input":"{\"path\":\"/tmp/x.log\"}","raw_input":"{\"path\":\"/tmp/x.log\"}","phase":"running","message":"Reading target."}}`,
		},
		{
			name: "tool_cancel",
			event: mustStreamEvent(t, "tool_cancel", chat.ToolCancelData{
				ID:     "tool-1",
				Name:   "pulse_read",
				Reason: "current_resource unavailable",
			}),
			want: `{"type":"tool_cancel","data":{"id":"tool-1","name":"pulse_read","reason":"current_resource unavailable"}}`,
		},
		{
			name: "tool_end",
			event: mustStreamEvent(t, "tool_end", chat.ToolEndData{
				ID:       "tool-1",
				Name:     "pulse_read",
				Input:    `{"path":"/tmp/x.log"}`,
				RawInput: `{"path":"/tmp/x.log"}`,
				Output:   "ok",
				Success:  true,
			}),
			want: `{"type":"tool_end","data":{"id":"tool-1","name":"pulse_read","input":"{\"path\":\"/tmp/x.log\"}","raw_input":"{\"path\":\"/tmp/x.log\"}","output":"ok","success":true}}`,
		},
		{
			name: "approval_needed",
			event: mustStreamEvent(t, "approval_needed", chat.ApprovalNeededData{
				ApprovalID:  "approval-1",
				ToolID:      "tool-2",
				ToolName:    "pulse_exec",
				Command:     "systemctl restart nginx",
				RunOnHost:   true,
				TargetHost:  "node-1",
				TargetType:  "agent",
				TargetID:    "agent-1",
				Risk:        "high",
				Description: "Restart web service",
				AuditID:     "action-1",
				Plan: &chat.ApprovalPlanData{
					ActionID:          "action-1",
					RequestID:         "approval-1",
					Summary:           "Restart web service",
					RequiresApproval:  true,
					ApprovalPolicy:    "admin",
					BlastRadius:       "service interruption on target",
					RollbackAvailable: true,
					PlanHash:          "hash-1",
					ExpiresAt:         "2026-04-23T12:30:00Z",
				},
				ContextConfidence: &chat.ApprovalContextConfidenceData{
					Level:    "verified",
					Summary:  "Target was resolved to a concrete resource before approval.",
					Evidence: []string{"Target identifier bound to agent-1."},
				},
				Preflight: &chat.ApprovalPreflightData{
					Target:            "agent:node-1 (agent-1)",
					CurrentState:      "Resolved approval target: agent:node-1 (agent-1).",
					IntendedChange:    "Restart web service",
					DryRunAvailable:   false,
					DryRunSummary:     "No provider-supported dry run is available for this action.",
					SafetyChecks:      []string{"Approval is scoped to this organization."},
					VerificationSteps: []string{"Read back the target state after execution."},
					GeneratedAt:       "2026-04-23T12:29:00Z",
				},
			}),
			want: `{"type":"approval_needed","data":{"approval_id":"approval-1","tool_id":"tool-2","tool_name":"pulse_exec","command":"systemctl restart nginx","run_on_host":true,"target_host":"node-1","target_type":"agent","target_id":"agent-1","risk":"high","description":"Restart web service","audit_id":"action-1","plan":{"action_id":"action-1","request_id":"approval-1","summary":"Restart web service","requires_approval":true,"approval_policy":"admin","blast_radius":"service interruption on target","rollback_available":true,"plan_hash":"hash-1","expires_at":"2026-04-23T12:30:00Z"},"context_confidence":{"level":"verified","summary":"Target was resolved to a concrete resource before approval.","evidence":["Target identifier bound to agent-1."]},"preflight":{"target":"agent:node-1 (agent-1)","current_state":"Resolved approval target: agent:node-1 (agent-1).","intended_change":"Restart web service","dry_run_available":false,"dry_run_summary":"No provider-supported dry run is available for this action.","safety_checks":["Approval is scoped to this organization."],"verification_steps":["Read back the target state after execution."],"generated_at":"2026-04-23T12:29:00Z"}}}`,
		},
		{
			name: "question",
			event: mustStreamEvent(t, "question", chat.QuestionData{
				SessionID:  "session-1",
				QuestionID: "question-1",
				Questions: []chat.Question{
					{
						ID:       "target",
						Type:     "select",
						Question: "Which node should I inspect?",
						Header:   "Target",
						Options: []chat.QuestionOption{
							{Label: "Node A", Value: "node-a", Description: "Primary compute node"},
							{Label: "Node B", Value: "node-b", Description: "Replica node"},
						},
					},
				},
			}),
			want: `{"type":"question","data":{"session_id":"session-1","question_id":"question-1","questions":[{"id":"target","type":"select","question":"Which node should I inspect?","header":"Target","options":[{"label":"Node A","value":"node-a","description":"Primary compute node"},{"label":"Node B","value":"node-b","description":"Replica node"}]}]}}`,
		},
		{
			name: "done",
			event: mustStreamEvent(t, "done", chat.DoneData{
				SessionID:    "session-1",
				InputTokens:  120,
				OutputTokens: 80,
			}),
			want: `{"type":"done","data":{"session_id":"session-1","input_tokens":120,"output_tokens":80}}`,
		},
		{
			name: "error",
			event: mustStreamEvent(t, "error", chat.ErrorData{
				Message: "request failed",
			}),
			want: `{"type":"error","data":{"message":"request failed"}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.event)
			if err != nil {
				t.Fatalf("marshal stream event: %v", err)
			}
			assertJSONSnapshot(t, got, tc.want)
		})
	}
}

func TestContract_LegacyAssistantStreamIdleEventJSONSnapshot(t *testing.T) {
	event := ai.StreamEvent{
		Type: "workflow_state",
		Data: chat.WorkflowStateData{
			Phase:   "stream_idle",
			Message: chatStreamIdleProgressMessage,
		},
	}
	got, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal legacy Assistant stream event: %v", err)
	}

	assertJSONSnapshot(t, got, `{"type":"workflow_state","data":{"phase":"stream_idle","message":"Assistant is still working; waiting for the next stream event."}}`)
}

func TestContract_AssistantChatStreamUsesClientSafeProjection(t *testing.T) {
	handlerSource, err := os.ReadFile(filepath.Clean("ai_handler.go"))
	if err != nil {
		t.Fatalf("read ai_handler.go: %v", err)
	}
	serviceSource, err := os.ReadFile(filepath.Clean("../ai/chat/service.go"))
	if err != nil {
		t.Fatalf("read chat service: %v", err)
	}
	typesSource, err := os.ReadFile(filepath.Clean("../ai/chat/types.go"))
	if err != nil {
		t.Fatalf("read chat types: %v", err)
	}

	handlerText := string(handlerSource)
	for _, required := range []string{
		"func (h *AIHandler) HandleChat(w http.ResponseWriter, r *http.Request)",
		"event, ok = event.ClientSafe()",
		"json.Marshal(event)",
	} {
		if !strings.Contains(handlerText, required) {
			t.Fatalf("Assistant chat SSE handler must encode client-safe stream projection: missing %q", required)
		}
	}

	serviceText := string(serviceSource)
	for _, required := range []string{
		"func (s *Service) ExecuteStream(ctx context.Context, req ExecuteRequest, callback StreamCallback) error",
		"event, ok = event.ClientSafe()",
		"streamCallback(event)",
	} {
		if !strings.Contains(serviceText, required) {
			t.Fatalf("chat service stream boundary must return client-safe stream events: missing %q", required)
		}
	}

	typesText := string(typesSource)
	for _, required := range []string{
		"func (e StreamEvent) ClientSafe() (StreamEvent, bool)",
		`case "thinking":`,
		"return StreamEvent{}, false",
		"data.Text = cleanToolCallArtifacts(data.Text)",
	} {
		if !strings.Contains(typesText, required) {
			t.Fatalf("chat stream client-safe projection must drop thinking and raw tool-call prose: missing %q", required)
		}
	}
}

func TestContract_AssistantMockModeRuntimeUsesEffectiveConfig(t *testing.T) {
	handlerSource, err := os.ReadFile(filepath.Clean("ai_handler.go"))
	if err != nil {
		t.Fatalf("read ai_handler.go: %v", err)
	}
	mockStreamSource, err := os.ReadFile(filepath.Clean("../ai/chat/mock_stream.go"))
	if err != nil {
		t.Fatalf("read chat mock stream: %v", err)
	}

	handlerText := string(handlerSource)
	for _, required := range []string{
		"func aiChatRuntimeConfigForMockMode(cfg *config.AIConfig) *config.AIConfig",
		"mockmode.IsEnabled()",
		"next.Enabled = true",
		"latestCfg := aiChatRuntimeConfigForMockMode(h.loadAIConfig(ctx))",
		"aiCfg := aiChatRuntimeConfigForMockMode(h.loadAIConfig(tenantCtx))",
		"aiCfg = aiChatRuntimeConfigForMockMode(aiCfg)",
		"newCfg := aiChatRuntimeConfigForMockMode(h.loadAIConfig(ctx))",
	} {
		if !strings.Contains(handlerText, required) {
			t.Fatalf("Assistant mock-mode runtime config must stay effective-only and cover startup/sync paths: missing %q", required)
		}
	}

	mockStreamText := string(mockStreamSource)
	for _, required := range []string{
		"var mockAssistantStreamPace =",
		"func pauseMockAssistantStream(ctx context.Context) bool",
		"time.NewTimer(mockAssistantStreamPace)",
		"pauseMockAssistantStream(ctx)",
		"Reading synthetic Pulse inventory.",
		"Summarizing mock inventory result.",
		"Reading mock device inventory.",
		"Summarizing mock device count.",
		"mockAssistantReadToolName",
		"Composing mock Assistant response.",
	} {
		if !strings.Contains(mockStreamText, required) {
			t.Fatalf("Assistant mock stream must pace browser-visible status/tool/content proof: missing %q", required)
		}
	}
}

func TestContract_PushNotificationJSONSnapshots(t *testing.T) {
	cases := []struct {
		name    string
		payload relay.PushNotificationPayload
		want    string
	}{
		{
			name:    "patrol_finding",
			payload: relay.NewPatrolFindingNotification("finding-1", "warning", "capacity", "Disk pressure detected"),
			want:    `{"type":"patrol_finding","priority":"normal","title":"Disk pressure detected","body":"New warning capacity finding detected","action_type":"view_finding","action_id":"finding-1","category":"capacity","severity":"warning"}`,
		},
		{
			name:    "patrol_critical",
			payload: relay.NewPatrolFindingNotification("finding-2", "critical", "performance", "CPU saturation detected"),
			want:    `{"type":"patrol_critical","priority":"high","title":"CPU saturation detected","body":"New critical performance finding detected","action_type":"view_finding","action_id":"finding-2","category":"performance","severity":"critical"}`,
		},
		{
			name:    "approval_request",
			payload: relay.NewApprovalRequestNotification("approval-1", "Fix queued", "high"),
			want:    `{"type":"approval_request","priority":"high","title":"Fix queued","body":"A high-risk fix requires your approval","action_type":"approve_fix","action_id":"approval-1"}`,
		},
		{
			name:    "fix_completed_success",
			payload: relay.NewFixCompletedNotification("finding-3", "CPU saturation detected", true),
			want:    `{"type":"fix_completed","priority":"normal","title":"CPU saturation detected","body":"Fix applied successfully","action_type":"view_fix_result","action_id":"finding-3"}`,
		},
		{
			name:    "fix_completed_failed",
			payload: relay.NewFixCompletedNotification("finding-4", "Disk pressure detected", false),
			want:    `{"type":"fix_completed","priority":"normal","title":"Disk pressure detected","body":"Fix attempt failed — review needed","action_type":"view_fix_result","action_id":"finding-4"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("marshal push payload: %v", err)
			}
			assertJSONSnapshot(t, got, tc.want)
		})
	}
}

func TestContract_AlertJSONSnapshot(t *testing.T) {
	start := time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC)
	lastSeen := start.Add(3 * time.Minute)

	payload := alerts.Alert{
		ID:           "cluster/qemu/100-cpu",
		Type:         "cpu",
		Level:        alerts.AlertLevelWarning,
		ResourceID:   "cluster/qemu/100",
		ResourceName: "test-vm",
		Node:         "pve-1",
		Instance:     "cpu0",
		Message:      "VM cpu at 95%",
		Value:        95.0,
		Threshold:    90.0,
		StartTime:    start,
		LastSeen:     lastSeen,
		Acknowledged: false,
		Metadata: map[string]interface{}{
			"resourceType":   "VM",
			"clearThreshold": 70.0,
			"unit":           "%",
			"monitorOnly":    true,
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal alert: %v", err)
	}

	const want = `{
		"id":"cluster/qemu/100-cpu",
		"type":"cpu",
		"level":"warning",
		"resourceId":"cluster/qemu/100",
		"resourceName":"test-vm",
		"node":"pve-1",
		"instance":"cpu0",
		"message":"VM cpu at 95%",
		"value":95,
		"threshold":90,
		"startTime":"2026-02-08T13:14:15Z",
		"lastSeen":"2026-02-08T13:17:15Z",
		"acknowledged":false,
		"metadata":{"clearThreshold":70,"monitorOnly":true,"resourceType":"VM","unit":"%"}
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_AlertAllFieldsJSONSnapshot(t *testing.T) {
	start := time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC)
	lastSeen := start.Add(3 * time.Minute)
	ackTime := start.Add(5 * time.Minute)
	lastNotified := start.Add(2 * time.Minute)
	escalationTimes := []time.Time{start.Add(1 * time.Minute), start.Add(3 * time.Minute)}

	payload := alerts.Alert{
		ID:              "cluster/qemu/100-cpu",
		Type:            "cpu",
		Level:           alerts.AlertLevelWarning,
		ResourceID:      "cluster/qemu/100",
		ResourceName:    "test-vm",
		Node:            "pve-1",
		NodeDisplayName: "Proxmox Node 1",
		Instance:        "cpu0",
		Message:         "VM cpu at 95%",
		Value:           95.0,
		Threshold:       90.0,
		StartTime:       start,
		LastSeen:        lastSeen,
		Acknowledged:    true,
		AckTime:         &ackTime,
		AckUser:         "admin",
		Metadata: map[string]interface{}{
			"resourceType":   "VM",
			"clearThreshold": 70.0,
			"unit":           "%",
			"monitorOnly":    true,
		},
		LastNotified:    &lastNotified,
		LastEscalation:  2,
		EscalationTimes: escalationTimes,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal alert with all fields: %v", err)
	}

	const want = `{
		"id":"cluster/qemu/100-cpu",
		"type":"cpu",
		"level":"warning",
		"resourceId":"cluster/qemu/100",
		"resourceName":"test-vm",
		"node":"pve-1",
		"nodeDisplayName":"Proxmox Node 1",
		"instance":"cpu0",
		"message":"VM cpu at 95%",
		"value":95,
		"threshold":90,
		"startTime":"2026-02-08T13:14:15Z",
		"lastSeen":"2026-02-08T13:17:15Z",
		"acknowledged":true,
		"ackTime":"2026-02-08T13:19:15Z",
		"ackUser":"admin",
		"metadata":{"clearThreshold":70,"monitorOnly":true,"resourceType":"VM","unit":"%"},
		"lastNotified":"2026-02-08T13:16:15Z",
		"lastEscalation":2,
		"escalationTimes":["2026-02-08T13:15:15Z","2026-02-08T13:17:15Z"]
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ModelAlertJSONSnapshot(t *testing.T) {
	start := time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC)
	ackTime := start.Add(5 * time.Minute)
	resolvedTime := start.Add(10 * time.Minute)

	t.Run("alert", func(t *testing.T) {
		payload := models.Alert{
			ID:              "cluster/qemu/100-cpu",
			Type:            "cpu",
			Level:           "warning",
			ResourceID:      "cluster/qemu/100",
			ResourceName:    "test-vm",
			Node:            "pve-1",
			NodeDisplayName: "Proxmox Node 1",
			Instance:        "cpu0",
			Message:         "VM cpu at 95%",
			Value:           95.0,
			Threshold:       90.0,
			StartTime:       start,
			Acknowledged:    true,
			AckTime:         &ackTime,
			AckUser:         "admin",
		}

		got, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal model alert: %v", err)
		}

		forbidden := []string{`"lastSeen"`, `"metadata"`, `"lastNotified"`, `"lastEscalation"`, `"escalationTimes"`}
		for _, field := range forbidden {
			if strings.Contains(string(got), field) {
				t.Fatalf("model alert json unexpectedly contains %s: %s", field, string(got))
			}
		}

		const want = `{
			"id":"cluster/qemu/100-cpu",
			"type":"cpu",
			"level":"warning",
			"resourceId":"cluster/qemu/100",
			"resourceName":"test-vm",
			"node":"pve-1",
			"nodeDisplayName":"Proxmox Node 1",
			"instance":"cpu0",
			"message":"VM cpu at 95%",
			"value":95,
			"threshold":90,
			"startTime":"2026-02-08T13:14:15Z",
			"acknowledged":true,
			"ackTime":"2026-02-08T13:19:15Z",
			"ackUser":"admin"
		}`

		assertJSONSnapshot(t, got, want)
	})

	t.Run("resolved_alert", func(t *testing.T) {
		payload := models.ResolvedAlert{
			Alert: models.Alert{
				ID:              "cluster/qemu/100-cpu",
				Type:            "cpu",
				Level:           "warning",
				ResourceID:      "cluster/qemu/100",
				ResourceName:    "test-vm",
				Node:            "pve-1",
				NodeDisplayName: "Proxmox Node 1",
				Instance:        "cpu0",
				Message:         "VM cpu at 95%",
				Value:           95.0,
				Threshold:       90.0,
				StartTime:       start,
				Acknowledged:    true,
				AckTime:         &ackTime,
				AckUser:         "admin",
			},
			ResolvedTime: resolvedTime,
		}

		got, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal model resolved alert: %v", err)
		}

		forbidden := []string{`"lastSeen"`, `"metadata"`, `"lastNotified"`, `"lastEscalation"`, `"escalationTimes"`}
		for _, field := range forbidden {
			if strings.Contains(string(got), field) {
				t.Fatalf("model resolved alert json unexpectedly contains %s: %s", field, string(got))
			}
		}

		const want = `{
			"id":"cluster/qemu/100-cpu",
			"type":"cpu",
			"level":"warning",
			"resourceId":"cluster/qemu/100",
			"resourceName":"test-vm",
			"node":"pve-1",
			"nodeDisplayName":"Proxmox Node 1",
			"instance":"cpu0",
			"message":"VM cpu at 95%",
			"value":95,
			"threshold":90,
			"startTime":"2026-02-08T13:14:15Z",
			"acknowledged":true,
			"ackTime":"2026-02-08T13:19:15Z",
			"ackUser":"admin",
			"resolvedTime":"2026-02-08T13:24:15Z"
		}`

		assertJSONSnapshot(t, got, want)
	})
}

func TestContract_IncidentJSONSnapshot(t *testing.T) {
	start := time.Date(2026, 2, 8, 13, 14, 15, 0, time.UTC)
	ackTime := start.Add(5 * time.Minute)
	closedAt := start.Add(10 * time.Minute)

	t.Run("open", func(t *testing.T) {
		payload := memory.Incident{
			ID:              "incident-1",
			AlertIdentifier: "cluster/qemu/100-cpu",
			AlertType:       "cpu",
			Level:           "warning",
			ResourceID:      "cluster/qemu/100",
			ResourceName:    "test-vm",
			ResourceType:    "guest",
			Node:            "pve-1",
			Instance:        "cpu0",
			Message:         "VM cpu at 95%",
			Status:          memory.IncidentStatusOpen,
			OpenedAt:        start,
			Acknowledged:    true,
			AckUser:         "admin",
			AckTime:         &ackTime,
			Events: []memory.IncidentEvent{
				{
					ID:        "evt-1",
					Type:      memory.IncidentEventAlertFired,
					Timestamp: start.Add(1 * time.Minute),
					Summary:   "CPU alert fired",
					Details: map[string]interface{}{
						"type":      "cpu",
						"level":     "warning",
						"value":     95,
						"threshold": 90,
					},
				},
				{
					ID:        "evt-2",
					Type:      memory.IncidentEventAlertAcknowledged,
					Timestamp: start.Add(5 * time.Minute),
					Summary:   "Alert acknowledged",
					Details: map[string]interface{}{
						"user": "admin",
					},
				},
			},
		}

		got, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal open incident: %v", err)
		}

		const want = `{
			"id":"incident-1",
			"alertIdentifier":"cluster/qemu/100-cpu",
			"alertType":"cpu",
			"level":"warning",
			"resourceId":"cluster/qemu/100",
			"resourceName":"test-vm",
			"resourceType":"guest",
			"node":"pve-1",
			"instance":"cpu0",
			"message":"VM cpu at 95%",
			"status":"open",
			"openedAt":"2026-02-08T13:14:15Z",
			"acknowledged":true,
			"ackUser":"admin",
			"ackTime":"2026-02-08T13:19:15Z",
			"events":[
				{"id":"evt-1","type":"alert_fired","timestamp":"2026-02-08T13:15:15Z","summary":"CPU alert fired","details":{"level":"warning","threshold":90,"type":"cpu","value":95}},
				{"id":"evt-2","type":"alert_acknowledged","timestamp":"2026-02-08T13:19:15Z","summary":"Alert acknowledged","details":{"user":"admin"}}
			]
		}`

		assertJSONSnapshot(t, got, want)
	})

	t.Run("resolved", func(t *testing.T) {
		payload := memory.Incident{
			ID:              "incident-1",
			AlertIdentifier: "cluster/qemu/100-cpu",
			AlertType:       "cpu",
			Level:           "warning",
			ResourceID:      "cluster/qemu/100",
			ResourceName:    "test-vm",
			ResourceType:    "guest",
			Node:            "pve-1",
			Instance:        "cpu0",
			Message:         "VM cpu at 95%",
			Status:          memory.IncidentStatusResolved,
			OpenedAt:        start,
			ClosedAt:        &closedAt,
			Acknowledged:    true,
			AckUser:         "admin",
			AckTime:         &ackTime,
			Events: []memory.IncidentEvent{
				{
					ID:        "evt-1",
					Type:      memory.IncidentEventAlertFired,
					Timestamp: start.Add(1 * time.Minute),
					Summary:   "CPU alert fired",
					Details: map[string]interface{}{
						"type":      "cpu",
						"level":     "warning",
						"value":     95,
						"threshold": 90,
					},
				},
				{
					ID:        "evt-2",
					Type:      memory.IncidentEventAlertAcknowledged,
					Timestamp: start.Add(5 * time.Minute),
					Summary:   "Alert acknowledged",
					Details: map[string]interface{}{
						"user": "admin",
					},
				},
			},
		}

		got, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal resolved incident: %v", err)
		}

		const want = `{
			"id":"incident-1",
			"alertIdentifier":"cluster/qemu/100-cpu",
			"alertType":"cpu",
			"level":"warning",
			"resourceId":"cluster/qemu/100",
			"resourceName":"test-vm",
			"resourceType":"guest",
			"node":"pve-1",
			"instance":"cpu0",
			"message":"VM cpu at 95%",
			"status":"resolved",
			"openedAt":"2026-02-08T13:14:15Z",
			"closedAt":"2026-02-08T13:24:15Z",
			"acknowledged":true,
			"ackUser":"admin",
			"ackTime":"2026-02-08T13:19:15Z",
			"events":[
				{"id":"evt-1","type":"alert_fired","timestamp":"2026-02-08T13:15:15Z","summary":"CPU alert fired","details":{"level":"warning","threshold":90,"type":"cpu","value":95}},
				{"id":"evt-2","type":"alert_acknowledged","timestamp":"2026-02-08T13:19:15Z","summary":"Alert acknowledged","details":{"user":"admin"}}
			]
		}`

		assertJSONSnapshot(t, got, want)
	})
}

func TestContract_IncidentEventTypeEnumSnapshot(t *testing.T) {
	type envelope struct {
		Type memory.IncidentEventType `json:"type"`
	}

	cases := []struct {
		name string
		typ  memory.IncidentEventType
		want string
	}{
		{name: "alert_fired", typ: memory.IncidentEventAlertFired, want: `{"type":"alert_fired"}`},
		{name: "alert_acknowledged", typ: memory.IncidentEventAlertAcknowledged, want: `{"type":"alert_acknowledged"}`},
		{name: "alert_unacknowledged", typ: memory.IncidentEventAlertUnacknowledged, want: `{"type":"alert_unacknowledged"}`},
		{name: "alert_resolved", typ: memory.IncidentEventAlertResolved, want: `{"type":"alert_resolved"}`},
		{name: "ai_analysis", typ: memory.IncidentEventAnalysis, want: `{"type":"ai_analysis"}`},
		{name: "command", typ: memory.IncidentEventCommand, want: `{"type":"command"}`},
		{name: "runbook", typ: memory.IncidentEventRunbook, want: `{"type":"runbook"}`},
		{name: "note", typ: memory.IncidentEventNote, want: `{"type":"note"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(envelope{Type: tc.typ})
			if err != nil {
				t.Fatalf("marshal incident event type %q: %v", tc.name, err)
			}
			assertJSONSnapshot(t, got, tc.want)
		})
	}
}

func TestContract_AlertFieldNamingConsistency(t *testing.T) {
	cases := []struct {
		name string
		typ  reflect.Type
	}{
		{name: "alerts.Alert", typ: reflect.TypeOf(alerts.Alert{})},
		{name: "memory.Incident", typ: reflect.TypeOf(memory.Incident{})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for i := 0; i < tc.typ.NumField(); i++ {
				field := tc.typ.Field(i)
				if !field.IsExported() {
					continue
				}

				jsonTag := field.Tag.Get("json")
				if jsonTag == "" || jsonTag == "-" {
					continue
				}

				tagName := strings.Split(jsonTag, ",")[0]
				if strings.Contains(tagName, "_") {
					t.Fatalf("field %s on %s uses snake_case json tag %q", field.Name, tc.name, tagName)
				}
			}
		})
	}
}

func TestContract_AlertResourceTypeConsistency(t *testing.T) {
	cases := []struct {
		resourceType string
		want         []string
	}{
		{resourceType: "VM", want: []string{"vm", "guest"}},
		{resourceType: "Container", want: []string{}},
		{resourceType: "Node", want: []string{"node"}},
		{resourceType: "Agent", want: []string{"agent", "node"}},
		{resourceType: "Agent Disk", want: []string{}},
		{resourceType: "PBS", want: []string{"pbs", "node"}},
		{resourceType: "Docker Container", want: []string{}},
		{resourceType: "DockerHost", want: []string{}},
		{resourceType: "Docker Service", want: []string{}},
		{resourceType: "Storage", want: []string{"storage"}},
		{resourceType: "PMG", want: []string{"pmg", "node"}},
		{resourceType: "K8s", want: []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.resourceType, func(t *testing.T) {
			got := alerts.CanonicalResourceTypeKeys(tc.resourceType)
			if len(tc.want) > 0 && len(got) == 0 {
				t.Fatalf("resource type %q returned no canonical keys", tc.resourceType)
			}
			if len(tc.want) == 0 && len(got) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("canonical keys mismatch for %q: got %v want %v", tc.resourceType, got, tc.want)
			}
		})
	}
}

func TestContractResourceFiltersAcceptNativeDockerAndKubernetesInventory(t *testing.T) {
	raw := strings.Join([]string{
		"docker-image",
		"docker-volume",
		"docker-network",
		"docker-task",
		"docker-swarm-node",
		"docker-secret",
		"docker-config",
		"k8s-namespace",
		"k8s-service",
		"k8s-replicaset",
		"k8s-statefulset",
		"k8s-daemonset",
		"k8s-job",
		"k8s-cronjob",
		"k8s-ingress",
		"k8s-endpoint-slice",
		"k8s-network-policy",
		"k8s-persistent-volume",
		"k8s-persistent-volume-claim",
		"k8s-storage-class",
		"k8s-configmap",
		"k8s-secret",
		"k8s-serviceaccount",
		"k8s-role",
		"k8s-cluster-role",
		"k8s-role-binding",
		"k8s-cluster-role-binding",
		"k8s-resource-quota",
		"k8s-limit-range",
		"k8s-pod-disruption-budget",
		"k8s-horizontal-pod-autoscaler",
		"k8s-event",
	}, ",")

	if unsupported := unsupportedResourceTypeFilterTokens(raw); len(unsupported) != 0 {
		t.Fatalf("native platform inventory filters should be supported, got unsupported=%v", unsupported)
	}

	got := parseResourceTypes(raw)
	for _, resourceType := range []unifiedresources.ResourceType{
		unifiedresources.ResourceTypeDockerImage,
		unifiedresources.ResourceTypeDockerVolume,
		unifiedresources.ResourceTypeDockerNetwork,
		unifiedresources.ResourceTypeDockerTask,
		unifiedresources.ResourceTypeDockerSwarmNode,
		unifiedresources.ResourceTypeDockerSecret,
		unifiedresources.ResourceTypeDockerConfig,
		unifiedresources.ResourceTypeK8sNamespace,
		unifiedresources.ResourceTypeK8sService,
		unifiedresources.ResourceTypeK8sReplicaSet,
		unifiedresources.ResourceTypeK8sStatefulSet,
		unifiedresources.ResourceTypeK8sDaemonSet,
		unifiedresources.ResourceTypeK8sJob,
		unifiedresources.ResourceTypeK8sCronJob,
		unifiedresources.ResourceTypeK8sIngress,
		unifiedresources.ResourceTypeK8sEndpointSlice,
		unifiedresources.ResourceTypeK8sNetworkPolicy,
		unifiedresources.ResourceTypeK8sPV,
		unifiedresources.ResourceTypeK8sPVC,
		unifiedresources.ResourceTypeK8sStorageClass,
		unifiedresources.ResourceTypeK8sConfigMap,
		unifiedresources.ResourceTypeK8sSecret,
		unifiedresources.ResourceTypeK8sServiceAccount,
		unifiedresources.ResourceTypeK8sRole,
		unifiedresources.ResourceTypeK8sClusterRole,
		unifiedresources.ResourceTypeK8sRoleBinding,
		unifiedresources.ResourceTypeK8sClusterRoleBinding,
		unifiedresources.ResourceTypeK8sResourceQuota,
		unifiedresources.ResourceTypeK8sLimitRange,
		unifiedresources.ResourceTypeK8sPDB,
		unifiedresources.ResourceTypeK8sHPA,
		unifiedresources.ResourceTypeK8sEvent,
	} {
		if _, ok := got[resourceType]; !ok {
			t.Fatalf("parseResourceTypes(%q) missing %q in %#v", raw, resourceType, got)
		}
	}
}

func TestContract_TenantResourcesDoNotFallbackToRawSnapshotSeeding(t *testing.T) {
	now := time.Date(2026, 3, 17, 9, 0, 0, 0, time.UTC)
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceStateProvider{snapshot: models.StateSnapshot{
		Hosts: []models.Host{{ID: "host-default", Hostname: "default", Status: "online", LastSeen: now}},
	}})
	h.SetTenantStateProvider(tenantResourceStateProvider{snapshots: map[string]models.StateSnapshot{
		"acme": {
			Hosts:      []models.Host{{ID: "host-tenant-snapshot", Hostname: "tenant-snapshot", Status: "online", LastSeen: now}},
			LastUpdate: time.Time{},
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=agent", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "acme"))
	rec := httptest.NewRecorder()

	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	const want = `{"data":[],"meta":{"page":1,"limit":50,"total":0,"totalPages":0},"aggregations":{"total":0,"byType":{},"byStatus":{},"bySource":{},"policyPosture":{"totalResources":0,"sensitivityCounts":{},"routingCounts":{},"redactionCounts":{}}}}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("tenant resource fallback contract = %s, want %s", got, want)
	}
}

func TestContract_ResourceListPolicyMetadata(t *testing.T) {
	now := time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC)
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unifiedresources.Resource{
			{
				ID:       "vm-sensitive",
				Type:     unifiedresources.ResourceTypeVM,
				Name:     "payments-vm",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
				Sources:  []unifiedresources.DataSource{unifiedresources.SourceProxmox},
				Tags:     []string{"customer-data"},
				Identity: unifiedresources.ResourceIdentity{
					Hostnames:   []string{"payments.internal"},
					IPAddresses: []string{"10.0.0.44"},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=vm", nil)
	rec := httptest.NewRecorder()

	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resp.Data))
	}

	resource := resp.Data[0]
	if resource.Canonical == nil {
		t.Fatal("expected canonical identity metadata in resource contract")
	}
	if strings.TrimSpace(resource.Canonical.DisplayName) == "" {
		t.Fatal("expected canonical display name in resource contract")
	}
	if resource.Policy == nil {
		t.Fatal("expected policy metadata in resource contract")
	}
	if got := resource.Policy.Sensitivity; got != unifiedresources.ResourceSensitivityRestricted {
		t.Fatalf("policy sensitivity = %q, want %q", got, unifiedresources.ResourceSensitivityRestricted)
	}
	if got := resource.Policy.Routing.Scope; got != unifiedresources.ResourceRoutingScopeLocalOnly {
		t.Fatalf("routing scope = %q, want %q", got, unifiedresources.ResourceRoutingScopeLocalOnly)
	}
	wantRedactions := []unifiedresources.ResourceRedactionHint{
		unifiedresources.ResourceRedactionHostname,
		unifiedresources.ResourceRedactionIPAddress,
		unifiedresources.ResourceRedactionPlatformID,
		unifiedresources.ResourceRedactionAlias,
	}
	if !reflect.DeepEqual(resource.Policy.Routing.Redact, wantRedactions) {
		t.Fatalf("policy redact = %#v, want %#v", resource.Policy.Routing.Redact, wantRedactions)
	}
	if got := resource.AISafeSummary; !strings.Contains(got, "virtual machine resource;") || !strings.Contains(got, "local-only context") {
		t.Fatalf("aiSafeSummary = %q", got)
	}
	if resp.Aggregations.PolicyPosture == nil {
		t.Fatal("expected policy posture aggregation in resource list contract")
	}
	if got := resp.Aggregations.PolicyPosture.TotalResources; got != 1 {
		t.Fatalf("policyPosture.totalResources = %d, want 1", got)
	}
	if got := resp.Aggregations.PolicyPosture.SensitivityCounts[unifiedresources.ResourceSensitivityRestricted]; got != 1 {
		t.Fatalf("policyPosture.sensitivityCounts[restricted] = %d, want 1", got)
	}
	if got := resp.Aggregations.PolicyPosture.RoutingCounts[unifiedresources.ResourceRoutingScopeLocalOnly]; got != 1 {
		t.Fatalf("policyPosture.routingCounts[local-only] = %d, want 1", got)
	}
	if got := resp.Aggregations.PolicyPosture.RedactionCounts[unifiedresources.ResourceRedactionHostname]; got != 1 {
		t.Fatalf("policyPosture.redactionCounts[hostname] = %d, want 1", got)
	}
}

func TestContract_ProxmoxWorkloadDiscoveryTargetUsesLinkedNodeAgent(t *testing.T) {
	now := time.Date(2026, 6, 4, 20, 0, 0, 0, time.UTC)
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unifiedresources.Resource{
			{
				ID:       "system-container:grafana",
				Type:     unifiedresources.ResourceTypeSystemContainer,
				Name:     "grafana",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
				Sources:  []unifiedresources.DataSource{unifiedresources.SourceProxmox},
				Proxmox: &unifiedresources.ProxmoxData{
					NodeName:      "delly",
					VMID:          124,
					LinkedAgentID: "agent-delly",
				},
			},
			{
				ID:       "system-container:unlinked",
				Type:     unifiedresources.ResourceTypeSystemContainer,
				Name:     "unlinked",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
				Sources:  []unifiedresources.DataSource{unifiedresources.SourceProxmox},
				Proxmox: &unifiedresources.ProxmoxData{
					NodeName: "delly",
					VMID:     125,
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=system-container", nil)
	rec := httptest.NewRecorder()
	h.HandleListResources(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	resourcesByID := make(map[string]unifiedresources.Resource, len(resp.Data))
	for _, resource := range resp.Data {
		resourcesByID[resource.ID] = resource
	}
	linkedTarget := resourcesByID["system-container:grafana"].DiscoveryTarget
	if linkedTarget == nil {
		t.Fatal("expected linked Proxmox workload discovery target")
	}
	if linkedTarget.AgentID != "agent-delly" || linkedTarget.ResourceID != "124" {
		t.Fatalf("linked discovery target = %+v, want agent-delly/124", linkedTarget)
	}
	if target := resourcesByID["system-container:unlinked"].DiscoveryTarget; target != nil {
		t.Fatalf("expected unlinked Proxmox workload to omit discovery target, got %+v", target)
	}
}

func TestContract_ResourceListUsesDeterministicNameTieBreakers(t *testing.T) {
	now := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unifiedresources.Resource{
			{ID: "storage-c", Type: unifiedresources.ResourceTypeStorage, Name: "backup-vault-a", Status: unifiedresources.StatusOnline, LastSeen: now},
			{ID: "storage-a", Type: unifiedresources.ResourceTypeStorage, Name: "backup-vault-a", Status: unifiedresources.StatusOnline, LastSeen: now},
			{ID: "storage-b", Type: unifiedresources.ResourceTypeStorage, Name: "backup-vault-a", Status: unifiedresources.StatusOnline, LastSeen: now},
		},
	})

	for attempt := 0; attempt < 2; attempt++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/resources?type=storage&page=1&limit=100", nil)
		h.HandleListResources(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d: status = %d, body=%s", attempt+1, rec.Code, rec.Body.String())
		}

		var resp ResourcesResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("attempt %d: decode response: %v", attempt+1, err)
		}

		gotIDs := make([]string, 0, len(resp.Data))
		for _, resource := range resp.Data {
			gotIDs = append(gotIDs, resource.ID)
		}

		wantIDs := []string{"storage-a", "storage-b", "storage-c"}
		if len(gotIDs) != len(wantIDs) {
			t.Fatalf("attempt %d: expected %d resources, got %d (%v)", attempt+1, len(wantIDs), len(gotIDs), gotIDs)
		}
		for index := range wantIDs {
			if gotIDs[index] != wantIDs[index] {
				t.Fatalf("attempt %d: position %d = %q, want %q (got=%v)", attempt+1, index, gotIDs[index], wantIDs[index], gotIDs)
			}
		}
	}
}

func TestContract_ResourceListAcceptsBrowserEncodedTypeCSV(t *testing.T) {
	now := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unifiedresources.Resource{
			{ID: "agent-delly", Type: unifiedresources.ResourceTypeAgent, Name: "delly", Status: unifiedresources.StatusOnline, LastSeen: now},
			{ID: "docker-host-frigate", Type: "docker-host", Name: "frigate", Status: unifiedresources.StatusOnline, LastSeen: now},
			{ID: "container-frigate", Type: unifiedresources.ResourceTypeAppContainer, Name: "frigate", Status: unifiedresources.StatusOnline, LastSeen: now},
			{ID: "vm-ignored", Type: unifiedresources.ResourceTypeVM, Name: "ignored-vm", Status: unifiedresources.StatusOnline, LastSeen: now},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=agent%2Cdocker-host%2Capp-container&page=1&limit=100", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	gotTypes := make(map[unifiedresources.ResourceType]int)
	for _, resource := range resp.Data {
		gotTypes[resource.Type]++
	}
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 encoded-filtered resources, got %d (types=%v)", len(resp.Data), gotTypes)
	}
	for _, typ := range []unifiedresources.ResourceType{
		unifiedresources.ResourceTypeAgent,
		"docker-host",
		unifiedresources.ResourceTypeAppContainer,
	} {
		if gotTypes[typ] != 1 {
			t.Fatalf("encoded type filter count for %q = %d, want 1 (types=%v)", typ, gotTypes[typ], gotTypes)
		}
	}
	if gotTypes[unifiedresources.ResourceTypeVM] != 0 {
		t.Fatalf("encoded type filter leaked VM resources: %v", gotTypes)
	}
}

func TestContract_StateAndResourceListShareCanonicalMockResourceContract(t *testing.T) {
	setMockModeForTest(t, true)

	dataPath := t.TempDir()
	hashedPassword, err := authpkg.HashPassword("password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	cfg := &config.Config{
		DataPath:           dataPath,
		MultiTenantEnabled: true,
		AuthUser:           "admin",
		AuthPass:           hashedPassword,
	}

	InitSessionStore(dataPath)
	InitCSRFStore(dataPath)

	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("new monitor: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })

	router := &Router{
		config:      cfg,
		monitor:     monitor,
		persistence: config.NewConfigPersistence(dataPath),
	}
	handlers := NewResourceHandlers(cfg)
	handlers.SetStateProvider(monitor)

	stateReq := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	stateReq.SetBasicAuth("admin", "password")
	stateRec := httptest.NewRecorder()
	router.handleState(stateRec, stateReq)
	if stateRec.Code != http.StatusOK {
		t.Fatalf("/api/state status = %d, body=%s", stateRec.Code, stateRec.Body.String())
	}

	var state models.StateFrontend
	if err := json.NewDecoder(stateRec.Body).Decode(&state); err != nil {
		t.Fatalf("decode /api/state: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/resources?type=agent,docker-host,pbs,pmg,k8s-cluster,k8s-node&page=1&limit=100", nil)
	listRec := httptest.NewRecorder()
	handlers.HandleListResources(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("/api/resources status = %d, body=%s", listRec.Code, listRec.Body.String())
	}

	var listResp ResourcesResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode /api/resources: %v", err)
	}

	allowedTypes := map[string]bool{
		"agent":       true,
		"docker-host": true,
		"k8s-cluster": true,
		"k8s-node":    true,
		"pbs":         true,
		"pmg":         true,
	}

	stateContracts := make(map[string]resourceContractSnapshot)
	for _, resource := range state.Resources {
		if !allowedTypes[resource.Type] {
			continue
		}
		if resource.Type == "node" {
			t.Fatalf("/api/state published legacy resource type %#v", resource)
		}
		if resource.Name == "pve1" || resource.Name == "edge-apps-01" {
			t.Fatalf("/api/state published legacy display name %#v", resource)
		}
		stateContracts[resource.ID] = resourceContractSnapshot{
			ID:   resource.ID,
			Name: resource.Name,
			Type: resource.Type,
		}
	}

	listContracts := make(map[string]resourceContractSnapshot)
	for _, resource := range listResp.Data {
		if !allowedTypes[string(resource.Type)] {
			continue
		}
		listContracts[resource.ID] = resourceContractSnapshot{
			ID:   resource.ID,
			Name: resource.Name,
			Type: string(resource.Type),
		}
	}

	if !reflect.DeepEqual(stateContracts, listContracts) {
		t.Fatalf("/api/state and /api/resources resource contracts diverged: state=%#v resources=%#v", stateContracts, listContracts)
	}
	foundCanonicalProxmoxName := false
	for _, resource := range stateContracts {
		if resource.Name == "West Production A" && resource.Type == "agent" {
			foundCanonicalProxmoxName = true
			break
		}
	}
	if !foundCanonicalProxmoxName {
		t.Fatalf("expected canonical proxmox resource contract in /api/state, got %#v", stateContracts)
	}
}

func TestContract_ResourceListCarriesTimelineAndCapabilityContracts(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	occurredAt := now.Add(-2 * time.Minute)

	payload := ResourcesResponse{
		Data: []unifiedresources.Resource{
			{
				ID:       "vm-42",
				Type:     unifiedresources.ResourceTypeVM,
				Name:     "web-42",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
				Capabilities: []unifiedresources.ResourceCapability{
					{
						Name:                 "restart",
						Type:                 unifiedresources.CapabilityTypeCommon,
						Description:          "Restart the VM",
						MinimumApprovalLevel: unifiedresources.ApprovalAdmin,
						Params: []unifiedresources.CapabilityParam{
							{
								Name:        "force",
								Type:        "boolean",
								Required:    false,
								Description: "Restart without graceful shutdown",
							},
						},
					},
				},
				Relationships: []unifiedresources.ResourceRelationship{
					{
						SourceID:   "vm-42",
						TargetID:   "node-1",
						Type:       unifiedresources.RelRunsOn,
						Confidence: 1,
						Active:     true,
						Discoverer: "proxmox_adapter",
						ObservedAt: now,
						LastSeenAt: now,
						Metadata: map[string]any{
							"source":  "live",
							"cluster": "pve-prod",
						},
					},
				},
				RecentChanges: []unifiedresources.ResourceChange{
					{
						ID:               "chg-42",
						ObservedAt:       now,
						OccurredAt:       &occurredAt,
						ResourceID:       "vm-42",
						Kind:             unifiedresources.ChangeStateTransition,
						From:             "offline",
						To:               "online",
						SourceType:       unifiedresources.SourcePlatformEvent,
						SourceAdapter:    unifiedresources.AdapterProxmox,
						Confidence:       unifiedresources.ConfidenceHigh,
						RelatedResources: []string{"node-1"},
						Reason:           "vm started",
						Metadata: map[string]any{
							"source": "snapshot",
							"ticket": "INC-1234",
						},
					},
				},
				FacetCounts: unifiedresources.ResourceFacetCounts{
					RecentChanges: 1,
				},
			},
		},
		Meta: ResourcesMeta{
			Page:       1,
			Limit:      50,
			Total:      1,
			TotalPages: 1,
		},
		Aggregations: unifiedresources.ResourceStats{
			Total:         1,
			ByType:        map[unifiedresources.ResourceType]int{unifiedresources.ResourceTypeVM: 1},
			ByStatus:      map[unifiedresources.ResourceStatus]int{unifiedresources.StatusOnline: 1},
			BySource:      map[unifiedresources.DataSource]int{unifiedresources.SourceProxmox: 1},
			PolicyPosture: unifiedresources.EmptyResourcePolicyPostureSummary(),
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal resource response: %v", err)
	}

	const want = `{
		"data":[
			{
				"id":"vm-42",
				"type":"vm",
				"name":"web-42",
				"status":"online",
				"lastSeen":"2026-03-18T12:00:00Z",
				"updatedAt":"0001-01-01T00:00:00Z",
				"sources":null,
				"identity":{},
				"capabilities":[
					{
						"name":"restart",
						"type":"common",
						"description":"Restart the VM",
						"minimumApprovalLevel":"admin",
						"params":[
							{
								"name":"force",
								"type":"boolean",
								"required":false,
								"isSensitive":false,
								"description":"Restart without graceful shutdown"
							}
						]
					}
				],
				"relationships":[
					{
						"sourceId":"vm-42",
						"targetId":"node-1",
						"type":"runs_on",
						"confidence":1,
						"active":true,
						"discoverer":"proxmox_adapter",
						"observedAt":"2026-03-18T12:00:00Z",
						"lastSeenAt":"2026-03-18T12:00:00Z",
						"metadata":{"cluster":"pve-prod","source":"live"}
					}
				],
				"recentChanges":[
					{
						"id":"chg-42",
						"observedAt":"2026-03-18T12:00:00Z",
						"occurredAt":"2026-03-18T11:58:00Z",
						"resourceId":"vm-42",
						"kind":"state_transition",
						"from":"offline",
						"to":"online",
						"sourceType":"platform_event",
						"sourceAdapter":"proxmox_adapter",
						"confidence":"high",
						"relatedResources":["node-1"],
						"reason":"vm started",
						"metadata":{"source":"snapshot","ticket":"INC-1234"}
					}
				],
				"facetCounts":{
					"recentChanges":1
				}
			}
		],
		"meta":{"page":1,"limit":50,"total":1,"totalPages":1},
		"aggregations":{"total":1,"byType":{"vm":1},"byStatus":{"online":1},"bySource":{"proxmox":1},"policyPosture":{"totalResources":0,"sensitivityCounts":{},"routingCounts":{},"redactionCounts":{}}}
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceListUsesTenantStateProviderAtStartup(t *testing.T) {
	cfg := &config.Config{
		DataPath:   t.TempDir(),
		ConfigPath: t.TempDir(),
	}
	router := NewRouter(cfg, nil, &monitoring.MultiTenantMonitor{}, nil, nil, "1.0.0")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?page=1&limit=100", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "tenant-a"))

	router.resourceHandlers.HandleListResources(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestContract_ResourceCapabilitiesJSONSnapshot(t *testing.T) {
	payload := struct {
		ResourceID   string                                `json:"resourceId"`
		Capabilities []unifiedresources.ResourceCapability `json:"capabilities"`
		Count        int                                   `json:"count"`
	}{
		ResourceID: "vm:42",
		Capabilities: []unifiedresources.ResourceCapability{
			{
				Name:                 "restart",
				Type:                 unifiedresources.CapabilityTypeCommon,
				Description:          "Restart the VM",
				MinimumApprovalLevel: unifiedresources.ApprovalAdmin,
			},
		},
		Count: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal resource capabilities response: %v", err)
	}

	const want = `{
		"resourceId":"vm:42",
		"capabilities":[
			{
				"name":"restart",
				"type":"common",
				"description":"Restart the VM",
				"minimumApprovalLevel":"admin"
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_AgentExecWebSocketRejectsUnboundToken(t *testing.T) {
	rawToken := "contract-agent-unbound-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentExec}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ts := newIPv4HTTPServer(t, router.Handler())
	defer ts.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL)+"/api/agent/ws", wsHeadersForHTTP(t, ts.URL))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	regMsg, err := agentexec.NewMessage(agentexec.MsgTypeAgentRegister, "", agentexec.AgentRegisterPayload{
		AgentID:  "agent-node-1",
		Hostname: "node-1",
		Version:  "1.0.0",
		Platform: "linux",
		Token:    rawToken,
	})
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	if err := conn.WriteJSON(regMsg); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	reg := readRegisteredPayload(t, conn)
	if reg.Success {
		t.Fatalf("expected unbound agent exec token registration to fail closed")
	}
}

func TestContract_AgentExecWebSocketAcceptsLegacyHostnameBinding(t *testing.T) {
	rawToken := "contract-agent-legacy-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentExec}, map[string]string{"bound_hostname": "node-1"})
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ts := newIPv4HTTPServer(t, router.Handler())
	defer ts.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL)+"/api/agent/ws", wsHeadersForHTTP(t, ts.URL))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	regMsg, err := agentexec.NewMessage(agentexec.MsgTypeAgentRegister, "", agentexec.AgentRegisterPayload{
		AgentID:  "agent-node-1",
		Hostname: "node-1",
		Version:  "1.0.0",
		Platform: "linux",
		Token:    rawToken,
	})
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	if err := conn.WriteJSON(regMsg); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	reg := readRegisteredPayload(t, conn)
	if !reg.Success {
		t.Fatalf("expected legacy hostname binding to resolve to canonical agent binding, got %q", reg.Message)
	}
}

func TestContract_AgentExecWebSocketBindsProxmoxInstallTokenOnFirstUse(t *testing.T) {
	rawToken := "contract-agent-proxmox-install-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentExec, config.ScopeAgentReport}, map[string]string{
		"install_type": "pve",
		"issued_via":   agentInstallIssuedViaConfig,
	})
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ts := newIPv4HTTPServer(t, router.Handler())
	defer ts.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL)+"/api/agent/ws", wsHeadersForHTTP(t, ts.URL))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	regMsg, err := agentexec.NewMessage(agentexec.MsgTypeAgentRegister, "", agentexec.AgentRegisterPayload{
		AgentID:  "agent-delly",
		Hostname: "delly",
		Version:  "1.0.0",
		Platform: "linux",
		Token:    rawToken,
	})
	if err != nil {
		conn.Close()
		t.Fatalf("NewMessage: %v", err)
	}
	if err := conn.WriteJSON(regMsg); err != nil {
		conn.Close()
		t.Fatalf("WriteJSON: %v", err)
	}
	reg := readRegisteredPayload(t, conn)
	conn.Close()
	if !reg.Success {
		t.Fatalf("expected Pulse-minted Proxmox install token to bind on first command registration, got %q", reg.Message)
	}

	config.Mu.Lock()
	if got := cfg.APITokens[0].Metadata["bound_hostname"]; got != "delly" {
		config.Mu.Unlock()
		t.Fatalf("bound_hostname = %q, want delly", got)
	}
	if got := cfg.APITokens[0].Metadata["bound_agent_id"]; got != "agent-delly" {
		config.Mu.Unlock()
		t.Fatalf("bound_agent_id = %q, want agent-delly", got)
	}
	config.Mu.Unlock()

	conn, _, err = websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL)+"/api/agent/ws", wsHeadersForHTTP(t, ts.URL))
	if err != nil {
		t.Fatalf("Dial rebound: %v", err)
	}
	defer conn.Close()

	regMsg, err = agentexec.NewMessage(agentexec.MsgTypeAgentRegister, "", agentexec.AgentRegisterPayload{
		AgentID:  "agent-other",
		Hostname: "other-host",
		Version:  "1.0.0",
		Platform: "linux",
		Token:    rawToken,
	})
	if err != nil {
		t.Fatalf("NewMessage rebound: %v", err)
	}
	if err := conn.WriteJSON(regMsg); err != nil {
		t.Fatalf("WriteJSON rebound: %v", err)
	}
	reg = readRegisteredPayload(t, conn)
	if reg.Success {
		t.Fatalf("expected first-use-bound Proxmox install token to reject a different command agent")
	}
}

func TestContract_AgentConfigSuppressesCommandsForUnboundExecToken(t *testing.T) {
	handler, monitor := newUnifiedAgentHandlers(t, nil)
	hostID := seedUnifiedAgentHost(t, monitor)
	state := monitorState(t, monitor)
	state.UpsertHost(models.Host{
		ID:       hostID,
		Hostname: "node-1",
		TokenID:  "runtime-token",
	})

	commandsEnabled := true
	if err := monitor.UpdateHostAgentConfig(hostID, &commandsEnabled); err != nil {
		t.Fatalf("UpdateHostAgentConfig: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent/"+hostID+"/config", nil)
	attachAPITokenRecord(req, &config.APITokenRecord{
		ID:     "runtime-token",
		Scopes: []string{config.ScopeAgentConfigRead, config.ScopeAgentReport, config.ScopeAgentExec},
	})
	rec := httptest.NewRecorder()
	handler.HandleConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Config monitoring.HostAgentConfig `json:"config"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode config response: %v", err)
	}
	if resp.Config.CommandsEnabled == nil || *resp.Config.CommandsEnabled {
		t.Fatalf("commandsEnabled = %+v, want false for an unbound command token", resp.Config.CommandsEnabled)
	}
}

func TestContract_AgentConfigAllowsFirstUseProxmoxInstallCommandToken(t *testing.T) {
	handler, monitor := newUnifiedAgentHandlers(t, nil)
	hostID := seedUnifiedAgentHost(t, monitor)
	state := monitorState(t, monitor)
	state.UpsertHost(models.Host{
		ID:       hostID,
		Hostname: "pve-node-1",
		TokenID:  "proxmox-runtime-token",
	})

	commandsEnabled := true
	if err := monitor.UpdateHostAgentConfig(hostID, &commandsEnabled); err != nil {
		t.Fatalf("UpdateHostAgentConfig: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent/"+hostID+"/config", nil)
	attachAPITokenRecord(req, &config.APITokenRecord{
		ID:     "proxmox-runtime-token",
		Scopes: []string{config.ScopeAgentConfigRead, config.ScopeAgentReport, config.ScopeAgentExec},
		Metadata: map[string]string{
			"install_type": proxmoxInstallTypePVE,
			"issued_via":   agentInstallIssuedViaConfig,
		},
	})
	rec := httptest.NewRecorder()
	handler.HandleConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Config monitoring.HostAgentConfig `json:"config"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode config response: %v", err)
	}
	if resp.Config.CommandsEnabled == nil || !*resp.Config.CommandsEnabled {
		t.Fatalf("commandsEnabled = %+v, want true for a first-use Proxmox install command token", resp.Config.CommandsEnabled)
	}
}

func TestContract_AdminBypassFailsClosedOutsideDevMode(t *testing.T) {
	t.Setenv("ALLOW_ADMIN_BYPASS", "1")
	t.Setenv("PULSE_DEV", "")
	t.Setenv("NODE_ENV", "production")
	resetAdminBypassState()

	if adminBypassEnabled() {
		t.Fatal("expected admin bypass to fail closed outside development mode")
	}
}

func TestContract_ResourceRelationshipsJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 17, 0, 0, 0, time.UTC)
	payload := struct {
		ResourceID    string                                  `json:"resourceId"`
		Relationships []unifiedresources.ResourceRelationship `json:"relationships"`
		Count         int                                     `json:"count"`
	}{
		ResourceID: "vm:42",
		Relationships: []unifiedresources.ResourceRelationship{
			{
				SourceID:   "vm:42",
				TargetID:   "node-1",
				Type:       unifiedresources.RelRunsOn,
				Confidence: 1,
				Active:     true,
				Discoverer: "proxmox_adapter",
				ObservedAt: now,
				LastSeenAt: now,
				Metadata: map[string]any{
					"source":  "live",
					"cluster": "pve-prod",
				},
			},
		},
		Count: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal resource relationships response: %v", err)
	}

	const want = `{
		"resourceId":"vm:42",
		"relationships":[
			{
				"sourceId":"vm:42",
				"targetId":"node-1",
				"type":"runs_on",
				"confidence":1,
				"active":true,
				"discoverer":"proxmox_adapter",
				"observedAt":"2026-03-18T17:00:00Z",
				"lastSeenAt":"2026-03-18T17:00:00Z",
				"metadata":{"cluster":"pve-prod","source":"live"}
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceTimelineJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 17, 0, 0, 0, time.UTC)
	payload := struct {
		ResourceID    string                            `json:"resourceId"`
		RecentChanges []unifiedresources.ResourceChange `json:"recentChanges"`
		Count         int                               `json:"count"`
	}{
		ResourceID: "vm:42",
		RecentChanges: []unifiedresources.ResourceChange{
			{
				ID:               "chg-42",
				ResourceID:       "vm:42",
				ObservedAt:       now,
				OccurredAt:       &now,
				Kind:             unifiedresources.ChangeStateTransition,
				From:             "offline",
				To:               "online",
				SourceType:       unifiedresources.SourcePlatformEvent,
				SourceAdapter:    unifiedresources.AdapterProxmox,
				Confidence:       unifiedresources.ConfidenceHigh,
				RelatedResources: []string{"node-1"},
				Reason:           "vm started",
				Metadata:         map[string]any{"source": "snapshot", "ticket": "INC-1234"},
			},
		},
		Count: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal resource timeline response: %v", err)
	}

	const want = `{
		"resourceId":"vm:42",
		"recentChanges":[
			{
				"id":"chg-42",
				"observedAt":"2026-03-18T17:00:00Z",
				"occurredAt":"2026-03-18T17:00:00Z",
				"resourceId":"vm:42",
				"kind":"state_transition",
				"from":"offline",
				"to":"online",
				"sourceType":"platform_event",
				"sourceAdapter":"proxmox_adapter",
				"confidence":"high",
				"relatedResources":["node-1"],
				"reason":"vm started",
				"metadata":{"source":"snapshot","ticket":"INC-1234"}
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceTimelineRelationshipJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 17, 5, 0, 0, time.UTC)
	payload := struct {
		ResourceID    string                            `json:"resourceId"`
		RecentChanges []unifiedresources.ResourceChange `json:"recentChanges"`
		Count         int                               `json:"count"`
	}{
		ResourceID: "vm:42",
		RecentChanges: []unifiedresources.ResourceChange{
			{
				ID:               "chg-relationship-42",
				ObservedAt:       now,
				OccurredAt:       &now,
				ResourceID:       "vm:42",
				Kind:             unifiedresources.ChangeRelationship,
				From:             "node-1",
				To:               "node-2",
				SourceType:       unifiedresources.SourcePulseDiff,
				SourceAdapter:    unifiedresources.AdapterProxmox,
				Confidence:       unifiedresources.ConfidenceHigh,
				RelatedResources: []string{"db:alpha", "service:beta"},
				Reason:           "relationship updated",
				Metadata: map[string]any{
					"edgeType": "depends_on",
					"active":   true,
				},
			},
		},
		Count: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal resource timeline relationship response: %v", err)
	}

	const want = `{
		"resourceId":"vm:42",
		"recentChanges":[
			{
				"id":"chg-relationship-42",
				"observedAt":"2026-03-18T17:05:00Z",
				"occurredAt":"2026-03-18T17:05:00Z",
				"resourceId":"vm:42",
				"kind":"relationship_change",
				"from":"node-1",
				"to":"node-2",
				"sourceType":"pulse_diff",
				"sourceAdapter":"proxmox_adapter",
				"confidence":"high",
				"relatedResources":["db:alpha","service:beta"],
				"reason":"relationship updated",
				"metadata":{"active":true,"edgeType":"depends_on"}
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceTimelineRestartJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 17, 10, 0, 0, time.UTC)
	payload := struct {
		ResourceID    string                            `json:"resourceId"`
		RecentChanges []unifiedresources.ResourceChange `json:"recentChanges"`
		Count         int                               `json:"count"`
	}{
		ResourceID: "container:7",
		RecentChanges: []unifiedresources.ResourceChange{
			{
				ID:               "chg-restart-7",
				ObservedAt:       now,
				OccurredAt:       &now,
				ResourceID:       "container:7",
				Kind:             unifiedresources.ChangeRestart,
				From:             "online|docker.restartCount=1|docker.uptimeSeconds=3600",
				To:               "online|docker.restartCount=2|docker.uptimeSeconds=120",
				SourceType:       unifiedresources.SourcePlatformEvent,
				SourceAdapter:    unifiedresources.AdapterDocker,
				Confidence:       unifiedresources.ConfidenceHigh,
				RelatedResources: []string{"node:1", "service:api"},
				Reason:           "resource restart detected",
				Metadata: map[string]any{
					"changedFields": []string{"docker.restartCount", "docker.uptimeSeconds"},
				},
			},
		},
		Count: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal resource timeline restart response: %v", err)
	}

	const want = `{
		"resourceId":"container:7",
		"recentChanges":[
			{
				"id":"chg-restart-7",
				"observedAt":"2026-03-18T17:10:00Z",
				"occurredAt":"2026-03-18T17:10:00Z",
				"resourceId":"container:7",
				"kind":"restart",
				"from":"online|docker.restartCount=1|docker.uptimeSeconds=3600",
				"to":"online|docker.restartCount=2|docker.uptimeSeconds=120",
				"sourceType":"platform_event",
				"sourceAdapter":"docker_adapter",
				"confidence":"high",
				"relatedResources":["node:1","service:api"],
				"reason":"resource restart detected",
				"metadata":{"changedFields":["docker.restartCount","docker.uptimeSeconds"]}
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceTimelineAnomalyJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 17, 12, 0, 0, time.UTC)
	payload := struct {
		ResourceID    string                            `json:"resourceId"`
		RecentChanges []unifiedresources.ResourceChange `json:"recentChanges"`
		Count         int                               `json:"count"`
	}{
		ResourceID: "storage:1",
		RecentChanges: []unifiedresources.ResourceChange{
			{
				ID:               "chg-anomaly-1",
				ObservedAt:       now,
				OccurredAt:       &now,
				ResourceID:       "storage:1",
				Kind:             unifiedresources.ChangeAnomaly,
				From:             "none",
				To:               "capacity_runway_low[warning]:PBS datastore archive is READ_ONLY",
				SourceType:       unifiedresources.SourcePulseDiff,
				SourceAdapter:    unifiedresources.AdapterProxmox,
				Confidence:       unifiedresources.ConfidenceHigh,
				RelatedResources: []string{"node-2", "service:db"},
				Reason:           "resource incident changed",
				Metadata: map[string]any{
					"changedFields": []string{"incidents"},
				},
			},
		},
		Count: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal resource timeline anomaly response: %v", err)
	}

	const want = `{
		"resourceId":"storage:1",
		"recentChanges":[
			{
				"id":"chg-anomaly-1",
				"observedAt":"2026-03-18T17:12:00Z",
				"occurredAt":"2026-03-18T17:12:00Z",
				"resourceId":"storage:1",
				"kind":"metric_anomaly",
				"from":"none",
				"to":"capacity_runway_low[warning]:PBS datastore archive is READ_ONLY",
				"sourceType":"pulse_diff",
				"sourceAdapter":"proxmox_adapter",
				"confidence":"high",
				"relatedResources":["node-2","service:db"],
				"reason":"resource incident changed",
				"metadata":{"changedFields":["incidents"]}
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceFacetsJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 17, 0, 0, 0, time.UTC)
	payload := struct {
		ResourceID    string                                  `json:"resourceId"`
		Capabilities  []unifiedresources.ResourceCapability   `json:"capabilities,omitempty"`
		Relationships []unifiedresources.ResourceRelationship `json:"relationships,omitempty"`
		RecentChanges []unifiedresources.ResourceChange       `json:"recentChanges"`
		Counts        struct {
			RecentChanges              int                                          `json:"recentChanges"`
			RecentChangeKinds          map[unifiedresources.ChangeKind]int          `json:"recentChangeKinds"`
			RecentChangeSourceTypes    map[unifiedresources.ChangeSourceType]int    `json:"recentChangeSourceTypes"`
			RecentChangeSourceAdapters map[unifiedresources.ChangeSourceAdapter]int `json:"recentChangeSourceAdapters"`
		} `json:"counts"`
	}{
		ResourceID: "vm:42",
		Capabilities: []unifiedresources.ResourceCapability{
			{
				Name:                 "restart",
				Type:                 unifiedresources.CapabilityTypeCommon,
				Description:          "Restart the VM",
				MinimumApprovalLevel: unifiedresources.ApprovalAdmin,
			},
		},
		Relationships: []unifiedresources.ResourceRelationship{
			{
				SourceID:   "vm:42",
				TargetID:   "node-1",
				Type:       unifiedresources.RelRunsOn,
				Confidence: 1,
				Active:     true,
				Discoverer: "proxmox_adapter",
				ObservedAt: now,
				LastSeenAt: now,
				Metadata: map[string]any{
					"source":  "live",
					"cluster": "pve-prod",
				},
			},
		},
		RecentChanges: []unifiedresources.ResourceChange{
			{
				ID:               "chg-42",
				ResourceID:       "vm:42",
				ObservedAt:       now,
				OccurredAt:       &now,
				Kind:             unifiedresources.ChangeStateTransition,
				From:             "offline",
				To:               "online",
				SourceType:       unifiedresources.SourcePlatformEvent,
				SourceAdapter:    unifiedresources.AdapterProxmox,
				Confidence:       unifiedresources.ConfidenceHigh,
				RelatedResources: []string{"node-1"},
				Metadata: map[string]any{
					"source": "snapshot",
					"ticket": "INC-1234",
				},
			},
		},
		Counts: struct {
			RecentChanges              int                                          `json:"recentChanges"`
			RecentChangeKinds          map[unifiedresources.ChangeKind]int          `json:"recentChangeKinds"`
			RecentChangeSourceTypes    map[unifiedresources.ChangeSourceType]int    `json:"recentChangeSourceTypes"`
			RecentChangeSourceAdapters map[unifiedresources.ChangeSourceAdapter]int `json:"recentChangeSourceAdapters"`
		}{
			RecentChanges:     3,
			RecentChangeKinds: map[unifiedresources.ChangeKind]int{unifiedresources.ChangeRestart: 1, unifiedresources.ChangeAnomaly: 2},
			RecentChangeSourceTypes: map[unifiedresources.ChangeSourceType]int{
				unifiedresources.SourcePlatformEvent: 1,
				unifiedresources.SourcePulseDiff:     2,
			},
			RecentChangeSourceAdapters: map[unifiedresources.ChangeSourceAdapter]int{
				unifiedresources.AdapterProxmox: 1,
				unifiedresources.AdapterDocker:  2,
			},
		},
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal resource facets response: %v", err)
	}

	const want = `{
		"resourceId":"vm:42",
		"capabilities":[
			{
				"name":"restart",
				"type":"common",
				"description":"Restart the VM",
				"minimumApprovalLevel":"admin"
			}
		],
		"relationships":[
			{
				"sourceId":"vm:42",
				"targetId":"node-1",
				"type":"runs_on",
				"confidence":1,
				"active":true,
				"discoverer":"proxmox_adapter",
				"observedAt":"2026-03-18T17:00:00Z",
				"lastSeenAt":"2026-03-18T17:00:00Z",
				"metadata":{"cluster":"pve-prod","source":"live"}
			}
		],
		"recentChanges":[
			{
				"id":"chg-42",
				"observedAt":"2026-03-18T17:00:00Z",
				"occurredAt":"2026-03-18T17:00:00Z",
				"resourceId":"vm:42",
				"kind":"state_transition",
				"from":"offline",
				"to":"online",
				"sourceType":"platform_event",
				"sourceAdapter":"proxmox_adapter",
				"confidence":"high",
				"relatedResources":["node-1"],
				"metadata":{"source":"snapshot","ticket":"INC-1234"}
			}
		],
		"counts":{
			"recentChanges":3,
			"recentChangeKinds":{
				"metric_anomaly":2,
				"restart":1
			},
			"recentChangeSourceTypes":{
				"platform_event":1,
				"pulse_diff":2
			},
			"recentChangeSourceAdapters":{
				"docker_adapter":2,
				"proxmox_adapter":1
			}
		}
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ResourceFacetsDeriveCanonicalParentRelationship(t *testing.T) {
	now := time.Date(2026, 3, 18, 17, 0, 0, 0, time.UTC)
	parentID := "k8s-cluster-1"
	payload := struct {
		ResourceID    string                                  `json:"resourceId"`
		Relationships []unifiedresources.ResourceRelationship `json:"relationships,omitempty"`
	}{
		ResourceID: "agent-1",
		Relationships: unifiedresources.ResourceRelationshipsWithCanonicalParent(unifiedresources.Resource{
			ID:       "agent-1",
			Type:     unifiedresources.ResourceTypeAgent,
			ParentID: &parentID,
			LastSeen: now,
		}),
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal derived parent relationship facets response: %v", err)
	}

	const want = `{
		"resourceId":"agent-1",
		"relationships":[
			{
				"sourceId":"agent-1",
				"targetId":"k8s-cluster-1",
				"type":"owned_by",
				"confidence":1,
				"active":true,
				"discoverer":"resource_registry",
				"observedAt":"2026-03-18T17:00:00Z",
				"lastSeenAt":"2026-03-18T17:00:00Z",
				"metadata":{"source":"parentId"}
			}
		]
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ActionPlanJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	resource := unifiedresources.Resource{
		ID:        "vm:42",
		Type:      unifiedresources.ResourceTypeVM,
		Name:      "web-42",
		Status:    unifiedresources.StatusWarning,
		LastSeen:  now,
		UpdatedAt: now,
		Sources:   []unifiedresources.DataSource{unifiedresources.SourceProxmox},
		Capabilities: []unifiedresources.ResourceCapability{
			{
				Name:                 "restart",
				Type:                 unifiedresources.CapabilityTypeCommon,
				Description:          "Restart the VM",
				MinimumApprovalLevel: unifiedresources.ApprovalAdmin,
				InternalHandler:      "proxmox.vm.restart",
				Params: []unifiedresources.CapabilityParam{
					{Name: "mode", Type: "string", Required: true, Enum: []string{"graceful", "force"}},
				},
			},
		},
		Relationships: []unifiedresources.ResourceRelationship{
			{
				SourceID:   "vm:42",
				TargetID:   "node-1",
				Type:       unifiedresources.RelRunsOn,
				Confidence: 1,
				Active:     true,
				Discoverer: "proxmox_adapter",
				ObservedAt: now,
				LastSeenAt: now,
			},
		},
	}
	req := unifiedresources.ActionRequest{
		RequestID:      "agent-run-123",
		ResourceID:     "vm:42",
		CapabilityName: "restart",
		Params:         map[string]any{"mode": "graceful"},
		Reason:         "Recover after confirmed outage",
		RequestedBy:    "agent:oncall-helper",
	}

	plan, err := (actionplanner.Planner{Now: func() time.Time { return now }}).Plan(req, resource)
	if err != nil {
		t.Fatalf("plan action: %v", err)
	}
	got, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal action plan: %v", err)
	}

	const want = `{
		"actionId":"act_8f428171be2762cec97fd4546a291d5f",
		"requestId":"agent-run-123",
		"allowed":true,
		"requiresApproval":true,
		"approvalPolicy":"admin",
		"predictedBlastRadius":["vm:42","node-1"],
		"rollbackAvailable":false,
		"message":"Plan created for restart on web-42. Execution requires admin approval and is not performed by this endpoint.",
		"plannedAt":"2026-05-03T10:00:00Z",
		"expiresAt":"2026-05-03T10:05:00Z",
		"resourceVersion":"resource:sha256:54fb6f0264f42e0f2724e513",
		"policyVersion":"policy:sha256:0bce3cd2df181ace685598eb",
		"planHash":"sha256:69631faa9da67496a8b5953de2bc2dceb7b2aab5b582be3d1536e5d67683791b",
		"preflight":{
			"target":"vm:42",
			"currentState":"web-42 is warning",
			"intendedChange":"Restart the VM",
			"dryRunAvailable":false,
			"dryRunSummary":"No provider-supported dry run is advertised for this capability.",
			"safetyChecks":[
				"Resource was resolved from the unified resource registry.",
				"Capability is advertised by the resource contract.",
				"This endpoint plans only; it does not approve or execute the action.",
				"Execution requires admin approval."
			],
			"verificationSteps":[
				"Refresh the resource and confirm the expected state after execution.",
				"Review /api/audit/actions/act_8f428171be2762cec97fd4546a291d5f/events for lifecycle evidence."
			],
			"generatedAt":"2026-05-03T10:00:00Z"
		}
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_ActionPlanAuditLifecycleSnapshot(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	resource := unifiedresources.Resource{
		ID:        "vm:42",
		Type:      unifiedresources.ResourceTypeVM,
		Name:      "web-42",
		Status:    unifiedresources.StatusWarning,
		LastSeen:  now,
		UpdatedAt: now,
		Sources:   []unifiedresources.DataSource{unifiedresources.SourceProxmox},
		Capabilities: []unifiedresources.ResourceCapability{
			{
				Name:                 "restart",
				Type:                 unifiedresources.CapabilityTypeCommon,
				Description:          "Restart the VM",
				MinimumApprovalLevel: unifiedresources.ApprovalAdmin,
				InternalHandler:      "proxmox.vm.restart",
				Params: []unifiedresources.CapabilityParam{
					{Name: "mode", Type: "string", Required: true, Enum: []string{"graceful", "force"}},
				},
			},
		},
	}
	req := unifiedresources.ActionRequest{
		RequestID:      "agent-run-123",
		ResourceID:     "vm:42",
		CapabilityName: "restart",
		Params:         map[string]any{"mode": "graceful"},
		Reason:         "Recover after confirmed outage",
		RequestedBy:    "agent:oncall-helper",
	}
	plan, err := (actionplanner.Planner{Now: func() time.Time { return now }}).Plan(req, resource)
	if err != nil {
		t.Fatalf("plan action: %v", err)
	}

	store := unifiedresources.NewMemoryStore()
	if err := persistActionPlanAudit(store, req, plan); err != nil {
		t.Fatalf("persist action plan audit: %v", err)
	}
	audits, err := store.GetActionAudits("vm:42", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionAudits: %v", err)
	}
	events, err := store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	payload := struct {
		Audit struct {
			ID               string                               `json:"id"`
			State            unifiedresources.ActionState         `json:"state"`
			ResourceID       string                               `json:"resourceId"`
			RequestID        string                               `json:"requestId"`
			RequestedBy      string                               `json:"requestedBy"`
			ApprovalPolicy   unifiedresources.ActionApprovalLevel `json:"approvalPolicy"`
			PlanHash         string                               `json:"planHash"`
			PreflightSummary string                               `json:"preflightSummary"`
		} `json:"audit"`
		Events []struct {
			State   unifiedresources.ActionState `json:"state"`
			Actor   string                       `json:"actor"`
			Message string                       `json:"message"`
		} `json:"events"`
	}{}
	if len(audits) != 1 {
		t.Fatalf("audits len = %d, want 1", len(audits))
	}
	payload.Audit.ID = audits[0].ID
	payload.Audit.State = audits[0].State
	payload.Audit.ResourceID = audits[0].Request.ResourceID
	payload.Audit.RequestID = audits[0].Request.RequestID
	payload.Audit.RequestedBy = audits[0].Request.RequestedBy
	payload.Audit.ApprovalPolicy = audits[0].Plan.ApprovalPolicy
	payload.Audit.PlanHash = audits[0].Plan.PlanHash
	if audits[0].Plan.Preflight != nil {
		payload.Audit.PreflightSummary = audits[0].Plan.Preflight.DryRunSummary
	}
	for _, event := range events {
		payload.Events = append(payload.Events, struct {
			State   unifiedresources.ActionState `json:"state"`
			Actor   string                       `json:"actor"`
			Message string                       `json:"message"`
		}{
			State:   event.State,
			Actor:   event.Actor,
			Message: event.Message,
		})
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal persisted action audit contract: %v", err)
	}
	const want = `{
		"audit":{
			"id":"act_7eed41cdc58507f340151d2497707eca",
			"state":"pending_approval",
			"resourceId":"vm:42",
			"requestId":"agent-run-123",
			"requestedBy":"agent:oncall-helper",
			"approvalPolicy":"admin",
			"planHash":"sha256:f60417e39f967eb0803af0a3f2e2abd70f20d8a77a09c2e36976bdda34b6dbaf",
			"preflightSummary":"No provider-supported dry run is advertised for this capability."
		},
		"events":[
			{
				"state":"pending_approval",
				"actor":"agent:oncall-helper",
				"message":"Action is waiting for approval before execution."
			},
			{
				"state":"planned",
				"actor":"agent:oncall-helper",
				"message":"Action plan created."
			}
		]
	}`
	assertJSONSnapshot(t, got, want)
}

func TestContract_ActionDecisionJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 5, 4, 15, 0, 0, 0, time.UTC)
	plannedAt := now.Add(-time.Minute)
	record := unifiedresources.ActionAuditRecord{
		ID:        "act_decision_contract",
		CreatedAt: plannedAt,
		UpdatedAt: plannedAt,
		State:     unifiedresources.ActionStatePending,
		Request: unifiedresources.ActionRequest{
			RequestID:      "agent-run-approve",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "Recover after confirmed outage",
			RequestedBy:    "agent:oncall-helper",
		},
		Plan: unifiedresources.ActionPlan{
			ActionID:          "act_decision_contract",
			RequestID:         "agent-run-approve",
			Allowed:           true,
			RequiresApproval:  true,
			ApprovalPolicy:    unifiedresources.ApprovalAdmin,
			RollbackAvailable: false,
			PlannedAt:         plannedAt,
			ExpiresAt:         now.Add(4 * time.Minute),
			ResourceVersion:   "resource:sha256:contract",
			PolicyVersion:     "policy:sha256:contract",
			PlanHash:          "sha256:contract",
			Preflight: &unifiedresources.ActionPreflight{
				Target:          "vm:42",
				CurrentState:    "web-42 is warning",
				IntendedChange:  "Restart the VM",
				DryRunAvailable: false,
				DryRunSummary:   "No provider-supported dry run is advertised for this capability.",
				SafetyChecks: []string{
					"Resource was resolved from the unified resource registry.",
					"This endpoint plans only; it does not approve or execute the action.",
				},
				VerificationSteps: []string{
					"Review /api/audit/actions/act_decision_contract/events for lifecycle evidence.",
				},
				GeneratedAt: plannedAt,
			},
		},
	}

	updated, event, err := unifiedresources.ApplyActionDecision(record, unifiedresources.ActionApprovalRecord{
		Actor:   "operator@example.com",
		Outcome: unifiedresources.OutcomeApproved,
		Reason:  "inside maintenance window",
	}, now)
	if err != nil {
		t.Fatalf("apply action decision: %v", err)
	}
	response := actionDecisionResponse{
		ActionID: updated.ID,
		State:    updated.State,
		Approval: updated.Approvals[len(updated.Approvals)-1],
		Audit:    updated,
	}

	payload := struct {
		Response actionDecisionResponse `json:"response"`
		Event    struct {
			ActionID string                               `json:"actionId"`
			State    unifiedresources.ActionState         `json:"state"`
			Actor    string                               `json:"actor"`
			Message  string                               `json:"message"`
			Result   *unifiedresources.ExecutionResult    `json:"result,omitempty"`
			Method   unifiedresources.ApprovalMethod      `json:"method"`
			Outcome  unifiedresources.ApprovalOutcome     `json:"outcome"`
			Policy   unifiedresources.ActionApprovalLevel `json:"policy"`
		} `json:"event"`
	}{Response: response}
	payload.Event.ActionID = event.ActionID
	payload.Event.State = event.State
	payload.Event.Actor = event.Actor
	payload.Event.Message = event.Message
	payload.Event.Result = response.Audit.Result
	payload.Event.Method = response.Approval.Method
	payload.Event.Outcome = response.Approval.Outcome
	payload.Event.Policy = response.Audit.Plan.ApprovalPolicy

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal action decision contract: %v", err)
	}
	const want = `{
		"response":{
			"actionId":"act_decision_contract",
			"state":"approved",
			"approval":{
				"actor":"operator@example.com",
				"method":"api",
				"timestamp":"2026-05-04T15:00:00Z",
				"outcome":"approved",
				"reason":"inside maintenance window"
			},
			"audit":{
				"id":"act_decision_contract",
				"createdAt":"2026-05-04T14:59:00Z",
				"updatedAt":"2026-05-04T15:00:00Z",
				"state":"approved",
				"request":{
					"requestId":"agent-run-approve",
					"resourceId":"vm:42",
					"capabilityName":"restart",
					"reason":"Recover after confirmed outage",
					"requestedBy":"agent:oncall-helper"
				},
				"plan":{
					"actionId":"act_decision_contract",
					"requestId":"agent-run-approve",
					"allowed":true,
					"requiresApproval":true,
					"approvalPolicy":"admin",
					"rollbackAvailable":false,
					"plannedAt":"2026-05-04T14:59:00Z",
					"expiresAt":"2026-05-04T15:04:00Z",
					"resourceVersion":"resource:sha256:contract",
					"policyVersion":"policy:sha256:contract",
					"planHash":"sha256:contract",
					"preflight":{
						"target":"vm:42",
						"currentState":"web-42 is warning",
						"intendedChange":"Restart the VM",
						"dryRunAvailable":false,
						"dryRunSummary":"No provider-supported dry run is advertised for this capability.",
						"safetyChecks":[
							"Resource was resolved from the unified resource registry.",
							"This endpoint plans only; it does not approve or execute the action."
						],
						"verificationSteps":[
							"Review /api/audit/actions/act_decision_contract/events for lifecycle evidence."
						],
						"generatedAt":"2026-05-04T14:59:00Z"
					}
				},
				"approvals":[
					{
						"actor":"operator@example.com",
						"method":"api",
						"timestamp":"2026-05-04T15:00:00Z",
						"outcome":"approved",
						"reason":"inside maintenance window"
					}
				],
				"verificationOutcome":{
					"status":"unknown"
				}
			}
		},
		"event":{
			"actionId":"act_decision_contract",
			"state":"approved",
			"actor":"operator@example.com",
			"message":"Action approved. Execution remains pending a separate execution contract.",
			"method":"api",
			"outcome":"approved",
			"policy":"admin"
		}
	}`
	assertJSONSnapshot(t, got, want)
}

func TestContract_ActionExecutionJSONSnapshot(t *testing.T) {
	startedAt := time.Date(2026, 5, 4, 15, 30, 0, 0, time.UTC)
	completedAt := startedAt.Add(30 * time.Second)
	plannedAt := startedAt.Add(-2 * time.Minute)
	approvedAt := startedAt.Add(-time.Minute)
	record := unifiedresources.ActionAuditRecord{
		ID:        "act_execution_contract",
		CreatedAt: plannedAt,
		UpdatedAt: approvedAt,
		State:     unifiedresources.ActionStateApproved,
		Request: unifiedresources.ActionRequest{
			RequestID:      "agent-run-execute",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "Recover after confirmed outage",
			RequestedBy:    "agent:oncall-helper",
		},
		Plan: unifiedresources.ActionPlan{
			ActionID:          "act_execution_contract",
			RequestID:         "agent-run-execute",
			Allowed:           true,
			RequiresApproval:  true,
			ApprovalPolicy:    unifiedresources.ApprovalAdmin,
			RollbackAvailable: false,
			PlannedAt:         plannedAt,
			ExpiresAt:         startedAt.Add(3 * time.Minute),
			ResourceVersion:   "resource:sha256:contract",
			PolicyVersion:     "policy:sha256:contract",
			PlanHash:          "sha256:contract",
			Preflight: &unifiedresources.ActionPreflight{
				Target:          "vm:42",
				CurrentState:    "web-42 is warning",
				IntendedChange:  "Restart the VM",
				DryRunAvailable: false,
				DryRunSummary:   "No provider-supported dry run is advertised for this capability.",
				SafetyChecks: []string{
					"Resource was resolved from the unified resource registry.",
					"Execution must use POST /api/actions/{id}/execute after approval.",
				},
				VerificationSteps: []string{
					"Review /api/audit/actions/act_execution_contract/events for lifecycle evidence.",
				},
				GeneratedAt: plannedAt,
			},
		},
		Approvals: []unifiedresources.ActionApprovalRecord{
			{
				Actor:     "operator@example.com",
				Method:    unifiedresources.MethodAPI,
				Timestamp: approvedAt,
				Outcome:   unifiedresources.OutcomeApproved,
				Reason:    "inside maintenance window",
			},
		},
	}

	started, startEvent, err := unifiedresources.BeginActionExecution(record, "operator@example.com", startedAt)
	if err != nil {
		t.Fatalf("begin action execution: %v", err)
	}
	completed, doneEvent, err := unifiedresources.CompleteActionExecution(started, &unifiedresources.ExecutionResult{
		Success: true,
		Output:  "restart dispatched",
	}, "operator@example.com", completedAt)
	if err != nil {
		t.Fatalf("complete action execution: %v", err)
	}
	response := actionExecutionResponse{
		ActionID: completed.ID,
		State:    completed.State,
		Result:   completed.Result,
		Audit:    completed,
	}

	payload := struct {
		Response actionExecutionResponse `json:"response"`
		Events   []struct {
			ActionID string                       `json:"actionId"`
			State    unifiedresources.ActionState `json:"state"`
			Actor    string                       `json:"actor"`
			Message  string                       `json:"message"`
		} `json:"events"`
	}{Response: response}
	for _, event := range []unifiedresources.ActionLifecycleEvent{startEvent, doneEvent} {
		payload.Events = append(payload.Events, struct {
			ActionID string                       `json:"actionId"`
			State    unifiedresources.ActionState `json:"state"`
			Actor    string                       `json:"actor"`
			Message  string                       `json:"message"`
		}{
			ActionID: event.ActionID,
			State:    event.State,
			Actor:    event.Actor,
			Message:  event.Message,
		})
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal action execution contract: %v", err)
	}
	const want = `{
		"response":{
			"actionId":"act_execution_contract",
			"state":"completed",
			"result":{
				"success":true,
				"output":"restart dispatched"
			},
			"audit":{
				"id":"act_execution_contract",
				"createdAt":"2026-05-04T15:28:00Z",
				"updatedAt":"2026-05-04T15:30:30Z",
				"state":"completed",
				"request":{
					"requestId":"agent-run-execute",
					"resourceId":"vm:42",
					"capabilityName":"restart",
					"reason":"Recover after confirmed outage",
					"requestedBy":"agent:oncall-helper"
				},
				"plan":{
					"actionId":"act_execution_contract",
					"requestId":"agent-run-execute",
					"allowed":true,
					"requiresApproval":true,
					"approvalPolicy":"admin",
					"rollbackAvailable":false,
					"plannedAt":"2026-05-04T15:28:00Z",
					"expiresAt":"2026-05-04T15:33:00Z",
					"resourceVersion":"resource:sha256:contract",
					"policyVersion":"policy:sha256:contract",
					"planHash":"sha256:contract",
					"preflight":{
						"target":"vm:42",
						"currentState":"web-42 is warning",
						"intendedChange":"Restart the VM",
						"dryRunAvailable":false,
						"dryRunSummary":"No provider-supported dry run is advertised for this capability.",
						"safetyChecks":[
							"Resource was resolved from the unified resource registry.",
							"Execution must use POST /api/actions/{id}/execute after approval."
						],
						"verificationSteps":[
							"Review /api/audit/actions/act_execution_contract/events for lifecycle evidence."
						],
						"generatedAt":"2026-05-04T15:28:00Z"
					}
				},
				"approvals":[
					{
						"actor":"operator@example.com",
						"method":"api",
						"timestamp":"2026-05-04T15:29:00Z",
						"outcome":"approved",
						"reason":"inside maintenance window"
					}
				],
				"result":{
					"success":true,
					"output":"restart dispatched"
				},
				"verificationOutcome":{
					"status":"unknown"
				}
			}
		},
		"events":[
			{
				"actionId":"act_execution_contract",
				"state":"executing",
				"actor":"operator@example.com",
				"message":"Action execution started."
			},
			{
				"actionId":"act_execution_contract",
				"state":"completed",
				"actor":"operator@example.com",
				"message":"Action execution completed."
			}
		]
	}`
	assertJSONSnapshot(t, got, want)
}

func TestContract_ActionDryRunOnlyExecutionErrorJSONSnapshot(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("get store: %v", err)
	}

	record := unifiedresources.ActionAuditRecord{
		ID:        "act_dry_run_contract",
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
		State:     unifiedresources.ActionStatePlanned,
		Request: unifiedresources.ActionRequest{
			RequestID:      "agent-run-dry-run",
			ResourceID:     "vm:42",
			CapabilityName: "restart",
			Reason:         "Inspect possible restart remediation",
			RequestedBy:    "agent:oncall-helper",
		},
		Plan: unifiedresources.ActionPlan{
			ActionID:          "act_dry_run_contract",
			RequestID:         "agent-run-dry-run",
			Allowed:           true,
			RequiresApproval:  false,
			ApprovalPolicy:    unifiedresources.ApprovalDryRun,
			RollbackAvailable: false,
			PlannedAt:         now.Add(-time.Minute),
			ExpiresAt:         now.Add(4 * time.Minute),
			ResourceVersion:   "resource:sha256:dry-run-contract",
			PolicyVersion:     "policy:sha256:dry-run-contract",
			PlanHash:          "sha256:dry-run-contract",
			Preflight: &unifiedresources.ActionPreflight{
				Target:          "vm:42",
				CurrentState:    "web-42 is warning",
				IntendedChange:  "Dry-run only restart inspection",
				DryRunAvailable: true,
				DryRunSummary:   "Provider advertised dry-run only; no execution is allowed.",
				SafetyChecks: []string{
					"Dry-run-only plans are not executable.",
				},
				VerificationSteps: []string{
					"Review /api/audit/actions/act_dry_run_contract/events for lifecycle evidence.",
				},
				GeneratedAt: now.Add(-time.Minute),
			},
		},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("record action audit: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/actions/act_dry_run_contract/execute", bytes.NewBufferString(`{}`))
	req.SetPathValue("id", "act_dry_run_contract")
	rec := httptest.NewRecorder()
	h.HandleExecuteAction(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	// The action endpoints now emit the agent-stable error
	// envelope (slice 58 refactor): the stable code lives under
	// the top-level `error` field and the human message lives
	// under `message`. The previous APIError shape (code under
	// `code`, message under `error`) is gone for these handlers
	// because they joined the agent surface.
	var envelope struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode dry-run execution error: %v", err)
	}
	payload := struct {
		Status   int    `json:"status"`
		ErrorKey string `json:"error"`
		Message  string `json:"message"`
	}{
		Status:   rec.Code,
		ErrorKey: envelope.Error,
		Message:  envelope.Message,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal dry-run execution error contract: %v", err)
	}
	const want = `{
		"status":409,
		"error":"action_dry_run_only",
		"message":"Action plan is dry-run only and cannot be executed"
	}`
	assertJSONSnapshot(t, got, want)

	gotAudit, ok, err := store.GetActionAudit("act_dry_run_contract")
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok || gotAudit.State != unifiedresources.ActionStateFailed || gotAudit.Result == nil || gotAudit.Result.Success || !strings.HasPrefix(gotAudit.Result.ErrorMessage, "action_dry_run_only:") {
		t.Fatalf("dry-run audit mutated: ok=%v state=%q result=%#v", ok, gotAudit.State, gotAudit.Result)
	}
	events, err := store.GetActionLifecycleEvents("act_dry_run_contract", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetActionLifecycleEvents: %v", err)
	}
	if len(events) != 1 || events[0].State != unifiedresources.ActionStateFailed || !strings.HasPrefix(events[0].Message, "action_dry_run_only:") {
		t.Fatalf("dry-run execution should append one failed lifecycle event, got %#v", events)
	}
}

func TestContract_APIActionExecutionRevalidatesPlanFreshness(t *testing.T) {
	source, err := os.ReadFile("actions.go")
	if err != nil {
		t.Fatalf("read actions.go: %v", err)
	}
	src := string(source)
	for _, snippet := range []string{
		"if unified.IsPermanentActionExecutionRefusal(err)",
		"if err := h.validateActionPlanFresh(orgID, record); err != nil",
		"errors.Is(err, unified.ErrActionPlanDrift)",
		"recordRefusedActionExecution(store, record, actor, now, err)",
		"unified.RefuseActionExecution(record, reason, actor, now)",
		"writeJSONError(w, http.StatusConflict, agentcapabilities.AgentErrCodeActionPlanDrift",
	} {
		if !strings.Contains(src, snippet) {
			t.Fatalf("actions.go must pin API execute plan freshness guard snippet %q", snippet)
		}
	}
	if strings.Index(src, "if err := h.validateActionPlanFresh(orgID, record); err != nil") >
		strings.Index(src, "started, startEvent, err := unified.BeginActionExecution(record, actor, now)") {
		t.Fatal("HandleExecuteAction must validate plan freshness before entering executing state or calling the executor")
	}
}

func readAgentCapabilitiesManifestSource(t *testing.T) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "manifest.go"))
	if err != nil {
		t.Fatalf("read shared agent capabilities manifest: %v", err)
	}
	return string(source)
}

func TestContract_ExecuteActionCapabilityDeclaresPlanExpired(t *testing.T) {
	manifest := agentcapabilities.CanonicalManifest()
	var executeAction agentcapabilities.Capability
	for _, cap := range manifest.Capabilities {
		if cap.Name == agentcapabilities.ExecuteActionCapabilityName {
			executeAction = cap
			break
		}
	}
	if executeAction.Name == "" {
		t.Fatalf("agent capabilities manifest must declare %s", agentcapabilities.ExecuteActionCapabilityName)
	}
	if !stringSliceContains(executeAction.ErrorCodes, agentcapabilities.AgentErrCodeActionPlanExpired) {
		t.Error("execute_action manifest must declare action_plan_expired so agents can branch on permanent expired-plan refusals")
	}
}

func TestContract_ResourceTimelineEndpointsIncludeRelatedChanges(t *testing.T) {
	now := time.Date(2026, 4, 25, 22, 15, 0, 0, time.UTC)
	h := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now},
		resources: []unifiedresources.Resource{
			{
				ID:       "node-contract-relationship",
				Type:     unifiedresources.ResourceTypeAgent,
				Name:     "node-contract-relationship",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
			},
			{
				ID:       "vm-contract-relationship",
				Type:     unifiedresources.ResourceTypeVM,
				Name:     "vm-contract-relationship",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
			},
		},
	})

	store, err := h.getStore("default")
	if err != nil {
		t.Fatalf("get resource store: %v", err)
	}
	for _, change := range []unifiedresources.ResourceChange{
		{
			ID:               "change-related-contract",
			ResourceID:       "vm-contract-relationship",
			ObservedAt:       now,
			Kind:             unifiedresources.ChangeRestart,
			SourceType:       unifiedresources.SourcePlatformEvent,
			SourceAdapter:    unifiedresources.AdapterProxmox,
			Confidence:       unifiedresources.ConfidenceHigh,
			RelatedResources: []string{" node-contract-relationship "},
		},
		{
			ID:            "change-direct-contract",
			ResourceID:    "node-contract-relationship",
			ObservedAt:    now.Add(-time.Minute),
			Kind:          unifiedresources.ChangeStateTransition,
			SourceType:    unifiedresources.SourcePulseDiff,
			SourceAdapter: unifiedresources.AdapterProxmox,
			Confidence:    unifiedresources.ConfidenceMedium,
		},
	} {
		if err := store.RecordChange(change); err != nil {
			t.Fatalf("record %s: %v", change.ID, err)
		}
	}

	timelineRec := httptest.NewRecorder()
	timelineReq := httptest.NewRequest(http.MethodGet, "/api/resources/node-contract-relationship/timeline?limit=10", nil)
	h.HandleResourceRoutes(timelineRec, timelineReq)
	if timelineRec.Code != http.StatusOK {
		t.Fatalf("timeline status = %d, body=%s", timelineRec.Code, timelineRec.Body.String())
	}
	var timeline struct {
		ResourceID    string                            `json:"resourceId"`
		RecentChanges []unifiedresources.ResourceChange `json:"recentChanges"`
		Count         int                               `json:"count"`
	}
	if err := json.NewDecoder(timelineRec.Body).Decode(&timeline); err != nil {
		t.Fatalf("decode relationship-aware timeline: %v", err)
	}
	if timeline.ResourceID != "node-contract-relationship" || timeline.Count != 2 || len(timeline.RecentChanges) != 2 {
		t.Fatalf("unexpected relationship-aware timeline: %#v", timeline)
	}
	if timeline.RecentChanges[0].ID != "change-related-contract" || timeline.RecentChanges[0].ResourceID != "vm-contract-relationship" {
		t.Fatalf("timeline did not preserve related originating resource: %#v", timeline.RecentChanges)
	}

	facetsRec := httptest.NewRecorder()
	facetsReq := httptest.NewRequest(http.MethodGet, "/api/resources/node-contract-relationship/facets?kind=restart&limit=10", nil)
	h.HandleResourceRoutes(facetsRec, facetsReq)
	if facetsRec.Code != http.StatusOK {
		t.Fatalf("facets status = %d, body=%s", facetsRec.Code, facetsRec.Body.String())
	}
	var facets struct {
		ResourceID    string                            `json:"resourceId"`
		RecentChanges []unifiedresources.ResourceChange `json:"recentChanges"`
		Counts        struct {
			RecentChanges     int                                          `json:"recentChanges"`
			RecentChangeKinds map[unifiedresources.ChangeKind]int          `json:"recentChangeKinds"`
			RecentAdapters    map[unifiedresources.ChangeSourceAdapter]int `json:"recentChangeSourceAdapters"`
		} `json:"counts"`
	}
	if err := json.NewDecoder(facetsRec.Body).Decode(&facets); err != nil {
		t.Fatalf("decode relationship-aware facets: %v", err)
	}
	if facets.ResourceID != "node-contract-relationship" || facets.Counts.RecentChanges != 1 || len(facets.RecentChanges) != 1 {
		t.Fatalf("unexpected relationship-aware facets: %#v", facets)
	}
	if facets.RecentChanges[0].ID != "change-related-contract" {
		t.Fatalf("facets did not include related restart: %#v", facets.RecentChanges)
	}
	if got := facets.Counts.RecentChangeKinds[unifiedresources.ChangeRestart]; got != 1 {
		t.Fatalf("restart facet count = %d, want 1", got)
	}
	if got := facets.Counts.RecentAdapters[unifiedresources.AdapterProxmox]; got != 1 {
		t.Fatalf("adapter facet count = %d, want 1", got)
	}
}

func TestContract_ResourceTimelineRejectsInvalidSourceAdapter(t *testing.T) {
	_, err := unifiedresources.ParseResourceChangeFilters(nil, nil, []string{"unsupported_adapter"})
	if err == nil {
		t.Fatal("expected invalid sourceAdapter to be rejected")
	}
}

func TestContract_UnifiedActionAuditsJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 16, 0, 0, 0, time.UTC)
	payload := struct {
		Audits     []unifiedresources.ActionAuditRecord `json:"audits"`
		Count      int                                  `json:"count"`
		ResourceID string                               `json:"resourceId,omitempty"`
	}{
		Audits: []unifiedresources.ActionAuditRecord{
			{
				ID:        "action-1",
				CreatedAt: now,
				UpdatedAt: now,
				State:     unifiedresources.ActionStateCompleted,
				Request: unifiedresources.ActionRequest{
					RequestID:      "req-1",
					ResourceID:     "vm:42",
					CapabilityName: "restart",
					Reason:         "maintenance",
					RequestedBy:    "agent:ops",
				},
				Plan: unifiedresources.ActionPlan{
					ActionID:          "action-1",
					RequestID:         "req-1",
					Allowed:           true,
					RequiresApproval:  false,
					ApprovalPolicy:    unifiedresources.ApprovalNone,
					RollbackAvailable: false,
					PlannedAt:         now,
					ExpiresAt:         now.Add(5 * time.Minute),
					ResourceVersion:   "rv-1",
					PolicyVersion:     "pv-1",
					PlanHash:          "hash-1",
					Preflight: &unifiedresources.ActionPreflight{
						Target:            "vm:42",
						CurrentState:      "online",
						IntendedChange:    "restart",
						DryRunAvailable:   false,
						DryRunSummary:     "No provider-supported dry run is available for this action.",
						SafetyChecks:      []string{"Approval and execution are scoped to vm:42."},
						VerificationSteps: []string{"Read back VM status after execution."},
						GeneratedAt:       now,
					},
				},
				Approvals: []unifiedresources.ActionApprovalRecord{
					{
						Actor:     "admin@example.com",
						Method:    unifiedresources.MethodUI,
						Timestamp: now.Add(time.Minute),
						Outcome:   unifiedresources.OutcomeApproved,
						Reason:    "approved",
					},
				},
				Result: &unifiedresources.ExecutionResult{
					Success: true,
					Output:  "done",
				},
				VerificationOutcome: unifiedresources.VerificationOutcome{
					Status: unifiedresources.VerificationUnknown,
				},
			},
		},
		Count:      1,
		ResourceID: "vm:42",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal action audits response: %v", err)
	}

	const want = `{
		"audits":[
			{
				"id":"action-1",
				"createdAt":"2026-03-18T16:00:00Z",
				"updatedAt":"2026-03-18T16:00:00Z",
				"state":"completed",
				"request":{
					"requestId":"req-1",
					"resourceId":"vm:42",
					"capabilityName":"restart",
					"reason":"maintenance",
					"requestedBy":"agent:ops"
				},
				"plan":{
					"actionId":"action-1",
					"requestId":"req-1",
					"allowed":true,
					"requiresApproval":false,
					"approvalPolicy":"none",
					"rollbackAvailable":false,
					"plannedAt":"2026-03-18T16:00:00Z",
					"expiresAt":"2026-03-18T16:05:00Z",
					"resourceVersion":"rv-1",
					"policyVersion":"pv-1",
					"planHash":"hash-1",
					"preflight":{
						"target":"vm:42",
						"currentState":"online",
						"intendedChange":"restart",
						"dryRunAvailable":false,
						"dryRunSummary":"No provider-supported dry run is available for this action.",
						"safetyChecks":["Approval and execution are scoped to vm:42."],
						"verificationSteps":["Read back VM status after execution."],
						"generatedAt":"2026-03-18T16:00:00Z"
					}
				},
				"approvals":[
					{
						"actor":"admin@example.com",
						"method":"ui",
						"timestamp":"2026-03-18T16:01:00Z",
						"outcome":"approved",
						"reason":"approved"
					}
				],
				"result":{
					"success":true,
					"output":"done"
				},
				"verificationOutcome":{
					"status":"unknown"
				}
			}
		],
		"count":1,
		"resourceId":"vm:42"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_UnifiedActionLifecycleEventsJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 16, 0, 0, 0, time.UTC)
	payload := struct {
		ActionID string                                  `json:"actionId"`
		Events   []unifiedresources.ActionLifecycleEvent `json:"events"`
		Count    int                                     `json:"count"`
	}{
		ActionID: "action-1",
		Events: []unifiedresources.ActionLifecycleEvent{
			{
				ActionID:  "action-1",
				Timestamp: now,
				State:     unifiedresources.ActionStatePlanned,
				Actor:     "system",
				Message:   "planned",
			},
		},
		Count: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal action lifecycle response: %v", err)
	}

	const want = `{
		"actionId":"action-1",
		"events":[
			{
				"actionId":"action-1",
				"timestamp":"2026-03-18T16:00:00Z",
				"state":"planned",
				"actor":"system",
				"message":"planned"
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_UnifiedExportAuditsJSONSnapshot(t *testing.T) {
	now := time.Date(2026, 3, 18, 16, 0, 0, 0, time.UTC)
	payload := struct {
		Audits []unifiedresources.ExportAuditRecord `json:"audits"`
		Count  int                                  `json:"count"`
	}{
		Audits: []unifiedresources.ExportAuditRecord{
			{
				ID:           "export-1",
				Timestamp:    now,
				Actor:        "agent:ops",
				EnvelopeHash: "hash-1",
				Decision:     unifiedresources.ExportRedacted,
				Destination:  "local-llama",
				Redactions:   []string{"metadata.hostname"},
			},
		},
		Count: 1,
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal export audits response: %v", err)
	}

	const want = `{
		"audits":[
			{
				"id":"export-1",
				"timestamp":"2026-03-18T16:00:00Z",
				"actor":"agent:ops",
				"envelopeHash":"hash-1",
				"decision":"redacted",
				"destination":"local-llama",
				"redactions":["metadata.hostname"]
			}
		],
		"count":1
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_UnifiedAuditLimitCapsOversizedRequests(t *testing.T) {
	if got := parseAuditLimit("5000", 100); got != 1000 {
		t.Fatalf("parseAuditLimit oversized request = %d, want 1000", got)
	}
	if got := parseAuditLimit("250", 100); got != 250 {
		t.Fatalf("parseAuditLimit normal request = %d, want 250", got)
	}
}

func TestContract_AuditStoreReadErrorsUseStructuredAPIErrorCodes(t *testing.T) {
	cases := []struct {
		name           string
		err            error
		wantStatus     int
		wantCode       string
		wantRetryAfter bool
	}{
		{
			name:           "busy",
			err:            fmt.Errorf("failed to query audit events: %w", fmt.Errorf("database is locked (5) (SQLITE_BUSY)")),
			wantStatus:     http.StatusServiceUnavailable,
			wantCode:       "audit_store_busy",
			wantRetryAfter: true,
		},
		{
			name:       "unavailable",
			err:        fmt.Errorf("failed to query audit events: %w", fmt.Errorf("no such table: audit_events")),
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "audit_store_unavailable",
		},
		{
			name:       "query_failed",
			err:        fmt.Errorf("failed to query audit events: %w", fmt.Errorf("unexpected query failure")),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "query_failed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.wantCode == "audit_store_busy" && !audit.IsStoreBusyError(tc.err) {
				t.Fatal("busy fixture must satisfy audit store busy classification")
			}
			if tc.wantCode == "audit_store_unavailable" && !audit.IsStoreUnavailableError(tc.err) {
				t.Fatal("unavailable fixture must satisfy audit store unavailable classification")
			}

			rec := httptest.NewRecorder()
			writeAuditReadErrorResponse(rec, tc.err, "Failed to query audit events")
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Header().Get("Retry-After"); tc.wantRetryAfter && got == "" {
				t.Fatal("busy audit-store errors must include Retry-After")
			} else if !tc.wantRetryAfter && got != "" {
				t.Fatalf("Retry-After = %q, want empty", got)
			}

			var payload APIError
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode APIError: %v", err)
			}
			if payload.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q", payload.Code, tc.wantCode)
			}
			if payload.StatusCode != tc.wantStatus {
				t.Fatalf("status_code = %d, want %d", payload.StatusCode, tc.wantStatus)
			}
		})
	}
}

func TestContract_EmbeddedFrontendWarningUsesCanonicalDevEntrypoints(t *testing.T) {
	path := filepath.Join("DO_NOT_EDIT_FRONTEND_HERE.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read embedded frontend warning: %v", err)
	}

	text := string(body)
	if !strings.Contains(text, "http://127.0.0.1:5173") {
		t.Fatalf("embedded frontend warning must point to the frontend dev shell on 5173")
	}
	if !strings.Contains(text, "http://127.0.0.1:7655") {
		t.Fatalf("embedded frontend warning must identify the backend on 7655")
	}
	if strings.Contains(text, "The dev server (port 7655) will hot-reload") {
		t.Fatalf("embedded frontend warning must not describe 7655 as the hot-reload dev server")
	}
}

func TestContract_ShippedSecurityDocReferencesStayLocal(t *testing.T) {
	if shippedSecurityDocPath != "/docs/SECURITY.md" {
		t.Fatalf("expected shipped security doc path, got %q", shippedSecurityDocPath)
	}
	if shippedSecurityContainerNoticeDocAnchor != "/docs/SECURITY.md#critical-security-notice-for-container-deployments" {
		t.Fatalf("expected shipped security container notice path, got %q", shippedSecurityContainerNoticeDocAnchor)
	}
}

// TestContract_ConnectionPayloadShapeStaysCanonical pins the JSON shape of
// the unified connections ledger so the Go types in
// `internal/api/connections_types.go` stay in lockstep with the
// `frontend-modern/src/api/connections.ts` client. Adding or renaming fields
// on this wire shape requires an explicit update to both ends in the same
// commit.
func TestContract_ConnectionPayloadShapeStaysCanonical(t *testing.T) {
	conn := Connection{
		ID:          "pve-lab",
		Type:        ConnectionTypePVE,
		Name:        "lab",
		Address:     "https://pve.lab:8006",
		State:       ConnectionStateActive,
		StateReason: "",
		Enabled:     true,
		Surfaces:    []string{"vms", "containers"},
		Scope:       map[string]bool{"vms": true, "containers": true},
		LastSeen:    timePtr(time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC)),
		LastError:   nil,
		Source:      ConnectionSourceManual,
		Fleet: ConnectionFleetGovernance{
			EnrollmentState:  fleetStateConfigured,
			LivenessState:    fleetStateActive,
			VersionDrift:     fleetStateNotApplicable,
			AdapterHealth:    fleetStateHealthy,
			ConfigRollout:    fleetStateConfigured,
			CredentialStatus: fleetStateVerified,
			UpdateStatus:     fleetStateNotApplicable,
			RemoteControl:    fleetStateNotApplicable,
		},
		Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
	}
	system := ConnectionSystem{
		ID:          "pve-lab",
		Type:        ConnectionTypePVE,
		ClusterName: "homelab",
		Components: []ConnectionSystemComponent{
			{
				ConnectionID: "pve-lab",
				Type:         ConnectionTypePVE,
				Role:         ConnectionSystemComponentRolePrimary,
			},
		},
	}
	body, err := json.Marshal(ConnectionsListResponse{
		Connections: []Connection{conn},
		Systems:     []ConnectionSystem{system},
	})
	if err != nil {
		t.Fatalf("marshal Connection: %v", err)
	}
	want := `{"connections":[{"id":"pve-lab","type":"pve","name":"lab","address":"https://pve.lab:8006","state":"active","enabled":true,"surfaces":["vms","containers"],"scope":{"containers":true,"vms":true},"lastSeen":"2026-04-19T10:00:00Z","source":"manual","fleet":{"enrollmentState":"configured","livenessState":"active","versionDrift":"not-applicable","adapterHealth":"healthy","configRollout":"configured","credentialStatus":"verified","updateStatus":"not-applicable","remoteControl":"not-applicable"},"capabilities":{"supportsPause":true,"supportsScope":true,"supportsTest":true}}],"systems":[{"id":"pve-lab","type":"pve","clusterName":"homelab","components":[{"connectionId":"pve-lab","type":"pve","role":"primary"}]}]}`
	assertJSONSnapshot(t, body, want)
}

func TestContract_ConnectionSystemMembersPayloadShapeStaysCanonical(t *testing.T) {
	system := ConnectionSystem{
		ID:          "pve-lab",
		Type:        ConnectionTypePVE,
		ClusterName: "homelab",
		Components: []ConnectionSystemComponent{
			{
				ConnectionID: "pve-lab",
				Type:         ConnectionTypePVE,
				Role:         ConnectionSystemComponentRolePrimary,
			},
			{
				ConnectionID: "agent:agent-lab",
				Type:         ConnectionTypeAgent,
				Role:         ConnectionSystemComponentRoleAttachment,
			},
		},
		Members: []ConnectionSystemMember{
			{
				ID:                "node-lab",
				Name:              "lab",
				Endpoint:          "https://lab:8006",
				HostAliases:       []string{"lab", "192.168.0.2"},
				State:             ConnectionStateActive,
				LastSeen:          timePtr(time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)),
				Primary:           true,
				AgentConnectionID: "agent:agent-lab",
			},
			{
				ID:          "node-minipc",
				Name:        "minipc",
				Endpoint:    "https://minipc:8006",
				HostAliases: []string{"minipc"},
				State:       ConnectionStateStale,
				LastSeen:    timePtr(time.Date(2026, 4, 23, 11, 55, 0, 0, time.UTC)),
			},
		},
	}

	body, err := json.Marshal(system)
	if err != nil {
		t.Fatalf("marshal ConnectionSystem with members: %v", err)
	}

	want := `{"id":"pve-lab","type":"pve","clusterName":"homelab","components":[{"connectionId":"pve-lab","type":"pve","role":"primary"},{"connectionId":"agent:agent-lab","type":"agent","role":"attachment"}],"members":[{"id":"node-lab","name":"lab","endpoint":"https://lab:8006","hostAliases":["lab","192.168.0.2"],"state":"active","lastSeen":"2026-04-23T12:00:00Z","primary":true,"agentConnectionId":"agent:agent-lab"},{"id":"node-minipc","name":"minipc","endpoint":"https://minipc:8006","hostAliases":["minipc"],"state":"stale","lastSeen":"2026-04-23T11:55:00Z"}]}`
	assertJSONSnapshot(t, body, want)
}

func TestContract_ConnectionSystemHostAgentAttachmentPayloadShapeStaysCanonical(t *testing.T) {
	response := ConnectionsListResponse{
		Connections: []Connection{
			{
				ID:          "pve:delly",
				Type:        ConnectionTypePVE,
				Name:        "delly",
				Address:     "https://delly:8006",
				HostAliases: []string{"delly"},
				State:       ConnectionStateUnauthorized,
				Enabled:     true,
				Surfaces:    []string{"vms", "containers", "storage", "backups"},
				Scope:       map[string]bool{"vms": true, "containers": true, "storage": true, "backups": true},
				Source:      ConnectionSourceAgent,
				Fleet: ConnectionFleetGovernance{
					EnrollmentState:  fleetStateConfigured,
					LivenessState:    string(ConnectionStateUnauthorized),
					VersionDrift:     fleetStateNotApplicable,
					AdapterHealth:    fleetStateBlocked,
					ConfigRollout:    fleetStateConfigured,
					CredentialStatus: fleetStateInvalid,
					UpdateStatus:     fleetStateNotApplicable,
					RemoteControl:    fleetStateNotApplicable,
				},
				Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
			},
			{
				ID:          "agent:agent-delly",
				Type:        ConnectionTypeAgent,
				Name:        "delly",
				Address:     "delly",
				HostAliases: []string{"delly", "192.168.0.5"},
				State:       ConnectionStateActive,
				Enabled:     true,
				Surfaces:    []string{"host"},
				Scope:       map[string]bool{"host": true},
				LastSeen:    timePtr(time.Date(2026, 5, 13, 23, 45, 0, 0, time.UTC)),
				Source:      ConnectionSourceAgent,
				Fleet: ConnectionFleetGovernance{
					EnrollmentState:  fleetStateEnrolled,
					LivenessState:    fleetStateActive,
					VersionDrift:     fleetStateUnknown,
					AdapterHealth:    fleetStateHealthy,
					ConfigRollout:    fleetStateReported,
					CredentialStatus: fleetStateVerified,
					UpdateStatus:     fleetStateUnknown,
					RemoteControl:    fleetStateDisabled,
				},
				Capabilities: ConnectionCapabilities{SupportsPause: false, SupportsScope: false, SupportsTest: false},
			},
		},
		Systems: []ConnectionSystem{
			{
				ID:   "pve:delly",
				Type: ConnectionTypePVE,
				Components: []ConnectionSystemComponent{
					{
						ConnectionID: "pve:delly",
						Type:         ConnectionTypePVE,
						Role:         ConnectionSystemComponentRolePrimary,
					},
					{
						ConnectionID: "agent:agent-delly",
						Type:         ConnectionTypeAgent,
						Role:         ConnectionSystemComponentRoleAttachment,
					},
				},
			},
		},
	}

	body, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal ConnectionsListResponse with direct host agent attachment: %v", err)
	}

	want := `{"connections":[{"id":"pve:delly","type":"pve","name":"delly","address":"https://delly:8006","hostAliases":["delly"],"state":"unauthorized","enabled":true,"surfaces":["vms","containers","storage","backups"],"scope":{"backups":true,"containers":true,"storage":true,"vms":true},"source":"agent","fleet":{"enrollmentState":"configured","livenessState":"unauthorized","versionDrift":"not-applicable","adapterHealth":"blocked","configRollout":"configured","credentialStatus":"invalid","updateStatus":"not-applicable","remoteControl":"not-applicable"},"capabilities":{"supportsPause":true,"supportsScope":true,"supportsTest":true}},{"id":"agent:agent-delly","type":"agent","name":"delly","address":"delly","hostAliases":["delly","192.168.0.5"],"state":"active","enabled":true,"surfaces":["host"],"scope":{"host":true},"lastSeen":"2026-05-13T23:45:00Z","source":"agent","fleet":{"enrollmentState":"enrolled","livenessState":"active","versionDrift":"unknown","adapterHealth":"healthy","configRollout":"reported","credentialStatus":"verified","updateStatus":"unknown","remoteControl":"disabled"},"capabilities":{"supportsPause":false,"supportsScope":false,"supportsTest":false}}],"systems":[{"id":"pve:delly","type":"pve","components":[{"connectionId":"pve:delly","type":"pve","role":"primary"},{"connectionId":"agent:agent-delly","type":"agent","role":"attachment"}]}]}`
	assertJSONSnapshot(t, body, want)
}

func TestContract_AgentConnectionPayloadIncludesVersionFields(t *testing.T) {
	conn := Connection{
		ID:          "agent:host-1",
		Type:        ConnectionTypeAgent,
		Name:        "host-1",
		Address:     "host-1",
		HostAliases: []string{"host-1", "192.168.0.2"},
		State:       ConnectionStateActive,
		Enabled:     true,
		Surfaces:    []string{"host"},
		Scope:       map[string]bool{"host": true},
		LastSeen:    timePtr(time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)),
		Source:      ConnectionSourceAgent,
		AgentIdentity: &ConnectionAgentIdentity{
			Hostname:        "host-1",
			Platform:        "linux",
			HostProfile:     "unraid",
			OSName:          "Unraid",
			OSVersion:       "7.1.0",
			KernelVersion:   "6.12.0",
			Architecture:    "x86_64",
			ReportIP:        "192.168.0.2",
			CommandsEnabled: true,
		},
		AgentVersion:         "6.0.0",
		ExpectedAgentVersion: "6.0.3",
		AgentUpdateAvailable: true,
		Fleet: ConnectionFleetGovernance{
			EnrollmentState:  fleetStateEnrolled,
			LivenessState:    fleetStateActive,
			VersionDrift:     fleetStateBehind,
			AdapterHealth:    fleetStateHealthy,
			ConfigRollout:    fleetStateReported,
			CredentialStatus: fleetStateVerified,
			UpdateStatus:     fleetStateUpdateAvailable,
			RemoteControl:    fleetStateEnabled,
			CommandPolicy: &ConnectionFleetCommandPolicy{
				Status:      fleetStateEnabled,
				Desired:     fleetStateEnabled,
				Applied:     fleetStateEnabled,
				Enforcement: fleetCommandPolicyInSync,
				Reason:      "agent command execution matches the desired enabled policy",
			},
		},
		Capabilities: ConnectionCapabilities{
			SupportsPause: false,
			SupportsScope: false,
			SupportsTest:  false,
		},
	}

	body, err := json.Marshal(conn)
	if err != nil {
		t.Fatalf("marshal agent Connection: %v", err)
	}
	if conn.AgentIdentity.Platform == conn.AgentIdentity.HostProfile {
		t.Fatalf("agent platform must remain runtime platform, got profile id %q", conn.AgentIdentity.Platform)
	}

	want := `{"id":"agent:host-1","type":"agent","name":"host-1","address":"host-1","hostAliases":["host-1","192.168.0.2"],"state":"active","enabled":true,"surfaces":["host"],"scope":{"host":true},"lastSeen":"2026-04-22T12:00:00Z","source":"agent","agentIdentity":{"hostname":"host-1","platform":"linux","hostProfile":"unraid","osName":"Unraid","osVersion":"7.1.0","kernelVersion":"6.12.0","architecture":"x86_64","reportIp":"192.168.0.2","commandsEnabled":true},"agentVersion":"6.0.0","expectedAgentVersion":"6.0.3","agentUpdateAvailable":true,"fleet":{"enrollmentState":"enrolled","livenessState":"active","versionDrift":"behind","adapterHealth":"healthy","configRollout":"reported","credentialStatus":"verified","updateStatus":"update-available","remoteControl":"enabled","commandPolicy":{"status":"enabled","desired":"enabled","applied":"enabled","enforcement":"in-sync","reason":"agent command execution matches the desired enabled policy"}},"capabilities":{"supportsPause":false,"supportsScope":false,"supportsTest":false}}`
	assertJSONSnapshot(t, body, want)
}

func TestContract_AgentDefaultDesiredConfigDoesNotCreateRolloutAttention(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 30, 0, 0, time.UTC)
	connections := buildConnections(aggregatorInputs{
		hosts: []models.Host{
			{
				ID:              "host-1",
				Hostname:        "host-1",
				ReportIP:        "192.0.2.42",
				LastSeen:        now.Add(-10 * time.Second),
				AgentVersion:    "6.0.0",
				Platform:        "linux",
				CommandsEnabled: false,
				TokenID:         "token-1",
			},
		},
		agentDesiredConfigs: map[string]connectionAgentDesiredConfig{
			"host-1": {},
		},
		expectedAgentVersion: "6.0.0",
		now:                  now,
	})
	if len(connections) != 1 {
		t.Fatalf("expected one agent connection, got %d", len(connections))
	}

	body, err := json.Marshal(connections[0])
	if err != nil {
		t.Fatalf("marshal agent Connection: %v", err)
	}

	want := `{"id":"agent:host-1","type":"agent","name":"host-1","address":"host-1","hostAliases":["host-1","192.0.2.42"],"state":"active","enabled":true,"surfaces":["host"],"scope":{"host":true},"lastSeen":"2026-05-14T10:29:50Z","source":"agent","agentIdentity":{"hostname":"host-1","platform":"linux","reportIp":"192.0.2.42"},"agentVersion":"6.0.0","expectedAgentVersion":"6.0.0","fleet":{"enrollmentState":"enrolled","livenessState":"active","versionDrift":"current","adapterHealth":"healthy","configRollout":"reported","credentialStatus":"verified","updateStatus":"current","remoteControl":"disabled","configDrift":{"status":"not-applicable","lastObservedAt":"2026-05-14T10:29:50Z","reason":"no managed agent configuration override is assigned"},"rollout":{"status":"current","stage":"applied","reason":"no managed agent configuration rollout is assigned"},"credentialHealth":{"status":"verified","kind":"agent-token","rotation":"healthy","lastVerifiedAt":"2026-05-14T10:29:50Z"},"commandPolicy":{"status":"disabled","desired":"unknown","applied":"disabled","enforcement":"not-applicable","reason":"no desired command-policy override is configured; reporting the agent-applied state"}},"capabilities":{"supportsPause":false,"supportsScope":false,"supportsTest":false}}`
	assertJSONSnapshot(t, body, want)
}

func TestContract_AgentConnectionPayloadUsesTokenEffectiveCommandPolicy(t *testing.T) {
	cfg := &config.Config{
		DataPath: t.TempDir(),
		APITokens: []config.APITokenRecord{
			{
				ID:     "runtime-without-exec",
				Scopes: []string{config.ScopeAgentReport, config.ScopeAgentConfigRead},
			},
		},
	}
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })

	hostID := "host-token-gated"
	rawDesiredCommands := true
	if err := monitor.UpdateHostAgentConfig(hostID, &rawDesiredCommands); err != nil {
		t.Fatalf("UpdateHostAgentConfig: %v", err)
	}

	now := time.Date(2026, 5, 14, 10, 30, 0, 0, time.UTC)
	host := models.Host{
		ID:              hostID,
		Hostname:        "host-token-gated",
		ReportIP:        "192.0.2.43",
		LastSeen:        now.Add(-10 * time.Second),
		AgentVersion:    "6.0.0",
		Platform:        "linux",
		CommandsEnabled: false,
		TokenID:         "runtime-without-exec",
	}
	desiredConfigs := connectionAgentDesiredConfigFingerprints(monitor, []models.Host{host}, cfg.APITokens)
	connections := buildConnections(aggregatorInputs{
		hosts:                []models.Host{host},
		apiTokens:            cfg.APITokens,
		agentDesiredConfigs:  desiredConfigs,
		expectedAgentVersion: "6.0.0",
		now:                  now,
	})
	if len(connections) != 1 {
		t.Fatalf("expected one agent connection, got %d", len(connections))
	}

	body, err := json.Marshal(connections[0])
	if err != nil {
		t.Fatalf("marshal agent Connection: %v", err)
	}

	want := `{"id":"agent:host-token-gated","type":"agent","name":"host-token-gated","address":"host-token-gated","hostAliases":["host-token-gated","192.0.2.43"],"state":"active","enabled":true,"surfaces":["host"],"scope":{"host":true},"lastSeen":"2026-05-14T10:29:50Z","source":"agent","agentIdentity":{"hostname":"host-token-gated","platform":"linux","reportIp":"192.0.2.43"},"agentVersion":"6.0.0","expectedAgentVersion":"6.0.0","fleet":{"enrollmentState":"enrolled","livenessState":"active","versionDrift":"current","adapterHealth":"healthy","configRollout":"reported","credentialStatus":"verified","updateStatus":"current","remoteControl":"disabled","configDrift":{"status":"pending","desired":{"version":"host-agent-config/v1","hash":"sha256:59378fe4db8132c2ce1d9c16b492c16093156579d09357db44e34a0fab494bea"},"reason":"Pulse has not received a comparable applied agent configuration fingerprint yet"},"rollout":{"status":"pending","stage":"pending","reason":"waiting for the agent to report an applied configuration fingerprint"},"credentialHealth":{"status":"verified","kind":"agent-token","rotation":"healthy","lastVerifiedAt":"2026-05-14T10:29:50Z"},"commandPolicy":{"status":"disabled","desired":"disabled","applied":"disabled","enforcement":"in-sync","reason":"agent command execution matches the desired disabled policy"}},"capabilities":{"supportsPause":false,"supportsScope":false,"supportsTest":false}}`
	assertJSONSnapshot(t, body, want)
	if strings.Contains(string(body), `"desired":"enabled"`) {
		t.Fatalf("connections payload used unsanitized command desire: %s", body)
	}
}

func TestContract_ConnectionsListIncludesAgentHostsFromUnifiedReadState(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })

	adapter := unifiedresources.NewMonitorAdapter(nil)
	setTestUnexportedField(t, monitor, "resourceStore", monitoring.ResourceStoreInterface(adapter))

	now := time.Now().UTC()
	adapter.PopulateSupplementalRecords(unifiedresources.SourceAgent, []unifiedresources.IngestRecord{
		{
			SourceID: "agent-1",
			Resource: unifiedresources.Resource{
				ID:       "resource-host-1",
				Type:     unifiedresources.ResourceTypeAgent,
				Name:     "mini-pc",
				Status:   unifiedresources.StatusOnline,
				LastSeen: now,
				Agent: &unifiedresources.AgentData{
					AgentID:   "agent-1",
					Hostname:  "mini-pc",
					MachineID: "machine-1",
				},
				Identity: unifiedresources.ResourceIdentity{
					MachineID: "machine-1",
					Hostnames: []string{"mini-pc"},
				},
			},
		},
	})

	if got := len(monitorState(t, monitor).GetHosts()); got != 0 {
		t.Fatalf("expected legacy host snapshot to stay empty, got %d hosts", got)
	}
	if got := len(monitor.GetUnifiedReadStateOrSnapshot().Hosts()); got != 1 {
		t.Fatalf("expected unified read-state to expose 1 agent host, got %d", got)
	}

	handler := NewConnectionsHandlers(
		func(context.Context) *config.Config { return cfg },
		func(context.Context) *config.ConfigPersistence { return nil },
		func(context.Context) *monitoring.Monitor { return monitor },
	)

	req := httptest.NewRequest(http.MethodGet, "/api/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ConnectionsListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode connections response: %v", err)
	}
	if len(resp.Connections) != 1 {
		t.Fatalf("expected 1 connection from unified read-state, got %d", len(resp.Connections))
	}
	if len(resp.Systems) != 1 {
		t.Fatalf("expected 1 grouped system from unified read-state, got %d", len(resp.Systems))
	}

	conn := resp.Connections[0]
	if conn.ID != "agent:agent-1" {
		t.Fatalf("connection id = %q, want %q", conn.ID, "agent:agent-1")
	}
	if conn.Type != ConnectionTypeAgent {
		t.Fatalf("connection type = %q, want %q", conn.Type, ConnectionTypeAgent)
	}
	if conn.Name != "mini-pc" || conn.Address != "mini-pc" {
		t.Fatalf("unexpected agent identity: name=%q address=%q", conn.Name, conn.Address)
	}
	if conn.State != ConnectionStateActive {
		t.Fatalf("connection state = %q, want %q", conn.State, ConnectionStateActive)
	}
	if conn.Source != ConnectionSourceAgent {
		t.Fatalf("connection source = %q, want %q", conn.Source, ConnectionSourceAgent)
	}
	if resp.Systems[0].ID != "agent:agent-1" || resp.Systems[0].Type != ConnectionTypeAgent {
		t.Fatalf("unexpected grouped system projection: %+v", resp.Systems[0])
	}
}

// TestContract_ProbePayloadShapeStaysCanonical pins the POST
// /api/connections/probe wire shape. Hint keys are free-form; the envelope
// fields (type, host, port, hints) are the contract boundary.
func TestContract_ProbePayloadShapeStaysCanonical(t *testing.T) {
	resp := ProbeResponse{
		Candidates: []ProbeCandidate{
			{
				Type:  ConnectionTypePVE,
				Host:  "https://pve.lab:8006",
				Port:  8006,
				Hints: map[string]string{"product": "Proxmox VE", "version": "8.2.4"},
			},
		},
		ProbedMs: 812,
	}
	body, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal ProbeResponse: %v", err)
	}
	want := `{"candidates":[{"type":"pve","host":"https://pve.lab:8006","port":8006,"hints":{"product":"Proxmox VE","version":"8.2.4"}}],"probedMs":812}`
	assertJSONSnapshot(t, body, want)
}

func TestContract_ProbeRejectsBlockedMetadataAddress(t *testing.T) {
	handler := NewConnectionsHandlers(nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/connections/probe", strings.NewReader(`{"address":"169.254.169.254"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandleProbe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var payload APIError
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if payload.Code != "invalid_address" {
		t.Fatalf("code = %q, want %q", payload.Code, "invalid_address")
	}
	if !strings.Contains(payload.ErrorMessage, "metadata service address is not allowed") {
		t.Fatalf("error = %q, want metadata-service rejection", payload.ErrorMessage)
	}
}

func TestContract_SimpleStatsUsesTextNodesForContainerFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	rec := httptest.NewRecorder()

	var router Router
	router.handleSimpleStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if strings.Contains(body, "row.innerHTML") {
		t.Fatalf("simple stats page must not use row.innerHTML for container rendering")
	}
	if !strings.Contains(body, "textContent") {
		t.Fatalf("simple stats page must use textContent for container rendering")
	}
}

func TestContract_APIRejectsUnsafeIncomingRequestIDHeader(t *testing.T) {
	handler := ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("X-Request-ID", "bad\nrequest-id")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	got := rec.Header().Get("X-Request-ID")
	if got == "" {
		t.Fatal("expected generated request id")
	}
	if got == "bad\nrequest-id" {
		t.Fatalf("unsafe request id header must not round-trip: %q", got)
	}
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("response request id must not contain control characters: %q", got)
	}
}

func TestContract_BootstrapTokenValidationRateLimitsPerClient(t *testing.T) {
	dataDir := t.TempDir()
	router := &Router{
		config: &config.Config{
			DataPath:   dataDir,
			ConfigPath: dataDir,
		},
		bootstrapTokenValidationLimiter: NewRateLimiter(1, time.Hour),
	}
	t.Cleanup(router.bootstrapTokenValidationLimiter.Stop)
	router.initializeBootstrapToken()

	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-bootstrap-token", strings.NewReader(`{"token":"deadbeef"}`))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	router.handleValidateBootstrapToken(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/security/validate-bootstrap-token", strings.NewReader(`{"token":"deadbeef"}`))
	req.RemoteAddr = "127.0.0.1:1234"
	rec = httptest.NewRecorder()
	router.handleValidateBootstrapToken(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}
	if retryAfter := rec.Header().Get("Retry-After"); retryAfter == "" {
		t.Fatal("expected Retry-After header on bootstrap token validation rate limit")
	}
}

func TestContract_DeployHandlersDoNotSurfaceLicenseSlotCapacityCopy(t *testing.T) {
	source, err := os.ReadFile("deploy_handlers.go")
	if err != nil {
		t.Fatalf("read deploy handlers: %v", err)
	}
	text := string(source)
	if strings.Contains(text, "No license slots available") {
		t.Fatal("deploy retry capacity denial must not surface legacy license-slot copy")
	}
	if strings.Contains(text, "reservedLicenseSlots") {
		t.Fatal("deploy response must not expose retired reservedLicenseSlots field")
	}
}

func mustStreamEvent(t *testing.T, eventType string, data interface{}) chat.StreamEvent {
	t.Helper()

	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal stream data: %v", err)
	}

	return chat.StreamEvent{
		Type: eventType,
		Data: raw,
	}
}

func assertJSONSnapshot(t *testing.T, got []byte, want string, excludeKeys ...string) {
	t.Helper()

	// excludeKeys lets callers strip top-level fields whose values are
	// dynamic per run (timestamps, durations) before snapshot comparison.
	// The structural shape of the response is still validated; only the
	// listed keys are removed from both `got` and `want`.
	gotBytes := got
	wantBytes := []byte(want)
	if len(excludeKeys) > 0 {
		var err error
		gotBytes, err = stripJSONTopLevelKeys(got, excludeKeys)
		if err != nil {
			t.Fatalf("strip got json: %v", err)
		}
		wantBytes, err = stripJSONTopLevelKeys([]byte(want), excludeKeys)
		if err != nil {
			t.Fatalf("strip want json: %v", err)
		}
	}

	var gotCompact bytes.Buffer
	var wantCompact bytes.Buffer
	if err := json.Compact(&gotCompact, gotBytes); err != nil {
		t.Fatalf("compact got json: %v", err)
	}
	if err := json.Compact(&wantCompact, wantBytes); err != nil {
		t.Fatalf("compact want json: %v", err)
	}
	if gotCompact.String() != wantCompact.String() {
		t.Fatalf("json snapshot mismatch\nwant: %s\ngot:  %s", wantCompact.String(), gotCompact.String())
	}
}

// stripJSONTopLevelKeys removes the named top-level keys from a JSON object
// and re-serializes. Used by assertJSONSnapshot to ignore dynamic fields.
func stripJSONTopLevelKeys(data []byte, keys []string) ([]byte, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	for _, k := range keys {
		delete(obj, k)
	}
	return json.Marshal(obj)
}

// TestContract_InvestigationProjectionCarriesNeverAutoRemediate pins
// the cross-path agreement: the projection-builder closure in
// router.go must populate NeverAutoRemediate on the projection so
// the investigation read path sees the same lock-against-remediation
// flag the action broker enforces. Without this, Patrol could
// propose fixes on locked resources that the broker would refuse —
// the fix proposal shouldn't have happened in the first place.
func TestContract_InvestigationProjectionCarriesNeverAutoRemediate(t *testing.T) {
	source, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	src := string(source)
	if !strings.Contains(src, "NeverAutoRemediate:   state.NeverAutoRemediate,") {
		t.Error("router.go must populate NeverAutoRemediate on the projection so the investigation read path sees the operator's lock — drift here means Patrol can propose fixes the broker refuses")
	}
}

// TestContract_AgentEventsStreamPublishesOnFindingCreated pins the
// agent SSE stream's most consequential producer hook: the
// findings-runtime callback in router.go must publish a
// finding.created event when a finding is new AND not auto-dismissed
// by operator-state suppression. Without this guard, every
// patrol-cycle re-detection would fire an event (noise) and findings
// the operator already silenced would still notify (contradicting
// slice 31/32 suppression semantics).
func TestContract_AgentEventsStreamPublishesOnFindingCreated(t *testing.T) {
	source, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	src := string(source)
	if !strings.Contains(src, "PublishFindingCreated(AgentEventFindingCreatedPayload{") {
		t.Error("router.go must publish finding.created events to the agent stream when a new finding is added")
	}
	if !strings.Contains(src, "isNew && r.agentEventBroadcaster != nil && f.DismissedReason == \"\"") {
		t.Error("publish gate must fire only when isNew, broadcaster wired, and finding not auto-dismissed by operator-state — otherwise the stream contradicts the suppression contract")
	}
}

// TestContract_AgentEventsStreamPublishesOnApprovalPending pins the
// agent SSE stream's governance-aware producer hook: the approval
// store's post-create callback must publish an approval.pending
// event so an agent holding /api/agent/events open hears about new
// pending approvals in real time without polling. The callback is
// installed via AIHandler.SetApprovalCreatedCallback so it survives
// the AIHandler's lazy approval-store rebuilds. Drift here means
// the only signal an agent has that an operator decision is needed
// is poll-based — losing the substrate's push-notification
// guarantee for governance state.
func TestContract_AgentEventsStreamPublishesOnApprovalPending(t *testing.T) {
	source, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	src := string(source)
	if !strings.Contains(src, "r.aiHandler.SetApprovalCreatedCallback(") {
		t.Error("router.go must install the approval-created callback on AIHandler so the agent SSE stream is wired before the first approval lands")
	}
	if !strings.Contains(src, "broadcaster.PublishApprovalPending(AgentEventApprovalPendingPayload{") {
		t.Error("router.go must bridge approval creation into PublishApprovalPending — drift means agents lose the push-notification path for governance state")
	}
	if !strings.Contains(src, "ApprovalID:  req.ID") {
		t.Error("approval.pending payload must carry the approval id so agents can correlate the event with /api/approvals/{id}")
	}
	if !strings.Contains(src, "ResourceID:  req.CanonicalResourceID()") {
		t.Error("approval.pending payload must derive the canonical resourceId via CanonicalResourceID so agents can match it against the rest of Pulse")
	}
}

// TestContract_ApprovalStorePostCreateCallback pins the seam the
// SSE bridge depends on. The approval store must expose a
// SetOnApprovalCreated setter and CreateApproval must dispatch the
// callback on its own goroutine after the approval is persisted.
// Removing or in-lining the callback breaks the bridge silently.
func TestContract_ApprovalStorePostCreateCallback(t *testing.T) {
	source, err := os.ReadFile("../ai/approval/store.go")
	if err != nil {
		t.Fatalf("read approval/store.go: %v", err)
	}
	src := string(source)
	if !strings.Contains(src, "func (s *Store) SetOnApprovalCreated(cb func(*ApprovalRequest))") {
		t.Error("approval store must expose SetOnApprovalCreated so the API layer can bridge approval creation into the agent SSE stream")
	}
	if !strings.Contains(src, "if cb := s.onApprovalCreated; cb != nil {") {
		t.Error("CreateApproval must check the post-create callback under lock and dispatch it; drift here drops approval.pending events")
	}
	if !strings.Contains(src, "go cb(&snapshot)") {
		t.Error("post-create callback must run on its own goroutine to keep the approval hot path off any consumer's slowness")
	}
}

// TestContract_AgentEventApprovalPendingKindIsStable pins the
// wire-stable event kind string. Renaming AgentEventApprovalPending
// breaks every agent that branches on the SSE event-type field; the
// constant is part of the contract, not an implementation detail.
func TestContract_AgentEventApprovalPendingKindIsStable(t *testing.T) {
	if AgentEventApprovalPending != AgentEventKind(agentcapabilities.EventKindApprovalPending) {
		t.Errorf("AgentEventApprovalPending = %q, want shared event kind %q",
			AgentEventApprovalPending, agentcapabilities.EventKindApprovalPending)
	}
	if string(AgentEventApprovalPending) != "approval.pending" {
		t.Errorf("AgentEventApprovalPending wire value = %q, want approval.pending", AgentEventApprovalPending)
	}
}

// TestContract_SubscribeEventsCapabilityListsApprovalPending pins
// the discovery contract: the capabilities manifest must mention
// approval.pending under subscribe_events so an agent reading the
// manifest learns the kind exists without out-of-band documentation.
// Drift here means external agents miss the event entirely because
// they didn't know to listen for it.
func TestContract_SubscribeEventsCapabilityListsApprovalPending(t *testing.T) {
	capability, ok := agentcapabilities.FindCapability(agentcapabilities.CanonicalManifest().Capabilities, agentcapabilities.EventSubscriptionCapabilityName)
	if !ok {
		t.Fatal("canonical manifest must include subscribe_events")
	}
	if !strings.Contains(capability.Description, string(agentcapabilities.EventKindApprovalPending)+" when a remediation request enters StatusPending") {
		t.Error("subscribe_events description must mention approval.pending so agents discover the kind through the manifest")
	}
}

// TestContract_AgentEventsHeartbeatIsStreamLocal pins that
// heartbeat keepalives are written to the connected SSE response
// rather than published through the broadcaster. Heartbeats are
// connection health signals, not product events; publishing them
// globally makes N subscribers create N heartbeat events that fan
// out to every other subscriber.
func TestContract_AgentEventsHeartbeatIsStreamLocal(t *testing.T) {
	source, err := os.ReadFile("agent_events.go")
	if err != nil {
		t.Fatalf("read agent_events.go: %v", err)
	}
	src := string(source)
	if !strings.Contains(src, "writeAgentSSEEvent(w, b.stampEvent(AgentEvent{Kind: AgentEventHeartbeat}))") {
		t.Error("HandleAgentEvents must write heartbeat events directly to the connected response")
	}
	if strings.Contains(src, "case <-heartbeat.C:\n\t\t\tb.Publish(AgentEvent{Kind: AgentEventHeartbeat})") {
		t.Error("HandleAgentEvents must not publish heartbeat events through the broadcaster")
	}
}

// TestContract_AgentEventsStreamPublishesOnActionCompleted pins the
// agent SSE stream's third producer hook: the executor's
// post-completion callback must publish action.completed events for
// every terminal-state audit so an agent holding /api/agent/events
// open closes the dispatch loop without polling. The bridge lives in
// wireAIChatDependenciesForService so the callback is re-installed
// on every per-org chat-service init. Drift here means agents lose
// the push-notification path for dispatch outcomes — including the
// refused-before-dispatch failures that carry stable error tokens.
func TestContract_AgentEventsStreamPublishesOnActionCompleted(t *testing.T) {
	source, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	src := string(source)
	if !strings.Contains(src, "executor.SetOnActionCompleted(func(record unifiedresources.ActionAuditRecord)") {
		t.Error("router.go must install the post-completion callback on the executor returned from chatService.GetExecutor() so the agent SSE stream is wired per-org")
	}
	if !strings.Contains(src, "broadcaster.PublishActionCompletedRecord(record)") {
		t.Error("router.go must bridge executor action completion into PublishActionCompletedRecord — drift means agents lose the push-notification path for dispatch outcomes")
	}
	if !strings.Contains(src, "r.resourceHandlers.SetActionCompletedPublisher(r.agentEventBroadcaster.PublishActionCompletedRecord)") {
		t.Error("router.go must also bridge API-owned action execution completions into the agent SSE stream")
	}
	eventsSource, err := os.ReadFile("agent_events.go")
	if err != nil {
		t.Fatalf("read agent_events.go: %v", err)
	}
	eventsSrc := string(eventsSource)
	if !strings.Contains(eventsSrc, "func (b *AgentEventBroadcaster) PublishActionCompletedRecord(record unifiedresources.ActionAuditRecord)") {
		t.Error("agent_events.go must keep a shared action-audit-record projector for every action.completed producer")
	}
	if !strings.Contains(eventsSrc, "ActionID:       record.ID") {
		t.Error("action.completed payload must carry the canonical action id so agents can correlate the event with /api/actions/{id}")
	}
	if !strings.Contains(eventsSrc, "ResourceID:     record.Request.ResourceID") {
		t.Error("action.completed payload must carry the canonical resource id so agents can match it against the rest of Pulse")
	}
}

// TestContract_ExternalAgentActivityUsesRouteSpecificMarkers pins the
// anonymous usage signal behind Pulse Intelligence external-agent telemetry:
// configured/readiness may come from token capability posture, but recent-use
// must come from authenticated calls into a manifest-backed agent/MCP
// capability route. Generic API-token LastUsedAt is too broad because the same
// token can hit unrelated routes; requiring every manifest scope is too narrow
// because read-only MCP clients are supported.
func TestContract_ExternalAgentActivityUsesRouteSpecificMarkers(t *testing.T) {
	activitySource, err := os.ReadFile("agent_activity_telemetry.go")
	if err != nil {
		t.Fatalf("read agent_activity_telemetry.go: %v", err)
	}
	activitySrc := string(activitySource)
	for _, required := range []string{
		"getAPITokenRecordFromRequest(req)",
		"externalAgentCapabilityActivity(capabilityName)",
		"agentcapabilities.FindCapability(agentcapabilities.CanonicalManifest().Capabilities, capabilityName)",
		"apiTokenCoversExternalAgentSurface(token, time.Now().UTC(), requiredScopes...)",
		"agentcapabilities.CanonicalManifest().RequiredScopes",
		"func externalAgentActivityForCapability(capabilityName string) (string, bool)",
		"persistence.RecordExternalAgentActivity(config.ExternalAgentActivityRecord{",
	} {
		if !strings.Contains(activitySrc, required) {
			t.Errorf("external-agent activity marker must stay token-scoped and manifest-capable; missing %s", required)
		}
	}
	readCapability, ok := agentcapabilities.FindCapability(agentcapabilities.CanonicalManifest().Capabilities, agentcapabilities.FleetContextCapabilityName)
	if !ok {
		t.Fatalf("manifest missing %s", agentcapabilities.FleetContextCapabilityName)
	}
	if _, requiredScope, ok := externalAgentCapabilityActivity(agentcapabilities.FleetContextCapabilityName); !ok || requiredScope != readCapability.Scope {
		t.Fatalf("external-agent activity must use the called capability scope, got scope=%q ok=%v want %q", requiredScope, ok, readCapability.Scope)
	}
	for _, capability := range agentcapabilities.ManifestSurfaceToolCapabilities(agentcapabilities.CanonicalManifest(), agentcapabilities.SurfaceIDPulseMCP) {
		if _, ok := externalAgentActivityForCapability(capability.Name); !ok {
			t.Errorf("MCP-published capability %q must have an external-agent activity class", capability.Name)
		}
	}

	routeFiles := []string{
		"router_routes_monitoring.go",
		"router_routes_registration.go",
		"router_routes_ai_relay.go",
		"router.go",
	}
	var routesBuilder strings.Builder
	for _, file := range routeFiles {
		routesSource, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		routesBuilder.Write(routesSource)
	}
	routesSrc := routesBuilder.String()
	for _, required := range []string{
		"agentcapabilities.ResourceContextCapabilityName",
		"agentcapabilities.FleetContextCapabilityName",
		"agentcapabilities.OperationsLoopStatusCapabilityName",
		"agentcapabilities.EventSubscriptionCapabilityName",
		"agentcapabilities.ListNodesCapabilityName",
		"agentcapabilities.AddNodeCapabilityName",
		"agentcapabilities.UpdateNodeCapabilityName",
		"agentcapabilities.RemoveNodeCapabilityName",
		"agentcapabilities.TestNodeCredentialsCapabilityName",
		"agentcapabilities.TestNodeConnectionCapabilityName",
		"agentcapabilities.RefreshNodeClusterMembershipCapabilityName",
		"agentcapabilities.DiscoverLANCapabilityName",
		"agentcapabilities.GetOperatorStateCapabilityName",
		"agentcapabilities.SetOperatorStateCapabilityName",
		"agentcapabilities.ClearOperatorStateCapabilityName",
		"agentcapabilities.ListFindingsCapabilityName",
		"agentcapabilities.AcknowledgeFindingCapabilityName",
		"agentcapabilities.SnoozeFindingCapabilityName",
		"agentcapabilities.DismissFindingCapabilityName",
		"agentcapabilities.ResolveFindingCapabilityName",
		"agentcapabilities.PlanActionCapabilityName",
		"agentcapabilities.DecideActionCapabilityName",
		"agentcapabilities.ExecuteActionCapabilityName",
	} {
		if !strings.Contains(routesSrc, required) {
			t.Errorf("manifest-backed external-agent routes must record capability-specific activity; missing %s", required)
		}
	}

	telemetrySource, err := os.ReadFile(filepath.Join("..", "..", "pkg", "server", "telemetry_pulse_intelligence.go"))
	if err != nil {
		t.Fatalf("read telemetry_pulse_intelligence.go: %v", err)
	}
	telemetrySrc := string(telemetrySource)
	if !strings.Contains(telemetrySrc, "persistence.LoadExternalAgentActivityHistory()") {
		t.Error("Pulse Intelligence external-agent recent-use telemetry must read route activity history")
	}
	for _, required := range []string{
		"pulseIntelligenceExternalAgentSurfaceScopes(agentcapabilities.CanonicalManifest())",
		"agentcapabilities.ManifestSurfaceToolCapabilities(manifest, agentcapabilities.SurfaceIDPulseMCP)",
		"agentcapabilities.RequiredCapabilityScopes(",
		"func pulseIntelligenceExternalAgentToken(token config.APITokenRecord, surfaceScopes []string, now time.Time) bool",
		"if token.HasScope(scope) {\n\t\t\treturn true",
	} {
		if !strings.Contains(telemetrySrc, required) {
			t.Errorf("Pulse Intelligence external-agent readiness telemetry must use MCP-published capability scopes; missing %s", required)
		}
	}
	if strings.Contains(telemetrySrc, "LastUsedAt != nil && !token.LastUsedAt.UTC().Before(since)") {
		t.Error("Pulse Intelligence external-agent recent-use telemetry must not infer usage from generic token LastUsedAt")
	}
	if strings.Contains(telemetrySrc, "requiredScopes := agentcapabilities.CanonicalManifest().RequiredScopes") {
		t.Error("Pulse Intelligence external-agent readiness telemetry must not require every manifest scope")
	}
}

// TestContract_UpdateFunnelTelemetryStaysContentFree pins the server-owned
// update funnel telemetry path. Update history may feed outbound usage adoption
// analysis, but only as aggregate 30-day counts and a coarse failure category,
// and the operator preview type must expose the same JSON fields.
func TestContract_UpdateFunnelTelemetryStaysContentFree(t *testing.T) {
	pingType := reflect.TypeOf(telemetry.Ping{})
	for _, field := range []string{
		`UpdateAttempts30d:update_attempts_30d`,
		`UpdateSuccesses30d:update_successes_30d`,
		`UpdateFailures30d:update_failures_30d`,
		`UpdateLastFailureCategory:update_last_failure_category,omitempty`,
	} {
		name, tag, _ := strings.Cut(field, ":")
		sf, ok := pingType.FieldByName(name)
		if !ok {
			t.Fatalf("telemetry.Ping missing %s", name)
		}
		if got := sf.Tag.Get("json"); got != tag {
			t.Fatalf("telemetry.Ping.%s json tag = %q, want %q", name, got, tag)
		}
	}

	serverSource, err := os.ReadFile(filepath.Join("..", "..", "pkg", "server", "server.go"))
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}
	if !strings.Contains(string(serverSource), "router.ApplyUpdateTelemetrySnapshot(&snap, now)") {
		t.Error("server telemetry snapshot must route update-history rollups through Router.ApplyUpdateTelemetrySnapshot before preview/send")
	}

	routerSource, err := os.ReadFile("telemetry_pulse_intelligence.go")
	if err != nil {
		t.Fatalf("read telemetry_pulse_intelligence.go: %v", err)
	}
	for _, fragment := range []string{
		"func (r *Router) ApplyUpdateTelemetrySnapshot(s *telemetry.Snapshot, now time.Time)",
		"counters to the outbound usage telemetry snapshot",
		"telemetry.ApplyUpdateTelemetrySnapshot(s, r.updateHistory, now)",
	} {
		if !strings.Contains(string(routerSource), fragment) {
			t.Errorf("router-owned update telemetry boundary missing %s", fragment)
		}
	}
	if strings.Contains(string(routerSource), "anonymous telemetry") {
		t.Error("router-owned update telemetry boundary must not describe outbound usage telemetry as anonymous")
	}

	settingsSource, err := os.ReadFile(filepath.Join("..", "..", "frontend-modern", "src", "api", "settings.ts"))
	if err != nil {
		t.Fatalf("read frontend settings API type: %v", err)
	}
	for _, fragment := range []string{
		"update_attempts_30d: number;",
		"update_successes_30d: number;",
		"update_failures_30d: number;",
		"update_last_failure_category?: string;",
	} {
		if !strings.Contains(string(settingsSource), fragment) {
			t.Errorf("TelemetryPingPreview must mirror update funnel field %s", fragment)
		}
	}
}

// TestContract_ExecutorPostCompletionCallback pins the seam the SSE
// bridge depends on. The executor must expose SetOnActionCompleted
// and the action_audit hot path must dispatch the callback for every
// terminal-state record (success, runtime fail, plan-drift refusal,
// remediation-lock refusal). Removing or in-lining the callback
// breaks the bridge silently.
func TestContract_ExecutorPostCompletionCallback(t *testing.T) {
	executor, err := os.ReadFile("../ai/tools/executor.go")
	if err != nil {
		t.Fatalf("read executor.go: %v", err)
	}
	executorSrc := string(executor)
	if !strings.Contains(executorSrc, "func (e *PulseToolExecutor) SetOnActionCompleted(cb func(unifiedresources.ActionAuditRecord))") {
		t.Error("executor must expose SetOnActionCompleted so the API layer can bridge action completion into the agent SSE stream")
	}

	audit, err := os.ReadFile("../ai/tools/action_audit.go")
	if err != nil {
		t.Fatalf("read action_audit.go: %v", err)
	}
	auditSrc := string(audit)
	if !strings.Contains(auditSrc, "func (e *PulseToolExecutor) publishActionCompleted(record unifiedresources.ActionAuditRecord)") {
		t.Error("action_audit.go must define publishActionCompleted helper so terminal-state writers route through one bridge point")
	}
	if !strings.Contains(auditSrc, "go cb(record)") {
		t.Error("post-completion callback must run on its own goroutine to keep the dispatch hot path off any consumer's slowness")
	}
	// Pin that all four terminal sites call publishActionCompleted.
	terminalSites := []string{
		`e.recordActionLifecycle(record.ID, unifiedresources.ActionStateFailed, requestedBy, "plan drift refused")
			e.publishActionCompleted(record)`,
		`e.recordActionLifecycle(record.ID, unifiedresources.ActionStateFailed, requestedBy, "resource remediation lock refused")
		e.publishActionCompleted(record)`,
		`e.recordActionLifecycle(record.ID, record.State, actor, message)
		e.publishActionCompleted(record)`,
	}
	for _, site := range terminalSites {
		if !strings.Contains(auditSrc, site) {
			t.Errorf("action_audit.go must call publishActionCompleted at every terminal-state site; missing block:\n%s", site)
		}
	}
}

// TestContract_AgentEventActionCompletedKindIsStable pins the
// wire-stable event kind string. Renaming AgentEventActionCompleted
// breaks every agent that branches on the SSE event-type field; the
// constant is part of the contract, not an implementation detail.
func TestContract_AgentEventActionCompletedKindIsStable(t *testing.T) {
	if AgentEventActionCompleted != AgentEventKind(agentcapabilities.EventKindActionCompleted) {
		t.Errorf("AgentEventActionCompleted = %q, want shared event kind %q",
			AgentEventActionCompleted, agentcapabilities.EventKindActionCompleted)
	}
	if string(AgentEventActionCompleted) != "action.completed" {
		t.Errorf("AgentEventActionCompleted wire value = %q, want action.completed", AgentEventActionCompleted)
	}
}

// TestContract_SubscribeEventsCapabilityListsActionCompleted pins
// the discovery contract: the capabilities manifest must mention
// action.completed under subscribe_events so an agent reading the
// manifest learns the kind exists without out-of-band documentation.
// Drift here means external agents miss the event entirely because
// they didn't know to listen for it.
func TestContract_SubscribeEventsCapabilityListsActionCompleted(t *testing.T) {
	capability, ok := agentcapabilities.FindCapability(agentcapabilities.CanonicalManifest().Capabilities, agentcapabilities.EventSubscriptionCapabilityName)
	if !ok {
		t.Fatal("canonical manifest must include subscribe_events")
	}
	if !strings.Contains(capability.Description, string(agentcapabilities.EventKindActionCompleted)+" when an action audit reaches a terminal state") {
		t.Error("subscribe_events description must mention action.completed so agents discover the kind through the manifest")
	}
}

// TestContract_AgentCapabilitiesManifestVersionIsPinned pins the
// manifest's version contract: bumping it is reserved for breaking
// shape changes, additive capabilities ship under the same version.
// Pinning the version string here means a refactor that accidentally
// bumps it shows up as a test failure rather than silently breaking
// every external agent that validates compatibility on startup.
func TestContract_AgentCapabilitiesManifestVersionIsPinned(t *testing.T) {
	src := readAgentCapabilitiesManifestSource(t)
	if !strings.Contains(src, `Version: "v1"`) {
		t.Error("agent capabilities manifest must pin Version to v1; bumping is a breaking-change decision and should not happen silently")
	}
	for _, required := range []string{
		`SurfaceContract: SurfaceContract{`,
		`ID:          "pulse_intelligence_core"`,
		`Label:       "Pulse Intelligence Core"`,
		`ID:          "pulse_patrol"`,
		`ID:              SurfaceIDPulseAssistant`,
		`ID:              SurfaceIDPulseMCP`,
		`InteractiveQuestions: true`,
		`CapabilityMetadata: true`,
		`SurfaceToolContracts: []SurfaceToolContract{`,
		`ToolNames:  canonicalPulseMCPSurfaceToolNames(),`,
		`publishedSurfaceToolContracts := CloneSurfaceToolContracts(canonicalManifest.SurfaceToolContracts)`,
		`SurfaceToolContracts: CloneSurfaceToolContracts(surfaceToolContracts)`,
		`WorkflowPrompts:`,
		`ClonePulseWorkflowPrompts(ProjectPulseWorkflowPrompts(capabilities))`,
	} {
		if !strings.Contains(src, required) {
			t.Errorf("agent capabilities manifest must pin the Pulse Intelligence surface contract; missing %s", required)
		}
	}
	if strings.Contains(src, `ID:              "pulse_patrol"`) {
		t.Error("Pulse Patrol must stay the primary built-in operator component, not be flattened into the compatibility access-path list")
	}
	// Capability surface anchors — these names are agent-stable
	// identifiers and renaming any of them breaks every external
	// integration. Pin them here so renames must be deliberate.
	required := []string{
		agentcapabilities.ResourceContextCapabilityName,
		agentcapabilities.GetOperatorStateCapabilityName,
		agentcapabilities.SetOperatorStateCapabilityName,
		agentcapabilities.ClearOperatorStateCapabilityName,
		agentcapabilities.ListFindingsCapabilityName,
	}
	manifest := agentcapabilities.CanonicalManifest()
	for _, name := range required {
		if _, ok := agentcapabilities.FindCapability(manifest.Capabilities, name); !ok {
			t.Errorf("agent capabilities manifest must declare canonical capability %q", name)
		}
	}
}

func TestContract_PulseIntelligenceSurfaceToolProjectionKeepsAssistantAndMCPDistinct(t *testing.T) {
	surfaceSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "surface_contract.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/surface_contract.go: %v", err)
	}
	surfaceContract := string(surfaceSource)
	for _, required := range []string{
		`type SurfaceToolContract struct`,
		`SurfaceToolSourceAssistantRegistry  = "assistant_registry"`,
		`SurfaceToolSourceCapabilityManifest = "capability_manifest"`,
		`func ProjectPulseAssistantSurfaceToolContract(`,
		`func ProjectManifestSurfaceToolContract(`,
		`func ProjectManifestSurfaceToolContracts(`,
		`func ResolveManifestSurfaceToolContract(`,
		`func ProjectPulseIntelligenceSurfaceToolContracts(`,
		`type ResolvedSurfaceAffordanceContract struct`,
		`func ResolveSurfaceAffordanceContract(`,
		`ResolveSurfaceAffordanceContract(contract, SurfaceIDPulseAssistant, "")`,
		`ResolveSurfaceAffordanceContract(manifest.SurfaceContract, surface.ID, surface.Label)`,
		`func ManifestSurfaceAffordances(`,
		`func IntersectSurfaceAffordances(`,
		`IntersectSurfaceAffordances(resolvedSurface.Affordances, contract.Affordances)`,
		`ProviderToolNames(providerTools)`,
		`AssistantNativeProviderToolNames()`,
		`if !surface.Affordances.Tools`,
		`if !surface.Affordances.InteractiveQuestions`,
		`ToolNames:         offeredNames`,
		`!surface.ExternalAdapter`,
		`FindManifestSurfaceToolContract(manifest, surface.ID)`,
		`requestResponseCapabilityNamesForNames(manifest.Capabilities, contract.ToolNames)`,
		`ProjectManifestSurfaceToolContract(manifest, surface.ID)`,
	} {
		if !strings.Contains(surfaceContract, required) {
			t.Errorf("agentcapabilities must own shared Assistant/MCP surface tool projection; missing %s", required)
		}
	}
	for _, forbidden := range []string{
		`func ProjectPulseMCPSurfaceToolContract(`,
		`projectManifestSurfaceToolContractFromCapabilities`,
		`requestResponseCapabilityNames(manifest.Capabilities)`,
		`ProjectManifestSurfaceToolContract(manifest, SurfaceIDPulseMCP)`,
		`ProjectTools(manifest.Capabilities)`,
	} {
		if strings.Contains(surfaceContract, forbidden) {
			t.Errorf("agentcapabilities surface resolver must not keep MCP-specific or raw-capability fallback projection; found %s", forbidden)
		}
	}

	projectionSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "projection.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/projection.go: %v", err)
	}
	projectionContract := string(projectionSource)
	for _, required := range []string{
		`ResolveManifestSurfaceToolContract(manifest, surfaceID)`,
		`ManifestSurfaceAffordances(manifest, contract.SurfaceID)`,
		`if !affordances.Tools`,
	} {
		if !strings.Contains(projectionContract, required) {
			t.Errorf("agentcapabilities projection must resolve external-surface tools through the normalized surface contract; missing %s", required)
		}
	}

	assistantProjectionSource, err := os.ReadFile(filepath.Join("..", "ai", "tools", "provider_projection.go"))
	if err != nil {
		t.Fatalf("read internal/ai/tools/provider_projection.go: %v", err)
	}
	assistantProjection := string(assistantProjectionSource)
	for _, required := range []string{
		`func (e *PulseToolExecutor) AssistantSurfaceToolContract(opts agentcapabilities.AssistantProviderToolOptions) agentcapabilities.SurfaceToolContract`,
		`agentcapabilities.ProjectPulseAssistantSurfaceToolContract(`,
		`agentcapabilities.CanonicalManifest().SurfaceContract`,
		`e.AssistantProviderTools(opts)`,
	} {
		if !strings.Contains(assistantProjection, required) {
			t.Errorf("Assistant executor must expose native surface projection through shared core contract; missing %s", required)
		}
	}

	chatServiceSource, err := os.ReadFile(filepath.Join("..", "ai", "chat", "service.go"))
	if err != nil {
		t.Fatalf("read internal/ai/chat/service.go: %v", err)
	}
	chatService := string(chatServiceSource)
	for _, required := range []string{
		`func (s *Service) AssistantSurfaceToolContract(_ context.Context) agentcapabilities.SurfaceToolContract`,
		`executor.AssistantSurfaceToolContract(agentcapabilities.AssistantProviderToolOptions{`,
		`IncludeQuestionTool: !s.isAutonomousModeEnabled(),`,
	} {
		if !strings.Contains(chatService, required) {
			t.Errorf("chat service must expose live native Assistant surface tools through the shared core contract; missing %s", required)
		}
	}

	aiHandlerSource, err := os.ReadFile("ai_handler.go")
	if err != nil {
		t.Fatalf("read internal/api/ai_handler.go: %v", err)
	}
	aiHandler := string(aiHandlerSource)
	for _, required := range []string{
		`AssistantSurfaceToolContract(ctx context.Context) agentcapabilities.SurfaceToolContract`,
		`func (h *AIHandler) HandleAssistantSurfaceTools(w http.ResponseWriter, r *http.Request)`,
		`svc.AssistantSurfaceToolContract(ctx)`,
		`Pulse Assistant is not running`,
	} {
		if !strings.Contains(aiHandler, required) {
			t.Errorf("AI handler must expose authenticated native Assistant surface tool contract from runtime service; missing %s", required)
		}
	}

	aiRelaySource, err := os.ReadFile("router_routes_ai_relay.go")
	if err != nil {
		t.Fatalf("read internal/api/router_routes_ai_relay.go: %v", err)
	}
	aiRelay := string(aiRelaySource)
	for _, required := range []string{
		`"/api/ai/assistant/surface-tools"`,
		`RequireScope(config.ScopeAIChat, r.aiHandler.HandleAssistantSurfaceTools)`,
	} {
		if !strings.Contains(aiRelay, required) {
			t.Errorf("Assistant surface tools endpoint must stay authenticated behind ai:chat route scope; missing %s", required)
		}
	}

	manifest := agentcapabilities.CanonicalManifest()
	contracts := agentcapabilities.ProjectPulseIntelligenceSurfaceToolContracts(manifest, []agentcapabilities.ProviderTool{
		{Name: agentcapabilities.PulseQueryToolName},
		{Name: agentcapabilities.PulseReadToolName},
		agentcapabilities.NewPulseQuestionProviderTool(),
	})
	if len(contracts) != 2 {
		t.Fatalf("Pulse Intelligence surface tool contracts = %+v, want Assistant and MCP", contracts)
	}

	bySurface := map[string]agentcapabilities.SurfaceToolContract{}
	for _, contract := range contracts {
		bySurface[contract.SurfaceID] = contract
	}
	assistant := bySurface[agentcapabilities.SurfaceIDPulseAssistant]
	if assistant.ToolSource != agentcapabilities.SurfaceToolSourceAssistantRegistry {
		t.Fatalf("Assistant tool source = %q", assistant.ToolSource)
	}
	if !reflect.DeepEqual(assistant.ToolNames, []string{
		agentcapabilities.PulseQueryToolName,
		agentcapabilities.PulseReadToolName,
		agentcapabilities.PulseQuestionToolName,
	}) {
		t.Fatalf("Assistant surface tool names = %#v", assistant.ToolNames)
	}
	if !reflect.DeepEqual(assistant.NativeToolNames, []string{agentcapabilities.PulseQuestionToolName}) {
		t.Fatalf("Assistant native tool names = %#v", assistant.NativeToolNames)
	}
	if len(assistant.CapabilityNames) != 0 {
		t.Fatalf("Assistant surface must not duplicate MCP manifest capabilities: %#v", assistant.CapabilityNames)
	}

	mcp := bySurface[agentcapabilities.SurfaceIDPulseMCP]
	if mcp.ToolSource != agentcapabilities.SurfaceToolSourceCapabilityManifest {
		t.Fatalf("MCP tool source = %q", mcp.ToolSource)
	}
	if len(mcp.CapabilityNames) == 0 || len(mcp.ToolNames) != len(mcp.CapabilityNames) {
		t.Fatalf("MCP tools must be manifest request/response capabilities, got tools=%#v capabilities=%#v", mcp.ToolNames, mcp.CapabilityNames)
	}
	if !reflect.DeepEqual(mcp.ToolNames, manifest.SurfaceToolContracts[0].ToolNames) {
		t.Fatalf("MCP tools must come from manifest-published surface contract, got tools=%#v published=%#v", mcp.ToolNames, manifest.SurfaceToolContracts[0].ToolNames)
	}
	for _, forbidden := range []string{
		agentcapabilities.PulseQuestionToolName,
		agentcapabilities.EventSubscriptionCapabilityName,
	} {
		if stringSliceContains(mcp.ToolNames, forbidden) {
			t.Fatalf("MCP tools must not include %s; names=%#v", forbidden, mcp.ToolNames)
		}
	}
	if len(mcp.RegistryToolNames) != 0 || len(mcp.NativeToolNames) != 0 {
		t.Fatalf("MCP surface must not expose Assistant registry/native tool buckets: %+v", mcp)
	}

	disabledManifest := manifest
	disabledManifest.SurfaceContract = agentcapabilities.CloneSurfaceContract(manifest.SurfaceContract)
	for i := range disabledManifest.SurfaceContract.OperatorSurfaces {
		if disabledManifest.SurfaceContract.OperatorSurfaces[i].ID == agentcapabilities.SurfaceIDPulseMCP {
			disabledManifest.SurfaceContract.OperatorSurfaces[i].Affordances = agentcapabilities.SurfaceAffordanceContract{
				Resources:          true,
				Prompts:            true,
				CapabilityMetadata: true,
			}
		}
	}
	disabledContracts := agentcapabilities.ProjectPulseIntelligenceSurfaceToolContracts(disabledManifest, []agentcapabilities.ProviderTool{
		{Name: agentcapabilities.PulseQueryToolName},
		agentcapabilities.NewPulseQuestionProviderTool(),
	})
	disabledBySurface := map[string]agentcapabilities.SurfaceToolContract{}
	for _, contract := range disabledContracts {
		disabledBySurface[contract.SurfaceID] = contract
	}
	disabledMCP := disabledBySurface[agentcapabilities.SurfaceIDPulseMCP]
	if disabledMCP.Affordances.Tools || len(disabledMCP.ToolNames) != 0 || len(disabledMCP.CapabilityNames) != 0 {
		t.Fatalf("MCP disabled tools affordance must clear static surface tools, got %+v", disabledMCP)
	}
}

func TestContract_AgentCapabilitiesManifestScopesUseAuthConstants(t *testing.T) {
	src := readAgentCapabilitiesManifestSource(t)
	for _, required := range []string{
		`agentCapabilityScopeMonitoringRead  = auth.ScopeMonitoringRead`,
		`agentCapabilityScopeMonitoringWrite = auth.ScopeMonitoringWrite`,
		`agentCapabilityScopeSettingsRead    = auth.ScopeSettingsRead`,
		`agentCapabilityScopeSettingsWrite   = auth.ScopeSettingsWrite`,
		`agentCapabilityScopeAIExecute       = auth.ScopeAIExecute`,
	} {
		if !strings.Contains(src, required) {
			t.Errorf("agent capabilities manifest must pin scope vocabulary to pkg/auth; missing %s", required)
		}
	}
	if strings.Contains(src, `Scope:          "`) || strings.Contains(src, `Scope:            "`) {
		t.Error("agent capabilities manifest must not declare scope values as local string literals")
	}

	known := map[string]bool{}
	for _, scope := range authpkg.AllKnownScopes {
		known[scope] = true
	}
	for _, cap := range agentcapabilities.CanonicalManifest().Capabilities {
		if !known[cap.Scope] {
			t.Errorf("capability %q scope = %q, want auth-owned scope vocabulary", cap.Name, cap.Scope)
		}
	}
}

func TestContract_AgentCapabilitiesFrontendTypesAreGeneratedFromManifest(t *testing.T) {
	generatorSource, err := os.ReadFile(filepath.Join("..", "..", "scripts", "generate-types.go"))
	if err != nil {
		t.Fatalf("read scripts/generate-types.go: %v", err)
	}
	generator := string(generatorSource)
	for _, required := range []string{
		`"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"`,
		`filepath.Join("frontend-modern", "src", "api", "generated", "agentCapabilities.ts")`,
		`reflect.TypeOf(agentcapabilities.Capability{})`,
		`reflect.TypeOf(agentcapabilities.CapabilityCategory{})`,
		`reflect.TypeOf(agentcapabilities.Manifest{})`,
		`reflect.TypeOf(agentcapabilities.MCPAdapterConfigFamily{})`,
		`reflect.TypeOf(agentcapabilities.MCPAdapterContract{})`,
		`reflect.TypeOf(agentcapabilities.OperatorSurfaceContract{})`,
		`reflect.TypeOf(agentcapabilities.PulseWorkflowPrompt{})`,
		`reflect.TypeOf(agentcapabilities.PulseWorkflowPromptArgument{})`,
		`reflect.TypeOf(agentcapabilities.SurfaceAffordanceContract{})`,
		`reflect.TypeOf(agentcapabilities.SurfaceContract{})`,
		`reflect.TypeOf(agentcapabilities.SurfaceContractComponent{})`,
		`reflect.TypeOf(agentcapabilities.SurfaceToolContract{})`,
		`func agentCapabilitiesAliases() string`,
		`return "AgentCapabilityActionMode"`,
		`return "AgentCapabilityApprovalPolicy"`,
		`return "Record<string, unknown>"`,
	} {
		if !strings.Contains(generator, required) {
			t.Errorf("generate-types.go must derive frontend agent capabilities types from internal/agentcapabilities; missing %s", required)
		}
	}

	generatedSource, err := os.ReadFile(filepath.Join("..", "..", "frontend-modern", "src", "api", "generated", "agentCapabilities.ts"))
	if err != nil {
		t.Fatalf("read generated agent capabilities frontend types: %v", err)
	}
	generated := string(generatedSource)
	for _, required := range []string{
		`// This file is generated from scripts/generate-types.go; DO NOT EDIT.`,
		`// Source: internal/agentcapabilities manifest structs.`,
		`export type AgentCapabilityActionMode = 'read' | 'mixed' | 'write';`,
		`export type AgentCapabilityApprovalPolicy = 'scope_only' | 'action_plan';`,
		`export interface Capability {`,
		`title?: string;`,
		`actionMode: AgentCapabilityActionMode;`,
		`approvalPolicy: AgentCapabilityApprovalPolicy;`,
		`inputSchema?: Record<string, unknown>;`,
		`outputSchema?: Record<string, unknown>;`,
		`export interface CapabilityCategory {`,
		`export interface MCPAdapterConfigFamily {`,
		`clientLabels?: string[];`,
		`export interface MCPAdapterContract {`,
		`serverName: string;`,
		`command: string;`,
		`baseUrlFlag: string;`,
		`defaultBaseUrl: string;`,
		`tokenEnv: string;`,
		`configFamilies: MCPAdapterConfigFamily[];`,
		`export interface Manifest {`,
		`surfaceContract: SurfaceContract;`,
		`surfaceToolContracts: SurfaceToolContract[];`,
		`mcpAdapter: MCPAdapterContract;`,
		`requiredScopes: string[];`,
		`categories: CapabilityCategory[];`,
		`workflowPrompts: PulseWorkflowPrompt[];`,
		`capabilities: Capability[];`,
		`export interface PulseWorkflowPrompt {`,
		`label?: string;`,
		`presentationKind?: string;`,
		`arguments?: PulseWorkflowPromptArgument[];`,
		`export interface PulseWorkflowPromptArgument {`,
		`export interface SurfaceContract {`,
		`core: SurfaceContractComponent;`,
		`proactiveEngine: SurfaceContractComponent;`,
		`operatorSurfaces: OperatorSurfaceContract[];`,
		`export interface OperatorSurfaceContract {`,
		`native: boolean;`,
		`externalAdapter: boolean;`,
		`affordances?: SurfaceAffordanceContract;`,
		`export interface SurfaceAffordanceContract {`,
		`interactiveQuestions?: boolean;`,
		`export interface SurfaceToolContract {`,
		`surfaceId: string;`,
		`toolSource: string;`,
		`toolNames: string[];`,
		`registryToolNames?: string[];`,
		`capabilityNames?: string[];`,
		`nativeToolNames?: string[];`,
	} {
		if !strings.Contains(generated, required) {
			t.Errorf("generated agent capabilities frontend types drifted from manifest contract; missing %s", required)
		}
	}

	clientSource, err := os.ReadFile(filepath.Join("..", "..", "frontend-modern", "src", "api", "agentCapabilities.ts"))
	if err != nil {
		t.Fatalf("read frontend agent capabilities client: %v", err)
	}
	client := string(clientSource)
	for _, required := range []string{
		`from './generated/agentCapabilities'`,
		`export type AgentCapability = Capability;`,
		`export type AgentCapabilityCategory = CapabilityCategory;`,
		`export type AgentCapabilitiesManifest = Manifest;`,
		`export type AgentMCPAdapterConfigFamily = MCPAdapterConfigFamily;`,
		`export type AgentMCPAdapterContract = MCPAdapterContract;`,
		`export type AgentOperatorSurfaceContract = OperatorSurfaceContract;`,
		`export type AgentWorkflowPrompt = PulseWorkflowPrompt;`,
		`export type AgentWorkflowPromptArgument = PulseWorkflowPromptArgument;`,
		`export type AgentSurfaceAffordanceContract = SurfaceAffordanceContract;`,
		`export type AgentSurfaceContractComponent = SurfaceContractComponent;`,
		`export type AgentSurfaceToolContract = SurfaceToolContract;`,
		`export interface AgentSurfaceToolPosturePresentation {`,
		`export interface AgentSurfaceContractEntry {`,
		`normalizeAgentSurfaceToolContract(`,
		`findAgentSurfaceToolContract(`,
		`getAgentManifestSurfaceToolContract(`,
		`getAgentManifestSurfaceToolContracts(`,
		`getAgentSurfaceToolPosturePresentation(`,
		`normalizeAgentMCPAdapter(`,
		`getAgentMCPClientExamples(`,
		`formatAgentOpenCodeMCPConfig(`,
		`formatAgentMCPServersConfig(`,
		`getAgentSurfaceContractEntries(`,
		`getAgentWorkflowPrompts(`,
		`normalizeSurfaceAffordances(`,
		`surfaceAffordanceLabels(`,
		`manifest?.workflowPrompts`,
		`manifest.surfaceContract?.core`,
		`manifest.surfaceContract?.proactiveEngine`,
		`manifest.surfaceContract?.operatorSurfaces`,
		`manifest?.surfaceToolContracts`,
		`manifest?.mcpAdapter`,
	} {
		if !strings.Contains(client, required) {
			t.Errorf("frontend agent capabilities client must consume generated manifest types; missing %s", required)
		}
	}
	for _, forbidden := range []string{
		`export interface AgentCapability {`,
		`export interface AgentCapabilityCategory {`,
		`export interface AgentCapabilitiesManifest {`,
		`export interface AgentMCPAdapterContract {`,
		`AGENT_CAPABILITY_NAME_SUBSCRIBE_EVENTS`,
		`getRequestResponseCapabilityNames(`,
		`isRequestResponseAgentCapability(`,
		`getAgentMCPSurfaceToolContract(`,
		`subscribe_events`,
	} {
		if strings.Contains(client, forbidden) {
			t.Errorf("frontend agent capabilities client must not redeclare manifest types locally; found %s", forbidden)
		}
	}

	panelSource, err := os.ReadFile(filepath.Join("..", "..", "frontend-modern", "src", "components", "Settings", "AgentIntegrationsPanel.tsx"))
	if err != nil {
		t.Fatalf("read AgentIntegrationsPanel.tsx: %v", err)
	}
	panel := string(panelSource)
	for _, required := range []string{
		`getAgentSurfaceContractEntries`,
		`normalizeAgentMCPAdapter`,
		`formatAgentOpenCodeMCPConfig`,
		`formatAgentMCPServersConfig`,
		`AGENT_SURFACE_ID_PULSE_MCP`,
		`getAgentManifestSurfaceToolContract`,
		`getAgentSurfaceToolPosturePresentation`,
		`surfaceContractEntries`,
		`mcpSurfaceToolPosture`,
		`data-testid="agent-mcp-tool-posture"`,
		`External agents expose`,
	} {
		if !strings.Contains(panel, required) {
			t.Errorf("Agent integrations panel must project the manifest surface contract through the shared client; missing %s", required)
		}
	}
	for _, forbidden := range []string{
		`Pulse Intelligence Core`,
		`Pulse Assistant`,
		`Pulse Patrol`,
		`Pulse MCP`,
	} {
		if strings.Contains(panel, forbidden) {
			t.Errorf("Agent integrations panel must not duplicate manifest surface labels locally; found %s", forbidden)
		}
	}
	for _, forbidden := range []string{
		`const MCP_CLIENT_EXAMPLES`,
		`function formatClaudeMcpConfig`,
		`function formatOpenCodeMcpConfig`,
		`command: 'pulse-mcp'`,
		`PULSE_API_TOKEN: '<your-api-token>'`,
		`getAgentMCPClientExamples`,
		`getAgentMCPSurfaceToolContract`,
		`subscribe_events`,
		`capability.name`,
	} {
		if strings.Contains(panel, forbidden) {
			t.Errorf("Agent integrations panel must consume manifest-backed MCP adapter helpers instead of local setup snippets; found %s", forbidden)
		}
	}

	assistantChatSource, err := os.ReadFile(filepath.Join("..", "..", "frontend-modern", "src", "components", "AI", "Chat", "index.tsx"))
	if err != nil {
		t.Fatalf("read AI Chat index.tsx: %v", err)
	}
	assistantChat := string(assistantChatSource)
	for _, required := range []string{
		`getAgentSurfaceToolPosturePresentation`,
		`type AgentSurfaceToolContract`,
		`AIChatAPI.getAssistantSurfaceTools()`,
		`data-testid="assistant-surface-tools-health"`,
	} {
		if !strings.Contains(assistantChat, required) {
			t.Errorf("Assistant chat shell must consume the authenticated runtime surface-tool contract through the shared frontend client; missing %s", required)
		}
	}
	for _, forbidden := range []string{
		`'pulse_query'`,
		`"pulse_query"`,
		`'pulse_read'`,
		`"pulse_read"`,
		`'pulse_question'`,
		`"pulse_question"`,
	} {
		if strings.Contains(assistantChat, forbidden) {
			t.Errorf("Assistant chat shell must not hardcode native Assistant tool names for surface posture; found %s", forbidden)
		}
	}
}

func TestContract_AgentCapabilitiesPatrolFindingScopesMatchAPIAuthorization(t *testing.T) {
	manifest := agentcapabilities.CanonicalManifest()
	byName := map[string]agentcapabilities.Capability{}
	for _, cap := range manifest.Capabilities {
		byName[cap.Name] = cap
	}

	relayBacked := map[string]relayMobileRuntimeRouteID{
		agentcapabilities.ListFindingsCapabilityName:       relayMobileRoutePatrolFindingsList,
		agentcapabilities.AcknowledgeFindingCapabilityName: relayMobileRoutePatrolAcknowledge,
		agentcapabilities.SnoozeFindingCapabilityName:      relayMobileRoutePatrolSnooze,
		agentcapabilities.DismissFindingCapabilityName:     relayMobileRoutePatrolDismiss,
	}
	for capabilityName, routeID := range relayBacked {
		capability, ok := byName[capabilityName]
		if !ok {
			t.Fatalf("agent capabilities manifest missing %s", capabilityName)
		}
		route := relayMobileRuntimeRouteSpecFor(routeID)
		if capability.Method != route.method || capability.Path != route.path || capability.Scope != route.requiredScope {
			t.Fatalf("%s manifest route/scope = %s %s => %s, want API authorization route %s %s => %s",
				capabilityName, capability.Method, capability.Path, capability.Scope, route.method, route.path, route.requiredScope)
		}
	}

	resolve, ok := byName[agentcapabilities.ResolveFindingCapabilityName]
	if !ok {
		t.Fatalf("agent capabilities manifest missing %s", agentcapabilities.ResolveFindingCapabilityName)
	}
	if resolve.Method != http.MethodPost || resolve.Path != "/api/ai/patrol/resolve" || resolve.Scope != config.ScopeAIExecute {
		t.Fatalf("resolve_finding manifest route/scope = %s %s => %s, want API authorization route POST /api/ai/patrol/resolve => %s",
			resolve.Method, resolve.Path, resolve.Scope, config.ScopeAIExecute)
	}
	var resolveSchema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(resolve.InputSchema, &resolveSchema); err != nil {
		t.Fatalf("decode resolve_finding input schema: %v", err)
	}
	if _, ok := resolveSchema.Properties[agentcapabilities.ResolutionNoteArgumentName]; !ok {
		t.Fatalf("resolve_finding input schema missing optional %s property", agentcapabilities.ResolutionNoteArgumentName)
	}
	for _, required := range resolveSchema.Required {
		if required == agentcapabilities.ResolutionNoteArgumentName {
			t.Fatalf("resolve_finding input schema made optional %s required", agentcapabilities.ResolutionNoteArgumentName)
		}
	}

	dismiss := byName[agentcapabilities.DismissFindingCapabilityName]
	var dismissSchema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(dismiss.InputSchema, &dismissSchema); err != nil {
		t.Fatalf("decode dismiss_finding input schema: %v", err)
	}
	if _, ok := dismissSchema.Properties[agentcapabilities.NoteArgumentName]; !ok {
		t.Fatalf("dismiss_finding input schema missing optional %s property", agentcapabilities.NoteArgumentName)
	}
	for _, required := range dismissSchema.Required {
		if required == agentcapabilities.NoteArgumentName {
			t.Fatalf("dismiss_finding input schema made optional %s required", agentcapabilities.NoteArgumentName)
		}
	}
}

func TestContract_AgentCapabilitiesManifestRoutesReachRouterWithAdvertisedScope(t *testing.T) {
	rawToken := "agent-route-contract-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	args := map[string]any{
		"actionId":                              "act_contract",
		"capabilityName":                        "restart",
		"duration_hours":                        1,
		agentcapabilities.FindingIDArgumentName: "finding-contract",
		"host":                                  "https://pve-contract.example.com:8006",
		"intentionallyOffline":                  false,
		"name":                                  "contract-node",
		"neverAutoRemediate":                    false,
		"nodeId":                                "pve-contract",
		"outcome":                               "approved",
		"params":                                map[string]any{},
		"password":                              "not-a-real-secret",
		agentcapabilities.ReasonArgumentName:    "contract proof",
		"requestedBy":                           "agent:contract",
		"requestId":                             "req-contract",
		"resourceId":                            "vm101",
		"subnet":                                "auto",
		"type":                                  "pve",
		"use_cache":                             true,
		"user":                                  "root@pam",
		"verifySSL":                             false,
		"monitorVMs":                            true,
		"monitorContainers":                     true,
		"monitorStorage":                        true,
		"monitorBackups":                        true,
		"monitorPhysicalDisks":                  true,
		"temperatureMonitoringEnabled":          true,
	}

	for _, capability := range agentcapabilities.CanonicalManifest().Capabilities {
		capability := capability
		t.Run(capability.Name, func(t *testing.T) {
			projected, err := agentcapabilities.ProjectCapabilityCall(capability, args)
			if err != nil {
				t.Fatalf("project manifest route %s %s: %v", capability.Method, capability.Path, err)
			}

			body := bytes.NewReader(nil)
			if projected.HasBody {
				body = bytes.NewReader(projected.Body)
			}
			req := httptest.NewRequest(capability.Method, projected.Path, body)
			req.Header.Set(agentcapabilities.AgentAPITokenHeader, rawToken)
			if projected.HasBody {
				req.Header.Set("Content-Type", "application/json")
			}

			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s projected to %s %s, got HTTP %d body=%s, want missing-scope 403 for %s",
					capability.Name, capability.Method, projected.Path, rec.Code, rec.Body.String(), capability.Scope)
			}
			if !strings.Contains(rec.Body.String(), capability.Scope) {
				t.Fatalf("%s projected to %s %s, missing-scope body %q does not mention advertised scope %q",
					capability.Name, capability.Method, projected.Path, rec.Body.String(), capability.Scope)
			}
		})
	}
}

func TestContract_AgentCapabilitiesRequiredScopeSummaryUsesManifestScopes(t *testing.T) {
	scopesSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "scopes.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/scopes.go: %v", err)
	}
	src := string(scopesSource)
	for _, required := range []string{
		`func NormalizeRequiredScopes(scopes []string) []string`,
		`func RequiredCapabilityScopes(capabilities []Capability) []string`,
		`for _, scope := range auth.AllKnownScopes`,
		`func RequiredScopeList(scopes []string) string`,
		`func RequiredCapabilityScopeList(capabilities []Capability) string`,
		`func ManifestRequiredScopeList(manifest *Manifest) string`,
	} {
		if !strings.Contains(src, required) {
			t.Errorf("agent capabilities scope summary must stay in the shared manifest contract; missing %s", required)
		}
	}

	got := agentcapabilities.RequiredCapabilityScopes(agentcapabilities.CanonicalManifest().Capabilities)
	want := []string{
		authpkg.ScopeMonitoringRead,
		authpkg.ScopeMonitoringWrite,
		authpkg.ScopeSettingsRead,
		authpkg.ScopeSettingsWrite,
		authpkg.ScopeAIExecute,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("canonical agent capability scopes = %v, want %v", got, want)
	}
	manifest := agentcapabilities.CanonicalManifest()
	if !reflect.DeepEqual(manifest.RequiredScopes, want) {
		t.Fatalf("canonical agent capability manifest requiredScopes = %v, want %v", manifest.RequiredScopes, want)
	}

	mcpSource, err := os.ReadFile(filepath.Join("..", "..", "cmd", "pulse-mcp", "main.go"))
	if err != nil {
		t.Fatalf("read cmd/pulse-mcp/main.go: %v", err)
	}
	mcp := string(mcpSource)
	if !strings.Contains(mcp, `agentcapabilities.ManifestRequiredScopeList(manifest)`) {
		t.Error("pulse-mcp token guidance must consume the manifest-owned requiredScopes summary")
	}
	if strings.Contains(mcp, `RequiredCapabilityScopeList(capabilities)`) {
		t.Error("pulse-mcp token guidance must not recompute current manifest scopes from capability rows")
	}
	if strings.Contains(mcp, `monitoring:read scope (and monitoring:write`) {
		t.Error("pulse-mcp startup guidance must not carry a hand-maintained partial scope list")
	}
}

func TestContract_PulseMCPReadmeManifestProjectionStaysGenerated(t *testing.T) {
	readmeBytes, err := os.ReadFile(filepath.Join("..", "..", "cmd", "pulse-mcp", "README.md"))
	if err != nil {
		t.Fatalf("read cmd/pulse-mcp/README.md: %v", err)
	}
	readme := string(readmeBytes)
	manifest := agentcapabilities.CanonicalManifest()
	generatorSource, err := os.ReadFile(filepath.Join("..", "..", "scripts", "generate-pulse-intelligence-docs.go"))
	if err != nil {
		t.Fatalf("read scripts/generate-pulse-intelligence-docs.go: %v", err)
	}
	generator := string(generatorSource)
	for _, required := range []string{
		"regeneratePulseIntelligenceDocs",
		"regeneratePulseMCPReadme",
		"regeneratePublicPulseIntelligenceOverview",
		"agentcapabilities.MCPSurfaceContractMarkdown(manifest.SurfaceContract)",
		"agentcapabilities.PulseIntelligenceOverviewMarkdown(manifest.SurfaceContract)",
		"agentcapabilities.MCPClientConfigMarkdown(manifest.MCPAdapter)",
		"agentcapabilities.MCPToolCapabilityInventoryMarkdown(manifest)",
		"agentcapabilities.MCPPromptInventoryMarkdown(manifest)",
		"agentcapabilities.MCPErrorCodeInventoryMarkdown(manifest)",
		"agentcapabilities.PulseIntelligenceOverviewStartMarker",
		"agentcapabilities.MCPReadmeSurfaceContractStartMarker",
		"agentcapabilities.MCPReadmeClientConfigStartMarker",
	} {
		if !strings.Contains(generator, required) {
			t.Errorf("Pulse Intelligence docs generator must own manifest-backed docs projection; missing %q", required)
		}
	}
	markdownSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "markdown.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/markdown.go: %v", err)
	}
	markdown := string(markdownSource)
	for _, required := range []string{
		"func MCPPromptInventoryMarkdown(manifest Manifest) string",
		"MCPManifestSurfacePromptProjectionSupported(manifest, SurfaceIDPulseMCP)",
		"ProjectMCPWorkflowPrompts(ManifestPulseWorkflowPrompts(manifest))",
	} {
		if !strings.Contains(markdown, required) {
			t.Errorf("MCP prompt inventory docs must use the shared surface prompt projection; missing %q", required)
		}
	}
	if strings.Contains(markdown, "prompts := ManifestPulseWorkflowPrompts(manifest)") {
		t.Fatal("MCP prompt inventory docs must not project raw workflow prompts without the MCP surface affordance gate")
	}

	surfaceBlock, err := markedContractBlock(readme, agentcapabilities.MCPReadmeSurfaceContractStartMarker, agentcapabilities.MCPReadmeSurfaceContractEndMarker)
	if err != nil {
		t.Fatal(err)
	}
	wantSurfaceBlock := agentcapabilities.MCPSurfaceContractMarkdown(manifest.SurfaceContract)
	if surfaceBlock != wantSurfaceBlock {
		t.Fatalf("pulse-mcp README surface-contract block drifted from canonical manifest; run `go run ./scripts/generate-pulse-intelligence-docs.go`\n%s\nwant:\n%s", surfaceBlock, wantSurfaceBlock)
	}

	scopeBlock, err := markedContractBlock(readme, agentcapabilities.MCPReadmeScopeListStartMarker, agentcapabilities.MCPReadmeScopeListEndMarker)
	if err != nil {
		t.Fatal(err)
	}
	wantScopeBlock := agentcapabilities.ManifestRequiredScopeMarkdownList(manifest)
	if scopeBlock != wantScopeBlock {
		t.Fatalf("pulse-mcp README scope block drifted from canonical manifest:\n%s\nwant:\n%s", scopeBlock, wantScopeBlock)
	}

	clientConfigBlock, err := markedContractBlock(readme, agentcapabilities.MCPReadmeClientConfigStartMarker, agentcapabilities.MCPReadmeClientConfigEndMarker)
	if err != nil {
		t.Fatal(err)
	}
	wantClientConfigBlock := agentcapabilities.MCPClientConfigMarkdown(manifest.MCPAdapter)
	if clientConfigBlock != wantClientConfigBlock {
		t.Fatalf("pulse-mcp README client config block drifted from canonical manifest; run `go run ./scripts/generate-pulse-intelligence-docs.go`\n%s\nwant:\n%s", clientConfigBlock, wantClientConfigBlock)
	}

	toolBlock, err := markedContractBlock(readme, agentcapabilities.MCPReadmeToolInventoryStartMarker, agentcapabilities.MCPReadmeToolInventoryEndMarker)
	if err != nil {
		t.Fatal(err)
	}
	wantToolBlock := agentcapabilities.MCPToolCapabilityInventoryMarkdown(manifest)
	if toolBlock != wantToolBlock {
		t.Fatalf("pulse-mcp README tool block drifted from canonical manifest; run `go run ./scripts/generate-pulse-intelligence-docs.go`\n%s\nwant:\n%s", toolBlock, wantToolBlock)
	}

	promptBlock, err := markedContractBlock(readme, agentcapabilities.MCPReadmePromptInventoryStartMarker, agentcapabilities.MCPReadmePromptInventoryEndMarker)
	if err != nil {
		t.Fatal(err)
	}
	wantPromptBlock := agentcapabilities.MCPPromptInventoryMarkdown(manifest)
	if promptBlock != wantPromptBlock {
		t.Fatalf("pulse-mcp README prompt block drifted from canonical manifest; run `go run ./scripts/generate-pulse-intelligence-docs.go`\n%s\nwant:\n%s", promptBlock, wantPromptBlock)
	}

	errorBlock, err := markedContractBlock(readme, agentcapabilities.MCPReadmeErrorInventoryStartMarker, agentcapabilities.MCPReadmeErrorInventoryEndMarker)
	if err != nil {
		t.Fatal(err)
	}
	wantErrorBlock := agentcapabilities.MCPErrorCodeInventoryMarkdown(manifest)
	if errorBlock != wantErrorBlock {
		t.Fatalf("pulse-mcp README error-code block drifted from canonical manifest; run `go run ./scripts/generate-pulse-intelligence-docs.go`\n%s\nwant:\n%s", errorBlock, wantErrorBlock)
	}
	if strings.Contains(readme, "As of this writing") {
		t.Fatal("pulse-mcp README must not frame the manifest-derived tool inventory as a hand-maintained snapshot")
	}
}

func TestContract_AgentSubstrateDocReflectsCurrentMCPOnboarding(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "docs", "AGENT_SUBSTRATE.md"))
	if err != nil {
		t.Fatalf("read docs/AGENT_SUBSTRATE.md: %v", err)
	}
	doc := string(source)
	for _, required := range []string{
		"Settings -> API Access -> Agent integrations",
		"client-ready `pulse-mcp` config snippets",
		"manifest-owned MCP adapter setup contract",
		"surface contract and affordance badges",
		"server name, command",
		"supported client config families",
		"OpenCode's native `opencode.json` / `mcp` shape",
		"common `mcpServers` shape for Claude-style clients",
		"from the same adapter contract",
		"shows both",
		"OpenCode's native `opencode.json` shape",
		"common `mcpServers`",
		"`pulse-mcp` also has a published distribution path",
		"`install-mcp.sh` and `install-mcp.ps1`",
		"release installers",
		"frontend-modern/src/components/Settings/AgentIntegrationsPanel.tsx",
	} {
		if !strings.Contains(doc, required) {
			t.Errorf("docs/AGENT_SUBSTRATE.md must reflect current MCP onboarding; missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"no Settings panel listing the declared capabilities",
		"no \"generate MCP config snippet\" button",
		"no token template that says \"create token for agent integration\"",
		"A distribution path for `pulse-mcp`. Today an integrator must",
		"clone the repo and `go build`",
		"generates the reusable `pulse-mcp` server block",
	} {
		if strings.Contains(doc, forbidden) {
			t.Errorf("docs/AGENT_SUBSTRATE.md must not preserve stale pre-onboarding gap copy; found %q", forbidden)
		}
	}
}

func TestContract_PulseIntelligenceOverviewPinsPatrolPrimaryCore(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "docs", "AI.md"))
	if err != nil {
		t.Fatalf("read docs/AI.md: %v", err)
	}
	doc := string(source)
	manifest := agentcapabilities.CanonicalManifest()
	overviewBlock, err := markedContractBlock(doc, agentcapabilities.PulseIntelligenceOverviewStartMarker, agentcapabilities.PulseIntelligenceOverviewEndMarker)
	if err != nil {
		t.Fatal(err)
	}
	wantOverviewBlock := agentcapabilities.PulseIntelligenceOverviewMarkdown(manifest.SurfaceContract)
	if overviewBlock != wantOverviewBlock {
		t.Fatalf("docs/AI.md Pulse Intelligence overview block drifted from canonical manifest:\n%s\nwant:\n%s", overviewBlock, wantOverviewBlock)
	}

	normalized := strings.Join(strings.Fields(doc), " ")
	for _, required := range []string{
		"Pulse Intelligence Core",
		"Canonical context, governed actions, safety gates, approval state, action audit, and verification",
		"Patrol as the primary built-in operator",
		"Assistant plus MCP as access paths",
		"Patrol is the first-party operations surface: it checks infrastructure, investigates issues, follows the chosen Patrol mode before acting, verifies outcomes, and records what happened",
		"Pulse Assistant",
		"Pulse MCP",
		"contextual explanation, approval, and handoff surface for Patrol findings",
		"Pulse Assistant and `pulse-mcp` are sibling surfaces over Pulse Intelligence, not competing implementations, and neither replaces the other",
		"Assistant remains the in-app Pro surface",
		"New operational capabilities should be added to the canonical API manifest first",
		"MCP-only actions and Assistant-only copies of the same business logic are drift",
	} {
		if !strings.Contains(normalized, required) {
			t.Errorf("docs/AI.md must pin Pulse Intelligence Core Patrol-primary product framing; missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"three supported operator-facing ways",
		"three supported surfaces",
		"two supported operator-facing surfaces",
		"proactive detection and investigation engine running on the shared Pulse Intelligence Core",
		"Pulse Assistant should be replaced by",
		"MCP replaces Pulse Assistant",
		"Assistant can be removed",
		"hand-maintained Assistant tool table",
	} {
		if strings.Contains(normalized, forbidden) {
			t.Errorf("docs/AI.md must not drift away from the shared-core/Patrol-primary strategy; found %q", forbidden)
		}
	}
}

func TestContract_AgentParadigmDocReflectsGenericMCPOnboarding(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "docs", "releases", "AGENT_PARADIGM.md"))
	if err != nil {
		t.Fatalf("read docs/releases/AGENT_PARADIGM.md: %v", err)
	}
	doc := string(source)
	for _, required := range []string{
		"Claude Desktop, Claude Code, OpenCode, other MCP clients",
		"client-ready MCP config snippets",
		"OpenCode's native\n  `opencode.json` / `mcp` shape",
		"Drivable from MCP clients in one command",
		"Wire it into any MCP-speaking client",
		"OpenCode-native `mcp` block",
		"common\n  `mcpServers` block",
		"manifest `requiredScopes`",
		"read-only subset",
	} {
		if !strings.Contains(doc, required) {
			t.Errorf("docs/releases/AGENT_PARADIGM.md must reflect generic MCP onboarding; missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"Claude Desktop / Claude Code",
		"Drivable from Claude in one command",
		"Wire it into Claude Desktop or Claude Code",
		"adapter for Claude Desktop and Claude Code",
		"clients that accept\n  `mcpServers`",
		"`monitoring:read` (and",
	} {
		if strings.Contains(doc, forbidden) {
			t.Errorf("docs/releases/AGENT_PARADIGM.md must not preserve stale Claude-only or partial-scope copy; found %q", forbidden)
		}
	}
}

func markedContractBlock(content, startMarker, endMarker string) (string, error) {
	start := strings.Index(content, startMarker)
	if start < 0 {
		return "", fmt.Errorf("missing marker %s", startMarker)
	}
	bodyStart := start + len(startMarker)
	end := strings.Index(content[bodyStart:], endMarker)
	if end < 0 {
		return "", fmt.Errorf("missing marker %s", endMarker)
	}
	return strings.TrimSpace(content[bodyStart : bodyStart+end]), nil
}

func TestContract_AssistantProviderSeamsDoNotUseMCPTerminology(t *testing.T) {
	chatServiceSource, err := os.ReadFile(filepath.Join("..", "ai", "chat", "service.go"))
	if err != nil {
		t.Fatalf("read internal/ai/chat/service.go: %v", err)
	}
	chatService := string(chatServiceSource)
	for _, fragment := range []string{
		`AssistantAlertProvider              = tools.AlertProvider`,
		`AssistantFindingsProvider           = tools.FindingsProvider`,
		`AssistantDiscoveryProvider          = tools.DiscoveryProvider`,
		`AssistantUnifiedResourceProvider    = tools.UnifiedResourceProvider`,
		`func (s *Service) ExecuteAssistantTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error)`,
		`Msg("Executing Assistant registry tool directly")`,
	} {
		if !strings.Contains(chatService, fragment) {
			t.Errorf("native Assistant provider seam must be Assistant-owned, missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`MCPAlertProvider`,
		`MCPFindingsProvider`,
		`MCPDiscoveryProvider`,
		`MCPUnifiedResourceProvider`,
		`func (s *Service) ExecuteMCPTool(`,
		`MCP provider type aliases`,
		`Msg("Executing MCP tool directly")`,
	} {
		if strings.Contains(chatService, fragment) {
			t.Errorf("native Assistant provider seam must not be named as the MCP adapter surface; found %s", fragment)
		}
	}

	apiHandlerSource, err := os.ReadFile("ai_handler.go")
	if err != nil {
		t.Fatalf("read internal/api/ai_handler.go: %v", err)
	}
	apiHandler := string(apiHandlerSource)
	for _, fragment := range []string{
		`SetAlertProvider(provider chat.AssistantAlertProvider)`,
		`SetFindingsProvider(provider chat.AssistantFindingsProvider)`,
		`SetDiscoveryProvider(provider chat.AssistantDiscoveryProvider)`,
		`SetUnifiedResourceProvider(provider chat.AssistantUnifiedResourceProvider)`,
	} {
		if !strings.Contains(apiHandler, fragment) {
			t.Errorf("API chat service interface must expose Assistant provider seams; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`chat.MCPAlertProvider`,
		`chat.MCPFindingsProvider`,
		`chat.MCPDiscoveryProvider`,
		`chat.MCPUnifiedResourceProvider`,
		`for MCP tools`,
	} {
		if strings.Contains(apiHandler, fragment) {
			t.Errorf("API chat service interface must not describe native Assistant providers as MCP tools; found %s", fragment)
		}
	}

	settingsHandlerSource, err := os.ReadFile("ai_handlers.go")
	if err != nil {
		t.Fatalf("read internal/api/ai_handlers.go: %v", err)
	}
	settingsHandler := string(settingsHandlerSource)
	for _, fragment := range []string{
		`Used by Router to update Assistant tool visibility without restarting AI chat.`,
		`Update Assistant control settings if control level or protected guests changed.`,
		`This updates tool visibility without restarting AI chat.`,
	} {
		if !strings.Contains(settingsHandler, fragment) {
			t.Errorf("AI settings handler must describe control refresh as native Assistant tool visibility; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`Update MCP control settings`,
		`MCP control settings`,
		`MCP tool visibility`,
	} {
		if strings.Contains(settingsHandler, fragment) {
			t.Errorf("AI settings handler must not describe native Assistant control refresh as MCP wiring; found %s", fragment)
		}
	}

	routerSource, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read internal/api/router.go: %v", err)
	}
	router := string(routerSource)
	for _, fragment := range []string{
		`ai.NewFindingsToolAdapter(findingsStore)`,
		`tools.NewAlertManagerToolAdapter(alertManager)`,
		`tools.NewDiscoveryToolAdapter(adapter)`,
		`NewAssistantAgentProfileManager(persistence, licenseSvc)`,
		`AI chat Assistant tool providers wired`,
		`Wire control settings change callback to update Assistant tool visibility`,
	} {
		if !strings.Contains(router, fragment) {
			t.Errorf("router must wire native Assistant providers without MCP terminology; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`ai.NewFindingsMCPAdapter(findingsStore)`,
		`tools.NewAlertManagerMCPAdapter(alertManager)`,
		`tools.NewDiscoveryMCPAdapter(adapter)`,
		`NewMCPAgentProfileManager(persistence, licenseSvc)`,
		`AI chat MCP tool providers wired`,
		`org-scoped MCP tool providers`,
	} {
		if strings.Contains(router, fragment) {
			t.Errorf("router must not describe native Assistant provider wiring as MCP adapter wiring; found %s", fragment)
		}
	}

	relaySource, err := os.ReadFile("router_routes_ai_relay.go")
	if err != nil {
		t.Fatalf("read internal/api/router_routes_ai_relay.go: %v", err)
	}
	relay := string(relaySource)
	for _, fragment := range []string{
		`toolAdapter := &assistantToolAdapter{handler: h}`,
		`AssistantToolExecutor: toolAdapter`,
		`type assistantToolAdapter struct`,
		`var _ aicontracts.ApprovedAssistantToolExecutor = (*assistantToolAdapter)(nil)`,
		`func (m *assistantToolAdapter) ExecuteApprovedAssistantTool(ctx context.Context, command, approvalID string) (string, int, error)`,
	} {
		if !strings.Contains(relay, fragment) {
			t.Errorf("approved Assistant tool adapter must expose native Assistant execution from the current runtime; missing %s", fragment)
		}
	}
	if !strings.Contains(relay, `chatService.ExecuteAssistantTool(ctx, params.Name, params.Arguments)`) {
		t.Error("approved Assistant tool adapter must delegate to the native Assistant tool executor")
	}
	if strings.Contains(relay, `type mcpToolAdapter struct`) || strings.Contains(relay, `&mcpToolAdapter{`) {
		t.Error("approved Assistant tool adapter must not be named as the MCP adapter surface")
	}
	for _, fragment := range []string{
		`MCPExecutor:           toolAdapter`,
		`MCPExecutor: toolAdapter`,
		`var _ aicontracts.MCPToolExecutor = (*assistantToolAdapter)(nil)`,
		`func (m *assistantToolAdapter) ExecuteMCPTool(ctx context.Context, command, approvalID string) (string, int, error)`,
		`chatService.ExecuteMCPTool(`,
	} {
		if strings.Contains(relay, fragment) {
			t.Errorf("current runtime must not expose native Assistant execution through the legacy MCP executor seam; found %s", fragment)
		}
	}

	contractSource, err := os.ReadFile(filepath.Join("..", "..", "pkg", "aicontracts", "fix_execution.go"))
	if err != nil {
		t.Fatalf("read pkg/aicontracts/fix_execution.go: %v", err)
	}
	contracts := string(contractSource)
	for _, fragment := range []string{
		`type ApprovedAssistantToolExecutor interface`,
		`ExecuteApprovedAssistantTool(ctx context.Context, command, approvalID string) (output string, exitCode int, err error)`,
	} {
		if !strings.Contains(contracts, fragment) {
			t.Errorf("cross-repo approved tool contract must expose native Assistant executor only; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`type MCPToolExecutor interface`,
		`ExecuteMCPTool(ctx context.Context, command, approvalID string) (output string, exitCode int, err error)`,
		`Prefer ApprovedAssistantToolExecutor for new integrations`,
	} {
		if strings.Contains(contracts, fragment) {
			t.Errorf("cross-repo approved tool contract must not preserve legacy MCP executor compatibility; found %s", fragment)
		}
	}

	extensionSource, err := os.ReadFile(filepath.Join("..", "..", "pkg", "extensions", "ai_autofix.go"))
	if err != nil {
		t.Fatalf("read pkg/extensions/ai_autofix.go: %v", err)
	}
	extensionsSrc := string(extensionSource)
	for _, fragment := range []string{
		`AssistantToolExecutor aicontracts.ApprovedAssistantToolExecutor`,
		`func (d AIAutoFixHandlerDeps) ResolveApprovedAssistantToolExecutor() aicontracts.ApprovedAssistantToolExecutor`,
		`return d.AssistantToolExecutor`,
	} {
		if !strings.Contains(extensionsSrc, fragment) {
			t.Errorf("auto-fix extension deps must expose native Assistant tool execution without MCP compatibility; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`MCPExecutor`,
		`MCPToolExecutor`,
		`legacyMCPApprovedAssistantToolExecutor`,
		`ExecuteMCPTool`,
		`compatibility fallback for enterprise binders`,
	} {
		if strings.Contains(extensionsSrc, fragment) {
			t.Errorf("auto-fix extension deps must not expose legacy MCP executor compatibility; found %s", fragment)
		}
	}

	findingsAdapterSource, err := os.ReadFile(filepath.Join("..", "ai", "findings_tools_adapter.go"))
	if err != nil {
		t.Fatalf("read internal/ai/findings_tools_adapter.go: %v", err)
	}
	findingsAdapter := string(findingsAdapterSource)
	if !strings.Contains(findingsAdapter, `type FindingsToolAdapter struct`) ||
		!strings.Contains(findingsAdapter, `func NewFindingsToolAdapter(store *FindingsStore) *FindingsToolAdapter`) {
		t.Error("findings store adapter must be named for the native tool registry")
	}
	if strings.Contains(findingsAdapter, `FindingsMCPAdapter`) || strings.Contains(findingsAdapter, `NewFindingsMCPAdapter`) {
		t.Error("findings store adapter must not be named as an MCP adapter")
	}

	toolAdaptersSource, err := os.ReadFile(filepath.Join("..", "ai", "tools", "adapters.go"))
	if err != nil {
		t.Fatalf("read internal/ai/tools/adapters.go: %v", err)
	}
	toolAdapters := string(toolAdaptersSource)
	for _, fragment := range []string{
		`type AlertManagerToolAdapter struct`,
		`func NewAlertManagerToolAdapter(manager AlertManager) *AlertManagerToolAdapter`,
		`type RecoveryPointsToolAdapter struct`,
		`func NewRecoveryPointsToolAdapter(manager *recoverymanager.Manager, orgID string) *RecoveryPointsToolAdapter`,
		`type DiscoveryToolAdapter struct`,
		`func NewDiscoveryToolAdapter(source DiscoverySource) *DiscoveryToolAdapter`,
	} {
		if !strings.Contains(toolAdapters, fragment) {
			t.Errorf("Assistant runtime adapters must be named for the native tool registry; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`MCPAdapter`,
		`NewAlertManagerMCPAdapter`,
		`NewRecoveryPointsMCPAdapter`,
		`NewDiscoveryMCPAdapter`,
		`MCP tools`,
		`mcp.`,
	} {
		if strings.Contains(toolAdapters, fragment) {
			t.Errorf("Assistant runtime adapters must not be named as MCP adapters; found %s", fragment)
		}
	}

	intelligenceAdaptersSource, err := os.ReadFile(filepath.Join("..", "ai", "adapters", "adapters.go"))
	if err != nil {
		t.Fatalf("read internal/ai/adapters/adapters.go: %v", err)
	}
	intelligenceAdapters := string(intelligenceAdaptersSource)
	for _, fragment := range []string{
		`type IncidentRecorderToolAdapter struct`,
		`func NewIncidentRecorderToolAdapter(recorder IncidentRecorderSource) *IncidentRecorderToolAdapter`,
		`type EventCorrelatorToolAdapter struct`,
		`func NewEventCorrelatorToolAdapter(correlator EventCorrelatorSource) *EventCorrelatorToolAdapter`,
	} {
		if !strings.Contains(intelligenceAdapters, fragment) {
			t.Errorf("Assistant intelligence adapters must be named for the native tool registry; missing %s", fragment)
		}
	}
	if strings.Contains(intelligenceAdapters, `MCPAdapter`) || strings.Contains(intelligenceAdapters, `NewIncidentRecorderMCPAdapter`) || strings.Contains(intelligenceAdapters, `NewEventCorrelatorMCPAdapter`) {
		t.Error("Assistant intelligence adapters must not be named as MCP adapters")
	}

	agentProfilesSource, err := os.ReadFile("agent_profiles_tools.go")
	if err != nil {
		t.Fatalf("read internal/api/agent_profiles_tools.go: %v", err)
	}
	agentProfiles := string(agentProfilesSource)
	if !strings.Contains(agentProfiles, `type AssistantAgentProfileManager struct`) ||
		!strings.Contains(agentProfiles, `func NewAssistantAgentProfileManager(persistence *config.ConfigPersistence, licenseService licenseFeatureChecker) *AssistantAgentProfileManager`) {
		t.Error("agent profile manager must be named for the native Assistant tool registry")
	}
	if strings.Contains(agentProfiles, `MCPAgentProfileManager`) || strings.Contains(agentProfiles, `NewMCPAgentProfileManager`) || strings.Contains(agentProfiles, `MCP tools`) {
		t.Error("agent profile manager must not be named as an MCP adapter")
	}
}

// TestContract_PulseMCPAdapterProjectsAgentCapabilitiesManifest pins the
// external-agent adapter boundary: pulse-mcp must stay a projection of the
// canonical capabilities manifest, not a second action registry.
func TestContract_PulseMCPAdapterProjectsAgentCapabilitiesManifest(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "cmd", "pulse-mcp", "main.go"))
	if err != nil {
		t.Fatalf("read cmd/pulse-mcp/main.go: %v", err)
	}
	src := string(source)
	required := []string{
		`internal/agentcapabilities`,
		`type agentCapability = agentcapabilities.Capability`,
		`type agentCapabilitiesManifest = agentcapabilities.Manifest`,
		`type jsonRPCRequest = agentcapabilities.JSONRPCRequest`,
		`agentcapabilities.ServeJSONRPCLines(`,
		`agentcapabilities.WriteJSONRPCMessage(out, v)`,
		`agentcapabilities.DispatchMCPToolServerRequest(ctx, *req, s.manifestToolServer().Handlers(func()`,
		`agentcapabilities.MCPManifestToolServer{`,
		`ServerName:                   pulseMCPServerName`,
		`SurfaceID:                    agentcapabilities.SurfaceIDPulseMCP`,
		`Manifest:                     manifest`,
		`agentcapabilities.FetchManifest(context.Background(), manifestClient, *baseURL)`,
		`agentcapabilities.StreamMCPEventNotifications(ctx, pulseMCPAgentSurfaceHTTPDoer{}, s.baseURL, path, s.token, func(notification agentcapabilities.JSONRPCRequest) error`,
	}
	for _, fragment := range required {
		if !strings.Contains(src, fragment) {
			t.Errorf("pulse-mcp must project the canonical agent capabilities manifest; missing %s", fragment)
		}
	}
	forbidden := []string{
		`type jsonRPCRequest struct`,
		`type jsonRPCResponse struct`,
		`type jsonRPCError struct`,
		`type jsonRPCResponse = agentcapabilities.JSONRPCResponse`,
		`type jsonRPCError = agentcapabilities.JSONRPCError`,
		`bufio.NewScanner(in)`,
		`scanner.Scan()`,
		`agentcapabilities.DecodeJSONRPCRequest(`,
		`agentcapabilities.NewJSONRPCParseErrorResponse(`,
		`agentcapabilities.JSONRPCRequestExpectsResponse(`,
		`json.Unmarshal([]byte(line)`,
		`agentcapabilities.JSONRPCErrorParse`,
		`len(req.ID) == 0`,
		`string(req.ID) == "null"`,
		`switch req.Method`,
		`agentcapabilities.NewJSONRPCResponse(req.ID, nil)`,
		`agentcapabilities.JSONRPCErrorInternal`,
		`agentcapabilities.JSONRPCErrorMethodNotFound`,
		`agentcapabilities.MCPToolServerHandlers{`,
		`agentcapabilities.MCPMethodToolsList`,
		`agentcapabilities.MCPMethodToolsCall`,
		`agentcapabilities.MCPMethodResourcesList`,
		`agentcapabilities.MCPMethodResourcesRead`,
		`agentcapabilities.MCPMethodPromptsList`,
		`agentcapabilities.MCPMethodPromptsGet`,
		`agentcapabilities.MCPMethodPing`,
		`func (s *mcpServer) handleInitialize(`,
		`func (s *mcpServer) handleToolsList(`,
		`func (s *mcpServer) handleToolsCall(`,
		`func (s *mcpServer) handleResourcesList(`,
		`func (s *mcpServer) handleResourcesRead(`,
		`func (s *mcpServer) handlePromptsList(`,
		`func (s *mcpServer) handlePromptsGet(`,
		`agentcapabilities.NewMCPToolServerInitializeResult(pulseMCPServerName`,
		`agentcapabilities.ProjectTools(s.manifest.Capabilities)`,
		`agentcapabilities.ProjectMCPPrompts(s.manifest.Capabilities)`,
		`agentcapabilities.GetMCPPrompt(s.manifest.Capabilities`,
		`agentcapabilities.BuildMCPPrompt(s.manifest.Capabilities`,
		`agentcapabilities.ExecuteMCPToolHTTP(`,
		`agentcapabilities.ExecuteMCPManifestSurfaceToolHTTP(ctx, s.http, s.baseURL, s.token, s.manifest, agentcapabilities.SurfaceIDPulseMCP, raw)`,
		`"protocolVersion": "2024-11-05"`,
		`"protocolVersion": "2025-06-18"`,
		`"isError": isError`,
		`NewMCPHTTPTextResult(resp.Body, resp.StatusCode)`,
		`agentcapabilities.MCPCapabilities{`,
		`agentcapabilities.MCPToolsCapability{}`,
		`agentcapabilities.MCPPulseNotificationsCapability{`,
		`json.Unmarshal(raw, &params)`,
		`fmt.Errorf("unknown tool: %s", params.Name)`,
		`agentcapabilities.FindCapability(s.manifest.Capabilities, params.Name)`,
		`agentcapabilities.CallCapabilityHTTP(ctx, s.http, s.baseURL, s.token, cap, params.Arguments)`,
		`agentcapabilities.NewMCPCapabilityHTTPResult(resp)`,
		`http.NewRequestWithContext(ctx, cap.Method`,
		`httpReq.Header.Set("X-API-Token"`,
		`baseURL+"/api/agent/capabilities"`,
		`req.Header.Set("Accept", "text/event-stream")`,
		`http.DefaultClient.Do(req)`,
		`scanner := bufio.NewScanner(resp.Body)`,
		`strings.HasPrefix(line, "event: ")`,
		`strings.HasPrefix(line, "data: ")`,
		`agentcapabilities.StreamAgentSSERecords(ctx, nil, s.baseURL, path, s.token`,
		`agentcapabilities.IsTransportEventKind(event)`,
		`agentcapabilities.NewMCPEventNotification(event, []byte(data))`,
		`agentcapabilities.NewJSONRPCNotification(agentcapabilities.MCPNotificationMethod(event), params)`,
	}
	for _, fragment := range forbidden {
		if strings.Contains(src, fragment) {
			t.Errorf("pulse-mcp must not preserve local MCP wire/result envelope logic; found %s", fragment)
		}
	}

	projectionSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "projection.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/projection.go: %v", err)
	}
	projection := string(projectionSource)
	sharedRequired := []string{
		`type ProjectedTool struct`,
		`Title        string`,
		`OutputSchema json.RawMessage`,
		`Annotations  *ToolBehaviorHints`,
		`Meta         map[string]any`,
		`type ToolBehaviorHints struct`,
		`ToolMetaPulseCapabilityKey = "pulse.capability"`,
		`ReadOnlyHint`,
		`DestructiveHint`,
		`IdempotentHint`,
		`OpenWorldHint`,
		`func ProjectTools(capabilities []Capability) []ProjectedTool`,
		`func ProjectManifestSurfaceTools(manifest Manifest, surfaceID string) []ProjectedTool`,
		`func ManifestSurfaceToolCapabilities(manifest Manifest, surfaceID string) []Capability`,
		`func FindManifestSurfaceToolContract(manifest Manifest, surfaceID string) (SurfaceToolContract, bool)`,
		`func ProjectTool(cap Capability) (ProjectedTool, bool)`,
		`func CapabilityTitle(cap Capability) string`,
		`func ToolAnnotations(cap Capability) *ToolBehaviorHints`,
		`func ToolGovernanceBehaviorHints(governance ToolGovernanceDescriptor) *ToolBehaviorHints`,
		`func CloneToolBehaviorHints(hints *ToolBehaviorHints) *ToolBehaviorHints`,
		`func ToolMeta(cap Capability) map[string]any`,
		`NormalizeCapabilityGovernance(cap)`,
		`func FindCapability(capabilities []Capability, name string) (Capability, bool)`,
		`type CapabilityLookupError struct`,
		`func ResolveCapability(capabilities []Capability, name string) (Capability, error)`,
		`func ResolveRequestResponseCapability(capabilities []Capability, name string) (Capability, error)`,
		`return CloneCapability(cap), true`,
		`IsRequestResponseCapability(cap)`,
		`ResolutionNoteArgumentName = "resolution_note"`,
		`NoteArgumentName = "note"`,
		`Title:        CapabilityTitle(cap)`,
		`Description:  ToolDescription(cap)`,
		`InputSchema:  ToolInputSchema(cap)`,
		`OutputSchema: ToolOutputSchema(cap)`,
		`Annotations:  ToolAnnotations(cap)`,
		`Meta:         ToolMeta(cap)`,
		`ToolMetaPulseCapabilityKey: capability`,
		`return CloneRawMessage(cap.InputSchema)`,
		`return CloneRawMessage(cap.OutputSchema)`,
	}
	for _, fragment := range sharedRequired {
		if !strings.Contains(projection, fragment) {
			t.Errorf("agentcapabilities must own the external-agent tool projection; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`Annotations *MCPToolAnnotations`,
		`type MCPToolAnnotations struct`,
		`func ToolAnnotations(cap Capability) *MCPToolAnnotations`,
	} {
		if strings.Contains(projection, fragment) {
			t.Errorf("external-agent tool projection must not make MCP the owner of shared behavior hints; found %s", fragment)
		}
	}

	mcpAliasSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "mcp.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/mcp.go: %v", err)
	}
	mcp := string(mcpAliasSource)
	if !strings.Contains(mcp, `type MCPToolAnnotations = ToolBehaviorHints`) {
		t.Error("MCP adapter edge must keep the protocol-facing tool annotations alias over shared behavior hints")
	}

	schemaSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "schema.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/schema.go: %v", err)
	}
	schema := string(schemaSource)
	for _, fragment := range []string{
		`type ProviderTool struct`,
		`type ProviderToolCall struct`,
		`type ProviderToolResult struct`,
		`func (t Tool) NormalizeCollections() Tool`,
		`func (s InputSchema) NormalizeCollections() InputSchema`,
		`func ClonePropertySchemas(properties map[string]PropertySchema) map[string]PropertySchema`,
		`func CloneProviderInputSchema(schema map[string]interface{}) map[string]interface{}`,
		`func CloneRawMessage(raw json.RawMessage) json.RawMessage`,
		`func EmptyProviderTool() ProviderTool`,
		`func (t ProviderTool) NormalizeCollections() ProviderTool`,
		`t.InputSchema = CloneProviderInputSchema(t.InputSchema)`,
		`t.BehaviorHints = CloneToolBehaviorHints(t.BehaviorHints)`,
		`BehaviorHints   *ToolBehaviorHints`,
		`PulseGovernance *ToolGovernanceDescriptor`,
		`func EmptyProviderToolCall() ProviderToolCall`,
		`func (t ProviderToolCall) NormalizeCollections() ProviderToolCall`,
		`t.Input = CloneToolArguments(t.Input)`,
		`t.ThoughtSignature = CloneRawMessage(t.ThoughtSignature)`,
		`type ProviderToolResultContextOptions struct`,
		`type ProviderToolResultTruncation struct`,
		`type ProviderToolResultContextProjection struct`,
		`func NewProviderToolResult(toolUseID, content string, isError bool) ProviderToolResult`,
		`func NewProviderToolErrorResult(toolUseID, content string) ProviderToolResult`,
		`func NewProviderToolResultFromToolResult(toolUseID string, result ToolResult) ProviderToolResult`,
		`func NewProviderToolResultContextProjection(toolUseID, content string, isError bool, opts ProviderToolResultContextOptions) ProviderToolResultContextProjection`,
		`func NewProviderToolResultContextProjectionFromToolResult(toolUseID string, result ToolResult, opts ProviderToolResultContextOptions) ProviderToolResultContextProjection`,
		`func ProviderToolResultModelContent(content string, opts ProviderToolResultContextOptions) (string, ProviderToolResultTruncation)`,
		`func ProviderInputSchemaFromRaw(raw json.RawMessage) map[string]interface{}`,
		`func ProjectProviderTool(tool Tool) ProviderTool`,
		`func ProjectProviderToolWithGovernance(tool Tool, governance ToolGovernanceDescriptor) ProviderTool`,
		`func ProjectProviderTools(tools []Tool) []ProviderTool`,
		`func ProjectProviderToolsWithGovernance(tools []Tool, governance []ToolGovernanceDescriptor) []ProviderTool`,
		`type AssistantProviderToolOptions struct`,
		`func ProjectAssistantProviderTools(tools []Tool, governance []ToolGovernanceDescriptor, opts AssistantProviderToolOptions) []ProviderTool`,
		`func ProjectPulseAssistantProviderTools(manifest Manifest, tools []Tool, governance []ToolGovernanceDescriptor, opts AssistantProviderToolOptions) []ProviderTool`,
		`func ProviderToolGovernanceDescriptors(tools []ProviderTool) ([]ToolGovernanceDescriptor, bool)`,
		`func AssistantNativeProviderTools() []ProviderTool`,
		`func AssistantNativeProviderToolNames() []string`,
		`func LegacyAssistantUtilityProviderTools() []ProviderTool`,
		`LegacyAssistantRunCommandToolName = "run_command"`,
		`LegacyAssistantFetchURLToolName = "fetch_url"`,
		`LegacyAssistantSetResourceURLToolName = "set_resource_url"`,
		`func ProviderToolNames(tools []ProviderTool) []string`,
		`type ProviderToolNameCatalog struct`,
		`func NewProviderToolNameCatalog(nameGroups ...[]string) ProviderToolNameCatalog`,
		`func NewAssistantProviderToolNameCatalog(registryToolNames []string) ProviderToolNameCatalog`,
		`func (c ProviderToolNameCatalog) Names() []string`,
		`func (c ProviderToolNameCatalog) Has(name string) bool`,
		`func (c ProviderToolNameCatalog) HasPrefix(prefix string) bool`,
		`func ProviderToolDescriptionWithGovernance(description string, governance ToolGovernanceDescriptor) string`,
		`func ProjectProviderToolCallToToolCall(tc ProviderToolCall) ToolCallParams`,
		`func NormalizeProviderToolCallForExecution(tc ProviderToolCall) ProviderToolCall`,
		`func NormalizeProviderToolCallsForExecution(calls []ProviderToolCall) []ProviderToolCall`,
		`func NewPulseQuestionProviderTool() ProviderTool`,
		`func PulseQuestionProviderInputSchema() map[string]interface{}`,
		`func ParseProviderToolInput(raw string) (map[string]interface{}, bool)`,
		`func ProviderToolInputOrRaw(raw string) map[string]interface{}`,
		`schema = schema.NormalizeCollections()`,
		`projected := CloneProviderInputSchema(schema)`,
		`property = property.NormalizeCollections()`,
		`tool = tool.NormalizeCollections()`,
		`InputSchema: ProviderInputSchema(tool.InputSchema)`,
		`ToolGovernancePromptDescription(governance)`,
		`Pulse governance: `,
		`projected.BehaviorHints = ToolGovernanceBehaviorHints(governance)`,
		`projected.PulseGovernance = &governance`,
		`return NewProviderToolResult(toolUseID, interpreted.Text, interpreted.IsError)`,
		`ProviderToolResultModelContent(content, opts)`,
		`return NormalizeToolCallParams(ToolCallParams{`,
		`Arguments: tc.Input,`,
		`params := ProjectProviderToolCallToToolCall(tc)`,
		`normalized.Name = params.Name`,
		`normalized.Input = params.Arguments`,
		`projected = append(projected, AssistantNativeProviderTools()...)`,
		`ManifestSurfaceAffordances(manifest, SurfaceIDPulseAssistant)`,
		`if !affordances.Tools`,
		`if !affordances.InteractiveQuestions`,
		`return ProviderToolNames(AssistantNativeProviderTools())`,
		`Name:        PulseQuestionToolName`,
		`InputSchema: PulseQuestionProviderInputSchema(),`,
		`Name:        LegacyAssistantRunCommandToolName`,
		`Name:        LegacyAssistantFetchURLToolName`,
		`Name:        LegacyAssistantSetResourceURLToolName`,
		`type PulseQuestionToolType string`,
		`PulseQuestionToolTypeText   PulseQuestionToolType = "text"`,
		`PulseQuestionToolTypeSelect PulseQuestionToolType = "select"`,
		`func PulseQuestionToolTypeValues() []string`,
		`func NormalizePulseQuestionToolType(rawType string, hasOptions bool) (PulseQuestionToolType, error)`,
		`type PulseQuestionToolQuestion struct`,
		`type PulseQuestionToolOption struct`,
		`func ParsePulseQuestionToolInput(input map[string]interface{}) ([]PulseQuestionToolQuestion, error)`,
		`func parsePulseQuestionToolOptions(rawOptions json.RawMessage) ([]PulseQuestionToolOption, error)`,
		`"enum":        PulseQuestionToolTypeValues(),`,
		`return map[string]interface{}{"raw": raw}`,
	} {
		if !strings.Contains(schema, fragment) {
			t.Errorf("agentcapabilities must own the Assistant provider tool projection; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`func ProjectProviderToolCallToMCP(tc ProviderToolCall) MCPCallToolParams`,
	} {
		if strings.Contains(schema, fragment) {
			t.Errorf("neutral provider schema must not own MCP compatibility aliases; found %s", fragment)
		}
	}

	providerArtifactSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "provider_tool_artifacts.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/provider_tool_artifacts.go: %v", err)
	}
	providerArtifacts := string(providerArtifactSource)
	for _, fragment := range []string{
		`func ProviderToolCallArtifactIndex(content string, catalog ProviderToolNameCatalog) int`,
		`func SplitTrailingProviderToolNamePrefix(content string, catalog ProviderToolNameCatalog) (visible string, held string)`,
		`providerJSONToolCallLeakIndex(content, catalog)`,
		`providerPlainFunctionToolCallLeakIndex(content, catalog)`,
		`catalog.Has(name)`,
		`catalog.HasPrefix(token)`,
	} {
		if !strings.Contains(providerArtifacts, fragment) {
			t.Errorf("agentcapabilities must own provider tool-call artifact detection; missing %s", fragment)
		}
	}

	mcpSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "mcp.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/mcp.go: %v", err)
	}
	mcpWire := string(mcpSource)
	toolCallSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "tool_call.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/tool_call.go: %v", err)
	}
	toolNameSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "tool_names.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/tool_names.go: %v", err)
	}
	toolResultSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "tool_result.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/tool_result.go: %v", err)
	}
	toolExecutionSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "tool_execution.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/tool_execution.go: %v", err)
	}
	toolCore := string(toolNameSource) + "\n" + string(toolCallSource) + "\n" + string(toolResultSource) + "\n" + string(toolExecutionSource) + "\n" + projection
	mcpWireContract := mcpWire + "\n" + toolCore
	toolCallSrc := string(toolCallSource)
	toolNameSrc := string(toolNameSource)
	for _, pair := range []struct {
		name  string
		value string
	}{
		{`PulseQueryToolName`, `"pulse_query"`},
		{`PulseControlToolName`, `"pulse_control"`},
		{`PulseReadToolName`, `"pulse_read"`},
		{`PulseSummarizeToolName`, `"pulse_summarize"`},
		{`PulseRunCommandToolName`, `"pulse_run_command"`},
		{`PatrolGetFindingsToolName`, `"patrol_get_findings"`},
		{`PatrolReportFindingToolName`, `"patrol_report_finding"`},
		{`PatrolResolveFindingToolName`, `"patrol_resolve_finding"`},
	} {
		if !strings.Contains(toolNameSrc, pair.name) || !strings.Contains(toolNameSrc, pair.value) {
			t.Errorf("agentcapabilities must own stable Pulse Intelligence tool identity %s=%s", pair.name, pair.value)
		}
	}
	for _, fragment := range []string{
		`type ToolCallKind int`,
		`ToolCallKindResolve`,
		`ToolCallKindRead`,
		`ToolCallKindWrite`,
		`ToolCallKindUserInput`,
		`func (k ToolCallKind) String() string`,
		`func ClassifyToolCall(toolName string, args map[string]interface{}) ToolCallKind`,
		`case PulseQuestionToolName:`,
		`case PulseQueryToolName, PulseDiscoveryToolName:`,
		`case PulseControlToolName:`,
		`case PulseReadToolName, LegacyAssistantFetchURLToolName:`,
		`LegacyAssistantRunCommandToolName, LegacyAssistantSetResourceURLToolName`,
		`return ToolCallKindWrite`,
	} {
		if !strings.Contains(toolCallSrc, fragment) {
			t.Errorf("agentcapabilities must own shared tool-call safety classification; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`type JSONRPCRequest struct`,
		`type JSONRPCResponse struct`,
		`type JSONRPCError struct`,
		`type MCPInitializeResult struct`,
		`type ToolCallParams struct`,
		`type MCPCallToolParams = ToolCallParams`,
		`func NormalizeToolCallParams(params ToolCallParams) ToolCallParams`,
		`func ValidateToolCallParams(params ToolCallParams) error`,
		`type MCPProjectedToolsResult struct`,
		`type MCPResource struct`,
		`type MCPListResourcesResult struct`,
		`type MCPReadResourceParams struct`,
		`type MCPReadResourceResult struct`,
		`type MCPResourceContent struct`,
		`type MCPPrompt struct`,
		`Title       string`,
		`type MCPPromptArgument struct`,
		`type MCPListPromptsResult struct`,
		`type MCPGetPromptParams struct`,
		`type MCPGetPromptResult struct`,
		`type MCPPromptMessage struct`,
		`type MCPCapabilities struct`,
		`Instructions    string`,
		`Resources    *MCPResourcesCapability`,
		`Prompts      *MCPPromptsCapability`,
		`type MCPResourcesCapability struct`,
		`type MCPPromptsCapability struct`,
		`type ProjectedTool struct`,
		`OutputSchema json.RawMessage`,
		`Meta         map[string]any`,
		`type ToolResult struct`,
		`StructuredContent map[string]any`,
		`type MCPToolResult = ToolResult`,
		`type ToolContent struct`,
		`type MCPContent = ToolContent`,
		`func (r MCPListResourcesResult) NormalizeCollections() MCPListResourcesResult`,
		`func (r MCPReadResourceResult) NormalizeCollections() MCPReadResourceResult`,
		`func (r MCPListPromptsResult) NormalizeCollections() MCPListPromptsResult`,
		`func (r MCPGetPromptResult) NormalizeCollections() MCPGetPromptResult`,
		`func EmptyToolResult() ToolResult`,
		`func (r ToolResult) NormalizeCollections() ToolResult`,
		`MCPPulseNotificationsExperimentalKey = "pulseNotifications"`,
		`func NewMCPToolServerInitializeResult(`,
		`func NewMCPManifestToolServerInitializeResult(`,
		`func NewMCPManifestSurfaceToolServerInitializeResult(`,
		`ManifestSurfaceAffordances(manifest, surfaceID)`,
		`ResolveManifestSurfaceToolContract(manifest, surfaceID)`,
		`affordances.Tools && hasSurfaceToolContract`,
		`MCPManifestSurfacePromptProjectionSupported(manifest, surfaceID)`,
		`Instructions: BuildPulseMCPOperatingInstructions(PulseMCPOperatingInstructionOptions{`,
		`SupportsTools:              exposeTools`,
		`func MCPManifestResourceProjectionSupported(manifest Manifest, surfaceID string) bool`,
		`func ManifestSurfaceResourceCapabilities(manifest Manifest, surfaceID string) []Capability`,
		`func MCPManifestPromptProjectionSupported(manifest Manifest) bool`,
		`func MCPManifestSurfacePromptProjectionSupported(manifest Manifest, surfaceID string) bool`,
		`func ProjectMCPWorkflowPrompts(workflowPrompts []PulseWorkflowPrompt) []MCPPrompt`,
		`func DecodeMCPGetPromptParams(raw json.RawMessage) (MCPGetPromptParams, error)`,
		`func GetMCPPromptFromManifest(manifest Manifest, raw json.RawMessage) (MCPGetPromptResult, error)`,
		`func GetMCPPromptFromManifestSurface(manifest Manifest, surfaceID string, raw json.RawMessage) (MCPGetPromptResult, error)`,
		`func BuildMCPPromptFromManifest(manifest Manifest, params MCPGetPromptParams) (MCPGetPromptResult, error)`,
		`ManifestPulseWorkflowPrompts(manifest)`,
		`func MCPResourceURI(resourceID string) string`,
		`func ResourceIDFromMCPResourceURI(rawURI string) (string, error)`,
		`func DecodeMCPReadResourceParams(raw json.RawMessage) (MCPReadResourceParams, error)`,
		`func ListMCPManifestSurfaceResourcesHTTP(`,
		`func ReadMCPManifestSurfaceResourceHTTP(`,
		`func resolveMCPManifestSurfaceResourceCapabilities(`,
		`CallRequestResponseCapabilityHTTPBodyByName(ctx, client, baseURL, token, capabilities, FleetContextCapabilityName, map[string]any{})`,
		`CallRequestResponseCapabilityHTTPBodyByName(ctx, client, baseURL, token, capabilities, ResourceContextCapabilityName, map[string]any{`,
		`ResourceIDArgumentName: resourceID`,
		`caps.Tools = &MCPToolsCapability{}`,
		`MCPPulseNotificationsExperimentalKey: MCPPulseNotificationsCapability`,
		`type MCPToolServerHandlers struct`,
		`func DecodeJSONRPCRequest(`,
		`fmt.Errorf("malformed JSON-RPC request: %w", err)`,
		`func NewJSONRPCParseErrorResponse(`,
		`func JSONRPCRequestExpectsResponse(`,
		`type JSONRPCLineDispatcher func(context.Context, JSONRPCRequest) JSONRPCResponse`,
		`type JSONRPCResponseWriter func(JSONRPCResponse) error`,
		`func ServeJSONRPCLines(`,
		`scanner.Buffer(make([]byte, 64*1024), 1<<22)`,
		`DecodeJSONRPCRequest([]byte(line))`,
		`NewJSONRPCParseErrorResponse(err)`,
		`JSONRPCRequestExpectsResponse(req)`,
		`func WriteJSONRPCMessage(`,
		`enc.SetEscapeHTML(false)`,
		`type MCPManifestToolServer struct`,
		`SurfaceID                    string`,
		`func (s MCPManifestToolServer) Handlers(`,
		`func (s MCPManifestToolServer) Initialize() MCPInitializeResult`,
		`func (s MCPManifestToolServer) ToolsList() MCPProjectedToolsResult`,
		`func (s MCPManifestToolServer) ToolsCall(ctx context.Context, raw json.RawMessage) (MCPToolResult, error)`,
		`func (s MCPManifestToolServer) ResourcesList(ctx context.Context) (MCPListResourcesResult, error)`,
		`func (s MCPManifestToolServer) ResourcesRead(ctx context.Context, raw json.RawMessage) (MCPReadResourceResult, error)`,
		`func (s MCPManifestToolServer) PromptsList() MCPListPromptsResult`,
		`func (s MCPManifestToolServer) PromptsGet(ctx context.Context, raw json.RawMessage) (MCPGetPromptResult, error)`,
		`return NewMCPManifestSurfaceToolServerInitializeResult(s.ServerName, s.ServerVersion, s.EmitPulseNotifications, s.Manifest, s.surfaceID())`,
		`if !s.surfaceAffordances().Tools`,
		`return MCPProjectedToolsResult{Tools: ProjectManifestSurfaceTools(s.Manifest, s.surfaceID())}`,
		`return ExecuteMCPManifestSurfaceToolHTTP(ctx, s.Client, s.BaseURL, s.Token, s.Manifest, s.surfaceID(), raw)`,
		`return ListMCPManifestSurfaceResourcesHTTP(ctx, s.Client, s.BaseURL, s.Token, s.Manifest, s.surfaceID())`,
		`return ReadMCPManifestSurfaceResourceHTTP(ctx, s.Client, s.BaseURL, s.Token, s.Manifest, s.surfaceID(), raw)`,
		`if !affordances.Resources`,
		`fmt.Errorf("MCP resources are not enabled for surface %s", surfaceID)`,
		`return (MCPListPromptsResult{Prompts: ProjectMCPWorkflowPrompts(ManifestPulseWorkflowPrompts(s.Manifest))}).NormalizeCollections()`,
		`result, err := GetMCPPromptFromManifestSurface(s.Manifest, s.surfaceID(), raw)`,
		`func (s MCPManifestToolServer) surfaceAffordances() SurfaceAffordanceContract`,
		`func (s MCPManifestToolServer) surfaceID() string`,
		`func normalizeMCPManifestSurfaceID(surfaceID string) string`,
		`func DispatchMCPToolServerRequest(`,
		`case MCPMethodInitialize:`,
		`case MCPMethodToolsList:`,
		`case MCPMethodToolsCall:`,
		`case MCPMethodResourcesList:`,
		`case MCPMethodResourcesRead:`,
		`case MCPMethodPromptsList:`,
		`case MCPMethodPromptsGet:`,
		`return NewJSONRPCResponse(req.ID, result.NormalizeCollections())`,
		`NewJSONRPCErrorResponse(req.ID, JSONRPCErrorInternal, err.Error(), nil)`,
		`NewJSONRPCErrorResponse(req.ID, JSONRPCErrorMethodNotFound, fmt.Sprintf("method not found: %s", req.Method), nil)`,
		`func DecodeMCPCallToolParams(`,
		`params = NormalizeToolCallParams(params)`,
		`ValidateToolCallParams(params)`,
		`func PrepareToolRegistryExecution(name string, args map[string]any) (ToolCallParams, ToolResult, bool)`,
		`func NewInvalidToolCallParamsResult(err error) ToolResult`,
		`func NewUnknownToolResult(name string) ToolResult`,
		`func NewControlToolsDisabledToolResult() ToolResult`,
		`func ExecuteMCPManifestSurfaceToolHTTP(`,
		`DecodeMCPCallToolParams(raw)`,
		`ManifestSurfaceAffordances(manifest, surfaceID)`,
		`ManifestSurfaceToolCapabilities(manifest, surfaceID)`,
		`return ExecuteCapabilityToolHTTP(ctx, client, baseURL, token, ManifestSurfaceToolCapabilities(manifest, surfaceID), params)`,
		`OutputSchema: ToolOutputSchema(cap)`,
		`func ToolOutputSchema(cap Capability) json.RawMessage`,
		`func ExecuteCapabilityToolHTTP(`,
		`params = NormalizeToolCallParams(params)`,
		`ValidateToolCallParams(params)`,
		`CallRequestResponseCapabilityHTTPByName(ctx, client, baseURL, token, capabilities, params.Name, params.Arguments)`,
		`var lookupErr CapabilityLookupError`,
		`errors.As(err, &lookupErr)`,
		`return NewCapabilityHTTPToolResult(resp), nil`,
		`func NewToolTextContent(text string) ToolContent`,
		`func NewToolTextResult(text string) ToolResult`,
		`func NewToolErrorResult(err error) ToolResult`,
		`func NewToolJSONResult(data any) ToolResult`,
		`func NewToolJSONResultWithIsError(data any, isError bool) ToolResult`,
		`func NewToolHTTPTextResult(`,
		`func NewCapabilityHTTPToolResult(`,
		`func ToolStructuredContentFromJSON(`,
		`func CloneToolStructuredContent(`,
		`func ToolResultText(result ToolResult) string`,
		`type ToolResultInterpretation struct`,
		`func InterpretToolResult(result ToolResult) ToolResultInterpretation`,
		`ApprovalRequired: HasApprovalRequiredToolMarker(text)`,
		`PolicyBlocked:    HasPolicyBlockedToolMarker(text)`,
		`type DirectToolExecutionOptions struct`,
		`type DirectToolExecutionOutcome struct`,
		`func InterpretDirectToolExecution(result ToolResult, opts DirectToolExecutionOptions) (DirectToolExecutionOutcome, error)`,
		`resp.OK()`,
		`func NewJSONRPCNotification(`,
		`func NewMCPEventNotification(`,
		`const (`,
		`MCPProtocolVersion = "2025-06-18"`,
		`MCPMethodResourcesList = "resources/list"`,
		`MCPMethodResourcesRead = "resources/read"`,
		`MCPMethodPromptsList   = "prompts/list"`,
		`MCPMethodPromptsGet    = "prompts/get"`,
		`MCPResourceContextURIHost  = "resource"`,
		`MCPResourceContextMIMEType = "application/json"`,
	} {
		if !strings.Contains(mcpWireContract, fragment) {
			t.Errorf("agentcapabilities must own the shared MCP wire/result envelope; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`func MCPResourceProjectionSupported(capabilities []Capability) bool`,
		`func ListMCPResourcesHTTP(ctx context.Context, client HTTPDoer, baseURL, token string, capabilities []Capability)`,
		`func ReadMCPResourceHTTP(ctx context.Context, client HTTPDoer, baseURL, token string, capabilities []Capability`,
		`return ListMCPResourcesHTTP(ctx, s.Client, s.BaseURL, s.Token, s.Manifest.Capabilities)`,
		`return ReadMCPResourceHTTP(ctx, s.Client, s.BaseURL, s.Token, s.Manifest.Capabilities, raw)`,
	} {
		if strings.Contains(mcpWireContract, fragment) {
			t.Errorf("shared MCP resource projection must be manifest-surface-owned, not raw-capability-owned; found %s", fragment)
		}
	}
	for _, fragment := range []string{
		`func MCPPromptProjectionSupported(capabilities []Capability) bool`,
		`func ProjectMCPPrompts(capabilities []Capability) []MCPPrompt`,
		`func GetMCPPrompt(capabilities []Capability, raw json.RawMessage)`,
		`func BuildMCPPrompt(capabilities []Capability, params MCPGetPromptParams)`,
		`return GetMCPPromptFromManifest(s.Manifest, raw)`,
	} {
		if strings.Contains(mcpWireContract, fragment) {
			t.Errorf("shared MCP prompt projection must be manifest-owned, not raw-capability-owned; found %s", fragment)
		}
	}
	for _, fragment := range []string{
		`type ToolCallParams struct`,
		`func NormalizeToolCallParams(params ToolCallParams) ToolCallParams`,
		`func ValidateToolCallParams(params ToolCallParams) error`,
		`func PrepareToolRegistryExecution(name string, args map[string]any) (ToolCallParams, ToolResult, bool)`,
		`func NewInvalidToolCallParamsResult(err error) ToolResult`,
		`func NewUnknownToolResult(name string) ToolResult`,
		`func NewControlToolsDisabledToolResult() ToolResult`,
		`type ProjectedTool struct`,
		`OutputSchema json.RawMessage`,
		`Meta         map[string]any`,
		`type ToolContent struct`,
		`type ToolResult struct`,
		`StructuredContent map[string]any`,
		`func EmptyToolResult() ToolResult`,
		`func (r ToolResult) NormalizeCollections() ToolResult`,
		`func NewToolTextContent(text string) ToolContent`,
		`func NewToolTextResult(text string) ToolResult`,
		`func NewToolErrorResult(err error) ToolResult`,
		`func NewToolJSONResult(data any) ToolResult`,
		`func NewToolHTTPTextResult(`,
		`func NewCapabilityHTTPToolResult(`,
		`func ToolOutputSchema(cap Capability) json.RawMessage`,
		`func ToolStructuredContentFromJSON(`,
		`func CloneToolStructuredContent(`,
		`func ToolResultText(result ToolResult) string`,
		`type ToolResultInterpretation struct`,
		`func InterpretToolResult(result ToolResult) ToolResultInterpretation`,
		`func ExecuteCapabilityToolHTTP(`,
		`type DirectToolExecutionOptions struct`,
		`type DirectToolExecutionOutcome struct`,
		`func InterpretDirectToolExecution(result ToolResult, opts DirectToolExecutionOptions) (DirectToolExecutionOutcome, error)`,
	} {
		if strings.Contains(mcpWire, fragment) {
			t.Errorf("neutral Pulse Intelligence tool primitive must not be implemented in mcp.go; found %s", fragment)
		}
		if !strings.Contains(toolCore, fragment) {
			t.Errorf("neutral Pulse Intelligence tool primitive must live outside mcp.go; missing %s", fragment)
		}
	}

	workflowPromptSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "workflow_prompt.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/workflow_prompt.go: %v", err)
	}
	workflowPromptCore := string(workflowPromptSource)
	for _, fragment := range []string{
		`PulseWorkflowPromptTriageFleet         = "pulse_triage_fleet"`,
		`PulseWorkflowPromptOperationsLoop`,
		`MCPPromptTriageFleet         = PulseWorkflowPromptTriageFleet`,
		`MCPPromptOperationsLoop`,
		`type PulseWorkflowPrompt struct`,
		`type PulseWorkflowPromptArgument struct`,
		`type PulseWorkflowPromptParams struct`,
		`type PulseWorkflowPromptResult struct`,
		`type PulseWorkflowPromptRenderOptions struct`,
		`func ManifestPulseWorkflowPrompts(manifest Manifest) []PulseWorkflowPrompt`,
		`func ProjectPulseWorkflowPrompts(capabilities []Capability) []PulseWorkflowPrompt`,
		`func BuildPulseWorkflowPrompt(capabilities []Capability, params PulseWorkflowPromptParams) (PulseWorkflowPromptResult, error)`,
		`func BuildPulseWorkflowPromptWithOptions(capabilities []Capability, params PulseWorkflowPromptParams, opts PulseWorkflowPromptRenderOptions) (PulseWorkflowPromptResult, error)`,
		`func BuildPulseWorkflowPromptFromManifest(manifest Manifest, params PulseWorkflowPromptParams) (PulseWorkflowPromptResult, error)`,
		`func BuildPulseWorkflowPromptFromManifestWithOptions(manifest Manifest, params PulseWorkflowPromptParams, opts PulseWorkflowPromptRenderOptions) (PulseWorkflowPromptResult, error)`,
		`func RequiredPulseWorkflowPromptArgument(args map[string]string, name string) (string, error)`,
		`Name:        ResourceIDArgumentName`,
		`RequiredPulseWorkflowPromptArgument(params.Arguments, ResourceIDArgumentName)`,
		`ResourceIDArgumentName, resourceID`,
		`Name:        FindingIDArgumentName`,
		`FindCapability(capabilities, ListFindingsCapabilityName)`,
		`RequiredPulseWorkflowPromptArgument(params.Arguments, FindingIDArgumentName)`,
		`ListFindingsCapabilityName`,
		`PlanActionCapabilityName`,
		`DecideActionCapabilityName`,
		`ExecuteActionCapabilityName`,
		`ResolveFindingCapabilityName`,
		`Triage the Pulse fleet using the canonical fleet context capability`,
		`Review one Patrol finding and propose the safest governed next step`,
		`Patrol issue handling`,
		`Do not execute write tools unless the operator explicitly asks for a governed action`,
	} {
		if !strings.Contains(workflowPromptCore, fragment) {
			t.Errorf("neutral Pulse Intelligence workflow prompt contract must live outside mcp.go; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`func PulseWorkflowPromptProjectionSupported(capabilities []Capability) bool`,
	} {
		if strings.Contains(workflowPromptCore, fragment) {
			t.Errorf("shared workflow prompt projection must be manifest-owned, not raw-capability-owned; found %s", fragment)
		}
	}
	for _, fragment := range []string{
		`Pulse fleet triage`,
		`Triage the Pulse fleet using the canonical fleet context capability`,
		`Review one Patrol finding and propose the safest governed next step`,
		`prompt argument %s is required`,
		`func RequiredPulseWorkflowPromptArgument`,
	} {
		if strings.Contains(mcpWire, fragment) {
			t.Errorf("MCP edge must not own neutral Pulse Intelligence workflow prompt behavior; found %s", fragment)
		}
	}
	for _, fragment := range []string{
		`func NormalizeMCPCallToolParams(`,
		`func ValidateMCPCallToolParams(`,
		`func ProjectProviderToolCallToMCP(`,
		`func PrepareMCPToolRegistryExecution(`,
		`func NewInvalidMCPToolCallParamsResult(`,
		`func NewUnknownMCPToolResult(`,
		`func NewControlToolsDisabledMCPToolResult(`,
		`func EmptyMCPToolResult(`,
		`func NewMCPTextContent(`,
		`func NewMCPTextResult(`,
		`func NewMCPErrorResult(`,
		`func NewProviderToolResultFromMCP(`,
		`func NewMCPToolResponseResult(`,
		`func NewMCPJSONResult(`,
		`func NewMCPJSONResultWithIsError(`,
		`func NewMCPHTTPTextResult(`,
		`func NewMCPCapabilityHTTPResult(`,
		`func MCPToolResultText(`,
		`type MCPToolResultInterpretation`,
		`func InterpretMCPToolResult(`,
		`type DirectMCPToolExecutionOptions`,
		`type DirectMCPToolExecutionOutcome`,
		`func InterpretDirectMCPToolExecution(`,
	} {
		if strings.Contains(mcpWire, fragment) {
			t.Errorf("MCP edge must not preserve compatibility wrappers around neutral tool helpers; found %s", fragment)
		}
	}

	sseSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "sse.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/sse.go: %v", err)
	}
	sse := string(sseSource)
	for _, fragment := range []string{
		`type SSERecord struct`,
		`const AgentSSEAccept = "text/event-stream"`,
		`func ScanSSERecords(`,
		`scanner.Buffer(make([]byte, 64*1024), 1<<22)`,
		`case "event":`,
		`case "data":`,
		`strings.Join(dataLines, "\n")`,
		`func StreamAgentSSERecords(`,
		`NewAgentHTTPRequest(ctx, http.MethodGet, baseURL, path, token, nil)`,
		`req.Header.Set("Accept", AgentSSEAccept)`,
		`httpClient(client).Do(req)`,
		`ScanSSERecords(resp.Body, handle)`,
		`func IsActionableSSERecord(`,
		`func StreamAgentActionableSSERecords(`,
		`type MCPEventNotificationWriter func(JSONRPCRequest) error`,
		`func StreamMCPEventNotifications(`,
		`StreamAgentActionableSSERecords(ctx, client, baseURL, path, token, func(record SSERecord) bool`,
		`NewMCPEventNotification(record.Event, []byte(record.Data))`,
	} {
		if !strings.Contains(sse, fragment) {
			t.Errorf("agentcapabilities must own the shared agent SSE parser, subscription transport, and MCP notification bridge; missing %s", fragment)
		}
	}

	textInvocationSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "text_tool_invocation.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/text_tool_invocation.go: %v", err)
	}
	textInvocation := string(textInvocationSource)
	for _, fragment := range []string{
		`func IsTextToolInvocation(command string) bool`,
		`func ParseTextToolInvocation(command string) (ToolCallParams, error)`,
		`const ApprovalArgumentKey = "_approval_id"`,
		`func ApprovalArgument(args map[string]any) string`,
		`func IsInternalToolArgument(name string) bool`,
		`func CloneToolArguments(args map[string]any) map[string]any`,
		`func PublicToolArguments(args map[string]any) map[string]any`,
		`func WithApprovalArgument(args map[string]any, approvalID string) map[string]any`,
		`cloned[k] = cloneSchemaValue(v)`,
		`public[k] = cloneSchemaValue(v)`,
		`strings.TrimPrefix(command, "default_api:")`,
		`args := make(map[string]any)`,
		`ToolCallParams{Name: toolName, Arguments: args}`,
		`ValidateToolCallParams(params)`,
		`func splitTextToolArguments(argsStr string) []string`,
	} {
		if !strings.Contains(textInvocation, fragment) {
			t.Errorf("agentcapabilities must own the shared text tool invocation parser; missing %s", fragment)
		}
	}

	aiRelaySource, err := os.ReadFile("router_routes_ai_relay.go")
	if err != nil {
		t.Fatalf("read internal/api/router_routes_ai_relay.go: %v", err)
	}
	aiRelay := string(aiRelaySource)
	for _, fragment := range []string{
		`agentcapabilities.ParseTextToolInvocation(command)`,
		`params.Arguments = agentcapabilities.WithApprovalArgument(params.Arguments, approvalID)`,
		`chatService.ExecuteAssistantTool(ctx, params.Name, params.Arguments)`,
	} {
		if !strings.Contains(aiRelay, fragment) {
			t.Errorf("AI relay MCP executor must consume the shared text tool invocation parser; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`func parseMCPToolCall(`,
		`func splitToolArgs(`,
		`func isMCPToolCall(`,
		`params.Arguments["_approval_id"]`,
	} {
		if strings.Contains(aiRelay, fragment) {
			t.Errorf("AI relay must not preserve local text tool invocation parsing; found %s", fragment)
		}
	}

	assistantToolFiles := []string{
		filepath.Join("..", "ai", "chat", "agentic.go"),
		filepath.Join("..", "ai", "tools", "tools_control.go"),
		filepath.Join("..", "ai", "tools", "tools_docker.go"),
		filepath.Join("..", "ai", "tools", "tools_file.go"),
		filepath.Join("..", "ai", "tools", "tools_kubernetes.go"),
	}
	for _, path := range assistantToolFiles {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(source), `["_approval_id"]`) {
			t.Errorf("%s must consume shared approved-action argument helpers instead of local key indexing", path)
		}
	}

	httpSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "http.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/http.go: %v", err)
	}
	agentHTTP := string(httpSource)
	for _, fragment := range []string{
		`const (`,
		`AgentCapabilitiesPath           = "/api/agent/capabilities"`,
		`AgentAPITokenHeader             = "X-API-Token"`,
		`type HTTPCallResponse struct`,
		`func NewAgentHTTPRequest(`,
		`func FetchManifest(`,
		`func BuildCapabilityHTTPRequest(`,
		`func CallCapabilityHTTP(`,
		`func CallCapabilityHTTPByName(`,
		`func CallRequestResponseCapabilityHTTPByName(`,
		`func (r HTTPCallResponse) FailureError() error`,
		`ProjectCapabilityCall(cap, args)`,
		`ResolveRequestResponseCapability(capabilities, name)`,
		`ResolveCapability(capabilities, name)`,
		`DecodeErrorEnvelope(r.Body)`,
	} {
		if !strings.Contains(agentHTTP, fragment) {
			t.Errorf("agentcapabilities must own the shared agent HTTP substrate; missing %s", fragment)
		}
	}
	if !strings.Contains(projection, `for k, v := range PublicToolArguments(args)`) {
		t.Error("manifest-backed HTTP projection must strip internal tool-call arguments before building public request bodies")
	}

	toolResponseSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "tool_response.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/tool_response.go: %v", err)
	}
	toolResponse := string(toolResponseSource)
	for _, fragment := range []string{
		`type ToolResponse struct`,
		`type ToolError struct`,
		`ErrCodeFSMBlocked`,
		`ErrCodeStrictResolution`,
		`ErrCodeRoutingMismatch`,
		`ErrCodeExecutionContextUnavailable`,
		`func NewToolBlockedError(`,
		`func NewToolResponseResult(resp ToolResponse) ToolResult`,
		`func ToolResultErrorCode(resultText string) (string, bool)`,
		`func ToolResultHasErrorCode(resultText, code string) bool`,
		`func ToolResultHasVerificationOK(resultText string) bool`,
	} {
		if !strings.Contains(toolResponse, fragment) {
			t.Errorf("agentcapabilities must own the shared tool response envelope; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`func NewMCPToolResponseResult(`,
	} {
		if strings.Contains(toolResponse, fragment) {
			t.Errorf("neutral tool response envelope must not own MCP compatibility wrappers; found %s", fragment)
		}
	}

	agenticVerificationSource, err := os.ReadFile(filepath.Join("..", "ai", "chat", "agentic_verification.go"))
	if err != nil {
		t.Fatalf("read internal/ai/chat/agentic_verification.go: %v", err)
	}
	if strings.Contains(string(agenticVerificationSource), `func toolResultHasVerificationOK(`) {
		t.Error("chat FSM verification evidence parsing must live in shared agentcapabilities, not a chat-local helper")
	}
	agenticSource, err := os.ReadFile(filepath.Join("..", "ai", "chat", "agentic.go"))
	if err != nil {
		t.Fatalf("read internal/ai/chat/agentic.go: %v", err)
	}
	if !strings.Contains(string(agenticSource), `agentcapabilities.ToolResultHasVerificationOK(resultText)`) {
		t.Error("chat FSM self-verification must consume the shared tool-result verification parser")
	}
	if !strings.Contains(string(agenticSource), `agentcapabilities.ToolResultHasErrorCode(resultText, agentcapabilities.ErrCodeStrictResolution)`) {
		t.Error("chat strict-resolution recovery must consume the shared tool-result error-code parser")
	}
	if strings.Contains(string(agenticSource), `strings.Contains(resultText, "STRICT_RESOLUTION")`) {
		t.Error("chat strict-resolution recovery must not use local string matching")
	}
	if !strings.Contains(string(agenticSource), `agentcapabilities.ErrCodeFSMBlocked`) {
		t.Error("chat FSM recovery tracking must consume the shared FSM-blocked error code")
	}
	if strings.Contains(string(agenticSource), `"FSM_BLOCKED"`) {
		t.Error("chat FSM recovery tracking must not hard-code the FSM-blocked error code")
	}

	toolMarkerSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "tool_marker.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/tool_marker.go: %v", err)
	}
	toolMarker := string(toolMarkerSource)
	for _, fragment := range []string{
		`ToolMarkerApprovalRequiredPrefix = ErrCodeApprovalRequired + ":"`,
		`ToolMarkerPolicyBlockedPrefix = ErrCodePolicyBlocked + ":"`,
		`ToolMarkerApprovalRequiredType = "approval_required"`,
		`ToolMarkerPolicyBlockedType = "policy_blocked"`,
		`type ApprovalRequiredToolMarkerData struct`,
		`func (d ApprovalRequiredToolMarkerData) TargetHint() string`,
		`func (d ApprovalRequiredToolMarkerData) DescriptionText() string`,
		`func ApprovalRequiredToolMarker(`,
		`func PolicyBlockedToolMarker(`,
		`func FormatApprovalRequiredToolMarker(`,
		`func FormatPolicyBlockedToolMarker(`,
		`func HasApprovalRequiredToolMarker(`,
		`func HasPolicyBlockedToolMarker(`,
		`func ApprovalRequiredToolMarkerPayloadJSON(`,
		`func ParseApprovalRequiredToolMarkerPayload(`,
		`func ParseApprovalRequiredToolMarkerData(`,
	} {
		if !strings.Contains(toolMarker, fragment) {
			t.Errorf("agentcapabilities must own the shared Assistant tool marker vocabulary; missing %s", fragment)
		}
	}

	controlLevelSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "control_level.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/control_level.go: %v", err)
	}
	controlLevel := string(controlLevelSource)
	for _, fragment := range []string{
		`type ControlLevel string`,
		`ControlLevelReadOnly`,
		`ControlLevelControlled`,
		`ControlLevelAutonomous`,
		`func NormalizeControlLevel(`,
		`func IsValidControlLevel(`,
		`func ControlLevelAllowsControlTools(`,
	} {
		if !strings.Contains(controlLevel, fragment) {
			t.Errorf("agentcapabilities must own the shared control-level contract; missing %s", fragment)
		}
	}

	typesSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "types.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/types.go: %v", err)
	}
	typesSrc := string(typesSource)
	for _, fragment := range []string{
		`const ControlToolsDisabledMessage = "Control tools are disabled.`,
		`func NormalizeToolGovernance(`,
		`func NormalizeCapabilityGovernance(cap Capability) ToolGovernance`,
		`func CapabilityActionMode(cap Capability) ActionMode`,
		`func NewToolGovernanceDescriptor(`,
		`func NormalizeToolGovernanceDescriptor(descriptor ToolGovernanceDescriptor) ToolGovernanceDescriptor`,
		`governance.ActionMode = ActionModeWrite`,
		`CapabilityActionMode(cap)`,
		`governance.ApprovalPolicy = ApprovalPolicyActionPlan`,
		`DefaultApprovalPolicyDescription(governance.ApprovalPolicy)`,
		`ToolGovernanceDescriptor{`,
	} {
		if !strings.Contains(typesSrc, fragment) {
			t.Errorf("agentcapabilities must own shared tool-governance defaults; missing %s", fragment)
		}
	}
	governancePromptSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "governance_prompt.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/governance_prompt.go: %v", err)
	}
	governancePromptSrc := string(governancePromptSource)
	for _, fragment := range []string{
		`type ToolGovernancePromptOptions struct`,
		`type PulseIntelligenceOperatingInstructionOptions struct`,
		`func BuildToolGovernancePromptSection(`,
		`func BuildAssistantToolGovernancePromptSection(`,
		`func BuildPulseIntelligenceOperatingInstructions() string`,
		`func BuildPulseAssistantOperatingInstructions() string`,
		`type PulseMCPOperatingInstructionOptions struct`,
		`func BuildPulseMCPOperatingInstructions(`,
		`func BuildPulseIntelligenceOperatingInstructionsForSurface(`,
		`ResolveSurfaceAffordanceContract(`,
		`func ToolGovernancePromptLine(`,
		`func ToolGovernancePromptDescription(`,
		`func AssistantGovernanceOfferedToolNames(`,
		`AdditionalToolGovernanceLines []string`,
		`const PulseQuestionToolName = "pulse_question"`,
		`func PulseQuestionToolGovernancePromptLine() string`,
		`SupportsInteractiveQuestions bool`,
		`SurfaceAffordanceLabels(surfaceAffordancesFromOptions(opts))`,
		`## PULSE INTELLIGENCE OPERATING MODEL`,
		`No Pulse tools are offered for this turn.`,
		`This manifest is generated from Pulse's governed tool registry.`,
		`ApprovalPolicyScopeOnly`,
		`ActionModeRead`,
	} {
		if !strings.Contains(governancePromptSrc, fragment) {
			t.Errorf("agentcapabilities must own shared tool-governance prompt projection; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`func CloneCapability(cap Capability) Capability`,
		`func CloneCapabilities(capabilities []Capability) []Capability`,
		`cap.InputSchema = CloneRawMessage(cap.InputSchema)`,
	} {
		if !strings.Contains(typesSrc, fragment) {
			t.Errorf("agentcapabilities must own detached capability projection helpers; missing %s", fragment)
		}
	}

	textToolInvocationSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "text_tool_invocation.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/text_tool_invocation.go: %v", err)
	}
	textToolInvocation := string(textToolInvocationSource)
	for _, fragment := range []string{
		`const CurrentResourceHandle = "current_resource"`,
		`func IsCurrentResourceReference(value string) bool`,
		`func ToolInputContainsCurrentResourceReference(value any) bool`,
		`currentResourceReferenceAliases`,
		`"redacted by policy"`,
	} {
		if !strings.Contains(textToolInvocation, fragment) {
			t.Errorf("agentcapabilities must own shared current_resource tool-argument semantics; missing %s", fragment)
		}
	}

	configSource, err := os.ReadFile(filepath.Join("..", "config", "ai.go"))
	if err != nil {
		t.Fatalf("read internal/config/ai.go: %v", err)
	}
	configAI := string(configSource)
	for _, fragment := range []string{
		`ControlLevelReadOnly = string(agentcapabilities.ControlLevelReadOnly)`,
		`ControlLevelControlled = string(agentcapabilities.ControlLevelControlled)`,
		`ControlLevelAutonomous = string(agentcapabilities.ControlLevelAutonomous)`,
		`return string(agentcapabilities.NormalizeControlLevel(c.ControlLevel))`,
		`return agentcapabilities.ControlLevelAllowsControlTools(agentcapabilities.ControlLevel(c.GetControlLevel()))`,
		`return agentcapabilities.IsValidControlLevel(level)`,
	} {
		if !strings.Contains(configAI, fragment) {
			t.Errorf("AI config must alias shared control-level vocabulary; missing %s", fragment)
		}
	}

	assistantProtocol, err := os.ReadFile(filepath.Join("..", "ai", "tools", "protocol.go"))
	if err != nil {
		t.Fatalf("read internal/ai/tools/protocol.go: %v", err)
	}
	assistantSrc := string(assistantProtocol)
	for _, fragment := range []string{
		`type Tool = agentcapabilities.Tool`,
		`type CallToolResult = agentcapabilities.ToolResult`,
		`type Content = agentcapabilities.ToolContent`,
		`type CallToolParams = agentcapabilities.ToolCallParams`,
		`type ToolResponse = agentcapabilities.ToolResponse`,
		`type ToolError = agentcapabilities.ToolError`,
		`ErrCodeStrictResolution`,
		`agentcapabilities.ErrCodeStrictResolution`,
		`ErrCodeRoutingMismatch`,
		`agentcapabilities.ErrCodeRoutingMismatch`,
		`return agentcapabilities.NewToolTextResult(text)`,
		`return agentcapabilities.NewToolJSONResultWithIsError(data, isError)`,
		`return agentcapabilities.NewToolResponseResult(resp)`,
	} {
		if !strings.Contains(assistantSrc, fragment) {
			t.Errorf("Assistant tool protocol must wrap the shared tool result envelope; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`type Request = agentcapabilities.JSONRPCRequest`,
		`type Response = agentcapabilities.JSONRPCResponse`,
		`type Error = agentcapabilities.JSONRPCError`,
		`type InitializeResult = agentcapabilities.MCPInitializeResult`,
		`type ServerInfo = agentcapabilities.MCPServerInfo`,
		`type ListToolsResult = agentcapabilities.MCPListToolsResult`,
		`type Resource = agentcapabilities.MCPResource`,
		`type Prompt = agentcapabilities.MCPPrompt`,
	} {
		if strings.Contains(assistantSrc, fragment) {
			t.Errorf("native Assistant tool protocol must not expose MCP/JSON-RPC wire aliases; found %s", fragment)
		}
	}

	providerSource, err := os.ReadFile(filepath.Join("..", "ai", "providers", "provider.go"))
	if err != nil {
		t.Fatalf("read internal/ai/providers/provider.go: %v", err)
	}
	providerSrc := string(providerSource)
	for _, fragment := range []string{
		`type Tool = agentcapabilities.ProviderTool`,
		`type ToolCall = agentcapabilities.ProviderToolCall`,
		`type ToolResult = agentcapabilities.ProviderToolResult`,
		`return agentcapabilities.EmptyProviderTool()`,
		`return agentcapabilities.EmptyProviderToolCall()`,
	} {
		if !strings.Contains(providerSrc, fragment) {
			t.Errorf("Assistant providers must alias the shared provider tool shape; missing %s", fragment)
		}
	}

	for _, path := range []string{
		filepath.Join("..", "ai", "chat", "tools.go"),
		filepath.Join("..", "ai", "tools", "schema_projection.go"),
	} {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s must not keep provider projection compatibility wrappers; use internal/agentcapabilities directly", path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", path, err)
		}
	}

	assistantQuestionToolSource, err := os.ReadFile(filepath.Join("..", "ai", "chat", "agentic_question_tool.go"))
	if err != nil {
		t.Fatalf("read internal/ai/chat/agentic_question_tool.go: %v", err)
	}
	assistantQuestionToolSrc := string(assistantQuestionToolSource)
	for _, fragment := range []string{
		`const pulseQuestionToolName = agentcapabilities.PulseQuestionToolName`,
		`agentcapabilities.ParsePulseQuestionToolInput(input)`,
	} {
		if !strings.Contains(assistantQuestionToolSrc, fragment) {
			t.Errorf("Assistant question tool must use shared provider declaration; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`InputSchema: map[string]interface{}{`,
		`Name:        pulseQuestionToolName`,
		`Description: "Ask the user for missing information using a structured prompt.`,
		`func userQuestionTool()`,
		`type questionToolInputPayload struct`,
		`type questionToolInputQuestion struct`,
		`type questionToolInputOption struct`,
		`func parseQuestionToolOptions(`,
		`type questionToolType string`,
		`questionToolTypeText`,
		`questionToolTypeSelect`,
		`func normalizeQuestionToolType(`,
		`agentcapabilities.NormalizePulseQuestionToolType(parsedQuestion.Type, len(opts) > 0)`,
	} {
		if strings.Contains(assistantQuestionToolSrc, fragment) {
			t.Errorf("Assistant question tool must not keep a chat-local provider declaration; found %s", fragment)
		}
	}

	assistantFSMSource, err := os.ReadFile(filepath.Join("..", "ai", "chat", "fsm.go"))
	if err != nil {
		t.Fatalf("read internal/ai/chat/fsm.go: %v", err)
	}
	assistantFSMSrc := string(assistantFSMSource)
	for _, fragment := range []string{
		`type ToolKind = agentcapabilities.ToolCallKind`,
		`ToolKindResolve = agentcapabilities.ToolCallKindResolve`,
		`ToolKindRead = agentcapabilities.ToolCallKindRead`,
		`ToolKindWrite = agentcapabilities.ToolCallKindWrite`,
		`ToolKindUserInput = agentcapabilities.ToolCallKindUserInput`,
		`return agentcapabilities.ClassifyToolCall(toolName, args)`,
	} {
		if !strings.Contains(assistantFSMSrc, fragment) {
			t.Errorf("Assistant FSM must consume shared tool-call safety classification; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`case agentcapabilities.PulseQuestionToolName:`,
		`case "pulse_question":`,
		`case "pulse_control":`,
		`case "pulse_read":`,
		`writeActions := map[string]bool`,
		`readActions := map[string]bool`,
		`actionLower := strings.ToLower(action)`,
	} {
		if strings.Contains(assistantFSMSrc, fragment) {
			t.Errorf("Assistant FSM must not keep local tool-call safety classification; found %s", fragment)
		}
	}

	chatTypesSource, err := os.ReadFile(filepath.Join("..", "ai", "chat", "types.go"))
	if err != nil {
		t.Fatalf("read internal/ai/chat/types.go: %v", err)
	}
	chatTypesSrc := string(chatTypesSource)
	for _, fragment := range []string{
		`func ToolCallFromProvider(tc agentcapabilities.ProviderToolCall) ToolCall`,
		`func (t ToolCall) ProviderToolCall() agentcapabilities.ProviderToolCall`,
		`type ToolResult = agentcapabilities.ProviderToolResult`,
	} {
		if !strings.Contains(chatTypesSrc, fragment) {
			t.Errorf("Assistant chat transcript must project through shared provider tool shapes; missing %s", fragment)
		}
	}

	aiServiceSource, err := os.ReadFile(filepath.Join("..", "ai", "service.go"))
	if err != nil {
		t.Fatalf("read internal/ai/service.go: %v", err)
	}
	aiServiceSrc := string(aiServiceSource)
	for _, fragment := range []string{
		`type ChatToolCall = agentcapabilities.ProviderToolCall`,
		`return agentcapabilities.EmptyProviderToolCall()`,
		`type ChatToolResult = agentcapabilities.ProviderToolResult`,
		`return agentcapabilities.ApprovalRequiredToolMarker(`,
		`return agentcapabilities.PolicyBlockedToolMarker(command, reason)`,
		`agentcapabilities.ParseApprovalRequiredToolMarkerData(result)`,
		`agentcapabilities.HasApprovalRequiredToolMarker(result)`,
		`agentcapabilities.LegacyAssistantUtilityProviderTools()`,
		`agentcapabilities.LegacyAssistantRunCommandToolName`,
		`agentcapabilities.LegacyAssistantCommandArgumentName`,
		`agentcapabilities.LegacyAssistantURLArgumentName`,
		`agentcapabilities.LegacyAssistantResourceTypeArgumentName`,
		`agentcapabilities.LegacyAssistantResourceIDArgumentName`,
		`projection := newLegacyServiceProviderToolResultContextProjection(tc.ID, result, !execution.Success)`,
		`projection := newLegacyServiceProviderToolResultContextProjection(tc.ID, result, true)`,
		`agentcapabilities.NewProviderToolResultContextProjection(toolUseID, content, isError, legacyServiceProviderToolResultContextOptions())`,
		`ToolResult: &projection.Model`,
	} {
		if !strings.Contains(aiServiceSrc, fragment) {
			t.Errorf("AI service chat messages and tool markers must use shared provider/tool shapes; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`ToolResult: &providers.ToolResult{`,
		`resultForContext`,
		`maxResultSize`,
		`output truncated (`,
	} {
		if strings.Contains(aiServiceSrc, fragment) {
			t.Errorf("AI service must not preserve local provider-result context shaping; found %s", fragment)
		}
	}
	for _, fragment := range []string{
		`Name:        "run_command"`,
		`Name:        "fetch_url"`,
		`Name:        "set_resource_url"`,
		`case "run_command":`,
		`case "fetch_url":`,
		`case "set_resource_url":`,
		`tc.Input["command"]`,
		`tc.Input["run_on_host"]`,
		`tc.Input["target_host"]`,
		`tc.Input["url"]`,
		`tc.Input["resource_type"]`,
		`tc.Input["resource_id"]`,
	} {
		if strings.Contains(aiServiceSrc, fragment) {
			t.Errorf("legacy Assistant utility tool schemas and input keys must stay in agentcapabilities; AI service found %s", fragment)
		}
	}
	if strings.Contains(aiServiceSrc, `func parseApprovalNeededMarker(`) {
		t.Error("AI service approval marker handling must call the shared marker parser directly")
	}

	assistantChatAgentic, err := os.ReadFile(filepath.Join("..", "ai", "chat", "agentic.go"))
	if err != nil {
		t.Fatalf("read internal/ai/chat/agentic.go: %v", err)
	}
	agenticSrc := string(assistantChatAgentic)
	for _, fragment := range []string{
		`providerTools := executor.AssistantProviderTools(agentcapabilities.AssistantProviderToolOptions{`,
		`a.tools = a.executor.AssistantProviderTools(agentcapabilities.AssistantProviderToolOptions{`,
		`IncludeQuestionTool: true`,
		`agentcapabilities.HasApprovalRequiredToolMarker(resultText)`,
		`agentcapabilities.ParseApprovalRequiredToolMarkerData(resultText)`,
		`inputWithApproval = agentcapabilities.WithApprovalArgument(inputWithApproval, approvalData.ApprovalID)`,
		`agentcapabilities.NewProviderToolResultFromToolResult(tc.ID, result)`,
		`projection := newProviderToolResultContextProjection(tc.ID, resultText, isError)`,
		`ToolResult: &projection.Transcript`,
		`ToolResult: &projection.Model`,
		`toolCalls = agentcapabilities.NormalizeProviderToolCallsForExecution(data.ToolCalls)`,
		`agentcapabilities.ToolInputContainsCurrentResourceReference(tc.Input)`,
	} {
		if !strings.Contains(agenticSrc, fragment) {
			t.Errorf("Assistant agentic loop must use shared provider-result and marker vocabulary; missing %s", fragment)
		}
	}
	if strings.Contains(agenticSrc, `ProjectAssistantProviderTools(`) || strings.Contains(agenticSrc, `ListToolGovernance()`) {
		t.Error("Assistant agentic loop must request provider-tool projection from the shared executor entrypoint")
	}
	if strings.Contains(agenticSrc, `resultText = FormatToolResult(result)`) {
		t.Error("Assistant agentic loop must project MCP tool results through shared provider-result construction, not local result text assignment")
	}
	for _, fragment := range []string{
		`agentcapabilities.ApprovalRequiredToolMarkerPayloadJSON(resultText)`,
		`var approvalData struct`,
		"\n\t\t\t\t\t\t\tagentcapabilities.WithApprovalArgument(inputWithApproval, approvalData.ApprovalID)",
		`ToolResult: &providers.ToolResult{`,
		`ToolResult: &ToolResult{`,
		`modelResultText := truncateToolResultForModel(resultText)`,
		`func toolInputContainsCurrentResourceReference`,
		`append(providerTools, userQuestionTool())`,
		`userQuestionTool()`,
		`toolCalls = NormalizeProviderToolCallsForExecution(data.ToolCalls)`,
		`func ConvertPulseToolsToProvider`,
		`func ConvertPulseToolsToProviderWithGovernance`,
		`func ConvertProviderToolCallToMCP`,
		`func NormalizeProviderToolCallForExecution`,
		`func NormalizeProviderToolCallsForExecution`,
	} {
		if strings.Contains(agenticSrc, fragment) {
			t.Errorf("Assistant agentic loop must not assemble provider tool-result structs locally; found %s", fragment)
		}
	}

	currentResourceSource, err := os.ReadFile(filepath.Join("..", "ai", "tools", "current_resource.go"))
	if err != nil {
		t.Fatalf("read internal/ai/tools/current_resource.go: %v", err)
	}
	currentResourceSrc := string(currentResourceSource)
	for _, fragment := range []string{
		`const currentResourceHandle = agentcapabilities.CurrentResourceHandle`,
		`return agentcapabilities.IsCurrentResourceReference(value)`,
	} {
		if !strings.Contains(currentResourceSrc, fragment) {
			t.Errorf("Assistant current_resource executor must use shared handle semantics; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`"attached_resource"`,
		`"selected_resource"`,
		`"this_resource"`,
		`"redacted by policy"`,
	} {
		if strings.Contains(currentResourceSrc, fragment) {
			t.Errorf("Assistant current_resource executor must not keep local handle aliases; found %s", fragment)
		}
	}

	assistantChatContext, err := os.ReadFile(filepath.Join("..", "ai", "chat", "agentic_context.go"))
	if err != nil {
		t.Fatalf("read internal/ai/chat/agentic_context.go: %v", err)
	}
	agenticContextSrc := string(assistantChatContext)
	for _, fragment := range []string{
		`agentcapabilities.ProviderToolResultModelContent(text, providerToolResultContextOptions())`,
		`agentcapabilities.NewProviderToolResultContextProjection(toolUseID, content, isError, providerToolResultContextOptions())`,
		`pm.ToolResult = &projection.Model`,
		`agentcapabilities.NewProviderToolErrorResult(`,
	} {
		if !strings.Contains(agenticContextSrc, fragment) {
			t.Errorf("Assistant provider message conversion must use shared provider-result construction; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`func newProviderToolResult(`,
		`func newProviderToolErrorResult(`,
		`ToolResult: &providers.ToolResult{`,
	} {
		if strings.Contains(agenticContextSrc, fragment) {
			t.Errorf("Assistant provider message conversion must not preserve local provider-result wrappers or structs; found %s", fragment)
		}
	}

	assistantChatService, err := os.ReadFile(filepath.Join("..", "ai", "chat", "service.go"))
	if err != nil {
		t.Fatalf("read internal/ai/chat/service.go: %v", err)
	}
	chatServiceSrc := string(assistantChatService)
	for _, fragment := range []string{
		`manifest = s.executor.ListToolGovernance()`,
		`agentcapabilities.ProviderToolNames(offeredTools)`,
		`agentcapabilities.ProviderToolGovernanceDescriptors(offeredTools)`,
		`agentcapabilities.BuildAssistantToolGovernancePromptSection(manifest, orderedOfferedNames)`,
		`tools.CanonicalToolGovernanceForManifestSurface(`,
		`agentcapabilities.SurfaceIDPulseAssistant`,
		`fallbackAssistantToolGovernanceOfferedNames(manifest)`,
		`agentcapabilities.ManifestSurfaceAffordances(`,
		`providerTools := s.executor.AssistantProviderTools(agentcapabilities.AssistantProviderToolOptions{`,
		`IncludeQuestionTool: includeQuestionTool`,
		`agentcapabilities.BuildPulseAssistantOperatingInstructions()`,
		`func (s *Service) executeAssistantRegistryToolDirect(ctx context.Context, toolName string, args map[string]interface{}, opts agentcapabilities.DirectToolExecutionOptions) (agentcapabilities.DirectToolExecutionOutcome, error)`,
		`outcome, executionErr := s.executeAssistantRegistryToolDirect(ctx, agentcapabilities.PulseRunCommandToolName, args, agentcapabilities.DirectToolExecutionOptions{`,
		`outcome, executionErr := s.executeAssistantRegistryToolDirect(ctx, toolName, args, agentcapabilities.DirectToolExecutionOptions{`,
		`result, toolErr := executor.ExecuteTool(ctx, toolName, args)`,
		`agentcapabilities.InterpretDirectToolExecution(result`,
		`FailurePrefix:           "command execution failed"`,
		`ApprovalRequiredMessage: "command requires approval (unexpected in autonomous mode)"`,
		`PolicyBlockedMessage:    "command blocked by security policy"`,
		`FailurePrefix:           "tool execution failed"`,
		`ApprovalRequiredMessage: "tool requires approval (unexpected in fix execution)"`,
		`PolicyBlockedMessage:    "tool blocked by security policy"`,
	} {
		if !strings.Contains(chatServiceSrc, fragment) {
			t.Errorf("Assistant chat service must use shared direct result execution mapping; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`agentcapabilities.DirectMCPToolExecutionOptions`,
		`agentcapabilities.DirectMCPToolExecutionOutcome`,
		`agentcapabilities.InterpretDirectMCPToolExecution`,
	} {
		if strings.Contains(chatServiceSrc, fragment) {
			t.Errorf("Assistant chat service must use neutral direct tool execution helpers, found %s", fragment)
		}
	}
	if strings.Contains(chatServiceSrc, `ProjectAssistantProviderTools(`) {
		t.Error("Assistant chat service must request provider-tool projection from the shared executor entrypoint")
	}
	if strings.Contains(chatServiceSrc, `tools.CanonicalToolGovernanceForSurface(`) ||
		strings.Contains(chatServiceSrc, `tools.ToolGovernanceSurfacePulseAssistant`) {
		t.Error("Assistant chat service fallback governance must enter through manifest surface affordances")
	}
	for _, fragment := range []string{
		`b.WriteString("## AVAILABLE TOOL GOVERNANCE`,
		`agentcapabilities.ToolGovernancePromptOptions{`,
		`agentcapabilities.PulseQuestionToolGovernancePromptLine()`,
		`assistantGovernanceOfferedToolNames`,
		`userQuestionTool()`,
		`mode := tool.ActionMode`,
		`approvalSummary := strings.TrimSpace(tool.ApprovalSummary)`,
		`fmt.Sprintf("- %s: mode=%s; approval=%s`,
		`for _, content := range result.Content`,
		`resultText += content.Text`,
		`agentcapabilities.HasApprovalRequiredToolMarker(resultText)`,
		`agentcapabilities.HasPolicyBlockedToolMarker(resultText)`,
		`interpreted := agentcapabilities.InterpretToolResult(result)`,
		`if interpreted.IsError`,
		`if interpreted.ApprovalRequired`,
		`if interpreted.PolicyBlocked`,
		`func assistantQuestionToolGovernancePromptLine() string`,
	} {
		if strings.Contains(chatServiceSrc, fragment) {
			t.Errorf("Assistant chat service must not flatten or interpret direct MCP result outcomes locally; found %s", fragment)
		}
	}

	chatServiceAdapterSource, err := os.ReadFile(filepath.Join("..", "api", "chat_service_adapter.go"))
	if err != nil {
		t.Fatalf("read internal/api/chat_service_adapter.go: %v", err)
	}
	chatServiceAdapterSrc := string(chatServiceAdapterSource)
	for _, fragment := range []string{
		`func adaptChatMessage(m chat.Message) ai.ChatMessage`,
		`msg.ToolCalls = append(msg.ToolCalls, tc.ProviderToolCall())`,
		`toolResult := *m.ToolResult`,
	} {
		if !strings.Contains(chatServiceAdapterSrc, fragment) {
			t.Errorf("chat service adapter must bridge messages through shared provider tool shapes; missing %s", fragment)
		}
	}

	orchestratorDepsSource, err := os.ReadFile(filepath.Join("..", "..", "pkg", "aicontracts", "orchestrator_deps.go"))
	if err != nil {
		t.Fatalf("read pkg/aicontracts/orchestrator_deps.go: %v", err)
	}
	orchestratorDepsSrc := string(orchestratorDepsSource)
	for _, fragment := range []string{
		`func OrchestratorToolCallInfoFromProvider(tc agentcapabilities.ProviderToolCall) OrchestratorToolCallInfo`,
		`func (t OrchestratorToolCallInfo) ProviderToolCall() agentcapabilities.ProviderToolCall`,
		`func OrchestratorToolResultInfoFromProvider(result agentcapabilities.ProviderToolResult) OrchestratorToolResultInfo`,
		`func (r OrchestratorToolResultInfo) ProviderToolResult() agentcapabilities.ProviderToolResult`,
		`return agentcapabilities.NewProviderToolResult(r.ToolUseID, r.Content, r.IsError)`,
	} {
		if !strings.Contains(orchestratorDepsSrc, fragment) {
			t.Errorf("orchestrator dependency contract must bridge messages through shared provider tool shapes; missing %s", fragment)
		}
	}

	aiHandlersSource, err := os.ReadFile(filepath.Join("..", "api", "ai_handlers.go"))
	if err != nil {
		t.Fatalf("read internal/api/ai_handlers.go: %v", err)
	}
	aiHandlersSrc := string(aiHandlersSource)
	for _, fragment := range []string{
		`aicontracts.OrchestratorToolCallInfoFromProvider(tc.ProviderToolCall())`,
		`toolResult := aicontracts.OrchestratorToolResultInfoFromProvider(*msg.ToolResult)`,
	} {
		if !strings.Contains(aiHandlersSrc, fragment) {
			t.Errorf("orchestrator chat adapter must bridge messages through shared provider tool shapes; missing %s", fragment)
		}
	}

	patrolRuntime, err := os.ReadFile(filepath.Join("..", "ai", "patrol_ai.go"))
	if err != nil {
		t.Fatalf("read internal/ai/patrol_ai.go: %v", err)
	}
	patrolSrc := string(patrolRuntime)
	if strings.Contains(patrolSrc, "func formatToolResult(") {
		t.Error("Patrol runtime must not keep a local result-text wrapper; call agentcapabilities.ToolResultText at the execution site")
	}

	patrolFindings, err := os.ReadFile(filepath.Join("..", "ai", "patrol_findings.go"))
	if err != nil {
		t.Fatalf("read internal/ai/patrol_findings.go: %v", err)
	}
	patrolFindingsSrc := string(patrolFindings)
	for _, fragment := range []string{
		`agentcapabilities.InterpretToolResult(result)`,
		`if interpreted.IsError`,
		`if interpreted.ApprovalRequired`,
		`if interpreted.PolicyBlocked`,
	} {
		if !strings.Contains(patrolFindingsSrc, fragment) {
			t.Errorf("Patrol tool-call records must use shared tool result interpretation; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`output = agentcapabilities.MCPToolResultText(result)`,
		`agentcapabilities.InterpretMCPToolResult(result)`,
		`agentcapabilities.MCPToolResultInterpretation`,
		`success = !result.IsError`,
		`if result.IsError`,
		`formatToolResult(result)`,
	} {
		if strings.Contains(patrolFindingsSrc, fragment) {
			t.Errorf("Patrol tool-call records must not duplicate shared result interpretation locally; found %s", fragment)
		}
	}

	assistantRegistry, err := os.ReadFile(filepath.Join("..", "ai", "tools", "registry.go"))
	if err != nil {
		t.Fatalf("read internal/ai/tools/registry.go: %v", err)
	}
	registrySrc := string(assistantRegistry)
	for _, fragment := range []string{
		`type ControlLevel = agentcapabilities.ControlLevel`,
		`ControlLevelReadOnly ControlLevel = agentcapabilities.ControlLevelReadOnly`,
		`ControlLevelControlled ControlLevel = agentcapabilities.ControlLevelControlled`,
		`ControlLevelAutonomous ControlLevel = agentcapabilities.ControlLevelAutonomous`,
		`!agentcapabilities.ControlLevelAllowsControlTools(controlLevel)`,
		`!agentcapabilities.ControlLevelAllowsControlTools(e.controlLevel)`,
		`agentcapabilities.NewToolGovernanceDescriptor(`,
		`tool.Definition = tool.Definition.NormalizeCollections()`,
		`result = append(result, tool.Definition.NormalizeCollections())`,
		`params, invalidResult, ok := agentcapabilities.PrepareToolRegistryExecution(name, args)`,
		`agentcapabilities.NewUnknownToolResult(name)`,
		`agentcapabilities.NewControlToolsDisabledToolResult()`,
		`return result.NormalizeCollections(), err`,
	} {
		if !strings.Contains(registrySrc, fragment) {
			t.Errorf("Assistant tool registry must use shared agentcapabilities contracts; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`assistantControlToolsDisabledMessage`,
		`func normalizeToolGovernance(`,
		`DefaultApprovalPolicyDescription(governance.ApprovalPolicy)`,
		`governance.ActionMode = ToolActionWrite`,
		`governance.ApprovalPolicy = ToolApprovalActionPlan`,
		`invalid tools/call params:`,
		`unknown tool: %s`,
		`agentcapabilities.ControlToolsDisabledMessage`,
	} {
		if strings.Contains(registrySrc, fragment) {
			t.Errorf("Assistant tool registry must not preserve local governance defaults; found %s", fragment)
		}
	}

	assistantRegistryToolFiles := []string{
		filepath.Join("..", "ai", "tools", "tools_alerts.go"),
		filepath.Join("..", "ai", "tools", "tools_control.go"),
		filepath.Join("..", "ai", "tools", "tools_discovery.go"),
		filepath.Join("..", "ai", "tools", "tools_docker.go"),
		filepath.Join("..", "ai", "tools", "tools_file.go"),
		filepath.Join("..", "ai", "tools", "tools_kubernetes.go"),
		filepath.Join("..", "ai", "tools", "tools_knowledge.go"),
		filepath.Join("..", "ai", "tools", "tools_metrics.go"),
		filepath.Join("..", "ai", "tools", "tools_patrol.go"),
		filepath.Join("..", "ai", "tools", "tools_pmg.go"),
		filepath.Join("..", "ai", "tools", "tools_query.go"),
		filepath.Join("..", "ai", "tools", "tools_read.go"),
		filepath.Join("..", "ai", "tools", "tools_storage.go"),
		filepath.Join("..", "ai", "tools", "tools_summarize.go"),
	}
	assistantRegistryToolSrc := strings.Builder{}
	for _, path := range assistantRegistryToolFiles {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		assistantRegistryToolSrc.Write(source)
		assistantRegistryToolSrc.WriteString("\n")
	}
	registryToolDeclarations := assistantRegistryToolSrc.String()
	for _, fragment := range []string{
		`Name: agentcapabilities.PulseAlertsToolName`,
		`Name:        agentcapabilities.PulseControlToolName`,
		`Name:        agentcapabilities.PulseDiscoveryToolName`,
		`Name:        agentcapabilities.PulseDockerToolName`,
		`Name: agentcapabilities.PulseFileEditToolName`,
		`Name:        agentcapabilities.PulseKubernetesToolName`,
		`Name: agentcapabilities.PulseKnowledgeToolName`,
		`Name: agentcapabilities.PulseMetricsToolName`,
		`Name: agentcapabilities.PatrolReportFindingToolName`,
		`Name: agentcapabilities.PatrolResolveFindingToolName`,
		`Name: agentcapabilities.PatrolGetFindingsToolName`,
		`Name:        agentcapabilities.PulsePMGToolName`,
		`Name:        agentcapabilities.PulseQueryToolName`,
		`Name:        agentcapabilities.PulseReadToolName`,
		`Name:        agentcapabilities.PulseStorageToolName`,
		`Name: agentcapabilities.PulseSummarizeToolName`,
	} {
		if !strings.Contains(registryToolDeclarations, fragment) {
			t.Errorf("Assistant registry tool declarations must use shared tool identities; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`Name: "pulse_`,
		`Name:        "pulse_`,
		`Name: "patrol_`,
		`Name:        "patrol_`,
	} {
		if strings.Contains(registryToolDeclarations, fragment) {
			t.Errorf("Assistant registry tool declarations must not own local Pulse tool-name strings; found %s", fragment)
		}
	}

	assistantPatrolTools, err := os.ReadFile(filepath.Join("..", "ai", "tools", "tools_patrol.go"))
	if err != nil {
		t.Fatalf("read internal/ai/tools/tools_patrol.go: %v", err)
	}
	assistantPatrolToolsSrc := string(assistantPatrolTools)
	for _, fragment := range []string{
		`agentcapabilities.FindingIDArgumentName`,
		`agentcapabilities.ReasonArgumentName`,
		`return NewJSONResult(result), nil`,
	} {
		if !strings.Contains(assistantPatrolToolsSrc, fragment) {
			t.Errorf("Assistant Patrol tools must use shared finding vocabulary and JSON result envelope; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`json.Marshal(result)`,
		`NewTextResult(string(b))`,
	} {
		if strings.Contains(assistantPatrolToolsSrc, fragment) {
			t.Errorf("Assistant Patrol tools must not manually marshal JSON text results; found %s", fragment)
		}
	}

	assistantProjection, err := os.ReadFile(filepath.Join("..", "ai", "tools", "provider_projection.go"))
	if err != nil {
		t.Fatalf("read internal/ai/tools/provider_projection.go: %v", err)
	}
	assistantProjectionSrc := string(assistantProjection)
	for _, fragment := range []string{
		`func (e *PulseToolExecutor) AssistantProviderTools(opts agentcapabilities.AssistantProviderToolOptions) []agentcapabilities.ProviderTool`,
		`return agentcapabilities.ProjectPulseAssistantProviderTools(agentcapabilities.CanonicalManifest(), e.ListTools(), e.ListToolGovernance(), opts)`,
		`return agentcapabilities.ProjectPulseAssistantProviderTools(agentcapabilities.CanonicalManifest(), nil, nil, opts)`,
	} {
		if !strings.Contains(assistantProjectionSrc, fragment) {
			t.Errorf("Assistant provider-tool projection must stay centralized on the shared executor; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`return agentcapabilities.ProjectAssistantProviderTools(e.ListTools(), e.ListToolGovernance(), opts)`,
		`return agentcapabilities.ProjectAssistantProviderTools(nil, nil, opts)`,
	} {
		if strings.Contains(assistantProjectionSrc, fragment) {
			t.Errorf("Assistant provider-tool projection must enter through manifest surface affordances; found %s", fragment)
		}
	}

	assistantToolNames, err := os.ReadFile(filepath.Join("..", "ai", "tools", "names.go"))
	if err != nil {
		t.Fatalf("read internal/ai/tools/names.go: %v", err)
	}
	assistantToolNamesSrc := string(assistantToolNames)
	for _, fragment := range []string{
		`agentcapabilities.ProviderToolNameCatalog`,
		`func AssistantProviderToolNameCatalog() agentcapabilities.ProviderToolNameCatalog`,
		`return AssistantProviderToolNameCatalog().Has(name)`,
		`return AssistantProviderToolNameCatalog().HasPrefix(prefix)`,
		`return knownToolNameCatalog`,
		`agentcapabilities.NewAssistantProviderToolNameCatalog(e.registry.allNames())`,
	} {
		if !strings.Contains(assistantToolNamesSrc, fragment) {
			t.Errorf("Assistant tool-name allowlist must adapt the shared provider-tool catalog; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`agentcapabilities.AssistantNativeProviderToolNames()`,
		`knownToolNamesList`,
		`knownToolNamesSet`,
		`strings.HasPrefix(name, prefix)`,
	} {
		if strings.Contains(assistantToolNamesSrc, fragment) {
			t.Errorf("Assistant tool-name allowlist must not keep local provider-tool catalog logic; found %s", fragment)
		}
	}
	if strings.Contains(assistantToolNamesSrc, `"pulse_question"`) {
		t.Error("Assistant tool-name allowlist must not hard-code native Assistant provider tool names")
	}

	assistantSanitize, err := os.ReadFile(filepath.Join("..", "ai", "chat", "agentic_sanitize.go"))
	if err != nil {
		t.Fatalf("read internal/ai/chat/agentic_sanitize.go: %v", err)
	}
	assistantSanitizeSrc := string(assistantSanitize)
	for _, fragment := range []string{
		`agentcapabilities.ProviderToolCallArtifactIndex(content, tools.AssistantProviderToolNameCatalog())`,
		`agentcapabilities.SplitTrailingProviderToolNamePrefix(content, tools.AssistantProviderToolNameCatalog())`,
	} {
		if !strings.Contains(assistantSanitizeSrc, fragment) {
			t.Errorf("Assistant chat sanitizer must use shared provider tool-call artifact detection; missing %s", fragment)
		}
	}
	for _, fragment := range []string{
		`dsmlMarkers`,
		`xmlToolCallRe`,
		`pipeMarkerRe`,
		`minimaxMarkerRe`,
		`jsonToolCallRe`,
		`plainFunctionToolCallRe`,
		`findJSONToolCallLeak`,
		`findPlainFunctionToolCallLeak`,
	} {
		if strings.Contains(assistantSanitizeSrc, fragment) {
			t.Errorf("Assistant chat sanitizer must not preserve local provider artifact detection; found %s", fragment)
		}
	}
}

// TestContract_AgentProbeUsesSharedAgentCapabilityProjection pins the reference
// client boundary: the probe may demonstrate transport mechanics, but manifest
// lookup and route projection must use the same shared helpers as MCP.
func TestContract_AgentProbeUsesSharedAgentCapabilityProjection(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "cmd", "agent-probe", "main.go"))
	if err != nil {
		t.Fatalf("read cmd/agent-probe/main.go: %v", err)
	}
	src := string(source)
	required := []string{
		`internal/agentcapabilities`,
		`agentcapabilities.FleetContextCapabilityName`,
		`agentcapabilities.ResourceContextCapabilityName`,
		`agentcapabilities.FindCapability(manifest.Capabilities, agentcapabilities.EventSubscriptionCapabilityName)`,
		`agentcapabilities.FetchManifest(context.Background(), client, *baseURL)`,
		`agentcapabilities.CallRequestResponseCapabilityHTTPBodyByName(`,
		`agentcapabilities.StreamAgentActionableSSERecords(ctx, nil, baseURL, path, token, func(record agentcapabilities.SSERecord) bool`,
	}
	for _, fragment := range required {
		if !strings.Contains(src, fragment) {
			t.Errorf("agent-probe must use the shared agent capability projection helpers; missing %s", fragment)
		}
	}
	forbidden := []string{
		`agentcapabilities.FindCapability(manifest.Capabilities, "get_fleet_context")`,
		`agentcapabilities.FindCapability(manifest.Capabilities, "get_resource_context")`,
		`agentcapabilities.CallCapabilityHTTPByName(`,
		`agentcapabilities.CallCapabilityHTTP(ctx, client, baseURL, token, cap, args)`,
		`resp.FailureError()`,
		`strings.Replace(contextCap.Path`,
		`byName := map[string]agentCapability`,
		`type errorEnvelope struct`,
		`baseURL + "/api/agent/capabilities"`,
		`req.Header.Set("X-API-Token", token)`,
		`agentcapabilities.DecodeErrorEnvelope(body)`,
		`req.Header.Set("Accept", "text/event-stream")`,
		`http.DefaultClient.Do(req)`,
		`scanner := bufio.NewScanner(resp.Body)`,
		`strings.HasPrefix(line, "event: ")`,
		`strings.HasPrefix(line, "data: ")`,
		`agentcapabilities.StreamAgentSSERecords(ctx, nil, baseURL, path, token`,
		`agentcapabilities.IsTransportEventKind(record.Event)`,
	}
	for _, fragment := range forbidden {
		if strings.Contains(src, fragment) {
			t.Errorf("agent-probe must not preserve local manifest projection logic; found %s", fragment)
		}
	}
}

// TestContract_AgentSurfaceErrorEnvelopeUsesSharedAgentCapabilitiesType pins
// the stable agent error envelope as part of the shared agent capabilities
// contract instead of an API-local map or probe-local struct.
func TestContract_AgentSurfaceErrorEnvelopeUsesSharedAgentCapabilitiesType(t *testing.T) {
	source, err := os.ReadFile("middleware_tenant.go")
	if err != nil {
		t.Fatalf("read middleware_tenant.go: %v", err)
	}
	if !strings.Contains(string(source), "agentcapabilities.NewErrorEnvelope(code, message, details)") {
		t.Error("writeJSONErrorWithDetails must emit the shared agentcapabilities.ErrorEnvelope shape")
	}

	agentCapabilitiesSource, err := os.ReadFile(filepath.Join("..", "agentcapabilities", "errors.go"))
	if err != nil {
		t.Fatalf("read internal/agentcapabilities/errors.go: %v", err)
	}
	for _, fragment := range []string{
		"type ErrorEnvelope struct",
		"Error   string",
		"Message string",
		"Details map[string]string",
		`AgentErrCodeResourceNotFound           = "resource_not_found"`,
		`AgentErrCodeOperatorStateNotSet        = "operator_state_not_set"`,
		`AgentErrCodeInvalidActionRequest       = "invalid_action_request"`,
		`AgentErrCodeActionExecutionUnavailable = "action_execution_unavailable"`,
		"func NewErrorEnvelope(",
		"func DecodeErrorEnvelope(",
	} {
		if !strings.Contains(string(agentCapabilitiesSource), fragment) {
			t.Errorf("agentcapabilities must own the shared agent error envelope; missing %s", fragment)
		}
	}
}

// TestContract_AgentCapabilitiesManifestDeclaresProvisioningSurface pins the
// node-lifecycle onboarding surface in the public agent manifest. This is the
// backend API payload proof required when the hand-authored manifest starts
// projecting /api/config/nodes and /api/discover to external agents.
func TestContract_AgentCapabilitiesManifestDeclaresProvisioningSurface(t *testing.T) {
	src := readAgentCapabilitiesManifestSource(t)

	required := []string{
		`"provisioning"`,
		`ActionMode:`,
		`ApprovalPolicy:`,
		`StrictObjectInputSchema`,
		`agentCapabilityActionModeRead`,
		`agentCapabilityActionModeMixed`,
		`agentCapabilityActionModeWrite`,
		`agentCapabilityApprovalPolicyScopeOnly`,
		`agentCapabilityApprovalPolicyActionPlan`,
		`ListNodesCapabilityName`,
		`Path:           "/api/config/nodes"`,
		`Scope:          agentCapabilityScopeSettingsRead`,
		`AddNodeCapabilityName`,
		`InputSchema:      addNodeInputSchema()`,
		`UpdateNodeCapabilityName`,
		`InputSchema:      updateNodeInputSchema()`,
		`RemoveNodeCapabilityName`,
		`InputSchema:    nodeIDInputSchema()`,
		`TestNodeCredentialsCapabilityName`,
		`Path:             "/api/config/nodes/test-config"`,
		`TestNodeConnectionCapabilityName`,
		`Path:           "/api/config/nodes/{nodeId}/test"`,
		`RefreshNodeClusterMembershipCapabilityName`,
		`Path:           "/api/config/nodes/{nodeId}/refresh-cluster"`,
		`DiscoverLANCapabilityName`,
		`Path:             "/api/discover"`,
		`InputSchema:      discoverLANInputSchema()`,
		`"tokenValue"`,
		`StrictObjectInputSchema`,
	}
	for _, fragment := range required {
		if !strings.Contains(src, fragment) {
			t.Errorf("agent capabilities manifest must declare provisioning contract fragment %s", fragment)
		}
	}

	manifest := agentcapabilities.CanonicalManifest()
	mcp, ok := agentcapabilities.FindManifestSurfaceToolContract(manifest, agentcapabilities.SurfaceIDPulseMCP)
	if !ok {
		t.Fatal("agent capabilities manifest must publish Pulse MCP surface tool contract")
	}
	for _, name := range []string{
		agentcapabilities.ListNodesCapabilityName,
		agentcapabilities.AddNodeCapabilityName,
		agentcapabilities.UpdateNodeCapabilityName,
		agentcapabilities.RemoveNodeCapabilityName,
		agentcapabilities.TestNodeCredentialsCapabilityName,
		agentcapabilities.TestNodeConnectionCapabilityName,
		agentcapabilities.RefreshNodeClusterMembershipCapabilityName,
		agentcapabilities.DiscoverLANCapabilityName,
	} {
		if _, ok := agentcapabilities.FindCapability(manifest.Capabilities, name); !ok {
			t.Errorf("agent capabilities manifest must declare provisioning capability %q", name)
		}
		if !stringSliceContains(mcp.ToolNames, name) {
			t.Errorf("Pulse MCP surface contract must explicitly allow provisioning capability %q", name)
		}
	}
}

// TestContract_AgentCapabilitiesManifestDeclaresTypedActionAndFindingSchemas
// pins that high-risk external-agent tools expose exact manifest-owned
// argument schemas instead of adapter-local free-form body conventions.
func TestContract_AgentCapabilitiesManifestDeclaresTypedActionAndFindingSchemas(t *testing.T) {
	src := readAgentCapabilitiesManifestSource(t)
	required := []string{
		`operatorStateInputSchema()`,
		`findingIDInputSchema(`,
		`snoozeFindingInputSchema()`,
		`dismissFindingInputSchema()`,
		`actionPlanInputSchema()`,
		`actionDecisionInputSchema()`,
		`actionExecutionInputSchema()`,
		`FindingIDArgumentName: stringOption(description)`,
		`FindingIDArgumentName, "duration_hours"`,
		`FindingIDArgumentName, ReasonArgumentName`,
		`NoteArgumentName: stringOption("Optional operator note explaining the dismissal.")`,
		`ResourceIDArgumentName, "intentionallyOffline", "neverAutoRemediate"`,
		`"maintenanceStartAt": {"maintenanceEndAt"}`,
		`"not_an_issue", "expected_behavior", "will_fix_later"`,
		`"approved", "rejected"`,
		`RequestIDArgumentName, ResourceIDArgumentName, CapabilityNameArgumentName, ReasonArgumentName, RequestedByArgumentName`,
		`ActionIDArgumentName, OutcomeArgumentName`,
	}
	for _, fragment := range required {
		if !strings.Contains(src, fragment) {
			t.Errorf("agent capabilities manifest must declare typed action/finding input schema fragment %s", fragment)
		}
	}
}

// TestContract_AgentCapabilitiesManifestDeclaresStructuredOutputSchemas pins
// that MCP structuredContent validation metadata is owned by the canonical
// capabilities manifest rather than a local adapter table.
func TestContract_AgentCapabilitiesManifestDeclaresStructuredOutputSchemas(t *testing.T) {
	src := readAgentCapabilitiesManifestSource(t)
	required := []string{
		`fleetContextOutputSchema()`,
		`operationsLoopStatusOutputSchema()`,
		`resourceContextOutputSchema()`,
		`operatorStateOutputSchema()`,
		`toolArrayOutputSchema("Configured infrastructure source records visible to the agent.")`,
		`nodeConnectionTestOutputSchema()`,
		`clusterRefreshOutputSchema()`,
		`discoveryOutputSchema()`,
		`findingActionOutputSchema()`,
		`actionPlanOutputSchema()`,
		`actionDecisionOutputSchema()`,
		`actionExecutionOutputSchema()`,
		`OutputSchema:   fleetContextOutputSchema()`,
		`OutputSchema:   operationsLoopStatusOutputSchema()`,
		`OutputSchema:   resourceContextOutputSchema()`,
		`OutputSchema:   operatorStateOutputSchema()`,
		`OutputSchema:   toolArrayOutputSchema("Patrol finding records returned by the lifecycle endpoint.")`,
		`OutputSchema:     actionPlanOutputSchema()`,
		`"resources", "generatedAt"`,
		`"nextAction", "progressLabel", "steps", "patrolEvidenceCount", "patrolIssueEvidenceCount", "activeFindingCount", "pendingApprovalCount", "governedActionCount", "approvedDecisionCount", "rejectedDecisionCount", "verifiedOutcomeCount", "operationsLoopStarterCount", "assistantOperationsLoopStarterCount", "patrolOperationsLoopStarterCount", "patrolControlOperationsLoopStarterCount", "patrolControlCompletedOperationsLoopCount", "patrolControlResolvedOperationsLoopCount", "patrolControlValueState", "patrolAutonomyOperationsLoopStarterCount", "patrolAutonomyCompletedOperationsLoopCount", "patrolAutonomyResolvedOperationsLoopCount", "patrolAutonomyValueState", "proActivationOperationsLoopStarterCount", "proActivationCompletedOperationsLoopCount", "proActivationResolvedOperationsLoopCount", "proActivationValueProofState", "mcpOperationsLoopStarterCount", "externalAgentReady", "windowStart", "generatedAt"`,
		`"Count of recent decision-backed governed actions without action ids or command content."`,
		`"Count of recent approved governed-action decisions without action ids or command content."`,
		`"Count of recent rejected governed-action decisions without action ids or command content."`,
		`"Count of recent approved governed actions with a verified post-action outcome."`,
		`"Count of content-free Patrol work prompt starter events in the evidence window."`,
		`"Count of Patrol work prompt starters rendered by Pulse Assistant in the evidence window."`,
		`"Count of Patrol work prompt starters launched from Pulse Patrol in the evidence window."`,
		`"Count of Patrol work prompt starters launched from the first-party Patrol control journey in the evidence window."`,
		`"Count-only evidence that the Patrol control journey reached a terminal Patrol work result in the evidence window: Patrol issue evidence, contextual Assistant or external-agent collaboration, and either a rejected governed decision or an approved governed decision with a verified outcome."`,
		`"Count-only evidence that the Patrol control journey reached a verified Patrol work result in the evidence window: Patrol issue evidence, contextual Assistant or external-agent collaboration, approved governed decision, and verified outcome."`,
		`"Content-safe Patrol control value state. Only verified means Patrol completed a verified work outcome; governed_decision_recorded means a safe terminal decision exists without verified outcome evidence."`,
		`"Compatibility field retained for older external-agent clients; primary clients should use patrolControlOperationsLoopStarterCount."`,
		`"Compatibility alias for patrolControlCompletedOperationsLoopCount."`,
		`"Compatibility alias for patrolControlResolvedOperationsLoopCount."`,
		`"Compatibility alias for patrolControlValueState."`,
		`"Count of Patrol work prompt starters rendered by the Pulse MCP surface in the evidence window."`,
		`"canonicalId", "resourceType", "resourceName", "activeFindings", "pendingApprovals", "recentActions", "contextSections", "generatedAt"`,
		`"canonicalId", "intentionallyOffline", "neverAutoRemediate", "setAt"`,
		`ActionIDArgumentName, RequestIDArgumentName, "allowed", "requiresApproval", "approvalPolicy", "rollbackAvailable", "plannedAt", "expiresAt", "resourceVersion", "policyVersion", "planHash"`,
	}
	for _, fragment := range required {
		if !strings.Contains(src, fragment) {
			t.Errorf("agent capabilities manifest must declare structured output schema fragment %s", fragment)
		}
	}
}

// TestContract_AgentResourceContextEndpointSurfacesStableShape pins
// the agent-paradigm substrate contract: the bundled endpoint must
// return arrays (never null) for activeFindings and recentActions so
// agents can iterate without nil-checking, must omit operatorState
// when no entry exists so agents can branch on field presence, and
// must preserve refusal-token prefixes verbatim so agents can branch
// on the stable code without parsing human messages. Without this
// pin, future refactors of the projection helpers could silently
// drift the wire shape and break external agent consumers.
func TestContract_AgentResourceContextEndpointSurfacesStableShape(t *testing.T) {
	source, err := os.ReadFile("agent_resource_context.go")
	if err != nil {
		t.Fatalf("read agent_resource_context.go: %v", err)
	}
	src := string(source)
	// Always-array initialization for the iteration-safe contract.
	if !regexp.MustCompile(`ActiveFindings:\s+\[\]AgentResourceFindingSnapshot\{\},`).MatchString(src) {
		t.Error("activeFindings must default to an empty slice, not nil — agents iterate without nil-checks")
	}
	if !regexp.MustCompile(`RecentActions:\s+\[\]AgentResourceActionSummary\{\},`).MatchString(src) {
		t.Error("recentActions must default to an empty slice, not nil")
	}
	// Server-computed maintenance-active flag — server pre-computes so
	// agents don't re-evaluate timestamps client-side.
	if !strings.Contains(src, "MaintenanceWindowActive: state.IsInMaintenanceAt(now)") {
		t.Error("MaintenanceWindowActive must be computed server-side via state.IsInMaintenanceAt(now)")
	}
	// Refusal-token preservation: ErrorMessage flows through verbatim
	// from the audit record's ExecutionResult, no rewriting.
	if !strings.Contains(src, "summary.ErrorMessage = audit.Result.ErrorMessage") {
		t.Error("audit ErrorMessage must round-trip verbatim so refusal tokens (resource_remediation_locked:, plan_drift:) reach agents")
	}
	// pendingApprovals must initialize as an empty slice so agents
	// iterate without nil-checking — same iteration-safe contract
	// as activeFindings and recentActions.
	if !regexp.MustCompile(`PendingApprovals:\s+\[\]AgentResourceApprovalSummary\{\},`).MatchString(src) {
		t.Error("pendingApprovals must default to an empty slice, not nil — agents iterate without nil-checks")
	}
	if !regexp.MustCompile(`ContextSections:\s+\[\]AgentResourceContextSection\{\},`).MatchString(src) {
		t.Error("contextSections must default to an empty slice, not nil — resource-aware agents iterate without nil-checks")
	}

	contextType := reflect.TypeOf(AgentResourceContext{})
	operatorStateField, ok := contextType.FieldByName("OperatorState")
	if !ok {
		t.Fatal("AgentResourceContext must carry OperatorState")
	}
	if operatorStateField.Type != reflect.TypeOf((*AgentResourceOperatorState)(nil)) ||
		operatorStateField.Tag.Get("json") != "operatorState,omitempty" {
		t.Error("operatorState must be an omitempty pointer so absent entries surface as a missing field")
	}
	discoveryReadinessField, ok := contextType.FieldByName("DiscoveryReadiness")
	if !ok {
		t.Fatal("AgentResourceContext must carry DiscoveryReadiness")
	}
	if discoveryReadinessField.Type != reflect.TypeOf((*AgentResourceDiscoveryReadiness)(nil)) ||
		discoveryReadinessField.Tag.Get("json") != "discoveryReadiness,omitempty" {
		t.Error("discoveryReadiness must be an omitempty pointer to the shared resource readiness projection")
	}
	pendingApprovalsField, ok := contextType.FieldByName("PendingApprovals")
	if !ok {
		t.Fatal("AgentResourceContext must carry PendingApprovals")
	}
	if pendingApprovalsField.Type != reflect.TypeOf([]AgentResourceApprovalSummary{}) ||
		pendingApprovalsField.Tag.Get("json") != "pendingApprovals" {
		t.Error("AgentResourceContext must carry PendingApprovals as a stable []AgentResourceApprovalSummary field")
	}
	contextSectionsField, ok := contextType.FieldByName("ContextSections")
	if !ok {
		t.Fatal("AgentResourceContext must carry ContextSections")
	}
	if contextSectionsField.Type != reflect.TypeOf([]AgentResourceContextSection{}) ||
		contextSectionsField.Tag.Get("json") != "contextSections" {
		t.Error("AgentResourceContext must carry ContextSections as a stable []AgentResourceContextSection field")
	}
	if !strings.Contains(src, "type AgentResourceContextRedaction = agentcontext.Redaction") ||
		!strings.Contains(src, "type AgentResourceContextFact = agentcontext.Fact") {
		t.Error("AgentResourceContext context sections must expose the shared typed fact and redaction metadata")
	}
}

// TestContract_AgentResourceContextWiresApprovalsProvider pins the
// startup-time wire-up that lets the bundle endpoint surface pending
// approvals scoped to a resource. Drift here means the field always
// shows up as an empty array even when there ARE pending approvals,
// silently breaking the substrate's "everything an agent needs in
// one read" contract for the governance dimension.
//
// The wire-up has two halves: router.go installs the provider with
// a provider that delegates to named functions in
// agent_resource_context.go, and those functions apply the org and
// resource filters while supporting the fleet endpoint's bulk count
// projection. Both halves are pinned here so a refactor that moves
// the filter logic still passes the test only if the resulting code
// preserves the safety and scale properties.
func TestContract_AgentResourceContextWiresApprovalsProvider(t *testing.T) {
	router, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	if !strings.Contains(string(router), "r.agentContextHandler.SetApprovalsProvider(agentApprovalStoreProvider{})") {
		t.Error("router.go must wire the approvals provider on the agent context handler so the bundle's pendingApprovals section is populated")
	}
	if !strings.Contains(string(router), "approval.SetStore") {
		t.Error("router.go must keep documenting that the agent approval provider resolves the process-global approval store at request time")
	}

	bundle, err := os.ReadFile("agent_resource_context.go")
	if err != nil {
		t.Fatalf("read agent_resource_context.go: %v", err)
	}
	if !strings.Contains(string(bundle), "type agentApprovalStoreProvider struct{}") {
		t.Error("agent_resource_context.go must define the request-time approval store provider that router.go wires")
	}
	if !strings.Contains(string(bundle), "return pendingApprovalsForResourceFromStore(approval.GetStore(), resourceID, orgID)") {
		t.Error("agentApprovalStoreProvider must resolve the approval store at request time for per-resource context reads")
	}
	if !strings.Contains(string(bundle), "return pendingApprovalCountsByResourceFromStore(approval.GetStore(), orgID)") {
		t.Error("agentApprovalStoreProvider must use the bulk count helper for fleet context reads")
	}
	if !strings.Contains(string(bundle), "func pendingApprovalsForResourceFromStore(") {
		t.Error("agent_resource_context.go must define pendingApprovalsForResourceFromStore as the named, testable filter — drift here breaks the cross-org / cross-resource isolation pins")
	}
	if !strings.Contains(string(bundle), "func pendingApprovalCountsByResourceFromStore(") {
		t.Error("agent_resource_context.go must define pendingApprovalCountsByResourceFromStore so fleet context counts are grouped by one bounded pending-approval scan")
	}
	if !strings.Contains(string(bundle), "if !approval.BelongsToOrg(req, orgID)") {
		t.Error("the named filters must scope pending approvals via BelongsToOrg so cross-tenant requests don't leak into an agent context bundle")
	}
	if !strings.Contains(string(bundle), "if req.CanonicalResourceID() != resourceID") {
		t.Error("the named filter must filter pending approvals by CanonicalResourceID so the bundle only carries approvals targeting the requested resource")
	}
	if !strings.Contains(string(bundle), "counts[resourceID]++") {
		t.Error("the bulk count helper must group pending approvals by canonical resource id instead of forcing fleet context into per-resource scans")
	}
}

// TestContract_GetResourceContextCapabilityListsPendingApprovals pins
// the discovery contract: the capabilities manifest must mention
// pendingApprovals under get_resource_context so an agent reading
// the manifest learns the bundle now carries the governance
// dimension. Drift here means external agents still expect the
// pre-slice-45 shape.
func TestContract_GetResourceContextCapabilityListsPendingApprovals(t *testing.T) {
	src := readAgentCapabilitiesManifestSource(t)
	if !strings.Contains(src, "pending approvals scoped to this resource") {
		t.Error("get_resource_context description must mention pending approvals so agents discover the bundle's new governance section through the manifest")
	}
}

// TestContract_ListResourceCapabilitiesCapabilitySurfacesStructuredParams pins
// the canonical agent-surface contract for action planning: agents must be able
// to discover a resource's governed capabilities and their parameter schemas
// through a dedicated structured tool, not only as the count-limited prose
// summary inside the resource-context bundle (formatCapabilityFact omits
// parameter schemas). Without this tool, agents guess plan_action inputs.
func TestContract_ListResourceCapabilitiesCapabilitySurfacesStructuredParams(t *testing.T) {
	manifest := agentcapabilities.CanonicalManifest()
	cap, ok := agentcapabilities.FindCapability(manifest.Capabilities, agentcapabilities.ListResourceCapabilitiesCapabilityName)
	if !ok {
		t.Fatalf("manifest missing %s", agentcapabilities.ListResourceCapabilitiesCapabilityName)
	}
	if cap.Method != http.MethodGet || cap.Scope != config.ScopeMonitoringRead {
		t.Fatalf("%s must be a GET under monitoring:read so any read-only agent can plan actions; got method=%s scope=%s",
			agentcapabilities.ListResourceCapabilitiesCapabilityName, cap.Method, cap.Scope)
	}
	if cap.Path != agentcapabilities.ListResourceCapabilitiesCapabilityPath {
		t.Fatalf("%s path = %q, want %q", agentcapabilities.ListResourceCapabilitiesCapabilityName, cap.Path, agentcapabilities.ListResourceCapabilitiesCapabilityPath)
	}
	if len(cap.ErrorCodes) == 0 || cap.ErrorCodes[0] != agentcapabilities.AgentErrCodeResourceNotFound {
		t.Fatalf("%s must declare resource_not_found so agents branch cleanly on unknown ids", agentcapabilities.ListResourceCapabilitiesCapabilityName)
	}
	// The capability must be projected to the MCP surface; otherwise external
	// agents (the primary consumers of plan_action) cannot reach it.
	found := false
	for _, name := range agentcapabilities.ManifestSurfaceToolCapabilities(manifest, agentcapabilities.SurfaceIDPulseMCP) {
		if name.Name == agentcapabilities.ListResourceCapabilitiesCapabilityName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("%s must be projected to the Pulse MCP surface", agentcapabilities.ListResourceCapabilitiesCapabilityName)
	}

	// The handler must return a non-nil capabilities array (never null) so an
	// agent can branch on len() without nil-checking — the same wire-shape rule
	// the resource-context and fleet-context bundles follow.
	handlerBytes, err := os.ReadFile("agent_resource_context.go")
	if err != nil {
		t.Fatalf("read agent_resource_context.go: %v", err)
	}
	handlerSrc := string(handlerBytes)
	if !strings.Contains(handlerSrc, "capabilities := []unified.ResourceCapability{}") {
		t.Errorf("%s handler must initialize capabilities as a non-nil empty array so the wire shape is always []", agentcapabilities.ListResourceCapabilitiesCapabilityName)
	}
}

func TestContract_AgentCommandPayloadsRequireActionScope(t *testing.T) {
	source, err := os.ReadFile("agent_command_redaction.go")
	if err != nil {
		t.Fatalf("read agent_command_redaction.go: %v", err)
	}
	src := string(source)
	if !strings.Contains(src, "requestCanReadAgentCommandPayloads") {
		t.Error("agent command redaction must stay behind a named request-scope helper")
	}
	if !strings.Contains(src, "token.HasScope(config.ScopeAIExecute)") {
		t.Error("agent command payloads must require ai:execute on API-token callers, not only monitoring:read")
	}
	if !strings.Contains(src, "redactAgentEventCommandsForRequest") {
		t.Error("agent SSE events must pass through command redaction before serialization")
	}
	if !strings.Contains(src, "redactAgentResourceContextCommandsForRequest") {
		t.Error("agent resource-context bundles must pass through command redaction before serialization")
	}
}

// TestContract_AgentFleetContextEndpointSurfacesStableShape pins the
// agent-paradigm fleet view contract: the endpoint must return a
// `resources` array (never null) so agents iterate without
// nil-checking, must initialize per-severity counts as a struct (not
// a map) so agents branch on stable field names, and must carry the
// operator-intent flags (intentionallyOffline, neverAutoRemediate,
// maintenanceWindowActive) that gate auto-action. Drift here would
// silently break the substrate's "where do I focus?" answer.
func TestContract_AgentFleetContextEndpointSurfacesStableShape(t *testing.T) {
	source, err := os.ReadFile("agent_resource_context.go")
	if err != nil {
		t.Fatalf("read agent_resource_context.go: %v", err)
	}
	src := string(source)
	if !strings.Contains(src, "Resources:   make([]AgentFleetResourceSummary, 0, len(resources))") {
		t.Error("AgentFleetContext.Resources must default to a non-nil empty slice — agents iterate without nil-checks")
	}
	if !strings.Contains(src, "Resources   []AgentFleetResourceSummary `json:\"resources\"`") {
		t.Error("AgentFleetContext must carry Resources as a stable []AgentFleetResourceSummary field")
	}
	if !strings.Contains(src, "IntentionallyOffline    bool                    `json:\"intentionallyOffline\"`") {
		t.Error("AgentFleetResourceSummary must carry IntentionallyOffline so agents see operator lock at a glance")
	}
	if !strings.Contains(src, "NeverAutoRemediate      bool                    `json:\"neverAutoRemediate\"`") {
		t.Error("AgentFleetResourceSummary must carry NeverAutoRemediate")
	}
	if !strings.Contains(src, "MaintenanceWindowActive bool                    `json:\"maintenanceWindowActive\"`") {
		t.Error("AgentFleetResourceSummary must carry MaintenanceWindowActive — server pre-computes so agents don't re-evaluate timestamps client-side")
	}
	if !strings.Contains(src, "Findings                AgentFleetFindingCounts `json:\"findings\"`") {
		t.Error("AgentFleetResourceSummary must carry per-severity finding counts as a struct so agents branch on stable field names")
	}
	if !strings.Contains(src, "PendingApprovalCount    int                     `json:\"pendingApprovalCount\"`") {
		t.Error("AgentFleetResourceSummary must carry PendingApprovalCount so agents see governance-blocked resources at a glance")
	}
}

func TestContract_AgentFleetDiagnosticsEndpointSurfacesStableShape(t *testing.T) {
	handler, err := os.ReadFile("agent_fleet_doctor.go")
	if err != nil {
		t.Fatalf("read agent_fleet_doctor.go: %v", err)
	}
	router, err := os.ReadFile("router_routes_registration.go")
	if err != nil {
		t.Fatalf("read router_routes_registration.go: %v", err)
	}
	monitoringSource, err := os.ReadFile("../monitoring/agent_fleet_doctor.go")
	if err != nil {
		t.Fatalf("read monitoring agent_fleet_doctor.go: %v", err)
	}
	handlerSrc := string(handler)
	routerSrc := string(router)
	monitoringSrc := string(monitoringSource)

	if !strings.Contains(routerSrc, `"/api/agents/diagnostics"`) ||
		!strings.Contains(routerSrc, `RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.handleAgentFleetDiagnostics))`) {
		t.Error("agent fleet diagnostics route must remain admin settings:read only")
	}
	if !strings.Contains(handlerSrc, "GetAgentFleetDiagnostics(serverVersion, time.Now().UTC())") {
		t.Error("agent fleet diagnostics handler must delegate to the monitoring-owned read-only producer")
	}
	for _, required := range []string{
		"GeneratedAt   int64                       `json:\"generatedAt\"`",
		"ServerVersion string                      `json:\"serverVersion,omitempty\"`",
		"Summary       AgentFleetDiagnosticSummary `json:\"summary\"`",
		"Agents        []AgentFleetAgentDiagnostic `json:\"agents\"`",
		"Reasons                []AgentFleetDiagnosticReason `json:\"reasons\"`",
		"RepairActions          []AgentFleetDiagnosticRepair `json:\"repairActions,omitempty\"`",
	} {
		if !strings.Contains(monitoringSrc, required) {
			t.Errorf("agent fleet diagnostics payload missing stable field %q", required)
		}
	}
}

// TestContract_AgentOperationsLoopStatusEndpointSurfacesStableShape pins the
// content-safe loop-status wire shape external agents use before choosing
// fleet, resource, finding, or action tools.
func TestContract_AgentOperationsLoopStatusEndpointSurfacesStableShape(t *testing.T) {
	source, err := os.ReadFile("agent_resource_context.go")
	if err != nil {
		t.Fatalf("read agent_resource_context.go: %v", err)
	}
	src := string(source)
	required := []string{
		"Steps:              []AgentOperationsLoopStep{}",
		"NextAction                          string                    `json:\"nextAction\"`",
		"ProgressLabel                       string                    `json:\"progressLabel\"`",
		"Steps                               []AgentOperationsLoopStep `json:\"steps\"`",
		"PatrolEvidenceCount                 int                       `json:\"patrolEvidenceCount\"`",
		"PatrolIssueEvidenceCount            int                       `json:\"patrolIssueEvidenceCount\"`",
		"ActiveFindingCount                  int                       `json:\"activeFindingCount\"`",
		"PendingApprovalCount                int                       `json:\"pendingApprovalCount\"`",
		"GovernedActionCount                 int                       `json:\"governedActionCount\"`",
		"ApprovedDecisionCount               int                       `json:\"approvedDecisionCount\"`",
		"RejectedDecisionCount               int                       `json:\"rejectedDecisionCount\"`",
		"VerifiedOutcomeCount                int                       `json:\"verifiedOutcomeCount\"`",
		"OperationsLoopStarterCount          int                       `json:\"operationsLoopStarterCount\"`",
		"AssistantOperationsLoopStarterCount int                       `json:\"assistantOperationsLoopStarterCount\"`",
		"PatrolOperationsLoopStarterCount    int                       `json:\"patrolOperationsLoopStarterCount\"`",
		"PatrolControlLoopStarterCount       int                       `json:\"patrolControlOperationsLoopStarterCount\"`",
		"PatrolControlCompletedLoopCount     int                       `json:\"patrolControlCompletedOperationsLoopCount\"`",
		"PatrolControlResolvedLoopCount      int                       `json:\"patrolControlResolvedOperationsLoopCount\"`",
		"PatrolControlValueState             string                    `json:\"patrolControlValueState\"`",
		"PatrolAutonomyLoopStarterCount      int                       `json:\"patrolAutonomyOperationsLoopStarterCount\"`",
		"PatrolAutonomyCompletedLoopCount    int                       `json:\"patrolAutonomyCompletedOperationsLoopCount\"`",
		"PatrolAutonomyResolvedLoopCount     int                       `json:\"patrolAutonomyResolvedOperationsLoopCount\"`",
		"PatrolAutonomyValueState            string                    `json:\"patrolAutonomyValueState\"`",
		"ProActivationLoopStarterCount       int                       `json:\"proActivationOperationsLoopStarterCount\"`",
		"ProActivationCompletedLoopCount     int                       `json:\"proActivationCompletedOperationsLoopCount\"`",
		"ProActivationResolvedLoopCount      int                       `json:\"proActivationResolvedOperationsLoopCount\"`",
		"ProActivationValueProofState        string                    `json:\"proActivationValueProofState\"`",
		"MCPOperationsLoopStarterCount       int                       `json:\"mcpOperationsLoopStarterCount\"`",
		"ExternalAgentReady                  bool                      `json:\"externalAgentReady\"`",
		"WindowStart                         time.Time                 `json:\"windowStart\"`",
		"GeneratedAt                         time.Time                 `json:\"generatedAt\"`",
		"External-agent readiness is a separate capability signal,",
		"manifest := agentcapabilities.CanonicalManifest()",
		"ExternalAgentReady: h.agentOperationsLoopExternalAgentReady(r.Context(), manifest, now)",
		"func agentOperationsLoopExternalAgentTokenReady(manifest agentcapabilities.Manifest, tokens []config.APITokenRecord, now time.Time) bool",
		"agentcapabilities.RequiredCapabilityScopes(",
		"coversLoop := true",
		"if !token.HasScope(scope) {",
		`store.GetActionLifecycleEvents("", since, 0)`,
		"return pulseIntelligenceActionWasApproved(record) || pulseIntelligenceActionWasRejected(record)",
		"return pulseIntelligenceActionVerifiedOutcome(record)",
		"governanceStepCount := status.PendingApprovalCount",
		"decisionCount := status.ApprovedDecisionCount + status.RejectedDecisionCount",
		"verificationStepCount := status.VerifiedOutcomeCount",
		"verificationStepCount = status.RejectedDecisionCount",
		"patrolControlProof := agentOperationsLoopPatrolControlProof(status)",
		"status.PatrolControlLoopStarterCount = starterCounts.patrolControl",
		"status.PatrolAutonomyLoopStarterCount = status.PatrolControlLoopStarterCount",
		"status.PatrolControlValueState = patrolControlProof.ValueProofState",
		"status.PatrolAutonomyValueState = status.PatrolControlValueState",
		"status.ProActivationValueProofState = status.PatrolControlValueState",
		"case config.WorkflowPromptActivitySurfacePatrolControl:",
		"case config.WorkflowPromptActivitySurfacePatrolAutonomy:",
		"counts.patrolControl++",
		"return telemetry.ClassifyPulseIntelligencePatrolControlProof(telemetry.PulseIntelligencePatrolControlProofInput{",
		"ContextualCollaborationCount: agentOperationsLoopContextualCollaborationCount(status)",
		"type AgentAggregateFindingsProvider interface {",
		"ActiveFindingCount() int",
		"if aggregateProvider, ok := h.findingsProvider.(AgentAggregateFindingsProvider); ok {",
		"aggregateProvider.ActiveFindingCount()",
		"case status.PendingApprovalCount > 0:",
		"Review pending Patrol approvals before treating previous verified work as current.",
		"case status.ActiveFindingCount > 0 && hasVerifiedOutcome:",
		"Open Assistant on the active Patrol finding before treating previous verified work as current.",
		"Review active Patrol findings before treating previous verified work as current.",
	}
	for _, fragment := range required {
		if !strings.Contains(src, fragment) {
			t.Errorf("AgentOperationsLoopStatus contract missing %s", fragment)
		}
	}
	if strings.Contains(src, "ExternalAgentReady:           status.ExternalAgentReady") {
		t.Error("AgentOperationsLoopStatus must not pass MCP readiness into Patrol control proof classification")
	}
	if strings.Contains(src, `ID: "external_agents"`) {
		t.Error("AgentOperationsLoopStatus steps must not make optional MCP readiness a Patrol operator step")
	}
}

// TestContract_ActionCompletedPayloadCarriesVerification pins that
// the action.completed SSE payload exposes the verification block
// — the agent-stable projection of the post-execution probe so
// agents close the "did it actually work?" loop without a follow-up
// audit fetch. Drift here forces every agent watching the stream to
// poll /api/actions/{id} after every dispatch, defeating the
// substrate's push-notification guarantee for dispatch certainty.
func TestContract_ActionCompletedPayloadCarriesVerification(t *testing.T) {
	source, err := os.ReadFile("agent_events.go")
	if err != nil {
		t.Fatalf("read agent_events.go: %v", err)
	}
	src := string(source)
	if !strings.Contains(src, "Verification    *AgentResourceActionVerification `json:\"verification,omitempty\"`") {
		t.Error("AgentEventActionCompletedPayload must carry Verification as a *AgentResourceActionVerification — agents close the certainty loop on this field")
	}
}

// TestContract_RouterBridgesVerificationOntoActionCompleted pins
// that the router-side bridge actually carries the verification
// projection from the canonical top-level verification helper onto
// the SSE payload.
// The payload field exists is one half of the contract; this is the
// other half — that the bridge populates it when the underlying
// audit has a verification result.
func TestContract_RouterBridgesVerificationOntoActionCompleted(t *testing.T) {
	source, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	src := string(source)
	if !strings.Contains(src, "if v := projectAgentResourceVerification(unifiedresources.CanonicalActionVerification(record)); v != nil {") {
		t.Error("router.go must project canonical action verification onto the action.completed payload via projectAgentResourceVerification — drift here means verification reaches the audit store but never the SSE stream")
	}
	if !strings.Contains(src, "payload.Verification = v") {
		t.Error("router.go must assign the projected verification onto payload.Verification")
	}
}

// TestContract_AgentResourceActionSummaryCarriesVerification pins
// that the resource-context bundle's recent-actions surface also
// carries verification, paralleling the SSE payload. Symmetry
// here matters: an agent should get the same verification fact
// whether they read the bundle (depth) or watch the doorbell
// (push). Drift would let one surface ship a feature the other
// doesn't.
func TestContract_AgentResourceActionSummaryCarriesVerification(t *testing.T) {
	source, err := os.ReadFile("agent_resource_context.go")
	if err != nil {
		t.Fatalf("read agent_resource_context.go: %v", err)
	}
	src := string(source)
	if !strings.Contains(src, "Verification    *AgentResourceActionVerification `json:\"verification,omitempty\"`") {
		t.Error("AgentResourceActionSummary must carry Verification so the bundle's recent-actions and the action.completed SSE payload speak the same vocabulary")
	}
	if !strings.Contains(src, "func projectAgentResourceVerification(v *unified.ActionVerificationResult)") {
		t.Error("projectAgentResourceVerification must exist as the shared helper — both the SSE bridge and the bundle's projector route through it so the wire shape cannot drift between surfaces")
	}
	if !strings.Contains(src, "projectAgentResourceVerification(unified.CanonicalActionVerification(audit))") {
		t.Error("AgentResourceActionSummary must project canonical action verification, not only legacy result.verification")
	}
}

// TestContract_PulseIntelligenceApprovedExecutionTelemetryUsesActionLifecycle
// pins that the paid-value approved execution counter is sourced from
// action lifecycle evidence and only then resolved back to the action audit
// approval state. Drift here would let generic planning, approval, or token
// use masquerade as a completed operations loop.
func TestContract_PulseIntelligenceApprovedExecutionTelemetryUsesActionLifecycle(t *testing.T) {
	source, err := os.ReadFile("telemetry_pulse_intelligence.go")
	if err != nil {
		t.Fatalf("read telemetry_pulse_intelligence.go: %v", err)
	}
	src := string(source)
	for _, fragment := range []string{
		`store.GetActionLifecycleEvents("", since, 0)`,
		`store.GetActionAudit(actionID)`,
		`pulseIntelligenceActionWasApproved(record)`,
		`pulseIntelligenceApprovedActionSuccess(record)`,
		`case unifiedresources.ActionStateExecuting, unifiedresources.ActionStateCompleted, unifiedresources.ActionStateFailed:`,
		`snapshot.ApprovedActionAttempts30d += len(approvedAttemptIDs)`,
		`snapshot.ApprovedActionSuccesses30d += len(approvedSuccessIDs)`,
	} {
		if !strings.Contains(src, fragment) {
			t.Errorf("Pulse Intelligence approved execution telemetry must stay lifecycle-backed and audit-approved; missing %s", fragment)
		}
	}
}

// TestContract_PulseIntelligenceApprovedDecisionTelemetryUsesActionDecisionEvidence
// pins that approved governed-action decisions are counted separately from
// approved execution attempts, so completed operations-loop proof can mean an
// approve/reject decision without pretending every approval executed.
func TestContract_PulseIntelligenceApprovedDecisionTelemetryUsesActionDecisionEvidence(t *testing.T) {
	source, err := os.ReadFile("telemetry_pulse_intelligence.go")
	if err != nil {
		t.Fatalf("read telemetry_pulse_intelligence.go: %v", err)
	}
	src := string(source)
	for _, fragment := range []string{
		`approvedDecisionIDs := pulseIntelligenceApprovedActionDecisionIDs(store, orgID, since)`,
		`if event.State != unifiedresources.ActionStateApproved`,
		`pulseIntelligenceActionWasApprovedSince(record, since)`,
		`if approval.Outcome != unifiedresources.OutcomeApproved`,
		`snapshot.ApprovedActionDecisions30d += len(approvedDecisionIDs)`,
	} {
		if !strings.Contains(src, fragment) {
			t.Errorf("Pulse Intelligence approved decision telemetry must stay decision-backed and separate from execution attempts; missing %s", fragment)
		}
	}
}

// TestContract_PulseIntelligenceApprovedSuccessTelemetryRequiresVerifiedOutcome
// pins that outbound Pulse Intelligence approved-success telemetry and the
// operations-loop status route share the same verified-outcome predicate.
// A completed action result alone is execution evidence, not post-action proof
// for Patrol control resolved-loop reporting.
func TestContract_PulseIntelligenceApprovedSuccessTelemetryRequiresVerifiedOutcome(t *testing.T) {
	telemetrySource, err := os.ReadFile("telemetry_pulse_intelligence.go")
	if err != nil {
		t.Fatalf("read telemetry_pulse_intelligence.go: %v", err)
	}
	telemetrySrc := string(telemetrySource)
	for _, fragment := range []string{
		`if record.State != unifiedresources.ActionStateCompleted`,
		`return pulseIntelligenceActionVerifiedOutcome(record)`,
		`func pulseIntelligenceActionVerifiedOutcome(record unifiedresources.ActionAuditRecord) bool`,
		`outcome := unifiedresources.NormalizeVerificationOutcome(record.VerificationOutcome)`,
		`outcome.Status == unifiedresources.VerificationVerified`,
		`verification := unifiedresources.CanonicalActionVerification(record)`,
		`return verification != nil && verification.Ran && verification.Success`,
	} {
		if !strings.Contains(telemetrySrc, fragment) {
			t.Errorf("Pulse Intelligence approved-success telemetry must require verified outcome proof; missing %s", fragment)
		}
	}
	if strings.Contains(telemetrySrc, `return record.Result == nil || record.Result.Success`) {
		t.Error("Pulse Intelligence approved-success telemetry must not treat a bare successful execution result as verified outcome proof")
	}

	statusSource, err := os.ReadFile("agent_resource_context.go")
	if err != nil {
		t.Fatalf("read agent_resource_context.go: %v", err)
	}
	if !strings.Contains(string(statusSource), `return pulseIntelligenceActionVerifiedOutcome(record)`) {
		t.Error("operations-loop status verified outcome count must use the shared Pulse Intelligence verified-outcome predicate")
	}
}

// TestContract_PulseIntelligenceRejectedDecisionTelemetryUsesActionLifecycle
// pins that rejected governed-action decisions are counted from decision
// lifecycle evidence and remain separate from approved execution proof.
func TestContract_PulseIntelligenceRejectedDecisionTelemetryUsesActionLifecycle(t *testing.T) {
	source, err := os.ReadFile("telemetry_pulse_intelligence.go")
	if err != nil {
		t.Fatalf("read telemetry_pulse_intelligence.go: %v", err)
	}
	src := string(source)
	for _, fragment := range []string{
		`rejectedDecisionIDs := pulseIntelligenceRejectedActionDecisionIDs(store, orgID, since)`,
		`if event.State != unifiedresources.ActionStateRejected`,
		`store.GetActionAudit(actionID)`,
		`pulseIntelligenceActionWasRejected(record)`,
		`snapshot.RejectedActionDecisions30d += len(rejectedDecisionIDs)`,
	} {
		if !strings.Contains(src, fragment) {
			t.Errorf("Pulse Intelligence rejected decision telemetry must stay lifecycle-backed and audit-rejected; missing %s", fragment)
		}
	}
}

// TestContract_OperatorStateWriteServerPopulatesAttribution pins the
// audit-honesty contract for the agent surface's only write loop:
// the PUT handler must populate SetAt and SetBy from the server,
// ignoring any client-supplied values. Drift here would let an
// agent (or a misconfigured client) spoof attribution timestamps,
// breaking the audit trail's trust model.
func TestContract_OperatorStateWriteServerPopulatesAttribution(t *testing.T) {
	source, err := os.ReadFile("resources_operator_state.go")
	if err != nil {
		t.Fatalf("read resources_operator_state.go: %v", err)
	}
	src := string(source)
	if !strings.Contains(src, "SetAt: time.Now().UTC(),") {
		t.Error("operator-state PUT must populate SetAt with the server clock — client cannot spoof attribution")
	}
	if !strings.Contains(src, "SetBy: getUserID(r),") {
		t.Error("operator-state PUT must populate SetBy from the authenticated identity — client cannot spoof who-did-it")
	}
	if !strings.Contains(src, "// canonical_id from URL wins over body") {
		t.Error("operator-state PUT must take the canonical id from the URL, not the body — drift would let scope-confusion writes succeed")
	}
}

// TestContract_OperatorStateWriteEmitsStableErrorTokens pins that
// the only two error codes the manifest declares for set/get
// operator-state actually reach the wire from the handler. Drift
// here means external agents branching on operator_state_not_set
// or operator_state_invalid would silently miss the relevant
// failures — manifest claims a code that the handler never emits.
func TestContract_OperatorStateWriteEmitsStableErrorTokens(t *testing.T) {
	source, err := os.ReadFile("resources_operator_state.go")
	if err != nil {
		t.Fatalf("read resources_operator_state.go: %v", err)
	}
	src := string(source)
	if !strings.Contains(src, "agentcapabilities.AgentErrCodeOperatorStateNotSet") {
		t.Error("GET handler must emit operator_state_not_set when no entry exists — manifest declares this code")
	}
	if !strings.Contains(src, "agentcapabilities.AgentErrCodeOperatorStateInvalid") {
		t.Error("PUT handler must emit operator_state_invalid on validation failure — manifest declares this code")
	}
	if !strings.Contains(src, "errors.Is(err, unified.ErrResourceOperatorStateInvalid)") {
		t.Error("PUT handler must branch on ErrResourceOperatorStateInvalid so domain validation errors map to the stable wire token — string-matching the error message would drift")
	}
}

// TestContract_AgentSurfaceErrorCodesMatchManifestDeclarations pins
// the symmetry between what handlers actually emit and what the
// capabilities manifest declares. The doc says the manifest's
// errorCodes list is "the closed set of values the error field may
// carry on failure"; this test enforces that claim by reading every
// writeJSONError call from the two agent-surface handler files and
// asserting each emitted code is either (a) declared by the matching
// capability, or (b) one of the three cross-cutting codes the auth
// middleware emits universally. Drift in either direction is a
// contract regression — emitting an undeclared code silently breaks
// agents that branch on the closed set; declaring a code the
// handler never emits misleads agents into writing dead-code paths.
func TestContract_AgentSurfaceErrorCodesMatchManifestDeclarations(t *testing.T) {
	// Codes emitted by the multi-tenant / auth middleware that apply
	// to every authenticated endpoint and are intentionally not
	// duplicated on per-capability errorCodes lists. Documented in
	// api-contracts.md "cross-cutting codes" paragraph.
	crossCutting := map[string]bool{
		"invalid_org":   true,
		"org_suspended": true,
		"access_denied": true,
	}

	// Handler files that back agent-surface capabilities. If a future
	// capability adds a new file, append it here so the audit follows.
	handlerFiles := []string{
		"agent_resource_context.go",
		"resources_operator_state.go",
		"actions.go",
		"ai_handlers.go",
	}

	// Codes the action handlers emit that are deliberately NOT in
	// the per-capability manifest entries: 5xx internal-failure
	// codes (audit-store outages, encode failures) and codes that
	// surface from edge paths the manifest declares against a
	// different capability. Agents branch on 5xx generically; they
	// don't need the specific token. Whitelisted here so the
	// emit/declare audit doesn't false-positive on them.
	internalOnlyCodes := map[string]bool{
		"action_audit_unavailable":        true,
		"action_audit_persist_failed":     true,
		"action_plan_failed":              true,
		"action_plan_encode_failed":       true,
		"action_audit_query_failed":       true,
		"action_decision_persist_failed":  true,
		"action_decision_encode_failed":   true,
		"action_decision_failed":          true,
		"action_execution_persist_failed": true,
		"action_execution_encode_failed":  true,
		"action_execution_failed":         true,
		"action_not_executing":            true,
		"action_policy_validation_failed": true,
		"action_plan_validation_failed":   true,
		"resource_registry_unavailable":   true,
	}

	agentErrorConstantValues := map[string]string{
		"AgentErrCodeResourceNotFound":           agentcapabilities.AgentErrCodeResourceNotFound,
		"AgentErrCodeOperatorStateNotSet":        agentcapabilities.AgentErrCodeOperatorStateNotSet,
		"AgentErrCodeOperatorStateInvalid":       agentcapabilities.AgentErrCodeOperatorStateInvalid,
		"AgentErrCodeInvalidFindingRequest":      agentcapabilities.AgentErrCodeInvalidFindingRequest,
		"AgentErrCodeFindingNotFound":            agentcapabilities.AgentErrCodeFindingNotFound,
		"AgentErrCodeFindingActionNotAllowed":    agentcapabilities.AgentErrCodeFindingActionNotAllowed,
		"AgentErrCodePatrolUnavailable":          agentcapabilities.AgentErrCodePatrolUnavailable,
		"AgentErrCodeInvalidActionRequest":       agentcapabilities.AgentErrCodeInvalidActionRequest,
		"AgentErrCodeCapabilityNotFound":         agentcapabilities.AgentErrCodeCapabilityNotFound,
		"AgentErrCodeActionExecutionUnavailable": agentcapabilities.AgentErrCodeActionExecutionUnavailable,
		"AgentErrCodeMissingID":                  agentcapabilities.AgentErrCodeMissingID,
		"AgentErrCodeInvalidID":                  agentcapabilities.AgentErrCodeInvalidID,
		"AgentErrCodeInvalidActionDecision":      agentcapabilities.AgentErrCodeInvalidActionDecision,
		"AgentErrCodeActionNotFound":             agentcapabilities.AgentErrCodeActionNotFound,
		"AgentErrCodeActionNotPending":           agentcapabilities.AgentErrCodeActionNotPending,
		"AgentErrCodeActionPlanExpired":          agentcapabilities.AgentErrCodeActionPlanExpired,
		"AgentErrCodeInvalidActionExecution":     agentcapabilities.AgentErrCodeInvalidActionExecution,
		"AgentErrCodeActionNotApproved":          agentcapabilities.AgentErrCodeActionNotApproved,
		"AgentErrCodeActionAlreadyExecuting":     agentcapabilities.AgentErrCodeActionAlreadyExecuting,
		"AgentErrCodeActionExecutionFinal":       agentcapabilities.AgentErrCodeActionExecutionFinal,
		"AgentErrCodeActionDryRunOnly":           agentcapabilities.AgentErrCodeActionDryRunOnly,
		"AgentErrCodeActionPlanDrift":            agentcapabilities.AgentErrCodeActionPlanDrift,
		"AgentErrCodeResourceRemediationLocked":  agentcapabilities.AgentErrCodeResourceRemediationLocked,
		"AgentErrCodeActionExecutorUnavailable":  agentcapabilities.AgentErrCodeActionExecutorUnavailable,
	}

	// Extract every emitted shared code from writeJSONError /
	// writeJSONErrorWithDetails calls. Agent-surface codes must flow
	// through agentcapabilities.AgentErrCode* constants so the manifest and
	// handlers cannot drift by editing duplicate literals.
	emitted := map[string]bool{}
	for _, file := range handlerFiles {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		constantRE := regexp.MustCompile(`(?s)writeJSONError(?:WithDetails)?\(\s*w\s*,\s*[^,]+,\s*agentcapabilities\.(AgentErrCode[A-Za-z0-9]+)`)
		for _, m := range constantRE.FindAllSubmatch(body, -1) {
			constantName := string(m[1])
			code, ok := agentErrorConstantValues[constantName]
			if !ok {
				t.Errorf("%s emits unknown shared agent error constant %s", file, constantName)
				continue
			}
			emitted[code] = true
		}

		literalRE := regexp.MustCompile(`(?s)writeJSONError(?:WithDetails)?\(\s*w\s*,\s*[^,]+,\s*"([a-z_][a-z_0-9]+)"`)
		for _, m := range literalRE.FindAllSubmatch(body, -1) {
			code := string(m[1])
			if internalOnlyCodes[code] || crossCutting[code] {
				continue
			}
			t.Errorf("%s emits agent-surface error code %q as a local literal; use an agentcapabilities.AgentErrCode* constant", file, code)
		}
	}
	if len(emitted) == 0 {
		t.Fatal("no shared writeJSONError emissions found in agent-surface handler files; the audit regex or handler constants must be wrong")
	}

	// Pull declared codes from the runtime manifest so the test follows
	// constant-backed manifest edits without source scraping.
	declared := map[string]bool{}
	for _, cap := range agentcapabilities.CanonicalManifest().Capabilities {
		for _, code := range cap.ErrorCodes {
			declared[code] = true
		}
	}

	// Every emitted code must be either declared somewhere in the
	// manifest OR a cross-cutting code.
	for code := range emitted {
		if declared[code] || crossCutting[code] {
			continue
		}
		t.Errorf("handler emits %q but no capability in the manifest declares it and it is not a documented cross-cutting code — drift here breaks agents that branch on the closed set", code)
	}

	// Every manifest-declared code must have a matching emission
	// somewhere in the agent-surface handlers, otherwise agents
	// follow a code path the substrate never walks.
	for code := range declared {
		if !emitted[code] {
			t.Errorf("manifest declares %q but no agent-surface handler emits it — declaring a dead code misleads agents into writing branches the substrate never triggers", code)
		}
	}
}

func TestContract_PlanActionDeclaresExecutionUnavailable(t *testing.T) {
	manifest := agentcapabilities.CanonicalManifest()
	var planAction agentcapabilities.Capability
	for _, cap := range manifest.Capabilities {
		if cap.Name == agentcapabilities.PlanActionCapabilityName {
			planAction = cap
			break
		}
	}
	if planAction.Name == "" {
		t.Fatalf("agent capabilities manifest must declare %s", agentcapabilities.PlanActionCapabilityName)
	}
	if !stringSliceContains(planAction.ErrorCodes, agentcapabilities.AgentErrCodeActionExecutionUnavailable) {
		t.Error("plan_action must declare action_execution_unavailable so agents can branch on executor-owned live-readiness refusal")
	}
}

func TestContract_ResourceActionReadinessPayloadShape(t *testing.T) {
	payload, err := json.Marshal(unifiedresources.Resource{
		ID:       "app-container:docker:web",
		Type:     unifiedresources.ResourceTypeAppContainer,
		Name:     "web",
		Status:   unifiedresources.StatusOnline,
		LastSeen: time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC),
		Sources:  []unifiedresources.DataSource{unifiedresources.SourceDocker},
		ActionReadiness: []unifiedresources.ResourceActionReadiness{
			{
				Name:       "restart",
				Available:  false,
				ReasonCode: "command_agent_disconnected",
				Reason:     "Docker / Podman command agent is not connected.",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal resource: %v", err)
	}
	body := string(payload)
	for _, want := range []string{
		`"actionReadiness":[`,
		`"name":"restart"`,
		`"available":false`,
		`"reasonCode":"command_agent_disconnected"`,
		`"reason":"Docker / Podman command agent is not connected."`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("resource action readiness payload missing %s: %s", want, body)
		}
	}
	if strings.Contains(body, "InternalHandler") {
		t.Fatalf("resource payload leaked internal action handler: %s", body)
	}
}

func TestContract_DockerLifecycleActionsResolveCommandAgentAndDispatchTrusted(t *testing.T) {
	source, err := os.ReadFile("docker_container_action_executor.go")
	if err != nil {
		t.Fatalf("read docker_container_action_executor.go: %v", err)
	}
	src := string(source)
	sharedSource, err := os.ReadFile("action_executor.go")
	if err != nil {
		t.Fatalf("read action_executor.go: %v", err)
	}
	sharedSrc := string(sharedSource)
	for _, snippet := range []string{
		"func (e dockerContainerActionExecutor) connectedDockerCommandAgentID(resource unified.Resource) (string, error)",
		"resource.Docker.AgentID",
		"e.agents.GetAgentForHost(strings.TrimSpace(resource.Docker.Hostname))",
		"Trusted:    true",
	} {
		if !strings.Contains(src, snippet) {
			t.Fatalf("docker lifecycle executor must pin command-agent/trusted dispatch snippet %q", snippet)
		}
	}
	if !strings.Contains(sharedSrc, "GetAgentForHost(hostname string) (string, bool)") {
		t.Fatal("shared action command interface must keep hostname-based command-agent resolution available")
	}
	if strings.Index(src, "if agentID := strings.TrimSpace(resource.Docker.AgentID)") >
		strings.Index(src, "e.agents.GetAgentForHost(strings.TrimSpace(resource.Docker.Hostname))") {
		t.Fatal("docker lifecycle executor must try the Docker reporting agent id before falling back to hostname resolution")
	}
}

func TestContract_ProxmoxLifecycleActionsResolveNodeCommandAgentAndVerifyState(t *testing.T) {
	source, err := os.ReadFile("proxmox_guest_action_executor.go")
	if err != nil {
		t.Fatalf("read proxmox_guest_action_executor.go: %v", err)
	}
	src := string(source)
	for _, snippet := range []string{
		"func (e proxmoxGuestActionExecutor) connectedProxmoxNodeCommandAgentID(resource unified.Resource) (string, error)",
		"resource.Proxmox.LinkedAgentID",
		"e.agents.GetAgentForHost(strings.TrimSpace(resource.Proxmox.NodeName))",
		"Trusted:    true",
		"func (e proxmoxGuestActionExecutor) verifyProxmoxGuestState(",
		"proxmoxGuestStatusCommand(kind, vmid)",
		"parseProxmoxGuestStatus(lastOutput) == expected",
	} {
		if !strings.Contains(src, snippet) {
			t.Fatalf("proxmox lifecycle executor must pin command-agent/trusted verification snippet %q", snippet)
		}
	}
	if strings.Index(src, "if agentID := strings.TrimSpace(resource.Proxmox.LinkedAgentID)") >
		strings.Index(src, "e.agents.GetAgentForHost(strings.TrimSpace(resource.Proxmox.NodeName))") {
		t.Fatal("proxmox lifecycle executor must try the linked Proxmox node agent before falling back to node hostname resolution")
	}

	router, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	routerSrc := string(router)
	for _, snippet := range []string{
		"newRoutedActionExecutor(",
		"newDockerContainerActionExecutor(r.resourceHandlers, r.agentExecServer)",
		"newProxmoxGuestActionExecutor(r.resourceHandlers, r.agentExecServer)",
	} {
		if !strings.Contains(routerSrc, snippet) {
			t.Fatalf("router must register Docker and Proxmox action executors through routed action executor; missing %q", snippet)
		}
	}
}

// TestContract_AgentCapabilitiesManifestIsPublic pins the auth
// contract for the discovery surface: the manifest must be in the
// router's publicPaths list so it serves without a token. The
// manifest's purpose is "let an agent introspect Pulse before it
// has credentials" — gating the manifest behind the same
// monitoring:read scope that gates everything else makes
// discovery a chicken-and-egg problem. Slice 40 had this wrong;
// slice 47 fixed it. Pin so it cannot regress silently.
func TestContract_AgentCapabilitiesManifestIsPublic(t *testing.T) {
	source, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	src := string(source)
	if !strings.Contains(src, "\"/api/agent/capabilities\",         // Agent-paradigm discovery manifest") {
		t.Error("router.go publicPaths must include /api/agent/capabilities so the discovery manifest serves without a token — agents need to introspect Pulse before they have credentials")
	}
}

// TestContract_GetFleetContextCapabilityListed pins the discovery
// contract: the capabilities manifest must declare get_fleet_context
// so an agent reading the manifest learns the fleet view exists and
// what its response shape is. Drift here means external agents
// still expect to walk every resource id one-by-one.
func TestContract_GetFleetContextCapabilityListed(t *testing.T) {
	var cap *AgentCapability
	manifest := agentcapabilities.CanonicalManifest()
	for i := range manifest.Capabilities {
		if manifest.Capabilities[i].Name == "get_fleet_context" {
			cap = &manifest.Capabilities[i]
			break
		}
	}
	if cap == nil {
		t.Fatal("agent capabilities manifest must declare get_fleet_context — the substrate's triage entry point")
	}
	if cap.Path != agentcapabilities.FleetContextCapabilityPath {
		t.Errorf("get_fleet_context path = %q, want %s", cap.Path, agentcapabilities.FleetContextCapabilityPath)
	}
	if cap.ResponseShape != "AgentFleetContext" {
		t.Errorf("get_fleet_context response shape = %q, want AgentFleetContext so agents know what to parse", cap.ResponseShape)
	}
}

// TestContract_GetFleetContextDeclaresAdditiveFilters pins the agent-surface
// contract for fleet triage at scale: the capability must declare optional
// additive filter arguments (hasFindings, severity, technology, resourceType)
// so an agent can narrow a large fleet to a relevant subset without paging,
// and the GET projection layer must forward non-path arguments as URL query
// parameters (not drop them, not body them). Without the inputSchema the
// filters are undiscoverable; without query forwarding they are unreachable
// through the MCP tools/call path.
func TestContract_GetFleetContextDeclaresAdditiveFilters(t *testing.T) {
	manifest := agentcapabilities.CanonicalManifest()
	cap, ok := agentcapabilities.FindCapability(manifest.Capabilities, agentcapabilities.FleetContextCapabilityName)
	if !ok {
		t.Fatalf("manifest missing %s", agentcapabilities.FleetContextCapabilityName)
	}
	if len(cap.InputSchema) == 0 {
		t.Fatalf("%s must declare an inputSchema so filter arguments are discoverable through the manifest", agentcapabilities.FleetContextCapabilityName)
	}
	var schema map[string]any
	if err := json.Unmarshal(cap.InputSchema, &schema); err != nil {
		t.Fatalf("%s inputSchema must be valid JSON: %v", agentcapabilities.FleetContextCapabilityName, err)
	}
	properties, _ := schema["properties"].(map[string]any)
	for _, filter := range []string{"hasFindings", "severity", "technology", "resourceType"} {
		if _, ok := properties[filter]; !ok {
			t.Errorf("%s inputSchema must declare filter %q; properties = %v", agentcapabilities.FleetContextCapabilityName, filter, properties)
		}
	}
	// None of the filters are required — omitting all returns the full fleet.
	if required, _ := schema["required"].([]any); len(required) != 0 {
		t.Errorf("%s filter args must all be optional (backward compat); required = %v", agentcapabilities.FleetContextCapabilityName, required)
	}

	// The GET projection must forward non-path arguments as query params. Pin
	// the contract end-to-end: a fleet-context call with filter args produces
	// a request whose RawQuery carries them.
	req, projected, err := agentcapabilities.BuildCapabilityHTTPRequest(
		context.Background(), "http://pulse.local/", "token", cap, map[string]any{
			"hasFindings": "true",
			"technology":  "docker",
		})
	if err != nil {
		t.Fatalf("BuildCapabilityHTTPRequest: %v", err)
	}
	if projected.Query == nil || projected.Query.Get("hasFindings") != "true" || projected.Query.Get("technology") != "docker" {
		t.Fatalf("GET projection must forward filter args as query params; projected = %+v", projected)
	}
	if req.URL.RawQuery == "" {
		t.Fatalf("filter args must reach the request URL; url = %q", req.URL.String())
	}
}

// TestContract_FindingsResourceOperatorStateProviderIsWired pins the
// startup wiring that gives the findings runtime access to
// per-resource operator-set state via the API layer's adapter. The
// router must call SetResourceOperatorStateProvider with a closure
// that reads the unified store's GetResourceOperatorState and
// projects IsInMaintenanceAt into the ActiveMaintenanceWindow shape;
// regressions here would silently disable maintenance-window
// auto-acknowledgement without breaking any unit tests because the
// behavior is opt-in.
func TestContract_FindingsResourceOperatorStateProviderIsWired(t *testing.T) {
	source, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	src := string(source)
	if !strings.Contains(src, "SetResourceOperatorStateProvider(") {
		t.Error("router.go must call SetResourceOperatorStateProvider on the findings store at startup")
	}
	if !strings.Contains(src, "SetResourceStoreProvider(r.resourceHandlers.getStore)") {
		t.Error("router.go must expose the resource store to the Patrol findings handler so read-time priority changes surface immediately")
	}
	if !strings.Contains(src, "ResourceOperatorStateProviderFunc(") {
		t.Error("router.go must use the ai.ResourceOperatorStateProviderFunc adapter so the closure satisfies the provider interface")
	}
	if !strings.Contains(src, "IsInMaintenanceAt(now)") {
		t.Error("router.go must gate the projection on state.IsInMaintenanceAt so a stale window does not return as active")
	}
	if !strings.Contains(src, "GetResourceOperatorState(canonicalID)") {
		t.Error("router.go must read state from the unified store's canonical accessor")
	}
	if !strings.Contains(src, "IntentionallyOffline: state.IntentionallyOffline") {
		t.Error("router.go must propagate state.IntentionallyOffline through the projection so the findings runtime sees it")
	}
	if !strings.Contains(src, "Criticality:          string(state.Criticality)") {
		t.Error("router.go must propagate state.Criticality through the projection so Patrol attention ordering sees it")
	}
}

// TestContract_ResourceOperatorStateUrlCanonicalIDWinsOverBody pins the
// security-relevant decision that the URL path's canonical_id always
// wins over any body-supplied value on PUT
// /api/resources/{id}/operator-state. Without this rule a request
// authorized to write at /vm:101 could retarget the write at /vm:102
// through body manipulation, defeating per-resource scoping.
func TestContract_ResourceOperatorStateUrlCanonicalIDWinsOverBody(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)

	body := []byte(`{"canonicalId":"vm:999","intentionallyOffline":true}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/resources/vm:101/operator-state", bytes.NewReader(body))
	h.HandleResourceOperatorState(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d body=%s", rec.Code, rec.Body.String())
	}
	var persisted resourceOperatorStateAPI
	if err := json.Unmarshal(rec.Body.Bytes(), &persisted); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if persisted.CanonicalID != "vm:101" {
		t.Errorf("URL canonical_id must override body; got %q (security regression)", persisted.CanonicalID)
	}
	// And the resource named in the body must NOT have a state row,
	// confirming the write didn't bleed across resource scope.
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/resources/vm:999/operator-state", nil)
	h.HandleResourceOperatorState(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("body-supplied canonical_id must not have been persisted; GET vm:999 returned %d", rec2.Code)
	}
}

// TestContract_ProxmoxGuestDockerDetectionRequiresExplicitOptIn pins the
// privacy boundary for Proxmox-side LXC Docker socket hinting and inventory.
// Router startup may wire a Docker checker or collector only when the server
// has the explicit opt-in config.
func TestContract_ProxmoxGuestDockerDetectionRequiresExplicitOptIn(t *testing.T) {
	t.Run("disabled without explicit opt-in", func(t *testing.T) {
		monitor := &monitoring.Monitor{}
		router := &Router{
			config: &config.Config{
				EnableProxmoxGuestDockerDetection: false,
			},
			agentExecServer: agentexec.NewServer(func(token string, agentID string, hostname string) bool {
				return false
			}),
		}

		router.configureProxmoxGuestDockerDetection(monitor)

		if got := monitor.GetDockerChecker(); got != nil {
			t.Fatalf("expected Proxmox guest Docker checker to stay disabled without explicit opt-in, got %T", got)
		}
		if got := monitor.GetDockerInventoryCollector(); got != nil {
			t.Fatalf("expected Proxmox guest Docker inventory collector to stay disabled without explicit opt-in, got %T", got)
		}
	})

	t.Run("socket hinting enabled only with explicit opt-in", func(t *testing.T) {
		monitor := &monitoring.Monitor{}
		router := &Router{
			config: &config.Config{
				EnableProxmoxGuestDockerDetection: true,
			},
			agentExecServer: agentexec.NewServer(func(token string, agentID string, hostname string) bool {
				return false
			}),
		}

		router.configureProxmoxGuestDockerDetection(monitor)

		if got := monitor.GetDockerChecker(); got == nil {
			t.Fatal("expected Proxmox guest Docker checker after explicit opt-in")
		}
		if got := monitor.GetDockerInventoryCollector(); got != nil {
			t.Fatalf("expected Proxmox guest Docker inventory collector to stay disabled without inventory opt-in, got %T", got)
		}
	})

	t.Run("inventory opt-in wires checker and collector", func(t *testing.T) {
		monitor := &monitoring.Monitor{}
		router := &Router{
			config: &config.Config{
				EnableProxmoxGuestDockerInventory: true,
			},
			agentExecServer: agentexec.NewServer(func(token string, agentID string, hostname string) bool {
				return false
			}),
		}

		router.configureProxmoxGuestDockerDetection(monitor)

		if got := monitor.GetDockerChecker(); got == nil {
			t.Fatal("expected Proxmox guest Docker checker after inventory opt-in")
		}
		if got := monitor.GetDockerInventoryCollector(); got == nil {
			t.Fatal("expected Proxmox guest Docker inventory collector after inventory opt-in")
		}
	})
}

func TestContract_DockerPodmanAdminCopyUsesPulseAgentModuleIdentity(t *testing.T) {
	files := []string{
		"diagnostics.go",
		"docker_agents.go",
		"types.go",
		"update_detection.go",
	}
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}
			text := string(src)
			for _, forbidden := range []string{
				"Docker" + " agent",
				"docker" + " agent",
				"Docker / Podman" + " agent",
				"Docker / Podman" + " agents",
			} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("%s must describe Docker / Podman as a pulse-agent module, not %q", file, forbidden)
				}
			}
		})
	}

	diagnostics, err := os.ReadFile("diagnostics.go")
	if err != nil {
		t.Fatalf("read diagnostics.go: %v", err)
	}
	for _, required := range []string{
		"Docker / Podman module is still using the shared API token",
		"No Docker / Podman modules have reported in yet",
		"All Docker / Podman modules are reporting with dedicated tokens and the expected version.",
	} {
		if !strings.Contains(string(diagnostics), required) {
			t.Errorf("diagnostics.go must preserve module copy %q", required)
		}
	}
}

func TestContract_UpdateReadinessIncludesV5AgentMigrationSecurityGuidance(t *testing.T) {
	source, err := os.ReadFile("update_readiness.go")
	if err != nil {
		t.Fatalf("read update_readiness.go: %v", err)
	}
	text := string(source)
	for _, required := range []string{
		`ID:      "agent-migration-security"`,
		"v5 agents can auto-update to v6, but the first hop depends on trusted transport.",
		"Use HTTPS, or keep the Pulse-to-agent migration path on a trusted local network",
		"For high-assurance environments, reinstall the v6 pulse-agent through the signed installer path",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("update_readiness.go must preserve v5-to-v6 migration security guidance %q", required)
		}
	}
}

func TestContract_MetadataGetPayloadsUseZeroRecordsInsteadOf404(t *testing.T) {
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	guestHandler := NewGuestMetadataHandler(mtp)
	dockerHandler := NewDockerMetadataHandler(mtp)

	// An empty store must serialize as an empty JSON object, never null.
	rec := httptest.NewRecorder()
	guestHandler.HandleGetMetadata(rec, httptest.NewRequest(http.MethodGet, "/api/guests/metadata", nil))
	if body := strings.TrimSpace(rec.Body.String()); body != "{}" {
		t.Fatalf("empty guest metadata map must serialize as {}, got %q", body)
	}
	rec = httptest.NewRecorder()
	dockerHandler.HandleGetMetadata(rec, httptest.NewRequest(http.MethodGet, "/api/docker/metadata", nil))
	if body := strings.TrimSpace(rec.Body.String()); body != "{}" {
		t.Fatalf("empty docker metadata map must serialize as {}, got %q", body)
	}

	// A missing resource must return a zero record carrying the requested ID,
	// never a 404 — clients rely on this to render empty metadata forms.
	rec = httptest.NewRecorder()
	guestHandler.HandleGetMetadata(rec, httptest.NewRequest(http.MethodGet, "/api/guests/metadata/qemu-100", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("missing guest metadata must respond 200, got %d", rec.Code)
	}
	var guestMeta config.GuestMetadata
	if err := json.Unmarshal(rec.Body.Bytes(), &guestMeta); err != nil {
		t.Fatalf("decode guest metadata response: %v", err)
	}
	if guestMeta.ID != "qemu-100" {
		t.Fatalf("missing guest metadata must echo the requested ID, got %q", guestMeta.ID)
	}

	rec = httptest.NewRecorder()
	dockerHandler.HandleGetMetadata(rec, httptest.NewRequest(http.MethodGet, "/api/docker/metadata/host-1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("missing docker metadata must respond 200, got %d", rec.Code)
	}
	var dockerMeta config.DockerMetadata
	if err := json.Unmarshal(rec.Body.Bytes(), &dockerMeta); err != nil {
		t.Fatalf("decode docker metadata response: %v", err)
	}
	if dockerMeta.ID != "host-1" {
		t.Fatalf("missing docker metadata must echo the requested ID, got %q", dockerMeta.ID)
	}
}

// TestContract_WebhookSigningSecretMaskedAndPreserved pins the webhook
// management payload contract for delivery signing secrets: the list API must
// mask a configured secret with the shared redaction placeholder, and an
// update that echoes the placeholder must preserve the stored secret instead
// of overwriting it with the literal placeholder string.
func TestContract_WebhookSigningSecretMaskedAndPreserved(t *testing.T) {
	mockMonitor := new(MockNotificationMonitor)
	mockManager := new(MockNotificationManager)
	mockPersistence := new(MockNotificationConfigPersistence)
	mockMonitor.On("GetNotificationManager").Return(mockManager)
	mockMonitor.On("GetConfigPersistence").Return(mockPersistence)
	h := NewNotificationHandlers(nil, mockMonitor)

	stored := notifications.WebhookConfig{
		ID:            "wh-signed",
		Name:          "Signed Webhook",
		URL:           "https://psa.example.com/inbound/pulse",
		Enabled:       true,
		Service:       "generic",
		SigningSecret: "stored-secret",
	}

	// List must mask the configured secret.
	mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{stored}).Once()
	rec := httptest.NewRecorder()
	h.GetWebhooks(rec, httptest.NewRequest(http.MethodGet, "/api/notifications/webhooks", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GetWebhooks responded %d", rec.Code)
	}
	var listed []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode webhook list: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected one webhook in list, got %d", len(listed))
	}
	if got := listed[0]["signingSecret"]; got != "***REDACTED***" {
		t.Fatalf("list signingSecret = %v, want masked placeholder", got)
	}

	// Update echoing the masked placeholder must preserve the stored secret.
	mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{stored}).Once()
	mockManager.On("ValidateWebhookURL", stored.URL).Return(nil).Once()
	mockManager.On("UpdateWebhook", "wh-signed", tmock.MatchedBy(func(w notifications.WebhookConfig) bool {
		return w.SigningSecret == "stored-secret"
	})).Return(nil).Once()
	mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{stored}).Once()
	mockPersistence.On("SaveWebhooks", tmock.Anything).Return(nil).Once()

	update := stored
	update.SigningSecret = "***REDACTED***"
	body, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("marshal update: %v", err)
	}
	rec = httptest.NewRecorder()
	h.UpdateWebhook(rec, httptest.NewRequest(http.MethodPut, "/api/notifications/webhooks/wh-signed", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("UpdateWebhook responded %d: %s", rec.Code, rec.Body.String())
	}
	mockManager.AssertExpectations(t)
}

// TestContract_OrgBoundTokenIsScopedAwayFromDefaultOrg pins the MSP tenant
// isolation boundary: a token explicitly bound to client organizations must
// not implicitly reach the default org's data (a leaked client-site token
// must not read the provider's own estate), while users and legacy unbound
// tokens retain default-org access for backward compatibility.
func TestContract_OrgBoundTokenIsScopedAwayFromDefaultOrg(t *testing.T) {
	checker := NewAuthorizationChecker(nil)

	if res := checker.CheckAccess(&config.APITokenRecord{OrgID: "client-acme"}, "", "default"); res.Allowed {
		t.Fatal("org-bound token must not implicitly access the default org")
	}
	if res := checker.CheckAccess(&config.APITokenRecord{OrgIDs: []string{"client-a", "client-b"}}, "", "default"); res.Allowed {
		t.Fatal("multi-org-bound token must not implicitly access the default org")
	}
	if res := checker.CheckAccess(&config.APITokenRecord{OrgID: "default"}, "", "default"); !res.Allowed {
		t.Fatal("token explicitly bound to default must access the default org")
	}
	if res := checker.CheckAccess(&config.APITokenRecord{}, "", "default"); !res.Allowed {
		t.Fatal("legacy unbound token must retain default-org access")
	}
	if res := checker.CheckAccess(nil, "admin", "default"); !res.Allowed {
		t.Fatal("authenticated users must retain default-org access")
	}
}

// TestContract_ProxyAuthConfiguredRoleHeaderMissingIsNonAdmin pins the
// proxy-auth admin boundary: once an admin role header is configured, admin
// authorization must be proven by that header rather than inherited from
// successful proxy authentication alone.
func TestContract_ProxyAuthConfiguredRoleHeaderMissingIsNonAdmin(t *testing.T) {
	cfg := &config.Config{
		ProxyAuthSecret:     "secret123",
		ProxyAuthUserHeader: "X-Remote-User",
		ProxyAuthRoleHeader: "X-Remote-Roles",
		ProxyAuthAdminRole:  "admin",
	}

	handlerCalled := false
	handler := RequireAdmin(cfg, func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/test", nil)
	req.Header.Set("X-Proxy-Secret", "secret123")
	req.Header.Set("X-Remote-User", "regular-user")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if handlerCalled {
		t.Fatal("admin handler was called without the configured role header")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestContract_LocalAuthConfigReadsUseSharedLock pins the shared mutable-config
// boundary for local username/password checks. Auth handlers may snapshot
// credentials under the config lock, but must not read mutable auth fields
// directly while hashing passwords or mutating sessions.
func TestContract_LocalAuthConfigReadsUseSharedLock(t *testing.T) {
	expectations := map[string][]string{
		"auth.go": {
			"config.Mu.RLock()\n\t\t\t\t\t\tcfgAuthUser := cfg.AuthUser\n\t\t\t\t\t\tcfgAuthPass := cfg.AuthPass\n\t\t\t\t\t\tconfig.Mu.RUnlock()",
			"constantTimeStringEqual(parts[0], cfgAuthUser)",
			"internalauth.CheckPasswordHash(parts[1], cfgAuthPass)",
		},
		"router.go": {
			"config.Mu.RLock()\n\t\tauthConfigured := (r.config.AuthUser != \"\" && r.config.AuthPass != \"\")",
			"r.config.ProxyAuthSecret != \"\"\n\t\tconfig.Mu.RUnlock()",
			"cfgAuthUser := r.config.AuthUser\n\tcfgAuthPass := r.config.AuthPass",
			"constantTimeStringEqual(loginReq.Username, cfgAuthUser) && auth.CheckPasswordHash(loginReq.Password, cfgAuthPass)",
		},
	}

	for file, snippets := range expectations {
		sourceBytes, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		source := string(sourceBytes)
		for _, snippet := range snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must contain %q", file, snippet)
			}
		}
	}
}

func TestContract_StateBroadcastsUseLazyCurrentStateInvalidation(t *testing.T) {
	expectations := map[string]struct {
		required  []string
		forbidden []string
	}{
		"agent_handlers_base.go": {
			required: []string{
				"b.wsHub.BroadcastCurrentStateToTenant(orgID)",
				"b.wsHub.BroadcastCurrentState()",
			},
			forbidden: []string{
				"monitor.BuildFrontendState()",
				"BroadcastStateToTenant(orgID, state)",
				"BroadcastState(state)",
			},
		},
		"alerts.go": {
			required: []string{
				"h.wsHub.BroadcastCurrentStateToTenant(orgID)",
				"h.wsHub.BroadcastCurrentState()",
			},
			forbidden: []string{
				"h.getMonitor(ctx).BuildFrontendState()",
				"BroadcastStateToTenant(orgID, frontendState)",
				"BroadcastState(frontendState)",
			},
		},
	}

	for file, expectation := range expectations {
		sourceBytes, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		source := string(sourceBytes)
		for _, snippet := range expectation.required {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must contain lazy state invalidation primitive %q", file, snippet)
			}
		}
		for _, snippet := range expectation.forbidden {
			if strings.Contains(source, snippet) {
				t.Fatalf("%s must not eagerly build or broadcast full state at mutation boundary: found %q", file, snippet)
			}
		}
	}
}
