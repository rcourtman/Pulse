package operationreceipt

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testKind = "test.safe_result"

func testConfig() Config {
	return Config{
		RecentTerminalTTL:     time.Hour,
		MaxRecentPayloadBytes: 1 << 20,
		Validators: map[string]map[int]TerminalValidator{
			testKind: {1: func(_ Identity, raw json.RawMessage) error {
				var value struct {
					Status string `json:"status"`
				}
				return decodeStrict(raw, &value)
			}},
		},
	}
}
func testIdentity(id string) Identity {
	digest, _ := DigestCanonicalJSON(map[string]string{"id": id})
	return Identity{AttemptID: id, ActionID: "action-" + id, OperationKind: "test.operation", OperationVersion: 1, RequestDigest: digest, AgentID: "agent-1"}
}
func testEnvelope(status string) TerminalEnvelope {
	raw, _ := json.Marshal(struct {
		Status string `json:"status"`
	}{status})
	return TerminalEnvelope{Kind: testKind, Version: 1, Payload: raw}
}
func openTestStore(t *testing.T, path string, config Config) *Store {
	t.Helper()
	s, err := Open(path, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSQLiteOperationReceiptDuplicateBindingAndTerminalReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipts.db")
	s := openTestStore(t, path, testConfig())
	id := testIdentity("attempt-1")
	if _, fresh, err := s.Admit(id); err != nil || !fresh {
		t.Fatalf("admit fresh=%v err=%v", fresh, err)
	}
	if _, fresh, err := s.Admit(id); err != nil || fresh {
		t.Fatalf("duplicate fresh=%v err=%v", fresh, err)
	}
	changed := id
	changed.ActionID = "other"
	if _, _, err := s.Admit(changed); !errors.Is(err, ErrBindingConflict) {
		t.Fatalf("mismatch err=%v", err)
	}
	if _, err := s.MarkStarted(id); err != nil {
		t.Fatal(err)
	}
	terminal, err := s.Complete(id, testEnvelope("ok"))
	if err != nil {
		t.Fatal(err)
	}
	if terminal.State != StateTerminal {
		t.Fatalf("state=%s", terminal.State)
	}
	replay, err := s.Complete(id, testEnvelope("ok"))
	if err != nil || string(replay.Result) != string(terminal.Result) {
		t.Fatalf("replay=%s err=%v", replay.Result, err)
	}
	if _, err := s.Complete(id, testEnvelope("different")); !errors.Is(err, ErrNotCompletable) {
		t.Fatalf("conflicting terminal err=%v", err)
	}
}

func TestSQLiteOperationReceiptCrashBoundariesAndReopen(t *testing.T) {
	for _, tc := range []struct {
		name            string
		start, complete bool
		want            State
	}{{"after_admit", false, false, StateInterrupted}, {"after_start_before_result", true, false, StateInterrupted}, {"after_terminal", true, true, StateTerminal}} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "receipts.db")
			s := openTestStore(t, path, testConfig())
			id := testIdentity(tc.name)
			if _, _, err := s.Admit(id); err != nil {
				t.Fatal(err)
			}
			if tc.start {
				if _, err := s.MarkStarted(id); err != nil {
					t.Fatal(err)
				}
			}
			if tc.complete {
				if _, err := s.Complete(id, testEnvelope("ok")); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			s2, err := Open(path, testConfig())
			if err != nil {
				t.Fatal(err)
			}
			defer s2.Close()
			q, err := s2.Query(id)
			if err != nil {
				t.Fatal(err)
			}
			if q.Record.State != tc.want {
				t.Fatalf("state=%s want=%s", q.Record.State, tc.want)
			}
			if tc.complete && string(q.Record.Result) != (string(testEnvelope("ok").Payload)) {
				t.Fatalf("terminal replay=%s", q.Record.Result)
			}
			if _, fresh, err := s2.Admit(id); err != nil || fresh {
				t.Fatalf("reopen duplicate fresh=%v err=%v", fresh, err)
			}
		})
	}
}

