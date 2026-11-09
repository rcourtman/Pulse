package installtests

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestRootInstallSystemdUnitPublication(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix installer and confined filesystem controls")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", repoFile("scripts", "tests", "test_server_unit_publication.py"), "-v")
	cmd.Env = append(os.Environ(), "PULSE_INSTALLER_UNDER_TEST="+repoFile("install.sh"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("complete server-unit publication and preservation controls: %v\n%s", err, out)
	}
	t.Logf("confined unit publication controls:\n%s", out)
}
