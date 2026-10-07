package unifiedresources

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// legacyDockerHistoryIdentity accepts the source identifier emitted by Docker
// alerts only when it contains a complete container ID. Names and short IDs
// cannot establish durable identity after inventory removal.
func legacyDockerHistoryIdentity(ref string) (sourceID, canonicalID string, ok bool) {
	ref = strings.TrimSpace(ref)
	if !strings.HasPrefix(ref, "docker:") {
		return "", "", false
	}
	host, container, found := strings.Cut(strings.TrimPrefix(ref, "docker:"), "/")
	if !found || host == "" || strings.TrimSpace(host) != host || len(container) != 64 || strings.ToLower(container) != container {
		return "", "", false
	}
	if _, err := hex.DecodeString(container); err != nil {
		return "", "", false
	}
	sourceID = host + "/container/" + container
	if host == "container" { // DockerContainerResourceID's explicit hostless form.
		sourceID = container
	}
	return sourceID, SourceSpecificID(ResourceTypeAppContainer, SourceDocker, sourceID), true
}

// isDockerHistoryReference marks Docker alert source references, including the
// hostless "docker-service:<name>" form. They never join history through
// general reference matching: only a Docker host, a Swarm service ID
// (dockerHostHistoryReference) or a complete container ID
// (legacyDockerHistoryIdentity) binds.
func isDockerHistoryReference(ref string) bool {
	ref = strings.TrimSpace(ref)
	return strings.HasPrefix(ref, "docker:") || strings.HasPrefix(ref, "docker-service:")
}

// isDockerNameHistoryReference marks Docker alert references that are not host
// or service ID references: container references, which only a complete
// container ID binds, the names of containers and services reported without an
// ID, and hostless service names. A name or shortened ID must never join
// history, not even through a retained binding.
func isDockerNameHistoryReference(ref string) bool {
	_, _, ok := dockerHostHistoryReference(ref)
	return isDockerHistoryReference(ref) && !ok
}

// dockerHostHistoryReference parses the Docker host alert reference
// ("docker:<host ID>", alerts.DockerHostResourceID) and the Swarm service
// alert reference ("docker:<host ID>/service/<service ID>",
// alerts.DockerServiceResourceID). serviceID is empty for a host reference.
// A service reported without an ID alerts under "name:<name>" in the ID
// position, and Swarm IDs never contain a colon, so that is a name, not a
// service reference. "docker:unknown" was the old builder's fallback for
// alerts without a host.
func dockerHostHistoryReference(ref string) (hostID, serviceID string, ok bool) {
	rest, found := strings.CutPrefix(strings.TrimSpace(ref), "docker:")
	if !found {
		return "", "", false
	}
	hostID, serviceID, isService := strings.Cut(rest, "/service/")
	if hostID == "" || hostID == "unknown" || strings.TrimSpace(hostID) != hostID || strings.Contains(hostID, "/") {
		return "", "", false
	}
	if isService && (serviceID == "" || strings.TrimSpace(serviceID) != serviceID || strings.ContainsAny(serviceID, "/:")) {
		return "", "", false
	}
	return hostID, serviceID, true
}

// dockerHistoryOwnerLocked resolves a Docker host or Swarm service alert
// reference through the registry's own Docker source identities. The host ID
// is the Docker host's source ID. A service reference names the service with
// that exact ID in the host's Swarm cluster, which every manager of the
// cluster reports under its own host ID.
func (rr *ResourceRegistry) dockerHistoryOwnerLocked(ref string) string {
	hostID, serviceID, ok := dockerHostHistoryReference(ref)
	if !ok {
		return ""
	}
	hostResourceID := rr.bySource[SourceDocker][hostID]
	host := rr.resources[hostResourceID]
	if host == nil || CanonicalResourceType(host.Type) != ResourceTypeAgent {
		return ""
	}
	if serviceID == "" {
		return hostResourceID
	}
	if host.Docker == nil {
		return ""
	}
	cluster := dockerSwarmClusterKeyFromMeta(host.Docker.Swarm)
	if cluster == "" {
		return ""
	}
	// The registry keys a service without an ID by its raw name, so the entry
	// must carry exactly this ID.
	serviceResourceID := rr.bySource[SourceDocker][normalizeSourceID(cluster+":service:"+serviceID)]
	service := rr.resources[serviceResourceID]
	if service == nil || CanonicalResourceType(service.Type) != ResourceTypeDockerService ||
		service.Docker == nil || strings.TrimSpace(service.Docker.ServiceID) != serviceID {
		return ""
	}
	return serviceResourceID
}

