package monitoring

import (
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/remoteconfig"
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

		assertDesiredConfigMetadata(t, cfg)
	})

	// Test Case 2: Agent without assignment
	t.Run("Agent without assignment", func(t *testing.T) {
		cfg := m.GetHostAgentConfig("other-agent")

		if len(cfg.Settings) != 0 {
			t.Errorf("Expected empty Settings for unassigned agent, got %v", cfg.Settings)
		}
		assertDesiredConfigMetadata(t, cfg)
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
		assertDesiredConfigMetadata(t, cfg)
	})
}

func TestGetHostAgentConfig_FingerprintIncludesCommandDecision(t *testing.T) {
	m := &Monitor{
		hostMetadataStore: config.NewHostMetadataStore(t.TempDir(), nil),
		config:            &config.Config{},
		state:             models.NewState(),
	}

	hostID := "agent-command-decision"
	before := m.GetHostAgentConfig(hostID)
	assertDesiredConfigMetadata(t, before)

	enabled := true
	if err := m.UpdateHostAgentConfig(hostID, &enabled); err != nil {
		t.Fatalf("UpdateHostAgentConfig: %v", err)
	}

	after := m.GetHostAgentConfig(hostID)
	assertDesiredConfigMetadata(t, after)
	if after.CommandsEnabled == nil || !*after.CommandsEnabled {
		t.Fatalf("expected commandsEnabled=true, got %#v", after.CommandsEnabled)
	}
	if before.DesiredConfig == nil || after.DesiredConfig == nil {
		t.Fatalf("expected desired config metadata before and after command decision")
	}
	if before.DesiredConfig.Hash == after.DesiredConfig.Hash {
		t.Fatalf("expected command decision to change desired config hash")
	}
}

func TestGetHostAgentConfig_UsesMergedProfileConfigForFingerprint(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "monitor_profiles_merged_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	persistence := config.NewConfigPersistence(tmpDir)
	parent := models.AgentProfile{
		ID:   "parent-profile",
		Name: "Parent Profile",
		Config: map[string]interface{}{
			"enable_docker": true,
			"log_level":     "warn",
		},
	}
	child := models.AgentProfile{
		ID:       "child-profile",
		Name:     "Child Profile",
		ParentID: "parent-profile",
		Config: map[string]interface{}{
			"interval":  "45s",
			"log_level": "debug",
		},
	}
	if err := persistence.SaveAgentProfiles([]models.AgentProfile{parent, child}); err != nil {
		t.Fatalf("Failed to save profiles: %v", err)
	}
	if err := persistence.SaveAgentProfileAssignments([]models.AgentProfileAssignment{{
		AgentID:   "agent-child",
		ProfileID: "child-profile",
	}}); err != nil {
		t.Fatalf("Failed to save assignments: %v", err)
	}

	m := &Monitor{
		persistence: persistence,
		config:      &config.Config{},
	}

	cfg := m.GetHostAgentConfig("agent-child")
	if cfg.Settings["enable_docker"] != true {
		t.Fatalf("expected inherited enable_docker=true, got %v", cfg.Settings["enable_docker"])
	}
	if cfg.Settings["interval"] != "45s" {
		t.Fatalf("expected child interval override, got %v", cfg.Settings["interval"])
	}
	if cfg.Settings["log_level"] != "debug" {
		t.Fatalf("expected child log_level override, got %v", cfg.Settings["log_level"])
	}
	assertDesiredConfigMetadata(t, cfg)

	childOnly := HostAgentConfig{CommandsEnabled: cfg.CommandsEnabled, Settings: child.Config}
	childOnlyExpected, err := remoteconfig.BuildDesiredConfigMetadata(childOnly.CommandsEnabled, childOnly.Settings)
	if err != nil {
		t.Fatalf("BuildDesiredConfigMetadata child only: %v", err)
	}
	if cfg.DesiredConfig.Hash == childOnlyExpected.Hash {
		t.Fatalf("expected inherited settings to affect desired config hash")
	}
}

func assertDesiredConfigMetadata(t *testing.T, cfg HostAgentConfig) {
	t.Helper()

	if cfg.DesiredConfig == nil {
		t.Fatalf("expected desired config metadata")
	}
	expected, err := remoteconfig.BuildDesiredConfigMetadata(cfg.CommandsEnabled, cfg.Settings)
	if err != nil {
		t.Fatalf("BuildDesiredConfigMetadata: %v", err)
	}
	if *cfg.DesiredConfig != expected {
		t.Fatalf("desired config metadata = %#v, want %#v", *cfg.DesiredConfig, expected)
	}
}
