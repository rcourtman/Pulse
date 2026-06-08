package chat

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/modelboundary"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// governedHomeAssistantMention returns a sensitive system-container mention with
// the same governed policy a real guest receives (local-first routing with
// hostname/IP/alias/path redaction), plus the cloud-routed @mention reference.
func governedHomeAssistantMention() ResourceMention {
	return ResourceMention{
		Name:          "homeassistant",
		ResourceType:  "system-container",
		ResourceID:    "101",
		TargetID:      "node1",
		AISafeSummary: "system container resource; status online; redacted for cloud summary",
		Policy: &unifiedresources.ResourcePolicy{
			Sensitivity: unifiedresources.ResourceSensitivitySensitive,
			Routing: unifiedresources.ResourceRoutingPolicy{
				Scope: unifiedresources.ResourceRoutingScopeLocalFirst,
				Redact: []unifiedresources.ResourceRedactionHint{
					unifiedresources.ResourceRedactionHostname,
					unifiedresources.ResourceRedactionIPAddress,
					unifiedresources.ResourceRedactionAlias,
					unifiedresources.ResourceRedactionPath,
				},
			},
		},
	}
}

// homeAssistantDiscovery mirrors what Discovery captures for the HA LXC. The
// Hostname/IP-bearing fields are present on the DTO precisely so the test can
// prove the cloud-safe path never emits them.
func homeAssistantDiscovery() *tools.ResourceDiscoveryInfo {
	return &tools.ResourceDiscoveryInfo{
		ID:           "system-container:node1:101",
		ResourceType: "system-container",
		ResourceID:   "101",
		TargetID:     "node1",
		Hostname:     "delly-ha-host", // PII: must never reach the cloud-safe context
		ServiceType:  "home-assistant",
		ServiceName:  "Home Assistant",
		Category:     "home-automation",
		CLIAccess:    "pct exec 101 -- docker exec homeassistant",
		ConfigPaths:  []string{"/config/configuration.yaml", "/config/automations.yaml"},
		LogPaths:     []string{"/config/home-assistant.log"},
		Ports: []tools.DiscoveryPortInfo{
			{Port: 8123, Protocol: "tcp", Address: "192.168.0.101"}, // bind addr is PII
		},
	}
}

func TestPrefetcherCloudContext_RedactedSharesAccessPathWithoutPII(t *testing.T) {
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{}), nil)

	// The "redacted" level shares the PII-free operational context for governed
	// resources: useful commands/paths/ports reach the model, identifying
	// hostnames/IPs do not.
	summary, spans := prefetcher.formatContextSummaryWithPolicy(
		[]ResourceMention{governedHomeAssistantMention()},
		[]*tools.ResourceDiscoveryInfo{homeAssistantDiscovery()},
		CloudContextPolicy{CloudRouting: true, Level: config.CloudContextPrivacyRedacted},
	)

	// The operational access path reaches the model.
	if !strings.Contains(summary, "pct exec 101 -- docker exec homeassistant") {
		t.Fatalf("opt-in cloud summary must include the access path, got:\n%s", summary)
	}
	if !strings.Contains(summary, "/config/automations.yaml") {
		t.Fatalf("opt-in cloud summary must include config paths, got:\n%s", summary)
	}
	if !strings.Contains(summary, "8123") {
		t.Fatalf("opt-in cloud summary must include the port number, got:\n%s", summary)
	}

	// PII never appears.
	if strings.Contains(summary, "delly-ha-host") {
		t.Fatalf("opt-in cloud summary leaked the hostname, got:\n%s", summary)
	}
	if strings.Contains(summary, "192.168.0.101") {
		t.Fatalf("opt-in cloud summary leaked the bind IP, got:\n%s", summary)
	}

	// The terse governed redaction is replaced, not appended.
	if strings.Contains(summary, unifiedresources.ResourcePolicyGovernedSummaryFooter()) {
		t.Fatalf("opt-in cloud summary must not fall back to the governed footer, got:\n%s", summary)
	}

	// The exact cloud-safe span is returned for allow-listing.
	if len(spans) != 1 {
		t.Fatalf("expected exactly one cloud-safe span, got %d: %#v", len(spans), spans)
	}
	if !strings.Contains(spans[0], "pct exec 101 -- docker exec homeassistant") {
		t.Fatalf("returned span must carry the access path, got %q", spans[0])
	}
	if strings.Contains(spans[0], "delly-ha-host") || strings.Contains(spans[0], "192.168.0.101") {
		t.Fatalf("returned span leaked PII, got %q", spans[0])
	}
}

