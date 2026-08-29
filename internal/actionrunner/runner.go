package actionrunner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
)

type Handler interface {
	ValidatePayload(json.RawMessage) error
	Execute(context.Context, Target, json.RawMessage) (Result, error)
}

type Config struct {
	ReceiptPath string
	Handlers    map[string]map[int]Handler
}

type Runner struct {
	store    *operationreceipt.Store
	handlers map[string]map[int]Handler
	mu       sync.Mutex
	active   map[string]activeAction
	now      func() time.Time
}

type activeAction struct {
	identity operationreceipt.Identity
	cancel   context.CancelFunc
}

func Open(config Config) (*Runner, error) {
	if strings.TrimSpace(config.ReceiptPath) == "" {
		return nil, fmt.Errorf("action runner receipt path is required")
	}
	handlers := make(map[string]map[int]Handler, len(config.Handlers))
	for operation, versions := range config.Handlers {
		operation = strings.ToLower(strings.TrimSpace(operation))
		copyVersions := make(map[int]Handler, len(versions))
		for version, handler := range versions {
			if !AllowedOperation(operation, version) || handler == nil {
				return nil, fmt.Errorf("%w: %s v%d", ErrUnsupported, operation, version)
			}
			copyVersions[version] = handler
		}
		handlers[operation] = copyVersions
	}
	store, err := operationreceipt.Open(config.ReceiptPath, operationreceipt.Config{Validators: map[string]map[int]operationreceipt.TerminalValidator{ReceiptKind: {ReceiptVersion: ValidateTerminal}}})
	if err != nil {
		return nil, err
	}
	return &Runner{store: store, handlers: handlers, active: make(map[string]activeAction), now: time.Now}, nil
}

func (r *Runner) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	for _, action := range r.active {
		action.cancel()
	}
	r.active = make(map[string]activeAction)
	r.mu.Unlock()
	return r.store.Close()
}

func (r *Runner) Execute(ctx context.Context, session Session, request Request) (operationreceipt.Record, error) {
	if r == nil || r.store == nil {
		return operationreceipt.Record{}, fmt.Errorf("action runner is not initialized")
	}
	now := r.now().UTC()
	if err := ValidateRequest(&request, now); err != nil {
		return operationreceipt.Record{}, err
	}
	if err := authorize(session, request); err != nil {
		return operationreceipt.Record{}, err
	}
	handler := r.handlers[request.Operation][request.OperationVersion]
	if handler == nil {
		return operationreceipt.Record{}, fmt.Errorf("%w: %s v%d", ErrUnsupported, request.Operation, request.OperationVersion)
	}
	if err := handler.ValidatePayload(request.Payload); err != nil {
		return operationreceipt.Record{}, fmt.Errorf("invalid typed action payload: %w", err)
	}
	identity := ReceiptIdentity(request, session.TokenID)
	record, admitted, err := r.store.Admit(identity)
	if err != nil {
		return operationreceipt.Record{}, err
	}
	if !admitted {
		if record.State == operationreceipt.StateTerminal {
			return record, nil
		}
		return operationreceipt.Record{}, ErrReplayInProgress
	}
	if _, err := r.store.MarkStarted(identity); err != nil {
		return operationreceipt.Record{}, err
	}
	actionCtx, cancel := context.WithDeadline(ctx, request.Deadline)
	r.mu.Lock()
	r.active[request.AttemptID] = activeAction{identity: identity, cancel: cancel}
	r.mu.Unlock()
	defer func() {
		cancel()
		r.mu.Lock()
		delete(r.active, request.AttemptID)
		r.mu.Unlock()
	}()

	result, executeErr := handler.Execute(actionCtx, request.Target, append(json.RawMessage(nil), request.Payload...))
	terminal := TerminalResult{ProtocolVersion: ProtocolVersion, AttemptID: request.AttemptID, ActionID: request.ActionID, Operation: request.Operation, OperationVersion: request.OperationVersion, RequestDigest: request.RequestDigest, Target: request.Target, Status: result.Status, ReasonCode: strings.TrimSpace(result.ReasonCode), Output: result.Output}
	if executeErr != nil {
		terminal.Output = nil
		switch {
		case errors.Is(actionCtx.Err(), context.DeadlineExceeded):
			terminal.Status, terminal.ReasonCode = ResultDeadline, "deadline_exceeded"
		case errors.Is(actionCtx.Err(), context.Canceled):
			terminal.Status, terminal.ReasonCode = ResultCanceled, "canceled"
		default:
			terminal.Status, terminal.ReasonCode = ResultFailed, "execution_failed"
		}
	}
	encoded, err := json.Marshal(terminal)
	if err != nil {
		return operationreceipt.Record{}, err
	}
	if err := ValidateTerminal(identity, encoded); err != nil {
		return operationreceipt.Record{}, err
	}
	completed, err := r.store.Complete(identity, operationreceipt.TerminalEnvelope{Kind: ReceiptKind, Version: ReceiptVersion, Payload: encoded})
	if err != nil {
		return operationreceipt.Record{}, err
	}
	return completed, executeErr
}

func (r *Runner) Cancel(session Session, identity operationreceipt.Identity) error {
	if strings.TrimSpace(session.TokenID) == "" || identity.AgentID != strings.TrimSpace(session.TokenID) || !session.Capabilities[ActionCapability] {
		return ErrUnauthorized
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	action, ok := r.active[identity.AttemptID]
	if !ok {
		return ErrNotActive
	}
	if action.identity != identity {
		return operationreceipt.ErrBindingConflict
	}
	action.cancel()
	return nil
}

func (r *Runner) Query(session Session, identity operationreceipt.Identity) (operationreceipt.QueryResult, error) {
	if strings.TrimSpace(session.TokenID) == "" || identity.AgentID != strings.TrimSpace(session.TokenID) || !session.Capabilities[ActionCapability] {
		return operationreceipt.QueryResult{}, ErrUnauthorized
	}
	return r.store.Query(identity)
}

func authorize(session Session, request Request) error {
	if !boundedID.MatchString(strings.TrimSpace(session.OrganizationID)) || !boundedID.MatchString(strings.TrimSpace(session.HostID)) || !boundedID.MatchString(strings.TrimSpace(session.TokenID)) || !session.Capabilities[ActionCapability] {
		return ErrUnauthorized
	}
	if session.OrganizationID != request.OrganizationID || session.HostID != request.HostID {
		return ErrUnauthorized
	}
	return nil
}
