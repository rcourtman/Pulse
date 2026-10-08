package installtests

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func windowsNativeAcceptanceContract(harness string, steps []nativeWindowsExitStep) error {
	for _, needle := range []string{
		`$PSVersionTable.PSEdition -ne 'Desktop'`,
		`$PSVersionTable.PSVersion.Major -ne 5`,
		`$PSVersionTable.PSVersion.Minor -ne 1`,
		`return Get-Service -Name $serviceName -ErrorAction Stop`,
		`$_.FullyQualifiedErrorId.Split(',')[0] -eq 'NoServiceFoundForGivenName'`,
		`$_.CategoryInfo.Category -eq [System.Management.Automation.ErrorCategory]::ObjectNotFound`,
		`$remaining = Get-LifecycleService`,
		"$previousDisableAutoUpdate = $null\n$restoreAutoUpdateAtExit = $false",
		`Stop-Process -Id $script:serverProcess.Id -Force -ErrorAction Stop`,
		`if (-not $script:serverProcess.WaitForExit(5000)) {`,
		`$listeners = @(Get-NetTCPConnection -ErrorAction Stop | Where-Object {`,
		`foreach ($label in @('uninstall', 'repeated uninstall')) {`,
		`Invoke-Installer -Label $label -Arguments '-Uninstall $true -NonInteractive $true'`,
		`Assert-LifecycleAgentAbsent`,
	} {
		if !strings.Contains(harness, needle) {
			return fmt.Errorf("missing native acceptance boundary %q", needle)
		}
	}
	if strings.Contains(harness, "Get-Service $serviceName -ErrorAction SilentlyContinue") ||
		strings.Contains(harness, "Get-NetTCPConnection -LocalPort 9191") {
		return fmt.Errorf("failed observation can stand in for absence")
	}
	preflight := "Assert-LifecycleAgentAbsent\n    $previousDisableAutoUpdate = [Environment]::GetEnvironmentVariable"
	if !strings.Contains(harness, preflight) {
		return fmt.Errorf("machine state can change before independent clean-runner admission")
	}

	for _, step := range steps {
		if step.Name == "Exercise native Windows service lifecycle" {
			if step.Shell != "powershell" || step.ContinueOnError ||
				strings.Contains(step.Run, "Remove-Item") || strings.Contains(step.Run, "SilentlyContinue") {
				return fmt.Errorf("lifecycle must run in Windows PowerShell 5.1 without erasing pre-existing state")
			}
			return nil
		}
	}
	return fmt.Errorf("native lifecycle step missing")
}

func TestWindowsAgentLifecycleRequiresExactEngineAndIndependentAbsence(t *testing.T) {
	content, err := os.ReadFile(repoFile("scripts", "installtests", "windows_agent_lifecycle.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	harness := string(content)
	steps := nativeWindowsExitSteps(t)
	if err := windowsNativeAcceptanceContract(harness, steps); err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{
		`$PSVersionTable.PSVersion.Minor -ne 1`,
		`return Get-Service -Name $serviceName -ErrorAction Stop`,
		`$_.FullyQualifiedErrorId.Split(',')[0] -eq 'NoServiceFoundForGivenName'`,
		`$_.CategoryInfo.Category -eq [System.Management.Automation.ErrorCategory]::ObjectNotFound`,
		`$remaining = Get-LifecycleService`,
		"$previousDisableAutoUpdate = $null\n$restoreAutoUpdateAtExit = $false",
		`Stop-Process -Id $script:serverProcess.Id -Force -ErrorAction Stop`,
		`if (-not $script:serverProcess.WaitForExit(5000)) {`,
		`$listeners = @(Get-NetTCPConnection -ErrorAction Stop | Where-Object {`,
		`foreach ($label in @('uninstall', 'repeated uninstall')) {`,
	} {
		t.Run(needle, func(t *testing.T) {
			if err := windowsNativeAcceptanceContract(strings.Replace(harness, needle, "", 1), steps); err == nil {
				t.Fatal("accepted removed engine, readback or repeated-uninstall boundary")
			}
		})
	}
	for _, mutation := range []string{"pwsh", "erase", "tolerate"} {
		t.Run(mutation, func(t *testing.T) {
			changed := append([]nativeWindowsExitStep(nil), steps...)
			for i := range changed {
				if changed[i].Name != "Exercise native Windows service lifecycle" {
					continue
				}
				switch mutation {
				case "pwsh":
					changed[i].Shell = "pwsh"
				case "erase":
					changed[i].Run += "\nRemove-Item $lifecycleStateDir -Recurse -Force"
				case "tolerate":
					changed[i].ContinueOnError = true
				}
			}
			if err := windowsNativeAcceptanceContract(harness, changed); err == nil {
				t.Fatal("accepted unsafe native proof route")
			}
		})
	}
	if parentPath := os.Getenv("PULSE_WINDOWS_LIFECYCLE_PARENT"); parentPath != "" {
		parent, err := os.ReadFile(parentPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := windowsNativeAcceptanceContract(string(parent), steps); err == nil {
			t.Fatal("accepted original single-uninstall suppressed-read proof")
		}
		t.Log("rejected complete supplied parent lifecycle harness")
	}
}

func TestWindowsAgentLifecycleObservationRuntime(t *testing.T) {
	powerShell := nativeInstallerPowerShell(t)
	cmd := exec.Command(powerShell, "-NoLogo", "-NoProfile", "-NonInteractive", "-File",
		repoFile("scripts", "installtests", "windows_lifecycle_observation_controls.ps1"),
		"-HarnessPath", repoFile("scripts", "installtests", "windows_agent_lifecycle.ps1"))
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "PASS all 14 lifecycle observation controls") {
		t.Fatalf("actual lifecycle observation functions rejected their controls: %v\n%s", err, output)
	}
	t.Logf("%s", output)
}
