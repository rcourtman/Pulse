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

// Substitute only the fixed config directory in an exact script copy. Every
// external lifecycle action is a fixture function, never a native pct/systemctl.
func sensorProxySafetyScript(t *testing.T, dir string) string {
	t.Helper()
	source, err := os.ReadFile(repoFile("scripts", "uninstall-sensor-proxy.sh"))
	if err != nil {
		t.Fatal(err)
	}
	const fixed = `conf="/etc/pve/lxc/${ctid}.conf"`
	if strings.Count(string(source), fixed) != 1 {
		t.Fatal("expected one fixed LXC config path")
	}
	script := strings.Replace(string(source), fixed, `conf="${FIXTURE_DIR}/${ctid}.conf"`, 1)
	path := filepath.Join(dir, "uninstall-fixture.sh")
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runSensorProxySafetyShell(t *testing.T, script, dir, scenario, body string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", body, "bash", script)
	cmd.Env = append(os.Environ(), "FIXTURE_DIR="+dir, "SCENARIO="+scenario)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("fixture deadline: %v\n%s", ctx.Err(), out)
	}
	if err == nil {
		return string(out), 0
	}
	if e, ok := err.(*exec.ExitError); ok {
		return string(out), e.ExitCode()
	}
	t.Fatalf("fixture execution: %v\n%s", err, out)
	return "", -1
}

const sensorProxyMountFixture = `source "$1"
timeout() { [[ "$1" != --kill-after=* ]] || shift; shift; "$@"; }
sleep() { :; }
pct() {
    printf '%s\n' "$*" >>"$FIXTURE_DIR/actions"
    local count=0 state=""
    case "$1" in
        list)
            printf 'VMID Status Lock Name\n101 running test\n'
            [[ "$SCENARIO" != list_failure ]] || return 9
            ;;
        status)
            [[ "$2" == 101 ]] || return 90
            [[ ! -f "$FIXTURE_DIR/count" ]] || read -r count <"$FIXTURE_DIR/count"
            count=$((count + 1)); printf '%s\n' "$count" >"$FIXTURE_DIR/count"
            case "$SCENARIO:$count" in
                status_failure:1) printf 'status: stopped\n'; return 9 ;;
                status_unknown:1) printf 'status: paused\n'; return 0 ;;
                status_malformed:1) printf 'status: stopped\nstatus: running\n'; return 0 ;;
                stopped_read_failure:2) return 9 ;;
                lost_before_set:3|lost_before_raw:4) printf 'status: running\n'; return 0 ;;
                lost_before_restore:5) return 9 ;;
            esac
            read -r state <"$FIXTURE_DIR/state"
            printf 'status: %s\n' "$state"
            ;;
        stop)
            [[ "$2" == 101 ]] || return 90
            [[ "$SCENARIO" != stop_failure ]] || return 9
            [[ "$SCENARIO" == stop_no_effect ]] || printf 'stopped\n' >"$FIXTURE_DIR/state"
            ;;
        set)
            [[ "$*" == 'set 101 -delete mp0' ]] || return 90
            [[ "$SCENARIO" != set_failure ]] || return 9
            ;;
        start)
            [[ "$2" == 101 ]] || return 90
            [[ "$SCENARIO" != start_failure ]] || return 9
            [[ "$SCENARIO" == start_no_effect ]] || printf 'running\n' >"$FIXTURE_DIR/state"
            ;;
        *) return 90 ;;
    esac
}
mv() {
    [[ "$SCENARIO" != write_failure ]] || return 9
    command mv "$@"
}
cleanup_stale_sensor_proxy_mounts
`

