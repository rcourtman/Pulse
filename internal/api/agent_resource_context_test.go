package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// staticFindingsProvider is a tiny test double for AgentFindingsProvider
// — keyed by resource id so each test can stage the snapshot it
// expects to see in the bundle.
type staticFindingsProvider struct {
	byResource  map[string][]AgentResourceFindingSnapshot
	activeCount *int
}

func (s staticFindingsProvider) ActiveFindingsForResource(resourceID string) []AgentResourceFindingSnapshot {
	if s.byResource == nil {
		return nil
	}
	return s.byResource[resourceID]
}

func (s staticFindingsProvider) ActiveFindingCount() int {
	if s.activeCount != nil {
		return *s.activeCount
	}
	total := 0
	for _, findings := range s.byResource {
		total += len(findings)
	}
	return total
}

// staticApprovalsProvider is a tiny test double for
// AgentApprovalsProvider — keyed by (resource id, org id) so each
// test can stage the pending approvals it expects to see in the
// bundle without setting up a global approval store.
type staticApprovalsProvider struct {
	byResource map[string][]AgentResourceApprovalSummary
}

type staticWorkflowPromptActivityProvider struct {
	history *config.WorkflowPromptActivityHistoryData
	err     error
}

func (s staticWorkflowPromptActivityProvider) WorkflowPromptActivityHistory(_ context.Context) (*config.WorkflowPromptActivityHistoryData, error) {
	return s.history, s.err
}

type staticAIUsageProvider struct {
	history *config.AIUsageHistoryData
	err     error
}

func (s staticAIUsageProvider) AIUsageHistory(_ context.Context) (*config.AIUsageHistoryData, error) {
	return s.history, s.err
}

func (s staticApprovalsProvider) PendingApprovalsForResource(resourceID, _ string) []AgentResourceApprovalSummary {
	if s.byResource == nil {
		return nil
	}
	return s.byResource[resourceID]
}

func (s staticApprovalsProvider) PendingApprovalCountsByResource(_ string) map[string]int {
	if s.byResource == nil {
		return nil
	}
	counts := map[string]int{}
	for resourceID, pending := range s.byResource {
		if len(pending) > 0 {
			counts[resourceID] = len(pending)
		}
	}
	if len(counts) == 0 {
		return nil
	}
	return counts
}

type countingApprovalsProvider struct {
	staticApprovalsProvider
	resourceCalls int
	countCalls    int
}

func (p *countingApprovalsProvider) PendingApprovalsForResource(resourceID, orgID string) []AgentResourceApprovalSummary {
	p.resourceCalls++
	return p.staticApprovalsProvider.PendingApprovalsForResource(resourceID, orgID)
}

func (p *countingApprovalsProvider) PendingApprovalCountsByResource(orgID string) map[string]int {
	p.countCalls++
	return p.staticApprovalsProvider.PendingApprovalCountsByResource(orgID)
}

type staticResourceDiscoveryReadinessProvider struct {
	byResource map[string]unified.ResourceDiscoveryReadiness
}

func (p staticResourceDiscoveryReadinessProvider) DiscoveryReadinessForResource(resource unified.Resource, now time.Time) unified.ResourceDiscoveryReadiness {
	if readiness, ok := p.byResource[unified.CanonicalResourceID(resource.ID)]; ok {
		if readiness.GeneratedAt.IsZero() {
			readiness.GeneratedAt = now
		}
		if readiness.ResourceType == "" && resource.DiscoveryTarget != nil {
			readiness.ResourceType = resource.DiscoveryTarget.ResourceType
		}
		if readiness.TargetID == "" && resource.DiscoveryTarget != nil {
			readiness.TargetID = resource.DiscoveryTarget.AgentID
		}
		if readiness.ResourceID == "" && resource.DiscoveryTarget != nil {
			readiness.ResourceID = resource.DiscoveryTarget.ResourceID
		}
		return readiness
	}
	readiness := unified.ResourceDiscoveryReadiness{
		State:       unified.ResourceDiscoveryReadinessMissing,
		Source:      "service-discovery",
		GeneratedAt: now,
	}
	if resource.DiscoveryTarget != nil {
		readiness.ResourceType = resource.DiscoveryTarget.ResourceType
		readiness.TargetID = resource.DiscoveryTarget.AgentID
		readiness.ResourceID = resource.DiscoveryTarget.ResourceID
	}
	return readiness
}

// agentContextFixtureHandlers wires a ResourceHandlers around a
// pre-built unified-resource seed (using the same
// `resourceUnifiedSeedProvider` pattern from resources_test.go) plus
// the agent context handler with a findings-provider stub. The seed
// provider bypasses the snapshot → unified adapter, letting us stage
// exactly the resource the test expects to see in the bundle.
func agentContextFixtureHandlersForResource(t *testing.T, resource unified.Resource) (*AgentContextHandler, *staticFindingsProvider) {
	t.Helper()
	cfg := &config.Config{DataPath: t.TempDir()}
	resources := NewResourceHandlers(cfg)
	resources.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: time.Now()},
		resources: []unified.Resource{resource},
	})
	provider := &staticFindingsProvider{byResource: map[string][]AgentResourceFindingSnapshot{}}
	h := NewAgentContextHandler(resources)
	h.SetFindingsProvider(provider)
	return h, provider
}

func agentContextFixtureHandlers(t *testing.T, resourceID string) (*AgentContextHandler, *staticFindingsProvider) {
	t.Helper()
	return agentContextFixtureHandlersForResource(t, unified.Resource{
		ID:   resourceID,
		Type: "vm",
		Name: "db-01",
	})
}

func findAgentContextSection(sections []AgentResourceContextSection, id string) (AgentResourceContextSection, bool) {
	for _, section := range sections {
		if section.ID == id {
			return section, true
		}
	}
	return AgentResourceContextSection{}, false
}

func findAgentContextFact(section AgentResourceContextSection, label string) (AgentResourceContextFact, bool) {
	for _, fact := range section.Facts {
		if fact.Label == label {
			return fact, true
		}
	}
	return AgentResourceContextFact{}, false
}

func TestHandleAgentResourceContext_BundlesIdentityOperatorStateAndFindings(t *testing.T) {
	h, findings := agentContextFixtureHandlers(t, "vm:101")

	// Stage operator state — server should compute
	// MaintenanceWindowActive once and surface it on the wire so agents
	// don't re-evaluate timestamps client-side.
	store, err := h.resources.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(time.Hour)
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID:        "vm:101",
		NeverAutoRemediate: true,
		MaintenanceStartAt: &start,
		MaintenanceEndAt:   &end,
		MaintenanceReason:  "Q3 storage upgrade",
		Criticality:        unified.CriticalityHigh,
		SetAt:              time.Now().UTC(),
		SetBy:              "operator:richard",
	}); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}

	// Stage two active findings the provider would return.
	findings.byResource["vm:101"] = []AgentResourceFindingSnapshot{
		{
			ID:              "f-cpu",
			Title:           "CPU saturated",
			Severity:        "warning",
			Category:        "performance",
			Impact:          "Workload stalls until pressure clears.",
			Recommendation:  "Restart workload service.",
			RegressionCount: 2,
			Confidence:      "medium",
		},
		{
			ID:       "f-disk",
			Title:    "Disk pressure",
			Severity: "critical",
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/vm:101", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "default"))
	h.HandleResourceContext(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d body=%s", rec.Code, rec.Body.String())
	}
	var bundle AgentResourceContext
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("unmarshal bundle: %v", err)
	}

	// Identity round-trips.
	if bundle.CanonicalID != "vm:101" {
		t.Errorf("canonical id: got %q want vm:101", bundle.CanonicalID)
	}
	if bundle.ResourceName != "db-01" {
		t.Errorf("resource name: got %q want db-01", bundle.ResourceName)
	}
	if bundle.ResourceType != "vm" {
		t.Errorf("resource type: got %q want vm", bundle.ResourceType)
	}

	// Operator-state projected with computed maintenance-active flag.
	if bundle.OperatorState == nil {
		t.Fatal("operator state must be present")
	}
	if !bundle.OperatorState.NeverAutoRemediate {
		t.Error("never_auto_remediate must round-trip")
	}
	if !bundle.OperatorState.MaintenanceWindowActive {
		t.Error("MaintenanceWindowActive must be true when window covers now (computed server-side, not by the agent)")
	}
	if bundle.OperatorState.Criticality != "high" {
		t.Errorf("criticality: got %q want high", bundle.OperatorState.Criticality)
	}

	// Findings come through the provider verbatim — no leakage of
	// internal Finding shape.
	if len(bundle.ActiveFindings) != 2 {
		t.Fatalf("expected 2 findings; got %d", len(bundle.ActiveFindings))
	}
	if bundle.ActiveFindings[0].Confidence != "medium" {
		t.Errorf("confidence must round-trip; got %q", bundle.ActiveFindings[0].Confidence)
	}
	if bundle.ActiveFindings[0].RegressionCount != 2 {
		t.Errorf("regression count must round-trip; got %d", bundle.ActiveFindings[0].RegressionCount)
	}

	// Generated timestamp populated so agents can age the data.
	if bundle.GeneratedAt.IsZero() {
		t.Error("GeneratedAt must be populated")
	}
	if len(bundle.ContextSections) == 0 {
		t.Fatal("contextSections must be populated for resource-aware agents")
	}
	if section, ok := findAgentContextSection(bundle.ContextSections, "identity"); !ok {
		t.Fatal("identity context section missing")
	} else if section.Source == "" || section.TrustTier == "" || section.GeneratedAt.IsZero() {
		t.Fatalf("identity section must carry provenance and freshness metadata: %+v", section)
	}
}

func TestHandleAgentResourceContext_IncludesDiscoveryReadiness(t *testing.T) {
	observedAt := time.Date(2026, 6, 4, 10, 30, 0, 0, time.UTC)
	resourceID := "vm:node-a:101"
	h, _ := agentContextFixtureHandlersForResource(t, unified.Resource{
		ID:        resourceID,
		Type:      unified.ResourceTypeVM,
		Name:      "homeassistant",
		Status:    unified.StatusOnline,
		LastSeen:  observedAt,
		UpdatedAt: observedAt,
		Proxmox: &unified.ProxmoxData{
			NodeName:      "node-a",
			VMID:          101,
			LinkedAgentID: "agent-a",
		},
	})
	h.resources.SetDiscoveryReadinessProvider(staticResourceDiscoveryReadinessProvider{
		byResource: map[string]unified.ResourceDiscoveryReadiness{
			resourceID: {
				State:             unified.ResourceDiscoveryReadinessFresh,
				Source:            "service-discovery",
				ObservedAt:        &observedAt,
				AgeSeconds:        120,
				StaleAfterSeconds: int64((30 * 24 * time.Hour).Seconds()),
				FactCount:         7,
				ServiceName:       "Home Assistant",
				ServiceCategory:   "home_automation",
				Confidence:        0.91,
			},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/"+resourceID, nil)
	h.HandleResourceContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d body=%s", rec.Code, rec.Body.String())
	}
	var bundle AgentResourceContext
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("unmarshal bundle: %v", err)
	}
	if bundle.DiscoveryReadiness == nil || bundle.DiscoveryReadiness.State != unified.ResourceDiscoveryReadinessFresh {
		t.Fatalf("discovery readiness = %+v, want fresh", bundle.DiscoveryReadiness)
	}

	runtimeSection, ok := findAgentContextSection(bundle.ContextSections, "runtime")
	if !ok {
		t.Fatal("runtime section missing")
	}
	readinessFact, ok := findAgentContextFact(runtimeSection, "Discovery readiness")
	if !ok {
		t.Fatal("Discovery readiness fact missing")
	}
	if !strings.Contains(readinessFact.Value, "fresh") || !strings.Contains(readinessFact.Value, "staleAfter=") {
		t.Fatalf("readiness fact value = %q", readinessFact.Value)
	}
	if readinessFact.Source != "service-discovery" || readinessFact.TrustTier != "pulse-authored" {
		t.Fatalf("readiness fact provenance = source %q trust %q", readinessFact.Source, readinessFact.TrustTier)
	}
	serviceFact, ok := findAgentContextFact(runtimeSection, "Discovered service")
	if !ok || serviceFact.TrustTier != "discovered" {
		t.Fatalf("discovered service fact = %+v", serviceFact)
	}
	if serviceFact.Value != "Home Assistant" && serviceFact.Value != unified.ResourcePolicyRedactedLabel {
		t.Fatalf("discovered service value = %q, want service name or policy redaction", serviceFact.Value)
	}
}

func TestHandleAgentResourceContext_ResolvesSourceIDReference(t *testing.T) {
	h, _ := agentContextFixtureHandlersForResource(t, unified.Resource{
		ID:         "system-container-6adaf34f529d241a",
		Type:       unified.ResourceTypeSystemContainer,
		Technology: "lxc",
		Name:       "homeassistant",
		Status:     unified.StatusOnline,
		Sources:    []unified.DataSource{unified.SourceProxmox},
		Proxmox: &unified.ProxmoxData{
			SourceID:      "delly:delly:101",
			NodeName:      "delly",
			Instance:      "delly",
			VMID:          101,
			ContainerType: "lxc",
		},
		DiscoveryTarget: &unified.DiscoveryTarget{
			ResourceType: "system-container",
			AgentID:      "delly",
			ResourceID:   "101",
			Hostname:     "homeassistant",
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/delly:delly:101", nil)
	h.HandleResourceContext(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d body=%s", rec.Code, rec.Body.String())
	}
	var bundle AgentResourceContext
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("unmarshal bundle: %v", err)
	}
	if bundle.CanonicalID != "system-container-6adaf34f529d241a" {
		t.Fatalf("canonical id = %q, want registered unified resource id", bundle.CanonicalID)
	}
	if bundle.ResourceName != "homeassistant" || bundle.ResourceType != "system-container" {
		t.Fatalf("bundle resource = %s/%s, want homeassistant/system-container", bundle.ResourceName, bundle.ResourceType)
	}
}

func TestHandleAgentResourceContext_UsesLinkedNodeAgentForProxmoxGuestDiscovery(t *testing.T) {
	parentID := "agent-delly"
	resources := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	resources.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: time.Now()},
		resources: []unified.Resource{
			{
				ID:     parentID,
				Type:   unified.ResourceTypeAgent,
				Name:   "delly",
				Status: unified.StatusOnline,
				Sources: []unified.DataSource{
					unified.SourceAgent,
				},
				Agent: &unified.AgentData{
					AgentID:  "agent-delly",
					Hostname: "delly",
				},
			},
			{
				ID:       "system-container-homeassistant",
				Type:     unified.ResourceTypeSystemContainer,
				Name:     "homeassistant",
				Status:   unified.StatusOnline,
				ParentID: &parentID,
				Sources: []unified.DataSource{
					unified.SourceProxmox,
				},
				Proxmox: &unified.ProxmoxData{
					SourceID:      "delly:delly:101",
					NodeName:      "delly",
					Instance:      "delly",
					VMID:          101,
					ContainerType: "lxc",
				},
			},
		},
	})
	resources.SetDiscoveryReadinessProvider(staticResourceDiscoveryReadinessProvider{})
	h := NewAgentContextHandler(resources)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/delly:delly:101", nil)
	h.HandleResourceContext(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d body=%s", rec.Code, rec.Body.String())
	}
	var bundle AgentResourceContext
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("unmarshal bundle: %v", err)
	}
	if bundle.CanonicalID != "system-container-homeassistant" {
		t.Fatalf("canonical id = %q, want system-container-homeassistant", bundle.CanonicalID)
	}
	if bundle.DiscoveryReadiness == nil {
		t.Fatal("expected discovery readiness in context bundle")
	}
	if bundle.DiscoveryReadiness.State != unified.ResourceDiscoveryReadinessMissing {
		t.Fatalf("readiness state = %q, want missing", bundle.DiscoveryReadiness.State)
	}
	if bundle.DiscoveryReadiness.ResourceType != "system-container" ||
		bundle.DiscoveryReadiness.TargetID != "agent-delly" ||
		bundle.DiscoveryReadiness.ResourceID != "101" {
		t.Fatalf("readiness target mismatch: %+v", bundle.DiscoveryReadiness)
	}
	runtime, ok := findAgentContextSection(bundle.ContextSections, "runtime")
	if !ok {
		t.Fatalf("expected runtime section in context bundle: %+v", bundle.ContextSections)
	}
	if _, ok := findAgentContextFact(runtime, "Discovery readiness"); !ok {
		t.Fatalf("runtime section must include discovery readiness fact: %+v", runtime.Facts)
	}
}

