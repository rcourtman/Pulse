package unifiedresources

import "fmt"

// CanonicalIDSuccession records that the physical host durably pinned under
// OldCanonicalID now mints NewCanonicalID. Canonical IDs hash the strongest
// identity key known at mint time, so a host's ID moves eras when a stronger
// key appears (a containerized agent gains /etc/machine-id) or when the
// derivation itself is corrected (cluster-scoped IDs switched from short to
// full hostnames so FQDN Swarm members stop collapsing). Succession re-keys
// the operator-owned rows, meaning operator state (never-auto-remediate,
// maintenance windows, criticality, notes) and action-audit history, onto
// the new canonical ID and drops the superseded identity pin row.
// Change-journal rows are intentionally left untouched: history is never
// rewritten, and ResourceIdentityPin.EraIDs merges old-era journal rows into
// reads keyed by the new ID.
type CanonicalIDSuccession struct {
	OldCanonicalID string
	NewCanonicalID string
}

// canonicalIDSuccessor is an optional store capability, mirroring the
// maintenance-lifecycle pattern in resource_operator_state.go. The durable
// registry applies successions during PersistIdentityPins when it detects
// that a new pin supersedes an earlier era's pin for the same physical host.
type canonicalIDSuccessor interface {
	ApplyCanonicalIDSuccessions(successions []CanonicalIDSuccession) error
}

// ApplyCanonicalIDSuccessions re-keys operator-owned rows from each
// superseded canonical ID to its successor and deletes the superseded
// identity pin row, in one transaction. Re-keys never clobber rows already
// present under the new ID (those are fresher); a shadowed old row is left
// behind, which matches the pre-succession orphaning behavior.
func (s *SQLiteResourceStore) ApplyCanonicalIDSuccessions(successions []CanonicalIDSuccession) error {
	if len(successions) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin canonical ID succession: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	for _, succession := range successions {
		oldID := CanonicalResourceID(succession.OldCanonicalID)
		newID := CanonicalResourceID(succession.NewCanonicalID)
		if oldID == "" || newID == "" || oldID == newID {
			continue
		}
		for _, stmt := range []string{
			`UPDATE OR IGNORE resource_operator_state SET canonical_id = ? WHERE canonical_id = ?`,
			`UPDATE action_audits SET canonical_id = ? WHERE canonical_id = ?`,
		} {
			if _, err := tx.Exec(stmt, newID, oldID); err != nil {
				return fmt.Errorf("re-key canonical rows %q -> %q: %w", oldID, newID, err)
			}
		}
		// The successor pin's EraIDs keep the old era's journal rows
		// readable; the superseded pin row itself is stale bucket noise.
		if _, err := tx.Exec(`DELETE FROM resource_identities WHERE canonical_id = ?`, oldID); err != nil {
			return fmt.Errorf("delete superseded identity pin %q: %w", oldID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit canonical ID succession: %w", err)
	}
	committed = true

	s.identityPinMu.Lock()
	s.identityPinFresh = false
	s.identityPinMu.Unlock()
	return nil
}

// ApplyCanonicalIDSuccessions mirrors the SQLite semantics for the in-memory
// test store. Action audits are not re-keyed here: MemoryStore filters them
// by the audit record's embedded request resource ID rather than a separate
// index column, and audit artifacts are never mutated.
func (m *MemoryStore) ApplyCanonicalIDSuccessions(successions []CanonicalIDSuccession) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, succession := range successions {
		oldID := CanonicalResourceID(succession.OldCanonicalID)
		newID := CanonicalResourceID(succession.NewCanonicalID)
		if oldID == "" || newID == "" || oldID == newID {
			continue
		}
		if state, ok := m.resourceOperatorState[oldID]; ok {
			if _, taken := m.resourceOperatorState[newID]; !taken {
				state.CanonicalID = newID
				m.resourceOperatorState[newID] = state
				delete(m.resourceOperatorState, oldID)
			}
		}
		delete(m.identityPins, oldID)
	}
	return nil
}
