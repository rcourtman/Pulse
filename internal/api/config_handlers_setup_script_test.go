package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleSetupScriptRejectsUnsafeAuthToken(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve&host=https://example.com&setup_token=$(touch%20/tmp/pwned)", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 bad request for unsafe auth token, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestHandleSetupScriptRejectsUnsafePulseURL(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve&host=https://example.com&pulse_url=http://example.com%5C%0Aecho%20oops", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 bad request for unsafe pulse_url, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestPVESetupScriptArgumentAlignment(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	// Use sentinel values to verify fmt.Sprintf argument alignment
	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pve&host=http://sentinel-host:8006&pulse_url=http://sentinel-url:7656&setup_token=deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	script := rr.Body.String()

	// Critical alignment checks to prevent fmt.Sprintf argument mismatch bugs
	// After refactor: script uses bash variables ($PULSE_URL, $TOKEN_NAME) instead of fmt.Sprintf substitutions
	tests := []struct {
		name     string
		contains string
		desc     string
	}{
		{
			name:     "token_id_uses_tokenname",
			contains: `Token ID: $PULSE_TOKEN_ID`,
			desc:     "Token ID should use $PULSE_TOKEN_ID bash variable",
		},
		{
			name:     "bash_variables_defined",
			contains: `PULSE_URL="http://sentinel-url:7656"`,
			desc:     "Bash variable PULSE_URL should be defined at top of script",
		},
		{
			name:     "token_name_variable_defined",
			contains: `TOKEN_NAME="pulse-sentinel-url"`,
			desc:     "Bash variable TOKEN_NAME should use deterministic Pulse token naming",
		},
		{
			name:     "register_json_marks_script_source",
			contains: `"source":"script"`,
			desc:     "Auto-register payload should mark script source so later canonical reruns can reuse confirmed script-created tokens",
		},
		{
			name:     "register_json_uses_secure_contract",
			contains: `"tokenValue":"'"$TOKEN_VALUE"'"`,
			desc:     "Generated setup scripts should submit locally created token completion through the canonical /api/auto-register payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !containsString(script, tt.contains) {
				t.Errorf("%s\nExpected to find: %s\nIn generated script (first 500 chars):\n%s",
					tt.desc, tt.contains, truncate(script, 500))
			}
		})
	}

	// Additional check: ensure authToken doesn't appear in --pulse-server flags
	if containsString(script, "--pulse-server deadbeef") {
		t.Error("BUG: authToken appearing in --pulse-server URL (argument misalignment)")
	}
}

func TestHandleSetupScriptUsesCanonicalShellDownloadHeaders(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/setup-script?type=pve&host=https://node.example.internal:8006&pulse_url=https://pulse.example.com:7655",
		nil,
	)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "text/x-shellscript; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", got, "text/x-shellscript; charset=utf-8")
	}
	if got := rr.Header().Get("Content-Disposition"); got != "attachment; filename=\"pulse-setup-pve.sh\"" {
		t.Fatalf("Content-Disposition = %q, want %q", got, "attachment; filename=\"pulse-setup-pve.sh\"")
	}
	if body := strings.TrimSpace(rr.Body.String()); body == "" {
		t.Fatal("expected non-empty setup script body")
	}
}

func TestHandleSetupScriptRejectsPBSBackupPerms(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/setup-script?type=pbs&host=https://pbs.example:8007&pulse_url=https://pulse.example.com:7655&backup_perms=true",
		nil,
	)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 bad request, got %d (%s)", rr.Code, rr.Body.String())
	}
	if got := strings.TrimSpace(rr.Body.String()); got != "backup_perms is only supported for type 'pve'" {
		t.Fatalf("body = %q, want canonical backup-perms guidance", got)
	}
}

func TestPVESetupScript_ConfiguresPulseMonitorRoleSafely(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pve&host=http://sentinel-host:8006&pulse_url=http://sentinel-url:7656&setup_token=deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	script := rr.Body.String()

	// Privileges must be comma-separated for pveum, and role updates should be non-destructive.
	wantSnippets := []string{
		`PRIV_STRING="$(IFS=,; echo "${EXTRA_PRIVS[*]}")"`,
		`pveum role modify PulseMonitor -privs "$PRIV_STRING" 2>/dev/null || pveum role add PulseMonitor -privs "$PRIV_STRING" 2>/dev/null`,
	}
	for _, snippet := range wantSnippets {
		if !containsString(script, snippet) {
			t.Fatalf("expected generated script to contain:\n%s\n\nGot (first 500 chars):\n%s", snippet, truncate(script, 500))
		}
	}
}

