package installtests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func nonInteractiveErrorContract(script string) error {
	start := strings.Index(script, "function Show-Error {")
	end := strings.Index(script, "function Write-InstallerEvent {")
	if start < 0 || end <= start {
		return fmt.Errorf("missing complete installer error boundary")
	}
	function := script[start:end]
	write := strings.Index(function, "Write-Host $Message")
	guard := strings.Index(function, "if ($NonInteractive) { return }")
	desktop := strings.Index(function, "Add-Type -AssemblyName System.Windows.Forms")
	if write < 0 || guard <= write || desktop <= guard || !strings.Contains(function, "MessageBox]::Show($Message") {
		return fmt.Errorf("unattended error must print and return before optional desktop UI")
	}
	return nil
}

func TestInstallPS1NonInteractiveErrorsKeepAnUnattendedExit(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "install.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := nonInteractiveErrorContract(string(content)); err != nil {
		t.Fatal(err)
	}
	for _, gate := range []string{"Write-Host $Message", "if ($NonInteractive) { return }", "MessageBox]::Show($Message"} {
		if err := nonInteractiveErrorContract(strings.Replace(string(content), gate, "", 1)); err == nil {
			t.Fatalf("accepted removed noninteractive/interactive error boundary: %s", gate)
		}
	}
	if path := os.Getenv("PULSE_WINDOWS_ERROR_PARENT"); path != "" {
		parent, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if nonInteractiveErrorContract(string(parent)) == nil {
			t.Fatal("accepted actual parent with unconditional modal error")
		}
		t.Log("rejected actual parent error function with unconditional desktop UI")
	}
}

func savedRemovalAcceptanceContract(script string) error {
	script = strings.ReplaceAll(script, "\r\n", "\n")
	for _, gate := range []string{
		`Invoke-SavedUninstall -Label $label -ExpectFailure $false`,
		`$start.EnvironmentVariables.Remove($name)`,
		`if (-not $child.WaitForExit(45000))`,
		`[System.Threading.Tasks.Task]::WaitAll($tasks, 5000)`,
		`$child.ExitCode -ne $expectedExit`,
		`$child.Kill()`, `$child.WaitForExit(5000)`, `$child.Dispose()`,
		`$output.Contains('lifecycle-private-response-must-not-appear')`,
		`$output.Contains('windows-lifecycle-fixture-not-a-secret')`,
		`$after.Files[$path].Hash -cne $Before.Files[$path].Hash`,
		`$after.Files[$path].Acl -cne $Before.Files[$path].Acl`,
		`$after.StateAcl -cne $Before.StateAcl`,
		`$after.Files.Count -ne $Before.Files.Count`,
		`foreach ($mode in @('http-503', 'wrong-id', 'string-success', 'missing-id'))`,
		`Invoke-SavedUninstall -Label $mode -ExpectFailure $true`,
		`Assert-UninstallObservation -Mode $mode -Requests 1`,
		`Assert-LifecycleRecoveryRetained -Before $before`,
		`Assert-UninstallObservation -Mode confirm -Requests 1`,
		`$observed.authenticated -ne $Requests`, `$observed.exactIdentity -ne $Requests`,
		`$agentProcess.WaitForExit(10000)`, `$agentProcess.Path -ine`,
		`if ($existingListeners.Count -ne 0)`,
	} {
		if !strings.Contains(script, gate) {
			return fmt.Errorf("missing installed saved-removal proof boundary %q", gate)
		}
	}
	if strings.Count(script, "\n        Invoke-SavedRemovalAcceptance\n") != 1 ||
		strings.Count(script, "\n    Invoke-SavedRemovalAcceptance\n") != 1 {
		return fmt.Errorf("both full and post-reboot paths must run saved-removal acceptance")
	}
	// Do not allow a supplied connection or credential to substitute for saved
	// state. This is the actual bounded child command, not a copied recipe.
	start := strings.Index(script, "function Invoke-SavedUninstall {")
	end := strings.Index(script, "function Get-LifecycleRecoverySnapshot {")
	if start < 0 || end <= start {
		return fmt.Errorf("saved-only invoker missing")
	}
	invoker := script[start:end]
	for _, name := range []string{"PULSE_URL", "PULSE_TOKEN", "PULSE_TOKEN_FILE", "PULSE_AGENT_ID", "PULSE_HOSTNAME", "PULSE_CACERT", "PULSE_SERVER_FINGERPRINT", "PULSE_INSECURE_SKIP_VERIFY"} {
		if !strings.Contains(invoker, "'"+name+"'") {
			return fmt.Errorf("ambient %s not isolated", name)
		}
	}
	for _, line := range strings.Split(invoker, "\n") {
		if strings.Contains(line, "$start.Arguments =") &&
			(!strings.Contains(line, "-Uninstall `$true -NonInteractive `$true") || strings.Contains(line, "-Token") || strings.Contains(line, "-Url") || strings.Contains(line, "-AgentId")) {
			return fmt.Errorf("child no longer exercises saved-only removal")
		}
	}
	return nil
}

func TestWindowsAgentLifecycleSavedRemovalRequiresAdverseAndCleanReadback(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "installtests", "windows_agent_lifecycle.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	script := strings.ReplaceAll(string(content), "\r\n", "\n")
	for _, ending := range []string{"\n", "\r\n"} {
		if err := savedRemovalAcceptanceContract(strings.ReplaceAll(script, "\n", ending)); err != nil {
			t.Fatal(err)
		}
	}
	for _, gate := range []string{
		`$start.EnvironmentVariables.Remove($name)`, `$child.ExitCode -ne $expectedExit`,
		`$child.WaitForExit(45000)`, `$child.Kill()`, `$child.Dispose()`,
		`$after.Files[$path].Hash -cne $Before.Files[$path].Hash`,
		`$after.Files[$path].Acl -cne $Before.Files[$path].Acl`,
		`Assert-LifecycleRecoveryRetained -Before $before`,
		`Assert-UninstallObservation -Mode confirm -Requests 1`,
		`$observed.authenticated -ne $Requests`, `$observed.exactIdentity -ne $Requests`,
		`$agentProcess.WaitForExit(10000)`, `if ($existingListeners.Count -ne 0)`,
		`'PULSE_TOKEN_FILE'`, `'PULSE_SERVER_FINGERPRINT'`,
	} {
		t.Run(gate, func(t *testing.T) {
			changed := strings.Replace(script, gate, "", 1)
			if changed == script || savedRemovalAcceptanceContract(changed) == nil {
				t.Fatal("accepted missing saved-removal boundary")
			}
		})
	}
}

func TestWindowsAgentLifecycleSavedRemovalInvokerRuntime(t *testing.T) {
	powerShell := nativeInstallerPowerShell(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, powerShell, "-NoLogo", "-NoProfile", "-NonInteractive", "-File",
		repoFile("scripts", "installtests", "windows_saved_removal_controls.ps1"),
		"-HarnessPath", repoFile("scripts", "installtests", "windows_agent_lifecycle.ps1"),
		"-InstallerPath", repoFile("scripts", "install.ps1"), "-TempDir", t.TempDir())
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "PASS all 10 saved removal controls") {
		t.Fatalf("actual error/saved-removal invoker controls failed: %v\n%s", err, output)
	}
	t.Logf("%s", output)
}
