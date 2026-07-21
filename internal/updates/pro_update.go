package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

// The compiled Pulse Pro binary must never update off the public community
// releases: the GitHub assets are community builds, so applying one would
// replace the Pro binary and silently strip Audit, RBAC, Reporting, and SSO.
// Instead the Pro binary checks and applies updates through the license
// server's authenticated download broker (GET /v1/downloads/pulse-pro), which
// returns the pinned private release version plus short-lived signed URLs for
// the archive, its sha256, and its .sshsig sidecar. The archive is signed with
// the same pinned pulse-installer key the community release path already
// verifies, so the Pro path runs at the same trust bar.
const (
	proDownloadBrokerPath = "/v1/downloads/pulse-pro"

	// proApplyVersionParam carries the target version in the broker apply URL
	// so the shared handler-side channel guard (ValidateApplyTargetVersion)
	// can classify it without a network call. ApplyUpdate re-resolves fresh
	// signed URLs from the broker at apply time; the URL the frontend echoes
	// back is only an intent marker, never fetched directly.
	proApplyVersionParam = "version"

	proBrokerFetchTimeout       = 30 * time.Second
	maxProBrokerResponseBytes   = 1 << 20 // 1 MiB
	proUpdatePortalInstructions = "you can still download the archive and its .sshsig sidecar from https://pulserelay.pro/download.html and run install.sh --archive"
)

// ProUpdateCredentials carries what the license-gated download broker needs
// for a bound installation: the activation's license server, the installation
// token, and the instance fingerprint header value.
type ProUpdateCredentials struct {
	LicenseServerURL    string
	InstallationToken   string
	InstanceFingerprint string
}

// SetProUpdateCredentialSource wires a lazy credential source consulted on
// each Pro update check/apply, so activation performed after startup is picked
// up without restarting. Call during startup wiring, before requests are
// served. The source returns false when no usable activation exists.
func (m *Manager) SetProUpdateCredentialSource(source func() (ProUpdateCredentials, bool)) {
	m.proCredentialSource = source
}

func (m *Manager) proUpdateCredentials() (ProUpdateCredentials, bool) {
	if m.proCredentialSource == nil {
		return ProUpdateCredentials{}, false
	}
	creds, ok := m.proCredentialSource()
	if !ok {
		return ProUpdateCredentials{}, false
	}
	if strings.TrimSpace(creds.LicenseServerURL) == "" || strings.TrimSpace(creds.InstallationToken) == "" {
		return ProUpdateCredentials{}, false
	}
	return creds, true
}

func errProUpdateNotActivated() error {
	return fmt.Errorf("Pulse Pro updates need an activated license: the updater downloads private Pulse Pro builds from the license server, which requires this installation's activation credentials; activate in Settings → License, or %s", proUpdatePortalInstructions)
}

// proBrokerResponse is the subset of the broker's response the updater needs.
type proBrokerResponse struct {
	Release struct {
		Version    string `json:"version"`
		Prerelease bool   `json:"prerelease"`
		Channel    string `json:"channel"`
	} `json:"release"`
	Artifacts []proBrokerArtifact `json:"artifacts"`
	Docker    *proBrokerDocker    `json:"docker"`
}

// proBrokerDocker mirrors the broker's docker block: the digest-pinned private
// Pro image plus ready-to-run login/compose commands. The broker deliberately
// never emits mutable-tag commands (a customer compose file pins the previous
// digest, so a plain `docker compose pull` can never update it).
type proBrokerDocker struct {
	Registry           string `json:"registry"`
	Image              string `json:"image"`
	Tag                string `json:"tag"`
	ImageDigest        string `json:"image_digest"`
	LoginCommand       string `json:"login_command"`
	ComposePullCommand string `json:"compose_pull_command"`
	ComposeUpCommand   string `json:"compose_up_command"`
}

type proBrokerArtifact struct {
	Target          string `json:"target"`
	Filename        string `json:"filename"`
	SHA256          string `json:"sha256"`
	DownloadURL     string `json:"download_url"`
	SSHSignatureURL string `json:"sshsig_url"`
}

// DockerUpdateCommands carries the digest-pinned Pulse Pro image reference and
// the copyable update commands for Docker deployments of the Pro binary. The
// self-updater cannot replace a binary inside a container, so this is the
// supported Docker update path: the commands come from the license server
// download broker and always pin the exact image digest. The community
// `docker pull rcourtman/pulse:<tag>` guidance must never be shown to a Pro
// container; following it silently downgrades the install to the community
// build (same failure mode the broker-based binary self-update already fixed).
type DockerUpdateCommands struct {
	Version            string `json:"version"`
	Image              string `json:"image"`
	ImageDigest        string `json:"imageDigest"`
	LoginCommand       string `json:"loginCommand,omitempty"`
	ComposePullCommand string `json:"composePullCommand"`
	ComposeUpCommand   string `json:"composeUpCommand"`
}

var dockerImageDigestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// proDockerUpdateCommands extracts the Docker update commands from a broker
// manifest. Fails closed: if the block is missing, incomplete, or either
// compose command does not reference the digest-pinned image, it returns nil
// rather than surface a command that could pull a mutable tag.
func proDockerUpdateCommands(manifest *proBrokerResponse) *DockerUpdateCommands {
	if manifest == nil || manifest.Docker == nil {
		return nil
	}
	d := manifest.Docker
	digest := strings.ToLower(strings.TrimSpace(d.ImageDigest))
	image := strings.TrimSpace(d.Image)
	pull := strings.TrimSpace(d.ComposePullCommand)
	up := strings.TrimSpace(d.ComposeUpCommand)
	if image == "" || pull == "" || up == "" || !dockerImageDigestRe.MatchString(digest) {
		return nil
	}
	pinnedRef := image + "@" + digest
	if !strings.Contains(pull, pinnedRef) || !strings.Contains(up, pinnedRef) {
		return nil
	}
	return &DockerUpdateCommands{
		Version:            "v" + strings.TrimPrefix(strings.TrimSpace(manifest.Release.Version), "v"),
		Image:              image,
		ImageDigest:        digest,
		LoginCommand:       strings.TrimSpace(d.LoginCommand),
		ComposePullCommand: pull,
		ComposeUpCommand:   up,
	}
}

type proBrokerErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// proUpdateTarget maps the running architecture to the broker's artifact
// target naming (same mapping the community release assets use).
func proUpdateTarget() string {
	arch := runtime.GOARCH
	if arch == "arm" {
		arch = "armv7"
	}
	return "linux-" + arch
}

func proBrokerBaseURL(licenseServerURL string) (*url.URL, error) {
	baseURL, err := securityutil.NormalizeHTTPBaseURL(licenseServerURL, "https")
	if err != nil {
		return nil, fmt.Errorf("invalid license server URL: %w", err)
	}
	return baseURL, nil
}

// proUpdateApplyURL builds the stable broker-intent URL returned as
// UpdateInfo.DownloadURL for Pro binaries. It embeds the version tag so the
// shared apply-target validation can infer it; it is never fetched directly.
func proUpdateApplyURL(licenseServerURL, version string) (string, error) {
	baseURL, err := proBrokerBaseURL(licenseServerURL)
	if err != nil {
		return "", err
	}
	target, err := securityutil.ResolveRelativeURL(baseURL, proDownloadBrokerPath)
	if err != nil {
		return "", fmt.Errorf("build Pulse Pro download URL: %w", err)
	}
	query := target.Query()
	query.Set(proApplyVersionParam, "v"+strings.TrimPrefix(strings.TrimSpace(version), "v"))
	target.RawQuery = query.Encode()
	return target.String(), nil
}

// validateProApplyRequestURL checks that an apply request on the Pro binary
// carries the broker-intent URL for this installation's license server, so a
// stale or hand-crafted community download URL can never reach the download
// stage on a Pro binary.
func validateProApplyRequestURL(rawURL, licenseServerURL string) error {
	requested, err := securityutil.NormalizeAbsoluteHTTPURL(rawURL)
	if err != nil {
		return fmt.Errorf("invalid download URL: %w", err)
	}
	baseURL, err := proBrokerBaseURL(licenseServerURL)
	if err != nil {
		return err
	}
	expected, err := securityutil.ResolveRelativeURL(baseURL, proDownloadBrokerPath)
	if err != nil {
		return fmt.Errorf("invalid download URL")
	}
	if requested.Scheme != expected.Scheme || !strings.EqualFold(requested.Host, expected.Host) || requested.Path != expected.Path {
		return fmt.Errorf("invalid download URL: Pulse Pro updates install from the license server download broker (%s)", expected.String())
	}
	return nil
}

