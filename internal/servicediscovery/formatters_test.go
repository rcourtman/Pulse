package servicediscovery

import (
	"strings"
	"testing"
	"time"
)

func TestFormattersAndTables(t *testing.T) {
	if FormatForAIContext(nil) != "" {
		t.Fatalf("expected empty context for nil discoveries")
	}

	discovery := &ResourceDiscovery{
		ID:             MakeResourceID(ResourceTypeDocker, "host1", "app"),
		ResourceType:   ResourceTypeDocker,
		ResourceID:     "app",
		TargetID:       "host1",
		AgentID:        "agent-1",
		Hostname:       "host1",
		ServiceType:    "app",
		ServiceName:    "App Service",
		ServiceVersion: "1.0",
		Category:       CategoryWebServer,
		CLIAccess:      "docker exec app ...",
		ConfigPaths:    []string{"/etc/app/config.yml"},
		DataPaths:      []string{"/var/lib/app"},
		Ports:          []PortInfo{{Port: 80, Protocol: "tcp"}},
		SuggestedURL:   "http://host1",
		UserNotes:      "keepalive enabled",
		Facts: []DiscoveryFact{
			{Category: FactCategoryHardware, Key: "gpu", Value: "nvidia", Confidence: 0.9},
			{Category: FactCategoryService, Key: "worker", Value: "enabled", Confidence: 0.9},
		},
	}

	ctx := FormatForAIContext([]*ResourceDiscovery{discovery})
	if !strings.Contains(ctx, "Infrastructure Discovery") || !strings.Contains(ctx, "App Service") {
		t.Fatalf("unexpected context: %s", ctx)
	}
	if !strings.Contains(ctx, "docker exec") || !strings.Contains(ctx, "User Notes") {
		t.Fatalf("missing expected fields in context")
	}
	if !strings.Contains(ctx, "Suggested Web URL") || !strings.Contains(ctx, "http://host1") {
		t.Fatalf("missing suggested URL in context")
	}

	if FormatSingleForAIContext(nil) != "" {
		t.Fatalf("expected empty string for nil discovery")
	}
	if !strings.Contains(FormatSingleForAIContext(discovery), "App Service") {
		t.Fatalf("expected single discovery output")
	}

	remediation := FormatForRemediation(discovery)
	if !strings.Contains(remediation, "How to Execute Commands") || !strings.Contains(remediation, "Hardware") {
		t.Fatalf("unexpected remediation output: %s", remediation)
	}
	if FormatForRemediation(nil) != "" {
		t.Fatalf("expected empty remediation output for nil")
	}

	example := GetCLIExample(discovery, "ls /")
	if !strings.Contains(example, "ls /") {
		t.Fatalf("unexpected cli example: %s", example)
	}
	if GetCLIExample(&ResourceDiscovery{}, "ls /") != "" {
		t.Fatalf("expected empty example when cli access missing")
	}

	table := FormatFactsTable([]DiscoveryFact{
		{Category: FactCategoryVersion, Key: "app", Value: strings.Repeat("x", 60)},
	})
	if !strings.Contains(table, "...") {
		t.Fatalf("expected truncated table value: %s", table)
	}
	if FormatFactsTable(nil) != "" {
		t.Fatalf("expected empty facts table for nil")
	}

	jsonMap := ToJSON(discovery)
	if jsonMap["service_name"] != "App Service" || jsonMap["resource_id"] != "app" {
		t.Fatalf("unexpected json map: %#v", jsonMap)
	}
	if jsonMap["target_id"] != "host1" {
		t.Fatalf("expected target_id=host1, got %#v", jsonMap["target_id"])
	}
	if jsonMap["agent_id"] != "agent-1" {
		t.Fatalf("expected agent_id=agent-1, got %#v", jsonMap["agent_id"])
	}
	if _, hasLegacyHostID := jsonMap["host_id"]; hasLegacyHostID {
		t.Fatalf("did not expect legacy host_id key in ToJSON output: %#v", jsonMap)
	}
	if ToJSON(nil) != nil {
		t.Fatalf("expected nil json map for nil discovery")
	}
}

