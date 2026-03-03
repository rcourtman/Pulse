package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

const integrationSetupAuthToken = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

func TestSetupScriptTokenLifecycleIntegration_PVE(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires bash-compatible environment")
	}

	script := generateSetupScriptForIntegration(t, "pve")
	section := extractSetupScriptSection(t, script, "SETUP_AUTH_TOKEN=\"", "# Set up permissions")

	harness := fmt.Sprintf(`#!/usr/bin/env bash
set -eo pipefail
PULSE_URL="http://SENTINEL_URL:7656"
SERVER_HOST="http://SENTINEL_HOST:8006"
TOKEN_NAME="pulse-sentinel-url"
PULSE_TOKEN_ID="pulse-monitor@pam!${TOKEN_NAME}"
PULSE_API_TOKEN=""
%s
attempt_auto_registration
echo "STATE AUTO_REG_SUCCESS=${AUTO_REG_SUCCESS} TOKEN_ROTATION_SKIPPED=${TOKEN_ROTATION_SKIPPED} TOKEN_VALUE=${TOKEN_VALUE}"
`, section)

	mocks := pveHarnessMockBinaries()

	t.Run("preserves_existing_token_by_default", func(t *testing.T) {
		output, trace := runSetupHarness(t, harness, mocks, map[string]string{
			"MOCK_TOKEN_EXISTS": "1",
		})

		assertContains(t, output, "Keeping it unchanged to avoid breaking existing Pulse credentials")
		assertContains(t, output, "Auto-registration skipped: existing token preserved to avoid credential drift")
		assertContains(t, output, "STATE AUTO_REG_SUCCESS=false TOKEN_ROTATION_SKIPPED=true TOKEN_VALUE=")

		assertContains(t, trace, "pveum user token list pulse-monitor@pam")
		assertNotContains(t, trace, "pveum user token remove pulse-monitor@pam pulse-sentinel-url")
		assertNotContains(t, trace, "pveum user token add pulse-monitor@pam pulse-sentinel-url --privsep 0")
		assertNotContains(t, trace, "curl -s -X POST")
	})

	t.Run("force_rotate_rotates_and_auto_registers", func(t *testing.T) {
		output, trace := runSetupHarness(t, harness, mocks, map[string]string{
			"MOCK_TOKEN_EXISTS":        "1",
			"PULSE_FORCE_TOKEN_ROTATE": "1",
		})

		assertContains(t, output, "API token rotated successfully")
		assertContains(t, output, "Node registered successfully")
		assertContains(t, output, "STATE AUTO_REG_SUCCESS=true TOKEN_ROTATION_SKIPPED=false TOKEN_VALUE=mocked-pve-secret")

		assertContains(t, trace, "pveum user token remove pulse-monitor@pam pulse-sentinel-url")
		assertContains(t, trace, "pveum user token add pulse-monitor@pam pulse-sentinel-url --privsep 0")
		assertContains(t, trace, "curl -s -X POST")
	})
}

func TestSetupScriptTokenLifecycleIntegration_PBS(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires bash-compatible environment")
	}

	script := generateSetupScriptForIntegration(t, "pbs")
	section := extractSetupScriptSection(t, script, "# Generate API token", "# Set up permissions")

	harness := fmt.Sprintf(`#!/usr/bin/env bash
set -eo pipefail
TOKEN_NAME="pulse-sentinel-url"
PULSE_TOKEN_ID="pulse-monitor@pbs!${TOKEN_NAME}"
PULSE_API_TOKEN=""
%s
echo "STATE AUTO_REG_SUCCESS=${AUTO_REG_SUCCESS} TOKEN_ROTATION_SKIPPED=${TOKEN_ROTATION_SKIPPED} TOKEN_VALUE=${TOKEN_VALUE}"
`, section)

	mocks := pbsHarnessMockBinaries()

	t.Run("preserves_existing_token_by_default", func(t *testing.T) {
		output, trace := runSetupHarness(t, harness, mocks, map[string]string{
			"MOCK_TOKEN_EXISTS": "1",
		})

		assertContains(t, output, "Keeping it unchanged to avoid breaking existing Pulse credentials")
		assertContains(t, output, "STATE AUTO_REG_SUCCESS=false TOKEN_ROTATION_SKIPPED=true TOKEN_VALUE=")

		assertContains(t, trace, "proxmox-backup-manager user list-tokens pulse-monitor@pbs")
		assertNotContains(t, trace, "proxmox-backup-manager user delete-token pulse-monitor@pbs pulse-sentinel-url")
		assertNotContains(t, trace, "proxmox-backup-manager user generate-token pulse-monitor@pbs pulse-sentinel-url")
		assertNotContains(t, trace, "curl -s -X POST")
	})

	t.Run("force_rotate_rotates_and_auto_registers", func(t *testing.T) {
		output, trace := runSetupHarness(t, harness, mocks, map[string]string{
			"MOCK_TOKEN_EXISTS":        "1",
			"PULSE_FORCE_TOKEN_ROTATE": "1",
		})

		assertContains(t, output, "Token rotated for Pulse monitoring")
		assertContains(t, output, "Successfully registered with Pulse")
		assertContains(t, output, "STATE AUTO_REG_SUCCESS=true TOKEN_ROTATION_SKIPPED=false TOKEN_VALUE=mocked-pbs-secret")

		assertContains(t, trace, "proxmox-backup-manager user delete-token pulse-monitor@pbs pulse-sentinel-url")
		assertContains(t, trace, "proxmox-backup-manager user generate-token pulse-monitor@pbs pulse-sentinel-url")
		assertContains(t, trace, "curl -s -X POST")
	})
}

