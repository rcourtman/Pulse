package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/edition"
)

// proBrokerFixture stands in for the license server download broker plus the
// signed R2 URLs it hands out: /v1/downloads/pulse-pro returns the manifest,
// and the artifact/.sshsig endpoints simulate the presigned object URLs.
type proBrokerFixture struct {
	t          *testing.T
	server     *httptest.Server
	version    string
	prerelease bool
	tarball    []byte
	sha256Hex  string // manifest hash; may deliberately mismatch the tarball
	omitSSHSig bool   // drop the sshsig_url from the manifest

	brokerCalls  int
	tarballCalls int
	sshsigCalls  int
}

func newProBrokerFixture(t *testing.T, version string, prerelease bool) *proBrokerFixture {
	t.Helper()

	f := &proBrokerFixture{t: t, version: version, prerelease: prerelease}
	f.tarball = buildDummyTarball(t)
	digest := sha256.Sum256(f.tarball)
	f.sha256Hex = hex.EncodeToString(digest[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/downloads/pulse-pro", func(w http.ResponseWriter, r *http.Request) {
		f.brokerCalls++
		if got := r.Header.Get("Authorization"); got != "Bearer pit_live_test" {
			t.Errorf("broker got Authorization %q, want installation token bearer", got)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if got := r.Header.Get("X-Pulse-Instance-Fingerprint"); got != "fp-test" {
			t.Errorf("broker got fingerprint %q, want fp-test", got)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if got := r.URL.Query().Get("target"); got != proUpdateTarget() {
			t.Errorf("broker got target %q, want %q", got, proUpdateTarget())
		}

		artifact := map[string]any{
			"target":       proUpdateTarget(),
			"filename":     fmt.Sprintf("pulse-pro-v%s-%s.tar.gz", f.version, proUpdateTarget()),
			"sha256":       f.sha256Hex,
			"download_url": f.server.URL + "/r2/pulse-pro.tar.gz?X-Amz-Signature=signed",
		}
		if !f.omitSSHSig {
			artifact["sshsig_url"] = f.server.URL + "/r2/pulse-pro.tar.gz.sshsig?X-Amz-Signature=signed"
		}
		resp := map[string]any{
			"release": map[string]any{
				"version":    f.version,
				"prerelease": f.prerelease,
			},
			"artifacts": []any{artifact},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode broker response: %v", err)
		}
	})
	mux.HandleFunc("/r2/pulse-pro.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		f.tarballCalls++
		if r.URL.Query().Get("X-Amz-Signature") == "" {
			t.Error("artifact download must use the presigned URL from the broker manifest")
		}
		if _, err := w.Write(f.tarball); err != nil {
			t.Errorf("write tarball: %v", err)
		}
	})
	mux.HandleFunc("/r2/pulse-pro.tar.gz.sshsig", func(w http.ResponseWriter, r *http.Request) {
		f.sshsigCalls++
		if r.URL.Query().Get("X-Amz-Signature") == "" {
			t.Error("signature download must use the presigned URL from the broker manifest")
		}
		if _, err := w.Write([]byte("dummy-sshsig")); err != nil {
			t.Errorf("write sshsig: %v", err)
		}
	})

	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *proBrokerFixture) credentialSource() func() (ProUpdateCredentials, bool) {
	return func() (ProUpdateCredentials, bool) {
		return ProUpdateCredentials{
			LicenseServerURL:    f.server.URL,
			InstallationToken:   "pit_live_test",
			InstanceFingerprint: "fp-test",
		}, true
	}
}

func setupProUpdateTest(t *testing.T, currentVersion string) {
	t.Helper()
	t.Setenv("PULSE_ALLOW_DOCKER_UPDATES", "true")
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("PULSE_INSTALL_DIR", t.TempDir())

	oldBuildVersion := BuildVersion
	BuildVersion = currentVersion
	t.Cleanup(func() { BuildVersion = oldBuildVersion })

	edition.SetEdition(edition.Pro)
	t.Cleanup(func() { edition.SetEdition(edition.Community) })
}

// TestCheckForUpdatesProUsesBroker proves the Pro binary's update check reads
// the license server download broker, never GitHub: the offered version is the
// broker's pinned private release and the download URL is the broker intent
// URL, not a public release asset.
func TestCheckForUpdatesProUsesBroker(t *testing.T) {
	setupProUpdateTest(t, "6.0.0")

	t.Run("offers the broker-pinned release", func(t *testing.T) {
		fixture := newProBrokerFixture(t, "6.0.5", false)
		manager := NewManager(&config.Config{DataPath: t.TempDir()})
		manager.SetProUpdateCredentialSource(fixture.credentialSource())

		info, err := manager.CheckForUpdatesWithChannel(context.Background(), "stable")
		if err != nil {
			t.Fatalf("CheckForUpdatesWithChannel: %v", err)
		}
		if fixture.brokerCalls == 0 {
			t.Fatal("expected the update check to query the download broker")
		}
		if !info.Available {
			t.Fatalf("expected update 6.0.0 → 6.0.5 to be available, got %+v", info)
		}
		if info.LatestVersion != "6.0.5" {
			t.Fatalf("LatestVersion = %q, want 6.0.5", info.LatestVersion)
		}
		if !strings.Contains(info.DownloadURL, proDownloadBrokerPath) {
			t.Fatalf("DownloadURL %q must be the broker intent URL", info.DownloadURL)
		}
		if !strings.Contains(info.DownloadURL, "version=v6.0.5") {
			t.Fatalf("DownloadURL %q must carry the target version for the apply channel guard", info.DownloadURL)
		}
		// The API handler runs the shared channel guard on this URL before
		// readiness checks; it must be able to infer the version from it.
		target, err := ValidateApplyTargetVersion("stable", info.DownloadURL)
		if err != nil {
			t.Fatalf("ValidateApplyTargetVersion on the broker intent URL: %v", err)
		}
		if target != "v6.0.5" {
			t.Fatalf("inferred target version = %q, want v6.0.5", target)
		}
	})

	t.Run("stable channel skips a prerelease broker pin", func(t *testing.T) {
		fixture := newProBrokerFixture(t, "6.1.0-rc.1", true)
		manager := NewManager(&config.Config{DataPath: t.TempDir()})
		manager.SetProUpdateCredentialSource(fixture.credentialSource())

		info, err := manager.CheckForUpdatesWithChannel(context.Background(), "stable")
		if err != nil {
			t.Fatalf("CheckForUpdatesWithChannel: %v", err)
		}
		if info.Available {
			t.Fatalf("stable channel must not offer prerelease %q, got %+v", "6.1.0-rc.1", info)
		}
		if !strings.Contains(info.Warning, "prerelease") {
			t.Fatalf("expected a prerelease-pin warning, got %q", info.Warning)
		}
	})

	t.Run("rc channel offers a prerelease broker pin", func(t *testing.T) {
		fixture := newProBrokerFixture(t, "6.1.0-rc.1", true)
		manager := NewManager(&config.Config{DataPath: t.TempDir()})
		manager.SetProUpdateCredentialSource(fixture.credentialSource())

		info, err := manager.CheckForUpdatesWithChannel(context.Background(), "rc")
		if err != nil {
			t.Fatalf("CheckForUpdatesWithChannel: %v", err)
		}
		if !info.Available || !info.IsPrerelease {
			t.Fatalf("rc channel should offer prerelease 6.1.0-rc.1, got %+v", info)
		}
	})

	t.Run("without activation reports unavailable with guidance", func(t *testing.T) {
		manager := NewManager(&config.Config{DataPath: t.TempDir()})

		info, err := manager.CheckForUpdatesWithChannel(context.Background(), "stable")
		if err != nil {
			t.Fatalf("CheckForUpdatesWithChannel: %v", err)
		}
		if info.Available {
			t.Fatalf("unactivated Pro must not offer updates, got %+v", info)
		}
		if !strings.Contains(info.Warning, "activated license") {
			t.Fatalf("expected activation guidance in warning, got %q", info.Warning)
		}
	})
}

// TestApplyUpdateProDownloadsFromBroker proves the Pro apply path resolves
// fresh signed URLs from the broker, downloads the private archive, verifies
// its .sshsig against the pinned key path and its sha256 against the manifest,
// and never fetches a public community asset. The dummy tarball deliberately
// contains no pulse binary, so a "pulse binary not found" failure at the apply
// stage is the proof that every Pro-specific stage before it succeeded.
func TestApplyUpdateProDownloadsFromBroker(t *testing.T) {
	setupProUpdateTest(t, "6.0.0")

	origVerify := verifyReleaseSignatureFunc
	verifyReleaseSignatureFunc = func(ctx context.Context, targetPath, signaturePath string) error {
		return nil
	}
	t.Cleanup(func() { verifyReleaseSignatureFunc = origVerify })

	t.Run("full flow through verification and extraction", func(t *testing.T) {
		fixture := newProBrokerFixture(t, "6.0.5", false)
		manager := NewManager(&config.Config{DataPath: t.TempDir()})
		manager.SetProUpdateCredentialSource(fixture.credentialSource())

		applyURL, err := proUpdateApplyURL(fixture.server.URL, "6.0.5")
		if err != nil {
			t.Fatalf("proUpdateApplyURL: %v", err)
		}

		err = manager.ApplyUpdate(context.Background(), ApplyUpdateRequest{DownloadURL: applyURL, Channel: "stable"})
		if err == nil {
			t.Fatal("expected apply to fail at the pre-install validation stage (dummy tarball has no pulse binary)")
		}
		if !strings.Contains(err.Error(), "pulse binary not found") {
			t.Fatalf("expected failure at the validation stage after all Pro verification stages passed, got: %v", err)
		}
		if fixture.brokerCalls == 0 {
			t.Fatal("apply must re-resolve signed URLs from the broker")
		}
		if fixture.tarballCalls != 1 {
			t.Fatalf("expected exactly one artifact download, got %d", fixture.tarballCalls)
		}
		if fixture.sshsigCalls != 1 {
			t.Fatalf("expected exactly one signature download, got %d", fixture.sshsigCalls)
		}
	})

	t.Run("corrupt pulse binary fails the pre-install self-test", func(t *testing.T) {
		fixture := newProBrokerFixture(t, "6.0.5", false)
		fixture.tarball = buildTarballWithPulseBinary(t, []byte{0x00, 0x01, 0x02, 0x03})
		digest := sha256.Sum256(fixture.tarball)
		fixture.sha256Hex = hex.EncodeToString(digest[:])
		manager := NewManager(&config.Config{DataPath: t.TempDir()})
		manager.SetProUpdateCredentialSource(fixture.credentialSource())

		applyURL, err := proUpdateApplyURL(fixture.server.URL, "6.0.5")
		if err != nil {
			t.Fatalf("proUpdateApplyURL: %v", err)
		}

		err = manager.ApplyUpdate(context.Background(), ApplyUpdateRequest{DownloadURL: applyURL, Channel: "stable"})
		if err == nil || !strings.Contains(err.Error(), "self-test") {
			t.Fatalf("expected the pre-install self-test to reject the corrupt binary, got: %v", err)
		}
	})

	t.Run("manifest hash mismatch fails closed", func(t *testing.T) {
		fixture := newProBrokerFixture(t, "6.0.5", false)
		fixture.sha256Hex = strings.Repeat("0", 64)
		manager := NewManager(&config.Config{DataPath: t.TempDir()})
		manager.SetProUpdateCredentialSource(fixture.credentialSource())

		applyURL, err := proUpdateApplyURL(fixture.server.URL, "6.0.5")
		if err != nil {
			t.Fatalf("proUpdateApplyURL: %v", err)
		}

		err = manager.ApplyUpdate(context.Background(), ApplyUpdateRequest{DownloadURL: applyURL, Channel: "stable"})
		if err == nil || !strings.Contains(err.Error(), "checksum verification failed") {
			t.Fatalf("expected checksum failure, got: %v", err)
		}
	})

	t.Run("missing signature URL fails closed", func(t *testing.T) {
		fixture := newProBrokerFixture(t, "6.0.5", false)
		fixture.omitSSHSig = true
		manager := NewManager(&config.Config{DataPath: t.TempDir()})
		manager.SetProUpdateCredentialSource(fixture.credentialSource())

		applyURL, err := proUpdateApplyURL(fixture.server.URL, "6.0.5")
		if err != nil {
			t.Fatalf("proUpdateApplyURL: %v", err)
		}

		err = manager.ApplyUpdate(context.Background(), ApplyUpdateRequest{DownloadURL: applyURL, Channel: "stable"})
		if err == nil || !strings.Contains(err.Error(), ".sshsig") {
			t.Fatalf("expected missing-signature refusal, got: %v", err)
		}
	})

	t.Run("stable channel refuses a prerelease broker pin at apply time", func(t *testing.T) {
		fixture := newProBrokerFixture(t, "6.1.0-rc.1", true)
		manager := NewManager(&config.Config{DataPath: t.TempDir()})
		manager.SetProUpdateCredentialSource(fixture.credentialSource())

		applyURL, err := proUpdateApplyURL(fixture.server.URL, "6.1.0-rc.1")
		if err != nil {
			t.Fatalf("proUpdateApplyURL: %v", err)
		}

		err = manager.ApplyUpdate(context.Background(), ApplyUpdateRequest{DownloadURL: applyURL, Channel: "stable"})
		if err == nil || !strings.Contains(err.Error(), "stable channel cannot install prerelease builds") {
			t.Fatalf("expected the stable-channel guard, got: %v", err)
		}
	})
}