func TestFormatDiscoverySummaryAndAge(t *testing.T) {
	now := time.Now()
	if FormatDiscoverySummary(nil) == "" {
		t.Fatalf("expected summary text for empty list")
	}
	if FormatDiscoveryAge(nil) != "unknown" {
		t.Fatalf("expected unknown age for nil")
	}
	if FormatDiscoveryAge(&ResourceDiscovery{}) != "unknown" {
		t.Fatalf("expected unknown age for zero timestamp")
	}
	discoveries := []*ResourceDiscovery{
		{
			ID:           MakeResourceID(ResourceTypeVM, "node1", "101"),
			ResourceType: ResourceTypeVM,
			ResourceID:   "101",
			TargetID:     "node1",
			ServiceName:  "VM One",
			Confidence:   0.95,
			UpdatedAt:    now.Add(-2 * time.Hour),
		},
		{
			ID:           MakeResourceID(ResourceTypeDocker, "host1", "app"),
			ResourceType: ResourceTypeDocker,
			ResourceID:   "app",
			TargetID:     "host1",
			ServiceName:  "App",
			Confidence:   0.75,
			UpdatedAt:    now.Add(-2 * 24 * time.Hour),
		},
	}

	summary := FormatDiscoverySummary(discoveries)
	if !strings.Contains(summary, "[high confidence]") || !strings.Contains(summary, "[medium confidence]") {
		t.Fatalf("unexpected summary: %s", summary)
	}

	tests := []struct {
		name     string
		updated  time.Time
		expected string
	}{
		{name: "just-now", updated: now.Add(-30 * time.Second), expected: "just now"},
		{name: "one-minute", updated: now.Add(-1 * time.Minute), expected: "1 minute ago"},
		{name: "minutes", updated: now.Add(-10 * time.Minute), expected: "10 minutes ago"},
		{name: "one-hour", updated: now.Add(-1 * time.Hour), expected: "1 hour ago"},
		{name: "hours", updated: now.Add(-2 * time.Hour), expected: "2 hours ago"},
		{name: "one-day", updated: now.Add(-24 * time.Hour), expected: "1 day ago"},
		{name: "days", updated: now.Add(-3 * 24 * time.Hour), expected: "3 days ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDiscoveryAge(&ResourceDiscovery{UpdatedAt: tt.updated})
			if got != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestBuildResourceContextForPatrol(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.crypto = nil

	discovery := &ResourceDiscovery{
		ID:           MakeResourceID(ResourceTypeDocker, "host1", "app"),
		ResourceType: ResourceTypeDocker,
		ResourceID:   "app",
		TargetID:     "host1",
		ServiceName:  "App Service",
	}
	if err := store.Save(discovery); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	ctx := BuildResourceContextForPatrol(store, []string{discovery.ID})
	if !strings.Contains(ctx, "App Service") {
		t.Fatalf("unexpected patrol context: %s", ctx)
	}

	if BuildResourceContextForPatrol(nil, []string{discovery.ID}) != "" {
		t.Fatalf("expected empty context for nil store")
	}
	if BuildResourceContextForPatrol(store, nil) != "" {
		t.Fatalf("expected empty context for empty ids")
	}
	if BuildResourceContextForPatrol(store, []string{"missing"}) != "" {
		t.Fatalf("expected empty context for missing discoveries")
	}
}

func TestFormatScopeHint(t *testing.T) {
	discovery := &ResourceDiscovery{
		ID:             MakeResourceID(ResourceTypeDocker, "host1", "app"),
		ResourceType:   ResourceTypeDocker,
		ResourceID:     "app",
		TargetID:       "host1",
		Hostname:       "host1",
		ServiceType:    "app",
		ServiceName:    "App Service",
		ServiceVersion: "1.2.3",
		CLIAccess:      "docker exec app -- ...",
		Ports:          []PortInfo{{Port: 80, Protocol: "tcp"}, {Port: 443, Protocol: "tcp"}},
	}

	hint := FormatScopeHint([]*ResourceDiscovery{discovery})
	if !strings.Contains(hint, "Discovery:") || !strings.Contains(hint, "App Service") {
		t.Fatalf("unexpected scope hint: %s", hint)
	}
	if FormatScopeHint(nil) != "" {
		t.Fatalf("expected empty hint for nil")
	}
}

func TestFormatForRemediationSurfacesControlAndMounts(t *testing.T) {
	d := &ResourceDiscovery{
		ID:           MakeResourceID(ResourceTypeDocker, "nuc", "homeassistant"),
		ResourceType: ResourceTypeDocker,
		Hostname:     "nuc",
		ServiceType:  "home-assistant",
		ServiceName:  "Home Assistant",
		CLIAccess:    "docker exec homeassistant bash",
		ConfigPaths:  []string{"/config/automations.yaml"},
		DockerMounts: []DockerBindMount{
			{ContainerName: "homeassistant", Source: "/opt/ha/config", Destination: "/config", Type: "bind"},
		},
		Facts: []DiscoveryFact{
			{Category: FactCategoryService, Key: "restart", Value: "docker restart homeassistant", Confidence: 0.9},
		},
	}

	out := FormatForRemediation(d)
	// Remediation must tell you how to restart the service and where to edit its
	// files on the host — the two core fix actions, previously both missing.
	for _, want := range []string{
		"docker restart homeassistant", // service-control fact
		"/opt/ha/config",               // host bind-mount source
		"/config/automations.yaml",     // config file to edit
	} {
		if !strings.Contains(out, want) {
			t.Errorf("remediation context missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestFilterImportantFactsLimit(t *testing.T) {
	var facts []DiscoveryFact
	for i := 0; i < 10; i++ {
		facts = append(facts, DiscoveryFact{
			Category:   FactCategoryVersion,
			Key:        "k",
			Value:      "v",
			Confidence: 0.9,
		})
	}
	// A trailing, most-actionable service-control fact among many informational
	// ones must survive the cap (and sort to the front) — it is how the
	// assistant restarts the workload.
	facts = append(facts, DiscoveryFact{
		Category:   FactCategoryService,
		Key:        "systemd_unit",
		Value:      "app.service",
		Confidence: 0.9,
	})

	important := filterImportantFacts(facts)
	if len(important) != 8 {
		t.Fatalf("expected cap of 8 facts, got %d", len(important))
	}
	if important[0].Category != FactCategoryService {
		t.Fatalf("expected the service-control fact surfaced first, got %s", important[0].Category)
	}
	found := false
	for _, f := range important {
		if f.Key == "systemd_unit" {
			found = true
		}
	}
	if !found {
		t.Fatalf("service-control fact was dropped by the cap")
	}
}
