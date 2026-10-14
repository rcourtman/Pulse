package updates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/edition"
)

// setupProRuntimeSwitchTest runs as the community binary: the switch exists to
// move a community install with a paid licence onto the Pro runtime.
func setupProRuntimeSwitchTest(t *testing.T, currentVersion string) {
	t.Helper()
	setupProUpdateTest(t, currentVersion)
	edition.SetEdition(edition.Community)
}

func refusingBroker(t *testing.T, status int, code string) func() (ProUpdateCredentials, bool) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": code, "message": "refused"}})
	}))
	t.Cleanup(server.Close)
	return func() (ProUpdateCredentials, bool) {
		return ProUpdateCredentials{LicenseServerURL: server.URL, InstallationToken: "pit_live_test", InstanceFingerprint: "fp-test"}, true
	}
}

func TestCheckProRuntimeSwitch(t *testing.T) {
	check := func(t *testing.T, source func() (ProUpdateCredentials, bool)) *ProRuntimeSwitch {
		t.Helper()
		manager := NewManager(&config.Config{DataPath: t.TempDir()})
		if source != nil {
			manager.SetProUpdateCredentialSource(source)
		}
		result, err := manager.CheckProRuntimeSwitch(context.Background(), "stable")
		if err != nil {
			t.Fatalf("CheckProRuntimeSwitch: %v", err)
		}
		return result
	}

	t.Run("same-version Pro build is offered in place", func(t *testing.T) {
		setupProRuntimeSwitchTest(t, "6.0.5")
		fixture := newProBrokerFixture(t, "6.0.5", false)
		result := check(t, fixture.credentialSource())
		if !result.Available || !result.InstallInPlace || result.Reason != "" {
			t.Fatalf("expected an in-place switch, got %+v", result)
		}
		if result.Version != "6.0.5" || result.CurrentVersion != "6.0.5" {
			t.Fatalf("unexpected versions %+v", result)
		}
		if result.Docker != nil {
			t.Fatalf("an in-place switch must not carry Docker commands, got %+v", result.Docker)
		}
		if fixture.brokerCalls != 1 {
			t.Fatalf("expected one broker call, got %d", fixture.brokerCalls)
		}
	})

	t.Run("older Pro pin is not offered", func(t *testing.T) {
		setupProRuntimeSwitchTest(t, "6.1.0")
		fixture := newProBrokerFixture(t, "6.0.5", false)
		result := check(t, fixture.credentialSource())
		if result.Available || result.Reason != ProRuntimeSwitchReasonOlderRelease || result.Version != "6.0.5" {
			t.Fatalf("expected older_release with the pinned version, got %+v", result)
		}
	})

	t.Run("Pro binary has nothing to switch", func(t *testing.T) {
		setupProUpdateTest(t, "6.0.5")
		fixture := newProBrokerFixture(t, "6.0.5", false)
		result := check(t, fixture.credentialSource())
		if result.Available || result.Reason != ProRuntimeSwitchReasonAlreadyPro {
			t.Fatalf("expected already_pro, got %+v", result)
		}
		if fixture.brokerCalls != 0 {
			t.Fatalf("a Pro binary must not query the broker for a switch, got %d calls", fixture.brokerCalls)
		}
	})

	t.Run("no activation is reported without a broker call", func(t *testing.T) {
		setupProRuntimeSwitchTest(t, "6.0.5")
		result := check(t, nil)
		if result.Available || result.Reason != ProRuntimeSwitchReasonNotActivated {
			t.Fatalf("expected not_activated, got %+v", result)
		}
	})

	t.Run("broker refusal means the licence is not eligible", func(t *testing.T) {
		setupProRuntimeSwitchTest(t, "6.0.5")
		result := check(t, refusingBroker(t, http.StatusForbidden, "PRO_DOWNLOAD_NOT_ALLOWED"))
		if result.Available || result.Reason != ProRuntimeSwitchReasonNotEligible {
			t.Fatalf("expected not_eligible, got %+v", result)
		}
	})

	t.Run("broker outage is unavailable, not ineligible", func(t *testing.T) {
		setupProRuntimeSwitchTest(t, "6.0.5")
		result := check(t, refusingBroker(t, http.StatusServiceUnavailable, "DOWNLOADS_UNAVAILABLE"))
		if result.Available || result.Reason != ProRuntimeSwitchReasonUnavailable {
			t.Fatalf("expected unavailable, got %+v", result)
		}
	})

	t.Run("wire keys", func(t *testing.T) {
		encoded, err := json.Marshal(ProRuntimeSwitch{Available: true, CurrentVersion: "6.0.5", Version: "6.0.5", InstallInPlace: true})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		for _, key := range []string{`"available":true`, `"currentVersion":"6.0.5"`, `"version":"6.0.5"`, `"installInPlace":true`} {
			if !strings.Contains(string(encoded), key) {
				t.Fatalf("ProRuntimeSwitch JSON missing %s: %s", key, encoded)
			}
		}
	})
}

