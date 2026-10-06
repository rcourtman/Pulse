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
	if host == "container" { // DockerResourceID's explicit hostless form.
		sourceID = container
	}
	return sourceID, SourceSpecificID(ResourceTypeAppContainer, SourceDocker, sourceID), true
}

// isDockerHistoryReference marks Docker alert source references, which only a
// complete container ID binds (legacyDockerHistoryIdentity). A container name
// or shortened ID must never join history through general reference matching.
func isDockerHistoryReference(ref string) bool {
	return strings.HasPrefix(strings.TrimSpace(ref), "docker:")
}

// resolveHistoryReference resolves a non-Docker alert reference to the
// resource whose history it joins. Only durable identities qualify: the
// canonical ID, a retired era, a source ID (Proxmox node, guest and storage
// IDs), a node-scoped Proxmox guest reference, or an agent alert reference
// ("agent:<host ID>"). Names and hostnames never bind history, so a resource
// named like a system reference cannot capture its events. claimed reports
// that inventory answers to the reference, possibly ambiguously; an ambiguous
// reference resolves to nothing and must not follow a retained binding.
func (rr *ResourceRegistry) resolveHistoryReference(ref string) (resourceID string, claimed bool) {
	ref = CanonicalResourceID(ref)
	if rr == nil || ref == "" || isDockerHistoryReference(ref) {
		return "", false
	}
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	if rr.resources[ref] != nil {
		return ref, true
	}
	if id := rr.supersededResourceIDLocked(ref); rr.resources[id] != nil {
		return id, true
	}
	matches := make(map[string]struct{}, 1)
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
	if len(matches) != 1 {
		return "", len(matches) > 1
	}
	for id := range matches {
		resourceID = id
	}
	return resourceID, true
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
// Docker references are excluded: the store migration binds full container
// IDs, and names or shortened IDs must never bind.
func (s *SQLiteResourceStore) unboundHistoryReferences() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT canonical_id FROM resource_changes
		WHERE kind GLOB 'alert_*' AND canonical_id NOT GLOB 'docker:*'
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
		refs = append(refs, ref)
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
