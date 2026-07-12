package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
)

const (
	// MaxServiceToolIterations is the provider-call budget for the legacy
	// Service tool loop. Both streaming and non-streaming orchestration consume
	// this same budget; a request never relies on a wall-clock timeout to stop a
	// provider that keeps returning tool calls.
	MaxServiceToolIterations = 10

	ServiceToolLoopFailureCode = "TOOL_LOOP_BOUNDED_FAILURE"
)

type ServiceToolLoopFailureReason string

const (
	ServiceToolLoopFailureRepeatedDenied ServiceToolLoopFailureReason = "repeated_denied_tool_call"
	ServiceToolLoopFailureMaxIterations  ServiceToolLoopFailureReason = "max_tool_iterations"
)

// ServiceToolLoopError is the typed terminal result for a bounded legacy
// Service model/tool loop. ProviderCalls is the exact number of provider calls
// already made; no additional provider or tool work occurs after this error.
type ServiceToolLoopError struct {
	Reason           ServiceToolLoopFailureReason
	ProviderCalls    int
	MaxProviderCalls int
	ToolName         string
}

func (e *ServiceToolLoopError) Error() string {
	if e == nil {
		return "Pulse Assistant tool loop terminated"
	}
	switch e.Reason {
	case ServiceToolLoopFailureRepeatedDenied:
		return fmt.Sprintf("Pulse Assistant tool loop terminated after provider repeated denied tool %q (%d provider calls)", e.ToolName, e.ProviderCalls)
	case ServiceToolLoopFailureMaxIterations:
		return fmt.Sprintf("Pulse Assistant tool loop reached the maximum of %d provider calls", e.MaxProviderCalls)
	default:
		return fmt.Sprintf("Pulse Assistant tool loop terminated after %d provider calls", e.ProviderCalls)
	}
}

// ServiceToolLoopFailureData is the stable typed stream projection of
// ServiceToolLoopError.
type ServiceToolLoopFailureData struct {
	Code             string                       `json:"code"`
	Reason           ServiceToolLoopFailureReason `json:"reason"`
	Message          string                       `json:"message"`
	ProviderCalls    int                          `json:"provider_calls"`
	MaxProviderCalls int                          `json:"max_provider_calls"`
	ToolName         string                       `json:"tool_name,omitempty"`
}

func newServiceToolLoopFailureData(err *ServiceToolLoopError) ServiceToolLoopFailureData {
	return ServiceToolLoopFailureData{
		Code:             ServiceToolLoopFailureCode,
		Reason:           err.Reason,
		Message:          err.Error(),
		ProviderCalls:    err.ProviderCalls,
		MaxProviderCalls: err.MaxProviderCalls,
		ToolName:         err.ToolName,
	}
}

type serviceToolLoopGuard struct {
	providerCalls int
	maxCalls      int
	deniedCalls   map[string]struct{}
}

func newServiceToolLoopGuard() *serviceToolLoopGuard {
	return &serviceToolLoopGuard{
		maxCalls:    MaxServiceToolIterations,
		deniedCalls: make(map[string]struct{}),
	}
}

func (g *serviceToolLoopGuard) beforeProviderCall() error {
	if g.providerCalls >= g.maxCalls {
		return &ServiceToolLoopError{
			Reason:           ServiceToolLoopFailureMaxIterations,
			ProviderCalls:    g.providerCalls,
			MaxProviderCalls: g.maxCalls,
		}
	}
	g.providerCalls++
	return nil
}

func (g *serviceToolLoopGuard) observeDeniedToolCall(call providers.ToolCall) error {
	fingerprint := serviceToolCallFingerprint(call)
	if _, repeated := g.deniedCalls[fingerprint]; repeated {
		return &ServiceToolLoopError{
			Reason:           ServiceToolLoopFailureRepeatedDenied,
			ProviderCalls:    g.providerCalls,
			MaxProviderCalls: g.maxCalls,
			ToolName:         strings.TrimSpace(call.Name),
		}
	}
	g.deniedCalls[fingerprint] = struct{}{}
	return nil
}

func serviceToolCallFingerprint(call providers.ToolCall) string {
	input, err := json.Marshal(call.Input)
	if err != nil {
		input = []byte(fmt.Sprintf("%#v", call.Input))
	}
	sum := sha256.Sum256(append([]byte(strings.TrimSpace(call.Name)+"\x00"), input...))
	return hex.EncodeToString(sum[:])
}

func (s *Service) observeDeniedServiceToolCalls(guard *serviceToolLoopGuard, calls []providers.ToolCall) error {
	for _, call := range calls {
		if !s.isDeniedServiceToolCall(call) {
			continue
		}
		if err := guard.observeDeniedToolCall(call); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) isDeniedServiceToolCall(call providers.ToolCall) bool {
	class := agentcapabilities.ClassifyLegacyAssistantInvocation(call.Name)
	if !s.legacyAssistantInvocationPolicy().Allows(call.Name, class) {
		return true
	}
	if call.Name != agentcapabilities.LegacyAssistantRunCommandToolName || s.policy == nil {
		return false
	}
	command, _ := call.Input[agentcapabilities.LegacyAssistantCommandArgumentName].(string)
	return s.policy.Evaluate(command) == agentexec.PolicyBlock
}

func emitServiceToolLoopFailure(callback StreamCallback, err error) {
	var loopErr *ServiceToolLoopError
	if !errors.As(err, &loopErr) || callback == nil {
		return
	}
	callback(StreamEvent{Type: "error", Data: newServiceToolLoopFailureData(loopErr)})
}
