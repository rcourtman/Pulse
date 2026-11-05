package eventlog

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func writerQueryOnly(t *testing.T, s *Store, on bool) {
	t.Helper()
	value := 0
	if on {
		value = 1
	}
	if _, err := s.db.Exec(fmt.Sprintf("PRAGMA query_only=%d", value)); err != nil {
		t.Fatal(err)
	}
}

func journalEventCount(t *testing.T, s *Store) int {
	t.Helper()
	var count int
	if err := s.db.QueryRow(`SELECT count(*) FROM alert_events`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestJournalRecoversRealBackgroundWriteFault(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	writerQueryOnly(t, s, true)
	at := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 40; i++ {
		s.Append(Event{Type: TypeNotificationDispatched, AlertID: fmt.Sprint(i), OccurredAt: at.Add(time.Duration(i) * time.Second)})
	}
	if err := s.Flush(); err == nil {
		t.Fatal("faulted flush reported success")
	}
	if got := s.DurabilityStatus(); got.PendingEvents != 40 || got.LostEvents != 0 || s.Healthy() {
		t.Fatalf("fault status: %+v", got)
	}
	if got := journalEventCount(t, s); got != 0 {
		t.Fatalf("faulted database contains %d events", got)
	}
	writerQueryOnly(t, s, false)
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if !s.Healthy() || journalEventCount(t, s) != 40 {
		t.Fatalf("recovery: %+v", s.DurabilityStatus())
	}
	rows, err := s.Query(Filter{AlertID: "0"})
	if err != nil || len(rows) != 1 || !rows[0].OccurredAt.Equal(at) {
		t.Fatalf("original event time changed: %+v %v", rows, err)
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Flush(); err != nil || journalEventCount(t, s) != 40 {
		t.Fatalf("reopen: %v %+v", err, s.DurabilityStatus())
	}
	info, err := os.Stat(filepath.Join(dir, journalDirectory, journalFileName))
	if err != nil || info.Size() != 0 {
		t.Fatalf("settled journal did not compact on reopen: %v %v", info, err)
	}
}

func TestJournalAdmissionDoesNotWaitForSQLiteOrDropAtOldBufferLimit(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	conn, err := s.db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	const offered = appendBufferSize + 500
	done := make(chan struct{})
	go func() {
		for i := 0; i < offered; i++ {
			s.Append(Event{Type: TypeNotificationDispatched, AlertID: fmt.Sprint(i)})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Minute):
		// This bounds a broken test, not filesystem admission latency. In
		// particular, race instrumentation and shared-disk syncs are not an
		// indication that admission waited for the held SQLite connection.
		status := s.DurabilityStatus()
		conn.Close()
		t.Fatalf("admissions did not complete with SQLite held: %+v", status)
	}
	if got := s.DurabilityStatus(); got.PendingEvents != offered || got.LostEvents != 0 || s.Dropped() != 0 {
		t.Fatalf("admission status: %+v", got)
	}
	var committed int
	if err := s.readDB.QueryRow(`SELECT count(*) FROM alert_events`).Scan(&committed); err != nil || committed != 0 {
		t.Fatalf("SQLite changed while its writer connection remained held: count=%d error=%v", committed, err)
	}
	conn.Close()
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if got := journalEventCount(t, s); got != offered {
		t.Fatalf("wrote %d of %d events", got, offered)
	}
}

func TestJournalSynchronousFailuresSettleExactFactsWithoutPoison(t *testing.T) {
	s := newTestStore(t)
	writerQueryOnly(t, s, true)
	if err := s.AppendDurable(Event{Type: TypeAcknowledged, AlertID: "sync"}); err == nil {
		t.Fatal("durable append succeeded under write fault")
	}
	if err := s.ImportEvents([]Event{{Type: TypeHistoryCleared, AlertID: "history"}}); err == nil {
		t.Fatal("import succeeded under write fault")
	}
	if s.DurabilityStatus().PendingEvents != 2 {
		t.Fatalf("failed callers' obligations were discarded: %+v", s.DurabilityStatus())
	}
	writerQueryOnly(t, s, false)
	s.Append(Event{Type: TypeNotificationDispatched, AlertID: "later"})
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if journalEventCount(t, s) != 3 || !s.Healthy() {
		t.Fatalf("settled failures poisoned health: %+v", s.DurabilityStatus())
	}
}

func TestJournalFailedCloseRetainsPendingAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	writerQueryOnly(t, s, true)
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "unsettled"}); err == nil {
		t.Fatal("write fault did not fail")
	}
	s.Close() // query_only still on: shutdown cannot settle the event
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Flush(); err != nil {
		t.Fatal(err)
	}
	if journalEventCount(t, reopened) != 1 || !reopened.Healthy() {
		t.Fatalf("reopen discarded pending fact: %+v", reopened.DurabilityStatus())
	}
}

