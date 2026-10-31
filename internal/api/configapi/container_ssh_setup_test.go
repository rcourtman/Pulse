package configapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
)

func TestContainerSSHSetupRefusalDoesNotPromiseCollectionBlock(t *testing.T) {
	for _, state := range []string{"missing_keys", "existing_keys"} {
		t.Run(state, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("PULSE_DOCKER", "true")
			t.Setenv("PULSE_DEV_ALLOW_CONTAINER_SSH", "")
			sshDir := filepath.Join(home, ".ssh")
			fixtures := map[string]string{
				"id_ed25519_sensors":     "synthetic private-key preservation sentinel",
				"id_ed25519_sensors.pub": "synthetic public-key preservation sentinel",
			}
			if state == "existing_keys" {
				if err := os.Mkdir(sshDir, 0700); err != nil {
					t.Fatal(err)
				}
				for name, data := range fixtures {
					if err := os.WriteFile(filepath.Join(sshDir, name), []byte(data), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			id, lines, _ := logging.GetBroadcaster().Subscribe()
			defer logging.GetBroadcaster().Unsubscribe(id)
			if got := (&ConfigHandlers{}).getOrGenerateSSHKeys(); got != (SSHKeyPair{}) {
				t.Fatal("default container setup returned an SSH key")
			}
			var messages strings.Builder
		capture:
			for {
				select {
				case line := <-lines:
					messages.WriteString(line)
				default:
					break capture
				}
			}
			for _, want := range []string{
				"SSH key generation disabled in containerized deployments",
				"This setup refusal does not disable legacy SSH collection with an existing key",
				"/docs/SECURITY.md#critical-security-notice-for-container-deployments",
				"NEVER in production",
			} {
				if !strings.Contains(messages.String(), want) {
					t.Fatalf("setup diagnostic omitted %q: %s", want, messages.String())
				}
			}
			if strings.Contains(messages.String(), "Temperature monitoring via SSH is disabled") {
				t.Fatal("setup diagnostic falsely promised a collection-wide block")
			}
			if state == "missing_keys" {
				if _, err := os.Stat(sshDir); !os.IsNotExist(err) {
					t.Fatalf("refused setup created or accessed SSH directory: %v", err)
				}
			} else {
				entries, err := os.ReadDir(sshDir)
				if err != nil || len(entries) != len(fixtures) {
					t.Fatalf("refused setup changed SSH inventory: entries=%d err=%v", len(entries), err)
				}
				for name, want := range fixtures {
					path := filepath.Join(sshDir, name)
					got, err := os.ReadFile(path)
					if err != nil || string(got) != want {
						t.Fatalf("refused setup changed existing %s: %v", name, err)
					}
					info, err := os.Stat(path)
					if err != nil || info.Mode().Perm() != 0600 {
						t.Fatalf("refused setup changed existing %s permissions: %v", name, err)
					}
				}
			}
		})
	}
}
