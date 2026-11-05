package eventlog

// The recovery journal is independent of events.db. Each bounded immutable
// batch is synced before admission, then deleted only after its cursor commits
// with the events and active-state projection. No timer expires unsettled facts
// or gap evidence. Normal event retention begins after they reach events.db.

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	maxDiagnosticEvents = maxJournalEvents * 3 / 4
	maxDiagnosticBytes  = maxJournalBytes * 3 / 4
	journalDirectory    = "event-history-journal"
	maxJournalEvents    = 16384
	maxJournalBytes     = 32 << 20
	journalFileName     = "events.journal"
	journalDamageName   = "damaged.journal"
)

var errJournalCapacity = errors.New("history journal capacity exhausted")

type journalState struct {
	ReservedThrough         int64     `json:"reservedThrough"`
	DamageCount             uint64    `json:"damageCount"`
	AcknowledgedDamageCount uint64    `json:"acknowledgedDamageCount"`
	LastDamageDigest        string    `json:"lastDamageDigest,omitempty"`
	LastDamageBytes         int       `json:"lastDamageBytes,omitempty"`
	Version                 int       `json:"version"`
	ID                      string    `json:"id"`
	Open                    bool      `json:"open"`
	GapRevision             uint64    `json:"gapRevision"`
	Lost                    uint64    `json:"lost"`
	Unknown                 bool      `json:"unknown"`
	AcknowledgedLost        uint64    `json:"acknowledgedLost"`
	AcknowledgedAt          time.Time `json:"acknowledgedAt,omitempty"`
	Acknowledgement         string    `json:"acknowledgement,omitempty"`
}

type journalRecord struct {
	Sequence   int64   `json:"sequence"`
	Diagnostic bool    `json:"diagnostic"`
	Events     []Event `json:"events"`
	bytes      int
}

type recoveryJournal struct {
	cleanupRequired bool
	sealed          bool
	file            *os.File
	physical        int
	frames          int
	compactRequired bool
	broken          bool
	dir             string
	state           journalState
	pending         []journalRecord
	next            int64
	bytes           int
	events          int
	stateDirty      bool
	err             error
	unlock          func()
}

// DurabilityStatus distinguishes retained work from missing facts. LostEvents
// is an aggregate, not a reconstruction of the missing events. UnknownGap
// includes an unclean process exit: journalled facts can be recovered, but an
// event interrupted before journal sync cannot be proved to have existed.
// Acknowledgement accepts that limitation; it never reconstructs missing data.
type DurabilityStatus struct {
	GapEvidencePersisted    bool      `json:"gapEvidencePersisted"`
	DamageCount             uint64    `json:"damageCount"`
	AcknowledgedDamageCount uint64    `json:"acknowledgedDamageCount"`
	LastDamageDigest        string    `json:"lastDamageDigest,omitempty"`
	LastDamageBytes         int       `json:"lastDamageBytes,omitempty"`
	PendingEvents           int       `json:"pendingEvents"`
	PendingBytes            int       `json:"pendingBytes"`
	LostEvents              uint64    `json:"lostEvents"`
	UnknownGap              bool      `json:"unknownGap"`
	GapRevision             uint64    `json:"gapRevision"`
	AcknowledgedLostEvents  uint64    `json:"acknowledgedLostEvents"`
	AcknowledgedAt          time.Time `json:"acknowledgedAt,omitempty"`
	Acknowledgement         string    `json:"acknowledgement,omitempty"`
	Error                   string    `json:"error,omitempty"`
	Closed                  bool      `json:"closed"`
}

func (s *Store) DurabilityStatus() DurabilityStatus {
	if s == nil {
		return DurabilityStatus{Error: "event log is not enabled"}
	}
	s.journalMu.Lock()
	defer s.journalMu.Unlock()
	j := s.journal
	status := DurabilityStatus{GapEvidencePersisted: !j.stateDirty, DamageCount: j.state.DamageCount, AcknowledgedDamageCount: j.state.AcknowledgedDamageCount, LastDamageDigest: j.state.LastDamageDigest, LastDamageBytes: j.state.LastDamageBytes, PendingEvents: j.events, PendingBytes: j.bytes, LostEvents: j.state.Lost,
		UnknownGap: j.state.Unknown, GapRevision: j.state.GapRevision, AcknowledgedLostEvents: j.state.AcknowledgedLost,
		AcknowledgedAt: j.state.AcknowledgedAt, Acknowledgement: j.state.Acknowledgement, Closed: s.closed}
	if j.err != nil {
		status.Error = j.err.Error()
	} else if j.broken {
		status.Error = "history journal tail awaits repair"
	}
	return status
}

