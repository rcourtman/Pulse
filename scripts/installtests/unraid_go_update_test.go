package installtests

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestInstallSHUnraidBootReplacementPreservesUnrelatedJobs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell/filesystem fixture; native Unraid acceptance remains separate")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "python3", repoFile("scripts", "installtests", "unraid_go_update.py"), "-v")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Unraid boot replacement controls: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "Ran 20 tests in ") ||
		!strings.Contains(string(output), "\nOK\n") || strings.Contains(string(output), "skipped") {
		t.Fatalf("incomplete Unraid boot replacement controls:\n%s", output)
	}
}
