package updates

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// updateSelfTestTimeout bounds the --version probe so a hung artifact cannot
// stall the update pipeline indefinitely.
const updateSelfTestTimeout = 30 * time.Second

// updateSelfTestMaxOutput caps how much probe output is echoed into errors.
const updateSelfTestMaxOutput = 2048

// updateSelfTestCommandContext is swapped out by tests.
var updateSelfTestCommandContext = exec.CommandContext

// selfTestVersionTokenRegex matches version tokens in --version output, with
// or without the leading v (release builds print "Pulse vX.Y.Z").
var selfTestVersionTokenRegex = regexp.MustCompile(`v?\d+\.\d+\.\d+(?:-[A-Za-z0-9.]*\d[A-Za-z0-9.]*)?`)

// selfTestNewBinary runs the extracted update binary with --version before it
// replaces the running one. Checksum and signature verification prove the
// download matches the published artifact; they cannot prove the artifact
// runs on this host (wrong-arch fallback asset, incompatible libc) or that it
// is the version the user approved. --version exits before any server
// startup, so the probe is side-effect free.
func selfTestNewBinary(ctx context.Context, binaryPath, workDir, expectedVersion string) error {
	if err := os.Chmod(binaryPath, 0o755); err != nil {
		return fmt.Errorf("failed to mark new binary executable for self-test: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, updateSelfTestTimeout)
	defer cancel()

	cmd := updateSelfTestCommandContext(ctx, binaryPath, "--version")
	// The tarball ships VERSION at its root; run from there so the probe sees
	// the same layout an installed binary would.
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	probeOutput := strings.TrimSpace(string(output))
	if len(probeOutput) > updateSelfTestMaxOutput {
		probeOutput = probeOutput[:updateSelfTestMaxOutput]
	}
	if err != nil {
		return fmt.Errorf("new pulse binary failed --version self-test: %w (output: %q)", err, probeOutput)
	}

	expected := strings.TrimPrefix(strings.TrimSpace(expectedVersion), "v")
	if expected == "" {
		return nil
	}
	for _, token := range selfTestVersionTokenRegex.FindAllString(probeOutput, -1) {
		if strings.TrimPrefix(token, "v") == expected {
			return nil
		}
	}
	return fmt.Errorf("new pulse binary self-test reported %q, which does not include expected version %s", probeOutput, expectedVersion)
}
