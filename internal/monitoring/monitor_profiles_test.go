package monitoring

import (
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestGetHostAgentConfig_WithProfiles(t *testing.T) {
	// Setup temp dir for persistence
	tmpDir, err := os.MkdirTemp("", "monitor_profiles_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize persistence
	persistence := config.NewConfigPersistence(tmpDir)

	// Create a profile
	profile := models.AgentProfile{
		ID:   "profile-1",
		Name: "Test Profile",
		Config: map[string]interface{}{
			"enable_docker": true,
			"log_level":     "debug",
		},
	}
	if err := persistence.SaveAgentProfiles([]models.AgentProfile{profile}); err != nil {
		t.Fatalf("Failed to save profiles: %v", err)
	}

	// Create an assignment
	assignment := models.AgentProfileAssignment{
		AgentID:   "agent-123",
		ProfileID: "profile-1",
	}
	if err := persistence.SaveAgentProfileAssignments([]models.AgentProfileAssignment{assignment}); err != nil {
		t.Fatalf("Failed to save assignments: %v", err)
	}

	// Initialize Monitor with persistence
	m := &Monitor{
		persistence: persistence,
		// minimal dependencies
		config: &config.Config{},
	}

	// Test Case 1: Agent with assigned profile
	t.Run("Agent with profile assignment", func(t *testing.T) {
		cfg := m.GetHostAgentConfig("agent-123")

		if cfg.Settings == nil {
			t.Fatal("Expected Settings to be non-nil")
		}

		if val, ok := cfg.Settings["enable_docker"]; !ok || val != true {
			t.Errorf("Expected enable_docker=true, got %v", val)
		}

		if val, ok := cfg.Settings["log_level"]; !ok || val != "debug" {
			t.Errorf("Expected log_level='debug', got %v", val)
		}
	})

	// Test Case 2: Agent without assignment
	t.Run("Agent without assignment", func(t *testing.T) {
		cfg := m.GetHostAgentConfig("other-agent")

		if len(cfg.Settings) != 0 {
			t.Errorf("Expected empty Settings for unassigned agent, got %v", cfg.Settings)
		}
	})

	// Test Case 3: Agent assigned to non-existent profile
	t.Run("Agent assigned to missing profile", func(t *testing.T) {
		badAssignment := models.AgentProfileAssignment{
			AgentID:   "agent-bad",
			ProfileID: "missing-profile",
		}
		if err := persistence.SaveAgentProfileAssignments([]models.AgentProfileAssignment{assignment, badAssignment}); err != nil {
			t.Fatalf("Failed to save assignments: %v", err)
		}

		cfg := m.GetHostAgentConfig("agent-bad")

		if len(cfg.Settings) != 0 {
			t.Errorf("Expected empty Settings for missing profile, got %v", cfg.Settings)
		}
	})
}
