// findings_pdm_alert_bridge.go implements a poll-based bridge from Proxmox
// Datacenter Manager (PDM) into the Pulse findings store. On every patrol
// observe trip the bridge fetches the PDM resource list via the
// pdmAlertSource interface, diffs each resource's status against a prior-state
// map, and returns reliability findings for resources that have transitioned
// into a known-offline state. Resources that come back online emit a lazy
// resolve sentinel on the next observe trip -- there is no sweep goroutine.
//
// MVP: the real HTTP client is a follow-on; this lane ships only the
// interface, the diff machinery, and a nil-source guard at the call site.
// pdmAlertBridgeConfig is reserved for the future env-based credential
// surface; the MVP leaves it zero-valued.
package ai

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	// PDMAlertFindingPrefix is the dedup-key prefix for PDM resource alerts.
	// Concrete keys are prefix + ":" + remoteID + "/" + resourceType + "/" + resourceName.
	PDMAlertFindingPrefix = "pdm:resource:offline"

	pdmAlertSourceLabel   = "pdm-alert-bridge"
	pdmAlertResolveReason = "pdm:resource:back_online"
)

// pdmAlertSource is the interface the bridge uses to fetch resource
// snapshots. Unit tests inject a deterministic fake; the real implementation
// (future) makes an authenticated HTTP call to /api2/json/resources.
type pdmAlertSource interface {
	ResourceList(ctx context.Context) ([]pdmResource, error)
}

// pdmResource is the minimal shape Pulse needs from a PDM resource list item.
type pdmResource struct {
	ID       string // unique per remote+type+name, e.g. "datacenter-a/node/pve1"
	RemoteID string // PDM remote cluster identifier, e.g. "datacenter-a"
	Name     string // resource name, e.g. "pve1"
	Type     string // "node", "qemu", "lxc", "storage"
	Status   string // "online", "offline", "running", "stopped", "failed", "unknown"
}

// pdmAlertBridgeConfig is reserved for the forward-compatible credential
// surface (PDM_API_URL, PDM_API_TOKEN). MVP leaves it zero-valued.
type pdmAlertBridgeConfig struct{}

// pdmAlertBridge diffs PDM resource status across observe trips.
type pdmAlertBridge struct {
	mu     sync.Mutex
	source pdmAlertSource
	prior  map[string]string // resource key -> last known status
}

// newPDMAlertBridge returns a bridge bound to the given source. A nil source
// is permitted: the constructor returns a bridge whose Observe is a no-op
// until a real source is injected.
func newPDMAlertBridge(source pdmAlertSource) *pdmAlertBridge {
	return &pdmAlertBridge{
		source: source,
		prior:  make(map[string]string),
	}
}

// Observe polls the configured source, diffs each resource's status against
// the prior-state map, and returns (emit, resolve) sentinels. The first
// observe trip for a given resource seeds prior state and returns nothing
// for that resource. Subsequent trips:
//   - status transitioned into a known-offline state -> emit one finding.
//   - status transitioned back to online from a known-offline state -> emit
//     one resolve sentinel.
//   - status of "unknown" or other non-actionable values -> ignored.
//
// Concurrency: holds b.mu for the duration of the trip.
func (b *pdmAlertBridge) Observe(ctx context.Context, now time.Time) (emit []*Finding, resolve []resolveSentinel) {
	if b == nil || b.source == nil {
		return nil, nil
	}
	resources, err := b.source.ResourceList(ctx)
	if err != nil {
		return nil, nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	seen := make(map[string]struct{}, len(resources))
	for _, r := range resources {
		key := pdmResourceKey(r)
		seen[key] = struct{}{}

		if !isPDMStatusActionable(r.Status) {
			// Don't seed unknown/empty status as prior state -- we can't
			// distinguish a flap from a genuine transition later.
			continue
		}

		prev, hadPrior := b.prior[key]
		b.prior[key] = r.Status

		if !hadPrior {
			// Seed only -- no emit on first observation of this resource.
			continue
		}
		if prev == r.Status {
			continue
		}

		nowOffline := isPDMOffline(r.Status)
		wasOffline := isPDMOffline(prev)

		switch {
		case nowOffline && !wasOffline:
			emit = append(emit, buildPDMAlertFinding(r, now))
		case !nowOffline && wasOffline:
			resolve = append(resolve, resolveSentinel{
				DedupKey: pdmAlertDedupKey(r),
				Reason:   pdmAlertResolveReason,
			})
		}
	}

	// Drop prior-state entries for resources that disappeared from the list
	// entirely; we can't reason about them and they would otherwise leak.
	for k := range b.prior {
		if _, ok := seen[k]; !ok {
			delete(b.prior, k)
		}
	}

	return emit, resolve
}

// isPDMStatusActionable reports whether a status value is one we can diff
// against. Empty and "unknown" are treated as non-actionable so we don't
// seed them as prior state.
func isPDMStatusActionable(status string) bool {
	switch status {
	case "online", "running", "offline", "stopped", "failed":
		return true
	}
	return false
}

// isPDMOffline reports whether a status value represents a known-offline
// condition that should emit a reliability finding.
func isPDMOffline(status string) bool {
	switch status {
	case "offline", "stopped", "failed":
		return true
	}
	return false
}

// pdmResourceKey is the in-memory map key for a PDM resource.
func pdmResourceKey(r pdmResource) string {
	return r.RemoteID + "/" + r.Type + "/" + r.Name
}

// pdmAlertDedupKey is the dedup key written onto the Finding (and used by
// the resolve sentinel). Format:
//
//	pdm:resource:offline:<remote>/<type>/<name>
func pdmAlertDedupKey(r pdmResource) string {
	return PDMAlertFindingPrefix + ":" + pdmResourceKey(r)
}

// findingResourceType maps a PDM resource type onto the Pulse finding
// resource_type vocabulary already rendered by FindingsPanel.
func findingResourceType(pdmType string) string {
	switch pdmType {
	case "qemu", "lxc":
		return "vm"
	case "node":
		return "node"
	case "storage":
		return "storage"
	}
	return pdmType
}

// buildPDMAlertFinding constructs a reliability finding for a PDM resource
// that has transitioned into a known-offline state.
func buildPDMAlertFinding(r pdmResource, now time.Time) *Finding {
	dedup := pdmAlertDedupKey(r)
	resType := findingResourceType(r.Type)
	title := fmt.Sprintf("PDM: %s %q is %s", resType, r.Name, r.Status)
	desc := fmt.Sprintf(
		"Proxmox Datacenter Manager reports %s %q in remote %q is %s.",
		resType, r.Name, r.RemoteID, r.Status,
	)
	evidence := fmt.Sprintf("status=%s remote=%s", r.Status, r.RemoteID)

	return &Finding{
		ID:           dedup,
		Key:          dedup,
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryReliability,
		ResourceID:   pdmResourceKey(r),
		ResourceName: r.Name,
		ResourceType: resType,
		Node:         r.RemoteID,
		Title:        title,
		Description:  desc,
		Evidence:     evidence,
		Source:       pdmAlertSourceLabel,
		DetectedAt:   now,
		LastSeenAt:   now,
	}
}
