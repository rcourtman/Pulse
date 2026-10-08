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

// This exercises reset_pulse from the actual sourced installer. All systemctl
// calls pass through the real timeout binary to a stateful executable double.
// Deletion is confined to fixture configuration; no host installation is used.
func TestRootInstallResetRejectsUnusableObservationsBeforeMutation(t *testing.T) {
	for _, unit := range []string{"pulse.service", "pulse-update.timer", "pulse-update.service", "pulse-backend.service", "pulse-backend-update.timer", "pulse-backend-update.service"} {
		for _, mode := range []string{"query-fails", "empty-load", "unknown-load", "empty-active", "unsettled", "empty-enabled", "unknown-enabled", "enabled-query-fails"} {
			t.Run(unit+"/"+mode, func(t *testing.T) {
				r := runServerResetFixture(t, mode, unit, "active", "enabled", false)
				assertResetIncomplete(t, r, true)
				for _, action := range []string{"stop ", "disable ", "start ", "enable "} {
					if strings.Contains(r.calls, action) {
						t.Fatalf("bad initial observation allowed mutation %s:\n%s", action, r.calls)
					}
				}
			})
		}
	}
}

func TestRootInstallResetPreservesDataOnUnconfirmedQuiescence(t *testing.T) {
	for _, unit := range []string{"pulse.service", "pulse-update.timer", "pulse-update.service", "pulse-backend.service", "pulse-backend-update.timer", "pulse-backend-update.service"} {
		for _, mode := range []string{"stop-fails", "stop-ignored", "stop-readback-fails", "still-failed", "reactivated"} {
			t.Run(unit+"/"+mode, func(t *testing.T) {
				assertResetIncomplete(t, runServerResetFixture(t, mode, unit, "active", "enabled", false), true)
			})
		}
	}
	for _, unit := range []string{"pulse-update.timer", "pulse-backend-update.timer"} {
		for _, mode := range []string{"disable-fails", "disable-ignored", "reenabled"} {
			t.Run(unit+"/"+mode, func(t *testing.T) {
				assertResetIncomplete(t, runServerResetFixture(t, mode, unit, "active", "enabled", false), true)
			})
		}
	}
}

func TestRootInstallResetRestoresOnlyPriorActiveStates(t *testing.T) {
	for _, active := range []string{"active", "inactive", "failed"} {
		for _, enabled := range []string{"enabled", "enabled-runtime", "disabled", "static", "indirect", "masked", "masked-runtime", "linked", "linked-runtime"} {
			t.Run(active+"/"+enabled, func(t *testing.T) {
				r := runServerResetFixture(t, "normal", "", active, enabled, false)
				if r.err != nil || !strings.Contains(r.output, "Pulse configuration has been reset") || r.configPresent {
					t.Fatalf("reset failed: %v\n%s\n%s", r.err, r.output, r.calls)
				}
				deleteAt := strings.Index(r.calls, "DELETE ")
				if deleteAt < 0 {
					t.Fatal("reset did not delete configuration")
				}
				for _, unit := range resetFixtureUnits(false) {
					if strings.Count(r.calls[:deleteAt], "show "+unit+" --property=ActiveState") < 3 {
						t.Fatalf("%s lacked snapshot, stop readback and final pre-delete reconciliation:\n%s", unit, r.calls)
					}
					expected := active
					if active == "failed" || strings.HasSuffix(unit, "-update.service") {
						expected = "inactive"
					}
					if r.states[unit+".active"] != expected || r.states[unit+".enabled"] != enabled {
						t.Fatalf("%s not restored: %v", unit, r.states)
					}
					if strings.HasSuffix(unit, "-update.service") && strings.Contains(r.calls, "start "+unit) {
						t.Fatalf("interrupted updater was replayed:\n%s", r.calls)
					}
				}
				if active != "active" && strings.Contains(r.calls, "start ") {
					t.Fatalf("originally inactive/failed unit was started:\n%s", r.calls)
				}
				if active == "active" {
					firstTimer := strings.Index(r.calls, "stop pulse-update.timer")
					lastTimer := strings.Index(r.calls, "stop pulse-backend-update.timer")
					firstUpdate := strings.Index(r.calls, "stop pulse-update.service")
					lastUpdate := strings.Index(r.calls, "stop pulse-backend-update.service")
					server := strings.Index(r.calls, "stop pulse.service")
					if !(firstTimer < lastTimer && lastTimer < firstUpdate && firstUpdate < lastUpdate && lastUpdate < server && server < deleteAt) {
						t.Fatalf("unsafe writer stop ordering:\n%s", r.calls)
					}
					if !strings.Contains(r.output, "remains stopped and was not replayed") {
						t.Fatalf("interrupted updater disposition is hidden:\n%s", r.output)
					}
				}
				if enabled == "enabled-runtime" && (!strings.Contains(r.calls, "disable --runtime pulse-update.timer") || !strings.Contains(r.calls, "enable --runtime pulse-update.timer")) {
					t.Fatalf("runtime enablement was not preserved:\n%s", r.calls)
				}
				for _, unit := range []string{"pulse.service", "pulse-backend.service", "pulse-update.service", "pulse-backend-update.service"} {
					if strings.Contains(r.calls, "enable "+unit) || strings.Contains(r.calls, "disable "+unit) {
						t.Fatalf("non-timer enablement was changed:\n%s", r.calls)
					}
				}
			})
		}
	}
}

