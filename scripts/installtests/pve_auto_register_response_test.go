package installtests

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPVEAutoRegisterConfirmationKeepsCredentialsOutOfArgvAndDiagnostics(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PVE root installer shell controls; Windows uses its native installer")
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate PVE confirmation controls")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", filepath.Join(filepath.Dir(source), "pve_auto_register_response.py"), "-v")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("PVE confirmation controls: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Ran 18 tests") || !strings.Contains(string(out), "\nOK\n") || strings.Contains(string(out), "skipped") {
		t.Fatalf("incomplete PVE confirmation controls:\n%s", out)
	}
	t.Logf("%s", out)
}
