package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestResolvePlainTextAssistantResourceReference_ReadOnlyCommand(t *testing.T) {
	provider := plainTextResourceTestProvider(plainTextResourceTestAgent("delly"))

	match, ok := resolvePlainTextAssistantResourceReference(
		"On the delly host, run the read-only command `ls /dev | wc -l` and tell me the count.",
		provider,
	)
	if !ok {
		t.Fatal("expected unambiguous plain-text resource reference to resolve")
	}
	if match.registration.Kind != "agent" {
		t.Fatalf("resolved kind = %q, want agent", match.registration.Kind)
	}
	if match.registration.Name != "delly" {
		t.Fatalf("resolved name = %q, want delly", match.registration.Name)
	}
}

func TestResolvePlainTextAssistantResourceReference_ProxmoxNodeName(t *testing.T) {
	provider := plainTextResourceTestProvider(unifiedresources.Resource{
		ID:     "agent:pve-node-1",
		Type:   unifiedresources.ResourceTypeAgent,
		Name:   "Proxmox node 1",
		Status: unifiedresources.StatusOnline,
		Tags:   []string{"sensitive"},
		Proxmox: &unifiedresources.ProxmoxData{
			NodeName: "delly",
		},
	})

	match, ok := resolvePlainTextAssistantResourceReference(
		"On the delly host, run the read-only command `ls /dev | wc -l` and tell me the count.",
		provider,
	)
	if !ok {
		t.Fatal("expected Proxmox node name to resolve as a plain-text resource reference")
	}
	if match.registration.Kind != "agent" {
		t.Fatalf("resolved kind = %q, want agent", match.registration.Kind)
	}
}

func TestResolvePlainTextAssistantResourceReference_CoalescesNodeAndAgentForSameHost(t *testing.T) {
	proxmoxNode := unifiedresources.Resource{
		ID:     "agent:pve-node-1",
		Type:   unifiedresources.ResourceTypeAgent,
		Name:   "Proxmox node 1",
		Status: unifiedresources.StatusOnline,
		Tags:   []string{"sensitive"},
		Proxmox: &unifiedresources.ProxmoxData{
			NodeName: "delly",
		},
	}
	pulseAgent := plainTextResourceTestAgent("delly")
	provider := plainTextResourceTestProvider(proxmoxNode, pulseAgent)

	match, ok := resolvePlainTextAssistantResourceReference(
		"On the delly host, run the read-only command `ls /dev | wc -l` and tell me the count.",
		provider,
	)
	if !ok {
		t.Fatal("expected matching Proxmox node and Pulse agent to coalesce")
	}
	if match.registration.Name != "delly" {
		t.Fatalf("resolved name = %q, want command-capable agent delly", match.registration.Name)
	}
	if !plainTextResourceRegistrationAllows(match.registration, "exec") {
		t.Fatalf("expected coalesced match to prefer exec-capable registration: %#v", match.registration)
	}
}

func TestResolvePlainTextAssistantResourceReference_HostQualifierPrefersAgent(t *testing.T) {
	provider := plainTextResourceTestProvider(
		plainTextResourceTestAgent("delly"),
		unifiedresources.Resource{
			ID:         "storage:local-zfs",
			Type:       unifiedresources.ResourceTypeStorage,
			Name:       "local-zfs",
			Status:     unifiedresources.StatusOnline,
			ParentName: "delly",
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: []string{"delly"},
			},
			Storage: &unifiedresources.StorageMeta{
				Type:  "zfspool",
				Nodes: []string{"delly"},
			},
			Proxmox: &unifiedresources.ProxmoxData{
				NodeName: "delly",
			},
		},
	)

	match, ok := resolvePlainTextAssistantResourceReference(
		"On the delly host, run the read-only command `ls /dev | wc -l` and tell me the count.",
		provider,
	)
	if !ok {
		t.Fatal("expected host qualifier to prefer the agent over storage rows carrying the same node label")
	}
	if match.registration.Kind != "agent" {
		t.Fatalf("resolved kind = %q, want agent", match.registration.Kind)
	}
}

