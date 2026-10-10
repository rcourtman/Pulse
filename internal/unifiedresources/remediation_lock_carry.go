package unifiedresources

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// An operator's remediation lock (ResourceOperatorState.BlocksRemediation) is
// stored on the row of one canonical ID, and every gate reads the row of the ID
// a resource currently has. Identity changes move resources between IDs, so the
// stores carry the block with them: a live manual link makes its members one
// resource, and the members of a link component share the block whenever the
// component changes (a link, an unlink) and whenever a succession lands on one
// of them. CarryRemediationLock owns what is written onto a row.

// A rebuild applies a succession to the store before it publishes the registry
// generation that lists the successor, and the previous generation, which still
// lists the retired ID, keeps serving in between. The gates read the operator
// state of the ID they are asked about, and the succession moved that ID's row,
// so a retired ID that answers "no row" lets Pulse act on a resource the
// operator locked. A read of an ID that holds no row therefore follows the
// recorded successions (canonical_id_successions) to the end of its chain and
// answers as the ID the chain ends on would: with the remediation block that
// ID's row carries, the block alone as a link fold carries it, attributed to the
// identity change and dated by the row it came from so two reads agree (a
// plan's policy revision hashes the state). The end of the chain decides, so a
// lock the operator lifted on the successor, by editing or clearing its row, is
// not read back from a stale row left behind on an ID before it; an ID that
// holds a row of its own answers with it; and a chain that returns to an ID it
// has seen has no end, so a lock held anywhere on it counts.
func followRetiredID(
	retiredID string,
	successorOf func(id string) (successor string, retired bool, err error),
	rowOf func(id string) (ResourceOperatorState, bool, error),
) (ResourceOperatorState, bool, error) {
	seen := map[string]struct{}{retiredID: {}}
	var end, nearestLock ResourceOperatorState
	var endFound, lockFound bool
	for current := retiredID; ; {
		successor, retired, err := successorOf(current)
		if err != nil {
			return ResourceOperatorState{}, false, err
		}
		successor = CanonicalResourceID(successor)
		if !retired || successor == "" {
			if endFound {
				lock, carried := retiredIDRemediationLock(retiredID, end)
				return lock, carried, nil
			}
			return ResourceOperatorState{}, false, nil
		}
		if _, again := seen[successor]; again {
			if lockFound {
				lock, carried := retiredIDRemediationLock(retiredID, nearestLock)
				return lock, carried, nil
			}
			return ResourceOperatorState{}, false, nil
		}
		seen[successor] = struct{}{}
		row, found, err := rowOf(successor)
		if err != nil {
			return ResourceOperatorState{}, false, err
		}
		end, endFound = row, found
		if found && !lockFound && row.BlocksRemediation() {
			nearestLock, lockFound = row, true
		}
		current = successor
	}
}

// retiredIDRemediationLock is what a retired ID reads of the row an ID on its
// chain holds: that row's remediation block, if it has one, and nothing else.
func retiredIDRemediationLock(retiredID string, holder ResourceOperatorState) (ResourceOperatorState, bool) {
	return CarryRemediationLock(retiredID, ResourceOperatorState{}, false, holder, true, holder.SetAt)
}

// retiredIDRemediationLockSQL is the read above for the SQLite store: one
// statement per hop, on the connection the succession's transaction commits on,
// so a read that finds the row gone from an ID also finds the succession that
// moved it.
func retiredIDRemediationLockSQL(queryer resourceOperatorStateQueryRower, canonicalID string) (ResourceOperatorState, bool, error) {
	retiredID := CanonicalResourceID(canonicalID)
	if retiredID == "" {
		return ResourceOperatorState{}, false, nil
	}
	return followRetiredID(retiredID,
		func(id string) (string, bool, error) {
			var successor string
			err := queryer.QueryRow(`SELECT new_canonical_id FROM canonical_id_successions WHERE old_canonical_id = ?`, id).Scan(&successor)
			if errors.Is(err, sql.ErrNoRows) {
				return "", false, nil
			}
			if err != nil {
				return "", false, fmt.Errorf("query canonical ID succession of %q: %w", id, err)
			}
			return successor, true, nil
		},
		func(id string) (ResourceOperatorState, bool, error) { return getResourceOperatorStateSQL(queryer, id) },
	)
}

// carriedLock records one remediation block written onto `to` from `from`.
type carriedLock struct {
	to   string
	from string
}