func TestJournalCapacityGapSurvivesSuccessAndReopenUntilAcknowledged(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Exercise the real event-count admission cap without filling the disk.
	events := make([]Event, maxJournalEvents+1)
	for i := range events {
		events[i] = Event{Type: TypeNotificationDispatched, AlertID: fmt.Sprint(i)}
	}
	if _, err := s.journalEvents(events, false); err == nil {
		t.Fatal("over-capacity admission succeeded")
	}
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "later"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err == nil || s.Healthy() {
		t.Fatal("unrelated success concealed gap")
	}
	before := s.DurabilityStatus()
	if before.LostEvents != uint64(len(events)) || before.PendingEvents != 0 {
		t.Fatalf("gap: %+v", before)
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.DurabilityStatus(); got.LostEvents != before.LostEvents || got.GapRevision != before.GapRevision || s.Flush() == nil {
		t.Fatalf("reopen forgot gap: %+v", got)
	}
	if err := s.AcknowledgeGaps(before.GapRevision+1, "accepted missing diagnostics"); err == nil {
		t.Fatal("stale acknowledgement succeeded")
	}
	if err := s.AcknowledgeGaps(before.GapRevision, "accepted missing diagnostics"); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil || !s.Healthy() {
		t.Fatalf("acknowledgement: %v %+v", err, s.DurabilityStatus())
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	status := s.DurabilityStatus()
	if !s.Healthy() || status.AcknowledgedLostEvents != before.LostEvents || status.Acknowledgement != "accepted missing diagnostics" {
		t.Fatalf("lost acknowledgement audit: %+v", status)
	}
}

func TestJournalCompactionFailureCannotDuplicateCommittedFacts(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.journalEvents([]Event{{Type: TypeFired, AlertID: "once"}}, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.journal.dir, journalFileName)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "prevent-replace"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	s.journal.compactRequired = true
	if err := s.Flush(); err == nil {
		t.Fatal("failed compaction reported success")
	}
	if journalEventCount(t, s) != 1 {
		t.Fatal("database commit did not happen before compaction failure")
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if journalEventCount(t, s) != 1 || !s.Healthy() {
		t.Fatalf("compaction retry duplicated facts: %+v", s.DurabilityStatus())
	}
}

func TestJournalOwnsCallerMemory(t *testing.T) {
	s := newTestStore(t)
	writerQueryOnly(t, s, true)
	event := Event{Type: TypeFired, AlertID: "owned", Details: map[string]string{"key": "original"}, Snapshot: []byte(`{"id":"owned"}`)}
	s.Append(event)
	event.Details["key"] = "changed"
	copy(event.Snapshot, []byte(`{"id":"other"}`))
	writerQueryOnly(t, s, false)
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	rows, err := s.Query(Filter{AlertID: "owned"})
	if err != nil || len(rows) != 1 || rows[0].Details["key"] != "original" || string(rows[0].Snapshot) != `{"id":"owned"}` {
		t.Fatalf("caller mutated retained facts: %+v %v", rows, err)
	}
}

func TestJournalRejectsSecondOwnerAndPostCloseAdmission(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := Open(dir); err == nil {
		second.Close()
		s.Close()
		t.Fatal("two journal owners admitted")
	}
	s.Close()
	s.Append(Event{Type: TypeFired, AlertID: "late"})
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "late"}); err == nil {
		t.Fatal("closed store accepted durable event")
	}
	if s.appended.Load() != 0 || s.Healthy() || s.Flush() == nil {
		t.Fatalf("closed admission/health: %+v", s.DurabilityStatus())
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if journalEventCount(t, reopened) != 0 {
		t.Fatal("post-close event was accepted")
	}
}

func TestJournalCrashHelper(t *testing.T) {
	mode := os.Getenv("PULSE_EVENT_JOURNAL_CRASH_MODE")
	if mode == "" {
		return
	}
	s, err := Open(os.Getenv("PULSE_EVENT_JOURNAL_CRASH_DIR"))
	if err != nil {
		t.Fatal(err)
	}
	// Stop the background replay lease so this test process controls the exact
	// durable boundary before the parent kills it. No production fault switch.
	s.applyMu.Lock()
	at := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	seq, err := s.journalEvents([]Event{{Type: TypeFired, AlertID: "crash", OccurredAt: at, Snapshot: []byte(`{"id":"crash","startTime":"2026-11-05T10:00:00Z"}`)}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if mode == "committed" {
		if err := s.insertJournalBatch(s.journal.pending[0].Events, fmt.Sprintf("%s:%d", s.journal.state.ID, seq)); err != nil {
			t.Fatal(err)
		}
	}
	fmt.Println("JOURNAL_CRASH_BOUNDARY_READY")
	select {} // parent sends a real process kill after observing the synced boundary
}

func TestJournalCrashBoundariesRecoverOnceAndReportUnknownTail(t *testing.T) {
	for _, mode := range []string{"journalled", "committed"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			cmd := exec.Command(os.Args[0], "-test.run=^TestJournalCrashHelper$", "-test.v")
			cmd.Env = append(os.Environ(), "PULSE_EVENT_JOURNAL_CRASH_MODE="+mode, "PULSE_EVENT_JOURNAL_CRASH_DIR="+dir)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stderr = cmd.Stdout
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
			ready := make(chan string, 1)
			go func() {
				scanner := bufio.NewScanner(stdout)
				var output strings.Builder
				for scanner.Scan() {
					line := scanner.Text()
					output.WriteString(line + "\n")
					if line == "JOURNAL_CRASH_BOUNDARY_READY" {
						ready <- ""
						return
					}
				}
				ready <- output.String()
			}()
			select {
			case failure := <-ready:
				if failure != "" {
					t.Fatalf("child did not reach boundary: %s", failure)
				}
			case <-time.After(20 * time.Second):
				t.Fatal("child boundary timeout")
			}
			if err := cmd.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			_ = cmd.Wait()
			s, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if journalEventCount(t, s) != 1 {
				t.Fatalf("%s crash recovery lost or duplicated fact", mode)
			}
			status := s.DurabilityStatus()
			if !status.UnknownGap || s.Healthy() || s.Flush() == nil {
				t.Fatalf("unclean tail was concealed: %+v", status)
			}
			var revision int64
			if err := s.db.QueryRow(`SELECT value FROM alert_store_meta WHERE key=?`, activeStateRevisionKey).Scan(&revision); err != nil || revision != 1 {
				t.Fatalf("active-state transaction replayed: %d %v", revision, err)
			}
			rows, err := s.Query(Filter{AlertID: "crash"})
			if err != nil || len(rows) != 1 || !rows[0].OccurredAt.Equal(time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)) {
				t.Fatalf("original time lost: %+v %v", rows, err)
			}
			if err := s.AcknowledgeGaps(status.GapRevision, "accepted unknown pre-journal tail after process crash"); err != nil {
				t.Fatal(err)
			}
			if !s.Healthy() {
				t.Fatalf("recovered journal remains unhealthy after explicit gap disposition: %+v", s.DurabilityStatus())
			}
		})
	}
}

func TestJournalCorruptionRetainsValidPrefixAndQuarantinesTail(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	writerQueryOnly(t, s, true)
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "valid"}); err == nil {
		t.Fatal("write fault did not fail")
	}
	s.Close()
	path := filepath.Join(dir, journalDirectory, journalFileName)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := append(append([]byte(nil), original...), []byte("incomplete fact")...)
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if journalEventCount(t, s) != 1 || !s.DurabilityStatus().UnknownGap || s.Flush() == nil {
		t.Fatalf("corrupt tail hid valid prefix or gap: %+v", s.DurabilityStatus())
	}
	after, err := os.ReadFile(filepath.Join(dir, journalDirectory, journalDamageName))
	if err != nil || !reflect.DeepEqual(after, corrupt) {
		t.Fatal("corrupt evidence was not preserved")
	}
	if err := s.AcknowledgeGaps(s.DurabilityStatus().GapRevision, "reviewed and accepted corrupt tail"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, journalDirectory, journalDamageName)); !os.IsNotExist(err) {
		t.Fatal("acknowledged damaged artifact was not removed")
	}
}

