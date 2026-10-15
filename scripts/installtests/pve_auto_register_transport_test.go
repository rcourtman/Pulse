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

// Keep the real-curl controls in the normal installer suite as well as runnable
// on their own without compiling the application's unrelated frontend embed.
func TestPVEAutoRegisterTransportAuthenticatesBeforeCredentials(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate PVE transport controls")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", filepath.Join(filepath.Dir(source), "pve_auto_register_transport.py"), "-v")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("PVE installer transport controls: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Ran 12 tests") || !strings.Contains(string(out), "\nOK\n") || strings.Contains(string(out), "skipped") {
		t.Fatalf("incomplete PVE installer transport controls:\n%s", out)
	}
	t.Logf("%s", out)
}
