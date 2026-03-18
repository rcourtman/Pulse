package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestKeepRealPollingInMockMode(t *testing.T) {
	t.Run("default_true", func(t *testing.T) {
		t.Setenv(mockKeepRealPollingEnv, "")
		if !keepRealPollingInMockMode() {
			t.Fatal("expected default behavior to keep real polling enabled")
		}
	})

	t.Run("explicit_false", func(t *testing.T) {
		t.Setenv(mockKeepRealPollingEnv, "false")
		if keepRealPollingInMockMode() {
			t.Fatal("expected false override to disable real polling in mock mode")
		}
	})

	t.Run("explicit_true", func(t *testing.T) {
		t.Setenv(mockKeepRealPollingEnv, "true")
		if !keepRealPollingInMockMode() {
			t.Fatal("expected true override to keep real polling in mock mode")
		}
	})
}

func TestNew_MockModeClientInitializationRespectsKeepRealPollingSetting(t *testing.T) {
	makeConfig := func(dataPath string) *config.Config {
		return &config.Config{
			DataPath: dataPath,
			PVEInstances: []config.PVEInstance{
				{
					Name:     "pve-test",
					Host:     "https://127.0.0.1:8006",
					User:     "root@pam",
					Password: "secret",
				},
			},
		}
	}

	withClientStub := func(t *testing.T, fn func(called *bool)) {
		t.Helper()
		orig := newProxmoxClientFunc
		t.Cleanup(func() { newProxmoxClientFunc = orig })
		called := false
		newProxmoxClientFunc = func(cfg proxmox.ClientConfig) (PVEClientInterface, error) {
			called = true
			return &mockPVEClient{}, nil
		}
		fn(&called)
	}

	t.Run("enabled_by_default", func(t *testing.T) {
		t.Setenv(mockKeepRealPollingEnv, "")
		mock.SetEnabled(true)
		t.Cleanup(func() { mock.SetEnabled(false) })

		withClientStub(t, func(called *bool) {
			monitor, err := New(makeConfig(t.TempDir()))
			if err != nil {
				t.Fatalf("New() returned error: %v", err)
			}
			t.Cleanup(func() { monitor.Stop() })

			if !*called {
				t.Fatal("expected PVE client initialization in mock mode by default")
			}
			if len(monitor.pveClients) != 1 {
				t.Fatalf("expected 1 initialized PVE client, got %d", len(monitor.pveClients))
			}
		})
	})

	t.Run("disabled_via_env", func(t *testing.T) {
		t.Setenv(mockKeepRealPollingEnv, "false")
		mock.SetEnabled(true)
		t.Cleanup(func() { mock.SetEnabled(false) })

		withClientStub(t, func(called *bool) {
			monitor, err := New(makeConfig(t.TempDir()))
			if err != nil {
				t.Fatalf("New() returned error: %v", err)
			}
			t.Cleanup(func() { monitor.Stop() })

			if *called {
				t.Fatal("expected PVE client initialization to be skipped when override is false")
			}
			if len(monitor.pveClients) != 0 {
				t.Fatalf("expected 0 initialized PVE clients, got %d", len(monitor.pveClients))
			}
		})
	})
}
