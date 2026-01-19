package api

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func newTestProfileManager(t *testing.T) *MCPAgentProfileManager {
	t.Helper()
	persistence := config.NewConfigPersistence(t.TempDir())
	return NewMCPAgentProfileManager(persistence, nil)
}

func TestMCPAgentProfileManagerApplyAndGetScope(t *testing.T) {
	manager := newTestProfileManager(t)
	ctx := context.Background()

	settings := map[string]interface{}{
		"enable_host": true,
	}

	profileID, profileName, created, err := manager.ApplyAgentScope(ctx, "agent-1", "Alpha", settings)
	if err != nil {
		t.Fatalf("ApplyAgentScope error: %v", err)
	}
	if !created || profileID == "" || profileName == "" {
		t.Fatalf("unexpected apply result: id=%q name=%q created=%v", profileID, profileName, created)
	}

	scope, err := manager.GetAgentScope(ctx, "agent-1")
	if err != nil {
		t.Fatalf("GetAgentScope error: %v", err)
	}
	if scope == nil || scope.ProfileID != profileID || scope.ProfileVersion != 1 {
		t.Fatalf("unexpected scope: %+v", scope)
	}
	if scope.Settings["enable_host"] != true {
		t.Fatalf("unexpected settings: %+v", scope.Settings)
	}

	updatedSettings := map[string]interface{}{
		"enable_host": false,
	}
	_, _, created, err = manager.ApplyAgentScope(ctx, "agent-1", "Alpha", updatedSettings)
	if err != nil {
		t.Fatalf("ApplyAgentScope update error: %v", err)
	}
	if created {
		t.Fatal("expected update to reuse profile")
	}

	scope, err = manager.GetAgentScope(ctx, "agent-1")
	if err != nil {
		t.Fatalf("GetAgentScope error: %v", err)
	}
	if scope.ProfileVersion != 2 {
		t.Fatalf("expected profile version 2, got %d", scope.ProfileVersion)
	}
	if scope.Settings["enable_host"] != false {
		t.Fatalf("unexpected updated settings: %+v", scope.Settings)
	}
}

func TestMCPAgentProfileManagerAssignProfile(t *testing.T) {
	manager := newTestProfileManager(t)
	ctx := context.Background()

	profile := models.AgentProfile{
		ID:          "profile-1",
		Name:        "Default",
		Description: "default",
		Config: map[string]interface{}{
			"enable_host": true,
		},
		Version: 1,
	}

	if err := manager.persistence.SaveAgentProfiles([]models.AgentProfile{profile}); err != nil {
		t.Fatalf("SaveAgentProfiles error: %v", err)
	}

	name, err := manager.AssignProfile(ctx, "agent-2", profile.ID)
	if err != nil {
		t.Fatalf("AssignProfile error: %v", err)
	}
	if name != profile.Name {
		t.Fatalf("unexpected profile name: %q", name)
	}

	scope, err := manager.GetAgentScope(ctx, "agent-2")
	if err != nil {
		t.Fatalf("GetAgentScope error: %v", err)
	}
	if scope == nil || scope.ProfileID != profile.ID || scope.ProfileName != profile.Name {
		t.Fatalf("unexpected scope: %+v", scope)
	}
}

func TestMCPAgentProfileManagerGetScopeMissing(t *testing.T) {
	manager := newTestProfileManager(t)

	scope, err := manager.GetAgentScope(context.Background(), "missing")
	if err != nil {
		t.Fatalf("GetAgentScope error: %v", err)
	}
	if scope != nil {
		t.Fatalf("expected nil scope, got %+v", scope)
	}
}
