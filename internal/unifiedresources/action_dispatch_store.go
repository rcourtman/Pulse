package unifiedresources

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func insertActionDispatchSQL(exec sqlExecutor, attempt ActionDispatchAttempt) error {
	attempt, err := NormalizeActionDispatchAttempt(attempt)
	if err != nil {
		return err
	}
	if _, err := exec.Exec(`
		INSERT INTO action_dispatch_attempts
			(attempt_id, action_id, state, created_at, updated_at, lease_owner, lease_expires_at, dispatch_count)
		VALUES (?, ?, ?, ?, ?, NULL, NULL, ?)
	`, attempt.ID, attempt.ActionID, string(attempt.State), attempt.CreatedAt, attempt.UpdatedAt, attempt.DispatchCount); err != nil {
		return fmt.Errorf("insert action dispatch attempt: %w", err)
	}
	if _, err := exec.Exec(`
		INSERT INTO action_dispatch_outbox (attempt_id, action_id, available_at)
		VALUES (?, ?, ?)
	`, attempt.ID, attempt.ActionID, attempt.CreatedAt); err != nil {
		return fmt.Errorf("insert action dispatch outbox: %w", err)
	}
	return nil
}

func scanActionDispatch(scanner interface{ Scan(...any) error }) (ActionDispatchAttempt, error) {
	var attempt ActionDispatchAttempt
	var state string
	var owner sql.NullString
	var lease sql.NullTime
	if err := scanner.Scan(&attempt.ID, &attempt.ActionID, &state, &attempt.CreatedAt, &attempt.UpdatedAt, &owner, &lease, &attempt.DispatchCount); err != nil {
		return ActionDispatchAttempt{}, err
	}
	attempt.State = ActionDispatchState(state)
	attempt.LeaseOwner = owner.String
	if lease.Valid {
		attempt.LeaseExpiresAt = lease.Time
	}
	return NormalizeActionDispatchAttempt(attempt)
}

func getActionDispatchFrom(queryer actionAuditQueryRower, actionID string) (ActionDispatchAttempt, bool, error) {
	row := queryer.QueryRow(`
		SELECT attempt_id, action_id, state, created_at, updated_at, lease_owner, lease_expires_at, dispatch_count
		FROM action_dispatch_attempts WHERE action_id = ?
	`, strings.TrimSpace(actionID))
	attempt, err := scanActionDispatch(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ActionDispatchAttempt{}, false, nil
	}
	return attempt, err == nil, err
}

func getActionDispatchByAttemptFrom(queryer actionAuditQueryRower, attemptID string) (ActionDispatchAttempt, bool, error) {
	row := queryer.QueryRow(`
		SELECT attempt_id, action_id, state, created_at, updated_at, lease_owner, lease_expires_at, dispatch_count
		FROM action_dispatch_attempts WHERE attempt_id = ?
	`, strings.TrimSpace(attemptID))
	attempt, err := scanActionDispatch(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ActionDispatchAttempt{}, false, nil
	}
	return attempt, err == nil, err
}

func validateExecutionAdmission(record ActionAuditRecord, event ActionLifecycleEvent, attempt ActionDispatchAttempt) (ActionAuditRecord, ActionLifecycleEvent, ActionDispatchAttempt, error) {
	record, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ActionDispatchAttempt{}, err
	}
	event, err = NormalizeActionLifecycleEvent(event)
	if err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ActionDispatchAttempt{}, err
	}
	attempt, err = NormalizeActionDispatchAttempt(attempt)
	if err != nil {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ActionDispatchAttempt{}, err
	}
	if record.State != ActionStateExecuting || event.State != ActionStateExecuting || event.ActionID != record.ID || attempt.ActionID != record.ID || attempt.State != ActionDispatchQueued {
		return ActionAuditRecord{}, ActionLifecycleEvent{}, ActionDispatchAttempt{}, errors.New("action dispatch admission identities or states do not match")
	}
	return RedactAuditRecord(record), event, attempt, nil
}