func TestSQLiteOperationReceiptCompletionRollbackLeavesUnknown(t *testing.T) {
	s := openTestStore(t, filepath.Join(t.TempDir(), "receipts.db"), testConfig())
	id := testIdentity("rollback")
	_, _, _ = s.Admit(id)
	_, _ = s.MarkStarted(id)
	if _, err := s.db.Exec(`CREATE TRIGGER fail_terminal BEFORE UPDATE ON operation_receipts WHEN NEW.state='terminal' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Complete(id, testEnvelope("ok")); err == nil {
		t.Fatal("expected injected failure")
	}
	q, err := s.Query(id)
	if err != nil || q.Status != QueryFoundInterrupted || q.Record.State != StateStarted {
		t.Fatalf("query=%+v err=%v", q, err)
	}
}

func TestSQLiteOperationReceiptCompactionRetainsReplayDenialAndCapacityForNewWork(t *testing.T) {
	cfg := testConfig()
	cfg.MaxRecentPayloadBytes = 20
	path := filepath.Join(t.TempDir(), "receipts.db")
	s := openTestStore(t, path, cfg)
	first := testIdentity("old")
	_, _, _ = s.Admit(first)
	_, _ = s.MarkStarted(first)
	_, err := s.Complete(first, testEnvelope("payload-large-enough"))
	if err != nil {
		t.Fatal(err)
	}
	q, err := s.Query(first)
	if err != nil || q.Record.State != StateTombstone || q.Status != QueryFoundInterrupted {
		t.Fatalf("compacted query=%+v err=%v", q, err)
	}
	if _, fresh, err := s.Admit(first); err != nil || fresh {
		t.Fatalf("tombstone replay fresh=%v err=%v", fresh, err)
	}
	for n := 0; n < 500; n++ {
		id := testIdentity("volume-" + time.Unix(int64(n), 0).Format("150405.000000000"))
		if _, fresh, err := s.Admit(id); err != nil || !fresh {
			t.Fatalf("volume %d fresh=%v err=%v", n, fresh, err)
		}
	}
	unrelated := testIdentity("unrelated")
	if _, fresh, err := s.Admit(unrelated); err != nil || !fresh {
		t.Fatalf("unrelated fresh=%v err=%v", fresh, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	q, err = reopened.Query(first)
	if err != nil || q.Record.State != StateTombstone {
		t.Fatalf("reopened tombstone=%+v err=%v", q, err)
	}
}

func TestSQLiteOperationReceiptTTLCompactionRetainsIdentityDigestTombstone(t *testing.T) {
	cfg := testConfig()
	cfg.RecentTerminalTTL = time.Minute
	s := openTestStore(t, filepath.Join(t.TempDir(), "receipts.db"), cfg)
	base := time.Date(2026, 7, 12, 1, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return base }
	id := testIdentity("ttl")
	_, _, _ = s.Admit(id)
	_, _ = s.MarkStarted(id)
	_, _ = s.Complete(id, testEnvelope("ok"))
	s.now = func() time.Time { return base.Add(2 * time.Minute) }
	if err := s.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
	q, err := s.Query(id)
	if err != nil || q.Record.State != StateTombstone || q.Record.Identity != id || len(q.Record.Result) != 0 {
		t.Fatalf("query=%+v err=%v", q, err)
	}
}

func TestSQLiteOperationReceiptConcurrentDuplicateHasOneAdmission(t *testing.T) {
	s := openTestStore(t, filepath.Join(t.TempDir(), "receipts.db"), testConfig())
	id := testIdentity("concurrent")
	var admitted atomic.Int32
	var wg sync.WaitGroup
	for n := 0; n < 32; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, fresh, err := s.Admit(id)
			if err != nil {
				t.Errorf("admit: %v", err)
				return
			}
			if fresh {
				admitted.Add(1)
			}
		}()
	}
	wg.Wait()
	if admitted.Load() != 1 {
		t.Fatalf("admissions=%d", admitted.Load())
	}
}

func TestSQLiteOperationReceiptConcurrentLifecycleAndCompaction(t *testing.T) {
	cfg := testConfig()
	cfg.MaxRecentPayloadBytes = 256
	s := openTestStore(t, filepath.Join(t.TempDir(), "receipts.db"), cfg)
	var wg sync.WaitGroup
	for n := 0; n < 64; n++ {
		n := n
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := testIdentity("parallel-" + time.Unix(int64(n), 0).Format("150405.000000000"))
			if _, fresh, err := s.Admit(id); err != nil || !fresh {
				t.Errorf("admit %d fresh=%v err=%v", n, fresh, err)
				return
			}
			if _, err := s.MarkStarted(id); err != nil {
				t.Errorf("start %d: %v", n, err)
				return
			}
			if _, err := s.Complete(id, testEnvelope("ok")); err != nil {
				t.Errorf("complete %d: %v", n, err)
			}
		}()
	}
	for n := 0; n < 16; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Compact(context.Background()); err != nil {
				t.Errorf("compact: %v", err)
			}
		}()
	}
	wg.Wait()
	for n := 0; n < 64; n++ {
		id := testIdentity("parallel-" + time.Unix(int64(n), 0).Format("150405.000000000"))
		q, err := s.Query(id)
		if err != nil || q.Status == QueryNotFound {
			t.Fatalf("query %d=%+v err=%v", n, q, err)
		}
	}
}

func TestSQLiteOperationReceiptCorruptionAndEnvelopeRefusal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "receipts.db")
	if err := os.WriteFile(path, []byte("truncated-not-sqlite"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path, testConfig()); !errors.Is(err, ErrStoreCorrupt) {
		t.Fatalf("corrupt open err=%v", err)
	}
	s := openTestStore(t, filepath.Join(dir, "valid.db"), testConfig())
	id := testIdentity("bad-envelope")
	_, _, _ = s.Admit(id)
	_, _ = s.MarkStarted(id)
	if _, err := s.Complete(id, TerminalEnvelope{Kind: "unknown", Version: 1, Payload: json.RawMessage(`{"status":"ok"}`)}); err == nil {
		t.Fatal("unknown kind accepted")
	}
	if _, err := s.Complete(id, TerminalEnvelope{Kind: testKind, Version: 2, Payload: json.RawMessage(`{"status":"ok"}`)}); err == nil {
		t.Fatal("unknown version accepted")
	}
	if _, err := s.Complete(id, TerminalEnvelope{Kind: testKind, Version: 1, Payload: json.RawMessage(`{"status":"ok","stderr":"secret"}`)}); err == nil {
		t.Fatal("unknown result field accepted")
	}
}

func TestSQLiteOperationReceiptRevalidatesFrozenTerminalCodecOnReadAndReopen(t *testing.T) {
	cfg := testConfig()
	source := cfg.Validators
	path := filepath.Join(t.TempDir(), "receipts.db")
	s := openTestStore(t, path, cfg)
	id := testIdentity("tamper")
	_, _, _ = s.Admit(id)
	_, _ = s.MarkStarted(id)
	_, _ = s.Complete(id, testEnvelope("ok"))
	// Mutating the caller's registry after Open must not alter store policy.
	source[testKind][1] = func(Identity, json.RawMessage) error { return nil }
	if _, err := s.db.Exec(`UPDATE operation_receipts SET result_json=? WHERE attempt_id=?`, []byte(`{"status":"ok","stderr":"secret"}`), id.AttemptID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Query(id); !errors.Is(err, ErrStoreCorrupt) {
		t.Fatalf("tampered query err=%v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path, testConfig()); !errors.Is(err, ErrStoreCorrupt) {
		t.Fatalf("tampered reopen err=%v", err)
	}
}

func TestOperationReceiptQueryCodecRejectsMalformedUnknownAndTrailing(t *testing.T) {
	id := testIdentity("codec")
	record := Record{Identity: id, State: StateInterrupted, AcceptedAt: time.Now().UTC(), StartedAt: time.Now().UTC()}
	valid, _ := json.Marshal(QueryResult{Version: ProtocolVersion, Status: QueryFoundInterrupted, Record: &record})
	for _, raw := range [][]byte{[]byte(`{"version":99,"status":"not_found"}`), append(valid, []byte(` {}`)...), []byte(`{"version":1,"status":"not_found","unknown":true}`), []byte(`{"version":1,"status":"found_terminal","record":{"identity":{}}}`)} {
		if _, err := DecodeQueryResult(raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if _, err := DecodeQueryResult(valid); err != nil {
		t.Fatal(err)
	}
}

func TestSQLiteOperationReceiptNotFoundIsNonAuthorizingState(t *testing.T) {
	s := openTestStore(t, filepath.Join(t.TempDir(), "receipts.db"), testConfig())
	q, err := s.Query(testIdentity("missing"))
	if err != nil || q.Status != QueryNotFound || q.Record != nil {
		t.Fatalf("query=%+v err=%v", q, err)
	}
	if err := s.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
}