func TestResolvePlainTextAssistantResourceReference_AmbiguousReferencesFailClosed(t *testing.T) {
	provider := plainTextResourceTestProvider(
		plainTextResourceTestAgent("delly"),
		plainTextResourceTestAgent("mini"),
	)

	_, ok := resolvePlainTextAssistantResourceReference(
		"Compare delly and mini, then run a read-only command.",
		provider,
	)
	if ok {
		t.Fatal("expected ambiguous plain-text resource references to fail closed")
	}
}

func TestServiceExecuteStream_AttachesPlainTextResourceReferenceAsCurrentResource(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	resource := plainTextResourceTestAgent("delly")
	providerResources := plainTextResourceTestProvider(resource)
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{UnifiedResourceProvider: providerResources})

	var capturedRequests [][]providers.Message
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			capturedMessages := append([]providers.Message(nil), req.Messages...)
			capturedRequests = append(capturedRequests, capturedMessages)
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "I will check that."},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}

	svc := &Service{
		cfg:                     &config.AIConfig{ChatModel: "openrouter:deepseek/deepseek-chat"},
		sessions:                store,
		executor:                executor,
		agenticLoop:             NewAgenticLoop(provider, executor, "system"),
		provider:                provider,
		unifiedResourceProvider: providerResources,
		started:                 true,
	}

	req := ExecuteRequest{
		SessionID: "sess-plain-text-resource",
		Prompt:    "On the delly host, run the read-only command `ls /dev | wc -l` and tell me the count.",
	}
	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	if len(capturedRequests) == 0 || len(capturedRequests[0]) == 0 {
		t.Fatal("expected provider-bound messages to be captured")
	}
	capturedMessages := capturedRequests[0]
	modelUserContent := capturedMessages[len(capturedMessages)-1].Content
	for _, expected := range []string{
		"[Resource Context Handoff Instructions]",
		"Source: Pulse resource reference resolution",
		"Selected Resource: Pulse resolved one unambiguous user-referenced resource as the attached resource for this turn.",
		"target_host=\"current_resource\" or resource_id=\"current_resource\"",
		"Provider-safe user request:",
		"On the current_resource host, run the read-only command",
		unifiedresources.ResourcePolicyRedactedLabel,
	} {
		if !strings.Contains(modelUserContent, expected) {
			t.Fatalf("provider-bound message missing %q: %q", expected, modelUserContent)
		}
	}
	if strings.Contains(modelUserContent, "delly") {
		t.Fatalf("provider-bound message leaked raw resource alias: %q", modelUserContent)
	}

	resolved := store.GetResolvedContext("sess-plain-text-resource")
	info, found := resolved.GetResolvedResourceByAlias("delly")
	if !found {
		t.Fatalf("expected resolved resource alias to be registered")
	}
	if !resolved.WasRecentlyAccessed(info.GetResourceID(), time.Minute) {
		t.Fatalf("expected resolved resource to be marked as explicit current_resource access")
	}
}

func TestAttachPlainTextAssistantResourceContext_FallsBackToReadStateProvider(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}
	session, err := store.EnsureSession("sess-read-state-provider")
	if err != nil {
		t.Fatalf("failed to ensure session: %v", err)
	}
	messages := []Message{{
		ID:      "msg-1",
		Role:    "user",
		Content: "On the delly host, run the read-only command `ls /dev | wc -l` and tell me the count.",
	}}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{{
			ID:              "host-delly",
			Hostname:        "delly",
			Status:          "online",
			Platform:        "linux",
			CommandsEnabled: true,
			Tags:            []string{"sensitive"},
		}},
	})

	ok := attachPlainTextAssistantResourceContext(session.ID, messages, store, nil, registry, messages[0].Content)
	if !ok {
		t.Fatal("expected ReadState provider fallback to attach resource context")
	}
	if !strings.Contains(messages[0].Content, "Source: Pulse resource reference resolution") {
		t.Fatalf("expected resource context directive in user message, got %q", messages[0].Content)
	}
	resolved := store.GetResolvedContext(session.ID)
	info, found := resolved.GetResolvedResourceByAlias("delly")
	if !found {
		t.Fatal("expected read-state resource to register by alias")
	}
	if !resolved.WasRecentlyAccessed(info.GetResourceID(), time.Minute) {
		t.Fatalf("expected read-state resource to be explicit current-turn access")
	}
}

