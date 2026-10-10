package unifiedresources

import (
	"fmt"
	"time"
)

// CanonicalIDSuccession records that the resource durably pinned under
// OldCanonicalID now mints NewCanonicalID. Canonical IDs hash the strongest
// identity key known at mint time, so a resource's ID moves eras when a
// stronger key appears (a containerized agent gains /etc/machine-id), or when
// the derivation itself is corrected (cluster-scoped IDs switched from short
// to full hostnames; Proxmox guests switched from node-scoped source IDs to
// instance+VMID). Succession re-keys the operator-owned rows, meaning
// operator state (never-auto-remediate, maintenance windows, criticality,
// notes), action-audit history, and manual link/exclusion pairs, onto the new
// canonical ID and drops the superseded identity pin row. Change-journal rows
// are intentionally left untouched: history is never rewritten. Reads merge
// the eras instead, through ResourceIdentityPin.EraIDs for pinned hosts and
// through the durable canonical_id_successions record for everything else.
type CanonicalIDSuccession struct {
	OldCanonicalID string
	NewCanonicalID string
}

// canonicalIDSuccessor is an optional store capability, mirroring the
// maintenance-lifecycle pattern in resource_operator_state.go. The durable
// registry applies successions during PersistIdentityPins when it detects
// that a new pin supersedes an earlier era's pin for the same physical host,
// and during snapshot/record ingest for record-declared eras.
type canonicalIDSuccessor interface {
	ApplyCanonicalIDSuccessions(successions []CanonicalIDSuccession) error
}

