package operationreceipt

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// This file exercises the previously uncovered branches of DecodeQuery and
// DecodeQueryResult in internal/operationreceipt/types.go: every error arm
// of decodeStrict, the version guards, the per-status switch arms of
// DecodeQueryResult (including the default), both sides of each record
// nil/state conditional, and ValidateRecord invocation on the non-nil path.

// withRogueField injects an unknown top-level JSON field into a marshalled
// object so we can exercise decodeStrict's DisallowUnknownFields branch
// without hand-rolling a long valid payload.
func withRogueField(t *testing.T, valid []byte) []byte {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(valid, &obj); err != nil {
		t.Fatalf("unmarshal for rogue injection: %v", err)
	}
	obj["rogue_field"] = json.RawMessage(`true`)
	out, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal for rogue injection: %v", err)
	}
	return out
}

func TestBranchCovDecodeQuery(t *testing.T) {
	validID := testIdentity("decode-query")
	validQueryBytes, err := json.Marshal(Query{Version: ProtocolVersion, Identity: validID})
	if err != nil {
		t.Fatalf("marshal valid query: %v", err)
	}

	// Identity whose string fields carry surrounding whitespace; after
	// DecodeQuery runs NormalizeIdentity the result must equal validID.
	spaceyID := validID
	spaceyID.AttemptID = "  " + validID.AttemptID + "\t"
	spaceyID.ActionID = " " + validID.ActionID + " "
	spaceyID.OperationKind = "\t" + validID.OperationKind + " "
	spaceyID.RequestDigest = "  " + validID.RequestDigest + "  "
	spaceyID.AgentID = " " + validID.AgentID + " "
	spaceyQueryBytes, err := json.Marshal(Query{Version: ProtocolVersion, Identity: spaceyID})
	if err != nil {
		t.Fatalf("marshal spacey query: %v", err)
	}

	badVersionBytes, err := json.Marshal(Query{Version: 99, Identity: validID})
	if err != nil {
		t.Fatalf("marshal bad-version query: %v", err)
	}

	emptyIdentityBytes, err := json.Marshal(Query{Version: ProtocolVersion, Identity: Identity{}})
	if err != nil {
		t.Fatalf("marshal empty-identity query: %v", err)
	}

	zeroVersionBytes, err := json.Marshal(Query{Version: ProtocolVersion, Identity: Identity{
		AttemptID: "a", ActionID: "b", OperationKind: "k", OperationVersion: 0,
		RequestDigest: validID.RequestDigest, AgentID: "g",
	}})
	if err != nil {
		t.Fatalf("marshal zero-version query: %v", err)
	}

	trailingBytes := append(append([]byte{}, validQueryBytes...), []byte(` {"after":true}`)...)

	cases := []struct {
		name      string
		data      []byte
		wantOK    bool
		errSub    string // asserted (as substring) only when !wantOK
		wantID    Identity
		checkTrim bool // on success, assert identity equals wantID (post-trim)
	}{
		{name: "nil payload rejected", data: nil, wantOK: false, errSub: "payload is empty"},
		{name: "whitespace-only payload rejected", data: []byte("   \n\t "), wantOK: false, errSub: "payload is empty"},
		{name: "malformed JSON rejected", data: []byte(`{"version":1`), wantOK: false},
		{name: "trailing JSON rejected", data: trailingBytes, wantOK: false, errSub: "trailing"},
		{name: "unknown top-level field rejected", data: withRogueField(t, validQueryBytes), wantOK: false, errSub: "unknown field"},
		{name: "unsupported version rejected", data: badVersionBytes, wantOK: false, errSub: "unsupported operation query version 99"},
		{name: "empty identity rejected", data: emptyIdentityBytes, wantOK: false, errSub: "identity fields are required"},
		{name: "non-positive operation version rejected", data: zeroVersionBytes, wantOK: false, errSub: "operation version must be positive"},
		{name: "valid query decodes", data: validQueryBytes, wantOK: true, wantID: validID, checkTrim: true},
		{name: "untrimmed identity fields are normalized", data: spaceyQueryBytes, wantOK: true, wantID: validID, checkTrim: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, err := DecodeQuery(tc.data)
			if !tc.wantOK {
				if err == nil {
					t.Fatalf("expected error, got nil (q=%+v)", q)
				}
				if tc.errSub != "" && !strings.Contains(err.Error(), tc.errSub) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.errSub)
				}
				if q.Version != 0 || q.Identity != (Identity{}) {
					t.Fatalf("expected zero-value Query on error, got %+v", q)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if q.Version != ProtocolVersion {
				t.Fatalf("version=%d want=%d", q.Version, ProtocolVersion)
			}
			if tc.checkTrim && q.Identity != tc.wantID {
				t.Fatalf("identity not normalized/round-tripped:\n got  %+v\n want %+v", q.Identity, tc.wantID)
			}
		})
	}
}

