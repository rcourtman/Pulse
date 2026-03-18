package chat

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestFilterToolsForPatrol_ConfigFlags(t *testing.T) {
	svc := &Service{
		cfg: &config.AIConfig{
			PatrolAnalyzeDocker:  false,
			PatrolAnalyzeStorage: false,
		},
	}

	tools := []providers.Tool{
		{Name: "pulse_query"},
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
		{Name: "pulse_docker"},
		{Name: "pulse_storage"},
		{Name: "pulse_kubernetes"},
		{Name: "pulse_pmg"},
	}

	filtered := svc.filterToolsForPatrol(tools)

	for _, name := range []string{"pulse_query", "pulse_docker", "pulse_storage", "pulse_kubernetes", "pulse_pmg"} {
		if !hasToolName(filtered, name) {
			t.Fatalf("expected %s to be included", name)
		}
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