func TestJournalAndGapPersistenceFailureLeavesOnlyProvenUnknownEvidence(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AppendDurable(Event{Type: TypeNotificationDispatched, AlertID: "retained-prefix"}); err != nil {
		t.Fatal(err)
	}
	// Real descriptor failure plus an unavailable journal directory. Neither the
	// offered fact nor the exact gap count can be persisted. The earlier synced
	// open-session marker is the only durable evidence we may claim on reopen.
	if err := s.journal.file.Close(); err != nil {
		t.Fatal(err)
	}
	original := s.journal.dir
	moved := original + "-unavailable"
	if err := os.Rename(original, moved); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "unjournalled"}); err == nil {
		t.Fatal("journal fault returned success")
	}
	status := s.DurabilityStatus()
	if status.Error == "" || !status.UnknownGap || s.Healthy() || s.Flush() == nil {
		t.Fatalf("unjournalled fault hidden: %+v", status)
	}
	s.Close() // clean-close metadata write also cannot succeed
	if err := os.Rename(moved, original); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	status = s.DurabilityStatus()
	if !status.UnknownGap || status.LostEvents != 0 || s.Healthy() || s.Flush() == nil || journalEventCount(t, s) != 1 {
		t.Fatalf("reopen invented exact loss or concealed unknown tail: %+v", status)
	}
}