func TestPVESetupScript_UsesFailFastRetryGuidance(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pve&host=http://sentinel-host:8006&pulse_url=http://sentinel-url:7656&setup_token=deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	script := rr.Body.String()
	if !containsString(script, `echo "  curl -fsSL \"$SETUP_SCRIPT_URL\" | { if [ \"$(id -u)\" -eq 0 ]; then PULSE_SETUP_TOKEN=\"$PULSE_SETUP_TOKEN\" bash; elif command -v sudo >/dev/null 2>&1; then sudo env PULSE_SETUP_TOKEN=\"$PULSE_SETUP_TOKEN\" bash; else echo \"Root privileges required. Run as root (su -) and retry.\" >&2; exit 1; fi; }"`) {
		t.Fatalf("expected fail-fast retry guidance in setup script, got: %s", truncate(script, 700))
	}
	if !containsString(script, `echo "Root privileges required. Run as root (su -) and retry."`) {
		t.Fatalf("expected canonical root requirement guidance in setup script, got: %s", truncate(script, 700))
	}
	if !containsString(script, `echo "This setup flow must run on the Proxmox host so Pulse can create"`) {
		t.Fatalf("expected canonical off-host rerun guidance in setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "  curl -sSL \"$SETUP_SCRIPT_URL\" | bash"`) || containsString(script, `echo "   curl -sSL \"$PULSE_URL/api/setup-script?type=pve&host=YOUR_PVE_URL&pulse_url=$PULSE_URL\" | bash"`) {
		t.Fatalf("expected stale non-fail-fast guidance to be removed, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "Manual setup steps:"`) || containsString(script, `echo "  2. In Pulse: Settings → Nodes → Add Node (enter token from above)"`) {
		t.Fatalf("expected stale off-host manual token flow to be removed from setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "Please run this script as root"`) {
		t.Fatalf("expected stale root-only setup script guidance to be removed, got: %s", truncate(script, 900))
	}
}

func TestPVESetupScript_PreservesEncodedRerunURLAndBackupPerms(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/setup-script?type=pve&host=https%3A%2F%2F%5B2001%3Adb8%3A%3A1%5D%3A8006&pulse_url=https%3A%2F%2Fpulse.example.com%3A7656&backup_perms=true&setup_token=deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		nil,
	)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	script := rr.Body.String()
	expectedSetupScriptURL := `SETUP_SCRIPT_URL="https://pulse.example.com:7656/api/setup-script?backup_perms=true&host=https%3A%2F%2F%5B2001%3Adb8%3A%3A1%5D%3A8006&pulse_url=https%3A%2F%2Fpulse.example.com%3A7656&type=pve"`
	if !containsString(script, expectedSetupScriptURL) {
		t.Fatalf("expected encoded setup rerun URL with preserved backup permissions, got: %s", truncate(script, 900))
	}
	if !containsString(script, `PULSE_SETUP_TOKEN="${PULSE_SETUP_TOKEN:-deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef}"`) {
		t.Fatalf("expected canonical embedded setup token initialization before PVE rerun guidance, got: %s", truncate(script, 900))
	}
	if !containsString(script, `pveum aclmod /storage -user pulse-monitor@pve -role PVEDatastoreAdmin`) {
		t.Fatalf("expected backup permission role setup to remain enabled, got: %s", truncate(script, 900))
	}
}

func TestPVESetupScript_RemovesDiscoveredOldTokensFromBothUsers(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pve&host=http://sentinel-host:8006&pulse_url=http://sentinel-url:7656",
		nil,
	)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	script := rr.Body.String()
	if !containsString(script, `done <<< "$OLD_TOKENS_PVE"`) {
		t.Fatalf("expected PVE cleanup loop to iterate discovered pve tokens, got: %s", truncate(script, 900))
	}
	if !containsString(script, `done <<< "$OLD_TOKENS_PAM"`) {
		t.Fatalf("expected PVE cleanup loop to iterate discovered pam tokens, got: %s", truncate(script, 900))
	}
	if containsString(script, `done <<< "$OLD_TOKENS"`) {
		t.Fatalf("expected stale undefined old-token cleanup variable to be removed, got: %s", truncate(script, 900))
	}
	if !containsString(script, `pveum user token remove pulse-monitor@pve "$TOKEN"`) {
		t.Fatalf("expected PVE cleanup to remove pve user tokens, got: %s", truncate(script, 900))
	}
	if !containsString(script, `pveum user token remove pulse-monitor@pam "$TOKEN"`) {
		t.Fatalf("expected PVE cleanup to remove pam user tokens, got: %s", truncate(script, 900))
	}
}

