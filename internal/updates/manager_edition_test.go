package updates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/edition"
)

// TestApplyUpdateProEditionGate verifies Guard 2 of the Pro download/update
// spec: the separately compiled Pro binary must never install the public
// community build. Without an activation the Pro path refuses with the manual
// portal fallback; with an activation it only accepts the broker-intent URL
// for its own license server; the community edition is untouched by the gate.
func TestApplyUpdateProEditionGate(t *testing.T) {
	// Allow an arbitrary download host (validateApplyDownloadURL) and force
	// Docker detection off, so the flow deterministically reaches the edition
	// gate whether the test runs on a dev host or inside a CI container.
	t.Setenv("PULSE_UPDATE_SERVER", "http://example.invalid")
	t.Setenv("PULSE_ALLOW_DOCKER_UPDATES", "true")
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	// A local server that always 404s keeps the community path hermetic: it
	// gets past the edition gate and then fails fast on download without
	// touching the real network.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	defer server.Close()

	cfg := &config.Config{DataPath: t.TempDir()}
	manager := NewManager(cfg)

	downloadURL := server.URL + "/pulse-v6.0.0-linux-amd64.tar.gz"

	t.Run("pro edition without activation refuses with portal fallback", func(t *testing.T) {
		edition.SetEdition(edition.Pro)
		t.Cleanup(func() { edition.SetEdition(edition.Community) })

		err := manager.ApplyUpdate(context.Background(), ApplyUpdateRequest{DownloadURL: downloadURL})
		if err == nil {
			t.Fatal("expected ApplyUpdate to refuse the unactivated Pro edition, got nil")
		}
		msg := err.Error()
		if !strings.Contains(msg, "activated license") {
			t.Fatalf("Pro edition error must explain the activation requirement, got: %v", err)
		}
		if !strings.Contains(msg, "pulserelay.pro/download.html") {
			t.Fatalf("Pro edition error must point at the download portal fallback, got: %v", err)
		}
		if !strings.Contains(msg, "install.sh --archive") {
			t.Fatalf("Pro edition error must mention the archive install path, got: %v", err)
		}
	})

	t.Run("pro edition rejects community download URLs", func(t *testing.T) {
		edition.SetEdition(edition.Pro)
		t.Cleanup(func() { edition.SetEdition(edition.Community) })

		manager.SetProUpdateCredentialSource(func() (ProUpdateCredentials, bool) {
			return ProUpdateCredentials{
				LicenseServerURL:    "https://license.example.invalid",
				InstallationToken:   "pit_live_test",
				InstanceFingerprint: "fp-test",
			}, true
		})
		t.Cleanup(func() { manager.SetProUpdateCredentialSource(nil) })

		err := manager.ApplyUpdate(context.Background(), ApplyUpdateRequest{DownloadURL: downloadURL})
		if err == nil {
			t.Fatal("expected ApplyUpdate to reject a community download URL on the Pro edition, got nil")
		}
		if !strings.Contains(err.Error(), "invalid download URL") {
			t.Fatalf("Pro edition must reject non-broker download URLs, got: %v", err)
		}
	})

	t.Run("community edition proceeds past the edition gate", func(t *testing.T) {
		edition.SetEdition(edition.Community)

		err := manager.ApplyUpdate(context.Background(), ApplyUpdateRequest{DownloadURL: downloadURL})
		// The community path still fails (the local server 404s), but it must
		// NOT be blocked by the Pro edition gate.
		if err != nil && strings.Contains(err.Error(), "activated license") {
			t.Fatalf("community edition must not hit the Pro edition gate, got: %v", err)
		}
	})
}