func TestJournalChecksumFaultReportsUnknownMissingFacts(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	writerQueryOnly(t, s, true)
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "checksum"}); err == nil {
		t.Fatal("write fault did not fail")
	}
	s.Close()
	path := filepath.Join(dir, journalDirectory, journalFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] ^= 1
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if !s.DurabilityStatus().UnknownGap || s.Flush() == nil || journalEventCount(t, s) != 0 {
		t.Fatalf("checksum corruption reported healthy: %+v", s.DurabilityStatus())
	}
}

func TestJournalDiagnosticsCannotConsumeLifecycleReserve(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	writerQueryOnly(t, s, true)
	events := make([]Event, maxDiagnosticEvents)
	for i := range events {
		events[i] = Event{Type: TypeNotificationDispatched, AlertID: fmt.Sprint(i)}
	}
	if _, err := s.journalEvents(events, true); err != nil {
		t.Fatal(err)
	}
	s.Append(Event{Type: TypeNotificationDispatched, AlertID: "diagnostic-over-reserve"})
	if s.Dropped() != 1 || s.DurabilityStatus().LostEvents != 1 {
		t.Fatalf("diagnostic reservation not enforced: %+v", s.DurabilityStatus())
	}
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "lifecycle-reserve"}); err == nil {
		t.Fatal("SQLite fault disappeared")
	}
	if got := s.DurabilityStatus(); got.PendingEvents != maxDiagnosticEvents+1 {
		t.Fatalf("diagnostics starved lifecycle admission: %+v", got)
	}
	writerQueryOnly(t, s, false)
	if err := s.Settle(); err != nil {
		t.Fatal(err)
	}
	if rows, err := s.Query(Filter{AlertID: "lifecycle-reserve"}); err != nil || len(rows) != 1 {
		t.Fatalf("retained lifecycle missing: %+v %v", rows, err)
	}
	if s.Flush() == nil {
		t.Fatal("diagnostic gap concealed by lifecycle recovery")
	}
}

