package declarativeconfig

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func strPtr(v string) *string { return &v }
func intPtr(v int) *int       { return &v }
func boolPtr(v bool) *bool    { return &v }

type fixtureSecrets struct {
	smtpPassword      string
	slackWebhookURL   string
	genericWebhookURL string
	webhookHeaderAuth string
	webhookCustomTok  string
	webhookSigning    string
	appriseConfigKey  string
	appriseAPIKey     string
	appriseTgram      string
	appriseHTTPS      string
	deadManPingURL    string
	httpBasicPassword string
	httpBearerToken   string
	httpHeaderSecret  string
	httpPostBody      string
}

func fullTestSnapshot() (Snapshot, fixtureSecrets) {
	sec := fixtureSecrets{
		smtpPassword:      "SMTP_SUPER_SECRET_9911",
		slackWebhookURL:   "https://hooks.slack.com/services/T000111/B000222/SLACK_TOKEN_SECRET_333",
		genericWebhookURL: "https://hooks.example.com/ingest/GENERIC_PATH_SECRET_444?custom=QUERY_SECRET_555",
		webhookHeaderAuth: "Bearer WEBHOOK_HDR_SECRET_666",
		webhookCustomTok:  "PUSHOVER_APP_TOKEN_SECRET_777",
		webhookSigning:    "WEBHOOK_HMAC_SIGNING_SECRET_888",
		appriseConfigKey:  "APPRISE_CONFIG_KEY_SECRET_999",
		appriseAPIKey:     "APPRISE_API_KEY_SECRET_1010",
		appriseTgram:      "tgram://TGRAM_BOT_SECRET_1111/TGRAM_CHAT_1212",
		appriseHTTPS:      "https://apprise.example.com/notify/APPRISE_URL_SECRET_1313",
		deadManPingURL:    "https://hc-ping.example.com/DEADMAN_UUID_SECRET_1414",
		httpBasicPassword: "HTTP_BASIC_PASSWORD_SECRET_1515",
		httpBearerToken:   "HTTP_BEARER_TOKEN_SECRET_1616",
		httpHeaderSecret:  "HTTP_HEADER_VALUE_SECRET_1717",
		httpPostBody:      `{"secretPayload":"HTTP_POST_BODY_SECRET_1818"}`,
	}

	maintStart := time.Date(2026, 12, 1, 2, 0, 0, 0, time.UTC)
	maintEnd := time.Date(2026, 12, 1, 4, 0, 0, 0, time.UTC)

	alertCfg := alerts.AlertConfig{
		Enabled:           true,
		MinimumDelta:      2.5,
		SuppressionWindow: 5,
		Overrides:         map[string]alerts.ThresholdConfig{},
		TimeThresholds:    map[string]int{"guest": 30, "node": 60},
		Schedule: alerts.ScheduleConfig{
			QuietHours: alerts.QuietHours{
				Enabled:  true,
				Timezone: "UTC",
				Start:    "22:00",
				End:      "06:00",
			},
		},
	}

	intentDoc := alerts.NewAlertIntentPolicyDocument()
	intentDoc.Revision = 4
	intentDoc.Defaults["*"] = alerts.AlertIntentRule{
		GraceSeconds:       intPtr(45),
		HonorOperatorState: boolPtr(true),
	}
	intentDoc.ResourceTypes["guest"] = map[string]alerts.AlertIntentRule{
		"state.offline": {
			GraceSeconds: intPtr(120),
		},
	}

	snap := Snapshot{
		AlertConfig:         &alertCfg,
		AlertIntentPolicies: &intentDoc,
		AvailabilityTargets: []config.AvailabilityTarget{
			{
				ID:                     "chk-http-basic",
				ConfigRevision:         3,
				Name:                   "Internal API Basic",
				TargetKind:             config.AvailabilityTargetService,
				Address:                "https://api.internal.example.com",
				Protocol:               config.AvailabilityProbeHTTPS,
				Port:                   443,
				Path:                   "/healthz",
				Enabled:                true,
				PollIntervalSecs:       30,
				TimeoutMillis:          2500,
				FailureThreshold:       2,
				ObservationLocationIDs: []string{config.AvailabilityObservationLocationLocal},
				HTTP: &config.AvailabilityHTTPConfig{
					Method: config.AvailabilityHTTPMethodPOST,
					Headers: []config.AvailabilityHTTPHeader{
						{ID: "hdr-1", Name: "X-API-Key", Value: strPtr(sec.httpHeaderSecret)},
					},
					Authentication: config.AvailabilityHTTPAuthentication{
						Type:     config.AvailabilityHTTPAuthBasic,
						Username: "probe-user",
						Password: strPtr(sec.httpBasicPassword),
					},
					Body:              strPtr(sec.httpPostBody),
					ExpectedStatusMin: 200,
					ExpectedStatusMax: 299,
					JSONPath:          "status",
					JSONEquals:        "ok",
				},
			},
			{
				ID:                     "chk-http-bearer",
				ConfigRevision:         2,
				Name:                   "Gateway Bearer",
				TargetKind:             config.AvailabilityTargetService,
				Address:                "https://gw.internal.example.com",
				Protocol:               config.AvailabilityProbeHTTPS,
				Path:                   "/ready",
				Enabled:                true,
				PollIntervalSecs:       60,
				TimeoutMillis:          2000,
				FailureThreshold:       2,
				ObservationLocationIDs: []string{config.AvailabilityObservationLocationLocal, "agent:edge-1"},
				HTTP: &config.AvailabilityHTTPConfig{
					Method: config.AvailabilityHTTPMethodGET,
					Authentication: config.AvailabilityHTTPAuthentication{
						Type:        config.AvailabilityHTTPAuthBearer,
						BearerToken: strPtr(sec.httpBearerToken),
					},
					ExpectedStatusMin: 200,
					ExpectedStatusMax: 204,
				},
			},
		},
		OperatorStates: []unified.ResourceOperatorState{
			{
				CanonicalID:        "pve:cluster-a:vm:101",
				MonitoringMode:     unified.MonitoringModeExpectedOffline,
				LifecycleState:     unified.LifecycleStateActive,
				NeverAutoRemediate: true,
				MaintenanceStartAt: &maintStart,
				MaintenanceEndAt:   &maintEnd,
				MaintenanceScope:   unified.MaintenanceScopeResource,
				MaintenanceReason:  "Monthly kernel patch",
				Criticality:        unified.CriticalityHigh,
				Note:               "Primary database replica",
				SetAt:              time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
				SetBy:              "admin",
			},
		},
		Email: &notifications.EmailConfig{
			Enabled:         true,
			Provider:        "custom",
			SMTPHost:        "smtp.example.com",
			SMTPPort:        587,
			Username:        "alerts@example.com",
			Password:        sec.smtpPassword,
			From:            "alerts@example.com",
			To:              []string{"ops@example.com"},
			TLS:             false,
			StartTLS:        true,
			RateLimit:       30,
			MinimumSeverity: "warning",
		},
		Webhooks: []notifications.WebhookConfig{
			{
				ID:            "wh-slack",
				Name:          "Ops Slack",
				URL:           sec.slackWebhookURL,
				Method:        "POST",
				Enabled:       true,
				Service:       "slack",
				SigningSecret: sec.webhookSigning,
				Headers: map[string]string{
					"X-Custom-Auth": sec.webhookHeaderAuth,
				},
			},
			{
				ID:      "wh-generic",
				Name:    "Pushover Bridge",
				URL:     sec.genericWebhookURL,
				Method:  "POST",
				Enabled: true,
				Service: "pushover",
				CustomFields: map[string]string{
					"token": sec.webhookCustomTok,
				},
			},
		},
		Apprise: &notifications.AppriseConfig{
			Enabled:        true,
			Mode:           notifications.AppriseModeHTTP,
			CLIPath:        "apprise",
			TimeoutSeconds: 15,
			ServerURL:      "https://apprise-srv.example.com",
			ConfigKey:      sec.appriseConfigKey,
			APIKey:         sec.appriseAPIKey,
			APIKeyHeader:   "X-API-KEY",
			Targets:        []string{sec.appriseTgram, sec.appriseHTTPS},
		},
		DeadMan: &notifications.DeadManConfig{
			PingURL: sec.deadManPingURL,
		},
	}
	return snap, sec
}