func TestPVESetupScript_UsesCanonicalTokenPrefixForCleanupDiscovery(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pve&host=https://node.example.internal:8006&pulse_url=https://pulse.example.com:7655",
		nil,
	)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	script := rr.Body.String()
	if !containsString(script, `TOKEN_MATCH_PREFIX="pulse-pulse-example-com"`) {
		t.Fatalf("expected cleanup discovery to reuse canonical token prefix, got: %s", truncate(script, 900))
	}
	if !containsString(script, `grep -E "^${TOKEN_MATCH_PREFIX}(-[0-9]+)?$"`) {
		t.Fatalf("expected cleanup discovery to match canonical token prefix and legacy suffixes, got: %s", truncate(script, 900))
	}
	if containsString(script, `PULSE_IP_PATTERN=`) {
		t.Fatalf("expected stale ip-pattern token discovery to be removed, got: %s", truncate(script, 900))
	}
}

func TestPBSSetupScript_UsesCanonicalTokenPrefixForCleanupDiscovery(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pbs&host=https://pbs-node.example.internal:8007&pulse_url=https://pulse.example.com:7655",
		nil,
	)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	script := rr.Body.String()
	if !containsString(script, `TOKEN_MATCH_PREFIX="pulse-pulse-example-com"`) {
		t.Fatalf("expected PBS cleanup discovery to reuse canonical token prefix, got: %s", truncate(script, 900))
	}
	if !containsString(script, `grep -oE "${TOKEN_MATCH_PREFIX}(-[0-9]+)?" | sort -u || true`) {
		t.Fatalf("expected PBS cleanup discovery to match canonical token prefix and legacy suffixes, got: %s", truncate(script, 900))
	}
	if containsString(script, `PULSE_IP_PATTERN=`) {
		t.Fatalf("expected stale PBS ip-pattern token discovery to be removed, got: %s", truncate(script, 900))
	}
}

func TestSetupScripts_UseExactTokenMatchForRotationDetection(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	pveReq := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pve&host=https://node.example.internal:8006&pulse_url=https://pulse.example.com:7655",
		nil,
	)
	pveRR := httptest.NewRecorder()
	handlers.HandleSetupScript(pveRR, pveReq)
	if pveRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for PVE, got %d (%s)", pveRR.Code, pveRR.Body.String())
	}
	pveScript := pveRR.Body.String()
	if !containsString(pveScript, `awk 'NR>3 {print $2}' | grep -Fx "$TOKEN_NAME" >/dev/null 2>&1`) {
		t.Fatalf("expected PVE token rotation detection to use exact token-name matching, got: %s", truncate(pveScript, 900))
	}
	if containsString(pveScript, `grep -q "$TOKEN_NAME"`) {
		t.Fatalf("expected stale broad PVE token detection to be removed, got: %s", truncate(pveScript, 900))
	}

	pbsReq := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pbs&host=https://pbs-node.example.internal:8007&pulse_url=https://pulse.example.com:7655",
		nil,
	)
	pbsRR := httptest.NewRecorder()
	handlers.HandleSetupScript(pbsRR, pbsReq)
	if pbsRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for PBS, got %d (%s)", pbsRR.Code, pbsRR.Body.String())
	}
	pbsScript := pbsRR.Body.String()
	if !containsString(pbsScript, `awk '{print $1}' | grep -Fx "$TOKEN_NAME" >/dev/null 2>&1`) {
		t.Fatalf("expected PBS token rotation detection to use exact token-name matching, got: %s", truncate(pbsScript, 900))
	}
	if containsString(pbsScript, `grep -q "$TOKEN_NAME"`) {
		t.Fatalf("expected stale broad PBS token detection to be removed, got: %s", truncate(pbsScript, 900))
	}
}