func TestPrefetcherCloudContext_FullKeepsGovernedPIIFreeInPrefetch(t *testing.T) {
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{}), nil)

	// Even at "full", the proactive prefetch keeps genuinely-governed resources
	// PII-free (the model-boundary sanitizer is where full opens identifiers up).
	// So a governed resource still surfaces only the cloud-safe operational span.
	summary, spans := prefetcher.formatContextSummaryWithPolicy(
		[]ResourceMention{governedHomeAssistantMention()},
		[]*tools.ResourceDiscoveryInfo{homeAssistantDiscovery()},
		CloudContextPolicy{CloudRouting: true, Level: config.CloudContextPrivacyFull},
	)

	if !strings.Contains(summary, "pct exec 101 -- docker exec homeassistant") {
		t.Fatalf("full prefetch must include the access path, got:\n%s", summary)
	}
	if strings.Contains(summary, "delly-ha-host") || strings.Contains(summary, "192.168.0.101") {
		t.Fatalf("full prefetch must not leak governed PII, got:\n%s", summary)
	}
	if len(spans) != 1 {
		t.Fatalf("expected one cloud-safe span at full, got %d: %#v", len(spans), spans)
	}
}

func TestPrefetcherCloudContext_RedactedWithoutDiscoveryKeepsGovernedSummary(t *testing.T) {
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{}), nil)

	// Redacted with no discovery data falls back to the terse governed summary —
	// and never emits the obsolete opt-in transparency string.
	summary, spans := prefetcher.formatContextSummaryWithPolicy(
		[]ResourceMention{governedHomeAssistantMention()},
		nil,
		CloudContextPolicy{CloudRouting: true, Level: config.CloudContextPrivacyRedacted},
	)

	if strings.Contains(summary, "pct exec") {
		t.Fatalf("redacted-without-discovery must withhold the access path, got:\n%s", summary)
	}
	if !strings.Contains(summary, unifiedresources.ResourcePolicyGovernedSummaryFooter()) {
		t.Fatalf("redacted-without-discovery must keep the governed redaction, got:\n%s", summary)
	}
	if strings.Contains(summary, "Share operational context with cloud models") {
		t.Fatalf("obsolete opt-in transparency string must not appear, got:\n%s", summary)
	}
	if len(spans) != 0 {
		t.Fatalf("redacted-without-discovery must not return cloud-safe spans, got %#v", spans)
	}
}

func TestPrefetcherCloudContext_LocalRoutingUnaffected(t *testing.T) {
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{}), nil)

	// Local routing (Ollama): not a cloud turn, so no cloud-safe injection —
	// behavior matches the historical governed path.
	summary, spans := prefetcher.formatContextSummaryWithPolicy(
		[]ResourceMention{governedHomeAssistantMention()},
		[]*tools.ResourceDiscoveryInfo{homeAssistantDiscovery()},
		CloudContextPolicy{CloudRouting: false, Level: config.CloudContextPrivacyFull},
	)

	if !strings.Contains(summary, unifiedresources.ResourcePolicyGovernedSummaryFooter()) {
		t.Fatalf("local routing must keep the governed prefetch redaction, got:\n%s", summary)
	}
	if len(spans) != 0 {
		t.Fatalf("local routing must not return cloud-safe spans, got %#v", spans)
	}
}