func allSecretSubstrings(sec fixtureSecrets) []string {
	return []string{
		sec.smtpPassword,
		"SLACK_TOKEN_SECRET_333",
		"T000111",
		"B000222",
		"GENERIC_PATH_SECRET_444",
		"QUERY_SECRET_555",
		sec.webhookHeaderAuth,
		sec.webhookCustomTok,
		sec.webhookSigning,
		sec.appriseConfigKey,
		sec.appriseAPIKey,
		"TGRAM_BOT_SECRET_1111",
		"TGRAM_CHAT_1212",
		"APPRISE_URL_SECRET_1313",
		"DEADMAN_UUID_SECRET_1414",
		sec.httpBasicPassword,
		sec.httpBearerToken,
		sec.httpHeaderSecret,
		"HTTP_POST_BODY_SECRET_1818",
	}
}

func assertNoSecretLeak(t *testing.T, label string, payload string, sec fixtureSecrets) {
	t.Helper()
	for _, needle := range allSecretSubstrings(sec) {
		if strings.Contains(payload, needle) {
			t.Errorf("%s leaked secret substring %q", label, needle)
		}
	}
}

func assertCleanCopy(t *testing.T, label string, text string) {
	t.Helper()
	if strings.Contains(text, "\u2014") {
		t.Errorf("%s contains forbidden em-dash: %q", label, text)
	}
	if strings.Contains(text, ";") {
		t.Errorf("%s contains forbidden semicolon: %q", label, text)
	}
}

