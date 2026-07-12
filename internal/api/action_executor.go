package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type actionAgentCommander interface {
	ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error)
	GetAgentForHost(hostname string) (string, bool)
	IsAgentConnected(agentID string) bool
}

type actionHandlerProvider interface {
	ActionHandlerNames() []string
}

type routedActionExecutor struct {
	resources *ResourceHandlers
	byHandler map[string]ActionExecutor
}

func newRoutedActionExecutor(resources *ResourceHandlers, executors ...ActionExecutor) ActionExecutor {
	if resources == nil {
		return nil
	}
	routed := routedActionExecutor{
		resources: resources,
		byHandler: map[string]ActionExecutor{},
	}
	for _, executor := range executors {
		if executor == nil {
			continue
		}
		provider, ok := executor.(actionHandlerProvider)
		if !ok {
			continue
		}
		for _, handler := range provider.ActionHandlerNames() {
			handler = strings.TrimSpace(handler)
			if handler != "" {
				routed.byHandler[handler] = executor
			}
		}
	}
	if len(routed.byHandler) == 0 {
		return nil
	}
	return routed
}

func (e routedActionExecutor) ExecuteAction(ctx context.Context, record unified.ActionAuditRecord) (*unified.ExecutionResult, error) {
	normalized, err := unified.NormalizeActionAuditRecord(record)
	if err != nil {
		return nil, err
	}
	executor, err := e.executorForAction(ctx, normalized.Request)
	if err != nil {
		return nil, err
	}
	return executor.ExecuteAction(ctx, normalized)
}

func (e routedActionExecutor) BindActionDispatch(ctx context.Context, record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt) (unified.ActionDispatchAttempt, error) {
	executor, err := e.executorForAction(ctx, record.Request)
	if err != nil {
		return unified.ActionDispatchAttempt{}, err
	}
	binder, ok := executor.(actionlifecycle.DispatchBinder)
	if !ok {
		return attempt, nil
	}
	return binder.BindActionDispatch(ctx, record, attempt)
}

func (e routedActionExecutor) ReconcileActionDispatch(ctx context.Context, record unified.ActionAuditRecord, attempt unified.ActionDispatchAttempt) (*unified.ExecutionResult, unified.ActionDispatchReceipt, bool, error) {
	executor, err := e.executorForAction(ctx, record.Request)
	if err != nil {
		return nil, unified.ActionDispatchReceipt{}, false, err
	}
	reconciler, ok := executor.(actionlifecycle.DispatchReconciler)
	if !ok {
		return nil, unified.ActionDispatchReceipt{}, false, nil
	}
	return reconciler.ReconcileActionDispatch(ctx, record, attempt)
}

func (e routedActionExecutor) CheckActionAvailable(ctx context.Context, req unified.ActionRequest, resource unified.Resource) unified.ResourceActionReadiness {
	capability, ok := resourceCapabilityByName(resource.Capabilities, req.CapabilityName)
	if !ok || strings.TrimSpace(capability.InternalHandler) == "" {
		return unified.ResourceActionReadiness{}
	}
	executor := e.byHandler[strings.TrimSpace(capability.InternalHandler)]
	if executor == nil {
		return unified.ResourceActionReadiness{
			Name:       strings.TrimSpace(req.CapabilityName),
			Available:  false,
			ReasonCode: "unsupported_handler",
			Reason:     "This action is not routed through a supported executor.",
		}
	}
	checker, ok := executor.(ActionAvailabilityChecker)
	if !ok {
		return unified.ResourceActionReadiness{}
	}
	return checker.CheckActionAvailable(ctx, req, resource)
}

func (e routedActionExecutor) executorForAction(ctx context.Context, req unified.ActionRequest) (ActionExecutor, error) {
	if e.resources == nil {
		return nil, fmt.Errorf("resource handler unavailable")
	}
	registry, err := e.resources.buildRegistry(GetOrgID(ctx))
	if err != nil {
		return nil, err
	}
	resource, ok := registry.Get(req.ResourceID)
	if !ok || resource == nil {
		return nil, fmt.Errorf("resource %q is no longer present", req.ResourceID)
	}
	capability, ok := resourceCapabilityByName(resource.Capabilities, req.CapabilityName)
	if !ok {
		return nil, fmt.Errorf("resource %q does not currently advertise %s capability", req.ResourceID, req.CapabilityName)
	}
	handler := strings.TrimSpace(capability.InternalHandler)
	if handler == "" {
		return nil, fmt.Errorf("resource %q capability %s has no executor handler", req.ResourceID, req.CapabilityName)
	}
	executor := e.byHandler[handler]
	if executor == nil {
		return nil, fmt.Errorf("resource %q capability %s is routed through unsupported handler %q", req.ResourceID, req.CapabilityName, handler)
	}
	return executor, nil
}

func resourceCapabilityByName(capabilities []unified.ResourceCapability, name string) (unified.ResourceCapability, bool) {
	name = strings.TrimSpace(name)
	for _, capability := range capabilities {
		if strings.TrimSpace(capability.Name) == name {
			return capability, true
		}
	}
	return unified.ResourceCapability{}, false
}
