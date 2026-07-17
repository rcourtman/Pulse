package operationreceipt

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type TerminalValidator func(Identity, json.RawMessage) error
type Config struct {
	RecentTerminalTTL     time.Duration
	MaxRecentPayloadBytes int64
	Validators            map[string]map[int]TerminalValidator
}
type Store struct {
	mu     sync.Mutex
	db     *sql.DB
	config Config
	now    func() time.Time
}

func Open(path string, config Config) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("operation receipt store path is required")
	}
	if config.RecentTerminalTTL <= 0 {
		config.RecentTerminalTTL = 30 * 24 * time.Hour
	}
	if config.MaxRecentPayloadBytes <= 0 {
		config.MaxRecentPayloadBytes = 32 << 20
	}
	if len(config.Validators) == 0 {
		return nil, fmt.Errorf("operation receipt terminal validators are required")
	}
	config.Validators = freezeValidators(config.Validators)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, config: config, now: time.Now}
	if err := s.init(); err != nil {
		db.Close()
		return nil, fmt.Errorf("%w: %v", ErrStoreCorrupt, err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.validateAll(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.recoverInterrupted(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.validateAll(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.Compact(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
func (s *Store) init() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS operation_receipts(attempt_id TEXT PRIMARY KEY,action_id TEXT NOT NULL,operation_kind TEXT NOT NULL,operation_version INTEGER NOT NULL,request_digest TEXT NOT NULL,agent_id TEXT NOT NULL,state TEXT NOT NULL,accepted_at DATETIME NOT NULL,started_at DATETIME,terminal_at DATETIME,result_kind TEXT NOT NULL DEFAULT '',result_version INTEGER NOT NULL DEFAULT 0,result_json BLOB);CREATE INDEX IF NOT EXISTS idx_operation_receipts_terminal ON operation_receipts(state,terminal_at);`)
	if err != nil {
		return err
	}
	var quick string
	if err := s.db.QueryRow("PRAGMA quick_check").Scan(&quick); err != nil || quick != "ok" {
		return fmt.Errorf("sqlite quick_check failed: %s: %v", quick, err)
	}
	return nil
}
func (s *Store) recoverInterrupted() error {
	_, err := s.db.Exec(`UPDATE operation_receipts SET state=?,started_at=COALESCE(started_at,accepted_at) WHERE state IN (?,?)`, string(StateInterrupted), string(StateAccepted), string(StateStarted))
	return err
}

func (s *Store) validateAll() error {
	rows, err := s.db.Query(`SELECT attempt_id FROM operation_receipts ORDER BY attempt_id`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range ids {
		if _, _, err := s.query(id); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) Admit(i Identity) (Record, bool, error) {
	i, err := NormalizeIdentity(i)
	if err != nil {
		return Record{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.compactLocked(context.Background()); err != nil {
		return Record{}, false, err
	}
	if r, ok, err := s.query(i.AttemptID); err != nil {
		return Record{}, false, err
	} else if ok {
		if r.Identity != i {
			return Record{}, false, ErrBindingConflict
		}
		return r, false, nil
	}
	now := s.currentTime()
	_, err = s.db.Exec(`INSERT INTO operation_receipts(attempt_id,action_id,operation_kind,operation_version,request_digest,agent_id,state,accepted_at)VALUES(?,?,?,?,?,?,?,?)`, i.AttemptID, i.ActionID, i.OperationKind, i.OperationVersion, i.RequestDigest, i.AgentID, string(StateAccepted), now)
	if err != nil {
		return Record{}, false, err
	}
	return Record{Identity: i, State: StateAccepted, AcceptedAt: now}, true, nil
}
func (s *Store) MarkStarted(i Identity) (Record, error) {
	i, err := NormalizeIdentity(i)
	if err != nil {
		return Record{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.currentTime()
	res, err := s.db.Exec(`UPDATE operation_receipts SET state=?,started_at=? WHERE attempt_id=? AND action_id=? AND operation_kind=? AND operation_version=? AND request_digest=? AND agent_id=? AND state=?`, string(StateStarted), now, i.AttemptID, i.ActionID, i.OperationKind, i.OperationVersion, i.RequestDigest, i.AgentID, string(StateAccepted))
	if err != nil {
		return Record{}, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		if r, ok, qerr := s.query(i.AttemptID); qerr == nil && ok && r.Identity != i {
			return Record{}, ErrBindingConflict
		}
		return Record{}, ErrNotStartable
	}
	return s.mustQuery(i.AttemptID)
}
func (s *Store) Complete(i Identity, e TerminalEnvelope) (Record, error) {
	i, err := NormalizeIdentity(i)
	if err != nil {
		return Record{}, err
	}
	e.Kind = strings.TrimSpace(e.Kind)
	versions := s.config.Validators[e.Kind]
	validator := versions[e.Version]
	if validator == nil {
		return Record{}, fmt.Errorf("unknown terminal envelope kind/version")
	}
	if len(e.Payload) == 0 || len(e.Payload) > MaxResultBytes {
		return Record{}, ErrCapacity
	}
	if err := validator(i, e.Payload); err != nil {
		return Record{}, fmt.Errorf("invalid terminal envelope: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.currentTime()
	res, err := s.db.Exec(`UPDATE operation_receipts SET state=?,terminal_at=?,result_kind=?,result_version=?,result_json=? WHERE attempt_id=? AND action_id=? AND operation_kind=? AND operation_version=? AND request_digest=? AND agent_id=? AND state=?`, string(StateTerminal), now, e.Kind, e.Version, []byte(e.Payload), i.AttemptID, i.ActionID, i.OperationKind, i.OperationVersion, i.RequestDigest, i.AgentID, string(StateStarted))
	if err != nil {
		return Record{}, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		r, ok, qerr := s.query(i.AttemptID)
		if qerr == nil && ok && r.Identity != i {
			return Record{}, ErrBindingConflict
		}
		if qerr == nil && ok && r.State == StateTerminal && r.ResultKind == e.Kind && r.ResultVersion == e.Version && string(r.Result) == string(e.Payload) {
			return r, nil
		}
		return Record{}, ErrNotCompletable
	}
	record, err := s.mustQuery(i.AttemptID)
	if err != nil {
		return Record{}, err
	}
	if err := s.compactLocked(context.Background()); err != nil {
		return Record{}, err
	}
	return record, nil
}
func (s *Store) Query(i Identity) (QueryResult, error) {
	i, err := NormalizeIdentity(i)
	if err != nil {
		return QueryResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok, err := s.query(i.AttemptID)
	if err != nil {
		return QueryResult{}, err
	}
	if !ok {
		return QueryResult{Version: ProtocolVersion, Status: QueryNotFound}, nil
	}
	if r.Identity != i {
		return QueryResult{}, ErrBindingConflict
	}
	status := QueryFoundInterrupted
	if r.State == StateTerminal {
		status = QueryFoundTerminal
	}
	return QueryResult{Version: ProtocolVersion, Status: status, Record: &r}, nil
}
func (s *Store) Compact(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.compactLocked(ctx)
}

func (s *Store) compactLocked(ctx context.Context) error {
	cutoff := s.currentTime().Add(-s.config.RecentTerminalTTL)
	if _, err := s.db.ExecContext(ctx, `UPDATE operation_receipts SET state=?,result_kind='',result_version=0,result_json=NULL WHERE state=? AND terminal_at<?`, string(StateTombstone), string(StateTerminal), cutoff); err != nil {
		return err
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(length(result_json)),0) FROM operation_receipts WHERE state=?`, string(StateTerminal)).Scan(&total); err != nil {
		return err
	}
	for total > s.config.MaxRecentPayloadBytes {
		var id string
		var size int64
		if err := s.db.QueryRowContext(ctx, `SELECT attempt_id,length(result_json) FROM operation_receipts WHERE state=? ORDER BY terminal_at ASC LIMIT 1`, string(StateTerminal)).Scan(&id, &size); err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE operation_receipts SET state=?,result_kind='',result_version=0,result_json=NULL WHERE attempt_id=? AND state=?`, string(StateTombstone), id, string(StateTerminal)); err != nil {
			return err
		}
		total -= size
	}
	return nil
}
func (s *Store) query(id string) (Record, bool, error) {
	row := s.db.QueryRow(`SELECT attempt_id,action_id,operation_kind,operation_version,request_digest,agent_id,state,accepted_at,started_at,terminal_at,result_kind,result_version,result_json FROM operation_receipts WHERE attempt_id=?`, id)
	var r Record
	var state string
	var started, terminal sql.NullTime
	var payload []byte
	if err := row.Scan(&r.Identity.AttemptID, &r.Identity.ActionID, &r.Identity.OperationKind, &r.Identity.OperationVersion, &r.Identity.RequestDigest, &r.Identity.AgentID, &state, &r.AcceptedAt, &started, &terminal, &r.ResultKind, &r.ResultVersion, &payload); errors.Is(err, sql.ErrNoRows) {
		return Record{}, false, nil
	} else if err != nil {
		return Record{}, false, err
	}
	r.State = State(state)
	if started.Valid {
		r.StartedAt = started.Time
	}
	if terminal.Valid {
		r.TerminalAt = terminal.Time
	}
	r.Result = append([]byte(nil), payload...)
	if err := ValidateRecord(r); err != nil {
		return Record{}, false, fmt.Errorf("%w: %v", ErrStoreCorrupt, err)
	}
	if r.State == StateTerminal {
		versions := s.config.Validators[r.ResultKind]
		validator := versions[r.ResultVersion]
		if validator == nil {
			return Record{}, false, fmt.Errorf("%w: unknown terminal envelope kind/version", ErrStoreCorrupt)
		}
		if err := validator(r.Identity, r.Result); err != nil {
			return Record{}, false, fmt.Errorf("%w: invalid terminal envelope: %v", ErrStoreCorrupt, err)
		}
	}
	return r, true, nil
}

func freezeValidators(source map[string]map[int]TerminalValidator) map[string]map[int]TerminalValidator {
	frozen := make(map[string]map[int]TerminalValidator, len(source))
	for kind, versions := range source {
		copyVersions := make(map[int]TerminalValidator, len(versions))
		for version, validator := range versions {
			copyVersions[version] = validator
		}
		frozen[kind] = copyVersions
	}
	return frozen
}
func (s *Store) mustQuery(id string) (Record, error) {
	r, ok, err := s.query(id)
	if err != nil {
		return Record{}, err
	}
	if !ok {
		return Record{}, ErrNotFound
	}
	return r, nil
}
func (s *Store) currentTime() time.Time {
	if s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}