func TestExportRoundTripYAMLAndJSONNoChangesAndZeroSecretLeakage(t *testing.T) {
	snap, sec := fullTestSnapshot()

	exported := Export(snap)
	if exported.APIVersion != APIVersion || exported.Kind != Kind {
		t.Fatalf("unexpected envelope: %+v", exported)
	}

	yamlBytes, err := MarshalYAML(exported)
	if err != nil {
		t.Fatalf("MarshalYAML() error = %v", err)
	}
	jsonBytes, err := json.Marshal(exported)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Verify ZERO plaintext secret leakage in both YAML and JSON exports.
	assertNoSecretLeak(t, "exported YAML", string(yamlBytes), sec)
	assertNoSecretLeak(t, "exported JSON", string(jsonBytes), sec)

	// Verify YAML uses camelCase keys (not lowercased Go field names).
	yamlStr := string(yamlBytes)
	for _, expectedKey := range []string{"apiVersion:", "availabilityTargets:", "operatorStates:", "intentPolicies:", "observationLocationIds:", "secretRef: preserved"} {
		if !strings.Contains(yamlStr, expectedKey) {
			t.Errorf("exported YAML missing expected key %q", expectedKey)
		}
	}

	// Round-trip via YAML.
	fromYAML, err := UnmarshalDocument(yamlBytes)
	if err != nil {
		t.Fatalf("UnmarshalDocument(yaml) error = %v", err)
	}
	yamlPlan, yamlResolved, err := Plan(snap, fromYAML, PlanOptions{})
	if err != nil {
		t.Fatalf("Plan(fromYAML) error = %v", err)
	}
	if yamlPlan.Summary.HasChanges {
		t.Fatalf("expected HasChanges=false after YAML round-trip, got changes: %+v", yamlPlan.Changes)
	}
	if yamlResolved.AlertsChanged || yamlResolved.AlertIntentChanged || yamlResolved.AvailabilityTargetsChanged ||
		yamlResolved.OperatorStatesChanged || yamlResolved.EmailChanged || yamlResolved.WebhooksChanged ||
		yamlResolved.AppriseChanged || yamlResolved.DeadManChanged {
		t.Fatalf("expected no ResolvedState flags changed on round-trip, got %+v", yamlResolved)
	}

	// Round-trip via JSON must yield the exact same PlanHash.
	fromJSON, err := UnmarshalDocument(jsonBytes)
	if err != nil {
		t.Fatalf("UnmarshalDocument(json) error = %v", err)
	}
	jsonPlan, _, err := Plan(snap, fromJSON, PlanOptions{})
	if err != nil {
		t.Fatalf("Plan(fromJSON) error = %v", err)
	}
	if jsonPlan.Summary.HasChanges {
		t.Fatalf("expected HasChanges=false after JSON round-trip, got changes: %+v", jsonPlan.Changes)
	}
	if yamlPlan.PlanHash != jsonPlan.PlanHash {
		t.Fatalf("PlanHash mismatch between YAML (%s) and JSON (%s)", yamlPlan.PlanHash, jsonPlan.PlanHash)
	}
	if !strings.HasPrefix(yamlPlan.PlanHash, "sha256:") || len(yamlPlan.PlanHash) != len("sha256:")+64 {
		t.Fatalf("unexpected PlanHash format: %q", yamlPlan.PlanHash)
	}

	// Verify PlanResult never leaks secrets even when rotating every secret.
	rotated := Export(snap)
	rotated.Notifications.Email.Password = &SecretField{Value: "NEW_SMTP_ROTATED_9001"}
	(*rotated.Notifications.Webhooks)[0].URL = &SecretField{Value: "https://hooks.slack.com/services/T9/B9/NEW_SLACK_ROTATED_9002"}
	(*rotated.Notifications.Webhooks)[0].SigningSecret = &SecretField{Value: "NEW_SIGNING_ROTATED_9003"}
	rotated.Notifications.Apprise.APIKey = &SecretField{Value: "NEW_APPRISE_ROTATED_9004"}
	rotated.Notifications.DeadMan.PingURL = &SecretField{Value: "https://hc-ping.example.com/NEW_DEADMAN_ROTATED_9005"}
	(*rotated.AvailabilityTargets)[0].HTTP.Authentication.Password = &SecretField{Value: "NEW_HTTP_PASS_ROTATED_9006"}

	rotPlan, rotResolved, err := Plan(snap, rotated, PlanOptions{})
	if err != nil {
		t.Fatalf("Plan(rotated) error = %v", err)
	}
	if !rotPlan.Summary.HasChanges || rotPlan.Summary.Updates == 0 {
		t.Fatalf("expected updates when rotating secrets, got %+v", rotPlan.Summary)
	}
	if rotResolved.Email == nil || rotResolved.Email.Password != "NEW_SMTP_ROTATED_9001" {
		t.Fatalf("expected resolved email password to contain rotated value")
	}
	planJSON, _ := json.Marshal(rotPlan)
	for _, rotatedSecret := range []string{
		"NEW_SMTP_ROTATED_9001",
		"NEW_SLACK_ROTATED_9002",
		"NEW_SIGNING_ROTATED_9003",
		"NEW_APPRISE_ROTATED_9004",
		"NEW_DEADMAN_ROTATED_9005",
		"NEW_HTTP_PASS_ROTATED_9006",
	} {
		if strings.Contains(string(planJSON), rotatedSecret) {
			t.Errorf("PlanResult JSON leaked rotated secret %q", rotatedSecret)
		}
	}
	assertNoSecretLeak(t, "PlanResult JSON", string(planJSON), sec)
	for _, ch := range rotPlan.Changes {
		assertCleanCopy(t, "change summary", ch.Summary)
	}
}