// historySubResourceOwner names the owner of a sub-resource alert reference,
// one that names no resource of its own, and the type that owner must have.
// Only these producer shapes qualify, each carrying its owner's durable
// identity:
//   - "<storage ID>/zfs-pool:<pool>[/device:<device>]" (ZFS pool state,
//     errors and device health) belongs to the storage.
//   - "agent:<host ID>/storage:<array>" (Unraid array) is the agent source ID
//     "<host ID>/storage:<array>" of the host's array storage.
//   - "agent:<host ID>/disk:<mount or device>", ".../disk_temp:<device>",
//     ".../raid:<device>" and ".../custom:<sensor>" belong to the host.
//
// Pool, mount and kernel device labels are names: a device name can belong to
// a different disk after a reboot, so these never bind a physical disk.
func historySubResourceOwner(ref string) (ownerRef string, ownerType ResourceType, ok bool) {
	if rest, found := strings.CutPrefix(ref, "agent:"); found {
		hostID, child, _ := strings.Cut(rest, "/")
		kind, label, _ := strings.Cut(child, ":")
		if hostID == "" || label == "" {
			return "", "", false
		}
		switch kind {
		case "storage":
			return hostID + "/" + child, ResourceTypeStorage, true
		case "disk", "disk_temp", "raid", "custom":
			return "agent:" + hostID, ResourceTypeAgent, true
		}
		return "", "", false
	}
	if i := strings.LastIndex(ref, "/zfs-pool:"); i > 0 {
		pool, device, hasDevice := strings.Cut(ref[i+len("/zfs-pool:"):], "/device:")
		if pool == "" || strings.Contains(pool, "/") || (hasDevice && (device == "" || strings.Contains(device, "/"))) {
			return "", "", false
		}
		return ref[:i], ResourceTypeStorage, true
	}
	return "", "", false
}

// resolveHistoryReference resolves an alert reference to the resource whose
// history it joins. Only durable identities qualify: the canonical ID, a
// retired era, a source ID (Proxmox node, guest and storage IDs), a
// node-scoped Proxmox guest reference, an agent alert reference
// ("agent:<host ID>"), a Docker host or Swarm service ID, or the owner of a
// sub-resource reference (historySubResourceOwner). Names and hostnames never
// bind history, so a resource named like a system reference cannot capture
// its events. claimed reports that inventory answers to the reference,
// possibly ambiguously; an ambiguous or conflicting reference resolves to
// nothing and must not follow a retained binding.
func (rr *ResourceRegistry) resolveHistoryReference(ref string) (resourceID string, claimed bool) {
	ref = CanonicalResourceID(ref)
	if rr == nil || ref == "" {
		return "", false
	}
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	if isDockerHistoryReference(ref) {
		resourceID = rr.dockerHistoryOwnerLocked(ref)
		return resourceID, resourceID != ""
	}
	if isProxmoxPhysicalDiskAlertReference(ref) {
		// A device path is not durable identity (proxmoxDiskAlertOwner).
		return "", false
	}
	matches := rr.durableHistoryMatchesLocked(ref)
	if ownerRef, ownerType, ok := historySubResourceOwner(ref); ok && len(matches) == 0 {
		// The owner must resolve uniquely and have the expected type. An owner
		// reference that names an ambiguous or incompatible resource is a
		// conflict: the event keeps its own reference.
		owners := rr.durableHistoryMatchesLocked(ownerRef)
		for id := range owners {
			if len(owners) != 1 || CanonicalResourceType(rr.resources[id].Type) != ownerType {
				return "", true
			}
			matches[id] = struct{}{}
		}
	}
	if len(matches) != 1 {
		return "", len(matches) > 1
	}
	for id := range matches {
		resourceID = id
	}
	return resourceID, true
}

// durableHistoryMatchesLocked lists the resources that answer to a reference
// by durable identity.
func (rr *ResourceRegistry) durableHistoryMatchesLocked(ref string) map[string]struct{} {
	matches := make(map[string]struct{}, 1)
	if rr.resources[ref] != nil {
		matches[ref] = struct{}{}
		return matches
	}
	if id := rr.supersededResourceIDLocked(ref); rr.resources[id] != nil {
		matches[id] = struct{}{}
		return matches
	}
	for _, mapping := range rr.bySource {
		if id := mapping[ref]; rr.resources[id] != nil {
			matches[id] = struct{}{}
		}
	}
	if hostID, ok := strings.CutPrefix(ref, "agent:"); ok && hostID != "" && !strings.Contains(hostID, "/") {
		if id := rr.bySource[SourceAgent][hostID]; rr.resources[id] != nil {
			matches[id] = struct{}{}
		}
	}
	if instance, _, vmid, ok := ParseProxmoxGuestSourceID(ref); ok {
		for _, resourceType := range []ResourceType{ResourceTypeVM, ResourceTypeSystemContainer} {
			if id := ProxmoxGuestCanonicalID(resourceType, instance, vmid); rr.resources[id] != nil {
				matches[id] = struct{}{}
			}
		}
	}
	return matches
}

