package installtests

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestInstallSHQNAPBootReplacementPreservesUnrelatedJobs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell and filesystem fixture; native NAS acceptance remains separate")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "python3", repoFile("scripts", "installtests", "qnap_autorun_update.py"), "-v")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("QNAP boot replacement controls: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "Ran 13 tests in ") ||
		!strings.Contains(string(output), "\nOK\n") || strings.Contains(string(output), "skipped") {
		t.Fatalf("incomplete QNAP boot replacement controls:\n%s", output)
	}
}
