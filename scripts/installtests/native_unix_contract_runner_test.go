//go:build !windows

package installtests

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// The Python controls exercise both workflow phases and real compiled Go tests
// without recursively running the installer suite. Native CI supplies the
// platform timing proof; these controls reject lost checks and false passes.
func TestNativeUnixContractRunnerPartitionsWithoutDroppingChecks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", "-I", "-S", repoFile("scripts", "tests", "test_native_installer_contract_runner.py"))
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("native installer runner control deadline: %v", ctx.Err())
	}
	if err != nil {
		t.Fatalf("native installer runner control: %v\n%s", err, output)
	}
}