func TestJournalRepeatedDamageRetainsPrefixAndBoundedEvidence(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "valid-prefix"}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	path := filepath.Join(dir, journalDirectory, journalFileName)
	var first []byte
	for i := 0; i < 2; i++ {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, []byte(fmt.Sprintf("different torn tail %d", i))...)
		if i == 0 {
			first = append([]byte(nil), data...)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		s, err = Open(dir)
		if err != nil {
			t.Fatalf("damage %d disabled journal owner: %v", i, err)
		}
		if journalEventCount(t, s) != i+1 || !s.DurabilityStatus().UnknownGap || s.DurabilityStatus().DamageCount != uint64(i+1) {
			t.Fatalf("damage inventory: %+v", s.DurabilityStatus())
		}
		if err := s.AppendDurable(Event{Type: TypeNotificationDispatched, AlertID: fmt.Sprintf("after-damage-%d", i)}); err != nil {
			t.Fatal(err)
		}
		if s.Flush() == nil {
			t.Fatal("new success concealed damage")
		}
		if got, err := os.ReadFile(filepath.Join(dir, journalDirectory, journalDamageName)); err != nil || !reflect.DeepEqual(got, first) {
			t.Fatal("later damage overwrote first forensic artifact")
		}
		s.Close()
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.DurabilityStatus().DamageCount != 2 {
		t.Fatalf("damage count forgotten: %+v", s.DurabilityStatus())
	}
	if err := s.AcknowledgeGaps(s.DurabilityStatus().GapRevision, "accepted two explicitly inventoried damaged tails"); err != nil {
		t.Fatal(err)
	}
	if s.DurabilityStatus().AcknowledgedDamageCount != 2 {
		t.Fatal("damage acknowledgement audit missing")
	}
}

func TestJournalCrashLeftTemporaryFilesAreReclaimed(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	path := filepath.Join(dir, journalDirectory, ".pending-interrupted-compact")
	if err := os.WriteFile(path, []byte("orphan replacement; original log remains authoritative"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("orphan compaction payload escaped retention bound")
	}
}

func TestJournalAcknowledgementPersistenceFailurePreservesArtifact(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.journalMu.Lock()
	s.journal.state.Unknown = true
	s.journal.state.GapRevision++
	if err := s.journal.saveState(); err != nil {
		t.Fatal(err)
	}
	s.journalMu.Unlock()
	artifact := filepath.Join(s.journal.dir, journalDamageName)
	if err := os.WriteFile(artifact, []byte("forensic evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(s.journal.dir, "state.json")
	original, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(statePath, 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.AcknowledgeGaps(s.DurabilityStatus().GapRevision, "acknowledgement must persist first"); err == nil {
		t.Fatal("failed disposition returned success")
	}
	if _, err := os.Stat(artifact); err != nil {
		t.Fatal("unrecorded acknowledgement deleted evidence")
	}
	if !s.DurabilityStatus().UnknownGap {
		t.Fatal("failed acknowledgement cleared unknown gap")
	}
	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, original, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestJournalCloseDrainsAdmittedOperationsAndReportsClosingRefusals(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := s.db.Conn(context.Background())
	if err != nil {
		s.Close()
		t.Fatal(err)
	}
	admitted := make(chan error, 1)
	go func() { admitted <- s.AppendDurable(Event{Type: TypeFired, AlertID: "before-close"}) }()
	deadline := time.Now().Add(10 * time.Second)
	for s.DurabilityStatus().PendingEvents != 1 {
		if time.Now().After(deadline) {
			conn.Close()
			t.Fatal("admission did not reach journal")
		}
		time.Sleep(time.Millisecond)
	}
	closed := make(chan struct{})
	go func() { s.Close(); close(closed) }()
	for !s.DurabilityStatus().Closed {
		if time.Now().After(deadline) {
			conn.Close()
			t.Fatal("close did not revoke admission")
		}
		time.Sleep(time.Millisecond)
	}
	if err := s.AppendDurable(Event{Type: TypeResolved, AlertID: "during-close"}); err == nil {
		conn.Close()
		t.Fatal("closing store accepted event")
	}
	s.Append(Event{Type: TypeNotificationDispatched, AlertID: "diagnostic-during-close"})
	conn.Close()
	if err := <-admitted; err != nil {
		t.Fatalf("pre-close admitted operation lost its owner: %v", err)
	}
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("close leaked an admitted operation")
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if journalEventCount(t, s) != 1 || s.DurabilityStatus().LostEvents != 2 || s.Flush() == nil {
		t.Fatalf("closing gap forgotten: %+v", s.DurabilityStatus())
	}
}

func TestJournalImportAdmissionRefusalDoesNotInventMissingFacts(t *testing.T) {
	s := newTestStore(t)
	event := Event{Type: TypeHistoryImported, AlertID: "caller-retains-source", Snapshot: []byte("invalid json")}
	if err := s.ImportEvents([]Event{event}); err == nil {
		t.Fatal("invalid import accepted")
	}
	if s.DurabilityStatus().LostEvents != 0 || s.DurabilityStatus().UnknownGap {
		t.Fatalf("retained caller source labelled lost: %+v", s.DurabilityStatus())
	}
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "later"}); err != nil {
		t.Fatal(err)
	}
	if s.Flush() != nil {
		t.Fatal("refused import poisoned completeness")
	}
}

func TestJournalDatabaseBootstrapFailureKeepsIndependentAdmission(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	target := filepath.Join(dir, eventLogFileName)
	saved := target + ".preserved"
	if err := os.Rename(target, saved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatalf("unavailable target disabled independent journal: %v", err)
	}
	at := time.Now().Add(-time.Hour).UTC()
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "during-bootstrap-fault", OccurredAt: at}); err == nil {
		t.Fatal("unavailable database reported commit")
	}
	if s.DurabilityStatus().PendingEvents != 1 || s.Healthy() {
		t.Fatalf("bootstrap fault admission: %+v", s.DurabilityStatus())
	}
	s.Close()
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(saved, target); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.Query(Filter{AlertID: "during-bootstrap-fault"})
	if err != nil || len(rows) != 1 || !rows[0].OccurredAt.Equal(at) || s.Flush() != nil {
		t.Fatalf("bootstrap-fault obligation lost: %+v %v %+v", rows, err, s.DurabilityStatus())
	}
}

func TestJournalWriteFailureRecoversWithoutRestart(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "before"}); err != nil {
		t.Fatal(err)
	}
	s.journalMu.Lock()
	if err := s.journal.file.Close(); err != nil {
		t.Fatal(err)
	}
	s.journalMu.Unlock()
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "uncertain"}); err == nil {
		t.Fatal("descriptor fault was hidden")
	}
	if err := s.Settle(); err != nil {
		t.Fatalf("transient journal fault poisoned settlement: %v", err)
	}
	if status := s.DurabilityStatus(); status.Error != "" || !status.UnknownGap || s.Healthy() {
		t.Fatalf("recovery concealed uncertainty: %+v", status)
	}
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "after"}); err != nil {
		t.Fatalf("admission did not recover: %v", err)
	}
	if rows, err := s.Query(Filter{}); err != nil || len(rows) != 2 {
		t.Fatalf("retained rows unavailable: %+v %v", rows, err)
	}
	if s.Flush() == nil {
		t.Fatal("successful recovery cleared gap")
	}
	if err := s.AcknowledgeGaps(s.DurabilityStatus().GapRevision, "accepted uncertain descriptor failure"); err != nil {
		t.Fatal(err)
	}
	if !s.Healthy() {
		t.Fatalf("acknowledged recovery unhealthy: %+v", s.DurabilityStatus())
	}
}