func TestRootInstallResetReportsIncompleteDeletionOrRestoration(t *testing.T) {
	for _, tc := range []struct{ mode, target string }{
		{"delete-fails", ""},
		{"start-fails", "pulse.service"},
		{"start-ignored", "pulse.service"},
		{"start-readback-fails", "pulse.service"},
		{"enable-fails", "pulse-update.timer"},
		{"enable-ignored", "pulse-update.timer"},
		{"start-fails", "pulse-update.timer"},
		{"start-ignored", "pulse-update.timer"},
		{"start-readback-fails", "pulse-update.timer"},
		{"final-updater-active", "pulse-update.service"},
	} {
		t.Run(tc.mode+"/"+tc.target, func(t *testing.T) {
			r := runServerResetFixture(t, tc.mode, tc.target, "active", "enabled", false)
			assertResetIncomplete(t, r, tc.mode == "delete-fails")
			if tc.mode == "delete-fails" || tc.target == "pulse.service" {
				if strings.Contains(r.calls, "start pulse-update.timer") || strings.Contains(r.calls, "enable pulse-update.timer") {
					t.Fatalf("timer restored despite uncertain deletion/server recovery:\n%s", r.calls)
				}
			}
		})
	}
}

func TestRootInstallResetEnforcesSystemdDeadlines(t *testing.T) {
	for _, mode := range []string{"query-timeout", "stop-timeout"} {
		t.Run(mode, func(t *testing.T) {
			start := time.Now()
			r := runServerResetFixture(t, mode, "pulse-update.service", "active", "enabled", false)
			assertResetIncomplete(t, r, true)
			if elapsed := time.Since(start); elapsed >= 15*time.Second {
				t.Fatalf("bounded systemctl action took %s", elapsed)
			}
		})
	}
}

func TestRootInstallResetDoesNotStartAnIdleServerWhenTimerWasActive(t *testing.T) {
	r := runServerResetFixture(t, "mixed", "", "active", "enabled", false)
	if r.err != nil || r.configPresent || strings.Contains(r.calls, "start pulse.service") || strings.Contains(r.calls, "start pulse-backend.service") {
		t.Fatalf("idle/failed server was restarted: %v\n%s\n%s", r.err, r.output, r.calls)
	}
	if !strings.Contains(r.calls, "start pulse-update.timer") || r.states["pulse.service.active"] != "inactive" {
		t.Fatalf("timer/server states not independently restored: %v\n%s", r.states, r.calls)
	}
}

func TestRootInstallResetKeepsCustomInstanceIsolated(t *testing.T) {
	r := runServerResetFixture(t, "normal", "", "active", "enabled", true)
	if r.err != nil || r.configPresent {
		t.Fatalf("custom reset failed: %v\n%s\n%s", r.err, r.output, r.calls)
	}
	for _, unit := range resetFixtureUnits(false) {
		if strings.Contains(r.calls, unit) {
			t.Fatalf("custom reset touched other installation %s:\n%s", unit, r.calls)
		}
	}
}

func TestRootInstallResetKeepsUpdateSuffixedServerRole(t *testing.T) {
	r := runServerResetFixture(t, "update-named-server", "", "active", "enabled", true)
	if r.err != nil || r.states["pulse-other-update.service.active"] != "active" || !strings.Contains(r.calls, "start pulse-other-update.service") || strings.Contains(r.calls, "start pulse-other-update-update.service") {
		t.Fatalf("server name was confused with updater role: %v\n%s\n%s", r.err, r.output, r.calls)
	}
}

