package installtests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The QNAP root filesystem is a small RAM-backed volume rebuilt on every
// boot, so the installer must stage, install, and run the agent from the
// data volume instead. Refs #1617.
func TestInstallSHRelocatesQNAPInstallToDataVolume(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	required := []string{
		`QNAP_EARLY_VOL=$(detect_qnap_data_volume || true)`,
		`INSTALL_DIR="${QNAP_EARLY_VOL}/.pulse-agent"`,
		`export TMPDIR="$QNAP_STAGING_TMPDIR"`,
		`if [[ "$RUNTIME_BINARY" != "$QNAP_STORED_BINARY" ]]; then`,
		`rm -f "/usr/local/bin/${BINARY_NAME}"`,
	}
	for _, needle := range required {
		if !strings.Contains(script, needle) {
			t.Fatalf("install.sh missing QNAP data-volume install handling: %s", needle)
		}
	}
}

// With the data-volume layout the stored and runtime binaries are one file;
// the rendered watchdog must start the agent without a boot-time self-copy.
func TestRenderedQNAPWatchdogRunsUnifiedStoredRuntimeBinary(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}

	script := string(content)
	start := strings.Index(script, "write_qnap_wrapper_script() {")
	if start < 0 {
		t.Fatal("install.sh missing QNAP wrapper renderer")
	}
	endOffset := strings.Index(script[start:], "\nappend_qnap_autorun_block() {")
	if endOffset < 0 {
		t.Fatal("could not isolate QNAP wrapper renderer")
	}
	renderer := script[start : start+endOffset]

	tempDir := t.TempDir()
	stateDir := filepath.Join(tempDir, "state")
	logDir := filepath.Join(stateDir, "logs")
	wrapperPath := filepath.Join(stateDir, "start-pulse-agent.sh")
	binaryPath := filepath.Join(stateDir, "pulse-agent")
	startsPath := filepath.Join(tempDir, "agent-starts")
	mockBinDir := filepath.Join(tempDir, "bin")

	for _, dir := range []string{stateDir, mockBinDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create test directory: %v", err)
		}
	}
	agentScript := "#!/bin/sh\n" +
		"echo \"$$\" >> \"" + startsPath + "\"\n" +
		"trap 'exit 0' INT TERM HUP\n" +
		"while :; do sleep 1; done\n"
	if err := os.WriteFile(binaryPath, []byte(agentScript), 0o755); err != nil {
		t.Fatalf("write fake agent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mockBinDir, "pkill"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake pkill: %v", err)
	}

	harness := renderer + `
AGENT_NAME=pulse-agent
SHELL_EXPORT_LINES=""
EXEC_ARGS=""
write_qnap_wrapper_script "$1" "$2" "$3" "$4" "$5"
`
	render := exec.Command("bash", "-c", harness, "_", wrapperPath, binaryPath, binaryPath, logDir, stateDir)
	if output, err := render.CombinedOutput(); err != nil {
		t.Fatalf("render QNAP wrapper: %v\n%s", err, output)
	}

	rendered, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatalf("read rendered wrapper: %v", err)
	}
	if !strings.Contains(string(rendered), `if [ "`+binaryPath+`" != "`+binaryPath+`" ]; then`) {
		t.Fatal("rendered wrapper lost the unified-layout copy guard")
	}

	watchdog := exec.Command("sh", wrapperPath)
	watchdog.Env = []string{"PATH=" + mockBinDir + ":/usr/bin:/bin"}
	if err := watchdog.Start(); err != nil {
		t.Fatalf("start watchdog: %v", err)
	}
	t.Cleanup(func() {
		if watchdog.Process != nil {
			_ = watchdog.Process.Signal(syscall.SIGTERM)
			_, _ = watchdog.Process.Wait()
		}
	})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(startsPath); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the unified-layout watchdog to start the agent")
}