func TestHandleAgentResourceContext_ContextSectionsIncludeRuntimeTopologyAndRecentChanges(t *testing.T) {
	now := time.Now().UTC()
	parentID := "agent:pve1"
	h, _ := agentContextFixtureHandlersForResource(t, unified.Resource{
		ID:         "app-container:homeassistant",
		Type:       unified.ResourceTypeAppContainer,
		Name:       "homeassistant",
		Status:     unified.StatusOnline,
		Technology: "docker",
		LastSeen:   now.Add(-time.Minute),
		UpdatedAt:  now,
		ParentID:   &parentID,
		ParentName: "ha-lxc",
		DiscoveryTarget: &unified.DiscoveryTarget{
			ResourceType: "app-container",
			AgentID:      "agent:pve1",
			ResourceID:   "homeassistant",
			Hostname:     "homeassistant.local",
		},
		Docker: &unified.DockerData{
			Runtime:        "docker",
			RuntimeVersion: "27.0",
			ContainerState: "running",
			Health:         "healthy",
			RestartCount:   2,
			Image:          "ghcr.io/home-assistant/home-assistant:stable",
			Ports: []unified.DockerPortMeta{
				{PrivatePort: 8123, PublicPort: 8123, Protocol: "tcp"},
			},
			Mounts: []unified.DockerMountMeta{
				{Source: "/srv/homeassistant/secret-config", Destination: "/config", RW: true},
			},
			Labels: map[string]string{
				"com.example.env": "TOKEN=should-not-leak",
			},
		},
		Relationships: []unified.ResourceRelationship{
			{
				SourceID:   "app-container:homeassistant",
				TargetID:   "agent:pve1",
				Type:       unified.RelRunsOn,
				Confidence: 0.95,
				Active:     true,
				Discoverer: "docker_adapter",
				ObservedAt: now.Add(-2 * time.Minute),
				LastSeenAt: now.Add(-time.Minute),
				Metadata:   map[string]any{"raw": "metadata is intentionally summarized only"},
			},
		},
		Capabilities: []unified.ResourceCapability{
			{
				Name:                 "restart_container",
				MinimumApprovalLevel: unified.ApprovalAdmin,
				Platform:             "docker",
				Params: []unified.CapabilityParam{
					{Name: "token", IsSensitive: true},
				},
			},
		},
	})
	store, err := h.resources.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	if err := store.RecordChange(unified.ResourceChange{
		ID:            "change-ha-restart",
		ResourceID:    "app-container:homeassistant",
		ObservedAt:    now.Add(-30 * time.Second),
		Kind:          unified.ChangeRestart,
		From:          "running",
		To:            "restarted",
		SourceType:    unified.SourcePulseDiff,
		SourceAdapter: unified.AdapterDocker,
		Confidence:    unified.ConfidenceHigh,
		Reason:        "container restarted after update",
	}); err != nil {
		t.Fatalf("RecordChange: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/app-container:homeassistant", nil)
	h.HandleResourceContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d body=%s", rec.Code, rec.Body.String())
	}
	var bundle AgentResourceContext
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("unmarshal bundle: %v", err)
	}

	runtimeSection, ok := findAgentContextSection(bundle.ContextSections, "runtime")
	if !ok {
		t.Fatalf("runtime section missing from context pack: %+v", bundle.ContextSections)
	}
	if fact, ok := findAgentContextFact(runtimeSection, "Ports"); !ok || !strings.Contains(fact.Value, "8123:8123/tcp") {
		t.Fatalf("runtime ports fact missing or wrong: %+v", fact)
	}
	if fact, ok := findAgentContextFact(runtimeSection, "Mounts"); !ok || fact.Value != "1" {
		t.Fatalf("runtime must include mount count without path values; got %+v", fact)
	}

	topologySection, ok := findAgentContextSection(bundle.ContextSections, "topology")
	if !ok {
		t.Fatal("topology section missing from context pack")
	}
	if fact, ok := findAgentContextFact(topologySection, "Relationship 1"); !ok || !strings.Contains(fact.Value, "runs_on") || !strings.Contains(fact.Value, "metadata present") {
		t.Fatalf("relationship fact missing bounded topology summary: %+v", fact)
	}

	safetySection, ok := findAgentContextSection(bundle.ContextSections, "safety")
	if !ok {
		t.Fatal("safety section missing from context pack")
	}
	if fact, ok := findAgentContextFact(safetySection, "Capability 1"); !ok || !strings.Contains(fact.Value, "1 sensitive params hidden") {
		t.Fatalf("capability fact must hide sensitive params: %+v", fact)
	}

	changesSection, ok := findAgentContextSection(bundle.ContextSections, "recent_changes")
	if !ok {
		t.Fatal("recent_changes section missing from context pack")
	}
	if fact, ok := findAgentContextFact(changesSection, "Change 1"); !ok || !strings.Contains(fact.Value, "container restarted after update") {
		t.Fatalf("recent change fact missing canonical timeline context: %+v", fact)
	}

	body := rec.Body.String()
	for _, forbidden := range []string{"/srv/homeassistant/secret-config", "TOKEN=should-not-leak", "metadata is intentionally summarized only"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("context pack leaked raw unsafe detail %q in body: %s", forbidden, body)
		}
	}
}

func TestHandleAgentResourceContext_ContextSectionsApplyPolicyRedactions(t *testing.T) {
	h, _ := agentContextFixtureHandlersForResource(t, unified.Resource{
		ID:        "storage:dataset-1",
		Type:      unified.ResourceTypeStorage,
		Name:      "dataset-1",
		Status:    unified.StatusOnline,
		LastSeen:  time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Policy: &unified.ResourcePolicy{
			Sensitivity: unified.ResourceSensitivitySensitive,
			Routing: unified.ResourceRoutingPolicy{
				Scope: unified.ResourceRoutingScopeLocalOnly,
				Redact: []unified.ResourceRedactionHint{
					unified.ResourceRedactionHostname,
					unified.ResourceRedactionPath,
					unified.ResourceRedactionPlatformID,
				},
			},
		},
		AISafeSummary: "storage resource; status online; local-only context",
		DiscoveryTarget: &unified.DiscoveryTarget{
			ResourceType: "storage",
			AgentID:      "agent-secret-id",
			ResourceID:   "storage-secret-id",
			Hostname:     "secret-storage.local",
		},
		Storage: &unified.StorageMeta{
			Type:        "zfs",
			Path:        "/mnt/secret-dataset",
			RiskSummary: "secret-storage.local has elevated writes",
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/storage:dataset-1", nil)
	h.HandleResourceContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d body=%s", rec.Code, rec.Body.String())
	}
	var bundle AgentResourceContext
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("unmarshal bundle: %v", err)
	}

	runtimeSection, ok := findAgentContextSection(bundle.ContextSections, "runtime")
	if !ok {
		t.Fatal("runtime section missing")
	}
	for _, label := range []string{"Discovery hostname", "Storage path"} {
		fact, ok := findAgentContextFact(runtimeSection, label)
		if !ok {
			t.Fatalf("%s fact missing", label)
		}
		if fact.Value != unified.ResourcePolicyRedactedLabel || !fact.Redacted {
			t.Fatalf("%s must be explicitly redacted by policy; got %+v", label, fact)
		}
	}
	policySection, ok := findAgentContextSection(bundle.ContextSections, "policy")
	if !ok || len(policySection.Redactions) == 0 {
		t.Fatalf("policy section must explain redactions; got %+v", policySection)
	}
	body := rec.Body.String()
	for _, forbidden := range []string{"secret-storage.local", "/mnt/secret-dataset"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("context pack leaked policy-redacted value %q in body: %s", forbidden, body)
		}
	}
}

func TestHandleAgentResourceContext_OperatorStateAbsentWhenNoEntry(t *testing.T) {
	// A resource with no operator state recorded must omit the
	// operatorState field entirely (omitempty), distinguishing "no
	// operator overrides" from "operator overrides happen to be all
	// zero." Agents branch on field presence rather than value.
	h, _ := agentContextFixtureHandlers(t, "vm:fresh")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/vm:fresh", nil)
	h.HandleResourceContext(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d body=%s", rec.Code, rec.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, has := raw["operatorState"]; has {
		t.Errorf("operatorState must be omitted when no entry exists; got %v", raw["operatorState"])
	}
	// Active findings must be an empty array (not null), so agents can
	// iterate without nil-checking the field.
	if findings, ok := raw["activeFindings"].([]any); !ok {
		t.Errorf("activeFindings must be an array; got %T", raw["activeFindings"])
	} else if len(findings) != 0 {
		t.Errorf("expected empty findings array; got %d", len(findings))
	}
}

func TestHandleAgentResourceContext_404OnUnknownResource(t *testing.T) {
	h, _ := agentContextFixtureHandlers(t, "vm:101")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/vm:nonexistent", nil)
	h.HandleResourceContext(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404; got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal error body: %v", err)
	}
	if body["error"] != "resource_not_found" {
		t.Errorf("expected error=resource_not_found; got %v", body)
	}
}

func TestHandleAgentResourceContext_RejectsNonGet(t *testing.T) {
	h, _ := agentContextFixtureHandlers(t, "vm:101")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/agent/resource-context/vm:101", nil)
	h.HandleResourceContext(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405; got %d", rec.Code)
	}
}

