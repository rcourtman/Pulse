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

// Exercise the actual sourced installer entrypoints. systemctl is an on-disk
// stateful double, so the real timeout wrapper runs it; destructive commands
// can affect only per-test files, never an installed service or host directory.
func TestRootInstallRemovalPreservesFilesOnUnconfirmedUnits(t *testing.T) {
	cases := []struct{ name, mode, target string }{
		{"initial_query_failure", "query-fails", "pulse.service"},
		{"empty_load", "empty-load", "pulse.service"},
		{"unknown_load", "unknown-load", "pulse.service"},
		{"empty_active", "empty-active", "pulse.service"},
		{"unsettled_active", "unsettled", "pulse.service"},
		{"stop_failure", "stop-fails", "pulse.service"},
		{"ignored_stop", "stop-ignored", "pulse.service"},
		{"failed_after_stop", "still-failed", "pulse.service"},
		{"stop_readback_failure", "readback-fails", "pulse.service"},
		{"disable_failure", "disable-fails", "pulse.service"},
		{"ignored_disable", "disable-ignored", "pulse.service"},
		{"empty_enabled_state", "empty-enabled", "pulse.service"},
		{"disable_readback_failure", "enabled-readback-fails", "pulse.service"},
		{"timer_stop_failure", "stop-fails", "pulse-update.timer"},
		{"inflight_update_stop_failure", "stop-fails", "pulse-update.service"},
		{"legacy_alias_stop_failure", "stop-fails", "pulse-backend.service"},
		{"timer_reactivated", "reactivated", "pulse-update.timer"},
		{"timer_reenabled", "reenabled", "pulse-update.timer"},
		{"legacy_proxy_stop_failure", "stop-fails", "pulse-sensor-proxy.service"},
	}
	for _, flow := range []string{"menu", "uninstall"} {
		for _, tc := range cases {
			t.Run(flow+"/"+tc.name, func(t *testing.T) {
				out, calls, err := runServerRemovalFixture(t, flow, tc.mode, tc.target, "active", false, true)
				if err == nil {
					t.Fatalf("expected rejected removal, got success:\n%s\n%s", out, calls)
				}
				if strings.Contains(calls, "DELETE ") || strings.Contains(calls, "start ") || strings.Contains(out, "successfully") || strings.Contains(out, "completely uninstalled") {
					t.Fatalf("unconfirmed unit allowed deletion, restart or completion:\n%s\n%s", out, calls)
				}
			})
		}
	}
}

func TestRootInstallRemovalQuiescesUpdaterAndAliases(t *testing.T) {
	for _, flow := range []string{"menu", "uninstall"} {
		for _, state := range []string{"active", "inactive", "failed", "not-found", "masked"} {
			t.Run(flow+"/"+state, func(t *testing.T) {
				out, calls, err := runServerRemovalFixture(t, flow, "normal", "", state, false, true)
				if err != nil {
					t.Fatalf("safe removal failed: %v\n%s\n%s", err, out, calls)
				}
				deleteAt := strings.Index(calls, "DELETE ")
				if deleteAt < 0 {
					t.Fatalf("removal did not reach file deletion:\n%s", calls)
				}
				for _, unit := range []string{"pulse-update.timer", "pulse-update.service", "pulse.service", "pulse-backend.service", "pulse-sensor-proxy.service"} {
					if !strings.Contains(calls[:deleteAt], "show "+unit+" --property=ActiveState") {
						t.Fatalf("%s was not observed before deletion:\n%s", unit, calls)
					}
					if state != "not-found" && !strings.Contains(calls[:deleteAt], "show "+unit+" --property=UnitFileState") {
						t.Fatalf("%s disablement was not observed:\n%s", unit, calls)
					}
				}
				if state == "active" || state == "failed" {
					timerAt := strings.Index(calls, "stop pulse-update.timer")
					updateAt := strings.Index(calls, "stop pulse-update.service")
					serverAt := strings.Index(calls, "stop pulse.service")
					if !(timerAt >= 0 && timerAt < updateAt && updateAt < serverAt && serverAt < deleteAt) {
						t.Fatalf("unsafe stop ordering:\n%s", calls)
					}
				}
				if strings.Contains(calls, "start ") || (state == "not-found" && strings.Contains(calls, "disable ")) || (state == "inactive" && strings.Contains(calls, "stop ")) {
					t.Fatalf("unnecessary service mutation:\n%s", calls)
				}
			})
		}
	}
}