func TestCloudContextPolicy_LevelSemantics(t *testing.T) {
	cases := []struct {
		name           string
		policy         CloudContextPolicy
		wantShares     bool
		wantSuppresses bool
	}{
		{"full cloud", CloudContextPolicy{CloudRouting: true, Level: config.CloudContextPrivacyFull}, true, false},
		{"redacted cloud", CloudContextPolicy{CloudRouting: true, Level: config.CloudContextPrivacyRedacted}, true, false},
		{"local_only cloud", CloudContextPolicy{CloudRouting: true, Level: config.CloudContextPrivacyLocalOnly}, false, true},
		{"empty level fails closed to redacted", CloudContextPolicy{CloudRouting: true, Level: ""}, true, false},
		{"unknown level fails closed to redacted", CloudContextPolicy{CloudRouting: true, Level: "bogus"}, true, false},
		{"full but local routing shares nothing extra", CloudContextPolicy{CloudRouting: false, Level: config.CloudContextPrivacyFull}, false, false},
		{"local_only but local routing never suppresses", CloudContextPolicy{CloudRouting: false, Level: config.CloudContextPrivacyLocalOnly}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.policy.sharesCloudOperationalContext(); got != tc.wantShares {
				t.Fatalf("sharesCloudOperationalContext() = %v, want %v", got, tc.wantShares)
			}
			if got := tc.policy.suppressesCloudContext(); got != tc.wantSuppresses {
				t.Fatalf("suppressesCloudContext() = %v, want %v", got, tc.wantSuppresses)
			}
		})
	}
}

// policiedResourceProvider is a minimal modelboundary.UnifiedResourceProvider
// returning one sensitive system container so the resource-policy sanitizer has
// real PII candidates to redact.
type policiedResourceProvider struct {
	resource unifiedresources.Resource
}

func (p *policiedResourceProvider) GetByType(t unifiedresources.ResourceType) []unifiedresources.Resource {
	if t == unifiedresources.ResourceTypeSystemContainer {
		return []unifiedresources.Resource{p.resource}
	}
	return nil
}

func TestCloudSafeContextSurvivesModelBoundarySanitizer(t *testing.T) {
	// A sensitive guest whose hostname, IP, and alias the policy must redact for
	// cloud routing.
	resource := unifiedresources.Resource{
		ID:   "system-container:node1:101",
		Type: unifiedresources.ResourceTypeSystemContainer,
		Name: "homeassistant",
		Tags: []string{"sensitive"},
		Identity: unifiedresources.ResourceIdentity{
			Hostnames:   []string{"delly-ha-host"},
			IPAddresses: []string{"192.168.0.101"},
		},
	}
	provider := &policiedResourceProvider{resource: resource}

	cloudSafe := cloudSafeOperationalContext(homeAssistantDiscovery())
	if cloudSafe == "" {
		t.Fatal("expected a non-empty cloud-safe context")
	}

	// The model request carries the cloud-safe span plus raw PII a model must not
	// see. "homeassistant" appears in the span (docker exec target) AND is an
	// alias the policy would redact, so the allow-list must protect it there.
	userContent := cloudSafe + "\n\nRaw host: delly-ha-host at 192.168.0.101 (homeassistant)"
	req := providers.ChatRequest{
		Messages: []providers.Message{{Role: "user", Content: userContent}},
	}

	sanitizer := modelboundary.RequestSanitizerForModel(
		"openai:gpt-4o",
		provider,
		modelboundary.AllowResourcePolicyText(cloudSafe),
	)
	if sanitizer == nil {
		t.Fatal("expected a sanitizer for a cloud-routed model")
	}
	out := sanitizer(req).Messages[0].Content

	// The access path survives intact.
	if !strings.Contains(out, "pct exec 101 -- docker exec homeassistant") {
		t.Fatalf("sanitizer stripped the allow-listed access path, got:\n%s", out)
	}
	// Raw PII outside the protected span is redacted.
	if strings.Contains(out, "delly-ha-host") {
		t.Fatalf("sanitizer must redact the raw hostname, got:\n%s", out)
	}
	if strings.Contains(out, "192.168.0.101") {
		t.Fatalf("sanitizer must redact the raw IP, got:\n%s", out)
	}
}