func (s *SQLiteResourceStore) RecordActionExecutionAdmission(record ActionAuditRecord, event ActionLifecycleEvent, attempt ActionDispatchAttempt) error {
	record, event, attempt, err := validateExecutionAdmission(record, event, attempt)
	if err != nil {
		return err
	}
	expected := []ActionState{ActionStateApproved}
	if record.Plan.Allowed && !record.Plan.RequiresApproval {
		expected = append(expected, ActionStatePlanned)
	}
	return s.recordActionAdmission(record, []ActionLifecycleEvent{event}, attempt, expected)
}

func (s *SQLiteResourceStore) RecordActionPolicyExecutionAdmission(record ActionAuditRecord, approvalEvent, executionEvent ActionLifecycleEvent, attempt ActionDispatchAttempt) error {
	record, executionEvent, attempt, err := validateExecutionAdmission(record, executionEvent, attempt)
	if err != nil {
		return err
	}
	approvalEvent, err = NormalizeActionLifecycleEvent(approvalEvent)
	if err != nil {
		return err
	}
	if approvalEvent.State != ActionStateApproved || approvalEvent.ActionID != record.ID || len(record.Approvals) == 0 || record.Approvals[len(record.Approvals)-1].PolicyLease == nil {
		return ErrActionPolicyAuthorizationInvalid
	}
	return s.recordActionAdmission(record, []ActionLifecycleEvent{approvalEvent, executionEvent}, attempt, []ActionState{ActionStatePlanned, ActionStatePending})
}

func (s *SQLiteResourceStore) recordActionAdmission(record ActionAuditRecord, events []ActionLifecycleEvent, attempt ActionDispatchAttempt, expected []ActionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin action dispatch admission transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	updated, err := updateActionAuditSQL(tx, record, expected...)
	if err != nil {
		return err
	}
	if !updated {
		current, found, queryErr := getActionAuditFrom(tx, record.ID)
		if queryErr != nil {
			return queryErr
		}
		if !found {
			return fmt.Errorf("action audit %q not found", record.ID)
		}
		return actionTransitionConflict(current, record, ErrActionNotApproved)
	}
	for _, event := range events {
		if err := recordActionLifecycleEventSQL(tx, event); err != nil {
			return err
		}
	}
	if err := insertActionDispatchSQL(tx, attempt); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit action dispatch admission transaction: %w", err)
	}
	committed = true
	return nil
}

func (s *SQLiteResourceStore) RecordActionExpiry(record ActionAuditRecord, event ActionLifecycleEvent) error {
	record, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return err
	}
	event, err = NormalizeActionLifecycleEvent(event)
	if err != nil {
		return err
	}
	if record.State != ActionStateExpired || event.State != ActionStateExpired || event.ActionID != record.ID {
		return errors.New("action expiry must persist matching expired state")
	}
	return s.recordActionTransition(record, event, []ActionState{ActionStatePlanned, ActionStatePending, ActionStateApproved}, ErrActionExecutionFinal)
}

func (s *SQLiteResourceStore) GetActionDispatchAttempt(actionID string) (ActionDispatchAttempt, bool, error) {
	return getActionDispatchFrom(s.db, actionID)
}

func (s *SQLiteResourceStore) GetActionDispatchReceipt(attemptID string) (ActionDispatchReceipt, bool, error) {
	row := s.db.QueryRow(`SELECT attempt_id, action_id, transport_request_id, received_at FROM action_dispatch_receipts WHERE attempt_id = ?`, strings.TrimSpace(attemptID))
	var receipt ActionDispatchReceipt
	if err := row.Scan(&receipt.AttemptID, &receipt.ActionID, &receipt.TransportRequestID, &receipt.ReceivedAt); errors.Is(err, sql.ErrNoRows) {
		return ActionDispatchReceipt{}, false, nil
	} else if err != nil {
		return ActionDispatchReceipt{}, false, err
	}
	receipt, err := NormalizeActionDispatchReceipt(receipt)
	return receipt, err == nil, err
}