func TestRootInstallResetHandlesAbsentOptionalUnits(t *testing.T) {
	r := runServerResetFixture(t, "optional-absent", "", "inactive", "disabled", false)
	if r.err != nil || r.configPresent || strings.Contains(r.calls, "stop ") || strings.Contains(r.calls, "start ") || strings.Contains(r.calls, "disable ") {
		t.Fatalf("absent optional units should not cause mutation: %v\n%s\n%s", r.err, r.output, r.calls)
	}
	assertResetIncomplete(t, runServerResetFixture(t, "server-absent", "", "inactive", "disabled", false), true)
}

type serverResetResult struct {
	output, calls string
	err           error
	configPresent bool
	states        map[string]string
}

func assertResetIncomplete(t *testing.T, r serverResetResult, preserve bool) {
	t.Helper()
	if r.err == nil || strings.Contains(r.output, "Pulse configuration has been reset") || strings.Contains(r.output, "Pulse has been reset") {
		t.Fatalf("incomplete reset claimed completion: %v\n%s\n%s", r.err, r.output, r.calls)
	}
	if preserve && (!r.configPresent || strings.Contains(r.calls, "start ") || (strings.Contains(r.calls, "DELETE ") && !strings.Contains(r.output, "Configuration deletion failed"))) {
		t.Fatalf("uncertain pre-deletion state allowed mutation/restart:\n%s\n%s", r.output, r.calls)
	}
}

func resetFixtureUnits(custom bool) []string {
	if custom {
		return []string{"pulse-other-update.timer", "pulse-other-update.service", "pulse-other.service"}
	}
	return []string{"pulse-update.timer", "pulse-backend-update.timer", "pulse-update.service", "pulse-backend-update.service", "pulse.service", "pulse-backend.service"}
}