// ApplyCanonicalIDSuccessions re-keys operator-owned rows from each
// superseded canonical ID to its successor and deletes the superseded
// identity pin row, in one transaction. Each (old, new) pair is applied once
// and durably recorded in canonical_id_successions; the record both memoizes
// re-runs (steady-state rebuilds re-declare the same retired eras every tick)
// and lets change-journal reads merge the old era without rewriting history.
// Re-keys never clobber rows already present under the new ID (those are
// fresher); a shadowed old row is left behind, which matches the
// pre-succession orphaning behavior. The one thing a shadowed row still
// contributes is its remediation lock: the operator's "never touch this" is
// carried onto the surviving row (CarryRemediationLock) rather than lost to
// whichever row happened to be keyed first. The carry happens with the
// succession's first application, so an operator who later lifts the lock is
// not overruled by the shadowed row on a re-declared succession.
func (s *SQLiteResourceStore) ApplyCanonicalIDSuccessions(successions []CanonicalIDSuccession) error {
	if len(successions) == 0 {
		return nil
	}

	recorded := s.successionMap()
	pending := make([]CanonicalIDSuccession, 0, len(successions))
	for _, succession := range successions {
		oldID := CanonicalResourceID(succession.OldCanonicalID)
		newID := CanonicalResourceID(succession.NewCanonicalID)
		if oldID == "" || newID == "" || oldID == newID {
			continue
		}
		if successor, done := recorded[oldID]; done && successor == newID {
			continue
		}
		pending = append(pending, CanonicalIDSuccession{OldCanonicalID: oldID, NewCanonicalID: newID})
	}
	if len(pending) == 0 {
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

	applied := make([]CanonicalIDSuccession, 0, len(pending))
	var carried []carriedLock
	var groups []savedGroup
	var rekeys []rekeyedID
	for _, succession := range pending {
		oldID := succession.OldCanonicalID
		newID := succession.NewCanonicalID
		result, err := tx.Exec(
			`INSERT OR IGNORE INTO canonical_id_successions (old_canonical_id, new_canonical_id) VALUES (?, ?)`,
			oldID, newID,
		)
		if err != nil {
			return fmt.Errorf("record canonical ID succession %q -> %q: %w", oldID, newID, err)
		}
		if inserted, err := result.RowsAffected(); err == nil && inserted == 0 {
			// Recorded by an earlier boot; the re-key already ran.
			applied = append(applied, succession)
			continue
		}
		carriedAt := time.Now().UTC()
		// The re-key below moves the predecessor's links onto the successor,
		// where a newer exclusion naming either ID can make one read as
		// unlinked. The IDs those links joined stay one group for the sharing
		// after the re-key.
		members, err := linkedMembersSQL(tx, []string{oldID, newID})
		if err != nil {
			return fmt.Errorf("read links of %q and %q: %w", oldID, newID, err)
		}
		// A successor row that already exists wins the re-key below, so the
		// predecessor's remediation lock would be shadowed by it. Carry the
		// block onto it first; with no successor row the re-key moves the
		// predecessor's whole row and there is nothing to carry.
		shadowed, err := carryRemediationLockSQL(tx, newID, oldID, carriedAt, false)
		if err != nil {
			return fmt.Errorf("carry remediation lock %q -> %q: %w", oldID, newID, err)
		}
		if shadowed {
			carried = append(carried, carriedLock{to: newID, from: oldID})
		}
		for _, stmt := range []string{
			`UPDATE OR IGNORE resource_operator_state SET canonical_id = ? WHERE canonical_id = ?`,
			`UPDATE action_audits SET canonical_id = ? WHERE canonical_id = ?`,
			`UPDATE OR IGNORE resource_links SET resource_a = ? WHERE resource_a = ?`,
			`UPDATE OR IGNORE resource_links SET resource_b = ? WHERE resource_b = ?`,
			`UPDATE resource_links SET primary_id = ? WHERE primary_id = ?`,
			`UPDATE OR IGNORE resource_exclusions SET resource_a = ? WHERE resource_a = ?`,
			`UPDATE OR IGNORE resource_exclusions SET resource_b = ? WHERE resource_b = ?`,
		} {
			if _, err := tx.Exec(stmt, newID, oldID); err != nil {
				return fmt.Errorf("re-key canonical rows %q -> %q: %w", oldID, newID, err)
			}
		}
		// The successor's era record keeps the old era's journal rows
		// readable; the superseded pin row itself is stale bucket noise.
		if _, err := tx.Exec(`DELETE FROM resource_identities WHERE canonical_id = ?`, oldID); err != nil {
			return fmt.Errorf("delete superseded identity pin %q: %w", oldID, err)
		}
		groups = append(groups, savedGroup{index: len(rekeys), members: groupAfterRekey(members, oldID, newID)})
		rekeys = append(rekeys, rekeyedID{oldID: oldID, newID: newID})
		applied = append(applied, succession)
	}

	// Whether a successor took its predecessor's whole row or kept its own, it
	// may be linked into another resource the registry keeps instead; the lock
	// has to reach whichever member survives. Sharing waits for the whole batch
	// so a member that had no row is given the block alone only once every
	// predecessor's whole row has had its move: a row made earlier would win
	// the re-key of a later predecessor into the same successor.
	sharedAt := time.Now().UTC()
	for _, group := range groups {
		shared, err := shareRemediationLockAmongSQL(tx, settledGroup(group, rekeys), sharedAt)
		if err != nil {
			return fmt.Errorf("share remediation lock across linked resources %v: %w", group, err)
		}
		carried = append(carried, shared...)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit canonical ID succession: %w", err)
	}
	committed = true
	logCarriedLocks("a canonical ID succession", carried)

	s.identityPinMu.Lock()
	s.identityPinFresh = false
	s.identityPinMu.Unlock()

	s.successionMu.Lock()
	if s.successionFresh {
		next := make(map[string]string, len(s.successionCache)+len(applied))
		for oldID, newID := range s.successionCache {
			next[oldID] = newID
		}
		for _, succession := range applied {
			if _, exists := next[succession.OldCanonicalID]; !exists {
				next[succession.OldCanonicalID] = succession.NewCanonicalID
			}
		}
		s.successionCache = next
	}
	s.successionMu.Unlock()
	return nil
}

// successionMap returns the cached old→successor canonical ID record. The
// returned map is shared and must be treated as read-only.
func (s *SQLiteResourceStore) successionMap() map[string]string {
	s.successionMu.Lock()
	defer s.successionMu.Unlock()
	if !s.successionFresh {
		successors := make(map[string]string)
		rows, err := s.db.Query(`SELECT old_canonical_id, new_canonical_id FROM canonical_id_successions`)
		if err == nil {
			for rows.Next() {
				var oldID, newID string
				if err := rows.Scan(&oldID, &newID); err != nil {
					continue
				}
				oldID = CanonicalResourceID(oldID)
				newID = CanonicalResourceID(newID)
				if oldID == "" || newID == "" || oldID == newID {
					continue
				}
				successors[oldID] = newID
			}
			_ = rows.Close()
			if err := rows.Err(); err == nil {
				s.successionCache = successors
				s.successionFresh = true
			}
		}
		if !s.successionFresh {
			// Query failed; serve what we have without caching so the next
			// call retries.
			return successors
		}
	}
	return s.successionCache
}

// ApplyCanonicalIDSuccessions mirrors the SQLite semantics for the in-memory
// test store. Action audits are not re-keyed here: MemoryStore filters them
// by the audit record's embedded request resource ID rather than a separate
// index column, and audit artifacts are never mutated.
func (m *MemoryStore) ApplyCanonicalIDSuccessions(successions []CanonicalIDSuccession) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var groups []savedGroup
	var rekeys []rekeyedID
	for _, succession := range successions {
		oldID := CanonicalResourceID(succession.OldCanonicalID)
		newID := CanonicalResourceID(succession.NewCanonicalID)
		if oldID == "" || newID == "" || oldID == newID {
			continue
		}
		// A succession applies once per predecessor, as SQLite's recorded
		// successions do: a repeat neither re-keys nor re-carries.
		if _, recorded := m.canonicalSuccessions[oldID]; recorded {
			continue
		}
		carriedAt := time.Now().UTC()
		// See the SQLite store: the IDs the links joined before the re-key stay
		// one group for the sharing after it.
		members := flattenComponents(linkComponents(m.livePairsLocked(), []string{oldID, newID}))
		if state, ok := m.resourceOperatorState[oldID]; ok {
			if _, taken := m.resourceOperatorState[newID]; !taken {
				state.CanonicalID = newID
				m.resourceOperatorState[newID] = state
				delete(m.resourceOperatorState, oldID)
			} else {
				m.carryRemediationLockLocked(newID, oldID, carriedAt, false)
			}
		}
		for i := range m.links {
			if m.links[i].ResourceA == oldID {
				m.links[i].ResourceA = newID
			}
			if m.links[i].ResourceB == oldID {
				m.links[i].ResourceB = newID
			}
			if m.links[i].PrimaryID == oldID {
				m.links[i].PrimaryID = newID
			}
		}
		for i := range m.exclusions {
			if m.exclusions[i].ResourceA == oldID {
				m.exclusions[i].ResourceA = newID
			}
			if m.exclusions[i].ResourceB == oldID {
				m.exclusions[i].ResourceB = newID
			}
		}
		delete(m.identityPins, oldID)
		if m.canonicalSuccessions == nil {
			m.canonicalSuccessions = make(map[string]string)
		}
		m.canonicalSuccessions[oldID] = newID
		groups = append(groups, savedGroup{index: len(rekeys), members: groupAfterRekey(members, oldID, newID)})
		rekeys = append(rekeys, rekeyedID{oldID: oldID, newID: newID})
	}
	// See the SQLite store: sharing waits for the whole batch.
	sharedAt := time.Now().UTC()
	for _, group := range groups {
		m.shareRemediationLockAmongLocked(settledGroup(group, rekeys), sharedAt)
	}
	return nil
}