func TestSecretPreservationMissingAndOriginSwap(t *testing.T) {
	snap, sec := fullTestSnapshot()

	t.Run("missing saved email password fails closed", func(t *testing.T) {
		doc := Document{
			APIVersion: APIVersion,
			Kind:       Kind,
			Notifications: &NotificationsSection{
				Email: &EmailSpec{
					Enabled:  true,
					SMTPHost: "smtp.example.com",
					SMTPPort: 587,
					Password: &SecretField{SecretRef: SecretRefPreserved},
				},
			},
		}
		_, _, err := Plan(Snapshot{}, doc, PlanOptions{})
		if !errors.Is(err, ErrSecretMissing) {
			t.Fatalf("expected ErrSecretMissing, got %v", err)
		}
		assertCleanCopy(t, "error", err.Error())
	})

	t.Run("missing saved webhook url fails closed", func(t *testing.T) {
		doc := Document{
			APIVersion: APIVersion,
			Kind:       Kind,
			Notifications: &NotificationsSection{
				Webhooks: &[]WebhookSpec{
					{
						ID:   "wh-new",
						Name: "Brand New",
						URL:  &SecretField{SecretRef: SecretRefPreserved},
					},
				},
			},
		}
		_, _, err := Plan(snap, doc, PlanOptions{})
		if !errors.Is(err, ErrSecretMissing) {
			t.Fatalf("expected ErrSecretMissing, got %v", err)
		}
		assertCleanCopy(t, "error", err.Error())
	})

	t.Run("missing saved deadman ping url fails closed", func(t *testing.T) {
		doc := Document{
			APIVersion: APIVersion,
			Kind:       Kind,
			Notifications: &NotificationsSection{
				DeadMan: &DeadManSpec{
					PingURL: &SecretField{SecretRef: SecretRefPreserved},
				},
			},
		}
		_, _, err := Plan(Snapshot{}, doc, PlanOptions{})
		if !errors.Is(err, ErrSecretMissing) {
			t.Fatalf("expected ErrSecretMissing, got %v", err)
		}
	})

	t.Run("missing saved apprise target fails closed", func(t *testing.T) {
		doc := Document{
			APIVersion: APIVersion,
			Kind:       Kind,
			Notifications: &NotificationsSection{
				Apprise: &AppriseSpec{
					Enabled: true,
					Mode:    notifications.AppriseModeCLI,
					Targets: []SecretField{{SecretRef: SecretRefPreserved}},
				},
			},
		}
		_, _, err := Plan(Snapshot{}, doc, PlanOptions{})
		if !errors.Is(err, ErrSecretMissing) {
			t.Fatalf("expected ErrSecretMissing, got %v", err)
		}
	})

	t.Run("availability HTTP origin swap rejects preserved secret", func(t *testing.T) {
		doc := Export(snap)
		// Swap host on chk-http-basic while keeping secretRef: preserved.
		(*doc.AvailabilityTargets)[0].Address = "https://attacker.example.com"
		_, _, err := Plan(snap, doc, PlanOptions{})
		if !errors.Is(err, ErrSecretMissing) {
			t.Fatalf("expected ErrSecretMissing on origin swap, got %v", err)
		}
		assertCleanCopy(t, "origin swap error", err.Error())
	})

	t.Run("availability HTTP path-only edit preserves secret on same origin", func(t *testing.T) {
		doc := Export(snap)
		(*doc.AvailabilityTargets)[0].Path = "/healthz-v2"
		plan, resolved, err := Plan(snap, doc, PlanOptions{})
		if err != nil {
			t.Fatalf("Plan() error = %v", err)
		}
		if !plan.Summary.HasChanges || !resolved.AvailabilityTargetsChanged {
			t.Fatalf("expected availability target update")
		}
		var updated config.AvailabilityTarget
		for _, item := range resolved.AvailabilityTargets {
			if item.ID == "chk-http-basic" {
				updated = item
			}
		}
		if updated.HTTP == nil || updated.HTTP.Authentication.Password == nil || *updated.HTTP.Authentication.Password != sec.httpBasicPassword {
			t.Fatalf("expected preserved basic password on same-origin path update")
		}
		if updated.ConfigRevision != 4 {
			t.Fatalf("expected ConfigRevision increment from 3 to 4 on path change, got %d", updated.ConfigRevision)
		}
	})

	t.Run("availability HTTP display-only edit keeps ConfigRevision unchanged", func(t *testing.T) {
		doc := Export(snap)
		(*doc.AvailabilityTargets)[0].Name = "Renamed Internal API"
		_, resolved, err := Plan(snap, doc, PlanOptions{})
		if err != nil {
			t.Fatalf("Plan() error = %v", err)
		}
		for _, item := range resolved.AvailabilityTargets {
			if item.ID == "chk-http-basic" && item.ConfigRevision != 3 {
				t.Fatalf("expected ConfigRevision to remain 3 on name-only change, got %d", item.ConfigRevision)
			}
		}
	})
}