// fetchProDownloadManifest queries the license server download broker for the
// current private Pulse Pro release and signed artifact URLs for this
// architecture.
func (m *Manager) fetchProDownloadManifest(ctx context.Context, creds ProUpdateCredentials, channel string) (*proBrokerResponse, error) {
	baseURL, err := proBrokerBaseURL(creds.LicenseServerURL)
	if err != nil {
		return nil, err
	}
	target, err := securityutil.ResolveRelativeURL(baseURL, proDownloadBrokerPath)
	if err != nil {
		return nil, fmt.Errorf("build Pulse Pro download broker URL: %w", err)
	}
	query := target.Query()
	query.Set("target", proUpdateTarget())
	// rc-channel installs ask the dual-channel broker for the RC slot; the
	// stable default is left implicit so the URL is unchanged for brokers
	// that predate the channel parameter.
	if channel == "rc" {
		query.Set("channel", "rc")
	}
	target.RawQuery = query.Encode()

	headers := map[string]string{
		"Accept":        "application/json",
		"Authorization": "Bearer " + creds.InstallationToken,
		"User-Agent":    "Pulse-Update-Checker",
	}
	if fingerprint := strings.TrimSpace(creds.InstanceFingerprint); fingerprint != "" {
		headers["X-Pulse-Instance-Fingerprint"] = fingerprint
	}

	client := &http.Client{Timeout: proBrokerFetchTimeout}
	resp, err := m.getWithRetry(ctx, client, target, headers, "fetch Pulse Pro download manifest")
	if err != nil {
		return nil, fmt.Errorf("fetch Pulse Pro download manifest: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProBrokerResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Pulse Pro download manifest: %w", err)
	}
	if int64(len(body)) > maxProBrokerResponseBytes {
		return nil, fmt.Errorf("Pulse Pro download manifest exceeds %d bytes", maxProBrokerResponseBytes)
	}

	if resp.StatusCode != http.StatusOK {
		detail := ""
		var brokerErr proBrokerErrorResponse
		if json.Unmarshal(body, &brokerErr) == nil && brokerErr.Error.Message != "" {
			detail = brokerErr.Error.Message
			if brokerErr.Error.Code != "" {
				detail = brokerErr.Error.Code + ": " + detail
			}
		}
		if detail == "" {
			detail = strings.TrimSpace(string(body))
		}
		if detail == "" {
			detail = resp.Status
		}
		return nil, fmt.Errorf("Pulse Pro download broker returned status %d: %s", resp.StatusCode, detail)
	}

	var manifest proBrokerResponse
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("decode Pulse Pro download manifest: %w", err)
	}
	if strings.TrimSpace(manifest.Release.Version) == "" {
		return nil, fmt.Errorf("Pulse Pro download manifest is missing a release version")
	}
	return &manifest, nil
}

// checkProUpdates is the Pro-binary counterpart to the GitHub release check:
// it compares the current version against the private release pinned on the
// download broker. Never touches GitHub.
func (m *Manager) checkProUpdates(ctx context.Context, channel string, currentInfo *VersionInfo, currentVer *Version) (*UpdateInfo, error) {
	creds, ok := m.proUpdateCredentials()
	if !ok {
		return &UpdateInfo{
			Available:      false,
			CurrentVersion: currentInfo.Version,
			LatestVersion:  currentInfo.Version,
			Warning:        "Update checks are unavailable: " + errProUpdateNotActivated().Error(),
		}, nil
	}

	manifest, err := m.fetchProDownloadManifest(ctx, creds, channel)
	if err != nil {
		return nil, err
	}

	latestVer, err := ParseVersion(manifest.Release.Version)
	if err != nil {
		return nil, fmt.Errorf("parse Pulse Pro release version %q: %w", manifest.Release.Version, err)
	}
	isPrerelease := manifest.Release.Prerelease || latestVer.IsPrerelease()

	// The broker pins a single release. A stable-channel install must not be
	// offered a prerelease pin; report "no update" with an explanatory warning
	// instead of dangling an apply that the channel guard would refuse.
	if channel == "stable" && isPrerelease && latestVer.IsNewerThan(currentVer) {
		return &UpdateInfo{
			Available:      false,
			CurrentVersion: currentInfo.Version,
			LatestVersion:  currentInfo.Version,
			Warning:        fmt.Sprintf("The private Pulse Pro release channel currently serves prerelease %s; stable-channel installs skip prereleases.", manifest.Release.Version),
		}, nil
	}

	downloadURL, err := proUpdateApplyURL(creds.LicenseServerURL, manifest.Release.Version)
	if err != nil {
		return nil, err
	}

	isMajorUpgrade := latestVer.Major > currentVer.Major
	info := &UpdateInfo{
		Available:      latestVer.IsNewerThan(currentVer),
		CurrentVersion: currentInfo.Version,
		LatestVersion:  strings.TrimPrefix(manifest.Release.Version, "v"),
		ReleaseNotes:   "",
		DownloadURL:    downloadURL,
		IsPrerelease:   isPrerelease,
		IsMajorUpgrade: isMajorUpgrade,
	}
	// A Pro binary in a container cannot self-update (ApplyUpdate refuses in
	// Docker), so relay the broker's digest-pinned image commands instead;
	// they ride the same channel-guarded manifest fetch as the version check.
	if currentInfo.IsDocker || currentInfo.DeploymentType == "docker" {
		info.DockerUpdate = proDockerUpdateCommands(manifest)
	}
	info.Warning = updateWarning(info.Available, isMajorUpgrade, isPrerelease, currentVer.Major, latestVer.Major)
	return info, nil
}

