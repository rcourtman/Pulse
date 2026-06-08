package modelboundary

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// UnifiedResourceProvider is the minimal policy source needed to sanitize
// provider-bound model requests.
type UnifiedResourceProvider interface {
	GetByType(t unifiedresources.ResourceType) []unifiedresources.Resource
}

type allUnifiedResourceProvider interface {
	GetAll() []unifiedresources.Resource
}

type requestSanitizerOptions struct {
	resourcePolicyAllowedText []string
	localOnlyFloorOnly        bool
}

// RequestSanitizerOption scopes model-bound sanitizer behavior for
// Pulse-originated exports. Options must never be derived from raw user text.
type RequestSanitizerOption func(*requestSanitizerOptions)

// AllowResourcePolicyText preserves exact Pulse-generated text spans from the
// resource-policy text redactor while still applying prompt-secret sanitation.
func AllowResourcePolicyText(values ...string) RequestSanitizerOption {
	return func(opts *requestSanitizerOptions) {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			opts.resourcePolicyAllowedText = append(opts.resourcePolicyAllowedText, value)
		}
	}
}

// RedactLocalOnlyResourcesOnly narrows the resource-identifier redaction pass to
// the hard floor: resources the policy engine routes local-only (Restricted
// sensitivity / "never leaves the local trust boundary"). Identifiers for all
// other resources — ordinary (Internal) and Sensitive (local-first) — may then
// reach a cloud model. It is the model-boundary half of the "full"
// cloud_context_privacy level: the operator chose to share real infrastructure
// detail, but an explicit local-only classification still must not be overridden
// by a blanket dial. Prompt-secret sanitation (API keys, passwords, tokens) ALWAYS
// still runs — credentials never cross the model boundary regardless of this
// option. Callers must only set this from the persisted privacy dial, never from
// raw user text.
func RedactLocalOnlyResourcesOnly() RequestSanitizerOption {
	return func(opts *requestSanitizerOptions) {
		opts.localOnlyFloorOnly = true
	}
}

// RequestSanitizerForModel returns a sanitizer for non-local model traffic.
// It is intentionally applied at the final provider transport boundary so
// operator-entered prompts, handoff text, tool-result turns, and provider-bound
// tool schemas cannot bypass prompt-secret or resource-policy sanitation.
func RequestSanitizerForModel(model string, provider UnifiedResourceProvider, opts ...RequestSanitizerOption) func(providers.ChatRequest) providers.ChatRequest {
	if !ModelUsesExternalProvider(model) {
		return nil
	}
	options := requestSanitizerOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	// At the "full" dial the operator opted into sharing real resource identifiers,
	// so the redaction pass narrows to the local-only floor (resources that must
	// never leave the local trust boundary); everything else flows. Otherwise the
	// pass covers every policied resource. Prompt-secret sanitation below always runs.
	resources := resourcePolicySanitizerResources(provider)
	if options.localOnlyFloorOnly {
		resources = localOnlyRoutedResources(resources)
	}
	return func(req providers.ChatRequest) providers.ChatRequest {
		req = sanitizeProviderRequestForPromptSecrets(req)
		if len(resources) == 0 {
			return req
		}
		allowedText := sanitizeResourcePolicyAllowedText(options.resourcePolicyAllowedText)
		return sanitizeProviderRequestForResources(req, resources, allowedText)
	}
}

// ModelUsesExternalProvider reports whether a model string routes outside the
// local Ollama trust boundary.
func ModelUsesExternalProvider(model string) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	provider, _ := config.ParseModelString(model)
	return provider != config.AIProviderOllama
}

func resourcePolicySanitizerResources(provider UnifiedResourceProvider) []unifiedresources.Resource {
	if provider == nil {
		return nil
	}
	if allProvider, ok := provider.(allUnifiedResourceProvider); ok {
		return resourcesWithPolicy(unifiedresources.RefreshCanonicalMetadataSlice(allProvider.GetAll()))
	}

	resourceTypes := []unifiedresources.ResourceType{
		unifiedresources.ResourceTypeAgent,
		unifiedresources.ResourceTypeVM,
		unifiedresources.ResourceTypeSystemContainer,
		unifiedresources.ResourceTypeAppContainer,
		unifiedresources.ResourceTypeDockerService,
		unifiedresources.ResourceTypeK8sCluster,
		unifiedresources.ResourceTypeK8sNode,
		unifiedresources.ResourceTypePod,
		unifiedresources.ResourceTypeK8sDeployment,
		unifiedresources.ResourceTypeStorage,
		unifiedresources.ResourceTypePBS,
		unifiedresources.ResourceTypePMG,
		unifiedresources.ResourceTypeCeph,
		unifiedresources.ResourceTypePhysicalDisk,
	}

	var resources []unifiedresources.Resource
	seen := make(map[string]struct{})
	for _, resourceType := range resourceTypes {
		for _, resource := range provider.GetByType(resourceType) {
			key := string(resource.Type) + "\x00" + resource.ID
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			resources = append(resources, resource)
		}
	}
	return resourcesWithPolicy(unifiedresources.RefreshCanonicalMetadataSlice(resources))
}