// resourceHistoryIdentityWriter binds a source reference to the canonical
// resource of one event. It affects history lookup only, never operator state,
// action requests, approvals or execution identities.
type resourceHistoryIdentityWriter interface {
	RecordChangeWithSourceIdentity(change ResourceChange, sourceID string) error
	ResolveHistorySourceIdentity(sourceID string) (string, bool, error)
}

func (s *SQLiteResourceStore) ResolveHistorySourceIdentity(sourceID string) (string, bool, error) {
	var id string
	err := s.db.QueryRow(`SELECT canonical_id FROM resource_history_aliases WHERE source_id = ?`, sourceID).Scan(&id)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return id, err == nil, err
}

func (m *MemoryStore) ResolveHistorySourceIdentity(sourceID string) (string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.historyAliases[sourceID]
	return id, ok, nil
}

func (s *SQLiteResourceStore) RecordChangeWithSourceIdentity(change ResourceChange, sourceID string) error {
	sourceID = CanonicalResourceID(sourceID)
	canonicalID := CanonicalResourceID(change.ResourceID)
	if sourceID == "" || canonicalID == "" || sourceID == canonicalID {
		return s.RecordChange(change)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin resource history identity: %w", err)
	}
	defer tx.Rollback()
	// Registry resolution is authoritative when available. Rebinding a source
	// reference affects subsequent history reads without rewriting past events.
	if _, err := tx.Exec(`INSERT INTO resource_history_aliases (source_id, canonical_id) VALUES (?, ?)
		ON CONFLICT(source_id) DO UPDATE SET canonical_id = excluded.canonical_id`, sourceID, canonicalID); err != nil {
		return fmt.Errorf("record resource history identity: %w", err)
	}
	if err := recordChangeSQL(tx, change, s.resourceChangesHasTimestamp); err != nil {
		return err
	}
	return tx.Commit()
}

func (m *MemoryStore) RecordChangeWithSourceIdentity(change ResourceChange, sourceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.historyAliases == nil {
		m.historyAliases = make(map[string]string)
	}
	if sourceID = CanonicalResourceID(sourceID); sourceID != "" && sourceID != change.ResourceID {
		m.historyAliases[sourceID] = change.ResourceID
	}
	return m.recordChangeLocked(change)
}

// legacyHistoryBindWindow bounds how long an unresolved alert journal
// reference is retried. Inventory normally names a resource within a few poll
// cycles; a reference still unresolved after the window names a removed
// resource or a sub-resource and keeps its own history.
const legacyHistoryBindWindow = 30 * time.Minute

// legacyHistoryBinder exposes alert journal references that no history binding
// covers. Rows recorded before their reference resolved join canonical history
// through a binding; the rows themselves are never rewritten.
type legacyHistoryBinder interface {
	unboundHistoryReferences() ([]string, error)
	bindHistorySourceIdentities(bindings map[string]string) error
}

// legacyHistoryBackfill holds unbound references with their retry deadlines:
// those already journaled when the process started, and those RecordChange
// wrote before inventory could name their resource.
type legacyHistoryBackfill struct {
	mu      sync.Mutex
	loaded  bool
	pending map[string]time.Time
}

func (b *legacyHistoryBackfill) addLocked(ref string, now time.Time) {
	if b.pending == nil {
		b.pending = make(map[string]time.Time)
	}
	if _, ok := b.pending[ref]; !ok {
		b.pending[ref] = now.Add(legacyHistoryBindWindow)
	}
}

func (b *legacyHistoryBackfill) add(ref string, now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.addLocked(ref, now)
}

