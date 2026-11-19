package api

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"regexp"
	"testing"
	"time"
)

// Ordinary API shards can leave a session store initialized before reaching
// these bare-router fixtures. The release batch need not have those preceding
// tests. Reuse this test binary (including its race instrumentation), but run
// each session-creating case as the only test in a fresh process.
func TestQuickSecuritySetupFixturesInFreshProcesses(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"TestQuickSecuritySetupAcceptsSetupTokenInBody",
		"TestQuickSecuritySetupRotatesWithBasicAuth",
		"TestQuickSecuritySetupAllowsRecoveryTokenRotation",
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable,
				"-test.run=^"+regexp.QuoteMeta(name)+"$", "-test.v", "-test.count=1", "-test.timeout=20s")
			// Own the child's temporary files even when a panic or timeout
			// prevents its TestMain from cleaning up.
			dir := t.TempDir()
			cmd.Env = append(cmd.Environ(), "TMPDIR="+dir, "TMP="+dir, "TEMP="+dir)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("fresh process failed: %v\n%s", err, output)
			}
			// Go can exit successfully if an exact selection matches nothing
			// or the fixture skips. Require the root's own passing result.
			if !bytes.Contains(output, []byte("\n--- PASS: "+name+" (")) {
				t.Fatalf("fresh process did not pass the selected test:\n%s", output)
			}
		})
	}
}