func TestProRuntimeSwitchMode(t *testing.T) {
	t.Setenv("PULSE_ALLOW_DOCKER_UPDATES", "")
	cases := []struct {
		info *VersionInfo
		want string
	}{
		{&VersionInfo{IsDocker: true, DeploymentType: "docker"}, proRuntimeSwitchModeDocker},
		{&VersionInfo{DeploymentType: "docker"}, proRuntimeSwitchModeDocker},
		{&VersionInfo{DeploymentType: "systemd"}, proRuntimeSwitchModeInPlace},
		{&VersionInfo{DeploymentType: "proxmoxve"}, proRuntimeSwitchModeInPlace},
		{&VersionInfo{DeploymentType: "aur"}, ""},
		{&VersionInfo{DeploymentType: "manual"}, ""},
		{nil, ""},
	}
	for _, tc := range cases {
		if got := proRuntimeSwitchMode(tc.info); got != tc.want {
			t.Errorf("proRuntimeSwitchMode(%+v) = %q, want %q", tc.info, got, tc.want)
		}
	}
}

func TestApplyProRuntimeSwitch(t *testing.T) {
	origVerify := verifyReleaseSignatureFunc
	verifyReleaseSignatureFunc = func(ctx context.Context, targetPath, signaturePath string) error {
		return nil
	}
	t.Cleanup(func() { verifyReleaseSignatureFunc = origVerify })

	t.Run("community binary installs the same-version Pro build from the broker", func(t *testing.T) {
		setupProRuntimeSwitchTest(t, "6.0.5")
		fixture := newProBrokerFixture(t, "6.0.5", false)
		manager := NewManager(&config.Config{DataPath: t.TempDir()})
		manager.SetProUpdateCredentialSource(fixture.credentialSource())

		err := manager.ApplyProRuntimeSwitch(context.Background(), "stable", InitiatedByUser, InitiatedViaUI)
		// The dummy tarball has no pulse binary, so reaching that check proves
		// download, signature and hash verification all ran on the Pro artifact.
		if err == nil || !strings.Contains(err.Error(), "pulse binary not found") {
			t.Fatalf("expected failure at the pre-install validation stage, got: %v", err)
		}
		if fixture.brokerCalls == 0 || fixture.tarballCalls != 1 || fixture.sshsigCalls != 1 {
			t.Fatalf("expected broker, artifact and signature fetches, got broker=%d tarball=%d sshsig=%d",
				fixture.brokerCalls, fixture.tarballCalls, fixture.sshsigCalls)
		}
	})

	t.Run("older Pro pin is refused before download", func(t *testing.T) {
		setupProRuntimeSwitchTest(t, "6.1.0")
		fixture := newProBrokerFixture(t, "6.0.5", false)
		manager := NewManager(&config.Config{DataPath: t.TempDir()})
		manager.SetProUpdateCredentialSource(fixture.credentialSource())

		err := manager.ApplyProRuntimeSwitch(context.Background(), "stable", InitiatedByUser, InitiatedViaUI)
		if err == nil || !strings.Contains(err.Error(), "older than this install") {
			t.Fatalf("expected the older-release refusal, got: %v", err)
		}
		if fixture.tarballCalls != 0 {
			t.Fatalf("an older pin must not be downloaded, got %d downloads", fixture.tarballCalls)
		}
	})

	t.Run("Pro binary refuses a switch", func(t *testing.T) {
		setupProUpdateTest(t, "6.0.5")
		manager := NewManager(&config.Config{DataPath: t.TempDir()})
		err := manager.ApplyProRuntimeSwitch(context.Background(), "stable", InitiatedByUser, InitiatedViaUI)
		if err == nil || !strings.Contains(err.Error(), "already runs the Pulse Pro runtime") {
			t.Fatalf("expected already-Pro refusal, got: %v", err)
		}
	})

	t.Run("switch needs an activation", func(t *testing.T) {
		setupProRuntimeSwitchTest(t, "6.0.5")
		manager := NewManager(&config.Config{DataPath: t.TempDir()})
		err := manager.ApplyProRuntimeSwitch(context.Background(), "stable", InitiatedByUser, InitiatedViaUI)
		if err == nil || !strings.Contains(err.Error(), "need an activated license") {
			t.Fatalf("expected the activation refusal, got: %v", err)
		}
	})

	t.Run("ordinary community apply still requires a download URL", func(t *testing.T) {
		setupProRuntimeSwitchTest(t, "6.0.5")
		manager := NewManager(&config.Config{DataPath: t.TempDir()})
		err := manager.ApplyUpdate(context.Background(), ApplyUpdateRequest{Channel: "stable"})
		if err == nil || !strings.Contains(err.Error(), "download URL is required") {
			t.Fatalf("expected the download URL requirement, got: %v", err)
		}
	})
}