// unboundHistoryReferences lists alert journal references without a binding.
// Docker container references and hostless service names are excluded: the
// store migration binds full container IDs, and names or shortened IDs must
// never bind. PVE disk alert references never bind either; their rows are
// owned row by row (proxmoxDiskAlertOwner).
func (s *SQLiteResourceStore) unboundHistoryReferences() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT canonical_id FROM resource_changes
		WHERE kind GLOB 'alert_*'
		AND canonical_id NOT IN (SELECT source_id FROM resource_history_aliases)
		ORDER BY canonical_id`)
	if err != nil {
		return nil, fmt.Errorf("read unbound resource history references: %w", err)
	}
	defer rows.Close()
	var refs []string
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			return nil, err
		}
		if !isDockerNameHistoryReference(ref) && !isProxmoxPhysicalDiskAlertReference(ref) {
			refs = append(refs, ref)
		}
	}
	return refs, rows.Err()
}

func (s *SQLiteResourceStore) bindHistorySourceIdentities(bindings map[string]string) error {
	if len(bindings) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin resource history identities: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for sourceID, canonicalID := range bindings {
		if _, err := tx.Exec(`INSERT INTO resource_history_aliases (source_id, canonical_id) VALUES (?, ?)
			ON CONFLICT(source_id) DO UPDATE SET canonical_id = excluded.canonical_id`, sourceID, canonicalID); err != nil {
			return fmt.Errorf("bind resource history identity: %w", err)
		}
	}
	return tx.Commit()
}

// bindLegacyHistory binds the pending references a published registry
// generation resolves, with the same rules as RecordChange. The journal is
// scanned once per process; later references arrive from RecordChange.
func (a *MonitorAdapter) bindLegacyHistory(registry *ResourceRegistry, now time.Time) {
	binder, ok := registry.store.(legacyHistoryBinder)
	if !ok {
		return
	}
	backfill := &a.legacyHistory
	backfill.mu.Lock()
	defer backfill.mu.Unlock()
	if !backfill.loaded {
		refs, err := binder.unboundHistoryReferences()
		if err != nil {
			log.Printf("unifiedresources: %v", err)
			return
		}
		backfill.loaded = true
		for _, ref := range refs {
			backfill.addLocked(ref, now)
		}
	}
	if len(backfill.pending) == 0 {
		return
	}
	bindings := make(map[string]string)
	var settled []string
	for ref, deadline := range backfill.pending {
		if resolvedID, _ := registry.resolveHistoryReference(ref); resolvedID != "" {
			if resolvedID != ref {
				bindings[ref] = resolvedID
			}
			settled = append(settled, ref)
		} else if !now.Before(deadline) {
			delete(backfill.pending, ref)
		}
	}
	if err := binder.bindHistorySourceIdentities(bindings); err != nil {
		log.Printf("unifiedresources: %v", err)
		return
	}
	for _, ref := range settled {
		delete(backfill.pending, ref)
	}
	if len(bindings) > 0 {
		log.Printf("unifiedresources: bound %d alert history references to canonical resources", len(bindings))
	}
}

// migrateResourceHistoryAliases adds an index for exact legacy Docker event
// identities, including resources already removed from live inventory. The
// event rows and all authority-bearing tables remain unchanged.
func (s *SQLiteResourceStore) migrateResourceHistoryAliases() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS resource_history_aliases (
		source_id TEXT PRIMARY KEY, canonical_id TEXT NOT NULL);
		CREATE INDEX IF NOT EXISTS idx_resource_history_aliases_canonical ON resource_history_aliases(canonical_id);`); err != nil {
		return fmt.Errorf("initialize resource history identities: %w", err)
	}
	rows, err := s.db.Query(`SELECT DISTINCT canonical_id FROM resource_changes WHERE canonical_id GLOB 'docker:*'`)
	if err != nil {
		return fmt.Errorf("read legacy resource history identities: %w", err)
	}
	aliases := make(map[string]string)
	for rows.Next() {
		var sourceID string
		if err := rows.Scan(&sourceID); err != nil {
			rows.Close()
			return err
		}
		if _, canonicalID, ok := legacyDockerHistoryIdentity(sourceID); ok {
			aliases[sourceID] = canonicalID
		}
	}
	readErr := rows.Err()
	rows.Close() // The store has one connection. Release it before writing.
	if readErr != nil {
		return readErr
	}
	for sourceID, canonicalID := range aliases {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO resource_history_aliases (source_id, canonical_id) VALUES (?, ?)`, sourceID, canonicalID); err != nil {
			return fmt.Errorf("index legacy resource history identity: %w", err)
		}
	}
	return nil
}

// expandHistoryAliases reads persisted identity bindings each time so separate
// monitor, API and Assistant store handles see new bindings immediately. The
// indexed traversal is restricted to the requested identities, not the fleet.
func (s *SQLiteResourceStore) expandHistoryAliases(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return ids, nil
	}
	seeds := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		seeds[i], args[i] = "(?)", id
	}
	rows, err := s.db.Query(`WITH RECURSIVE history_ids(id) AS (
		VALUES `+strings.Join(seeds, ",")+`
		UNION SELECT a.canonical_id FROM resource_history_aliases a JOIN history_ids h ON a.source_id = h.id
		UNION SELECT a.source_id FROM resource_history_aliases a JOIN history_ids h ON a.canonical_id = h.id
		UNION SELECT s.old_canonical_id FROM canonical_id_successions s JOIN history_ids h ON s.new_canonical_id = h.id
	) SELECT id FROM history_ids`, args...)
	if err != nil {
		return nil, fmt.Errorf("read resource history identities: %w", err)
	}
	defer rows.Close()
	var expanded []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		expanded = append(expanded, id)
	}
	return expanded, rows.Err()
}