func TestJournalImportRetryReusesPendingObligation(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER import_fault BEFORE INSERT ON alert_events BEGIN SELECT RAISE(ABORT,'continuing fault'); END`); err != nil {
		t.Fatal(err)
	}
	events := []Event{{Type: TypeHistoryImported, AlertID: "legacy", OccurredAt: time.Now().UTC(), Snapshot: []byte(`{"id":"legacy"}`)}}
	for boot := 0; boot < 3; boot++ {
		if err := s.ImportEvents(events); err == nil {
			t.Fatal("target fault hidden")
		}
		if status := s.DurabilityStatus(); status.PendingEvents != 1 || status.LostEvents != 0 {
			t.Fatalf("retry consumed lifecycle reserve: %+v", status)
		}
		s.Close()
		s, err = Open(dir)
		if err != nil {
			t.Fatal(err)
		}
	}
	defer s.Close()
	if _, err := s.db.Exec(`DROP TRIGGER import_fault`); err != nil {
		t.Fatal(err)
	}
	if err := s.ImportEvents(events); err != nil {
		t.Fatal(err)
	}
	if count := journalEventCount(t, s); count != 1 {
		t.Fatalf("import duplicated: %d", count)
	}
}

func TestJournalUnreadableCursorRetainsPhysicalFramesBeyondAdmissionCap(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	// A physical file may include both committed and pending frames. When its
	// target cursor cannot be read, the byte bound remains provable but its
	// pending event count is deliberately conservative rather than an open error.
	var data []byte
	at := time.Now().UTC()
	for i := 1; i <= maxJournalEvents+1; i++ {
		frame, err := encodeJournalRecord(journalRecord{Sequence: int64(i), Events: []Event{{Type: TypeFired, AlertID: fmt.Sprint(i), OccurredAt: at}}})
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, frame...)
	}
	if err := os.WriteFile(filepath.Join(dir, journalDirectory, journalFileName), data, 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, eventLogFileName)
	if err := os.Rename(target, target+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatalf("uncertain cursor disabled retained journal: %v", err)
	}
	if status := s.DurabilityStatus(); status.PendingEvents != maxJournalEvents+1 || status.Error == "" {
		t.Fatalf("uncertain prefix lost: %+v", status)
	}
	s.Close()
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(target+".saved", target); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if count := journalEventCount(t, s); count != maxJournalEvents+1 {
		t.Fatalf("retained prefix lost: %d", count)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
}

func TestJournalStartupDoesNotRequirePendingPayloadCompaction(t *testing.T) {
	const child = "PULSE_TEST_JOURNAL_LOW_FILE_LIMIT"
	if dir := os.Getenv(child); dir != "" {
		s, err := Open(dir)
		if err != nil {
			t.Fatalf("startup cleanup disabled journal under bounded write budget: %v", err)
		}
		defer s.Close()
		if got := s.DurabilityStatus(); got.PendingEvents != 1 {
			t.Fatalf("pending fact lost at constrained startup: %+v", got)
		}
		return
	}
	if runtime.GOOS == "windows" {
		t.Skip("POSIX child file-size limit")
	}
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER startup_fault BEFORE INSERT ON alert_events BEGIN SELECT RAISE(ABORT,'target fault'); END`); err != nil {
		t.Fatal(err)
	}
	snapshot := []byte(`{"id":"large-pending","payload":"` + strings.Repeat("x", 128<<10) + `"}`)
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "large-pending", Snapshot: snapshot}); err == nil {
		t.Fatal("trigger fault hidden")
	}
	s.Close()
	// The inherited file-size budget permits metadata but cannot copy the pending
	// frame. This was a real EFBIG open failure when startup always compacted.
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", `ulimit -f 64; exec "$@"`, "history-startup-budget", binary, "-test.run=^TestJournalStartupDoesNotRequirePendingPayloadCompaction$", "-test.count=1")
	cmd.Env = append(os.Environ(), child+"="+dir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("constrained restart: %v\n%s", err, output)
	}
}

