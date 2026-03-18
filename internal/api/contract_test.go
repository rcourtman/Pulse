package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	authpkg "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
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
	if !strings.Contains(script, `awk 'NR>3 {print $2}' | grep -Fx "$TOKEN_NAME" >/dev/null 2>&1`) {
		t.Fatalf("setup script missing exact PVE token rotation detection: %s", script)
	}
	if strings.Contains(script, `PULSE_IP_PATTERN=`) {
		t.Fatalf("setup script preserved stale ip-pattern cleanup discovery: %s", script)
	}
	if strings.Contains(script, `grep -q "$TOKEN_NAME"`) {
		t.Fatalf("setup script preserved stale broad PVE token rotation detection: %s", script)
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
	if !strings.Contains(script, `"source":"script"`) {
		t.Fatalf("setup script missing canonical /api/auto-register source marker: %s", script)
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
    attempt_auto_registration
else
    AUTO_REG_SUCCESS=false
fi`) {
		t.Fatalf("setup script does not skip PVE auto-registration when no usable token is ready: %s", script)
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
				ClusterLabel:   "other-cluster",
				NodeHostLabel:  "worker-2",
				NamespaceLabel: "kube-system",
				IsWorkload:     true,
			},
		},
	}

	filtered := filterRecoveryPointsForRollups(points, recovery.ListPointsOptions{
		Query:          "repo-a",
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
	foundMonitoredSystemLimit := false
	for _, limit := range payload.Limits {
		if limit.Key == pkglicensing.MaxMonitoredSystemsLicenseGateKey {
			foundMonitoredSystemLimit = true
			if limit.Limit != 50 {
				t.Fatalf("max_monitored_systems limit=%d, want 50", limit.Limit)
			}
		}
	}
	if !foundMonitoredSystemLimit {
		t.Fatalf("expected max_monitored_systems limit in payload, got %+v", payload.Limits)
	}
}

func TestContract_EntitlementPayloadMonitoredSystemUsageJSONSnapshot(t *testing.T) {
	payload := buildEntitlementPayloadWithUsage(&licenseStatus{
		Valid:               true,
		Tier:                pkglicensing.TierPro,
		Features:            append([]string(nil), pkglicensing.TierFeatures[pkglicensing.TierPro]...),
		MaxMonitoredSystems: 15,
	}, string(pkglicensing.SubStateActive), entitlementUsageSnapshot{
		MonitoredSystems: 7,
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
		"capabilities":["update_alerts","sso","ai_patrol","relay","mobile_app","push_notifications","long_term_metrics","ai_alerts","ai_autofix","kubernetes_ai","agent_profiles","advanced_sso","rbac","audit_logging","advanced_reporting"],
		"limits":[{"key":"max_monitored_systems","limit":15,"current":7,"state":"ok"}],
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
		"limits":{"max_monitored_systems":50},
		"meters_enabled":[],
		"plan_version":"msp_starter",
		"subscription_state":"active",
		"stripe_customer_id":"cus_hosted",
		"stripe_subscription_id":"sub_hosted",
		"stripe_price_id":"price_hosted"
	}`

	assertJSONSnapshot(t, rec.Body.Bytes(), want)
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

func TestContract_InstallScriptReleaseAssetURL(t *testing.T) {
	router := &Router{serverVersion: "v6.0.0-rc.1"}

	got, err := router.installScriptReleaseAssetURL("install.sh")
	if err != nil {
		t.Fatalf("install script release asset URL: %v", err)
	}

	const want = "https://github.com/rcourtman/Pulse/releases/download/v6.0.0-rc.1/install.sh"
	if got != want {
		t.Fatalf("install script release asset URL = %q, want %q", got, want)
	}
}

func TestContract_InstallScriptReleaseAssetURLUsesConfiguredRepo(t *testing.T) {
	t.Setenv("PULSE_GITHUB_REPO", "example/pulse-fork")

	router := &Router{serverVersion: "v6.0.0-rc.1"}

	got, err := router.installScriptReleaseAssetURL("install.sh")
	if err != nil {
		t.Fatalf("install script release asset URL: %v", err)
	}

	const want = "https://github.com/example/pulse-fork/releases/download/v6.0.0-rc.1/install.sh"
	if got != want {
		t.Fatalf("install script release asset URL = %q, want %q", got, want)
	}
}

func TestContract_InstallScriptReleaseAssetURLRejectsUnreleasedBuild(t *testing.T) {
	router := &Router{serverVersion: "dev"}

	if _, err := router.installScriptReleaseAssetURL("install.sh"); err == nil {
		t.Fatalf("expected development build to reject release asset lookup")
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
	if strings.Contains(got, "//install.sh") {
		t.Fatalf("install command preserved double-slash install path: %s", got)
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

func TestContract_QuickSecuritySetupBootstrapRetrievalGuidance(t *testing.T) {
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

	handler := handleQuickSecuritySetupFixed(router)
	body := `{"username":"bootstrap","password":"StrongPass!1","apiToken":"` + strings.Repeat("aa", 32) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(body))
	req.RemoteAddr = "198.51.100.40:54321"
	rec := httptest.NewRecorder()

	authLimiter.Reset("198.51.100.40")
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

func TestContract_AutoRegisterWebSocketEventJSONSnapshot(t *testing.T) {
	payload := map[string]any{
		"type":      "pve",
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

	const want = `{"data":[],"meta":{"page":1,"limit":50,"total":0,"totalPages":0},"aggregations":{"total":0,"byType":{},"byStatus":{},"bySource":{}}}`
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
	if resource.Policy == nil {
		t.Fatal("expected policy metadata in resource contract")
	}
	if got := resource.Policy.Sensitivity; got != unifiedresources.ResourceSensitivityRestricted {
		t.Fatalf("policy sensitivity = %q, want %q", got, unifiedresources.ResourceSensitivityRestricted)
	}
	if got := resource.Policy.Routing.Scope; got != unifiedresources.ResourceRoutingScopeLocalOnly {
		t.Fatalf("routing scope = %q, want %q", got, unifiedresources.ResourceRoutingScopeLocalOnly)
	}
	if resource.Policy.Routing.AllowCloudSummary {
		t.Fatal("expected restricted resource to block cloud summary")
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