func logCarriedLocks(cause string, carried []carriedLock) {
	for _, lock := range carried {
		log.Printf("unifiedresources: carried the remediation lock of %q onto %q (%s)", lock.from, lock.to, cause)
	}
}

// rowsQueryer is the read side shared by *sql.DB and *sql.Tx.
type rowsQueryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

// liveLinkPairsSQL returns the ID pairs of the links that are their pair's
// latest operator decision, read through q so a caller inside a transaction
// sees its own writes. It reads only the columns that decide a pair (IDs and
// time), which the schema never lets be NULL except the time, so a row with an
// empty reason or author cannot stop a succession or the upgrade carry.
func liveLinkPairsSQL(q rowsQueryer) ([][2]string, error) {
	links, err := queryPairDecisions(q, `SELECT resource_a, resource_b, created_at FROM resource_links`)
	if err != nil || len(links) == 0 {
		return nil, err
	}
	decided, err := queryPairDecisions(q, `SELECT resource_a, resource_b, created_at FROM resource_exclusions`)
	if err != nil {
		return nil, err
	}
	exclusions := make([]ResourceExclusion, 0, len(decided))
	for _, decision := range decided {
		exclusions = append(exclusions, ResourceExclusion{ResourceA: decision.ResourceA, ResourceB: decision.ResourceB, CreatedAt: decision.CreatedAt})
	}
	live, _ := effectiveManualPairDecisions(links, exclusions)
	return linkPairs(live), nil
}

// decisionTime reads a stored created_at leniently: a time the driver cannot
// parse (NULL, an integer, free text) counts as the zero time, which only
// affects whether a newer decision supersedes it. One malformed row must not
// stop an unrelated succession, link or unlink from settling a lock.
func decisionTime(raw any) time.Time {
	switch value := raw.(type) {
	case time.Time:
		return value
	case string:
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05"} {
			if parsed, err := time.Parse(layout, value); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}

// queryPairDecisions reads (resource_a, resource_b, created_at) rows as links;
// only the pair and its time are meaningful.
func queryPairDecisions(q rowsQueryer, query string) (decisions []ResourceLink, err error) {
	rows, err := q.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query manual pair decisions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close manual pair decisions: %w", closeErr)
		}
	}()
	for rows.Next() {
		var (
			a, b string
			at   any
		)
		if scanErr := rows.Scan(&a, &b, &at); scanErr != nil {
			return nil, fmt.Errorf("scan manual pair decision: %w", scanErr)
		}
		decisions = append(decisions, ResourceLink{ResourceA: CanonicalResourceID(a), ResourceB: CanonicalResourceID(b), CreatedAt: decisionTime(at)})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate manual pair decisions: %w", rowsErr)
	}
	return decisions, nil
}

func linkPairs(links []ResourceLink) [][2]string {
	pairs := make([][2]string, 0, len(links))
	for _, link := range links {
		pairs = append(pairs, [2]string{link.ResourceA, link.ResourceB})
	}
	return pairs
}

// linkComponents groups the IDs reachable from seeds through pairs, one sorted
// slice per connected component that holds at least one seed. A pair names a
// link the registry treats as one resource, in either direction, so a chain of
// links is one component.
func linkComponents(pairs [][2]string, seeds []string) [][]string {
	adjacent := make(map[string][]string, len(pairs)*2)
	for _, pair := range pairs {
		a, b := strings.TrimSpace(pair[0]), strings.TrimSpace(pair[1])
		if a == "" || b == "" || a == b {
			continue
		}
		adjacent[a] = append(adjacent[a], b)
		adjacent[b] = append(adjacent[b], a)
	}
	seen := make(map[string]struct{}, len(seeds))
	var components [][]string
	for _, seed := range seeds {
		seed = strings.TrimSpace(seed)
		if seed == "" {
			continue
		}
		if _, done := seen[seed]; done {
			continue
		}
		seen[seed] = struct{}{}
		component := []string{seed}
		for queue := []string{seed}; len(queue) > 0; queue = queue[1:] {
			for _, next := range adjacent[queue[0]] {
				if _, done := seen[next]; done {
					continue
				}
				seen[next] = struct{}{}
				component = append(component, next)
				queue = append(queue, next)
			}
		}
		sort.Strings(component)
		components = append(components, component)
	}
	return components
}