func TestHandleAgentResourceCapabilities_ReturnsStructuredCapabilitiesAndParams(t *testing.T) {
	// Stage a resource advertising one capability with typed params. The
	// response must surface the full param schema (what formatCapabilityFact
	// deliberately omits in the resource-context bundle) so an agent can
	// populate plan_action.params without guessing. The internal handler
	// field must be absent from the JSON.
	resource := unified.Resource{
		ID:   "vm:101",
		Type: "vm",
		Name: "db-01",
		Capabilities: []unified.ResourceCapability{
			{
				Name:                 "restart",
				Type:                 unified.CapabilityTypeNative,
				Description:          "Restart the guest workload.",
				MinimumApprovalLevel: unified.ApprovalAdmin,
				Platform:             "qemu",
				InternalHandler:      "shouldNotAppearOnWire",
				Params: []unified.CapabilityParam{
					{
						Name:        "force",
						Type:        "boolean",
						Required:    false,
						Description: "Force a hard restart when the graceful stop times out.",
					},
					{
						Name:     "timeout",
						Type:     "int",
						Required: true,
						Enum:     []string{"30", "60", "120"},
						Pattern:  `^\d+$`,
					},
				},
			},
		},
	}
	h, _ := agentContextFixtureHandlersForResource(t, resource)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-capabilities/vm:101", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "default"))
	h.HandleResourceCapabilities(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d body=%s", rec.Code, rec.Body.String())
	}
	var bundle AgentResourceCapabilities
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("unmarshal bundle: %v", err)
	}
	if bundle.ResourceID != "vm:101" {
		t.Errorf("resourceId: got %q want vm:101", bundle.ResourceID)
	}
	if len(bundle.Capabilities) != 1 {
		t.Fatalf("capabilities: got %d want 1", len(bundle.Capabilities))
	}
	cap := bundle.Capabilities[0]
	if cap.Name != "restart" {
		t.Errorf("capability name: got %q want restart", cap.Name)
	}
	if cap.Platform != "qemu" {
		t.Errorf("platform: got %q want qemu", cap.Platform)
	}
	if cap.MinimumApprovalLevel != unified.ApprovalAdmin {
		t.Errorf("approval level: got %q want admin", cap.MinimumApprovalLevel)
	}
	if len(cap.Params) != 2 {
		t.Fatalf("params: got %d want 2", len(cap.Params))
	}

	// Param schemas must round-trip fully — this is the whole point of
	// the new endpoint relative to the prose summary in get_resource_context.
	force := cap.Params[0]
	if force.Name != "force" || force.Type != "boolean" || force.Required {
		t.Errorf("force param = %+v, want {force boolean false}", force)
	}
	if force.Description == "" {
		t.Errorf("force param description must be preserved, got empty")
	}
	timeout := cap.Params[1]
	if !timeout.Required || len(timeout.Enum) != 3 || timeout.Pattern == "" {
		t.Errorf("timeout param = %+v, want required with enum+pattern", timeout)
	}

	// Internal execution plumbing must never leak to the wire.
	if raw := strings.TrimSpace(rec.Body.String()); strings.Contains(raw, "shouldNotAppearOnWire") {
		t.Errorf("internal handler leaked to wire: %s", raw)
	}
}

func TestHandleAgentResourceCapabilities_EmptyCapabilitiesReturns200(t *testing.T) {
	// A resource advertising nothing is the common case across the fleet
	// and is exactly the signal an agent needs before attempting
	// plan_action. It must be 200 with a JSON array, never null.
	h, _ := agentContextFixtureHandlers(t, "vm:101")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-capabilities/vm:101", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "default"))
	h.HandleResourceCapabilities(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d body=%s", rec.Code, rec.Body.String())
	}
	var bundle AgentResourceCapabilities
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("unmarshal bundle: %v", err)
	}
	if bundle.Capabilities == nil {
		t.Fatal("capabilities is null; want non-nil empty array on the wire")
	}
	if len(bundle.Capabilities) != 0 {
		t.Errorf("capabilities: got %d want 0", len(bundle.Capabilities))
	}
	// Pin the wire shape directly: capabilities must serialize as [].
	if !strings.Contains(rec.Body.String(), `"capabilities":[]`) {
		t.Errorf("wire body must contain capabilities:[]; got %s", rec.Body.String())
	}
}

func TestHandleAgentResourceCapabilities_404OnUnknownResource(t *testing.T) {
	h, _ := agentContextFixtureHandlers(t, "vm:101")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-capabilities/vm:nonexistent", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "default"))
	h.HandleResourceCapabilities(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404; got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal error body: %v", err)
	}
	if body["error"] != agentcapabilities.AgentErrCodeResourceNotFound {
		t.Errorf("expected error=%s; got %v", agentcapabilities.AgentErrCodeResourceNotFound, body)
	}
}

func TestHandleAgentResourceCapabilities_RejectsNonGet(t *testing.T) {
	h, _ := agentContextFixtureHandlers(t, "vm:101")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/agent/resource-capabilities/vm:101", nil)
	h.HandleResourceCapabilities(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405; got %d", rec.Code)
	}
}

func TestHandleAgentResourceContext_RecentActionsCarryRefusalTokens(t *testing.T) {
	// Refused dispatches (resource_remediation_locked:, plan_drift:)
	// must surface verbatim in the recent-actions slice so agents can
	// branch on the stable token without parsing human messages. Pin
	// this to keep the agent-paradigm contract honest — Pulse already
	// records these refusals as Failed audit records (slices 17, 33);
	// this test verifies the bundle exposes them.
	h, _ := agentContextFixtureHandlers(t, "vm:locked")

	store, err := h.resources.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	now := time.Now().UTC()
	plan := unified.ActionPlan{
		ActionID:  "act-refused",
		RequestID: "req-refused",
		PlannedAt: now,
		ExpiresAt: now.Add(5 * time.Minute),
	}
	if err := store.RecordActionAudit(unified.ActionAuditRecord{
		ID:        "act-refused",
		CreatedAt: now,
		UpdatedAt: now,
		State:     unified.ActionStateFailed,
		Request: unified.ActionRequest{
			RequestID:      "req-refused",
			ResourceID:     "vm:locked",
			CapabilityName: "pulse_control",
			Reason:         "restart workload",
			RequestedBy:    "pulse_patrol",
			Params:         map[string]any{"command": "systemctl restart workload"},
		},
		Plan: plan,
		Result: &unified.ExecutionResult{
			Success:      false,
			ErrorMessage: "resource_remediation_locked: resource is operator-locked against automated remediation",
		},
	}); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/vm:locked", nil)
	h.HandleResourceContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d", rec.Code)
	}
	var bundle AgentResourceContext
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("unmarshal bundle: %v", err)
	}
	if len(bundle.RecentActions) != 1 {
		t.Fatalf("expected 1 recent action; got %d", len(bundle.RecentActions))
	}
	action := bundle.RecentActions[0]
	if action.State != string(unified.ActionStateFailed) {
		t.Errorf("state: got %q want failed", action.State)
	}
	if action.Success {
		t.Error("Success must be false on refused dispatch")
	}
	if action.ErrorMessage == "" || action.Command == "" {
		t.Errorf("error message and command must round-trip; got %+v", action)
	}
	// Stable token preservation — agents branch on the prefix.
	if !strings.Contains(action.ErrorMessage, "resource_remediation_locked:") {
		t.Errorf("ErrorMessage must preserve the stable refusal token; got %q", action.ErrorMessage)
	}
}