// resolvedUpdateArtifact carries where ApplyUpdate downloads the release from
// and how it verifies it. The community path derives the .sshsig sidecar and
// SHA256SUMS location from the download URL; the Pro path carries explicit
// signed URLs and an inline hash from the broker manifest.
type resolvedUpdateArtifact struct {
	downloadURL string
	sshsigURL   string // "" → derive <downloadURL>.sshsig
	sha256      string // "" → discover a SHA256SUMS manifest next to downloadURL
	version     string // target version tag, e.g. v6.0.5
}

// resolveProUpdateArtifact fetches fresh signed URLs from the broker at apply
// time (the signed URLs expire in minutes, so anything captured at check time
// is stale by design) and enforces the channel guard against the broker's
// current pin.
func (m *Manager) resolveProUpdateArtifact(ctx context.Context, channel string) (resolvedUpdateArtifact, error) {
	creds, ok := m.proUpdateCredentials()
	if !ok {
		return resolvedUpdateArtifact{}, errProUpdateNotActivated()
	}

	manifest, err := m.fetchProDownloadManifest(ctx, creds, channel)
	if err != nil {
		return resolvedUpdateArtifact{}, err
	}

	latestVer, err := ParseVersion(manifest.Release.Version)
	if err != nil {
		return resolvedUpdateArtifact{}, fmt.Errorf("parse Pulse Pro release version %q: %w", manifest.Release.Version, err)
	}
	if channel == "stable" && (manifest.Release.Prerelease || latestVer.IsPrerelease()) {
		return resolvedUpdateArtifact{}, fmt.Errorf("stable channel cannot install prerelease builds (the private Pulse Pro channel currently serves %s)", manifest.Release.Version)
	}

	wantTarget := proUpdateTarget()
	var artifact *proBrokerArtifact
	for i := range manifest.Artifacts {
		if manifest.Artifacts[i].Target == wantTarget {
			artifact = &manifest.Artifacts[i]
			break
		}
	}
	if artifact == nil {
		return resolvedUpdateArtifact{}, fmt.Errorf("Pulse Pro download broker has no artifact for target %q", wantTarget)
	}
	if strings.TrimSpace(artifact.DownloadURL) == "" {
		return resolvedUpdateArtifact{}, fmt.Errorf("Pulse Pro artifact %q is missing a download URL", wantTarget)
	}
	// Fail closed: the Pro path never installs an archive it cannot verify
	// against both the pinned signing key and the manifest hash.
	if strings.TrimSpace(artifact.SSHSignatureURL) == "" {
		return resolvedUpdateArtifact{}, fmt.Errorf("Pulse Pro artifact %q is missing its .sshsig signature URL", wantTarget)
	}
	if strings.TrimSpace(artifact.SHA256) == "" {
		return resolvedUpdateArtifact{}, fmt.Errorf("Pulse Pro artifact %q is missing its sha256", wantTarget)
	}

	return resolvedUpdateArtifact{
		downloadURL: artifact.DownloadURL,
		sshsigURL:   artifact.SSHSignatureURL,
		sha256:      strings.ToLower(strings.TrimSpace(artifact.SHA256)),
		version:     "v" + strings.TrimPrefix(strings.TrimSpace(manifest.Release.Version), "v"),
	}, nil
}

// downloadAndVerifySignatureFromURL fetches an explicit .sshsig URL (the
// broker hands out signed sidecar URLs that cannot be derived by suffixing
// the artifact URL) and verifies it against assetPath with the same pinned
// pulse-installer trust root as the community path.
func (m *Manager) downloadAndVerifySignatureFromURL(ctx context.Context, sigURLRaw, assetPath string) error {
	sigURL, err := securityutil.NormalizeAbsoluteHTTPURL(sigURLRaw)
	if err != nil {
		return fmt.Errorf("invalid signature URL: %w", err)
	}

	client := &http.Client{Timeout: signatureFetchTimeout}
	resp, err := m.getWithRetry(ctx, client, sigURL, nil, "download release signature")
	if err != nil {
		return fmt.Errorf("fetch %s: %w", sigURL.String(), err)
	}
	defer resp.Body.Close()

	return readSignatureAndVerify(ctx, resp, sigURL.String(), assetPath)
}

// verifyFileSHA256 checks a downloaded file against an expected hex digest
// carried inline by the broker manifest.
func verifyFileSHA256(path, expectedHex string) error {
	expected := strings.ToLower(strings.TrimSpace(expectedHex))
	if len(expected) != sha256.Size*2 {
		return fmt.Errorf("invalid expected sha256 %q", expectedHex)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %q for checksum: %w", path, err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("compute checksum for %q: %w", path, err)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}
