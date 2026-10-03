package installtests

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDemoReachabilityStopsBeforeUnreadyOrFailedTailnetProbes(t *testing.T) {
	for _, tc := range []struct {
		name, mode, state, rawStatus  string
		statusExit, pingExit, tcpExit int
		wantExit, pings, tcpProbes    int
		want                          string
	}{
		{name: "needs-login", state: "NeedsLogin", wantExit: 1, want: "no demo reachability probes attempted"},
		{name: "machine-auth", state: "NeedsMachineAuth", wantExit: 1, want: "no demo reachability probes attempted"},
		{name: "stopped", state: "Stopped", wantExit: 1},
		{name: "starting", state: "Starting", wantExit: 1},
		{name: "unknown-state", state: "private-data-never-log", wantExit: 1, want: "Tailscale backend: unknown"},
		{name: "unavailable-status", statusExit: 1, wantExit: 1, want: "status JSON is unavailable"},
		{name: "invalid-json", rawStatus: "private-data-never-log", wantExit: 1},
		{name: "array-status", rawStatus: "[]", wantExit: 1},
		{name: "null-status", rawStatus: "null", wantExit: 1},
		{name: "missing-state", rawStatus: "{}", wantExit: 1},
		{name: "invalid-state-type", rawStatus: `{"BackendState":[]}`, wantExit: 1},
		{name: "diagnose-after-login-failure", mode: "diagnose", state: "NeedsLogin", want: "Diagnostic mode is local-only"},
		{name: "diagnose-after-running-setup-failure", mode: "diagnose", state: "Running", want: "installed state remain unverified"},
		{name: "diagnose-unavailable-status", mode: "diagnose", statusExit: 1, want: "Diagnostic mode is local-only"},
		{name: "failed-tailnet-ping", state: "Running", pingExit: 1, wantExit: 1, pings: 1, want: "Tailscale cannot reach the demo peer"},
		{name: "closed-ssh", state: "Running", tcpExit: 1, wantExit: 1, pings: 1, tcpProbes: 2, want: "TCP/22 remained closed"},
		{name: "ready", state: "Running", pings: 1, tcpProbes: 1, want: "Demo SSH transport is reachable over Tailscale"},
		{name: "malformed-peer-map", rawStatus: `{"BackendState":"Running","Peer":[]}`, pings: 1, tcpProbes: 1},
		{name: "malformed-peer", rawStatus: `{"BackendState":"Running","Peer":{"a":null,"b":{"DNSName":[],"TailscaleIPs":3}}}`, pings: 1, tcpProbes: 1},
		{name: "invalid-mode", mode: "unsupported", state: "Running", wantExit: 2, want: "Usage:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			bin := filepath.Join(tmp, "bin")
			if err := os.Mkdir(bin, 0o755); err != nil {
				t.Fatal(err)
			}
			status := tc.rawStatus
			if status == "" {
				data, err := json.Marshal(map[string]any{
					"BackendState": tc.state,
					"Self":         map[string]any{"TailscaleIPs": []string{"100.100.100.1"}, "DNSName": "private-data-never-log.test.", "Tags": []string{"tag:private-data-never-log"}},
					"Peer":         map[string]any{"demo": map[string]any{"DNSName": "demo.synthetic.test.", "Online": true, "Active": true, "Relay": "private-data-never-log"}},
				})
				if err != nil {
					t.Fatal(err)
				}
				status = string(data)
			}
			statusFile := filepath.Join(tmp, "status.json")
			if err := os.WriteFile(statusFile, []byte(status), 0o600); err != nil {
				t.Fatal(err)
			}
			callsFile := filepath.Join(tmp, "calls")
			for name, script := range map[string]string{
				"tailscale": `#!/bin/sh
printf 'tailscale %s\n' "$*" >> "$CALLS_FILE"
case "$1" in
  status) cat "$STATUS_FILE"; echo 'private-data-never-log' >&2; exit "$STATUS_EXIT" ;;
  ping) echo 'private-data-never-log'; echo 'private-data-never-log' >&2; exit "$PING_EXIT" ;;
  *) exit 97 ;;
esac
`,
				"nc": `#!/bin/sh
printf 'nc %s\n' "$*" >> "$CALLS_FILE"
echo 'private-data-never-log'
echo 'private-data-never-log' >&2
exit "$TCP_EXIT"
`,
			} {
				if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			mode := tc.mode
			if mode == "" {
				mode = "check"
			}
			cmd := exec.Command("bash", repoFile(".github", "scripts", "check-demo-reachability.sh"), mode)
			cmd.Env = append(os.Environ(),
				"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"DEMO_SERVER_HOST=demo.synthetic.test", "DEMO_SERVER_PORT=22", "DEMO_TCP_ATTEMPTS=2", "DEMO_TCP_RETRY_SECONDS=0",
				"CALLS_FILE="+callsFile, "STATUS_FILE="+statusFile,
				"STATUS_EXIT="+strconv.Itoa(tc.statusExit), "PING_EXIT="+strconv.Itoa(tc.pingExit), "TCP_EXIT="+strconv.Itoa(tc.tcpExit),
			)
			output, err := cmd.CombinedOutput()
			if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.wantExit {
				t.Fatalf("helper exit: %v, want %d; output=%s", err, tc.wantExit, output)
			}
			calls, err := os.ReadFile(callsFile)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if got := strings.Count(string(calls), "tailscale ping "); got != tc.pings {
				t.Fatalf("tailnet probes=%d, want %d; calls=%s", got, tc.pings, calls)
			}
			if got := strings.Count(string(calls), "nc "); got != tc.tcpProbes {
				t.Fatalf("TCP probes=%d, want %d; calls=%s", got, tc.tcpProbes, calls)
			}
			if tc.mode == "unsupported" && len(calls) != 0 {
				t.Fatalf("invalid mode attempted a diagnostic: %s", calls)
			}
			if !strings.Contains(string(output), tc.want) {
				t.Fatalf("output missing %q: %s", tc.want, output)
			}
			for _, private := range []string{"private-data-never-log", "100.100.100.1", "demo.synthetic.test", "Traceback"} {
				if strings.Contains(string(output), private) {
					t.Fatalf("helper exposed private or raw diagnostic content %q: %s", private, output)
				}
			}
			if tc.wantExit != 0 || tc.mode == "diagnose" {
				if strings.Contains(string(output), "Demo SSH transport is reachable") {
					t.Fatalf("helper claimed connectivity without successful check: %s", output)
				}
			}
		})
	}
}