func TestRootInstallRemovalKeepsOtherInstanceUntouched(t *testing.T) {
	for _, flow := range []string{"menu", "uninstall"} {
		t.Run(flow, func(t *testing.T) {
			out, calls, err := runServerRemovalFixture(t, flow, "normal", "", "active", true, false)
			if err != nil {
				t.Fatalf("custom instance removal: %v\n%s", err, out)
			}
			for _, other := range []string{"pulse.service", "pulse-backend.service", "pulse-update.timer", "pulse-update.service"} {
				if strings.Contains(calls, " "+other) || strings.Contains(calls, "/"+other) {
					t.Fatalf("custom-instance removal touched %s:\n%s", other, calls)
				}
			}
		})
	}
}

func runServerRemovalFixture(t *testing.T, flow, mode, target, state string, custom, legacy bool) (string, string, error) {
	t.Helper()
	dir := t.TempDir()
	binDir := filepath.Join(dir, "tools")
	if err := os.Mkdir(binDir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"install", "config", "proxy"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, d, "preserve"), []byte("original-"+d), 0600); err != nil {
			t.Fatal(err)
		}
	}
	service := "pulse"
	if custom {
		service = "pulse-other"
	}
	units := []string{service + ".service", service + "-update.service", service + "-update.timer"}
	if !custom {
		units = append(units, "pulse-backend.service")
	}
	if legacy {
		units = append(units, "pulse-sensor-proxy.service", "pulse-sensor-proxy-selfheal.timer", "pulse-sensor-proxy-selfheal.service", "pulse-sensor-cleanup.path", "pulse-sensor-cleanup.service")
	}
	for _, unit := range units {
		load, active, enabled := "loaded", state, "enabled"
		if state == "not-found" {
			load, active, enabled = "not-found", "inactive", ""
		}
		if state == "masked" {
			load, active, enabled = "masked", "inactive", "masked"
		}
		for suffix, val := range map[string]string{"load": load, "active": active, "enabled": enabled} {
			if err := os.WriteFile(filepath.Join(dir, unit+"."+suffix), []byte(val+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	mock := `#!/bin/bash
set -eu
printf '%s\n' "$*" >> "$FIXTURE_DIR/calls"
action=$1; unit=${2:-}
if [[ "$action" == show ]]; then
    prop=$3
    if [[ "$unit" == "$TARGET" ]]; then
        case "$MODE:$prop" in
          query-fails:*) exit 1 ;;
          empty-load:--property=LoadState|empty-active:--property=ActiveState|empty-enabled:--property=UnitFileState) exit 0 ;;
          unknown-load:--property=LoadState) echo error; exit 0 ;;
          unsettled:--property=ActiveState) echo activating; exit 0 ;;
          readback-fails:--property=ActiveState) [[ ! -e "$FIXTURE_DIR/$unit.stopped" ]] || exit 1 ;;
          enabled-readback-fails:--property=UnitFileState) exit 1 ;;
        esac
        if [[ -e "$FIXTURE_DIR/$SERVICE.service.disabled" ]]; then
            case "$MODE:$prop" in
              reactivated:--property=ActiveState) echo active; exit 0 ;;
              reenabled:--property=UnitFileState) echo enabled; exit 0 ;;
            esac
        fi
    fi
    case "$prop" in
      --property=LoadState) cat "$FIXTURE_DIR/$unit.load" ;;
      --property=ActiveState) cat "$FIXTURE_DIR/$unit.active" ;;
      --property=UnitFileState) cat "$FIXTURE_DIR/$unit.enabled" ;;
      *) exit 2 ;;
    esac
elif [[ "$action" == stop ]]; then
    [[ "$unit:$MODE" != "$TARGET:stop-fails" ]] || exit 1
    touch "$FIXTURE_DIR/$unit.stopped"
    if [[ "$unit:$MODE" == "$TARGET:still-failed" ]]; then
        echo failed > "$FIXTURE_DIR/$unit.active"
    elif [[ "$unit:$MODE" != "$TARGET:stop-ignored" ]]; then
        echo inactive > "$FIXTURE_DIR/$unit.active"
    fi
elif [[ "$action" == disable ]]; then
    [[ "$unit:$MODE" != "$TARGET:disable-fails" ]] || exit 1
    touch "$FIXTURE_DIR/$unit.disabled"
    if [[ "$unit:$MODE" != "$TARGET:disable-ignored" ]]; then
        case "$(cat "$FIXTURE_DIR/$unit.enabled")" in masked) ;; *) echo disabled > "$FIXTURE_DIR/$unit.enabled" ;; esac
    fi
elif [[ "$action" != daemon-reload ]]; then
    exit 2
fi
`
	if err := os.WriteFile(filepath.Join(binDir, "systemctl"), []byte(mock), 0700); err != nil {
		t.Fatal(err)
	}
	installer := os.Getenv("PULSE_TEST_ROOT_INSTALLER")
	if installer == "" {
		installer = filepath.Join("..", "..", "install.sh")
	}
	installer, err := filepath.Abs(installer)
	if err != nil {
		t.Fatal(err)
	}
	script := `
source "$INSTALLER_UNDER_TEST"
CONFIG_DIR="$FIXTURE_DIR/config"
INSTALL_DIR="$FIXTURE_DIR/install"
BINARY_LINK_PATH="$FIXTURE_DIR/link"
AUTO_UPDATE_DEST="$FIXTURE_DIR/updater"
UPDATE_HELPER_PATH="$FIXTURE_DIR/helper"
UPDATE_SERVICE_PATH="$FIXTURE_DIR/$SERVICE-update.service"
UPDATE_TIMER_PATH="$FIXTURE_DIR/$SERVICE-update.timer"
UPDATE_SERVICE_UNIT="$SERVICE-update.service"
UPDATE_TIMER_UNIT="$SERVICE-update.timer"
SERVICE_NAME="$SERVICE"
SERVICE_NAME_EXPLICIT="$CUSTOM"
SENSOR_PROXY_BINARY_PATH="$FIXTURE_DIR/proxy/binary"
SENSOR_PROXY_INSTALL_ROOT="$FIXTURE_DIR/proxy"
SENSOR_PROXY_RUNTIME_DIR="$FIXTURE_DIR/proxy/run"
SENSOR_PROXY_WORK_DIR="$FIXTURE_DIR/proxy/work"
SENSOR_PROXY_CONFIG_DIR="$FIXTURE_DIR/proxy/config"
SENSOR_PROXY_LOG_DIR="$FIXTURE_DIR/proxy/log"
SENSOR_PROXY_AUTHORIZED_KEYS_PATH="$FIXTURE_DIR/proxy/absent-keys"
SENSOR_PROXY_SYSTEMD_DIR="$FIXTURE_DIR/proxy/units"
IN_CONTAINER=true
IN_DOCKER=false
CURRENT_VERSION=v6.5.0
FORCE_VERSION=""
for fn in print_header check_root detect_os check_docker_environment; do eval "$fn() { :; }"; done
check_existing_installation() { return 0; }
detect_service_name() { echo "$SERVICE"; }
resolve_latest_release_tag_for_channel() { :; }
read_configured_update_channel() { echo stable; }
curl() { :; }
local_sensor_proxy_present() { [[ "$LEGACY" == true ]]; }
id() { return 1; }
getent() { return 1; }
userdel() { echo USER_DELETE >> "$FIXTURE_DIR/calls"; }
rm() {
  echo "DELETE $*" >> "$FIXTURE_DIR/calls"
  # Hard-coded system paths are observed but never mutated. All actual removals
  # must be in the fixture; unexpected options are a fixture error.
  local arg
  for arg in "$@"; do
    case "$arg" in -f|-rf) ;; "$FIXTURE_DIR"/*) /bin/rm -rf -- "$arg" ;; /etc/systemd/system/*|/usr/local/bin/pulse-sensor-cleanup.sh|/var/log/pulse*.log|/opt/pulse.log) ;; *) return 91 ;; esac
  done
}
safe_read() { printf -v "$2" '%s' 2; }
safe_read_with_default() { printf -v "$2" '%s' y; }
if [[ "$FLOW" == menu ]]; then main; else uninstall_pulse; fi
`
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"), "INSTALLER_UNDER_TEST="+installer, "FIXTURE_DIR="+dir, "FLOW="+flow, "MODE="+mode, "TARGET="+target, "SERVICE="+service, "CUSTOM="+boolString(custom), "LEGACY="+boolString(legacy))
	out, runErr := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("fixture deadline: %v\n%s", ctx.Err(), out)
	}
	calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
	if mode != "normal" {
		for _, d := range []string{"install", "config", "proxy"} {
			b, e := os.ReadFile(filepath.Join(dir, d, "preserve"))
			if e != nil || string(b) != "original-"+d {
				t.Fatalf("unconfirmed removal changed %s: %v\n%s\n%s", d, e, out, calls)
			}
		}
	}
	return string(out), string(calls), runErr
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
