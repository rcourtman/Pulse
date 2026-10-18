package updates

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/edition"
)

// A paid licence activated on the public community build unlocks only the
// licence-gated features compiled into that build. Patrol modes, Audit, RBAC,
// and Reporting need the separately compiled Pulse Pro runtime. The switch
// moves such an install onto the Pro runtime through the same license server
// download broker the Pro binary updates from: in place for installs the
// in-app updater already applies to, and as the broker's digest-pinned image
// commands for Docker. The broker decides eligibility from the licence, so a
// community install without a qualifying licence is refused there.

// Reasons a switch is not offered. The frontend renders copy per reason.
const (
	ProRuntimeSwitchReasonAlreadyPro   = "already_pro"
	ProRuntimeSwitchReasonNotActivated = "not_activated"
	ProRuntimeSwitchReasonNotEligible  = "not_eligible"
	ProRuntimeSwitchReasonSourceBuild  = "source_build"
	ProRuntimeSwitchReasonUnsupported  = "unsupported_deployment"
	ProRuntimeSwitchReasonOlderRelease = "older_release"
	// The broker's Pro pin is a prerelease and this install follows the
	// stable channel, which never installs prereleases.
	ProRuntimeSwitchReasonPrereleaseOnly = "prerelease_only"
	ProRuntimeSwitchReasonUnavailable    = "unavailable"
)

const (
	proRuntimeSwitchModeInPlace = "in_place"
	proRuntimeSwitchModeDocker  = "docker"
	proRuntimeSwitchHistoryNote = "Switch to Pulse Pro"
)

// ProRuntimeSwitch describes whether this install can move to the Pulse Pro
// runtime and how.
type ProRuntimeSwitch struct {
	Available      bool   `json:"available"`
	Reason         string `json:"reason,omitempty"`
	CurrentVersion string `json:"currentVersion"`
	// Version is the private Pro release the switch installs.
	Version string `json:"version,omitempty"`
	// InstallInPlace means POST /api/updates/pro-runtime/apply can perform the
	// switch from the browser.
	InstallInPlace bool `json:"installInPlace"`
	// Docker carries the broker's digest-pinned image commands for container
	// installs, which the in-app updater cannot replace.
	Docker *DockerUpdateCommands `json:"docker,omitempty"`
}

// proRuntimeSwitchMode reports how a switch would run for this install, or ""
// when neither path applies. In-place follows the deployments whose update
// plan already applies from the browser; the mock/CI escape hatch mirrors the
// rest of the updater.
func proRuntimeSwitchMode(info *VersionInfo) string {
	if info == nil {
		return ""
	}
	if info.IsDocker || info.DeploymentType == "docker" {
		return proRuntimeSwitchModeDocker
	}
	switch info.DeploymentType {
	case "systemd", "proxmoxve":
		return proRuntimeSwitchModeInPlace
	}
	if dockerUpdatesAllowed() {
		return proRuntimeSwitchModeInPlace
	}
	return ""
}

// isProBrokerRefusal reports a broker reply that says this licence cannot have
// Pro downloads (ineligible plan, lapsed licence, unknown credential), as
// opposed to an outage worth retrying.
func isProBrokerRefusal(err error) bool {
	var statusErr *proBrokerStatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	return statusErr.StatusCode == http.StatusForbidden || statusErr.StatusCode == http.StatusUnauthorized
}

// CheckProRuntimeSwitch reports whether this community install can switch to
// the Pulse Pro runtime. Expected refusals come back as a Reason, not an error.
func (m *Manager) CheckProRuntimeSwitch(ctx context.Context, channel string) (*ProRuntimeSwitch, error) {
	currentInfo, err := GetCurrentVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get current version: %w", err)
	}
	result := &ProRuntimeSwitch{CurrentVersion: currentInfo.Version}

	if edition.IsPro() {
		result.Reason = ProRuntimeSwitchReasonAlreadyPro
		return result, nil
	}
	if currentInfo.IsSourceBuild {
		result.Reason = ProRuntimeSwitchReasonSourceBuild
		return result, nil
	}
	mode := proRuntimeSwitchMode(currentInfo)
	if mode == "" {
		result.Reason = ProRuntimeSwitchReasonUnsupported
		return result, nil
	}
	creds, ok := m.proUpdateCredentials()
	if !ok {
		result.Reason = ProRuntimeSwitchReasonNotActivated
		return result, nil
	}

	channel = m.resolveChannel(channel, currentInfo)
	manifest, err := m.fetchProDownloadManifest(ctx, creds, channel)
	if err != nil {
		if isProBrokerRefusal(err) {
			result.Reason = ProRuntimeSwitchReasonNotEligible
			return result, nil
		}
		result.Reason = ProRuntimeSwitchReasonUnavailable
		return result, nil
	}
	result.Version = strings.TrimPrefix(strings.TrimSpace(manifest.Release.Version), "v")

	if err := ensureSwitchTargetIsNotOlder(currentInfo.Version, manifest.Release.Version); err != nil {
		result.Reason = ProRuntimeSwitchReasonOlderRelease
		return result, nil
	}
	// Apply refuses a prerelease pin on the stable channel, so the check must
	// not offer one: not as an in-place button that would fail, and not as
	// Docker commands that would pin a stable install to a preview image.
	if stableChannelRefusesProPin(channel, manifest) {
		result.Reason = ProRuntimeSwitchReasonPrereleaseOnly
		return result, nil
	}

	if mode == proRuntimeSwitchModeDocker {
		result.Docker = proDockerUpdateCommands(manifest)
		if result.Docker == nil {
			result.Reason = ProRuntimeSwitchReasonUnavailable
			return result, nil
		}
		result.Available = true
		return result, nil
	}
	result.InstallInPlace = true
	result.Available = true
	return result, nil
}

// ensureSwitchTargetIsNotOlder allows the Pro build of the running version: a
// switch changes the runtime, so the same version is the expected target. It
// still refuses an older Pro pin, which would be a silent downgrade.
func ensureSwitchTargetIsNotOlder(currentVersion, targetVersion string) error {
	current, err := ParseVersion(currentVersion)
	if err != nil {
		return nil
	}
	target, err := ParseVersion(targetVersion)
	if err != nil {
		return nil
	}
	if target.Compare(current) < 0 {
		return fmt.Errorf("the Pulse Pro runtime is at %s, older than this install's %s; switch after the next Pulse Pro release", target.String(), current.String())
	}
	return nil
}

// ApplyProRuntimeSwitch installs the Pulse Pro runtime over this community
// binary through the shared apply pipeline (broker download, signature and
// hash checks, pre-install self-test, backup, restart).
func (m *Manager) ApplyProRuntimeSwitch(ctx context.Context, channel string, initiatedBy InitiatedBy, initiatedVia InitiatedVia) error {
	return m.ApplyUpdate(ctx, ApplyUpdateRequest{
		Channel:            channel,
		InitiatedBy:        initiatedBy,
		InitiatedVia:       initiatedVia,
		Notes:              proRuntimeSwitchHistoryNote,
		SwitchToProRuntime: true,
	})
}