func TestResolveEnvSecretRefsAndPlanRejection(t *testing.T) {
	doc := Document{
		APIVersion: APIVersion,
		Kind:       Kind,
		AvailabilityTargets: &[]AvailabilityTargetSpec{
			{
				ID:       "chk-env",
				Name:     "Env Probe",
				Address:  "https://service.example.com",
				Protocol: config.AvailabilityProbeHTTPS,
				Enabled:  true,
				HTTP: &AvailabilityHTTPSpec{
					Method: config.AvailabilityHTTPMethodPOST,
					Headers: []AvailabilityHTTPHeaderSpec{
						{ID: "h1", Name: "X-Token", Value: &SecretField{SecretRef: "env:PROBE_HDR"}},
					},
					Authentication: AvailabilityHTTPAuthSpec{
						Type:     config.AvailabilityHTTPAuthBasic,
						Username: "user",
						Password: &SecretField{SecretRef: "env:PROBE_PASS"},
					},
					Body:              &SecretField{SecretRef: "env:PROBE_BODY"},
					ExpectedStatusMin: 200,
					ExpectedStatusMax: 299,
				},
			},
		},
		Notifications: &NotificationsSection{
			Email: &EmailSpec{
				Enabled:  true,
				SMTPHost: "smtp.example.com",
				SMTPPort: 587,
				Password: &SecretField{SecretRef: "env:SMTP_PASS"},
			},
			Webhooks: &[]WebhookSpec{
				{
					ID:            "wh-env",
					Name:          "Env Hook",
					Enabled:       true,
					URL:           &SecretField{SecretRef: "env:HOOK_URL"},
					SigningSecret: &SecretField{SecretRef: "env:HOOK_SIGN"},
					Headers: map[string]SecretField{
						"Authorization": {SecretRef: "env:HOOK_HDR"},
					},
					CustomFields: map[string]SecretField{
						"token": {SecretRef: "env:HOOK_CUSTOM"},
					},
				},
			},
			Apprise: &AppriseSpec{
				Enabled:   true,
				Mode:      notifications.AppriseModeHTTP,
				ServerURL: "https://apprise.example.com",
				ConfigKey: &SecretField{SecretRef: "env:APPRISE_CFG"},
				APIKey:    &SecretField{SecretRef: "env:APPRISE_API"},
				Targets:   []SecretField{{SecretRef: "env:APPRISE_TARGET"}},
			},
			DeadMan: &DeadManSpec{
				PingURL: &SecretField{SecretRef: "env:DEADMAN_URL"},
			},
		},
	}

	// Plan must reject unresolved env: refs before ResolveEnvSecretRefs runs.
	_, _, err := Plan(Snapshot{}, doc, PlanOptions{})
	if !errors.Is(err, ErrUnresolvedSecretRef) {
		t.Fatalf("expected ErrUnresolvedSecretRef from Plan, got %v", err)
	}
	assertCleanCopy(t, "unresolved env error", err.Error())

	envMap := map[string]string{
		"PROBE_HDR":      "hdr-val",
		"PROBE_PASS":     "probe-pass",
		"PROBE_BODY":     `{"ok":true}`,
		"SMTP_PASS":      "smtp-pass",
		"HOOK_URL":       "https://hooks.example.com/resolved",
		"HOOK_SIGN":      "sign-val",
		"HOOK_HDR":       "Bearer tok",
		"HOOK_CUSTOM":    "custom-tok",
		"APPRISE_CFG":    "cfg-key",
		"APPRISE_API":    "api-key",
		"APPRISE_TARGET": "https://notify.example.com/t1",
		"DEADMAN_URL":    "https://watchdog.example.com/ping/resolved-uuid",
	}

	// Missing env var must fail closed without partially mutating doc.
	missingEnv := func(k string) string {
		if k == "DEADMAN_URL" {
			return ""
		}
		return envMap[k]
	}
	if err := ResolveEnvSecretRefs(&doc, missingEnv); !errors.Is(err, ErrUnresolvedSecretRef) {
		t.Fatalf("expected ErrUnresolvedSecretRef for missing env var, got %v", err)
	}
	if doc.Notifications.Email.Password.SecretRef != "env:SMTP_PASS" || doc.Notifications.Email.Password.Value != "" {
		t.Fatalf("expected ResolveEnvSecretRefs not to partially mutate doc on failure")
	}

	// Full resolution succeeds and Plan produces resolved values cleanly.
	if err := ResolveEnvSecretRefs(&doc, func(k string) string { return envMap[k] }); err != nil {
		t.Fatalf("ResolveEnvSecretRefs() error = %v", err)
	}
	plan, resolved, err := Plan(Snapshot{}, doc, PlanOptions{})
	if err != nil {
		t.Fatalf("Plan() after ResolveEnvSecretRefs error = %v", err)
	}
	if !plan.Summary.HasChanges {
		t.Fatalf("expected creates on empty snapshot")
	}
	if resolved.Email == nil || resolved.Email.Password != "smtp-pass" {
		t.Fatalf("expected resolved SMTP password")
	}
	if resolved.DeadMan == nil || resolved.DeadMan.PingURL != "https://watchdog.example.com/ping/resolved-uuid" {
		t.Fatalf("expected resolved DeadMan PingURL")
	}
	if len(resolved.Webhooks) != 1 || resolved.Webhooks[0].URL != "https://hooks.example.com/resolved" {
		t.Fatalf("expected resolved webhook URL")
	}
	if len(resolved.AvailabilityTargets) != 1 || resolved.AvailabilityTargets[0].HTTP.Authentication.Password == nil || *resolved.AvailabilityTargets[0].HTTP.Authentication.Password != "probe-pass" {
		t.Fatalf("expected resolved availability HTTP password")
	}
}

