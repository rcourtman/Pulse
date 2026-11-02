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
	buildDir := t.TempDir()
	binary := filepath.Join(buildDir, "pulse-agent")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/pulse-agent")
	build.Dir = repoFile()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build real staged agent: %v\n%s", err, out)
	}
	binaryBytes, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("actual staged agent sha256=%x bytes=%d", sha256.Sum256(binaryBytes), len(binaryBytes))
	functions := extractInstallShellFunction(t, "fail") + "\n" + extractInstallShellFunction(t, "json_event") + "\n" + extractInstallShellFunction(t, "validate_staged_agent_url")
	for _, tc := range []struct {
		name, url string
		wantExit  int
	}{
		{"https", "https://pulse.example.invalid", 0},
		{"private", "http://192.168.1.20:7655", 0},
		{"loopback", "http://127.0.0.1:7655", 0},
		{"local-hostname", "http://pulse:7655", 0},
		{"public-http-despite-insecure-and-env-consent", "http://203.0.113.10", 14},
		{"unresolved-http", "http://pulse.example.invalid", 14},
		{"missing-url", "", 14},
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
INSECURE=true
PULSE_URL=%q
%s
validate_staged_agent_url %q
printf 'ADMITTED\n'
`, tc.url, functions, binary)
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
	gate := strings.Index(text, `validate_staged_agent_url "$TMP_BIN"`)
	trust := strings.Index(text, `verify_download_signature "$TMP_BIN"`)
	if gate < 0 || trust < 0 || trust >= gate {
		t.Fatal("URL admission must execute the verified staged agent")
	}
	for _, mutation := range []string{"safe_profile_begin_transaction", "reduce_safe_profile_collector_authority", "stop_existing_agent_service", `systemctl stop "${AGENT_NAME}"`, `mv "$TMP_BIN" "${INSTALL_DIR}/${BINARY_NAME}"`} {
		if index := strings.Index(text, mutation); index < 0 || index <= gate {
			t.Errorf("URL admission must precede %s", mutation)
		}
	}
}