func TestUninstallSensorProxyMountSafety(t *testing.T) {
	main := "hostname: workload\nmp0: /run/pulse-sensor-proxy,mp=/sensor\nmp1: /data,mp=/data\nlxc.mount.entry: /run/pulse-sensor-proxy sensor none bind 0 0\n"
	snapshot := "[saved]\nmp0: /run/pulse-sensor-proxy,mp=/sensor\nlxc.mount.entry: /run/pulse-sensor-proxy sensor none bind 0 0\n"
	cases := []struct {
		name, initial, conf, want, actions string
		success                            bool
	}{
		{"stopped", "stopped", main + snapshot, "hostname: workload\nmp1: /data,mp=/data\n" + snapshot, "list\nstatus 101\nstatus 101\nstatus 101\nset 101 -delete mp0\nstatus 101\n", true},
		{"running", "running", main + snapshot, "hostname: workload\nmp1: /data,mp=/data\n" + snapshot, "list\nstatus 101\nstop 101\nstatus 101\nstatus 101\nset 101 -delete mp0\nstatus 101\nstatus 101\nstart 101\nstatus 101\n", true},
		{"no_snapshot", "stopped", main, "hostname: workload\nmp1: /data,mp=/data\n", "list\nstatus 101\nstatus 101\nstatus 101\nset 101 -delete mp0\nstatus 101\n", true},
		{"snapshot_only", "running", "hostname: workload\n" + snapshot, "hostname: workload\n" + snapshot, "list\n", true},
		{"snapshot_first", "running", snapshot, snapshot, "list\n", true},
		{"list_failure", "running", main, main, "list\n", false},
		{"status_failure", "running", main, main, "list\nstatus 101\n", false},
		{"status_unknown", "running", main, main, "list\nstatus 101\n", false},
		{"status_malformed", "running", main, main, "list\nstatus 101\n", false},
		{"stop_failure", "running", main, main, "list\nstatus 101\nstop 101\n", false},
		{"stop_no_effect", "running", main, main, "list\nstatus 101\nstop 101\nstatus 101\n", false},
		{"stopped_read_failure", "running", main, main, "list\nstatus 101\nstop 101\nstatus 101\n", false},
		{"lost_before_set", "running", main, main, "list\nstatus 101\nstop 101\nstatus 101\nstatus 101\n", false},
		{"lost_before_raw", "running", main, main, "list\nstatus 101\nstop 101\nstatus 101\nstatus 101\nset 101 -delete mp0\nstatus 101\n", false},
		{"set_failure", "running", main, main, "list\nstatus 101\nstop 101\nstatus 101\nstatus 101\nset 101 -delete mp0\nstatus 101\nstart 101\nstatus 101\n", false},
		{"write_failure", "running", main, main, "list\nstatus 101\nstop 101\nstatus 101\nstatus 101\nset 101 -delete mp0\nstatus 101\nstatus 101\nstart 101\nstatus 101\n", false},
		{"lost_before_restore", "running", main, "hostname: workload\nmp1: /data,mp=/data\n", "list\nstatus 101\nstop 101\nstatus 101\nstatus 101\nset 101 -delete mp0\nstatus 101\nstatus 101\n", false},
		{"start_failure", "running", main, "hostname: workload\nmp1: /data,mp=/data\n", "list\nstatus 101\nstop 101\nstatus 101\nstatus 101\nset 101 -delete mp0\nstatus 101\nstatus 101\nstart 101\n", false},
		{"start_no_effect", "running", main, "hostname: workload\nmp1: /data,mp=/data\n", "list\nstatus 101\nstop 101\nstatus 101\nstatus 101\nset 101 -delete mp0\nstatus 101\nstatus 101\nstart 101\nstatus 101\n", false},
	}
	for _, tc := range cases {
		// Adverse controls include a real snapshot section so the original
		// script reaches its ignored-stop/mutation path (not its unrelated
		// no-snapshot pipefail bug).
		if !tc.success {
			tc.conf += snapshot
			tc.want += snapshot
		}
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			script := sensorProxySafetyScript(t, dir)
			conf := filepath.Join(dir, "101.conf")
			if err := os.WriteFile(conf, []byte(tc.conf), 0640); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "state"), []byte(tc.initial+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			out, code := runSensorProxySafetyShell(t, script, dir, tc.name, sensorProxyMountFixture)
			if (code == 0) != tc.success {
				t.Fatalf("exit %d; success=%v\n%s", code, tc.success, out)
			}
			if !tc.success && !strings.Contains(out, "[WARN]") {
				t.Fatalf("silent incomplete cleanup: %s", out)
			}
			got, err := os.ReadFile(conf)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("configuration changed incorrectly:\n%s\nwant:\n%s", got, tc.want)
			}
			info, err := os.Stat(conf)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0640 {
				t.Fatalf("configuration permissions changed: %v", info.Mode())
			}
			actions, err := os.ReadFile(filepath.Join(dir, "actions"))
			if err != nil {
				t.Fatal(err)
			}
			if string(actions) != tc.actions {
				t.Fatalf("lifecycle/actions:\n%s\nwant:\n%s", actions, tc.actions)
			}
		})
	}
}