func TestPruneBehaviorAndPlanHashDrift(t *testing.T) {
	snap, _ := fullTestSnapshot()

	// Partial document naming only one availability target and one webhook, omitting alerts and operatorStates.
	onlyOneTarget := []AvailabilityTargetSpec{exportAvailabilityTarget(config.NormalizeAvailabilityTarget(snap.AvailabilityTargets[0]))}
	onlyOneWebhook := []WebhookSpec{exportWebhookSpecs(snap.Webhooks)[0]}

	partialDoc := Document{
		APIVersion:          APIVersion,
		Kind:                Kind,
		AvailabilityTargets: &onlyOneTarget,
		Notifications: &NotificationsSection{
			Webhooks: &onlyOneWebhook,
		},
	}

	// With Prune=false, omitted items in included lists are preserved.
	noPrunePlan, noPruneResolved, err := Plan(snap, partialDoc, PlanOptions{Prune: false})
	if err != nil {
		t.Fatalf("Plan(Prune=false) error = %v", err)
	}
	if noPrunePlan.Summary.HasChanges || noPrunePlan.Summary.Deletes != 0 {
		t.Fatalf("expected no deletes with Prune=false, got %+v", noPrunePlan.Summary)
	}
	if noPruneResolved.AvailabilityTargetsChanged || noPruneResolved.WebhooksChanged {
		t.Fatalf("expected no changes with Prune=false")
	}

	// With Prune=true, omitted items in included lists are planned for deletion,
	// while omitted top-level sections (operatorStates) remain untouched.
	prunePlan, pruneResolved, err := Plan(snap, partialDoc, PlanOptions{Prune: true})
	if err != nil {
		t.Fatalf("Plan(Prune=true) error = %v", err)
	}
	if !prunePlan.Summary.HasChanges || prunePlan.Summary.Deletes != 2 {
		t.Fatalf("expected 2 deletes with Prune=true, got %+v (%+v)", prunePlan.Summary, prunePlan.Changes)
	}
	if !pruneResolved.AvailabilityTargetsChanged || len(pruneResolved.AvailabilityTargets) != 1 {
		t.Fatalf("expected 1 remaining availability target, got %d", len(pruneResolved.AvailabilityTargets))
	}
	if !pruneResolved.WebhooksChanged || len(pruneResolved.Webhooks) != 1 {
		t.Fatalf("expected 1 remaining webhook, got %d", len(pruneResolved.Webhooks))
	}
	if pruneResolved.OperatorStatesChanged || len(pruneResolved.OperatorStatesToClear) != 0 {
		t.Fatalf("omitted operatorStates section must remain untouched even when Prune=true")
	}

	// Explicit empty operatorStates slice with Prune=true clears existing operator states.
	emptyStates := []OperatorStateSpec{}
	clearStatesDoc := Document{
		APIVersion:     APIVersion,
		Kind:           Kind,
		OperatorStates: &emptyStates,
	}
	clearPlan, clearResolved, err := Plan(snap, clearStatesDoc, PlanOptions{Prune: true})
	if err != nil {
		t.Fatalf("Plan(clear operatorStates) error = %v", err)
	}
	if clearPlan.Summary.Deletes != 1 || len(clearResolved.OperatorStatesToClear) != 1 || clearResolved.OperatorStatesToClear[0] != "pve:cluster-a:vm:101" {
		t.Fatalf("expected operator state to be cleared, got plan=%+v resolved=%+v", clearPlan.Summary, clearResolved.OperatorStatesToClear)
	}

	// PlanHash must change when current state drifts.
	driftedSnap := snap
	driftedSnap.DeadMan = &notifications.DeadManConfig{PingURL: "https://hc-ping.example.com/DRIFTED_UUID"}
	driftedPlan, _, err := Plan(driftedSnap, partialDoc, PlanOptions{Prune: true})
	if err != nil {
		t.Fatalf("Plan(driftedSnap) error = %v", err)
	}
	if driftedPlan.PlanHash == prunePlan.PlanHash {
		t.Fatalf("expected PlanHash to change when current state drifts")
	}
	if noPrunePlan.PlanHash == prunePlan.PlanHash {
		t.Fatalf("expected PlanHash to differ between Prune=false and Prune=true")
	}
}