func (s *SQLiteResourceStore) ClaimActionDispatch(actionID, owner string, now time.Time, lease time.Duration) (ActionDispatchAttempt, bool, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" || lease <= 0 {
		return ActionDispatchAttempt{}, false, ErrActionDispatchNotClaimable
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return ActionDispatchAttempt{}, false, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	attempt, found, err := getActionDispatchFrom(tx, actionID)
	if err != nil || !found {
		if err == nil {
			err = ErrActionDispatchNotFound
		}
		return ActionDispatchAttempt{}, false, err
	}
	if attempt.State == ActionDispatchClaimed {
		if attempt.LeaseOwner == owner && now.Before(attempt.LeaseExpiresAt) {
			_ = tx.Rollback()
			committed = true
			return attempt, false, nil
		}
		if !attempt.LeaseExpiresAt.IsZero() && !now.Before(attempt.LeaseExpiresAt) {
			// Claiming does not authorize transport. Until MarkActionDispatchStarted
			// commits the pre-send boundary, an expired lease is safe to requeue.
			attempt.State, attempt.UpdatedAt, attempt.LeaseOwner, attempt.LeaseExpiresAt = ActionDispatchQueued, now, "", time.Time{}
			if _, err = tx.Exec(`UPDATE action_dispatch_attempts SET state=?, updated_at=?, lease_owner=NULL, lease_expires_at=NULL WHERE attempt_id=?`, string(attempt.State), now, attempt.ID); err != nil {
				return ActionDispatchAttempt{}, false, err
			}
			if _, err = tx.Exec(`INSERT OR IGNORE INTO action_dispatch_outbox (attempt_id, action_id, available_at) VALUES (?, ?, ?)`, attempt.ID, attempt.ActionID, now); err != nil {
				return ActionDispatchAttempt{}, false, err
			}
			attempt.State, attempt.UpdatedAt, attempt.LeaseOwner, attempt.LeaseExpiresAt = ActionDispatchClaimed, now, owner, now.Add(lease)
			if _, err = tx.Exec(`UPDATE action_dispatch_attempts SET state=?, updated_at=?, lease_owner=?, lease_expires_at=? WHERE attempt_id=? AND state=?`, string(attempt.State), now, owner, attempt.LeaseExpiresAt, attempt.ID, string(ActionDispatchQueued)); err != nil {
				return ActionDispatchAttempt{}, false, err
			}
			if err = tx.Commit(); err != nil {
				return ActionDispatchAttempt{}, false, err
			}
			committed = true
			return attempt, true, nil
		}
		return attempt, false, nil
	}
	if attempt.State != ActionDispatchQueued {
		_ = tx.Rollback()
		committed = true
		return attempt, false, nil
	}
	attempt.State, attempt.UpdatedAt, attempt.LeaseOwner, attempt.LeaseExpiresAt = ActionDispatchClaimed, now, owner, now.Add(lease)
	result, err := tx.Exec(`UPDATE action_dispatch_attempts SET state=?, updated_at=?, lease_owner=?, lease_expires_at=? WHERE attempt_id=? AND state=?`, string(attempt.State), now, owner, attempt.LeaseExpiresAt, attempt.ID, string(ActionDispatchQueued))
	if err != nil {
		return ActionDispatchAttempt{}, false, err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return ActionDispatchAttempt{}, false, ErrActionDispatchNotClaimable
	}
	if err = tx.Commit(); err != nil {
		return ActionDispatchAttempt{}, false, err
	}
	committed = true
	return attempt, true, nil
}

