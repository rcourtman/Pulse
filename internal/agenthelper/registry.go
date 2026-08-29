package agenthelper

import (
	"context"
	"encoding/json"
	"sort"
)

type operationHandler func(context.Context, json.RawMessage) (json.RawMessage, *ResponseError)

type operationKey struct {
	name    string
	version int
}

type registeredOperation struct {
	name      string
	version   int
	available bool
	handle    operationHandler
}

// Registry is a closed registry of the helper operations Pulse defines. It
// intentionally has no public Register method: a caller cannot add a generic
// exec, path-read, or daemon-proxy primitive at runtime.
type Registry struct {
	operations map[operationKey]registeredOperation
}

type Capability struct {
	Operation string `json:"operation"`
	Version   int    `json:"version"`
	Available bool   `json:"available"`
}

type CapabilitiesResult struct {
	ProtocolVersion int          `json:"protocolVersion"`
	Operations      []Capability `json:"operations"`
}

type HealthResult struct {
	Status          string `json:"status"`
	ProtocolVersion int    `json:"protocolVersion"`
}

type emptyOperationRequest struct{}

type Providers struct {
	Containers ContainerProvider
	Updates    UpdateProvider
}

func NewRegistry(smart SMARTProvider, proxmox ProxmoxProvider) *Registry {
	return NewRegistryWithProviders(smart, proxmox, Providers{})
}

func NewRegistryWithProviders(smart SMARTProvider, proxmox ProxmoxProvider, providers Providers) *Registry {
	registry := &Registry{operations: make(map[operationKey]registeredOperation)}
	registry.add(OperationHealth, OperationVersion1, true, func(_ context.Context, payload json.RawMessage) (json.RawMessage, *ResponseError) {
		var request emptyOperationRequest
		if err := decodePayload(payload, &request); err != nil {
			return nil, invalidPayloadError(err)
		}
		return marshalOperationResult(HealthResult{Status: "ok", ProtocolVersion: ProtocolVersion})
	})
	registry.add(OperationCapabilities, OperationVersion1, true, func(_ context.Context, payload json.RawMessage) (json.RawMessage, *ResponseError) {
		var request emptyOperationRequest
		if err := decodePayload(payload, &request); err != nil {
			return nil, invalidPayloadError(err)
		}
		return marshalOperationResult(registry.capabilities())
	})
	registry.add(OperationSMARTSnapshot, OperationVersion1, smart != nil, func(ctx context.Context, payload json.RawMessage) (json.RawMessage, *ResponseError) {
		var request emptyOperationRequest
		if err := decodePayload(payload, &request); err != nil {
			return nil, invalidPayloadError(err)
		}
		if smart == nil {
			return nil, unavailableError("SMART snapshot provider is not configured")
		}
		result, err := smart.Snapshot(ctx)
		return validateProviderResult(result, err)
	})
	registry.add(OperationProxmoxLXCFilesystems, OperationVersion1, proxmox != nil, func(ctx context.Context, payload json.RawMessage) (json.RawMessage, *ResponseError) {
		var request emptyOperationRequest
		if err := decodePayload(payload, &request); err != nil {
			return nil, invalidPayloadError(err)
		}
		if proxmox == nil {
			return nil, unavailableError("Proxmox LXC filesystem provider is not configured")
		}
		result, err := proxmox.LXCFilesystems(ctx)
		return validateProviderResult(result, err)
	})
	registry.add(OperationContainerInventory, OperationVersion1, providers.Containers != nil, func(ctx context.Context, payload json.RawMessage) (json.RawMessage, *ResponseError) {
		var request emptyOperationRequest
		if err := decodePayload(payload, &request); err != nil {
			return nil, invalidPayloadError(err)
		}
		if providers.Containers == nil {
			return nil, unavailableError("container runtime inventory provider is not configured")
		}
		result, err := providers.Containers.Inventory(ctx)
		return validateProviderResult(result, err)
	})
	registry.add(OperationAgentUpdateStage, OperationVersion1, providers.Updates != nil, func(ctx context.Context, payload json.RawMessage) (json.RawMessage, *ResponseError) {
		var request UpdateStageRequest
		if err := decodePayload(payload, &request); err != nil {
			return nil, invalidPayloadError(err)
		}
		if providers.Updates == nil {
			return nil, unavailableError("agent update provider is not configured")
		}
		result, err := providers.Updates.Stage(ctx, request)
		if err != nil {
			return nil, providerError(err)
		}
		return marshalOperationResult(result)
	})
	registry.add(OperationAgentUpdateActivate, OperationVersion1, providers.Updates != nil, func(ctx context.Context, payload json.RawMessage) (json.RawMessage, *ResponseError) {
		var request UpdateActivateRequest
		if err := decodePayload(payload, &request); err != nil {
			return nil, invalidPayloadError(err)
		}
		if providers.Updates == nil {
			return nil, unavailableError("agent update provider is not configured")
		}
		result, err := providers.Updates.Activate(ctx, request)
		if err != nil {
			return nil, providerError(err)
		}
		return marshalOperationResult(result)
	})
	registry.add(OperationAgentUpdateRollback, OperationVersion1, providers.Updates != nil, func(ctx context.Context, payload json.RawMessage) (json.RawMessage, *ResponseError) {
		var request UpdateRollbackRequest
		if err := decodePayload(payload, &request); err != nil {
			return nil, invalidPayloadError(err)
		}
		if providers.Updates == nil {
			return nil, unavailableError("agent update provider is not configured")
		}
		result, err := providers.Updates.Rollback(ctx, request)
		if err != nil {
			return nil, providerError(err)
		}
		return marshalOperationResult(result)
	})
	return registry
}