func TestHandleAgentResourceContext_PendingApprovalsAreScopedToResource(t *testing.T) {
	// pendingApprovals must list every still-pending approval
	// targeting the bundle's resource. The provider is the seam that
	// scopes by (resource id, org id) — the bundle handler trusts
	// the provider's filtering. Pin the contract: if the provider
	// returns N entries, the bundle returns N entries with the same
	// agent-stable shape (id, command, riskLevel, requestedBy,
	// requestedAt, expiresAt) and an empty array (never null) when
	// the provider returns nothing.
	h, _ := agentContextFixtureHandlers(t, "vm:101")
	now := time.Now().UTC()
	expires := now.Add(5 * time.Minute)
	h.SetApprovalsProvider(staticApprovalsProvider{
		byResource: map[string][]AgentResourceApprovalSummary{
			"vm:101": {
				{
					ID:          "appr-1",
					Command:     "systemctl restart nginx",
					RiskLevel:   "medium",
					RequestedBy: "ai:patrol",
					RequestedAt: now,
					ExpiresAt:   expires,
				},
			},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/vm:101", nil)
	h.HandleResourceContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var bundle AgentResourceContext
	if err := json.NewDecoder(rec.Body).Decode(&bundle); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(bundle.PendingApprovals) != 1 {
		t.Fatalf("PendingApprovals len = %d; want 1; body=%s", len(bundle.PendingApprovals), rec.Body.String())
	}
	got := bundle.PendingApprovals[0]
	if got.ID != "appr-1" {
		t.Errorf("ID: got %q, want %q", got.ID, "appr-1")
	}
	if got.Command != "systemctl restart nginx" {
		t.Errorf("Command did not round-trip: got %q", got.Command)
	}
	if got.RiskLevel != "medium" {
		t.Errorf("RiskLevel did not round-trip: got %q", got.RiskLevel)
	}
	if !got.ExpiresAt.Equal(expires) {
		t.Errorf("ExpiresAt did not round-trip: got %v, want %v", got.ExpiresAt, expires)
	}
}

func TestHandleAgentResourceContext_RedactsCommandsForMonitoringReadTokens(t *testing.T) {
	h, _ := agentContextFixtureHandlers(t, "vm:101")
	now := time.Now().UTC()
	expires := now.Add(5 * time.Minute)
	h.SetApprovalsProvider(staticApprovalsProvider{
		byResource: map[string][]AgentResourceApprovalSummary{
			"vm:101": {
				{
					ID:          "appr-1",
					Command:     "systemctl restart nginx",
					RiskLevel:   "medium",
					RequestedBy: "ai:patrol",
					RequestedAt: now,
					ExpiresAt:   expires,
				},
			},
		},
	})
	store, err := h.resources.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	if err := store.RecordActionAudit(unified.ActionAuditRecord{
		ID:        "act-1",
		CreatedAt: now,
		UpdatedAt: now,
		State:     unified.ActionStateCompleted,
		Request: unified.ActionRequest{
			RequestID:      "req-1",
			ResourceID:     "vm:101",
			CapabilityName: "pulse_control",
			RequestedBy:    "pulse_patrol",
			Params:         map[string]any{"command": "systemctl restart nginx"},
		},
		Result: &unified.ExecutionResult{
			Success: true,
			Verification: &unified.ActionVerificationResult{
				Ran:     true,
				Success: true,
				Command: "systemctl is-active nginx",
				RanAt:   now,
			},
		},
	}); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/vm:101", nil)
	token := &config.APITokenRecord{Scopes: []string{config.ScopeMonitoringRead}}
	attachAPITokenRecord(req, token)

	h.HandleResourceContext(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var bundle AgentResourceContext
	if err := json.NewDecoder(rec.Body).Decode(&bundle); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(bundle.PendingApprovals) != 1 {
		t.Fatalf("PendingApprovals len = %d; want 1", len(bundle.PendingApprovals))
	}
	if got := bundle.PendingApprovals[0]; got.Command != "" || !got.CommandRedacted {
		t.Fatalf("pending approval command must be redacted for monitoring-read token; got %+v", got)
	}
	if len(bundle.RecentActions) != 1 {
		t.Fatalf("RecentActions len = %d; want 1", len(bundle.RecentActions))
	}
	action := bundle.RecentActions[0]
	if action.Command != "" || !action.CommandRedacted {
		t.Fatalf("recent action command must be redacted for monitoring-read token; got %+v", action)
	}
	if action.Verification == nil {
		t.Fatal("expected verification to remain present")
	}
	if action.Verification.Command != "" || !action.Verification.CommandRedacted {
		t.Fatalf("verification command must be redacted for monitoring-read token; got %+v", action.Verification)
	}
}

func TestProjectAgentResourceActionsUsesCanonicalVerification(t *testing.T) {
	ranAt := time.Now().UTC()
	summaries := projectAgentResourceActions([]unified.ActionAuditRecord{
		{
			ID:        "action-resource-canonical-verification",
			CreatedAt: ranAt.Add(-time.Minute),
			UpdatedAt: ranAt,
			State:     unified.ActionStateCompleted,
			Request: unified.ActionRequest{
				ResourceID:     "vm:resource-canonical",
				CapabilityName: "restart_service",
				RequestedBy:    "agent:ops",
			},
			Result: &unified.ExecutionResult{Success: true},
			Verification: &unified.ActionVerificationResult{
				Ran:     true,
				Success: true,
				Command: "systemctl is-active nginx",
				RanAt:   ranAt,
			},
		},
		{
			ID:        "action-resource-unrun-verification",
			CreatedAt: ranAt.Add(-2 * time.Minute),
			UpdatedAt: ranAt.Add(-time.Minute),
			State:     unified.ActionStateCompleted,
			Request: unified.ActionRequest{
				ResourceID:     "vm:resource-unrun",
				CapabilityName: "restart_service",
				RequestedBy:    "agent:ops",
			},
			Result: &unified.ExecutionResult{
				Success: true,
				Verification: &unified.ActionVerificationResult{
					Ran:     false,
					Success: true,
					Command: "should not leak",
					Output:  "sensitive output",
					Note:    "sensitive note",
					RanAt:   ranAt,
				},
			},
		},
	})
	if len(summaries) != 2 {
		t.Fatalf("summaries len = %d, want 2", len(summaries))
	}
	if summaries[0].Verification == nil || summaries[0].Verification.Command != "systemctl is-active nginx" {
		t.Fatalf("top-level canonical verification did not project onto resource summary: %+v", summaries[0].Verification)
	}
	if summaries[1].Verification == nil || summaries[1].Verification.Ran {
		t.Fatalf("expected sanitized ran=false verification, got %+v", summaries[1].Verification)
	}
	if summaries[1].Verification.Command != "" || summaries[1].Verification.Note != "" || !summaries[1].Verification.RanAt.IsZero() || summaries[1].Verification.Success {
		t.Fatalf("resource summary leaked ran=false verification details: %+v", summaries[1].Verification)
	}
}

func TestHandleAgentResourceContext_PendingApprovalsEmptyArrayWhenNone(t *testing.T) {
	// Absent or empty must surface as an empty array, not as a
	// missing field — agents iterate without nil-checking. This
	// mirrors the contract for ActiveFindings and RecentActions.
	h, _ := agentContextFixtureHandlers(t, "vm:404")
	// No approvals provider wired — same observable shape as a
	// provider that returns nil for this resource.

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/vm:404", nil)
	h.HandleResourceContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"pendingApprovals":[]`) {
		t.Errorf("expected pendingApprovals to surface as empty array; body=%s", body)
	}
}

// fleetFixtureHandlers seeds the registry with a multi-resource set
// so the fleet sweep has something to walk. Mirrors the per-resource
// fixture but with N resources and an explicit findings/approvals
// stub for each.
func fleetFixtureHandlers(t *testing.T, resources []unified.Resource) (*AgentContextHandler, *staticFindingsProvider, *staticApprovalsProvider) {
	t.Helper()
	cfg := &config.Config{DataPath: t.TempDir()}
	rh := NewResourceHandlers(cfg)
	rh.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: time.Now()},
		resources: resources,
	})
	findings := &staticFindingsProvider{byResource: map[string][]AgentResourceFindingSnapshot{}}
	approvals := &staticApprovalsProvider{byResource: map[string][]AgentResourceApprovalSummary{}}
	h := NewAgentContextHandler(rh)
	h.SetFindingsProvider(findings)
	h.SetApprovalsProvider(approvals)
	h.SetExternalAgentReadinessProvider(agentExternalAgentReadinessProviderFunc(func(context.Context, agentcapabilities.Manifest, time.Time) bool {
		return true
	}))
	return h, findings, approvals
}

func TestHandleAgentFleetContext_RollsUpEveryResource(t *testing.T) {
	// The fleet view must surface one entry per resource in the
	// registry so an agent can scan the whole org in one read. Pin
	// the contract: identity round-trips, count is 1:1 with the
	// registry, list is always an array (never null).
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
		{ID: "container:web-1", Type: "container", Name: "web-1", Technology: "docker"},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context", nil)
	h.HandleFleetContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var fleet AgentFleetContext
	if err := json.NewDecoder(rec.Body).Decode(&fleet); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(fleet.Resources) != 2 {
		t.Fatalf("Resources len = %d; want 2; body=%s", len(fleet.Resources), rec.Body.String())
	}
	byID := map[string]AgentFleetResourceSummary{}
	for _, s := range fleet.Resources {
		byID[s.CanonicalID] = s
	}
	if got, ok := byID["vm:101"]; !ok || got.ResourceName != "db-01" || got.ResourceType != "vm" {
		t.Errorf("vm:101 entry missing or wrong: %+v", got)
	}
	if got, ok := byID["container:web-1"]; !ok || got.Technology != "docker" {
		t.Errorf("container:web-1 entry missing or wrong technology: %+v", got)
	}
}

func TestHandleAgentFleetContext_CountsFindingsBySeverity(t *testing.T) {
	// Per-severity finding counts are the headline number agents use
	// to triage. Pin the contract: counts roll up correctly across
	// (critical, warning, info) and total is the sum.
	h, findings, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
	})
	findings.byResource["vm:101"] = []AgentResourceFindingSnapshot{
		{ID: "f1", Severity: "critical"},
		{ID: "f2", Severity: "warning"},
		{ID: "f3", Severity: "warning"},
		{ID: "f4", Severity: "info"},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context", nil)
	h.HandleFleetContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	var fleet AgentFleetContext
	if err := json.NewDecoder(rec.Body).Decode(&fleet); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(fleet.Resources) != 1 {
		t.Fatalf("Resources len = %d; want 1", len(fleet.Resources))
	}
	got := fleet.Resources[0].Findings
	want := AgentFleetFindingCounts{Total: 4, Critical: 1, Warning: 2, Info: 1}
	if got != want {
		t.Errorf("findings counts = %+v; want %+v", got, want)
	}
}

func TestHandleAgentFleetContext_PropagatesPendingApprovalCount(t *testing.T) {
	// pendingApprovalCount must reflect the provider's per-resource
	// count so an agent triaging the fleet sees governance-blocked
	// resources at a glance.
	h, _, approvals := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
		{ID: "container:web-1", Type: "container", Name: "web-1"},
	})
	approvals.byResource["vm:101"] = []AgentResourceApprovalSummary{
		{ID: "appr-1"},
		{ID: "appr-2"},
	}
	// container:web-1 has no pending approvals — should surface as 0.

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context", nil)
	h.HandleFleetContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	var fleet AgentFleetContext
	if err := json.NewDecoder(rec.Body).Decode(&fleet); err != nil {
		t.Fatalf("decode: %v", err)
	}
	byID := map[string]AgentFleetResourceSummary{}
	for _, s := range fleet.Resources {
		byID[s.CanonicalID] = s
	}
	if got := byID["vm:101"].PendingApprovalCount; got != 2 {
		t.Errorf("vm:101 pendingApprovalCount = %d; want 2", got)
	}
	if got := byID["container:web-1"].PendingApprovalCount; got != 0 {
		t.Errorf("container:web-1 pendingApprovalCount = %d; want 0", got)
	}
}

func TestHandleAgentFleetContext_UsesBulkPendingApprovalCounts(t *testing.T) {
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
		{ID: "vm:202", Type: "vm", Name: "db-02"},
		{ID: "container:web-1", Type: "container", Name: "web-1"},
	})
	approvals := &countingApprovalsProvider{
		staticApprovalsProvider: staticApprovalsProvider{
			byResource: map[string][]AgentResourceApprovalSummary{
				"vm:101": {
					{ID: "appr-1"},
					{ID: "appr-2"},
				},
				"container:web-1": {
					{ID: "appr-3"},
				},
			},
		},
	}
	h.SetApprovalsProvider(approvals)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context", nil)
	h.HandleFleetContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	if approvals.countCalls != 1 {
		t.Fatalf("PendingApprovalCountsByResource calls = %d; want 1", approvals.countCalls)
	}
	if approvals.resourceCalls != 0 {
		t.Fatalf("PendingApprovalsForResource calls = %d; want 0", approvals.resourceCalls)
	}
	var fleet AgentFleetContext
	if err := json.NewDecoder(rec.Body).Decode(&fleet); err != nil {
		t.Fatalf("decode: %v", err)
	}
	byID := map[string]AgentFleetResourceSummary{}
	for _, s := range fleet.Resources {
		byID[s.CanonicalID] = s
	}
	if got := byID["vm:101"].PendingApprovalCount; got != 2 {
		t.Errorf("vm:101 pendingApprovalCount = %d; want 2", got)
	}
	if got := byID["vm:202"].PendingApprovalCount; got != 0 {
		t.Errorf("vm:202 pendingApprovalCount = %d; want 0", got)
	}
	if got := byID["container:web-1"].PendingApprovalCount; got != 1 {
		t.Errorf("container:web-1 pendingApprovalCount = %d; want 1", got)
	}
}

func TestHandleAgentFleetContext_EmptyArrayWhenRegistryEmpty(t *testing.T) {
	// Empty registry must surface as `resources: []`, never null —
	// agents iterate without nil-checking. This mirrors the
	// iteration-safe contract the per-resource bundle's sections
	// already follow.
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context", nil)
	h.HandleFleetContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"resources":[]`) {
		t.Errorf("expected resources to surface as empty array; body=%s", body)
	}
}

func TestHandleAgentFleetContext_RejectsNonGet(t *testing.T) {
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/agent/fleet-context", nil)
	h.HandleFleetContext(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d, want 405", rec.Code)
	}
}

// fleetFilterFixture seeds a mixed fleet for filter tests: one healthy VM
// (vm:healthy, no findings), one degraded VM (vm:critical, a critical finding),
// one docker container (container:web, a warning finding), and one LXC
// (container:lxcone, info finding). Covers every filter dimension.
func fleetFilterFixture(t *testing.T) *AgentContextHandler {
	t.Helper()
	h, findings, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:healthy", Type: "vm", Name: "healthy-vm", Technology: "qemu"},
		{ID: "vm:critical", Type: "vm", Name: "critical-vm", Technology: "qemu"},
		{ID: "container:web", Type: "container", Name: "web", Technology: "docker"},
		{ID: "container:lxcone", Type: "system-container", Name: "lxcone", Technology: "lxc"},
	})
	findings.byResource["vm:critical"] = []AgentResourceFindingSnapshot{
		{ID: "c1", Severity: "critical"},
	}
	findings.byResource["container:web"] = []AgentResourceFindingSnapshot{
		{ID: "w1", Severity: "warning"},
	}
	findings.byResource["container:lxcone"] = []AgentResourceFindingSnapshot{
		{ID: "i1", Severity: "info"},
	}
	return h
}

func decodeFleetResponse(t *testing.T, rec *httptest.ResponseRecorder) AgentFleetContext {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var fleet AgentFleetContext
	if err := json.NewDecoder(rec.Body).Decode(&fleet); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return fleet
}

func fleetCanonicalIDs(fleet AgentFleetContext) []string {
	ids := make([]string, 0, len(fleet.Resources))
	for _, r := range fleet.Resources {
		ids = append(ids, r.CanonicalID)
	}
	return ids
}

func TestHandleAgentFleetContext_HasFindingsFilter(t *testing.T) {
	// hasFindings=true must drop healthy resources and keep only those with at
	// least one finding — the headline triage filter. vm:healthy has none and
	// must be excluded.
	h := fleetFilterFixture(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context?hasFindings=true", nil)
	h.HandleFleetContext(rec, req)
	fleet := decodeFleetResponse(t, rec)
	got := fleetCanonicalIDs(fleet)
	want := []string{"vm:critical", "container:web", "container:lxcone"}
	if len(got) != len(want) {
		t.Fatalf("hasFindings=true returned %v; want %v (healthy must be excluded)", got, want)
	}
	gotSet := map[string]bool{}
	for _, id := range got {
		gotSet[id] = true
	}
	for _, id := range want {
		if !gotSet[id] {
			t.Errorf("hasFindings=true missing %s; got %v", id, got)
		}
	}
	if gotSet["vm:healthy"] {
		t.Errorf("vm:healthy must be excluded by hasFindings=true; got %v", got)
	}
}

func TestHandleAgentFleetContext_SeverityFilter(t *testing.T) {
	// severity=critical must keep only resources with a critical finding.
	h := fleetFilterFixture(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context?severity=critical", nil)
	h.HandleFleetContext(rec, req)
	fleet := decodeFleetResponse(t, rec)
	got := fleetCanonicalIDs(fleet)
	if len(got) != 1 || got[0] != "vm:critical" {
		t.Fatalf("severity=critical returned %v; want [vm:critical]", got)
	}

	// severity=warning must keep only the warning-bearing resource.
	h2 := fleetFilterFixture(t)
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context?severity=warning", nil)
	h2.HandleFleetContext(rec2, req2)
	fleet2 := decodeFleetResponse(t, rec2)
	got2 := fleetCanonicalIDs(fleet2)
	if len(got2) != 1 || got2[0] != "container:web" {
		t.Fatalf("severity=warning returned %v; want [container:web]", got2)
	}
}

func TestHandleAgentFleetContext_TechnologyFilter(t *testing.T) {
	// technology=docker (case-insensitive) must keep only the docker container.
	h := fleetFilterFixture(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context?technology=docker", nil)
	h.HandleFleetContext(rec, req)
	fleet := decodeFleetResponse(t, rec)
	got := fleetCanonicalIDs(fleet)
	if len(got) != 1 || got[0] != "container:web" {
		t.Fatalf("technology=docker returned %v; want [container:web]", got)
	}

	// Case-insensitivity: DOCKER must match the same resource.
	h2 := fleetFilterFixture(t)
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context?technology=DOCKER", nil)
	h2.HandleFleetContext(rec2, req2)
	fleet2 := decodeFleetResponse(t, rec2)
	got2 := fleetCanonicalIDs(fleet2)
	if len(got2) != 1 || got2[0] != "container:web" {
		t.Fatalf("technology=DOCKER returned %v; want [container:web] (case-insensitive)", got2)
	}
}

func TestHandleAgentFleetContext_ResourceTypeFilter(t *testing.T) {
	// resourceType=vm must keep only VMs (the two qemu VMs).
	h := fleetFilterFixture(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context?resourceType=vm", nil)
	h.HandleFleetContext(rec, req)
	fleet := decodeFleetResponse(t, rec)
	got := fleetCanonicalIDs(fleet)
	if len(got) != 2 {
		t.Fatalf("resourceType=vm returned %v; want 2 vms", got)
	}
	gotSet := map[string]bool{}
	for _, id := range got {
		gotSet[id] = true
	}
	if !gotSet["vm:healthy"] || !gotSet["vm:critical"] {
		t.Errorf("resourceType=vm must return both vms; got %v", got)
	}
}

func TestHandleAgentFleetContext_NoFilterReturnsAll(t *testing.T) {
	// Backward compatibility: no query params returns the full fleet.
	h := fleetFilterFixture(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context", nil)
	h.HandleFleetContext(rec, req)
	fleet := decodeFleetResponse(t, rec)
	if len(fleet.Resources) != 4 {
		t.Fatalf("no-filter fleet returned %d resources; want 4 (backward compat)", len(fleet.Resources))
	}
}

func TestHandleAgentFleetContext_UnknownFilterValueReturnsEmpty(t *testing.T) {
	// An unknown technology must return 200 with an empty array, not a 400.
	// The signal "no resource matched this filter" is a valid triage answer.
	h := fleetFilterFixture(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context?technology=nonexistent", nil)
	h.HandleFleetContext(rec, req)
	fleet := decodeFleetResponse(t, rec)
	if fleet.Resources == nil {
		t.Fatal("resources is null; want non-nil empty array")
	}
	if len(fleet.Resources) != 0 {
		t.Errorf("unknown technology returned %d resources; want 0", len(fleet.Resources))
	}
}

func TestHandleAgentFleetContext_FiltersCompose(t *testing.T) {
	// Composed filters must intersect: hasFindings=true AND technology=qemu
	// returns only the degraded VM (vm:critical), excluding the healthy qemu
	// VM and the non-qemu degraded resources.
	h := fleetFilterFixture(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context?hasFindings=true&technology=qemu", nil)
	h.HandleFleetContext(rec, req)
	fleet := decodeFleetResponse(t, rec)
	got := fleetCanonicalIDs(fleet)
	if len(got) != 1 || got[0] != "vm:critical" {
		t.Fatalf("composed filter returned %v; want [vm:critical] (degraded qemu only)", got)
	}
}

func TestHandleAgentOperationsLoopStatus_StartsAtPatrolWithoutIssueEvidence(t *testing.T) {
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/operations-loop/status", nil)
	h.HandleOperationsLoopStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var status AgentOperationsLoopStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertPatrolControlCompatibilityAliases(t, status)
	if status.NextAction != "run_patrol" {
		t.Fatalf("NextAction = %q; want run_patrol", status.NextAction)
	}
	if status.PatrolIssueEvidenceCount != 0 || status.ActiveFindingCount != 0 || status.PendingApprovalCount != 0 {
		t.Fatalf("empty loop should have zero issue counts: %+v", status)
	}
	if status.ProActivationValueProofState != "not_started" {
		t.Fatalf("ProActivationValueProofState = %q; want not_started", status.ProActivationValueProofState)
	}
	steps := agentOperationsLoopStepsByID(status.Steps)
	if got := steps["patrol"].Status; got != AgentOperationsLoopStepCurrent {
		t.Fatalf("patrol step status = %q; want current", got)
	}
	if got := steps["assistant"].Status; got != AgentOperationsLoopStepPending {
		t.Fatalf("assistant step status = %q; want pending", got)
	}
}

func TestAgentOperationsLoopExternalAgentTokenReadyUsesPulseMCPSurfaceScopes(t *testing.T) {
	now := time.Now().UTC()
	expiredAt := now.Add(-time.Minute)
	manifest := agentcapabilities.CanonicalManifest()
	operationLoopScopes := agentcapabilities.RequiredCapabilityScopes(
		agentcapabilities.ManifestSurfaceToolCapabilities(manifest, agentcapabilities.SurfaceIDPulseMCP),
	)
	if len(operationLoopScopes) < 2 {
		t.Fatalf("Pulse MCP operations loop must require multiple scope classes for this readiness test, got %v", operationLoopScopes)
	}

	if agentOperationsLoopExternalAgentTokenReady(manifest, nil, now) {
		t.Fatal("missing token must not satisfy external-agent readiness")
	}
	if agentOperationsLoopExternalAgentTokenReady(manifest, []config.APITokenRecord{
		{Scopes: []string{config.ScopeAIChat}, CreatedAt: now.Add(-time.Hour)},
	}, now) {
		t.Fatal("generic AI chat token must not satisfy external-agent readiness")
	}
	if agentOperationsLoopExternalAgentTokenReady(manifest, []config.APITokenRecord{
		{Scopes: []string{config.ScopeMonitoringRead}, ExpiresAt: &expiredAt, CreatedAt: now.Add(-time.Hour)},
	}, now) {
		t.Fatal("expired MCP-capable token must not satisfy external-agent readiness")
	}
	if agentOperationsLoopExternalAgentTokenReady(manifest, []config.APITokenRecord{
		{Scopes: []string{config.ScopeMonitoringRead}, CreatedAt: now.Add(-time.Hour)},
	}, now) {
		t.Fatal("read-only Pulse MCP token must not satisfy full operations-loop readiness")
	}
	if agentOperationsLoopExternalAgentTokenReady(manifest, []config.APITokenRecord{
		{Scopes: []string{config.ScopeAIExecute}, CreatedAt: now.Add(-time.Hour)},
	}, now) {
		t.Fatal("AI-execute-only Pulse MCP token must not satisfy full operations-loop readiness")
	}
	if agentOperationsLoopExternalAgentTokenReady(manifest, []config.APITokenRecord{
		{Scopes: []string{config.ScopeMonitoringRead}, CreatedAt: now.Add(-time.Hour)},
		{Scopes: []string{config.ScopeAIExecute}, CreatedAt: now.Add(-time.Hour)},
	}, now) {
		t.Fatal("split partial tokens must not satisfy single-token operations-loop readiness")
	}
	if !agentOperationsLoopExternalAgentTokenReady(manifest, []config.APITokenRecord{
		{Scopes: operationLoopScopes, CreatedAt: now.Add(-time.Hour)},
	}, now) {
		t.Fatalf("one token covering all Pulse MCP operations-loop scopes should satisfy external-agent readiness; scopes=%v", operationLoopScopes)
	}
}

func TestHandleAgentOperationsLoopStatus_AggregatesContentSafeLoopCounts(t *testing.T) {
	h, findings, approvals := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
	})
	findings.byResource["vm:101"] = []AgentResourceFindingSnapshot{
		{ID: "finding-cpu", Title: "CPU pressure", Severity: "warning"},
		{ID: "finding-disk", Title: "Disk pressure", Severity: "critical"},
	}
	approvals.byResource["vm:101"] = []AgentResourceApprovalSummary{{ID: "approval-1"}}

	store, err := h.resources.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	now := time.Now().UTC()
	h.SetAIUsageProvider(staticAIUsageProvider{
		history: &config.AIUsageHistoryData{
			Events: []config.AIUsageEventRecord{{
				Timestamp:    now.Add(-7 * time.Minute),
				UseCase:      "chat",
				ContextScope: "patrol_finding",
				FindingID:    "finding-cpu",
			}},
		},
	})
	if err := store.RecordActionAudit(unified.ActionAuditRecord{
		ID:        "act-verified",
		CreatedAt: now.Add(-5 * time.Minute),
		UpdatedAt: now,
		State:     unified.ActionStateCompleted,
		Request: unified.ActionRequest{
			RequestID:      "req-verified",
			ResourceID:     "vm:101",
			CapabilityName: "restart_service",
			Params:         map[string]any{"command": "systemctl restart nginx"},
			RequestedBy:    "pulse_patrol",
		},
		Plan: unified.ActionPlan{
			ActionID:  "act-verified",
			RequestID: "req-verified",
			PlannedAt: now.Add(-6 * time.Minute),
			ExpiresAt: now.Add(time.Hour),
		},
		Approvals: []unified.ActionApprovalRecord{{
			Actor:     "operator@example.com",
			Method:    unified.MethodUI,
			Timestamp: now.Add(-4 * time.Minute),
			Outcome:   unified.OutcomeApproved,
		}},
		Result: &unified.ExecutionResult{
			Success: true,
			Output:  "service restarted",
		},
		VerificationOutcome: unified.VerificationOutcome{Status: unified.VerificationVerified},
	}); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/operations-loop/status", nil)
	h.HandleOperationsLoopStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, forbidden := range []string{
		"finding-cpu",
		"finding-disk",
		"act-verified",
		"approval-1",
		"vm:101",
		"db-01",
		"systemctl",
		"service restarted",
		"operator@example.com",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("operations-loop status leaked %q: %s", forbidden, body)
		}
	}

	var status AgentOperationsLoopStatus
	if err := json.Unmarshal([]byte(body), &status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertPatrolControlCompatibilityAliases(t, status)
	if status.NextAction != "review_approvals" {
		t.Fatalf("NextAction = %q; want review_approvals; status=%+v", status.NextAction, status)
	}
	if !status.ExternalAgentReady {
		t.Fatal("canonical manifest should make external-agent readiness true")
	}
	if status.ActiveFindingCount != 2 ||
		status.PendingApprovalCount != 1 ||
		status.GovernedActionCount != 1 ||
		status.ApprovedDecisionCount != 1 ||
		status.RejectedDecisionCount != 0 ||
		status.VerifiedOutcomeCount != 1 {
		t.Fatalf("loop counts wrong: %+v", status)
	}
	if status.PatrolEvidenceCount != 4 || status.PatrolIssueEvidenceCount != 5 {
		t.Fatalf("evidence counts wrong: patrol=%d issue=%d", status.PatrolEvidenceCount, status.PatrolIssueEvidenceCount)
	}
	steps := agentOperationsLoopStepsByID(status.Steps)
	if got := steps["governance"].Count; got != 1 {
		t.Fatalf("governance step count = %d; want approved decision count", got)
	}
	if got := steps["assistant"].Count; got != 1 {
		t.Fatalf("assistant step count = %d; want contextual collaboration count", got)
	}
	if got := steps["verification"].Count; got != 1 {
		t.Fatalf("verification step count = %d; want verified outcome count", got)
	}
	if got := steps["patrol"].Status; got != AgentOperationsLoopStepComplete {
		t.Fatalf("patrol step status = %q; want complete; steps=%+v", got, status.Steps)
	}
	if got := steps["assistant"].Status; got != AgentOperationsLoopStepComplete {
		t.Fatalf("assistant step status = %q; want complete; steps=%+v", got, status.Steps)
	}
	if got := steps["governance"].Status; got != AgentOperationsLoopStepCurrent {
		t.Fatalf("governance step status = %q; want current; steps=%+v", got, status.Steps)
	}
	if got := steps["verification"].Status; got != AgentOperationsLoopStepPending {
		t.Fatalf("verification step status = %q; want pending; steps=%+v", got, status.Steps)
	}
}

func TestHandleAgentOperationsLoopStatus_ActiveAggregateFindingOutranksHistoricalCompletion(t *testing.T) {
	h, findings, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
	})
	activeCount := 1
	findings.activeCount = &activeCount
	now := time.Now().UTC()
	h.SetWorkflowPromptActivityProvider(staticWorkflowPromptActivityProvider{
		history: &config.WorkflowPromptActivityHistoryData{
			Events: []config.WorkflowPromptActivityRecord{{
				Timestamp:  now.Add(-10 * time.Minute),
				Surface:    config.WorkflowPromptActivitySurfacePulsePatrol,
				PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
			}},
		},
	})
	h.SetAIUsageProvider(staticAIUsageProvider{
		history: &config.AIUsageHistoryData{
			Events: []config.AIUsageEventRecord{{
				Timestamp:    now.Add(-7 * time.Minute),
				UseCase:      "chat",
				ContextScope: "patrol_finding",
			}},
		},
	})
	store, err := h.resources.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	if err := store.RecordActionAudit(unified.ActionAuditRecord{
		ID:        "act-old-verified",
		CreatedAt: now.Add(-5 * time.Minute),
		UpdatedAt: now,
		State:     unified.ActionStateCompleted,
		Request: unified.ActionRequest{
			RequestID:      "req-old-verified",
			ResourceID:     "vm:101",
			CapabilityName: "restart_service",
			RequestedBy:    "pulse_patrol",
		},
		Plan: unified.ActionPlan{
			ActionID:  "act-old-verified",
			RequestID: "req-old-verified",
			PlannedAt: now.Add(-6 * time.Minute),
			ExpiresAt: now.Add(time.Hour),
		},
		Approvals: []unified.ActionApprovalRecord{{
			Method:    unified.MethodUI,
			Timestamp: now.Add(-4 * time.Minute),
			Outcome:   unified.OutcomeApproved,
		}},
		Result:              &unified.ExecutionResult{Success: true},
		VerificationOutcome: unified.VerificationOutcome{Status: unified.VerificationVerified},
	}); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/operations-loop/status", nil)
	h.HandleOperationsLoopStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var status AgentOperationsLoopStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertPatrolControlCompatibilityAliases(t, status)
	if status.ActiveFindingCount != 1 {
		t.Fatalf("ActiveFindingCount = %d; want aggregate active finding", status.ActiveFindingCount)
	}
	if status.PatrolEvidenceCount != 2 || status.PatrolIssueEvidenceCount != 3 {
		t.Fatalf("evidence counts wrong: patrol=%d issue=%d status=%+v", status.PatrolEvidenceCount, status.PatrolIssueEvidenceCount, status)
	}
	if status.PatrolControlResolvedLoopCount != 1 || status.PatrolControlValueState != "verified" {
		t.Fatalf("historical resolved proof should remain recorded: %+v", status)
	}
	if status.NextAction != "open_assistant" {
		t.Fatalf("NextAction = %q; want open_assistant for the current active finding; status=%+v", status.NextAction, status)
	}
	if status.ProgressLabel != "Open Assistant on the active Patrol finding before treating previous verified work as current." {
		t.Fatalf("ProgressLabel = %q", status.ProgressLabel)
	}
	steps := agentOperationsLoopStepsByID(status.Steps)
	if steps["patrol"].Status != AgentOperationsLoopStepComplete ||
		steps["assistant"].Status != AgentOperationsLoopStepCurrent ||
		steps["verification"].Status != AgentOperationsLoopStepPending {
		t.Fatalf("steps should point back to the current active finding: %+v", status.Steps)
	}
}

func TestHandleAgentOperationsLoopStatus_AggregatesWorkflowStarterCounts(t *testing.T) {
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
	})
	now := time.Now().UTC()
	h.SetWorkflowPromptActivityProvider(staticWorkflowPromptActivityProvider{
		history: &config.WorkflowPromptActivityHistoryData{
			Events: []config.WorkflowPromptActivityRecord{
				{
					Timestamp:  now.Add(-10 * time.Minute),
					Surface:    config.WorkflowPromptActivitySurfacePulseAssistant,
					PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
				},
				{
					Timestamp:  now.Add(-9 * time.Minute),
					Surface:    config.WorkflowPromptActivitySurfacePulseAssistant,
					PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
				},
				{
					Timestamp:  now.Add(-8 * time.Minute),
					Surface:    config.WorkflowPromptActivitySurfacePulsePatrol,
					PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
				},
				{
					Timestamp:  now.Add(-7 * time.Minute),
					Surface:    config.WorkflowPromptActivitySurfacePatrolControl,
					PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
				},
				{
					Timestamp:  now.Add(-6 * time.Minute),
					Surface:    config.WorkflowPromptActivitySurfacePatrolAutonomy,
					PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
				},
				{
					Timestamp:  now.Add(-5 * time.Minute),
					Surface:    config.WorkflowPromptActivitySurfaceProActivation,
					PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
				},
				{
					Timestamp:  now.Add(-4 * time.Minute),
					Surface:    config.WorkflowPromptActivitySurfacePulseMCP,
					PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
				},
				{
					Timestamp:  now.Add(-3 * time.Minute),
					Surface:    config.WorkflowPromptActivitySurfaceAgentAPI,
					PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
				},
				{
					Timestamp:  now.Add(-2 * time.Minute),
					Surface:    config.WorkflowPromptActivitySurfacePulseAssistant,
					PromptName: agentcapabilities.PulseWorkflowPromptTriageFleet,
				},
				{
					Timestamp:  now.Add(-agentOperationsLoopActionWindow - time.Hour),
					Surface:    config.WorkflowPromptActivitySurfacePulseMCP,
					PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
				},
			},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/operations-loop/status", nil)
	h.HandleOperationsLoopStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, forbidden := range []string{
		agentcapabilities.PulseWorkflowPromptOperationsLoop,
		agentcapabilities.PulseWorkflowPromptTriageFleet,
		config.WorkflowPromptActivitySurfacePulseAssistant,
		config.WorkflowPromptActivitySurfacePulsePatrol,
		config.WorkflowPromptActivitySurfacePatrolControl,
		config.WorkflowPromptActivitySurfacePatrolAutonomy,
		config.WorkflowPromptActivitySurfaceProActivation,
		config.WorkflowPromptActivitySurfacePulseMCP,
		config.WorkflowPromptActivitySurfaceAgentAPI,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("operations-loop status leaked workflow activity detail %q: %s", forbidden, body)
		}
	}

	var status AgentOperationsLoopStatus
	if err := json.Unmarshal([]byte(body), &status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertPatrolControlCompatibilityAliases(t, status)
	if status.OperationsLoopStarterCount != 8 ||
		status.AssistantOperationsLoopStarterCount != 2 ||
		status.PatrolOperationsLoopStarterCount != 1 ||
		status.PatrolControlLoopStarterCount != 4 ||
		status.PatrolControlResolvedLoopCount != 0 ||
		status.ProActivationLoopStarterCount != 1 ||
		status.ProActivationResolvedLoopCount != 0 ||
		status.MCPOperationsLoopStarterCount != 1 {
		t.Fatalf("workflow starter counts wrong: %+v", status)
	}
	if status.PatrolControlValueState != "in_progress" {
		t.Fatalf("PatrolControlValueState = %q; want in_progress", status.PatrolControlValueState)
	}
	if status.ProActivationValueProofState != "in_progress" {
		t.Fatalf("ProActivationValueProofState = %q; want in_progress", status.ProActivationValueProofState)
	}
	if status.NextAction != "run_patrol" ||
		status.ProgressLabel != "Patrol is ready to check the fleet and investigate the next real infrastructure issue." {
		t.Fatalf("workflow starter progress wrong: %+v", status)
	}
}

func TestHandleAgentOperationsLoopStatus_CountsPulsePatrolStarterAsPatrolControl(t *testing.T) {
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
	})
	now := time.Now().UTC()
	h.SetWorkflowPromptActivityProvider(staticWorkflowPromptActivityProvider{
		history: &config.WorkflowPromptActivityHistoryData{
			Events: []config.WorkflowPromptActivityRecord{{
				Timestamp:  now.Add(-10 * time.Minute),
				Surface:    config.WorkflowPromptActivitySurfacePulsePatrol,
				PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
			}},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/operations-loop/status", nil)
	h.HandleOperationsLoopStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}

	var status AgentOperationsLoopStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertPatrolControlCompatibilityAliases(t, status)
	if status.PatrolOperationsLoopStarterCount != 1 ||
		status.PatrolControlLoopStarterCount != 1 ||
		status.ProActivationLoopStarterCount != 0 {
		t.Fatalf("native Patrol starter counts wrong: %+v", status)
	}
	if status.PatrolControlValueState != "in_progress" ||
		status.ProActivationValueProofState != "in_progress" {
		t.Fatalf("native Patrol starter value state wrong: %+v", status)
	}
	if status.NextAction != "run_patrol" ||
		status.ProgressLabel != "Patrol is ready to check the fleet and investigate the next real infrastructure issue." {
		t.Fatalf("native Patrol starter progress wrong: %+v", status)
	}
}

func TestHandleAgentOperationsLoopStatus_CountsPatrolControlStarterWithoutLegacyProActivation(t *testing.T) {
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
	})
	now := time.Now().UTC()
	h.SetWorkflowPromptActivityProvider(staticWorkflowPromptActivityProvider{
		history: &config.WorkflowPromptActivityHistoryData{
			Events: []config.WorkflowPromptActivityRecord{{
				Timestamp:  now.Add(-10 * time.Minute),
				Surface:    config.WorkflowPromptActivitySurfacePatrolControl,
				PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
			}},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/operations-loop/status", nil)
	h.HandleOperationsLoopStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}

	var status AgentOperationsLoopStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertPatrolControlCompatibilityAliases(t, status)
	if status.PatrolOperationsLoopStarterCount != 0 ||
		status.PatrolControlLoopStarterCount != 1 ||
		status.ProActivationLoopStarterCount != 0 {
		t.Fatalf("Patrol control starter counts wrong: %+v", status)
	}
	if status.PatrolControlValueState != "in_progress" ||
		status.ProActivationValueProofState != "in_progress" {
		t.Fatalf("Patrol control starter value state wrong: %+v", status)
	}
	if status.NextAction != "run_patrol" ||
		status.ProgressLabel != "Patrol is ready to check the fleet and investigate the next real infrastructure issue." {
		t.Fatalf("Patrol control starter progress wrong: %+v", status)
	}
}

func TestHandleAgentOperationsLoopStatus_ProjectsPatrolControlResolvedLoop(t *testing.T) {
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
	})
	now := time.Now().UTC()
	h.SetAIUsageProvider(staticAIUsageProvider{
		history: &config.AIUsageHistoryData{
			Events: []config.AIUsageEventRecord{{
				Timestamp:    now.Add(-9 * time.Minute),
				UseCase:      "chat",
				ContextScope: "patrol_finding",
				FindingID:    "finding-pro",
			}},
		},
	})
	h.SetWorkflowPromptActivityProvider(staticWorkflowPromptActivityProvider{
		history: &config.WorkflowPromptActivityHistoryData{
			Events: []config.WorkflowPromptActivityRecord{
				{
					Timestamp:  now.Add(-10 * time.Minute),
					Surface:    config.WorkflowPromptActivitySurfacePulsePatrol,
					PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
				},
			},
		},
	})

	store, err := h.resources.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	if err := store.RecordActionAudit(unified.ActionAuditRecord{
		ID:        "act-pro-verified",
		CreatedAt: now.Add(-5 * time.Minute),
		UpdatedAt: now,
		State:     unified.ActionStateCompleted,
		Request: unified.ActionRequest{
			RequestID:      "req-pro-verified",
			ResourceID:     "vm:101",
			CapabilityName: "restart_service",
			Params:         map[string]any{"command": "systemctl restart nginx"},
			RequestedBy:    "pulse_patrol",
		},
		Plan: unified.ActionPlan{
			ActionID:  "act-pro-verified",
			RequestID: "req-pro-verified",
			PlannedAt: now.Add(-6 * time.Minute),
			ExpiresAt: now.Add(time.Hour),
		},
		Approvals: []unified.ActionApprovalRecord{{
			Actor:     "operator@example.com",
			Method:    unified.MethodUI,
			Timestamp: now.Add(-4 * time.Minute),
			Outcome:   unified.OutcomeApproved,
		}},
		Result: &unified.ExecutionResult{
			Success: true,
			Output:  "service restarted",
		},
		VerificationOutcome: unified.VerificationOutcome{Status: unified.VerificationVerified},
	}); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/operations-loop/status", nil)
	h.HandleOperationsLoopStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, forbidden := range []string{
		agentcapabilities.PulseWorkflowPromptOperationsLoop,
		config.WorkflowPromptActivitySurfacePulsePatrol,
		config.WorkflowPromptActivitySurfacePatrolAutonomy,
		config.WorkflowPromptActivitySurfaceProActivation,
		"act-pro-verified",
		"req-pro-verified",
		"vm:101",
		"db-01",
		"systemctl",
		"service restarted",
		"operator@example.com",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("operations-loop status leaked %q: %s", forbidden, body)
		}
	}

	var status AgentOperationsLoopStatus
	if err := json.Unmarshal([]byte(body), &status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertPatrolControlCompatibilityAliases(t, status)
	if status.PatrolControlLoopStarterCount != 1 ||
		status.PatrolControlCompletedLoopCount != 1 ||
		status.PatrolControlResolvedLoopCount != 1 ||
		status.ProActivationLoopStarterCount != 0 ||
		status.ProActivationCompletedLoopCount != 1 ||
		status.ProActivationResolvedLoopCount != 1 ||
		status.VerifiedOutcomeCount != 1 ||
		status.ApprovedDecisionCount != 1 {
		t.Fatalf("Patrol control resolved-loop counts wrong: %+v", status)
	}
	if status.PatrolControlValueState != "verified" {
		t.Fatalf("PatrolControlValueState = %q; want verified", status.PatrolControlValueState)
	}
	if status.ProActivationValueProofState != "verified" {
		t.Fatalf("ProActivationValueProofState = %q; want verified", status.ProActivationValueProofState)
	}
	if status.NextAction != "complete" ||
		status.ProgressLabel != "Patrol handled an infrastructure issue, verified the outcome, and recorded what happened." {
		t.Fatalf("Patrol control resolved-loop progress wrong: %+v", status)
	}
	steps := agentOperationsLoopStepsByID(status.Steps)
	if got := steps["assistant"].Count; got != 1 {
		t.Fatalf("assistant step count = %d; want contextual collaboration count", got)
	}
	for _, step := range status.Steps {
		if step.Status != AgentOperationsLoopStepComplete {
			t.Fatalf("step %s status = %q; want complete; steps=%+v", step.ID, step.Status, status.Steps)
		}
	}
}

func TestHandleAgentOperationsLoopStatus_ResolvesPatrolControlWithoutExternalAgentReadiness(t *testing.T) {
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
	})
	h.SetExternalAgentReadinessProvider(nil)
	now := time.Now().UTC()
	h.SetAIUsageProvider(staticAIUsageProvider{
		history: &config.AIUsageHistoryData{
			Events: []config.AIUsageEventRecord{{
				Timestamp:    now.Add(-9 * time.Minute),
				UseCase:      "chat",
				ContextScope: "patrol_finding",
				FindingID:    "finding-pro",
			}},
		},
	})
	h.SetWorkflowPromptActivityProvider(staticWorkflowPromptActivityProvider{
		history: &config.WorkflowPromptActivityHistoryData{
			Events: []config.WorkflowPromptActivityRecord{{
				Timestamp:  now.Add(-10 * time.Minute),
				Surface:    config.WorkflowPromptActivitySurfaceProActivation,
				PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
			}},
		},
	})

	store, err := h.resources.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	if err := store.RecordActionAudit(unified.ActionAuditRecord{
		ID:        "act-pro-verified",
		CreatedAt: now.Add(-5 * time.Minute),
		UpdatedAt: now,
		State:     unified.ActionStateCompleted,
		Request: unified.ActionRequest{
			RequestID:      "req-pro-verified",
			ResourceID:     "vm:101",
			CapabilityName: "restart_service",
			Params:         map[string]any{"command": "systemctl restart nginx"},
			RequestedBy:    "pulse_patrol",
		},
		Plan: unified.ActionPlan{
			ActionID:  "act-pro-verified",
			RequestID: "req-pro-verified",
			PlannedAt: now.Add(-6 * time.Minute),
			ExpiresAt: now.Add(time.Hour),
		},
		Approvals: []unified.ActionApprovalRecord{{
			Actor:     "operator@example.com",
			Method:    unified.MethodUI,
			Timestamp: now.Add(-4 * time.Minute),
			Outcome:   unified.OutcomeApproved,
		}},
		Result: &unified.ExecutionResult{
			Success: true,
			Output:  "service restarted",
		},
		VerificationOutcome: unified.VerificationOutcome{Status: unified.VerificationVerified},
	}); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/operations-loop/status", nil)
	h.HandleOperationsLoopStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}

	var status AgentOperationsLoopStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertPatrolControlCompatibilityAliases(t, status)
	if status.ExternalAgentReady ||
		status.PatrolControlCompletedLoopCount != 1 ||
		status.PatrolControlResolvedLoopCount != 1 ||
		status.ProActivationCompletedLoopCount != 1 ||
		status.ProActivationResolvedLoopCount != 1 ||
		status.VerifiedOutcomeCount != 1 ||
		status.ApprovedDecisionCount != 1 {
		t.Fatalf("Patrol control should resolve independently from optional external-agent readiness: %+v", status)
	}
	if status.PatrolControlValueState != "verified" {
		t.Fatalf("PatrolControlValueState = %q; want verified", status.PatrolControlValueState)
	}
	if status.ProActivationValueProofState != "verified" {
		t.Fatalf("ProActivationValueProofState = %q; want verified", status.ProActivationValueProofState)
	}
	if status.NextAction != "complete" ||
		status.ProgressLabel != "Patrol handled an infrastructure issue, verified the outcome, and recorded what happened." {
		t.Fatalf("Patrol control without external-agent readiness progress wrong: %+v", status)
	}
	steps := agentOperationsLoopStepsByID(status.Steps)
	if _, ok := steps["external_agents"]; ok {
		t.Fatalf("external-agent readiness must not render as an operator step: %+v", status.Steps)
	}
	for _, stepID := range []string{"patrol", "assistant", "governance", "verification"} {
		if got := steps[stepID].Status; got != AgentOperationsLoopStepComplete {
			t.Fatalf("%s step status = %q; want complete; steps=%+v", stepID, got, status.Steps)
		}
	}
}

