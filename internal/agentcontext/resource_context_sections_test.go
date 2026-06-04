package agentcontext

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestBuildResourceContextSectionsAppliesRedactionTrustAndFreshness(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	observedAt := now.Add(-2 * time.Minute)
	operatorSetAt := now.Add(-1 * time.Hour)
	dockerCheckedAt := now.Add(-3 * time.Minute)
	parentID := "agent:prod-node"

	resource := sensitiveTestResource(now, observedAt)
	resource.ParentID = &parentID
	resource.ParentName = "prod-node"
	resource.DiscoveryReadiness = &unified.ResourceDiscoveryReadiness{
		State:             unified.ResourceDiscoveryReadinessFresh,
		Source:            "service-discovery",
		ObservedAt:        &observedAt,
		AgeSeconds:        60,
		StaleAfterSeconds: int64((30 * 24 * time.Hour).Seconds()),
		FactCount:         4,
		ServiceName:       "finance-db.internal",
		ServiceCategory:   "database",
		Confidence:        0.84,
	}
	resource.Proxmox = &unified.ProxmoxData{
		NodeName:        "prod-node",
		ClusterName:     "prod-west",
		VMID:            101,
		ContainerType:   "lxc",
		OSName:          "Debian",
		OSVersion:       "12",
		HasDocker:       true,
		DockerCheckedAt: &dockerCheckedAt,
	}
	resource.Docker = &unified.DockerData{
		Image:          "registry.internal/finance-db:latest",
		ContainerState: "running",
		Ports: []unified.DockerPortMeta{
			{PublicPort: 8123, PrivatePort: 8123, Protocol: "tcp"},
		},
		Networks: []unified.DockerNetworkMeta{
			{Name: "backend", IPv4: "10.99.0.2"},
		},
		Mounts: []unified.DockerMountMeta{
			{Type: "bind", Source: "/var/lib/homeassistant", Destination: "/config", RW: true},
		},
		Labels: map[string]string{"com.example.secret": "label-secret"},
	}
	resource.Storage = &unified.StorageMeta{
		Type:        "zfs",
		Platform:    "proxmox",
		Path:        "/mnt/pve/finance-db",
		RiskSummary: "finance-db.internal uses /mnt/pve/finance-db",
	}
	resource.Capabilities = []unified.ResourceCapability{
		{
			Name:                 "restart_container",
			MinimumApprovalLevel: unified.ApprovalAdmin,
			Platform:             "docker",
			Params: []unified.CapabilityParam{
				{Name: "api_token", Type: "string", IsSensitive: true, DefaultValue: "super-secret-token"},
				{Name: "password", Type: "string", IsSensitive: true, DefaultValue: "another-secret"},
			},
		},
	}

	sections := BuildResourceContextSections(resource, nil, BuildOptions{
		GeneratedAt:          now,
		OperatorState:        &OperatorState{IntentionallyOffline: true, NeverAutoRemediate: true, Criticality: "high", NotePresent: true, SetAt: operatorSetAt},
		ActiveFindingCount:   2,
		PendingApprovalCount: 1,
		RecentActionCount:    3,
	})

	identity := requireSection(t, sections, "identity")
	if identity.Source != agentContextSourceUnifiedResource || identity.TrustTier != agentContextTrustPulseAuthored {
		t.Fatalf("identity provenance = source %q trust %q", identity.Source, identity.TrustTier)
	}
	if !identity.GeneratedAt.Equal(now) {
		t.Fatalf("identity generatedAt = %v, want %v", identity.GeneratedAt, now)
	}
	if identity.ObservedAt == nil || !identity.ObservedAt.Equal(observedAt) {
		t.Fatalf("identity observedAt = %v, want %v", identity.ObservedAt, observedAt)
	}
	displayName := requireFact(t, identity, "Display name")
	if displayName.Value != resource.AISafeSummary {
		t.Fatalf("display name = %q, want AI-safe summary %q", displayName.Value, resource.AISafeSummary)
	}
	canonicalID := requireFact(t, identity, "Canonical ID")
	if canonicalID.Value != unified.ResourcePolicyRedactedLabel || !canonicalID.Redacted {
		t.Fatalf("canonical ID fact = %#v, want redacted by policy", canonicalID)
	}

	runtime := requireSection(t, sections, "runtime")
	for _, label := range []string{"Discovery agent", "Discovery resource", "Discovery hostname", "Proxmox node", "Proxmox VMID", "Image", "Storage path"} {
		fact := requireFact(t, runtime, label)
		if fact.Value != unified.ResourcePolicyRedactedLabel || !fact.Redacted {
			t.Fatalf("%s fact = %#v, want redacted by policy", label, fact)
		}
	}
	readiness := requireFact(t, runtime, "Discovery readiness")
	if readiness.Source != agentContextSourceServiceDiscovery || readiness.TrustTier != agentContextTrustPulseAuthored {
		t.Fatalf("readiness provenance = source %q trust %q", readiness.Source, readiness.TrustTier)
	}
	if !strings.Contains(readiness.Value, "fresh") || !strings.Contains(readiness.Value, "staleAfter=") {
		t.Fatalf("readiness value = %q", readiness.Value)
	}
	if service := requireFact(t, runtime, "Discovered service"); service.Value != unified.ResourcePolicyRedactedLabel || !service.Redacted {
		t.Fatalf("discovered service fact = %#v, want redacted by policy", service)
	}
	if factCount := requireFact(t, runtime, "Discovery facts"); factCount.Value != "4" || factCount.TrustTier != agentContextTrustDiscovered {
		t.Fatalf("discovery facts = %#v, want discovered count", factCount)
	}
	if risk := requireFact(t, runtime, "Risk summary"); !strings.Contains(risk.Value, unified.ResourcePolicyRedactedLabel) {
		t.Fatalf("risk summary = %q, want redacted label", risk.Value)
	}
	if mounts := requireFact(t, runtime, "Mounts"); mounts.Value != "1" {
		t.Fatalf("mounts fact = %q, want count only", mounts.Value)
	}

	safety := requireSection(t, sections, "safety")
	operatorState := requireFact(t, safety, "Operator state")
	if operatorState.Source != agentContextSourceOperatorState || operatorState.TrustTier != agentContextTrustUserTaught {
		t.Fatalf("operator state provenance = source %q trust %q", operatorState.Source, operatorState.TrustTier)
	}
	if operatorState.ObservedAt == nil || !operatorState.ObservedAt.Equal(operatorSetAt) {
		t.Fatalf("operator state observedAt = %v, want %v", operatorState.ObservedAt, operatorSetAt)
	}
	for _, want := range []string{"intentionally offline", "never auto-remediate", "criticality=high", "note present"} {
		if !strings.Contains(operatorState.Value, want) {
			t.Fatalf("operator state value = %q, want %q", operatorState.Value, want)
		}
	}
	capability := requireFact(t, safety, "Capability 1")
	if !strings.Contains(capability.Value, "2 sensitive params hidden") {
		t.Fatalf("capability fact = %q, want sensitive param count", capability.Value)
	}
	if capability.Source != agentContextSourceUnifiedResource || capability.TrustTier != agentContextTrustPulseAuthored {
		t.Fatalf("capability provenance = source %q trust %q", capability.Source, capability.TrustTier)
	}

	policy := requireSection(t, sections, "policy")
	if len(policy.Redactions) == 0 {
		t.Fatal("policy section should expose redaction reasons")
	}
	redactionReason := policy.Redactions[0].Reason
	for _, want := range []string{"Hostname", "Platform ID", "Alias", "Path"} {
		if !strings.Contains(redactionReason, want) {
			t.Fatalf("policy redaction reason = %q, want %q", redactionReason, want)
		}
	}

	allText := sectionsAndModelText(t, resource, sections)
	assertContainsAll(t, allText,
		"[Resource Context Pack]",
		"trust=user-taught",
		"redacted=true",
		"2 sensitive params hidden",
		"Context Pack Boundary: Resource context facts are read-only",
		"not approval or execution authority",
		"must not be expanded into raw provider commands, config files, environment variables, paths, or secret-bearing metadata",
	)
	assertOmitsAll(t, allText,
		"finance-db",
		"finance-db.internal",
		"10.10.0.5",
		"prod-west:101",
		"prod-node",
		"registry.internal",
		"/mnt/pve/finance-db",
		"/var/lib/homeassistant",
		"/config",
		"label-secret",
		"api_token",
		"super-secret-token",
		"another-secret",
	)
}

