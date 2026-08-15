package chat

import (
	"reflect"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestRestrictInvestigationProviderToolsForResourceType(t *testing.T) {
	providerTools := []providers.Tool{
		{Name: agentcapabilities.PulseQueryToolName},
		{Name: agentcapabilities.PulseMetricsToolName},
		{Name: agentcapabilities.PulseStorageToolName},
		{Name: agentcapabilities.PulseDockerToolName},
		{Name: agentcapabilities.PulseKubernetesToolName},
		{Name: agentcapabilities.PulseReadToolName},
		{Name: agentcapabilities.PulseKnowledgeToolName},
		{Name: agentcapabilities.PatrolActionCapabilitiesToolName},
		{Name: agentcapabilities.PatrolProposeActionToolName},
	}
	filtered, err := restrictInvestigationProviderToolsForResourceType(providerTools, "app-container")
	if err != nil {
		t.Fatalf("restrict Docker investigation: %v", err)
	}
	var names []string
	for _, tool := range filtered {
		names = append(names, tool.Name)
	}
	want := []string{
		agentcapabilities.PulseQueryToolName,
		agentcapabilities.PulseDockerToolName,
		agentcapabilities.PulseReadToolName,
		agentcapabilities.PatrolActionCapabilitiesToolName,
		agentcapabilities.PatrolProposeActionToolName,
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("Docker investigation tools = %v, want %v", names, want)
	}

	unchanged, err := restrictInvestigationProviderToolsForResourceType(providerTools, "future-resource-kind")
	if err != nil {
		t.Fatalf("unknown resource type: %v", err)
	}
	if !reflect.DeepEqual(unchanged, providerTools) {
		t.Fatalf("unknown resource type changed governed profile: %v", unchanged)
	}
	unchanged[0].Name = "mutated"
	if providerTools[0].Name == "mutated" {
		t.Fatal("unknown-type fallback aliased the provider manifest")
	}
}

func TestRestrictInvestigationProviderToolsAllowsUnavailableOptionalDeepRead(t *testing.T) {
	providerTools := []providers.Tool{
		{Name: agentcapabilities.PulseQueryToolName},
		{Name: agentcapabilities.PulseMetricsToolName},
		{Name: agentcapabilities.PatrolActionCapabilitiesToolName},
		{Name: agentcapabilities.PatrolProposeActionToolName},
	}
	filtered, err := restrictInvestigationProviderToolsForResourceType(providerTools, "vm")
	if err != nil {
		t.Fatalf("restrict VM investigation without agent deep-read adapter: %v", err)
	}
	var names []string
	for _, tool := range filtered {
		names = append(names, tool.Name)
	}
	want := []string{
		agentcapabilities.PulseQueryToolName,
		agentcapabilities.PulseMetricsToolName,
		agentcapabilities.PatrolActionCapabilitiesToolName,
		agentcapabilities.PatrolProposeActionToolName,
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("VM investigation tools = %v, want %v", names, want)
	}
}

func TestRestrictInvestigationProviderToolsRequiresEvidenceAndProposalBoundary(t *testing.T) {
	tests := []struct {
		name  string
		tools []providers.Tool
		want  string
	}{
		{
			name: "missing proposal sink",
			tools: []providers.Tool{
				{Name: agentcapabilities.PulseQueryToolName},
				{Name: agentcapabilities.PatrolActionCapabilitiesToolName},
			},
			want: agentcapabilities.PatrolProposeActionToolName,
		},
		{
			name: "missing evidence",
			tools: []providers.Tool{
				{Name: agentcapabilities.PatrolActionCapabilitiesToolName},
				{Name: agentcapabilities.PatrolProposeActionToolName},
			},
			want: "no evidence tool",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := restrictInvestigationProviderToolsForResourceType(tc.tools, "vm"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestFilterToolsForPatrol_ConfigFlags(t *testing.T) {
	svc := &Service{
		cfg: &config.AIConfig{
			PatrolAnalyzeDocker:  false,
			PatrolAnalyzeStorage: false,
		},
	}

	tools := []providers.Tool{
		{Name: "pulse_query"},
		{Name: "pulse_alerts"},
		{Name: "patrol_get_findings"},
		{Name: "pulse_docker"},
		{Name: "pulse_storage"},
		{Name: "pulse_kubernetes"},
		{Name: "pulse_pmg"},
	}

	filtered := svc.filterToolsForPatrol(tools)

	if hasToolName(filtered, "pulse_docker") {
		t.Fatalf("expected pulse_docker to be excluded when PatrolAnalyzeDocker=false")
	}
	if hasToolName(filtered, "pulse_storage") {
		t.Fatalf("expected pulse_storage to be excluded when PatrolAnalyzeStorage=false")
	}
	if !hasToolName(filtered, "pulse_kubernetes") {
		t.Fatalf("expected pulse_kubernetes to be included for patrol")
	}
	if !hasToolName(filtered, "pulse_pmg") {
		t.Fatalf("expected pulse_pmg to be included for patrol")
	}
	if !hasToolName(filtered, "pulse_query") {
		t.Fatalf("expected pulse_query to remain included")
	}
	if hasToolName(filtered, "pulse_alerts") {
		t.Fatalf("expected interactive pulse_alerts to be excluded from the Patrol manifest")
	}
	if !hasToolName(filtered, "patrol_get_findings") {
		t.Fatalf("expected canonical patrol_get_findings to remain included")
	}
}

func TestFilterToolsForPatrol_DockerDisabled(t *testing.T) {
	svc := &Service{
		cfg: &config.AIConfig{
			PatrolAnalyzeDocker:  false,
			PatrolAnalyzeStorage: true,
		},
	}
	tools := []providers.Tool{
		{Name: "pulse_query"},
		{Name: "pulse_alerts"},
		{Name: "patrol_get_findings"},
		{Name: "pulse_docker"},
		{Name: "pulse_storage"},
	}

	filtered := svc.filterToolsForPatrol(tools)

	if hasToolName(filtered, "pulse_docker") {
		t.Fatalf("expected pulse_docker to be excluded")
	}
	if !hasToolName(filtered, "pulse_storage") {
		t.Fatalf("expected pulse_storage to remain included")
	}
}

func TestFilterToolsForPatrol_StorageDisabled(t *testing.T) {
	svc := &Service{
		cfg: &config.AIConfig{
			PatrolAnalyzeDocker:  true,
			PatrolAnalyzeStorage: false,
		},
	}
	tools := []providers.Tool{
		{Name: "pulse_query"},
		{Name: "pulse_docker"},
		{Name: "pulse_storage"},
	}

	filtered := svc.filterToolsForPatrol(tools)

	if hasToolName(filtered, "pulse_storage") {
		t.Fatalf("expected pulse_storage to be excluded")
	}
	if !hasToolName(filtered, "pulse_docker") {
		t.Fatalf("expected pulse_docker to remain included")
	}
}

func TestFilterToolsForPatrol_AllEnabled(t *testing.T) {
	svc := &Service{
		cfg: &config.AIConfig{
			PatrolAnalyzeDocker:  true,
			PatrolAnalyzeStorage: true,
		},
	}
	tools := []providers.Tool{
		{Name: "pulse_query"},
		{Name: "pulse_alerts"},
		{Name: "patrol_get_findings"},
		{Name: "pulse_docker"},
		{Name: "pulse_storage"},
		{Name: "pulse_kubernetes"},
		{Name: "pulse_pmg"},
	}

	filtered := svc.filterToolsForPatrol(tools)

	for _, name := range []string{"pulse_query", "patrol_get_findings", "pulse_docker", "pulse_storage", "pulse_kubernetes", "pulse_pmg"} {
		if !hasToolName(filtered, name) {
			t.Fatalf("expected %s to be included", name)
		}
	}
	if hasToolName(filtered, "pulse_alerts") {
		t.Fatalf("expected pulse_alerts to remain Assistant-only when every Patrol subsystem is enabled")
	}
}

func hasToolName(tools []providers.Tool, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}
