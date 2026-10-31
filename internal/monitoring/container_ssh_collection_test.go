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

// The runner and keyscan are both synthetic: never contact a real SSH host.
// Default container mode warns, but an existing key still reaches the normal
// host-key and strict SSH path. Missing keys and host-key failures still refuse.
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
			require.Contains(t, logs.String(), "This is insecure for production deployments.")
			if state == "existing_key" {
				require.True(t, temperature.Available)
				require.Equal(t, 44.0, temperature.CPUPackage)
				require.Equal(t, 1, keyscans)
				require.Equal(t, 1, commands)
			} else {
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