// shareRemediationLock returns the rows a component must hold so that every
// member carries the block any one of them holds. states holds the members'
// stored rows; a member with no row is absent from it. The donor is the first
// blocking member in ID order, so the result does not depend on map order.
func shareRemediationLock(component []string, states map[string]ResourceOperatorState, at time.Time) (writes []ResourceOperatorState, carried []carriedLock) {
	donorID := ""
	for _, id := range component {
		if state, found := states[id]; found && state.BlocksRemediation() {
			donorID = id
			break
		}
	}
	if donorID == "" {
		return nil, nil
	}
	for _, id := range component {
		if id == donorID {
			continue
		}
		state, found := states[id]
		if next, changed := CarryRemediationLock(id, state, found, states[donorID], true, at); changed {
			writes = append(writes, next)
			carried = append(carried, carriedLock{to: id, from: donorID})
		}
	}
	return writes, carried
}

// carryRemediationLockSQL writes donor's remediation block onto survivor's
// operator-state row inside the caller's transaction (CarryRemediationLock owns
// what is written). createRow false leaves a survivor with no row alone, for
// a caller whose next statement moves the donor's whole row onto it. Reports
// whether a row was written.
func carryRemediationLockSQL(tx *sql.Tx, survivorID, donorID string, at time.Time, createRow bool) (bool, error) {
	survivorID = strings.TrimSpace(survivorID)
	donorID = strings.TrimSpace(donorID)
	if survivorID == "" || donorID == "" || survivorID == donorID {
		return false, nil
	}
	donor, donorFound, err := getResourceOperatorStateSQL(tx, donorID)
	if err != nil || !donorFound || !donor.BlocksRemediation() {
		return false, err
	}
	survivor, survivorFound, err := getResourceOperatorStateSQL(tx, survivorID)
	if err != nil {
		return false, err
	}
	if !survivorFound && !createRow {
		return false, nil
	}
	next, changed := CarryRemediationLock(survivorID, survivor, survivorFound, donor, donorFound, at)
	if !changed {
		return false, nil
	}
	if err := setResourceOperatorStateSQL(tx, next); err != nil {
		return false, err
	}
	return true, nil
}

// shareRemediationLockAmongSQL makes every ID in group carry the block any of
// them holds, inside the caller's transaction, and reports what it wrote.
func shareRemediationLockAmongSQL(tx *sql.Tx, group []string, at time.Time) ([]carriedLock, error) {
	if len(group) < 2 {
		return nil, nil
	}
	states := make(map[string]ResourceOperatorState, len(group))
	for _, id := range group {
		state, found, err := getResourceOperatorStateSQL(tx, id)
		if err != nil {
			return nil, err
		}
		if found {
			states[id] = state
		}
	}
	writes, carried := shareRemediationLock(group, states, at)
	for _, next := range writes {
		if err := setResourceOperatorStateSQL(tx, next); err != nil {
			return nil, err
		}
	}
	return carried, nil
}

// shareRemediationLockAcrossLinksSQL makes every member of each live link
// component holding a seed carry the block any member holds, inside the
// caller's transaction. A link written in the same transaction counts as live.
func shareRemediationLockAcrossLinksSQL(tx *sql.Tx, seeds []string, at time.Time) ([]carriedLock, error) {
	pairs, err := liveLinkPairsSQL(tx)
	if err != nil {
		return nil, err
	}
	var carried []carriedLock
	for _, component := range linkComponents(pairs, seeds) {
		shared, err := shareRemediationLockAmongSQL(tx, component, at)
		if err != nil {
			return nil, err
		}
		carried = append(carried, shared...)
	}
	return carried, nil
}

// linkedMembersSQL returns the IDs in the live link components of seeds, seeds
// included, as one sorted list.
func linkedMembersSQL(tx *sql.Tx, seeds []string) ([]string, error) {
	pairs, err := liveLinkPairsSQL(tx)
	if err != nil {
		return nil, err
	}
	return flattenComponents(linkComponents(pairs, seeds)), nil
}