func generateSetupScriptForIntegration(t *testing.T, nodeType string) string {
	t.Helper()

	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	handlers := newTestConfigHandlers(t, cfg)

	var reqPath string
	switch nodeType {
	case "pve":
		reqPath = fmt.Sprintf("/api/setup-script?type=pve&host=http://SENTINEL_HOST:8006&pulse_url=http://SENTINEL_URL:7656&auth_token=%s", integrationSetupAuthToken)
	case "pbs":
		reqPath = fmt.Sprintf("/api/setup-script?type=pbs&host=https://192.168.0.10:8007&pulse_url=http://SENTINEL_URL:7656&auth_token=%s", integrationSetupAuthToken)
	default:
		t.Fatalf("unsupported node type: %s", nodeType)
	}

	req := httptest.NewRequest(http.MethodGet, reqPath, nil)
	rr := httptest.NewRecorder()
	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	return rr.Body.String()
}

func extractSetupScriptSection(t *testing.T, script, startMarker, endMarker string) string {
	t.Helper()

	start := strings.Index(script, startMarker)
	if start == -1 {
		t.Fatalf("start marker not found: %q", startMarker)
	}

	remainder := script[start:]
	end := strings.Index(remainder, endMarker)
	if end == -1 {
		t.Fatalf("end marker not found: %q", endMarker)
	}

	return remainder[:end]
}

func runSetupHarness(t *testing.T, harness string, mockBinaries map[string]string, extraEnv map[string]string) (output string, trace string) {
	t.Helper()

	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}

	for name, content := range mockBinaries {
		path := filepath.Join(binDir, name)
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatalf("write mock %s: %v", name, err)
		}
	}

	harnessPath := filepath.Join(root, "harness.sh")
	if err := os.WriteFile(harnessPath, []byte(harness), 0o755); err != nil {
		t.Fatalf("write harness: %v", err)
	}

	tracePath := filepath.Join(root, "trace.log")
	cmd := exec.Command("bash", harnessPath)
	env := append(os.Environ(),
		"PATH="+binDir+":/usr/bin:/bin",
		"TRACE_FILE="+tracePath,
	)
	for k, v := range extraEnv {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("harness execution failed: %v\noutput:\n%s", err, string(out))
	}

	traceBytes, err := os.ReadFile(tracePath)
	if err != nil {
		if os.IsNotExist(err) {
			traceBytes = []byte{}
		} else {
			t.Fatalf("read trace: %v", err)
		}
	}

	return string(out), string(traceBytes)
}

func pveHarnessMockBinaries() map[string]string {
	return map[string]string{
		"pveum": `#!/usr/bin/env bash
set -e
printf 'pveum %s\n' "$*" >> "$TRACE_FILE"
case "$*" in
  "user token list pulse-monitor@pam")
    if [ "${MOCK_TOKEN_EXISTS:-0}" = "1" ]; then
      echo "pulse-sentinel-url"
    fi
    ;;
  "user token remove pulse-monitor@pam pulse-sentinel-url")
    ;;
  "user token add pulse-monitor@pam pulse-sentinel-url --privsep 0")
    # Emit the box-drawing format parsed by the setup script (│ value │ secret │)
    printf '\342\224\202 value \342\224\202 mocked-pve-secret \342\224\202\n'
    ;;
  *)
    echo "unexpected pveum args: $*" >&2
    exit 1
    ;;
esac
`,
		"hostname": `#!/usr/bin/env bash
set -e
if [ "${1:-}" = "-s" ]; then
  echo "pve-node"
  exit 0
fi
if [ "${1:-}" = "-I" ]; then
  echo "192.0.2.50"
  exit 0
fi
echo "pve-node"
`,
		"curl": `#!/usr/bin/env bash
set -e
printf 'curl %s\n' "$*" >> "$TRACE_FILE"
echo '{"success":true}'
`,
	}
}

func pbsHarnessMockBinaries() map[string]string {
	return map[string]string{
		"proxmox-backup-manager": `#!/usr/bin/env bash
set -e
printf 'proxmox-backup-manager %s\n' "$*" >> "$TRACE_FILE"
case "$*" in
  "user list-tokens pulse-monitor@pbs")
    if [ "${MOCK_TOKEN_EXISTS:-0}" = "1" ]; then
      echo "pulse-sentinel-url"
    fi
    ;;
  "user delete-token pulse-monitor@pbs pulse-sentinel-url")
    ;;
  "user generate-token pulse-monitor@pbs pulse-sentinel-url")
    echo '{"tokenid":"pulse-monitor@pbs!pulse-sentinel-url","value":"mocked-pbs-secret"}'
    ;;
  *)
    echo "unexpected proxmox-backup-manager args: $*" >&2
    exit 1
    ;;
esac
`,
		"hostname": `#!/usr/bin/env bash
set -e
if [ "${1:-}" = "-s" ]; then
  echo "pbs-node"
  exit 0
fi
if [ "${1:-}" = "-I" ]; then
  echo "192.0.2.60"
  exit 0
fi
echo "pbs-node"
`,
		"curl": `#!/usr/bin/env bash
set -e
printf 'curl %s\n' "$*" >> "$TRACE_FILE"
echo '{"success":true}'
`,
	}
}

func assertContains(t *testing.T, value, needle string) {
	t.Helper()
	if !strings.Contains(value, needle) {
		t.Fatalf("expected to find %q\nvalue:\n%s", needle, value)
	}
}

func assertNotContains(t *testing.T, value, needle string) {
	t.Helper()
	if strings.Contains(value, needle) {
		t.Fatalf("did not expect to find %q\nvalue:\n%s", needle, value)
	}
}
