package unifiedresources

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
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