func TestBranchCovDecodeQueryResult(t *testing.T) {
	id := testIdentity("decode-result")
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

	acceptedRec := Record{Identity: id, State: StateAccepted, AcceptedAt: now}
	startedRec := Record{Identity: id, State: StateStarted, AcceptedAt: now, StartedAt: now}
	interruptedRec := Record{Identity: id, State: StateInterrupted, AcceptedAt: now, StartedAt: now}
	tombstoneRec := Record{Identity: id, State: StateTombstone, AcceptedAt: now, StartedAt: now, TerminalAt: now}
	terminalRec := Record{
		Identity:      id,
		State:         StateTerminal,
		AcceptedAt:    now,
		StartedAt:     now,
		TerminalAt:    now,
		ResultKind:    "test.kind",
		ResultVersion: 1,
		Result:        json.RawMessage(`{}`),
	}
	// Record that passes the found_interrupted state check (State==accepted)
	// but fails ValidateRecord because AcceptedAt is zero.
	boundsBadRec := Record{Identity: id, State: StateAccepted}

	marshal := func(status QueryStatus, record *Record) []byte {
		t.Helper()
		data, err := json.Marshal(QueryResult{Version: ProtocolVersion, Status: status, Record: record})
		if err != nil {
			t.Fatalf("marshal result: %v", err)
		}
		return data
	}
	marshalVersion := func(version int, status QueryStatus, record *Record) []byte {
		t.Helper()
		data, err := json.Marshal(QueryResult{Version: version, Status: status, Record: record})
		if err != nil {
			t.Fatalf("marshal result: %v", err)
		}
		return data
	}

	emptyRecord := &Record{}
	cases := []struct {
		name       string
		data       []byte
		wantOK     bool
		errSub     string
		wantStatus QueryStatus
		wantState  State // checked when non-empty and wantOK
		wantNilRec bool
	}{
		// decodeStrict / version branches.
		{name: "empty payload rejected", data: nil, wantOK: false, errSub: "payload is empty"},
		{name: "trailing JSON rejected", data: append(append([]byte{}, marshal(QueryNotFound, nil)...), []byte(" {}")...), wantOK: false, errSub: "trailing"},
		{name: "unknown field rejected", data: withRogueField(t, marshal(QueryNotFound, nil)), wantOK: false, errSub: "unknown field"},
		{name: "unsupported version rejected", data: marshalVersion(7, QueryNotFound, nil), wantOK: false, errSub: "unsupported operation query result version 7"},

		// QueryNotFound arm.
		{name: "not_found without record succeeds", data: marshal(QueryNotFound, nil), wantOK: true, wantStatus: QueryNotFound, wantNilRec: true},
		{name: "not_found with record rejected", data: marshal(QueryNotFound, emptyRecord), wantOK: false, errSub: "not-found query result cannot include a record"},

		// QueryFoundTerminal arm: nil record, wrong-state record, and valid record.
		{name: "found_terminal with nil record rejected", data: marshal(QueryFoundTerminal, nil), wantOK: false, errSub: "terminal query result requires terminal record"},
		{name: "found_terminal with non-terminal record rejected", data: marshal(QueryFoundTerminal, &startedRec), wantOK: false, errSub: "terminal query result requires terminal record"},
		{name: "found_terminal with valid terminal record succeeds", data: marshal(QueryFoundTerminal, &terminalRec), wantOK: true, wantStatus: QueryFoundTerminal, wantState: StateTerminal},

		// QueryFoundInterrupted arm: nil record, wrong-state record, and each
		// accepted state.
		{name: "found_interrupted with nil record rejected", data: marshal(QueryFoundInterrupted, nil), wantOK: false, errSub: "interrupted query result requires nonterminal or tombstone record"},
		{name: "found_interrupted with terminal record rejected", data: marshal(QueryFoundInterrupted, &terminalRec), wantOK: false, errSub: "interrupted query result requires nonterminal or tombstone record"},
		{name: "found_interrupted with accepted record succeeds", data: marshal(QueryFoundInterrupted, &acceptedRec), wantOK: true, wantStatus: QueryFoundInterrupted, wantState: StateAccepted},
		{name: "found_interrupted with started record succeeds", data: marshal(QueryFoundInterrupted, &startedRec), wantOK: true, wantStatus: QueryFoundInterrupted, wantState: StateStarted},
		{name: "found_interrupted with interrupted record succeeds", data: marshal(QueryFoundInterrupted, &interruptedRec), wantOK: true, wantStatus: QueryFoundInterrupted, wantState: StateInterrupted},
		{name: "found_interrupted with tombstone record succeeds", data: marshal(QueryFoundInterrupted, &tombstoneRec), wantOK: true, wantStatus: QueryFoundInterrupted, wantState: StateTombstone},

		// Non-nil record path must still run ValidateRecord; a record that
		// passes the state check but fails validation is rejected.
		{name: "non-nil record failing ValidateRecord rejected", data: marshal(QueryFoundInterrupted, &boundsBadRec), wantOK: false, errSub: "invalid operation receipt bounds"},

		// default arm: unsupported status.
		{name: "unsupported status rejected", data: marshal(QueryStatus("bogus"), nil), wantOK: false, errSub: "unsupported operation query status"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := DecodeQueryResult(tc.data)
			if !tc.wantOK {
				if err == nil {
					t.Fatalf("expected error, got nil (r=%+v)", r)
				}
				if tc.errSub != "" && !strings.Contains(err.Error(), tc.errSub) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.errSub)
				}
				if r.Version != 0 || r.Status != "" || r.Record != nil {
					t.Fatalf("expected zero-value QueryResult on error, got %+v", r)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.Version != ProtocolVersion {
				t.Fatalf("version=%d want=%d", r.Version, ProtocolVersion)
			}
			if r.Status != tc.wantStatus {
				t.Fatalf("status=%q want=%q", r.Status, tc.wantStatus)
			}
			if tc.wantNilRec {
				if r.Record != nil {
					t.Fatalf("expected nil record, got %+v", r.Record)
				}
				return
			}
			if r.Record == nil {
				t.Fatalf("expected non-nil record")
			}
			if tc.wantState != "" && r.Record.State != tc.wantState {
				t.Fatalf("state=%q want=%q", r.Record.State, tc.wantState)
			}
		})
	}
}