func TestJournalOrphanCleanupFailureRetainsOwnerAndRetries(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER orphan_fault BEFORE INSERT ON alert_events BEGIN SELECT RAISE(ABORT,'target fault'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "retained"}); err == nil {
		t.Fatal("target fault hidden")
	}
	s.Close()
	orphan := filepath.Join(dir, journalDirectory, ".pending-blocker")
	if err := os.Mkdir(orphan, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orphan, "busy"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatalf("orphan cleanup disabled owner: %v", err)
	}
	defer s.Close()
	if _, err := s.db.Exec(`DROP TRIGGER orphan_fault`); err != nil {
		t.Fatal(err)
	}
	if err := s.Settle(); err == nil {
		t.Fatal("blocked cleanup hidden")
	}
	if journalEventCount(t, s) != 1 {
		t.Fatal("cleanup failure prevented retained replay")
	}
	if err := os.RemoveAll(orphan); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if !s.Healthy() {
		t.Fatalf("cleanup retry did not recover: %+v", s.DurabilityStatus())
	}
}

func TestJournalOpenSessionMetadataFailureKeepsRetryingOwner(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("POSIX non-root directory permission fault")
	}
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	journalDir := filepath.Join(dir, journalDirectory)
	if err := os.Chmod(journalDir, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(journalDir, 0700)
	s, err = Open(dir)
	if err != nil {
		t.Fatalf("session metadata failure disabled owner: %v", err)
	}
	defer s.Close()
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "refused-before-session-sync"}); err == nil {
		t.Fatal("admitted without session evidence")
	}
	if status := s.DurabilityStatus(); status.LostEvents != 1 || status.GapEvidencePersisted || s.Healthy() {
		t.Fatalf("unsynced refusal hidden: %+v", status)
	}
	if err := os.Chmod(journalDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.Settle(); err != nil {
		t.Fatal(err)
	}
	if status := s.DurabilityStatus(); status.LostEvents != 1 || !status.GapEvidencePersisted {
		t.Fatalf("recovered metadata lost gap: %+v", status)
	}
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "after-session-repair"}); err != nil {
		t.Fatal(err)
	}
	if s.Flush() == nil {
		t.Fatal("metadata repair cleared refused fact")
	}
}