func flattenComponents(components [][]string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, component := range components {
		for _, id := range component {
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// rekeyedID is one applied succession of a batch.
type rekeyedID struct {
	oldID string
	newID string
}

// savedGroup is the sharing group of the succession at position index in a
// batch's application order.
type savedGroup struct {
	index   int
	members []string
}

// settledGroup maps a saved group through the re-keys applied AFTER it, in
// application order, so a member a later succession in the batch re-keyed away
// is replaced by its successor instead of being given a row of its own. The
// re-keys before the group's own succession are already reflected in it.
func settledGroup(group savedGroup, rekeys []rekeyedID) []string {
	members := append([]string(nil), group.members...)
	for _, rekey := range rekeys[group.index+1:] {
		for i, id := range members {
			if id == rekey.oldID {
				members[i] = rekey.newID
			}
		}
	}
	return flattenComponents([][]string{members})
}

// groupAfterRekey is the set a succession's pre-re-key members form once the
// predecessor's ID has become the successor's: they were one linked resource
// before the re-key and stay one group for sharing whatever the re-keyed
// links now read as.
func groupAfterRekey(members []string, oldID, newID string) []string {
	group := make([]string, 0, len(members)+1)
	group = append(group, newID)
	for _, id := range members {
		if id == oldID {
			continue
		}
		group = append(group, id)
	}
	return flattenComponents([][]string{group})
}

// carryRemediationLockLocked is carryRemediationLockSQL for the in-memory
// store. The caller holds m.mu.
func (m *MemoryStore) carryRemediationLockLocked(survivorID, donorID string, at time.Time, createRow bool) {
	survivorID = strings.TrimSpace(survivorID)
	donorID = strings.TrimSpace(donorID)
	if survivorID == "" || donorID == "" || survivorID == donorID {
		return
	}
	donor, donorFound := m.resourceOperatorState[donorID]
	survivor, survivorFound := m.resourceOperatorState[survivorID]
	if !survivorFound && !createRow {
		return
	}
	if next, changed := CarryRemediationLock(survivorID, survivor, survivorFound, donor, donorFound, at); changed {
		if m.resourceOperatorState == nil {
			m.resourceOperatorState = make(map[string]ResourceOperatorState)
		}
		m.resourceOperatorState[survivorID] = next
	}
}

// shareRemediationLockAmongLocked is shareRemediationLockAmongSQL for the
// in-memory store. The caller holds m.mu.
func (m *MemoryStore) shareRemediationLockAmongLocked(group []string, at time.Time) {
	if len(group) < 2 {
		return
	}
	states := make(map[string]ResourceOperatorState, len(group))
	for _, id := range group {
		if state, found := m.resourceOperatorState[id]; found {
			states[id] = state
		}
	}
	writes, _ := shareRemediationLock(group, states, at)
	for _, next := range writes {
		if m.resourceOperatorState == nil {
			m.resourceOperatorState = make(map[string]ResourceOperatorState)
		}
		m.resourceOperatorState[next.CanonicalID] = next
	}
}

// shareRemediationLockAcrossLinksLocked is shareRemediationLockAcrossLinksSQL
// for the in-memory store. The caller holds m.mu.
func (m *MemoryStore) shareRemediationLockAcrossLinksLocked(seeds []string, at time.Time) {
	for _, component := range linkComponents(m.livePairsLocked(), seeds) {
		m.shareRemediationLockAmongLocked(component, at)
	}
}

// livePairsLocked returns the in-memory store's live link pairs. The caller
// holds m.mu.
func (m *MemoryStore) livePairsLocked() [][2]string {
	live, _ := effectiveManualPairDecisions(m.links, m.exclusions)
	return linkPairs(live)
}

const remediationLockCarryMigration = "remediation-lock-carry-v1"

// migrateRemediationLockCarry carries the locks that identity changes dropped
// before the stores carried them. It runs once per store, so an operator who
// later lifts a lock is not overruled by a dormant row on every start:
//
//   - a succession applied while its successor already had a row left the
//     predecessor's row behind, shadowed, and with it the predecessor's lock;
//   - a link recorded before a lock followed it left the lock on a member the
//     registry folded away.
//
// Successions are carried first so a lock they restore then spreads across the
// successor's links. A failure is logged and retried on the next start rather
// than keeping the store from opening.
func (s *SQLiteResourceStore) migrateRemediationLockCarry() error {
	var applied int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM resource_store_migrations WHERE name = ?`, remediationLockCarryMigration).Scan(&applied); err != nil {
		return fmt.Errorf("read remediation lock carry marker: %w", err)
	}
	if applied > 0 {
		return nil
	}

	rows, err := s.db.Query(`SELECT old_canonical_id, new_canonical_id FROM canonical_id_successions`)
	if err != nil {
		return fmt.Errorf("query recorded canonical ID successions: %w", err)
	}
	var successions []CanonicalIDSuccession
	for rows.Next() {
		var succession CanonicalIDSuccession
		if err := rows.Scan(&succession.OldCanonicalID, &succession.NewCanonicalID); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan recorded canonical ID succession: %w", err)
		}
		successions = append(successions, succession)
	}
	rowsErr := rows.Err()
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close recorded canonical ID successions: %w", err)
	}
	if rowsErr != nil {
		return fmt.Errorf("iterate recorded canonical ID successions: %w", rowsErr)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin remediation lock carry: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	at := time.Now().UTC()
	var carried []carriedLock
	// Successions chain (a -> b, then b -> c), an intermediate row may have
	// moved on to its own successor, and a lock a succession restores can reach
	// a link whose far member heads another succession. A block therefore goes
	// to every ID its chain reaches that still has a row, and the two steps
	// repeat until a pass changes nothing. Every productive pass sets a block
	// that was unset on some ID, so the passes needed are bounded by the number
	// of IDs the successions and links name.
	livePairs, err := liveLinkPairsSQL(tx)
	if err != nil {
		return err
	}
	maxPasses := 2*(len(successions)+len(livePairs)) + 8
	converged := false
	for pass := 0; pass <= maxPasses; pass++ {
		changedAny := false
		for _, oldID := range recordedPredecessors(successions) {
			for _, descendant := range successionDescendants(successions, oldID) {
				changed, err := carryRemediationLockSQL(tx, descendant, oldID, at, false)
				if err != nil {
					return fmt.Errorf("carry remediation lock %q -> %q: %w", oldID, descendant, err)
				}
				if changed {
					changedAny = true
					carried = append(carried, carriedLock{to: descendant, from: oldID})
				}
			}
		}
		pairs, err := liveLinkPairsSQL(tx)
		if err != nil {
			return err
		}
		var seeds []string
		for _, pair := range pairs {
			seeds = append(seeds, pair[0], pair[1])
		}
		shared, err := shareRemediationLockAcrossLinksSQL(tx, seeds, at)
		if err != nil {
			return err
		}
		if len(shared) > 0 {
			changedAny = true
			carried = append(carried, shared...)
		}
		if !changedAny {
			converged = true
			break
		}
	}
	if !converged {
		// Cannot happen while every change sets a block that was unset, but a
		// store that did not finish must not record that it did.
		return fmt.Errorf("remediation lock carry did not converge")
	}

	if _, err := tx.Exec(`INSERT INTO resource_store_migrations (name) VALUES (?)`, remediationLockCarryMigration); err != nil {
		return fmt.Errorf("record remediation lock carry marker: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit remediation lock carry: %w", err)
	}
	committed = true
	logCarriedLocks("restoring a lock an earlier link or succession dropped", carried)
	return nil
}

// recordedPredecessors lists the predecessor IDs of successions, sorted and
// deduplicated.
func recordedPredecessors(successions []CanonicalIDSuccession) []string {
	seen := make(map[string]struct{}, len(successions))
	var out []string
	for _, succession := range successions {
		oldID := CanonicalResourceID(succession.OldCanonicalID)
		if oldID == "" {
			continue
		}
		if _, dup := seen[oldID]; dup {
			continue
		}
		seen[oldID] = struct{}{}
		out = append(out, oldID)
	}
	sort.Strings(out)
	return out
}

// successionDescendants returns every ID the successions lead from oldID to,
// directly or through further successions, in sorted order. oldID itself is
// never returned, and a cycle ends the walk instead of repeating it.
func successionDescendants(successions []CanonicalIDSuccession, oldID string) []string {
	successors := make(map[string][]string, len(successions))
	for _, succession := range successions {
		from, to := CanonicalResourceID(succession.OldCanonicalID), CanonicalResourceID(succession.NewCanonicalID)
		if from == "" || to == "" || from == to {
			continue
		}
		successors[from] = append(successors[from], to)
	}
	seen := map[string]struct{}{oldID: {}}
	var out []string
	for queue := []string{oldID}; len(queue) > 0; queue = queue[1:] {
		for _, next := range successors[queue[0]] {
			if _, dup := seen[next]; dup {
				continue
			}
			seen[next] = struct{}{}
			out = append(out, next)
			queue = append(queue, next)
		}
	}
	sort.Strings(out)
	return out
}