func TestBuildResourceContextSectionsRedactsRecentChangesThroughStore(t *testing.T) {
	now := time.Date(2026, 6, 4, 13, 0, 0, 0, time.UTC)
	observedAt := now.Add(-10 * time.Minute)
	resource := sensitiveTestResource(now, observedAt)
	resource.Storage = &unified.StorageMeta{Path: "/mnt/pve/finance-db"}

	store, err := unified.NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatalf("NewSQLiteResourceStore: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close store: %v", err)
		}
	})
	if err := store.RecordChange(unified.ResourceChange{
		ID:            "change-sensitive-context",
		ResourceID:    unified.CanonicalResourceID(resource.ID),
		ObservedAt:    now.Add(-30 * time.Second),
		Kind:          unified.ChangeConfigUpdate,
		From:          "finance-db.internal",
		To:            "10.10.0.5",
		SourceType:    unified.SourcePulseDiff,
		SourceAdapter: unified.AdapterDocker,
		Confidence:    unified.ConfidenceHigh,
		Reason:        "updated path /mnt/pve/finance-db",
	}); err != nil {
		t.Fatalf("RecordChange: %v", err)
	}

	sections := BuildResourceContextSections(resource, store, BuildOptions{GeneratedAt: now})
	recentChanges := requireSection(t, sections, "recent_changes")
	if recentChanges.Source != agentContextSourceResourceStore || recentChanges.TrustTier != agentContextTrustRuntimeObserved {
		t.Fatalf("recent changes provenance = source %q trust %q", recentChanges.Source, recentChanges.TrustTier)
	}
	change := requireFact(t, recentChanges, "Change 1")
	if change.ObservedAt == nil || !change.ObservedAt.Equal(now.Add(-30*time.Second)) {
		t.Fatalf("change observedAt = %v, want recorded observation", change.ObservedAt)
	}
	assertContainsAll(t, change.Value,
		string(unified.ChangeConfigUpdate),
		unified.ResourcePolicyRedactedLabel+" -> "+unified.ResourcePolicyRedactedLabel,
		"reason=updated path "+unified.ResourcePolicyRedactedLabel,
		"source="+string(unified.SourcePulseDiff),
		"adapter="+string(unified.AdapterDocker),
		"confidence="+string(unified.ConfidenceHigh),
	)

	allText := sectionsAndModelText(t, resource, sections)
	assertOmitsAll(t, allText,
		"finance-db.internal",
		"10.10.0.5",
		"/mnt/pve/finance-db",
	)
}