func resourcesWithPolicy(resources []unifiedresources.Resource) []unifiedresources.Resource {
	filtered := make([]unifiedresources.Resource, 0, len(resources))
	for _, resource := range resources {
		if resource.Policy == nil {
			continue
		}
		filtered = append(filtered, resource)
	}
	return filtered
}

// localOnlyRoutedResources keeps only resources the policy engine routes
// local-only — the "never leaves the local trust boundary" floor that the "full"
// cloud_context_privacy dial must not override.
func localOnlyRoutedResources(resources []unifiedresources.Resource) []unifiedresources.Resource {
	filtered := make([]unifiedresources.Resource, 0, len(resources))
	for _, resource := range resources {
		if resource.Policy == nil {
			continue
		}
		if resource.Policy.Routing.Scope == unifiedresources.ResourceRoutingScopeLocalOnly {
			filtered = append(filtered, resource)
		}
	}
	return filtered
}

func sanitizeProviderRequestForResources(req providers.ChatRequest, resources []unifiedresources.Resource, allowedText []string) providers.ChatRequest {
	if len(resources) == 0 {
		return req
	}
	req.System = sanitizeResourcePolicyText(req.System, resources, allowedText)

	if len(req.Messages) > 0 {
		req.Messages = append([]providers.Message(nil), req.Messages...)
		for i := range req.Messages {
			req.Messages[i] = sanitizeProviderMessageForResources(req.Messages[i], resources, allowedText)
		}
	}
	if len(req.Tools) > 0 {
		req.Tools = append([]providers.Tool(nil), req.Tools...)
		for i := range req.Tools {
			req.Tools[i] = sanitizeProviderToolForResources(req.Tools[i], resources, allowedText)
		}
	}
	return req
}

func sanitizeProviderMessageForResources(msg providers.Message, resources []unifiedresources.Resource, allowedText []string) providers.Message {
	msg.Content = sanitizeResourcePolicyText(msg.Content, resources, allowedText)
	msg.ReasoningContent = sanitizeResourcePolicyText(msg.ReasoningContent, resources, allowedText)
	if msg.ToolResult != nil {
		toolResult := *msg.ToolResult
		toolResult.Content = sanitizeResourcePolicyText(toolResult.Content, resources, allowedText)
		msg.ToolResult = &toolResult
	}
	if len(msg.ToolCalls) > 0 {
		msg.ToolCalls = append([]providers.ToolCall(nil), msg.ToolCalls...)
		for i := range msg.ToolCalls {
			msg.ToolCalls[i].Input = sanitizeResourcePolicyMap(msg.ToolCalls[i].Input, resources, allowedText)
		}
	}
	return msg
}

func sanitizeProviderToolForResources(tool providers.Tool, resources []unifiedresources.Resource, allowedText []string) providers.Tool {
	tool.Description = sanitizeResourcePolicyText(tool.Description, resources, allowedText)
	tool.InputSchema = sanitizeResourcePolicyMap(tool.InputSchema, resources, allowedText)
	return tool
}

func sanitizeResourcePolicyText(value string, resources []unifiedresources.Resource, allowedText []string) string {
	if strings.TrimSpace(value) == "" || len(resources) == 0 {
		return value
	}
	protected, restore := protectResourcePolicyAllowedText(value, allowedText)
	redacted := protected
	for _, resource := range resources {
		redacted = unifiedresources.ResourcePolicyRedactedText(redacted, resource)
	}
	for placeholder, original := range restore {
		redacted = strings.ReplaceAll(redacted, placeholder, original)
	}
	return redacted
}

