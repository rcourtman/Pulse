package installtests

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The ordinary repository test command builds the real CLI. Only the service
// and saved installation are fixtures; URL policy is never reimplemented here.
func TestAgentInstallerURLAdmission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix installer")
	}
	binary := buildPulseAgentLifecycleBinary(t)
	binaryBytes, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("actual staged agent sha256=%x bytes=%d", sha256.Sum256(binaryBytes), len(binaryBytes))
	var functions strings.Builder
	for _, name := range []string{"log_info", "redact_token", "pulse_url_uses_plain_http", "auto_enable_insecure_for_plain_http_url", "fail", "json_event", "validate_staged_agent_url"} {
		functions.WriteString(extractInstallShellFunction(t, name) + "\n")
	}
	for _, tc := range []struct {
		name, url, wantHint string
		wantExit            int
	}{
		{"https", "https://pulse.example.invalid", "", 0},
		{"private", "http://192.168.1.20:7655", "", 0},
		{"loopback", "http://127.0.0.1:7655", "", 0},
		{"local-hostname", "HTTP://pulse:7655", "", 0},
		{"public-http-despite-insecure-and-env-consent", "http://203.0.113.10", "Plain HTTP requires a local or private address.", 14},
		{"unresolved-http", "http://pulse.example.invalid", "check that it resolves only to local or private addresses on this host", 14},
		{"invalid-https", "https://pulse.example.invalid/#rejected-fragment", "Check the configured HTTPS address", 14},
		{"missing-url", "", "Check the configured HTTPS address", 14},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			preserved := map[string]string{"pulse-agent": "installed-predecessor\n", "agent-id": "saved-identity\n", "runtime.token": "saved-runtime\n", "token": "saved-token\n", "connection.env": "saved-connection\n", "pulse-agent.service": "saved-service\n", "install.sh": "saved-recovery\n"}
			for name, value := range preserved {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			script := fmt.Sprintf(`set -euo pipefail
OUTPUT_FORMAT=json
NON_INTERACTIVE=true
EXIT_PREFLIGHT_FAILED=14
INSECURE=false
PULSE_TOKEN=""
TOKEN_FILE_PATH=""
PULSE_URL=%q
%s
auto_enable_insecure_for_plain_http_url
printf 'INSECURE=%%s\n' "$INSECURE"
validate_staged_agent_url %q download
printf 'ADMITTED\n'
`, tc.url, functions.String(), binary)
			runCtx, stop := context.WithTimeout(context.Background(), 15*time.Second)
			defer stop()
			cmd := exec.CommandContext(runCtx, "bash", "-c", script)
			cmd.Env = append(os.Environ(), "PULSE_AGENT_ALLOW_PLAINTEXT_HTTP=true", "PULSE_STATE_DIR="+dir, "PULSE_LOG_FILE="+filepath.Join(dir, "unexpected.log"))
			output, err := cmd.CombinedOutput()
			exit := 0
			if err != nil {
				var failure *exec.ExitError
				if !errors.As(err, &failure) {
					t.Fatal(err)
				}
				exit = failure.ExitCode()
			}
			text := string(output)
			if runCtx.Err() != nil || exit != tc.wantExit || strings.Contains(text, "ADMITTED") != (tc.wantExit == 0) {
				t.Fatalf("URL gate exit=%d, want=%d, timeout=%v\n%s", exit, tc.wantExit, runCtx.Err(), text)
			}
			if tc.wantExit != 0 && (!strings.Contains(text, `"exitCode":14`) || !strings.Contains(text, "saved credentials have not been changed")) {
				t.Fatalf("missing truthful failure: %s", text)
			}
			if strings.Contains(text, "The downloaded agent rejected the Pulse URL.") != (tc.wantExit != 0) {
				t.Fatalf("the installer did not distinguish a confirmed URL refusal: %s", text)
			}
			if !strings.Contains(text, tc.wantHint) || strings.Contains(text, "rejected-fragment") || strings.Contains(text, "does not permit HTTP") {
				t.Fatalf("ambiguous or unredacted URL admission result: %s", text)
			}
			if strings.HasPrefix(strings.ToLower(tc.url), "http://") {
				if !strings.Contains(text, "INSECURE=true") || !strings.Contains(text, "still requires a local or private address for HTTP") {
					t.Fatalf("plain HTTP notice confused insecure mode with URL admission: %s", text)
				}
				if tc.wantExit != 0 && !strings.Contains(text, "--insecure skips TLS verification; it does not change the HTTP address check") {
					t.Fatalf("missing HTTP-specific correction: %s", text)
				}
			} else if !strings.Contains(text, "INSECURE=false") || strings.Contains(text, "Plain HTTP requires") {
				t.Fatalf("non-HTTP admission acquired a plaintext notice or setting: %s", text)
			}
			for name, value := range preserved {
				path := filepath.Join(dir, name)
				got, err := os.ReadFile(path)
				info, statErr := os.Stat(path)
				if err != nil || statErr != nil || string(got) != value || info.Mode().Perm() != 0600 {
					t.Fatalf("URL admission changed %s", name)
				}
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != len(preserved) {
				t.Fatalf("URL admission added state/log files: %v, %v", entries, err)
			}
		})
	}
}

func TestAgentURLAdmissionPrecedesInstallMutation(t *testing.T) {
	source, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "# Release metadata verification")
	if start < 0 {
		t.Fatal("missing staged trust boundary")
	}
	text = text[start:]
	staging := strings.Index(text, "prepare_truenas_scale_url_validator")
	gate := strings.Index(text, `validate_staged_agent_url "$VERSION_PROBE_BINARY"`)
	trust := strings.Index(text, `verify_download_signature "$TMP_BIN"`)
	if trust < 0 || staging <= trust || gate <= staging {
		t.Fatal("verification must precede runtime path selection and URL admission")
	}
	for _, mutation := range []string{"safe_profile_begin_transaction", "reduce_safe_profile_collector_authority", "stop_existing_agent_service", `systemctl stop "${AGENT_NAME}"`, `mv "$STAGED_AGENT_BIN" "${INSTALL_DIR}/${BINARY_NAME}"`} {
		if index := strings.Index(text, mutation); index < 0 || index <= gate {
			t.Errorf("URL admission must precede %s", mutation)
		}
	}
}
