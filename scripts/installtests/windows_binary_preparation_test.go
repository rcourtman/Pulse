package installtests

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestInstallPS1PreparedBinaryRuntime(t *testing.T) {
	powerShell := nativeInstallerPowerShell(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, powerShell, "-NoLogo", "-NoProfile", "-NonInteractive", "-File",
		repoFile("scripts", "installtests", "windows_binary_preparation_controls.ps1"),
		"-InstallerPath", repoFile("scripts", "install.ps1"), "-TempDir", t.TempDir())
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "PASS all 9 Windows binary preparation controls and inherited-ACL/unique-stage controls") {
		t.Fatalf("production Windows binary preparation controls failed: %v\n%s", err, output)
	}
	t.Logf("%s", output)
}