func sanitizeResourcePolicyAllowedText(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(sanitizePromptSecretText(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sortStringsByLengthDesc(out)
	return out
}

func protectResourcePolicyAllowedText(value string, allowedText []string) (string, map[string]string) {
	if strings.TrimSpace(value) == "" || len(allowedText) == 0 {
		return value, nil
	}
	protected := value
	restore := make(map[string]string)
	for idx, allowed := range allowedText {
		if allowed == "" || !strings.Contains(protected, allowed) {
			continue
		}
		placeholder := fmt.Sprintf("\x00pulse-allowed-resource-export-%d\x00", idx)
		protected = strings.ReplaceAll(protected, allowed, placeholder)
		restore[placeholder] = allowed
	}
	return protected, restore
}

func sortStringsByLengthDesc(values []string) {
	sort.Slice(values, func(i, j int) bool {
		if len(values[i]) == len(values[j]) {
			return values[i] < values[j]
		}
		return len(values[i]) > len(values[j])
	})
}

func sanitizeResourcePolicyMap(values map[string]interface{}, resources []unifiedresources.Resource, allowedText []string) map[string]interface{} {
	if len(values) == 0 {
		return values
	}
	sanitized := make(map[string]interface{}, len(values))
	for key, value := range values {
		sanitized[key] = sanitizeResourcePolicyValue(value, resources, allowedText)
	}
	return sanitized
}

func sanitizeResourcePolicyValue(value interface{}, resources []unifiedresources.Resource, allowedText []string) interface{} {
	switch typed := value.(type) {
	case string:
		return sanitizeResourcePolicyText(typed, resources, allowedText)
	case []string:
		out := make([]string, len(typed))
		for i := range typed {
			out[i] = sanitizeResourcePolicyText(typed[i], resources, allowedText)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i := range typed {
			out[i] = sanitizeResourcePolicyValue(typed[i], resources, allowedText)
		}
		return out
	case map[string]interface{}:
		return sanitizeResourcePolicyMap(typed, resources, allowedText)
	case map[string]string:
		out := make(map[string]string, len(typed))
		for key, nested := range typed {
			out[key] = sanitizeResourcePolicyText(nested, resources, allowedText)
		}
		return out
	default:
		return value
	}
}

func sanitizeProviderRequestForPromptSecrets(req providers.ChatRequest) providers.ChatRequest {
	req.System = sanitizePromptSecretText(req.System)

	if len(req.Messages) > 0 {
		req.Messages = append([]providers.Message(nil), req.Messages...)
		for i := range req.Messages {
			req.Messages[i] = sanitizeProviderMessageForPromptSecrets(req.Messages[i])
		}
	}
	if len(req.Tools) > 0 {
		req.Tools = append([]providers.Tool(nil), req.Tools...)
		for i := range req.Tools {
			req.Tools[i] = sanitizeProviderToolForPromptSecrets(req.Tools[i])
		}
	}
	return req
}

func sanitizeProviderMessageForPromptSecrets(msg providers.Message) providers.Message {
	msg.Content = sanitizePromptSecretText(msg.Content)
	msg.ReasoningContent = sanitizePromptSecretText(msg.ReasoningContent)
	if msg.ToolResult != nil {
		toolResult := *msg.ToolResult
		toolResult.Content = sanitizePromptSecretText(toolResult.Content)
		msg.ToolResult = &toolResult
	}
	if len(msg.ToolCalls) > 0 {
		msg.ToolCalls = append([]providers.ToolCall(nil), msg.ToolCalls...)
		for i := range msg.ToolCalls {
			msg.ToolCalls[i].Input = sanitizePromptSecretMap(msg.ToolCalls[i].Input, false)
		}
	}
	return msg
}

func sanitizeProviderToolForPromptSecrets(tool providers.Tool) providers.Tool {
	tool.Description = sanitizePromptSecretText(tool.Description)
	tool.InputSchema = sanitizePromptSecretMap(tool.InputSchema, false)
	return tool
}

func sanitizePromptSecretText(value string) string {
	redacted, _ := safety.RedactSensitiveText(value)
	return redacted
}

func sanitizePromptSecretSensitiveValue(value string) string {
	redacted, _ := safety.RedactSensitiveValue(value)
	return redacted
}

func sanitizePromptSecretMap(values map[string]interface{}, sensitiveParent bool) map[string]interface{} {
	if len(values) == 0 {
		return values
	}
	sanitized := make(map[string]interface{}, len(values))
	for key, value := range values {
		keyIsSensitive := safety.IsSensitiveFieldName(key)
		valueIsSensitive := keyIsSensitive || sensitiveParent && safety.IsSensitiveValueCarrierFieldName(key)
		sanitized[key] = sanitizePromptSecretValue(key, value, valueIsSensitive)
	}
	return sanitized
}

func sanitizePromptSecretValue(fieldName string, value interface{}, sensitiveValue bool) interface{} {
	switch typed := value.(type) {
	case string:
		if sensitiveValue {
			return sanitizePromptSecretSensitiveValue(typed)
		}
		return sanitizePromptSecretText(typed)
	case []string:
		out := make([]string, len(typed))
		for i := range typed {
			if sensitiveValue {
				out[i] = sanitizePromptSecretSensitiveValue(typed[i])
				continue
			}
			out[i] = sanitizePromptSecretText(typed[i])
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i := range typed {
			out[i] = sanitizePromptSecretValue(fieldName, typed[i], sensitiveValue)
		}
		return out
	case map[string]interface{}:
		return sanitizePromptSecretMap(typed, sensitiveValue)
	case map[string]string:
		out := make(map[string]string, len(typed))
		for key, nested := range typed {
			keyIsSensitive := safety.IsSensitiveFieldName(key)
			valueIsSensitive := keyIsSensitive || sensitiveValue && safety.IsSensitiveValueCarrierFieldName(key)
			if valueIsSensitive {
				out[key] = sanitizePromptSecretSensitiveValue(nested)
				continue
			}
			out[key] = sanitizePromptSecretText(nested)
		}
		return out
	default:
		return value
	}
}
