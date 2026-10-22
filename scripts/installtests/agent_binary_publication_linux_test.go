//go:build linux

package installtests

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// The standard Linux source VM supplies two real filesystems. This executes
// the production signed-download/publication slice, not a native service.
func TestInstallSHBinaryPublicationPreservesPreviousExecutable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", repoFile("scripts", "installtests", "agent_binary_publication.py"), "-v")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("agent executable publication controls: %v\n%s", err, output)
	}
	for _, name := range []string{
		"test_cross_filesystem_copy_failure_preserves_old_file",
		"test_success_publishes_one_complete_same_filesystem_file",
		"test_preparation_failures_do_not_stop_or_replace_the_collector",
		"test_publication_refusal_retains_complete_previous_binary",
		"test_unsafe_destination_does_not_absorb_the_agent",
		"test_signature_checksum_and_tls_refusals_precede_host_changes",
	} {
		if !strings.Contains(string(output), name+" (__main__.AgentBinaryPublication."+name+")") {
			t.Fatalf("missing executable publication control %s:\n%s", name, output)
		}
	}
	if strings.Contains(string(output), "skipped=") || !strings.Contains(string(output), "Ran 6 tests") {
		t.Fatalf("publication controls were skipped or incomplete:\n%s", output)
	}
	t.Logf("%s", output)
}