func TestPlanCanonicalValidation(t *testing.T) {
	t.Run("invalid apiVersion and kind", func(t *testing.T) {
		_, _, err := Plan(Snapshot{}, Document{APIVersion: "wrong/v1", Kind: Kind}, PlanOptions{})
		if !errors.Is(err, ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument, got %v", err)
		}
		_, _, err = Plan(Snapshot{}, Document{APIVersion: APIVersion, Kind: "WrongKind"}, PlanOptions{})
		if !errors.Is(err, ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument, got %v", err)
		}
	})

	t.Run("invalid dead-man loopback url rejected", func(t *testing.T) {
		doc := Document{
			APIVersion: APIVersion,
			Kind:       Kind,
			Notifications: &NotificationsSection{
				DeadMan: &DeadManSpec{
					PingURL: &SecretField{Value: "http://127.0.0.1:8000/ping/token"},
				},
			},
		}
		_, _, err := Plan(Snapshot{}, doc, PlanOptions{})
		if !errors.Is(err, ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument for loopback dead-man URL, got %v", err)
		}
	})

	t.Run("invalid availability target rejected", func(t *testing.T) {
		badTargets := []AvailabilityTargetSpec{
			{
				ID:       "chk-bad",
				Name:     "Bad ICMP",
				Address:  "192.0.2.10",
				Protocol: config.AvailabilityProbeICMP,
				Port:     80,
				Enabled:  true,
			},
		}
		_, _, err := Plan(Snapshot{}, Document{
			APIVersion:          APIVersion,
			Kind:                Kind,
			AvailabilityTargets: &badTargets,
		}, PlanOptions{})
		if !errors.Is(err, ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument for ICMP target with port, got %v", err)
		}
	})

	t.Run("invalid operator state rejected", func(t *testing.T) {
		start := time.Date(2026, 12, 1, 5, 0, 0, 0, time.UTC)
		end := time.Date(2026, 12, 1, 4, 0, 0, 0, time.UTC)
		badStates := []OperatorStateSpec{
			{
				CanonicalID:        "pve:node:1",
				MaintenanceStartAt: &start,
				MaintenanceEndAt:   &end,
			},
		}
		_, _, err := Plan(Snapshot{}, Document{
			APIVersion:     APIVersion,
			Kind:           Kind,
			OperatorStates: &badStates,
		}, PlanOptions{})
		if !errors.Is(err, ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument for invalid maintenance window, got %v", err)
		}
	})

	t.Run("invalid alert intent policy rejected", func(t *testing.T) {
		badIntent := alerts.AlertIntentPolicyDocument{
			SchemaVersion: 99,
		}
		_, _, err := Plan(Snapshot{}, Document{
			APIVersion: APIVersion,
			Kind:       Kind,
			Alerts: &AlertsSection{
				IntentPolicies: &badIntent,
			},
		}, PlanOptions{})
		if !errors.Is(err, ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument for unsupported schemaVersion, got %v", err)
		}
	})

	t.Run("unmarshal rejects empty or multi-document YAML", func(t *testing.T) {
		if _, err := UnmarshalDocument([]byte("   \n")); !errors.Is(err, ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument on empty input, got %v", err)
		}
		multi := []byte("apiVersion: pulse.monitoring/v1\nkind: MonitoringConfig\n---\napiVersion: pulse.monitoring/v1\nkind: MonitoringConfig\n")
		if _, err := UnmarshalDocument(multi); !errors.Is(err, ErrInvalidDocument) {
			t.Fatalf("expected ErrInvalidDocument on multi-document YAML, got %v", err)
		}
	})
}
