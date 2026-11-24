package modelboundary

import (
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

// RequestSanitizerForModel returns a sanitizer for non-local model traffic.
// It is intentionally applied at the final provider transport boundary so
// operator-entered prompts, handoff text, tool-result turns, and provider-bound
// tool schemas cannot bypass prompt-secret or resource-policy sanitation.
//
// Pulse is a self-hosted homelab/SMB tool: a cloud-routed model receives real
// infrastructure context (names, IPs, config) so the Assistant is actually
// useful — the operator chose a cloud model. Two invariants always hold at this
// boundary regardless: prompt-secret sanitation strips credentials (API keys,
// passwords, tokens), and the resource-identifier redaction is scoped to the
// local-only floor — resources the policy engine routes local-only (Restricted:
// tagged secret/pii, PMG, k8s-secrets, ...) never leave the local trust boundary.
// Everything else (Internal and Sensitive) flows. Local (Ollama) models skip the
// sanitizer entirely.
func RequestSanitizerForModel(model string, provider UnifiedResourceProvider) func(providers.ChatRequest) providers.ChatRequest {
	if !ModelUsesExternalProvider(model) {
		return nil
	}
	governed := resourcePolicySanitizerResources(provider)
	resources := localOnlyRoutedResources(governed)
	redactor := unifiedresources.NewResourcePolicyTextRedactor(resources, modelBoundResourceIDs(governed))
	return func(req providers.ChatRequest) providers.ChatRequest {
		req = sanitizeProviderRequestForPromptSecrets(req)
		if len(resources) == 0 {
			return req
		}
		return sanitizeProviderRequestForResources(req, redactor)
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
// local-only — the "never leaves the local trust boundary" floor for cloud-bound
// model requests.
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

// modelBoundResourceIDs collects the ID of every resource that is not
// local-only. An ID is the handle the model needs to look that resource up, so
// a local-only resource that happens to carry the same string - a Kubernetes
// Secret named after a host's ID - must not take it out of the request. Names
// are not collected: a name that a local-only resource shares with another
// resource stays behind the local-only floor.
func modelBoundResourceIDs(resources []unifiedresources.Resource) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, resource := range resources {
		if resource.Policy == nil || resource.Policy.Routing.Scope == unifiedresources.ResourceRoutingScopeLocalOnly {
			continue
		}
		if id := strings.TrimSpace(resource.ID); id != "" {
			ids[id] = struct{}{}
		}
	}
	return ids
}

func sanitizeProviderRequestForResources(req providers.ChatRequest, redactor *unifiedresources.ResourcePolicyTextRedactor) providers.ChatRequest {
	req.System = sanitizeResourcePolicyText(req.System, redactor)

	if len(req.Messages) > 0 {
		req.Messages = append([]providers.Message(nil), req.Messages...)
		for i := range req.Messages {
			req.Messages[i] = sanitizeProviderMessageForResources(req.Messages[i], redactor)
		}
	}
	if len(req.Tools) > 0 {
		req.Tools = append([]providers.Tool(nil), req.Tools...)
		for i := range req.Tools {
			req.Tools[i] = sanitizeProviderToolForResources(req.Tools[i], redactor)
		}
	}
	return req
}

func sanitizeProviderMessageForResources(msg providers.Message, redactor *unifiedresources.ResourcePolicyTextRedactor) providers.Message {
	msg.Content = sanitizeResourcePolicyText(msg.Content, redactor)
	msg.ReasoningContent = sanitizeResourcePolicyText(msg.ReasoningContent, redactor)
	if msg.ToolResult != nil {
		toolResult := *msg.ToolResult
		toolResult.Content = sanitizeResourcePolicyText(toolResult.Content, redactor)
		msg.ToolResult = &toolResult
	}
	if len(msg.ToolCalls) > 0 {
		msg.ToolCalls = append([]providers.ToolCall(nil), msg.ToolCalls...)
		for i := range msg.ToolCalls {
			msg.ToolCalls[i].Input = sanitizeResourcePolicyMap(msg.ToolCalls[i].Input, redactor)
		}
	}
	return msg
}

func sanitizeProviderToolForResources(tool providers.Tool, redactor *unifiedresources.ResourcePolicyTextRedactor) providers.Tool {
	tool.Description = sanitizeResourcePolicyText(tool.Description, redactor)
	tool.InputSchema = sanitizeResourcePolicySchema(tool.InputSchema, redactor)
	return tool
}

func sanitizeResourcePolicyText(value string, redactor *unifiedresources.ResourcePolicyTextRedactor) string {
	if strings.TrimSpace(value) == "" {
		return value
	}
	return redactor.RedactedText(value)
}

// sanitizeResourcePolicySchema redacts a tool's JSON Schema. The values of the
// "type" and "required" keywords are JSON Schema type names and the schema's
// own property names, which are sent as keys anyway. They are kept as they
// are: a resource named "object" or "string" must not turn the schema into one
// the provider rejects.
func sanitizeResourcePolicySchema(schema map[string]interface{}, redactor *unifiedresources.ResourcePolicyTextRedactor) map[string]interface{} {
	if len(schema) == 0 {
		return schema
	}
	sanitized := make(map[string]interface{}, len(schema))
	for key, value := range schema {
		sanitized[key] = sanitizeResourcePolicySchemaValue(key, value, redactor)
	}
	return sanitized
}

func sanitizeResourcePolicySchemaValue(keyword string, value interface{}, redactor *unifiedresources.ResourcePolicyTextRedactor) interface{} {
	// A map is a nested schema, also when it belongs to a property that is
	// itself called "type" or "required".
	if nested, ok := value.(map[string]interface{}); ok {
		return sanitizeResourcePolicySchema(nested, redactor)
	}
	if keyword == "type" || keyword == "required" {
		return value
	}
	if list, ok := value.([]interface{}); ok {
		out := make([]interface{}, len(list))
		for i := range list {
			out[i] = sanitizeResourcePolicySchemaValue(keyword, list[i], redactor)
		}
		return out
	}
	return sanitizeResourcePolicyValue(value, redactor)
}

func sanitizeResourcePolicyMap(values map[string]interface{}, redactor *unifiedresources.ResourcePolicyTextRedactor) map[string]interface{} {
	if len(values) == 0 {
		return values
	}
	sanitized := make(map[string]interface{}, len(values))
	for key, value := range values {
		sanitized[key] = sanitizeResourcePolicyValue(value, redactor)
	}
	return sanitized
}

func sanitizeResourcePolicyValue(value interface{}, redactor *unifiedresources.ResourcePolicyTextRedactor) interface{} {
	switch typed := value.(type) {
	case string:
		return sanitizeResourcePolicyText(typed, redactor)
	case []string:
		out := make([]string, len(typed))
		for i := range typed {
			out[i] = sanitizeResourcePolicyText(typed[i], redactor)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i := range typed {
			out[i] = sanitizeResourcePolicyValue(typed[i], redactor)
		}
		return out
	case map[string]interface{}:
		return sanitizeResourcePolicyMap(typed, redactor)
	case map[string]string:
		out := make(map[string]string, len(typed))
		for key, nested := range typed {
			out[key] = sanitizeResourcePolicyText(nested, redactor)
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