func TestHandleAgentOperationsLoopStatus_DoesNotResolvePatrolControlWithoutContextualCollaboration(t *testing.T) {
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
	})
	now := time.Now().UTC()
	h.SetWorkflowPromptActivityProvider(staticWorkflowPromptActivityProvider{
		history: &config.WorkflowPromptActivityHistoryData{
			Events: []config.WorkflowPromptActivityRecord{
				{
					Timestamp:  now.Add(-10 * time.Minute),
					Surface:    config.WorkflowPromptActivitySurfacePulsePatrol,
					PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
				},
			},
		},
	})

	store, err := h.resources.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	if err := store.RecordActionAudit(unified.ActionAuditRecord{
		ID:        "act-pro-verified-no-context",
		CreatedAt: now.Add(-5 * time.Minute),
		UpdatedAt: now,
		State:     unified.ActionStateCompleted,
		Request: unified.ActionRequest{
			RequestID:      "req-pro-verified-no-context",
			ResourceID:     "vm:101",
			CapabilityName: "restart_service",
			RequestedBy:    "pulse_patrol",
		},
		Plan: unified.ActionPlan{
			ActionID:  "act-pro-verified-no-context",
			RequestID: "req-pro-verified-no-context",
			PlannedAt: now.Add(-6 * time.Minute),
			ExpiresAt: now.Add(time.Hour),
		},
		Approvals: []unified.ActionApprovalRecord{{
			Actor:     "operator@example.com",
			Method:    unified.MethodUI,
			Timestamp: now.Add(-4 * time.Minute),
			Outcome:   unified.OutcomeApproved,
		}},
		Result:              &unified.ExecutionResult{Success: true},
		VerificationOutcome: unified.VerificationOutcome{Status: unified.VerificationVerified},
	}); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/operations-loop/status", nil)
	h.HandleOperationsLoopStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}

	var status AgentOperationsLoopStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertPatrolControlCompatibilityAliases(t, status)
	if status.ProActivationLoopStarterCount != 0 ||
		status.PatrolControlLoopStarterCount != 1 ||
		status.PatrolControlResolvedLoopCount != 0 ||
		status.ProActivationResolvedLoopCount != 0 ||
		status.ApprovedDecisionCount != 1 ||
		status.VerifiedOutcomeCount != 1 {
		t.Fatalf("Patrol control weak-evidence counts wrong: %+v", status)
	}
	if status.NextAction != "open_assistant" ||
		status.ProgressLabel != "Open Assistant to explain the Patrol issue and safest next step." {
		t.Fatalf("weak Patrol control progress wrong: %+v", status)
	}
	steps := agentOperationsLoopStepsByID(status.Steps)
	if got := steps["assistant"].Status; got != AgentOperationsLoopStepCurrent {
		t.Fatalf("assistant step status = %q; want current; steps=%+v", got, status.Steps)
	}
	if got := steps["verification"].Status; got != AgentOperationsLoopStepPending {
		t.Fatalf("verification step status = %q; want pending until context proof; steps=%+v", got, status.Steps)
	}
}