func TestUninstallSensorProxyMainReportsIncompleteLocalCleanup(t *testing.T) {
	for _, phase := range []string{"services", "containers", "reload", "success"} {
		t.Run(phase, func(t *testing.T) {
			dir := t.TempDir()
			body := `source "$1"
mark() { printf '%s\n' "$1" >>"$FIXTURE_DIR/actions"; }
disable_legacy_units() { mark services; [[ "$SCENARIO" != services ]]; }
cleanup_cluster_authorized_keys() { mark keys; }
cleanup_stale_sensor_proxy_mounts() { mark containers; [[ "$SCENARIO" != containers ]]; }
remove_legacy_files() { mark files; }
remove_proxmox_access() { mark access; }
systemctl_if_available() { mark reload; [[ "$SCENARIO" != reload ]]; }
main --local-only
`
			out, code := runSensorProxySafetyShell(t, repoFile("scripts", "uninstall-sensor-proxy.sh"), dir, phase, body)
			if (code == 0) != (phase == "success") {
				t.Fatalf("exit %d:\n%s", code, out)
			}
			if strings.Contains(out, "Legacy pulse-sensor-proxy cleanup complete") != (phase == "success") {
				t.Fatalf("incorrect completion message: %s", out)
			}
			want := "services\n"
			if phase != "services" {
				want += "keys\ncontainers\n"
			}
			if phase == "reload" || phase == "success" {
				want += "files\naccess\nreload\n"
			}
			got, err := os.ReadFile(filepath.Join(dir, "actions"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != want {
				t.Fatalf("actions: %s want: %s", got, want)
			}
		})
	}
}

func TestUninstallSensorProxyServiceSafety(t *testing.T) {
	for _, scenario := range []string{"absent", "inactive", "read_failure", "empty", "stop_failure", "still_active", "state_failure", "disable_failure", "reload_failure"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			body := `source "$1"
systemctl() {
    printf '%s\n' "$*" >>"$FIXTURE_DIR/actions"
    case "$1" in
        show)
            if [[ "$3" == --property=LoadState ]]; then
                case "$SCENARIO" in absent) echo not-found;; read_failure) return 9;; empty) :;; *) echo loaded;; esac
            else
                case "$SCENARIO" in still_active) echo active;; state_failure) return 9;; *) echo inactive;; esac
            fi ;;
        stop) [[ "$SCENARIO" != stop_failure ]] ;;
        disable) [[ "$SCENARIO" != disable_failure ]] ;;
        daemon-reload) [[ "$SCENARIO" != reload_failure ]] ;;
        *) return 90 ;;
    esac
}
disable_legacy_units
`
			out, code := runSensorProxySafetyShell(t, repoFile("scripts", "uninstall-sensor-proxy.sh"), dir, scenario, body)
			if (code == 0) != (scenario == "absent" || scenario == "inactive") {
				t.Fatalf("exit %d:\n%s", code, out)
			}
			got, err := os.ReadFile(filepath.Join(dir, "actions"))
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "absent" && (strings.Contains(string(got), "stop ") || strings.Contains(string(got), "disable ")) {
				t.Fatalf("absent units mutated: %s", got)
			}
			if (scenario == "stop_failure" || scenario == "still_active" || scenario == "state_failure" || scenario == "empty" || scenario == "read_failure") && strings.Contains(string(got), "disable ") {
				t.Fatalf("unconfirmed unit disabled: %s", got)
			}
		})
	}
}
