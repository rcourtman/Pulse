package installtests

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestDemoTransactionConnectedRecoveryControls(t *testing.T) {
	cmd := exec.Command("python3", repoFile(".github", "scripts", "tests", "test_demo_runtime_transaction.py"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("demo connected transaction controls failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "\nOK\n") {
		t.Fatalf("demo controls did not return a complete unittest verdict: %s", output)
	}
}

func TestDemoTransactionKeepsSignedAdmissionAndVerificationOnlyReadOnly(t *testing.T) {
	workflow, err := os.ReadFile(repoFile(".github", "workflows", "update-demo-server.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(workflow)
	start := strings.Index(text, "- name: Apply guarded demo transaction")
	if start < 0 {
		t.Fatal("missing retained demo transaction entry")
	}
	end := strings.Index(text[start:], "- name: Retain guarded demo transaction evidence")
	if end < 0 {
		t.Fatal("missing retained demo transaction entry")
	}
	entry := text[start : start+end]
	if !strings.Contains(entry, "if: inputs.verify_only != true") {
		t.Fatal("verification-only invocation must never submit a transaction")
	}
	for _, forbidden := range []string{"Pruning demo volatile runtime stores", "Removing demo backup to restore install headroom", "sudo systemctl restart"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("unguarded demo mutation remains: %s", forbidden)
		}
	}
	for _, required := range []string{"Require exact committed activation marker for mutation", "release-activation.json", "Stable demo mutation refuses mutable, inactive, or prerelease tag", "ssh-keygen -Y verify", "-n pulse-install", "${asset}.sshsig", "Verify public browser smoke"} {
		if !strings.Contains(text, required) {
			t.Fatalf("lost signed/admission/customer verification boundary: %s", required)
		}
	}
	recovery, err := os.ReadFile(repoFile(".github", "workflows", "recover-demo-server.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(recovery), "sudo systemctl stop pulse") || strings.Contains(string(recovery), "Compensate failed mutated recovery") {
		t.Fatal("later workflow failure must not stop a committed or unrelated service")
	}
	if !strings.Contains(string(recovery), ".forward.elapsed_seconds >= 300") || !strings.Contains(string(recovery), "if: always()") {
		t.Fatal("recovery must validate and retain the actual terminal observation")
	}
}

func TestDemoNativeAcceptanceIsSecretFreeDisposableAndKeepsRealWindows(t *testing.T) {
	data, err := os.ReadFile(repoFile(".github", "workflows", "demo-runtime-native.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{
		"pull_request:", "branches: [main]", "contents: read", "runs-on: ubuntu-24.04", "timeout-minutes: 35",
		"persist-credentials: false", "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1",
		"demo_runtime_native.py", "if: always()", "demo-native-result.json", "if-no-files-found: error",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("lost disposable native CI/source or terminal-receipt boundary: %s", required)
		}
	}
	for _, forbidden := range []string{"secrets.", "pull_request_target", "self-hosted", "contents: write", "id-token: write"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("native fixture gained credentials, privileged trigger or non-disposable target: %s", forbidden)
		}
	}
	driver, err := os.ReadFile(repoFile(".github", "scripts", "tests", "demo_runtime_native.py"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"not-an-empty-disposable-public-ci-runner", "dispatcher.BOOTSTRAP", "proc.terminate()", "--signal=TERM", "original_engine_sha256", "fixture_engine_sha256", "cleanup_complete", "signed_published_installer_acceptance", "elapsed_seconds\"] >= 300"} {
		if !strings.Contains(string(driver), required) {
			t.Fatalf("lost native fixture observation or limitation: %s", required)
		}
	}
	if strings.Contains(string(driver), "WINDOW =") || strings.Contains(string(driver), "time.monotonic =") || strings.Contains(string(driver), "time.sleep =") {
		t.Fatal("native acceptance must not shorten production windows or replace real clocks")
	}
}
