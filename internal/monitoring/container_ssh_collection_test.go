package monitoring

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type containerSSHFixtureRunner func(context.Context, string, ...string) ([]byte, error)

func (f containerSSHFixtureRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return f(ctx, name, args...)
}

const (
	containerSSHNoticePhrase   = "using direct SSH"
	containerSSHInsecureNotice = "This is insecure for production deployments."
	containerSSHDevNotice      = "dev mode override active"
)

// The runner and keyscan are both synthetic: never contact a real SSH host.
// Default container mode warns when ssh really runs, and an existing key still
// reaches the normal host-key and strict SSH path. Missing keys and host-key
// failures still refuse, and refuse quietly because no ssh ran.
func TestContainerSSHCollectionWarnsWithoutBypassingKeyOrHostTrust(t *testing.T) {
	for _, state := range []string{"missing_key", "existing_key", "host_key_refused"} {
		t.Run(state, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("PULSE_DOCKER", "true")
			t.Setenv("PULSE_DEV_ALLOW_CONTAINER_SSH", "")
			key := filepath.Join(home, "synthetic-key")
			const contents = "synthetic private-key preservation sentinel"
			if state != "missing_key" {
				require.NoError(t, os.WriteFile(key, []byte(contents), 0600))
			}
			keyscans, commands := 0, 0
			trust, err := NewKnownHostsManager(filepath.Join(home, "known_hosts"), WithKeyscanFunc(func(_ context.Context, host string, port int, _ time.Duration) ([]byte, error) {
				keyscans++
				if state == "host_key_refused" {
					return nil, errors.New("synthetic host-key refusal")
				}
				return []byte(fmt.Sprintf("[%s]:%d ssh-ed25519 AAAA\n", host, port)), nil
			}))
			require.NoError(t, err)
			collector := &TemperatureCollector{
				sshUser: "fixture", sshKeyPath: key, sshPort: 2222, hostKeys: trust,
				runner: containerSSHFixtureRunner(func(_ context.Context, name string, args ...string) ([]byte, error) {
					commands++
					require.Equal(t, "ssh", name)
					command := strings.Join(args, " ")
					for _, want := range []string{"StrictHostKeyChecking=yes", "BatchMode=yes", "UserKnownHostsFile=" + trust.Path(), "-i " + key, "fixture@container-ssh.invalid"} {
						require.Contains(t, command, want)
					}
					require.Equal(t, pulseSensorsSSHCommand, args[len(args)-1])
					return []byte(`{"coretemp-isa-0000":{"Package id 0":{"temp1_input":44}}}`), nil
				}),
			}
			logs := newMonitoringLogCapture(t)
			temperature, err := collector.CollectTemperature(context.Background(), "container-ssh.invalid", "synthetic-node")
			require.NoError(t, err)
			require.NotNil(t, temperature)
			if state == "existing_key" {
				require.Contains(t, logs.String(), containerSSHInsecureNotice)
				require.True(t, temperature.Available)
				require.Equal(t, 44.0, temperature.CPUPackage)
				require.Equal(t, 1, keyscans)
				require.Equal(t, 1, commands)
			} else {
				require.NotContains(t, logs.String(), containerSSHNoticePhrase)
				require.False(t, temperature.Available)
				require.Zero(t, commands, "missing key or failed host-key trust must not reach SSH execution")
				if state == "missing_key" {
					require.Zero(t, keyscans)
				}
			}
			if state != "missing_key" {
				got, err := os.ReadFile(key)
				require.NoError(t, err)
				require.Equal(t, contents, string(got))
				info, err := os.Stat(key)
				require.NoError(t, err)
				require.Equal(t, os.FileMode(0600), info.Mode().Perm())
			}
		})
	}
}

// A Docker install polls every node on every cycle, so the notice has to follow
// real ssh runs and not the poll. Both nodes share a name because the SSH target
// is the host: same-named machines are still separate targets.
func TestContainerSSHNoticeFollowsRealSSHOncePerNode(t *testing.T) {
	nodes := []string{"https://alpha.invalid:8006", "https://beta.invalid:8006"}
	const polls = 3

	cases := []struct {
		name       string
		keyPath    func(home string) string
		devMode    string
		wantNotice string // logged once per node; empty means no notice at all
		wantRuns   int
	}{
		{
			name:    "no_key_configured",
			keyPath: func(string) string { return "" },
		},
		{
			name:    "key_file_missing",
			keyPath: func(home string) string { return filepath.Join(home, "absent-key") },
		},
		{
			name:       "key_used",
			keyPath:    func(home string) string { return filepath.Join(home, "synthetic-key") },
			wantNotice: containerSSHInsecureNotice,
			wantRuns:   polls * len(nodes),
		},
		{
			name:       "key_used_with_dev_override",
			keyPath:    func(home string) string { return filepath.Join(home, "synthetic-key") },
			devMode:    "true",
			wantNotice: containerSSHDevNotice,
			wantRuns:   polls * len(nodes),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("PULSE_DOCKER", "true")
			t.Setenv("PULSE_DEV_ALLOW_CONTAINER_SSH", tc.devMode)
			if tc.wantRuns > 0 {
				require.NoError(t, os.WriteFile(tc.keyPath(home), []byte("synthetic key"), 0600))
			}
			runs := 0
			collector := &TemperatureCollector{
				sshUser: "fixture", sshKeyPath: tc.keyPath(home), sshPort: 2222,
				runner: containerSSHFixtureRunner(func(context.Context, string, ...string) ([]byte, error) {
					runs++
					return []byte(`{"coretemp-isa-0000":{"Package id 0":{"temp1_input":44}}}`), nil
				}),
			}

			logs := newMonitoringLogCapture(t)
			for poll := 0; poll < polls; poll++ {
				for _, node := range nodes {
					_, err := collector.CollectTemperature(context.Background(), node, "pve")
					require.NoError(t, err)
				}
			}

			require.Equal(t, tc.wantRuns, runs)
			var notices []string
			for _, line := range strings.Split(logs.String(), "\n") {
				if strings.Contains(line, containerSSHNoticePhrase) {
					notices = append(notices, line)
				}
			}
			if tc.wantNotice == "" {
				require.Empty(t, notices, "no ssh ran, so the log must not claim it did")
				return
			}
			require.Len(t, notices, len(nodes), "one notice per node across every poll")
			require.Contains(t, notices[0], "alpha.invalid")
			require.Contains(t, notices[1], "beta.invalid")
			for _, notice := range notices {
				require.Contains(t, notice, tc.wantNotice)
			}
		})
	}
}
