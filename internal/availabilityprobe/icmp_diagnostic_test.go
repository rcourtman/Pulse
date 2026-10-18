package availabilityprobe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestICMPProbeDiagnostics(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("synthetic ping fixture requires a POSIX shell; no real ping is used")
	}

	runners := []struct {
		name string
		run  func(context.Context, config.AvailabilityTarget) (ProbeResult, error)
	}{
		{"Run", func(ctx context.Context, target config.AvailabilityTarget) (ProbeResult, error) {
			return ProbeResult{}, Run(ctx, target)
		}},
		{"Result", func(ctx context.Context, target config.AvailabilityTarget) (ProbeResult, error) {
			outcome, err := Result(ctx, target)
			return ProbeResult{Outcome: outcome}, err
		}},
		{"DetailedResult", DetailedResult},
	}
	const privateMarker = "synthetic-private-ping-details"
	cases := []struct {
		name       string
		output     string
		exit       int
		blocked    bool
		wantError  string
		contextErr error
		missing    bool
	}{
		{name: "operation-not-permitted", output: "ping: socket: Operation not permitted\n" + privateMarker, exit: 1, blocked: true},
		{name: "cap-net-raw", output: "ping: missing cap_net_raw capability\n" + privateMarker, exit: 2, blocked: true},
		{name: "ordinary-error", output: "  ping: unknown host\n", exit: 1, wantError: "icmp probe failed: ping: unknown host"},
		{name: "bounded-output", output: strings.Repeat("x", 300), exit: 1, wantError: "icmp probe failed: " + strings.Repeat("x", 240)},
		{name: "empty-output", exit: 17, wantError: "icmp probe failed: exit status 17"},
		{name: "success-with-permission-like-output", output: "Operation not permitted cap_net_raw", exit: 0},
		{name: "cancelled", contextErr: context.Canceled},
		{name: "expired", contextErr: context.DeadlineExceeded},
		{name: "missing-ping", missing: true},
	}
	for _, runner := range runners {
		t.Run(runner.name, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					dir := t.TempDir()
					argsFile := filepath.Join(dir, "args")
					if !tc.missing {
						// Shell builtins only: this fixture cannot contact a target or
						// accidentally delegate to the host's actual ping binary.
						script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$PULSE_TEST_PING_ARGS\"\nprintf '%s' \"$PULSE_TEST_PING_OUTPUT\" >&2\nexit \"$PULSE_TEST_PING_EXIT\"\n"
						if err := os.WriteFile(filepath.Join(dir, "ping"), []byte(script), 0700); err != nil {
							t.Fatal(err)
						}
					}
					t.Setenv("PATH", dir)
					t.Setenv("PULSE_TEST_PING_ARGS", argsFile)
					t.Setenv("PULSE_TEST_PING_OUTPUT", tc.output)
					t.Setenv("PULSE_TEST_PING_EXIT", fmt.Sprint(tc.exit))

					ctx := context.Background()
					if tc.contextErr == context.Canceled {
						cancelled, cancel := context.WithCancel(ctx)
						cancel()
						ctx = cancelled
					} else if tc.contextErr == context.DeadlineExceeded {
						expired, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
						defer cancel()
						ctx = expired
					}
					target := config.AvailabilityTarget{
						Address: "192.0.2.10", Protocol: config.AvailabilityProbeICMP, Enabled: true, TimeoutMillis: 1375,
					}
					result, err := runner.run(ctx, target)
					if runner.name == "DetailedResult" && (result.TransportOutcome != result.Outcome || result.Application != nil || result.Certificate != nil) {
						t.Errorf("ICMP result changed non-diagnostic fields: %+v", result)
					}
					switch {
					case tc.blocked:
						if err == nil {
							t.Fatal("permission failure was reported as success")
						}
						message := err.Error()
						for _, required := range []string{"icmp probe blocked", "local permissions", "selected observation host", "server or agent", "docs/CONFIGURATION.md#icmp-probe-privileges", "NoNewPrivileges", "intentional", "restrictions"} {
							if !strings.Contains(message, required) {
								t.Errorf("permission diagnostic %q omits %q", message, required)
							}
						}
						for _, unsafe := range []string{privateMarker, "Pulse service unit", "installer", "AmbientCapabilities=", "CapabilityBoundingSet=", "restart", "setcap", "--privileged", "--cap-add", "as root"} {
							if strings.Contains(message, unsafe) {
								t.Errorf("permission diagnostic %q contains unsupported advice or raw details %q", message, unsafe)
							}
						}
						if len(message) > 240 {
							t.Errorf("diagnostic has %d bytes; agent reports truncate after 240", len(message))
						}
					case tc.contextErr != nil:
						if !errors.Is(err, tc.contextErr) {
							t.Fatalf("error = %v, want %v", err, tc.contextErr)
						}
					case tc.missing:
						if !errors.Is(err, exec.ErrNotFound) || !strings.HasPrefix(err.Error(), "icmp probe failed: ") {
							t.Fatalf("error = %v, want wrapped missing executable error", err)
						}
					case tc.wantError != "":
						if err == nil || err.Error() != tc.wantError {
							t.Fatalf("error = %v, want %q", err, tc.wantError)
						}
						if tc.output == "" {
							var exitError *exec.ExitError
							if !errors.As(err, &exitError) || exitError.ExitCode() != tc.exit {
								t.Fatalf("error = %v, want wrapped exit %d", err, tc.exit)
							}
						}
					default:
						if err != nil {
							t.Fatalf("successful ping returned error: %v", err)
						}
					}
					if runner.name != "Run" {
						want := OutcomeReachable
						if err != nil {
							want = OutcomeUnreachable // Existing classification is unchanged.
						}
						if result.Outcome != want {
							t.Fatalf("outcome = %q, want %q", result.Outcome, want)
						}
					}
					args, argsErr := os.ReadFile(argsFile)
					if tc.contextErr != nil || tc.missing {
						if !errors.Is(argsErr, os.ErrNotExist) {
							t.Fatalf("unstarted ping left arguments: %q, error %v", args, argsErr)
						}
					} else if want := strings.Join(pingArgs(target.Address, target.TimeoutMillis), "\n") + "\n"; argsErr != nil || string(args) != want {
						t.Fatalf("ping arguments = %q, error %v; want %q", args, argsErr, want)
					}
				})
			}
		})
	}
}
