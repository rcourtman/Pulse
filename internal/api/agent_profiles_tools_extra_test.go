package api

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestFormatValidationIssues(t *testing.T) {
	result := models.ValidationResult{
		Errors: []models.ValidationError{
			{Key: "bad", Message: "invalid"},
		},
		Warnings: []models.ValidationError{
			{Key: "warn", Message: "unknown"},
		},
	}
	msg := formatValidationIssues(result)
	if !strings.Contains(msg, "bad") || !strings.Contains(msg, "warning") {
		t.Fatalf("expected errors and warnings in message: %s", msg)
	}

	if formatValidationIssues(models.ValidationResult{}) != "unknown validation error" {
		t.Fatalf("expected fallback message")
	}
}

func TestBuildScopeProfileName(t *testing.T) {
	if buildScopeProfileName("", "agent") != "Patrol Scope: agent" {
		t.Fatalf("expected default name")
	}
	if buildScopeProfileName("agent", "agent") != "Patrol Scope: agent" {
		t.Fatalf("expected label equal to agent ID to be simplified")
	}
	if buildScopeProfileName("Alpha", "agent") != "Patrol Scope: Alpha (agent)" {
		t.Fatalf("expected label to be included")
	}
}

func TestMCPAgentProfileManager_ValidateSettings(t *testing.T) {
	manager := newTestProfileManager(t)
	if err := manager.validateSettings(map[string]interface{}{"unknown_key": true}); err == nil {
		t.Fatalf("expected validation error for warnings")
	}
	if err := manager.validateSettings(map[string]interface{}{"enable_host": "nope"}); err == nil {
		t.Fatalf("expected validation error for invalid type")
	}
}

func TestMCPAgentProfileManager_RequireLicense(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	licenseService := license.NewService()
	manager := NewMCPAgentProfileManager(persistence, licenseService)

	_, _, _, err := manager.ApplyAgentScope(context.Background(), "agent-1", "Alpha", map[string]interface{}{"enable_host": true})
	if err == nil {
		t.Fatalf("expected license error")
	}
}

func TestMCPAgentProfileManager_SaveVersion(t *testing.T) {
	manager := newTestProfileManager(t)
	profile := models.AgentProfile{
		ID:      "profile-1",
		Name:    "Default",
		Version: 2,
		Config: map[string]interface{}{
			"enable_host": true,
		},
	}
	if err := manager.saveVersion(profile, "note"); err != nil {
		t.Fatalf("unexpected saveVersion error: %v", err)
	}
	versions, err := manager.persistence.LoadAgentProfileVersions()
	if err != nil {
		t.Fatalf("unexpected load versions error: %v", err)
	}
	if len(versions) != 1 || versions[0].Version != 2 {
		t.Fatalf("expected version to be saved")
	}
}

func TestMCPAgentProfileManager_AssignProfile_NotFound(t *testing.T) {
	manager := newTestProfileManager(t)
	if _, err := manager.AssignProfile(context.Background(), "agent-1", "missing"); err == nil {
		t.Fatalf("expected error for missing profile")
	}
}

func TestMCPAgentProfileManager_GetScope_EmptyAgent(t *testing.T) {
	manager := newTestProfileManager(t)
	if _, err := manager.GetAgentScope(context.Background(), ""); err == nil {
		t.Fatalf("expected error for empty agent ID")
	}
}
