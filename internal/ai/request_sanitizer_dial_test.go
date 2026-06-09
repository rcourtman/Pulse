package ai

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// TestRequestSanitizerForModel_HonorsCloudPrivacyDial proves the shared sanitizer
// helper (used by discovery analysis, report/fleet narrators, quick analysis, and
// ExecuteAgentic) respects the cloud_context_privacy dial — not just the chat seam.
func TestRequestSanitizerForModel_HonorsCloudPrivacyDial(t *testing.T) {
	resources := []unifiedresources.Resource{
		{
			ID: "vm-1", Name: "finance-vm", Type: unifiedresources.ResourceTypeVM,
			Status: unifiedresources.StatusOnline, Tags: []string{"sensitive"}, // -> Sensitive / local-first
			Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"finance-vm.lan"}},
			Proxmox:  &unifiedresources.ProxmoxData{VMID: 1, NodeName: "n1"},
		},
		{
			ID: "agent/vault", Name: "vault", Type: unifiedresources.ResourceTypeAgent,
			Status: unifiedresources.StatusOnline, Tags: []string{"secret"}, // -> Restricted / local-only floor
			Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"vault.lan"}},
		},
	}
	urp := &mockUnifiedResourceProvider{getAllFunc: func() []unifiedresources.Resource {
		return append([]unifiedresources.Resource(nil), resources...)
	}}
	req := providers.ChatRequest{System: "finance-vm.lan and vault.lan. Authorization: Bearer sk-leak-secret-token"}

	sanitizerFor := func(level string) func(providers.ChatRequest) providers.ChatRequest {
		s := &Service{cfg: &config.AIConfig{CloudContextPrivacy: level}, unifiedResourceProvider: urp}
		return s.requestSanitizerForModel("openai:gpt-4o")
	}

	// full: Sensitive (local-first) identifier flows; Restricted (local-only) stays
	// redacted as the hard floor; secrets always redacted.
	full := sanitizerFor(config.CloudContextPrivacyFull)
	if full == nil {
		t.Fatal("expected a sanitizer for an external model")
	}
	out := full(req).System
	if !strings.Contains(out, "finance-vm.lan") {
		t.Fatalf("full must keep the sensitive identifier, got: %s", out)
	}
	if strings.Contains(out, "vault.lan") {
		t.Fatalf("full must keep the local-only floor (vault.lan redacted), got: %s", out)
	}
	if strings.Contains(out, "sk-leak-secret-token") {
		t.Fatalf("full must still redact the bearer token, got: %s", out)
	}

	// redacted: every policied identifier is redacted.
	red := sanitizerFor(config.CloudContextPrivacyRedacted)
	out = red(req).System
	if strings.Contains(out, "finance-vm.lan") || strings.Contains(out, "vault.lan") {
		t.Fatalf("redacted must redact all identifiers, got: %s", out)
	}

	// local (Ollama): no sanitizer — local is always full.
	localSvc := &Service{cfg: &config.AIConfig{CloudContextPrivacy: config.CloudContextPrivacyRedacted}, unifiedResourceProvider: urp}
	if localSvc.requestSanitizerForModel("ollama:llama3") != nil {
		t.Fatal("expected no sanitizer for a local Ollama model")
	}
}
