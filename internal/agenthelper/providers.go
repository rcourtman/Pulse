package agenthelper

import (
	"context"
	"encoding/json"
	"errors"
)

// SMARTProvider owns the privileged implementation of a complete SMART
// snapshot. The protocol deliberately supplies no caller-selected path or
// command arguments.
type SMARTProvider interface {
	Snapshot(context.Context) (json.RawMessage, error)
}

// ProxmoxProvider owns the privileged implementation of the bounded LXC
// filesystem inventory. The protocol deliberately supplies no VMID, path, or
// pct arguments.
type ProxmoxProvider interface {
	LXCFilesystems(context.Context) (json.RawMessage, error)
}

// ContainerProvider owns a complete, bounded inventory of the helper's fixed
// local Docker and Podman endpoints. The protocol supplies no socket, URL,
// daemon method, or query fields.
type ContainerProvider interface {
	Inventory(context.Context) (json.RawMessage, error)
}

// UpdateProvider owns the fixed-target, root-owned binary activation
// transaction. Request values identify a pre-staged artifact; they are never
// interpreted as filesystem paths.
type UpdateProvider interface {
	Stage(context.Context, UpdateStageRequest) (UpdateStageResult, error)
	Activate(context.Context, UpdateActivateRequest) (UpdateResult, error)
	Rollback(context.Context, UpdateRollbackRequest) (UpdateResult, error)
}

type ProviderError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e *ProviderError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func providerError(err error) *ResponseError {
	if err == nil {
		return nil
	}
	var typed *ProviderError
	if errors.As(err, &typed) {
		code := stableProviderErrorCode(typed.Code)
		message := typed.Message
		if message == "" {
			message = "helper provider failed"
		}
		return &ResponseError{Code: code, Message: message, Retryable: typed.Retryable}
	}
	return &ResponseError{Code: ErrorInternal, Message: "helper provider failed"}
}

func stableProviderErrorCode(code string) string {
	switch code {
	case ErrorProviderUnavailable, ErrorDeadlineExceeded, ErrorArtifactInvalid, ErrorStateConflict:
		return code
	default:
		return ErrorInternal
	}
}