func TestHandleAgentOperationsLoopStatus_DoesNotTreatUnapprovedExecutionAsGovernedDecision(t *testing.T) {
	h, findings, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
	})
	findings.byResource["vm:101"] = []AgentResourceFindingSnapshot{
		{ID: "finding-cpu", Title: "CPU pressure", Severity: "warning"},
	}

	store, err := h.resources.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	now := time.Now().UTC()
	if err := store.RecordActionAudit(unified.ActionAuditRecord{
		ID:        "act-unapproved",
		CreatedAt: now.Add(-5 * time.Minute),
		UpdatedAt: now,
		State:     unified.ActionStateCompleted,
		Request: unified.ActionRequest{
			RequestID:      "req-unapproved",
			ResourceID:     "vm:101",
			CapabilityName: "restart_service",
			Params:         map[string]any{"command": "systemctl restart nginx"},
			RequestedBy:    "pulse_patrol",
		},
		Plan: unified.ActionPlan{
			ActionID:         "act-unapproved",
			RequestID:        "req-unapproved",
			PlannedAt:        now.Add(-6 * time.Minute),
			ExpiresAt:        now.Add(time.Hour),
			RequiresApproval: false,
		},
		Result: &unified.ExecutionResult{
			Success: true,
			Output:  "service restarted",
		},
		VerificationOutcome: unified.VerificationOutcome{Status: unified.VerificationVerified},
	}); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/operations-loop/status", nil)
	h.HandleOperationsLoopStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var status AgentOperationsLoopStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertPatrolControlCompatibilityAliases(t, status)
	if status.NextAction != "open_assistant" {
		t.Fatalf("NextAction = %q; want open_assistant; status=%+v", status.NextAction, status)
	}
	if status.GovernedActionCount != 0 ||
		status.ApprovedDecisionCount != 0 ||
		status.RejectedDecisionCount != 0 ||
		status.VerifiedOutcomeCount != 0 {
		t.Fatalf("unapproved execution must not satisfy governance or verification: %+v", status)
	}
	steps := agentOperationsLoopStepsByID(status.Steps)
	if got := steps["governance"].Status; got != AgentOperationsLoopStepPending {
		t.Fatalf("governance status = %q; want pending; steps=%+v", got, status.Steps)
	}
	if got := steps["verification"].Status; got != AgentOperationsLoopStepPending {
		t.Fatalf("verification status = %q; want pending; steps=%+v", got, status.Steps)
	}
}

