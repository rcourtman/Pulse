package operationreceipt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	ProtocolVersion = 1
	MaxResultBytes  = 64 << 10
)

type State string

const (
	StateAccepted    State = "accepted"
	StateStarted     State = "started"
	StateInterrupted State = "interrupted_unknown"
	StateTerminal    State = "terminal"
	StateTombstone   State = "replay_denied_tombstone"
)

type QueryStatus string

const (
	QueryNotFound         QueryStatus = "not_found"
	QueryFoundTerminal    QueryStatus = "found_terminal"
	QueryFoundInterrupted QueryStatus = "found_interrupted_unknown"
)

var (
	ErrBindingConflict = errors.New("operation receipt identity binding conflict")
	ErrNotFound        = errors.New("operation receipt not found")
	ErrNotStartable    = errors.New("operation receipt is not startable")
	ErrNotCompletable  = errors.New("operation receipt is not completable")
	ErrStoreCorrupt    = errors.New("operation receipt store is corrupt")
	ErrCapacity        = errors.New("operation receipt capacity unavailable")
)

type Identity struct {
	AttemptID        string `json:"attempt_id"`
	ActionID         string `json:"action_id"`
	OperationKind    string `json:"operation_kind"`
	OperationVersion int    `json:"operation_version"`
	RequestDigest    string `json:"request_digest"`
	AgentID          string `json:"agent_id"`
}
type Record struct {
	Identity      Identity        `json:"identity"`
	State         State           `json:"state"`
	AcceptedAt    time.Time       `json:"accepted_at"`
	StartedAt     time.Time       `json:"started_at,omitempty"`
	TerminalAt    time.Time       `json:"terminal_at,omitempty"`
	ResultKind    string          `json:"result_kind,omitempty"`
	ResultVersion int             `json:"result_version,omitempty"`
	Result        json.RawMessage `json:"result,omitempty"`
}
type TerminalEnvelope struct {
	Kind    string
	Version int
	Payload json.RawMessage
}
type Query struct {
	Version  int      `json:"version"`
	Identity Identity `json:"identity"`
}
type QueryResult struct {
	Version int         `json:"version"`
	Status  QueryStatus `json:"status"`
	Record  *Record     `json:"record,omitempty"`
}

func DigestCanonicalJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
func NormalizeIdentity(i Identity) (Identity, error) {
	i.AttemptID = strings.TrimSpace(i.AttemptID)
	i.ActionID = strings.TrimSpace(i.ActionID)
	i.OperationKind = strings.TrimSpace(i.OperationKind)
	i.RequestDigest = strings.TrimSpace(i.RequestDigest)
	i.AgentID = strings.TrimSpace(i.AgentID)
	if i.AttemptID == "" || i.ActionID == "" || i.OperationKind == "" || i.AgentID == "" {
		return Identity{}, fmt.Errorf("operation receipt identity fields are required")
	}
	if len(i.AttemptID) > 128 || len(i.ActionID) > 128 || len(i.OperationKind) > 128 || len(i.AgentID) > 128 {
		return Identity{}, fmt.Errorf("operation receipt identity exceeds bounded length")
	}
	if i.OperationVersion <= 0 {
		return Identity{}, fmt.Errorf("operation version must be positive")
	}
	if len(i.RequestDigest) != 71 || !strings.HasPrefix(i.RequestDigest, "sha256:") {
		return Identity{}, fmt.Errorf("request digest must be sha256")
	}
	if _, err := hex.DecodeString(strings.TrimPrefix(i.RequestDigest, "sha256:")); err != nil {
		return Identity{}, fmt.Errorf("request digest must be sha256: %w", err)
	}
	return i, nil
}
func ValidateRecord(r Record) error {
	i, err := NormalizeIdentity(r.Identity)
	if err != nil {
		return err
	}
	if i != r.Identity {
		return fmt.Errorf("operation receipt identity is not normalized")
	}
	if r.AcceptedAt.IsZero() || len(r.Result) > MaxResultBytes {
		return fmt.Errorf("invalid operation receipt bounds")
	}
	if !r.StartedAt.IsZero() && r.StartedAt.Before(r.AcceptedAt) {
		return fmt.Errorf("operation receipt start precedes admission")
	}
	if !r.TerminalAt.IsZero() && (r.StartedAt.IsZero() || r.TerminalAt.Before(r.StartedAt)) {
		return fmt.Errorf("operation receipt terminal time is invalid")
	}
	switch r.State {
	case StateAccepted:
		if !r.StartedAt.IsZero() || !r.TerminalAt.IsZero() || len(r.Result) > 0 || r.ResultKind != "" || r.ResultVersion != 0 {
			return fmt.Errorf("accepted receipt contains later state")
		}
	case StateStarted, StateInterrupted:
		if r.StartedAt.IsZero() || !r.TerminalAt.IsZero() || len(r.Result) > 0 || r.ResultKind != "" || r.ResultVersion != 0 {
			return fmt.Errorf("nonterminal receipt contains terminal state")
		}
	case StateTerminal:
		if r.StartedAt.IsZero() || r.TerminalAt.IsZero() || len(r.Result) == 0 || strings.TrimSpace(r.ResultKind) == "" || r.ResultVersion <= 0 {
			return fmt.Errorf("terminal receipt is incomplete")
		}
	case StateTombstone:
		if r.StartedAt.IsZero() || r.TerminalAt.IsZero() || len(r.Result) != 0 || r.ResultKind != "" || r.ResultVersion != 0 {
			return fmt.Errorf("receipt tombstone contains payload")
		}
	default:
		return fmt.Errorf("unsupported operation receipt state %q", r.State)
	}
	return nil
}
func DecodeQuery(data []byte) (Query, error) {
	var q Query
	if err := decodeStrict(data, &q); err != nil {
		return Query{}, err
	}
	if q.Version != ProtocolVersion {
		return Query{}, fmt.Errorf("unsupported operation query version %d", q.Version)
	}
	i, err := NormalizeIdentity(q.Identity)
	if err != nil {
		return Query{}, err
	}
	q.Identity = i
	return q, nil
}
func DecodeQueryResult(data []byte) (QueryResult, error) {
	var r QueryResult
	if err := decodeStrict(data, &r); err != nil {
		return QueryResult{}, err
	}
	if r.Version != ProtocolVersion {
		return QueryResult{}, fmt.Errorf("unsupported operation query result version %d", r.Version)
	}
	switch r.Status {
	case QueryNotFound:
		if r.Record != nil {
			return QueryResult{}, fmt.Errorf("not-found query result cannot include a record")
		}
	case QueryFoundTerminal:
		if r.Record == nil || r.Record.State != StateTerminal {
			return QueryResult{}, fmt.Errorf("terminal query result requires terminal record")
		}
	case QueryFoundInterrupted:
		if r.Record == nil || (r.Record.State != StateAccepted && r.Record.State != StateStarted && r.Record.State != StateInterrupted && r.Record.State != StateTombstone) {
			return QueryResult{}, fmt.Errorf("interrupted query result requires nonterminal or tombstone record")
		}
	default:
		return QueryResult{}, fmt.Errorf("unsupported operation query status %q", r.Status)
	}
	if r.Record != nil {
		if err := ValidateRecord(*r.Record); err != nil {
			return QueryResult{}, err
		}
	}
	return r, nil
}
func decodeStrict(data []byte, target any) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("operation receipt payload is empty")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("operation receipt payload contains trailing JSON")
		}
		return fmt.Errorf("operation receipt payload contains trailing data: %w", err)
	}
	return nil
}
