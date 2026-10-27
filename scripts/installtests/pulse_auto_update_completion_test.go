package installtests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exercise the real main entry after a controlled installer result. Discovery,
// installation and logging are private doubles: no service, provider, journal
// or notification destination is contacted. The transaction tests separately
// establish what a successful perform_update result requires.
func TestAutoUpdateCompletionDoesNotCallAnUnregisteredNotificationRoute(t *testing.T) {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		t.Fatal("updater main-flow proof requires a non-Docker environment")
	}
	updater := os.Getenv("PULSE_AUTO_UPDATE_COMPLETION_SOURCE")
	if updater == "" {
		updater = repoFile("scripts", "pulse-auto-update.sh")
	}
	updater, err := filepath.Abs(updater)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, configFile string
		installerExit    string
		current          bool
	}{
		{name: "without_legacy_webhooks", installerExit: "0"},
		{name: "legacy_encrypted_webhooks", configFile: "webhooks.enc", installerExit: "0"},
		{name: "legacy_json_webhooks", configFile: "webhooks.json", installerExit: "0"},
		{name: "installer_failure", configFile: "webhooks.json", installerExit: "17"},
		{name: "already_current", configFile: "webhooks.enc", installerExit: "0", current: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.configFile != "" {
				if err := os.WriteFile(filepath.Join(dir, tc.configFile), []byte("synthetic legacy configuration\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			latest := "v6.6.1"
			if tc.current {
				latest = "v6.6.0"
			}
			script := `source "$UPDATER_UNDER_TEST"
log() { printf '[%s] %s\n' "$1" "${*:2}"; }
check_auto_updates_enabled() { :; }
installed_binary_is_pulse_pro() { return 1; }
get_current_version() { echo v6.6.0; }
get_latest_stable_version() { printf '%s\n' "$LATEST"; }
perform_update() {
    printf '%s\n' "$*" >> "$FIXTURE/installer-calls"
    return "$INSTALLER_EXIT"
}
curl() {
    printf '%s\n' "$*" >> "$FIXTURE/unexpected-network"
    return 0
}
main
`
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", "-c", script)
			cmd.Env = append(os.Environ(), "UPDATER_UNDER_TEST="+updater,
				"FIXTURE="+dir, "PULSE_CONFIG_DIR="+dir, "LATEST="+latest,
				"INSTALLER_EXIT="+tc.installerExit)
			out, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("main did not terminate: %v\n%s", ctx.Err(), out)
			}
			exit := 0
			if err != nil {
				failure, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatal(err)
				}
				exit = failure.ExitCode()
			}
			wantExit := 0
			if tc.installerExit != "0" {
				wantExit = 1
			}
			if exit != wantExit || strings.Contains(string(out), "Auto-update check completed") != (wantExit == 0) {
				t.Fatalf("completion exit=%d want=%d:\n%s", exit, wantExit, out)
			}
			calls, readErr := os.ReadFile(filepath.Join(dir, "installer-calls"))
			if tc.current {
				if !os.IsNotExist(readErr) || !strings.Contains(string(out), "Already running latest version") {
					t.Fatalf("current install was changed: calls=%q error=%v\n%s", calls, readErr, out)
				}
			} else if readErr != nil || string(calls) != latest+"\n" {
				t.Fatalf("lost or repeated exact installer handoff: calls=%q error=%v", calls, readErr)
			}
			if tc.installerExit == "0" && !tc.current && !strings.Contains(string(out), "Update completed successfully to "+latest) {
				t.Fatalf("lost verified update completion:\n%s", out)
			}
			if wantExit != 0 && (strings.Contains(string(out), "Update completed successfully") || !strings.Contains(string(out), "Update failed")) {
				t.Fatalf("failed installer was reported as successful:\n%s", out)
			}
			if calls, readErr := os.ReadFile(filepath.Join(dir, "unexpected-network")); !os.IsNotExist(readErr) {
				t.Fatalf("completion attempted an unsupported network request: %q (read=%v)", calls, readErr)
			}
			if tc.configFile != "" {
				bytes, err := os.ReadFile(filepath.Join(dir, tc.configFile))
				if err != nil || string(bytes) != "synthetic legacy configuration\n" {
					t.Fatalf("legacy notification configuration changed: %q, %v", bytes, err)
				}
			}
		})
	}
}
