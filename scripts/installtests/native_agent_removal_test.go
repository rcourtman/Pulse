package installtests

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNativeAgentRemovalSystemdRecipeRuntime(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only native systemd lab; Windows lifecycle remains independently required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "python3", repoFile("scripts", "tests", "test_agent_removal_systemd.py"), "-v")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("native systemd lab controls: %v\n%s", err, output)
	}
	for _, name := range []string{
		"test_definition_selection_preserves_each_actual_function_and_refuses_drift",
		"test_admission_excludes_worker_self_hosted_nonroot_existing_hosts_and_unbound_source",
		"test_recipe_owns_exact_clean_source_mandatory_linux_execution_and_bounded_retention",
		"test_all_nine_phase_verdicts_and_owned_cleanup_with_closed_manager_model",
		"test_lab_rejects_removal_that_discards_final_batch_readback",
		"test_preexisting_unit_is_never_stopped_or_removed",
		"test_failed_cleanup_stays_failed_and_keeps_ownership_for_reconciliation",
	} {
		if !strings.Contains(string(output), name+" (") {
			t.Fatalf("native systemd lab control %s did not execute:\n%s", name, output)
		}
	}
	if !strings.Contains(string(output), "\nOK\n") || strings.Contains(string(output), "skipped") {
		t.Fatalf("native systemd lab controls incomplete:\n%s", output)
	}
}