func runServerResetFixture(t *testing.T, mode, target, active, enabled string, custom bool) serverResetResult {
	t.Helper()
	dir := t.TempDir()
	binDir := filepath.Join(dir, "tools")
	for _, d := range []string{binDir, filepath.Join(dir, "config"), filepath.Join(dir, "install"), filepath.Join(dir, "other-config")} {
		if err := os.Mkdir(d, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "preserve"), []byte("original"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	service := "pulse"
	if custom {
		service = "pulse-other"
	}
	if mode == "update-named-server" {
		service = "pulse-other-update"
	}
	units := resetFixtureUnits(custom)
	if mode == "update-named-server" {
		for i := range units {
			units[i] = strings.Replace(units[i], "pulse-other", service, 1)
		}
	}
	for _, unit := range units {
		load, a, e := "loaded", active, enabled
		if mode == "mixed" {
			if unit == "pulse.service" {
				a = "inactive"
			}
			if unit == "pulse-backend.service" {
				a = "failed"
			}
		}
		if mode == "server-absent" || (mode == "optional-absent" && unit != "pulse.service") {
			load, a, e = "not-found", "inactive", ""
		}
		for suffix, value := range map[string]string{"load": load, "active": a, "enabled": e} {
			if err := os.WriteFile(filepath.Join(dir, unit+"."+suffix), []byte(value+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	mock := `#!/bin/bash
set -eu
printf '%s\n' "$*" >> "$FIXTURE_DIR/calls"
action=$1; shift
runtime=false
if [[ "${1:-}" == --runtime ]]; then runtime=true; shift; fi
if [[ "${1:-}" == --quiet ]]; then shift; fi
unit=${1:-}; shift || true
[[ "$unit" == *.service || "$unit" == *.timer ]] || unit="$unit.service"
if [[ "$action" == show ]]; then
    prop=$1
    if [[ "$unit" == "$TARGET" ]]; then
        case "$MODE:$prop" in
          query-fails:*) exit 1 ;;
          query-timeout:*) sleep 7; exit 1 ;;
          empty-load:--property=LoadState|empty-active:--property=ActiveState|empty-enabled:--property=UnitFileState) exit 0 ;;
          unknown-load:--property=LoadState|unknown-enabled:--property=UnitFileState) echo unexpected; exit 0 ;;
          unsettled:--property=ActiveState) echo activating; exit 0 ;;
          enabled-query-fails:--property=UnitFileState) exit 1 ;;
          stop-readback-fails:--property=ActiveState) [[ ! -e "$FIXTURE_DIR/$unit.stopped" ]] || exit 1 ;;
          start-readback-fails:--property=ActiveState) [[ ! -e "$FIXTURE_DIR/$unit.started" ]] || exit 1 ;;
        esac
        if [[ -e "$FIXTURE_DIR/pulse-backend.service.stopped" && "$prop" == --property=ActiveState && "$MODE" == reactivated ]]; then echo active; exit 0; fi
        if [[ -e "$FIXTURE_DIR/pulse-backend.service.stopped" && "$prop" == --property=UnitFileState && "$MODE" == reenabled ]]; then echo enabled; exit 0; fi
        if [[ -e "$FIXTURE_DIR/pulse-update.timer.started" && "$prop" == --property=ActiveState && "$MODE" == final-updater-active ]]; then echo active; exit 0; fi
    fi
    case "$prop" in
      --property=LoadState) cat "$FIXTURE_DIR/$unit.load" ;;
      --property=ActiveState) cat "$FIXTURE_DIR/$unit.active" ;;
      --property=UnitFileState) cat "$FIXTURE_DIR/$unit.enabled" ;;
      *) exit 2 ;;
    esac
elif [[ "$action" == is-active ]]; then
    [[ "$(cat "$FIXTURE_DIR/$unit.active")" == active ]]
elif [[ "$action" == stop ]]; then
    [[ "$unit:$MODE" != "$TARGET:stop-timeout" ]] || { sleep 7; exit 1; }
    [[ "$unit:$MODE" != "$TARGET:stop-fails" ]] || exit 1
    touch "$FIXTURE_DIR/$unit.stopped"
    case "$unit:$MODE" in
      "$TARGET:stop-ignored") ;;
      "$TARGET:still-failed") echo failed > "$FIXTURE_DIR/$unit.active" ;;
      *) echo inactive > "$FIXTURE_DIR/$unit.active" ;;
    esac
elif [[ "$action" == disable ]]; then
    [[ "$unit:$MODE" != "$TARGET:disable-fails" ]] || exit 1
    [[ "$unit:$MODE" == "$TARGET:disable-ignored" ]] || echo disabled > "$FIXTURE_DIR/$unit.enabled"
elif [[ "$action" == enable ]]; then
    [[ "$unit:$MODE" != "$TARGET:enable-fails" ]] || exit 1
    if [[ "$unit:$MODE" != "$TARGET:enable-ignored" ]]; then
        if [[ "$runtime" == true ]]; then echo enabled-runtime; else echo enabled; fi > "$FIXTURE_DIR/$unit.enabled"
    fi
elif [[ "$action" == start ]]; then
    [[ "$unit:$MODE" != "$TARGET:start-fails" ]] || exit 1
    touch "$FIXTURE_DIR/$unit.started"
    [[ "$unit:$MODE" == "$TARGET:start-ignored" ]] || echo active > "$FIXTURE_DIR/$unit.active"
else exit 2
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
SERVICE_NAME="$SERVICE"
SERVICE_NAME_EXPLICIT="$CUSTOM"
UPDATE_SERVICE_UNIT="$SERVICE-update.service"
UPDATE_TIMER_UNIT="$SERVICE-update.timer"
check_root() { :; }
print_header() { :; }
detect_service_name() { echo "$SERVICE"; }
rm() {
  echo "DELETE $*" >> "$FIXTURE_DIR/calls"
  [[ "$MODE" != delete-fails ]] || return 1
  local arg
  for arg in "$@"; do
    case "$arg" in -rf|--) ;; "$FIXTURE_DIR/config/"*) /bin/rm -rf -- "$arg" ;; *) return 91 ;; esac
  done
}
reset_pulse
`
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"), "INSTALLER_UNDER_TEST="+installer, "FIXTURE_DIR="+dir, "MODE="+mode, "TARGET="+target, "SERVICE="+service, "CUSTOM="+boolString(custom))
	out, runErr := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("fixture deadline: %v\n%s", ctx.Err(), out)
	}
	calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
	config, configErr := os.ReadFile(filepath.Join(dir, "config", "preserve"))
	r := serverResetResult{output: string(out), calls: string(calls), err: runErr, configPresent: configErr == nil && string(config) == "original", states: map[string]string{}}
	for _, d := range []string{"install", "other-config"} {
		b, e := os.ReadFile(filepath.Join(dir, d, "preserve"))
		if e != nil || string(b) != "original" {
			t.Fatalf("reset changed %s: %v\n%s", d, e, out)
		}
	}
	for _, unit := range units {
		for _, suffix := range []string{"active", "enabled"} {
			b, e := os.ReadFile(filepath.Join(dir, unit+"."+suffix))
			if e != nil {
				t.Fatal(e)
			}
			r.states[unit+"."+suffix] = strings.TrimSpace(string(b))
		}
	}
	return r
}
