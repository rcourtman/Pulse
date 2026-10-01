package installtests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the real main flow and read-default wrapper, not a model of the
// prompt. External install mutations are stubs confined to each fixture.
func TestRootInstallExistingAutoUpdatesRequireAffirmativeChoice(t *testing.T) {
	type scenario struct {
		name, config, answer                            string
		timer, enabled, eof, explicit, optIn, wantSetup bool
	}
	cases := []scenario{
		{name: "missing_timer_enter"},
		{name: "missing_timer_non_tty", eof: true},
		{name: "disabled_enter", timer: true, config: `{"autoUpdateEnabled":false}`},
		{name: "disabled_non_tty", timer: true, config: `{"autoUpdateEnabled":false}`, eof: true},
		{name: "disabled_no", timer: true, config: `{"autoUpdateEnabled":false}`, answer: "n"},
		{name: "invalid_answer", answer: "maybe"},
		{name: "enabled_preserved", timer: true, enabled: true, config: `{"autoUpdateEnabled":true}`},
		{name: "disabled_timer_preserved", timer: true, config: `{"autoUpdateEnabled":true}`},
		{name: "explicit_disable", explicit: true, answer: "y"},
		{name: "explicit_enable", explicit: true, optIn: true, wantSetup: true},
		{name: "affirmative_y", answer: "y", wantSetup: true},
		{name: "affirmative_yes", answer: "Yes", timer: true, config: `{"autoUpdateEnabled":false}`, wantSetup: true},
	}
	for _, flow := range []string{"version_upgrade", "version_rollback", "version_reinstall", "menu_update", "menu_reinstall"} {
		for _, tc := range cases {
			t.Run(flow+"/"+tc.name, func(t *testing.T) {
				dir := t.TempDir()
				config := filepath.Join(dir, "system.json")
				if tc.config != "" {
					if err := os.WriteFile(config, []byte(tc.config), 0600); err != nil {
						t.Fatal(err)
					}
				}
				installer, err := filepath.Abs(filepath.Join("..", "..", "install.sh"))
				if err != nil {
					t.Fatal(err)
				}
				script := `
source "$INSTALLER_UNDER_TEST"
CONFIG_DIR="$FIXTURE_DIR"
INSTALL_DIR="$FIXTURE_DIR/install"
CURRENT_VERSION=v6.4.5
LATEST_RELEASE=v6.4.6
FORCE_VERSION=""
case "$FLOW" in
  version_upgrade) FORCE_VERSION=v6.4.6 ;;
  version_rollback) FORCE_VERSION=v6.4.1 ;;
  version_reinstall) FORCE_VERSION=v6.4.5 ;;
esac
AUTO_UPDATE_CHOICE_EXPLICIT="$EXPLICIT"
ENABLE_AUTO_UPDATES="$OPT_IN"
UPDATE_CHANNEL=stable
for fn in print_header check_root detect_os check_docker_environment backup_existing stop_pulse_for_update create_user download_pulse setup_directories setup_update_command ensure_systemd_service_installed install_systemd_service start_pulse; do
  eval "$fn() { :; }"
done
check_proxmox_host() { return 1; }
check_existing_installation() { return 0; }
detect_service_name() { echo pulse; }
resolve_latest_release_tag_for_channel() { echo v6.4.6; }
read_configured_update_channel() { echo stable; }
curl() { :; }
timeout() { shift; "$@"; }
run_upgrade_readiness_preflight() { return 0; }
update_timer_exists() { [[ "$TIMER" == true ]]; }
refresh_auto_updates() { echo REFRESH; }
setup_auto_updates() {
  echo SETUP
  printf '{"autoUpdateEnabled":true}' > "$CONFIG_DIR/system.json"
  TIMER=true
  TIMER_ENABLED=true
}
create_marker_file() { echo INSTALL_FINISHED; }
print_completion() { printf 'TIMER_ENABLED=%s\n' "$TIMER_ENABLED"; }
safe_read() {
  if [[ "$1" == "Select option "* ]]; then
    local selection=1
    [[ "$FLOW" == menu_reinstall ]] && selection=2
    printf -v "$2" '%s' "$selection"
  else
    echo AUTO_UPDATE_PROMPT
    [[ "$READ_EOF" == true ]] && return 1
    printf -v "$2" '%s' "$ANSWER"
  fi
}
main
`
				cmd := exec.Command("bash", "-c", script)
				cmd.Env = append(os.Environ(), "INSTALLER_UNDER_TEST="+installer, "FIXTURE_DIR="+dir,
					"FLOW="+flow, "TIMER="+fmt.Sprint(tc.timer), "TIMER_ENABLED="+fmt.Sprint(tc.enabled),
					"EXPLICIT="+fmt.Sprint(tc.explicit), "OPT_IN="+fmt.Sprint(tc.optIn),
					"READ_EOF="+fmt.Sprint(tc.eof), "ANSWER="+tc.answer)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("main: %v\n%s", err, out)
				}
				if !strings.Contains(string(out), "INSTALL_FINISHED") {
					t.Fatalf("install did not complete:\n%s", out)
				}
				if got := strings.Contains(string(out), "\nSETUP\n"); got != tc.wantSetup {
					t.Fatalf("auto-update setup=%v, want %v:\n%s", got, tc.wantSetup, out)
				}
				if !tc.wantSetup {
					got, readErr := os.ReadFile(config)
					if tc.config == "" {
						if !os.IsNotExist(readErr) {
							t.Fatalf("absent config was changed: %s, %v", got, readErr)
						}
					} else if readErr != nil || string(got) != tc.config {
						t.Fatalf("existing choice was changed: %s, %v", got, readErr)
					}
					if !strings.Contains(string(out), "TIMER_ENABLED="+fmt.Sprint(tc.enabled)) {
						t.Fatalf("timer intent changed:\n%s", out)
					}
					if tc.timer && !strings.Contains(string(out), "REFRESH") {
						t.Fatalf("existing helper was not refreshed:\n%s", out)
					}
				}
				if (tc.explicit || tc.enabled) && strings.Contains(string(out), "AUTO_UPDATE_PROMPT") {
					t.Fatalf("existing/explicit choice was re-prompted:\n%s", out)
				}
			})
		}
	}
}

func TestRootInstallSafeReadNonTTYDefaultsToNoAutoUpdates(t *testing.T) {
	setsid, err := exec.LookPath("setsid")
	if err != nil {
		t.Skip("setsid unavailable for a controlling-terminal-free read")
	}
	script := `set -euo pipefail
print_info() { :; }
` + extractRootInstallShellFunction(t, "safe_read") + "\n" + extractRootInstallShellFunction(t, "safe_read_with_default") + `
answer=""
safe_read_with_default "Enable auto-updates? [y/N]: " answer "n"
[[ "$answer" == n ]]
`
	out, err := exec.Command(setsid, "bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("non-TTY fallback: %v\n%s", err, out)
	}
}
