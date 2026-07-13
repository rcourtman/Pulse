package ai

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

const (
	aptWorkflowFindingSource       = "apt-workflow"
	aptHostUpdateFindingKey        = "apt-host-updates"
	aptPackageCacheFindingKey      = "apt-package-cache-pressure"
	aptHostUpdateFindingIDPrefix   = "apt:host-update:"
	aptPackageCacheFindingIDPrefix = "apt:cache-cleanup:"
	aptWorkflowResolveCleared      = "apt_workflow:condition_cleared"
	aptWorkflowResolveRemoved      = "apt_workflow:resource_removed"
)

type aptWorkflowWatcher struct{}

func newAPTWorkflowWatcher() *aptWorkflowWatcher { return &aptWorkflowWatcher{} }

// DetectAPTWorkflowFindings is the deterministic, model-free detector entry
// point used by Patrol and end-to-end action lifecycle proofs. It emits only
// findings supported by fresh canonical host views; active-finding resolution
// remains owned by aptWorkflowWatcher so missing or stale authority cannot be
// mistaken for a cleared condition.
func DetectAPTWorkflowFindings(readState unifiedresources.ReadState, now time.Time) []*Finding {
	if readState == nil {
		return nil
	}
	now = now.UTC()
	var findings []*Finding
	for _, host := range readState.Hosts() {
		if host == nil || strings.TrimSpace(host.ID()) == "" {
			continue
		}
		if finding, _, _ := aptHostUpdateFinding(host, now); finding != nil {
			findings = append(findings, finding)
		}
		if finding, _, _ := aptPackageCacheFinding(host, now); finding != nil {
			findings = append(findings, finding)
		}
	}
	return findings
}

func (w *aptWorkflowWatcher) Observe(state patrolRuntimeState, active []*Finding, now time.Time) (emit []*Finding, resolve []resolveSentinel) {
	if w == nil {
		return nil, nil
	}
	if state.readState == nil {
		state = state.withDerivedProviders()
	}
	if state.readState == nil {
		return nil, nil
	}
	now = now.UTC()
	present := make(map[string]struct{})
	resolved := make(map[string]struct{})
	detected := make(map[string]struct{})
	for _, finding := range DetectAPTWorkflowFindings(state.readState, now) {
		emit = append(emit, finding)
		detected[finding.ID] = struct{}{}
	}
	for _, host := range state.readState.Hosts() {
		if host == nil || strings.TrimSpace(host.ID()) == "" {
			continue
		}
		resourceID := strings.TrimSpace(host.ID())
		present[resourceID] = struct{}{}
		updateID := aptHostUpdateFindingIDPrefix + resourceID
		if _, found := detected[updateID]; !found {
			_, known, cleared := aptHostUpdateFinding(host, now)
			if known && cleared {
				id := aptHostUpdateFindingIDPrefix + resourceID
				resolve = append(resolve, resolveSentinel{DedupKey: id, Reason: aptWorkflowResolveCleared})
				resolved[id] = struct{}{}
			}
		}
		cleanupID := aptPackageCacheFindingIDPrefix + resourceID
		if _, found := detected[cleanupID]; !found {
			_, known, cleared := aptPackageCacheFinding(host, now)
			if known && cleared {
				id := aptPackageCacheFindingIDPrefix + resourceID
				resolve = append(resolve, resolveSentinel{DedupKey: id, Reason: aptWorkflowResolveCleared})
				resolved[id] = struct{}{}
			}
		}
	}
	for _, finding := range active {
		if finding == nil || finding.Source != aptWorkflowFindingSource {
			continue
		}
		if _, ok := present[strings.TrimSpace(finding.ResourceID)]; ok {
			continue
		}
		if _, done := resolved[finding.ID]; done {
			continue
		}
		resolve = append(resolve, resolveSentinel{DedupKey: finding.ID, Reason: aptWorkflowResolveRemoved})
	}
	return emit, resolve
}

func aptHostUpdateFinding(host *unifiedresources.HostView, now time.Time) (*Finding, bool, bool) {
	status := host.PackageUpdates()
	if status == nil || strings.TrimSpace(status.Error) != "" || !status.Supported || strings.TrimSpace(status.Manager) != "apt" || !unifiedresources.ValidHostAPTDigest(status.InventoryHash) || !unifiedresources.HostPackageUpdateTelemetryFresh(status, now) {
		return nil, false, false
	}
	if !aptHostHasCapability(host, "install_os_updates", "host.package_updates") {
		return nil, false, false
	}
	if status.PendingCount <= 0 {
		return nil, true, true
	}
	name := aptWorkflowHostName(host)
	id := aptHostUpdateFindingIDPrefix + host.ID()
	return &Finding{
		ID: id, Key: aptHostUpdateFindingKey, Severity: FindingSeverityWarning, Category: FindingCategoryReliability,
		ResourceID: host.ID(), ResourceName: name, ResourceType: string(unifiedresources.ResourceTypeAgent),
		Title:          fmt.Sprintf("%d operating system update(s) are ready on %s", status.PendingCount, name),
		Description:    "Pulse found standard APT upgrades using fresh Unified Agent telemetry. The governed action refreshes metadata, refuses package removal, and never reboots the host.",
		Impact:         "Leaving operating system updates pending can delay security and reliability fixes.",
		Recommendation: "Review the proposed host update and its current policy before approving it.",
		Evidence:       fmt.Sprintf("pending_updates=%d inventory=%s checked_at=%s received_at=%s reboot_required=%t", status.PendingCount, status.InventoryHash, status.CheckedAt.UTC().Format(time.RFC3339), status.ObservedAt.UTC().Format(time.RFC3339), status.RebootRequired),
		Source:         aptWorkflowFindingSource, DetectedAt: now, LastSeenAt: now,
	}, true, false
}

