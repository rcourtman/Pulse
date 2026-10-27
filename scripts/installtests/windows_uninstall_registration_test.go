package installtests

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestInstallPS1UninstallRegistrationRuntime(t *testing.T) {
	powerShell := nativeInstallerPowerShell(t)
	for _, test := range []struct {
		name           string
		ok             bool
		requests       int
		credentialRead bool
	}{
		{"saved-success", true, 1, true},
		{"legacy-inline-token", true, 1, false},
		{"agent-id-file", true, 1, true},
		{"explicit-identity", true, 1, false},
		{"explicit-token-file", true, 1, true},
		{"optional-auth", true, 1, false},
		{"repeated", true, 0, false},
		{"saved-ca", true, 1, true},
		{"saved-insecure", true, 1, true},
		{"service-fails", false, 0, false},
		{"missing-url", false, 0, true},
		{"missing-id", false, 0, true},
		{"unreadable-token", false, 0, false},
		{"empty-token", false, 0, false},
		{"invalid-ca", false, 0, true},
		{"tls-fails", false, 0, true},
		{"http-401", false, 1, true},
		{"http-403", false, 1, true},
		{"http-500", false, 1, true},
		{"timeout", false, 1, true},
		{"wrong-id", false, 1, true},
		{"wrong-case-id", false, 1, true},
		{"false-success", false, 1, true},
		{"string-success", false, 1, true},
		{"missing-id-response", false, 1, true},
		{"empty-response", false, 1, true},
		{"array-response", false, 1, true},
		{"cleanup-fails", false, 1, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, powerShell, "-NoLogo", "-NoProfile", "-NonInteractive", "-File",
				repoFile("scripts", "installtests", "windows_uninstall_registration_controls.ps1"),
				"-InstallerPath", repoFile("scripts", "install.ps1"), "-Scenario", test.name)
			output, err := cmd.CombinedOutput()
			text := string(output)
			if (err == nil) != test.ok || (strings.Contains(text, "Uninstallation complete.")) != test.ok {
				t.Fatalf("unexpected complete-branch outcome: %v\n%s", err, text)
			}
			if !test.ok {
				if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
					t.Fatalf("failure did not retain exit 1: %v\n%s", err, text)
				}
			}
			if strings.Count(text, "EVENT request") != test.requests || strings.Count(text, "EVENT service") != 1 ||
				strings.Contains(text, "EVENT credential-read") != test.credentialRead {
				t.Fatalf("wrong service, identity/credential or one-POST boundary: %s", text)
			}
			if strings.Contains(text, "synthetic-private-token-never-print") {
				t.Fatal("private credential or remote exception was printed")
			}
			if !test.ok && test.name != "cleanup-fails" && strings.Contains(text, "EVENT erase") {
				t.Fatalf("erased recovery material after a refusal: %s", text)
			}
			if test.ok && test.name != "repeated" && strings.Count(text, "EVENT erase") != 2 {
				t.Fatalf("confirmed removal did not clean binary and state: %s", text)
			}
			if test.name == "saved-ca" && strings.Count(text, "EVENT ca-dispose") != 1 {
				t.Fatalf("saved CA was not disposed: %s", text)
			}
			t.Logf("%s: expected complete-branch outcome, one-POST/retention/secret controls pass", test.name)
		})
	}
}