func TestBuildResourceContextSectionsBoundsDiscoveredDetails(t *testing.T) {
	now := time.Date(2026, 6, 4, 14, 0, 0, 0, time.UTC)
	resource := unified.Resource{
		ID:        "app-container:homeassistant",
		Type:      unified.ResourceTypeAppContainer,
		Name:      "homeassistant",
		Status:    unified.StatusOnline,
		UpdatedAt: now.Add(-time.Minute),
		Docker: &unified.DockerData{
			Image: "ghcr.io/home-assistant/home-assistant:stable",
			Networks: []unified.DockerNetworkMeta{
				{Name: "iot", IPv4: "172.20.0.5"},
			},
			Mounts: []unified.DockerMountMeta{
				{Type: "bind", Source: "/var/lib/private", Destination: "/container/private", RW: true},
			},
			Labels: map[string]string{"secret.label": "label-secret"},
		},
		Relationships: []unified.ResourceRelationship{
			relationshipForLimitTest(1, now),
			relationshipForLimitTest(2, now),
			relationshipForLimitTest(3, now),
			relationshipForLimitTest(4, now),
			relationshipForLimitTest(5, now),
			relationshipForLimitTest(6, now),
		},
		Capabilities: []unified.ResourceCapability{
			capabilityForLimitTest(1),
			capabilityForLimitTest(2),
			capabilityForLimitTest(3),
			capabilityForLimitTest(4),
			capabilityForLimitTest(5),
			capabilityForLimitTest(6),
		},
	}

	sections := BuildResourceContextSections(resource, nil, BuildOptions{GeneratedAt: now})
	runtime := requireSection(t, sections, "runtime")
	if networks := requireFact(t, runtime, "Networks"); networks.Value != "1" {
		t.Fatalf("networks fact = %q, want count only", networks.Value)
	}
	if mounts := requireFact(t, runtime, "Mounts"); mounts.Value != "1" {
		t.Fatalf("mounts fact = %q, want count only", mounts.Value)
	}

	topology := requireSection(t, sections, "topology")
	if count := requireFact(t, topology, "Relationship count"); count.Value != "6" {
		t.Fatalf("relationship count = %q, want 6", count.Value)
	}
	if limit := requireFact(t, topology, "Relationship limit"); limit.Value != "1 additional relationships omitted from context pack" {
		t.Fatalf("relationship limit = %q", limit.Value)
	}
	if _, ok := findFact(topology, "Relationship 6"); ok {
		t.Fatal("relationship 6 should be omitted by context-pack limit")
	}

	safety := requireSection(t, sections, "safety")
	if count := requireFact(t, safety, "Governed capabilities"); count.Value != "6" {
		t.Fatalf("capability count = %q, want 6", count.Value)
	}
	if limit := requireFact(t, safety, "Capability limit"); limit.Value != "1 additional capabilities omitted from context pack" {
		t.Fatalf("capability limit = %q", limit.Value)
	}
	if _, ok := findFact(safety, "Capability 6"); ok {
		t.Fatal("capability 6 should be omitted by context-pack limit")
	}

	allText := sectionsAndModelText(t, resource, sections)
	assertContainsAll(t, allText, "metadata present")
	assertOmitsAll(t, allText,
		"secretPath",
		"/root/hidden",
		"172.20.0.5",
		"/var/lib/private",
		"/container/private",
		"label-secret",
	)
}