func TestJournalReadOnlyUpgradeKeepsLegacyRowsReadable(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "legacy-readable"}); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{`DROP INDEX idx_alert_events_journal`, `ALTER TABLE alert_events DROP COLUMN journal_id`, `ALTER TABLE alert_events DROP COLUMN journal_sequence`} {
		if _, err := s.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	target := filepath.Join(dir, eventLogFileName)
	s, err = openDSN(target, sqliteDSN(target)+"&_pragma=query_only(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.targetReady {
		t.Fatal("readonly schema migration unexpectedly succeeded")
	}
	rows, err := s.Query(Filter{})
	if err != nil || len(rows) != 1 {
		t.Fatalf("legacy retained rows blocked by origin migration: %+v %v", rows, err)
	}
	for _, walk := range []func(Filter, func(Event) error) error{s.WalkByID, s.WalkOldest} {
		count := 0
		err := walk(Filter{}, func(Event) error { count++; return nil })
		if err != nil || count != 1 {
			t.Fatalf("legacy walk blocked by origin migration: count=%d error=%v", count, err)
		}
	}
}

func TestJournalDelayedControlCannotReactivateResolvedOccurrence(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	snapshot := []byte(`{"id":"terminal-control","startTime":"2026-11-05T00:00:00Z","level":"warning"}`)
	for _, kind := range []string{TypeFired, TypeResolved, TypeAcknowledged, TypeSnoozed, TypeEscalated} {
		if err := s.AppendDurable(Event{Type: kind, AlertID: "terminal-control", Snapshot: snapshot}); err != nil {
			t.Fatal(err)
		}
	}
	active, err := s.LoadActiveState()
	if err != nil || len(active) != 0 {
		t.Fatalf("delayed control reactivated terminal occurrence: %+v %v", active, err)
	}
	if count := journalEventCount(t, s); count != 5 {
		t.Fatalf("historical control facts were dropped: %d", count)
	}
}

func TestJournalNullLifecycleSnapshotIsRefusedWithoutPoison(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "non-object", Snapshot: []byte("null")}); err == nil {
		t.Fatal("null lifecycle state admitted")
	}
	if status := s.DurabilityStatus(); status.LostEvents != 1 || status.PendingEvents != 0 {
		t.Fatalf("invalid state obligation hidden: %+v", status)
	}
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "valid-after-null", Snapshot: []byte(`{"id":"valid-after-null"}`)}); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendDurable(Event{Type: TypeAcknowledged, AlertID: "valid-after-null", Snapshot: []byte(`{"id":"valid-after-null","acknowledged":true}`)}); err != nil {
		t.Fatal(err)
	}
	if journalEventCount(t, s) != 2 || s.Flush() == nil {
		t.Fatal("valid later work was poisoned or refused state gap concealed")
	}
}

func TestJournalActivationDoesNotProveCompleteActiveBaseline(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.AppendDurable(Event{Type: TypeFired, AlertID: "isolated", Snapshot: []byte(`{"id":"isolated"}`)}); err != nil {
		t.Fatal(err)
	}
	if complete, err := s.ActiveCheckpointComplete(); err != nil || complete {
		t.Fatalf("one occurrence attested a complete baseline: %t %v", complete, err)
	}
	snapshots, err := s.LoadActiveState()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceActiveState(snapshots); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if complete, err := s.ActiveCheckpointComplete(); err != nil || !complete {
		t.Fatalf("complete checkpoint declaration did not survive: %t %v", complete, err)
	}
}

func TestJournalLegacyActiveAuthorityMigratesBeforeReplay(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceActiveState(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM alert_store_meta WHERE key IN (?,?)`, activeCheckpointCompleteKey, activeCheckpointProtocolKey); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if complete, err := s.ActiveCheckpointComplete(); err != nil || !complete {
		t.Fatalf("legacy declared authority lost on migration: %t %v", complete, err)
	}
}
