package aidiscovery

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
		HostID:         "host1",
		Hostname:       "host1",
		ServiceType:    "app",
		ServiceName:    "App Service",
		ServiceVersion: "1.0",
		Category:       CategoryWebServer,
		CLIAccess:      "docker exec app ...",
		ConfigPaths:    []string{"/etc/app/config.yml"},
		DataPaths:      []string{"/var/lib/app"},
		Ports:          []PortInfo{{Port: 80, Protocol: "tcp"}},
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
			HostID:       "node1",
			ServiceName:  "VM One",
			Confidence:   0.95,
			UpdatedAt:    now.Add(-2 * time.Hour),
		},
		{
			ID:           MakeResourceID(ResourceTypeDocker, "host1", "app"),
			ResourceType: ResourceTypeDocker,
			ResourceID:   "app",
			HostID:       "host1",
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
		HostID:       "host1",
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

func TestFilterImportantFactsLimit(t *testing.T) {
	var facts []DiscoveryFact
	for i := 0; i < 7; i++ {
		facts = append(facts, DiscoveryFact{
			Category:   FactCategoryVersion,
			Key:        "k",
			Value:      "v",
			Confidence: 0.9,
		})
	}

	important := filterImportantFacts(facts)
	if len(important) != 5 {
		t.Fatalf("expected 5 facts, got %d", len(important))
	}
}