func TestHandleAgentOperationsLoopStatus_UsesDecisionLifecycleEvidenceForOlderPlans(t *testing.T) {
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
	})

	store, err := h.resources.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	now := time.Now().UTC()
	h.SetAIUsageProvider(staticAIUsageProvider{
		history: &config.AIUsageHistoryData{
			Events: []config.AIUsageEventRecord{{
				Timestamp:    now.Add(-10 * time.Minute),
				UseCase:      "chat",
				ContextScope: "patrol_finding",
			}},
		},
	})
	h.SetWorkflowPromptActivityProvider(staticWorkflowPromptActivityProvider{
		history: &config.WorkflowPromptActivityHistoryData{
			Events: []config.WorkflowPromptActivityRecord{{
				PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
				Surface:    config.WorkflowPromptActivitySurfaceProActivation,
				Timestamp:  now.Add(-9 * time.Minute),
			}},
		},
	})
	old := now.Add(-agentOperationsLoopActionWindow - time.Hour)
	record := unified.ActionAuditRecord{
		ID:        "act-rejected",
		CreatedAt: old,
		UpdatedAt: old,
		State:     unified.ActionStatePending,
		Request: unified.ActionRequest{
			RequestID:      "req-rejected",
			ResourceID:     "vm:101",
			CapabilityName: "restart_service",
			Params:         map[string]any{"command": "systemctl restart nginx"},
			RequestedBy:    "pulse_patrol",
		},
		Plan: unified.ActionPlan{
			ActionID:         "act-rejected",
			RequestID:        "req-rejected",
			PlannedAt:        old,
			ExpiresAt:        now.Add(time.Hour),
			RequiresApproval: true,
		},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}
	rejected, event, err := unified.ApplyActionDecision(record, unified.ActionApprovalRecord{
		Actor:   "operator@example.com",
		Method:  unified.MethodUI,
		Outcome: unified.OutcomeRejected,
	}, now)
	if err != nil {
		t.Fatalf("ApplyActionDecision: %v", err)
	}
	if err := store.RecordActionDecision(rejected, event); err != nil {
		t.Fatalf("RecordActionDecision: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/operations-loop/status", nil)
	h.HandleOperationsLoopStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var status AgentOperationsLoopStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertPatrolControlCompatibilityAliases(t, status)
	if status.GovernedActionCount != 1 || status.RejectedDecisionCount != 1 || status.ApprovedDecisionCount != 0 {
		t.Fatalf("decision counts wrong for rejected action: %+v", status)
	}
	if status.VerifiedOutcomeCount != 0 {
		t.Fatalf("VerifiedOutcomeCount = %d; want 0 for rejected decision", status.VerifiedOutcomeCount)
	}
	if status.PatrolControlCompletedLoopCount != 1 ||
		status.PatrolControlResolvedLoopCount != 0 ||
		status.ProActivationCompletedLoopCount != 1 ||
		status.ProActivationResolvedLoopCount != 0 {
		t.Fatalf("Patrol control terminal counts wrong for rejected action: %+v", status)
	}
	if status.PatrolControlValueState != "governed_decision_recorded" {
		t.Fatalf("PatrolControlValueState = %q; want governed_decision_recorded", status.PatrolControlValueState)
	}
	if status.ProActivationValueProofState != "governed_decision_recorded" {
		t.Fatalf("ProActivationValueProofState = %q; want governed_decision_recorded", status.ProActivationValueProofState)
	}
	if status.NextAction != "complete" {
		t.Fatalf("NextAction = %q; want complete; status=%+v", status.NextAction, status)
	}
	steps := agentOperationsLoopStepsByID(status.Steps)
	if got := steps["governance"].Status; got != AgentOperationsLoopStepComplete {
		t.Fatalf("governance status = %q; want complete; steps=%+v", got, status.Steps)
	}
	if got := steps["governance"].Count; got != 1 {
		t.Fatalf("governance step count = %d; want rejected decision count", got)
	}
	if got := steps["verification"].Status; got != AgentOperationsLoopStepComplete {
		t.Fatalf("verification status = %q; want complete for rejected terminal decision; steps=%+v", got, status.Steps)
	}
	if got := steps["verification"].Count; got != 1 {
		t.Fatalf("verification step count = %d; want terminal rejection count", got)
	}
	if status.ProgressLabel != "Patrol recorded a rejected change decision. Nothing was changed; approve a safer fix before marking the issue resolved." {
		t.Fatalf("ProgressLabel = %q; want Patrol control governed rejection summary", status.ProgressLabel)
	}
}

func TestHandleAgentOperationsLoopStatus_ApprovedDecisionStillNeedsVerifiedOutcome(t *testing.T) {
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
	})

	store, err := h.resources.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	now := time.Now().UTC()
	h.SetAIUsageProvider(staticAIUsageProvider{
		history: &config.AIUsageHistoryData{
			Events: []config.AIUsageEventRecord{{
				Timestamp:    now.Add(-10 * time.Minute),
				UseCase:      "chat",
				ContextScope: "patrol_finding",
			}},
		},
	})
	record := unified.ActionAuditRecord{
		ID:        "act-approved",
		CreatedAt: now.Add(-10 * time.Minute),
		UpdatedAt: now.Add(-10 * time.Minute),
		State:     unified.ActionStatePending,
		Request: unified.ActionRequest{
			RequestID:      "req-approved",
			ResourceID:     "vm:101",
			CapabilityName: "restart_service",
			Params:         map[string]any{"command": "systemctl restart nginx"},
			RequestedBy:    "pulse_patrol",
		},
		Plan: unified.ActionPlan{
			ActionID:         "act-approved",
			RequestID:        "req-approved",
			PlannedAt:        now.Add(-10 * time.Minute),
			ExpiresAt:        now.Add(time.Hour),
			RequiresApproval: true,
		},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}
	approved, event, err := unified.ApplyActionDecision(record, unified.ActionApprovalRecord{
		Actor:   "operator@example.com",
		Method:  unified.MethodUI,
		Outcome: unified.OutcomeApproved,
	}, now)
	if err != nil {
		t.Fatalf("ApplyActionDecision: %v", err)
	}
	if err := store.RecordActionDecision(approved, event); err != nil {
		t.Fatalf("RecordActionDecision: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/operations-loop/status", nil)
	h.HandleOperationsLoopStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var status AgentOperationsLoopStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertPatrolControlCompatibilityAliases(t, status)
	if status.GovernedActionCount != 1 || status.ApprovedDecisionCount != 1 || status.RejectedDecisionCount != 0 {
		t.Fatalf("decision counts wrong for approved action: %+v", status)
	}
	if status.VerifiedOutcomeCount != 0 {
		t.Fatalf("VerifiedOutcomeCount = %d; want 0 before verification", status.VerifiedOutcomeCount)
	}
	if status.NextAction != "review_findings" {
		t.Fatalf("NextAction = %q; want review_findings; status=%+v", status.NextAction, status)
	}
	steps := agentOperationsLoopStepsByID(status.Steps)
	if got := steps["governance"].Status; got != AgentOperationsLoopStepComplete {
		t.Fatalf("governance status = %q; want complete; steps=%+v", got, status.Steps)
	}
	if got := steps["governance"].Count; got != 1 {
		t.Fatalf("governance step count = %d; want approved decision count", got)
	}
	if got := steps["verification"].Status; got != AgentOperationsLoopStepCurrent {
		t.Fatalf("verification status = %q; want current; steps=%+v", got, status.Steps)
	}
	if got := steps["verification"].Count; got != 0 {
		t.Fatalf("verification step count = %d; want zero before verified outcome", got)
	}
}

