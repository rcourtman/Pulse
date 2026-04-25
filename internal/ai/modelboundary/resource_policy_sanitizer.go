package modelboundary

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
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
// later tool-result turns cannot bypass the resource-policy posture exported
// to the operator-facing Data Handling surface.
func RequestSanitizerForModel(model string, provider UnifiedResourceProvider) func(providers.ChatRequest) providers.ChatRequest {
	if !ModelUsesExternalProvider(model) || provider == nil {
		return nil
	}
	resources := resourcePolicySanitizerResources(provider)
	if len(resources) == 0 {
		return nil
	}
	return func(req providers.ChatRequest) providers.ChatRequest {
		return sanitizeProviderRequestForResources(req, resources)
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

func sanitizeProviderRequestForResources(req providers.ChatRequest, resources []unifiedresources.Resource) providers.ChatRequest {
	if len(resources) == 0 {
		return req
	}
	req.System = sanitizeResourcePolicyText(req.System, resources)

	if len(req.Messages) > 0 {
		req.Messages = append([]providers.Message(nil), req.Messages...)
		for i := range req.Messages {
			req.Messages[i] = sanitizeProviderMessageForResources(req.Messages[i], resources)
		}
	}
	if len(req.Tools) > 0 {
		req.Tools = append([]providers.Tool(nil), req.Tools...)
		for i := range req.Tools {
			req.Tools[i] = sanitizeProviderToolForResources(req.Tools[i], resources)
		}
	}
	return req
}

func sanitizeProviderMessageForResources(msg providers.Message, resources []unifiedresources.Resource) providers.Message {
	msg.Content = sanitizeResourcePolicyText(msg.Content, resources)
	msg.ReasoningContent = sanitizeResourcePolicyText(msg.ReasoningContent, resources)
	if msg.ToolResult != nil {
		toolResult := *msg.ToolResult
		toolResult.Content = sanitizeResourcePolicyText(toolResult.Content, resources)
		msg.ToolResult = &toolResult
	}
	if len(msg.ToolCalls) > 0 {
		msg.ToolCalls = append([]providers.ToolCall(nil), msg.ToolCalls...)
		for i := range msg.ToolCalls {
			msg.ToolCalls[i].Input = sanitizeResourcePolicyMap(msg.ToolCalls[i].Input, resources)
		}
	}
	return msg
}

func sanitizeProviderToolForResources(tool providers.Tool, resources []unifiedresources.Resource) providers.Tool {
	tool.Description = sanitizeResourcePolicyText(tool.Description, resources)
	tool.InputSchema = sanitizeResourcePolicyMap(tool.InputSchema, resources)
	return tool
}

func sanitizeResourcePolicyText(value string, resources []unifiedresources.Resource) string {
	if strings.TrimSpace(value) == "" || len(resources) == 0 {
		return value
	}
	redacted := value
	for _, resource := range resources {
		redacted = unifiedresources.ResourcePolicyRedactedText(redacted, resource)
	}
	return redacted
}

func sanitizeResourcePolicyMap(values map[string]interface{}, resources []unifiedresources.Resource) map[string]interface{} {
	if len(values) == 0 {
		return values
	}
	sanitized := make(map[string]interface{}, len(values))
	for key, value := range values {
		sanitized[key] = sanitizeResourcePolicyValue(value, resources)
	}
	return sanitized
}

func sanitizeResourcePolicyValue(value interface{}, resources []unifiedresources.Resource) interface{} {
	switch typed := value.(type) {
	case string:
		return sanitizeResourcePolicyText(typed, resources)
	case []string:
		out := make([]string, len(typed))
		for i := range typed {
			out[i] = sanitizeResourcePolicyText(typed[i], resources)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i := range typed {
			out[i] = sanitizeResourcePolicyValue(typed[i], resources)
		}
		return out
	case map[string]interface{}:
		return sanitizeResourcePolicyMap(typed, resources)
	case map[string]string:
		out := make(map[string]string, len(typed))
		for key, nested := range typed {
			out[key] = sanitizeResourcePolicyText(nested, resources)
		}
		return out
	default:
		return value
	}
}