func aptPackageCacheFinding(host *unifiedresources.HostView, now time.Time) (*Finding, bool, bool) {
	status := host.StorageCleanup()
	if status == nil || strings.TrimSpace(status.Error) != "" || !status.Supported || strings.TrimSpace(status.Provider) != "apt-package-cache" || !unifiedresources.ValidHostAPTDigest(status.Fingerprint) || !unifiedresources.HostStorageCleanupTelemetryFresh(status, now) {
		return nil, false, false
	}
	if !aptHostHasCapability(host, "clean_package_cache", "host.storage_cleanup") {
		return nil, false, false
	}
	disk, pressured := unifiedresources.HostStorageCleanupPressureDisk(host.Disks())
	if status.ReclaimableBytes < unifiedresources.HostStorageCleanupMinReclaimableBytes || !pressured {
		return nil, true, true
	}
	name := aptWorkflowHostName(host)
	id := aptPackageCacheFindingIDPrefix + host.ID()
	return &Finding{
		ID: id, Key: aptPackageCacheFindingKey, Severity: FindingSeverityWarning, Category: FindingCategoryCapacity,
		ResourceID: host.ID(), ResourceName: name, ResourceType: string(unifiedresources.ResourceTypeAgent),
		Title:          fmt.Sprintf("Package downloads can free %s on %s", formatAPTBytes(status.ReclaimableBytes), name),
		Description:    "The filesystem containing the fixed APT package cache is under pressure. Pulse can remove only cached package archives through the governed cleanup action.",
		Impact:         "The pressured filesystem has less room for normal host operation.",
		Recommendation: "Review the proposed package-cache cleanup and the reported reclaimed-space evidence.",
		Evidence:       fmt.Sprintf("reclaimable_bytes=%d filesystem_usage=%.1f fingerprint=%s checked_at=%s received_at=%s", status.ReclaimableBytes, disk.Usage, status.Fingerprint, status.CheckedAt.UTC().Format(time.RFC3339), status.ObservedAt.UTC().Format(time.RFC3339)),
		Source:         aptWorkflowFindingSource, DetectedAt: now, LastSeenAt: now,
	}, true, false
}

func aptHostHasCapability(host *unifiedresources.HostView, name, handler string) bool {
	if host == nil || host.Status() == unifiedresources.StatusOffline {
		return false
	}
	for _, capability := range host.Capabilities() {
		if strings.TrimSpace(capability.Name) == name && strings.TrimSpace(capability.InternalHandler) == handler {
			return true
		}
	}
	return false
}

func aptWorkflowHostName(host *unifiedresources.HostView) string {
	if name := strings.TrimSpace(host.Name()); name != "" {
		return name
	}
	if name := strings.TrimSpace(host.Hostname()); name != "" {
		return name
	}
	return host.ID()
}

func formatAPTBytes(value int64) string {
	const mib = int64(1024 * 1024)
	if value < 1024*mib {
		return fmt.Sprintf("%d MiB", value/mib)
	}
	return fmt.Sprintf("%.1f GiB", float64(value)/float64(1024*mib))
}

func verifyAPTWorkflowFinding(state patrolRuntimeState, findingKey, resourceID string, now time.Time) (bool, bool, error) {
	if findingKey != aptHostUpdateFindingKey && findingKey != aptPackageCacheFindingKey {
		return false, false, nil
	}
	if state.readState == nil {
		state = state.withDerivedProviders()
	}
	if state.readState == nil {
		return false, true, fmt.Errorf("%w: unified resource state unavailable", aicontracts.ErrVerificationUnknown)
	}
	for _, host := range state.readState.Hosts() {
		if host == nil || host.ID() != resourceID {
			continue
		}
		switch findingKey {
		case aptHostUpdateFindingKey:
			finding, known, cleared := aptHostUpdateFinding(host, now)
			if !known {
				return false, true, fmt.Errorf("%w: host update telemetry is not fresh and trustworthy", aicontracts.ErrVerificationUnknown)
			}
			return finding == nil && cleared, true, nil
		case aptPackageCacheFindingKey:
			finding, known, cleared := aptPackageCacheFinding(host, now)
			if !known {
				return false, true, fmt.Errorf("%w: package-cache telemetry is not fresh and trustworthy", aicontracts.ErrVerificationUnknown)
			}
			return finding == nil && cleared, true, nil
		}
	}
	return false, true, fmt.Errorf("%w: governed host resource is no longer present", aicontracts.ErrVerificationUnknown)
}