func (r *Registry) add(name string, version int, available bool, handler operationHandler) {
	r.operations[operationKey{name: name, version: version}] = registeredOperation{
		name: name, version: version, available: available, handle: handler,
	}
}

func (r *Registry) lookup(name string, version int) (registeredOperation, bool) {
	operation, ok := r.operations[operationKey{name: name, version: version}]
	return operation, ok
}

func (r *Registry) hasName(name string) bool {
	for key := range r.operations {
		if key.name == name {
			return true
		}
	}
	return false
}

func (r *Registry) capabilities() CapabilitiesResult {
	capabilities := make([]Capability, 0, len(r.operations))
	for _, operation := range r.operations {
		capabilities = append(capabilities, Capability{
			Operation: operation.name,
			Version:   operation.version,
			Available: operation.available,
		})
	}
	sort.Slice(capabilities, func(i, j int) bool {
		if capabilities[i].Operation == capabilities[j].Operation {
			return capabilities[i].Version < capabilities[j].Version
		}
		return capabilities[i].Operation < capabilities[j].Operation
	})
	return CapabilitiesResult{ProtocolVersion: ProtocolVersion, Operations: capabilities}
}

func marshalOperationResult(value any) (json.RawMessage, *ResponseError) {
	result, err := json.Marshal(value)
	if err != nil {
		return nil, &ResponseError{Code: ErrorInternal, Message: "encode helper result"}
	}
	return result, nil
}

func validateProviderResult(result json.RawMessage, err error) (json.RawMessage, *ResponseError) {
	if err != nil {
		return nil, providerError(err)
	}
	if len(result) == 0 || !json.Valid(result) {
		return nil, &ResponseError{Code: ErrorInternal, Message: "helper provider returned invalid JSON"}
	}
	return result, nil
}

func invalidPayloadError(err error) *ResponseError {
	return &ResponseError{Code: ErrorInvalidRequest, Message: "invalid operation payload: " + err.Error()}
}

func unavailableError(message string) *ResponseError {
	return &ResponseError{Code: ErrorProviderUnavailable, Message: message, Retryable: false}
}