func sensitiveTestResource(now, observedAt time.Time) unified.Resource {
	return unified.Resource{
		ID:            "system-container:prod-west:101",
		Type:          unified.ResourceTypeSystemContainer,
		Technology:    "lxc",
		Name:          "finance-db",
		Status:        unified.StatusOnline,
		LastSeen:      now.Add(-5 * time.Minute),
		UpdatedAt:     observedAt,
		AISafeSummary: "system container resource; status online; local-only context",
		Policy: &unified.ResourcePolicy{
			Sensitivity: unified.ResourceSensitivityRestricted,
			Routing: unified.ResourceRoutingPolicy{
				Scope: unified.ResourceRoutingScopeLocalOnly,
				Redact: []unified.ResourceRedactionHint{
					unified.ResourceRedactionHostname,
					unified.ResourceRedactionIPAddress,
					unified.ResourceRedactionPlatformID,
					unified.ResourceRedactionAlias,
					unified.ResourceRedactionPath,
				},
			},
		},
		DiscoveryTarget: &unified.DiscoveryTarget{
			ResourceType: "system-container",
			AgentID:      "prod-node",
			ResourceID:   "prod-west:101",
			Hostname:     "finance-db.internal",
		},
		Canonical: &unified.CanonicalIdentity{
			DisplayName: "finance-db",
			Hostname:    "finance-db.internal",
			PlatformID:  "prod-west:101",
			PrimaryID:   "system-container:prod-west:101",
			Aliases:     []string{"finance-db-primary"},
		},
		Sources: []unified.DataSource{unified.SourceProxmox, unified.SourceDocker},
		SourceStatus: map[unified.DataSource]unified.SourceStatus{
			unified.SourceProxmox: {Status: "online", LastSeen: now.Add(-30 * time.Second)},
		},
		Identity: unified.ResourceIdentity{
			Hostnames:   []string{"finance-db.internal"},
			IPAddresses: []string{"10.10.0.5"},
			ClusterName: "prod-west",
		},
	}
}

func relationshipForLimitTest(index int, now time.Time) unified.ResourceRelationship {
	return unified.ResourceRelationship{
		SourceID:   "app-container:homeassistant",
		TargetID:   "agent:node-" + string(rune('0'+index)),
		Type:       unified.RelRunsOn,
		Confidence: 0.9,
		Active:     true,
		Discoverer: "docker_adapter",
		ObservedAt: now.Add(-time.Duration(index) * time.Minute),
		LastSeenAt: now.Add(-time.Duration(index) * time.Minute),
		Metadata: map[string]any{
			"secretPath": "/root/hidden",
		},
	}
}

func capabilityForLimitTest(index int) unified.ResourceCapability {
	return unified.ResourceCapability{
		Name:                 "capability_" + string(rune('0'+index)),
		MinimumApprovalLevel: unified.ApprovalAdmin,
		Platform:             "docker",
	}
}

func requireSection(t *testing.T, sections []Section, id string) Section {
	t.Helper()
	section, ok := findSection(sections, id)
	if !ok {
		t.Fatalf("section %q missing from %+v", id, sections)
	}
	return section
}

func findSection(sections []Section, id string) (Section, bool) {
	for _, section := range sections {
		if section.ID == id {
			return section, true
		}
	}
	return Section{}, false
}

func requireFact(t *testing.T, section Section, label string) Fact {
	t.Helper()
	fact, ok := findFact(section, label)
	if !ok {
		t.Fatalf("fact %q missing from section %+v", label, section)
	}
	return fact
}

func findFact(section Section, label string) (Fact, bool) {
	for _, fact := range section.Facts {
		if fact.Label == label {
			return fact, true
		}
	}
	return Fact{}, false
}

func sectionsAndModelText(t *testing.T, resource unified.Resource, sections []Section) string {
	t.Helper()
	encoded, err := json.MarshalIndent(sections, "", "  ")
	if err != nil {
		t.Fatalf("marshal sections: %v", err)
	}
	return string(encoded) + "\n" + FormatSectionsForModelContext(resource, sections)
}

func assertContainsAll(t *testing.T, haystack string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			t.Fatalf("expected text to contain %q\ntext:\n%s", needle, haystack)
		}
	}
}

func assertOmitsAll(t *testing.T, haystack string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if strings.Contains(haystack, needle) {
			t.Fatalf("expected text to omit %q\ntext:\n%s", needle, haystack)
		}
	}
}