func TestHandleAgentOperationsLoopStatus_ReviewApprovalsWhenApprovalPending(t *testing.T) {
	h, findings, approvals := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
	})
	findings.byResource["vm:101"] = []AgentResourceFindingSnapshot{{ID: "finding-cpu", Severity: "warning"}}
	approvals.byResource["vm:101"] = []AgentResourceApprovalSummary{{ID: "approval-1"}}
	now := time.Now().UTC()
	h.SetAIUsageProvider(staticAIUsageProvider{
		history: &config.AIUsageHistoryData{
			Events: []config.AIUsageEventRecord{{
				Timestamp:    now.Add(-10 * time.Minute),
				UseCase:      "chat",
				ContextScope: "patrol_finding",
			}},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/operations-loop/status", nil)
	h.HandleOperationsLoopStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var status AgentOperationsLoopStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertPatrolControlCompatibilityAliases(t, status)
	if status.NextAction != "review_approvals" {
		t.Fatalf("NextAction = %q; want review_approvals", status.NextAction)
	}
	steps := agentOperationsLoopStepsByID(status.Steps)
	if got := steps["governance"].Status; got != AgentOperationsLoopStepCurrent {
		t.Fatalf("governance status = %q; want current; steps=%+v", got, status.Steps)
	}
	if got := steps["governance"].Count; got != 1 {
		t.Fatalf("governance step count = %d; want pending approval count", got)
	}
	if got := steps["verification"].Status; got != AgentOperationsLoopStepPending {
		t.Fatalf("verification status = %q; want pending; steps=%+v", got, status.Steps)
	}
}

func TestHandleAgentOperationsLoopStatus_RejectsNonGet(t *testing.T) {
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/agent/operations-loop/status", nil)
	h.HandleOperationsLoopStatus(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d, want 405", rec.Code)
	}
}

func agentOperationsLoopStepsByID(steps []AgentOperationsLoopStep) map[string]AgentOperationsLoopStep {
	byID := map[string]AgentOperationsLoopStep{}
	for _, step := range steps {
		byID[step.ID] = step
	}
	return byID
}

func assertPatrolControlCompatibilityAliases(t *testing.T, status AgentOperationsLoopStatus) {
	t.Helper()
	if status.PatrolAutonomyLoopStarterCount != status.PatrolControlLoopStarterCount {
		t.Fatalf("legacy Patrol autonomy starter alias drifted from Patrol control: control=%d legacy=%d",
			status.PatrolControlLoopStarterCount, status.PatrolAutonomyLoopStarterCount)
	}
	if status.PatrolAutonomyCompletedLoopCount != status.PatrolControlCompletedLoopCount ||
		status.PatrolAutonomyResolvedLoopCount != status.PatrolControlResolvedLoopCount ||
		status.PatrolAutonomyValueState != status.PatrolControlValueState {
		t.Fatalf("legacy Patrol autonomy aliases drifted from Patrol control: control=%d/%d/%q legacy=%d/%d/%q",
			status.PatrolControlCompletedLoopCount,
			status.PatrolControlResolvedLoopCount,
			status.PatrolControlValueState,
			status.PatrolAutonomyCompletedLoopCount,
			status.PatrolAutonomyResolvedLoopCount,
			status.PatrolAutonomyValueState,
		)
	}
	if status.ProActivationCompletedLoopCount != status.PatrolControlCompletedLoopCount ||
		status.ProActivationResolvedLoopCount != status.PatrolControlResolvedLoopCount ||
		status.ProActivationValueProofState != status.PatrolControlValueState {
		t.Fatalf("legacy Pro activation aliases drifted from Patrol control: control=%d/%d/%q legacy=%d/%d/%q",
			status.PatrolControlCompletedLoopCount,
			status.PatrolControlResolvedLoopCount,
			status.PatrolControlValueState,
			status.ProActivationCompletedLoopCount,
			status.ProActivationResolvedLoopCount,
			status.ProActivationValueProofState,
		)
	}
}

// pendingApprovalsForResourceFromStore is the named seam the
// router-side AgentApprovalsProvider closure delegates to. The
// next four tests pin the substrate's tenant- and
// resource-isolation property: every approval the bundle returns
// must match BOTH the requested org AND the requested resource.
// Cross-org or cross-resource leaks at this seam would let an
// agent with one org's token see approvals targeting another org's
// infrastructure, which is the multi-tenant property the rest of
// Pulse depends on.

// newApprovalsTestStore builds an in-memory approval store seeded
// with the given approvals. Used as the fixture for each isolation
// test below.
func newApprovalsTestStore(t *testing.T, approvals ...*approval.ApprovalRequest) *approval.Store {
	t.Helper()
	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            t.TempDir(),
		DefaultTimeout:     5 * time.Minute,
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("approval.NewStore: %v", err)
	}
	for _, req := range approvals {
		if err := store.CreateApproval(req); err != nil {
			t.Fatalf("CreateApproval(%s): %v", req.ID, err)
		}
	}
	return store
}

func TestPendingApprovalsForResource_FiltersByOrg(t *testing.T) {
	// Two orgs each have a pending approval against the same
	// canonical resource id. The filter must return only the org
	// that matches.
	orgA := &approval.ApprovalRequest{
		ID: "appr-org-a", OrgID: "org-a",
		Command: "systemctl restart nginx", TargetType: "agent", TargetID: "host-1", TargetName: "host-1",
	}
	orgB := &approval.ApprovalRequest{
		ID: "appr-org-b", OrgID: "org-b",
		Command: "systemctl restart nginx", TargetType: "agent", TargetID: "host-1", TargetName: "host-1",
	}
	store := newApprovalsTestStore(t, orgA, orgB)

	gotA := pendingApprovalsForResourceFromStore(store, "agent:host-1", "org-a")
	if len(gotA) != 1 || gotA[0].ID != "appr-org-a" {
		t.Errorf("org-a query must return only org-a's approval; got %+v", gotA)
	}
	gotB := pendingApprovalsForResourceFromStore(store, "agent:host-1", "org-b")
	if len(gotB) != 1 || gotB[0].ID != "appr-org-b" {
		t.Errorf("org-b query must return only org-b's approval; got %+v", gotB)
	}
}

func TestPendingApprovalsForResource_FiltersByResource(t *testing.T) {
	// Same org, two approvals against different resources. The
	// filter must return only the one matching the requested
	// canonical resource id, never both.
	a := &approval.ApprovalRequest{
		ID: "appr-vm-101", OrgID: "org-a",
		Command: "systemctl restart nginx", TargetType: "vm", TargetID: "101", TargetName: "vm-101",
	}
	b := &approval.ApprovalRequest{
		ID: "appr-vm-202", OrgID: "org-a",
		Command: "systemctl restart nginx", TargetType: "vm", TargetID: "202", TargetName: "vm-202",
	}
	store := newApprovalsTestStore(t, a, b)

	got := pendingApprovalsForResourceFromStore(store, "vm:101", "org-a")
	if len(got) != 1 || got[0].ID != "appr-vm-101" {
		t.Errorf("vm:101 query must return only that resource's approval; got %+v", got)
	}
}

func TestPendingApprovalsForResource_LegacyEmptyOrgIsDefaultOnly(t *testing.T) {
	// Approvals without OrgID set are treated as default-org per
	// approval.BelongsToOrg. Pin that legacy approvals do not leak
	// into a non-default org's bundle, and that the default-org
	// query sees them.
	legacy := &approval.ApprovalRequest{
		ID:      "appr-legacy",
		Command: "systemctl restart nginx", TargetType: "agent", TargetID: "host-1", TargetName: "host-1",
		// OrgID intentionally empty.
	}
	store := newApprovalsTestStore(t, legacy)

	gotDefault := pendingApprovalsForResourceFromStore(store, "agent:host-1", "default")
	if len(gotDefault) != 1 {
		t.Errorf("default org must see legacy approvals; got %d", len(gotDefault))
	}
	gotOther := pendingApprovalsForResourceFromStore(store, "agent:host-1", "org-other")
	if len(gotOther) != 0 {
		t.Errorf("non-default org must NOT see legacy approvals; got %+v", gotOther)
	}
}

func TestPendingApprovalsForResource_EmptyInputsReturnNil(t *testing.T) {
	// Nil store, empty resource id, and an empty store all return
	// nil. The bundle handler normalizes nil to []
	// AgentResourceApprovalSummary{} so the wire shape stays an
	// array; this function only owns the "no data" case.
	if got := pendingApprovalsForResourceFromStore(nil, "vm:101", "org-a"); got != nil {
		t.Errorf("nil store must return nil; got %+v", got)
	}
	var typedNil *approval.Store
	if got := pendingApprovalsForResourceFromStore(typedNil, "vm:101", "org-a"); got != nil {
		t.Errorf("typed nil store must return nil; got %+v", got)
	}
	store := newApprovalsTestStore(t)
	if got := pendingApprovalsForResourceFromStore(store, "", "org-a"); got != nil {
		t.Errorf("empty resource id must return nil; got %+v", got)
	}
	if got := pendingApprovalsForResourceFromStore(store, "vm:none", "org-a"); got != nil {
		t.Errorf("empty store must return nil; got %+v", got)
	}
}

func TestPendingApprovalCountsByResourceFromStore_EmptyInputsReturnNil(t *testing.T) {
	if got := pendingApprovalCountsByResourceFromStore(nil, "org-a"); got != nil {
		t.Errorf("nil store must return nil; got %+v", got)
	}
	var typedNil *approval.Store
	if got := pendingApprovalCountsByResourceFromStore(typedNil, "org-a"); got != nil {
		t.Errorf("typed nil store must return nil; got %+v", got)
	}
	store := newApprovalsTestStore(t)
	if got := pendingApprovalCountsByResourceFromStore(store, "org-a"); got != nil {
		t.Errorf("empty store must return nil; got %+v", got)
	}
}

func TestPendingApprovalCountsByResourceFromStore_GroupsByOrgAndResource(t *testing.T) {
	store := newApprovalsTestStore(t,
		&approval.ApprovalRequest{
			ID: "appr-org-a-vm-101-a", OrgID: "org-a",
			Command: "systemctl restart nginx", TargetType: "vm", TargetID: "101", TargetName: "vm-101",
		},
		&approval.ApprovalRequest{
			ID: "appr-org-a-vm-101-b", OrgID: "org-a",
			Command: "systemctl restart nginx", TargetType: "vm", TargetID: "101", TargetName: "vm-101",
		},
		&approval.ApprovalRequest{
			ID: "appr-org-a-vm-202", OrgID: "org-a",
			Command: "systemctl restart nginx", TargetType: "vm", TargetID: "202", TargetName: "vm-202",
		},
		&approval.ApprovalRequest{
			ID: "appr-org-b-vm-101", OrgID: "org-b",
			Command: "systemctl restart nginx", TargetType: "vm", TargetID: "101", TargetName: "vm-101",
		},
	)

	got := pendingApprovalCountsByResourceFromStore(store, "org-a")
	if got["vm:101"] != 2 {
		t.Errorf("vm:101 count = %d; want 2", got["vm:101"])
	}
	if got["vm:202"] != 1 {
		t.Errorf("vm:202 count = %d; want 1", got["vm:202"])
	}
	if len(got) != 2 {
		t.Errorf("counts len = %d; want 2; got %+v", len(got), got)
	}

	gotB := pendingApprovalCountsByResourceFromStore(store, "org-b")
	if gotB["vm:101"] != 1 || len(gotB) != 1 {
		t.Errorf("org-b counts = %+v; want only vm:101=1", gotB)
	}
}

func TestExtractAgentResourceContextID(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/api/agent/resource-context/vm:101", "vm:101"},
		{"/api/agent/resource-context/vm:101/", "vm:101"},
		{"/api/agent/resource-context/instance:node:200", "instance:node:200"},
		{"/api/agent/resource-context/", ""},
	}
	for _, tc := range cases {
		if got := extractAgentResourceContextID(tc.path); got != tc.want {
			t.Errorf("extractAgentResourceContextID(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestExtractAgentResourceCapabilitiesID(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/api/agent/resource-capabilities/vm:101", "vm:101"},
		{"/api/agent/resource-capabilities/vm:101/", "vm:101"},
		{"/api/agent/resource-capabilities/instance:node:200", "instance:node:200"},
		{"/api/agent/resource-capabilities/", ""},
	}
	for _, tc := range cases {
		if got := extractAgentResourceCapabilitiesID(tc.path); got != tc.want {
			t.Errorf("extractAgentResourceCapabilitiesID(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}