func TestServiceSetReadStateEnablesPlainTextResourceReferenceFallback(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{{
			ID:              "host-delly",
			Hostname:        "delly",
			Status:          "online",
			Platform:        "linux",
			CommandsEnabled: true,
			Tags:            []string{"sensitive"},
		}},
	})
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})

	var capturedRequests [][]providers.Message
	provider := &stubServiceProvider{
		streamFn: func(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
			capturedMessages := append([]providers.Message(nil), req.Messages...)
			capturedRequests = append(capturedRequests, capturedMessages)
			callback(providers.StreamEvent{
				Type: "content",
				Data: providers.ContentEvent{Text: "I will check that."},
			})
			callback(providers.StreamEvent{
				Type: "done",
				Data: providers.DoneEvent{InputTokens: 1, OutputTokens: 1},
			})
			return nil
		},
	}

	svc := &Service{
		cfg:         &config.AIConfig{ChatModel: "openrouter:deepseek/deepseek-chat"},
		sessions:    store,
		executor:    executor,
		agenticLoop: NewAgenticLoop(provider, executor, "system"),
		provider:    provider,
		started:     true,
	}
	svc.SetReadState(registry)

	req := ExecuteRequest{
		SessionID: "sess-read-state-late",
		Prompt:    "On the delly host, run the read-only command `ls /dev | wc -l` and tell me the count.",
	}
	if err := svc.ExecuteStream(context.Background(), req, func(StreamEvent) {}); err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	if len(capturedRequests) == 0 || len(capturedRequests[0]) == 0 {
		t.Fatal("expected provider-bound messages to be captured")
	}
	modelUserContent := capturedRequests[0][len(capturedRequests[0])-1].Content
	for _, expected := range []string{
		"Source: Pulse resource reference resolution",
		"target_host=\"current_resource\" or resource_id=\"current_resource\"",
	} {
		if !strings.Contains(modelUserContent, expected) {
			t.Fatalf("provider-bound message missing %q: %q", expected, modelUserContent)
		}
	}
	resolved := store.GetResolvedContext("sess-read-state-late")
	info, found := resolved.GetResolvedResourceByAlias("delly")
	if !found {
		t.Fatal("expected late read-state resource to register by alias")
	}
	if !resolved.WasRecentlyAccessed(info.GetResourceID(), time.Minute) {
		t.Fatalf("expected late read-state resource to be explicit current-turn access")
	}
}

func plainTextResourceTestAgent(name string) unifiedresources.Resource {
	return unifiedresources.Resource{
		ID:     "agent:" + name,
		Type:   unifiedresources.ResourceTypeAgent,
		Name:   name,
		Status: unifiedresources.StatusOnline,
		Tags:   []string{"sensitive"},
		Identity: unifiedresources.ResourceIdentity{
			Hostnames: []string{name},
		},
		Agent: &unifiedresources.AgentData{
			AgentID:         "agent-" + name,
			Hostname:        name,
			CommandsEnabled: true,
		},
		Policy: &unifiedresources.ResourcePolicy{
			Sensitivity: unifiedresources.ResourceSensitivitySensitive,
			Routing: unifiedresources.ResourceRoutingPolicy{
				Scope: unifiedresources.ResourceRoutingScopeCloudSummary,
				Redact: []unifiedresources.ResourceRedactionHint{
					unifiedresources.ResourceRedactionAlias,
					unifiedresources.ResourceRedactionHostname,
				},
			},
		},
	}
}

func plainTextResourceTestProvider(resources ...unifiedresources.Resource) handoffUnifiedProvider {
	byType := make(map[unifiedresources.ResourceType][]unifiedresources.Resource)
	for _, resource := range resources {
		byType[resource.Type] = append(byType[resource.Type], resource)
	}
	return handoffUnifiedProvider{
		resources: byType,
	}
}
