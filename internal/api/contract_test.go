package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

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
		LoopState:              "remediation_planned",
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
		"loop_state":"remediation_planned",
		"lifecycle":[{"at":"2026-02-08T13:14:15Z","type":"state_change","message":"Moved to investigating","from":"detected","to":"investigating","metadata":{"from":"detected","to":"investigating"}}],
		"regression_count":1,
		"last_regression_at":"2026-02-08T13:44:15Z"
	}`

	assertJSONSnapshot(t, got, want)
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

func TestContract_HostedSignupResponseJSONSnapshot(t *testing.T) {
	payload := hostedSignupResponse{
		OrgID:   "org-123",
		UserID:  "owner@example.com",
		Message: "Check your email for a magic link to finish signing in.",
	}

	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal hosted signup response: %v", err)
	}

	const want = `{
		"org_id":"org-123",
		"user_id":"owner@example.com",
		"message":"Check your email for a magic link to finish signing in."
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_BillingStateJSONSnapshot(t *testing.T) {
	payload := entitlements.BillingState{
		Capabilities:         []string{"relay", "mobile_app"},
		Limits:               map[string]int64{"max_agents": 10},
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
		"limits":{"max_agents":10},
		"meters_enabled":["api_requests"],
		"plan_version":"cloud_starter",
		"subscription_state":"active",
		"stripe_customer_id":"cus_123",
		"stripe_subscription_id":"sub_123",
		"stripe_price_id":"price_123"
	}`

	assertJSONSnapshot(t, got, want)
}

func TestContract_SystemSettingsResponseJSONSnapshot(t *testing.T) {
	payload := struct {
		*config.SystemSettings
		EnvOverrides map[string]bool `json:"envOverrides,omitempty"`
	}{
		SystemSettings: &config.SystemSettings{
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
		},
		EnvOverrides: map[string]bool{
			"PULSE_TELEMETRY": true,
		},
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

func TestContract_AutoRegisterRequestJSONSnapshot(t *testing.T) {
	payload := AutoRegisterRequest{
		Type:         "pve",
		Host:         "https://pve.local:8006",
		TokenID:      "pulse-monitor@pve!pulse-homelab",
		TokenValue:   "secret-token",
		ServerName:   "pve-node-1",
		SetupCode:    "deprecated-setup-code",
		AuthToken:    "runtime-auth-token",
		Source:       "agent",
		RequestToken: true,
		Username:     "root@pam",
		Password:     "super-secret",
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
		"setupCode":"deprecated-setup-code",
		"authToken":"runtime-auth-token",
		"source":"agent",
		"requestToken":true,
		"username":"root@pam",
		"password":"super-secret"
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

func TestContract_PatrolStatusResponseJSONSnapshot(t *testing.T) {
	lastPatrolAt := time.Date(2026, 3, 12, 9, 30, 0, 0, time.UTC)
	nextPatrolAt := lastPatrolAt.Add(6 * time.Hour)
	blockedAt := lastPatrolAt.Add(15 * time.Minute)

	payload := PatrolStatusResponse{
		Running:                    false,
		Enabled:                    true,
		LastPatrolAt:               &lastPatrolAt,
		NextPatrolAt:               &nextPatrolAt,
		LastDurationMs:             12345,
		ResourcesChecked:           18,
		FindingsCount:              3,
		ErrorCount:                 1,
		Healthy:                    false,
		IntervalMs:                 21600000,
		FixedCount:                 2,
		BlockedReason:              "Awaiting AI provider configuration",
		BlockedAt:                  &blockedAt,
		QuickstartCreditsRemaining: 7,
		QuickstartCreditsTotal:     pkglicensing.QuickstartCreditsTotal,
		UsingQuickstart:            true,
		LicenseRequired:            true,
		LicenseStatus:              "none",
		UpgradeURL:                 "https://pulserelay.pro/upgrade?feature=ai_autofix",
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
		"running":false,
		"enabled":true,
		"last_patrol_at":"2026-03-12T09:30:00Z",
		"next_patrol_at":"2026-03-12T15:30:00Z",
		"last_duration_ms":12345,
		"resources_checked":18,
		"findings_count":3,
		"error_count":1,
		"healthy":false,
		"interval_ms":21600000,
		"fixed_count":2,
		"blocked_reason":"Awaiting AI provider configuration",
		"blocked_at":"2026-03-12T09:45:00Z",
		"quickstart_credits_remaining":7,
		"quickstart_credits_total":25,
		"using_quickstart":true,
		"license_required":true,
		"license_status":"none",
		"upgrade_url":"https://pulserelay.pro/upgrade?feature=ai_autofix",
		"summary":{"critical":1,"warning":2,"watch":0,"info":4}
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
			name: "content",
			event: mustStreamEvent(t, "content", chat.ContentData{
				Text: "hello",
			}),
			want: `{"type":"content","data":{"text":"hello"}}`,
		},
		{
			name: "explore_status",
			event: mustStreamEvent(t, "explore_status", chat.ExploreStatusData{
				Phase:   "started",
				Message: "Explore pre-pass running (read-only context).",
				Model:   "openai:explore-fast",
			}),
			want: `{"type":"explore_status","data":{"phase":"started","message":"Explore pre-pass running (read-only context).","model":"openai:explore-fast"}}`,
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
				Risk:        "high",
				Description: "Restart web service",
			}),
			want: `{"type":"approval_needed","data":{"approval_id":"approval-1","tool_id":"tool-2","tool_name":"pulse_exec","command":"systemctl restart nginx","run_on_host":true,"target_host":"node-1","risk":"high","description":"Restart web service"}}`,
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

func assertJSONSnapshot(t *testing.T, got []byte, want string) {
	t.Helper()

	var gotCompact bytes.Buffer
	var wantCompact bytes.Buffer
	if err := json.Compact(&gotCompact, got); err != nil {
		t.Fatalf("compact got json: %v", err)
	}
	if err := json.Compact(&wantCompact, []byte(want)); err != nil {
		t.Fatalf("compact want json: %v", err)
	}
	if gotCompact.String() != wantCompact.String() {
		t.Fatalf("json snapshot mismatch\nwant: %s\ngot:  %s", wantCompact.String(), gotCompact.String())
	}
}