func (s *Store) Healthy() bool { return s.DurabilityStatus().Healthy() }

func (status DurabilityStatus) Healthy() bool {
	return !status.Closed && status.PendingEvents == 0 && status.LostEvents == 0 && !status.UnknownGap && status.Error == ""
}

// AcknowledgeGaps is an explicit administrative disposition of missing facts,
// guarded by the exact observed revision. It cannot dispose of pending events
// or clear an I/O fault. No automatic expiry or successful-write baseline does
// this. The last acknowledgement and cumulative acknowledged count survive
// restart, in bounded metadata. No HTTP write route is exposed here.
func (s *Store) AcknowledgeGaps(revision uint64, reason string) error {
	if s == nil {
		return fmt.Errorf("event log is not enabled")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 1024 {
		return fmt.Errorf("a reason of 1 to 1024 bytes is required")
	}
	s.journalMu.Lock()
	defer s.journalMu.Unlock()
	j := s.journal
	if s.closed || j.events != 0 || j.err != nil || j.broken {
		return fmt.Errorf("history obligations or storage errors remain unsettled")
	}
	if j.state.GapRevision != revision {
		return fmt.Errorf("history gap revision changed")
	}

	old := j.state
	j.state.AcknowledgedLost += j.state.Lost
	j.state.AcknowledgedDamageCount = j.state.DamageCount
	j.state.Lost = 0
	j.state.Unknown = false
	j.state.AcknowledgedAt = time.Now().UTC()
	j.state.Acknowledgement = reason
	j.state.GapRevision++
	if err := j.saveState(); err != nil {
		j.state = old
		return err
	}
	// Disposition is durable before forensic evidence is removed. A cleanup
	// refusal retains the bounded artifact and the successful acknowledgement.
	if j.dir != "" {
		if err := os.Remove(filepath.Join(j.dir, journalDamageName)); err != nil && !os.IsNotExist(err) {
			log.Warn().Err(err).Msg("acknowledged damaged journal cleanup deferred")
		} else if err := syncJournalDirectory(j.dir); err != nil {
			log.Warn().Err(err).Msg("acknowledged damaged journal cleanup sync deferred")
		}
	}
	return nil
}

func (j *recoveryJournal) saveState() error {
	if j.dir == "" {
		j.stateDirty = false
		return nil
	}
	data, err := json.Marshal(j.state)
	if err == nil {
		err = journalAtomicWrite(j.dir, "state.json", data)
	}
	j.stateDirty = err != nil
	return err
}

// Atomic replacement includes syncing the containing directory. Temporary
// files are bounded and owned; failed writes never replace existing evidence.
func journalAtomicWrite(dir, name string, data []byte) error {
	f, err := os.CreateTemp(dir, ".pending-")
	if err != nil {
		return err
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = replaceJournalFile(path, filepath.Join(dir, name))
	}
	if err == nil {
		err = syncJournalDirectory(dir)
	}
	return err
}

func (s *Store) openJournal(dir string) error {
	j := &recoveryJournal{state: journalState{Version: 1, ID: uuid.NewString()}}
	s.journal = j
	if s.dbPath == ":memory:" {
		return nil
	}
	j.dir = filepath.Join(dir, journalDirectory)
	if err := os.MkdirAll(j.dir, 0700); err != nil {
		return err
	}
	if err := syncJournalDirectory(dir); err != nil {
		return err
	}
	unlock, err := lockJournal(j.dir)
	if err != nil {
		return err
	}
	j.unlock = unlock
	success := false
	defer func() {
		if !success {
			unlock()
		}
	}()
	if err := j.cleanupTemps(); err != nil {
		j.err = err
	}
	stateFile, err := os.Open(filepath.Join(j.dir, "state.json"))
	var data []byte
	if err == nil {
		data, err = io.ReadAll(io.LimitReader(stateFile, 8193))
		stateFile.Close()
	}
	if err == nil {
		if len(data) > 8192 {
			return fmt.Errorf("history journal state exceeds limit")
		}
		if err := json.Unmarshal(data, &j.state); err != nil {
			return err
		}
		if j.state.Version != 1 || j.state.ID == "" {
			return fmt.Errorf("unsupported history journal state")
		}
		if j.state.Open {
			j.state.Unknown = true
			j.state.GapRevision++
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	var cursor string
	err = s.db.QueryRow(`SELECT value FROM alert_store_meta WHERE key='history_journal_cursor'`).Scan(&cursor)
	if err != nil && err != sql.ErrNoRows {
		j.err = err
		cursor = ""
	}
	prefix := j.state.ID + ":"
	committed := int64(0)
	if strings.HasPrefix(cursor, prefix) {
		committed, err = strconv.ParseInt(strings.TrimPrefix(cursor, prefix), 10, 64)
		if err != nil {
			return fmt.Errorf("invalid event history cursor: %w", err)
		}
	} else if cursor != "" {
		j.state.Unknown = true
		j.state.GapRevision++
	}
	j.next = max(committed, j.state.ReservedThrough)

	j.state.Open = true
	if err := j.saveState(); err != nil {
		// Keep the owner for reads, retained replay and retry. Admission cannot
		// proceed until this open-session marker (and any refusal gap) is synced.
		j.err = err
	}
	path := filepath.Join(j.dir, journalFileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	j.file = f
	defer func() {
		if !success {
			f.Close()
		}
	}()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > maxJournalBytes {
		return fmt.Errorf("history journal exceeds byte limit")
	}
	data, err = io.ReadAll(io.LimitReader(f, maxJournalBytes+1))
	if err != nil {
		return err
	}
	j.physical = len(data)
	allData := data
	lastSequence := int64(0)
	damaged := false
	for len(data) > 0 {
		if len(data) < 4 {
			damaged = true
			break
		}
		length := int(binary.BigEndian.Uint32(data[:4]))
		if length < 36 || length > len(data) {
			damaged = true
			break
		}
		record, err := decodeJournalRecord(data[:length])
		if err != nil || record.Sequence <= lastSequence {
			damaged = true
			break
		}
		lastSequence = record.Sequence
		j.frames++
		if record.Sequence > j.next {
			j.next = record.Sequence
		}
		if record.Sequence > committed {
			record.bytes = length
			j.pending = append(j.pending, record)
			j.events += len(record.Events)
			j.bytes += length
			// A failed target cursor read makes committed-but-uncompacted frames
			// uncertain too. Retain that bounded physical file even when its event
			// count exceeds the admission cap; refuse new work until it settles.
		}
		data = data[length:]
	}
	if damaged {
		// Preserve the entire damaged file as bounded forensic evidence, retain its
		// valid prefix, and report the remaining tail as an unknown gap. A second
		// unresolved corruption never overwrites the first artifact.
		mayReplace := j.state.DamageCount == j.state.AcknowledgedDamageCount
		j.state.Unknown = true
		j.state.GapRevision++
		digest := sha256.Sum256(allData)
		digestText := fmt.Sprintf("%x", digest[:])
		if j.state.DamageCount == j.state.AcknowledgedDamageCount || j.state.LastDamageDigest != digestText || j.state.LastDamageBytes != len(allData) {
			j.state.DamageCount++
		}
		j.state.LastDamageDigest = digestText
		j.state.LastDamageBytes = len(allData)
		if err := j.saveState(); err != nil {
			j.err = err
		}
		damagedPath := filepath.Join(j.dir, journalDamageName)
		if existing, err := os.ReadFile(damagedPath); err == nil {
			if !bytes.Equal(existing, allData) {
				if mayReplace {
					if err := journalAtomicWrite(j.dir, journalDamageName, allData); err != nil {
						j.err = err
					}
				} else {
					log.Warn().Str("sha256", j.state.LastDamageDigest).Int("bytes", len(allData)).Msg("additional damaged journal accounted; first forensic artifact retained")
				}
			}
		} else if os.IsNotExist(err) {
			if err := journalAtomicWrite(j.dir, journalDamageName, allData); err != nil {
				j.err = err
			}
		} else {
			j.err = err
		}

		j.broken = true
		j.compactRequired = true
	}
	// Startup never needs a second payload-sized allocation merely to open a
	// valid journal. Failed cleanup retains the owner and the recoverable prefix.
	if j.events == 0 && j.physical > 0 {
		j.compactRequired = true
	}
	if damaged || j.events == 0 {
		if err := j.maybeCompact(); err != nil {
			j.err = err
		}
	}
	success = true
	return nil
}

func encodeJournalRecord(record journalRecord) ([]byte, error) {
	body, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	if len(body) > maxJournalBytes-36 {
		return nil, fmt.Errorf("%w: frame exceeds byte limit", errJournalCapacity)
	}
	data := make([]byte, 36+len(body))
	binary.BigEndian.PutUint32(data[:4], uint32(len(data)))
	hash := sha256.Sum256(body)
	copy(data[4:36], hash[:])
	copy(data[36:], body)
	return data, nil
}

func decodeJournalRecord(data []byte) (journalRecord, error) {
	var record journalRecord
	if len(data) < 36 || int(binary.BigEndian.Uint32(data[:4])) != len(data) {
		return record, fmt.Errorf("incomplete frame")
	}
	hash := sha256.Sum256(data[36:])
	if !bytes.Equal(hash[:], data[4:36]) {
		return record, fmt.Errorf("checksum mismatch")
	}
	if err := json.Unmarshal(data[36:], &record); err != nil {
		return record, err
	}
	if len(record.Events) == 0 {
		return record, fmt.Errorf("empty frame")
	}
	return record, nil
}

// compact atomically rewrites only the unsettled suffix, then replaces the
// append handle. Peak payload on disk is bounded by the journal, one compact
// replacement and one quarantined damaged journal (at most 3 * 32 MiB).
func (j *recoveryJournal) compact() error {
	if j.dir == "" {
		j.physical = j.bytes
		j.frames = len(j.pending)
		j.compactRequired = false
		return nil
	}
	var data []byte
	for _, record := range j.pending {
		frame, err := encodeJournalRecord(record)
		if err != nil {
			return err
		}
		data = append(data, frame...)
	}
	if j.file != nil {
		if err := j.file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			j.file = nil
			j.compactRequired = true
			return err
		}
		j.file = nil
	}
	if err := journalAtomicWrite(j.dir, journalFileName, data); err != nil {
		j.compactRequired = true
		return err
	}
	f, err := os.OpenFile(filepath.Join(j.dir, journalFileName), os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		j.compactRequired = true
		return err
	}
	j.file = f
	j.physical = len(data)
	j.frames = len(j.pending)
	j.compactRequired = false
	j.broken = false
	return nil
}

func (j *recoveryJournal) cleanupTemps() error {
	if j.dir == "" {
		return nil
	}
	j.cleanupRequired = true
	entries, err := os.ReadDir(j.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".pending-") {
			if err := os.Remove(filepath.Join(j.dir, entry.Name())); err != nil {
				return err
			}
		}
	}
	if err := syncJournalDirectory(j.dir); err != nil {
		return err
	}
	j.cleanupRequired = false
	return nil
}

func (j *recoveryJournal) maybeCompact() error {
	if j.cleanupRequired {
		if err := j.cleanupTemps(); err != nil {
			return err
		}
	}
	if j.broken {
		// The in-memory prefix contains only successfully synced admissions.
		// Persist uncertainty before replacing an ambiguous write/fsync tail.
		if j.stateDirty {
			if err := j.saveState(); err != nil {
				return err
			}
		}
		return j.compact()
	}
	if j.compactRequired || (j.frames >= 1024 && j.bytes < j.physical/2) || (j.physical >= 1<<20 && j.bytes < j.physical/2) {
		return j.compact()
	}
	return nil
}

func (s *Store) journalEvents(events []Event, diagnostic bool, retainedByCaller ...bool) (int64, error) {
	s.journalMu.Lock()
	defer s.journalMu.Unlock()
	j := s.journal
	fail := func(err error, unknown bool) (int64, error) {
		if unknown {
			j.state.Unknown = true
		} else if len(retainedByCaller) > 0 && retainedByCaller[0] {
			return 0, err
		} else {
			j.state.Lost += uint64(len(events))
		}
		j.state.GapRevision++
		if saveErr := j.saveState(); saveErr != nil {
			j.err = fmt.Errorf("%v; gap persistence failed: %w", err, saveErr)
		}
		return 0, err
	}
	if s.closed {
		return fail(fmt.Errorf("event log is closed; event rejected"), false)
	}
	if j.cleanupRequired {
		if err := j.cleanupTemps(); err != nil {
			return fail(err, false)
		}
	}
	if j.broken {
		if err := j.maybeCompact(); err != nil {
			return fail(err, false)
		}
	}
	if diagnostic && j.events+len(events) > maxDiagnosticEvents {
		return fail(errJournalCapacity, false)
	}
	// Migration retries can overlap a previously admitted chunk while SQLite
	// remains unavailable. Reuse its obligation rather than consume the reserve.
	if len(retainedByCaller) > 0 && retainedByCaller[0] {
		filtered := make([]Event, 0, len(events))
		last := int64(0)
		for _, event := range events {
			duplicate := false
			if event.Type == TypeHistoryImported {
				for _, pending := range j.pending {
					for _, prior := range pending.Events {
						if prior.Type == TypeHistoryImported && prior.AlertID == event.AlertID && prior.OccurredAt.Equal(event.OccurredAt) && bytes.Equal(prior.Snapshot, event.Snapshot) {
							duplicate = true
							last = max(last, pending.Sequence)
							break
						}
					}
					if duplicate {
						break
					}
				}
			}
			if !duplicate {
				filtered = append(filtered, event)
			}
		}
		if len(filtered) == 0 {
			return last, nil
		}
		events = filtered
	}
	if len(events) > maxJournalEvents-j.events {
		return fail(errJournalCapacity, false)
	}
	for _, event := range events {
		if event.Type == TypeActiveMoved && (event.Details["previousAlertId"] == "" || event.Details["previousAlertId"] == event.AlertID) {
			return fail(fmt.Errorf("identity move requires a distinct previous alert identity"), false)
		}
		switch event.Type {
		case TypeFired, TypeRefired, TypeResolved, TypeActiveRetired, TypeActiveMoved, TypeAcknowledged, TypeUnacknowledged, TypeSnoozed, TypeUnsnoozed, TypeEscalated:
			if len(event.Snapshot) > 0 {
				if _, err := activeSnapshotFromEvent(event); err != nil {
					return fail(err, false)
				}
			}
		}
	}
	record := journalRecord{Sequence: j.next + 1, Diagnostic: diagnostic, Events: append([]Event(nil), events...)}
	for i := range record.Events {
		record.Events[i].ID = 0
		if record.Events[i].OccurredAt.IsZero() {
			record.Events[i].OccurredAt = time.Now().UTC()
		}
	}
	data, err := encodeJournalRecord(record)
	if err != nil {
		return fail(err, false)
	}
	if len(data) > maxJournalBytes-j.bytes || (diagnostic && j.bytes+len(data) > maxDiagnosticBytes) {
		return fail(errJournalCapacity, false)
	}
	if j.stateDirty {
		if err := j.saveState(); err != nil {
			return fail(err, false)
		}
	}
	if j.compactRequired || j.physical+len(data) > maxJournalBytes {
		if err := j.compact(); err != nil {
			return fail(err, false)
		}
	}
	if record.Sequence > j.state.ReservedThrough {
		j.state.ReservedThrough = record.Sequence + 1023
		if err := j.saveState(); err != nil {
			return fail(err, false)
		}
	}
	j.next = record.Sequence
	if j.file != nil {
		n, err := j.file.Write(data)
		j.physical += n
		if err == nil && n != len(data) {
			err = io.ErrShortWrite
		}
		if err == nil {
			err = j.file.Sync()
		}
		if err != nil {
			// A partial frame must not be followed by another one. The pre-existing
			// synced open-session marker remains the fallback if gap persistence also
			// fails. Repair retains the synced admitted prefix and preserves unknown-tail evidence.
			j.broken = true
			return fail(err, true)
		}
	} else {
		j.physical += len(data)
	}
	owned, err := decodeJournalRecord(data)
	if err != nil {
		return fail(err, false)
	}
	owned.bytes = len(data)
	j.frames++
	j.pending = append(j.pending, owned)
	j.events += len(events)
	j.bytes += len(data)
	return record.Sequence, nil
}

// applyMu keeps target commits and acknowledgements in journal admission order.
// Journal admission holds only journalMu, never this SQLite I/O lease.
func (s *Store) recoverThrough(target int64) error {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	if !s.targetReady {
		if err := s.initSchema(); err != nil {
			s.journalMu.Lock()
			s.journal.err = err
			s.journalMu.Unlock()
			return err
		}
		s.targetReady = true
	}
	s.journalMu.Lock()
	if target == 0 {
		target = s.journal.next
	}
	s.journalMu.Unlock()
	for {
		s.journalMu.Lock()
		j := s.journal
		count := 0
		var batch []Event
		diagnostic := int64(0)
		for count < len(j.pending) && count < 128 && j.pending[count].Sequence <= target {
			r := j.pending[count]

			if r.Diagnostic {
				diagnostic += int64(len(r.Events))
			}
			count++
		}
		if count == 0 {
			err := j.maybeCompact()
			if err == nil && j.stateDirty {
				err = j.saveState()
			}
			j.err = err
			s.journalMu.Unlock()
			return err
		}
		records := append([]journalRecord(nil), j.pending[:count]...)
		id := j.state.ID
		cursor := fmt.Sprintf("%s:%d", id, records[count-1].Sequence)
		s.journalMu.Unlock()
		// Read the durable cursor even in-process: Commit or journal compaction can fail
		// after the database actually committed. Replaying that prefix would invent
		// duplicate transitions and could resurrect an already resolved occurrence.
		committed, cursorErr := s.journalCommittedSequence(id)
		if cursorErr != nil {
			s.journalMu.Lock()
			j.err = cursorErr
			s.journalMu.Unlock()
			return cursorErr
		}
		batch = nil
		for _, r := range records {
			if r.Sequence > committed {
				for _, event := range r.Events {
					event.journalID = id
					event.journalSequence = r.Sequence
					batch = append(batch, event)
				}
			}
		}
		if len(batch) > 0 {
			if err := s.insertJournalBatch(batch, cursor); err != nil {
				s.journalMu.Lock()
				j.err = err
				s.journalMu.Unlock()
				return err
			}
		}
		s.journalMu.Lock()

		for _, r := range records {
			j.events -= len(r.Events)
			j.bytes -= r.bytes
		}
		for i := 0; i < count; i++ {
			j.pending[i] = journalRecord{}
		}
		j.pending = j.pending[count:]
		if len(j.pending) == 0 {
			j.pending = nil
		}
		if err := j.maybeCompact(); err != nil {
			j.err = err
			s.journalMu.Unlock()
			return err
		}
		if j.stateDirty {
			if err := j.saveState(); err != nil {
				j.err = err
				s.journalMu.Unlock()
				return err
			}
		}
		j.err = nil
		s.written.Add(diagnostic)
		s.journalMu.Unlock()
	}
}

func (s *Store) gapError() error {
	status := s.DurabilityStatus()
	if status.LostEvents != 0 || status.UnknownGap || status.Error != "" {
		return fmt.Errorf("event history incomplete: lost=%d unknown=%t revision=%d storage=%s", status.LostEvents, status.UnknownGap, status.GapRevision, status.Error)
	}
	return nil
}

func (s *Store) closeJournal() {
	s.journalMu.Lock()
	defer s.journalMu.Unlock()
	j := s.journal
	if j.file != nil {
		_ = j.file.Close()
	}
	j.sealed = true
	j.state.Open = false
	if err := j.saveState(); err != nil {
		log.Error().Err(err).Msg("event history journal clean-close marker failed")
	}
	if j.unlock != nil {
		j.unlock()
	}
}

// JournalBoundary is a durable admission prefix. Recovery snapshots capture it
// before copying active state, which is updated before event admission. A
// snapshot therefore contains every accepted lifecycle fact through this token.
type JournalBoundary struct {
	ID       string `json:"id"`
	Sequence int64  `json:"sequence"`
}

func (event Event) JournalBoundary() JournalBoundary {
	return JournalBoundary{ID: event.journalID, Sequence: event.journalSequence}
}

func (s *Store) JournalBoundary() JournalBoundary {
	if s == nil {
		return JournalBoundary{}
	}
	s.journalMu.Lock()
	defer s.journalMu.Unlock()
	return JournalBoundary{ID: s.journal.state.ID, Sequence: s.journal.next}
}

// beginOperation linearizes public work against Close. Close drains both the
// background owner and every admitted public operation before sealing the
// journal and releasing its physical lock.
func (s *Store) beginOperation(offered int, trackGap bool) error {
	s.journalMu.Lock()
	defer s.journalMu.Unlock()
	if s.closed {
		if trackGap && !s.journal.sealed {
			s.journal.state.Lost += uint64(offered)
			s.journal.state.GapRevision++
			if err := s.journal.saveState(); err != nil {
				s.journal.err = err
			}
		}
		return fmt.Errorf("event log is closed; event rejected")
	}
	s.operations.Add(1)
	return nil
}

func (s *Store) journalCommittedSequence(id string) (int64, error) {
	var cursor string
	err := s.db.QueryRow(`SELECT value FROM alert_store_meta WHERE key='history_journal_cursor'`).Scan(&cursor)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !strings.HasPrefix(cursor, id+":") {
		return 0, nil
	}
	sequence, err := strconv.ParseInt(strings.TrimPrefix(cursor, id+":"), 10, 64)
	if err != nil || sequence < 0 {
		return 0, fmt.Errorf("invalid history journal cursor %q", cursor)
	}
	return sequence, nil
}

// WalkRecoverySince supplies a bounded, consistent lifecycle recovery view of
// committed rows followed by still-pending journal facts. It never invokes live
// lifecycle/delivery callbacks. The visitor must not call durable store writes.
func (s *Store) WalkRecoverySince(boundary JournalBoundary, visit func(Event) error) error {
	if s == nil || visit == nil {
		return nil
	}
	if err := s.beginOperation(0, false); err != nil {
		return err
	}
	defer s.operations.Done()
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	s.journalMu.Lock()
	id, target := s.journal.state.ID, s.journal.next
	s.journalMu.Unlock()
	if boundary.ID != "" && boundary.ID != id {
		return fmt.Errorf("active recovery checkpoint journal identity changed")
	}
	committed, cursorErr := s.journalCommittedSequence(id)
	readErr := cursorErr
	if readErr == nil {
		readErr = s.WalkByID(Filter{JournalID: id, AfterJournalSequence: boundary.Sequence}, visit)
	}
	if readErr != nil {
		// With no readable target cursor, every retained physical frame is a
		// possible obligation. Replaying its explicit facts is still safer than
		// resurrecting a stale mirror. The caller retains its conservative floor.
		committed = 0
	}
	s.journalMu.Lock()
	var tail []Event
	for _, record := range s.journal.pending {
		if record.Sequence <= max(committed, boundary.Sequence) || record.Sequence > target {
			continue
		}
		for _, event := range record.Events {
			event.journalID, event.journalSequence = id, record.Sequence
			event.Snapshot = append(json.RawMessage(nil), event.Snapshot...)
			details := make(map[string]string, len(event.Details))
			for key, value := range event.Details {
				details[key] = value
			}
			event.Details = details
			tail = append(tail, event)
		}
	}
	s.journalMu.Unlock()
	for _, event := range tail {
		if err := visit(event); err != nil {
			return err
		}
	}
	return readErr
}

// LoadActiveStateAtJournalBoundary reads the active projection and its durable
// admission prefix under the same replay lease. Later committed or pending
// facts can then be folded without inferring order from occurrence clocks.
func (s *Store) LoadActiveStateAtJournalBoundary() ([]ActiveStateSnapshot, JournalBoundary, error) {
	if s == nil {
		return nil, JournalBoundary{}, fmt.Errorf("event log is not enabled")
	}
	if err := s.beginOperation(0, false); err != nil {
		return nil, JournalBoundary{}, err
	}
	defer s.operations.Done()
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	snapshots, err := s.LoadActiveState()
	if err != nil {
		return nil, JournalBoundary{}, err
	}
	s.journalMu.Lock()
	id := s.journal.state.ID
	s.journalMu.Unlock()
	sequence, err := s.journalCommittedSequence(id)
	return snapshots, JournalBoundary{ID: id, Sequence: sequence}, err
}