func (s *SQLiteResourceStore) MarkActionDispatchStarted(attemptID, owner string, now time.Time) (ActionDispatchAttempt, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return ActionDispatchAttempt{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	attempt, found, err := getActionDispatchByAttemptFrom(tx, attemptID)
	if err != nil || !found {
		if err == nil {
			err = ErrActionDispatchNotFound
		}
		return ActionDispatchAttempt{}, err
	}
	if attempt.State != ActionDispatchClaimed {
		return ActionDispatchAttempt{}, ErrActionDispatchNotClaimable
	}
	if attempt.LeaseOwner != strings.TrimSpace(owner) {
		return ActionDispatchAttempt{}, ErrActionDispatchLeaseMismatch
	}
	attempt.State, attempt.UpdatedAt, attempt.DispatchCount, attempt.LeaseOwner, attempt.LeaseExpiresAt = ActionDispatchReceiptPending, now, attempt.DispatchCount+1, "", time.Time{}
	result, err := tx.Exec(`UPDATE action_dispatch_attempts SET state=?, updated_at=?, lease_owner=NULL, lease_expires_at=NULL, dispatch_count=? WHERE attempt_id=? AND state=? AND lease_owner=?`, string(attempt.State), now, attempt.DispatchCount, attempt.ID, string(ActionDispatchClaimed), owner)
	if err != nil {
		return ActionDispatchAttempt{}, err
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return ActionDispatchAttempt{}, ErrActionDispatchNotClaimable
	}
	if _, err = tx.Exec(`DELETE FROM action_dispatch_outbox WHERE attempt_id=?`, attempt.ID); err != nil {
		return ActionDispatchAttempt{}, err
	}
	if err = tx.Commit(); err != nil {
		return ActionDispatchAttempt{}, err
	}
	committed = true
	return attempt, nil
}

func (s *SQLiteResourceStore) RecordActionDispatchReceipt(receipt ActionDispatchReceipt) (ActionDispatchAttempt, error) {
	receipt, err := NormalizeActionDispatchReceipt(receipt)
	if err != nil {
		return ActionDispatchAttempt{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return ActionDispatchAttempt{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	attempt, found, err := getActionDispatchByAttemptFrom(tx, receipt.AttemptID)
	if err != nil || !found {
		if err == nil {
			err = ErrActionDispatchNotFound
		}
		return ActionDispatchAttempt{}, err
	}
	var existingID, transportID string
	var received time.Time
	scanErr := tx.QueryRow(`SELECT attempt_id, transport_request_id, received_at FROM action_dispatch_receipts WHERE attempt_id=?`, receipt.AttemptID).Scan(&existingID, &transportID, &received)
	if scanErr == nil {
		if transportID != receipt.TransportRequestID {
			return ActionDispatchAttempt{}, ErrActionDispatchReceiptConflict
		}
		_ = tx.Rollback()
		committed = true
		return attempt, nil
	}
	if !errors.Is(scanErr, sql.ErrNoRows) {
		return ActionDispatchAttempt{}, scanErr
	}
	if attempt.State != ActionDispatchReceiptPending {
		return ActionDispatchAttempt{}, ErrActionDispatchReceiptConflict
	}
	if _, err = tx.Exec(`INSERT INTO action_dispatch_receipts (attempt_id, action_id, transport_request_id, received_at) VALUES (?, ?, ?, ?)`, receipt.AttemptID, receipt.ActionID, receipt.TransportRequestID, receipt.ReceivedAt); err != nil {
		return ActionDispatchAttempt{}, err
	}
	attempt.State, attempt.UpdatedAt, attempt.LeaseOwner, attempt.LeaseExpiresAt = ActionDispatchReceiptRecorded, receipt.ReceivedAt, "", time.Time{}
	if _, err = tx.Exec(`UPDATE action_dispatch_attempts SET state=?, updated_at=?, lease_owner=NULL, lease_expires_at=NULL WHERE attempt_id=?`, string(attempt.State), attempt.UpdatedAt, attempt.ID); err != nil {
		return ActionDispatchAttempt{}, err
	}
	if _, err = tx.Exec(`DELETE FROM action_dispatch_outbox WHERE attempt_id=?`, attempt.ID); err != nil {
		return ActionDispatchAttempt{}, err
	}
	if err = tx.Commit(); err != nil {
		return ActionDispatchAttempt{}, err
	}
	committed = true
	return attempt, nil
}

func normalizeActionDispatchCompletion(receipt ActionDispatchReceipt, record ActionAuditRecord, event ActionLifecycleEvent) (ActionDispatchReceipt, ActionAuditRecord, ActionLifecycleEvent, error) {
	receipt, err := NormalizeActionDispatchReceipt(receipt)
	if err != nil {
		return ActionDispatchReceipt{}, ActionAuditRecord{}, ActionLifecycleEvent{}, err
	}
	record, err = NormalizeActionAuditRecord(record)
	if err != nil {
		return ActionDispatchReceipt{}, ActionAuditRecord{}, ActionLifecycleEvent{}, err
	}
	event, err = NormalizeActionLifecycleEvent(event)
	if err != nil {
		return ActionDispatchReceipt{}, ActionAuditRecord{}, ActionLifecycleEvent{}, err
	}
	if receipt.ActionID != record.ID || event.ActionID != record.ID || event.State != record.State || (record.State != ActionStateCompleted && record.State != ActionStateFailed) {
		return ActionDispatchReceipt{}, ActionAuditRecord{}, ActionLifecycleEvent{}, errors.New("correlated dispatch completion identities or terminal states do not match")
	}
	return receipt, RedactAuditRecord(record), event, nil
}

// RecordActionDispatchCompletion atomically persists the correlated transport
// receipt and today's canonical terminal audit/event. It adds no new result
// truth; it only prevents a crash from separating an existing result from its
// receipt correlation.
func (s *SQLiteResourceStore) RecordActionDispatchCompletion(receipt ActionDispatchReceipt, record ActionAuditRecord, event ActionLifecycleEvent) error {
	receipt, record, event, err := normalizeActionDispatchCompletion(receipt, record, event)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	attempt, found, err := getActionDispatchByAttemptFrom(tx, receipt.AttemptID)
	if err != nil || !found {
		if err == nil {
			err = ErrActionDispatchNotFound
		}
		return err
	}
	var existingTransportID string
	receiptErr := tx.QueryRow(`SELECT transport_request_id FROM action_dispatch_receipts WHERE attempt_id=?`, receipt.AttemptID).Scan(&existingTransportID)
	switch {
	case receiptErr == nil:
		if existingTransportID != receipt.TransportRequestID || attempt.State != ActionDispatchReceiptRecorded {
			return ErrActionDispatchReceiptConflict
		}
	case errors.Is(receiptErr, sql.ErrNoRows):
		if attempt.State != ActionDispatchReceiptPending {
			return ErrActionDispatchReceiptConflict
		}
		if _, err = tx.Exec(`INSERT INTO action_dispatch_receipts (attempt_id, action_id, transport_request_id, received_at) VALUES (?, ?, ?, ?)`, receipt.AttemptID, receipt.ActionID, receipt.TransportRequestID, receipt.ReceivedAt); err != nil {
			return err
		}
		result, updateErr := tx.Exec(`UPDATE action_dispatch_attempts SET state=?, updated_at=?, lease_owner=NULL, lease_expires_at=NULL WHERE attempt_id=? AND state=?`, string(ActionDispatchReceiptRecorded), receipt.ReceivedAt, attempt.ID, string(ActionDispatchReceiptPending))
		if updateErr != nil {
			return updateErr
		}
		if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
			return ErrActionDispatchReceiptConflict
		}
	case receiptErr != nil:
		return receiptErr
	}
	updated, err := updateActionAuditSQL(tx, record, ActionStateExecuting)
	if err != nil {
		return err
	}
	if !updated {
		current, currentFound, queryErr := getActionAuditFrom(tx, record.ID)
		if queryErr != nil {
			return queryErr
		}
		if !currentFound {
			return fmt.Errorf("action audit %q not found", record.ID)
		}
		return actionTransitionConflict(current, record, ErrActionNotExecuting)
	}
	if err := recordActionLifecycleEventSQL(tx, event); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM action_dispatch_outbox WHERE attempt_id=?`, attempt.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *SQLiteResourceStore) RecoverActionDispatch(actionID string, now time.Time) (ActionDispatchAttempt, bool, error) {
	attempt, found, err := s.GetActionDispatchAttempt(actionID)
	if err != nil || !found {
		return attempt, found, err
	}
	if attempt.State != ActionDispatchClaimed || attempt.LeaseExpiresAt.IsZero() || now.Before(attempt.LeaseExpiresAt) {
		return attempt, false, nil
	}
	return s.ClaimActionDispatch(actionID, "recovery", now, time.Second)
}

func (s *SQLiteResourceStore) ExpireActionAudits(now time.Time, limit int) ([]ActionAuditRecord, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, action_id, request_id, created_at, updated_at, state, request_json, plan_json, approvals_json, result_json, verification_outcome_json, origin_json FROM action_audits WHERE state IN (?, ?, ?) ORDER BY updated_at ASC LIMIT ?`, ActionStatePlanned, ActionStatePending, ActionStateApproved, limit)
	if err != nil {
		return nil, err
	}
	var candidates []ActionAuditRecord
	for rows.Next() {
		record, scanErr := scanActionAuditRecord(rows)
		if scanErr != nil {
			rows.Close()
			return nil, scanErr
		}
		if !record.Plan.ExpiresAt.IsZero() && !now.Before(record.Plan.ExpiresAt) {
			candidates = append(candidates, record)
		}
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	var expired []ActionAuditRecord
	for _, record := range candidates {
		updated, event, applyErr := ExpireAction(record, "system:expiry", now)
		if applyErr != nil {
			continue
		}
		if persistErr := s.RecordActionExpiry(updated, event); persistErr == nil {
			expired = append(expired, updated)
		} else if !errors.Is(persistErr, ErrActionExecutionFinal) {
			return expired, persistErr
		}
	}
	return expired, nil
}

func (s *SQLiteResourceStore) GetActionAuditsByStates(states []ActionState, limit int) ([]ActionAuditRecord, error) {
	if len(states) == 0 {
		return []ActionAuditRecord{}, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	marks := make([]string, len(states))
	args := make([]any, 0, len(states)+1)
	for i, state := range states {
		if !isValidActionState(state) {
			return nil, fmt.Errorf("unsupported action state %q", state)
		}
		marks[i] = "?"
		args = append(args, string(state))
	}
	args = append(args, limit)
	rows, err := s.db.Query(`SELECT id, action_id, request_id, created_at, updated_at, state, request_json, plan_json, approvals_json, result_json, verification_outcome_json, origin_json FROM action_audits WHERE state IN (`+strings.Join(marks, ",")+`) ORDER BY updated_at DESC, created_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActionAuditRecord
	for rows.Next() {
		record, scanErr := scanActionAuditRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

// MemoryStore mirrors the durable transport authority contract for unit tests.
func (m *MemoryStore) RecordActionExecutionAdmission(record ActionAuditRecord, event ActionLifecycleEvent, attempt ActionDispatchAttempt) error {
	record, event, attempt, err := validateExecutionAdmission(record, event, attempt)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.actionAudits {
		if m.actionAudits[i].ID == record.ID {
			if err := ValidateActionExecutionStart(m.actionAudits[i], event.Timestamp); err != nil {
				return err
			}
			m.actionAudits[i] = record
			m.actionLifecycleEvents = append(m.actionLifecycleEvents, event)
			m.actionDispatchAttempts[record.ID] = attempt
			return nil
		}
	}
	return fmt.Errorf("action audit %q not found", record.ID)
}

func (m *MemoryStore) RecordActionPolicyExecutionAdmission(record ActionAuditRecord, approvalEvent, executionEvent ActionLifecycleEvent, attempt ActionDispatchAttempt) error {
	record, executionEvent, attempt, err := validateExecutionAdmission(record, executionEvent, attempt)
	if err != nil {
		return err
	}
	approvalEvent, err = NormalizeActionLifecycleEvent(approvalEvent)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.actionAudits {
		if m.actionAudits[i].ID == record.ID {
			current := m.actionAudits[i]
			if current.State != ActionStatePlanned && current.State != ActionStatePending {
				return actionTransitionConflict(current, record, ErrActionNotApproved)
			}
			m.actionAudits[i] = record
			m.actionLifecycleEvents = append(m.actionLifecycleEvents, approvalEvent, executionEvent)
			m.actionDispatchAttempts[record.ID] = attempt
			return nil
		}
	}
	return fmt.Errorf("action audit %q not found", record.ID)
}

func (m *MemoryStore) RecordActionExpiry(record ActionAuditRecord, event ActionLifecycleEvent) error {
	record, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return err
	}
	event, err = NormalizeActionLifecycleEvent(event)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.actionAudits {
		if m.actionAudits[i].ID == record.ID {
			current := m.actionAudits[i]
			switch current.State {
			case ActionStatePlanned, ActionStatePending, ActionStateApproved:
				m.actionAudits[i] = record
				m.actionLifecycleEvents = append(m.actionLifecycleEvents, event)
				return nil
			default:
				return actionTransitionConflict(current, record, ErrActionExecutionFinal)
			}
		}
	}
	return fmt.Errorf("action audit %q not found", record.ID)
}

func (m *MemoryStore) GetActionDispatchAttempt(actionID string) (ActionDispatchAttempt, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.actionDispatchAttempts[strings.TrimSpace(actionID)]
	return a, ok, nil
}
func (m *MemoryStore) GetActionDispatchReceipt(attemptID string) (ActionDispatchReceipt, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.actionDispatchReceipts[strings.TrimSpace(attemptID)]
	return r, ok, nil
}

func (m *MemoryStore) ClaimActionDispatch(actionID, owner string, now time.Time, lease time.Duration) (ActionDispatchAttempt, bool, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" || lease <= 0 {
		return ActionDispatchAttempt{}, false, ErrActionDispatchNotClaimable
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.actionDispatchAttempts[actionID]
	if !ok {
		return a, false, ErrActionDispatchNotFound
	}
	if a.State == ActionDispatchClaimed && !a.LeaseExpiresAt.IsZero() && !now.Before(a.LeaseExpiresAt) {
		a.State, a.UpdatedAt, a.LeaseOwner, a.LeaseExpiresAt = ActionDispatchQueued, now, "", time.Time{}
		m.actionDispatchAttempts[actionID] = a
	}
	if a.State == ActionDispatchClaimed && a.LeaseOwner == owner && now.Before(a.LeaseExpiresAt) {
		return a, false, nil
	}
	if a.State != ActionDispatchQueued {
		return a, false, nil
	}
	a.State, a.UpdatedAt, a.LeaseOwner, a.LeaseExpiresAt = ActionDispatchClaimed, now, owner, now.Add(lease)
	m.actionDispatchAttempts[actionID] = a
	return a, true, nil
}

func (m *MemoryStore) MarkActionDispatchStarted(attemptID, owner string, now time.Time) (ActionDispatchAttempt, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for actionID, a := range m.actionDispatchAttempts {
		if a.ID != attemptID {
			continue
		}
		if a.State != ActionDispatchClaimed {
			return a, ErrActionDispatchNotClaimable
		}
		if a.LeaseOwner != strings.TrimSpace(owner) {
			return a, ErrActionDispatchLeaseMismatch
		}
		a.State, a.UpdatedAt, a.DispatchCount, a.LeaseOwner, a.LeaseExpiresAt = ActionDispatchReceiptPending, now, a.DispatchCount+1, "", time.Time{}
		m.actionDispatchAttempts[actionID] = a
		return a, nil
	}
	return ActionDispatchAttempt{}, ErrActionDispatchNotFound
}

func (m *MemoryStore) RecordActionDispatchReceipt(receipt ActionDispatchReceipt) (ActionDispatchAttempt, error) {
	receipt, err := NormalizeActionDispatchReceipt(receipt)
	if err != nil {
		return ActionDispatchAttempt{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.actionDispatchAttempts[receipt.ActionID]
	if !ok {
		return a, ErrActionDispatchNotFound
	}
	if current, exists := m.actionDispatchReceipts[receipt.AttemptID]; exists {
		if current.TransportRequestID != receipt.TransportRequestID {
			return a, ErrActionDispatchReceiptConflict
		}
		return a, nil
	}
	if a.State != ActionDispatchReceiptPending {
		return a, ErrActionDispatchReceiptConflict
	}
	a.State, a.UpdatedAt, a.LeaseOwner, a.LeaseExpiresAt = ActionDispatchReceiptRecorded, receipt.ReceivedAt, "", time.Time{}
	m.actionDispatchAttempts[receipt.ActionID] = a
	m.actionDispatchReceipts[receipt.AttemptID] = receipt
	return a, nil
}

func (m *MemoryStore) RecordActionDispatchCompletion(receipt ActionDispatchReceipt, record ActionAuditRecord, event ActionLifecycleEvent) error {
	receipt, record, event, err := normalizeActionDispatchCompletion(receipt, record, event)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	attempt, found := m.actionDispatchAttempts[record.ID]
	if !found || attempt.ID != receipt.AttemptID {
		return ErrActionDispatchNotFound
	}
	existingReceipt, receiptExists := m.actionDispatchReceipts[receipt.AttemptID]
	if receiptExists {
		if existingReceipt.TransportRequestID != receipt.TransportRequestID || attempt.State != ActionDispatchReceiptRecorded {
			return ErrActionDispatchReceiptConflict
		}
	} else if attempt.State != ActionDispatchReceiptPending {
		return ErrActionDispatchReceiptConflict
	}
	auditIndex := -1
	for i := range m.actionAudits {
		if m.actionAudits[i].ID == record.ID {
			auditIndex = i
			break
		}
	}
	if auditIndex < 0 {
		return fmt.Errorf("action audit %q not found", record.ID)
	}
	current := m.actionAudits[auditIndex]
	if current.State != ActionStateExecuting {
		return actionTransitionConflict(current, record, ErrActionNotExecuting)
	}
	if !ActionAuditIdentityMatches(current, record) {
		return ErrActionIdentityConflict
	}
	attempt.State = ActionDispatchReceiptRecorded
	attempt.UpdatedAt = receipt.ReceivedAt
	attempt.LeaseOwner = ""
	attempt.LeaseExpiresAt = time.Time{}
	m.actionDispatchAttempts[record.ID] = attempt
	m.actionDispatchReceipts[receipt.AttemptID] = receipt
	m.actionAudits[auditIndex] = record
	m.actionLifecycleEvents = append(m.actionLifecycleEvents, event)
	return nil
}

func (m *MemoryStore) RecoverActionDispatch(actionID string, now time.Time) (ActionDispatchAttempt, bool, error) {
	a, ok, err := m.GetActionDispatchAttempt(actionID)
	if err != nil || !ok || a.State != ActionDispatchClaimed || a.LeaseExpiresAt.IsZero() || now.Before(a.LeaseExpiresAt) {
		return a, false, err
	}
	return m.ClaimActionDispatch(actionID, "recovery", now, time.Second)
}

func (m *MemoryStore) ExpireActionAudits(now time.Time, limit int) ([]ActionAuditRecord, error) {
	m.mu.RLock()
	var candidates []ActionAuditRecord
	for _, r := range m.actionAudits {
		if (r.State == ActionStatePlanned || r.State == ActionStatePending || r.State == ActionStateApproved) && !r.Plan.ExpiresAt.IsZero() && !now.Before(r.Plan.ExpiresAt) {
			candidates = append(candidates, r)
		}
	}
	m.mu.RUnlock()
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	var out []ActionAuditRecord
	for _, r := range candidates {
		expired, event, err := ExpireAction(r, "system:expiry", now)
		if err == nil && m.RecordActionExpiry(expired, event) == nil {
			out = append(out, expired)
		}
	}
	return out, nil
}

func (m *MemoryStore) GetActionAuditsByStates(states []ActionState, limit int) ([]ActionAuditRecord, error) {
	set := map[ActionState]bool{}
	for _, s := range states {
		set[s] = true
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []ActionAuditRecord
	for i := len(m.actionAudits) - 1; i >= 0; i-- {
		if set[m.actionAudits[i].State] {
			out = append(out, m.actionAudits[i])
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}