func TestPBSSetupScript_ShowsTokenCopyBannerOnlyAfterSuccessfulCreation(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pbs&host=https://pbs-node.example.internal:8007&pulse_url=https://pulse.example.com:7655",
		nil,
	)
	rr := httptest.NewRecorder()
	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	script := rr.Body.String()
	tokenCreateIndex := strings.Index(script, `TOKEN_CREATE_RC=$?`)
	bannerIndex := strings.Index(script, `echo "IMPORTANT: Copy the token value below - it's only shown once!"`)
	successBranchIndex := strings.Index(script, "else\n    TOKEN_CREATED=true")

	if tokenCreateIndex == -1 || bannerIndex == -1 || successBranchIndex == -1 {
		t.Fatalf("expected PBS token creation flow markers to exist, got: %s", truncate(script, 1200))
	}
	if bannerIndex < tokenCreateIndex {
		t.Fatalf("expected PBS token-copy banner to appear after token creation runs, got: %s", truncate(script, 1200))
	}
	if bannerIndex < successBranchIndex {
		t.Fatalf("expected PBS token-copy banner to be inside the successful token-create branch, got: %s", truncate(script, 1200))
	}
}

func TestPVESetupScript_FailsClosedOnAutoRegisterSuccessDetection(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pve&host=http://sentinel-host:8006&pulse_url=http://sentinel-url:7656&setup_token=deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	script := rr.Body.String()
	if !containsString(script, `curl -fsS -X POST "$PULSE_URL/api/auto-register"`) {
		t.Fatalf("expected fail-fast auto-register transport in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `"source":"script"`) {
		t.Fatalf("expected PVE setup script to use canonical /api/auto-register source marker, got: %s", truncate(script, 900))
	}
	if !containsString(script, `REGISTER_RC=$?`) {
		t.Fatalf("expected explicit auto-register curl exit-code handling in setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `curl -s -X POST "$PULSE_URL/api/auto-register"`) {
		t.Fatalf("expected stale non-fail-fast auto-register transport to be removed, got: %s", truncate(script, 900))
	}
	if !containsString(script, `grep -Eq '"status"[[:space:]]*:[[:space:]]*"success"'`) {
		t.Fatalf("expected secure success detection in setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `grep -q "success"`) {
		t.Fatalf("expected broad success substring detection to be removed, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "The provided Pulse setup token was invalid or expired"`) {
		t.Fatalf("expected invalid setup-token guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Get a fresh setup token from Pulse Settings → Nodes and rerun this script."`) {
		t.Fatalf("expected fresh setup-token rerun guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `SETUP_TOKEN_INVALID=true`) {
		t.Fatalf("expected PVE auth-failure state tracking in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `elif [ "$SETUP_TOKEN_INVALID" = true ]; then`) {
		t.Fatalf("expected PVE auth-failure completion branch in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Pulse setup token authentication failed."`) {
		t.Fatalf("expected PVE auth-failure completion guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "📝 Use the token details below in Pulse Settings → Nodes to finish registration."`) {
		t.Fatalf("expected canonical PVE auto-register failure continuation guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `if [ "$AUTO_REG_SUCCESS" != true ] && [ "$SETUP_TOKEN_INVALID" != true ]; then`) {
		t.Fatalf("expected PVE manual footer to stay disabled on auth failure, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "⚠️  Auto-registration failed. Finish registration manually in Pulse Settings → Nodes."`) {
		t.Fatalf("expected canonical PVE auto-register failure summary guidance in setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "To enable auto-registration, add your API token to the setup URL"`) {
		t.Fatalf("expected stale API-token auth guidance to be removed from setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "To enable auto-registration, rerun with a valid Pulse setup token"`) {
		t.Fatalf("expected stale split setup-token auth guidance to be removed from setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "The provided API token was invalid"`) {
		t.Fatalf("expected stale invalid API-token guidance to be removed from setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "📝 For manual setup:"`) {
		t.Fatalf("expected stale PVE numbered manual-setup fallback to be removed from setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "   2. Add this node manually in Pulse Settings"`) {
		t.Fatalf("expected stale PVE auto-register failure guidance to be removed from setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "⚠️  Auto-registration failed. Manual configuration may be needed."`) {
		t.Fatalf("expected stale PVE auto-register failure summary to be removed from setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `if [ "$AUTO_REG_SUCCESS" = true ]; then`) {
		t.Fatalf("expected conditional completion messaging in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Successfully registered with Pulse monitoring."`) {
		t.Fatalf("expected canonical PVE success messaging in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Pulse monitoring token setup completed."`) {
		t.Fatalf("expected manual completion messaging in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Pulse monitoring token setup failed."`) {
		t.Fatalf("expected token-create failure completion messaging in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Pulse monitoring token setup could not be completed."`) {
		t.Fatalf("expected token-extract failure completion messaging in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Fix the token creation error above and rerun this script on the node."`) {
		t.Fatalf("expected token-create failure immediate rerun guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Resolve the token creation error shown above and rerun this script on the node."`) {
		t.Fatalf("expected token-create failure rerun guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "   Resolve the token output issue above and rerun this script on the node."`) {
		t.Fatalf("expected token-extract failure rerun guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Resolve the token output issue shown above and rerun this script on the node."`) {
		t.Fatalf("expected token-extract completion rerun guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "  Token Value: [See token output above]"`) {
		t.Fatalf("expected canonical PVE token placeholder guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Finish registration in Pulse using the manual setup details below."`) {
		t.Fatalf("expected truthful manual registration guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Add this server to Pulse with:"`) {
		t.Fatalf("expected canonical PVE manual-add heading in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Use these details in Pulse Settings → Nodes to finish registration."`) {
		t.Fatalf("expected canonical PVE manual registration continuation guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "  Host URL: $SERVER_HOST"`) {
		t.Fatalf("expected canonical PVE host continuity in manual setup instructions, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "Manual setup instructions:"`) {
		t.Fatalf("expected stale PVE manual setup heading to be removed, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "  Host URL: YOUR_PROXMOX_HOST:8006"`) {
		t.Fatalf("expected stale placeholder PVE host guidance to be removed, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "   Token Value: [See above]"`) {
		t.Fatalf("expected stale PVE short token placeholder guidance to be removed, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "Node registered successfully"`) || containsString(script, `echo "Node successfully registered with Pulse monitoring."`) {
		t.Fatalf("expected stale PVE success copy to be removed, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "Manual registration may be required."`) {
		t.Fatalf("expected stale PVE manual-registration token-failure guidance to be removed, got: %s", truncate(script, 900))
	}
	if !containsString(script, `if [ "$TOKEN_READY" = true ]; then`) {
		t.Fatalf("expected PVE manual footer to be gated on usable token extraction, got: %s", truncate(script, 1100))
	}
	if !containsString(script, `if [ "$TOKEN_READY" = true ]; then
    attempt_auto_registration
else
    AUTO_REG_SUCCESS=false
fi`) {
		t.Fatalf("expected PVE auto-registration to be skipped when no usable token is ready, got: %s", truncate(script, 1400))
	}
	if containsString(script, `elif [ "$TOKEN_READY" != true ]; then
    echo "Pulse monitoring token setup completed."`) {
		t.Fatalf("expected PVE token-extract failure to avoid completed token-setup messaging, got: %s", truncate(script, 1400))
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(findSubstring(s, substr) >= 0))
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func TestHandleSetupScript_MethodNotAllowed(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := httptest.NewRequest(method, "/api/setup-script?type=pve", nil)
		rr := httptest.NewRecorder()

		handlers.HandleSetupScript(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: expected 405 Method Not Allowed, got %d", method, rr.Code)
		}
	}
}

func TestHandleSetupScript_MissingTypeParameter(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	// No type parameter
	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?host=https://example.com", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for missing type, got %d", rr.Code)
	}
}

func TestHandleSetupScript_MissingHostParameter(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for missing host, got %d (%s)", rr.Code, rr.Body.String())
	}
	if got := strings.TrimSpace(rr.Body.String()); got != "Missing required parameter: host" {
		t.Fatalf("expected canonical missing host guidance, got %q", got)
	}
}

func TestHandleSetupScript_RejectsUnknownType(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pmg&host=https://example.com:8006&pulse_url=https://pulse.example.com:7655", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for unknown type, got %d (%s)", rr.Code, rr.Body.String())
	}
	if got := strings.TrimSpace(rr.Body.String()); got != "type must be 'pve' or 'pbs'" {
		t.Fatalf("expected canonical type guidance, got %q", got)
	}
}

func TestHandleSetupScript_MissingPulseURLParameter(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve&host=https://example.com:8006", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for missing pulse_url, got %d (%s)", rr.Code, rr.Body.String())
	}
	if got := strings.TrimSpace(rr.Body.String()); got != "Missing required parameter: pulse_url" {
		t.Fatalf("expected canonical missing pulse_url guidance, got %q", got)
	}
}

func TestHandleSetupScript_NormalizesCanonicalHost(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve&host=https://pve-node.example.internal&pulse_url=https://pulse.example.com:7655", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	script := rr.Body.String()
	if !containsString(script, `SERVER_HOST="https://pve-node.example.internal:8006"`) {
		t.Fatalf("expected setup script to normalize host with default PVE port, got: %s", truncate(script, 500))
	}
	if !containsString(script, `HOST_URL="$SERVER_HOST"`) {
		t.Fatalf("expected setup script to preserve normalized host through HOST_URL, got: %s", truncate(script, 500))
	}
	if !containsString(script, `SETUP_SCRIPT_URL="https://pulse.example.com:7655/api/setup-script?host=https%3A%2F%2Fpve-node.example.internal%3A8006&pulse_url=https%3A%2F%2Fpulse.example.com%3A7655&type=pve"`) {
		t.Fatalf("expected rerun URL to preserve normalized host identity, got: %s", truncate(script, 700))
	}
}

func TestHandleSetupScript_InvalidHostParameter(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	// Host with shell injection attempt
	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve&host=https://example.com%5C%0Aecho%20pwned", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid host, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestHandleSetupScript_RejectsShellExpansionHost(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve&host=https://example$(id).com:8006", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for shell-expansion host, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestHandleSetupScript_PBSTypeGeneratesScript(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pbs&host=https://192.168.0.10:8007&pulse_url=http://pulse.local:7656", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for PBS type, got %d (%s)", rr.Code, rr.Body.String())
	}

	script := rr.Body.String()

	// Verify PBS-specific content
	tests := []struct {
		name     string
		contains string
		desc     string
	}{
		{
			name:     "pbs_header",
			contains: "Pulse Monitoring Setup for PBS",
			desc:     "Should have PBS-specific header",
		},
		{
			name:     "proxmox_backup_manager_check",
			contains: "proxmox-backup-manager",
			desc:     "Should check for proxmox-backup-manager command",
		},
		{
			name:     "pbs_user_realm",
			contains: "pulse-monitor@pbs",
			desc:     "Should use @pbs realm for user",
		},
		{
			name:     "pbs_acl_update",
			contains: "proxmox-backup-manager acl update",
			desc:     "Should set PBS ACLs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !containsString(script, tt.contains) {
				t.Errorf("%s\nExpected to find: %s", tt.desc, tt.contains)
			}
		})
	}

	// Verify PBS script does NOT contain PVE-specific content
	if containsString(script, "pveum user add") {
		t.Error("PBS script should not contain PVE commands like 'pveum user add'")
	}
}

func TestPBSSetupScript_UsesFailFastRetryGuidance(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pbs&host=https://192.168.0.10:8007&pulse_url=http://pulse.local:7656", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	script := rr.Body.String()
	if !containsString(script, `echo "Root privileges required. Run as root (su -) and retry."`) {
		t.Fatalf("expected canonical PBS root requirement guidance in setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "   curl -sSL \"$PULSE_URL/api/setup-script?type=pbs&host=YOUR_PBS_URL&pulse_url=$PULSE_URL\" | bash"`) {
		t.Fatalf("expected stale PBS non-fail-fast guidance to be removed, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "Please run this script as root"`) {
		t.Fatalf("expected stale PBS root-only setup script guidance to be removed, got: %s", truncate(script, 900))
	}
}

func TestPBSSetupScript_FailsClosedOnAutoRegisterSuccessDetection(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pbs&host=https://192.168.0.10:8007&pulse_url=http://pulse.local:7656", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	script := rr.Body.String()
	if !containsString(script, `curl -fsS -X POST "$PULSE_URL/api/auto-register"`) {
		t.Fatalf("expected fail-fast PBS auto-register transport in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `"source": "script"`) {
		t.Fatalf("expected PBS setup script to use canonical /api/auto-register source marker, got: %s", truncate(script, 900))
	}
	if !containsString(script, `REGISTER_RC=$?`) {
		t.Fatalf("expected explicit PBS auto-register curl exit-code handling in setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `curl -s -X POST "$PULSE_URL/api/auto-register"`) {
		t.Fatalf("expected stale non-fail-fast PBS auto-register transport to be removed, got: %s", truncate(script, 900))
	}
	if !containsString(script, `grep -Eq '"status"[[:space:]]*:[[:space:]]*"success"'`) {
		t.Fatalf("expected secure PBS success detection in setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `grep -q "success"`) {
		t.Fatalf("expected broad PBS success substring detection to be removed, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "⚠️  Auto-registration skipped: token value unavailable"`) {
		t.Fatalf("expected fail-closed PBS token-value-unavailable guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "⚠️  Auto-registration skipped: no setup token provided"`) {
		t.Fatalf("expected truthful PBS setup-token-skip guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `PULSE_SETUP_TOKEN="${PULSE_SETUP_TOKEN:-}"`) {
		t.Fatalf("expected canonical PBS setup token initialization before rerun guidance, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "⚠️  Auto-registration skipped: no setup token provided"
            AUTO_REG_SUCCESS=false
            REGISTER_RESPONSE=""
            REGISTER_RC=1`) {
		t.Fatalf("expected skipped PBS setup-token path to avoid forcing fake request-failure state, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "⚠️  Auto-registration skipped: token value unavailable"
        AUTO_REG_SUCCESS=false
        REGISTER_RESPONSE=""
        REGISTER_RC=1`) {
		t.Fatalf("expected skipped PBS token-value path to avoid forcing fake request-failure state, got: %s", truncate(script, 900))
	}
	if !containsString(script, `if [ "$REGISTER_ATTEMPTED" != true ]; then`) {
		t.Fatalf("expected PBS auto-register reporting to distinguish skipped from attempted requests, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "The provided Pulse setup token was invalid or expired"`) {
		t.Fatalf("expected invalid PBS setup-token guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Get a fresh setup token from Pulse Settings → Nodes and rerun this script."`) {
		t.Fatalf("expected fresh PBS setup-token rerun guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `SETUP_TOKEN_INVALID=true`) {
		t.Fatalf("expected PBS auth-failure state tracking in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `elif [ "$SETUP_TOKEN_INVALID" = true ]; then`) {
		t.Fatalf("expected PBS auth-failure completion branch in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Pulse setup token authentication failed."`) {
		t.Fatalf("expected PBS auth-failure completion guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "📝 Use the token details below in Pulse Settings → Nodes to finish registration."`) {
		t.Fatalf("expected canonical PBS auto-register failure continuation guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `if [ "$AUTO_REG_SUCCESS" != true ] && [ "$SETUP_TOKEN_INVALID" != true ]; then`) {
		t.Fatalf("expected PBS manual footer to stay disabled on auth failure, got: %s", truncate(script, 900))
	}
	if strings.Count(script, `echo "📝 Use the token details below in Pulse Settings → Nodes to finish registration."`) < 2 {
		t.Fatalf("expected PBS request-failure and response-failure branches to share canonical manual continuation guidance, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "⚠️  Auto-registration failed. Finish registration manually in Pulse Settings → Nodes."`) {
		t.Fatalf("expected canonical PBS auto-register failure summary guidance in setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "To enable auto-registration, add your API token to the setup URL"`) {
		t.Fatalf("expected stale PBS API-token auth guidance to be removed from setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "To enable auto-registration, rerun with a valid Pulse setup token"`) {
		t.Fatalf("expected stale split PBS setup-token auth guidance to be removed from setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "The provided API token was invalid"`) {
		t.Fatalf("expected stale invalid PBS API-token guidance to be removed from setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "📝 For manual setup:"`) {
		t.Fatalf("expected stale PBS numbered manual-setup fallback to be removed from setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "   2. Add this node manually in Pulse Settings"`) {
		t.Fatalf("expected stale PBS auto-register failure guidance to be removed from setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "⚠️  Auto-registration failed. Manual configuration may be needed."`) {
		t.Fatalf("expected stale PBS auto-register failure summary to be removed from setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Successfully registered with Pulse monitoring."`) {
		t.Fatalf("expected canonical PBS success completion messaging in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Pulse monitoring token setup completed."`) {
		t.Fatalf("expected PBS manual completion messaging in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Pulse monitoring token setup failed."`) {
		t.Fatalf("expected PBS token-create failure completion messaging in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Pulse monitoring token setup could not be completed."`) {
		t.Fatalf("expected PBS token-extract failure completion messaging in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Fix the token creation error above and rerun this script on the node."`) {
		t.Fatalf("expected PBS token-create failure immediate rerun guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Resolve the token creation error shown above and rerun this script on the node."`) {
		t.Fatalf("expected PBS token-create failure rerun guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "   Resolve the token output issue above and rerun this script on the node."`) {
		t.Fatalf("expected PBS token-extract failure rerun guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Resolve the token output issue shown above and rerun this script on the node."`) {
		t.Fatalf("expected PBS token-extract completion rerun guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "  Token Value: [See token output above]"`) {
		t.Fatalf("expected canonical PBS token placeholder guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Finish registration in Pulse using the manual setup details below."`) {
		t.Fatalf("expected truthful PBS manual registration guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "Use these details in Pulse Settings → Nodes to finish registration."`) {
		t.Fatalf("expected canonical PBS manual registration continuation guidance in setup script, got: %s", truncate(script, 900))
	}
	if !containsString(script, `echo "  Host URL: $HOST_URL"`) {
		t.Fatalf("expected canonical PBS host continuity in manual setup instructions, got: %s", truncate(script, 900))
	}
	hostURLIdx := strings.Index(script, `HOST_URL="https://192.168.0.10:8007"`)
	authPromptIdx := strings.Index(script, `if [ -z "$PULSE_SETUP_TOKEN" ]; then`)
	if hostURLIdx == -1 || authPromptIdx == -1 || hostURLIdx > authPromptIdx {
		t.Fatalf("expected PBS host URL continuity to be established before auth-token gating, got: %s", truncate(script, 1200))
	}
	tokenCreateFailureIdx := strings.Index(script, `if [ "$TOKEN_CREATE_RC" -ne 0 ]; then`)
	if hostURLIdx == -1 || tokenCreateFailureIdx == -1 || hostURLIdx > tokenCreateFailureIdx {
		t.Fatalf("expected PBS host URL continuity to be established before token-create failure fallback, got: %s", truncate(script, 1200))
	}
	if !containsString(script, `if [ "$TOKEN_READY" = true ]; then`) {
		t.Fatalf("expected PBS manual footer to be gated on usable token extraction, got: %s", truncate(script, 1100))
	}
	if containsString(script, `PULSE_REG_TOKEN=your-token ./setup.sh`) {
		t.Fatalf("expected stale PBS rerun token guidance to be removed from setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "Manual registration may be required."`) {
		t.Fatalf("expected stale PBS manual-registration token-failure guidance to be removed, got: %s", truncate(script, 900))
	}
	if containsString(script, `Generate a registration token in Pulse Settings → Security`) {
		t.Fatalf("expected stale PBS registration-token guidance to be removed from setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "   Token Value: [See above]"`) || containsString(script, `echo "  Token Value: [Check the output above for the token or instructions]"`) {
		t.Fatalf("expected stale PBS token placeholder guidance to be removed from setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "✅ Successfully registered with Pulse!"`) || containsString(script, `echo "Server successfully registered with Pulse monitoring."`) {
		t.Fatalf("expected stale PBS success copy to be removed from setup script, got: %s", truncate(script, 900))
	}
	if containsString(script, `echo "  Host URL: https://$SERVER_IP:8007"`) {
		t.Fatalf("expected stale PBS runtime-IP host guidance to be removed from setup script, got: %s", truncate(script, 900))
	}
}

func TestHandleSetupScript_PBSWithAuthToken(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	// Valid auth token (64 hex chars)
	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pbs&host=https://192.168.0.10:8007&pulse_url=http://pulse.local:7656&setup_token=deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	script := rr.Body.String()

	// Verify auth token handling in PBS script
	if !containsString(script, `PULSE_SETUP_TOKEN="${PULSE_SETUP_TOKEN:-deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef}"`) {
		t.Error("PBS script should define canonical PULSE_SETUP_TOKEN bootstrap variable")
	}
}

func TestHandleSetupScript_PBSRejectsMissingHost(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pbs", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d (%s)", rr.Code, rr.Body.String())
	}
	if got := strings.TrimSpace(rr.Body.String()); got != "Missing required parameter: host" {
		t.Fatalf("expected canonical missing host guidance, got %q", got)
	}
}

func TestPBSSetupScript_OnlyShowsAttemptBannerOnRealRequestPath(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pbs&host=https://pbs-node.example.internal:8007&pulse_url=https://pulse.example.com:7655",
		nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	script := rr.Body.String()
	attemptBannerIndex := strings.Index(script, `echo "🔄 Attempting auto-registration with Pulse..."`)
	authTokenGateIndex := strings.Index(script, `if [ -n "$PULSE_SETUP_TOKEN" ]; then`)
	tokenSkipIndex := strings.Index(script, `echo "⚠️  Auto-registration skipped: token value unavailable"`)
	if attemptBannerIndex == -1 || authTokenGateIndex == -1 || tokenSkipIndex == -1 {
		t.Fatalf("expected PBS auto-registration truth markers, got: %s", truncate(script, 900))
	}
	if attemptBannerIndex < authTokenGateIndex {
		t.Fatalf("expected PBS auto-registration banner to stay inside the real request path, got: %s", truncate(script, 900))
	}
	if attemptBannerIndex < tokenSkipIndex {
		t.Fatalf("expected PBS auto-registration banner to stay after token-unavailable skip handling, got: %s", truncate(script, 900))
	}
}
